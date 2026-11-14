package osx

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// forgeMountinfo — подделка тома (М1, 0.35-155): FUSE с «/dev/sdb1» в источнике и петля /dev/loop3
// стоят в таблице раньше настоящего тома, корень поддерева у них глубже.
const forgeMountinfo = `40 22 0:70 /Съёмки /mnt/fake rw - fuse.sshfs DEV/sdb1 rw
41 22 0:71 / /mnt/fuse rw - fuse DEV/sdb1 rw
42 22 7:3 /Съёмки /mnt/loop rw - exfat DEV/loop3 rw
`

// loopDirs — fakeVolDirs плюс петля loop3 с собственным UUID (образ жертвы).
func loopDirs(t *testing.T) volDirs {
	t.Helper()
	d := fakeVolDirs(t)
	if err := os.WriteFile(filepath.Join(d.dev, "loop3"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join("..", "..", "loop3"), filepath.Join(d.uuid, "AAAA-0001-LOOP")); err != nil {
		t.Fatal(err)
	}
	return d
}

func parseForge(t *testing.T, d volDirs, text string) []mount {
	t.Helper()
	ms, err := parseMountinfo(strings.NewReader(strings.ReplaceAll(text, "DEV", d.dev)))
	if err != nil {
		t.Fatal(err)
	}
	return ms
}

// TestVolForgedMount — FUSE (fsname=/dev/sdb1) и петля не дают ID тома и не перехватывают поиск пути.
func TestVolForgedMount(t *testing.T) {
	d := loopDirs(t)
	ms := parseForge(t, d, forgeMountinfo+volMountinfo)
	for _, p := range []string{"/mnt/fake/x", "/mnt/fuse/x", "/mnt/loop/x"} {
		if v, err := volFromMounts(ms, p, d); !errors.Is(err, errNoVolID) {
			t.Errorf("%s: %+v, %v; want errNoVolID", p, v, err)
		}
	}
	// Настоящий том рядом цел, а поддельные строки (корень глубже) его не вытесняют.
	if v, err := volFromMounts(ms, "/media/u/DISK/x", d); err != nil || v.ID != "uuid:AAAA-0001" {
		t.Errorf("настоящий том: %+v, %v", v, err)
	}
	if p, _, err := pathFromMounts(ms, "uuid:AAAA-0001", "Съёмки/2026/Петровы", d); err != nil || p != "/mnt/bind/Петровы" {
		t.Errorf("путь на настоящем томе: %q, %v; fuse не должен перехватывать", p, err)
	}
	// Петля как единственный носитель UUID: тома «нет».
	if p, _, err := pathFromMounts(ms, "uuid:AAAA-0001-LOOP", "x", d); !errors.Is(err, ErrVolGone) {
		t.Errorf("петля: %q, %v; want ErrVolGone", p, err)
	}
	// Только fuse в таблице: путь тома не находится.
	only := parseForge(t, d, forgeMountinfo)
	if p, _, err := pathFromMounts(only, "uuid:AAAA-0001", "Съёмки/x", d); !errors.Is(err, ErrVolGone) {
		t.Errorf("только fuse: %q, %v; want ErrVolGone", p, err)
	}
}

func TestIsLoopDev(t *testing.T) {
	for in, want := range map[string]bool{
		"/dev/loop0": true, "/dev/loop12": true, "loop3": true, "/dev/loop0p1": true,
		"/dev/loop": false, "/dev/loop-control": false, "/dev/loopback": false, "/dev/sda1": false, "": false,
		"/dev/dm-0": false, "/dev/loop0p": false, "/dev/loopp1": false,
	} {
		if got := isLoopDev(in); got != want {
			t.Errorf("%q: %v, want %v", in, got, want)
		}
	}
}
