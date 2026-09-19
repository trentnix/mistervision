# Remote control

## Jellyfin

After sign-in, select **MiSTerVision** as the playback device in another Jellyfin client. Remote control works on MiSTer and in the development harness without an extra listening port or settings section.

Only the active connection accepts remote control. Switching to Plex cancels and joins the Jellyfin listener before the Plex browser starts. Pending remote commands belong to the old session and cannot reach the new one. Keeping Jellyfin's saved sign-in does not keep its remote listener active. Switching back to Jellyfin starts the listener again.

### Commands and queues

| Command | Behavior |
| --- | --- |
| Play | Start audio/video, including an ordered queue, starting index, and explicit position. |
| Play next / Play last | Insert after the current entry or append. Duplicate items have separate queue identifiers. |
| Previous / Next | Move one queue entry per command without first restarting the track. |
| Pause / Resume / Toggle | Work independently of visible menus. Repeated explicit Pause/Resume is idempotent. |
| Stop | Cancel pending remote playback and return to browsing. |
| Seek | Recorded video replaces its stream. Music uses decoder seeking. Live TV does not seek. |
| Shuffle | Reorder upcoming entries without restarting the current item. Disabling restores original order. |
| Repeat | RepeatNone, RepeatAll, and RepeatOne. Explicit track navigation still changes entries. |
| Message | Display an administrator message for eight seconds through the shared UI. |

Playback reports include queue order, current occurrence, repeat/shuffle state, and seek capability. Reordering around the current item preserves the decoder when no new start position is requested. Local album playback and rolling library-shuffle batches also report their queues. An explicit remote queue operation takes over the retained batch.

Queues are limited to 10,000 entries, with a 30-second lookup limit. Queue and repeat state are not saved across application restarts. Only Audio and Video are advertised. Remote volume, mute, photos, screenshots, subtitles, and audio-track selection are not advertised. Local video options remain available. FFplay lacks music seeking and cannot show shared messages inside its separate video window.

### Connection and recovery

The adapter uses the authenticated Jellyfin WebSocket and the configured HTTP/HTTPS base path. It sends the normal authorization header, verifies TLS unless `server.insecure_tls` is true (or legacy `INSECURE_TLS` applies), and refuses socket redirects. Capabilities register on every connection. Heartbeats check liveness, and disconnected sockets retry after five seconds while browsing stays usable.

A new sign-in cancels the old source and rejects stale account commands. Exit cancels and joins the source. Frames, queued commands, and messages are bounded. [Diagnostics](GO_DIAGNOSTICS.md) records connection status as `remote.socket`, excluding credentials and socket URLs. If no cast target appears, check that sign-in succeeded and registration reached the server.

## Implementation

[`remote.Source`](../internal/remote/source.go) emits owned commands. Optional `remote.QueueObserver` and `remote.PlaybackObserver` interfaces receive queue and playback facts without network work on the browser loop. [`jellyfin/remote.Source`](../internal/jellyfin/remote/source.go) owns protocol translation, capability registration, and reconnection. [`remote.Queue`](../internal/remote/queue.go) owns occurrence IDs, ordering, shuffle, and repeat. A different control mechanism can implement `remote.Source`.

The browser owns catalog requests and playback transitions. `PlaybackController` owns the decoder handoff. Remote sources never render or call players directly.

`browser.Config.Connector` supplies sign-in and returns a `connection.Session`. Its `Remote` field holds the authenticated source. A nil source disables remote control. The browser starts and stops the source with that session.

