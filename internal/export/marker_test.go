package export

import (
	"bytes"
	"context"
	"image/color"
	"os"
	"path/filepath"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/imgfile"
	"github.com/IliaSA101/sosninaalbums/internal/render"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Метка выгрузки (121): каждый файл программы — разворот, лист страницы и обложка — несёт её так,
// что и сканер ленты (imgfile.Header), и перезапись (IsOurs) узнают его; обычное фото — нет.
func TestMarkerInHeader(t *testing.T) {
	// Текст метки — формат файла: выгрузки прежних версий несут именно его.
	for _, m := range []string{Marker, imgfile.ExportMarker, testkit.ExportMarker} {
		if m != "Sosnina Albums" {
			t.Fatalf("метка %q, ждали Sosnina Albums", m)
		}
	}
	white := [2]color.RGBA{{255, 255, 255, 255}, {255, 255, 255, 255}}
	spreads := []render.Spread{{Bg: white}, {Bg: white}, {Bg: white}}
	opt := render.Options{PageW: 40, PageH: 30, DPI: 50, Quality: 90}
	dir := filepath.Join(testkit.TempDir(t), "out")
	jobs := []Job{{Index: 0, Whole: true}, {Index: 1, Whole: true, Left: true, Right: true}, {Index: 2, Right: true, Prefix: "1-"}}
	if err := RunJobs(context.Background(), spreads, jobs, opt, dir, func(int, int) {}); err != nil {
		t.Fatal(err)
	}
	cover := &CoverJob{Spread: spreads[0], Opt: opt}
	if err := runCover(context.Background(), cover, dir); err != nil {
		t.Fatal(err)
	}
	names := []string{"01.jpg", "02.jpg", "02-1.jpg", "02-2.jpg", "1-03-2.jpg", CoverName}
	for _, n := range names {
		p := filepath.Join(dir, n)
		f, err := os.Open(p)
		if err != nil {
			t.Fatal(err)
		}
		h := imgfile.Header(f)
		_ = f.Close()
		if !h.Ours || !IsOurs(p) {
			t.Errorf("%s: Header.Ours = %v, IsOurs = %v, ждали оба true", n, h.Ours, IsOurs(p))
		}
	}

	// Обычное фото метки не несёт, ни тем, ни другим способом.
	photo := testkit.JPEG(t, dir, "IMG_1.jpg", 80, 60)
	f, err := os.Open(photo)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	if imgfile.Header(f).Ours || IsOurs(photo) {
		t.Error("обычное фото принято за выгрузку")
	}

	// Помощник testkit.ExportJPEG: JFIF и COM на тех же местах и той же длины, что у файла Run.
	fake := testkit.ExportJPEG(t, dir, "fake.jpg", 80, 60)
	got, _ := os.ReadFile(fake)
	want, _ := os.ReadFile(filepath.Join(dir, "01.jpg"))
	const comEnd = 2 + 18 + 16 // SOI, APP0, COM
	if len(got) < comEnd || !bytes.Equal(got[:6], want[:6]) || !bytes.Equal(got[20:comEnd], want[20:comEnd]) {
		t.Errorf("шапка testkit.ExportJPEG % x, у выгрузки % x", got[:min(len(got), comEnd)], want[:comEnd])
	}
	if !IsOurs(fake) {
		t.Error("ExportJPEG не узнан как выгрузка")
	}
}
