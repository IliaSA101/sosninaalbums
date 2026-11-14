package app

import (
	"os"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/osx"
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
)

// TestLinuxOptions: ProgramName = имя папки данных (у стенда своё), GPU-политика Never,
// значок передан как есть; пустой значок не роняет сборку опций.
func TestLinuxOptions(t *testing.T) {
	icon := []byte{0x89, 'P', 'N', 'G'}
	for _, stand := range []string{"", "b", "qa2"} {
		data := osx.DataName(stand)
		o := linuxOptions(data, icon)
		if o.ProgramName != data {
			t.Errorf("стенд %q: ProgramName %q, нужно %q", stand, o.ProgramName, data)
		}
		if o.WebviewGpuPolicy != linux.WebviewGpuPolicyNever {
			t.Errorf("стенд %q: GPU-политика %v, нужна Never", stand, o.WebviewGpuPolicy)
		}
		if len(o.Icon) == 0 || o.Icon[1] != 'P' {
			t.Errorf("стенд %q: значок %v", stand, o.Icon)
		}
	}
	if o := linuxOptions(osx.AppName, nil); len(o.Icon) != 0 || o.ProgramName != "SosninaAlbums" {
		t.Errorf("без значка: %+v", o)
	}
}

// TestLinuxIconEmbedded: встроенный в main значок (build/appicon.png) — настоящий PNG, а не
// пустой: иначе окно Linux осталось бы без значка. Файл читается тем же путём, что go:embed.
func TestLinuxIconEmbedded(t *testing.T) {
	raw := mustRead(t, "../../build/appicon.png")
	if len(raw) < 1000 || string(raw[1:4]) != "PNG" {
		t.Fatalf("build/appicon.png: %d байт, не PNG", len(raw))
	}
}

// TestAppMenuDarwin: на macOS — три роли по порядку: App (Cmd+Q/H), Edit (Cmd+C/V/X/A/Z), Window.
func TestAppMenuDarwin(t *testing.T) {
	m := appMenu("darwin")
	if m == nil {
		t.Fatal("меню macOS nil")
	}
	want := []menu.Role{menu.AppMenuRole, menu.EditMenuRole, menu.WindowMenuRole}
	if len(m.Items) != len(want) {
		t.Fatalf("пунктов %d, нужно %d", len(m.Items), len(want))
	}
	for i, it := range m.Items {
		if it.Role != want[i] {
			t.Errorf("пункт %d: роль %v, нужна %v", i, it.Role, want[i])
		}
	}
}

// TestAppMenuOtherOS: на остальных ОС меню нет (nil) — Wails не рисует родную строку над нашей.
func TestAppMenuOtherOS(t *testing.T) {
	for _, goos := range []string{"windows", "linux", "", "freebsd", "Darwin"} {
		if m := appMenu(goos); m != nil {
			t.Errorf("%q: меню %+v, нужен nil", goos, m)
		}
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
