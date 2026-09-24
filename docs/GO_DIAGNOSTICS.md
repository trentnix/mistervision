# Diagnostics

Diagnostics records startup, requests, and playback milestones. It is off by default and uses the same implementation on MiSTer and desktop.

## Automatic launcher log

The shipped MiSTer launcher captures early errors without enabling application diagnostics or editing JSON. After a failed launch, attach `/media/fat/mistervision/startup-error.log`. The file includes the installed package version, captured output, and launcher exit status. It survives menu restoration and later successful launches. A later failed launch replaces it. The version comes from the installed `VERSION` file and can differ from a manually replaced binary.

During a run, `startup.log` and `startup.log.1` hold at most 64 KiB each. The last-failure copy holds at most 128 KiB. If the application hangs or the machine resets before the launcher records an exit, collect the rolling pair instead. Recent output is flushed at least once per second while the logger can write. An abrupt power loss can still lose filesystem buffers. If the installation directory cannot be written at startup, the launcher uses `/tmp/mistervision-startup.log` and `/tmp/mistervision-startup-error.log`. Those temporary files do not survive reboot.

Review launcher logs before posting them. They capture raw process output, unlike the filtered application diagnostics below. Startup capture does not read settings or saved credentials. Logging failures do not prevent the application from running. The updated launcher can also capture failures from an installed v1.5.1 binary. Replace only `Scripts/MiSTerVision.sh` to use it with that release.

## Enable and collect logs

Merge the `diagnostics` section below into the existing object in `settings.json`. Keep the existing connection and display settings. Separate adjacent sections with a comma. Do not paste a second JSON object after the existing one. Restart, reproduce the issue, then exit normally:

```json
{
  "diagnostics": {
    "enabled": true,
    "path": "debug.log",
    "max_bytes": 1048576
  }
}
```

Collect `debug.log` and `debug.log.1` before launching again. Each process clears its log pair at startup. Relative paths resolve beside the settings file. The standard MiSTer log is `/media/fat/mistervision/debug.log`. An absolute path can select other storage, including `/tmp`.

Without a JSON `server` section, `DEBUGLOG` on a line in legacy `jellyfin.conf` enables the same defaults. With `server` configured, only the diagnostics section controls logging. Explicit `diagnostics.enabled` overrides it. To disable logging, set false, or omit the section and remove `DEBUGLOG`.

`max_bytes` applies to each file and accepts 4,096–67,108,864 bytes. Use a dedicated path because startup truncates it and owns its `.1` companion. Only one application instance can use a log path.

Display supervisors for interlaced output, HDMI scaling, and ConsoleMode have an additional pair, `debug.log.supervisor` and `debug.log.supervisor.1`. Collect both pairs for a display startup or exit problem. For ConsoleMode, also collect `/tmp/mistervision-consolemode.log` before rebooting. Each file uses the same limit, so defaults retain up to 4 MiB when a display supervisor runs. If the child never starts, its existing log can be from a previous run. Check timestamps.

## Events

Each line is JSON with a timestamp and event name in `msg`.

| Event family | Information |
| --- | --- |
| `application.start`, `.exit`, `.phase`, `.failure` | Build/platform, application or supervisor role, startup stage, elapsed time, and failure category. |
| `application.display` | Logical and physical dimensions. |
| `input.backend`, `.device`, `.unavailable` | Backend, initial devices and bindings, and identification/open failures. No button presses. |
| `mister.ini_profile` | Active INI filename, or a failure category if the host selection cannot be resolved. |
| `mister.display`, `.framebuffer`, `.setting`, `.settings` | Interlaced state, kernel framebuffer geometry, numeric INI settings, and known host/connector names. Includes the application’s handoff section and matching wildcard or `+Menu` groups. Groups use known section labels so private section names stay out of logs. |
| `mister.state`, `.state.framebuffer` | Known active hosts, ConsoleMode frontend presence, known core identity, active virtual terminal, and framebuffer geometry at startup and each ConsoleMode handoff stage. |
| `mister.consolemode.setting` | Saved CRT, video-mode, and rotation selections as raw numeric values with their byte lengths. Missing or malformed files produce a failure category, not guessed defaults. |
| `mister.handoff`, `.handoff.failure`, `.handoff.result` | ConsoleMode handoff stage, elapsed time, and failure category, including cleanup and return. |
| `update.start`, `.end`, `.recovered`, `.restart`, `.manager` | Installation start, completion flags for failure/cancellation/recovery, startup rollback, and a restart request after successful cleanup. Update-manager configuration read failures. No download URLs or raw errors. |
| `configuration.fallback` | Logical setting, safe error category, and selected recovery behavior. |
| `connection.discovery`, `connection.rediscovery` | Discovered server count and failure flag for initial discovery or remembered-address recovery. No server names or addresses. |
| `connection.address-recovered` | A confirmed new address was authenticated and saved successfully. No server names, addresses, or credentials. |
| `authentication.session-recovered` | Damaged saved sign-in was backed up and replaced. No file contents or paths. |
| `http.request`, `remote.socket` | Endpoint/status/timing or WebSocket connection result. No query strings or credentials. |
| `browser.page`, `.home` | Accepted page/feed results, bounded identifiers, counts, and failures. |
| `browser.artwork` | Failed image kind, without URLs or raw errors. Optional cover and background failures stay in diagnostics. Photo and authentication failures remain visible. |
| `browser.playback-ready`, `.subtitle` | First position and successful subtitle changes applied by the browser. Local decoder generation, position ticks, or selected stream index. |
| `playback.start`, `.phase`, `.prepared` | Decoder, preparation stages, resume offset, and requested transcode limits. |
| `playback.first-position`, `.first-frame` | Separate milestones for position feedback and first presented frame feedback. |
| `playback.pause`, `.buffering`, `.progress` | State transitions and ten-second progress summaries. |
| `playback.decoder-exit`, `.end` | Exit/signal, cancellation, elapsed time, and final stage. |
| `playback.preferences-write` | A failed preferences write, retained for retry. No item identifiers or paths. |
| `diagnostics.dropped` | Events discarded when the writer queue filled. |

