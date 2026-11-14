package testkit

import (
	"image"
	_ "image/jpeg"
	"os"
	"testing"
)

func TestJPEGSize(t *testing.T) {
	p := JPEG(t, TempDir(t), "a.jpg", 30, 20)
	f, _ := os.Open(p)
	defer f.Close()
	cfg, _, err := image.DecodeConfig(f)
	if err != nil || cfg.Width != 30 || cfg.Height != 20 {
		t.Fatalf("cfg=%v err=%v", cfg, err)
	}
}

// Lite и Steps следуют -short и -race; нулевые и единичные значения не ломают выбор.
func TestLiteSteps(t *testing.T) {
	lite := testing.Short() || Race
	if Lite() != lite {
		t.Fatalf("Lite()=%v, ждали %v", Lite(), lite)
	}
	for _, c := range [][2]int{{1500, 500}, {10, 0}, {0, 0}, {1, 1}} {
		want := c[0]
		if testing.Short() {
			want = c[1]
		}
		if got := Steps(c[0], c[1]); got != want {
			t.Errorf("Steps(%d, %d)=%d, ждали %d", c[0], c[1], got, want)
		}
	}
}

func TestHugeJPEGHeader(t *testing.T) {
	p := HugeJPEG(t, TempDir(t), "h.jpg", 20000, 20000)
	f, _ := os.Open(p)
	defer f.Close()
	cfg, _, err := image.DecodeConfig(f)
	if err != nil || cfg.Width != 20000 || cfg.Height != 20000 {
		t.Fatalf("cfg=%v err=%v", cfg, err)
	}
}
