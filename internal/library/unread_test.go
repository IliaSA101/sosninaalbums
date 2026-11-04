package library

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"image/gif"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/imgfile"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// webpSample — образец WebP из imgfile/testdata (сплошная заливка ffmpeg, не фото).
func webpSample(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "imgfile", "testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func writeFile(t *testing.T, dir, name string, data []byte) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func gifBytes(t *testing.T) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := gif.Encode(&b, image.NewPaletted(image.Rect(0, 0, 8, 8), color.Palette{color.Black, color.White}), nil); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// bmpBytes — BMP 1×1 24 бит, собранный вручную (отдельный декодер библиотеке не нужен): файловый
// заголовок «BM», нули в зарезервированных полях, BITMAPINFOHEADER и один пиксель.
func bmpBytes(t *testing.T) []byte {
	t.Helper()
	b := make([]byte, 58)
	copy(b, "BM")
	binary.LittleEndian.PutUint32(b[2:], uint32(len(b)))
	binary.LittleEndian.PutUint32(b[10:], 54) // смещение пикселей
	binary.LittleEndian.PutUint32(b[14:], 40) // размер BITMAPINFOHEADER
	binary.LittleEndian.PutUint32(b[18:], 1)  // ширина
	binary.LittleEndian.PutUint32(b[22:], 1)  // высота
	binary.LittleEndian.PutUint16(b[26:], 1)  // плоскости
	binary.LittleEndian.PutUint16(b[28:], 24) // бит на пиксель
	return b
}

// ftyp — заголовок HEIF-семейства с брендом brand.
func ftyp(brand string) []byte {
	return append([]byte("\x00\x00\x00\x18ftyp"+brand+"\x00\x00\x00\x00"), make([]byte, 16)...)
}

// WebP под .jpg и .png — обычные фото ленты: размер из заголовка, пропусков нет.
func TestScanWebPAsJPG(t *testing.T) {
	dir := testkit.TempDir(t)
	writeFile(t, dir, "lossy.jpg", webpSample(t, "lossy16.webp"))
	writeFile(t, dir, "lossless.png", webpSample(t, "lossless24x16.webp"))
	writeFile(t, dir, "up.JPEG", webpSample(t, "lossy16.webp"))
	testkit.JPEG(t, dir, "real.jpg", 40, 30) // обычный JPEG рядом не пострадал
	photos, skipped := Scan([]string{dir})
	if len(photos) != 4 || len(skipped) != 0 {
		t.Fatalf("фото %d, пропущено %v", len(photos), skipped)
	}
	for _, p := range photos {
		if p.Name == "lossless.png" && (p.W != 24 || p.H != 16 || p.Rotate != 1) {
			t.Errorf("lossless: %dx%d, поворот %d", p.W, p.H, p.Rotate)
		}
		if p.Name == "lossy.jpg" && (p.W != 16 || p.H != 16 || p.Shot == 0) {
			t.Errorf("lossy: %dx%d, shot %d (ждали время файла)", p.W, p.H, p.Shot)
		}
	}
}

// GIF и BMP под .jpg раньше входили в ленту, а миниатюра и экспорт их не читали: теперь одни ворота.
func TestScanGIFAsJPGSkipped(t *testing.T) {
	dir := testkit.TempDir(t)
	g := writeFile(t, dir, "g.jpg", gifBytes(t))
	b := writeFile(t, dir, "b.jpg", bmpBytes(t))
	testkit.JPEG(t, dir, "ok.jpg", 40, 30)
	photos, skipped := Scan([]string{dir})
	if len(photos) != 1 || len(skipped) != 2 {
		t.Fatalf("фото %d, пропущено %v", len(photos), skipped)
	}
	why := map[string]string{}
	for _, s := range skipped {
		why[s.Path] = s.Why
	}
	if !strings.Contains(why[g], "внутри GIF") || !strings.Contains(why[b], "внутри BMP") {
		t.Errorf("причины: %v", why)
	}
}

func TestUnreadWhy(t *testing.T) {
	dir := testkit.TempDir(t)
	sof9 := []byte{0xFF, 0xD8, 0xFF, 0xC9, 0, 17, 8, 0, 16, 0, 16, 3, 1, 0x11, 0, 2, 0x11, 0, 3, 0x11, 0}
	anim := append([]byte("RIFF\x1a\x00\x00\x00WEBPVP8X\x0a\x00\x00\x00\x02\x00\x00\x00"), make([]byte, 12)...)
	png := readAll(t, testkit.PNG(t, dir, "a.png", 8, 8, color.NRGBA{A: 255}))
	jpg := readAll(t, testkit.JPEG(t, dir, "a.jpg", 20, 20))
	tiff := append([]byte("II*\x00\x08\x00\x00\x00"), make([]byte, 30)...)
	cases := []struct {
		name string
		data []byte
		want string
	}{
		{"HEIC", ftyp("heic"), "внутри HEIC, а не JPEG"},
		{"HEIF mif1", ftyp("mif1"), "внутри HEIC, а не JPEG"},
		{"AVIF", ftyp("avif"), "внутри AVIF, а не JPEG"},
		{"GIF", gifBytes(t), "внутри GIF, а не JPEG"},
		{"BMP", bmpBytes(t), "внутри BMP, а не JPEG"},
		{"TIFF", tiff, "внутри TIFF, а не JPEG"},
		{"JPEG arithmetic", sof9, "JPEG со сжатием, которое программа не читает"},
		{"WebP анимированный", anim, "анимированный WebP"},
		{"WebP обрезан", webpSample(t, "lossy16.webp")[:20], "WebP, который программа не читает"},
		{"JPEG обрезан", jpg[:12], "файл повреждён"},
		{"PNG обрезан", png[:20], "файл повреждён"},
		{"пустой", nil, "пустой файл"},
		{"текст", []byte("просто текст, не картинка"), "не похоже на картинку"},
		{"BM без нулей", []byte("BM-текст, который не BMP по заголовку"), "не похоже на картинку"},
	}
	for _, c := range cases {
		_, err := imgfile.DimsOf(bytes.NewReader(c.data), 0)
		if err == nil {
			t.Errorf("%s: файл прочитан", c.name)
			continue
		}
		head := c.data[:min(len(c.data), headLen)]
		got := Why(err, head)
		if !strings.HasPrefix(got, c.want) || !strings.HasSuffix(got, advice) {
			t.Errorf("%s: %q, ждали начало %q и совет", c.name, got, c.want)
		}
		if short := ShortWhy(got); !strings.HasPrefix(short, c.want) || strings.Contains(short, "Paint") {
			t.Errorf("%s: короткая причина %q", c.name, short)
		}
	}
	// Слишком большое: без совета «пересохраните», с пределом.
	big := readAll(t, testkit.HugeJPEG(t, dir, "big.jpg", 20000, 20000))
	_, err := imgfile.DimsOf(bytes.NewReader(big), 0)
	if got := Why(err, big[:headLen]); got != "слишком большое (больше 250 Мп)" {
		t.Errorf("слишком большое: %q", got)
	}
}

func readAll(t *testing.T, p string) []byte {
	t.Helper()
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// Ошибка открытия и чтения — не по виду файла: в фразе операция и причина ОС, пути нет.
func TestUnreadWhyOpenError(t *testing.T) {
	const secret = "/home/клиент-Иванова/свадьба/IMG_0001.jpg"
	pe := &fs.PathError{Op: "open", Path: secret, Err: syscall.EACCES}
	for name, got := range map[string]string{"Why": Why(pe, nil), "whyOf": whyOf(pe), "обёрнутая": whyOf(errors.Join(pe))} {
		if !strings.Contains(got, "open: ") || !strings.Contains(got, syscall.EACCES.Error()) || strings.Contains(got, "Иванова") || strings.Contains(got, "IMG_0001") {
			t.Errorf("%s: %q", name, got)
		}
	}
	// Настоящие отказы: нет файла и каталог под именем фото.
	dir := testkit.TempDir(t)
	if _, err := ReadInfo(filepath.Join(dir, "нет-такого-фото.jpg")); err == nil {
		t.Fatal("несуществующий файл прочитан")
	} else if got := whyOf(err); strings.Contains(got, "нет-такого-фото") || !strings.Contains(got, "open: ") {
		t.Errorf("нет файла: %q", got)
	}
	d := filepath.Join(dir, "папка-фото.jpg")
	if err := os.Mkdir(d, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadInfo(d); err == nil {
		t.Fatal("каталог прочитан")
	} else if got := whyOf(err); strings.Contains(got, "папка-фото") || !strings.Contains(got, "не обычный файл") {
		t.Errorf("каталог: %q", got)
	}
}

// Повторное чтение берёт причину из памяти сканера и файл не открывает.
func TestUnreadWhyMemo(t *testing.T) {
	dir := testkit.TempDir(t)
	h := writeFile(t, dir, "h.jpg", ftyp("heic"))
	e := writeFile(t, dir, "e.jpg", nil)
	s := NewScanner()
	var opened atomic.Int32
	s.beforeRead = func(string) { opened.Add(1) }
	scan := func() map[string]string {
		t.Helper()
		photos, skipped, err := s.Scan(context.Background(), Sources{Folders: []string{dir}}, func([]Photo, ScanProgress, bool) {})
		if err != nil || len(photos) != 0 {
			t.Fatalf("фото %d, ошибка %v", len(photos), err)
		}
		m := map[string]string{}
		for _, sk := range skipped {
			m[sk.Path] = sk.Why
		}
		return m
	}
	first := scan()
	if opened.Load() != 2 || !strings.HasPrefix(first[h], "внутри HEIC") || !strings.HasPrefix(first[e], "пустой файл") {
		t.Fatalf("первое чтение: открыто %d, причины %v", opened.Load(), first)
	}
	second := scan()
	if opened.Load() != 2 {
		t.Fatalf("из памяти файлы открывались: %d", opened.Load())
	}
	if second[h] != first[h] || second[e] != first[e] || len(second) != 2 {
		t.Fatalf("причины из памяти: %v, было %v", second, first)
	}
	// Файл заменён настоящим JPEG (другой размер и время): причина уходит, фото появляется.
	testkit.JPEG(t, dir, "h.jpg", 40, 30)
	photos, skipped, err := s.Scan(context.Background(), Sources{Folders: []string{dir}}, func([]Photo, ScanProgress, bool) {})
	if err != nil || len(photos) != 1 || len(skipped) != 1 || skipped[0].Path != e {
		t.Fatalf("после замены: фото %d, пропущено %v, ошибка %v", len(photos), skipped, err)
	}
}
