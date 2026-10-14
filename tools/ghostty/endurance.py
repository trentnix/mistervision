"""Exercise the built browser and real decoder repeatedly against a local fixture.

No user configuration, media-server account, terminal graphics, or audio device
is used. JSON reports retain timings and Linux resource samples for comparison.
"""

import argparse
import ctypes.util
import hashlib
import json
import os
from pathlib import Path
import shutil
import signal
import subprocess
import threading
import time
import unittest

from .fixtures.browser import BINARY, BrowserFixture, Scenario
from tools.raw_to_png import raw_to_png


def resources(pid):
    """Read Linux process identity and counters without collecting private arguments."""
    path = Path('/proc') / str(pid)
    try:
        stat = (path / 'stat').read_text().rsplit(')', 1)[1].split()
        status = dict(line.split(':', 1) for line in (path / 'status').read_text().splitlines())
        children = set()
        for task in (path / 'task').iterdir():
            try:
                children.update(map(int, (task / 'children').read_text().split()))
            except FileNotFoundError:
                pass
        try:
            fds = len(list((path / 'fd').iterdir()))
        except PermissionError:
            fds = None  # Linux can revoke access while a child exits.
        return {'pid': pid, 'start': stat[19], 'rss_kib': int(status.get('VmRSS', '0').split()[0]),
                'fds': fds, 'threads': int(status['Threads']),
                'children': sorted(children)}
    except (FileNotFoundError, ProcessLookupError):
        return None


