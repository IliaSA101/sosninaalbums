package decor

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	qrcode "github.com/skip2/go-qrcode"
)

// linkOfModules подбирает ссылку, чей код имеет n модулей по стороне (версия 3 — 29, версия 9 — 53).
func linkOfModules(t *testing.T, n int) string {
	t.Helper()
	for l := 1; l <= QRMaxLen-8; l++ {
		link := "https://" + strings.Repeat("a", l)
		q, err := EncodeQR(link)
		if err != nil {
			t.Fatal(err)
		}
		if len(q.Modules) == n {
			return link
		}
	}
	t.Fatalf("нет ссылки с кодом %d модулей", n)
	return ""
}

// sameColor — цвета равны по каналам (после PNG палитра приходит как color.RGBA).
func sameColor(a, b color.Color) bool {
	ar, ag, ab, aa := a.RGBA()
	br, bg, bb, ba := b.RGBA()
	return ar == br && ag == bg && ab == bb && aa == ba
}

// sameAsLib — матрица совпадает с Bitmap() библиотеки: текст, сторона, каждый модуль.
func sameAsLib(t *testing.T, q QR, link string, l int) {
	t.Helper()
	c, _ := qrcode.New(link, qrcode.Medium)
	c.DisableBorder = true
	want := c.Bitmap()
	if q.Text != link || len(q.Modules) != len(want) {
		t.Fatalf("длина %d: текст %q, сторона %d, ждали %d", l, q.Text, len(q.Modules), len(want))
	}
	for y := range want {
		if len(q.Modules[y]) != len(want) {
			t.Fatalf("длина %d: строка %d не квадратная", l, y)
		}
		for x := range want[y] {
			if q.Modules[y][x] != want[y][x] {
				t.Fatalf("длина %d: модуль %d,%d не совпал с библиотекой", l, x, y)
			}
		}
	}
}

// finders — угловые «искатели» 7×7: рамка тёмная, зазор светлый, центр 3×3 тёмный.
func finders(t *testing.T, q QR, l int) {
	t.Helper()
	n := len(q.Modules)
	for _, o := range [][2]int{{0, 0}, {n - 7, 0}, {0, n - 7}} {
		for dy := 0; dy < 7; dy++ {
			for dx := 0; dx < 7; dx++ {
				edge := dx == 0 || dx == 6 || dy == 0 || dy == 6
				core := dx >= 2 && dx <= 4 && dy >= 2 && dy <= 4
				if got := q.Modules[o[1]+dy][o[0]+dx]; got != (edge || core) {
					t.Fatalf("длина %d: искатель %v, модуль %d,%d = %v", l, o, dx, dy, got)
				}
			}
		}
	}
}

// Матрица совпадает с библиотекой; версия растёт с длиной; код квадратный, а угловые «искатели»
// 7×7 на месте (рамка тёмная, зазор светлый, центр тёмный).
func TestQREncode(t *testing.T) {
	prev := 0
	for _, l := range []int{1, 20, 60, 120, 300, QRMaxLen - 8} {
		link := "https://" + strings.Repeat("x", l)
		q, err := EncodeQR(link)
		if err != nil {
			t.Fatalf("длина %d: %v", l, err)
		}
		sameAsLib(t, q, link, l)
		if len(q.Modules) < prev {
			t.Errorf("длина %d: код уменьшился (%d < %d)", l, len(q.Modules), prev)
		}
		prev = len(q.Modules)
		finders(t, q, l)
	}
	if prev <= 29 {
		t.Errorf("длинная ссылка — код всего %d модулей", prev)
	}
	a, _ := EncodeQR("https://a.ru")
	b, _ := EncodeQR("https://" + strings.Repeat("b", 200))
	if len(a.Modules) >= len(b.Modules) {
		t.Errorf("версия не растёт: %d и %d", len(a.Modules), len(b.Modules))
	}
}

