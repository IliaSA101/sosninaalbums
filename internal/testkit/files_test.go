package testkit

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"unicode/utf16"
)

// RawJPEG: SOI, переданные сегменты по порядку, SOS-заглушка; без сегментов — только SOI и SOS.
func TestRawJPEG(t *testing.T) {
	a, b := Segment(0xE1, []byte("aa")), Segment(0xE2, []byte("b"))
	got := RawJPEG(a, b)
	want := append(append([]byte{0xFF, 0xD8}, a...), b...)
	want = append(want, 0xFF, 0xDA, 0x00, 0x02, 0x11, 0x22)
	if !bytes.Equal(got, want) {
		t.Errorf("RawJPEG: % x", got)
	}
	if e := RawJPEG(); !bytes.Equal(e, []byte{0xFF, 0xD8, 0xFF, 0xDA, 0x00, 0x02, 0x11, 0x22}) {
		t.Errorf("без сегментов: % x", e)
	}
	// длина в сегменте считает и свои два байта
	if s := Segment(0xE1, nil); !bytes.Equal(s, []byte{0xFF, 0xE1, 0, 2}) {
		t.Errorf("пустой сегмент: % x", s)
	}
}

// JPEGSegments: файл настоящий (декодируется), сегменты лежат сразу после SOI в заданном порядке.
func TestJPEGSegments(t *testing.T) {
	dir := TempDir(t)
	s1, s2 := Segment(0xE1, []byte("one")), ICCSegment(1, 2, []byte("data"))
	p := JPEGSegments(t, dir, "sub/a.jpg", 16, 8, s1, s2)
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(data, append(append([]byte{0xFF, 0xD8}, s1...), s2...)) {
		t.Errorf("сегменты не сразу после SOI: % x", data[:24])
	}
	f, _ := os.Open(p)
	defer f.Close()
	if cfg, _, err := image.DecodeConfig(f); err != nil || cfg.Width != 16 || cfg.Height != 8 {
		t.Errorf("cfg=%v err=%v", cfg, err)
	}
	// без сегментов — обычный файл, нулевой размер не делит на ноль
	JPEGSegments(t, dir, "b.jpg", 1, 1)
}

// ICCSegment: APP2 с «ICC_PROFILE\0», номером куска и числом кусков.
func TestICCSegment(t *testing.T) {
	s := ICCSegment(2, 3, []byte("xy"))
	if s[1] != 0xE2 || !bytes.HasPrefix(s[4:], []byte("ICC_PROFILE\x00")) || s[16] != 2 || s[17] != 3 || string(s[18:]) != "xy" {
		t.Errorf("% x", s)
	}
}

// exifColorTags читает записи IFD0, Exif IFD и Interop IFD из APP1: тег → значение (SHORT/LONG).
func exifColorTags(t *testing.T, seg []byte, big bool) map[uint16]uint32 {
	t.Helper()
	if seg[1] != 0xE1 || int(binary.BigEndian.Uint16(seg[2:]))+2 != len(seg) || string(seg[4:10]) != "Exif\x00\x00" {
		t.Fatalf("сегмент % x", seg)
	}
	tf := seg[10:]
	var bo binary.ByteOrder = binary.LittleEndian
	if big {
		bo = binary.BigEndian
	}
	if (string(tf[:2]) == "MM") != big {
		t.Fatalf("порядок %q", tf[:2])
	}
	found := map[uint16]uint32{}
	if len(tf) <= 8 { // без полей IFD нет вовсе
		return found
	}
	var walk func(off uint32)
	walk = func(off uint32) {
		for i := range int(bo.Uint16(tf[off:])) {
			e := tf[int(off)+2+12*i:]
			tag := bo.Uint16(e)
			found[tag] = bo.Uint32(e[8:])
			if bo.Uint16(e[2:]) == 3 {
				found[tag] = uint32(bo.Uint16(e[8:]))
			}
			if tag == 0x8769 || tag == 0xA005 {
				walk(found[tag])
			}
		}
	}
	walk(bo.Uint32(tf[4:]))
	return found
}

// ExifColor: оба порядка байт, все сочетания полей; тег читается обратно независимо от генератора.
func TestExifColor(t *testing.T) {
	type tags struct {
		orient, cs int
		interop    string
	}
	for _, big := range []bool{true, false} {
		for _, c := range []tags{{0, 0, ""}, {6, 0, ""}, {0, 1, ""}, {0, 0, "R98"}, {8, 65535, "R03"}} {
			seg := ExifColor(big, c.orient, uint16(c.cs), c.interop)
			found := exifColorTags(t, seg, big)
			if uint32(c.orient) != found[0x0112] || uint32(c.cs) != found[0xA001] {
				t.Errorf("big=%v %+v: ориентация %d, пространство %d", big, c, found[0x0112], found[0xA001])
			}
			if (c.interop != "") != (found[0xA005] != 0) {
				t.Errorf("big=%v %+v: interop-указатель %d", big, c, found[0xA005])
			}
			if c.interop != "" && !bytes.Contains(seg, []byte(c.interop+"\x00")) {
				t.Errorf("big=%v %+v: индекса нет в данных", big, c)
			}
		}
	}
}

