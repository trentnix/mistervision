# MiSTerVision v1.6.0 page-flipping application test

This experimental build uses the v1.6.0 application and the page-flipping player tested in issue #22. It is not a stable release or an updater package.

1. Extract the ZIP to the SD card root, preserving its folders. The test has its own binary, player, and Scripts entry. It does not replace the normal installation or edit MiSTer.ini.
2. Exit MiSTerVision. Use the standard MiSTer Scripts menu with progressive output (`display.interlaced` must be false). Keep your working HDMI or VGA timing.
3. Run **MiSTerVision-Pageflip-App-Test**. It uses your existing connections and account state. Do not run the updater from the test app.
4. Play the same Plex sample, then a movie or episode that previously tore. Check tearing, smoothness, and audio sync. Show and hide controls, pause and resume, seek while playing and paused, and switch picture modes. Stop playback and confirm browsing still works. Repeat with VGA if available, keeping supported monitor timings.
5. Exit normally. Zip the new `run-...` folder under `/media/fat/mistervision-pageflip-app-test/logs/` and share it with your observations. Review logs before sharing. Do not include settings or sign-in files.

Main is paused while the test app owns the framebuffer hardware. The supervisor stops child players and resumes Main on normal exit or application failure. The normal MiSTerVision Scripts entry remains available for comparison. The test caps progressive framebuffer dimensions at 640×480 and leaves output timings unchanged.

Remove `Scripts/MiSTerVision-Pageflip-App-Test.sh` and `mistervision-pageflip-app-test` to uninstall the test. Existing account and playback state is shared with the normal app.

## Source and build

The test branch starts at v1.6.0. The native player sources and build inputs are unchanged between v1.5.2 and v1.6.0, so this package reuses the exact instrumented page-flipping player from the successful v3 comparison. Its corresponding source is published as `mplayer-pageflip-source.tar.gz` on the diagnostic release. The opt-in patch and instrumentation are included in this branch under `tools/diagnostics`.

Build the Go executable with `ZIG=/path/to/zig make arm VERSION=v1.6.0-pageflip-test.1`, then run `python3 tools/diagnostics/package_app_pageflip.py DESTINATION PLAYER MPLAYER_LICENSE`. Both the application source and native player source accompany the diagnostic download.
