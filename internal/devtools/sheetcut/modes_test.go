package sheetcut

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func quartersOpts() Options {
	o := DefaultOptions()
	o.Mode = ModeQuarters
	o.Stem = "ang"
	return o
}

// quartersSheet — лист 400×400 на белом, в каждой четверти по прямоугольнику разного размера.
func quartersSheet() *image.NRGBA {
	img := canvas(400, 400, white)
	fillRect(img, image.Rect(50, 70, 110, 130), navy)   // tl 60×60
	fillRect(img, image.Rect(250, 90, 330, 130), navy)  // tr 80×40
	fillRect(img, image.Rect(60, 260, 90, 350), navy)   // bl 30×90
	fillRect(img, image.Rect(280, 290, 350, 360), navy) // br 70×70
	return img
}

func TestQuartersCut(t *testing.T) {
	res := mustCut(t, quartersSheet(), quartersOpts())
	if res.BG != BGSeed || len(res.Stickers) != 4 {
		t.Fatalf("фон %s, стикеров %d, нужно seed и 4", res.BG, len(res.Stickers))
	}
	want := []struct {
		file string
		w, h int
		rect [4]int // в координатах листа
		cell [2]int
	}{
		{"ang-tl.png", 60, 60, [4]int{50, 70, 110, 130}, [2]int{0, 0}},
		{"ang-tr.png", 80, 40, [4]int{250, 90, 330, 130}, [2]int{1, 0}},
		{"ang-bl.png", 30, 90, [4]int{60, 260, 90, 350}, [2]int{0, 1}},
		{"ang-br.png", 70, 70, [4]int{280, 290, 350, 360}, [2]int{1, 1}},
	}
	for i, w := range want {
		s := res.Stickers[i]
		if s.File != w.file || s.W != w.w || s.H != w.h || s.Rect != w.rect || s.Cell == nil || *s.Cell != w.cell || s.Index != i+1 {
			t.Errorf("четверть %d: %s %d×%d %v %v, нужно %+v", i, s.File, s.W, s.H, s.Rect, s.Cell, w)
		}
		if s.Frame != nil {
			t.Errorf("четверть %d: «тянется» только у рамок", i)
		}
	}
	// Запись: имена по четвертям, а не NN.png.
	dir := tempDir(t)
	if err := Save(res, dir); err != nil {
		t.Fatal(err)
	}
	for _, w := range want {
		if _, err := os.Stat(filepath.Join(dir, w.file)); err != nil {
			t.Errorf("нет файла %s: %v", w.file, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "01.png")); err == nil {
		t.Error("записан NN.png при заданном имени")
	}
}

func TestQuartersEmptyAndNoStem(t *testing.T) {
	img := quartersSheet()
	fillRect(img, image.Rect(0, 200, 200, 400), white) // bl пуста
	o := quartersOpts()
	o.Stem = ""
	res := mustCut(t, img, o)
	if len(res.Stickers) != 3 || len(res.Notes) != 1 || !strings.Contains(res.Notes[0], "bl") {
		t.Fatalf("стикеров %d, замечания %v: нужна пустая bl", len(res.Stickers), res.Notes)
	}
	if got := res.Stickers[2].fileName(); got != "03.png" {
		t.Fatalf("без имени файл %q, нужно NN.png по порядку", got)
	}
	if _, err := Cut(canvas(400, 400, white), o); err == nil {
		t.Fatal("все четверти пусты: нужна ошибка")
	}
}

// bandsSheet — лист 300×200: пять полос по 20 px, промежутки 6, 30, 8, 20.
func bandsSheet() *image.NRGBA {
	img := canvas(300, 200, white)
	for _, y := range []int{10, 36, 86, 114, 154} {
		fillRect(img, image.Rect(20, y, 280, y+20), navy)
	}
	return img
}

func rowsOpts(n int) Options {
	o := DefaultOptions()
	o.Mode, o.SplitRows, o.Stem = ModeRows, n, "div"
	return o
}

func heights(res *Result) []int {
	var h []int
	for _, s := range res.Stickers {
		h = append(h, s.H)
	}
	return h
}

func eqInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestRowsN(t *testing.T) {
	cases := []struct {
		n    int
		want []int // высоты рядов сверху вниз
		note bool
	}{
		// note — подсказка «задано N, по явным промежуткам (≥ 12 px) видно 3»: только при N ≠ 3.
		{1, []int{164}, true},                // 10…174
		{2, []int{46, 88}, true},             // разрез по самому широкому (30)
		{3, []int{46, 48, 20}, false},        // + второй по ширине (20): совпало с «авто»
		{4, []int{46, 20, 20, 20}, true},     // + третий (8)
		{5, []int{20, 20, 20, 20, 20}, true}, // все промежутки
		{9, []int{20, 20, 20, 20, 20}, true}, // просили больше, чем есть
	}
	for _, c := range cases {
		res := mustCut(t, bandsSheet(), rowsOpts(c.n))
		if got := heights(res); !eqInts(got, c.want) {
			t.Errorf("N=%d: высоты %v, нужно %v", c.n, got, c.want)
		}
		if (len(res.Notes) > 0) != c.note {
			t.Errorf("N=%d: замечания %v", c.n, res.Notes)
		}
		if c.note && c.n < 9 && (len(res.Notes) != 1 || !strings.Contains(res.Notes[0], "видно 3 (полос альфы 5)")) {
			t.Errorf("N=%d: подсказка без числа рядов и полос: %v", c.n, res.Notes)
		}
	}
	res := mustCut(t, bandsSheet(), rowsOpts(3))
	for i, s := range res.Stickers {
		if s.W != 260 || s.File != "div-"+string(rune('1'+i))+".png" {
			t.Errorf("ряд %d: %d px, файл %s", i, s.W, s.File)
		}
	}
	if res.Stickers[1].Rect != [4]int{20, 86, 280, 134} {
		t.Errorf("рамка второго ряда %v в координатах листа", res.Stickers[1].Rect)
	}
	// Равные промежутки: при N−1 < их числа остаётся верхний — порядок воспроизводим.
	eq := canvas(100, 100, white)
	for _, y := range []int{0, 30, 60} {
		fillRect(eq, image.Rect(10, y+5, 90, y+25), navy)
	}
	if got := heights(mustCut(t, eq, rowsOpts(2))); !eqInts(got, []int{20, 50}) {
		t.Errorf("равные промежутки: %v, нужно верхний разрез [20 50]", got)
	}
}

func TestRowsAutoMerge(t *testing.T) {
	cases := []struct {
		gap  int
		want []int
	}{
		{0, []int{46, 48, 20}},         // по умолчанию 12: делят промежутки 30 и 20
		{5, []int{20, 20, 20, 20, 20}}, // 6, 8, 20, 30 — все не уже 5
		{25, []int{46, 88}},            // делит только 30
		{31, []int{164}},               // никто не делит: один ряд
	}
	for _, c := range cases {
		o := rowsOpts(0)
		o.RowGap = c.gap
		if got := heights(mustCut(t, bandsSheet(), o)); !eqInts(got, c.want) {
			t.Errorf("rowgap %d: %v, нужно %v", c.gap, got, c.want)
		}
	}
	// Промежуток ровно в rowgap делит (граница включена), на px уже — склеивает.
	o := rowsOpts(0)
	o.RowGap = 20
	if got := len(mustCut(t, bandsSheet(), o).Stickers); got != 3 {
		t.Errorf("rowgap 20: рядов %d, нужно 3 (промежуток 20 делит)", got)
	}
	o.RowGap = 21
	if got := len(mustCut(t, bandsSheet(), o).Stickers); got != 2 {
		t.Errorf("rowgap 21: рядов %d, нужно 2", got)
	}
}

func TestRowsNoise(t *testing.T) {
	// Крошка в промежутке не склеивает ряды: мусор меньше MinArea стирается до проекции.
	img := bandsSheet()
	fillRect(img, image.Rect(100, 62, 102, 64), navy)
	if got := heights(mustCut(t, img, rowsOpts(2))); !eqInts(got, []int{46, 88}) {
		t.Fatalf("с крошкой: %v", got)
	}
}

// pngHeader — PNG из одной подписи и IHDR (RGBA 8 бит) с заявленными размерами; тела нет.
func pngHeader(w, h uint32) []byte {
	var b bytes.Buffer
	b.WriteString("\x89PNG\r\n\x1a\n")
	ihdr := make([]byte, 13)
	binary.BigEndian.PutUint32(ihdr[0:], w)
	binary.BigEndian.PutUint32(ihdr[4:], h)
	ihdr[8], ihdr[9] = 8, 6
	_ = binary.Write(&b, binary.BigEndian, uint32(len(ihdr)))
	body := append([]byte("IHDR"), ihdr...)
	b.Write(body)
	_ = binary.Write(&b, binary.BigEndian, crc32.ChecksumIEEE(body))
	return b.Bytes()
}

func encodePNG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestDecodeSheet(t *testing.T) {
	src := ringFrame(40, 30, [4]int{}, 5, white, navy)
	good := encodePNG(t, src)
	var jb bytes.Buffer
	if err := jpeg.Encode(&jb, src, nil); err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string][]byte{"png": good, "jpeg": jb.Bytes()} {
		img, err := DecodeSheet(bytes.NewReader(data), int64(len(data)))
		if err != nil || img.Bounds().Dx() != 40 || img.Bounds().Dy() != 30 {
			t.Errorf("%s: %v %v", name, img, err)
		}
	}
	bad := map[string][]byte{
		"пусто":          nil,
		"мусор":          []byte("это не картинка, а просто текст достаточной длины"),
		"усечённый png":  good[:len(good)/2],
		"только подпись": good[:8],
		"усечённый jpeg": jb.Bytes()[:jb.Len()/2],
		"огромный png":   pngHeader(60000, 60000), // 3,6 Гпикс по заголовку, тела нет
		"широкий png":    pngHeader(1<<30, 1),     // произведение не переполняется
		"нулевой png":    pngHeader(0, 10),        // ширина 0
	}
	for name, data := range bad {
		if _, err := DecodeSheet(bytes.NewReader(data), int64(len(data))); err == nil {
			t.Errorf("%s: нужна ошибка", name)
		}
	}
	// Заголовок обещает много, но отказ — до декодирования: ошибка про предел, а не про усечённое тело.
	if _, err := DecodeSheet(bytes.NewReader(pngHeader(60000, 60000)), -1); err == nil || !strings.Contains(err.Error(), "вне пределов") {
		t.Errorf("огромный лист: %v, нужен отказ по пределу", err)
	}
	// Файл больше предела не читается вовсе.
	if _, err := DecodeSheet(bytes.NewReader(good), MaxInputBytes+1); err == nil || !strings.Contains(err.Error(), "больше предела") {
		t.Errorf("большой файл: %v", err)
	}
}

func FuzzDecodeSheet(f *testing.F) {
	src := ringFrame(24, 20, [4]int{2, 2, 2, 2}, 4, white, navy)
	var pb, jb bytes.Buffer
	_ = png.Encode(&pb, src)
	_ = jpeg.Encode(&jb, src, nil)
	f.Add(pb.Bytes())
	f.Add(jb.Bytes())
	f.Add(pb.Bytes()[:pb.Len()/2])
	f.Add(pngHeader(60000, 60000))
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, data []byte) {
		img, err := DecodeSheet(bytes.NewReader(data), int64(len(data)))
		if err != nil || img.Bounds().Dx()*img.Bounds().Dy() > 1<<20 {
			return
		}
		for _, m := range []string{ModeFrame, ModeQuarters, ModeRows} {
			o := DefaultOptions()
			o.Mode = m
			_, _ = Cut(img, o)
		}
	})
}
