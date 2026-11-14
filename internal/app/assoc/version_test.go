package assoc

import (
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
)

func TestCmpVer(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"0.26.0", "0.25.3", 1},
		{"0.25.3", "0.26.0", -1},
		{"0.25.0", "0.25.0", 0},
		{"0.25.0", "0.25.0-rc2", 1},
		{"0.25.0-rc2", "0.25.0", -1},
		{"0.25.0-rc2", "0.25.0-rc10", 1}, // пред-выпуски — по строке: порядок rc важен лишь внутри одной версии
		{"0.25.0+abc", "0.25.0", 0},
		{"0.10.0", "0.9.9", 1},
		{"dev", "0.0.1", -1},
		{"0.0.1", "dev", 1},
		{"", "0.0.1", -1},
		{"dev", "", 0},
		{"1.0", "0.9.0", -1}, // «1.0» не x.y.z: не версия
	}
	for _, c := range cases {
		if got := cmpVer(c.a, c.b); got != c.want {
			t.Errorf("cmpVer(%q, %q) = %d, ждали %d", c.a, c.b, got, c.want)
		}
	}
}

// Две цифры совпали (файл 0.25.1, программа 0.25.0) — в тексте три; разошлись — две как раньше;
// полной версии нет — остаются две.
func TestShownVers(t *testing.T) {
	cases := []struct{ theirs, mine, wantT, wantM string }{
		{"0.25.1", "0.25.0", "0.25.1", "0.25.0"},
		{"0.25.1-rc2+abc", "0.25.0-dev", "0.25.1", "0.25.0"},
		{"0.25.0", "0.25.0", "0.25.0", "0.25.0"},
		{"0.26.0", "0.25.3", "0.26", "0.25"},
		{"0.25", "0.25.0", "0.25", "0.25"},
		{"0.25.1", "0.25", "0.25", "0.25"},
		{"0.25.1", "dev", "0.25", ""},
		{"", "0.25.0", "", "0.25"},
		{"dev", "", "", ""},
	}
	for _, c := range cases {
		if gt, gm := shownVers(c.theirs, c.mine); gt != c.wantT || gm != c.wantM {
			t.Errorf("shownVers(%q, %q) = %q, %q; ждали %q, %q", c.theirs, c.mine, gt, gm, c.wantT, c.wantM)
		}
	}
	err := &salbum.NewerError{Version: model.FormatVersion + 1, App: "0.25.1"}
	if got, want := NewerText(err, "альбом", "0.25.0"), "альбом сохранён в версии 0.25.1. Эта программа — 0.25.0: обновите её"; got != want {
		t.Errorf("текст: %q, ждали %q", got, want)
	}
}
