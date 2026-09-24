"""Exercise framebuffer previews through the real browser and native presenter.

The fixed-frame helper isolates composition from decoding. The optional native
check delegates to MiSTer's production validation without executing ARM code.
"""

import os
import shutil
import subprocess
import unittest
from pathlib import Path
import time
from urllib.parse import urlparse

from .fixtures.browser import BrowserFixture, Scenario
from tools.raw_to_png import raw_to_png


class DisplayGeometryTests(BrowserFixture):
    def open_movie(self):
        self.key(b"b")
        self.wait_request("/Items", ParentId="view-movies", StartIndex=0)
        self.key(b"b")
        self.wait_request("/Items/movie-tricky-0")
        self.key(b"b")

    def capture(self, name, frame):
        """Optionally retain deterministic screenshots for local inspection."""
        directory = os.environ.get("MISTERVISION_GEOMETRY_PREVIEWS")
        if directory:
            path = Path(directory)
            path.mkdir(parents=True, exist_ok=True)
            raw = path / (name + ".raw")
            raw.write_bytes(frame)
            width, height = map(int, self.scenario.framebuffer.split("x"))
            raw_to_png(raw, width, height, path / (name + ".png"))
            raw.unlink()

    def test_hd_native_rejection_before_opening_stream(self):
        self.start_browser(Scenario(player="inline", framebuffer="1922x1080", mister_display_check=True))
        self.open_movie()
        self.wait_event("playback.end", stage="decoder-configuration", error_kind="unsupported-display", failed=True)
        self.assertFalse(any(urlparse(request).path.endswith("/stream") for request in self.requests))
        self.assertFalse((self.directory / "picture-modes").exists(), "decoder unexpectedly launched")
        # Wait for the browser to present the result after receiving the failure.
        time.sleep(.15)
        self.capture("oversized-native-rejection", self.read_frame())
        self.assertIsNone(self.process.poll())

    def exercise_preview(self, dimensions, native_check=False):
        self.start_browser(Scenario(player="inline", framebuffer=dimensions, mister_display_check=native_check))
        self.open_movie()
        self.wait_video_ready()
        width, height = map(int, dimensions.split("x"))
        viewport = width
        margin = (width - viewport) // 2
        row = bytes(margin * 4) + bytes([23]) * viewport * 4 + bytes(margin * 4)
        clean = row * height
        deadline = time.monotonic() + 5
        while self.read_frame() != clean:
            self.assertLess(time.monotonic(), deadline, "logical video frame did not fill the viewport")
            time.sleep(.02)
        self.key(b"\x1b[A")
        deadline = time.monotonic() + 5
        while True:
            overlay = self.read_frame()
            if overlay != clean:
                break
            self.assertLess(time.monotonic(), deadline, "controls did not appear")
            time.sleep(.02)
        # Overlays do not change video outside their own bounds.
        for y in range(height):
            start = y * width * 4
            self.assertEqual(overlay[start:start+margin*4], bytes(margin*4))
            self.assertEqual(overlay[start+(width-margin)*4:start+width*4], bytes(margin*4))
        self.capture(dimensions + "-controls", overlay)
        self.key(b"\x1b[A")
        deadline = time.monotonic() + 5
        while self.read_frame() != clean:
            self.assertLess(time.monotonic(), deadline, "dismissed controls left stale pixels")
            time.sleep(.02)
        self.key(b"a")
        self.wait_event("playback.end", failed=False)

    def test_480_line_preview_accepts_native_geometry(self):
        self.exercise_preview("640x480", native_check=True)

    def test_scaled_1080p_canvas(self):
        self.exercise_preview("480x270", native_check=True)

    def test_scaled_720p_canvas(self):
        self.exercise_preview("640x360", native_check=True)

    def test_vga_1280x480_reduced_canvas(self):
        self.exercise_preview("640x240", native_check=True)

    def test_vga_720x480_reduced_canvas(self):
        self.exercise_preview("360x240", native_check=True)

    def test_720p_preview_composes_and_clears_controls(self):
        self.exercise_preview("1280x720")

    def test_1080p_preview_composes_and_clears_controls(self):
        self.exercise_preview("1920x1080")

    def exercise_decoded_preview(self, dimensions, source_size="320x180", display_aspect="auto"):
        media = subprocess.check_output([
            "ffmpeg", "-v", "error", "-f", "lavfi", "-i", f"color=red:s={source_size}:r=30",
            "-t", "8", "-c:v", "mpeg2video", "-f", "mpegts", "pipe:1"], timeout=30)
        self.start_browser(Scenario(player="decode", framebuffer=dimensions, media=media, settings={"display": {"aspect_ratio": display_aspect}}))
        self.open_movie()
        self.wait_event("playback.first-position")
        width, height = map(int, dimensions.split("x"))
        def pixel(frame, x, y):
            offset = (y * width + x) * 4
            return frame[offset:offset+3]
        deadline = time.monotonic() + 5
        while True:
            frame = self.read_frame()
            center = pixel(frame,width//2,height//2)
            if center[2] > 230 and center[0] < 30 and center[1] < 30:
                break
            self.assertLess(time.monotonic(), deadline, "decoded video did not reach the physical frame")
            time.sleep(.02)
        # Check actual decoded pixels, including overrides for non-square displays.
        sw, sh = map(int, source_size.split("x"))
        target = 4 / 3 if dimensions in ("640x240", "640x288", "640x480", "640x576") else width / height
        if display_aspect == "4:3":
            target = 4 / 3
        elif display_aspect == "16:9":
            target = 16 / 9
        top = pixel(frame, width//2, height//32)
        side = pixel(frame, width//32, height//2)
        if sw/sh > target + .01:
            self.assertEqual(top, bytes(3))
        else:
            self.assertGreater(top[2], 230)
        if sw/sh < target - .01:
            self.assertEqual(side, bytes(3))
        else:
            self.assertGreater(side[2], 230)
        self.capture(dimensions + "-decoded", frame)
        self.key(b"a")
        self.wait_event("playback.end", failed=False)

    @unittest.skipUnless(shutil.which("ffmpeg"), "FFmpeg required")
    def test_real_video_on_vga_super_resolution(self):
        self.exercise_decoded_preview("640x240", "320x240")

    @unittest.skipUnless(shutil.which("ffmpeg"), "FFmpeg required")
    def test_wide_video_on_vga_super_resolution(self):
        self.exercise_decoded_preview("640x240")

    @unittest.skipUnless(shutil.which("ffmpeg"), "FFmpeg required")
    def test_real_video_at_480_lines(self):
        self.exercise_decoded_preview("640x480")

    @unittest.skipUnless(shutil.which("ffmpeg"), "FFmpeg required")
    def test_real_video_at_720p(self):
        self.exercise_decoded_preview("1280x720")

    @unittest.skipUnless(shutil.which("ffmpeg"), "FFmpeg required")
    def test_real_video_at_1080p(self):
        self.exercise_decoded_preview("1920x1080")

    @unittest.skipUnless(shutil.which("ffmpeg"), "FFmpeg required")
    def test_real_video_at_reduced_1080p(self):
        self.exercise_decoded_preview("480x270")

    @unittest.skipUnless(shutil.which("ffmpeg"), "FFmpeg required")
    def test_four_three_source_on_wide_display(self):
        self.exercise_decoded_preview("640x360", "320x240")

    @unittest.skipUnless(shutil.which("ffmpeg"), "FFmpeg required")
    def test_cinema_source_on_wide_display(self):
        self.exercise_decoded_preview("640x360", "384x160")

    @unittest.skipUnless(shutil.which("ffmpeg"), "FFmpeg required")
    def test_explicit_four_three_display(self):
        self.exercise_decoded_preview("640x360", display_aspect="4:3")

    @unittest.skipUnless(shutil.which("ffmpeg"), "FFmpeg required")
    def test_explicit_wide_display_with_four_three_raster(self):
        self.exercise_decoded_preview("640x480", display_aspect="16:9")
