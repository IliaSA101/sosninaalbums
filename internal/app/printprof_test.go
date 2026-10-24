package app

import (
	"bytes"
	"errors"
	"log"
	"math"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/model/printprof"
	"github.com/IliaSA101/sosninaalbums/internal/decor"
)

// Свои типографии в App (123 Б1): окно, запись, реестр, удаление. Файл .saprint — printprof_file_test.go.

// printApp — App с открытой библиотекой и реестром из встроенных; реестр общий на процесс,
// к концу теста возвращается прежний.
func printApp(t *testing.T) (*App, *events) {
	t.Helper()
	a := decorApp(t)
	printprof.UseForTest(t, nil)
	a.reloadPrints()
	return a, catchEvents(a)
}

// editOf — заготовка окна на основе base с названием name.
func editOf(t *testing.T, a *App, base, name string) PrintEdit {
	t.Helper()
	e, err := a.PrintProfileEdit(base)
	if err != nil {
		t.Fatal(err)
	}
	e.Name = name
	return e
}

// saved — Save без применения; провал теста при ошибке.
func saved(t *testing.T, a *App, e PrintEdit) PrintSaved {
	t.Helper()
	res, err := a.SavePrintProfile(e, false)
	if err != nil {
		t.Fatalf("сохранение %q: %v", e.Name, err)
	}
	return res
}

func ownCount(a *App) int {
	n := 0
	for _, in := range a.PrintProfiles() {
		if in.Source == printprof.SourceOwn {
			n++
		}
	}
	return n
}

func TestPrintProfileEditBase(t *testing.T) {
	a, _ := printApp(t)
	e, err := a.PrintProfileEdit("fabrika")
	if err != nil {
		t.Fatal(err)
	}
	if e.Name != "Фабрика Фотокниги (своя)" || e.Base != "fabrika" || e.DPI != 300 || !e.Layflat || e.Safe != 10 || e.Bleed != 3 ||
		e.Prefix != "1-" || e.Quality != 100 || e.Spreads != 2 || len(e.Formats) != 5 {
		t.Fatalf("заготовка: %+v", e)
	}
	// 30×20 — px шаблона из PSD, обложка при 2 разворотах (корешок 8): 2·17 + 2·310 + 8 = 662 мм.
	if f := e.Formats[3]; f.Name != "30×20" || f.PxW != 7181 || f.PxH != 2398 || f.CoverW != 7819 || f.CoverH != 2835 {
		t.Errorf("формат 30×20: %+v", f)
	}
	// Пустая заготовка: форматов нет, но срез не nil (в JSON — [], а не null).
	if z, err := a.PrintProfileEdit(""); err != nil || z.Formats == nil || z.Base != "" || z.DPI != 300 || z.Quality != 100 {
		t.Errorf("пустая заготовка: %+v %v", z, err)
	}
	if _, err := a.PrintProfileEdit("нет-такой"); err == nil {
		t.Error("основа без типографии принята")
	}
	// Название заготовки занято — следующее свободное.
	saved(t, a, e)
	if e2, _ := a.PrintProfileEdit("fabrika"); e2.Name != "Фабрика Фотокниги (своя 2)" {
		t.Errorf("второе название: %q", e2.Name)
	}
}

// Расчёт окна: страница и сторонка из px совпадают с шаблоном Фабрики, корешок — по таблице основы.
func TestCheckPrintProfile(t *testing.T) {
	a, _ := printApp(t)
	e := editOf(t, a, "fabrika", "Моя")
	e.Spreads = 40
	e.Formats = []PrintEditFormat{{Name: "30×20", PxW: 7181, PxH: 2398, CoverW: 8031, CoverH: 2835}}
	c := a.CheckPrintProfile(e)
	if !c.OK || len(c.Errs) != 0 || len(c.Formats) != 1 {
		t.Fatalf("расчёт: %+v", c)
	}
	if r := c.Formats[0]; r.PageW != 304 || r.PageH != 203 || r.SideW != 310 || r.SideH != 206 || r.Err != "" {
		t.Errorf("строка: %+v", r)
	}
	if c.Spine != 26 {
		t.Errorf("корешок при 40 разворотах: %v", c.Spine)
	}
	if c.Names != "1-00.jpg — обложка, 1-01.jpg…1-02.jpg" {
		t.Errorf("имена: %q", c.Names)
	}
	if len(c.Lines) != 1 || !strings.HasPrefix(c.Lines[0], "Сейчас альбом даёт: разворот ") {
		t.Errorf("строки альбома: %q", c.Lines)
	}
	// Альбом ровно на странице формата — строка называет формат.
	p, _ := a.editor().Snapshot()
	p.Settings.PageW, p.Settings.PageH = 304, 203
	a.ed.Store(mustEditor(t, p, ""))
	if c = a.CheckPrintProfile(e); len(c.Lines) != 1 || !strings.HasSuffix(c.Lines[0], "— это формат «30×20»") {
		t.Errorf("совпавший формат: %q", c.Lines)
	}
	// Проект проверка не меняет.
	if v := a.editor().View(); v.CanUndo || v.Dirty {
		t.Errorf("расчёт тронул проект: %+v", v)
	}
}