// DescV4: mluc с одной записью, строка UTF-16BE (кириллица и символ вне BMP — пара суррогатов).
func TestDescV4(t *testing.T) {
	for _, s := range []string{"", "sRGB", "Профиль цвета", "😀 x"} {
		d := DescV4(s)
		if string(d[:4]) != "mluc" || binary.BigEndian.Uint32(d[8:]) != 1 || string(d[16:20]) != "enUS" {
			t.Fatalf("%q: заголовок % x", s, d[:20])
		}
		n, off := int(binary.BigEndian.Uint32(d[20:])), int(binary.BigEndian.Uint32(d[24:]))
		u := make([]uint16, n/2)
		for i := range u {
			u[i] = binary.BigEndian.Uint16(d[off+2*i:])
		}
		if got := string(utf16.Decode(u)); got != s || off+n != len(d) {
			t.Errorf("%q: прочитали %q, длина %d/%d", s, got, off+n, len(d))
		}
	}
}

// PNG: левая половина — цвет, правая прозрачна; нечётная ширина делит по w/2.
func TestPNGHalf(t *testing.T) {
	c := color.NRGBA{200, 10, 30, 255}
	for _, w := range []int{10, 5, 1} {
		f, err := os.Open(PNG(t, TempDir(t), "a.png", w, 4, c))
		if err != nil {
			t.Fatal(err)
		}
		img, err := png.Decode(f)
		_ = f.Close()
		if err != nil {
			t.Fatal(err)
		}
		for x := range w {
			got := color.NRGBAModel.Convert(img.At(x, 3)).(color.NRGBA)
			if x < w/2 && got != c || x >= w/2 && got.A != 0 {
				t.Errorf("w=%d x=%d: %v", w, x, got)
			}
		}
	}
}

// CMYKJPEG: заголовок читается как CMYK нужного размера, полный декод не удаётся.
func TestCMYKJPEG(t *testing.T) {
	p := CMYKJPEG(t, TempDir(t), "c.jpg", 640, 480)
	f, _ := os.Open(p)
	defer f.Close()
	cfg, _, err := image.DecodeConfig(f)
	if err != nil || cfg.Width != 640 || cfg.Height != 480 || cfg.ColorModel != color.CMYKModel {
		t.Fatalf("cfg=%+v err=%v", cfg, err)
	}
	_, _ = f.Seek(0, 0)
	if _, _, err := image.Decode(f); err == nil {
		t.Error("полный декод заготовки без данных не должен удаваться")
	}
}

// Ориентация меняет только тег, пиксели не трогает; orient ≤ 1 тега не пишет.
func TestJPEGOrientedTag(t *testing.T) {
	dir := TempDir(t)
	plain, _ := os.ReadFile(JPEGOriented(t, dir, "1.jpg", 12, 6, 1))
	rot, _ := os.ReadFile(JPEGOriented(t, dir, "6.jpg", 12, 6, 6))
	if bytes.Contains(plain[:40], []byte("Exif")) || !bytes.Contains(rot[:40], []byte("Exif")) {
		t.Errorf("Exif: без поворота %v, с поворотом %v", bytes.Contains(plain[:40], []byte("Exif")), bytes.Contains(rot[:40], []byte("Exif")))
	}
	if !bytes.HasSuffix(rot, plain[2:]) {
		t.Error("тело кадра изменилось")
	}
}

