# Plex remote control

**Status: parked on `feat/plex-companion-prototype`, not ready to merge or release.** Tracking issue: [Plex server-proxy CORS report](https://forums.plex.tv/t/companion-timeline-stays-stale-pms-proxy-does-not-expose-x-plex-client-identifier/943091). Resume when Plex responds or a supported connection path is found. First repeat normal-browser pause, seek, resume, and timeline checks without diagnostic header or preference changes. The remaining checkpoints below still apply.

The first milestone adds a shared playback-state boundary and an isolated Plex Companion compatibility probe. The Plex Web compatibility checkpoint passed on MiSTer. Authenticated video control is now integrated with active Plex sessions. Automated validation passes. Plex Web movie and episode playback now works on MiSTer. Paused seeks pass device and Web tests. Default cross-origin Plex Web connections still have a timeline limitation in the server proxy. The separate probe still does not read saved accounts, play media, or change settings.

## Shared boundary

[`remote.Source`](../internal/remote/source.go) supplies commands. Sources can implement `remote.QueueObserver` for queue changes and [`remote.PlaybackObserver`](../internal/remote/playback.go) to receive immutable playback facts. The browser publishes an initial snapshot and subsequent changes without network I/O. Queue state stays separate so position updates do not copy large queues. Jellyfin keeps its existing server reporting.

`PlaybackController.RemoteState` supplies item identity, position, duration, media kind, and loading, playing, paused, buffering, seeking, stopped, or failed status. Pause and position come from decoder feedback rather than optimistic UI state. A prepared seek destination is not a completed seek. Snapshots are not command acknowledgments. Optional command receipts distinguish application acceptance from decoder completion. Per-controller command ordering belongs to the Plex adapter.

Only the active session receives observations. Stopping its remote source removes the observer. A newly started observer receives idle state rather than the previous account's stopped item. The existing session generation rejects stale commands. Sources never access rendering or decoder handles.

## Compatibility probe

[`cmd/plex-companion-probe`](../cmd/plex-companion-probe) runs [`internal/plex/companion`](../internal/plex/companion) independently of the application. It advertises **MiSTerVision Probe**, answers GDM searches, supplies `/resources`, and supports immediate and waiting timeline polls. It answers idle polls without media timelines. Repeated synthetic `stopped` timelines cause Plex Web to discard a pending resume dialog. Actual stops retain an item and produce a stopped timeline. The initial command baseline is zero and never acknowledges a rejected playback command. The diagnostic receiver advertises `timeline,playback` so clients can send a playback request for inspection. That advertisement does not mean the probe can play media. Playback commands and push subscriptions return HTTP 501. They do not execute or acknowledge a completed operation.

The probe requires an explicit IPv4 peer allowlist. Include the Plex server and the computer running the controlling browser. The allowlist is a diagnostic restriction, not Plex account authentication. Request logs record a recognized operation and HTTP method, status, command number, and whether client identification or a token was present. Logs never contain tokens, media paths, request URLs, or controller identities. The probe never follows controller-supplied URLs or makes subscription callbacks.

Build and run from the repository root, replacing the example addresses with your Plex server and controller addresses:

```sh
go build -o build/plex-companion-probe ./cmd/plex-companion-probe
./build/plex-companion-probe \
  -allow 192.168.1.100 \
  -allow 192.168.1.50
```

The default HTTP port is TCP 32433. Discovery listens on UDP 32412 and announces to `239.0.0.250:32413`. Use `-interface` if the default multicast interface is unsuitable. Use `-listen` to choose another HTTP address and `-id` to distinguish simultaneous probes. Stop with Ctrl+C. The probe withdraws its discovery advertisement on normal shutdown. Do not expose it through router port forwarding.

To build the standalone probe for MiSTer:

```sh
CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 \
  go build -o build/plex-companion-probe-arm ./cmd/plex-companion-probe
```

Run that executable from a temporary directory. It needs no framebuffer or Scripts entry.

## Compatibility checkpoint

1. Open the server-hosted Plex Web app and select your Home profile normally.
2. Open the player selector and look for **MiSTerVision Probe**.
3. Select the probe and attempt movie playback. Playback must not start. Check that the probe records the request and responds with an intentional unsupported-operation error.
4. Repeat with hosted Plex Web and the mobile app you actually use. Record app versions and whether discovery, selection, polling, and command receipt work.
5. Stop the probe and confirm the server removes its player entry. Confirm ordinary MiSTerVision playback remains available.

Plex's [Companion support matrix](https://support.plex.tv/articles/203082707-supported-plex-companion-apps/) excludes mobile apps version 2025.10 and newer from that article. Current phone compatibility requires testing. The archived [Plex player protocol reference](https://github.com/plexinc/plex-media-player/wiki/Remote-control-API) describes discovery, HTTP commands, and timeline reporting. Its age makes real-client verification necessary.

### Findings on September 19, 2026

- Plex Media Server `1.43.4.10903-e5521bd8c` lists the desktop probe in `/clients`, including its HTTP address and timeline capability. Announcements make registration immediate.
- The real Plex server successfully proxied a timeline poll to the probe and returned its XML status. It also forwarded a pause command with its command number. The probe returned HTTP 501. Both requests carried a token. Token presence does not establish the correct authorization policy.
- GDM replies, HTTP resources, timeline conversion, deliberate command rejection, malformed requests, peer restrictions, cancellation, and concurrent publication have automated coverage. Those tests do not establish compatibility with Plex Web's complete player-selection flow.
- Server-hosted Plex Web opened in an isolated Chromium session but required the owner's Home PIN. Automated testing stopped at that prompt. No PIN bypass or account change was attempted. The subsequent interactive checks confirmed player selection and recurring timeline polls. Early polls carried no token. After correcting idle timelines, the user completed the playback choice and Plex sent two `playMedia` requests, followed by Stop requests. Those playback requests carried client identification, a token, and increasing command IDs. The probe returned HTTP 501 as intended. Discovery, selection, polling, and playback-command receipt are verified for this Plex Web setup. Hosted Plex Web and mobile compatibility remain unverified.
- After MiSTer became available, 24 selected browser and Companion test cases passed on its ARM processor. The isolated device probe registered with Plex, answered direct resource requests and server-proxied timeline polls, and rejected a proxied pause command as intended. Normal shutdown removed its player entry. The installed application and settings were not replaced.
- Full Go tests with and without cgo, targeted race tests, lint, 37 built-client browsing tests, and host/ARM builds passed. An unchanged playback observation used zero allocations in a desktop benchmark.
- Plex Web `4.160.0` clears its remote queue for every accepted stopped timeline. The initial probe repeatedly synthesized those timelines while idle, which interrupted the resume/start-over dialog. Idle responses now omit media timelines and preserve the zero command baseline. Actual stopped items still report their final state.
- During the compatibility checkpoint, only the explicitly started probe opened a listening port. The authenticated receiver now opens TCP 32433 for an active Plex session.

## Authenticated receiver

The production connector now supplies [`companion.Source`](../internal/plex/companion/source.go) for the active Plex session. [`plex/remote.go`](../internal/plex/remote.go) verifies controller account credentials against the selected viewer and resolves individual movies or episodes using that viewer's existing server access. No incoming address or transient playback token replaces the configured connection.

Protocol parsing, authorization caching, receiver synchronization, and timeline formatting stay in separate files under `internal/plex/companion`. The shared browser owns execution. Command receipts report admission, while playback snapshots report actual loading, playing, pause, buffering, seeking, stop, or failure. Canceled catalog results cannot start late playback. Queue observation is optional so the video-only receiver does not alter local music navigation.

Full Go suites with and without cgo, targeted race tests, lint, and host/ARM builds pass. Unit tests cover wrong-profile authorization, expired credentials, duplicate and old command IDs, invalid positions, server/path restrictions, cancellation, handoffs, and listener replacement. The deployed ARM binary matches the local build. Seventeen Companion tests and seventeen selected browser tests pass on MiSTer. All 37 headless browser integration tests pass on the host. The application launches successfully with its remembered Jellyfin connection, where the Plex listener stays closed. The user verified Plex Web authorization and movie and episode playback on MiSTer. Paused seeks briefly resumed, and the Web timeline did not reliably follow seeks. The corrected paused-start and repeated-seek behavior has regression coverage and passed direct MiSTer tests. The framebuffer stayed unchanged after paused seeks and advanced after Resume, including when another seek replaced a loading destination. The first attempted early-pause fix was rejected because MPlayer discarded commands during cache opening. Pause now waits for first-frame feedback within the playback loop. Plex Web timeline diagnostics identified a separate proxy CORS limitation, documented in [remote control](GO_REMOTE.md#plex-web-timeline-troubleshooting). With Web and API requests using the same LAN origin, real Web pause, seek, resume, and timeline updates passed without response modification. Framebuffer hashes confirmed the picture stayed paused after seeking and advanced after Resume. Temporary per-poll application logging has been removed. Web sent identical absolute targets for three rapid forward presses. Selecting the receiver during local Web loading produced no playback command. Those Web behaviors and the diagnostic LAN test are documented with the proxy limitation. The public Allow Fallback to Insecure Connections setting controls a different preference and did not resolve the user's normal-browser failure. A supported normal-browser solution is still required. Hosted Web and mobile compatibility are still unverified.

### Direct connection follow-up

An isolated browser could poll MiSTer directly and read its identifier, but Plex Web `4.160.0` routes LAN players from `/clients` through PMS. Runtime model checks and an inert GDM registration test found no supported advertisement field that enables a direct connection. Sonos uses a separate cloud-provider path. The diagnostic receiver was removed, and installed playback was not changed. See the [direct connection findings](GO_REMOTE.md#direct-connection-investigation). The cross-origin Web timeline blocker remains open.

### Developer documentation review

Plex's [official PMS API reference](https://developer.plex.tv/pms/) exposes OpenAPI version `1.2.3`. The reviewed specification contains 207 paths and documents `POST /:/timeline` for playback reporting to PMS. It does not list `/clients`, `/player/timeline/poll`, or Companion CORS handling. The [archived Companion guide](https://github.com/plexinc/plex-media-player/wiki/Remote-control-API) still explicitly requires exposing the receiver identifier and describes direct player connections. Neither reference supplies a verified fix for the observed Web proxy behavior.

A [Music Assistant discovery report](https://github.com/music-assistant/support/issues/5782) describes successful Plexamp iOS control after registering a player with plex.tv and publishing its local connection URI. This is a third-party experiment, not Plex's documented browser workaround. The isolated registration test below did not produce a direct Plex Web connection. The inspected Web version selects LAN players from PMS and cloud players from the separate companions service. The current prototype remains parked.

The archived [Arcanemagus Plex API wiki](https://github.com/Arcanemagus/plex-api/wiki) documents player commands and account device connection addresses, but does not explain how a receiver can make Plex Web choose a direct connection. Its linked [Python PlexAPI client documentation](https://python-plexapi.readthedocs.io/en/latest/modules/client.html) supports direct and server-proxied requests. That choice belongs to the controller implementation and does not establish a Plex Web configuration option. The wiki review found no new workaround.

### Account registration experiment

A temporary, non-playing receiver was linked through plex.tv/link with its own identity and `client,player,pubsub-player` registration. Plex accepted publication of its local HTTP connection with HTTP 200, and `/devices.xml` contained the matching address. Publication was checked using both the numeric device ID and the client identifier. The legacy and v2 resources responses did not list the player. A separate saved controller credential for the same account confirmed its absence from v2 resources. The Plexamp discovery result reported by Music Assistant was not reproduced, and Plexamp itself was not tested.

In an isolated Plex Web `4.160.0` session, account registration alone did not add the receiver to the player selector. Enabling GDM for the same identity made it appear in PMS's `/clients` list and Web's selector. Selecting it sent timeline polls through PMS, not directly to the published address. This browser session selected an HTTP PMS connection and received HTTP 200 with a matching identifier. It did not reproduce the usual browser's HTTPS proxy route or establish a fix for that route. No media was played.

Cleanup removed the exact diagnostic device from plex.tv, withdrew its GDM advertisement, closed the test browser and receiver, and deleted its temporary credential file. Independent checks confirmed that both the cloud device and LAN player entries were gone and the installed MiSTer receiver remained available. No production registration, application, or configuration was changed.

## Remaining checkpoints

### Authenticated video control

Validate the implemented authorization policy with real Plex Web owner and Home-profile sessions. Automated tests already cover cross-profile rejection and missing/expired credentials. Verify direct and server-proxied requests. Device identifiers alone cannot authorize commands.

Exercise implemented play, pause, resume, stop, seek, and timelines on the CRT. Support subscriptions only if tested clients require them. Callbacks must be constrained to verified controller endpoints, have deadlines, and refuse redirects. Startup or network failures must preserve local browsing and playback.

Validate local/remote interaction, seeking during buffering, repeated commands, stopping during preparation, profile changes, sign-out, switching to Jellyfin, and reconnects. Advertise playback capabilities only as they become usable.

### Music and queues

Add ordered music queues, next/previous, shuffle, repeat, duplicate occurrences, queue refresh, and automatic advancement. Reuse the shared queue and playback controller. Keep Plex queue IDs, versions, and protocol rules in the adapter.

The first release excludes remote volume, track selection, menu navigation, photos, Live TV initiation, and internet-wide control. Confirm the tested client matrix before choosing release scope.
