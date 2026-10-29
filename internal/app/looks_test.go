package app

import (
	"errors"
	"net/http"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// testLook — макет: титул, повторяемый шаг, финал и обложка; в каждом шаге два места.
func testLook(color string) model.AlbumTpl {
	tpl := model.SpreadTpl{V: model.TplV, Pages: 2, Ratio: 2, Places: []model.Box{{X: 0.05, Y: 0.1, W: 0.4, H: 0.8}, {X: 0.55, Y: 0.1, W: 0.4, H: 0.8}},
		Page: []model.TplBg{{Color: color}, {Color: color}}}
	side := model.SpreadTpl{V: model.TplV, Pages: 1, Ratio: 1, Places: []model.Box{{X: 0.2, Y: 0.2, W: 0.6, H: 0.6}}, Page: []model.TplBg{{Color: color}}}
	return model.AlbumTpl{V: model.LookV,
		Cover: &model.CoverTpl{Bg: &model.TplBg{Color: color}, Front: &side},
		Steps: []model.LookStep{{Name: "Титул", Tpl: tpl}, {Role: model.RoleRepeat, Tpl: tpl}, {Name: "Финал", Tpl: tpl}}}
}

// lookApp — App с библиотекой и пустым альбомом из трёх разворотов, подключённым к библиотеке макетов.
func lookApp(t *testing.T) *App {
	t.Helper()
	a := decorApp(t)
	p := model.New()
	p.Spreads = []*model.Spread{{ID: "s1"}, {ID: "s2"}, {ID: "s3"}}
	a.setEditor(mustEditor(t, p, ""))
	return a
}

// reloaded — проект после круга Save→Load (файл .salbum).
func reloaded(t *testing.T, p *model.Project) *model.Project {
	t.Helper()
	path := filepath.Join(testkit.TempDir(t), "a.salbum")
	if err := salbum.Save(p, path); err != nil {
		t.Fatal(err)
	}
	q, err := salbum.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	return q
}

func saveLook(t *testing.T, a *App, name, color string) string {
	t.Helper()
	s, err := a.decorStore()
	if err != nil {
		t.Fatal(err)
	}
	it, err := s.SaveLook("mine", name, testLook(color))
	if err != nil {
		t.Fatal(err)
	}
	return it.ID
}

func TestLookInfoCount(t *testing.T) {
	a := lookApp(t)
	id := saveLook(t, a, "Свадьба", "#ffffff")
	info, err := a.LookInfo(id)
	if err != nil || len(info.Steps) != 3 || !info.Cover || info.Min != 3 || info.Max != model.MaxSpreads {
		t.Fatalf("LookInfo: %+v %v", info, err)
	}
	if st := info.Steps[1]; st.Role != "repeat" || st.Places != 2 || st.Pages != 2 || st.Name != "" {
		t.Fatalf("шаг 2: %+v", st)
	}
	c, err := a.LookCount(id, 5, 7)
	if err != nil || c.N != 5 || c.Places != 10 || c.Fit != 4 { // 7 фото: титул 2 + финал 2 + повторов 2 по 2 места
		t.Fatalf("LookCount: %+v %v", c, err)
	}
	for _, n := range []int{-5, 0, 1000000} { // края: зажим 1…MaxSpreads, k < 0
		c, err = a.LookCount(id, n, -3)
		if err != nil || c.N < 3 || c.N > model.MaxSpreads || c.Fit < 3 {
			t.Fatalf("LookCount(%d): %+v %v", n, c, err)
		}
	}
	for _, bad := range []string{"", "sa:x", "my:zz", "my:0000000000000000", "../x"} {
		if _, err := a.LookInfo(bad); err == nil {
			t.Errorf("LookInfo(%q) без ошибки", bad)
		}
		if _, err := a.LookCount(bad, 3, 3); err == nil {
			t.Errorf("LookCount(%q) без ошибки", bad)
		}
	}
}

func TestApplyLookSeries(t *testing.T) {
	a := lookApp(t)
	id, id2 := saveLook(t, a, "Свадьба", "#ffffff"), saveLook(t, a, "Школа", "#eeeeee")
	before := projectJSON(t, a)
	plan, err := a.PlanLook(id, model.LookOpts{Mode: "series", Spreads: 5})
	if err != nil || plan.Spreads != 5 || plan.Places != 10 || plan.Full {
		t.Fatalf("PlanLook: %+v %v", plan, err)
	}
	if projectJSON(t, a) != before {
		t.Fatal("PlanLook изменил альбом")
	}
	r, err := a.ApplyLook(id, model.LookOpts{Mode: "series", Spreads: 5})
	if err != nil || len(r.View.Spreads) != 5 || r.Plan.Spreads != 5 {
		t.Fatalf("ApplyLook: %d %+v %v", len(r.View.Spreads), r.Plan, err)
	}
	if l := r.View.Look; l == nil || l.Key != id || l.Name != "Свадьба" || !l.Found {
		t.Fatalf("View.look: %+v", l)
	}
	if r.View.Spreads[1].LookStep != 2 {
		t.Fatalf("lookStep: %d", r.View.Spreads[1].LookStep)
	}
	// Круг Save→Load: макет и номера шагов живут в файле.
	p, _ := a.editor().Snapshot()
	q := reloaded(t, p)
	if q.Look == nil || q.Look.Key != id || q.Spreads[1].LookStep != 2 {
		t.Fatalf("после круга: %+v", q.Look)
	}
	// Повтор — холостая правка: альбом тот же, шаг отмены один.
	after := projectJSON(t, a)
	if _, err = a.ApplyLook(id, model.LookOpts{Mode: "series", Spreads: 5}); err != nil || projectJSON(t, a) != after {
		t.Fatalf("повтор менял альбом: %v", err)
	}
	if v := a.Undo(); len(v.Spreads) != 3 || projectJSON(t, a) != before {
		t.Fatalf("отмена одним шагом: %d разворотов", len(v.Spreads))
	}
	// Второй макет поверх первого и зажим числа: 0 → 1 (серия не меньше шагов), 1e6 → MaxSpreads.
	a.Redo()
	for _, n := range []int{0, 1_000_000} {
		r, err = a.ApplyLook(id2, model.LookOpts{Spreads: n})
		if err != nil || len(r.View.Spreads) < 3 || len(r.View.Spreads) > model.MaxSpreads || r.View.Look.Key != id2 {
			t.Fatalf("n=%d: %d %v", n, len(r.View.Spreads), err)
		}
	}
	if _, err = a.ApplyLook(id, model.LookOpts{Mode: "всё сразу", Spreads: 3}); err == nil {
		t.Error("неизвестный режим принят")
	}
	if _, err = a.PlanLook(id, model.LookOpts{Mode: "x"}); err == nil {
		t.Error("PlanLook: неизвестный режим принят")
	}
}

func TestLookSheetRefused(t *testing.T) {
	a := lookApp(t)
	id := saveLook(t, a, "Свадьба", "#ffffff")
	a.setEditor(mustEditor(t, album.NewVignetteSheet(300, 400), ""))
	before := projectJSON(t, a)
	if _, err := a.PlanLook(id, model.LookOpts{Mode: "series", Spreads: 3}); !errors.Is(err, errLookSheet) {
		t.Errorf("PlanLook: %v", err)
	}
	if _, err := a.ApplyLook(id, model.LookOpts{Mode: "series", Spreads: 3}); !errors.Is(err, errLookSheet) {
		t.Errorf("ApplyLook: %v", err)
	}
	sid := a.editor().View().Spreads[0].ID
	if _, err := a.PlanLookStep(sid, id, 1); !errors.Is(err, errLookSheet) {
		t.Errorf("PlanLookStep: %v", err)
	}
	if _, err := a.ApplyLookStep(sid, id, 1); !errors.Is(err, errLookSheet) {
		t.Errorf("ApplyLookStep: %v", err)
	}
	if _, err := a.NewLookSpread(); !errors.Is(err, errLookSheet) {
		t.Errorf("NewLookSpread: %v", err)
	}
	if projectJSON(t, a) != before {
		t.Error("лист изменился")
	}
}

func TestLookStepAndNewSpread(t *testing.T) {
	a := lookApp(t)
	id := saveLook(t, a, "Свадьба", "#ffffff")
	if _, err := a.NewLookSpread(); !errors.Is(err, album.ErrNoLook) {
		t.Fatalf("альбом без макета: %v", err)
	}
	plan, err := a.PlanLookStep("s2", id, 2)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Left != 0 {
		t.Fatalf("план: %+v", plan)
	}
	r, err := a.ApplyLookStep("s2", id, 2)
	if err != nil || r.View.Spreads[1].LookStep != 2 || len(r.View.Spreads[1].Places) != 2 || r.View.Look == nil || !r.View.Look.Found {
		t.Fatalf("ApplyLookStep: %+v %v", r.View.Look, err)
	}
	for _, c := range []struct {
		sp   string
		step int
	}{{"s2", 0}, {model.CoverID, 1}, {"s2", 4}, {"s2", -1}} { // обложка — G5; шага нет
		before := projectJSON(t, a)
		if _, err := a.ApplyLookStep(c.sp, id, c.step); err == nil || projectJSON(t, a) != before {
			t.Errorf("%+v: err=%v", c, err)
		}
	}
	n := len(a.editor().View().Spreads)
	v, err := a.NewLookSpread()
	if err != nil || len(v.Spreads) != n+1 {
		t.Fatalf("NewLookSpread: %d→%d %v", n, len(v.Spreads), err)
	}
	if v.Seq == 0 { // вид идёт по цепочке событий (a.changed), иначе фронт делает resync
		t.Fatalf("вид мимо цепочки: Seq=%d", v.Seq)
	}
	if v2, err := a.NewLookSpread(); err != nil || v2.Seq <= v.Seq {
		t.Fatalf("второй разворот: Seq %d → %d, %v", v.Seq, v2.Seq, err)
	} else {
		a.Undo()
	}
	if v = a.Undo(); len(v.Spreads) != n {
		t.Fatalf("отмена NewLookSpread: %d", len(v.Spreads))
	}
	// Макет альбома пропал с этого ПК: понятная ошибка с именем, found = false.
	p, _ := a.editor().Snapshot()
	p.Look = &model.LookRef{Key: "my:ffffffffffffffff", Name: "Чужой"}
	a.setEditor(mustEditor(t, p, ""))
	if _, err = a.NewLookSpread(); err == nil || !strings.Contains(err.Error(), "«Чужой» нет в библиотеке этого компьютера") {
		t.Fatalf("макета нет: %v", err)
	}
	if l := a.editor().View().Look; l == nil || l.Found || l.Name != "Чужой" {
		t.Fatalf("found: %+v", l)
	}
}

// Смена формата альбома с макетом: Editor сам находит макет по Project.Look (0.28-131-53) —
// итог равен явному SetSettingsLook и отличается от формата без макета.
func TestSetSettingsFindsLook(t *testing.T) {
	a := lookApp(t)
	id := saveLook(t, a, "Свадьба", "#ffffff")
	if _, err := a.ApplyLook(id, model.LookOpts{Mode: "series", Spreads: 4}); err != nil {
		t.Fatal(err)
	}
	p0, _ := a.editor().Snapshot()
	clone := func() *album.Editor { return mustEditor(t, reloaded(t, p0), "") }
	ns := p0.Settings
	ns.PageW, ns.PageH = 300, 150 // другая пропорция страницы
	tpl := testLook("#ffffff")
	want, bare := clone(), clone()
	want.SetSettingsLook(ns, &tpl)
	bare.SetSettings(ns) // без подключённой библиотеки — места тянутся как всегда
	a.SetSettings(ns)
	pa, _ := a.editor().Snapshot()
	pw, _ := want.Snapshot()
	pb, _ := bare.Snapshot()
	ja, jw, jb := string(model.Marshal(pa)), string(model.Marshal(pw)), string(model.Marshal(pb))
	if ja != jw {
		t.Error("SetSettings не вписал макет так же, как SetSettingsLook")
	}
	if jw == jb {
		t.Error("тест не различает вписывание и растяжение мест")
	}
	// SetSettingsToAll и ApplyPrint вписывают макет так же: места макетных разворотов — как после SetSettingsLook.
	placesOf := func(ed *album.Editor) (out [][]model.PlaceView) {
		for _, sp := range ed.View().Spreads {
			out = append(out, sp.Places)
		}
		return out
	}
	for name, run := range map[string]func(ed *album.Editor){
		"SetSettingsToAll": func(ed *album.Editor) { ed.SetSettingsToAll(ns) },
		"ApplyPrint": func(ed *album.Editor) {
			if _, err := ed.ApplyPrint(ns, "netprint-royal"); err != nil {
				t.Fatalf("ApplyPrint: %v", err)
			}
		},
	} {
		ed := mustEditor(t, reloaded(t, p0), "")
		ed.SetLookHost(a.lookHost())
		run(ed)
		if !reflect.DeepEqual(placesOf(ed), placesOf(want)) {
			t.Errorf("%s: места не вписаны по макету", name)
		}
		if reflect.DeepEqual(placesOf(ed), placesOf(bare)) {
			t.Errorf("%s: тест не различает вписывание и растяжение", name)
		}
	}
}

func TestRenameHideLookDecor(t *testing.T) {
	a := lookApp(t)
	id := saveLook(t, a, "Свадьба", "#ffffff")
	other := saveLook(t, a, "Школа", "#eeeeee")
	names := func() map[string]string {
		c, err := a.DecorCatalog("album")
		if err != nil {
			t.Fatal(err)
		}
		m := map[string]string{}
		for _, it := range c.Items {
			m[it.ID] = it.Name
		}
		return m
	}
	if m := names(); m[id] != "Свадьба" || m[other] != "Школа" {
		t.Fatalf("каталог: %v", m)
	}
	if err := a.RenameDecor("album", id, "  Новая  "); err != nil || names()[id] != "Новая" {
		t.Fatalf("RenameDecor: %v %v", err, names())
	}
	if err := a.RenameDecor("album", id, " "); err == nil {
		t.Error("пустое имя принято")
	}
	if err := a.HideDecor("album", id); err != nil {
		t.Fatal(err)
	}
	if m := names(); m[id] != "" || m[other] != "Школа" {
		t.Fatalf("после HideDecor: %v", m)
	}
	// Скрытый макет остаётся на ПК: альбом, оформленный им, продолжает его находить.
	if _, err := a.ApplyLook(id, model.LookOpts{Mode: "series", Spreads: 3}); err != nil {
		t.Fatalf("скрытый макет не ставится: %v", err)
	}
}

func getLook(a *App, method, url string) int {
	return getTpl(a, method, url).Code
}

func TestMediaLook(t *testing.T) {
	a := lookApp(t)
	id := saveLook(t, a, "Свадьба", "#336699")
	hex := strings.TrimPrefix(id, "my:")
	base := "/media/look/" + hex
	ok := []string{"?v=1&i=0&px=256&r=1.5&bg=0", "?v=1&i=1&px=64&r=0.2&bg=1", "?i=3&px=1024&r=5", "?v=1&i=2&px=300&r=2&bg=0", "?i=0&px=128&r=1&bg=1",
		"?v=1&i=0&px=256&r=0.75&bg=0&f=1", "?i=0&px=64&r=0.2&f=0", "?i=0&px=64&r=5&bg=1&f=1", "?i=0&px=65&r=0.2"}
	for _, q := range ok {
		rec := getTpl(a, http.MethodGet, base+q)
		if rec.Code != 200 || rec.Header().Get("Content-Type") != "image/png" || rec.Body.Len() == 0 {
			t.Errorf("%s: %d", q, rec.Code)
		}
	}
	if getLook(a, http.MethodHead, base+ok[0]) != 200 {
		t.Error("HEAD")
	}
	// Мусор: путь, параметры, чужой метод, ключ не из индекса, шаг вне макета — 404 без паники.
	bad := []string{
		"?i=0&px=256", "?px=256&r=1", "?i=1&r=1", // обязательных нет
		"?i=25&px=256&r=1", "?i=-1&px=256&r=1", "?i=x&px=256&r=1", "?i=1&px=63&r=1", "?i=1&px=1025&r=1",
		"?i=1&px=256&r=0.19", "?i=1&px=256&r=5.01", "?i=1&px=256&r=NaN", "?i=1&px=256&r=Inf", "?i=1&px=256&r=1e400",
		"?i=1&px=256&r=1&bg=2", "?i=1&px=256&r=1&bg=", "?i=1&px=256&r=1&f=1", "?i=0&px=256&r=1&f=2", "?i=0&px=256&r=1&f=", "?i=0&px=256&r=1&f=1&f=1", "?i=1&i=2&px=256&r=1", "?i=1&px=256&r=1&x=1", "?i=1&px=256&r=1&v=x",
		"?i=4&px=256&r=1", "?i=1&px=%zz&r=1", "?%zz", "?i=1&px=256&r=1&path=../../etc/passwd",
	}
	for _, q := range bad {
		if c := getLook(a, http.MethodGet, base+q); c != 404 {
			t.Errorf("%s: %d, ждали 404", q, c)
		}
	}
	good := "?i=1&px=256&r=1"
	for _, path := range []string{
		"/media/look/", "/media/look/zzzzzzzzzzzzzzzz", "/media/look/0000000000000000", "/media/look/" + strings.ToUpper(hex),
		"/media/look/" + hex + "/", "/media/look/" + hex + "x", "/media/look/../" + hex, "/media/look/my:" + hex, "/media/look/%2e%2e%2f" + hex,
		"/media/look/" + strings.Repeat("a", 5000),
	} {
		if c := getLook(a, http.MethodGet, path+good); c != wantRefusal(path) {
			t.Errorf("%s: %d, ждали %d", path[:min(len(path), 60)], c, wantRefusal(path))
		}
	}
	if c := getLook(a, http.MethodPost, base+good); c != 404 {
		t.Errorf("POST: %d", c)
	}
	// Макет без обложки: i=0 — 404.
	noCover := testLook("#112233")
	noCover.Cover = nil
	s, _ := a.decorStore()
	it, err := s.SaveLook("mine", "Без обложки", noCover)
	if err != nil {
		t.Fatal(err)
	}
	if c := getLook(a, http.MethodGet, "/media/look/"+strings.TrimPrefix(it.ID, "my:")+"?i=0&px=256&r=1"); c != 404 {
		t.Errorf("обложки нет: %d", c)
	}
}

func TestLookTile(t *testing.T) {
	l := testLook("#ffffff")
	l.Steps = append(l.Steps, model.LookStep{Role: model.RoleVignette, Tpl: l.Steps[0].Tpl, Vig: &model.LookVig{Tpl: "classic", Teachers: 1, Pupils: 4}})
	l.Steps[0].Tpl.Texts = []model.TplText{{Text: "Привет", W: 0.5}}
	one := func(i int, bg, front bool, r float64) (model.SpreadTpl, bool) {
		t.Helper()
		tp, ok := lookTile(l, i, bg, front, r)
		if ok && len(tp) != 1 {
			t.Fatalf("шаг %d: шаблонов %d, ждали 1", i, len(tp))
		}
		if !ok {
			return model.SpreadTpl{}, false
		}
		return tp[0], true
	}
	if tp, ok := one(1, true, false, 1.5); !ok || len(tp.Places) != 0 || len(tp.Texts) != 0 || tp.Page[0].Color != "#ffffff" {
		t.Errorf("bg: %+v", tp)
	}
	if tp, ok := one(1, false, false, 1); !ok || len(tp.Places) != 2 || len(tp.Texts) != 1 || tp.Places[0].Y == l.Steps[0].Tpl.Places[0].Y {
		t.Errorf("места по LookBoxes для r=1: %+v", tp.Places) // композиция Ratio 2 в квадрате ниже
	}
	if tp, ok := one(4, false, false, 1.5); !ok || len(tp.Places) != 0 {
		t.Errorf("виньетка: %+v", tp)
	}
	// Обложка: две стороны рядом (задней у testLook нет — пустая страница с фоном листа).
	cv, ok := lookTile(l, 0, false, false, 1.5)
	if !ok || len(cv) != 2 || len(cv[0].Places) != 0 || len(cv[1].Places) != 1 || cv[0].Page[0].Color != "#ffffff" || cv[1].Pages != 1 {
		t.Fatalf("обложка: %+v", cv)
	}
	// Места лицевой — в её зоне (вся сторона, отношение r/2 = 0,75): Ratio 1 вписывается по высоте, по центру.
	if want := model.LookBoxes(*l.Cover.Front, 0.75); !reflect.DeepEqual(cv[1].Places, want) {
		t.Errorf("места лицевой: %v, ждали %v", cv[1].Places, want)
	}
	if fr, ok := lookTile(l, 0, false, true, 0.75); !ok || len(fr) != 1 || !reflect.DeepEqual(fr[0].Places, cv[1].Places) {
		t.Errorf("f=1 (r — отношение стороны) совпадает с правой частью листа: %+v", fr)
	}
	if bgT, ok := lookTile(l, 0, true, false, 1.5); !ok || len(bgT) != 1 || bgT[0].Pages != 2 || len(bgT[0].Places) != 0 {
		t.Errorf("фон листа обложки — один разворот: %+v", bgT)
	}
	if bgF, ok := lookTile(l, 0, true, true, 0.75); !ok || len(bgF) != 1 || bgF[0].Pages != 1 || len(bgF[0].Places) != 0 {
		t.Errorf("фон лицевой — одна страница без мест: %+v", bgF)
	}
	for _, i := range []int{-1, 5, 25} {
		if _, ok := lookTile(l, i, false, false, 1); ok {
			t.Errorf("шаг %d", i)
		}
	}
	l.Cover = nil
	for _, front := range []bool{false, true} {
		if _, ok := lookTile(l, 0, false, front, 1); ok {
			t.Error("обложки нет")
		}
	}
}

func TestMediaLookFast(t *testing.T) {
	l := testLook("#336699")
	var worst time.Duration
	for i := range 4 {
		tp, ok := lookTile(l, i, false, false, 1.5)
		if !ok {
			t.Fatal(i)
		}
		worst = max(worst, testkit.Fastest(3, func() {
			if _, err := drawLook(tp, tplQuery{px: 256, r: 1.5}, nil); err != nil {
				t.Fatal(err)
			}
		}))
	}
	testkit.CostTest(t)
	if limit := testkit.Limit(30 * time.Millisecond); worst > limit {
		t.Errorf("плитка %v, предел %v", worst, limit)
	}
}

// Разбор запроса плитки макета: принятое — в пределах белого списка.
func FuzzMediaLookQuery(f *testing.F) {
	for _, s := range []string{"v=1&i=0&px=256&r=2&bg=0", "i=24&px=64&r=0.2&bg=1", "i=1&px=1024&r=5", "i=1&px=1&r=1e400", "i=%zz", "v=99999&i=1&px=256&r=1", "i=0&px=256&r=0.5&f=1", "i=1&px=256&r=1&f=1", ""} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, raw string) {
		q, ok := parseLookQuery(raw)
		if ok && q.front && q.i != 0 {
			t.Fatalf("%q: f=1 только у обложки: %+v", raw, q)
		}
		if ok && (q.i < 0 || q.i > model.MaxLookSteps || q.px < tplMinPx || q.px > tplMaxPx || !(q.r >= tplMinR && q.r <= tplMaxR)) {
			t.Fatalf("%q: %+v", raw, q)
		}
	})
}