// PSD: заголовок 8BPS, размеры, ресурсы 1005/1032/1057 и пиксели одного цвета читаются обратно.
func TestPSDBytes(t *testing.T) {
	be := binary.BigEndian
	c := color.NRGBA{9, 8, 7, 255}
	b := PSDBytes(PSDSpec{W: 3, H: 2, C: c, DPI: 300, Guides: []PSDGuide{{Pos: 10.5, Horz: true}, {Pos: 4}}, NoMerged: true, Version: 2})
	if string(b[:4]) != "8BPS" || be.Uint16(b[4:]) != 2 || be.Uint32(b[14:]) != 2 || be.Uint32(b[18:]) != 3 {
		t.Fatalf("заголовок: % x", b[:26])
	}
	// color mode data (4) → ресурсы
	rl := int(be.Uint32(b[30:]))
	res := b[34 : 34+rl]
	ids := map[uint16][]byte{}
	for p := 0; p < len(res); {
		id, n := be.Uint16(res[p+4:]), int(be.Uint32(res[p+8:]))
		ids[id] = res[p+12 : p+12+n]
		p += 12 + n + n%2
	}
	if len(ids) != 3 {
		t.Fatalf("ресурсов %d: %v", len(ids), ids)
	}
	if got := float64(be.Uint32(ids[1005])) / 65536; math.Abs(got-300) > 1e-6 {
		t.Errorf("dpi %v", got)
	}
	g := ids[1032]
	if be.Uint32(g[12:]) != 2 || int32(be.Uint32(g[16:])) != 336 || g[20] != 1 || int32(be.Uint32(g[21:])) != 128 || g[25] != 0 {
		t.Errorf("направляющие: % x", g)
	}
	if len(ids[1057]) != 5 {
		t.Errorf("1057: % x", ids[1057])
	}
	// пиксели: после ресурсов 4 байта слоёв и 2 байта сжатия, затем 3 плоскости по 6
	px := b[34+rl+4+2:]
	want := append(append(bytes.Repeat([]byte{9}, 6), bytes.Repeat([]byte{8}, 6)...), bytes.Repeat([]byte{7}, 6)...)
	if !bytes.Equal(px, want) {
		t.Errorf("пиксели: % x", px)
	}
	// минимальный файл: версия по умолчанию 1, ни одного ресурса
	m := PSDBytes(PSDSpec{W: 1, H: 1})
	if be.Uint16(m[4:]) != 1 || be.Uint32(m[30:]) != 0 {
		t.Errorf("минимальный PSD: % x", m[:34])
	}
}

// Ресурс нечётной длины выровнен нулём: следующий блок читается с чётного смещения.
func TestPSDBlockPadding(t *testing.T) {
	var b bytes.Buffer
	psdBlock(&b, 7, []byte{1, 2, 3})
	psdBlock(&b, 8, []byte{4, 5})
	if b.Len() != (12+3+1)+(12+2) || string(b.Bytes()[16:20]) != "8BIM" {
		t.Errorf("% x", b.Bytes())
	}
}

func TestPSDFile(t *testing.T) {
	p := PSD(t, TempDir(t), "x/a.psd", PSDSpec{W: 2, H: 2, C: color.NRGBA{1, 2, 3, 255}})
	data, err := os.ReadFile(p)
	if err != nil || !bytes.Equal(data, PSDBytes(PSDSpec{W: 2, H: 2, C: color.NRGBA{1, 2, 3, 255}})) {
		t.Errorf("файл не совпал с байтами: %v", err)
	}
}

// DirLink: по ссылке видны файлы целевой папки.
func TestDirLink(t *testing.T) {
	dir := TempDir(t)
	target := filepath.Join(dir, "real")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "f.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := DirLink(t, target, filepath.Join(dir, "link"))
	if data, err := os.ReadFile(filepath.Join(link, "f.txt")); err != nil || string(data) != "x" {
		t.Errorf("через ссылку: %q %v", data, err)
	}
}

// MissingDrive: на Windows — корень вида «Q:\», которого нет; не Windows — тест пропущен.
func TestMissingDrive(t *testing.T) {
	root := MissingDrive(t)
	if runtime.GOOS != "windows" {
		t.Fatalf("не Windows: ждали t.Skip, получили %q", root)
	}
	if !regexp.MustCompile(`^[D-Z]:\\$`).MatchString(root) {
		t.Errorf("корень %q", root)
	}
	if _, err := os.Lstat(root); err == nil {
		t.Errorf("том %q существует", root)
	}
}

// RemoveAll: убирает дерево, пустой путь не падает; TempDir обрезает длинное имя теста.
func TestRemoveAllTree(t *testing.T) {
	dir := TempDir(t)
	p := filepath.Join(dir, "a", "b")
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p, "f"), []byte("1"), 0o644); err != nil {
		t.Fatal(err)
	}
	RemoveAll(t, filepath.Join(dir, "a"))
	if _, err := os.Stat(filepath.Join(dir, "a")); !os.IsNotExist(err) {
		t.Errorf("дерево осталось: %v", err)
	}
	RemoveAll(t, filepath.Join(dir, "нет такой")) // отсутствующий путь — не ошибка
}

func TestTempDirLongName(t *testing.T) {
	t.Run(strings.Repeat("длинное/имя ", 8), func(t *testing.T) {
		d := TempDir(t)
		if base := filepath.Base(d); len(base) > 40+20 || strings.ContainsAny(base, `/\ `) {
			t.Errorf("имя папки %q", base)
		}
		if st, err := os.Stat(d); err != nil || !st.IsDir() {
			t.Errorf("папки нет: %v", err)
		}
	})
}
