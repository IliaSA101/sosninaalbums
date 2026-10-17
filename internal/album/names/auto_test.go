package names_test

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// 060 (А): имя из названия файла при броске.

// restAdd — ответ броска «остальные плиткой» (spreads.RestAdd; область spreads отсюда не видна).
const restAdd = "add"

// nameTpl — шаблон подписи с полями имени.
const nameTpl = "{Фамилия} {Имя}"

// named — фото с названием файла file (ID id).
func named(id, file string) model.PhotoInfo {
	return model.PhotoInfo{ID: id, Path: edtest.Abs(file), W: 300, H: 200}
}

// capsEd — сетка s1 из places пустых мест с шаблоном подписи tpl ("" — без шаблона) и «Авто» s2
// из фото "old" с названием «Петров Пётр.jpg»-подобным вольным именем (оно уже в альбоме).
func capsEd(t *testing.T, places int, tpl string) *album.Editor {
	t.Helper()
	pls := make([]*model.Place, places)
	for i := range pls {
		pls[i] = edtest.Gp(10+float64(i)*60, 10, 50, 50, "")
	}
	e := edtest.GridEd(t, pls, []string{"x"})
	edtest.Change(e, func(p *model.Project) {
		p.Photos["old"] = model.PhotoRef{Path: edtest.Abs("05_петров_пётр.jpg"), W: 300, H: 200}
		p.Spreads[1].Photos = append(p.Spreads[1].Photos, "old")
		if tpl != "" {
			p.Spreads[0].Caps = &model.SheetCaps{Style: model.NewCaptionStyle(p.Settings.PageH), Text: tpl}
			p.Spreads[1].Caps = &model.SheetCaps{Style: model.NewCaptionStyle(p.Settings.PageH), Text: tpl}
		}
	})
	return e
}

// who — имя фото id одной строкой; нет имени — "".
func who(e *album.Editor, id string) string { return person(e, id) }

// Бросок на лист: вольное имя — сразу и тем же шагом; строгое, «IMG», перенос из альбома —
// без записи; Ctrl+Z снимает бросок вместе с именами.
func TestDropAutoNames(t *testing.T) {
	e := capsEd(t, 6, nameTpl)
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	ph := []model.PhotoInfo{
		named("a", "01_иванов_петя.jpg"), named("b", "IMG_1234.jpg"), named("c", "Сидоров Ваня.jpg"),
		named("d", "ПЕТРОВА анна сергеевна (2).jpg"), {ID: "old", Path: edtest.Abs("05_петров_пётр.jpg"), W: 300, H: 200},
	}
	_, out, err := e.DropPhotos("s1", "", ph, -1, restAdd)
	if err != nil || !slices.Equal(out.Named, []string{"a", "d"}) {
		t.Fatalf("named %v, ошибка %v", out.Named, err)
	}
	if edtest.Rev(e) != rev+1 || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("шагов %d, rev +%d: бросок и имена — один шаг", edtest.UndoLen(e)-undo, edtest.Rev(e)-rev)
	}
	want := map[string]string{"a": "Иванов Петя", "b": "", "c": "", "d": "Петрова Анна Сергеевна", "old": ""}
	for id, w := range want {
		if got := who(e, id); got != w {
			t.Errorf("%s: имя %q, ждали %q", id, got, w)
		}
	}
	// Строгое имя видно и без записи.
	if pv, _ := e.Person("c"); !pv.FromFile || pv.Last != "Сидоров" {
		t.Fatalf("строгое имя c: %+v", pv)
	}
	e.Undo()
	if _, ok := edtest.P(e).Photos["a"]; ok || edtest.UndoLen(e) != undo {
		t.Fatalf("Ctrl+Z оставил фото a или шаг: undo %d", edtest.UndoLen(e))
	}
}

