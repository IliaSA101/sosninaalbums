package spreads_test

// Спека 041: разнообразие автосборки на уровне Editor — выбор раскладок сборки с учётом
// соседей, «Разнообразить раскладки», поля подборки (семейство, похожие, «Другие варианты»).

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/layout/variety"
)

var levels = []model.Variety{model.VarietyLess, model.VarietyNormal, model.VarietyMore}

// spreadsShape — фото и ключ каждого разворота: ID разворотов случайные, сравнивать их нельзя.
func spreadsShape(p *model.Project) []string {
	out := make([]string, len(p.Spreads))
	for i, s := range p.Spreads {
		out[i] = strings.Join(s.Photos, ",") + "|" + s.Layout
	}
	return out
}

// photosOf — только фото разворотов (без ключа).
func photosOf(p *model.Project) []string {
	out := make([]string, len(p.Spreads))
	for i, s := range p.Spreads {
		out[i] = strings.Join(s.Photos, ",")
	}
	return out
}

// horizontals — n одинаковых горизонтальных 3:2: серия, на которой 0.10 повторяет вид.
func horizontals(prefix string, n int) []model.PhotoInfo {
	out := make([]model.PhotoInfo, n)
	for i := range out {
		id := fmt.Sprint(prefix, i)
		out[i] = model.PhotoInfo{ID: id, Path: edtest.Abs(id + ".jpg"), W: 300, H: 200}
	}
	return out
}

// looks — вид действующей раскладки каждого разворота ("" — не «Авто» или без фото).
func looks(e *album.Editor) []string {
	p := edtest.P(e)
	out := make([]string, len(p.Spreads))
	for i, s := range p.Spreads {
		if r, _, ok := p.Layout(s); ok && len(s.Grid) == 0 {
			out[i] = variety.Look(r.Rects, p.Params())
		}
	}
	return out
}

// repeats — соседних пар с одним видом.
func repeats(ls []string) int {
	n := 0
	for i := 1; i < len(ls); i++ {
		if ls[i] != "" && ls[i] == ls[i-1] {
			n++
		}
	}
	return n
}

