package platform

// ResolveAspect returns the screen width/height ratio. Auto preserves known
// non-square-pixel CRT rasters and assumes square pixels for other outputs.
// Callers must validate setting before opening hardware.
func ResolveAspect(setting string, width, height int) float64 {
	switch setting {
	case "4:3":
		return 4.0 / 3
	case "16:9":
		return 16.0 / 9
	}
	if width == 640 && (height == 240 || height == 288 || height == 480 || height == 576) {
		return 4.0 / 3
	}
	if width <= 0 || height <= 0 {
		return 4.0 / 3
	}
	return float64(width) / float64(height)
}

// VideoPresenter accepts a logical video frame that fills the complete output.
// Browsing still uses Present and its 4:3 viewport. Pixels remain borrowed.
type VideoPresenter interface {
	PresentVideo(pixels []byte) error
}

// PresentVideo uses full-output presentation when supported. Simple presenters
// can retain their normal presentation path, including test and companion outputs.
func PresentVideo(d Presenter, pixels []byte) error {
	if video, ok := d.(VideoPresenter); ok {
		return video.PresentVideo(pixels)
	}
	return d.Present(pixels)
}
