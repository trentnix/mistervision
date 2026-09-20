package main

import (
	"os"
	"path/filepath"
	"testing"

	"mistervision/internal/browser"
)

func TestUpdaterOnlyTargetsPermanentPair(t *testing.T) {
	good := launchOptions{browse: true, player: installationRoot + "/mplayer-arm"}
	if installedUpdater(good, installationRoot+"/mistervision") == nil {
		t.Fatal("permanent pair disabled")
	}
	for _, kind := range []string{"headless", "preview", "custom-player", "custom-client"} {
		t.Run(kind, func(t *testing.T) {
			o := good
			executable := installationRoot + "/mistervision"
			switch kind {
			case "headless":
				o.headless = "640x240"
			case "preview":
				o.browse = false
			case "custom-player":
				o.player = "/tmp/player"
			case "custom-client":
				executable = "/tmp/client"
			}
			if installedUpdater(o, executable) != nil {
				t.Fatal("enabled updater for a different installation")
			}
		})
	}
}

// Registration changes installation ownership, never the recovery capability.
func TestInstalledUpdateOwnership(t *testing.T) {
	for _, kind := range []string{"manual", "downloader", "unreadable"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			if kind != "manual" {
				text := "[MultiDatabases/mister-vision]\ndb_url=https://example.org/db.json\n"
				if kind == "unreadable" {
					text = "[MultiDatabases/mister-vision]\n"
				}
				if err := os.WriteFile(filepath.Join(root, "downloader.ini"), []byte(text), 0600); err != nil {
					t.Fatal(err)
				}
			}
			o := launchOptions{browse: true, player: installationRoot + "/mplayer-arm"}
			installer := installedUpdater(o, installationRoot+"/mistervision")
			if installer == nil {
				t.Fatal("recovery capability lost")
			}
			var config browser.Config
			err := configureInstalledUpdates(&config, installer, root)
			if (err != nil) != (kind == "unreadable") {
				t.Fatalf("unexpected error: %v", err)
			}
			if kind == "manual" {
				if config.Updater == nil || config.UpdateInstructions != "" {
					t.Fatal("manual installation disabled")
				}
			} else if config.Updater != nil || config.UpdateInstructions == "" {
				t.Fatal("external management not enforced")
			}
		})
	}
}

// Restarting with a changed registration must restore the appropriate updater.
func TestInstalledUpdateRegistrationLifecycle(t *testing.T) {
	t.Setenv("MISTERVISION_AUTO_RESTART", "1")
	root := t.TempDir()
	path := filepath.Join(root, "downloader_MultiDatabases_mister-vision.ini")
	installer := installedUpdater(launchOptions{browse: true, player: installationRoot + "/mplayer-arm"}, installationRoot+"/mistervision")
	var config browser.Config
	for _, state := range []string{"manual", "registered", "invalid", "removed"} {
		switch state {
		case "registered":
			if err := os.WriteFile(path, []byte("[MultiDatabases/mister-vision]\ndb_url=https://example.org/db.json\n"), 0600); err != nil {
				t.Fatal(err)
			}
		case "invalid":
			if err := os.WriteFile(path, []byte("[MultiDatabases/mister-vision]\n"), 0600); err != nil {
				t.Fatal(err)
			}
		case "removed":
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
		}
		err := configureInstalledUpdates(&config, installer, root)
		if (err != nil) != (state == "invalid") {
			t.Fatalf("%s: %v", state, err)
		}
		builtin := state == "manual" || state == "removed"
		if (config.Updater != nil) != builtin || config.RestartAfterUpdate != builtin || (config.UpdateInstructions == "") != builtin {
			t.Fatalf("%s: incorrect update ownership", state)
		}
	}
}
