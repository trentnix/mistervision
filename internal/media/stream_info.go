package media

// VideoFormat describes source metadata or decoder input. Zero and empty fields
// mean unknown. Callers must not substitute requested limits for observations.
type VideoFormat struct {
	Codec         string
	Width, Height int
	FrameRate     float64
	BitRate       int64
}

// StreamDelivery distinguishes requested processing from server decisions.
// Decisions are absent when the provider did not report them. AddressClass is
// a literal-address classification, not proof that the server is on the LAN.
type StreamDelivery struct {
	Provider, RequestedMethod, VideoDecision, AudioDecision string
	AddressClass                                            string
	TLS                                                     bool
}
