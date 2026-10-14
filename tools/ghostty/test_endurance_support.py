"""Check endurance synchronization, failure evidence, and process sampling."""

import json
import os
from pathlib import Path
import tempfile
import unittest
from unittest.mock import Mock, patch

from tools.ghostty import endurance
from tools.ghostty.endurance import resources
from tools.ghostty.fixtures.browser import Scenario, websocket_frames


def masked(opcode, payload):
    mask = b'1234'
    length = bytes([len(payload) | 128]) if len(payload) < 126 else b'\xfe' + len(payload).to_bytes(2, 'big')
    return bytes([0x80 | opcode]) + length + mask + bytes(b ^ mask[i % 4] for i, b in enumerate(payload))


class EnduranceSupportTests(unittest.TestCase):
    def test_partial_and_coalesced_heartbeat_frames(self):
        ping = masked(9, b'ping')
        buffer = bytearray(ping[:5])
        self.assertEqual(list(websocket_frames(buffer)), [])
        buffer.extend(ping[5:] + masked(1, b'x' * 130) + masked(8, b''))
        self.assertEqual(list(websocket_frames(buffer)), [(9, b'ping'), (1, b'x' * 130), (8, b'')])
        self.assertEqual(buffer, b'')

    def test_oversized_or_unmasked_frames_are_rejected(self):
        for payload in (b'\x89\x01x', b'\x81\xff' + (2 << 20).to_bytes(8, 'big')):
            with self.subTest(payload=payload), self.assertRaises(ValueError):
                list(websocket_frames(bytearray(payload)))

    def test_resource_sample_records_identity_and_absent_process(self):
        sample = resources(os.getpid())
        self.assertEqual(sample['pid'], os.getpid())
        self.assertGreater(sample['rss_kib'], 0)
        self.assertGreater(sample['fds'], 0)
        self.assertTrue(sample['start'].isdigit())
        self.assertIsNone(resources(-1))


class EnduranceFrameTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.harness = endurance.Endurance(1, 0, self.root / 'report.json')
        self.harness.frame = self.root / 'frame.raw'
        self.harness.scenario = Scenario(framebuffer='2x2')
        self.decoder = Path(str(self.harness.frame) + '.video')
        self.clean = bytes([10, 20, 30, 0]) * 4
        self.controls = self.clean[:8] + bytes([40, 50, 60, 0]) * 2

    def test_video_advancement_is_not_controls(self):
        newer = bytes([70, 80, 90, 0]) * 4
        # A new frame used to satisfy the "controls visible" assertion just
        # because it differed from an earlier baseline.
        self.assertFalse(endurance.overlay_matches(newer, newer, True))
        self.assertFalse(endurance.overlay_matches(self.clean, newer, True))
        self.assertTrue(endurance.overlay_matches(self.controls, self.clean, True))
        self.assertFalse(endurance.overlay_matches(self.controls, self.clean, False))
        self.assertTrue(endurance.overlay_matches(newer, newer, False))

    def test_capture_rejects_partial_display_and_missing_decoder(self):
        self.decoder.write_bytes(self.clean)
        self.harness.frame.write_bytes(self.clean[:8])
        self.assertFalse(self.harness.capture_frames('partial')['consistent'])
        self.decoder.unlink()
        sample = self.harness.capture_frames('missing')
        self.assertIn('decoder_error', sample)
        self.assertEqual(sample['display'], self.clean[:8])

    def test_capture_rejects_decoder_publication_during_display_read(self):
        self.decoder.write_bytes(self.clean)
        self.harness.frame.write_bytes(self.clean)
        read_bytes = Path.read_bytes

        def read(path):
            data = read_bytes(path)
            if path == self.harness.frame:
                self.decoder.write_bytes(bytes([70, 80, 90, 0]) * 4)
            return data

        with patch.object(Path, 'read_bytes', read):
            self.assertFalse(self.harness.capture_frames('changed')['consistent'])

    def test_padding_does_not_count_as_visible_pixels(self):
        self.decoder.write_bytes(bytes([10, 20, 30, 255]) * 4)
        self.harness.frame.write_bytes(self.clean)
        sample = self.harness.capture_frames('padding')
        self.assertTrue(sample['consistent'])
        self.assertEqual(sample['decoder'], sample['display'])
        self.assertEqual(endurance.visible_pixels(bytes([10, 20, 30, 255]) * 4), self.clean)

    def test_late_frame_resets_settling_without_requiring_old_baseline(self):
        self.decoder.write_bytes(self.clean)
        self.harness.frame.write_bytes(self.clean)
        newer = bytes([70, 80, 90, 0]) * 4
        clock = [0.0]
        observed = []

        def until(predicate, description):
            for step in range(20):
                clock[0] = step * .05
                if step == 2:
                    # A queued decoder frame arrives after pause was requested.
                    self.decoder.write_bytes(newer)
                if step == 4:
                    self.harness.frame.write_bytes(newer)
                result = predicate()
                observed.append(result)
                if result:
                    return result
            self.fail(description)

        with patch.object(self.harness, 'until', until), patch.object(endurance.time, 'monotonic', lambda: clock[0]):
            self.assertEqual(self.harness.stable_frame(), newer)
        self.assertFalse(any(observed[:7]), 'accepted an unsettled or stale frame')

    def test_stuck_controls_do_not_pass_clean_frame_wait(self):
        self.decoder.write_bytes(self.clean)
        self.harness.frame.write_bytes(self.controls)
        clock = [0.0]

        def until(predicate, description):
            for step in range(20):
                clock[0] = step * .05
                self.assertFalse(predicate())
            raise AssertionError(description)

        with patch.object(self.harness, 'until', until), patch.object(endurance.time, 'monotonic', lambda: clock[0]):
            with self.assertRaisesRegex(AssertionError, 'clean paused frame restored'):
                self.harness.stable_frame()

    def test_timeout_writes_failure_frames(self):
        self.decoder.write_bytes(self.clean)
        self.harness.frame.write_bytes(self.controls)
        self.harness.process = Mock()
        self.harness.process.poll.return_value = None
        with patch.object(self.harness, 'events', return_value=[]), \
                patch.object(endurance.time, 'monotonic', side_effect=[0, 13]):
            with self.assertRaisesRegex(AssertionError, 'timed out: controls stuck'):
                self.harness.until(lambda: False, 'controls stuck')
        summary = json.loads((self.harness.report.with_suffix('.frames') / 'summary.json').read_text())
        self.assertEqual(summary['failure'], 'controls stuck')
        self.assertEqual(summary['stages']['timeout']['different_pixels'], 2)

    def test_failure_evidence_keeps_decoder_display_and_difference(self):
        self.decoder.write_bytes(self.clean)
        self.harness.frame.write_bytes(self.controls)
        self.harness.capture_frames('controls')
        self.harness.frame.write_bytes(self.clean[:8])
        self.harness.capture_frames('partial')
        self.harness.save_frames('clean paused frame restored')
        directory = self.harness.report.with_suffix('.frames')
        summary = json.loads((directory / 'summary.json').read_text())
        self.assertEqual(summary['stages']['controls']['different_pixels'], 2)
        self.assertFalse(summary['stages']['partial']['consistent'])
        for name in ('display', 'decoder', 'difference'):
            self.assertTrue((directory / f'controls-{name}.png').read_bytes().startswith(b'\x89PNG'))
        self.assertEqual((directory / 'controls-decoder.raw').read_bytes(), self.clean)
        self.assertEqual((directory / 'partial-display.raw').read_bytes(), self.clean[:8])
        self.assertFalse((directory / 'partial-display.png').exists())
