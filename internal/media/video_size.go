package media

// VideoSize bounds square-pixel server output. Servers preserve the source
// aspect ratio within this rectangle and need not upscale smaller sources.
// A zero size retains the legacy 720×576 limit for callers without a display.
type VideoSize struct {
	Width, Height int
}

// Capped applies explicit server limits without changing the requested aspect
// ratio policy. Zero caps leave the display-derived limit unchanged.
func (s VideoSize) Capped(width, height int) VideoSize {
	if s.Width <= 0 || s.Height <= 0 {
		s = VideoSize{width, height}
		if s.Width <= 0 {
			s.Width = 720
		}
		if s.Height <= 0 {
			s.Height = 576
		}
	}
	if width > 0 {
		s.Width = min(s.Width, width)
	}
	if height > 0 {
		s.Height = min(s.Height, height)
	}
	// The video encoders use even chroma dimensions. Round limits down.
	s.Width -= s.Width % 2
	s.Height -= s.Height % 2
	return s
}
