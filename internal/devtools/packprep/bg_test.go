package packprep

import (
	"bytes"
	"image"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/devtools/devtest"
)

func bgItem(src, out string) Item { return Item{Src: src, Mode: ModeBg, Out: out, Name: "Фон"} }

func jpegSize(t testing.TB, data []byte) (int, int) {
	t.Helper()
	cfg, err := jpeg.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	return cfg.Width, cfg.Height
}

// Страница 2048 → 3602: ×1,76, флага нет; 1024 → ×3,52, флаг; размер и JPEG q92.
func TestBgUpscale(t *testing.T) {
	src := tempDir(t)
	writeJPEG(t, filepath.Join(src, "bg/page.jpg"), texture(2048, 2048))
	writeJPEG(t, filepath.Join(src, "bg/small.jpg"), texture(1024, 1024))
	items := []Item{bgItem("bg/page.jpg", "bg/page.jpg"), bgItem("bg/small.jpg", "bg/small.jpg")}
	if devtest.Lite() {
		items = items[1:] // быстрый уровень: один Lanczos до 3602 px (без флага — в полном)
	}
	rep, out := run(t, src, baseRecipe(items...))
	if !rep.OK {
		t.Fatal(rep.Errors)
	}
	if devtest.Lite() {
		small := byFile(t, rep, "bg/small.jpg")
		if small.W != 3602 || small.H != 3602 || small.Upscale == nil || small.Upscale.Factor < 3.5 || !hasFlag(small, "апскейл ×3.52 от 1024 px") {
			t.Fatalf("×3,5 должен дать флаг: %dx%d %+v %v", small.W, small.H, small.Upscale, small.Flags)
		}
		if w, h := jpegSize(t, readOut(t, out, "bg/small.jpg")); w != 3602 || h != 3602 {
			t.Fatalf("файл %d×%d", w, h)
		}
		return
	}
	page, small := byFile(t, rep, "bg/page.jpg"), byFile(t, rep, "bg/small.jpg")
	if page.W != 3602 || page.H != 3602 {
		t.Fatalf("страница %d×%d, ждали 3602×3602", page.W, page.H)
	}
	if w, h := jpegSize(t, readOut(t, out, "bg/page.jpg")); w != 3602 || h != 3602 {
		t.Fatalf("файл %d×%d", w, h)
	}
	if page.Upscale == nil || page.Upscale.From != 2048 || page.Upscale.Factor < 1.75 || page.Upscale.Factor > 1.77 {
		t.Fatalf("увеличение страницы: %+v", page.Upscale)
	}
	if hasFlag(page, "апскейл") {
		t.Fatalf("×1,76 — не флаг: %v", page.Flags)
	}
	if small.Upscale == nil || small.Upscale.Factor < 3.5 || !hasFlag(small, "апскейл ×3.52 от 1024 px") {
		t.Fatalf("×3,5 должен дать флаг: %+v %v", small.Upscale, small.Flags)
	}
}

// Разворот 2880×1472 → 2:1 (срез по центру) → 7205×3602, ×2,5 — флаг.
func TestBgSpread2to1(t *testing.T) {
	if devtest.Lite() {
		t.Skip("разворот 7205×3602 — Lanczos на 26 Мп: полный уровень")
	}
	src := tempDir(t)
	writeJPEG(t, filepath.Join(src, "bg/spread.jpg"), texture(2880, 1472))
	rep, out := run(t, src, baseRecipe(bgItem("bg/spread.jpg", "bg/spread.jpg")))
	if !rep.OK {
		t.Fatal(rep.Errors)
	}
	o := byFile(t, rep, "bg/spread.jpg")
	if o.W != 7205 || o.H != 3602 {
		t.Fatalf("разворот %d×%d, ждали 7205×3602", o.W, o.H)
	}
	if w, h := jpegSize(t, readOut(t, out, "bg/spread.jpg")); w != 7205 || h != 3602 {
		t.Fatalf("файл %d×%d", w, h)
	}
	if o.Upscale == nil || o.Upscale.From != 2880 || o.Upscale.Factor < 2.49 || o.Upscale.Factor > 2.51 {
		t.Fatalf("увеличение разворота: %+v", o.Upscale)
	}
	if !hasFlag(o, "апскейл ×2.50 от 2880 px") || !hasFlag(o, "обрезан до 2:1: 2880×1472 → 2880×1440") {
		t.Fatalf("флаги разворота: %v", o.Flags)
	}
}

