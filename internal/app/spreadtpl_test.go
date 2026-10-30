package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"image/color"
	"image/png"
	"math"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/decor"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// spreadApp — App с библиотекой: s1 «Авто» из пяти фото, s2 и s3 пустые.
func spreadApp(t *testing.T) *App {
	t.Helper()
	a := decorApp(t)
	p := model.New()
	ids := []string{"p1", "p2", "p3", "p4", "p5"}
	for _, id := range ids {
		p.Photos[id] = model.PhotoRef{Path: testkit.AbsPath(id + ".jpg"), W: 300, H: 200}
	}
	p.Spreads = []*model.Spread{{ID: "s1", Photos: ids}, {ID: "s2"}, {ID: "s3"}}
	a.ed.Store(mustEditor(t, p, ""))
	return a
}

func projectJSON(t *testing.T, a *App) string {
	t.Helper()
	p, _ := a.editor().Snapshot()
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// Плитка разворота: вопрос до правки, места и оформление — один шаг отмены, повтор — холостой
// без шага, круг Save→Load; страница на весь разворот — обе страницы; отказы — без правки.
func TestApplySpreadTpl(t *testing.T) {
	a := spreadApp(t)
	before := projectJSON(t, a)
	rev := a.editor().View().Rev
	const hero = "sa:tpl-ivory-hero"
	plan, err := a.PlanSpreadTpl("s1", "spread", hero)
	if err != nil || plan.Left != 2 || plan.Frozen != 1 {
		t.Fatalf("вопрос: %+v, %v", plan, err)
	}
	if a.editor().View().Rev != rev {
		t.Fatal("вопрос изменил проект")
	}
	r, err := a.ApplySpreadTpl("s1", "spread", hero)
	s := r.View.Spreads[0]
	if err != nil || r.Left != 2 || r.Missing != 0 || r.View.Rev != rev+1 || !s.Grid || len(s.Places) != 3 ||
		len(s.Patterns) != 1 || s.Patterns[0].Preset != "sa:ivory-glow" || len(s.Stickers) != 1 || s.Bg[0] != "#fffaf0" {
		t.Fatalf("поставить: %v %+v", err, r)
	}
	if !slices.Equal(prefs.RecentDecor(a.cfgDir, decor.KindSpread), []string{hero}) {
		t.Errorf("недавние %v", prefs.RecentDecor(a.cfgDir, decor.KindSpread))
	}
	after := projectJSON(t, a)
	if again, err := a.ApplySpreadTpl("s1", "spread", hero); err != nil || again.View.Rev != r.View.Rev || again.Left != 0 {
		t.Fatalf("повтор не холостой: %v rev %d→%d", err, r.View.Rev, again.View.Rev)
	}
	path := filepath.Join(testkit.TempDir(t), "a.salbum")
	if err := a.editor().Save(path); err != nil {
		t.Fatal(err)
	}
	p, err := salbum.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if sp := p.Spreads[0]; len(sp.Grid) != 3 || sp.Pat == nil || sp.Pat.Preset != "sa:ivory-glow" || len(sp.Stickers) != 1 || len(sp.Photos) != 3 {
		t.Fatalf("после Load: %+v", sp)
	}
	a.Undo() // один шаг: повтор шага не добавил
	if got := projectJSON(t, a); got != before {
		t.Fatalf("отмена не вернула проект:\n%s\n%s", before, got)
	}
	a.Redo()
	if got := projectJSON(t, a); got != after {
		t.Fatal("возврат не тот")
	}
	// Страница на весь разворот: левая как есть, правая зеркально; на одну страницу — только она.
	r, err = a.ApplySpreadTpl("s2", "spread", "sa:tpl-rose-page")
	if err != nil || len(r.View.Spreads[1].Places) != 4 || len(r.View.Spreads[1].Stickers) != 2 {
		t.Fatalf("страница на разворот: %v %+v", err, r.View.Spreads[1])
	}
	r, err = a.ApplySpreadTpl("s3", "right", "sa:tpl-gold-one")
	if pl := r.View.Spreads[2].Places; err != nil || len(pl) != 1 || pl[0].X < 300 {
		t.Fatalf("на правую: %v %+v", err, pl)
	}
	rev = r.View.Rev
	for _, c := range []struct{ spread, scope, id string }{
		{"s3", "left", hero}, {model.CoverID, "spread", "sa:tpl-wave"}, {"s3", "spread", "sa:tpl-нет"},
		{"s3", "spread", "my:0123456789abcdef"}, {"s3", "top", "sa:tpl-wave"},
	} {
		if r, err := a.ApplySpreadTpl(c.spread, c.scope, c.id); err == nil || r.View.Rev != rev {
			t.Errorf("%+v: %v rev %d→%d", c, err, rev, r.View.Rev)
		}
		if _, err := a.PlanSpreadTpl(c.spread, c.scope, c.id); err == nil {
			t.Errorf("вопрос %+v без ошибки", c)
		}
	}
	if got := prefs.RecentDecor(a.cfgDir, decor.KindSpread); len(got) != 3 {
		t.Errorf("отказы попали в недавние: %v", got)
	}
}

// «Сохранить разворот…»: снимок в свои (плитка с числами в каталоге), свой ставится на другой
// разворот теми же долями; ✎ и × — общие 048; пустая область и пустое имя — отказ; альбом не
// меняется.
func TestSaveSpreadTpl(t *testing.T) {
	a := spreadApp(t)
	if _, err := a.ApplySpreadTpl("s1", "spread", "sa:tpl-stars"); err != nil {
		t.Fatal(err)
	}
	rev := a.editor().View().Rev
	it, err := a.SaveSpreadTpl("s1", "spread", "grad", "Мой выпуск", true, false)
	if err != nil || !it.Own || it.Pages != 2 || it.Places != 4 || it.Theme != "grad" || !strings.HasPrefix(it.URL, decor.TplPath) {
		t.Fatalf("сохранить: %v %+v", err, it)
	}
	page, err := a.SaveSpreadTpl("s1", "left", "mine", "Левая", false, false)
	if err != nil || page.Pages != 1 || page.Places != 0 {
		t.Fatalf("страница без мест: %v %+v", err, page)
	}
	if a.editor().View().Rev != rev {
		t.Fatal("сохранение изменило альбом")
	}
	if _, err := a.SaveSpreadTpl("s1", "spread", "grad", "Ещё", true, false); err == nil {
		t.Fatal("тот же шаблон второй раз")
	}
	if _, err := a.SaveSpreadTpl("s2", "spread", "mine", "Пусто", true, true); err == nil || !strings.Contains(err.Error(), "сохранять нечего") {
		t.Fatalf("пустая область: %v", err)
	}
	if _, err := a.SaveSpreadTpl("s1", "right", "mine", " ", true, true); !errors.Is(err, decor.ErrName) {
		t.Fatalf("пустое имя: %v", err)
	}
	cat, err := a.DecorCatalog(decor.KindSpread)
	if err != nil || len(cat.Items) != 14 || cat.Items[12].ID != page.ID || cat.Items[13].ID != it.ID {
		t.Fatalf("каталог: %v %+v", err, cat.Items)
	}
	r, err := a.ApplySpreadTpl("s2", "spread", it.ID)
	if err != nil {
		t.Fatal(err)
	}
	v := r.View
	for i, pl := range v.Spreads[1].Places {
		if src := v.Spreads[0].Places[i]; math.Abs(src.X-pl.X) > 0.05 || math.Abs(src.W-pl.W) > 0.05 || math.Abs(src.H-pl.H) > 0.05 {
			t.Errorf("место %d: %+v, ждали %+v", i, pl, src)
		}
	}
	if err := a.RenameDecor(decor.KindSpread, it.ID, "Переименован"); err != nil {
		t.Fatal(err)
	}
	if err := a.HideDecor(decor.KindSpread, page.ID); err != nil {
		t.Fatal(err)
	}
	cat, _ = a.DecorCatalog(decor.KindSpread)
	if len(cat.Items) != 13 || cat.Items[12].Name != "Переименован" || !slices.Equal(cat.Recent, []string{it.ID, "sa:tpl-stars"}) {
		t.Fatalf("после ✎ и ×: %+v, недавние %v", cat.Items[12:], cat.Recent)
	}
}

// Свой фон шаблона ставится по ключу, если файл есть на этом ПК; нет — слой пропущен и
// посчитан в missing, остальное встаёт.
func TestApplySpreadTplMissing(t *testing.T) {
	a := spreadApp(t)
	ids := importBg(t, a, testkit.JPEG(t, testkit.TempDir(t), "фон.jpg", 60, 60))
	s := a.decorLib.Load()
	mk := func(lib string) string {
		t.Helper()
		tp := model.SpreadTpl{V: model.TplV, Pages: 1, Ratio: 1, Page: []model.TplBg{{Color: "#eeeeee", Lib: lib}},
			Places: []model.Box{{X: 0.1, Y: 0.1, W: 0.8, H: 0.8}}}
		it, err := s.SaveSpreadTpl("mine", "С фоном", tp)
		if err != nil {
			t.Fatal(err)
		}
		return it.ID
	}
	have, gone := mk(ids[0]), mk("my:00000000000000ee")
	r, err := a.ApplySpreadTpl("s2", "left", have)
	if bd := r.View.Spreads[1].Backdrops; err != nil || r.Missing != 0 || len(bd) != 1 || bd[0].Lib != ids[0] {
		t.Fatalf("свой фон: %v %+v", err, r)
	}
	r, err = a.ApplySpreadTpl("s3", "left", gone)
	if sp := r.View.Spreads[2]; err != nil || r.Missing != 1 || len(sp.Backdrops) != 0 || len(sp.Places) != 1 || sp.Bg[0] != "#eeeeee" {
		t.Fatalf("пропавший фон: %v %+v", err, r)
	}
}

func getTpl(a *App, method, url string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	a.media(http.NotFoundHandler()).ServeHTTP(rec, httptest.NewRequest(method, url, nil))
	return rec
}

// /media/tpl: плитка встроенного и своего — PNG ширины px в пропорции r (страница — одна
// страница), пустое место — серая плашка; мусор в пути и запросе, чужой метод — 404.
func TestMediaTpl(t *testing.T) {
	a := spreadApp(t)
	for _, c := range []struct {
		url  string
		w, h int
	}{
		{"/media/tpl/sa:tpl-gold-one?v=1&px=256&r=1", 256, 256},
		{"/media/tpl/sa:tpl-ivory-hero?v=1&px=256&r=2", 256, 128},
		{"/media/tpl/sa:tpl-rose-page?px=128&r=0.5", 128, 256},
	} {
		rec := getTpl(a, "GET", c.url)
		if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "image/png" {
			t.Fatalf("%s: %d", c.url, rec.Code)
		}
		img, err := png.Decode(bytes.NewReader(rec.Body.Bytes()))
		if err != nil {
			t.Fatal(err)
		}
		if b := img.Bounds(); b.Dx() != c.w || math.Abs(float64(b.Dy())-float64(b.Dx()*c.h/c.w)) > 3 {
			t.Errorf("%s: %v, ждали ≈ %d×%d", c.url, b, c.w, c.h)
		}
		if strings.Contains(c.url, "gold-one") { // место 0,15…0,85 × 0,12…0,82 — серое
			if got := color.RGBAModel.Convert(img.At(128, 120)).(color.RGBA); got != (color.RGBA{0xcf, 0xcf, 0xd4, 255}) {
				t.Errorf("пустое место %v", got)
			}
		}
	}
	if _, err := a.ApplySpreadTpl("s1", "spread", "sa:tpl-party"); err != nil {
		t.Fatal(err)
	}
	it, err := a.SaveSpreadTpl("s1", "spread", "mine", "Свой", true, true)
	if err != nil {
		t.Fatal(err)
	}
	own := it.URL + "&px=200&r=2"
	first := getTpl(a, "GET", own)
	if first.Code != http.StatusOK || !bytes.Equal(getTpl(a, "GET", own).Body.Bytes(), first.Body.Bytes()) {
		t.Fatalf("свой: %d", first.Code)
	}
	// URL плитки постоянный, а свой фон шаблона может появиться позже: браузер не держит
	// плитку без сверки (ревью 067) — ни своего, ни встроенного.
	for _, u := range []string{own, "/media/tpl/sa:tpl-gold-one?v=1&px=256&r=1"} {
		if got := getTpl(a, "GET", u).Header().Get("Cache-Control"); got != "no-cache" {
			t.Errorf("%s: Cache-Control %q", u, got)
		}
	}
	for _, u := range []string{
		"/media/tpl/sa:tpl-wave?px=63&r=1", "/media/tpl/sa:tpl-wave?px=1025&r=1", "/media/tpl/sa:tpl-wave?px=256&r=0.1",
		"/media/tpl/sa:tpl-wave?px=256&r=6", "/media/tpl/sa:tpl-wave?px=256&r=NaN", "/media/tpl/sa:tpl-wave?px=256",
		"/media/tpl/sa:tpl-wave?r=1", "/media/tpl/sa:tpl-wave?px=256&r=1&x=1", "/media/tpl/sa:tpl-wave?px=256&px=300&r=1",
		"/media/tpl/sa:tpl-нет?px=256&r=1", "/media/tpl/sa:cream-gradient?px=256&r=1", "/media/tpl/0123456789abcdef?px=256&r=1",
		"/media/tpl/../lib/t/x?px=256&r=1", "/media/tpl/?px=256&r=1", "/media/tpl/sa:tpl-wave/x?px=256&r=1",
	} {
		if rec := getTpl(a, "GET", u); rec.Code != wantRefusal(u) {
			t.Errorf("%s: %d, ждали %d", u, rec.Code, wantRefusal(u))
		}
	}
	if rec := getTpl(a, "POST", "/media/tpl/sa:tpl-wave?px=256&r=1"); rec.Code != http.StatusNotFound {
		t.Errorf("POST: %d", rec.Code)
	}
	if rec := getTpl(bgApp(t), "GET", "/media/tpl/sa:tpl-wave?px=256&r=1"); rec.Code != http.StatusNotFound {
		t.Errorf("без библиотеки: %d", rec.Code)
	}
}

// Плитка 256 px рисуется быстро (design §3: гипотеза ≤ 30 мс; кэш в обход — drawTpl).
func TestMediaTplFast(t *testing.T) {
	s := decor.Open(testkit.TempDir(t))
	var worst time.Duration
	for _, id := range []string{"sa:tpl-ivory-hero", "sa:tpl-stars", "sa:tpl-watercolor"} {
		tp, err := s.SpreadTpl(id)
		if err != nil {
			t.Fatal(err)
		}
		d := testkit.Fastest(3, func() {
			if _, err := drawTpl(tp, tplQuery{px: 256, r: 1.5}, nil); err != nil {
				t.Fatal(err)
			}
		})
		worst = max(worst, d)
	}
	testkit.CostTest(t)
	if limit := testkit.Limit(30 * time.Millisecond); worst > limit {
		t.Errorf("плитка %v, предел %v", worst, limit)
	}
}

// Разбор запроса плитки: принятое — в пределах; лишнее и повторы — отказ.
func FuzzMediaTplQuery(f *testing.F) {
	for _, s := range []string{"v=1&px=256&r=2", "px=64&r=0.2", "px=1024&r=5", "px=256&r=1e400", "px=256&r=-0", "px=%zz", "v=99999&px=256&r=1"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, raw string) {
		q, ok := parseTplQuery(raw)
		if ok && (q.px < tplMinPx || q.px > tplMaxPx || !(q.r >= tplMinR && q.r <= tplMaxR)) {
			t.Fatalf("%q: %+v", raw, q)
		}
	})
}