// Дополнение схемы, границы длины и мусор: ошибки по-русски и без самой ссылки.
func TestQRNormalize(t *testing.T) {
	ok := []struct{ in, want string }{
		{"site.ru/x", "https://site.ru/x"},
		{"www.site.ru", "https://www.site.ru"},
		{"  t.me/test \n", "https://t.me/test"},
		{"https://t.me/test", "https://t.me/test"},
		{"http://a.ru", "http://a.ru"},
		{"mailto:a@b.ru", "mailto:a@b.ru"},
		{"MAILTO:a@b.ru", "MAILTO:a@b.ru"},
		{"tel:+79001234567", "tel:+79001234567"},
		{"ftp://x/y", "ftp://x/y"},
		{"сайт.рф/путь с пробелом", "https://сайт.рф/путь с пробелом"},
		{"a", "https://a"},
		{"site.ru/go?u=https://x", "https://site.ru/go?u=https://x"}, // «://» не в начале — схемы нет
		{"x/y://z", "https://x/y://z"},
		{"HTTPS://a.ru", "HTTPS://a.ru"},
		{strings.Repeat("я", QRMaxLen), "https://" + strings.Repeat("я", QRMaxLen)}, // предел — в знаках, не в байтах
	}
	for _, c := range ok {
		got, err := NormalizeLink(c.in)
		if err != nil || got != c.want {
			t.Errorf("%q -> %q, %v; ждали %q", c.in, got, err, c.want)
		}
	}
	bad := []struct {
		in   string
		want error
	}{
		{"", ErrQREmpty},
		{" \t\n ", ErrQREmpty},
		{strings.Repeat("a", QRMaxLen+1), ErrQRLong},
		{strings.Repeat("ж", QRMaxLen+1), ErrQRLong},
		{"secret.ru/a\nb", ErrQRChars},
		{"secret.ru/\x00", ErrQRChars},
		{"secret.ru/\x7f", ErrQRChars},
		{"secret.ru/\tx", ErrQRChars},
		{"secret.ru/a\u2028b", ErrQRChars},
		{"secret.ru/a\u2029b", ErrQRChars},
	}
	for _, c := range bad {
		_, err := NormalizeLink(c.in)
		if err != c.want {
			t.Errorf("%q: ошибка %v, ждали %v", c.in, err, c.want)
			continue
		}
		if strings.Contains(err.Error(), "secret") || !strings.ContainsAny(err.Error(), "абвгдеёжзийклмнопрстуфхцчшщъыьэюя") {
			t.Errorf("%q: текст ошибки %q — ссылка в нём или не по-русски", c.in, err)
		}
		if _, err := EncodeQR(c.in); err != c.want {
			t.Errorf("EncodeQR(%q): %v", c.in, err)
		}
	}
	// Предел — на ввод пользователя, дополненная схема сверху.
	if q, err := EncodeQR(strings.Repeat("a", QRMaxLen)); err != nil || utf8.RuneCountInString(q.Text) != QRMaxLen+8 {
		t.Errorf("500 знаков: %v", err)
	}
	// Невалидный UTF-8 — не управляющий знак и не паника: код строится по байтам.
	if q, err := EncodeQR("a.ru/\xff\xfe"); err != nil || len(q.Modules) == 0 {
		t.Errorf("невалидный UTF-8: %v", err)
	}
}

// Минимальный размер печати: версия 3 — 26 мм, версия 9 — 31 мм; пустой код — 0.
func TestQRMinMM(t *testing.T) {
	cases := []struct {
		n    int
		want float64
	}{{29, 26}, {53, 31}}
	for _, c := range cases {
		q, err := EncodeQR(linkOfModules(t, c.n))
		if err != nil {
			t.Fatal(err)
		}
		if got := q.MinMM(); got != c.want {
			t.Errorf("%d модулей: %v мм, ждали %v", c.n, got, c.want)
		}
	}
	// Для любого кода минимум целый и не меньше обоих условий.
	for l := 1; l < QRMaxLen-8; l += 37 {
		q, _ := EncodeQR("https://" + strings.Repeat("z", l))
		mm := q.MinMM()
		if mm != float64(int(mm)) ||
			mm < QRMinCodeMM*float64(q.Side())/float64(len(q.Modules)) || mm < QRMinModuleMM*float64(q.Side()) {
			t.Errorf("длина %d: %v мм нарушает условие", l, mm)
		}
	}
	if (QR{}).MinMM() != 0 {
		t.Error("пустой код: минимум не 0")
	}
}

// PNG: два цвета, модуль — квадрат целых px, совпадает с матрицей, поле белое 4 модуля, сторона
// ≤ 4096 и использует предел почти целиком (остаток меньше стороны кода в модулях).
func TestQRPNGModules(t *testing.T) {
	for _, l := range []int{1, 24, 113, 300, QRMaxLen - 8} {
		checkQRPNG(t, l)
	}
}

