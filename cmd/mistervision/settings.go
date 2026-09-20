package main

import (
	"errors"
	"os"
	"path/filepath"

	"mistervision/internal/jellyfin"
	"mistervision/internal/settings"
)

// loadSettings resolves the one application settings file. Missing default
// settings enable legacy-file compatibility. Explicit overrides must exist,
// except when creating that file with the migration command. A packaged native
// launcher may seed the display choice for a fresh installation only.
func loadSettings(o launchOptions) (*settings.File, error) {
	path := o.settingsPath
	if path == "" {
		path = filepath.Join(filepath.Dir(o.config), "settings.json")
	}
	source, err := settings.Load(path, o.settingsPath != "" && !o.migrateSettings)
	if err != nil || o.settingsPath != "" || !o.browse || o.headless != "" || o.migrateSettings {
		return source, err
	}
	preset := os.Getenv("MISTERVISION_INITIAL_INTERLACED")
	if preset != "0" && preset != "1" {
		return source, nil
	}
	// An existing legacy connection also identifies an established installation.
	if _, err := os.Lstat(o.config); !errors.Is(err, os.ErrNotExist) {
		return source, err
	}
	if err := source.InitializeDisplay(preset == "1"); err != nil {
		return nil, err
	}
	return settings.Load(path, false)
}

// migrateSettings consolidates legacy connection and application settings without
// opening a display or contacting a server. Existing sign-in files stay untouched.
func migrateSettings(o launchOptions, source *settings.File) error {
	if section := source.Section("server"); section.Data != nil || section.Err != nil {
		return errors.New(messageMigrationNotNeeded)
	}
	legacy, err := jellyfin.LoadConfig(o.config)
	if errors.Is(err, os.ErrNotExist) {
		return source.Migrate()
	}
	if err != nil {
		return err
	}
	server := settings.Server{Provider: "jellyfin", URL: legacy.Server, InsecureTLS: legacy.InsecureTLS, Transcode: settings.Transcode{
		MaxWidth: legacy.Transcode.MaxWidth, MaxHeight: legacy.Transcode.MaxHeight, VideoBitrate: legacy.Transcode.VideoBitrate}}
	if legacy.APIKey != "" || legacy.Username != "" {
		server.Jellyfin = &settings.JellyfinLogin{APIKey: legacy.APIKey, Username: legacy.Username}
	}
	return source.MigrateServer(server, legacy.DebugLog)
}
