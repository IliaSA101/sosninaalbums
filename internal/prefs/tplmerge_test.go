package prefs

import (
	"fmt"
	"reflect"
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// seqID — предсказуемые новые ID: проверка, что ID сменился именно у совпавшего.
func seqID() func() string {
	n := 0
	return func() string { n++; return fmt.Sprintf("new%d", n) }
}

// gridTpl — сетка с местом, сдвинутым на i: у разных i < 6400 разное содержимое.
func gridTpl(id, name string, i int) model.Template {
	return model.Template{ID: id, Name: name, Scope: model.TplPage,
		Boxes: []model.Box{{X: float64(i%80) / 100, Y: float64(i/80%80) / 100, W: 0.1, H: 0.2}}}
}

// Тот же шаблон под другим именем и ID — «уже был»; в памяти доли округлены записью, в чужом
// файле — нет (хвост ниже 1e-5): всё равно тот же. cur не меняется.
func TestMergeTemplatesSameContent(t *testing.T) {
	cur := model.RoundTemplates([]model.Template{gridTpl("a1", "Мой", 3), gridTpl("a2", "Второй", 4)})
	keep := slices.Clone(cur)
	same := gridTpl("zz", "Переименованный", 3)
	same.Boxes[0].X += 1e-7
	other := gridTpl("b1", "Новый", 5)
	out, res := MergeTemplates(cur, []model.Template{same, other}, seqID())
	if res != (MergeResult{Added: 1, Same: 1}) {
		t.Fatalf("итог %+v", res)
	}
	if len(out) != 3 || out[2].ID != "b1" || out[2].Name != "Новый" {
		t.Fatalf("список %+v", out)
	}
	if !reflect.DeepEqual(cur, keep) {
		t.Fatal("cur изменён")
	}
	// Пустой ввод и пустой список.
	if out, res := MergeTemplates(cur, nil, seqID()); res != (MergeResult{}) || len(out) != 2 {
		t.Fatalf("пустой add: %+v %d", res, len(out))
	}
	if out, res := MergeTemplates(nil, []model.Template{other}, seqID()); res.Added != 1 || len(out) != 1 {
		t.Fatalf("пустой cur: %+v %d", res, len(out))
	}
}

// Совпал только ID — шаблон добавлен с новым ID, имя прежнее; ID, занятый ранее добавленным из
// того же файла, — тоже новый.
func TestMergeTemplatesIDClash(t *testing.T) {
	cur := []model.Template{gridTpl("a1", "Мой", 3)}
	add := []model.Template{gridTpl("a1", "Чужой", 7), gridTpl("a1", "Ещё", 8), gridTpl("", "Без ID", 9)}
	out, res := MergeTemplates(cur, add, seqID())
	if res != (MergeResult{Added: 3}) {
		t.Fatalf("итог %+v", res)
	}
	ids := []string{out[0].ID, out[1].ID, out[2].ID, out[3].ID}
	if !slices.Equal(ids, []string{"a1", "new1", "new2", "new3"}) {
		t.Fatalf("ID %v", ids)
	}
	if out[1].Name != "Чужой" || out[2].Name != "Ещё" {
		t.Fatalf("имена %q %q", out[1].Name, out[2].Name)
	}
}

// 190 своих + 15 из файла → 10 добавлено, 5 не вошли; совпавший сверх предела — «уже был».
func TestMergeTemplatesLimit(t *testing.T) {
	var cur, add []model.Template
	for i := range 190 {
		cur = append(cur, gridTpl(fmt.Sprintf("c%d", i), "", i))
	}
	for i := range 15 {
		add = append(add, gridTpl(fmt.Sprintf("f%d", i), "", 1000+i))
	}
	out, res := MergeTemplates(cur, add, seqID())
	if res != (MergeResult{Added: 10, Over: 5}) || len(out) != model.MaxTemplates {
		t.Fatalf("итог %+v, шаблонов %d", res, len(out))
	}
	if out[190].ID != "f0" || out[199].ID != "f9" {
		t.Fatalf("порядок: %s … %s", out[190].ID, out[199].ID)
	}
	// Полный список: новый — не вошёл, тот же — уже был.
	_, res = MergeTemplates(out, []model.Template{gridTpl("x", "", 2000), out[5]}, seqID())
	if res != (MergeResult{Same: 1, Over: 1}) {
		t.Fatalf("полный: %+v", res)
	}
}

// Шаблон виньетки с ролями, шапкой и стилем — добавлен целиком; тот же с другим стилем — другой.
func TestMergeTemplatesVignette(t *testing.T) {
	vig := bigVigTemplates(2, 20)
	vig = model.ClampTemplates(vig)
	out, res := MergeTemplates([]model.Template{gridTpl("a1", "Мой", 3)}, vig[:1], seqID())
	if res.Added != 1 || !reflect.DeepEqual(out[1], vig[0]) {
		t.Fatalf("виньетка: %+v\n%+v\n%+v", res, out[1], vig[0])
	}
	restyled := vig[0]
	vv := *restyled.Vig
	vv.Style.TText = "Классный руководитель"
	restyled.Vig, restyled.ID = &vv, model.NewID()
	_, res = MergeTemplates(out, []model.Template{vig[0], restyled}, seqID())
	if res != (MergeResult{Added: 1, Same: 1}) {
		t.Fatalf("стиль виньетки — не содержимое: %+v", res)
	}
}

// Два одинаковых в файле — добавлен один.
func TestMergeTemplatesDupInFile(t *testing.T) {
	a, b := gridTpl("f1", "Раз", 3), gridTpl("f2", "Два", 3)
	out, res := MergeTemplates(nil, []model.Template{a, b, gridTpl("f3", "Три", 4)}, seqID())
	if res != (MergeResult{Added: 2, Same: 1}) || len(out) != 2 || out[0].ID != "f1" || out[1].ID != "f3" {
		t.Fatalf("итог %+v, %+v", res, out)
	}
}