// checkQRPNG — PNG кода ссылки из l знаков: размер, палитра и каждый пиксель каждого модуля.
func checkQRPNG(t *testing.T, l int) {
	t.Helper()
	q, err := EncodeQR("https://" + strings.Repeat("q", l))
	if err != nil {
		t.Fatal(err)
	}
	data, err := q.PNG()
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	b, side := img.Bounds(), q.Side()
	if b.Dx() != b.Dy() || b.Dx() > MaxStickerSide || b.Dx()%side != 0 || MaxStickerSide-b.Dx() >= side {
		t.Fatalf("длина %d: %dx%d при стороне %d модулей", l, b.Dx(), b.Dy(), side)
	}
	mod := b.Dx() / side
	p, ok := img.(*image.Paletted)
	if mod < 1 || !ok || len(p.Palette) != 2 || !sameColor(p.Palette[0], color.White) || !sameColor(p.Palette[1], color.Black) {
		t.Fatalf("длина %d: модуль %d px, не палитра {белый, чёрный}", l, mod)
	}
	for py := 0; py < b.Dy(); py++ {
		for px := 0; px < b.Dx(); px++ {
			want := uint8(0)
			if x, y := px/mod-QRQuiet, py/mod-QRQuiet; x >= 0 && y >= 0 && x < len(q.Modules) && y < len(q.Modules) && q.Modules[y][x] {
				want = 1
			}
			if got := p.ColorIndexAt(px, py); got != want {
				t.Fatalf("длина %d: пиксель %d,%d (модуль %d,%d) = %d, ждали %d", l, px, py, px/mod, py/mod, got, want)
			}
		}
	}
}

// Путь SVG: ряды восстанавливают матрицу, начало — угол поля; пустой код — пустой путь и ошибка PNG.
func TestQRSVGPath(t *testing.T) {
	for _, l := range []int{1, 50, 200} {
		q, _ := EncodeQR("https://" + strings.Repeat("p", l))
		n := len(q.Modules)
		got := make([][]bool, n)
		for i := range got {
			got[i] = make([]bool, n)
		}
		sum := 0
		for _, seg := range strings.Split(strings.TrimSuffix(q.SVGPath(), "z"), "z") {
			// "M{x} {y}h{n}v1h-{n}"
			xy, tail, _ := strings.Cut(strings.TrimPrefix(seg, "M"), "h")
			xs, ys, _ := strings.Cut(xy, " ")
			x, _ := strconv.Atoi(xs)
			y, _ := strconv.Atoi(ys)
			ns, tail2, _ := strings.Cut(tail, "v1h-")
			run, _ := strconv.Atoi(ns)
			if tail2 != ns || run < 1 {
				t.Fatalf("длина %d: ряд %q разобран неверно", l, seg)
			}
			for i := 0; i < run; i++ {
				got[y-QRQuiet][x-QRQuiet+i] = true
				sum++
			}
		}
		want := 0
		for y := range q.Modules {
			for x := range q.Modules[y] {
				if q.Modules[y][x] {
					want++
				}
				if got[y][x] != q.Modules[y][x] {
					t.Fatalf("длина %d: модуль %d,%d по пути %v, в матрице %v", l, x, y, got[y][x], q.Modules[y][x])
				}
			}
		}
		if sum != want {
			t.Errorf("длина %d: тёмных по пути %d, в матрице %d", l, sum, want)
		}
	}
	if (QR{}).SVGPath() != "" {
		t.Error("пустой код — непустой путь")
	}
	if _, err := (QR{}).PNG(); err == nil {
		t.Error("пустой код — PNG без ошибки")
	}
}

// Имя плитки: адрес без схемы, не длиннее 60 знаков, обрезанное — с «…» (по знакам, не байтам).
func TestQRName(t *testing.T) {
	if got := qrName("https://t.me/test"); got != "QR · t.me/test" {
		t.Errorf("короткое: %q", got)
	}
	if got := qrName("mailto:a@b.ru"); got != "QR · mailto:a@b.ru" {
		t.Errorf("без схемы-слэшей: %q", got)
	}
	long := qrName("https://" + strings.Repeat("ж", 200))
	if utf8.RuneCountInString(long) != 60 || !strings.HasSuffix(long, "…") || !strings.HasPrefix(long, "QR · жж") {
		t.Errorf("длинное: %q (%d)", long, utf8.RuneCountInString(long))
	}
	exact := "QR · " + strings.Repeat("a", 55)
	if got := qrName("https://" + strings.Repeat("a", 55)); got != exact {
		t.Errorf("ровно 60 знаков обрезано: %q", got)
	}
}
