package roundtrip_test

// 064 «Фото через сгиб», формат v11: прежняя галочка layflat читается как «Только широкие», а
// «Авто», чья первая по правилу меняется, закрепляется прежним ключом (decisions 0.15 №2, №15).

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

var (
	vert  = [2]int{200, 300}
	horiz = [2]int{300, 200}
	// foldSets — три вертикальных 2:3 (первая «Любых» — H(0,1,2) с узким фото на корешке), два
	// горизонтальных (первая одна во всех положениях), три вертикальных с ключом руками, три
	// вертикальных с негодным ключом (показывался как «Авто»).
	foldSets = [][][2]int{{vert, vert, vert}, {horiz, horiz}, {vert, vert, vert}, {vert, vert, vert}}
)

// foldFile — файл версии v с настройками settings (сырой JSON поверх полей tieFile; nil — как
// есть) и ключами раскладок keys по разворотам.
func foldFile(t *testing.T, dir string, v int, settings map[string]any, keys ...string) []byte {
	t.Helper()
	return editFile(t, tieFile(t, dir, v, foldSets), func(m map[string]any) {
		if settings != nil {
			m["settings"] = settings
		}
		for i, k := range keys {
			m["spreads"].([]any)[i].(map[string]any)["layout"] = k
		}
	})
}

