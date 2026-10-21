package text

import (
	"encoding/binary"
	"errors"
	"image"
	"strings"
	"time"
	"unicode/utf8"

	xfont "golang.org/x/image/font"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

// MaxFontBytes — предел файла своего шрифта (082-12): Arial Unicode — 23,4 МБ.
const MaxFontBytes = 32 << 20

// maxFamilyRunes — предел имени гарнитуры в списке.
const maxFamilyRunes = 60

// inspectBudget — срок обхода глифов и пробной отрисовки вместе: злой файл не
// вешает загрузку (замер: 50 тыс. глифов — 151 мс). Переменная — для тестов.
var inspectBudget = 2 * time.Second

// Причины отказа Inspect — текст для тоста, без пути.
var (
	ErrFontTooBig     = errors.New("файл больше 32 МБ")
	ErrNotFont        = errors.New("не файл шрифта")
	ErrFontCollection = errors.New("коллекция шрифтов (.ttc): сохраните нужное начертание отдельным .ttf")
	ErrFontWeb        = errors.New("веб-шрифт: нужен файл .ttf или .otf")
	ErrFontVariable   = errors.New("переменный шрифт: возьмите статические файлы (папка static в архиве шрифта)")
	ErrFontColor      = errors.New("цветной шрифт не поддерживается")
	ErrFontNoLetters  = errors.New("нет букв (символьный шрифт)")
	ErrFontBroken     = errors.New("файл шрифта повреждён или не поддерживается")
)

// FaceInfo — что программа узнала о файле шрифта (недоверенный ввод).
type FaceInfo struct {
	Family       string // имя гарнитуры для списка и поиска (082-4); пусто — имени в файле нет
	Bold, Italic bool
	Ext          string // "ttf" | "otf" — по сигнатуре, не по имени файла
	Cyr          bool   // есть все А–Я, а–я
	Restricted   bool   // OS/2.fsType: автор запретил перенос файла (082-13)
	Glyphs       int
}

// trialText — пробная строка: кириллица, латиница и цифры, как в подписях.
const trialText = "Анна и Павел AaЖж 0123"

// inspectTimeout — жёсткий срок Inspect целиком. Срок inspectBudget проверяется только
// между глифами, а один LoadGlyph CFF с вложенными подпрограммами может идти почти
// бесконечно (аудит 082). Переменная — для тестов.
var inspectTimeout = 4 * time.Second

// minUnitsPerEm — нижний предел OpenType: при меньшем глиф из int16-координат выходит
// в сотни em, и маска растёт как кегль² (аудит 082).
const minUnitsPerEm = 16

// maxGlyphEm — предел рамки глифа в em: росчерки рукописных шрифтов — до 2–3 em.
const maxGlyphEm = 4

// Inspect проверяет файл шрифта целиком до того, как он попадёт в библиотеку:
// sfnt читает глифы лениво, и битый глиф иначе уронил бы правку или выгрузку.
// Порядок и пределы — design 082 §2. Разбор идёт в своей горутине: по inspectTimeout
// вызывающий получает ErrFontBroken, а зависшая горутина остаётся доживать — прервать
// sfnt снаружи нельзя, а держать окно и замок библиотеки ради неё хуже.
func Inspect(b []byte) (FaceInfo, error) {
	type res struct {
		info FaceInfo
		err  error
	}
	ch := make(chan res, 1) // буфер: опоздавшая горутина не блокируется на отправке
	go func() {
		info, err := inspect(b)
		ch <- res{info, err}
	}()
	t := time.NewTimer(inspectTimeout)
	defer t.Stop()
	select {
	case r := <-ch:
		return r.info, r.err
	case <-t.C:
		return FaceInfo{}, ErrFontBroken
	}
}

// sniff — расширение по сигнатуре и предел размера (общие для Inspect и PeekFamily).
func sniff(b []byte) (ext string, err error) {
	if len(b) > MaxFontBytes {
		return "", ErrFontTooBig
	}
	if len(b) < 12 {
		return "", ErrNotFont
	}
	switch string(b[:4]) {
	case "ttcf":
		return "", ErrFontCollection
	case "wOFF", "wOF2":
		return "", ErrFontWeb
	case "OTTO":
		return "otf", nil
	case "\x00\x01\x00\x00", "true":
		return "ttf", nil
	}
	return "", ErrNotFont
}

// PeekFamily — имя гарнитуры, как его вернёт Inspect, но по одной таблице name, без обхода
// глифов: папка «Шрифты» чужого альбома фильтруется по нужным гарнитурам до полной проверки
// (аудит 082). Имени нет — пусто. ok = false — файл не разобрался: Inspect откажет на том же
// шаге, до обхода глифов, и назовёт причину. Это не проверка файла.
func PeekFamily(b []byte) (fam string, ok bool) {
	defer func() {
		if recover() != nil { // упал разбор — ok ещё false; упало имя — как файл без имени
			fam = ""
		}
	}()
	if _, err := sniff(b); err != nil {
		return "", false
	}
	sf, err := sfnt.Parse(b)
	if err != nil {
		return "", false
	}
	ok = true
	return faceFamily(sf), true
}

// faceFamily — имя гарнитуры из таблицы name (082-4).
func faceFamily(sf *sfnt.Font) string {
	var buf sfnt.Buffer
	name := func(id sfnt.NameID) string {
		s, err := sf.Name(&buf, id)
		if err != nil {
			return ""
		}
		return oneLine(s)
	}
	return familyName(name(sfnt.NameIDFamily), name(sfnt.NameIDSubfamily), name(sfnt.NameIDTypographicFamily))
}

// inspect — тело Inspect (в своей горутине).
func inspect(b []byte) (info FaceInfo, err error) {
	defer func() {
		if recover() != nil {
			info, err = FaceInfo{}, ErrFontBroken
		}
	}()
	if info.Ext, err = sniff(b); err != nil {
		return FaceInfo{}, err
	}
	tabs, err := readTables(b)
	if err != nil {
		return FaceInfo{}, err
	}
	switch {
	case tabs.has("fvar"), tabs.has("CFF2"):
		return FaceInfo{}, ErrFontVariable
	case tabs.has("COLR"), tabs.has("CBDT"), tabs.has("sbix"), tabs.has("SVG "):
		return FaceInfo{}, ErrFontColor
	case !tabs.has("OS/2"), !tabs.has("head"):
		return FaceInfo{}, ErrFontBroken
	}
	sf, err := sfnt.Parse(b)
	if err != nil || sf.UnitsPerEm() < minUnitsPerEm {
		return FaceInfo{}, ErrFontBroken
	}
	deadline := time.Now().Add(inspectBudget)
	if err := walkAllGlyphs(sf, deadline); err != nil {
		return FaceInfo{}, err
	}
	info.Glyphs = sf.NumGlyphs()
	lat, cyr := letters(sf)
	if !lat && !cyr {
		return FaceInfo{}, ErrFontNoLetters
	}
	info.Cyr = cyr
	info.Family = faceFamily(sf)
	info.Bold, info.Italic, info.Restricted = styleBits(tabs.bytes(b, "OS/2"), tabs.bytes(b, "head"))
	if !trialDraw(sf) || expired(deadline) {
		return FaceInfo{}, ErrFontBroken
	}
	return info, nil
}

// tables — каталог таблиц файла: тег → [начало, конец).
type tables map[string][2]int

func (t tables) has(tag string) bool { _, ok := t[tag]; return ok }

// bytes — содержимое таблицы (границы проверены в readTables); нет — nil.
func (t tables) bytes(b []byte, tag string) []byte {
	r, ok := t[tag]
	if !ok {
		return nil
	}
	return b[r[0]:r[1]]
}

// readTables читает каталог таблиц сам, с проверкой границ: признаки
// переменного и цветного шрифта sfnt не отдаёт. Таблица за концом файла — повреждён.
func readTables(b []byte) (tables, error) {
	n := int(binary.BigEndian.Uint16(b[4:]))
	if n == 0 || n > 256 || 12+16*n > len(b) {
		return nil, ErrFontBroken
	}
	t := make(tables, n)
	for i := range n {
		rec := b[12+16*i:]
		off, ln := int(binary.BigEndian.Uint32(rec[8:])), int(binary.BigEndian.Uint32(rec[12:]))
		if off > len(b) || ln > len(b)-off {
			return nil, ErrFontBroken
		}
		t[string(rec[:4])] = [2]int{off, off + ln}
	}
	return t, nil
}

// walkAllGlyphs разбирает контур и ширину каждого глифа. Единичные ошибки
// допустимы (такой глиф потом выпадет, как знак без глифа), больше 1 % — файл битый.
// Глиф с рамкой больше maxGlyphEm — сразу битый: буквы такими не бывают.
func walkAllGlyphs(sf *sfnt.Font, deadline time.Time) error {
	n := sf.NumGlyphs()
	if n <= 0 {
		return ErrFontBroken
	}
	var buf sfnt.Buffer
	unit := fixed.Int26_6(sf.UnitsPerEm()) << 6
	bad := 0
	for g := range n {
		if g%1024 == 0 && expired(deadline) {
			return ErrFontBroken
		}
		segs, e1 := sf.LoadGlyph(&buf, sfnt.GlyphIndex(g), unit, nil)
		_, e2 := sf.GlyphAdvance(&buf, sfnt.GlyphIndex(g), unit, xfont.HintingNone)
		if e1 != nil || e2 != nil {
			bad++
			continue
		}
		if tooBig(segs, unit) {
			return ErrFontBroken
		}
	}
	if bad*100 > n || expired(deadline) {
		return ErrFontBroken
	}
	return nil
}

// tooBig — рамка точек контура шире или выше maxGlyphEm (unit — em в единицах контура).
func tooBig(segs sfnt.Segments, unit fixed.Int26_6) bool {
	if len(segs) == 0 {
		return false
	}
	p0 := segs[0].Args[0]
	minX, maxX, minY, maxY := p0.X, p0.X, p0.Y, p0.Y
	for _, s := range segs {
		for i := range segArgs(s.Op) {
			p := s.Args[i]
			minX, maxX, minY, maxY = min(minX, p.X), max(maxX, p.X), min(minY, p.Y), max(maxY, p.Y)
		}
	}
	lim := int64(unit) * maxGlyphEm
	return int64(maxX)-int64(minX) > lim || int64(maxY)-int64(minY) > lim
}

// expired — срок вышел. «Не раньше», а не «после»: часы Windows грубые, и при
// нулевом сроке Now может совпасть с ним.
func expired(deadline time.Time) bool { return !time.Now().Before(deadline) }

// letters — есть ли хоть одна латинская буква и все ли 64 буквы А–я.
func letters(sf *sfnt.Font) (lat, cyr bool) {
	var buf sfnt.Buffer
	has := func(r rune) bool {
		g, err := sf.GlyphIndex(&buf, r)
		return err == nil && g != 0
	}
	for r := 'A'; r <= 'z' && !lat; r++ {
		lat = (r <= 'Z' || r >= 'a') && has(r)
	}
	cyr = true
	for r := 'А'; r <= 'я' && cyr; r++ {
		cyr = has(r)
	}
	return lat, cyr
}

// oneLine — имя из таблицы name одной строкой без лишних пробелов.
func oneLine(s string) string {
	return strings.Join(strings.Fields(CleanText(s)), " ")
}

// ribbi — подсемейства модели «обычный, жирный, курсив, жирный курсив»: они
// собираются в одну гарнитуру, остальные дописываются к имени (082-4).
var ribbi = map[string]bool{
	"regular": true, "normal": true, "bold": true, "italic": true,
	"oblique": true, "bold italic": true, "bold oblique": true,
}

// familyName — имя гарнитуры из name 1 (пусто — name 16) и подсемейства
// name 2: «Lato» + «Light Italic» → «Lato Light» (курсив — признаком).
func familyName(fam, sub, typo string) string {
	if fam == "" {
		fam = typo
	}
	if fam == "" {
		return ""
	}
	if sub != "" && !ribbi[strings.ToLower(sub)] {
		var rest []string
		for _, w := range strings.Fields(sub) {
			if lw := strings.ToLower(w); lw != "italic" && lw != "oblique" {
				rest = append(rest, w)
			}
		}
		if len(rest) > 0 {
			fam += " " + strings.Join(rest, " ")
		}
	}
	return cutRunes(fam, maxFamilyRunes)
}

func cutRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return strings.TrimSpace(string([]rune(s)[:n]))
}

