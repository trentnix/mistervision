package remote

// PlaybackObserver is an optional Source capability for controllers that need
// player state directly. Providers that already report through media.Progress
// can omit it. PublishPlayback must not perform I/O or block on network work.
// Calls come from the browser loop, including an initial snapshot before
// Source.Run starts. Calls may overlap Run and stop when its session is canceled.
type PlaybackObserver interface {
	PublishPlayback(PlaybackState)
}

// PlaybackStatus describes observed playback, independently of a pending command.
// Seeking does not mean the requested destination has been reached.
type PlaybackStatus string

const (
	Stopped   PlaybackStatus = "stopped"
	Loading   PlaybackStatus = "loading"
	Playing   PlaybackStatus = "playing"
	Paused    PlaybackStatus = "paused"
	Buffering PlaybackStatus = "buffering"
	Seeking   PlaybackStatus = "seeking"
	Failed    PlaybackStatus = "error"
)

// PlaybackState is an immutable, credential-free snapshot. Times use
// 100-nanosecond ticks. PositionTicks is the latest decoder position, never the
// pending seek target. ItemID belongs to the active authenticated catalog.
// Zero values describe an idle player. Queue occurrences remain in QueueState.
// A snapshot reports playback facts, not acknowledgment of a particular command.
type PlaybackState struct {
	Status                       PlaybackStatus
	ItemID                       string
	PositionTicks, DurationTicks int64
	Audio, Live                  bool
}
