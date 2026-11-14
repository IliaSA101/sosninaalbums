package table

import (
	"errors"
	"runtime"
	"strings"
	"testing"
)

// allocated — сколько байт выделил f (TotalAlloc растёт монотонно, GC его не уменьшает,
// поэтому замер стабилен); данные для f строятся до вызова.
func allocated(f func()) uint64 {
	runtime.GC()
	var a, b runtime.MemStats
	runtime.ReadMemStats(&a)
	f()
	runtime.ReadMemStats(&b)
	return b.TotalAlloc - a.TotalAlloc
}

// maxAlloc — потолок роста памяти на разбор бомбы: без пределов было бы сотни МБ,
// с пределами — единицы; 50 МБ даёт запас под разброс сборщика и кэши zip/flate.
const maxAlloc = 50 << 20

// expectErr читает книгу, проверяет ошибку и рост памяти.
func expectErr(t *testing.T, data []byte, want error) {
	t.Helper()
	var err error
	n := allocated(func() { _, err = Read(data) })
	if !errors.Is(err, want) {
		t.Fatalf("ошибка %v, ждали %v", err, want)
	}
	if n > maxAlloc {
		t.Errorf("разбор выделил %d МБ, предел %d МБ", n>>20, maxAlloc>>20)
	}
}

func TestXMLStreamLimits(t *testing.T) {
	deep := func(n int) string { return strings.Repeat("<x>", n) + strings.Repeat("</x>", n) }
	cellRow := `<row><c t="inlineStr"><is><t>Иванов</t></is></c></row>`

	t.Run("вложенность 10^5 в листе", func(t *testing.T) {
		// Незакрытые <a> — самый дешёвый для атакующего вариант (3 байта на уровень).
		expectErr(t, book(t, sheetXML(strings.Repeat("<a>", 100000)), ""), ErrBroken)
		expectErr(t, book(t, sheetXML(deep(100000)), ""), ErrBroken)
	})
	t.Run("вложенность 10^5 в общих строках", func(t *testing.T) {
		sheet := sheetXML(`<row><c t="s"><v>0</v></c></row>`)
		expectErr(t, book(t, sheet, `<sst>`+strings.Repeat("<a>", 100000)), ErrBroken)
		expectErr(t, book(t, sheet, `<sst><si>`+deep(100000)+`</si></sst>`), ErrBroken)
	})
	t.Run("вложенность и пространства имён", func(t *testing.T) {
		// Каждый уровень объявляет префикс: стек пространств имён Decoder растёт вместе с глубиной.
		expectErr(t, book(t, sheetXML(strings.Repeat(`<a xmlns:p="u">`, 100000)), ""), ErrBroken)
	})
	t.Run("граница глубины", func(t *testing.T) {
		// worksheet и sheetData — два уровня, остальное — вложенные <x>.
		ok := book(t, sheetXML(deep(MaxDepth-2)+cellRow), "")
		if s, err := Read(ok); err != nil || len(s.Rows) != 1 {
			t.Fatalf("глубина %d: %v, %q", MaxDepth, err, s.Rows)
		}
		expectErr(t, book(t, sheetXML(deep(MaxDepth-1)+cellRow), ""), ErrBroken)
	})
	t.Run("вложенность в служебных частях", func(t *testing.T) {
		// Закрытая вложенность: без предела глубины разбор бы прошёл и дал «нет листов».
		wb := `<workbook>` + deep(100000) + `</workbook>`
		data := zipOf(t, part{"xl/workbook.xml", wb}, part{"xl/_rels/workbook.xml.rels", relsXML([2]string{"rId1", "worksheets/sheet1.xml"})}, part{"xl/worksheets/sheet1.xml", sheetXML(cellRow)})
		expectErr(t, data, ErrBroken)
		rels := `<Relationships>` + deep(100000) + `</Relationships>`
		data = zipOf(t, part{"xl/workbook.xml", workbookXML([3]string{"A", "", "rId1"})}, part{"xl/_rels/workbook.xml.rels", rels}, part{"xl/worksheets/sheet1.xml", sheetXML(cellRow)})
		expectErr(t, data, ErrBroken)
	})
	t.Run("атрибуты 10^6", func(t *testing.T) {
		expectErr(t, book(t, sheetXML(`<row><c`+strings.Repeat(` a=""`, 1000000)+`/></row>`), ""), ErrTooBig)
	})
	t.Run("объявления xmlns 10^6", func(t *testing.T) {
		expectErr(t, book(t, sheetXML(`<row><c`+strings.Repeat(` xmlns:p=""`, 1000000)+`/></row>`), ""), ErrTooBig)
		// Разные префиксы: ни одно объявление не затирает прежнее.
		var b strings.Builder
		for i := range 300000 {
			b.WriteString(` xmlns:p` + string(rune('a'+i%26)) + string(rune('a'+i/26%26)) + string(rune('a'+i/676%26)) + `=""`)
		}
		expectErr(t, book(t, sheetXML(`<row><c`+b.String()+`/></row>`), ""), ErrTooBig)
	})
	t.Run("атрибуты в общих строках и служебных частях", func(t *testing.T) {
		attrs := strings.Repeat(` a=""`, 1000000)
		sheet := sheetXML(`<row><c t="s"><v>0</v></c></row>`)
		expectErr(t, book(t, sheet, sstXML(`<si`+attrs+`/>`)), ErrTooBig)
		wb := `<workbook><sheets><sheet name="A"` + attrs + `/></sheets></workbook>`
		data := zipOf(t, part{"xl/workbook.xml", wb}, part{"xl/_rels/workbook.xml.rels", relsXML([2]string{"rId1", "worksheets/sheet1.xml"})}, part{"xl/worksheets/sheet1.xml", sheet})
		expectErr(t, data, ErrTooBig)
	})
	t.Run("текст токена больше MaxToken", func(t *testing.T) {
		sheet := sheetXML(`<row><c t="s"><v>0</v></c></row>`)
		// 2 МБ текста одной ячейки — один токен CharData.
		expectErr(t, book(t, sheet, sstXML(`<si><t>`+strings.Repeat("а", 1<<20)+`</t></si>`)), ErrTooBig)
		// Под пределом (и дольше MaxCell): читается, ячейка обрезается до MaxCell знаков.
		s, err := Read(book(t, sheet, sstXML(`<si><t>`+strings.Repeat("a", MaxToken/2)+`</t></si>`)))
		if err != nil || len([]rune(s.Rows[0][0])) != MaxCell {
			t.Fatalf("текст MaxToken/2: %v, %d знаков", err, len(s.Rows[0][0]))
		}
	})
	t.Run("много мелких токенов не считаются одним", func(t *testing.T) {
		// 5 МБ из тысяч коротких ячеек: предел токена сбрасывается на каждом Token().
		cells := strings.Repeat(`<c r="XFD1"><v>1</v></c>`, 200000)
		_, err := Read(book(t, sheetXML(`<row>`+cells+cellRow+`</row>`), ""))
		if err != nil {
			t.Fatalf("много токенов: %v", err)
		}
	})
}

// Target связи, уводящий из книги (../, абсолютный путь, диск, UNC, URL), листа не находит:
// zip на диск не распаковывается, части ищутся только по имени внутри архива.
func TestRelTargetEscapes(t *testing.T) {
	for _, tg := range []string{
		"../../../Windows/win.ini", "../../x.xml", "/../../x.xml", "/etc/passwd",
		`C:\Windows\win.ini`, `\\host\share\a.xml`, "file:///C:/Windows/win.ini", "http://example.com/a.xml",
	} {
		data := zipOf(t,
			part{"xl/workbook.xml", workbookXML([3]string{"A", "", "rId1"})},
			part{"xl/_rels/workbook.xml.rels", relsXML([2]string{"rId1", tg})},
			part{"xl/worksheets/sheet1.xml", sheetXML(`<row><c><v>1</v></c></row>`)})
		if _, err := Read(data); !errors.Is(err, ErrBroken) {
			t.Errorf("Target %q: ошибка %v, ждали %v", tg, err, ErrBroken)
		}
	}
}
