package displaymode

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestINIProfileIndex(t *testing.T) {
	for _, tc := range []struct {
		name   string
		record []byte
		want   int
		fail   bool
	}{
		{"unset", []byte{0, 0, 0, 255}, 0, false},
		{"main", []byte{0x34, 0x99, 0xba, 0}, 0, false},
		{"first", []byte{0x34, 0x99, 0xba, 1}, 1, false},
		{"third", []byte{0x34, 0x99, 0xba, 3}, 3, false},
		{"invalid", []byte{0x34, 0x99, 0xba, 4}, 0, true},
		{"short", []byte{0x34}, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := iniProfileIndex(tc.record)
			if got != tc.want || (err != nil) != tc.fail {
				t.Fatalf("got %d, %v", got, err)
			}
		})
	}
}

func TestINIProfilePath(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"MiSTer.ini", "MiSTer_z.ini", "MiSTer_A.ini", "MiSTer_b.INI", "unrelated.ini"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("[MiSTer]\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	for i, name := range []string{"MiSTer.ini", "MiSTer_A.ini", "MiSTer_b.INI", "MiSTer_z.ini"} {
		got, err := iniProfilePath(root, i)
		if err != nil || got != filepath.Join(root, name) {
			t.Fatalf("index %d: %q, %v", i, got, err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "MiSTer_extra.ini"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := iniProfilePath(root, 1); err == nil {
		t.Fatal("accepted ambiguous alternate layout")
	}
	if _, err := iniProfilePath(root, 0); err != nil {
		t.Fatalf("Main should not depend on alternate layout: %v", err)
	}
}

func TestINIProfileRejectsMissingAndUnsafeFiles(t *testing.T) {
	for _, tc := range []struct {
		name  string
		file  string
		index int
		kind  string
	}{
		{"missing", "", 1, ""}, {"invalid-index", "", 4, ""},
		{"directory", "MiSTer_a.ini", 1, "directory"},
		{"symlink", "MiSTer_a.ini", 1, "symlink"},
		{"long-name", "MiSTer_" + strings.Repeat("a", 60) + ".ini", 1, "file"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, tc.file)
			var err error
			switch tc.kind {
			case "directory":
				err = os.Mkdir(path, 0700)
			case "symlink":
				err = os.Symlink("/missing", path)
			case "file":
				err = os.WriteFile(path, nil, 0600)
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err := iniProfilePath(root, tc.index); err == nil {
				t.Fatal("accepted unsafe profile")
			}
		})
	}
}

func TestConfigureINIOnlyChangesSelectedProfile(t *testing.T) {
	for _, tc := range []struct {
		name      string
		transform func([]byte) ([]byte, error)
	}{
		{"consolemode", consoleModeConfig}, {"interlaced", CoreConfig},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			backup := t.TempDir()
			main := filepath.Join(root, "MiSTer.ini")
			alternate := filepath.Join(root, "MiSTer_TV.ini")
			original := []byte("[MiSTer]\nvga_mode=rgb\n[Menu]\nvga_scaler=1\n")
			for _, path := range []string{main, alternate} {
				if err := os.WriteFile(path, original, 0600); err != nil {
					t.Fatal(err)
				}
			}
			selected, err := iniProfilePath(root, 1)
			if err != nil {
				t.Fatal(err)
			}
			for range 2 {
				if err := configureINI(selected, backup, tc.name, tc.transform); err != nil {
					t.Fatal(err)
				}
			}
			for _, path := range []string{main, filepath.Join(backup, "MiSTer_TV.ini.before-"+tc.name)} {
				got, err := os.ReadFile(path)
				if err != nil || !bytes.Equal(got, original) {
					t.Fatalf("original changed at %s: %v", path, err)
				}
			}
			got, err := os.ReadFile(alternate)
			if err != nil {
				t.Fatal(err)
			}
			want, err := tc.transform(original)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, want) {
				t.Fatal("selected profile differs from managed transform")
			}
			// A later edit must not replace the first backup.
			changed := append([]byte("; later edit\n"), original...)
			if err := os.WriteFile(alternate, changed, 0600); err != nil {
				t.Fatal(err)
			}
			if err := configureINI(selected, backup, tc.name, tc.transform); err != nil {
				t.Fatal(err)
			}
			got, err = os.ReadFile(filepath.Join(backup, "MiSTer_TV.ini.before-"+tc.name))
			if err != nil || !bytes.Equal(got, original) {
				t.Fatal("backup was replaced")
			}
		})
	}
}

func TestConfigureINIBackupFailureLeavesProfileUnchanged(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "MiSTer.ini")
	original := []byte("[MiSTer]\n")
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	if err := configureINI(path, filepath.Join(root, "missing"), "consolemode", consoleModeConfig); err == nil {
		t.Fatal("missing backup directory accepted")
	}
	got, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(got, original) {
		t.Fatal("profile changed after failed backup")
	}
}

func TestConfigureINIRejectsInvalidBackup(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "MiSTer.ini")
	original := []byte("[MiSTer]\n")
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path+".before-consolemode", 0700); err != nil {
		t.Fatal(err)
	}
	if err := configureINI(path, root, "consolemode", consoleModeConfig); err == nil {
		t.Fatal("accepted a directory as backup")
	}
	got, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(got, original) {
		t.Fatal("profile changed without a valid backup")
	}
}

func TestININameKeyPreservesNonASCII(t *testing.T) {
	if got := iniNameKey("MiSTer_É.ini"); got != "mister_É.ini" {
		t.Fatalf("host filename ordering changed: %q", got)
	}
}

func TestConfigureINIIgnoresInterruptedTemporaryBackup(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "MiSTer.ini")
	original := []byte("[MiSTer]\nvga_mode=rgb\n")
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	// Simulate a process killed while writing its unpublished temporary backup.
	if err := os.WriteFile(filepath.Join(root, ".mistervision-interrupted"), original[:4], 0600); err != nil {
		t.Fatal(err)
	}
	if err := configureINI(path, root, "consolemode", consoleModeConfig); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path + ".before-consolemode")
	if err != nil || !bytes.Equal(got, original) {
		t.Fatalf("backup incomplete: %q, %v", got, err)
	}
	configured, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(configured, []byte(consoleModeBlockStart)) {
		t.Fatal("configuration not published")
	}
}

func TestReplaceFailureRemovesUnpublishedFile(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "destination")
	if err := os.Mkdir(target, 0700); err != nil {
		t.Fatal(err)
	}
	if err := replace(target, []byte("new contents")); err == nil {
		t.Fatal("replaced a directory")
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "destination" || !entries[0].IsDir() {
		t.Fatal("failed publication changed destination or leaked a temporary file")
	}
}
