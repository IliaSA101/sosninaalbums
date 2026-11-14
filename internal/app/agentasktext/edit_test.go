package agentasktext

// Вопрос о правке при «Спрашивать» (150, design §5.2): шаблоны текста и полнота по таблице инструментов.

import (
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

// TestEditAskText: текст по входу каждого инструмента — числа, номера, названия; пустые и предельные числа.
func TestEditAskText(t *testing.T) {
	cases := []struct {
		name  string
		in    EditAsk
		title string
		ok    string
		undo  bool // строка «Ctrl+Z отменит одним шагом.»
	}{
		{"autobuild", EditAsk{Tool: ToolAutobuild, Count: 96}, "Собрать развороты из 96 свободных фото в конец альбома?", "Собрать", true},
		{"autobuild без числа", EditAsk{Tool: ToolAutobuild}, "Собрать развороты из всех свободных фото в конец альбома?", "Собрать", true},
		{"autobuild с числом разворотов", EditAsk{Tool: ToolAutobuild, Count: 96, Build: 3}, "Собрать 3 разворота из 96 свободных фото в конец альбома?", "Собрать", true},
		{"autobuild 5 разворотов", EditAsk{Tool: ToolAutobuild, Count: 96, Build: 5}, "Собрать 5 разворотов из 96 свободных фото в конец альбома?", "Собрать", true},
		{"autobuild 1 разворот", EditAsk{Tool: ToolAutobuild, Count: 1, Build: 1}, "Собрать 1 разворот из 1 свободного фото в конец альбома?", "Собрать", true},
		{"autobuild 1", EditAsk{Tool: ToolAutobuild, Count: 1}, "Собрать развороты из 1 свободного фото в конец альбома?", "Собрать", true},
		{"autobuild 21", EditAsk{Tool: ToolAutobuild, Count: 21}, "Собрать развороты из 21 свободного фото в конец альбома?", "Собрать", true},
		{"autobuild 11", EditAsk{Tool: ToolAutobuild, Count: 11}, "Собрать развороты из 11 свободных фото в конец альбома?", "Собрать", true},
		{"autobuild отрицательное", EditAsk{Tool: ToolAutobuild, Count: -5}, "Собрать развороты из всех свободных фото в конец альбома?", "Собрать", true},
		{"autobuild предел", EditAsk{Tool: ToolAutobuild, Count: 1 << 30}, "Собрать развороты из 1073741824 свободных фото в конец альбома?", "Собрать", true},
		{"vary", EditAsk{Tool: ToolVary, Places: []string{"8"}, Families: []string{"Плотно"}}, "Другие раскладки на развороте 8 — «Плотно»?", "Изменить раскладки", true},
		{"vary несколько", EditAsk{Tool: ToolVary, Places: []string{"3", "8", "12"}}, "Другие раскладки на разворотах 3, 8, 12?", "Изменить раскладки", true},
		{"vary много", EditAsk{Tool: ToolVary, Places: []string{"1", "2", "3", "4", "5", "6", "7"}}, "Другие раскладки на 7 разворотах?", "Изменить раскладки", true},
		{"vary лист", EditAsk{Tool: ToolVary, Places: []string{"2"}, Sheet: true}, "Другие раскладки на листе 2?", "Изменить раскладки", true},
		{"vary пусто", EditAsk{Tool: ToolVary}, "Другие раскладки на выбранных разворотах?", "Изменить раскладки", true},
		{"show", EditAsk{Tool: ToolShow, Places: []string{"8"}}, "Показать разворот 8?", "Показать", false},
		{"show обложка", EditAsk{Tool: ToolShow, Places: []string{CoverWord}}, "Показать обложку?", "Показать", false},
		{"undo", EditAsk{Tool: ToolUndo}, "Отменить последнюю правку агента?", "Отменить правку", false},
		{"marks", EditAsk{Tool: ToolMarks, Count: 12}, "Изменить отметки у 12 фото?", "Изменить отметки", true},
		{"marks 0", EditAsk{Tool: ToolMarks}, "Изменить отметки у выбранных фото?", "Изменить отметки", true},
		{"cover", EditAsk{Tool: ToolCover, Count: 1}, "Поставить другое фото на обложку?", "Поставить", true},
		{"bg", EditAsk{Tool: ToolBg, Item: "Цветы", Places: []string{"3", "5"}}, "Поставить фон «Цветы» на разворотах 3, 5?", "Поставить фон", true},
		{"bg без названия", EditAsk{Tool: ToolBg, Places: []string{CoverWord, "2"}}, "Поставить выбранный фон на обложке и развороте 2?", "Поставить фон", true},
		{"sticker", EditAsk{Tool: ToolSticker, Count: 3, Places: []string{"2", "5"}}, "Добавить 3 стикера на разворотах 2, 5?", "Добавить", true},
		{"sticker 12", EditAsk{Tool: ToolSticker, Count: 12, Places: []string{"1"}}, "Добавить 12 стикеров на развороте 1?", "Добавить", true},
		{"sticker 0", EditAsk{Tool: ToolSticker}, "Добавить стикеры на выбранных разворотах?", "Добавить", true},
		{"print", EditAsk{Tool: ToolPrint, Item: "Фабрика", Format: "30×20"}, "Поставить формат «Фабрика 30×20»?", "Применить формат", true},
		{"print без названия", EditAsk{Tool: ToolPrint}, "Поставить формат выбранной типографии?", "Применить формат", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			c.in.Client = "Claude Code"
			b, ok := EditBody(c.in)
			if !ok || b.Title != c.title || b.OK != c.ok {
				t.Fatalf("заголовок %q (%v), кнопка %q; ждали %q, %q", b.Title, ok, b.OK, c.title, c.ok)
			}
			if len(b.Lines) < 2 || !strings.HasPrefix(b.Lines[0], "Клиент «Claude Code»") || b.Lines[len(b.Lines)-1] != Who {
				t.Errorf("абзацы: %q", b.Lines)
			}
			if has := strings.Contains(strings.Join(b.Lines, "\n"), undoLine); has != c.undo {
				t.Errorf("строка про Ctrl+Z: есть %v, ждали %v", has, c.undo)
			}
		})
	}
	// Клиент с плохим именем — «Неизвестный клиент», своё слово агента в окно не идёт.
	b, _ := EditBody(EditAsk{Tool: ToolShow, Client: "Sosnina Albums"})
	if !strings.HasPrefix(b.Lines[0], "Неизвестный клиент") {
		t.Errorf("подделка под программу: %q", b.Lines[0])
	}
	if _, ok := EditBody(EditAsk{Tool: "сам_придумал"}); ok {
		t.Error("шаблон для неизвестного инструмента")
	}
}

