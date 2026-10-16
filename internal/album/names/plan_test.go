package names_test

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/names"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// vigEd — лист-виньетка head-t-left (1 учитель, 3 ученика), места заполнены фото по порядку
// сетки; teacher — фото на месте учителя, pupils — на местах учеников.
func vigEd(t *testing.T) (e *album.Editor, sid, teacher string, pupils []string) {
	t.Helper()
	e = edtest.Open(t, album.NewVignetteSheet(204, 304), "")
	sid = edtest.P(e).Spreads[0].ID
	if _, _, err := e.SetVignette(sid, model.Vignette{Tpl: "head-t-left", Teachers: 1, Pupils: 3}, nil); err != nil {
		t.Fatal(err)
	}
	e.FillPlaces(edtest.Photos("v1", "v2", "v3", "v4"), sid)
	for _, pl := range edtest.P(e).Spreads[0].Grid {
		switch model.RoleOf(pl) {
		case model.RoleTeacher:
			teacher = pl.Photo
		case model.RolePupil:
			pupils = append(pupils, pl.Photo)
		}
	}
	if teacher == "" || len(pupils) != 3 {
		t.Fatalf("виньетка: учитель %q, ученики %v", teacher, pupils)
	}
	return e, sid, teacher, pupils
}

// fileEd — обычный разворот из фото с именами файлов files (ID p1, p2, … по порядку).
func fileEd(t *testing.T, files ...string) (*album.Editor, string) {
	t.Helper()
	e := edtest.Open(t, model.New(), "")
	ph := make([]model.PhotoInfo, len(files))
	for i, f := range files {
		ph[i] = model.PhotoInfo{ID: fmt.Sprintf("p%d", i+1), Path: edtest.Abs(f), W: 300, H: 200}
	}
	return e, edtest.NewSpread(e, -1, ph).Spreads[0].ID
}

func person(e *album.Editor, id string) string {
	if who := edtest.P(e).Photos[id].Person; who != nil {
		return strings.TrimSpace(who.Last + " " + who.First + " " + who.Middle)
	}
	return ""
}

// Имя из названия файла и строка списка вместе: Named считает оба, Missing — места без имени.
func TestPasteNamesFileNameAndList(t *testing.T) {
	e, sid := fileEd(t, "a.jpg", "Иванов Петя.jpg", "c.jpg")
	_, plan, err := e.PasteNames(sid, model.NamesOpts{Text: "Сидоров Иван"})
	if err != nil || plan.Seats != 3 || plan.Named != 2 || plan.Missing != 1 || plan.Extra != 0 {
		t.Fatalf("вставка: места %d, с именем %d, без %d, лишних %d, %v", plan.Seats, plan.Named, plan.Missing, plan.Extra, err)
	}
	// Список лёг на первое место; имя второго фото остаётся в названии файла, своего не пишется.
	if person(e, "p1") != "Сидоров Иван" || person(e, "p2") != "" || person(e, "p3") != "" {
		t.Fatalf("имена: %q %q %q", person(e, "p1"), person(e, "p2"), person(e, "p3"))
	}
	// Строка списка ложится на первое место и вытесняет имя из названия файла: с именем одно.
	e1, sid1 := fileEd(t, "Иванов Петя.jpg", "b.jpg", "c.jpg")
	if _, plan, _ = e1.PasteNames(sid1, model.NamesOpts{Text: "Сидоров Иван"}); plan.Named != 1 || plan.Missing != 2 {
		t.Fatalf("список на первом месте: с именем %d, без %d", plan.Named, plan.Missing)
	}
	// Пустой список: именовано только фото с именем в названии файла, правки нет (холостая).
	e2, sid2 := fileEd(t, "a.jpg", "Иванов Петя.jpg", "c.jpg")
	rev := edtest.Rev(e2)
	if _, plan, _ = e2.PasteNames(sid2, model.NamesOpts{Text: ""}); plan.Named != 1 || plan.Missing != 2 || edtest.Rev(e2) != rev {
		t.Fatalf("пустой список: с именем %d, без %d, rev %d → %d", plan.Named, plan.Missing, rev, edtest.Rev(e2))
	}
}

