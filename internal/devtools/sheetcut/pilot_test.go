package sheetcut

import (
	"math"
	"os"
	"path/filepath"
	"testing"
)

// pilotFrames — рамки пилота (23) и их «тянется, %» по замеру 25.10 (после обрезки, порог по листу,
// holes clean); open — окно у края, срез по третям. Допуск ±2 пункта: JPEG и округление среза.
var pilotFrames = []struct {
	name    string
	stretch float64
	open    bool
}{
	{"sf-leaves-corners-2-flux.jpg", 91, false},
	{"sf-leaves-corners-flux.jpg", 79, false},
	{"sf-leaves-diagonal-mystic.png", 79, false},
	{"sf-leaves-four-mystic.png", 75, false},
	{"sf-pencils-bright-flux.jpg", 17, false},
	{"sf-pencils-double-flux.jpg", 1, false},
	{"sf-supplies-diagonal-mystic.png", 77, false},
	{"sf-supplies-neat-flux.jpg", 0, true},
	{"sf-supplies-rich-flux.jpg", 93, false},
	{"kg-fr-ball-mystic-1.png", 54, false},
	{"kg-fr-ball-mystic-8.png", 88, false},
	{"kg-fr-stripes-flux-1.jpg", 0, true},
	{"kg-fr-stripes-mystic-5.png", 31, false},
	{"kg-fr-toys-abc-flux-1.jpg", 68, false},
	{"kg-fr-toys-abc-mystic-10.png", 75, false},
	{"kg-fr-toys-abc-mystic-11.png", 73, false},
	{"kg-fr-toys2-flux-1.jpg", 82, false},
	{"kg-fr-toys2-flux-2.jpg", 60, false},
	{"kg-fr-toys2-mystic-1.png", 73, false},
	{"ns-frame-diagonal-double-mystic.png", 77, false},
	{"ns-frame-diagonal-lush-mystic.png", 93, false},
	{"ns-frame-double-silver-flux.jpg", 88, false},
	{"ns-frame-four-corners-mystic.png", 95, false},
}

// TestPilotFramesRegression гоняет все рамки пилота: окно находится у всех 23, «тянется» держится
// около замера. Исходники вне git (docs/private/library-src/pilot), путь — в SHEETCUT_PILOT; без
// переменной тест пропускается (н/п на чистом клоне и в CI).
func TestPilotFramesRegression(t *testing.T) {
	root := os.Getenv("SHEETCUT_PILOT")
	if root == "" {
		t.Skip("SHEETCUT_PILOT не задан: исходники пилота вне git")
	}
	var files []string
	for _, g := range []string{"school-frames/*", "kindergarten-common/kg-fr-*", "wedding-navy-silver/frames/*"} {
		m, err := filepath.Glob(filepath.Join(root, g))
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, m...)
	}
	if len(files) != len(pilotFrames) {
		t.Fatalf("рамок %d, в таблице %d", len(files), len(pilotFrames))
	}
	for i, f := range files {
		fh, err := os.Open(f)
		if err != nil {
			t.Fatal(err)
		}
		st, err := fh.Stat()
		if err != nil {
			t.Fatal(err)
		}
		img, err := DecodeSheet(fh, st.Size())
		_ = fh.Close()
		if err != nil {
			t.Fatal(err)
		}
		o := DefaultOptions()
		o.Mode = ModeFrame
		fi := mustCut(t, img, o).Stickers[0].Frame
		want := pilotFrames[i]
		if filepath.Base(f) != want.name {
			t.Fatalf("порядок: %s, в таблице %s", filepath.Base(f), want.name)
		}
		if !fi.Window || fi.Open != want.open || math.Abs(fi.Stretch-want.stretch) > 2 {
			t.Errorf("%s: окно %v, открытая %v, тянется %.1f; ждали открытая %v, тянется %.0f ± 2", want.name, fi.Window, fi.Open, fi.Stretch, want.open, want.stretch)
		}
	}
}
