package library

import (
	"image"
	"image/color"
	"image/gif"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/imgfile"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

func rat(n, d uint32) [2]uint32 { return [2]uint32{n, d} }

// Строки параметров: выдержка дробью и секундами, диафрагма, фокусное, ISO, дата; выпавшие
// параметры пропускаются; мусор вне пределов не показывается.
func TestAboutShotLines(t *testing.T) {
	at := time.Date(2026, 6, 12, 14, 32, 9, 0, time.UTC).Unix()
	cases := []struct {
		name string
		s    imgfile.Shot
		want []string
	}{
		{"полный набор", imgfile.Shot{
			Make: "Canon", Model: "Canon EOS R6", Lens: "RF24-70mm F2.8 L IS USM",
			Exposure: rat(1, 200), FNumber: rat(28, 10), Focal: rat(50, 1), ISO: 400, Taken: at,
		}, []string{"Canon EOS R6 · RF24-70mm F2.8 L IS USM", "50 мм · f/2,8 · 1/200 с · ISO 400", "Снято 12.06.2026, 14:32"}},
		{"полсекунды", imgfile.Shot{Exposure: rat(1, 2)}, []string{"0,5 с"}},
		{"две секунды", imgfile.Shot{Exposure: rat(2, 1)}, []string{"2 с"}},
		{"1,3 с", imgfile.Shot{Exposure: rat(13, 10)}, []string{"1,3 с"}},
		{"числитель не 1", imgfile.Shot{Exposure: rat(10, 6000)}, []string{"1/600 с"}},
		{"длинная выдержка", imgfile.Shot{Exposure: rat(30, 1)}, []string{"30 с"}},
		{"f/11", imgfile.Shot{FNumber: rat(11, 1)}, []string{"f/11"}},
		{"f/1,4", imgfile.Shot{FNumber: rat(14, 10)}, []string{"f/1,4"}},
		{"дробное фокусное", imgfile.Shot{Focal: rat(43, 10)}, []string{"4,3 мм"}},
		{"фокусное 49,96 → 50", imgfile.Shot{Focal: rat(4996, 100)}, []string{"50 мм"}},
		{"ISO 100", imgfile.Shot{ISO: 100}, []string{"ISO 100"}},
		{"без ISO", imgfile.Shot{FNumber: rat(18, 10), Exposure: rat(1, 60)}, []string{"f/1,8 · 1/60 с"}},
		{"только дата", imgfile.Shot{Taken: at}, []string{"Снято 12.06.2026, 14:32"}},
		{"полночь не сдвигается поясом", imgfile.Shot{Taken: time.Date(2026, 1, 2, 0, 5, 0, 0, time.UTC).Unix()}, []string{"Снято 02.01.2026, 00:05"}},
		{"только объектив", imgfile.Shot{Lens: "35mm F1.4"}, []string{"35mm F1.4"}},
		{"нули и мусор", imgfile.Shot{Exposure: rat(0, 5), FNumber: rat(1, 0), Focal: rat(4_000_000_000, 1), ISO: -3}, nil},
		{"пусто", imgfile.Shot{}, nil},
	}
	withLocal(t, time.FixedZone("UTC+10", 10*3600)) // местный пояс не влияет на «стенное» время камеры
	for _, c := range cases {
		if got := ShotLines(c.s); !slices.Equal(got, c.want) {
			t.Errorf("%s: %q, ждали %q", c.name, got, c.want)
		}
	}
}

// Камера: производитель не повторяется в модели.
func TestAboutMakeModel(t *testing.T) {
	cases := []struct{ mk, model, lens, want string }{
		{"Canon", "Canon EOS R6", "", "Canon EOS R6"},
		{"NIKON CORPORATION", "NIKON Z 6_2", "", "NIKON Z 6_2"},
		{"SONY", "ILCE-7M3", "", "SONY ILCE-7M3"},
		{"Canon", "", "", "Canon"},
		{"", "EOS R6", "", "EOS R6"},
		{"canon", "CANON EOS R6", "", "CANON EOS R6"},
		{"  Canon ", " Canon EOS R6", "", "Canon EOS R6"},
		{"Canon", "Canon EOS R6", "RF50mm F1.2", "Canon EOS R6 · RF50mm F1.2"},
		{"", "", "RF50mm F1.2", "RF50mm F1.2"},
		{"", "", "", ""},
	}
	for _, c := range cases {
		got := cameraLine(imgfile.Shot{Make: c.mk, Model: c.model, Lens: c.lens})
		if got != c.want {
			t.Errorf("%q+%q+%q: %q, ждали %q", c.mk, c.model, c.lens, got, c.want)
		}
	}
}

func TestAboutFileLine(t *testing.T) {
	cases := []struct {
		w, h int
		size int64
		want string
	}{
		{6000, 4000, 12_897_485, "6000 × 4000 px · 24 Мп · 12,3 МБ"},
		{6000, 4000, 850 << 10, "6000 × 4000 px · 24 Мп · 850 КБ"},
		{3000, 2000, 5 << 20, "3000 × 2000 px · 6 Мп · 5 МБ"},
		{3264, 2448, 2 << 20, "3264 × 2448 px · 8 Мп · 2 МБ"},
		{4000, 2500, 1 << 20, "4000 × 2500 px · 10 Мп · 1 МБ"},
		{100, 100, 700, "100 × 100 px · <0,1 Мп · 700 Б"},
		{100, 100, 1048500, "100 × 100 px · <0,1 Мп · 1 МБ"}, // не «1024 КБ»
		{100, 100, 1048000, "100 × 100 px · <0,1 Мп · 1023 КБ"},
		{800, 600, 0, "800 × 600 px · 0,5 Мп"},
		{0, 0, 1000, ""},
		{-5, 10, 1000, ""},
	}
	for _, c := range cases {
		if got := FileLine(c.w, c.h, c.size); got != c.want {
			t.Errorf("%dx%d %d: %q, ждали %q", c.w, c.h, c.size, got, c.want)
		}
	}
}

// ReadAbout по настоящим файлам: JPEG с EXIF (несколько разных сразу), поворот меняет размеры,
// JPEG без EXIF, PNG, нет файла, не картинка.
func TestAboutNoExif(t *testing.T) {
	dir := testkit.TempDir(t)
	full := testkit.ShotFields{
		Make: "Canon", Model: "Canon EOS R6", Exposure: rat(1, 200), FNumber: rat(28, 10),
		Focal: rat(50, 1), ISO: 400, Orig: "2026:06:12 14:32:00", Orient: 6,
	}
	testkit.JPEGSegments(t, dir, "a.jpg", 60, 40, testkit.ExifShot(false, full))
	testkit.JPEGSegments(t, dir, "b.jpg", 60, 40, testkit.ExifShot(true, testkit.ShotFields{Model: "X100V", ISO: 200}))
	testkit.JPEG(t, dir, "plain.jpg", 60, 40)
	testkit.PNG(t, dir, "p.png", 60, 40, color.NRGBA{R: 200, A: 255})
	if err := os.WriteFile(filepath.Join(dir, "bad.jpg"), []byte("не картинка"), 0o600); err != nil {
		t.Fatal(err)
	}

	a, err := ReadAbout(filepath.Join(dir, "a.jpg"))
	if err != nil {
		t.Fatal(err)
	}
	wantShot := []string{"Canon EOS R6", "50 мм · f/2,8 · 1/200 с · ISO 400", "Снято 12.06.2026, 14:32"}
	if !slices.Equal(a.Shot, wantShot) {
		t.Errorf("a.jpg: %q", a.Shot)
	}
	if want := "40 × 60 px"; !strings.HasPrefix(a.File, want) { // ориентация 6 — ширина и высота наоборот
		t.Errorf("a.jpg File: %q", a.File)
	}
	b, err := ReadAbout(filepath.Join(dir, "b.jpg"))
	if err != nil || !slices.Equal(b.Shot, []string{"X100V", "ISO 200"}) {
		t.Errorf("b.jpg: %+v %v", b, err)
	}
	// без данных съёмки: File есть, Shot пуст
	for _, name := range []string{"plain.jpg", "p.png"} {
		ab, err := ReadAbout(filepath.Join(dir, name))
		if err != nil || len(ab.Shot) != 0 || !strings.HasPrefix(ab.File, "60 × 40 px") {
			t.Errorf("%s: %+v %v", name, ab, err)
		}
	}
	for _, name := range []string{"bad.jpg", "нет.jpg", ""} {
		if _, err := ReadAbout(filepath.Join(dir, name)); err == nil {
			t.Errorf("%q: ждали ошибку", name)
		}
	}
}

// Путь из .salbum недоверенный (аудит 0.21): UNC, поток NTFS и относительный — отказ до открытия;
// картинка не JPEG и не PNG (GIF) — отказ, как у всех заголовков 0.21 (imgfile.DimsOf).
func TestAboutRefusesUnsafe(t *testing.T) {
	dir := testkit.TempDir(t)
	jp := testkit.JPEG(t, dir, "a.jpg", 60, 40)
	for _, p := range []string{`\\127.0.0.1\c$\a.jpg`, `//host/share/a.jpg`, jp + ":zone", "a.jpg"} {
		if _, err := ReadAbout(p); err == nil {
			t.Errorf("%q: ждали отказ", p)
		}
	}
	gp := filepath.Join(dir, "g.gif")
	f, err := os.Create(gp)
	if err != nil {
		t.Fatal(err)
	}
	err = gif.Encode(f, image.NewPaletted(image.Rect(0, 0, 20, 10), color.Palette{color.Black, color.White}), nil)
	if cerr := f.Close(); err != nil || cerr != nil {
		t.Fatal(err, cerr)
	}
	if ab, err := ReadAbout(gp); err == nil {
		t.Fatalf("GIF: %+v", ab)
	}
	if _, err := ReadAbout(jp); err != nil { // тот же JPEG по локальному пути читается
		t.Fatal(err)
	}
}