// Виньетка: учитель первой строкой таблицы ложится на место учителя (у head-t-left оно
// последнее), ученики — на места учеников; лишние учителя — noseat, лишние ученики — extra.
func TestPasteNamesRoles(t *testing.T) {
	e, sid, teacher, pupils := vigEd(t)
	text := "Фамилия\tИмя\tПредмет\nСидорова\tОльга\tМатематика\nКозлов\tИван\nОрлов\tОлег\nПетрова\tАнна\n" +
		"Лишний\tУченик\nВторая\tУчительница\tФизика"
	_, plan, err := e.PasteNames(sid, model.NamesOpts{Text: text})
	if err != nil {
		t.Fatal(err)
	}
	if person(e, teacher) != "Сидорова Ольга" || person(e, pupils[0]) != "Козлов Иван" || person(e, pupils[2]) != "Петрова Анна" {
		t.Fatalf("учитель %q, ученики %q %q", person(e, teacher), person(e, pupils[0]), person(e, pupils[2]))
	}
	if plan.Teachers != 2 || plan.Pupils != 4 || plan.Seats != 4 || plan.Named != 4 || plan.Missing != 0 || plan.Extra != 2 || plan.NeedCaption {
		t.Fatalf("итог: %+v", plan)
	}
	wantLeft := []model.NamesLeft{
		{Line: 6, Who: model.Person{Last: "Лишний", First: "Ученик"}, Reason: "extra"},
		{Line: 7, Who: model.Person{Last: "Вторая", First: "Учительница", Subject: "Физика"}, Reason: "noseat"},
	}
	if !reflect.DeepEqual(plan.Left, wantLeft) {
		t.Fatalf("строки без фото: %+v", plan.Left)
	}
	tags := map[string]string{}
	for _, row := range plan.Rows {
		tags[row.Photo] = fmt.Sprintf("%s/%d/%d", row.Tag, row.No, row.Line)
	}
	if tags[teacher] != "учитель/4/2" || tags[pupils[0]] != "ученик 1/1/3" || tags[pupils[2]] != "ученик 3/3/5" {
		t.Fatalf("метки мест: %v", tags)
	}
	// Без учителей в таблице — по порядку мест, как в 030: учитель получает 4-ю строку.
	e, sid, teacher, pupils = vigEd(t)
	if _, plan, err = e.PasteNames(sid, model.NamesOpts{Text: "А\nБ\nВ\nГ\nД"}); err != nil || plan.Extra != 1 {
		t.Fatalf("по порядку: %+v, %v", plan, err)
	}
	if person(e, pupils[0]) != "А" || person(e, teacher) != "Г" {
		t.Fatalf("по порядку: ученик %q, учитель %q", person(e, pupils[0]), person(e, teacher))
	}
	// Учителей больше мест учителя, учеников нет: ученики имён не получают, лишний — noseat.
	e, sid, teacher, pupils = vigEd(t)
	_, plan, _ = e.PasteNames(sid, model.NamesOpts{Text: "ФИО\tПредмет\nИванова Мария\tПение\nПетрова Ольга\tИЗО"})
	if person(e, teacher) != "Иванова Мария" || person(e, pupils[0]) != "" || plan.Extra != 1 || plan.Left[0].Reason != "noseat" || plan.Missing != 3 {
		t.Fatalf("учителя: %+v", plan)
	}
}

