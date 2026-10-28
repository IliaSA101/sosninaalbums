package library

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/imgfile"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Превью больших JPEG/PNG разметки (086 G2): картинку строит Go.

// checkFit — длинная сторона превью равна MarkupMaxSide, короткая — пропорционально (±1 px),
// ориентация (портрет/альбом) та же, что у w×h.
func checkFit(t *testing.T, b image.Rectangle, w, h int) {
	t.Helper()
	long, short := max(w, h), min(w, h)
	wantShort := short * MarkupMaxSide / long
	gotLong, gotShort := max(b.Dx(), b.Dy()), min(b.Dx(), b.Dy())
	if gotLong != MarkupMaxSide || gotShort < wantShort-1 || gotShort > wantShort+1 || (w >= h) != (b.Dx() >= b.Dy()) {
		t.Fatalf("превью %v из %d×%d, ждали длинную %d и короткую %d±1", b, w, h, MarkupMaxSide, wantShort)
	}
}

// TestMarkupPreviewLargeJPEG — JPEG длиннее MarkupMaxSide: уменьшенный JPEG с теми же
// пропорциями (и альбомный, и портретный лист), Big() истинно, бюджет декодирования возвращён.
func TestMarkupPreviewLargeJPEG(t *testing.T) {
	dir := testkit.TempDir(t)
	for _, c := range []struct {
		name string
		w, h int
	}{{"big.jpg", 5000, 3000}, {"tall.jpg", 3000, 5000}} {
		p := testkit.JPEG(t, dir, c.name, c.w, c.h)
		m, err := MarkupFile(p)
		if err != nil || !m.Big() || m.IsPSD() {
			t.Fatalf("%s MarkupFile: %v %+v", c.name, err, m)
		}
		data, ct, err := MarkupPreview(p)
		if err != nil || ct != "image/jpeg" {
			t.Fatalf("%s превью: %v %q", c.name, err, ct)
		}
		img, err := jpeg.Decode(bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		checkFit(t, img.Bounds(), c.w, c.h)
		if u := imgfile.DecodeBudget.Used(); u != 0 { // тесты пакета не параллельны
			t.Fatalf("%s: бюджет не возвращён: %d", c.name, u)
		}
	}
}

// TestMarkupPreviewLargePNGAlpha — прозрачный PNG остаётся PNG с альфой; непрозрачный PNG
// (даже с альфа-каналом) — JPEG.
func TestMarkupPreviewLargePNGAlpha(t *testing.T) {
	dir := testkit.TempDir(t)
	c := color.NRGBA{R: 200, G: 30, B: 40, A: 255}
	p := testkit.PNG(t, dir, "big.png", 5000, 2000, c) // правая половина прозрачна
	data, ct, err := MarkupPreview(p)
	if err != nil || ct != "image/png" {
		t.Fatalf("превью: %v %q", err, ct)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	checkFit(t, img.Bounds(), 5000, 2000)
	b := img.Bounds()
	if _, _, _, a := img.At(b.Dx()-5, b.Dy()/2).RGBA(); a != 0 {
		t.Fatalf("прозрачность потеряна: альфа %d", a)
	}
	if got := color.NRGBAModel.Convert(img.At(5, b.Dy()/2)).(color.NRGBA); got != c {
		t.Fatalf("цвет %v, ждали %v", got, c)
	}
	op := image.NewNRGBA(image.Rect(0, 0, 4500, 100))
	for i := 3; i < len(op.Pix); i += 4 {
		op.Pix[i] = 255
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, op); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "opaque.png")
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, ct, err := MarkupPreview(path); err != nil || ct != "image/jpeg" {
		t.Fatalf("непрозрачный PNG: %v %q", err, ct)
	}
}

// TestMarkupPreviewSmall — JPEG/PNG до MarkupMaxSide включительно: nil (отдаётся файлом),
// Big() ложно; на 1 px больше — уже превью.
func TestMarkupPreviewSmall(t *testing.T) {
	dir := testkit.TempDir(t)
	for _, p := range []string{
		testkit.JPEG(t, dir, "s.jpg", 3000, 2000),
		testkit.JPEG(t, dir, "edge.jpg", MarkupMaxSide, 8),
		testkit.PNG(t, dir, "s.png", 100, 100, color.NRGBA{A: 255}),
	} {
		m, err := MarkupFile(p)
		if err != nil || m.Big() {
			t.Fatalf("%s: Big %v %v", filepath.Base(p), m.Big(), err)
		}
		if data, ct, err := MarkupPreview(p); data != nil || ct != "" || err != nil {
			t.Fatalf("%s: %d байт %q %v", filepath.Base(p), len(data), ct, err)
		}
	}
	if data, _, err := MarkupPreview(testkit.JPEG(t, dir, "over.jpg", MarkupMaxSide+1, 8)); err != nil || len(data) == 0 {
		t.Fatalf("на 1 px больше предела: %d байт %v", len(data), err)
	}
}

// TestMarkupPreviewOriented — EXIF Orientation 6: файл 5000×3000 лежит на боку, превью —
// портрет; Big() считается по размеру после поворота (4096×3000 с поворотом — не большой).
func TestMarkupPreviewOriented(t *testing.T) {
	dir := testkit.TempDir(t)
	p := testkit.JPEGOriented(t, dir, "o.jpg", 5000, 3000, 6)
	data, ct, err := MarkupPreview(p)
	if err != nil || ct != "image/jpeg" {
		t.Fatalf("превью: %v %q", err, ct)
	}
	img, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	checkFit(t, img.Bounds(), 3000, 5000)
	if b := img.Bounds(); b.Dy() <= b.Dx() {
		t.Fatalf("не портрет: %v", b)
	}
	if m, err := MarkupFile(testkit.JPEGOriented(t, dir, "o2.jpg", 4096, 3000, 6)); err != nil || m.Big() {
		t.Fatalf("4096×3000 с поворотом: Big %v %v", m.Big(), err)
	}
}

// TestMarkupPreviewBroken — обрезанный «большой» файл и заголовок на 900 Мп: ошибка, не паника;
// бюджет декодирования возвращён.
func TestMarkupPreviewBroken(t *testing.T) {
	dir := testkit.TempDir(t)
	p := testkit.JPEG(t, dir, "cut.jpg", 5000, 3000)
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, data[:len(data)/3], 0o600); err != nil {
		t.Fatal(err)
	}
	if out, _, err := MarkupPreview(p); err == nil && len(out) == 0 {
		t.Fatal("обрезанный файл: пустой ответ без ошибки")
	}
	huge := testkit.HugeJPEG(t, dir, "huge.jpg", 30000, 30000)
	if _, _, err := MarkupPreview(huge); err == nil {
		t.Fatal("900 Мп принят")
	}
	if u := imgfile.DecodeBudget.Used(); u != 0 {
		t.Fatalf("бюджет не возвращён: %d", u)
	}
}

