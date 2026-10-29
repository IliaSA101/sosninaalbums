package imgfile

import (
	"bytes"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Образцы 16×16 (lossy) и 24×16 (lossless) — сплошная заливка из ffmpeg, не фото.
func webpSamples(t *testing.T) []struct {
	name   string
	data   []byte
	w, h   int
	format string
} {
	t.Helper()
	return []struct {
		name   string
		data   []byte
		w, h   int
		format string
	}{
		{"lossy", readFile(t, filepath.Join("testdata", "lossy16.webp")), 16, 16, "webp"},
		{"lossless", readFile(t, filepath.Join("testdata", "lossless24x16.webp")), 24, 16, "webp"},
	}
}

// vp8lHeader — WebP lossless с настоящим заголовком w×h и без данных: хватает DecodeConfig.
func vp8lHeader(w, h uint32) []byte {
	bits := (w - 1) | (h-1)<<14 // alpha и версия — нули
	chunk := []byte{0x2f, 0, 0, 0, 0}
	binary.LittleEndian.PutUint32(chunk[1:], bits)
	var b bytes.Buffer
	b.WriteString("RIFF")
	_ = binary.Write(&b, binary.LittleEndian, uint32(4+8+len(chunk)+1))
	b.WriteString("WEBP")
	b.WriteString("VP8L")
	_ = binary.Write(&b, binary.LittleEndian, uint32(len(chunk)))
	b.Write(chunk)
	b.WriteByte(0) // выравнивание до чётного
	return b.Bytes()
}

func TestDimsOfWebP(t *testing.T) {
	for _, c := range webpSamples(t) {
		d, err := DimsOf(bytes.NewReader(c.data), 0)
		if err != nil || d.W != c.w || d.H != c.h || d.Format != c.format || d.Color == nil {
			t.Errorf("%s: %+v, %v", c.name, d, err)
		}
		// Предел площади действует и для WebP: ровно в пределе проходит, на пиксель меньше — отказ.
		if _, err := DimsOf(bytes.NewReader(c.data), int64(c.w*c.h)); err != nil {
			t.Errorf("%s: ровно в пределе: %v", c.name, err)
		}
		if _, err := DimsOf(bytes.NewReader(c.data), int64(c.w*c.h-1)); !errors.Is(err, ErrTooBig) {
			t.Errorf("%s: на пиксель больше: %v", c.name, err)
		}
		// Обрезанный файл — отказ, не паника: RIFF-подпись есть, данных нет.
		if _, err := DimsOf(bytes.NewReader(c.data[:12]), 0); err == nil {
			t.Errorf("%s: заголовок без данных принят", c.name)
		}
	}
	// Анимированный WebP (флаг VP8X): заголовок читается, а декод отказал бы — отказ сразу.
	anim := append([]byte("RIFF\x1a\x00\x00\x00WEBPVP8X\x0a\x00\x00\x00\x02\x00\x00\x00"), make([]byte, 12)...)
	if _, err := DimsOf(bytes.NewReader(anim), 0); err == nil {
		t.Error("анимированный WebP принят")
	}
	// Подпись RIFF без WEBP (WAV, AVI) — не картинка.
	wav := append([]byte("RIFF\x24\x00\x00\x00WAVEfmt "), make([]byte, 20)...)
	if _, err := DimsOf(bytes.NewReader(wav), 0); err == nil {
		t.Error("RIFF/WAVE принят")
	}
}

func TestDimsOfWebPTooBig(t *testing.T) {
	// 16384×16384 = 268 Мп > 250 Мп: отказ по заголовку, до декода.
	_, err := DimsOf(bytes.NewReader(vp8lHeader(16384, 16384)), 0)
	if !errors.Is(err, ErrTooBig) {
		t.Fatalf("16384²: %v", err)
	}
	if _, err := DimsOf(bytes.NewReader(vp8lHeader(15000, 15000)), 0); err != nil { // 225 Мп < 250 Мп
		t.Errorf("15000² при MaxPixels: %v", err)
	}
	// Одна из сторон в нуле невозможна в WebP (хранится w-1), но 1×1 — предел снизу.
	if d, err := DimsOf(bytes.NewReader(vp8lHeader(1, 1)), 0); err != nil || d.W != 1 || d.H != 1 {
		t.Errorf("1×1: %+v, %v", d, err)
	}
}

func TestOpenOrientedWebP(t *testing.T) {
	dir := testkit.TempDir(t)
	before := DecodeBudget.Used()
	for _, c := range webpSamples(t) {
		// Формат определяется по содержимому: WebP под .jpg и .png декодируется как есть.
		for _, ext := range []string{".jpg", ".png"} {
			p := filepath.Join(dir, c.name+ext)
			if err := os.WriteFile(p, c.data, 0o600); err != nil {
				t.Fatal(err)
			}
			img, release, err := OpenOriented(p)
			if err != nil {
				t.Fatalf("%s%s: %v", c.name, ext, err)
			}
			if b := img.Bounds(); b.Dx() != c.w || b.Dy() != c.h {
				t.Errorf("%s%s: размер %v", c.name, ext, b)
			}
			release()
		}
	}
	if got := DecodeBudget.Used(); got != before {
		t.Fatalf("бюджет не возвращён: %d вместо %d", got, before)
	}
	// Огромный по заголовку WebP не доходит до декода и бюджета.
	big := filepath.Join(dir, "big.jpg")
	if err := os.WriteFile(big, vp8lHeader(16384, 16384), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := OpenOriented(big); !errors.Is(err, ErrTooBig) {
		t.Fatalf("огромный WebP: %v", err)
	}
	if got := DecodeBudget.Used(); got != before {
		t.Fatalf("бюджет тронут отказом: %d вместо %d", got, before)
	}
}
