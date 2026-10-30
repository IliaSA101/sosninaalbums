package app

import (
	"strings"
	"testing"
)

// Тексты вопроса: заголовок и кнопка из шаблона, имя клиента в «…», судьба открытого альбома
// по состоянию, последняя строка про «Отмена»; чужие знаки и подделка под программу не проходят.
func TestAskText(t *testing.T) {
	b, ok := askText(askKindCreate, "Claude Desktop", "Альбом Петровых", askDocSaved)
	if !ok || b.Title != "Создать альбом по просьбе агента?" || b.OK != "Выбрать папку…" {
		t.Fatalf("create: %+v", b)
	}
	if !strings.Contains(b.Lines[0], "«Claude Desktop»") {
		t.Errorf("клиент не в «…»: %q", b.Lines[0])
	}
	if b.Lines[1] != "Открытый «Альбом Петровых» сохранится в свой файл." {
		t.Errorf("saved: %q", b.Lines[1])
	}
	if last := b.Lines[len(b.Lines)-1]; last != "Агент называет себя сам: если вы ничего не просили — «Отмена»." {
		t.Errorf("последняя строка: %q", last)
	}
	for _, l := range b.Lines {
		if strings.Contains(l, "предлагает папку") {
			t.Errorf("фраза про подсказку принадлежит интерфейсу: %q", l)
		}
	}

	for _, c := range []struct {
		album string
		doc   askDoc
		want  string
	}{
		{"Черновик", askDocDraft, "Несохранённый черновик «Черновик» уйдёт в резервные копии."},
		{"", askDocDraft, "Несохранённый черновик уйдёт в резервные копии."},
		{"", askDocEmpty, "Пустой альбом закроется."},
		{"x", askDocEmpty, "Пустой альбом закроется."},
		{"a\u202Eb", askDocSaved, "Открытый «без названия» сохранится в свой файл."},
		{strings.Repeat("я", 90), askDocSaved, "Открытый «" + strings.Repeat("я", 59) + "…» сохранится в свой файл."},
	} {
		b, _ := askText(askKindCreate, "Клиент", c.album, c.doc)
		if b.Lines[1] != c.want {
			t.Errorf("%q/%d: %q, ждали %q", c.album, c.doc, b.Lines[1], c.want)
		}
	}

	add, ok := askText(askKindAdd, "Cursor", "Свадьба", askDocSaved)
	if !ok || add.Title == b.Title || add.OK != "Выбрать папку…" || len(add.Lines) < 2 {
		t.Fatalf("add: %+v", add)
	}
	if !strings.Contains(add.Lines[0], "«Свадьба»") || !strings.Contains(add.Lines[0], "«Cursor»") {
		t.Errorf("add: %q", add.Lines[0])
	}
	if _, ok := askText("delete", "x", "y", askDocSaved); ok {
		t.Error("неизвестный вид принят")
	}
}

// Имя клиента в тексте: чужие знаки, пустое, длинное и похожее на программу — «неизвестный клиент».
func TestAskClientName(t *testing.T) {
	for in, want := range map[string]string{
		"Claude Desktop": "Claude Desktop", "  Cursor ": "Cursor", "": askUnknown,
		"Sosnina Albums": askUnknown, "ПРОГРАММА СОСНИНА": askUnknown, "Сосн": askUnknown, "sOsNiNa": askUnknown,
		"Sοsnina": askUnknown, "S0snina": askUnknown, "Sosnína": askUnknown, "Ｓｏｓｎｉｎａ": askUnknown, "Sos nina": askUnknown,
		"a\u2028b": askUnknown, "a\u3164b": askUnknown, "a\u202Eb": askUnknown, "\xff": askUnknown,
		strings.Repeat("я", 61): askUnknown, strings.Repeat("я", 60): strings.Repeat("я", 60),
	} {
		if got := clientName(in); got != want {
			t.Errorf("clientName(%q) = %q, ждали %q", in, got, want)
		}
	}
	b, _ := askText(askKindAdd, "Sosnina Albums", "A", askDocSaved)
	if !strings.HasPrefix(b.Lines[0], "Неизвестный клиент просит") || strings.Contains(b.Lines[0], "Sosnina") {
		t.Errorf("подделка под программу: %q", b.Lines[0])
	}
	if got := askDialogTitle("Claude"); got != "Папка с фотографиями — просит Claude" {
		t.Errorf("заголовок окна ОС: %q", got)
	}
	if got := askDialogTitle("a\u202Eb"); !strings.HasSuffix(got, askUnknown) {
		t.Errorf("заголовок окна ОС с чужими знаками: %q", got)
	}
	if !strings.HasPrefix(askFolderReject, "Выберите папку съёмки") {
		t.Errorf("тост отказа: %q", askFolderReject)
	}
}