// Строгое имя из файла тоже подчиняется стоп-списку (0.14 decisions 32): «Свадьба Ивановых»
// на листе с полями имени — ни автоимени, ни имени из файла в карточке и подписи; «Иванов
// Петя» — строгое имя из файла, как прежде (видно без записи).
func TestDropStrictNameStop(t *testing.T) {
	e := capsEd(t, 4, nameTpl)
	ph := []model.PhotoInfo{named("w", "Свадьба Ивановых.jpg"), named("i", "Иванов Петя.jpg"), named("f", "Фото Петрова Анна.jpg")}
	_, out, err := e.DropPhotos("s1", "", ph, -1, "")
	if err != nil || out.Named != nil {
		t.Fatalf("бросок: %v, %+v", err, out)
	}
	for _, id := range []string{"w", "f"} {
		if pv, _ := e.Person(id); pv != (model.PersonView{}) || who(e, id) != "" {
			t.Errorf("%s: имя %+v, своё %q — ждали без имени", id, pv, who(e, id))
		}
	}
	if pv, _ := e.Person("i"); !pv.FromFile || pv.Last != "Иванов" || pv.First != "Петя" || who(e, "i") != "" {
		t.Fatalf("строгое имя i: %+v, своё %q", pv, who(e, "i"))
	}
	caps := map[string]string{}
	for _, c := range e.View().Spreads[0].Captions {
		caps[c.Photo] = c.Text
	}
	if strings.Contains(caps["w"]+caps["f"], "Свадьба") || strings.Contains(caps["w"]+caps["f"], "Фото") || !strings.Contains(caps["i"], "Иванов") {
		t.Fatalf("подписи по шаблону: %q", caps)
	}
}

// Имя круг Save → Load переживает (пишется в файл как своё).
func TestDropAutoNamesSaveLoad(t *testing.T) {
	e := capsEd(t, 2, nameTpl)
	if _, out, err := e.DropPhotos("s1", "", []model.PhotoInfo{named("a", "иванов_петя.jpg"), named("b", "ivanov.petya.jpg")}, -1, ""); err != nil || len(out.Named) != 2 {
		t.Fatalf("бросок: %v, %+v", err, out)
	}
	p := edtest.SaveLoad(t, e)
	// Load переводит ID в ключи от путей: имя ищется по файлу.
	byFile := map[string]*model.Person{}
	for _, r := range p.Photos {
		byFile[filepath.Base(r.Path)] = r.Person
	}
	for file, w := range map[string]model.Person{"иванов_петя.jpg": {Last: "Иванов", First: "Петя"}, "ivanov.petya.jpg": {Last: "Ivanov", First: "Petya"}} {
		if got := byFile[file]; got == nil || *got != w {
			t.Fatalf("%s после Load: %+v (%v)", file, got, byFile)
		}
	}
}

// Шаблона нет, в шаблоне нет полей имени или только «Предмет» — имён нет, бросок как прежде.
func TestDropAutoNamesNoFields(t *testing.T) {
	for _, tpl := range []string{"", "{Школа}", "{Предмет}", "Без полей"} {
		e := capsEd(t, 2, tpl)
		_, out, err := e.DropPhotos("s1", "", []model.PhotoInfo{named("a", "иванов_петя.jpg")}, -1, "")
		if err != nil || out.Named != nil || who(e, "a") != "" || !slices.Contains(edtest.P(e).Spreads[0].Photos, "a") {
			t.Fatalf("%q: %v, %+v, имя %q", tpl, err, out, who(e, "a"))
		}
	}
}

// Своё имя (фото уже на фоне с набранным именем) не переписывается; свой предмет без имени
// остаётся рядом с автоименем.
func TestDropAutoNamesOwn(t *testing.T) {
	e := capsEd(t, 3, nameTpl)
	edtest.Change(e, func(p *model.Project) {
		p.Photos["own"] = model.PhotoRef{Path: edtest.Abs("иванов_петя.jpg"), W: 300, H: 200, Person: &model.Person{Last: "Свой"}}
		p.Photos["subj"] = model.PhotoRef{Path: edtest.Abs("сидорова_мария.jpg"), W: 300, H: 200, Person: &model.Person{Subject: "Химия"}}
		p.Spreads[1].BgPhoto = &model.Backdrop{Photo: "own", W: 100, H: 100}
		p.Spreads[1].BgPhotoL = &model.Backdrop{Photo: "subj", W: 100, H: 100}
	})
	if edtest.P(e).Photos["own"].Person == nil || edtest.P(e).Photos["subj"].Person == nil {
		t.Fatal("фото на фоне без имени: проверить нечего")
	}
	_, out, err := e.DropPhotos("s1", "", []model.PhotoInfo{named("own", "иванов_петя.jpg"), named("subj", "сидорова_мария.jpg")}, -1, "")
	if err != nil || !slices.Equal(out.Named, []string{"subj"}) {
		t.Fatalf("named %v, %v", out.Named, err)
	}
	if got := *edtest.P(e).Photos["own"].Person; got != (model.Person{Last: "Свой"}) {
		t.Fatalf("своё имя переписано: %+v", got)
	}
	if got := *edtest.P(e).Photos["subj"].Person; got != (model.Person{Last: "Сидорова", First: "Мария", Subject: "Химия"}) {
		t.Fatalf("предмет и автоимя: %+v", got)
	}
}

