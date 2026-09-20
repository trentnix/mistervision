# MiSTerVision v1.4.0 announcement images

These PNGs show the application’s production launch and browsing screens with real Jellyfin media and Plex Live TV guide data. Each image is 640×480 with the intended 4:3 display proportions. Captures come from the desktop harness and do not simulate CRT scanlines.

| Image | Contents |
| --- | --- |
| [Akira launch screen](akira-launch.png) | Artwork, description, runtime, and playback actions. |
| [Hackers launch screen](hackers-launch.png) | Artwork, description, runtime, and playback actions. |
| [Dungeons & Dragons episode launch screen](dungeons-and-dragons-launch.png) | Episode information and playback actions. |
| [Plex Live TV guide](plex-live-tv-guide.png) | Channel list, PBS logo, and current/next program details from Plex. |
| [Kids TV Shows](kids-tv-dungeons-and-dragons.png) | The show list with Dungeons & Dragons selected and its artwork displayed. |

Media artwork belongs to its respective copyright holders. The application’s license does not grant rights to that media.

## Release notes

This release adds current and upcoming Live TV programs and refreshes the interface for CRT viewing.

- **Live TV guide information:** See the current show beneath each channel and current/next programs, times, and channel artwork in the information panel. Returning to the channel list retains usable guide information while refreshing it from the server. Plex uses friendly guide names when available.
- **Clearer text:** Browsing, About, connection setup, and PIN entry use the smoother Unicode font already used for captions. List titles are bold, subtext is indented, and selection bars have balanced padding. Navigation hints and the clock retain their bitmap font.
- **Consistent browsing:** The home List view shares the carousel's title, update notice, viewer identity, and cached library counts. Counts load in the background without restarting when you move or switch views. Movie, TV, and music lists use a wider artwork column with centered images.
- **Fewer steps into a show:** Shows with one season open directly to their episodes. Season lists display episode counts when available.
- **Small UX fixes:** Checking for updates keeps the About layout steady and displays its status for at least one second. Exit confirmation uses a full-width translucent stripe.

### Install or update

Existing standard MiSTerVision installations can update through **About → View release → Install**. The app restarts after installation. Your settings, sign-ins, artwork caches, and display configuration are preserved.

For a new installation, download `mistervision-v1.4.0-mister.zip` and follow its `INSTALL.txt`. The ZIP includes the matching client, MPlayer, and launcher. The optional interlaced core remains a separate download.

Guide information requires listings from Jellyfin or Plex. A full schedule grid, DVR controls, and Live TV timeshifting are not included.

[View the published release and downloads](https://github.com/trentnix/mistervision/releases/tag/v1.4.0).