def visible_pixels(data):
    """Return BGRX bytes with unused padding zeroed for visible-image comparisons."""
    pixels = bytearray(data)
    pixels[3::4] = bytes(len(pixels) // 4)
    return bytes(pixels)


def overlay_matches(display, decoder, controls):
    """Check the fixture's control region against the current clean decoder frame."""
    if not display or len(display) != len(decoder):
        return False
    if not controls:
        return display == decoder
    middle = len(display) // 2
    return display[:middle] == decoder[:middle] and display[middle:] != decoder[middle:]


class Endurance(BrowserFixture):
    """Run repeated controls, stream-open failures, cancellation, and recovery."""

    def __init__(self, cycles, seconds, report):
        super().__init__('exercise')
        self.cycles, self.seconds, self.report = cycles, seconds, report
        self.samples = []
        self.seen_children = {}
        self.frame_evidence = {}
        self.phase = "startup"

    def events(self):
        """Read complete diagnostic records, including the previous rotated file."""
        events = []
        for path in (Path(str(self.diagnostics) + '.1'), self.diagnostics):
            if path.exists():
                events.extend(json.loads(line) for line in path.read_bytes().split(b'\n')[:-1])
        return events

    def until(self, predicate, description, timeout=12):
        deadline = time.monotonic() + timeout
        while time.monotonic() < deadline:
            self.assertIsNone(self.process.poll(), 'browser exited: ' + description)
            result = predicate()
            if result:
                return result
            self.remember_children()
            time.sleep(.025)
        self.capture_frames('timeout')
        self.save_frames(description)
        self.fail('timed out: ' + description + '; recent events: ' + json.dumps(self.events()[-8:]))

    def remember_children(self):
        """Track decoder identities so PID reuse cannot hide or misidentify a leak."""
        sample = resources(self.process.pid)
        if sample:
            for pid in sample['children']:
                child = resources(pid)
                if child:
                    self.seen_children[pid] = child['start']

    def event(self, name, playback=None, **values):
        return next((e for e in reversed(self.events()) if e['msg'] == name
                     and (playback is None or e.get('playback') == playback)
                     and all(e.get(k) == v for k, v in values.items())), None)

    def stable_frame(self, controls=False):
        """Wait for settled video with controls present or fully removed.

        Pause delivery and decoder publication are asynchronous. Compare with
        the current decoder image, never a baseline saved before toggling UI.
        The fixture has no captions or segment prompts. Its controls occupy
        the lower half, leaving the upper half unchanged.
        """
        previous, since = None, time.monotonic()

        def settled():
            nonlocal previous, since
            sample = self.capture_frames('latest')
            display, decoder = sample.get('display'), sample.get('decoder')
            pair = (display, decoder)
            valid = sample['consistent'] and overlay_matches(display, decoder, controls)
            if not valid or pair != previous:
                previous, since = pair, time.monotonic()
            return display if valid and time.monotonic() - since >= .15 else None

        description = 'controls visible while paused' if controls else 'clean paused frame restored'
        return self.until(settled, description)

    def capture_frames(self, stage):
        """Retain one coherent decoder/display pair, or mark the read incomplete."""
        sample = {'phase': self.phase, 'time': time.time(), 'consistent': False}
        decoder = Path(str(self.frame) + '.video')
        before_decoder = None
        try:
            before_decoder = decoder.read_bytes()
        except OSError as error:
            sample['decoder_error'] = str(error)
        try:
            before_display = self.frame.stat()
            sample['display'] = self.frame.read_bytes()
            after_display = self.frame.stat()
            sample['display_consistent'] = (before_display.st_mtime_ns == after_display.st_mtime_ns
                                            and before_display.st_size == after_display.st_size == len(sample['display'])
                                            and before_display.st_ino == after_display.st_ino)
        except OSError as error:
            sample['display_error'] = str(error)
        try:
            sample['decoder'] = decoder.read_bytes()
        except OSError as error:
            sample['decoder_error'] = str(error)
        width, height = map(int, self.scenario.framebuffer.split('x'))
        sample['consistent'] = (sample.get('display_consistent', False)
                                and len(sample.get('display', b'')) == len(sample.get('decoder', b'')) == width * height * 4
                                and before_decoder == sample.get('decoder'))
        # The fourth byte is padding, not part of the displayed image.
        for name in ('display', 'decoder'):
            if name in sample:
                sample[name] = visible_pixels(sample[name])
        self.frame_evidence[stage] = sample
        return sample

    def save_frames(self, description):
        """Save fixture-only frames and differences when a wait fails.

        The difference image is not an alpha overlay. It shows every pixel that
        differs from the decoder, including overlays and presentation lag.
        """
        directory = self.report.with_suffix('.frames')
        directory.mkdir(parents=True, exist_ok=True)
        width, height = map(int, self.scenario.framebuffer.split('x'))
        size = width * height * 4
        summary = {'failure': description, 'width': width, 'height': height,
                   'pixel_format': 'BGRX8888 (padding zeroed)', 'stages': {}}
        for stage, sample in self.frame_evidence.items():
            details = {k: v for k, v in sample.items() if not isinstance(v, bytes)}
            for name in ('display', 'decoder'):
                data = sample.get(name)
                if data is None:
                    continue
                details[name] = {'bytes': len(data), 'sha256': hashlib.sha256(data).hexdigest()}
                raw = directory / f'{stage}-{name}.raw'
                raw.write_bytes(data)
                if len(data) == size:
                    raw_to_png(raw, width, height, raw.with_suffix('.png'))
            display, decoder = sample.get('display', b''), sample.get('decoder', b'')
            if len(display) == len(decoder) == size:
                difference = bytearray(size)
                changed = 0
                for offset in range(0, size, 4):
                    if display[offset:offset+3] != decoder[offset:offset+3]:
                        difference[offset:offset+3] = b'\xff\xff\xff'
                        changed += 1
                details['different_pixels'] = changed
                raw = directory / f'{stage}-difference.raw'
                raw.write_bytes(difference)
                raw_to_png(raw, width, height, raw.with_suffix('.png'))
            summary['stages'][stage] = details
        (directory / 'summary.json').write_text(json.dumps(summary, indent=2) + '\n')

    def command(self, command, **values):
        self.remote_commands.put({'MessageType': 'Playstate', 'Data': {'Command': command, **values}})

    def play(self, item, failure=False):
        previous = max((e.get('playback', 0) for e in self.events()), default=0)
        ready = max((e.get('generation', 0) for e in self.events()), default=0)
        started = time.monotonic()
        self.remote_commands.put({'MessageType': 'Play', 'Data': {
            'PlayCommand': 'PlayNow', 'ItemIds': [item], 'StartPositionTicks': 0}})
        event = self.until(lambda: next((e for e in self.events()
                           if e['msg'] == 'playback.start' and e['playback'] > previous), None), 'new playback')
        playback = event['playback']
        if failure:
            self.until(lambda: self.event('playback.end', playback, failed=True), 'failed stream reported')
        else:
            self.until(lambda: self.event('playback.first-position', playback), 'decoded first frame')
            self.until(lambda: any(e['msg'] == 'browser.playback-ready' and e['generation'] > ready
                                   for e in self.events()), 'browser accepted playback')
        return playback, (time.monotonic() - started) * 1000

    def stop_playback(self, playback):
        self.command('Stop')
        self.until(lambda: self.event('playback.end', playback), 'playback stopped')
        self.until(lambda: not resources(self.process.pid)['children'], 'decoder reaped')

    def exercise(self):
        started = time.monotonic()
        outcome = {'passed': False, 'samples': self.samples}
        self.addCleanup(self.write_report, outcome, started)
        self.addCleanup(self.reap_leaked_children)
        media = subprocess.check_output([
            'ffmpeg', '-v', 'error', '-f', 'lavfi', '-i', 'testsrc2=s=320x180:r=24',
            '-f', 'lavfi', '-i', 'sine=frequency=440:sample_rate=48000', '-t', '30',
            '-c:v', 'mpeg2video', '-c:a', 'mp2', '-f', 'mpegts', 'pipe:1'], timeout=30)
        browser_started = time.monotonic()
        self.start_browser(Scenario(player='decode', media=media, remote_control=True))
        self.addCleanup(self.save_evidence)
        startup_ms = (time.monotonic() - browser_started) * 1000
        self.until(lambda: any(p == '/Sessions/Capabilities/Full' for p, _ in self.reports), 'remote registration')
        navigation_started = time.monotonic()
        self.key(b'b')
        self.wait_request('/Items', ParentId='view-movies', StartIndex=0)
        navigation_ms = (time.monotonic() - navigation_started) * 1000
        cycle = 0
        while cycle < self.cycles or time.monotonic() - started < self.seconds:
            self.frame_evidence.clear()
            self.phase = f'cycle {cycle + 1}: starting'
            item = f'movie-tricky-{cycle % 2}'
            playback, ready_ms = self.play(item)
            # Observe a real moving image, then exercise local controls.
            frame = visible_pixels(self.read_frame())
            self.until(lambda: visible_pixels(self.read_frame()) != frame, 'decoded frame advancement')
            self.command('Pause')
            self.until(lambda: self.event('playback.pause', playback, paused=True), 'pause')
            self.phase = f'cycle {cycle + 1}: pause and controls'
            self.stable_frame()
            self.frame_evidence['paused'] = self.frame_evidence['latest']
            self.key(b'\x1b[A')
            self.stable_frame(controls=True)
            self.frame_evidence['controls'] = self.frame_evidence['latest']
            self.key(b'\x1b[A')
            clean = self.stable_frame()
            self.frame_evidence['hidden'] = self.frame_evidence['latest']
            self.phase = f'cycle {cycle + 1}: seek and recovery'
            self.command('Unpause')
            self.until(lambda: self.event('playback.pause', playback, paused=False), 'resume')
            self.until(lambda: visible_pixels(self.read_frame()) != clean, 'motion after hiding controls')
            seek_started = time.monotonic()
            self.key(b'll')
            next_start = self.until(lambda: next((e for e in self.events() if
                e['msg'] == 'playback.start' and e['playback'] > playback), None), 'seek replacement')
            playback = next_start['playback']
            self.until(lambda: self.event('playback.first-position', playback), 'frame after seek')
            seek_ms = (time.monotonic() - seek_started) * 1000
            self.stop_playback(playback)
            # The next attempt fails at stream open. Restore service and retry.
            self.media_unavailable.set()
            self.play(item, failure=True)
            self.media_unavailable.clear()
            playback, recovery_ms = self.play(item)
            self.stop_playback(playback)
            # Hold response headers, then cancel before a decoder can start.
            gate = threading.Event()
            self.video_response_gate = gate
            before = len(self.requests)
            self.remote_commands.put({'MessageType': 'Play', 'Data': {
                'PlayCommand': 'PlayNow', 'ItemIds': [item]}})
            self.until(lambda: any('/Videos/' in p for p in self.requests[before:]), 'blocked stream request')
            pending = max(e.get('playback', 0) for e in self.events())
            try:
                self.command('Stop')
                self.until(lambda: self.event('playback.end', pending, canceled=True), 'canceled stream cleanup')
            finally:
                gate.set()
                self.video_response_gate = None
            self.until(lambda: not resources(self.process.pid)['children'], 'no orphan decoder')
            # A recovered run must still decode after cancellation.
            playback, _ = self.play(item)
            self.stop_playback(playback)
            sample = resources(self.process.pid)
            sample.update(cycle=cycle + 1, playback_ready_ms=ready_ms, seek_ms=seek_ms, recovery_ms=recovery_ms)
            self.samples.append(sample)
            print(f'Cycle {cycle + 1}: decoded, sought, recovered; RSS {sample["rss_kib"]} KiB', flush=True)
            cycle += 1
        self.stop()
        self.assertEqual(self.process.returncode, 0)
        for pid, identity in self.seen_children.items():
            child = resources(pid)
            self.assertTrue(child is None or child['start'] != identity, 'orphan decoder after exit')
        outcome.update(passed=True, startup_ms=startup_ms, navigation_ms=navigation_ms)

    def reap_leaked_children(self):
        """Clean up tracked decoders even if the main test assertion fails."""
        leaked = []
        for pid, identity in self.seen_children.items():
            child = resources(pid)
            if child is not None and child['start'] == identity:
                leaked.append(pid)
                try:
                    os.kill(pid, signal.SIGKILL)
                except ProcessLookupError:
                    pass
        self.assertFalse(leaked, f'decoders survived browser shutdown: {leaked}')

    def save_evidence(self):
        """Keep fixture diagnostics before temporary data is removed."""
        self.report.parent.mkdir(parents=True, exist_ok=True)
        self.report.with_suffix('.events.json').write_text(json.dumps(self.events(), indent=2) + '\n')

    def write_report(self, outcome, started):
        outcome['elapsed_seconds'] = time.monotonic() - started
        outcome['binary'] = str(BINARY)
        with BINARY.open('rb') as binary:
            outcome['binary_sha256'] = hashlib.file_digest(binary, 'sha256').hexdigest()
        outcome['resources'] = {name: {'first': self.samples[0][name], 'last': self.samples[-1][name],
                                      'min': min(s[name] for s in self.samples),
                                      'max': max(s[name] for s in self.samples)}
                                for name in ('rss_kib', 'fds', 'threads')
                                if self.samples and all(s[name] is not None for s in self.samples)}
        self.report.parent.mkdir(parents=True, exist_ok=True)
        self.report.write_text(json.dumps(outcome, indent=2) + '\n')


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--cycles', type=int, default=2)
    parser.add_argument('--seconds', type=int, default=0, help='minimum run time; finish the current cycle')
    parser.add_argument('--report', type=Path, default=Path('build/reports/endurance.json'))
    args = parser.parse_args()
    if args.cycles < 1 or args.seconds < 0:
        parser.error('cycles must be positive and seconds must be nonnegative')
    if not BINARY.is_file() or not shutil.which('ffmpeg') or not ctypes.util.find_library('mpv') or not Path('/proc/self/status').is_file():
        parser.error('build the host client and install FFmpeg and libmpv on Linux')
    result = unittest.TextTestRunner(verbosity=2).run(Endurance(args.cycles, args.seconds, args.report))
    raise SystemExit(not result.wasSuccessful())


if __name__ == '__main__':
    main()
