package names_test

import (
	"strings"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/names"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Чужое фото и чужой разворот — холостые правки: ни шага отмены, ни rev.
func TestNamesUnknownIDsNoop(t *testing.T) {
	e, _, _ := edtest.NewTwo(t)
	undo, rev := edtest.UndoLen(e), edtest.Rev(e)
	e.SetPerson("нет", model.Person{Last: "Иванов"}, "p")
	e.SetSheetCaps("нет", true, model.SheetCaps{Style: model.DefaultCaptionStyle()})
	e.SetSheetCaps("нет", false, model.SheetCaps{})
	if edtest.UndoLen(e) != undo || edtest.Rev(e) != rev {
		t.Fatalf("шагов %d, rev %d → %d", edtest.UndoLen(e)-undo, rev, edtest.Rev(e))
	}
}

// Фото листа, которого нет в списке фото проекта (битый файл), вставка пропускает без паники
// и в «без имени» не считает.
func TestPasteSkipsMissingPhoto(t *testing.T) {
	p := model.New()
	p.Spreads = []*model.Spread{{ID: "s", Photos: []string{"ghost", "a"}}}
	p.Photos["a"] = model.PhotoRef{Path: `D:\x\DSC_0001.jpg`}
	r, err := names.Read(model.NamesOpts{Text: "Иванов\nПетров\nСидоров"})
	if err != nil {
		t.Fatal(err)
	}
	plan, err := names.Plan(p, "s", r)
	if err != nil || plan.Extra != 1 || plan.Missing != 0 {
		t.Fatalf("лишних %d, без имени %d, err %v", plan.Extra, plan.Missing, err)
	}
	names.Apply(p, "s", plan, true)
	if got := p.Photos["a"].Person; got == nil || got.Last != "Петров" {
		t.Fatalf("имя по индексу в ленте: %+v", got)
	}
	if _, ok := p.Photos["ghost"]; ok {
		t.Fatal("чужое фото появилось в проекте")
	}
	if _, err := names.Plan(p, "нет", r); err == nil {
		t.Fatal("чужой разворот принят")
	}
}

// Заголовок из 10^5 столбцов (поле до 256 КБ вмещает): Cols и Ignored не длиннее 64, разбор
// быстрый (раньше на каждую ячейку строился новый Replacer), роли за 64-м столбцом не действуют.
func TestHeaderColumnsCapped(t *testing.T) {
	const tail = "\tx"
	for name, head := range map[string]string{
		"Tab":             "Фамилия\tИмя" + strings.Repeat(tail, 100000),
		"роль за 64":      "Фамилия\tИмя" + strings.Repeat(tail, 70) + "\tПредмет" + strings.Repeat(tail, 100000),
		"точка с запятой": "Фамилия;Имя" + strings.Repeat(";x", 100000),
	} {
		start := time.Now()
		// Строка данных делится разделителем заголовка.
		row := "Иванов\tПётр\tучитель"
		if !strings.Contains(head, "\t") {
			row = "Иванов;Пётр;учитель"
		}
		r, err := names.Read(model.NamesOpts{Text: head + "\n" + row})
		if d := time.Since(start); d > testkit.Limit(time.Second) {
			t.Errorf("%s: разбор %v", name, d)
		}
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if len(r.Cols) > 64 || len(r.Ignored) > 64 || len(r.Cols) == 0 {
			t.Errorf("%s: Cols %d, Ignored %d", name, len(r.Cols), len(r.Ignored))
		}
		for _, c := range r.Cols {
			if c == "Предмет" {
				t.Errorf("%s: роль за пределом столбцов действует", name)
			}
		}
		if len(r.People) != 1 || r.People[0].Who.Last != "Иванов" || r.People[0].Teacher {
			t.Errorf("%s: люди %+v", name, r.People)
		}
	}
	// Ровно 64 столбца — все на месте: предел не режет обычные таблицы.
	r, err := names.Read(model.NamesOpts{Text: "Фамилия\tИмя" + strings.Repeat(tail, 62) + "\nИванов\tПётр"})
	if err != nil || len(r.Cols) != 64 || len(r.Ignored) != 62 {
		t.Fatalf("64 столбца: %v, Cols %d, Ignored %d", err, len(r.Cols), len(r.Ignored))
	}
}

// Предел поля списка один на весь код: App режет текст таблицы по model.MaxNamesBytes, а Read
// — по names.MaxPasteBytes; расхождение пропускало бы в окно текст, который Read отклонит.
func TestPasteLimitShared(t *testing.T) {
	if names.MaxPasteBytes != model.MaxNamesBytes {
		t.Fatalf("names.MaxPasteBytes %d ≠ model.MaxNamesBytes %d", names.MaxPasteBytes, model.MaxNamesBytes)
	}
	if _, err := names.Read(model.NamesOpts{Text: strings.Repeat("я", model.MaxNamesBytes/2+1)}); err == nil {
		t.Fatal("текст больше model.MaxNamesBytes принят")
	}
}
