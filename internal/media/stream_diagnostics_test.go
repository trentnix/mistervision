package media

import "testing"

func TestDeliveryAddressClassification(t *testing.T) {
	for _, tc := range []struct {
		url, class string
		tls        bool
	}{
		{"http://192.168.1.100:32400/?token=secret", "private", false},
		{"https://8.8.8.8/private", "public", true},
		{"https://server.plex.direct", "unknown", true},
		{"http://127.0.0.1", "loopback", false},
		{"http://[::ffff:192.168.1.100]", "private", false},
		{"http://[fd00::1]", "private", false},
	} {
		d := Delivery("plex", "transcode", tc.url)
		if d.AddressClass != tc.class || d.TLS != tc.tls {
			t.Fatalf("classification: %+v", d)
		}
	}
}