// TestMarkupPreview16BitBudget — 16-битная разметка (NRGBA64, 8 байт на пиксель) берёт в бюджете
// вдвое больше W×H: при свободных ровно W×H 8-битная того же размера строится, а 16-битная ждёт
// до отмены ctx (бюджет не тронут); бюджет свободен — строится и она.
func TestMarkupPreview16BitBudget(t *testing.T) {
	dir := testkit.TempDir(t)
	const w, h = MarkupMaxSide + 4, 8
	p8 := testkit.PNG(t, dir, "8.png", w, h, color.NRGBA{R: 9, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewNRGBA64(image.Rect(0, 0, w, h))); err != nil {
		t.Fatal(err)
	}
	p16 := filepath.Join(dir, "16.png")
	if err := os.WriteFile(p16, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	busy := imgfile.DecodeBudgetPixels - w*h
	imgfile.DecodeBudget.Acquire(busy)
	held := true
	t.Cleanup(func() {
		if held {
			imgfile.DecodeBudget.Release(busy)
		}
	})
	if _, _, err := MarkupPreviewCtx(context.Background(), p8); err != nil {
		t.Fatalf("8 бит при свободных W×H: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if _, _, err := MarkupPreviewCtx(ctx, p16); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("16 бит при свободных W×H: ждали ожидание до отмены, получено %v", err)
	}
	if u := imgfile.DecodeBudget.Used(); u != busy {
		t.Fatalf("бюджет %d, ждали %d", u, busy)
	}
	imgfile.DecodeBudget.Release(busy)
	held = false
	if data, ct, err := MarkupPreview(p16); err != nil || len(data) == 0 || ct != "image/png" {
		t.Fatalf("16 бит при свободном бюджете: %v %q", err, ct)
	}
	if u := imgfile.DecodeBudget.Used(); u != 0 {
		t.Fatalf("бюджет не возвращён: %d", u)
	}
}

// Кодирование превью идёт уже без бюджета декода (оригинал отпущен после уменьшения), а после
// возврата бюджет не уходит в минус от двойного release; прозрачность определяется по оригиналу.
func TestMarkupPreviewReleasesBeforeEncode(t *testing.T) {
	dir := testkit.TempDir(t)
	old := beforeEncode
	t.Cleanup(func() { beforeEncode = old })
	for _, c := range []struct {
		name, ctype string
		path        string
	}{
		{"jpeg", "image/jpeg", testkit.JPEG(t, dir, "o.jpg", MarkupMaxSide+10, 20)},
		{"png с прозрачностью", "image/png", testkit.PNG(t, dir, "a.png", MarkupMaxSide+10, 20, color.NRGBA{R: 9, A: 100})},
	} {
		used, calls := int64(-1), 0
		beforeEncode = func() { used, calls = imgfile.DecodeBudget.Used(), calls+1 }
		_, ct, err := MarkupPreview(c.path)
		if err != nil || ct != c.ctype || calls != 1 {
			t.Fatalf("%s: %v %q, вызовов %d", c.name, err, ct, calls)
		}
		if used != 0 {
			t.Fatalf("%s: при кодировании бюджет занят: %d", c.name, used)
		}
		if u := imgfile.DecodeBudget.Used(); u != 0 {
			t.Fatalf("%s: бюджет после: %d", c.name, u)
		}
	}
}