// По столбцу «Файл»: с расширением и без, регистр, путь, число по последней группе цифр;
// неоднозначное число, нет фото, повтор, пустая ячейка — в Left со своей причиной.
// Фото без пары своё имя не теряют.
func TestPasteNamesByFile(t *testing.T) {
	e, sid := fileEd(t, "IMG_0012.jpg", "IMG_0014.JPG", "Портрет.jpg", "DSC_0020.jpg", "IMG_0020.jpg")
	e.SetPerson("p3", model.Person{Last: "Своё", First: "Имя"}, "")
	text := "Фото\tФамилия\tИмя\nimg_0012.jpg\tИванов\tПётр\n14\tОрлов\tОлег\n20\tДвое\tФото\n99\tЛишний\tЧеловек\n" +
		"IMG_0014\tПовтор\tСтроки\n\tБез\tФайла\nC:\\x\\DSC_0020.JPG\tКозлов\tИван\n0012\tНоль\tДвенадцать"
	_, plan, err := e.PasteNames(sid, model.NamesOpts{Text: text})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, id := range []string{"p1", "p2", "p3", "p4", "p5"} {
		got[id] = person(e, id)
	}
	want := map[string]string{"p1": "Иванов Пётр", "p2": "Орлов Олег", "p3": "Своё Имя", "p4": "Козлов Иван", "p5": ""}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("имена: %v", got)
	}
	reasons := []string{}
	for _, l := range plan.Left {
		reasons = append(reasons, fmt.Sprintf("%d:%s:%s", l.Line, l.Reason, l.File))
	}
	wantReasons := []string{"4:many:20", "5:nofile:99", "6:dup:IMG_0014", "7:nofile:", "9:dup:0012"}
	if !reflect.DeepEqual(reasons, wantReasons) || !plan.ByFile || plan.Extra != 5 {
		t.Fatalf("строки без фото: %v", reasons)
	}
	if plan.Seats != 5 || plan.Named != 4 || plan.Missing != 1 {
		t.Fatalf("итог: подписано %d из %d, без имени %d", plan.Named, plan.Seats, plan.Missing)
	}
	// Число найдёт фото, только если оно одно: «12» — IMG_0012, а «7» — ничего.
	e, sid = fileEd(t, "IMG_0012.jpg", "Фото 7 (2).jpg")
	_, plan, _ = e.PasteNames(sid, model.NamesOpts{Text: "Файл\tФИО\n12\tИванов Петя\n2\tПетров Вася\n7\tНет Такого"})
	if person(e, "p1") != "Иванов Петя" || person(e, "p2") != "Петров Вася" || plan.Extra != 1 || plan.Left[0].Reason != "nofile" {
		t.Fatalf("число: p1 %q, p2 %q, %+v", person(e, "p1"), person(e, "p2"), plan.Left)
	}
}

// previewThenPaste — предпросмотр без rev и шага, план вставки равен ему, на фото легло
// показанное (Who; nil — прежнее имя), «подписано» сходится с именами после вставки.
func previewThenPaste(t *testing.T, name string, e *album.Editor, sid string, o model.NamesOpts) {
	t.Helper()
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	pre, err := e.PreviewNames(sid, o)
	if err != nil || edtest.Rev(e) != rev || edtest.UndoLen(e) != undo {
		t.Fatalf("%s: предпросмотр менял проект: %v, rev %d → %d", name, err, rev, edtest.Rev(e))
	}
	before := map[string]*model.Person{}
	for _, row := range pre.Rows {
		before[row.Photo] = edtest.P(e).Photos[row.Photo].Person
	}
	_, plan, err := e.PasteNames(sid, o)
	if err != nil || !reflect.DeepEqual(pre, plan) {
		t.Fatalf("%s: план вставки отличается от предпросмотра:\n%+v\n%+v", name, pre, plan)
	}
	named := 0
	for _, row := range pre.Rows {
		r := edtest.P(e).Photos[row.Photo]
		want := row.Who
		if want == nil {
			want = before[row.Photo]
		}
		if !reflect.DeepEqual(r.Person, want) {
			t.Fatalf("%s: %s получило %+v, предпросмотр %+v", name, row.Photo, r.Person, row.Who)
		}
		if who, _ := r.Name(); who != nil && row.No > 0 {
			named++
		}
	}
	if named != pre.Named {
		t.Fatalf("%s: подписано %d, предпросмотр %d", name, named, pre.Named)
	}
}

