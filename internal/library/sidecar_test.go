package library

import (
	"bytes"
	"context"
	"fmt"
	"image/color"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/imgfile"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// xmpSegment — APP1 с основным пакетом XMP.
func xmpSegment(packet []byte) []byte {
	return testkit.Segment(0xE1, append([]byte(imgfile.XMPPrefix), packet...))
}

func writeSide(t testing.TB, dir, name, attrs string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, xmpAttrs(attrs), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// marksByName — отметки найденных фото по имени файла; nil — отметок нет.
func marksByName(photos []Photo) map[string]*Marks {
	out := map[string]*Marks{}
	for _, p := range photos {
		out[p.Name] = p.Xmp
	}
	return out
}

func wantMarks(t *testing.T, got map[string]*Marks, name string, want *Marks) {
	t.Helper()
	g, ok := got[name]
	if !ok {
		t.Errorf("%s: фото нет в списке", name)
		return
	}
	if (g == nil) != (want == nil) || (g != nil && *g != *want) {
		t.Errorf("%s: отметки %+v, ждали %+v", name, g, want)
	}
}

func TestJPEGHeadXMP(t *testing.T) {
	srgb := testkit.MatrixProfile(testkit.XYZSRGB, 2.2, testkit.DescV2("sRGB"))
	packet := xmpAttrs(`xmp:Rating="4"`)
	data := testkit.RawJPEG(testkit.ExifColor(false, 6, 0, ""), xmpSegment(packet), testkit.ICCSegment(1, 1, srgb),
		xmpSegment(xmpAttrs(`xmp:Rating="1"`))) // второй пакет не берётся
	h := imgfile.Header(bytes.NewReader(data))
	if h.Orient != 6 || h.Color != imgfile.ColorSRGB || h.Profile != "sRGB" || !bytes.Equal(h.XMP, packet) {
		t.Fatalf("за один проход: ориентация %d, %q/%q, XMP %q", h.Orient, h.Color, h.Profile, h.XMP)
	}
	if h := imgfile.Header(bytes.NewReader(testkit.RawJPEG(testkit.ExifColor(false, 6, 0, "")))); h.XMP != nil {
		t.Errorf("без XMP: %q", h.XMP)
	}
	// после SOS сегменты не читаются
	after := append(testkit.RawJPEG(testkit.ExifColor(false, 3, 0, "")), xmpSegment(packet)...)
	if h := imgfile.Header(bytes.NewReader(after)); h.XMP != nil || h.Orient != 3 {
		t.Errorf("XMP после SOS: %+v", h)
	}
	// расширенный XMP и APP1 с чужим префиксом — не пакет
	ext := testkit.Segment(0xE1, append([]byte("http://ns.adobe.com/xmp/extension/\x00"), packet...))
	if h := imgfile.Header(bytes.NewReader(testkit.RawJPEG(ext))); h.XMP != nil {
		t.Errorf("расширенный XMP прочитан: %q", h.XMP)
	}
	// APP1, добитый нулями: нули отрезаются, пакет разбирается
	padded := xmpSegment(append(append([]byte{}, packet...), make([]byte, 100)...))
	h = imgfile.Header(bytes.NewReader(testkit.RawJPEG(padded)))
	if !bytes.Equal(h.XMP, packet) {
		t.Fatalf("нули в конце пакета: %q", h.XMP[max(0, len(h.XMP)-20):])
	}
	if m, _, err := parseXMP(h.XMP); err != nil || m.Rating != 4 {
		t.Errorf("пакет с нулями: %+v %v", m, err)
	}
}

func TestSidecar(t *testing.T) {
	dir := testkit.TempDir(t)
	for _, n := range []string{"a.jpg", "b.jpg", "c.JPG", "d.jpg", "f.jpg"} {
		testkit.JPEG(t, dir, n, 40, 30)
	}
	testkit.PNG(t, dir, "e.png", 40, 30, color.NRGBA{200, 100, 50, 255})
	writeSide(t, dir, "a.xmp", `xmp:Rating="1"`)
	writeSide(t, dir, "b.jpg.xmp", `xmp:Rating="2"`)
	writeSide(t, dir, "C.XMP", `xmp:Rating="3"`)
	writeSide(t, dir, "e.xmp", `xmp:Label="Green"`)
	writeSide(t, dir, "f.jpg.xmp", `xmp:Rating="5"`) // `имя.jpg.xmp` главнее `имя.xmp`
	writeSide(t, dir, "f.xmp", `xmp:Rating="1"`)
	writeSide(t, dir, "orphan.xmp", `xmp:Rating="4"`)

	photos, skipped := Scan([]string{dir})
	if len(photos) != 6 || len(skipped) != 0 {
		t.Fatalf("фото %d, пропущено %v", len(photos), skipped)
	}
	for _, p := range photos {
		if isSidecar(p.Name) {
			t.Errorf("сайдкар в списке фото: %s", p.Name)
		}
	}
	got := marksByName(photos)
	wantMarks(t, got, "a.jpg", &Marks{Rating: 1})
	wantMarks(t, got, "b.jpg", &Marks{Rating: 2})
	wantMarks(t, got, "c.JPG", &Marks{Rating: 3})
	wantMarks(t, got, "d.jpg", nil)
	wantMarks(t, got, "e.png", &Marks{Label: "green"})
	wantMarks(t, got, "f.jpg", &Marks{Rating: 5})
}

// Сайдкар главнее встроенного XMP по каждому полю; чего в нём нет или что не читается — из JPEG.
func TestSidecarPriority(t *testing.T) {
	dir := testkit.TempDir(t)
	embedded := xmpSegment(xmpAttrs(`xmp:Rating="2" xmp:Label="Red" xmpDM:pick="1"`))
	cases := []struct {
		name, side string // side == "" — сайдкара нет
		want       *Marks
	}{
		{"none.jpg", "", &Marks{Rating: 2, Label: "red", Flag: 1}},
		{"rating.jpg", `xmp:Rating="5"`, &Marks{Rating: 5, Label: "red", Flag: 1}},
		{"label.jpg", `xmp:Label="Клиенту"`, &Marks{Rating: 2, Label: "other", LabelText: "Клиенту", Flag: 1}},
		{"zero.jpg", `xmp:Rating="0" xmp:Label=""`, &Marks{Flag: 1}},
		{"reject.jpg", `xmp:Rating="-1"`, &Marks{Label: "red", Flag: -1}},
		{"junk.jpg", `xmp:Rating="abc" xmpDM:pick="7"`, &Marks{Rating: 2, Label: "red", Flag: 1}},
		{"all-zero.jpg", `xmp:Rating="0" xmp:Label="" xmpDM:pick="0"`, nil},
	}
	for _, c := range cases {
		testkit.JPEGSegments(t, dir, c.name, 40, 30, embedded)
		if c.side != "" {
			writeSide(t, dir, strings.TrimSuffix(c.name, ".jpg")+".xmp", c.side)
		}
	}
	// отказ встроенного: из Rating = −1 уходит с новым рейтингом сайдкара, из pick — остаётся
	rej := map[string]struct {
		embedded, side string
		want           *Marks
	}{
		"rej-rating.jpg":      {`xmp:Rating="-1"`, `xmp:Rating="3"`, &Marks{Rating: 3}},
		"rej-rating-pick.jpg": {`xmp:Rating="-1"`, `xmp:Rating="3" xmpDM:pick="-1"`, &Marks{Rating: 3, Flag: -1}},
		"rej-rating-lbl.jpg":  {`xmp:Rating="-1"`, `xmp:Label="Red"`, &Marks{Label: "red", Flag: -1}},
		"rej-pick.jpg":        {`xmp:Rating="2" xmpDM:pick="-1"`, `xmp:Rating="3"`, &Marks{Rating: 3, Flag: -1}},
		"rej-both.jpg":        {`xmp:Rating="-1" xmpDM:pick="-1"`, `xmp:Rating="3"`, &Marks{Rating: 3, Flag: -1}},
	}
	for name, c := range rej {
		testkit.JPEGSegments(t, dir, name, 40, 30, xmpSegment(xmpAttrs(c.embedded)))
		writeSide(t, dir, strings.TrimSuffix(name, ".jpg")+".xmp", c.side)
	}
	// битый сайдкар не затирает встроенное
	testkit.JPEGSegments(t, dir, "broken.jpg", 40, 30, embedded)
	if err := os.WriteFile(filepath.Join(dir, "broken.xmp"), []byte(`<x:xmpmeta xmp:Rating="5"`), 0o644); err != nil {
		t.Fatal(err)
	}
	photos, _ := Scan([]string{dir})
	got := marksByName(photos)
	for _, c := range cases {
		wantMarks(t, got, c.name, c.want)
	}
	for name, c := range rej {
		wantMarks(t, got, name, c.want)
	}
	wantMarks(t, got, "broken.jpg", &Marks{Rating: 2, Label: "red", Flag: 1})
}

// Битый или враждебный XMP в JPEG не делает фото нечитаемым.
func TestSidecarHostileEmbedded(t *testing.T) {
	dir := testkit.TempDir(t)
	testkit.JPEGSegments(t, dir, "doctype.jpg", 40, 30, xmpSegment(readXMPFile(t, "doctype.xmp")))
	testkit.JPEGSegments(t, dir, "broken.jpg", 40, 30, xmpSegment(readXMPFile(t, "broken.xmp")))
	testkit.JPEGSegments(t, dir, "empty.jpg", 40, 30, xmpSegment(nil))
	photos, skipped := Scan([]string{dir})
	if len(photos) != 3 || len(skipped) != 0 {
		t.Fatalf("фото %d, пропущено %v", len(photos), skipped)
	}
	for _, p := range photos {
		if p.Xmp != nil {
			t.Errorf("%s: отметки %+v из битого пакета", p.Name, *p.Xmp)
		}
	}
}

// Ошибки XMP пишутся в лог не больше maxXMPLog строк за сканирование и одной сводкой.
func TestSidecarLogLimit(t *testing.T) {
	dir := testkit.TempDir(t)
	broken := xmpSegment(readXMPFile(t, "broken.xmp"))
	n := maxXMPLog + 10
	for i := range n {
		testkit.JPEGSegments(t, dir, fmt.Sprintf("img-%02d.jpg", i), 8, 8, broken)
	}
	var buf bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(prev)
	if photos, _ := Scan([]string{dir}); len(photos) != n {
		t.Fatalf("фото %d", len(photos))
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != maxXMPLog+1 || !strings.Contains(lines[maxXMPLog], fmt.Sprintf("за сканирование %d,", n)) {
		t.Errorf("строк %d, последняя %q", len(lines), lines[len(lines)-1])
	}
}

// ReadInfo (проект, пересохранённый файл) отметок не читает: ему нужны только размеры.
func TestReadInfoNoMarks(t *testing.T) {
	dir := testkit.TempDir(t)
	p := testkit.JPEGSegments(t, dir, "a.jpg", 40, 30, xmpSegment(xmpAttrs(`xmp:Rating="4"`)))
	ph, err := ReadInfo(p)
	if err != nil || ph.Xmp != nil || ph.W != 40 {
		t.Fatalf("ReadInfo: %+v, %v", ph, err)
	}
}

func TestSidecarLimits(t *testing.T) {
	dir := testkit.TempDir(t)
	testkit.JPEG(t, dir, "big.jpg", 40, 30)
	huge := append(xmpAttrs(`xmp:Rating="5"`), bytes.Repeat([]byte(" "), 2<<20)...)
	if err := os.WriteFile(filepath.Join(dir, "big.xmp"), huge, 0o644); err != nil {
		t.Fatal(err)
	}
	photos, skipped := Scan([]string{dir})
	if len(photos) != 1 || len(skipped) != 0 || photos[0].Xmp != nil {
		t.Fatalf("сайдкар 2 МБ: фото %+v, пропущено %v", photos, skipped)
	}
	// файл вырос после листинга: LimitReader не даёт прочитать больше предела
	if _, _, err := readSidecar(sidecar{path: filepath.Join(dir, "big.xmp"), size: 100}); err == nil {
		t.Error("сайдкар больше предела прочитан")
	}
}

// Ссылка вместо сайдкара не читается. Отдельно от TestSidecarLimits: на Windows без прав
// симлинк не создать, и Skip не должен прятать проверки размера.
func TestSidecarSymlink(t *testing.T) {
	other := testkit.TempDir(t)
	target := writeSide(t, other, "real.xmp", `xmp:Rating="3"`)
	ldir := testkit.TempDir(t)
	testkit.JPEG(t, ldir, "a.jpg", 40, 30)
	if err := os.Symlink(target, filepath.Join(ldir, "a.xmp")); err != nil {
		t.Skipf("симлинк не создался: %v", err)
	}
	if photos, _ := Scan([]string{ldir}); len(photos) != 1 || photos[0].Xmp != nil {
		t.Fatalf("сайдкар-ссылка: %+v", photos)
	}
}

func TestScannerSidecarChange(t *testing.T) {
	dir := testkit.TempDir(t)
	testkit.JPEG(t, dir, "a.jpg", 40, 30)
	testkit.JPEG(t, dir, "b.jpg", 40, 30)
	side := writeSide(t, dir, "a.xmp", `xmp:Rating="3"`)
	s := NewScanner()
	scan := func() (map[string]*Marks, int) {
		t.Helper()
		var fin ScanProgress
		photos, _, err := s.Scan(context.Background(), Sources{Folders: []string{dir}}, func(_ []Photo, pr ScanProgress, final bool) {
			if final {
				fin = pr
			}
		})
		if err != nil || len(photos) != 2 {
			t.Fatalf("фото %d, %v", len(photos), err)
		}
		return marksByName(photos), fin.Memo
	}
	got, memo := scan()
	wantMarks(t, got, "a.jpg", &Marks{Rating: 3})
	if memo != 0 {
		t.Fatalf("первое чтение из памяти: %d", memo)
	}

	// изменился только сайдкар (тот же размер, время +1 с) — фото перечитано
	st, err := os.Stat(side)
	if err != nil {
		t.Fatal(err)
	}
	writeSide(t, dir, "a.xmp", `xmp:Rating="5"`)
	later := st.ModTime().Add(time.Second)
	if err := os.Chtimes(side, later, later); err != nil {
		t.Fatal(err)
	}
	got, memo = scan()
	wantMarks(t, got, "a.jpg", &Marks{Rating: 5})
	if memo != 1 {
		t.Errorf("после правки сайдкара из памяти %d, ждали 1", memo)
	}

	// без изменений — всё из памяти, отметки те же
	got, memo = scan()
	wantMarks(t, got, "a.jpg", &Marks{Rating: 5})
	if memo != 2 {
		t.Errorf("без изменений из памяти %d, ждали 2", memo)
	}

	// сайдкар удалён — отметок нет; появился у другого фото — есть
	if err := os.Remove(side); err != nil {
		t.Fatal(err)
	}
	writeSide(t, dir, "b.jpg.xmp", `xmp:Label="Blue"`)
	got, memo = scan()
	wantMarks(t, got, "a.jpg", nil)
	wantMarks(t, got, "b.jpg", &Marks{Label: "blue"})
	if memo != 0 {
		t.Errorf("после удаления и появления сайдкаров из памяти %d, ждали 0", memo)
	}
}

// Сайдкар не открылся (пропал между обходом и чтением) — результат не запоминается:
// когда файл вернётся с теми же размером и временем, он будет прочитан.
func TestSidecarRetry(t *testing.T) {
	dir := testkit.TempDir(t)
	testkit.JPEG(t, dir, "a.jpg", 40, 30)
	side := writeSide(t, dir, "a.xmp", `xmp:Rating="4"`)
	hidden := filepath.Join(testkit.TempDir(t), "a.xmp")
	s := NewScanner()
	s.beforeRead = func(string) {
		if err := os.Rename(side, hidden); err != nil {
			t.Error(err)
		}
	}
	photos, _, err := s.Scan(context.Background(), Sources{Folders: []string{dir}}, func([]Photo, ScanProgress, bool) {})
	if err != nil || len(photos) != 1 || photos[0].Xmp != nil {
		t.Fatalf("сайдкар пропал: %+v, %v", photos, err)
	}
	s.beforeRead = nil
	if err := os.Rename(hidden, side); err != nil {
		t.Fatal(err)
	}
	photos, _, err = s.Scan(context.Background(), Sources{Folders: []string{dir}}, func([]Photo, ScanProgress, bool) {})
	if err != nil || len(photos) != 1 || photos[0].Xmp == nil || photos[0].Xmp.Rating != 4 {
		t.Fatalf("сайдкар вернулся: %+v, %v", photos, err)
	}
}

// Сравнить «plain» и «xmp»: Scan 500 новых фото с пакетами по 40 КБ и сайдкарами (цель — не больше +0,3 с, design §1.5).
// «embedded» и «sidecar» — каждый источник отдельно: на JPEG 8×8 база дешевле настоящих фото,
// и почти всё время «xmp» — разбор встроенного пакета (≈ 2 мс ЦП на 40 КБ).
func BenchmarkScanXMP(b *testing.B) {
	packet := xmpSized(b, 40_000)
	for _, c := range []struct {
		name        string
		embed, side bool
	}{{"plain", false, false}, {"embedded", true, false}, {"sidecar", false, true}, {"xmp", true, true}} {
		dir := testkit.TempDir(b)
		for i := range 500 {
			name := fmt.Sprintf("img-%03d.jpg", i)
			if c.embed {
				testkit.JPEGSegments(b, dir, name, 8, 8, xmpSegment(packet))
			} else {
				testkit.JPEG(b, dir, name, 8, 8)
			}
			if c.side {
				writeSide(b, dir, name+".xmp", fmt.Sprintf(`xmp:Rating="%d"`, i%6))
			}
		}
		b.Run(c.name, func(b *testing.B) {
			for b.Loop() {
				if photos, _ := Scan([]string{dir}); len(photos) != 500 {
					b.Fatalf("фото %d", len(photos))
				}
			}
		})
	}
}

// Ошибка открытия сайдкара пишет в лог вид файла и операцию — ни полного пути из PathError,
// ни имени (073-7).
func TestSidecarLogNoDir(t *testing.T) {
	dir := testkit.TempDir(t)
	p := testkit.JPEGSegments(t, dir, "a.jpg", 8, 8)
	gone := filepath.Join(dir, "gone-dir", "Иванов.xmp")
	var buf bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(prev)
	_, retry, err := readScan(scanFile{path: p, key: strings.ToLower(p), side: sidecar{path: gone}}, &xmpLog{})
	if err != nil || !retry {
		t.Fatalf("err %v, retry %v", err, retry)
	}
	out := buf.String()
	if !strings.Contains(out, `сайдкар .xmp фото ".jpg" (имя.xmp): open`) || strings.Contains(out, "Иванов") || strings.Contains(out, "gone-dir") || strings.Contains(out, dir) {
		t.Errorf("в логе каталог: %q", out)
	}
}

// Форма имени сайдкара и вид фото в строке лога; ни имени, ни каталога (073-7).
func TestSidecarLogKind(t *testing.T) {
	big := bytes.Repeat([]byte("x"), maxXMPBytes+1)
	for _, c := range []struct{ photo, side, want string }{
		{"Иванов.jpg", "Иванов.jpg.xmp", `фото ".jpg" (имя.расш.xmp)`},
		{"Иванов.jpg", "Иванов.xmp", `фото ".jpg" (имя.xmp)`},
		{"Иванов.CR2", "Иванов.CR2.xmp", `фото ".cr2" (имя.расш.xmp)`},
		{"Иванов.cr2", "Иванов.xmp", `фото ".cr2" (имя.xmp)`},
	} {
		dir := testkit.TempDir(t)
		p := testkit.JPEGSegments(t, dir, c.photo, 8, 8)
		side := filepath.Join(dir, c.side)
		if err := os.WriteFile(side, big, 0o644); err != nil {
			t.Fatal(err)
		}
		var buf bytes.Buffer
		prev := log.Writer()
		log.SetOutput(&buf)
		_, retry, err := readScan(scanFile{path: p, key: strings.ToLower(p), side: sidecar{path: side, size: int64(len(big))}}, &xmpLog{})
		log.SetOutput(prev)
		out := buf.String()
		if err != nil || retry || !strings.Contains(out, "сайдкар .xmp "+c.want+": ") || !strings.Contains(out, "не читается") {
			t.Errorf("%s/%s: err %v retry %v, лог %q", c.photo, c.side, err, retry, out)
		}
		if strings.Contains(out, "Иванов") || strings.Contains(out, dir) {
			t.Errorf("%s/%s: в логе имя или каталог: %q", c.photo, c.side, out)
		}
	}
}
