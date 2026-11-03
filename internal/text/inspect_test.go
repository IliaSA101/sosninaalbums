package text

import (
	"bytes"
	"encoding/binary"
	"errors"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
	"unicode/utf16"

	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

// builtinBytes — байты встроенного начертания (тесты берут их как «свой» файл).
func builtinBytes(t testing.TB, file string) []byte {
	t.Helper()
	b, ok := FontFile(file)
	if !ok {
		t.Fatalf("нет встроенного %s", file)
	}
	return b
}

// fixture — файл из testdata (scripts/fonts-testdata.py).
func fixture(t testing.TB, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// splitTables — таблицы файла по тегам (каталог читается тем же readTables).
func splitTables(t testing.TB, b []byte) map[string][]byte {
	t.Helper()
	tabs, err := readTables(b)
	if err != nil {
		t.Fatal(err)
	}
	m := map[string][]byte{}
	for tag, r := range tabs {
		m[tag] = append([]byte(nil), b[r[0]:r[1]]...)
	}
	return m
}

// rebuild собирает файл заново: каталог по возрастанию тегов (так требует sfnt),
// таблицы с границы 4 байт. Контрольные суммы — нули: sfnt их не сверяет.
func rebuild(sig []byte, tabs map[string][]byte) []byte {
	tags := make([]string, 0, len(tabs))
	for tag := range tabs {
		tags = append(tags, tag)
	}
	sort.Strings(tags)
	n := len(tags)
	out := make([]byte, 12+16*n)
	copy(out, sig[:4])
	binary.BigEndian.PutUint16(out[4:], uint16(n))
	for i, tag := range tags {
		for len(out)%4 != 0 {
			out = append(out, 0)
		}
		rec := out[12+16*i:]
		copy(rec, tag)
		binary.BigEndian.PutUint32(rec[8:], uint32(len(out)))
		binary.BigEndian.PutUint32(rec[12:], uint32(len(tabs[tag])))
		out = append(out, tabs[tag]...)
	}
	return out
}

// withTables — копия файла с заменёнными (nil — убранными) таблицами.
func withTables(t testing.TB, b []byte, change map[string][]byte) []byte {
	t.Helper()
	tabs := splitTables(t, b)
	for tag, v := range change {
		if v == nil {
			delete(tabs, tag)
		} else {
			tabs[tag] = v
		}
	}
	return rebuild(b, tabs)
}

// patchU16 — копия файла с числом v по смещению off в таблице tag.
func patchU16(t testing.TB, b []byte, tag string, off int, v uint16) []byte {
	t.Helper()
	tabs := splitTables(t, b)
	tb := tabs[tag]
	binary.BigEndian.PutUint16(tb[off:], v)
	return rebuild(b, tabs)
}

// nameTable — таблица name формата 0 (Windows, UCS-2, en-US) с записями id → текст.
func nameTable(names map[uint16]string) []byte {
	ids := make([]int, 0, len(names))
	for id := range names {
		ids = append(ids, int(id))
	}
	sort.Ints(ids)
	var str []byte
	hdr := make([]byte, 6+12*len(ids))
	binary.BigEndian.PutUint16(hdr[2:], uint16(len(ids)))
	binary.BigEndian.PutUint16(hdr[4:], uint16(len(hdr)))
	for i, id := range ids {
		var s []byte
		for _, u := range utf16.Encode([]rune(names[uint16(id)])) {
			s = binary.BigEndian.AppendUint16(s, u)
		}
		rec := hdr[6+12*i:]
		binary.BigEndian.PutUint16(rec[0:], 3)
		binary.BigEndian.PutUint16(rec[2:], 1)
		binary.BigEndian.PutUint16(rec[4:], 0x409)
		binary.BigEndian.PutUint16(rec[6:], uint16(id))
		binary.BigEndian.PutUint16(rec[8:], uint16(len(s)))
		binary.BigEndian.PutUint16(rec[10:], uint16(len(str)))
		str = append(str, s...)
	}
	return append(hdr, str...)
}

func TestInspectTTF(t *testing.T) {
	// все встроенные начертания проходят проверку, признаки — как в таблице гарнитур
	for _, f := range families {
		for _, fc := range f.Faces {
			info, err := Inspect(builtinBytes(t, fc.File))
			if err != nil {
				t.Errorf("%s: %v", fc.File, err)
				continue
			}
			if info.Ext != "ttf" || info.Glyphs < 100 || info.Family == "" || info.Restricted {
				t.Errorf("%s: %+v", fc.File, info)
			}
			if info.Bold != fc.Bold || info.Italic != fc.Italic {
				t.Errorf("%s: начертание b=%v i=%v, ждали b=%v i=%v", fc.File, info.Bold, info.Italic, fc.Bold, fc.Italic)
			}
		}
	}
	info, err := Inspect(builtinBytes(t, "Montserrat-BoldItalic.ttf"))
	if err != nil || info.Family != "Montserrat" || !info.Cyr || !info.Bold || !info.Italic {
		t.Errorf("Montserrat BI: %+v %v", info, err)
	}
	// только латиница — принимается без признака кириллицы
	if info, err := Inspect(fixture(t, "sa-test-latin.otf")); err != nil || info.Cyr || info.Family != "SA Test Latin" {
		t.Errorf("латиница: %+v %v", info, err)
	}
}

func TestInspectCFF(t *testing.T) {
	b := fixture(t, "sa-test-cff.otf")
	info, err := Inspect(b)
	if err != nil {
		t.Fatal(err)
	}
	if info.Ext != "otf" || info.Family != "SA Test Cff" || !info.Cyr || info.Bold || info.Italic || info.Glyphs < 150 {
		t.Errorf("фикстура CFF: %+v", info)
	}
	// фикстура — Montserrat в CFF: ширины и кернинг те же, что у TTF
	SetCustom([]CustomFamily{{ID: FamilyID("SA Test Cff"), Name: "SA Test Cff",
		Faces: []CustomFace{{Key: keyOf(b), Ext: "otf"}}}}, loader(b))
	t.Cleanup(func() { SetCustom(nil, nil) })
	const txt = "Анна и Павел AVATAR То Wo 0123"
	got := Layout(txt, Style{Font: FamilyID("SA Test Cff"), Size: 20}, 500)
	want := Layout(txt, Style{Font: DefaultFont, Size: 20}, 500)
	if len(got.Lines) != 1 || math.Abs(got.Lines[0].W-want.Lines[0].W) > 1e-9 || got.Lines[0].W == 0 {
		t.Errorf("ширина CFF %+v, TTF %+v", got.Lines, want.Lines)
	}
}

func TestInspectRejects(t *testing.T) {
	ttf := builtinBytes(t, "Montserrat-Regular.ttf")
	junk := bytes.Repeat([]byte{0xA5}, 200)
	tooBig := make([]byte, MaxFontBytes+1)
	copy(tooBig, ttf)
	tooMany := append([]byte(nil), ttf...)
	binary.BigEndian.PutUint16(tooMany[4:], 300)
	noTables := append([]byte(nil), ttf[:12]...)
	binary.BigEndian.PutUint16(noTables[4:], 0)
	cases := []struct {
		name string
		b    []byte
		want error
	}{
		{"пустой", nil, ErrNotFont},
		{"11 байт", ttf[:11], ErrNotFont},
		{"мусор", junk, ErrNotFont},
		{"32 МБ + 1", tooBig, ErrFontTooBig},
		{"ttc", append([]byte("ttcf"), junk...), ErrFontCollection},
		{"woff", append([]byte("wOFF"), junk...), ErrFontWeb},
		{"woff2", append([]byte("wOF2"), junk...), ErrFontWeb},
		{"fvar", withTables(t, ttf, map[string][]byte{"fvar": make([]byte, 16)}), ErrFontVariable},
		{"CFF2", withTables(t, ttf, map[string][]byte{"CFF2": make([]byte, 16)}), ErrFontVariable},
		{"COLR", withTables(t, ttf, map[string][]byte{"COLR": make([]byte, 16)}), ErrFontColor},
		{"CBDT", withTables(t, ttf, map[string][]byte{"CBDT": make([]byte, 16)}), ErrFontColor},
		{"sbix", withTables(t, ttf, map[string][]byte{"sbix": make([]byte, 16)}), ErrFontColor},
		{"SVG", withTables(t, ttf, map[string][]byte{"SVG ": make([]byte, 16)}), ErrFontColor},
		{"без OS/2", withTables(t, ttf, map[string][]byte{"OS/2": nil}), ErrFontBroken},
		{"без head", withTables(t, ttf, map[string][]byte{"head": nil}), ErrFontBroken},
		{"без glyf", withTables(t, ttf, map[string][]byte{"glyf": nil}), ErrFontBroken},
		{"обрезанный", ttf[:len(ttf)/2], ErrFontBroken},
		{"300 таблиц", tooMany, ErrFontBroken},
		{"0 таблиц", noTables, ErrFontBroken},
		{"без букв", fixture(t, "sa-test-symbol.otf"), ErrFontNoLetters},
	}
	for _, c := range cases {
		info, err := Inspect(c.b)
		if !errors.Is(err, c.want) {
			t.Errorf("%s: %v, ждали %v", c.name, err, c.want)
		}
		if info != (FaceInfo{}) {
			t.Errorf("%s: при отказе заполнено %+v", c.name, info)
		}
		if err != nil && strings.ContainsAny(err.Error(), `\/`) {
			t.Errorf("%s: в тексте ошибки похоже на путь: %q", c.name, err)
		}
	}
	// подставная таблица сама по себе файл не портит
	if _, err := Inspect(withTables(t, ttf, map[string][]byte{"zzzz": make([]byte, 16)})); err != nil {
		t.Errorf("лишняя таблица: %v", err)
	}
}

func TestInspectStyleBits(t *testing.T) {
	ttf := builtinBytes(t, "Montserrat-Regular.ttf")
	const fsSel, macStyle = 62, 44
	cases := []struct {
		name         string
		tag          string
		off          int
		v            uint16
		bold, italic bool
	}{
		{"обычный", "OS/2", fsSel, 1 << 6, false, false},
		{"fsSelection жирный", "OS/2", fsSel, 1 << 5, true, false},
		{"fsSelection курсив", "OS/2", fsSel, 1, false, true},
		{"fsSelection наклонный", "OS/2", fsSel, 1 << 9, false, true},
		{"fsSelection ЖК", "OS/2", fsSel, 1<<5 | 1, true, true},
		{"macStyle жирный", "head", macStyle, 1, true, false},
		{"macStyle курсив", "head", macStyle, 2, false, true},
	}
	for _, c := range cases {
		b := patchU16(t, ttf, c.tag, c.off, c.v)
		if c.tag == "head" { // признак только в macStyle
			b = patchU16(t, b, "OS/2", fsSel, 1<<6)
		}
		info, err := Inspect(b)
		if err != nil || info.Bold != c.bold || info.Italic != c.italic {
			t.Errorf("%s: %+v %v", c.name, info, err)
		}
	}
	// короткая OS/2 (версия 0 без fsSelection) — признаки только из head
	b, s, _ := styleBits(make([]byte, 20), nil)
	if b || s {
		t.Error("короткая OS/2 дала признаки")
	}
}

func TestInspectFamilyName(t *testing.T) {
	cases := []struct{ fam, sub, typo, want string }{
		{"Lato", "Regular", "", "Lato"},
		{"Lato", "Bold Italic", "", "Lato"},
		{"Lato", "bold", "", "Lato"},
		{"Lato", "Light Italic", "", "Lato Light"},
		{"Lato", "Black", "", "Lato Black"},
		{"Lato", "Oblique", "", "Lato"},
		{"Lato", "", "", "Lato"},
		{"", "Regular", "Lato", "Lato"},
		{"", "Medium", "", ""},
		{strings.Repeat("Ш", 70), "Regular", "", strings.Repeat("Ш", 60)},
	}
	for _, c := range cases {
		if got := familyName(c.fam, c.sub, c.typo); got != c.want {
			t.Errorf("%q + %q (%q) = %q, ждали %q", c.fam, c.sub, c.typo, got, c.want)
		}
	}
	// из файла: name 1/2/16, управляющие знаки и переводы строк — одной строкой
	ttf := builtinBytes(t, "Montserrat-Regular.ttf")
	b := withTables(t, ttf, map[string][]byte{"name": nameTable(map[uint16]string{
		1: "  Свадебный\n\tшрифт" + string(rune(0x200b)) + " ", 2: "Light Italic", 16: "Не то"})})
	info, err := Inspect(b)
	if err != nil || info.Family != "Свадебный шрифт Light" {
		t.Errorf("имя из файла: %+v %v", info, err)
	}
	b = withTables(t, ttf, map[string][]byte{"name": nameTable(map[uint16]string{16: "Типографское", 2: "Regular"})})
	if info, err := Inspect(b); err != nil || info.Family != "Типографское" {
		t.Errorf("name 16: %+v %v", info, err)
	}
	b = withTables(t, ttf, map[string][]byte{"name": nameTable(map[uint16]string{4: "Полное"})})
	if info, err := Inspect(b); err != nil || info.Family != "" {
		t.Errorf("без имени: %+v %v", info, err)
	}
}

func TestInspectRestricted(t *testing.T) {
	// sfnt не сверяет контрольные суммы таблиц (sfnt.go: «We ignore the checksums»),
	// поэтому fsType правится в копии без пересчёта
	ttf := builtinBytes(t, "Montserrat-Regular.ttf")
	for _, c := range []struct {
		fsType uint16
		want   bool
	}{{0, false}, {0x0002, true}, {0x0302, true}, {0x0004, false}, {0x0008, false}, {0x000A, false}} {
		info, err := Inspect(patchU16(t, ttf, "OS/2", 8, c.fsType))
		if err != nil || info.Restricted != c.want {
			t.Errorf("fsType %#04x: %+v %v", c.fsType, info, err)
		}
	}
	if _, _, r := styleBits(make([]byte, 9), nil); r {
		t.Error("OS/2 короче fsType дала Restricted")
	}
}

func TestInspectTrialDraw(t *testing.T) {
	cff := fixture(t, "sa-test-cff.otf")
	if _, err := Inspect(cff); err != nil {
		t.Fatalf("настоящая пробная отрисовка: %v", err)
	}
	orig := trialDraw
	t.Cleanup(func() { trialDraw = orig })
	trialDraw = func(*sfnt.Font) bool { panic("злой глиф") }
	if _, err := Inspect(cff); !errors.Is(err, ErrFontBroken) {
		t.Errorf("паника пробной отрисовки: %v", err)
	}
	trialDraw = func(sf *sfnt.Font) bool { orig(sf); return false }
	if _, err := Inspect(cff); !errors.Is(err, ErrFontBroken) {
		t.Errorf("отказ пробной отрисовки: %v", err)
	}
	// срок вышел за время отрисовки (обход 170 глифов — доли миллисекунды)
	trialDraw = func(sf *sfnt.Font) bool {
		time.Sleep(300 * time.Millisecond)
		return orig(sf)
	}
	budget := inspectBudget
	t.Cleanup(func() { inspectBudget = budget })
	inspectBudget = 200 * time.Millisecond
	if _, err := Inspect(cff); !errors.Is(err, ErrFontBroken) {
		t.Errorf("срок в пробной отрисовке: %v", err)
	}
	// срок вышел при обходе глифов
	trialDraw = orig
	inspectBudget = 0
	if _, err := Inspect(cff); !errors.Is(err, ErrFontBroken) {
		t.Errorf("срок обхода глифов: %v", err)
	}
	inspectBudget = budget
	if _, err := Inspect(cff); err != nil {
		t.Errorf("после теста: %v", err)
	}
}

// FuzzInspect: мусор и испорченные шрифты не роняют и не вешают разбор.
// Долгий прогон: go test ./internal/text -run ^$ -fuzz FuzzInspect -fuzztime 30s.
func FuzzInspect(f *testing.F) {
	for _, file := range []string{"Montserrat-Regular.ttf", "Lobster-Regular.ttf", "BadScript-Regular.ttf"} {
		f.Add(builtinBytes(f, file))
	}
	f.Add(fixture(f, "sa-test-cff.otf"))
	f.Add(fixture(f, "sa-test-symbol.otf"))
	f.Add([]byte("ttcf\x00\x01\x00\x00\x00\x00\x00\x00"))
	f.Add([]byte("OTTO\x00\x01\x00\x00\x00\x00\x00\x00"))
	f.Fuzz(func(t *testing.T, b []byte) {
		start := time.Now()
		info, err := Inspect(b)
		if d := time.Since(start); d > inspectBudget+5*time.Second {
			t.Fatalf("разбор шёл %s", d)
		}
		if err != nil {
			return
		}
		if info.Ext != "ttf" && info.Ext != "otf" {
			t.Fatalf("расширение %q", info.Ext)
		}
		if len([]rune(info.Family)) > maxFamilyRunes || strings.ContainsAny(info.Family, "\n\r\t") {
			t.Fatalf("имя %q", info.Family)
		}
	})
}

// TestInspectUnitsPerEm — аудит 082: крошечный head.unitsPerEm раздувает глифы в десятки em,
// и маска при отрисовке растёт как кегль². Такой файл — повреждён.
func TestInspectUnitsPerEm(t *testing.T) {
	const upemOff = 18 // head.unitsPerEm
	ttf := builtinBytes(t, "Montserrat-Regular.ttf")
	cff := fixture(t, "sa-test-cff.otf")
	for _, c := range []struct {
		name string
		b    []byte
	}{
		{"TTF upem 16: глифы ~60 em", patchU16(t, ttf, "head", upemOff, 16)},
		{"TTF upem 15", patchU16(t, ttf, "head", upemOff, 15)},
		{"TTF upem 1", patchU16(t, ttf, "head", upemOff, 1)},
		{"TTF upem 0", patchU16(t, ttf, "head", upemOff, 0)},
		{"CFF upem 16", patchU16(t, cff, "head", upemOff, 16)},
	} {
		if info, err := Inspect(c.b); !errors.Is(err, ErrFontBroken) || info != (FaceInfo{}) {
			t.Errorf("%s: %+v %v", c.name, info, err)
		}
	}
	// у настоящего шрифта рамка глифа укладывается в 4 em с запасом (росчерки рукописных — тоже)
	for _, f := range []string{"GreatVibes-Regular.ttf", "Comforter-Regular.ttf", "MarckScript-Regular.ttf"} {
		if _, err := Inspect(builtinBytes(t, f)); err != nil {
			t.Errorf("%s: %v", f, err)
		}
	}
}

// TestTooBig — рамка глифа: ровно 4 em принимается, больше — нет; пустой контур — не большой.
func TestTooBig(t *testing.T) {
	const unit = fixed.Int26_6(1000 << 6)
	seg := func(op sfnt.SegmentOp, x, y fixed.Int26_6) sfnt.Segment {
		return sfnt.Segment{Op: op, Args: [3]fixed.Point26_6{{X: x, Y: y}}}
	}
	box := func(w, h fixed.Int26_6) sfnt.Segments {
		return sfnt.Segments{seg(sfnt.SegmentOpMoveTo, -w/3, 0), seg(sfnt.SegmentOpLineTo, w-w/3, 0), seg(sfnt.SegmentOpLineTo, 0, -h)}
	}
	for _, c := range []struct {
		name string
		segs sfnt.Segments
		want bool
	}{
		{"пусто", nil, false},
		{"1 em", box(unit, unit), false},
		{"ровно 4 em", box(4*unit, 4*unit), false},
		{"шире 4 em", box(4*unit+1, unit), true},
		{"выше 4 em", box(unit, 4*unit+1), true},
	} {
		if got := tooBig(c.segs, unit); got != c.want {
			t.Errorf("%s: %v", c.name, got)
		}
	}
}

// TestInspectTimeout — аудит 082: срок inspectBudget проверяется только между глифами; зависший
// разбор (здесь — пробная отрисовка) отпускает вызывающего по inspectTimeout.
func TestInspectTimeout(t *testing.T) {
	cff := fixture(t, "sa-test-cff.otf")
	orig, timeout := trialDraw, inspectTimeout
	entered, release := make(chan struct{}), make(chan struct{})
	t.Cleanup(func() { close(release); trialDraw, inspectTimeout = orig, timeout })
	trialDraw = func(*sfnt.Font) bool { close(entered); <-release; return true } // зависает до конца теста
	inspectTimeout = 50 * time.Millisecond
	done := make(chan error, 1)
	go func() { _, err := Inspect(cff); done <- err }()
	select {
	case err := <-done:
		if !errors.Is(err, ErrFontBroken) {
			t.Errorf("по сроку: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Inspect висит дольше срока")
	}
	<-entered // зависшая горутина уже прочла trialDraw: его замена ниже — не гонка
	// быстрый разбор срок не задевает
	trialDraw = orig
	inspectTimeout = timeout
	if _, err := Inspect(cff); err != nil {
		t.Errorf("обычный разбор: %v", err)
	}
}

// TestPeekFamily — дешёвое имя гарнитуры совпадает с именем Inspect; не разобранный файл — ok=false.
func TestPeekFamily(t *testing.T) {
	files := map[string][]byte{"cff": fixture(t, "sa-test-cff.otf"), "symbol": fixture(t, "sa-test-symbol.otf")}
	for _, f := range families {
		for _, fc := range f.Faces {
			files[fc.File] = builtinBytes(t, fc.File)
		}
	}
	for name, b := range files {
		fam, ok := PeekFamily(b)
		info, err := Inspect(b)
		if !ok || fam == "" || (err == nil && fam != info.Family) {
			t.Errorf("%s: %q %v, Inspect %q %v", name, fam, ok, info.Family, err)
		}
	}
	ttf := builtinBytes(t, "Montserrat-Regular.ttf")
	if fam, ok := PeekFamily(withTables(t, ttf, map[string][]byte{"name": nameTable(map[uint16]string{})})); !ok || fam != "" {
		t.Errorf("без имени: %q %v", fam, ok)
	}
	for name, b := range map[string][]byte{"пусто": nil, "мусор": bytes.Repeat([]byte{0xA5}, 200),
		"ttc": append([]byte("ttcf"), ttf[4:]...), "обрезанный": ttf[:100]} {
		if fam, ok := PeekFamily(b); ok || fam != "" {
			t.Errorf("%s: %q %v", name, fam, ok)
		}
	}
}