// Предпросмотр не меняет ни rev, ни отмену; что он показал, то и ложится (сц. 4–6).
func TestPreviewNamesMatchesPaste(t *testing.T) {
	type cas struct {
		name string
		ed   func() (*album.Editor, string)
		o    model.NamesOpts
	}
	vig := func() (*album.Editor, string) { e, sid, _, _ := vigEd(t); return e, sid }
	files := func() (*album.Editor, string) {
		return fileEd(t, "01_иванов_петя.jpg", "02_Петрова_Анна (2).jpg", "DSC_0012.jpg", "IMG_0014.jpg")
	}
	cases := []cas{
		{"по порядку", files, model.NamesOpts{Text: "Сидоров Иван\nКозлов Пётр", FirstFirst: true}},
		{"по файлу", files, model.NamesOpts{Text: "Фото\tФамилия\tИмя\n14\tОрлов\tОлег\n99\tЛишний\tЧеловек"}},
		{"имена файлов", files, model.NamesOpts{From: "files"}},
		{"виньетка", vig, model.NamesOpts{Text: "Фамилия\tИмя\tПредмет\nКозлов\tИван\nСидорова\tОльга\tМатематика"}},
	}
	for _, c := range cases {
		e, sid := c.ed()
		previewThenPaste(t, c.name, e, sid, c.o)
	}
	// Режим «Имена файлов»: номер, отметка копии и регистр сняты; «DSC_0012» — без изменений.
	e, sid := files()
	pre, _ := e.PreviewNames(sid, model.NamesOpts{From: "files"})
	var who []string
	for _, row := range pre.Rows {
		if row.Who == nil {
			who = append(who, "—")
		} else {
			who = append(who, row.Who.Last+" "+row.Who.First)
		}
	}
	if strings.Join(who, ", ") != "Иванов Петя, Петрова Анна, —, —" || pre.Named != 2 || pre.Missing != 2 || !pre.NeedCaption {
		t.Fatalf("имена файлов: %v, %+v", who, pre)
	}
	// Ошибки — без правки: чужой разворот, слишком длинный список.
	rev := edtest.Rev(e)
	if _, err := e.PreviewNames("нет", model.NamesOpts{Text: "А"}); err == nil {
		t.Fatal("чужой разворот принят")
	}
	if _, _, err := e.PasteNames(sid, model.NamesOpts{Text: strings.Repeat("я", names.MaxPasteBytes+1)}); err == nil || edtest.Rev(e) != rev {
		t.Fatal("длинный список принят")
	}
}

