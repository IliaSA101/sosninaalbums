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

func TestHugeJPEGHeader(t *testing.T) {
	p := HugeJPEG(t, TempDir(t), "h.jpg", 20000, 20000)
	f, _ := os.Open(p)
	defer f.Close()
	cfg, _, err := image.DecodeConfig(f)
	if err != nil || cfg.Width != 20000 || cfg.Height != 20000 {
		t.Fatalf("cfg=%v err=%v", cfg, err)
	}
}
