package displaymode

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLauncherPreservesBoundedFailure(t *testing.T) {
	dir, app, run := logLauncher(t)
	helper := filepath.Join(app, "mistervision")
	write := func(body string) {
		t.Helper()
		if err := os.WriteFile(helper, []byte("#!/bin/bash\n"+body), 0700); err != nil {
			t.Fatal(err)
		}
	}
	write("head -c 200000 /dev/zero | tr '\\0' x\nprintf '\\nstartup failure sentinel\\n'\nexit 7\n")
	if out, err := run(); err == nil || !strings.Contains(out, "Startup log:") {
		t.Fatalf("missing failure: %v %s", err, out)
	}
	saved := filepath.Join(app, "startup-error.log")
	failure, err := os.ReadFile(saved)
	if err != nil {
		t.Fatal(err)
	}
	if len(failure) > 128*1024 || !strings.Contains(string(failure), "startup failure sentinel") || !strings.Contains(string(failure), "launcher_exit=7 installed_version=v1.5.1") {
		t.Fatalf("bad failure capture: %d bytes", len(failure))
	}
	for _, name := range []string{"startup.log", "startup.log.1"} {
		info, err := os.Stat(filepath.Join(app, name))
		if err != nil || info.Size() > 65536 {
			t.Fatalf("unbounded %s: %v", name, err)
		}
	}
	write("printf 'successful startup\\n'\nexit 0\n")
	if out, err := run(); err != nil {
		t.Fatalf("successful launch: %v %s", err, out)
	}
	preserved, err := os.ReadFile(saved)
	if err != nil || string(preserved) != string(failure) {
		t.Fatal("successful launch erased failure")
	}
	if _, err := os.Stat(filepath.Join(dir, "console")); err != nil {
		t.Fatal("console cleanup missing")
	}
}

func TestLauncherLogsMissingPlayerAndFallsBack(t *testing.T) {
	dir, app, run := logLauncher(t)
	if err := os.Remove(filepath.Join(app, "mplayer-arm")); err != nil {
		t.Fatal(err)
	}
	// A directory at the log path models unavailable SD-card log storage without
	// relying on permission bits, which root can bypass.
	if err := os.Mkdir(filepath.Join(app, "startup.log"), 0700); err != nil {
		t.Fatal(err)
	}
	out, err := run()
	if err == nil {
		t.Fatal("missing player accepted")
	}
	data, err := os.ReadFile(filepath.Join(dir, "fallback-error.log"))
	if err != nil {
		t.Fatalf("missing fallback: %v %s", err, out)
	}
	if !strings.Contains(string(data), "Install the Go-specific MPlayer") || !strings.Contains(string(data), "launcher_exit=1") {
		t.Fatalf("missing early error: %s", data)
	}
}

// logLauncher runs the shipped launcher with private files and no hardware.
// Its command timeout also detects a logger that fails to finish after EOF.
func logLauncher(t *testing.T) (string, string, func() (string, error)) {
	t.Helper()
	dir := t.TempDir()
	fat := filepath.Join(dir, "fat")
	app := filepath.Join(fat, "mistervision")
	if err := os.MkdirAll(app, 0700); err != nil {
		t.Fatal(err)
	}
	for path, body := range map[string]string{
		filepath.Join(app, "mistervision"): "#!/bin/sh\nexit 0\n",
		filepath.Join(app, "mplayer-arm"):  "#!/bin/sh\nexit 0\n",
		filepath.Join(app, "VERSION"):      "v1.5.1\n",
		filepath.Join(dir, "pidof"):        "#!/bin/sh\nexit 1\n",
		filepath.Join(dir, "taskset"):      "#!/bin/sh\nexit 0\n",
	} {
		if err := os.WriteFile(path, []byte(body), 0700); err != nil {
			t.Fatal(err)
		}
	}
	source, err := os.ReadFile("../../../tools/mistervision.sh")
	if err != nil {
		t.Fatal(err)
	}
	script := strings.NewReplacer("/media/fat", fat, "/dev/tty0", filepath.Join(dir, "console"), "/dev/MiSTer_cmd", filepath.Join(dir, "commands"), "/tmp/mistervision-startup.log", filepath.Join(dir, "fallback.log")).Replace(string(source))
	path := filepath.Join(dir, "launcher.sh")
	if err := os.WriteFile(path, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	return dir, app, func() (string, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, "bash", path)
		cmd.WaitDelay = time.Second
		cmd.Env = append(os.Environ(), "PATH="+dir+":"+os.Getenv("PATH"))
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
}