// Вставка — один шаг отмены, повтор — холостой; исправленная строка меняет одно имя.
func TestPasteNamesUndo(t *testing.T) {
	e, sid := fileEd(t, "иванов_петя.jpg", "Петрова Анна.jpg", "c.jpg")
	text := "Сидоров Иван\nКозлов Пётр\nОрлов Олег"
	for _, o := range []model.NamesOpts{{Text: text}, {From: "files"}, {Text: text, Caption: true}} {
		undo, rev := edtest.UndoLen(e), edtest.Rev(e)
		if _, _, err := e.PasteNames(sid, o); err != nil || edtest.UndoLen(e) != undo+1 || edtest.Rev(e) == rev {
			t.Fatalf("%+v: шагов %d, %v", o, edtest.UndoLen(e)-undo, err)
		}
		undo, rev = edtest.UndoLen(e), edtest.Rev(e)
		if _, _, err := e.PasteNames(sid, o); err != nil || edtest.UndoLen(e) != undo || edtest.Rev(e) != rev {
			t.Fatalf("%+v: повтор — шагов %d, rev %d → %d", o, edtest.UndoLen(e)-undo, rev, edtest.Rev(e))
		}
	}
	// Подпись листа и имена — одним шагом: отмена снимает и то, и другое.
	if c := edtest.P(e).Spreads[0].Caps; c == nil || c.Text != names.NameCaption || person(e, "p1") != "Сидоров Иван" {
		t.Fatalf("подпись %+v, p1 %q", c, person(e, "p1"))
	}
	e.Undo()
	if edtest.P(e).Spreads[0].Caps != nil || person(e, "p1") != "Иванов Петя" {
		t.Fatalf("отмена: подпись %+v, p1 %q", edtest.P(e).Spreads[0].Caps, person(e, "p1"))
	}
	// Исправленная таблица меняет лишь изменившееся имя.
	e.Redo()
	before := model.Marshal(edtest.P(e))
	undo := edtest.UndoLen(e)
	e.PasteNames(sid, model.NamesOpts{Text: "Сидоров Иван\nКозлов Павел\nОрлов Олег"})
	if person(e, "p1") != "Сидоров Иван" || person(e, "p2") != "Козлов Павел" || person(e, "p3") != "Орлов Олег" || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("исправление: %q %q %q", person(e, "p1"), person(e, "p2"), person(e, "p3"))
	}
	e.Undo()
	if string(model.Marshal(edtest.P(e))) != string(before) {
		t.Fatal("отмена исправления не вернула проект")
	}
}

// Обычный лист без шаблона подписи: флажок «Подписать фото» ставит шаблон {Фамилия} {Имя} тем
// же шагом; свой стиль листа без шаблона сохраняется; с шаблоном и на виньетке — не нужен.
func TestPasteNamesCaption(t *testing.T) {
	e, sid := fileEd(t, "a.jpg", "b.jpg")
	pre, _ := e.PreviewNames(sid, model.NamesOpts{Text: "А"})
	if !pre.NeedCaption {
		t.Fatal("лист без шаблона: NeedCaption")
	}
	// Без флажка — только имена.
	e.PasteNames(sid, model.NamesOpts{Text: "А"})
	if edtest.P(e).Spreads[0].Caps != nil {
		t.Fatal("подпись без флажка")
	}
	// Свой стиль листа без шаблона: кегль остаётся, шаблон добавляется.
	st := model.DefaultCaptionStyle()
	st.Size = 13
	e.SetSheetCaps(sid, true, model.SheetCaps{Style: st, Pad: 3})
	undo := edtest.UndoLen(e)
	_, plan, _ := e.PasteNames(sid, model.NamesOpts{Text: "Б\nВ", Caption: true})
	c := edtest.P(e).Spreads[0].Caps
	if !plan.NeedCaption || c == nil || c.Text != names.NameCaption || c.Style.Size != 13 || c.Pad != 3 || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("подпись: %+v, шагов %d", c, edtest.UndoLen(e)-undo)
	}
	if pre, _ = e.PreviewNames(sid, model.NamesOpts{Text: "Б"}); pre.NeedCaption {
		t.Fatal("с шаблоном: NeedCaption")
	}
	// Свой шаблон не заменяется.
	e.SetSheetCaps(sid, true, model.SheetCaps{Style: st, Pad: 3, Text: "{Имя}"})
	e.PasteNames(sid, model.NamesOpts{Text: "Г", Caption: true})
	if edtest.P(e).Spreads[0].Caps.Text != "{Имя}" {
		t.Fatal("свой шаблон заменён")
	}
	// Лист без стиля: стиль новой подписи альбома, отступ 1 мм.
	e2, sid2 := fileEd(t, "a.jpg")
	e2.PasteNames(sid2, model.NamesOpts{Text: "А", Caption: true})
	if c := edtest.P(e2).Spreads[0].Caps; c == nil || c.Pad != model.DefaultPad || c.Style != model.NewCaptionStyle(edtest.P(e2).Settings.PageH) {
		t.Fatalf("новый стиль: %+v", c)
	}
	// Виньетка: шаблон подписей у неё свой, флажок не нужен.
	ev, sidv, _, _ := vigEd(t)
	if pre, _ = ev.PreviewNames(sidv, model.NamesOpts{Text: "А"}); pre.NeedCaption {
		t.Fatal("виньетка: NeedCaption")
	}
}

