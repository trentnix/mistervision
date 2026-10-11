package media

import "context"

// WatchedState is an optional capability for explicit movie and episode actions.
// SetWatched updates the authenticated user's state and clears the resume point.
// It must honor cancellation and report failures without exposing credentials.
type WatchedState interface {
	SetWatched(context.Context, string, bool) error
}