// Ссылка альбома на макет находится по ключу и по записи набора; чужое и мусор — нет.
func TestLookKeyOfRefs(t *testing.T) {
	a := lookApp(t)
	id := saveLook(t, a, "Свадьба", "#ffffff")
	s, _ := a.decorStore()
	ref, ok := s.LookRefOf(id)
	if !ok || ref.Key != id || ref.Name != "Свадьба" || ref.Pack != "" {
		t.Fatalf("LookRefOf: %+v %v", ref, ok)
	}
	if k, ok := s.LookKeyOf(ref); !ok || k != id || !s.HasLook(ref) {
		t.Fatalf("LookKeyOf: %s %v", k, ok)
	}
	for _, r := range []model.LookRef{{}, {Key: "my:ffffffffffffffff"}, {Key: "../x"}, {Key: "my:ffffffffffffffff", Pack: "nope", Item: "looks/main.json"}, {Key: "x", Pack: "wedding", Item: "../x"}} {
		if k, ok := s.LookKeyOf(r); ok || s.HasLook(r) {
			t.Errorf("%+v найден: %s", r, k)
		}
	}
	if _, ok := s.LookRefOf("sa:x"); ok {
		t.Error("LookRefOf(sa:x)")
	}
}

// Путь смены проекта (retire/openPath): открытый файл с макетом получает библиотеку макетов —
// found и вписывание макета при смене формата работают и в нём.
func TestOpenedAlbumKnowsLooks(t *testing.T) {
	a := lookApp(t)
	id := saveLook(t, a, "Свадьба", "#ffffff")
	if _, err := a.ApplyLook(id, model.LookOpts{Mode: "series", Spreads: 4}); err != nil {
		t.Fatal(err)
	}
	p0, _ := a.editor().Snapshot()
	path := filepath.Join(testkit.TempDir(t), "look.salbum")
	if err := salbum.Save(p0, path); err != nil {
		t.Fatal(err)
	}
	a.setEditor(mustEditor(t, model.New(), "")) // другой альбом
	if _, err := a.openPath(path); err != nil {
		t.Fatal(err)
	}
	if l := a.editor().View().Look; l == nil || !l.Found || l.Key != id {
		t.Fatalf("открытый альбом: %+v", l)
	}
	ns := p0.Settings
	ns.PageW, ns.PageH = 300, 150
	tpl := testLook("#ffffff")
	want := mustEditor(t, reloaded(t, p0), "")
	want.SetSettingsLook(ns, &tpl)
	a.SetSettings(ns)
	got, exp := a.editor().View().Spreads, want.View().Spreads
	for i := range exp {
		if !reflect.DeepEqual(got[i].Places, exp[i].Places) {
			t.Fatalf("разворот %d: места не вписаны по макету после открытия файла", i)
		}
	}
}