This implements the applicable queue/control behavior requested in [MiSTerFin issue #39](https://github.com/puddingstudio/MiSTerFin/issues/39). Automated tests cover protocol validation, reconnects, cancellation, queue ordering, stale results, and the built-client path. Remote movie and episode playback and controls have also been tested on the CRT.

## Plex video control

This prototype is parked on `feat/plex-companion-prototype`. Movie and episode controls have been tested, but the [Plex Web timeline issue](#plex-web-timeline-troubleshooting) blocks normal browser use. It is not ready for release.

Select **MiSTerVision** as the playback device in Plex Web. Use the same Plex viewer selected in MiSTerVision. The receiver supports individual movies and episodes, resume/start-over positions, pause, resume, stop, and absolute seeking. Playback uses the existing controller and the active server's credentials. Controller-supplied addresses cannot change the server or viewer.

Plex uses a LAN receiver on TCP 32433. Discovery listens on UDP 32412 and announces to `239.0.0.250:32413`. HTTP requests must come from private or loopback IPv4 addresses. Do not forward these ports through your router. Startup failures leave local browsing and playback available. Diagnostics records credential-free events as `plex.companion`.

Playback commands and private timelines require an account token verified against the active viewer through plex.tv. Successful checks are cached for one minute using credential hashes. Cloud verification must succeed when that cache expires. A transient media token alone cannot authorize control. Unauthenticated discovery and polls expose no playback details. Switching profiles, signing out, or switching servers stops the receiver and clears its authorization cache.

The command cursor reports application acceptance. It does not claim that loading or seeking has finished. Timelines report confirmed decoder position and pause state. An incoming video reports loading during the outgoing video's cleanup. Idle polls omit repeated stop notifications because those notifications dismiss Plex Web's resume dialog.

This milestone uses timeline polling. Plex push subscriptions, music queues, next/previous, shuffle, repeat, volume, track selection, photos, and remote Live TV initiation are not supported. Local playback retains those features where already available. See the [implementation plan and validation status](PLEX_REMOTE_PLAN.md).

### Plex Web timeline troubleshooting

If commands work but the timeline and pause button stop reflecting playback, inspect the response to `/player/timeline/poll`. Plex Web verifies `X-Plex-Client-Identifier` before applying a timeline. Cross-origin responses must expose that header through `Access-Control-Expose-Headers`. MiSTerVision supplies both headers, but Plex Media Server `1.43.4.10903-e5521bd8c` replaces the exposed-header list with `Location, Date` when proxying commands. Plex Web `4.160.0` then ignores otherwise valid updates. This was reproduced with Web opened through the server's LAN IP while requests used its HTTPS `plex.direct` address.

Direct tests against the server on port 32400 reproduced the replacement with four combinations of receiver CORS headers. No external reverse proxy is configured on the tested TrueNAS installation. Changing MiSTerVision's exposed-header list cannot repair the response that Plex's proxy sends to Web.

The public **Allow Fallback to Insecure Connections** setting changes `allowHttpFallback`. It permits HTTP when secure connections fail. It does not force HTTP while HTTPS works, and setting it to **Always** did not fix the user's timeline. It must not be presented as a workaround for this problem.

A separate internal preference, `preferInsecureConnections=always`, restored timeline updates in an isolated session by keeping the page and control requests on the same LAN origin. Pause–seek–resume tests passed without response modification, but that diagnostic change does not establish a supported configuration fix. Opening Web through the HTTPS `plex.direct` hostname failed discovery in the isolated session. Default cross-origin Web connections remain affected by the proxy limitation. A reliable normal-browser solution is still required.

Paused video seeks retain pause intent while preparing a replacement. The playback loop applies pause when the first frame arrives, before the next position poll. MPlayer discards ordinary slave commands during cache opening, so sending pause before the first frame does not reliably pause playback.

#### Direct connection investigation

Plex Web `4.160.0` obtains LAN players from PMS's `/clients` endpoint. Its `PlexPlayerModel` retains the advertised address and port, but the request dispatcher uses the associated server unless the model receives a `baseURL` through its construction context. Adding URL attributes to the player data does not supply that context. Cloud providers use a separate `CloudPlayerCollection` populated through `/api/v2/companions`. That collection supplies the provider's `baseURL`, and Web also accepts its timelines without the response-identifier match required for LAN players. The user's working Sonos polls went to `sonos.plex.tv`, not the PMS proxy.

An isolated browser loaded through the LAN-hosted Web page successfully polled the running MiSTer receiver directly. The response was HTTP 200, contained timeline XML, and exposed the identifier to JavaScript. A separate inert receiver registered with PMS using both the standard advertisement and additional direct-URL hints. Both registrations produced the same `/clients` fields, without a direct URL or child connections. PMS did not request that receiver's enriched `/resources` response during registration. The diagnostic receiver withdrew successfully. These checks required no playback commands or changes to the installed application.

No supported advertisement-only fix was found for this Web version. The receiver already advertises its address, port, and Plex protocol correctly. Direct polling works in the tested LAN browser context, but that does not establish a normal Web selection path or compatibility from an HTTPS page. Do not add unused discovery fields or depend on injected Web state as a product workaround. The ordinary cross-origin proxy path remains unresolved.

### Plex Web command behavior

Three rapid presses of Web's forward button produced three identical absolute seek destinations in the tested version, not three accumulating steps. MiSTerVision honors each destination. It must not reinterpret identical absolute targets as relative steps because retries and timeline clicks use the same command.

Select MiSTerVision before starting playback. Selecting it while local Web playback was loading switched Web to its remote-player view without sending `playMedia`. MiSTerVision cannot start an item it has not received. Close Web's player and start the item again with MiSTerVision already selected.
