package buildkit

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildInitScript_NoCommand(t *testing.T) {
	if _, err := BuildInitScript(nil, nil, nil, ""); err == nil {
		t.Fatal("expected error for image with no ENTRYPOINT/CMD")
	}
}

func TestBuildInitScript_ExecutesWithQuoting(t *testing.T) {
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("no sh")
	}
	dir := t.TempDir()
	// Hostile values: quotes, spaces, $, backticks, ; must all stay literal.
	script, err := BuildInitScript(
		[]string{"/bin/echo"},
		[]string{"a b", `it's`, "$HOME", "`id`", "x;y"},
		[]string{"FOO=bar baz", `Q=it's "q"`, "1BAD=x", "NOEQ", "EMPTY="},
		dir,
	)
	if err != nil {
		t.Fatal(err)
	}
	// Stub the pseudo-fs setup: mount/mkdir must be no-ops for an unprivileged test.
	script = strings.NewReplacer(
		"mount -t", ": mount -t",
		"mkdir -p /dev/pts /dev/shm && ", "",
	).Replace(script)
	if strings.Contains(script, "1BAD") || strings.Contains(script, "NOEQ") {
		t.Fatal("invalid env entries must be dropped")
	}
	out, err := exec.Command(sh, "-c", script+"").CombinedOutput()
	if err != nil {
		t.Fatalf("run: %v: %s", err, out)
	}
	want := "a b it's $HOME `id` x;y\n"
	if string(out) != want {
		t.Fatalf("got %q want %q", out, want)
	}
}

func TestBuildInitScript_WorkdirAndEnvVisible(t *testing.T) {
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("no sh")
	}
	dir := t.TempDir()
	script, err := BuildInitScript([]string{"/bin/sh", "-c"}, []string{`echo "$FOO:$(pwd)"`}, []string{"FOO=bar"}, dir)
	if err != nil {
		t.Fatal(err)
	}
	script = strings.NewReplacer("mount -t", ": mount -t", "mkdir -p /dev/pts /dev/shm && ", "").Replace(script)
	out, err := exec.Command(sh, "-c", script).CombinedOutput()
	if err != nil {
		t.Fatalf("run: %v: %s", err, out)
	}
	resolved, _ := filepath.EvalSymlinks(dir)
	if got := strings.TrimSpace(string(out)); got != "bar:"+resolved && got != "bar:"+dir {
		t.Fatalf("got %q", got)
	}
}

func TestInstallInit_ReplacesSymlinkWithoutWritingThrough(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "sbin"), 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "bin-busybox")
	if err := os.WriteFile(target, []byte("BUSYBOX"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(root, InitPath)); err != nil {
		t.Fatal(err)
	}
	if err := InstallInit(root, "#!/bin/sh\nexec true\n"); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(target); string(b) != "BUSYBOX" {
		t.Fatalf("wrote through symlink and clobbered target: %q", b)
	}
	fi, err := os.Lstat(filepath.Join(root, InitPath))
	if err != nil || fi.Mode()&os.ModeSymlink != 0 || fi.Mode().Perm() != 0o755 {
		t.Fatalf("init not a 0755 regular file: %v %v", fi, err)
	}
}