A `playback.end` event with `error_kind=unsupported-display` at `stage=decoder-configuration` means native video rejected the framebuffer before contacting the server for playback. Check `application.display` for physical output dimensions and the [framebuffer requirements](GO_DISPLAY.md#hdmi-framebuffer-scaling).

For slow startup, compare request timing, `stream-open`, `decoder-start`, and first-frame feedback. For seeks, follow the new process-local `playback` counter. Cancellation can mean Stop, a superseded seek, or exit, rather than failure. First-frame feedback is not a measurement of light from the CRT.

If picture or track choices disappear after restart, look for `playback.preferences-write` and check that the state directory is writable. A later save retries failed writes, and shutdown makes a final attempt. An unexpected sign-in prompt with `authentication.session-recovered` indicates [damaged saved sign-in](GO_CONFIGURATION.md#saved-sign-in-recovery). Do not share the backup file.

Jellyfin metadata/artwork timings include buffered body reads. Its `/media-stream` event measures opening through response headers, not the whole stream. Its `/audio-stream` event includes one proxy request through completion. Plex labels ordinary API requests `/plex-request` and tune requests `/livetv/dvrs/:dvr/channels/:channel/tune`. Both omit private identifiers, and timing includes the buffered response body. GitHub release checks are outside the media-server request log.

The MiSTer inventory reads numeric display settings from `/media/fat/MiSTer.ini`, preserving section identity. It does not resolve alternate INI files or prove active signal timing. Reads are bounded to 128 KiB and 64 setting events.

## Configuration fallbacks

Handled invalid title, background, sound, and music settings record one event when recovery occurs. Missing custom assets also record their fallback. Intentional omissions, empty values, and disabled settings are not failures.

```json
{
  "msg": "configuration.fallback",
  "configuration": "ui",
  "error_kind": "invalid",
  "fallback": "default-title"
}
```

Invalid diagnostics settings or file-open failures disable logging, write a short stderr message, and queue a settings notice. No alternate log file is opened. Malformed `settings.json` can stop startup before its diagnostics section is usable. Invalid command-line arguments use stderr. Preview commands do not enable logging.

## Limits and privacy

A 128-entry queue and one writer keep disk I/O off playback and UI loops. A full queue drops events. Normal exit drains accepted events. Abrupt termination can lose them. Write failures disable logging without stopping playback.

Logs exclude server origins, queries, authorization, response bodies, raw network/player errors, media titles, captions, and Quick Connect secrets. They can contain item/user identifiers in endpoint paths, device names, build details, numeric display settings, and playback timing. Review logs before sharing. Files request owner-only permissions where supported.

Diagnostics does not measure dropped frames, rendered FPS, decoder load, or A/V drift. Those require player instrumentation and hardware checks. Position and buffering events alone cannot establish smooth playback.

## Stream details

`playback.source` records the selected source video's codec, dimensions, frame rate, and bitrate from server metadata. `playback.prepared` records requested transcode limits, not the delivered format. `playback.delivery` records the provider, requested method, and separate server-reported video/audio decisions. Plex decisions come from its existing negotiation response. A successful negotiation without explicit decisions leaves those fields `unknown`. Jellyfin's generated transcode request is identified as a request, not a server-confirmed decision.

`playback.decoder-input` records MPlayer's identification of the received video before client-side scaling: codec, dimensions, frame rate, and bitrate when reported. Initial unknown fields are replaced by cumulative observations as MPlayer opens the stream. This works with the existing bundled player and does not require a replacement decoder. Other decoder implementations currently leave these fields unknown. Frame rate is stream metadata, not measured rendering speed. Bitrate may be absent or estimated by the decoder. These events do not measure tearing or dropped frames.

Missing or invalid numeric observations are `null`. Unrecognized codec and decision identifiers are `unknown`. Titles, source IDs, file paths, addresses, and authentication values are excluded from these events. Codec names and decisions use allowlists instead of accepting arbitrary server/player text.

Delivery also includes `tls` and `address_class`. Literal IP addresses are classified as `private`, `public`, or `loopback`. Hostnames and unclassifiable addresses remain `unknown`. Private addresses do not prove the server is on the local network, and public addresses do not prove the connection crosses the internet. No DNS lookup is added for diagnostics.