// Файл нужного размера — байт в байт; другой формат выхода — перекодирование без увеличения.
func TestBgCopyBytes(t *testing.T) {
	big := canvas(3700, 3650, navy) // плоский: быстро кодируется
	fillRect(big, image.Rect(100, 100, 900, 700), coral)
	if devtest.Lite() {
		// Быстрый уровень: без кодека JPEG 3700×3650 — makeBg получает «файл» как есть и картинку.
		orig := []byte("исходные байты файла")
		res, err := makeBg(orig, ".jpg", big, "bg/ready.jpg", 0)
		if err != nil || !res.Copied || !bytes.Equal(res.Data, orig) || res.Factor != 1 {
			t.Fatalf("копия байт в байт: %v %+v", err, res)
		}
		return
	}
	src := tempDir(t)
	writeJPEG(t, filepath.Join(src, "bg/ready.jpg"), big)
	orig, err := os.ReadFile(filepath.Join(src, "bg/ready.jpg"))
	if err != nil {
		t.Fatal(err)
	}
	rep, out := run(t, src, baseRecipe(bgItem("bg/ready.jpg", "bg/ready.jpg"), bgItem("bg/ready.jpg", "bg/ready.png")))
	if !rep.OK {
		t.Fatal(rep.Errors)
	}
	if !bytes.Equal(readOut(t, out, "bg/ready.jpg"), orig) {
		t.Fatal("готовый фон изменён")
	}
	if o := byFile(t, rep, "bg/ready.jpg"); o.Upscale != nil || !hasFlag(o, "байт в байт") {
		t.Fatalf("копия: %+v %v", o.Upscale, o.Flags)
	}
	pngBg := byFile(t, rep, "bg/ready.png")
	if pngBg.W != 3700 || pngBg.H != 3650 || pngBg.Upscale != nil {
		t.Fatalf("PNG из JPEG без увеличения: %dx%d", pngBg.W, pngBg.H)
	}
}

// trim срезает края до увеличения: ширина источника уменьшается, ход без среза по краям.
func TestBgTrim(t *testing.T) {
	if devtest.Lite() {
		t.Skip("Lanczos до 3602 px — полный уровень (в быстром идёт TestBgUpscale)")
	}
	src := tempDir(t)
	writeJPEG(t, filepath.Join(src, "bg/t.jpg"), texture(2000, 2000))
	it := bgItem("bg/t.jpg", "bg/t.jpg")
	it.Trim = 5
	rep, _ := run(t, src, baseRecipe(it))
	o := byFile(t, rep, "bg/t.jpg")
	if o.Upscale == nil || o.Upscale.From != 1800 {
		t.Fatalf("после среза 5 %% от 2000 остаётся 1800: %+v", o.Upscale)
	}
	if o.W != 3602 {
		t.Fatalf("ширина %d", o.W)
	}
}

// Прозрачность фона подкладывается белым с замечанием; не квадрат — короткая сторона 3602.
func TestBgAlphaAndRect(t *testing.T) {
	if !devtest.Lite() { // быстрый уровень: Lanczos до 4803×3602 пропущен, ошибка trim ниже идёт всегда
		img := canvas(2400, 1800, white)
		img.SetNRGBA(5, 5, transp)
		res, err := makeBg(nil, ".png", img, "bg/x.jpg", 0)
		if err != nil {
			t.Fatal(err)
		}
		if res.Img.Bounds() != image.Rect(0, 0, 4803, 3602) {
			t.Fatalf("размер %v", res.Img.Bounds())
		}
		if len(res.Notes) != 1 {
			t.Fatalf("замечания: %v", res.Notes)
		}
	}
	if _, err := makeBg(nil, ".png", canvas(10, 10, white), "bg/x.jpg", 50); err == nil {
		t.Fatal("trim съел картинку — ждали ошибку")
	}
}
