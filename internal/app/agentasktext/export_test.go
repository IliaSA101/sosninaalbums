package agentasktext

import (
	"strings"
	"testing"
)

// TestExportQuestionLines: заголовок и строки вопроса — путь, создание, соседняя папка, облако, сеть, «заменится»,
// чужие файлы (тогда «Выбрать папку…» и без «Выбрать другую…»); имя папки с плохими знаками в заголовок не идёт.
func TestExportQuestionLines(t *testing.T) {
	sum := []string{"Развороты 1–24 и обложка · 300 dpi · качество 95 · 25 файлов", "Обычная выгрузка"}
	in := func(p ExportDir) Body {
		return ExportText(ExportIn{Client: "Claude Code", Album: "Петровы", Lines: sum, Dir: p})
	}
	b := in(ExportDir{Shown: "/v/Петровы", Create: true, NextTo: true, Net: true, Cloud: true, Ours: 2})
	all := strings.Join(b.Lines, "\n")
	for _, want := range []string{"Клиент «Claude Code» просит выгрузить альбом «Петровы» в JPEG:", sum[0], "Папка будет создана.",
		"Рядом с папкой фото альбома", "Сетевой диск.", "синхронизируется с облаком", "уже есть выгрузка (2 файла) — заменится", Who} {
		if !strings.Contains(all, want) {
			t.Errorf("нет строки %q в %q", want, all)
		}
	}
	if b.Title != "Выгрузить альбом в папку «Петровы»?" || b.OK != OKExport {
		t.Errorf("заголовок %q, кнопка %q", b.Title, b.OK)
	}
	f := in(ExportDir{Shown: "/v/Петровы", Foreign: true})
	if f.OK != OKExportPick || !strings.Contains(strings.Join(f.Lines, "\n"), "Сюда выгрузить нельзя") {
		t.Errorf("чужие файлы: %+v", f)
	}
	n := in(ExportDir{})
	if n.Title != "Выгрузить альбом по просьбе агента?" || n.OK != OKExportPick || !strings.Contains(strings.Join(n.Lines, "\n"), "Папку выберете вы") {
		t.Errorf("без пути: %+v", n)
	}
	if bad := in(ExportDir{Shown: "/v/a\u202eb"}); strings.Contains(bad.Title, "a") {
		t.Errorf("плохое имя в заголовке: %q", bad.Title)
	}
	for n, w := range map[int]string{1: "файл", 2: "файла", 5: "файлов", 11: "файлов", 21: "файл", 112: "файлов"} {
		if filesWord(n) != w {
			t.Errorf("filesWord(%d) = %q", n, filesWord(n))
		}
	}
}