// Края: каждая плохая вещь — отказ в своём поле, ничего не пишется и события нет.
func TestSavePrintProfileRejects(t *testing.T) {
	a, ev := printApp(t)
	good := editOf(t, a, "fabrika", "Хорошая")
	long := strings.Repeat("я", 41)
	cases := []struct {
		name  string
		field string
		mut   func(e *PrintEdit)
	}{
		{"пустое название", "name", func(e *PrintEdit) { e.Name = "  " }},
		{"длинное название", "name", func(e *PrintEdit) { e.Name = long }},
		{"название встроенной", "name", func(e *PrintEdit) { e.Name = "Фабрика Фотокниги" }},
		{"то же без регистра и с краями", "name", func(e *PrintEdit) { e.Name = " фабрика фотокниги " }},
		{"управляющий знак", "name", func(e *PrintEdit) { e.Name = "А\x07Б" }},
		{"основы нет", "base", func(e *PrintEdit) { e.Base = "нет-такой" }},
		{"dpi мал", "dpi", func(e *PrintEdit) { e.DPI = 71 }},
		{"dpi велик", "dpi", func(e *PrintEdit) { e.DPI = 1201 }},
		{"зона NaN", "safe", func(e *PrintEdit) { e.Safe = math.NaN() }},
		{"зона велика", "safe", func(e *PrintEdit) { e.Safe = 31 }},
		{"обрез отрицателен", "bleed", func(e *PrintEdit) { e.Bleed = -1 }},
		{"обрез Inf", "bleed", func(e *PrintEdit) { e.Bleed = math.Inf(1) }},
		{"нет форматов", "formats", func(e *PrintEdit) { e.Formats = nil }},
		{"форматов много", "formats", func(e *PrintEdit) {
			e.Formats = nil
			for i := range 33 {
				e.Formats = append(e.Formats, PrintEditFormat{Name: strings.Repeat("a", 1) + string(rune('A'+i)), PxW: 3000, PxH: 2000})
			}
		}},
		{"px ноль", "formats", func(e *PrintEdit) { e.Formats = []PrintEditFormat{{Name: "x", PxW: 0, PxH: 0}} }},
		{"px огромны", "formats", func(e *PrintEdit) { e.Formats = []PrintEditFormat{{Name: "x", PxW: 90000, PxH: 90000}} }},
		{"px отрицательны", "formats", func(e *PrintEdit) { e.Formats = []PrintEditFormat{{Name: "x", PxW: -5, PxH: 2000}} }},
		{"подпись пуста", "formats", func(e *PrintEdit) { e.Formats[0].Name = "" }},
		{"подпись повторена", "formats", func(e *PrintEdit) { e.Formats[1].Name = e.Formats[0].Name }},
		{"подпись длинна", "formats", func(e *PrintEdit) { e.Formats[0].Name = long }},
		{"одна сторона обложки", "formats", func(e *PrintEdit) { e.Formats[0].CoverH = 0 }},
		{"обложка без основы", "formats", func(e *PrintEdit) { e.Base = ""; e.Formats = e.Formats[:1] }},
		{"обложка велика", "formats", func(e *PrintEdit) { e.Formats[0].CoverW = 60000 }},
		{"разворотов много", "spreads", func(e *PrintEdit) { e.Spreads = model.MaxSpreads + 1 }},
		{"префикс с точкой", "prefix", func(e *PrintEdit) { e.Prefix = "../" }},
		{"префикс длинен", "prefix", func(e *PrintEdit) { e.Prefix = "123456789" }},
		{"префикс не ASCII", "prefix", func(e *PrintEdit) { e.Prefix = "Ф-" }},
		{"качество 80", "quality", func(e *PrintEdit) { e.Quality = 80 }},
		{"сайт http", "url", func(e *PrintEdit) { e.URL = "http://example.com/" }},
		{"сайт IP", "url", func(e *PrintEdit) { e.URL = "https://127.0.0.1/" }},
		{"сайт: смесь алфавитов в метке", "url", func(e *PrintEdit) { e.URL = "https://exаmple.com/" }}, // «а» кириллическая
		{"сайт с логином", "url", func(e *PrintEdit) { e.URL = "https://user@example.com/" }},
	}
	for _, c := range cases {
		e := good
		e.Formats = slices.Clone(good.Formats)
		c.mut(&e)
		chk := a.CheckPrintProfile(e)
		if chk.OK || chk.Errs[c.field] == "" {
			t.Errorf("%s: расчёт %+v, ждали ошибку поля %s", c.name, chk.Errs, c.field)
		}
		if _, err := a.SavePrintProfile(e, false); err == nil {
			t.Errorf("%s: сохранено", c.name)
		}
	}
	if ownCount(a) != 0 || ev.count("print:changed") != 0 || len(a.mustStore(t).Prints()) != 0 {
		t.Errorf("после отказов свои: %d, событий %d", ownCount(a), ev.count("print:changed"))
	}
	if _, err := a.SavePrintProfile(PrintEdit{Name: "Фабрика Фотокниги"}, false); err == nil || err.Error() != "Такое название уже есть" {
		t.Errorf("текст дубля названия: %v", err)
	}
}