// TestEditAskAllTools: у каждого инструмента, у которого возможно «Спрашивать» (правка E и показ V),
// есть шаблон; шаблон не берёт чужих (папка, альбом, чтение).
func TestEditAskAllTools(t *testing.T) {
	have := map[string]bool{}
	for _, tool := range EditTools() {
		have[tool] = true
		if _, ok := EditBody(EditAsk{Tool: tool}); !ok {
			t.Errorf("%s: шаблон перечислен, но текста нет", tool)
		}
	}
	for _, tool := range prefs.AgentToolNames() {
		c := prefs.AgentToolClass(tool)
		asks := c == prefs.ClassEdit || c == prefs.ClassShow // F и A спрашивают своими видами (add, create)
		if asks && !have[tool] {
			t.Errorf("%s (класс %s): «Спрашивать» возможно, шаблона вопроса нет", tool, c)
		}
		if !asks && have[tool] {
			t.Errorf("%s (класс %s): шаблон есть, а вопроса о правке не бывает", tool, c)
		}
		if asks {
			if _, ok := EditBody(EditAsk{Tool: tool}); !ok {
				t.Errorf("%s: нет текста", tool)
			}
		}
	}
}

// TestAlbumBgLines: строки о своих фонах листов при фоне альбома — точный список; лист/разворот, один и
// несколько, пустые входы.
func TestAlbumBgLines(t *testing.T) {
	cases := []struct {
		name string
		in   EditAsk
		want []string
	}{
		{"лист: несколько и один", EditAsk{Sheet: true, OwnCat: []int{3, 7}, TakeClient: []int{5}, Photos: 2},
			[]string{"Свои фоны из каталога уйдут с листов: 3, 7", "Фото фона уйдут с листа 5: 2 фото"}},
		{"разворот: один свой фон", EditAsk{OwnCat: []int{1}}, []string{"Свои фоны из каталога уйдут с разворота: 1"}},
		{"оставить фото клиента", EditAsk{KeepClient: []int{2, 4}},
			[]string{"Фото клиента на фоне останутся на разворотах: 2, 4"}},
		{"лист: оставить", EditAsk{Sheet: true, KeepClient: []int{2}}, []string{"Фото клиента на фоне останутся на листах: 2"}},
		{"все три строки", EditAsk{OwnCat: []int{1, 2}, KeepClient: []int{3}, TakeClient: []int{4, 6}, Photos: 9},
			[]string{"Свои фоны из каталога уйдут с разворотов: 1, 2", "Фото клиента на фоне останутся на разворотах: 3",
				"Фото фона уйдут с разворотов 4, 6: 9 фото"}},
		{"пусто", EditAsk{Photos: 5}, nil},
	}
	for _, c := range cases {
		if got := c.in.albumBgLines(); !slices.Equal(got, c.want) {
			t.Errorf("%s: %q, ждали %q", c.name, got, c.want)
		}
	}
}
