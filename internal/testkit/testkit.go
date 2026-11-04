// Package testkit — генерация тестовых изображений во временных папках.
package testkit

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// JPEG пишет в dir файл name размера w×h (градиент, чтобы кадр не был однотонным).
func JPEG(t testing.TB, dir, name string, w, h int) string {
	return JPEGOriented(t, dir, name, w, h, 1)
}

// JPEGOriented — то же, но с EXIF Orientation (1–8). Пиксели не поворачиваются:
// это именно тег, как у снимка с камеры, лежащей на боку.
func JPEGOriented(t testing.TB, dir, name string, w, h, orient int) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{uint8(x * 255 / max(w, 1)), uint8(y * 255 / max(h, 1)), 128, 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 80}); err != nil {
		t.Fatal(err)
	}
	data := buf.Bytes()
	if orient > 1 {
		data = append(append(append([]byte{}, data[:2]...), ExifColor(true, orient, 0, "")...), data[2:]...)
	}
	return write(t, dir, name, data)
}

// PNG пишет w×h PNG с альфой: левая половина — c, правая — полностью прозрачная
// (цвет под прозрачностью чёрный, как у многих редакторов).
func PNG(t testing.TB, dir, name string, w, h int, c color.NRGBA) string {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w / 2 {
			img.SetNRGBA(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return write(t, dir, name, buf.Bytes())
}

// HugeJPEG пишет файл, у которого в заголовке (SOF) заявлен размер w×h, а данных нет:
// для проверки отказа до декода. DecodeConfig такой файл читает, полный декод — нет.
func HugeJPEG(t testing.TB, dir, name string, w, h int) string {
	t.Helper()
	sof := []byte{0xFF, 0xC0, 0, 11, 8, 0, 0, 0, 0, 1, 1, 0x11, 0}
	binary.BigEndian.PutUint16(sof[5:], uint16(h))
	binary.BigEndian.PutUint16(sof[7:], uint16(w))
	// DecodeConfig останавливается на SOF только после JFIF APP0
	jfif := []byte{0xFF, 0xE0, 0, 16, 'J', 'F', 'I', 'F', 0, 1, 1, 0, 0, 1, 0, 1, 0, 0}
	data := append(append([]byte{0xFF, 0xD8}, jfif...), sof...)
	return write(t, dir, name, data)
}

// CMYKJPEG пишет файл с заголовком CMYK-JPEG (SOF с четырьмя компонентами) без данных:
// DecodeConfig видит color.CMYKModel, полный декод не получится. ICC в нём нет.
func CMYKJPEG(t testing.TB, dir, name string, w, h int) string {
	t.Helper()
	sof := []byte{0xFF, 0xC0, 0, 20, 8, 0, 0, 0, 0, 4, 1, 0x11, 0, 2, 0x11, 0, 3, 0x11, 0, 4, 0x11, 0}
	binary.BigEndian.PutUint16(sof[5:], uint16(h))
	binary.BigEndian.PutUint16(sof[7:], uint16(w))
	jfif := []byte{0xFF, 0xE0, 0, 16, 'J', 'F', 'I', 'F', 0, 1, 1, 0, 0, 1, 0, 1, 0, 0}
	return write(t, dir, name, append(append([]byte{0xFF, 0xD8}, jfif...), sof...))
}

func write(t testing.TB, dir, name string, data []byte) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}
