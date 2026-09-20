package update

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const registration = "[MultiDatabases/mister-vision]\ndb_url = https://raw.githubusercontent.com/theypsilon/MultiDatabases_MiSTer/db/mister-vision/db.json\n"

func TestDownloaderRegistrationLocations(t *testing.T) {
	for _, name := range []string{"downloader.ini", "downloader/mistervision.ini", "downloader_MultiDatabases_mister-vision.ini"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, name)
			if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
				t.Fatal(err)
			}
			content := registration
			if name == "downloader.ini" {
				content = "[MiSTer]\nbase_path=/media/fat\n[other]\ndb_url=https://example.org/db.json\n" + registration
			}
			if err := os.WriteFile(path, []byte(content), 0600); err != nil {
				t.Fatal(err)
			}
			got, err := DownloaderRegistered(root)
			if err != nil || !got {
				t.Fatalf("registration lost: %v %v", got, err)
			}
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			got, err = DownloaderRegistered(root)
			if err != nil || got {
				t.Fatalf("registration persisted: %v %v", got, err)
			}
		})
	}
}

func TestDownloaderRegistrationParsing(t *testing.T) {
	for _, tc := range []struct {
		name, text string
		want, fail bool
	}{
		{"absent", "", false, false},
		{"published", registration, true, false},
		{"unrelated", "[MultiDatabases/misterfin]\ndb_url=x", false, false},
		{"commented", ";[MultiDatabases/mister-vision]\n;db_url=x", false, false},
		{"quoted CRLF", "[MULTIDATABASES/MISTER-VISION] ; comment\r\nDB_URL: 'https://example.org/db.json' # comment\r\n[other]\ndb_url=x", true, false},
		{"missing URL", "[MultiDatabases/mister-vision]\n[other]\ndb_url=x", false, true},
		{"empty URL", "[MultiDatabases/mister-vision]\ndb_url=\"\"", false, true},
		{"duplicate URL", registration + "db_url=x", false, true},
		{"first section wins", registration + "[MultiDatabases/mister-vision]\ndb_url=", true, false},
		{"invalid first section wins", "[MultiDatabases/mister-vision]\n" + registration, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := registeredDatabase(strings.NewReader(tc.text))
			if got != tc.want || (err != nil) != tc.fail {
				t.Fatalf("got %v %v", got, err)
			}
		})
	}
}

func TestDownloaderRegistrationPrecedenceAndLimits(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "downloader"), 0700); err != nil {
		t.Fatal(err)
	}
	write := func(name, data string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, name), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write("downloader/.hidden.ini", registration)
	write("downloader_MultiDatabases_mister-vision.ini.disabled", registration)
	write("downloader_empty.ini", "; removed registration")
	if got, err := DownloaderRegistered(root); got || err != nil {
		t.Fatal("hidden or disabled file treated as active")
	}
	write("downloader/first.ini", registration)
	write("downloader_second.ini", "[MultiDatabases/mister-vision]\n")
	if got, err := DownloaderRegistered(root); !got || err != nil {
		t.Fatal("drop-in directory should win")
	}
	write("downloader.ini", "[MultiDatabases/mister-vision]\n")
	if _, err := DownloaderRegistered(root); err == nil {
		t.Fatal("invalid main registration must not fall through")
	}
	write("downloader.ini", strings.Repeat("x", (256<<10)+1))
	if _, err := DownloaderRegistered(root); err == nil {
		t.Fatal("oversized configuration accepted")
	}
}