func build(t *testing.T, e *album.Editor, ph []model.PhotoInfo, at int, o model.AutoOpts) model.View {
	t.Helper()
	v, err := e.AutoBuild(ph, at, o)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

// TestAutoBuildVarietyLessSame: «Меньше» (и нулевое, и вне 0…2 снизу) — поведение 0.10: у всех
// разворотов Layout "" (лучшая), проект байт в байт один; уровни меняют только раскладки —
// фото по разворотам те же, план тот же.
func TestAutoBuildVarietyLessSame(t *testing.T) {
	ph := mixedPhotos("v", 40)
	type res struct {
		ap     model.AutoPlan
		shape  []string
		photos []string
	}
	run := func(v model.Variety) res {
		e := edtest.Open(t, model.New(), "")
		o := model.AutoOpts{Spreads: 6, MaxPer: 12, Variety: v}
		ap, err := e.AutoPlan(ph, o)
		if err != nil {
			t.Fatal(err)
		}
		build(t, e, ph, -1, o)
		return res{ap, spreadsShape(edtest.P(e)), photosOf(edtest.P(e))}
	}
	less := run(model.VarietyLess)
	if less.ap.Spreads != 6 || len(less.shape) != 6 {
		t.Fatalf("план %+v, разворотов %d", less.ap, len(less.shape))
	}
	for _, s := range less.shape {
		if !strings.HasSuffix(s, "|") {
			t.Fatalf("«Меньше» записало ключ: %s", s)
		}
	}
	if got := run(-3); !slices.Equal(got.shape, less.shape) {
		t.Fatalf("Variety −3 ≠ «Меньше»: %v", got.shape)
	}
	changed := false
	for _, v := range []model.Variety{model.VarietyNormal, model.VarietyMore, 99} {
		got := run(v)
		if got.ap != less.ap || !slices.Equal(got.photos, less.photos) {
			t.Fatalf("Variety %d: план %+v, фото %v — ждали как у «Меньше»", v, got.ap, got.photos)
		}
		changed = changed || !slices.Equal(got.shape, less.shape)
	}
	if !changed {
		t.Fatal("«Обычное» и «Больше» не поменяли ни одной раскладки")
	}
}

// TestAutoBuildVarietyKeys: 96 горизонтальных на 12 разворотов — «Меньше» повторяет вид соседа,
// «Обычное» и «Больше» — нет, видов ≥ 5 (критерий спеки на уровне альбома). Каждый записанный
// ключ строится, View рисует ровно Build этого ключа; лучшая хранится как "".
func TestAutoBuildVarietyKeys(t *testing.T) {
	ph := horizontals("h", 96)
	for _, v := range levels {
		e := edtest.Open(t, model.New(), "")
		view := build(t, e, ph, -1, model.AutoOpts{Spreads: 12, MaxPer: 12, Variety: v})
		p := edtest.P(e)
		ls := looks(e)
		kinds := map[string]bool{}
		for _, l := range ls {
			kinds[l] = true
		}
		switch r := repeats(ls); {
		case v == model.VarietyLess && r == 0:
			t.Fatalf("«Меньше» без повторов вида: тест не различает уровни")
		case v != model.VarietyLess && (r != 0 || len(kinds) < 5):
			t.Fatalf("Variety %d: соседних повторов %d, видов %d", v, r, len(kinds))
		}
		for i, s := range p.Spreads {
			asp := p.Aspects(s.Photos)
			best := layout.Cached(asp, p.Params(), model.RowLen(len(asp)))[0]
			if s.Layout == best.Key {
				t.Fatalf("разворот %d: лучшая записана ключом, а не \"\"", i)
			}
			key := cmpOr(s.Layout, best.Key)
			r, ok := layout.Build(key, asp, p.Params())
			if !ok || view.Spreads[i].Layout != key || !slices.Equal(view.Spreads[i].Rects, r.Rects) {
				t.Fatalf("разворот %d: ключ %q не строится или View ≠ Build", i, key)
			}
		}
	}
}

func cmpOr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// TestAutoBuildVarietyUndo: сборка с выбором — один шаг отмены (как в 0.10): Undo возвращает
// проект байт в байт, Redo — те же ключи; и в пустой альбом, и в середину непустого.
func TestAutoBuildVarietyUndo(t *testing.T) {
	for _, v := range levels {
		e := edtest.Open(t, model.New(), "")
		build(t, e, horizontals("a", 24), -1, model.AutoOpts{Spreads: 3, MaxPer: 64})
		before, undo := model.Marshal(edtest.P(e)), edtest.UndoLen(e)
		build(t, e, horizontals("b", 32), 1, model.AutoOpts{Spreads: 4, MaxPer: 64, Variety: v})
		after := spreadsShape(edtest.P(e))
		if len(after) != 7 || edtest.UndoLen(e) != undo+1 {
			t.Fatalf("Variety %d: разворотов %d, шагов отмены %d → %d", v, len(after), undo, edtest.UndoLen(e))
		}
		e.Undo()
		if got := model.Marshal(edtest.P(e)); string(got) != string(before) {
			t.Fatalf("Variety %d: отмена не вернула проект", v)
		}
		e.Redo()
		if got := spreadsShape(edtest.P(e)); !slices.Equal(got, after) {
			t.Fatalf("Variety %d: возврат %v, ждали %v", v, got, after)
		}
	}
}

// TestAutoBuildVarietyNeighborsAt: вставка в середину учитывает соседей альбома: новый разворот
// не повторяет вид ни предыдущего, ни следующего (у «Меньше» — тот же вид, что у них).
func TestAutoBuildVarietyNeighborsAt(t *testing.T) {
	for _, v := range levels {
		e := edtest.Open(t, model.New(), "")
		build(t, e, horizontals("a", 24), -1, model.AutoOpts{Spreads: 3, MaxPer: 8})
		was := looks(e)
		if was[0] != was[1] || was[1] != was[2] {
			t.Fatalf("заготовка: виды разные %v", was)
		}
		build(t, e, horizontals("b", 8), 1, model.AutoOpts{Spreads: 1, MaxPer: 64, Variety: v})
		ls := looks(e)
		if len(ls) != 4 {
			t.Fatalf("разворотов %d", len(ls))
		}
		same := ls[1] == ls[0] || ls[1] == ls[2]
		if same != (v == model.VarietyLess) {
			t.Fatalf("Variety %d: вид вставленного %q, соседи %q / %q", v, ls[1], ls[0], ls[2])
		}
	}
}

// TestAutoPlanMatchesBuildVariety: подсказка окна от уровня не зависит, а сборка любого уровня
// кладёт фото ровно по плану (ручка меняет только раскладки).
func TestAutoPlanMatchesBuildVariety(t *testing.T) {
	ph := mixedPhotos("p", 30)
	for _, c := range []struct{ n, m int }{{1, 64}, {5, 64}, {1, 4}, {30, 64}, {3, 10}} {
		var ref []string
		for _, v := range levels {
			e := edtest.Open(t, model.New(), "")
			o := model.AutoOpts{Spreads: c.n, MaxPer: c.m, Variety: v}
			ap, err := e.AutoPlan(ph, o)
			if err != nil {
				t.Fatal(err)
			}
			build(t, e, ph, -1, o)
			got := photosOf(edtest.P(e))
			sizes := make([]int, len(got))
			for i, s := range edtest.P(e).Spreads {
				sizes[i] = len(s.Photos)
			}
			if len(got) != ap.Spreads || slices.Min(sizes) != ap.Min || slices.Max(sizes) != ap.Max {
				t.Fatalf("N=%d m=%d Variety %d: сборка %v, план %+v", c.n, c.m, v, sizes, ap)
			}
			if ref == nil {
				ref = got
			} else if !slices.Equal(got, ref) {
				t.Fatalf("N=%d m=%d Variety %d: фото по разворотам другие", c.n, c.m, v)
			}
		}
	}
}

// varyEd — пять разворотов по 8 горизонтальных одного вида («Меньше»), третий — сетка,
// четвёртый — виньетка; ID по порядку.
func varyEd(t *testing.T) (*album.Editor, []string) {
	e := edtest.Open(t, model.New(), "")
	ids := edtest.SpreadIDs(build(t, e, horizontals("h", 40), -1, model.AutoOpts{Spreads: 5, MaxPer: 8}))
	edtest.FreezeAt(t, e, 2)
	if _, _, err := e.SetVignette(ids[3], model.DefaultVignette(), nil); err != nil {
		t.Fatal(err)
	}
	if ls := looks(e); ls[0] != ls[1] || ls[1] != ls[4] || !edtest.P(e).Spreads[3].IsVignette() {
		t.Fatalf("заготовка: виды %v", ls)
	}
	return e, ids
}

// TestVaryLayouts: только выделенные «Авто»; сетка и виньетка не трогаются; цепочка подряд
// идущих не повторяет вид ни внутри, ни текущий; один шаг отмены; невыделенные стоят;
// холостая (нечего менять) — ни шага, ни rev; «Меньше» = «Обычное».
func TestVaryLayouts(t *testing.T) {
	e, ids := varyEd(t)
	frozen := func() string { p := edtest.P(e); return spreadJSON(p, 2) + spreadJSON(p, 3) }
	fixed, was := frozen(), looks(e)
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	before := model.Marshal(edtest.P(e))

	// холостые: ничего подходящего
	for _, sel := range [][]string{nil, {}, {"нет"}, {ids[2], ids[3]}} {
		if v := e.VaryLayouts(sel, model.VarietyNormal, nil); v.Rev != rev || edtest.UndoLen(e) != undo {
			t.Fatalf("VaryLayouts(%v): rev %d → %d", sel, rev, v.Rev)
		}
	}

	v := e.VaryLayouts([]string{ids[0], ids[1], "нет"}, model.VarietyNormal, nil)
	ls := looks(e)
	if v.Rev != rev+1 || edtest.UndoLen(e) != undo+1 || !v.CanUndo {
		t.Fatalf("правка: rev %d → %d, отмена %d → %d", rev, v.Rev, undo, edtest.UndoLen(e))
	}
	if ls[0] == was[0] || ls[1] == was[1] || ls[0] == ls[1] {
		t.Fatalf("цепочка из двух: было %v, стало %v", was[:2], ls[:2])
	}
	if ls[4] != was[4] || frozen() != fixed {
		t.Fatal("тронут невыделенный разворот, сетка или виньетка")
	}
	e.Undo()
	if string(model.Marshal(edtest.P(e))) != string(before) {
		t.Fatal("отмена не вернула проект")
	}

	// всё выделено: сетка и виньетка стоят, две цепочки ([0,1] и [4]) меняются
	e.VaryLayouts(append(slices.Clone(ids), ids[0]), model.VarietyLess, nil)
	ls = looks(e)
	if ls[0] == was[0] || ls[1] == was[1] || ls[4] == was[4] || frozen() != fixed {
		t.Fatalf("всё выделено: было %v, стало %v", was, ls)
	}
	less := spreadsShape(edtest.P(e))
	e.Undo()
	e.VaryLayouts(ids, model.VarietyNormal, nil)
	if got := spreadsShape(edtest.P(e)); !slices.Equal(got, less) {
		t.Fatalf("«Меньше» ≠ «Обычное»: %v / %v", less, got)
	}
}

// spreadJSON — разворот i как JSON (сравнение «не тронут»).
func spreadJSON(p *model.Project, i int) string {
	data, err := json.Marshal(p.Spreads[i])
	if err != nil {
		panic(err)
	}
	return string(data)
}

// TestVaryLayoutsNoop: разворот, которому менять нечего (лист с одним квадратным фото — в пуле
// одна раскладка), — холостая на любом уровне: rev, отмена и dirty те же.
func TestVaryLayoutsNoop(t *testing.T) {
	e := edtest.Open(t, model.NewSheet(204, 304), "")
	sq := []model.PhotoInfo{{ID: "o", Path: edtest.Abs("o.jpg"), W: 250, H: 250}}
	ids := edtest.SpreadIDs(build(t, e, sq, -1, model.AutoOpts{Spreads: 1, MaxPer: 64}))
	edtest.SetDirty(e, false)
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	before := model.Marshal(edtest.P(e))
	for _, v := range levels {
		e.VaryLayouts(ids, v, nil)
	}
	if edtest.Rev(e) != rev || edtest.UndoLen(e) != undo || edtest.Dirty(e) || string(model.Marshal(edtest.P(e))) != string(before) {
		t.Fatalf("холостая оставила след: rev %d → %d, отмена %d → %d", rev, edtest.Rev(e), undo, edtest.UndoLen(e))
	}
}

// bigOf — во сколько раз площадь фото i больше наибольшей из остальных.
func bigOf(rects []layout.Rect, i int) float64 {
	other := 0.0
	for j, r := range rects {
		if j != i {
			other = max(other, r.W*r.H)
		}
	}
	return rects[i].W * rects[i].H / other
}

// TestVaryLayoutsStar: 5★ у последнего из 4 горизонтальных — главное фото (≥ 1.6× любого
// другого), хотя у лучшей раскладки главное — первое: и своя отметка, и XMP из infos; своя 2★
// поверх XMP 5★ — звезды нет (как без оценок).
func TestVaryLayoutsStar(t *testing.T) {
	ph := horizontals("s", 4)
	run := func(own *int, xmp int) (float64, string) {
		e := edtest.Open(t, model.New(), "")
		ids := edtest.SpreadIDs(build(t, e, ph, -1, model.AutoOpts{Spreads: 1, MaxPer: 64}))
		if big := bigOf(e.View().Spreads[0].Rects, 3); big >= 1.6 {
			t.Fatalf("у лучшей раскладки главное уже фото 3 (%.2f): тест не различает", big)
		}
		if own != nil {
			edtest.SetMarks(t, e, ph[3:], model.MarkPatch{Rating: own})
		}
		infos := slices.Clone(ph)
		infos[3].Xmp.Rating = xmp
		v := e.VaryLayouts(ids, model.VarietyNormal, infos)
		return bigOf(v.Spreads[0].Rects, 3), v.Spreads[0].Layout
	}
	if big, key := run(edtest.Ip(5), 0); big < 1.6 {
		t.Fatalf("своя 5★: фото 3 больше других в %.2f раза (%s)", big, key)
	}
	if big, key := run(nil, 5); big < 1.6 {
		t.Fatalf("XMP 5★: фото 3 больше других в %.2f раза (%s)", big, key)
	}
	_, plain := run(nil, 0)
	if _, key := run(edtest.Ip(2), 5); key != plain {
		t.Fatalf("своя 2★ поверх XMP 5★: %s, без оценок %s", key, plain)
	}
}

// TestAutoBuildVarietyStar: звезда работает и в сборке: 5★ у последнего фото — главное;
// «Меньше» звезду не смотрит (лучшая, Layout "").
func TestAutoBuildVarietyStar(t *testing.T) {
	ph := horizontals("s", 4)
	ph[3].Xmp.Rating = 5
	e := edtest.Open(t, model.New(), "")
	v := build(t, e, ph, -1, model.AutoOpts{Spreads: 1, MaxPer: 64, Variety: model.VarietyNormal})
	if big := bigOf(v.Spreads[0].Rects, 3); big < 1.6 {
		t.Fatalf("фото 3 больше других в %.2f раза (%s)", big, v.Spreads[0].Layout)
	}
	e = edtest.Open(t, model.New(), "")
	if build(t, e, ph, -1, model.AutoOpts{Spreads: 1, MaxPer: 64}); edtest.P(e).Spreads[0].Layout != "" {
		t.Fatal("«Меньше» со звездой записало ключ")
	}
}

// checkRow — поля плиток ряда: семейство из четырёх, Twin — более ранний с тем же видом.
func checkRow(t *testing.T, name string, rows []model.Choice, prm layout.Params) (folded int) {
	t.Helper()
	fams := map[string]bool{"dense": true, "hero": true, "air": true, "bleed": true}
	for i, c := range rows {
		if !fams[c.Family] {
			t.Fatalf("%s %s: семейство %q", name, c.Key, c.Family)
		}
		switch {
		case c.Twin == -1:
		case c.Twin < 0 || c.Twin >= i || variety.Look(rows[c.Twin].Rects, prm) != variety.Look(c.Rects, prm) || rows[c.Twin].Twin != -1:
			t.Fatalf("%s %s: twin %d", name, c.Key, c.Twin)
		default:
			folded++
		}
	}
	return folded
}

// TestLayoutChoicesVariety: 12 одинаковых горизонтальных — видимых без похожих ≤ 8 из 24;
// смешанный набор — свёрнуто ≤ 2. «Другие варианты» — вне ряда «Авто», строятся, по убыванию
// оценки, ≤ variety.ExtraLimit (воздух ≤ 12 + с главным фото ≤ 12, решение 0.11 №57), с теми же
// полями; у страниц — пусто (не null).
func TestLayoutChoicesVariety(t *testing.T) {
	for _, c := range []struct {
		name    string
		ph      []model.PhotoInfo
		maxSeen int // видимых в ряду «Авто», −1 — не проверять
		maxFold int // свёрнутых, −1 — не проверять
	}{
		{"12 гориз.", horizontals("h", 12), 8, -1},
		{"смесь 10", mixedPhotos("m", 10), -1, 2},
		{"смесь 6", mixedPhotos("x", 6), -1, -1},
	} {
		e := edtest.Open(t, model.New(), "")
		id := edtest.SpreadIDs(build(t, e, c.ph, -1, model.AutoOpts{Spreads: 1, MaxPer: 64}))[0]
		ch, p := e.LayoutChoices(id), edtest.P(e)
		prm, asp := p.Params(), p.Aspects(p.Spreads[0].Photos)
		folded := checkRow(t, c.name, ch.Whole.InOrder, prm)
		checkRow(t, c.name+" перест.", ch.Whole.Rearranged, prm)
		if seen := len(ch.Whole.InOrder) - folded; c.maxSeen >= 0 && seen > c.maxSeen || c.maxFold >= 0 && folded > c.maxFold {
			t.Fatalf("%s: видимых %d, свёрнуто %d", c.name, seen, folded)
		}
		ex := ch.Whole.Extra
		if len(ex) == 0 || len(ex) > variety.ExtraLimit {
			t.Fatalf("%s: «Других вариантов» %d", c.name, len(ex))
		}
		checkRow(t, c.name+" другие", ex, prm)
		last := 2.0
		for _, x := range ex {
			r, ok := layout.Build(x.Key, asp, prm)
			if !ok || !slices.Equal(r.Rects, x.Rects) || slices.ContainsFunc(ch.Whole.InOrder, func(y model.Choice) bool { return y.Key == x.Key }) {
				t.Fatalf("%s: другой вариант %s не строится или есть в ряду «Авто»", c.name, x.Key)
			}
			if r.Score > last+1e-9 { // ничьи — в квантах, как в layout
				t.Fatalf("%s: «Другие варианты» не по убыванию оценки", c.name)
			}
			last = r.Score
		}
		for _, side := range []*model.ChoiceRows{ch.Left, ch.Right} {
			if side != nil {
				checkRow(t, c.name+" страница", side.InOrder, prm)
				if side.Extra == nil || len(side.Extra) != 0 {
					t.Fatalf("%s: у страницы Extra %v", c.name, side.Extra)
				}
			}
		}
	}
	// чужой разворот — пустые массивы, не null
	data, _ := json.Marshal(edtest.Open(t, model.New(), "").LayoutChoices("нет"))
	if !strings.Contains(string(data), `"extra":[]`) {
		t.Fatalf("JSON без extra: %s", data)
	}
}

// TestStepLayoutExtra (решение 0.11 №36): ключ из «Других вариантов» ставится SetLayout;
// Пробел и ‹ › идут по ряду «Авто», затем по «Другим вариантам» и по кругу; счётчик «i / N» —
// по объединённому ряду.
func TestStepLayoutExtra(t *testing.T) {
	e := edtest.Open(t, model.New(), "")
	sid := edtest.SpreadIDs(build(t, e, mixedPhotos("x", 6), -1, model.AutoOpts{Spreads: 1, MaxPer: 64}))[0]
	ch := e.LayoutChoices(sid)
	row, ex := ch.Whole.InOrder, ch.Whole.Extra
	if len(ex) < 2 {
		t.Fatalf("«Других вариантов» %d — мало для листания по ним", len(ex))
	}
	n := len(row) + len(ex)
	at := func(v model.View) (string, int, int) { s := v.Spreads[0]; return s.Layout, s.Variant, s.Variants }
	for _, c := range []struct {
		name  string
		from  string
		delta int
		want  string
		i     int
	}{
		{"из ряда в другие", row[len(row)-1].Key, 1, ex[0].Key, len(row)},
		{"по другим", ex[0].Key, 1, ex[1].Key, len(row) + 1},
		{"с последнего другого — к первому", ex[len(ex)-1].Key, 1, row[0].Key, 0},
		{"назад с первого — на последний другой", row[0].Key, -1, ex[len(ex)-1].Key, n - 1},
		{"назад с первого другого — в ряд", ex[0].Key, -1, row[len(row)-1].Key, len(row) - 1},
	} {
		if key, i, all := at(e.SetLayout(sid, c.from)); key != c.from || all != n {
			t.Fatalf("%s: SetLayout(%s) → %s, %d / %d", c.name, c.from, key, i, all)
		}
		if key, i, all := at(e.StepLayout(sid, "spread", c.delta)); key != c.want || i != c.i || all != n {
			t.Fatalf("%s: %s %d / %d, ждали %s %d / %d", c.name, key, i, all, c.want, c.i, n)
		}
	}
}

// TestPhotosOf: фото выделенных разворотов по порядку ids; неизвестные пропускаются, пусто — [].
func TestPhotosOf(t *testing.T) {
	e, s1, s2 := edtest.NewTwo(t)
	for _, c := range []struct {
		ids  []string
		want []string
	}{
		{[]string{s2, s1}, []string{"c", "a", "b"}},
		{[]string{s1, "нет", s2}, []string{"a", "b", "c"}},
		{[]string{"нет"}, []string{}},
		{nil, []string{}},
	} {
		if got := e.PhotosOf(c.ids); got == nil || !slices.Equal(got, c.want) {
			t.Fatalf("PhotosOf(%v) = %#v, ждали %v", c.ids, got, c.want)
		}
	}
}