func (a *App) mustStore(t *testing.T) *decor.Store {
	t.Helper()
	s, err := a.decorStore()
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// Круг Save → Load: типография пережила перезапуск (новый App на той же папке), тот же профиль
// байт в байт; изделия, пределы и корешки — копия основы; несколько своих сразу.
func TestSavePrintProfileRoundTrip(t *testing.T) {
	a, ev := printApp(t)
	rev := a.editor().View().Rev
	base, _ := printprof.Current().ByID("fabrika")
	res1 := saved(t, a, editOf(t, a, "fabrika", "Моя Фабрика"))
	res2 := saved(t, a, editOf(t, a, "fsfera", "Моя Фотосфера"))
	res3 := saved(t, a, editOf(t, a, "netprint", "Мой Нетпринт"))
	if ev.count("print:changed") != 3 || ownCount(a) != 3 {
		t.Fatalf("событий %d, своих %d", ev.count("print:changed"), ownCount(a))
	}
	if res1.ID == res2.ID || res2.ID == res3.ID || !printprof.ValidOwnID(res1.ID) {
		t.Fatalf("id: %s %s %s", res1.ID, res2.ID, res3.ID)
	}
	p1, ok := printprof.Current().ByID(res1.ID)
	if !ok || printprof.Current().Source(res1.ID) != printprof.SourceOwn || p1.Name != "Моя Фабрика" || p1.Version != 1 {
		t.Fatalf("профиль: %+v %v", p1, ok)
	}
	if len(p1.Products) != len(base.Products) || p1.Default != printprof.OwnProductID(res1.ID, 1) || p1.Export == nil || p1.Export.Prefix != "1-" {
		t.Fatalf("изделия: %d, по умолчанию %q", len(p1.Products), p1.Default)
	}
	for i, f := range p1.Formats { // страница и сторонка из px вернули таблицу шаблона
		if bf := base.Formats[i]; f.Page != bf.Page || *f.Side != *bf.Side || f.Wrap != bf.Wrap || f.Px == nil || f.Px[0] != model.Px(2*bf.Page[0], 300) || f.Px[1] != model.Px(bf.Page[1], 300) {
			t.Errorf("формат %s: %+v, в основе %+v", f.Name, f, bf)
		}
	}
	if !slices.Equal(p1.Products[2].SpineBy["30×20"], base.Products[2].SpineBy["30×20"]) || len(p1.Products[1].Limits) != len(base.Products[1].Limits) {
		t.Errorf("корешок формата или пределы потеряны: %+v", p1.Products[2])
	}
	// Сохранение — не правка альбома: ни шага, ни rev.
	if v := a.editor().View(); v.Rev != rev || v.CanUndo || v.Dirty {
		t.Errorf("сохранение типографии дало правку: %+v", v)
	}
	// Новый App на той же папке: реестр читает свои с диска.
	b := newApp(t)
	b.cfgDir, b.emit = a.cfgDir, func(string, ...any) {}
	b.openDecor()
	t.Cleanup(func() { printprof.Install(nil) })
	b.reloadPrints()
	for _, id := range []string{res1.ID, res2.ID, res3.ID} {
		want, _ := printprof.Current().ByID(id)
		if want.ID != id {
			t.Fatalf("после перезапуска нет %s", id)
		}
	}
	got, _ := printprof.Current().ByID(res1.ID)
	g, _ := printprof.Encode(got)
	w, _ := printprof.Encode(p1)
	if !bytes.Equal(g, w) {
		t.Error("профиль после перезапуска изменился")
	}
}

// Убранный формат уносит свои таблицу корешка и предел; профиль остаётся годным.
func TestSavePrintProfileDropsFormat(t *testing.T) {
	a, _ := printApp(t)
	e := editOf(t, a, "fabrika", "Без 30×20")
	e.Formats = slices.DeleteFunc(e.Formats, func(f PrintEditFormat) bool { return f.Name == "30×20" })
	res := saved(t, a, e)
	p, _ := printprof.Current().ByID(res.ID)
	pr := p.Products[2] // прослойка 2 мм: SpineBy и предел были только у 30×20
	if pr.SpineBy != nil || slices.ContainsFunc(pr.Limits, func(l printprof.Limit) bool { return l.Format == "30×20" }) {
		t.Errorf("следы убранного формата: %+v", pr)
	}
	if len(p.Formats) != 4 {
		t.Errorf("форматов %d", len(p.Formats))
	}
	// Без основы — ни изделий, ни обложки; px-формат один.
	z := saved(t, a, PrintEdit{Name: "Голая", DPI: 300, Quality: 95, Formats: []PrintEditFormat{{Name: "A4", PxW: 3508, PxH: 2480}}})
	zp, _ := printprof.Current().ByID(z.ID)
	if len(zp.Products) != 0 || zp.Default != "" || zp.Cover != nil || zp.Export.Quality != 95 || zp.Formats[0].Template() {
		t.Errorf("голая типография: %+v", zp)
	}
}

// Сайт: кнопка называет хост, адрес изделий основы не переносится.
func TestSavePrintProfileSite(t *testing.T) {
	a, _ := printApp(t)
	e := editOf(t, a, "fabrika", "С сайтом")
	e.URL = "https://example.com/order"
	res := saved(t, a, e)
	p, _ := printprof.Current().ByID(res.ID)
	if p.Export.URL != e.URL || p.Export.SiteButton != "Открыть example.com" {
		t.Errorf("сайт: %+v", p.Export)
	}
	for _, pr := range p.Products {
		if pr.URL != "" {
			t.Errorf("адрес изделия основы перенесён: %q", pr.URL)
		}
	}
}

func TestDeletePrintProfile(t *testing.T) {
	a, ev := printApp(t)
	keep := saved(t, a, editOf(t, a, "fabrika", "Остаётся"))
	gone := saved(t, a, editOf(t, a, "fsfera", "Уходит"))
	preset := printprof.OwnProductID(gone.ID, 1)
	if _, ok := model.CoverPresetOf(preset); !ok {
		t.Fatalf("изделие своей не в таблицах: %s", preset)
	}
	before := ev.count("print:changed")
	if _, err := a.DeletePrintProfile(gone.ID); err != nil {
		t.Fatal(err)
	}
	if ev.count("print:changed") != before+1 || ownCount(a) != 1 || len(a.mustStore(t).Prints()) != 1 {
		t.Fatalf("после удаления: событий %d, своих %d", ev.count("print:changed"), ownCount(a))
	}
	if _, ok := printprof.Current().ByID(keep.ID); !ok {
		t.Error("удалена не та")
	}
	// Изделие уходит из таблиц; пометка в альбоме остаётся, карточка пишет «не найдена».
	if _, ok := model.CoverPresetOf(preset); ok {
		t.Error("изделие удалённой осталось в таблицах")
	}
	if got := model.CoverMissing(preset); got != "Типография не найдена: "+preset {
		t.Errorf("пометка: %q", got)
	}
	// Повтор, встроенная, чужой id, пустой: отказ, события нет.
	for _, id := range []string{gone.ID, "fabrika", "fsfera", "нет", ""} {
		if _, err := a.DeletePrintProfile(id); !errors.Is(err, errPrintNotOwn) {
			t.Errorf("удаление %q: %v", id, err)
		}
	}
	if ev.count("print:changed") != before+1 {
		t.Error("отказ послал событие")
	}
	// Удалённую можно сохранить заново: новое имя свободно.
	saved(t, a, editOf(t, a, "fsfera", "Уходит"))
}

// Пересборка реестра: свой файл, записанный мимо окна, появляется; битый и спорный пропускаются
// со строкой в log.txt, остальные работают.
func TestReloadPrints(t *testing.T) {
	a, _ := printApp(t)
	var logBuf bytes.Buffer
	log.SetOutput(&logBuf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	s := a.mustStore(t)
	mk := func(id, name string) []byte {
		p := buildOwn(editOf(t, a, "fabrika", "Временная "+id), id, printprof.Builtin(), 2).p
		if p.ID != id {
			t.Fatalf("профиль %s не собран", id)
		}
		p.Name, p.Gen = name, name // имя, занятое в реестре, окно не пропустит — файл мимо окна может
		data, err := printprof.Encode(p)
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	for _, f := range []struct {
		name string
		data []byte
	}{
		{"Первая", mk("own-0000000a", "Первая")},
		{"Мусор", []byte("{не json")},
		{"Дубль имени", mk("own-0000000b", "Первая")},
		{"Чужое имя", mk("own-0000000c", "Фабрика Фотокниги")},
		{"Вторая", mk("own-0000000d", "Вторая")},
	} {
		if _, err := s.SavePrint("", f.name, f.data); err != nil {
			t.Fatal(err)
		}
	}
	a.reloadPrints()
	var got []string
	for _, in := range a.PrintProfiles() {
		if in.Source == printprof.SourceOwn {
			got = append(got, in.Name)
		}
	}
	if !slices.Equal(got, []string{"Первая", "Вторая"}) {
		t.Fatalf("свои в реестре: %q", got)
	}
	out := logBuf.String()
	for _, want := range []string{`пропущена: профиль: это не JSON`, `"own-0000000b" пропущена`, `"own-0000000c" пропущена`} {
		if !strings.Contains(out, want) {
			t.Errorf("в log.txt нет %q:\n%s", want, out)
		}
	}
	// Без библиотеки (startup не дошёл): остаются встроенные, паники нет.
	b := newApp(t)
	b.reloadPrints()
	if len(printprof.Current().Profiles()) != 3 {
		t.Errorf("без библиотеки: %d типографий", len(printprof.Current().Profiles()))
	}
}

// Ревью 5: основа — уже своя: хвост «(своя)»/«(своя N)» не удваивается; несколько подряд.
func TestPrintProfileEditOwnBaseName(t *testing.T) {
	a, _ := printApp(t)
	alpha := saved(t, a, editOf(t, a, "fabrika", "Альфа (своя)"))
	e, err := a.PrintProfileEdit(alpha.ID)
	if err != nil || e.Name != "Альфа (своя 2)" {
		t.Fatalf("на основе «Альфа (своя)»: %q", e.Name)
	}
	e.Name = "Альфа (своя 2)"
	two := saved(t, a, e)
	if e3, _ := a.PrintProfileEdit(two.ID); e3.Name != "Альфа (своя 3)" {
		t.Errorf("на основе «Альфа (своя 2)»: %q", e3.Name)
	}
	// Хвост только в конце и с пробелом: середина названия и «(своя)» без пробела не режутся.
	for name, want := range map[string]string{"Бета (своя) плюс": "Бета (своя) плюс (своя)", "(своя)": "(своя) (своя)"} {
		res := saved(t, a, editOf(t, a, "fabrika", name))
		if got, _ := a.PrintProfileEdit(res.ID); got.Name != want {
			t.Errorf("%q → %q, ждали %q", name, got.Name, want)
		}
	}
}

// QA-2: плотность негодна — одна ошибка у dpi; строки без своей ошибки и без «Исправьте строки».
// Строка с собственной ошибкой (подпись) остаётся с ошибкой и признаком поля.
func TestCheckPrintProfileNoDPI(t *testing.T) {
	a, _ := printApp(t)
	for _, dpi := range []int{0, -1, 71, 1201} {
		e := editOf(t, a, "fabrika", "Без плотности")
		e.DPI = dpi
		c := a.CheckPrintProfile(e)
		if c.OK || c.Errs["dpi"] == "" || len(c.Errs) != 1 {
			t.Fatalf("dpi %d: ошибки %v", dpi, c.Errs)
		}
		if len(c.Formats) != len(e.Formats) || len(c.Lines) != 0 {
			t.Fatalf("dpi %d: строк %d, строк альбома %q", dpi, len(c.Formats), c.Lines)
		}
		for i, r := range c.Formats {
			if r.Err != "" || r.Field != "" || r.Hit || r.PageW != 0 {
				t.Errorf("dpi %d, строка %d: %+v", dpi, i, r)
			}
		}
		e.Formats[1].Name = e.Formats[0].Name
		c = a.CheckPrintProfile(e)
		if r := c.Formats[1]; r.Err == "" || r.Field != "name" || c.Errs["formats"] == "" || c.Formats[0].Err != "" {
			t.Errorf("dpi %d, повтор подписи: %+v %v", dpi, r, c.Errs)
		}
	}
}

// Ревью 8: признаки строки — поле с ошибкой (name | px | cover) и совпадение с альбомом (hit) —
// у нескольких строк сразу; hit только у первой годной совпавшей.
func TestCheckPrintProfileRowFlags(t *testing.T) {
	a, _ := printApp(t)
	p, _ := a.editor().Snapshot()
	p.Settings.PageW, p.Settings.PageH = 304, 203
	a.ed.Store(mustEditor(t, p, ""))
	e := editOf(t, a, "fabrika", "Признаки")
	hit := slices.IndexFunc(e.Formats, func(f PrintEditFormat) bool { return f.Name == "30×20" })
	if hit < 0 {
		t.Fatal("нет формата 30×20")
	}
	same := e.Formats[hit]
	same.Name = "Копия"
	e.Formats = append(e.Formats,
		same,                               // те же px, но ниже совпавшей: без hit
		PrintEditFormat{Name: ""},          // подпись
		PrintEditFormat{Name: "x", PxW: 0}, // px
		PrintEditFormat{Name: "y", PxW: 3000, PxH: 2000, CoverW: 10}) // обложка
	n := len(e.Formats)
	c := a.CheckPrintProfile(e)
	if len(c.Formats) != n {
		t.Fatalf("строк %d", len(c.Formats))
	}
	want := map[int]string{n - 3: "name", n - 2: "px", n - 1: "cover"}
	for i, r := range c.Formats {
		if r.Field != want[i] || (r.Err == "") != (want[i] == "") || r.Hit != (i == hit) {
			t.Errorf("строка %d: %+v, ждали поле %q hit=%v", i, r, want[i], i == hit)
		}
	}
	if len(c.Lines) == 0 || !strings.HasSuffix(c.Lines[0], "— это формат «30×20»") {
		t.Errorf("строка альбома: %q", c.Lines)
	}
	// Первая совпавшая разошлась на px — hit у следующей годной; у той повтор подписи — hit нет ни у кого.
	e.Formats[hit].PxW++
	if c = a.CheckPrintProfile(e); c.Formats[hit].Hit || !c.Formats[n-4].Hit {
		t.Errorf("hit после сдвига: %+v / %+v", c.Formats[hit], c.Formats[n-4])
	}
	e.Formats[n-4].Name = e.Formats[0].Name
	if c = a.CheckPrintProfile(e); slices.ContainsFunc(c.Formats, func(r PrintCheckRow) bool { return r.Hit }) ||
		strings.Contains(c.Lines[0], "это формат") {
		t.Errorf("hit у строки с ошибкой: %q", c.Lines)
	}
}
