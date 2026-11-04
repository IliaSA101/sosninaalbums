package app

// Цена двух нормализаций тихого открытия (design §2, «Стоимость»): DecodeDry + DecodeDisk на альбоме в 200 разворотов.
// Порог 300 мс — сказать оркестратору, если выше.

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

func TestOpenQuietDecodeCost(t *testing.T) {
	if testing.Short() {
		t.Skip("замер: не для -short")
	}
	r := newOQRig(t)
	dir := filepath.Join(r.place, "Большой")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	src := testkit.JPEG(t, dir, "src.jpg", 60, 40)
	b, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for i := range 1200 {
		p := filepath.Join(dir, fmt.Sprintf("img-%04d.jpg", i))
		if err := os.WriteFile(p, b, 0o644); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, library.IDFor(p))
	}
	if _, err := r.a.AddFolderPath(dir); err != nil {
		t.Fatal(err)
	}
	waitRead(t, r.a)
	if _, err := r.a.AutoBuild(ids, -1, model.AutoOpts{Spreads: 200, MaxPer: 6, MinPer: 6, Variety: model.VarietyNormal}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(r.dir, "Большой"+projectExt)
	if err := r.a.editor().Save(path); err != nil {
		t.Fatal(err)
	}
	data, err := salbum.ReadLimited(path)
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	dry, err := salbum.DecodeDry(data, r.dir)
	if err != nil {
		t.Fatal(err)
	}
	dryT := time.Since(start)
	if _, err := salbum.DecodeDisk(data, r.dir, salbum.Bounded(func(string) bool { return true }, nil)); err != nil {
		t.Fatal(err)
	}
	total := time.Since(start)
	t.Logf("разворотов %d, фото %d, файл %d КБ: DecodeDry %v, обе нормализации %v", len(dry.Spreads), len(dry.Photos), len(data)>>10, dryT, total)
	if total > testkit.Limit(300*time.Millisecond) {
		t.Errorf("две нормализации заняли %v (> 300 мс): сказать оркестратору", total)
	}
}