// Макет без повтора и с шагом-виньеткой: границы числа разворотов и страницы в карточке.
func TestLookInfoNoRepeatVignette(t *testing.T) {
	a := lookApp(t)
	l := testLook("#ffffff")
	l.Steps = []model.LookStep{l.Steps[0], {Role: model.RoleVignette, Tpl: l.Steps[0].Tpl, Vig: &model.LookVig{Tpl: "classic", Teachers: 1, Pupils: 4}}, l.Steps[2]}
	s, _ := a.decorStore()
	it, err := s.SaveLook("mine", "Класс", l)
	if err != nil {
		t.Fatal(err)
	}
	info, err := a.LookInfo(it.ID)
	if err != nil || len(info.Steps) != 3 || info.Min != 3 || info.Max != 3 {
		t.Fatalf("без повтора min=max=3: %+v %v", info, err)
	}
	if st := info.Steps[1]; st.Role != "vignette" || st.Pages != 1 || st.Places != 5 {
		t.Fatalf("виньетка: %+v", st)
	}
	if info.Steps[0].Pages != 2 {
		t.Fatalf("разворот: %+v", info.Steps[0])
	}
	page := l
	page.Steps = []model.LookStep{l.Steps[0], l.Steps[2]}
	page.Steps[0].Tpl.Pages, page.Steps[0].Tpl.Page = 1, page.Steps[0].Tpl.Page[:1]
	it, err = s.SaveLook("mine", "Страницы", page)
	if err != nil {
		t.Fatal(err)
	}
	if info, err = a.LookInfo(it.ID); err != nil || info.Steps[0].Pages != 1 || info.Steps[0].Places != 4 || info.Steps[1].Pages != 2 {
		t.Fatalf("страница зеркалится: %+v %v", info.Steps, err)
	}
	c, err := a.LookCount(it.ID, 9, 3)
	if err != nil || c.N != 2 || c.Places != 6 {
		t.Fatalf("без повтора серия фиксирована: %+v %v", c, err)
	}
}