// 1000 строк таблицы × 64 фото — предпросмотр (разбор и план) за миллисекунды: в тишине ~2 мс,
// предел 15 мс (под нагрузкой ЦП втрое медленнее) всё равно ловит квадратичный рост — он на порядки больше.
func TestPreviewNamesFast(t *testing.T) {
	files := make([]string, model.MaxPhotos)
	for i := range files {
		files[i] = fmt.Sprintf("IMG_%04d.jpg", i)
	}
	e, sid := fileEd(t, files...)
	var b strings.Builder
	b.WriteString("№\tФамилия\tИмя\tОтчество\tПредмет\tФото\n")
	for i := range names.MaxPasteLines - 1 {
		fmt.Fprintf(&b, "%d\tФамилия%d\tИмя\tОтчество\t\t%d\n", i+1, i, i%200)
	}
	o := model.NamesOpts{Text: b.String()}
	var plan model.NamesPlan
	d := testkit.Fastest(7, func() { plan, _ = e.PreviewNames(sid, o) })
	if plan.Named != model.MaxPhotos || plan.Extra != names.MaxPasteLines-1-model.MaxPhotos {
		t.Fatalf("план: подписано %d, лишних %d", plan.Named, plan.Extra)
	}
	if lim := testkit.Limit(15 * time.Millisecond); d > lim {
		t.Fatalf("предпросмотр %v, порог %v", d, lim)
	}
	o.From = "files"
	if d = testkit.Fastest(7, func() { plan, _ = e.PreviewNames(sid, o) }); d > testkit.Limit(15*time.Millisecond) {
		t.Fatalf("имена файлов %v", d)
	}
}

// Виньетка с групповым фото и пустым местом: групповое получает строку без номера и имени
// (по файлу имя в него не ложится — строка уходит в Left), у пустого места строки нет.
func TestPasteNamesGroupAndEmptySeat(t *testing.T) {
	setup := func() (e *album.Editor, sid, teacher string, pupils []string) {
		e, sid, teacher, pupils = vigEd(t)
		edtest.Change(e, func(p *model.Project) {
			for _, pl := range p.Spreads[0].Grid {
				switch pl.Photo {
				case pupils[0]:
					pl.Role = model.RoleGroup
				case pupils[2]:
					pl.Photo = ""
				}
			}
			// Фото без места садится обратно в первое пустое: убираем и из Photos.
			p.Spreads[0].Photos = model.GridPhotos(p.Spreads[0])
		})
		return e, sid, teacher, pupils
	}
	// По файлу: имя на групповое не ложится, соседнее фото получает своё.
	e, sid, _, pupils := setup()
	o := model.NamesOpts{Text: "Файл\tФИО\n" + pupils[0] + ".jpg\tГрупповой Не\n" + pupils[1] + ".jpg\tИванов Пётр"}
	previewThenPaste(t, "по файлу", e, sid, o)
	e, sid, _, pupils = setup()
	_, plan, err := e.PasteNames(sid, o)
	if err != nil {
		t.Fatal(err)
	}
	if person(e, pupils[0]) != "" || person(e, pupils[1]) != "Иванов Пётр" {
		t.Fatalf("имена: групповое %q, ученик %q", person(e, pupils[0]), person(e, pupils[1]))
	}
	if len(plan.Rows) != 3 || plan.Seats != 2 || plan.Named != 1 || plan.Missing != 1 || plan.Extra != 1 ||
		len(plan.Left) != 1 || plan.Left[0].Reason != "nofile" || plan.Left[0].Line != 2 {
		t.Fatalf("итог: %+v", plan)
	}
	for _, row := range plan.Rows {
		if row.Photo == "" {
			t.Fatalf("пустое место попало в строки: %+v", plan.Rows)
		}
		if row.Photo == pupils[0] && (row.Tag != "групповое" || row.No != 0 || row.Who != nil) {
			t.Fatalf("групповое: %+v", row)
		}
	}
	// По порядку: групповое и пустое место счёт не занимают — три строки на два места.
	e, sid, teacher, pupils := setup()
	_, plan, err = e.PasteNames(sid, model.NamesOpts{Text: "А\nБ\nВ"})
	if err != nil || plan.Seats != 2 || plan.Extra != 1 || person(e, pupils[1]) != "А" || person(e, teacher) != "Б" || person(e, pupils[0]) != "" {
		t.Fatalf("по порядку: %+v, %v", plan, err)
	}
}