// Вопрос броска (Ask) ничего не меняет; отказ правки — ни фото, ни имён, named nil.
func TestDropAutoNamesAskAndFail(t *testing.T) {
	e := capsEd(t, 1, nameTpl)
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	_, out, err := e.DropPhotos("s1", "", []model.PhotoInfo{named("a", "иванов_петя.jpg"), named("b", "петров_иван.jpg")}, -1, "")
	if err != nil || !out.Ask || out.Named != nil {
		t.Fatalf("вопрос: %v, %+v", err, out)
	}
	edtest.Same(t, e, rev, undo, "вопрос")
	// «Авто» с шаблоном: больше MaxPhotos — отказ AddPhotos, имена тоже откатываются.
	many := make([]model.PhotoInfo, model.MaxPhotos)
	for i := range many {
		many[i] = named("m"+string(rune('a'+i%26))+string(rune('a'+i/26)), "иванов_петя.jpg")
	}
	_, out, err = e.DropPhotos("s2", "", many, -1, "")
	if err == nil || out.Named != nil {
		t.Fatalf("отказ: %v, %+v", err, out)
	}
	edtest.Same(t, e, rev, undo, "отказ")
	if _, ok := edtest.P(e).Photos["ma"]; ok {
		t.Fatal("отказ оставил фото")
	}
}

// «Поставить на места» и «Заполнить» — тот же путь: тем же шагом, только новым фото.
func TestPlaceFillAutoNames(t *testing.T) {
	e := capsEd(t, 5, nameTpl)
	pl := edtest.P(e).Spreads[0].Grid[1].ID
	undo := edtest.UndoLen(e)
	_, left, nm, err := e.PlacePhotos("s1", pl, []model.PhotoInfo{named("a", "иванов_петя.jpg"), named("b", "DSC_0012.jpg"), named("c", "кузнецов_олег.jpg")})
	if err != nil || left != 0 || !slices.Equal(nm, []string{"a", "c"}) || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("PlacePhotos: %v, left %d, named %v, шагов %d", err, left, nm, edtest.UndoLen(e)-undo)
	}
	// Перенос фото альбома на место — без имени.
	if _, _, nm, _ = e.PlacePhotos("s1", edtest.P(e).Spreads[0].Grid[0].ID, []model.PhotoInfo{{ID: "old", Path: edtest.Abs("05_петров_пётр.jpg"), W: 300, H: 200}}); nm != nil || who(e, "old") != "" {
		t.Fatalf("перенос: named %v, имя %q", nm, who(e, "old"))
	}
	_, left, nm = e.FillPlaces([]model.PhotoInfo{named("d", "смирнова_ольга.jpg"), named("f", "морозов_иван.jpg")}, "s1")
	if left != 1 || !slices.Equal(nm, []string{"d"}) || who(e, "d") != "Смирнова Ольга" || who(e, "f") != "" {
		t.Fatalf("FillPlaces: left %d, named %v, d %q, f %q", left, nm, who(e, "d"), who(e, "f"))
	}
	// Мест нет — холостая, named nil.
	rev := edtest.Rev(e)
	if _, _, nm = e.FillPlaces([]model.PhotoInfo{named("g", "орлов_иван.jpg")}, "s1"); nm != nil || edtest.Rev(e) != rev {
		t.Fatalf("без мест: named %v, rev %d → %d", nm, rev, edtest.Rev(e))
	}
}