// Плитка попадает в недавние после ApplyLook и ApplyLookStep; прежний макет альбома даёт свои файлы библиотеки.
func TestLookRecentAndPrevLibs(t *testing.T) {
	a := lookApp(t)
	id := saveLook(t, a, "Свадьба", "#ffffff")
	other := saveLook(t, a, "Школа", "#eeeeee")
	recent := func() []string { return prefs.RecentDecor(a.cfgDir, "album") }
	if len(recent()) != 0 {
		t.Fatalf("до: %v", recent())
	}
	if _, err := a.ApplyLookStep("s1", id, 1); err != nil || !slices.Equal(recent(), []string{id}) {
		t.Fatalf("после шага: %v %v", recent(), err)
	}
	if _, err := a.ApplyLook(other, model.LookOpts{Mode: "series", Spreads: 3}); err != nil || !slices.Equal(recent(), []string{other, id}) {
		t.Fatalf("после ApplyLook: %v %v", recent(), err)
	}
	// Прежний макет альбома: его свой фон входит в libs (по ним сверяется «своё» фона).
	bg := importBg(t, a, testkit.JPEG(t, testkit.TempDir(t), "Фон.jpg", 60, 40))[0]
	prev := testLook("#ffffff")
	prev.Steps[0].Tpl.Page[0].Lib = bg
	s, _ := a.decorStore()
	it, err := s.SaveLook("mine", "С фоном", prev)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = a.ApplyLook(it.ID, model.LookOpts{Mode: "series", Spreads: 3}); err != nil {
		t.Fatal(err)
	}
	t2, _ := s.Look(other)
	if libs := a.lookLibsPrev(s, t2); libs[bg].Path == "" {
		t.Fatalf("файл фона прежнего макета не в libs: %v", libs)
	}
}