// Слишком длинный список отвергается предпросмотром до чтения проекта — как и вставкой.
func TestPreviewNamesTooLong(t *testing.T) {
	e, sid := fileEd(t, "a.jpg")
	rev := edtest.Rev(e)
	o := model.NamesOpts{Text: strings.Repeat("я", names.MaxPasteBytes+1)}
	plan, err := e.PreviewNames(sid, o)
	if err == nil || edtest.Rev(e) != rev || len(plan.Rows) != 0 || plan.Extra != 0 {
		t.Fatalf("длинный список: %+v, %v", plan, err)
	}
	// Короткий список принимается.
	if _, err = e.PreviewNames(sid, model.NamesOpts{Text: "Иванов"}); err != nil {
		t.Fatalf("короткий список: %v", err)
	}
}

// Места виньетки считаются вместе с пустыми (Rows — только с фото); не виньетка — нули.
func TestPreviewNamesSeats(t *testing.T) {
	e, sid, _, _ := vigEd(t)
	check := func(what string, teachers, pupils, rows int) {
		t.Helper()
		rev := edtest.Rev(e)
		plan, err := e.PreviewNames(sid, model.NamesOpts{Text: "А\nБ"})
		if err != nil || plan.TeacherSeats != teachers || plan.PupilSeats != pupils || len(plan.Rows) != rows {
			t.Fatalf("%s: учителей %d, учеников %d, строк %d (ждали %d, %d, %d), %v",
				what, plan.TeacherSeats, plan.PupilSeats, len(plan.Rows), teachers, pupils, rows, err)
		}
		if edtest.Rev(e) != rev {
			t.Fatalf("%s: предпросмотр сменил rev", what)
		}
	}
	check("все места с фото", 1, 3, 4)
	// Ученик 3→4 (шаг «Настроить виньетку»): новое место пустое, строк по-прежнему 4.
	if _, _, err := e.SetVignette(sid, model.Vignette{Tpl: "head-t-left", Teachers: 1, Pupils: 4}, nil); err != nil {
		t.Fatal(err)
	}
	check("пустое место ученика", 1, 4, 4)
	// Ещё два пустых места: освобождены два первых по порядку сетки.
	edtest.Change(e, func(p *model.Project) {
		cleared := 0
		for _, pl := range p.Spreads[0].Grid {
			if pl.Photo != "" && cleared < 2 {
				pl.Photo = ""
				cleared++
			}
		}
		p.Spreads[0].Photos = model.GridPhotos(p.Spreads[0])
	})
	check("несколько пустых мест", 1, 4, 2)
	// Обычный лист: нули.
	e2, sid2 := fileEd(t, "a.jpg", "b.jpg")
	plan, err := e2.PreviewNames(sid2, model.NamesOpts{Text: "А"})
	if err != nil || plan.TeacherSeats != 0 || plan.PupilSeats != 0 || len(plan.Rows) != 2 {
		t.Fatalf("не виньетка: %+v, %v", plan, err)
	}
}
