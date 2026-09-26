# Jellyfin and Plex gap analysis

This document records missing features, current limitations, and intentional exclusions for Jellyfin and Plex without assigning priorities. MiSTerVision targets personal media on a consumer CRT and also supports HDMI displays. Both providers are supported, but matching every feature of either ecosystem is not the goal.

Implementation status was reviewed September 26, 2026 for v1.6.1. Comparisons with other clients retain the September 19 documentation review.

## Current baseline

Both providers support discovery, saved connections, user switching, Continue Watching, movie and episode playback, resume, seeking, audio and subtitle selection, picture modes, music, photos, collections, and playlists. Plex Home includes avatars, PINs, and sign-out. Plex tuner Live TV includes captions and alternate audio when available. Album and playlist playback can advance through entries. Playlist support does not include an editor.

Both providers now supply current and next Live TV programs through the shared channel-list guide. Valid cached listings appear immediately on return while the server refreshes them. Channel logos and friendly Plex guide names appear when available. See [Live TV listings](GO_BROWSING.md#live-tv-listings).

The interface supports progressive and interlaced CRT output, HDMI framebuffer scaling, automatic or explicit display aspect, and full-width widescreen backgrounds. v1.6.0 extends the carousel across widescreen displays and improves music presentation and background persistence. Music no longer shows separate audio meters. Single-season shows open directly to episodes, and season lists show episode counts when supplied. These are implemented behaviors, not remaining gaps.

### Fixes in v1.6.1

- [Progressive page flipping (PR #38)](https://github.com/trentnix/mistervision/pull/38) addresses tearing by preparing video in a back page and presenting it under exclusive scanout ownership. Remaining SuperStation One stutter is still under investigation.
- [Complete Plex artist albums (PR #39)](https://github.com/trentnix/mistervision/pull/39) replaces an artist-children query that omitted some compilations, EPs, and other releases. Jellyfin’s existing query is unchanged.

Both fixes are included in v1.6.1.

### Pending experiments

`feat/output-sized-transcoding` requests server conversion dimensions suited to the playback framebuffer. It is committed and pushed but unmerged. Controlled local encodes used less CPU on the maintainer’s MiSTer. Reporter validation is pending, so it is not a confirmed fix for [SuperStation One stutter (#22)](https://github.com/trentnix/mistervision/issues/22). Current `main` still defaults to a 720×576 transcode ceiling, independent of framebuffer size.

Jellyfin remote control is supported. Plex Companion video control exists only on the parked `feat/plex-companion-prototype` branch. It is not a released capability or a prerequisite for unrelated improvements. See the [Plex remote plan](https://github.com/trentnix/mistervision/blob/feat/plex-companion-prototype/docs/PLEX_REMOTE_PLAN.md) and [forum report](https://forums.plex.tv/t/companion-timeline-stays-stale-pms-proxy-does-not-expose-x-plex-client-identifier/943091).

## Shared feature gaps

Other Jellyfin and Plex clients offer the capabilities below. Availability varies by client, server version, metadata, permissions, and Plex subscription. A server API does not guarantee that every client implements a feature. MiSTerVision's shared browser lacks these controls for both providers unless noted.

| Feature | What MiSTerVision lacks | Provider differences |
| --- | --- | --- |
| Search, filters, and sorting | Text search, selectable genre/year/unwatched filters, and sort controls. | Both adapters need query support. Current ordering is fixed by the adapters. |
| Manual watched-state changes | Mark watched/unwatched actions. Automatic progress and completion reporting already work. | Both servers own per-user watched state. |
| Personal shortcuts | A favorites view and favorite/unfavorite actions. | Jellyfin favorites are not equivalent to Plex Universal Watchlist or cloud Lists. Do not silently map one to the other. |
| [Segment skipping (#25)](https://github.com/trentnix/mistervision/issues/25) | Buttons or automatic actions for intros, credits, recaps, and recorded commercials. | Jellyfin supplies typed media segments populated by plugins. Plex supplies markers subject to server analysis and feature eligibility. Neither requires MiSTerVision to detect segments itself. |
| Chapters and seek previews | Chapter selection and thumbnail previews while seeking. | Jellyfin chapter/trickplay data and Plex chapter/preview data need separate adapters. |
| [Post-play experience (#26)](https://github.com/trentnix/mistervision/issues/26) | A next-episode screen with an optional countdown. Existing album and playlist advancement is separate. | Both providers already contribute to Continue Watching. That feed is not itself an in-playback next-episode queue. |
| Queue controls | Local Play next/Add to queue actions, remove/reorder, and repeat/shuffle controls. | Jellyfin remote commands already exercise shared queue functionality. The local UI can reuse it. Plex remote queues remain unfinished. |
| Playlist editing | Creating playlists and adding/removing entries from the device. Existing playlists browse and play. | Persist changes through the active provider, respecting its permissions. |
| Source-version selection | A chooser for alternate encodes or versions. Source metadata already exists internally. | Both providers represent multiple sources. Plex multi-file movies are a separate unsupported case. |
| Trailers and extras | A dedicated extras browser and trailer playback flow. | Jellyfin exposes local extras. Plex can also supply subscription-dependent online extras. |
| Playback speed | Faster/slower playback controls. | Requires decoder capabilities as well as shared controls. |
| Live TV schedule grid and DVR | A full schedule grid, browsing beyond the current/next display, favorite-channel controls, and recording scheduling. The channel-list guide is implemented. | Both adapters implement `media.ProgramGuide`. DVR operations need separate capabilities. Existing recordings can play from ordinary libraries. |
| [Music navigation (#27)](https://github.com/trentnix/mistervision/issues/27) | Easier movement between playback, albums, tracks, and the current queue. v1.6.0 improved the playback layout and backgrounds, but the broader navigation work remains open. | Both providers share this UX. Album lists and previous/next track controls already work. |
| Music discovery and lyrics | Lyrics display and related-track mixes. | Jellyfin Instant Mix works through remote commands but has no local action. Plexamp radio/sonic features are different services. Whole-library shuffle is already implemented. |
| Offline media | Downloaded media and offline browsing. Artwork caching does not provide offline playback. | Available in selected clients in each ecosystem, not every client. |
| Original-video playback | A Direct Play option for video. Both current adapters transcode video within configured limits. | Decoder and output constraints determine feasibility. HDMI output scaling is supported, but it does not establish smooth native HD decoding or HDR support. |

Evidence for Jellyfin includes its [library UI](https://jellyfin.org/), [Web client actions](https://github.com/jellyfin/jellyfin-web/blob/master/src/strings/en-us.json), [media segments](https://jellyfin.org/docs/general/server/metadata/media-segments/), [multiple versions and extras](https://jellyfin.org/docs/general/server/media/movies/), [Live TV and DVR](https://jellyfin.org/docs/general/server/live-tv/), and [client catalog](https://jellyfin.org/downloads/clients/all/). Its [MPV Shim controls](https://github.com/jellyfin/jellyfin-mpv-shim/blob/master/docs/configuration.md) document chapters, previews, and playback speed. These references describe available features, not a guarantee for every Jellyfin client.

Plex documents [library controls](https://support.plex.tv/articles/200392126-using-the-library-view/), [item actions](https://support.plex.tv/articles/202462186-viewing-item-details/), [player behavior](https://support.plex.tv/articles/settings-android-tv/), [play queues](https://support.plex.tv/articles/202188298-play-queues/), [multiple versions](https://support.plex.tv/articles/200381043-multi-version-movies/), [chapter thumbnails](https://support.plex.tv/articles/200289526-library/), [playback speed](https://support.plex.tv/articles/video-playback-speed-controls/), [DVR guide](https://support.plex.tv/articles/225877387-program-guide/), and [downloads](https://support.plex.tv/articles/downloads-overview/).

## Jellyfin-specific gaps and limits

- **SyncPlay:** MiSTerVision can receive ordinary Jellyfin remote commands, but cannot join a synchronized multi-client viewing group. SyncPlay adds clock coordination and group buffering behavior. It is a separate feature, not a missing remote-control button. See [SyncPlay operations](https://typescript-sdk.jellyfin.org/classes/generated-client.SyncPlayApi.html).
- **Favorites and local Instant Mix controls:** MiSTerVision has no favorites management or local Instant Mix action. Jellyfin remote `PlayInstantMix` commands already resolve a server-generated mix and start the shared queue. A local action can reuse that path.
- **Sign-in alternatives:** Saved Quick Connect users work. A general username/password entry flow is not implemented. An on-screen server-address editor is also missing for both providers. These are optional setup additions, not unfinished user switching.
- **Live TV alternate audio:** MiSTerVision does not expose it for Jellyfin. The tested negotiated stream contained only one audio track, even when the broadcast carried another. A client-side track chooser alone would not make the missing stream available.
- **Books, comics, and audiobooks:** Dedicated categories are intentionally excluded from the carousel. Plex audiobooks stored in an ordinary music library cannot be distinguished from music. These formats are not planned parity work.

## Plex-specific gaps and limits

| Feature | MiSTerVision gap | Reference |
| --- | --- | --- |
| [Plex remote control (#21)](https://github.com/trentnix/mistervision/issues/21) | Receiver prototype is blocked by stale Plex Web timelines through the PMS proxy. Plex music queues and broader controller compatibility remain unfinished. | [Investigation](https://github.com/trentnix/mistervision/blob/feat/plex-companion-prototype/docs/GO_REMOTE.md#plex-video-control) |
| Live TV timeshifting | No supported pause buffer, rewind, or catch-up playback. | [Live TV](https://support.plex.tv/articles/115007689648-watching-live-tv/) |
| Plexamp music features | No gapless track handoff, loudness leveling, Sweet Fades, lyrics display, or sonic/radio discovery. Album playback, whole-library shuffle, visuals, and existing smart playlists already work. | [Plexamp](https://www.plex.tv/plexamp/), [music features](https://support.plex.tv/articles/202526943-plex-free-vs-paid/) |
| Relay fallback | Reachable local and remote server addresses work. Plex's indirect relay transport does not. | [Relay](https://support.plex.tv/articles/216766168-accessing-a-server-through-relay/) |
| Watchlist and cloud Lists | No Universal Watchlist or Plex cloud Lists. Those are distinct from supported server playlists and collections. | [Watchlist](https://support.plex.tv/articles/universal-watchlist/), [Lists](https://support.plex.tv/articles/lists/) |
| Plex online services | No Discover streaming-service integration, Plex free streaming catalog, online Live TV, rentals, or purchases. | [Watchlist and Discover](https://support.plex.tv/articles/universal-watchlist/), [Plex services](https://support.plex.tv/articles/202526943-plex-free-vs-paid/) |

Live TV timeshifting is absent from both MiSTerVision adapters. Plex provides a concrete comparison, but Jellyfin feasibility must be checked against the selected tuner, stream, and client rather than assumed equivalent. The older Plex Live TV article understates caption/audio support. MiSTerVision's tested captions and Plex alternate audio remain supported. Plex retired Watch Together in 2025. Jellyfin SyncPlay remains a separate comparison and is not excluded by Plex's decision.

## Other shared gaps

- **[Controller recognition (#24)](https://github.com/trentnix/mistervision/issues/24):** Configurable mappings and labels work, but automatic controller-family detection and matching labels are not implemented.
- **Photo controls:** Manual navigation works. Automatic slideshows and photo zoom are not implemented.
- **Manual server entry:** Configured addresses and discovery work. There is no on-screen address editor.

## Open compatibility work

- **[SuperStation One (#22)](https://github.com/trentnix/mistervision/issues/22):** ConsoleMode handoff and diagnostics are implemented. Hardware-specific playback investigation remains open. The maintainer has no SS1, so fixes need reporter validation. The merged tearing fix does not establish that remaining stutter is resolved.
- **[Xbox trigger mappings (#30)](https://github.com/trentnix/mistervision/issues/30):** Some controllers report trigger rest positions that cause unintended seeking. A candidate fix is saved on `fix/controller-trigger-mapping` but is not merged. This is separate from automatic controller recognition.
- **[PAL validation (#28)](https://github.com/trentnix/mistervision/issues/28):** PAL code paths exist, but 288p/576i output needs validation by someone with suitable hardware.

## Implementation considerations

Shared UX belongs in the browser and renderer. Provider-specific requests belong in adapters behind small capability interfaces. Optional capabilities must allow one provider to expose a feature without forcing an inaccurate equivalent on the other. SyncPlay is distinct from ordinary remote control, and favorites are distinct from cloud watchlists. These boundaries describe how an addition could fit, not a commitment to build it.

## Live TV guide status

The first guide increment shipped in v1.4.0. Channel rows show current-program titles. The information panel shows a channel logo and current/next programs with local times. Selecting a channel starts playback directly. Missing guide data leaves the panel blank and does not block tuning.

Both adapters implement the optional `media.ProgramGuide` interface. The browser requests a bounded six-hour window, refreshes once a minute while browsing, and refreshes on every return to the list while retaining valid cached data. Provider identifiers and requests stay inside the adapters. The UI and cache behavior are shared.

Plex guide information has been tested with the maintainer’s server on MiSTer. Jellyfin has automated adapter coverage, but its guide display still needs validation against a Jellyfin server with populated listings. A full schedule grid, channel favorites, DVR scheduling, and timeshifting remain separate gaps.

## Status and scope

Plex remote control stays parked until the forum response or new evidence provides a supported route. Before merging, normal-browser pause, seek, resume, timeline updates, session ownership, and cleanup must pass without injected headers or private browser preferences. Reliable video control is a prerequisite for broader Plex remote queues and media types.

Source-version selection, multi-file movies, trailers/extras, photo controls, manual server entry, and Relay support have no assigned priority. Gapless music needs a separate decoder-handoff design and performance review for either provider. Jellyfin SyncPlay needs multi-device timing tests.

Offline downloads, Plex cloud services, social features, server administration, native HD decoding and HDR, and Plexamp's radio and sonic features have no implementation commitment. In-app screenshots and exact copies of the C music visualizers remain out of scope.

Direct MiSTer YPbPr component output has been tested at 240p and 480i. PAL output and background-music restoration on the optional add-on need contributors with suitable hardware. Zaparoo DDR integration stays deferred until it can be tested. Existing automated checks continue across releases. CRT readability, controller feel, motion cadence, and audio sync still need hardware checks.
