# Display setup

The default keeps MiSTer's current display, normally 240p for NTSC or 288p for PAL. Optional interlaced output uses a standalone Menu core at 480i or 576i. Synchronization is automatic. Exit the application before changing settings.

## Choose an output

Start with the progressive installation package and `display.interlaced: false`. This keeps the current MiSTer display mode. The package includes everything needed to enable interlacing later. Its name does not force 240p or select a connector.

| Connection | Start here |
| --- | --- |
| Analog CRT | Keep working CRT timing. The analog scaler must display the Linux framebuffer. Use settings for your adapter. |
| Super Video Custard composite | Use the tested NTSC 240p configuration below. |
| HDMI or HDMI-to-DVI monitor | Keep interlacing off and use a supported HDMI timing. Keep the default framebuffer limits. |
| Optional 480i CRT output | Establish working output first, then follow [interlaced setup](#enable-or-disable-interlaced-output). |

The HDMI scaling and aspect improvements in this guide describe current source and are not included in v1.4.2.

## Composite and S-Video output

MiSTerVision draws through the Linux framebuffer. Analog output must display that framebuffer through the scaler. If the CRT says to disable the framebuffer or enable the VGA scaler, use `vga_scaler=1` with a CRT-compatible timing. Disabling `fb_terminal` does not solve the application’s display requirement.

The following **NTSC 240p** settings worked over composite through a Super Video Custard on a MiSTer Multisystem 2. Back up `/media/fat/MiSTer.ini`, then merge these keys into its existing `[Menu]` section. Preserve other sections. Exit MiSTerVision before editing and reload the Menu core afterward.

```ini
[Menu]
vga_mode=rgb
vga_scaler=1
vga_sog=0
composite_sync=0
forced_scandoubler=0
direct_video=0
fb_terminal=1
menu_pal=0
video_mode=640,30,60,70,240,4,4,14,12587
vscale_mode=0
vscale_border=8
```

Set `display.interlaced` to `false` in `settings.json` for this configuration. The border value suits my CRT’s overscan and can be adjusted. These are adapter-specific settings, not a preset for every composite or S-Video connection. The Custard converts RGB to composite/S-Video, so it uses `vga_mode=rgb`. Other adapters may require different settings.

Follow the [Custard manual](https://multisystem.uk/media/2025/04/Super_Video_Custard_Manual.pdf) for its NTSC/PAL, termination, and sync-on-green switches. Its normal game-core instructions use `vga_scaler=0`. MiSTerVision requires the framebuffer route above instead. Composite browsing and playback have been tested. S-Video shares the adapter’s input, but S-Video and 480i through the Custard have not been tested. The SS1 S-Video path also needs separate validation.

## HDMI output

For a conventional HDMI or HDMI-to-DVI monitor, use a timing supported by the monitor. For example, merge these keys into the existing `[Menu]` section of `/media/fat/MiSTer.ini` and set `display.interlaced` to `false` in `settings.json`:

```ini
[Menu]
direct_video=0
vga_scaler=0
fb_terminal=1
video_mode=0
vscale_mode=0
vscale_border=0
```

`video_mode=0` selects 720p60. Use `video_mode=8` for 1080p60. Replace any existing custom `[Menu]` timing when switching. This example leaves analog output on the core’s native path, which does not display MiSTerVision’s framebuffer. Do not enable the analog scaler with these HD timings on a standard-definition CRT or the Custard.

## Simultaneous analog and HDMI output

Many game cores send their native picture to analog while the scaler produces a separate HD picture for HDMI. MiSTerVision’s Linux framebuffer needs the scaler on analog too. With `vga_scaler=1`, analog uses the timing selected by `video_mode`, as HDMI does. A 240p timing can work on the CRT while leaving an HDMI/DVI monitor blank because that monitor does not accept the timing. A passive HDMI-to-DVI adapter does not upscale it. See [MiSTer’s video settings](https://mister-devel.github.io/MkDocs_MiSTer/advanced/ini/).

Simultaneous display therefore requires both displays to accept the output timing, or an external scaler to convert the CRT-compatible signal for the modern display. MiSTerVision does not currently provide independent HD and CRT framebuffer outputs. Simultaneous output has not been validated. The interlaced RGB path also enables `direct_video`, which MiSTer documents as incompatible with ordinary HDMI monitors. See [Direct Video](https://mister-devel.github.io/MkDocs_MiSTer/advanced/directvideo/).

## HDMI framebuffer scaling

On non-CRT framebuffers, the client asks Main to reduce the framebuffer while the FPGA scaler enlarges it across the existing output. HDMI timing stays unchanged. The default ceiling is 640×480. Both 1280×720 and 1920×1080 signals use a 640×360 framebuffer, with a centered 4:3 browsing viewport. Video fills the screen according to `display.aspect_ratio`. Overlays retain their proportions within a centered 4:3 area. Native 640×240, 640×288, 640×480, and 640×576 CRT rasters keep their existing paths, including full-height interlaced playback.

Keep the default limits for initial use. On the maintainer’s 1080p HDMI setup, Live TV dropped frames with a 960×540 framebuffer and became much smoother after returning to 640×360. The smaller framebuffer also produced satisfactory browsing and overlay quality. One framebuffer size applies to the entire application, including browsing and all video playback.

Browsing text and artwork render at the pixels available in the centered 4:3 viewport. The logical layout stays 640×288, so raster resolution does not change visible rows or menu proportions. Navigation hints and the clock retain their bitmap font. Native CRT rendering and video overlay resolution keep their existing paths.

### Optional framebuffer limits

`display.framebuffer_max_width` and `display.framebuffer_max_height` default to 640 and 480. Omitted or zero values use those defaults. Valid nonzero limits are 320–1920 pixels wide and 240–1080 pixels high. Invalid values stop startup.

Main supports divisors of one through four. The client selects the smallest divisor whose even-sized framebuffer fits both limits. If no divisor fits, startup stops and restores Menu. These are framebuffer ceilings, not HDMI signal settings.

Higher limits are experimental. They increase work for both browsing and playback, even when the server sends smaller video. A 960×540 framebuffer provides a 720×540 browsing raster. A 1280×720 framebuffer provides 960×720. An isolated MiSTer benchmark measured approximately 12 ms and 23 ms per cached carousel frame at those browsing sizes, before display presentation. The 720-line raster alone exceeds a 60 Hz frame budget.

The application does not yet switch to a smaller framebuffer when video starts. Keep the defaults unless you have checked Live TV, movies, overlays, and audio sync at the larger size.

The session first probes at one-quarter signal dimensions to avoid allocating a full HD or larger framebuffer. It uses Main's [`fb_cmd0` command](https://github.com/MiSTer-devel/Main_MiSTer/blob/master/video.cpp), waits for the kernel to acknowledge the dimensions, and starts the client only afterward. It writes no persistent display configuration. Exit restores Menu. A supervisor stops orphaned decoders before restoring hardware after a failure. Use the matching updated application and MPlayer together.

Local tests cover negotiation, rejected settings, native picture geometry, real desktop decoding, and overlay composition. On a MiSTer connected through an HDMI-to-DVI adapter to a Dell U2412M, 720p and 1080p tests both negotiated a 640×360 framebuffer and played video successfully. The 1080p test also had responsive menus. Playback diagnostics showed steady position advancement, which does not measure visible frame drops. Returning from the 720p application restored the 1280×720 menu framebuffer. Simultaneous analog output has not been validated. The higher 960×540 limit was tested and reduced Live TV smoothness. The scaler does not fix an analog route that cannot display the Linux framebuffer. Do not infer connector type or CRT capability from framebuffer dimensions alone.

Diagnostics also report `browsing_width` and `browsing_height`, independently of the logical `ui_width` and `ui_height`. Diagnostics report the render framebuffer as `output_width` and `output_height` in `application.display`. Those dimensions are not the HDMI signal resolution. Unsafe native dimensions produce an “Unsupported display mode” message before opening a stream. Audio-only playback does not require video dimensions.

## Display aspect

`display.aspect_ratio` accepts `"auto"` (the default), `"4:3"`, or `"16:9"`. Invalid values stop startup. Auto preserves the four native CRT rasters as 4:3 and infers other outputs from framebuffer proportions. Explicit values describe the screen even when its pixels are not square. They do not change signal timing. Restart after changing the setting.

The output layer and decoder receive the same resolved aspect. Native MPlayer and the desktop libmpv helper fit Original to the full video canvas and crop Zoom to the screen aspect. The presenter keeps browsing at 4:3, and the output backend centers overlays without stretching their text. A separate full-output presentation method lets video fill the framebuffer without changing browsing geometry. The native filter keeps its four-argument legacy fit for older invocations. Updated clients pass a fifth display-aspect argument and require the matching player.

Widescreen fitting has local native-filter and real desktop-decoder coverage. Auto aspect has also been visually checked on the maintainer’s CRT and HDMI display.

## Enable or disable interlaced output

Starting with v1.4.1, both [installation packages](GO_BUILD.md#release-bundles) include the matching client, player, and [InterlacedMenu.rbf v0.0.1](https://github.com/iwalton3/Menu_MiSTer/releases/tag/v0.0.1). The interlaced ZIP is a convenience preset for compatible CRT setups, not the recommended first installation. It enables interlacing without editing settings. Existing installations retain their configuration. Source builds must still place the pinned core beside `settings.json`, normally in `/media/fat/mistervision`. The supported core has this SHA-256:

```text
0158e0338a00441271f38be0703c22253d53ea39b60a1a96b7ec964bedae8999
```

To change an existing installation, set the `display` section in `settings.json`:

```json
{
  "display": {
    "interlaced": true
  }
}
```

Use this mode only with a compatible CRT route. Ordinary HDMI monitors must keep interlacing disabled. S-Video and Super Video Custard interlaced output have not been validated.

Launch **MiSTerVision** from the normal Scripts menu using the main `MiSTer.ini`. The client verifies and loads the core, then restores the normal menu on exit. No replacement of `MiSTer`, `menu.rbf`, or the kernel is part of this setup. Zaparoo is not required.

To return to progressive output, set `interlaced` to `false` or omit the section. Preserve other settings when editing. The same launcher supports both modes.

## Configuration changes and recovery

The application writes `Interlaced.mgl` beside `settings.json` and a marked `[MiSTerVisionInterlaced]` section in `/media/fat/MiSTer.ini`. Before its first change, it saves `/media/fat/mistervision/MiSTer.ini.before-interlaced`. Existing sections remain intact. An unmarked section with the same name causes an error instead of being overwritten. Disabling interlacing leaves the isolated section available for later use.

The scoped section inherits RGB/component and PAL/NTSC settings. RGB enables `direct_video` and `forced_scandoubler`. Component enables `direct_video` without forcing the scandoubler. These rules do not establish compatibility with every cable or DAC.

The supervisor waits for the expected framebuffer, manages console modes, and pauses Main while the child owns hardware. It stops orphaned decoders, restores consoles, and resumes Main. The Scripts launcher reloads the menu after successful exit in either mode. The supervisor reloads the menu after a failure, before an update restart, or when run without the launcher.

A display lock prevents two supervisors. SIGKILL of the supervisor bypasses cleanup and can require restarting MiSTer. [Supervisor logs](GO_DIAGNOSTICS.md) help diagnose failed handoffs.

Native output requires a kernel with working framebuffer mapping and VSync support. Early MiSTer Linux 6.18 builds omitted framebuffer callbacks and failed with `mmap framebuffer: No such device`. That failure requires the [kernel callback fix](https://github.com/MiSTer-devel/Linux-Kernel_MiSTer/commit/ea2212221ad137cf26bf5caa7ad3dab7216435a6), not an application setting.

## Picture and timing

The UI layout remains 640×240 or 640×288. Interlaced video uses all 640×480 or 640×576 pixels. Letterboxing and shared overlays are centered in that full framebuffer. Original/Zoom, subtitles, captions, pause, and seeking retain their normal controls.

MPlayer prepares a back page before its presentation deadline, then submits the flip at that deadline. A kernel field counter prevents reuse while a flip is pending. Paused redraws use the same ownership rules. The loading indicator clears on the first presented frame, not while a frame is merely being prepared.

For 480i Live TV, Jellyfin and Plex conversion is capped at 30000/1001 fps. Progressive NTSC uses 30 fps and PAL uses 25 fps. Slower sources are not forced to those rates. The player retains audio-clock correction and does not force playback speed. Interlaced output does not recover source fields lost during conversion or add 50/60 fps transcoding. Film-rate material can retain normal 3:2 cadence, and thin detail can show interline flicker.

## Tested scope

I test 240p and 480i on a consumer 4:3 CRT, using a MiSTer configured for RGB through its 9-pin output and a Retrovision YPbPr cable. This is not validation of MiSTer's direct YPbPr mode. Browsing, playback, shared overlays, paused picture changes, and menu restoration have been checked on that setup. Jellyfin testing used version 12.

240p browsing and playback have also been checked through a Super Video Custard composite connection on a MiSTer Multisystem 2. That setup used `vga_scaler=1` and `composite_sync=0` in `[Menu]`, with a 15 kHz 240p video timing. S-Video and 480i through that adapter have not been validated. The UI uses bright secondary text and white playback-status text over a dark backing to improve composite readability.

The warning to disable the framebuffer or enable the VGA scaler means the analog output is not displaying the Linux framebuffer. MiSTerVision needs the framebuffer, so disabling it is not a solution. The analog output must use the scaler with timings and sync appropriate for the connected adapter and CRT. Do not copy an HDMI timing into an analog CRT configuration.

Generated patterns and matched media comparisons found no sustained decoder drops after the timing fixes. Field-counter measurements describe presentation requests, not light emitted by the CRT. Perceived judder still depends on source cadence and display mode. Preserve the current timing unless a reproducible case supports a change.

Someone with suitable hardware will need to validate PAL 288p/576i and direct MiSTer YPbPr output. I do not have that hardware. Zaparoo DDR integration is not implemented. The original C project's reports for SCART, professional monitors, VGA, and HDMI do not establish support in this client.
