package library

import (
	"bytes"
	"errors"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/imgfile"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// TestMarkupFile — PSD: размер, DPI и направляющие обеих осей без чтения картинки; JPEG и PNG —
// размер, картинка есть, направляющих и DPI нет; PSB (по расширению и по заголовку), чужое
// расширение, повреждённый PSD и UNC — ошибка.
func TestMarkupFile(t *testing.T) {
	dir := testkit.TempDir(t)
	p := testkit.PSD(t, dir, "spread.psd", testkit.PSDSpec{W: 60, H: 30, DPI: 300,
		Guides: []testkit.PSDGuide{{Pos: 23.5}, {Pos: 10, Horz: true}}})
	m, err := MarkupFile(p)
	if err != nil || !m.IsPSD() || m.W != 60 || m.H != 30 || m.DPI != 300 || !m.Image || m.Note != "" || len(m.Guides) != 2 {
		t.Fatalf("PSD: %v, %+v", err, m)
	}
	if g := m.Guides; g[0].Pos != 23.5 || g[0].Horz || g[1].Pos != 10 || !g[1].Horz {
		t.Fatalf("направляющие: %+v", g)
	}
	st, _ := os.Stat(p)
	if m.Size != st.Size() || m.Path != filepath.Clean(p) {
		t.Fatalf("размер %d из %d, путь %q", m.Size, st.Size(), m.Path)
	}
	flat := testkit.PSD(t, dir, "flat.psd", testkit.PSDSpec{W: 8, H: 4, NoMerged: true})
	if m, err := MarkupFile(flat); err != nil || m.Image || m.Note == "" || m.DPI != 0 || len(m.Guides) != 0 {
		t.Fatalf("PSD без сведённого: %v, %+v", err, m)
	}
	for _, f := range []string{testkit.JPEG(t, dir, "g.jpg", 40, 20), testkit.PNG(t, dir, "g.png", 40, 20, color.NRGBA{A: 255})} {
		if m, err := MarkupFile(f); err != nil || m.IsPSD() || m.W != 40 || m.H != 20 || !m.Image || m.DPI != 0 || m.Guides != nil {
			t.Fatalf("%s: %v, %+v", filepath.Base(f), err, m)
		}
	}
	psb := testkit.PSD(t, dir, "big.psb", testkit.PSDSpec{W: 8, H: 4, Version: 2})
	if _, err := MarkupFile(psb); !errors.Is(err, ErrPSB) {
		t.Fatalf("PSB по расширению: %v", err)
	}
	psbIn := testkit.PSD(t, dir, "big2.psd", testkit.PSDSpec{W: 8, H: 4, Version: 2})
	if _, err := MarkupFile(psbIn); !errors.Is(err, ErrPSB) {
		t.Fatalf("PSB под именем .psd: %v", err)
	}
	bad := filepath.Join(dir, "bad.psd")
	if err := os.WriteFile(bad, testkit.PSDBytes(testkit.PSDSpec{W: 8, H: 4})[:30], 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := MarkupFile(bad); err == nil {
		t.Fatal("обрезанный PSD принят")
	}
	tif := filepath.Join(dir, "g.tif")
	if err := os.WriteFile(tif, []byte("II*\x00"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := MarkupFile(tif); !errors.Is(err, errMarkupExt) || err.Error() != "файл разметки — только PSD, JPEG или PNG" {
		t.Fatalf("TIFF: %v", err)
	}
	if _, err := MarkupFile(`\\server\share\g.psd`); err == nil {
		t.Fatal("UNC принят")
	}
	if _, err := MarkupFile(filepath.Join(dir, "нет.psd")); err == nil {
		t.Fatal("несуществующий файл принят")
	}
}

// TestMarkupPreview — PSD: PNG того же размера и цвета (маленький — без уменьшения), бюджет
// декодирования возвращён; JPEG/PNG — nil без ошибки (их отдаёт файл); PSD без сведённого —
// ErrNoMarkupImage; большой — уменьшен до MarkupMaxSide по длинной стороне.
func TestMarkupPreview(t *testing.T) {
	dir := testkit.TempDir(t)
	c := color.NRGBA{R: 200, G: 100, B: 50, A: 255}
	p := testkit.PSD(t, dir, "a.psd", testkit.PSDSpec{W: 12, H: 6, C: c, DPI: 300})
	data, ct, err := MarkupPreview(p)
	if err != nil || ct != "image/png" {
		t.Fatalf("превью: %v %q", err, ct)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil || img.Bounds().Dx() != 12 || img.Bounds().Dy() != 6 {
		t.Fatalf("PNG: %v %v", err, img)
	}
	if got := color.NRGBAModel.Convert(img.At(5, 3)).(color.NRGBA); got != c {
		t.Fatalf("цвет %v, ждали %v", got, c)
	}
	if u := imgfile.DecodeBudget.Used(); u != 0 { // тесты пакета не параллельны
		t.Fatalf("бюджет не возвращён: %d", u)
	}
	if data, ct, err := MarkupPreview(testkit.PNG(t, dir, "g.png", 4, 4, c)); data != nil || ct != "" || err != nil {
		t.Fatalf("PNG разметка: %d байт %q %v", len(data), ct, err)
	}
	flat := testkit.PSD(t, dir, "flat.psd", testkit.PSDSpec{W: 8, H: 4, NoMerged: true})
	if _, _, err := MarkupPreview(flat); !errors.Is(err, ErrNoMarkupImage) {
		t.Fatalf("без сведённого: %v", err)
	}
	big := testkit.PSD(t, dir, "big.psd", testkit.PSDSpec{W: MarkupMaxSide*2 + 1, H: 3, C: c})
	data, _, err = MarkupPreview(big)
	if err != nil {
		t.Fatal(err)
	}
	if img, err = png.Decode(bytes.NewReader(data)); err != nil || img.Bounds().Dx() > MarkupMaxSide || img.Bounds().Dx() != previewSide(MarkupMaxSide*2+1) {
		t.Fatalf("уменьшение: %v %v", err, img.Bounds())
	}
	if _, _, err := MarkupPreview(filepath.Join(dir, "нет.psd")); err == nil {
		t.Fatal("несуществующий файл без ошибки")
	}
}

// previewSide — сторона превью при длинной стороне n (k = ceil(n / MarkupMaxSide)).
func previewSide(n int) int {
	k := (n + MarkupMaxSide - 1) / MarkupMaxSide
	return (n + k - 1) / k
}

// TestPreviewPixels — бюджет считается по размеру превью, а не документа.
func TestPreviewPixels(t *testing.T) {
	for _, c := range []struct {
		w, h int
		want int64
	}{
		{12, 6, 72}, {MarkupMaxSide, MarkupMaxSide, MarkupMaxSide * MarkupMaxSide},
		{MarkupMaxSide + 1, 2, int64(previewSide(MarkupMaxSide+1)) * 1}, {30000, 30000, 3750 * 3750},
	} {
		if got := previewPixels(c.w, c.h); got != c.want {
			t.Errorf("%d×%d: %d, ждали %d", c.w, c.h, got, c.want)
		}
	}
}

// TestMarkupFileMod — Mod берётся из времени записи файла (часть отпечатка GuideView.Key) для
// PSD и PNG; новое время — другое значение, тот же файл — то же.
func TestMarkupFileMod(t *testing.T) {
	dir := testkit.TempDir(t)
	for _, p := range []string{
		testkit.PSD(t, dir, "s.psd", testkit.PSDSpec{W: 8, H: 4}),
		testkit.PNG(t, dir, "s.png", 8, 4, color.NRGBA{A: 255}),
	} {
		a, err := MarkupFile(p)
		if err != nil || a.Mod == 0 {
			t.Fatalf("%s: %v, Mod %d", filepath.Base(p), err, a.Mod)
		}
		if b, _ := MarkupFile(p); b.Mod != a.Mod {
			t.Errorf("%s: Mod нестабилен", filepath.Base(p))
		}
		later := time.Unix(0, a.Mod).Add(time.Hour)
		if err := os.Chtimes(p, later, later); err != nil {
			t.Fatal(err)
		}
		if b, _ := MarkupFile(p); b.Mod != later.UnixNano() {
			t.Errorf("%s: Mod %d, ждали %d", filepath.Base(p), b.Mod, later.UnixNano())
		}
	}
}