// styleBits читает начертание из OS/2.fsSelection (бит 0 — курсив, 5 — жирный,
// 9 — наклонный) и head.macStyle (бит 0 — жирный, 1 — курсив), запрет переноса —
// OS/2.fsType (младшие 4 бита = 2, Restricted License). Короткая таблица — признака нет.
func styleBits(os2, head []byte) (bold, italic, restricted bool) {
	if len(os2) >= 10 {
		restricted = binary.BigEndian.Uint16(os2[8:])&0x000F == 0x0002
	}
	if len(os2) >= 64 {
		sel := binary.BigEndian.Uint16(os2[62:])
		bold = sel&(1<<5) != 0
		italic = sel&(1|1<<9) != 0
	}
	if len(head) >= 46 {
		mac := binary.BigEndian.Uint16(head[44:])
		bold = bold || mac&1 != 0
		italic = italic || mac&2 != 0
	}
	return bold, italic, restricted
}

// trialDraw — пробный путь «замер → растр» строкой trialText (082-21): обход
// глифов не ловит контур, тяжёлый именно при растеризации. Переменная — для тестов.
var trialDraw = func(sf *sfnt.Font) (ok bool) {
	defer func() {
		if recover() != nil {
			ok = false
		}
	}()
	fb, _ := font(Style{Font: DefaultFont})
	st := Style{Font: DefaultFont, Size: 14, Color: "#000000", Align: "left"}
	box := layoutWith(trialText, st, 200, sf, fb)
	drawWith(image.NewRGBA(image.Rect(0, 0, 64, 64)), box, st, 0, 0, 0, 72, sf, fb)
	return true
}
