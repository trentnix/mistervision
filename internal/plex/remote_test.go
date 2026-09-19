package plex

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"mistervision/internal/media"
	"mistervision/internal/serverstate"
)

func TestRemoteControllerMustMatchActiveViewer(t *testing.T) {
	account := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/user" {
			t.Error("unexpected account path")
		}
		switch r.Header.Get("X-Plex-Token") {
		case "owner":
			w.Write([]byte(`{"id":11}`))
		case "child":
			w.Write([]byte(`{"id":22}`))
		default:
			w.WriteHeader(http.StatusUnauthorized)
		}
	}))
	defer account.Close()
	c := NewClient(Config{}, serverstate.Session{UserID: "22", Token: "active-server-token"})
	c.accountURL = account.URL
	for _, token := range []string{"owner", "", "expired", "transient-123"} {
		if err := c.authorizeController(context.Background(), token); err == nil {
			t.Fatalf("accepted %s", token)
		}
	}
	if err := c.authorizeController(context.Background(), "child"); err != nil {
		t.Fatal(err)
	}
	c.Session.UserID = "11"
	if err := c.authorizeController(context.Background(), "child"); !errors.Is(err, media.ErrUnauthorized) {
		t.Fatalf("cross-profile result %v", err)
	}
	if err := c.authorizeController(context.Background(), "owner"); err != nil {
		t.Fatal(err)
	}
}

func TestRemoteVideoUsesActiveServerCredential(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Plex-Token") != "active-viewer" {
			t.Fatal("wrong credential")
		}
		switch r.URL.Path {
		case "/library/metadata/1":
			w.Write([]byte(`{"MediaContainer":{"Metadata":[{"ratingKey":"1","type":"movie","title":"Movie"}]}}`))
		case "/library/metadata/2":
			w.Write([]byte(`{"MediaContainer":{"Metadata":[{"ratingKey":"2","type":"track","title":"Music"}]}}`))
		default:
			w.WriteHeader(http.StatusForbidden)
		}
	}))
	defer server.Close()
	c := NewClient(Config{Server: server.URL}, serverstate.Session{Token: "active-viewer"})
	items, err := c.RemoteItems(context.Background(), []string{"1"}, false)
	if err != nil || len(items) != 1 || items[0].Type != "Movie" {
		t.Fatalf("movie %v %v", items, err)
	}
	for _, ids := range [][]string{{"2"}, {"3"}, {"../1"}, {"1", "2"}, nil} {
		if _, err := c.RemoteItems(context.Background(), ids, false); err == nil {
			t.Fatalf("accepted %v", ids)
		}
	}
}

func TestRemoteReceiverConstruction(t *testing.T) {
	c := NewClient(Config{}, serverstate.Session{UserID: "11", ServerID: "server", DeviceID: "device"})
	if c.remoteSource() == nil {
		t.Fatal("missing configured receiver")
	}
	c.Session.ServerID = ""
	if c.remoteSource() != nil {
		t.Fatal("receiver without active server")
	}
}
