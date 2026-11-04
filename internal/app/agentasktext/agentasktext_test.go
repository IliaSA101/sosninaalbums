package agentasktext

import (
	"strings"
	"testing"
)

// Тексты вопроса: заголовок и кнопка из шаблона, имя клиента в «…», судьба открытого альбома
// по состоянию, последняя строка про «Отмена»; чужие знаки и подделка под программу не проходят.
func TestAskText(t *testing.T) {
	b, ok := Text(KindCreate, "Claude Desktop", "Альбом Петровых", DocSaved)
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
		doc   Doc
		want  string
	}{
		{"Черновик", DocDraft, "Несохранённый черновик «Черновик» уйдёт в резервные копии."},
		{"", DocDraft, "Несохранённый черновик уйдёт в резервные копии."},
		{"", DocEmpty, "Пустой альбом закроется."},
		{"x", DocEmpty, "Пустой альбом закроется."},
		{"a\u202Eb", DocSaved, "Открытый «без названия» сохранится в свой файл."},
		{strings.Repeat("я", 90), DocSaved, "Открытый «" + strings.Repeat("я", 59) + "…» сохранится в свой файл."},
	} {
		b, _ := Text(KindCreate, "Клиент", c.album, c.doc)
		if b.Lines[1] != c.want {
			t.Errorf("%q/%d: %q, ждали %q", c.album, c.doc, b.Lines[1], c.want)
		}
	}

	add, ok := Text(KindAdd, "Cursor", "Свадьба", DocSaved)
	if !ok || add.Title == b.Title || add.OK != "Выбрать папку…" || len(add.Lines) < 2 {
		t.Fatalf("add: %+v", add)
	}
	if !strings.Contains(add.Lines[0], "«Свадьба»") || !strings.Contains(add.Lines[0], "«Cursor»") {
		t.Errorf("add: %q", add.Lines[0])
	}
	if _, ok := Text("delete", "x", "y", DocSaved); ok {
		t.Error("неизвестный вид принят")
	}
}

// Имя клиента в тексте: чужие знаки, пустое, длинное и похожее на программу — «неизвестный клиент».
func TestAskClientName(t *testing.T) {
	for in, want := range map[string]string{
		"Claude Desktop": "Claude Desktop", "  Cursor ": "Cursor", "": unknown,
		"Sosnina Albums": unknown, "ПРОГРАММА СОСНИНА": unknown, "Сосн": unknown, "sOsNiNa": unknown,
		"Sοsnina": unknown, "S0snina": unknown, "Sosnína": unknown, "Ｓｏｓｎｉｎａ": unknown, "Sos nina": unknown,
		"a\u2028b": unknown, "a\u3164b": unknown, "a\u202Eb": unknown, "\xff": unknown,
		strings.Repeat("я", 61): unknown, strings.Repeat("я", 60): strings.Repeat("я", 60),
	} {
		if got := ClientName(in); got != want {
			t.Errorf("ClientName(%q) = %q, ждали %q", in, got, want)
		}
	}
	b, _ := Text(KindAdd, "Sosnina Albums", "A", DocSaved)
	if !strings.HasPrefix(b.Lines[0], "Неизвестный клиент просит") || strings.Contains(b.Lines[0], "Sosnina") {
		t.Errorf("подделка под программу: %q", b.Lines[0])
	}
	if got := DialogTitle("Claude"); got != "Папка с фотографиями — просит Claude" {
		t.Errorf("заголовок окна ОС: %q", got)
	}
	if got := DialogTitle("a\u202Eb"); !strings.HasSuffix(got, unknown) {
		t.Errorf("заголовок окна ОС с чужими знаками: %q", got)
	}
	if !strings.HasPrefix(FolderReject, "Выберите папку съёмки") {
		t.Errorf("тост отказа: %q", FolderReject)
	}
}
