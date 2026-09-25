# MiSTerVision v1.6.0 announcement images

These 1920×1080 PNGs were captured from the v1.6.0 application with a 16:9 output.

- [Widescreen carousel](home-widescreen.png)
- [Music with artist artwork](music-artwork-widescreen.png)
- [Now Spinning](music-spinning-widescreen.png)

## Release notes

- Backgrounds now fill widescreen displays throughout browsing, previews, music playback, and setup screens. The carousel shows more neighboring libraries across the full display width.
- Music playback has a cleaner layout: bold artist, song, and album information at the lower left, with timing aligned above the progress bar. The music meters and persistent header have been removed.
- Music remembers your background choice when you stop and start another song. Background options skip unavailable artwork, and loading artwork no longer briefly shows an unrelated effect.
- Text rendering is clearer in lists and the exit confirmation. Fixed a crash while scrolling long headings.

Validated with automated playback and rendering tests and on-device testing across HDMI, 240p CRT, and 480i CRT output. The final music layout was checked on 240p CRT and 720p HDMI.

Install or update using the existing instructions. The download keeps the `progressive.zip` filename and includes support for interlaced output through configuration.
