# MiSTerVision

<p align="center">
  <img src="docs/images/mistervision-logo.png" alt="MiSTerVision logo" width="256" height="256">
</p>

MiSTerVision is a Jellyfin and Plex client for CRT televisions on MiSTer FPGA. It supports movies, TV shows, live TV, music, photos, collections, and playlists through one interface.

Both providers share browsing and playback controls, server discovery, and saved connection switching. Jellyfin can switch saved Quick Connect users. Plex supports Home profiles with avatars and PIN entry. About also lets you forget a Jellyfin user or sign out of Plex on this device.

My goal is a great media experience on CRTs. I use a consumer 4:3 CRT television for everyday testing, including RGB and composite connections. I also test HDMI through an HDMI-to-DVI monitor. I have tested both server providers, including Jellyfin 12.

![MiSTerVision library carousel](docs/images/screenshots/home-carousel.png)

## Run on MiSTer

Start with the **progressive package** or the Downloader installation below. Both keep MiSTer’s current display mode and include the core needed for optional interlaced output. You do not need a configuration file to discover and link a local server.

1. Install MiSTerVision using one method below.
2. Check the [display setup](#choose-your-display) for your connection.
3. Launch **MiSTerVision** from Scripts and [connect to a server](#connect-to-a-server).

### Install through update_all or Downloader

1. Download theypsilon’s [MiSTerVision database ZIP](https://raw.githubusercontent.com/theypsilon/MultiDatabases_MiSTer/db/mister-vision/downloader_MultiDatabases_mister-vision.zip).
2. Extract `downloader_MultiDatabases_mister-vision.ini` to `/media/fat/` on the SD card. This registers the database without replacing `downloader.ini`.
3. If MiSTerVision is running, exit it. Run `update_all` or MiSTer Downloader.

The database installs the progressive package and preserves existing settings and sign-in. Use the same updater for later releases. See the maintainer’s [database instructions](https://github.com/theypsilon/MultiDatabases_MiSTer/tree/main/mister-vision).

### Manual installation

1. Download `mistervision-vX.Y.Z-progressive.zip` from [Releases](https://github.com/trentnix/mistervision/releases).
2. If MiSTerVision is running, exit it. Extract the ZIP and copy its `Scripts` and `mistervision` directories to `/media/fat/`, merging with existing directories.
3. Keep existing configuration and state files. Copy the application and matching MPlayer together.

The result includes `/media/fat/Scripts/MiSTerVision.sh`, `/media/fat/mistervision/mistervision`, `/media/fat/mistervision/mplayer-arm`, and `/media/fat/mistervision/InterlacedMenu.rbf`. Keep the launcher filename free of spaces. Make the launcher and binaries executable if your filesystem requires it.

The interlaced ZIP contains the same application and core with a different first-launch preset. It does not change an existing installation’s display setting. Use it only for a compatible CRT setup. See [interlaced output](#progressive-and-interlaced-output).

Upgrading from v1.4.0 or earlier requires one Downloader or manual installation. For older MiSTerFin CRT installations, follow the [rename instructions](docs/GO_BUILD.md#moving-from-misterfin-crt). Unreleased changes require a [source build](docs/GO_BUILD.md).

## Choose your display

| Connection | Recommended starting point |
| --- | --- |
| Analog CRT | Keep working CRT timing. Enable the analog framebuffer route described in the [display guide](docs/GO_DISPLAY.md). |
| Composite or S-Video adapter | Use settings specific to the adapter. A tested Super Video Custard composite example is in the [display guide](docs/GO_DISPLAY.md#composite-and-s-video-output). |
| HDMI or HDMI-to-DVI monitor | Keep interlacing off and the default framebuffer limits. See [HDMI setup](docs/GO_DISPLAY.md#hdmi-output). |

The HDMI scaling and aspect behavior described below is in current source and is not included in v1.4.2.

### Composite and S-Video output

If the CRT says “Either disable framebuffer: fb_terminal=0 or enable scaler on VGA: vga_scaler=1”, the analog route is not showing the Linux framebuffer. MiSTerVision needs that framebuffer. Use `vga_scaler=1` with adapter-appropriate CRT timing, not an HD timing. See the [tested settings and hardware limits](docs/GO_DISPLAY.md#composite-and-s-video-output).

### HDMI output

Keep `display.interlaced` off, `display.aspect_ratio` on `"auto"`, and framebuffer limits at their defaults. At 720p or 1080p HDMI timing, MiSTerVision uses a 640×360 framebuffer and the hardware scaler enlarges it. This applies to browsing and playback. The larger 960×540 framebuffer reduced Live TV smoothness in testing, so it is not recommended for initial setup.

Browsing and overlays retain a centered 4:3 layout. Video uses the screen’s aspect without stretching the source. Auto infers aspect from framebuffer geometry, not connector detection. Use an explicit `"4:3"` or `"16:9"` only if that inference is wrong. See [aspect settings](docs/GO_DISPLAY.md#display-aspect).

MiSTerVision does not provide independent HD and CRT outputs. Both displays must accept the chosen timing, or you need an external scaler. See [simultaneous output](docs/GO_DISPLAY.md#simultaneous-analog-and-hdmi-output).

### Progressive and interlaced output

The default keeps MiSTer’s current mode, normally 240p on an NTSC CRT. After verifying browsing and playback, compatible CRT setups can try 480i for finer text and subtitles. Interlacing can introduce flicker and is not suitable for ordinary HDMI monitors. PAL modes and interlaced output through the Super Video Custard or SS1 S-Video path still need validation.

To enable it, exit MiSTerVision and add or edit the `display` section in `/media/fat/mistervision/settings.json`, preserving other settings:

```json
{
  "display": {
    "interlaced": true
  }
}
```

The bundled core loads automatically, and the normal menu returns on exit. Set `interlaced` to `false` to return to the default. Both modes use the same launcher. See [interlaced setup and recovery](docs/GO_DISPLAY.md#enable-or-disable-interlaced-output).

## Connect to a server

With no saved or explicit connection, the app starts Jellyfin discovery.

- **Jellyfin:** Select a server and approve its Quick Connect code in a signed-in Jellyfin client. See [discovery troubleshooting](docs/GO_BROWSING.md#jellyfin-discovery) if no server appears.
- **Plex:** Open About with Start, choose **Connections**, then **Plex**. Approve the code at [plex.tv/link](https://plex.tv/link), choose a Home viewer if offered, and select a server. Plex prefers a reachable local address. Linking and Home profile checks require internet access.

The last successful connection opens automatically. Use **About → Connections → Use existing connection** to switch servers. Only the active connection accepts remote commands. Back cancels setup and returns to the previous connection when one is available.

### Specify an address

Discovery needs no JSON. For a remote server or fixed address, add a `server` section to `settings.json` without replacing other settings:

```json
{
  "server": {
    "provider": "jellyfin",
    "url": "http://your-jellyfin-server:8096"
  }
}
```

For Plex, use `"provider": "plex"` and your server address, normally `http://your-plex-server:32400`. Explicit addresses stay fixed. See [connection configuration](docs/GO_CONFIGURATION.md#server-connection).

### Switch connections or users

About offers **Switch profile** when multiple saved Jellyfin users or Plex Home viewers are available. Jellyfin’s **Add user** opens Quick Connect. Approve the code as the user you want to add. Protected Plex viewers enter their PIN after an application restart.

**Forget user** removes a saved Jellyfin user. **Sign out** removes the linked Plex sign-in on this device. Both require confirmation and leave server accounts and media intact. Back from a profile picker returns to About without changing users. See [Jellyfin users](docs/GO_BROWSING.md#jellyfin-users), [Plex Home](docs/GO_PLEX.md#plex-home-profiles), and [multiple connections](docs/GO_CONFIGURATION.md#multiple-connections).

## Updates

- **Downloader installation:** Exit MiSTerVision and run `update_all` or Downloader. About shows release information and directs you to the external updater.
- **Manual installation:** Open **About → View release → Install**. The app restarts after a successful update.

Updates preserve settings, sign-in, playback preferences, and caches. Do not mix update methods because Downloader can restore the version its database lists. To return to built-in updates, remove the MiSTerVision database registration and restart. See [update ownership and recovery](docs/GO_BUILD.md#application-updates).

## Controls


I test with an Xbox controller. The default layout follows MiSTer: B selects, plays, or pauses, and A goes back, cancels, or stops. Use the D-pad or left analog stick to navigate. During video or music playback, any direction shows or hides controls. Triggers seek, and shoulder buttons change music tracks.

Controller mappings are configurable in the `input` section of `settings.json`. For example, if you prefer A to select and B to go back, add this section while preserving your other settings:

```json
{
  "input": {
    "profiles": [
      {
        "match": "*Xbox*",
        "buttons": {
          "304": "open",
          "305": "back"
        }
      }
    ]
  }
}
```

The numbers are Linux input event codes. For a standard Xbox mapping, `304` is A (`BTN_SOUTH`) and `305` is B (`BTN_EAST`). Other controllers or drivers can report different codes. Use an input inspector such as `evtest` to read the code when you press a button. See [finding device names and button codes](docs/GO_INPUT.md#finding-device-names-and-button-codes).

Restart after editing. Unspecified bindings keep their defaults. See the [input guide](docs/GO_INPUT.md) for device matching, axes, and labels.

The [playback guide](docs/GO_PLAYBACK.md#playback-controls) lists controller and keyboard controls. [About](docs/GO_BROWSING.md#about-and-updates) shows the installed version and provides [updates](#updates).

To control playback from another Jellyfin client, select **MiSTerVision** as the playback device. Remote play, queues, pause/resume, seeking, shuffle, and repeat are supported. See [remote control](docs/GO_REMOTE.md).

## Screenshots


These images show the v1.4.0 interface. Browsing captures come from the desktop harness. Setup previews use example names, addresses, approval codes, and a sample avatar. Controller and keyboard hints follow the active input device.

| Continue Watching | Movie details |
| --- | --- |
| ![Continue Watching with saved playback positions](docs/images/screenshots/continue-watching.png) | ![Movie artwork, summary, runtime, and playback controls](docs/images/screenshots/movie-info.png) |
| **Choose a connection** | **Discover Jellyfin** |
| ![Saved connections and Jellyfin or Plex setup](docs/images/screenshots/connections.png) | ![A discovered Jellyfin server with its name and address](docs/images/screenshots/jellyfin-discovery.png) |
| **Plex Home viewers** | **Protected profile** |
| ![Three visible Plex Home cards and a counter for four viewers](docs/images/screenshots/plex-profiles.png) | ![Viewer avatar and name above the centered PIN keypad](docs/images/screenshots/plex-pin.png) |
| **Saved Jellyfin users** | **About and Plex sign-out** |
| ![Saved Jellyfin users with Add user and Forget user controls](docs/images/screenshots/jellyfin-users.png) | ![About with Switch profile, Sign out, Connections, and update controls](docs/images/screenshots/about.png) |

The home carousel and root List view share library counts and show the active viewer and an update notice when available. Live TV lists show current and upcoming programs when the server supplies guide data. Shows with a single season open directly to their episodes. See [browsing behavior](docs/GO_BROWSING.md).

Interface text uses the same smooth font as captions. Navigation hints and the clock keep the bitmap font.

The [full gallery](docs/SCREENSHOTS.md) also shows the home List view, account linking, server selection, the movie list, setup help, and About.

## Configuration


Connection and application settings live in **`settings.json`**, normally `/media/fat/mistervision/settings.json` on MiSTer. Both providers use `server.provider`, `server.url`, `server.insecure_tls`, and `server.transcode`. Use [settings.example.json](settings.example.json) as a reference. A discovered connection does not require copying the example. Preserve existing settings when editing. Omitted optional fields use defaults. Restart after changing settings.

| Setting | Defaults and options | Guide |
| --- | --- | --- |
| `connections.profiles` | No configured entries. Discovered connections are remembered automatically. Add up to 16 named server connections. | [Connections](docs/GO_CONFIGURATION.md#multiple-connections) |
| `server` | Provider: `jellyfin`. URL required when the section exists. TLS verified. Transcode limits: 720×576 at 12 Mbps. | [Connection](docs/GO_CONFIGURATION.md#server-connection) |
| `ui.title` | Heading: `MiSTerVision`. An explicit empty `title` hides it. Long titles are truncated. | [Title](docs/GO_CONFIGURATION.md#browsing-title) |
| `ui.show_collections`, `ui.show_playlists` | Both `true`. Show nonempty categories. Set either to `false` to hide its card. | [Carousel](docs/GO_CONFIGURATION.md#carousel-categories) |
| `ui.navigation_sounds` | `enabled: true`, `volume: 10` out of 100. False or volume zero silences navigation sounds. | [Sounds](docs/GO_CONFIGURATION.md#navigation-sounds) |
| `background` | Generated carousel mosaics and item artwork on lists. `image` selects one custom background. | [Background](docs/GO_CONFIGURATION.md#browsing-background) |
| `display` | `interlaced: false`, `aspect_ratio: "auto"`, framebuffer ceiling 640×480. Keep these defaults for initial use. | [Display](docs/GO_DISPLAY.md) |
| `input` | Built-in controller mappings and button labels. Profiles override matching devices. | [Input](docs/GO_INPUT.md) |
| `music_visuals` | Music playback appearance only. `default_background: "Starfield"`, `show_audio_meters: true`. Missing optional Toasty sprites are omitted. | [Music visuals](docs/GO_MUSIC.md) |
| `diagnostics` | Off. Legacy `DEBUGLOG` applies only without a `server` section. Path: `debug.log`. Limit: 1 MiB per file. | [Diagnostics](docs/GO_DIAGNOSTICS.md) |

Existing installations can retain legacy settings or [migrate them into one file](docs/GO_CONFIGURATION.md#migration). Migration preserves the original files. Invalid connection settings stop startup, while recoverable UI settings use the defaults documented in the [configuration guide](docs/GO_CONFIGURATION.md#application-settings).

### Browsing background

To use one custom image on the carousel and browsing lists, set `background.image` in `settings.json`:

```json
{
  "background": {
    "image": "background.png"
  }
}
```

Place the image in the same directory, or use an absolute path. PNG and JPEG are supported, up to 4 MiB and 2048 pixels in either dimension. A 4:3 image fits best. The client crops and dims it to keep the interface readable. Restart to apply changes. An omitted or empty `image` keeps the normal artwork.

The client checks the file contents, not its extension. A video, text file, unsupported image format, or corrupt image is rejected. Missing or invalid image files fall back to the normal artwork with a brief on-screen notice. With diagnostics enabled, the fallback also records a `configuration.fallback` event. Startup continues.

### Sounds and caches

To turn off navigation sounds, set `ui.navigation_sounds`:

```json
{
  "ui": {
    "navigation_sounds": {
      "enabled": false
    }
  }
}
```

Sound settings affect browsing feedback only. They do not change music or video volume.

`MISTERVISION_CACHE_ROOT` changes where artwork and carousel collages are cached. The default root is `/media/fat` on MiSTer, `/tmp/mistervision-cache` in the Ghostty harness, and the user’s cache directory (usually `~/.cache`) for direct desktop runs. To keep the Ghostty cache across reboots, set `MISTERVISION_CACHE_ROOT="$HOME/.cache"` before launching the harness. The client stores caches under `mistervision` within that directory. See [artwork caching](docs/GO_BROWSING.md#persistent-artwork-cache) for details.

## Local development and testing

I use the Ghostty harness on Linux to develop and test the interface without MiSTer hardware. It also helps verify that the architecture supports different display pipelines while reusing the same UI and application logic. From the repository directory, run the browsing demo:

```bash
python3 tools/ghostty/ghostty_harness.py --demo --ntsc
```

The harness builds the client automatically. The demo uses mock data and does not play media. See the [development harness guide](tools/ghostty/README.md) for dependencies, copyable Jellyfin discovery and Plex connection commands, separate configuration profiles, and playback inside Ghostty.

## Server support and limits

Jellyfin supports remote control from other Jellyfin clients. Plex supports local DVR Live TV with alternate audio when the stream provides it. Both providers share playback controls, music visuals, picture modes, captions, and the photo viewer.

Search, automatic photo slideshows, and photo zoom are not implemented. Plex relay connections, remote control, multi-file movies, and free online TV are not supported. See [Plex details](docs/GO_PLEX.md) and [current limits](docs/README.md#current-limits).

## Deferred work

See the [Jellyfin and Plex gap analysis](docs/GAP_ANALYSIS.md) for missing features, current limitations, and intentional exclusions.

- **Broader controller support:** Testing more controllers, recognizing controller families, and showing their button labels automatically are potential future improvements. Other controllers may need a custom input profile today.
- **PAL 288p/576i and direct MiSTer YPbPr validation:** Someone with suitable hardware will need to test these output paths. I do not have that hardware. Tested analog paths include RGB through a Retrovision YPbPr cable and Super Video Custard composite. Those do not validate MiSTer’s direct YPbPr mode.
- **Zaparoo DDR integration:** Deferred until I have a way to test it. Zaparoo is not required for the supported interlaced output.
- **MiSTer background-music hardware validation:** Suspension and restoration are implemented and covered by automated tests. Testing with the actual add-on is deferred because I do not use it. This is separate from music played through Jellyfin or Plex.

## More information

See the [documentation index](docs/README.md) for all guides and current limits.

- [Browsing, Continue Watching, and artwork caches](docs/GO_BROWSING.md)
- [Playback and controls](docs/GO_PLAYBACK.md)
- [Subtitles, audio tracks, and picture modes](docs/GO_PLAYBACK.md#video-options)
- [Builds and tests](docs/GO_BUILD.md)
- [Rendering architecture](docs/GO_RENDERING.md)

## Origins and license

I started this project as MiSTerFin CRT, a Go port of [MiSTerFin](https://github.com/puddingstudio/MiSTerFin) by Pudding Studio, including my [C changes](https://github.com/trentnix/MiSTerFin). I maintain it independently. It remains heavily based on MiSTerFin, an excellent project.

Original MiSTerFin material is copyright © 2026 Pudding Studio. My additions and modifications are copyright © 2026 trentnix. I distribute the application under [CC BY-NC 4.0](LICENSE), except for components covered by [separate licenses](docs/THIRD_PARTY.md), including the GPL-licensed MPlayer.