// Стык 067 G2 и G3 (порядок 133): шаблон разворота из набора — в каталоге выше встроенных и
// своих, с плиткой /media/tpl, страницами и местами; ставится одним шагом отмены; встроенных в окне
// «Наборы» — 12; удаление набора убирает плитку и возможность поставить.
func TestPackSpreadTpl(t *testing.T) {
	a := spreadApp(t)
	if _, err := a.AddPackPath(d1Zip(t)); err != nil {
		t.Fatal(err)
	}
	own, err := a.decorLib.Load().SaveSpreadTpl("mine", "Свой", model.SpreadTpl{V: model.TplV, Pages: 1, Ratio: 1,
		Page: []model.TplBg{{Color: "#eeeeee"}}, Places: []model.Box{{X: 0.1, Y: 0.1, W: 0.8, H: 0.8}}})
	if err != nil {
		t.Fatal(err)
	}
	cat, err := a.DecorCatalog(decor.KindSpread)
	if err != nil || len(cat.Items) != 14 {
		t.Fatalf("каталог: %v %d", err, len(cat.Items))
	}
	for i, it := range cat.Items[1:13] {
		if !strings.HasPrefix(it.ID, "sa:tpl-") || it.Pack != "" || it.PackID != decor.BuiltinPack || it.Peek != "" {
			t.Errorf("встроенный %d: %+v", i, it)
		}
	}
	pk := cat.Items[0]
	if pk.Pack != "Свадьба — айвори" || pk.Own || pk.Name != "Главное и два" || pk.Theme != "wedding" ||
		pk.URL != decor.TplURL(pk.ID) || pk.PackID != "ivory" || pk.Peek != "" || pk.Pages != 2 || pk.Places != 3 || pk.Aspect != 1.5 {
		t.Fatalf("плитка набора: %+v", pk)
	}
	if cat.Items[13].ID != own.ID || !cat.Items[13].Own || cat.Items[13].PackID != "" || cat.Items[13].Peek != "" {
		t.Errorf("свой не последним: %+v", cat.Items[13])
	}
	if rec := getTpl(a, "GET", pk.URL+"&px=128&r=1.5"); rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("плитка набора: %d", rec.Code)
	}
	list, err := a.LibraryPacks()
	if err != nil || len(list.Packs) != 2 || list.Packs[0].Spread != 12 || list.Packs[1].Spread != 1 {
		t.Fatalf("наборы: %v %+v", err, list)
	}
	before := projectJSON(t, a)
	rev := a.editor().View().Rev
	r, err := a.ApplySpreadTpl("s1", "spread", pk.ID)
	if s := r.View.Spreads[0]; err != nil || r.View.Rev != rev+1 || !s.Grid || len(s.Places) != 3 || s.Bg[0] != "#f4efe6" {
		t.Fatalf("поставить: %v %+v", err, r)
	}
	if again, err := a.ApplySpreadTpl("s1", "spread", pk.ID); err != nil || again.View.Rev != r.View.Rev {
		t.Fatalf("повтор не холостой: %v", err)
	}
	a.Undo()
	if projectJSON(t, a) != before {
		t.Fatal("отмена не вернула проект")
	}
	if _, err := a.RemovePack("ivory"); err != nil {
		t.Fatal(err)
	}
	cat, _ = a.DecorCatalog(decor.KindSpread)
	if len(cat.Items) != 13 || slices.ContainsFunc(cat.Items, func(it DecorItem) bool { return it.ID == pk.ID }) {
		t.Fatalf("после удаления набора: %d плиток", len(cat.Items))
	}
	if rec := getTpl(a, "GET", pk.URL+"&px=128&r=1.5"); rec.Code != http.StatusNotFound {
		t.Errorf("плитка удалённого набора: %d", rec.Code)
	}
	rev = a.editor().View().Rev
	if r, err := a.ApplySpreadTpl("s1", "spread", pk.ID); err == nil || r.View.Rev != rev {
		t.Errorf("шаблон удалённого набора встал: %v", err)
	}
}

// TestTplKeysBorders — ключи шаблона для App.tplLibs: фоны, стикеры и рамочки-картинки
// (131 G1); встроенная рамочка ключа не даёт.
func TestTplKeysBorders(t *testing.T) {
	tp := model.SpreadTpl{Page: []model.TplBg{{Lib: "my:00000000000000b1"}}, Whole: &model.TplBg{Lib: "my:00000000000000b2"},
		Stickers: []model.TplItem{{Lib: "my:00000000000000c1"}, {}},
		Borders:  []model.TplBorder{{I: 0, Kind: "line"}, {I: 1, Lib: "my:00000000000000d1"}}}
	want := []string{"my:00000000000000b2", "my:00000000000000b1", "my:00000000000000c1", "my:00000000000000d1"}
	if got := tplKeys(tp); !slices.Equal(got, want) {
		t.Errorf("ключи %v, ждали %v", got, want)
	}
	if got := tplKeys(model.SpreadTpl{}); len(got) != 0 {
		t.Errorf("пустой шаблон: %v", got)
	}
}
