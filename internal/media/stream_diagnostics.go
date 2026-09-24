package media

import (
	"net/netip"
	"net/url"
	"strings"
)

// Delivery describes a request without retaining its server address or secrets.
// Hostnames remain unknown because resolving them adds network work and does
// not establish whether playback travels over a local or remote connection.
func Delivery(provider, method, raw string) StreamDelivery {
	d := StreamDelivery{Provider: provider, RequestedMethod: method, AddressClass: "unknown"}
	u, err := url.Parse(raw)
	if err != nil {
		return d
	}
	d.TLS = u.Scheme == "https"
	if ip, err := netip.ParseAddr(u.Hostname()); err == nil {
		ip = ip.Unmap()
		switch {
		case ip.IsLoopback():
			d.AddressClass = "loopback"
		case ip.IsPrivate() || ip.IsLinkLocalUnicast():
			d.AddressClass = "private"
		case ip.IsGlobalUnicast():
			d.AddressClass = "public"
		}
	}
	return d
}

// DiagnosticCodec allowlists codec identifiers so server text cannot introduce
// titles, URLs, or credentials into logs. Unsupported identifiers stay unknown.
func DiagnosticCodec(value string) string {
	switch strings.ToLower(value) {
	case "h264", "avc", "avc1", "ffh264":
		return "h264"
	case "hevc", "h265", "ffhevc":
		return "hevc"
	case "mpeg2video", "mpeg2", "ffmpeg2":
		return "mpeg2video"
	case "mpeg1video", "ffmpeg1":
		return "mpeg1video"
	case "mpeg4", "ffodivx":
		return "mpeg4"
	case "vp8", "ffvp8":
		return "vp8"
	case "vp9", "ffvp9":
		return "vp9"
	case "av1", "ffav1", "fflibdav1d":
		return "av1"
	case "vc1", "ffvc1":
		return "vc1"
	case "mjpeg", "ffmjpeg":
		return "mjpeg"
	}
	return "unknown"
}

// DiagnosticDecision accepts only known server processing decisions.
func DiagnosticDecision(value string) string {
	switch value {
	case "transcode", "copy", "directplay", "directstream":
		return value
	}
	return "unknown"
}