// Виньетка: ученики — по шаблону листа; учитель — по своему шаблону (TText) без полей имени
// имени не получает; групповое — никогда (как окно «Имена»).
func TestVignetteAutoNames(t *testing.T) {
	e := edtest.Open(t, album.NewVignetteSheet(204, 304), "")
	sid := edtest.P(e).Spreads[0].ID
	if _, _, err := e.SetVignette(sid, model.Vignette{Tpl: "t-top-g", Teachers: 1, Pupils: 2}, nil); err != nil {
		t.Fatal(err)
	}
	edtest.Change(e, func(p *model.Project) {
		s := p.Spreads[0]
		s.Caps = &model.SheetCaps{Style: model.NewCaptionStyle(p.Settings.PageH), Text: nameTpl}
		s.Vig.TText = "{Предмет}"
	})
	ph := []model.PhotoInfo{named("v1", "иванов_петя.jpg"), named("v2", "петров_иван.jpg"),
		named("v3", "сидоров_олег.jpg"), named("v4", "орлова_анна.jpg")}
	_, _, nm := e.FillPlaces(ph, sid)
	roles := map[string]string{}
	for _, pl := range edtest.P(e).Spreads[0].Grid {
		roles[pl.Photo] = model.RoleOf(pl)
	}
	var pupils []string
	for _, p := range ph {
		got := who(e, p.ID)
		switch roles[p.ID] {
		case model.RolePupil:
			pupils = append(pupils, p.ID)
			if got == "" {
				t.Errorf("ученик %s без имени", p.ID)
			}
		case model.RoleTeacher, model.RoleGroup:
			if got != "" {
				t.Errorf("%s (%s): имя %q", p.ID, roles[p.ID], got)
			}
		default:
			t.Fatalf("%s не встало: %v", p.ID, roles)
		}
	}
	if !slices.Equal(nm, pupils) || len(pupils) != 2 {
		t.Fatalf("named %v, ученики %v", nm, pupils)
	}
}

// «Убрать имена»: снимает только автоимена, которые не правили; один шаг; повтор и пустой
// список — холостые; Ctrl+Z возвращает имена.
func TestForgetFileNames(t *testing.T) {
	e := capsEd(t, 4, nameTpl)
	_, out, err := e.DropPhotos("s1", "", []model.PhotoInfo{named("a", "иванов_петя.jpg"), named("b", "петров_иван.jpg"),
		named("c", "сидоров_олег.jpg"), named("d", "IMG_0001.jpg")}, -1, "")
	if err != nil || len(out.Named) != 3 {
		t.Fatalf("бросок: %v, %+v", err, out)
	}
	e.SetPerson("b", model.Person{Last: "Петров", First: "Иван", Middle: "Ильич"}, "")
	edtest.Change(e, func(p *model.Project) {
		r := p.Photos["c"]
		r.Person = &model.Person{Last: "Сидоров", First: "Олег", Subject: "Химия"}
		p.Photos["c"] = r
	})
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	for _, ids := range [][]string{nil, {}, {"нет", "d"}} {
		e.ForgetFileNames(ids)
		edtest.Same(t, e, rev, undo, "холостая")
	}
	e.ForgetFileNames(append(out.Named, "нет", "d"))
	if edtest.Rev(e) != rev+1 || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("шагов %d: «Убрать имена» — один шаг", edtest.UndoLen(e)-undo)
	}
	if who(e, "a") != "" || who(e, "b") != "Петров Иван Ильич" {
		t.Fatalf("a %q, b %q: снять только неизменённое", who(e, "a"), who(e, "b"))
	}
	if got := edtest.P(e).Photos["c"].Person; got == nil || *got != (model.Person{Subject: "Химия"}) {
		t.Fatalf("c: предмет должен остаться без имени: %+v", got)
	}
	if !slices.Contains(edtest.P(e).Spreads[0].Photos, "a") {
		t.Fatal("фото ушло с места")
	}
	rev, undo = edtest.Rev(e), edtest.UndoLen(e)
	e.ForgetFileNames(out.Named)
	edtest.Same(t, e, rev, undo, "повтор")
	e.Undo()
	if who(e, "a") != "Иванов Петя" {
		t.Fatalf("Ctrl+Z: a %q", who(e, "a"))
	}
}
