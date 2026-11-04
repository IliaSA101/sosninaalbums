package decor

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"image"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// pngHeaderOnly — PNG с настоящим заголовком w×h и без данных: для DecodeConfig хватает IHDR, а
// 15000² одного цвета (~1 ГБ в памяти) в тесте не нужны.
func pngHeaderOnly(w, h uint32) []byte {
	var b bytes.Buffer
	b.Write([]byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'})
	ihdr := make([]byte, 13)
	binary.BigEndian.PutUint32(ihdr[0:], w)
	binary.BigEndian.PutUint32(ihdr[4:], h)
	ihdr[8], ihdr[9] = 8, 6 // 8 бит, RGBA
	chunk := append([]byte("IHDR"), ihdr...)
	_ = binary.Write(&b, binary.BigEndian, uint32(13))
	b.Write(chunk)
	_ = binary.Write(&b, binary.BigEndian, crc32.ChecksumIEEE(chunk))
	return b.Bytes()
}

// spyPic — счётчики чтения заголовка и полного декода на время теста.
func spyPic(t *testing.T) (cfgs, decodes *int) {
	t.Helper()
	cfgs, decodes = new(int), new(int)
	rc, oo := readPicConfig, openOriented
	readPicConfig = func(p string) (image.Config, error) { *cfgs++; return rc(p) }
	openOriented = func(p string) (image.Image, func(), error) { *decodes++; return oo(p) }
	t.Cleanup(func() { readPicConfig, openOriented = rc, oo })
	return cfgs, decodes
}

// TestFramePicHugeRefused — 079-35: огромная (больше 64 Мп) картинка отклоняется по заголовку, без декода и
// бюджета; отказ помнится по (путь, размер, время): повтор не открывает файл, а замена файла —
// проверяется заново.
func TestFramePicHugeRefused(t *testing.T) {
	testkit.CostTest(t)
	cfgs, decodes := spyPic(t)
	dir := testkit.TempDir(t)
	q := "w=60&h=80&c=0.2,0.2,0.2,0.2&v=1&px=120"
	for _, c := range []struct {
		name string
		w, h uint32
	}{{"huge", 15000, 15000}, {"wide", 64001, 1000}, {"tall", 1000, 64001}, {"square", 8001, 8000}} { // больше 64 Мп
		path := filepath.Join(dir, c.name+".png")
		if err := os.WriteFile(path, pngHeaderOnly(c.w, c.h), 0o644); err != nil {
			t.Fatal(err)
		}
		*cfgs, *decodes = 0, 0
		d := testkit.Fastest(1, func() {
			if rec := picGet(http.MethodGet, path, q); rec.Code != http.StatusNotFound {
				t.Errorf("%s: %d, ждали 404", c.name, rec.Code)
			}
		})
		if limit := testkit.Limit(200 * time.Millisecond); d > limit {
			t.Errorf("%s: отказ за %v, предел %v", c.name, d, limit)
		}
		if *cfgs != 1 || *decodes != 0 {
			t.Errorf("%s: заголовков %d, декодов %d; ждали 1 и 0", c.name, *cfgs, *decodes)
		}
		// Повтор (и другой запрос к тому же файлу, и маска): файл не читается совсем.
		for _, qq := range []string{q, q + "&m=1", "w=30&h=40&c=0.2,0.2,0.2,0.2&v=1&px=64"} {
			if rec := picGet(http.MethodGet, path, qq); rec.Code != http.StatusNotFound {
				t.Errorf("%s повтор: %d", c.name, rec.Code)
			}
		}
		if *cfgs != 1 || *decodes != 0 {
			t.Errorf("%s повтор: заголовков %d, декодов %d; ждали 1 и 0", c.name, *cfgs, *decodes)
		}
	}
	if len(genSem) != 0 {
		t.Errorf("слоты не освобождены: %d", len(genSem))
	}

	picReplacedAndJunk(t, dir, q, cfgs, decodes)
}

// picReplacedAndJunk — продолжение TestFramePicHugeRefused: замена файла и мусор.
func picReplacedAndJunk(t *testing.T, dir, q string, cfgs, decodes *int) {
	t.Helper()
	// Файл сменился (другой размер и время): прежний отказ не действует, проверка заново.
	path := filepath.Join(dir, "huge.png")
	ok := pngBytes(t, ringImage(60, 80, 6, 8, 6, 8))
	if err := os.WriteFile(path, ok, 0o644); err != nil {
		t.Fatal(err)
	}
	later := time.Now().Add(time.Hour)
	if err := os.Chtimes(path, later, later); err != nil {
		t.Fatal(err)
	}
	*cfgs, *decodes = 0, 0
	if rec := picGet(http.MethodGet, path, "w=60&h=80&c=0.1,0.1,0.1,0.1&v=1&px=120"); rec.Code != http.StatusOK {
		t.Fatalf("после замены файла: %d", rec.Code)
	}
	if *cfgs != 1 || *decodes != 1 {
		t.Errorf("после замены: заголовков %d, декодов %d; ждали 1 и 1", *cfgs, *decodes)
	}

	// Мусор вместо картинки тоже запоминается; предел 4096 включительно проходит заголовок.
	bad := filepath.Join(dir, "bad.png")
	if err := os.WriteFile(bad, []byte("не картинка"), 0o644); err != nil {
		t.Fatal(err)
	}
	*cfgs = 0
	for range 3 {
		if rec := picGet(http.MethodGet, bad, q); rec.Code != http.StatusNotFound {
			t.Errorf("мусор: %d", rec.Code)
		}
	}
	if *cfgs != 1 {
		t.Errorf("мусор: заголовков %d, ждали 1", *cfgs)
	}
	edge := filepath.Join(dir, "edge.png")
	if err := os.WriteFile(edge, pngHeaderOnly(MaxFrameSide, MaxFrameSide), 0o644); err != nil {
		t.Fatal(err)
	}
	st := picStamp{path: edge}
	if err := picCheckHeader(st); err != nil || picRejected(st) {
		t.Errorf("4096×4096 — предел включительно: %v", err)
	}
}