// editFile — файл data, поправленный f на уровне сырого JSON (поля, которых нет в FileProject).
func editFile(t *testing.T, data []byte, f func(m map[string]any)) []byte {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	f(m)
	out, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// foldRows — первая раскладка разворота i при «Любых» и при «Широких».
func foldRows(t *testing.T, p *model.Project, i int) (anyFirst, wideFirst layout.Result) {
	t.Helper()
	asp := p.Aspects(p.Spreads[i].Photos)
	prm := p.Params()
	prm.FoldAny = true
	a := layout.Cached(asp, prm, model.RowLen(len(asp)))
	prm.FoldAny = false
	w := layout.Cached(asp, prm, model.RowLen(len(asp)))
	if len(a) == 0 || len(w) == 0 {
		t.Fatalf("разворот %d: пустой ряд", i)
	}
	return a[0], w[0]
}

func layoutKeys(p *model.Project) []string {
	out := make([]string, len(p.Spreads))
	for i, s := range p.Spreads {
		out[i] = s.Layout
	}
	return out
}

const handKey = "V(0,1,2)" // выбран руками: строится на трёх вертикальных; узкие на корешке — не трогается

// TestLoadV10FoldWide — layflat-файл v10: «Только широкие»; «Авто» трёх вертикальных закреплено
// прежним ключом с теми же Rects, «Авто» горизонтальных осталось "", ключ руками не тронут,
// негодный ключ (показывался как прежняя первая) закреплён ею же. Параллельно и по очереди —
// одно и то же. Save → Load: v11 с foldAny, ничего не меняется.
func TestLoadV10FoldWide(t *testing.T) {
	dir := testkit.TempDir(t)
	data := foldFile(t, dir, 10, nil, "", "", handKey, "H(0,1)")
	p := edtest.MustDecode(t, data, dir)
	if st := p.Settings; !st.Layflat || st.FoldAny {
		t.Fatalf("v10 layflat: layflat %v, foldAny %v — ждали «Только широкие»", st.Layflat, st.FoldAny)
	}
	a0, w0 := foldRows(t, p, 0)
	if asp := p.Aspects(p.Spreads[0].Photos); a0.Key == w0.Key || layout.FoldOK(a0.Rects, asp, p.Params()) {
		t.Fatalf("три вертикальных: «Любые» %q, «Широкие» %q — тест не проверяет", a0.Key, w0.Key)
	}
	if a1, w1 := foldRows(t, p, 1); a1.Key != w1.Key {
		t.Fatalf("горизонтальные: первая меняется (%q → %q) — тест не проверяет", a1.Key, w1.Key)
	}
	want := []string{a0.Key, "", handKey, a0.Key}
	if got := layoutKeys(p); !slices.Equal(got, want) {
		t.Fatalf("ключи %q, ждали %q", got, want)
	}
	if r, _, ok := p.Layout(p.Spreads[0]); !ok || !slices.Equal(r.Rects, a0.Rects) {
		t.Fatalf("закреплённое «Авто» сдвинулось: %v, было %v", r.Rects, a0.Rects)
	}

	model.SetParallelLayouts(false)
	t.Cleanup(func() { model.SetParallelLayouts(true) })
	if seq := edtest.MustDecode(t, data, dir); !bytes.Equal(model.Marshal(seq), model.Marshal(p)) {
		t.Fatalf("по очереди и параллельно разное:\n%s\n%s", model.Marshal(seq), model.Marshal(p))
	}
	model.SetParallelLayouts(true)

	path := filepath.Join(dir, "a.salbum")
	e := edtest.Open(t, p, path)
	if err := e.Save(path); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	if f := fileJSON(t, path); f.Version != 11 || !bytes.Contains(raw, []byte(`"foldAny": false`)) {
		t.Fatalf("файл v%d без foldAny", f.Version)
	}
	q, err := salbum.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := layoutKeys(q); !slices.Equal(got, want) || q.Settings.FoldAny || edtest.From(q) != model.FormatVersion {
		t.Fatalf("Save → Load: ключи %q, foldAny %v, from %d", got, q.Settings.FoldAny, edtest.From(q))
	}
}

// TestLoadV10FoldNone — файл без layflat («Нет»): FoldAny нет, закреплять нечего — «Авто» и
// негодный ключ остаются как в файле, ключ руками стоит.
func TestLoadV10FoldNone(t *testing.T) {
	dir := testkit.TempDir(t)
	st := map[string]any{"pageW": 300, "pageH": 300, "margin": 12, "gap": 3, "layflat": false, "dpi": 300, "safe": 5}
	p := edtest.MustDecode(t, foldFile(t, dir, 10, st, "", "", handKey, "H(0,1)"), dir)
	if p.Settings.Layflat || p.Settings.FoldAny {
		t.Fatalf("layflat %v, foldAny %v", p.Settings.Layflat, p.Settings.FoldAny)
	}
	if got, want := layoutKeys(p), []string{"", "", handKey, "H(0,1)"}; !slices.Equal(got, want) {
		t.Fatalf("ключи %q, ждали %q", got, want)
	}
}

// TestLoadNoLayflatField — в файле нет поля layflat: прежнее значение по умолчанию (true), значит
// «Только широкие» с закреплением, как у явного true. Пустой альбом (0 разворотов) — без паники.
func TestLoadNoLayflatField(t *testing.T) {
	dir := testkit.TempDir(t)
	st := map[string]any{"pageW": 300, "pageH": 300, "margin": 12, "gap": 3, "dpi": 300}
	p := edtest.MustDecode(t, foldFile(t, dir, 10, st, "", ""), dir)
	a0, _ := foldRows(t, p, 0)
	if !p.Settings.Layflat || p.Settings.FoldAny || p.Spreads[0].Layout != a0.Key || p.Spreads[1].Layout != "" {
		t.Fatalf("layflat %v, foldAny %v, ключи %q (ждали %q, \"\")", p.Settings.Layflat, p.Settings.FoldAny, layoutKeys(p)[:2], a0.Key)
	}
	empty := edtest.MustDecode(t, []byte(`{"version":10,"settings":{"pageW":300,"pageH":300}}`), dir)
	if !empty.Settings.Layflat || empty.Settings.FoldAny || len(empty.Spreads) != 0 {
		t.Fatalf("пустой: layflat %v, foldAny %v, разворотов %d", empty.Settings.Layflat, empty.Settings.FoldAny, len(empty.Spreads))
	}
}

// TestLoadV5FoldLegacy — файл до v6: «Авто» трёх вертикальных — первая прежнего порядка ничьих
// (LegacyLayouts, без правила сгиба): записал ли её migrate или закрепил pinFoldAuto — итог один.
// Порядок «foldLegacy до migrate» сторожит TestLoadFoldLegacyBeforeMigrate.
func TestLoadV5FoldLegacy(t *testing.T) {
	dir := testkit.TempDir(t)
	p := edtest.MustDecode(t, foldFile(t, dir, 5, nil), dir)
	prm := p.Params()
	if !prm.Layflat || prm.FoldAny {
		t.Fatalf("v5: layflat %v, foldAny %v", prm.Layflat, prm.FoldAny)
	}
	for i, s := range p.Spreads {
		asp := p.Aspects(s.Photos)
		legacy := layout.LegacyLayouts(asp, prm, model.VariantLimit)[0]
		if r, _, ok := p.Layout(s); !ok || r.Key != legacy.Key || !slices.Equal(r.Rects, legacy.Rects) {
			t.Errorf("разворот %d: действует %q (ключ в файле %q), ждали прежний %q", i, r.Key, s.Layout, legacy.Key)
		}
	}
	if p.Spreads[0].Layout == "" {
		t.Error("три вертикальных: «Авто» v5 не закреплено")
	}
}

// TestSaveFoldV11 — v11 читается как записан: «Любые» остаются, «Широкие» не закрепляют «Авто»
// (переход уже был), foldAny без layflat обнуляется зажимом; круг Save → Load сохраняет поле.
func TestSaveFoldV11(t *testing.T) {
	dir := testkit.TempDir(t)
	set := func(layflat, foldAny bool) map[string]any {
		return map[string]any{"pageW": 300, "pageH": 300, "margin": 12, "gap": 3, "layflat": layflat, "foldAny": foldAny, "dpi": 300}
	}
	wide := edtest.MustDecode(t, foldFile(t, dir, 11, set(true, false)), dir)
	if wide.Settings.FoldAny || !slices.Equal(layoutKeys(wide), []string{"", "", "", ""}) {
		t.Fatalf("v11 «Широкие»: foldAny %v, ключи %q", wide.Settings.FoldAny, layoutKeys(wide))
	}
	if r, _, _ := wide.Layout(wide.Spreads[0]); !layout.FoldOK(r.Rects, wide.Aspects(wide.Spreads[0].Photos), wide.Params()) {
		t.Fatalf("v11 «Широкие»: «Авто» %q вне правила", r.Key)
	}
	if none := edtest.MustDecode(t, foldFile(t, dir, 11, set(false, true)), dir); none.Settings.FoldAny || none.Settings.Layflat {
		t.Fatalf("v11 foldAny без layflat: layflat %v, foldAny %v", none.Settings.Layflat, none.Settings.FoldAny)
	}
	p := edtest.MustDecode(t, foldFile(t, dir, 11, set(true, true)), dir)
	if !p.Settings.FoldAny || !slices.Equal(layoutKeys(p), []string{"", "", "", ""}) {
		t.Fatalf("v11 «Любые»: foldAny %v, ключи %q", p.Settings.FoldAny, layoutKeys(p))
	}
	q := edtest.SaveLoad(t, edtest.Open(t, p, ""))
	if !q.Settings.FoldAny || !q.Settings.Layflat || !bytes.Equal(model.Marshal(q), model.Marshal(p)) {
		t.Fatalf("Save → Load «Любых»: foldAny %v\n%s\n%s", q.Settings.FoldAny, model.Marshal(p), model.Marshal(q))
	}
}

// TestLoadV10FoldMany — pinFoldAuto на развороте от 17 фото (путь many, ряд RowLen): «Авто»
// после чтения строит те же Rects, что первая «Любых»; ключ — она же, если первая «Широких»
// другая, иначе "". Узкие страницы 10×30: у 19 фото (вертикальные через одно с почти
// квадратными) первая «Любых» — ряды на весь разворот с узким на сгибе (закрепляется), у 17
// вертикальных и 20 горизонтальных первая не меняется.
func TestLoadV10FoldMany(t *testing.T) {
	dir := testkit.TempDir(t)
	near := [2]int{220, 200}
	sets := [][][2]int{make([][2]int, 19), make([][2]int, layout.ManyFrom), make([][2]int, 20)}
	for i := range sets[0] {
		sets[0][i] = [][2]int{vert, near}[i%2]
	}
	for i := range sets[1] {
		sets[1][i] = vert
	}
	for i := range sets[2] {
		sets[2][i] = horiz
	}
	st := map[string]any{"pageW": 100, "pageH": 300, "margin": 5, "gap": 3, "layflat": true, "dpi": 300}
	p := edtest.MustDecode(t, editFile(t, tieFile(t, dir, 10, sets), func(m map[string]any) { m["settings"] = st }), dir)
	if p.Settings.FoldAny || p.Settings.PageW != 100 || len(p.Spreads) != len(sets) {
		t.Fatalf("foldAny %v, pageW %v, разворотов %d", p.Settings.FoldAny, p.Settings.PageW, len(p.Spreads))
	}
	if a0, w0 := foldRows(t, p, 0); a0.Key == w0.Key {
		t.Fatalf("19 фото: первая одна в обоих положениях (%q) — тест не проверяет", a0.Key)
	}
	for i, s := range p.Spreads {
		if len(s.Photos) < layout.ManyFrom {
			t.Fatalf("разворот %d: %d фото — не путь many", i, len(s.Photos))
		}
		a0, w0 := foldRows(t, p, i)
		want := ""
		if a0.Key != w0.Key {
			want = a0.Key
		}
		if r, _, ok := p.Layout(s); s.Layout != want || !ok || !slices.Equal(r.Rects, a0.Rects) {
			t.Errorf("разворот %d (%d фото): ключ %q, ждали %q; «Авто» сдвинулось: %v", i, len(s.Photos), s.Layout, want, !slices.Equal(r.Rects, a0.Rects))
		}
	}
}

// TestLoadV10FoldSheet — лист виньетки v10 (layflat ставит зажим, шва нет): FoldAny не ставится,
// pinFoldAuto не трогает листы — «Авто» и негодный ключ остаются как в файле, Rects — первая ряда.
func TestLoadV10FoldSheet(t *testing.T) {
	dir := testkit.TempDir(t)
	st := map[string]any{"pageW": 600, "pageH": 300, "margin": 12, "gap": 3, "layflat": true, "sheet": true, "dpi": 300}
	p := edtest.MustDecode(t, foldFile(t, dir, 10, st, "", "", "", "H(0,1)"), dir)
	if !p.Settings.Sheet || p.Settings.FoldAny {
		t.Fatalf("лист: sheet %v, foldAny %v", p.Settings.Sheet, p.Settings.FoldAny)
	}
	if got, want := layoutKeys(p), []string{"", "", "", "H(0,1)"}; !slices.Equal(got, want) {
		t.Fatalf("ключи %q, ждали %q", got, want)
	}
	asp := p.Aspects(p.Spreads[0].Photos)
	first := layout.Cached(asp, p.Params(), model.RowLen(len(asp)))
	if r, _, ok := p.Layout(p.Spreads[0]); !ok || len(first) == 0 || !slices.Equal(r.Rects, first[0].Rects) {
		t.Fatalf("лист: «Авто» %q, первая ряда %v", r.Key, first)
	}
}

// withVariant — файл data с номером варианта v у разворота i (поле файлов до ключей).
func withVariant(t *testing.T, data []byte, i, v int) []byte {
	t.Helper()
	return editFile(t, data, func(m map[string]any) { m["spreads"].([]any)[i].(map[string]any)["variant"] = v })
}

// TestLoadFoldLegacyBeforeMigrate — сторож порядка «foldLegacy до migrate» (decisions 0.15 №15):
// номер варианта и ничьи до v6 считаются по рядам «Любых», а не «Широких». v5: вариант v —
// тот, где прежний ряд даёт первую «Широких» (≠ первой «Любых»): migrate по «Любым» записывает
// её ключ, по «Широким» счёл бы «Авто» (""), и pinFoldAuto закрепил бы первую «Любых». v10:
// вариант u, где ряды «Любых» и «Широких» расходятся, — ключ из ряда «Любых».
func TestLoadFoldLegacyBeforeMigrate(t *testing.T) {
	dir := testkit.TempDir(t)
	set := [][][2]int{{vert, vert, vert}}
	base := edtest.MustDecode(t, tieFile(t, dir, 5, set), dir)
	asp, prm := base.Aspects(base.Spreads[0].Photos), base.Params()
	anyPrm := prm
	anyPrm.FoldAny = true
	anyRow, wide := layout.Cached(asp, anyPrm, model.VariantLimit), layout.Cached(asp, prm, model.VariantLimit)
	legacy := layout.LegacyLayouts(asp, prm, model.VariantLimit)
	v := slices.IndexFunc(legacy, func(r layout.Result) bool { return r.Key == wide[0].Key })
	if v < 0 || wide[0].Key == anyRow[0].Key {
		t.Fatalf("первая «Широких» %q, «Любых» %q, в прежнем ряду №%d — тест не проверяет", wide[0].Key, anyRow[0].Key, v)
	}
	p := edtest.MustDecode(t, withVariant(t, tieFile(t, dir, 5, set), 0, v), dir)
	if r, _, ok := p.Layout(p.Spreads[0]); p.Spreads[0].Layout != wide[0].Key || !ok || !slices.Equal(r.Rects, legacy[v].Rects) {
		t.Fatalf("v5, вариант %d: ключ %q, ждали %q (первая «Любых» %q)", v, p.Spreads[0].Layout, wide[0].Key, anyRow[0].Key)
	}

	u := 0
	for i := 1; i < min(len(anyRow), len(wide)) && u == 0; i++ {
		if anyRow[i].Key != wide[i].Key {
			u = i
		}
	}
	if u == 0 {
		t.Fatal("ряды «Любых» и «Широких» совпадают — тест не проверяет")
	}
	q := edtest.MustDecode(t, withVariant(t, tieFile(t, dir, 10, set), 0, u), dir)
	if q.Spreads[0].Layout != anyRow[u].Key {
		t.Fatalf("v10, вариант %d: ключ %q, ждали %q из ряда «Любых»", u, q.Spreads[0].Layout, anyRow[u].Key)
	}
}
