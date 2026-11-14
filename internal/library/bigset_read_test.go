package library

import (
	"image"
	"os"
	"path/filepath"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/imgfile"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Сегменты набора e2e (testkit.BigSetSegments) читает сама программа: дата съёмки, ориентация
// (стороны меняются местами) и Adobe RGB у кадров с разными флагами. 1023 = 3·11·31 — и Adobe,
// и поворот сразу. Тест здесь, а не в testkit: depguard не пускает testkit в internal.
func TestBigSetSegmentsRead(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 90, 60))
	for i := 0; i < len(src.Pix); i += 4 {
		src.Pix[i], src.Pix[i+1], src.Pix[i+2], src.Pix[i+3] = 90, 120, 30, 255
	}
	dir := testkit.TempDir(t)
	tile := testkit.BigSetTile(1)
	for _, i := range []int{1, 4, 20, 31, 33, 600, 1023} {
		p := testkit.BigSetPlan(i, 1)
		data, err := testkit.BigSetEncode(testkit.BigSetRender(src, p, tile, 100), p)
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(dir, "x.jpg")
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
		ph, err := ReadInfo(path)
		if err != nil {
			t.Fatalf("кадр %d: %v", i, err)
		}
		wantW, wantH := max(1, p.W/100), max(1, p.H/100)
		if ph.W != wantW || ph.H != wantH {
			t.Errorf("кадр %d: %d×%d, ждали %d×%d", i, ph.W, ph.H, wantW, wantH)
		}
		if want := map[bool]int{false: 1, true: 6}[p.Rotate]; ph.Rotate != want {
			t.Errorf("кадр %d: Orientation %d, ждали %d", i, ph.Rotate, want)
		}
		if ph.Taken != p.Taken.Unix() {
			t.Errorf("кадр %d: дата %d, ждали %d (%s)", i, ph.Taken, p.Taken.Unix(), p.Orig())
		}
		if (ph.Color == imgfile.ColorAdobe) != p.Adobe {
			t.Errorf("кадр %d: цвет %q при Adobe=%v", i, ph.Color, p.Adobe)
		}
	}
}
