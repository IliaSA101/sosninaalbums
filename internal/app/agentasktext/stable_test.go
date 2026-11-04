package agentasktext

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"testing"
)

// askTextsSum — SHA-256 таблицы всех текстов вопросов агента (матрица ниже). Снята с кода до переноса
// текстов из app (162 Ш2а): перенос не меняет ни байта. Правка текста сознательно — пересчитать сумму
// (тест печатает новую) и записать причину в decisions.
const askTextsSum = "48eddd27b8527f909996a1652955e713ea55be9f690763ded81e5939d540c5ea"

func flat(b Body, ok bool) string {
	return fmt.Sprintf("%v|%s|%s|%s", ok, b.Title, b.OK, strings.Join(b.Lines, "\n"))
}

// askTextsTable — тексты всех видов вопросов по матрице входов: клиент (обычный, пустой, под программу,
// с чужим знаком) × альбом (обычный, пустой, с чужим знаком, длинный) × судьба альбома × вид; правка по
// инструментам, местам, числам, листу; выгрузка по планам папки. Строки «ключ<TAB>текст», по ключам.
func askTextsTable() string {
	m := map[string]string{}
	tableText(m)
	tableEdit(m)
	tableExport(m)
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var sb strings.Builder
	for _, k := range keys {
		fmt.Fprintf(&sb, "%q\t%q\n", k, m[k])
	}
	return sb.String()
}

// tableText — вопросы создания и добавления папки, заголовок окна ОС.
func tableText(m map[string]string) {
	clients := map[string]string{"c": "Claude Desktop", "e": "", "s": "Sosnina Albums", "b": "a\u202eb"}
	albums := map[string]string{"n": "Альбом Петровых", "e": "", "b": "a\u202eb", "l": strings.Repeat("я", 90)}
	docs := map[string]Doc{"saved": DocSaved, "draft": DocDraft, "empty": DocEmpty, "dirty": DocDirty}
	for ck, c := range clients {
		for ak, a := range albums {
			for dk, d := range docs {
				for _, kind := range []string{KindCreate, KindAdd, "delete"} {
					k := fmt.Sprintf("text/%s/%s/%s/%s", kind, ck, ak, dk)
					m[k] = flat(Text(kind, c, a, d))
					for lk, l := range map[string]string{"p": "Петровы", "e": "", "b": "x\u202ey", "l": strings.Repeat("ю", 70)} {
						m[k+"/path-"+lk] = flat(TextPath(kind, c, a, d, l))
					}
				}
			}
		}
		m["dialog/"+ck] = DialogTitle(c)
	}
}

// tableEdit — вопросы о правке: инструменты × места × числа × лист, плюс особые входы.
func tableEdit(m map[string]string) {
	places := [][]string{nil, {"8"}, {"3", "8", "12"}, {"1", "2", "3", "4", "5", "6", "7"}, {CoverWord}, {CoverWord, "2"}}
	for _, tool := range append(EditTools(), "нет") {
		for pk, pl := range places {
			for _, sheet := range []bool{false, true} {
				for _, cnt := range []int{-5, 0, 1, 3, 11, 21, 96} {
					for _, build := range []int{0, 1, 5} {
						e := EditAsk{Client: "Claude Code", Tool: tool, Count: cnt, Build: build, Places: pl, Sheet: sheet,
							Families: []string{"Плотно", "Воздух"}, Item: "Цветы", Format: "30×20"}
						m[fmt.Sprintf("edit/%s/%d/%v/%d/%d", tool, pk, sheet, cnt, build)] = flat(EditBody(e))
					}
				}
			}
		}
		for i, e := range []EditAsk{
			{Tool: tool}, {Tool: tool, Item: "Цветы"}, {Tool: tool, Families: []string{"Плотно"}},
			{Tool: tool, AlbumBg: true, OwnCat: []int{3, 7}, TakeClient: []int{5}, KeepClient: []int{1, 2, 3, 4}, Photos: 2},
			{Tool: tool, AlbumBg: true, Sheet: true, OwnCat: []int{3, 7}, TakeClient: []int{5, 6}, Photos: 2},
			{Tool: tool, AlbumBg: true, Pages: true},
			{Tool: tool, AlbumBg: true, Sheet: true, Pages: true},
			{Tool: tool, Photos: 4}, {Tool: tool, Photos: 4, Sheet: true, Places: []string{"2"}},
			{Tool: tool, Item: "Фабрика"},
			{Tool: tool, AlbumBg: true, OwnCat: []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}, TakeClient: []int{20}, Photos: 1},
		} {
			m[fmt.Sprintf("editx/%s/%d", tool, i)] = flat(EditBody(e))
		}
	}
}

// tableExport — вопрос выгрузки по планам папки.
func tableExport(m map[string]string) {
	sum := []string{"Развороты 1–24 и обложка · 300 dpi · 25 файлов", "Обычная выгрузка"}
	for i, p := range []ExportDir{
		{}, {Shown: "/v/Петровы"}, {Shown: "/v/Петровы", Create: true}, {Shown: "/v/Петровы", NextTo: true}, {Shown: "/v/Петровы", Net: true},
		{Shown: "/v/Петровы", Cloud: true}, {Shown: "/v/Петровы", Foreign: true}, {Shown: "/v/Петровы", Ours: 1},
		{Shown: "/v/Петровы", Ours: 5}, {Shown: "/v/Петровы", Ours: 11},
		{Shown: "/v/Петровы", Ours: 21}, {Shown: "/v/Петровы", Ours: 112},
		{Shown: "/v/a\u202eb"}, {Shown: `C:\v\Петровы`}, {Shown: "/v/"}, {Shown: "/v/Петровы", Create: true, NextTo: true, Net: true, Cloud: true, Ours: 2},
		{Shown: "/v/Петровы", Foreign: true, Create: true, Ours: 1},
		{Shown: "/v/" + strings.Repeat("я", 90)},
	} {
		for _, a := range []string{"Петровы", "", "a\u202eb"} {
			for _, c := range []string{"Claude Code", "Sosnina"} {
				for _, ls := range [][]string{nil, sum} {
					m[fmt.Sprintf("export/%d/%q/%q/%d", i, a, c, len(ls))] = flat(ExportText(ExportIn{Client: c, Album: a, Lines: ls, Dir: p}), true)
				}
			}
		}
	}
}

// TestAskTextsStable: тексты всех видов вопросов агента — побайтно как до переноса из app (162 Ш2а).
func TestAskTextsStable(t *testing.T) {
	table := askTextsTable()
	if n := strings.Count(table, "\n"); n != 3810 {
		t.Errorf("записей в таблице %d, ждали 3810", n)
	}
	sum := sha256.Sum256([]byte(table))
	if got := hex.EncodeToString(sum[:]); got != askTextsSum {
		t.Errorf("тексты вопросов изменились: сумма %s, снята %s", got, askTextsSum)
	}
}
