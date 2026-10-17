package spreads_test

// 055 «Лесенка честно и чаще»: лесенка в автосборке на живых пропорциях stock, чередование
// направлений у «Разнообразить» и ключ лесенки альбомов 0.13, ушедший из ряда.

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/layout/forms"
	"github.com/IliaSA101/sosninaalbums/internal/layout/variety"
)

// ladderDirs — направление лесенки каждого разворота ('L', 'A'; 0 — не лесенка).
func ladderDirs(p *model.Project) []byte {
	out := make([]byte, len(p.Spreads))
	for i, s := range p.Spreads {
		if r, _, ok := p.Layout(s); ok && len(s.Grid) == 0 && variety.Describe(r, p.Params()).Form == forms.Ladder {
			out[i] = forms.LadderDir(r.Key)
		}
	}
	return out
}

// stockBuild — stock 100 фото на n разворотов «Авто» уровня v (30×30 по умолчанию).
func stockBuild(t *testing.T, n int, v model.Variety) *album.Editor {
	t.Helper()
	e := edtest.Open(t, model.New(), "")
	build(t, e, stockPhotos(t), -1, model.AutoOpts{Spreads: n, Auto: true, Variety: v})
	return e
}

// TestAutoBuildLadderStock (разборы A и B): stock 100, 30×30. «Меньше» — без лесенок (лучшая, как
// было); «Обычное» на 20 и 25 разворотов — лесенка есть; не чаще 20 % разворотов; «Больше» по
// 16–25 разворотам вместе — не реже «Обычного» (на одном N у них по 1–2 лесенки и на 25 «Больше»
// ставит 1 против 2: decisions 0.13.1, «055 — реализация»); двух лесенок подряд нет; встречаются L и
// A; в «Обычном» нет лесенки с пустой страницей (её Q за крупными).
func TestAutoBuildLadderStock(t *testing.T) {
	seen := map[byte]bool{}
	count := map[model.Variety]int{}
	for _, v := range []model.Variety{model.VarietyLess, model.VarietyNormal, model.VarietyMore} {
		for _, n := range []int{16, 20, 25} {
			e := stockBuild(t, n, v)
			p := edtest.P(e)
			dirs, k := ladderDirs(p), 0
			for i, d := range dirs {
				if d == 0 {
					continue
				}
				k++
				seen[d] = true
				if i > 0 && dirs[i-1] != 0 {
					t.Errorf("уровень %d, %d разворотов: лесенки подряд на %d и %d", v, n, i-1, i)
				}
				if key := p.Spreads[i].Layout; v == model.VarietyNormal && (strings.Contains(key, "[-|") || strings.Contains(key, "|-]")) {
					t.Errorf("«Обычное», %d разворотов: лесенка с пустой страницей %q", n, key)
				}
			}
			count[v] += k
			t.Logf("уровень %d, %d разворотов: лесенок %d из %d, направления %q", v, n, k, len(dirs), strings.ReplaceAll(string(dirs), "\x00", "·"))
			switch {
			case v == model.VarietyLess && k != 0:
				t.Errorf("«Меньше», %d разворотов: лесенок %d", n, k)
			case v == model.VarietyNormal && n >= 20 && k < 1, k*5 > len(dirs):
				t.Errorf("уровень %d, %d разворотов: лесенок %d из %d", v, n, k, len(dirs))
			}
		}
	}
	if count[model.VarietyMore] < count[model.VarietyNormal] {
		t.Errorf("«Больше» реже «Обычного»: %v", count)
	}
	if !seen['L'] || !seen['A'] {
		t.Errorf("направления лесенок: L %v, A %v", seen['L'], seen['A'])
	}
}

// TestVaryLadderCtx: «Разнообразить» части альбома после разворота с лесенкой ставит первую
// лесенку другого направления — чередование не начинается заново с L (Ctx.Ladder из ctxOf).
func TestVaryLadderCtx(t *testing.T) {
	probe := edtest.P(stockBuild(t, 25, model.VarietyMore))
	dirs := ladderDirs(probe)
	at := -1
	for i, d := range dirs {
		if d != 0 && i+1 < len(dirs) {
			at = i
			break
		}
	}
	if at < 0 {
		t.Fatal("в сборке нет лесенки: тест не проверяет")
	}
	key := probe.Spreads[at].Layout
	for _, k := range []string{key, forms.MirrorLadder(key)} {
		e := stockBuild(t, 25, model.VarietyMore)
		e.SetLayout(edtest.P(e).Spreads[at].ID, k)
		var ids []string
		for _, s := range edtest.P(e).Spreads[at+1:] {
			ids = append(ids, s.ID)
		}
		e.VaryLayouts(ids, model.VarietyMore, nil)
		after := ladderDirs(edtest.P(e))
		if after[at] != forms.LadderDir(k) {
			t.Fatalf("лесенка %q не встала: %q", k, after[at])
		}
		next := byte(0)
		for _, d := range after[at+1:] {
			if d != 0 {
				next = d
				break
			}
		}
		if next == 0 {
			t.Fatalf("после %q «Разнообразить» не поставило лесенку: тест не проверяет", k)
		}
		if next == after[at] {
			t.Errorf("после %q первая лесенка того же направления %q", k, next)
		}
	}
}

// TestOldLadderKeyStays (риск 1 разбора A): ключ лесенки альбома 0.13, ушедший из ряда «Авто»
// (P[A(0,1,2)|3] на 30×20, 4 фото «V V H V»), рисуется как был, переживает круг Save → Load, а
// листание ‹ › с него начинает с начала ряда, не падая.
func TestOldLadderKeyStays(t *testing.T) {
	const key = "P[A(0,1,2)|3]"
	e := oldLadderAlbum(t, key)
	p := edtest.P(e)
	s := p.Spreads[0]
	asp, prm := p.Aspects(s.Photos), p.Params()
	want, ok := layout.Build(key, asp, prm)
	if !ok {
		t.Fatalf("%s не строится", key)
	}
	if _, i, _ := model.AutoRow(asp, prm, key); i >= 0 {
		t.Logf("%s в ряду «Авто» на месте %d", key, i)
	}
	if r, _, ok := p.Layout(s); !ok || r.Key != key || len(r.Rects) != len(want.Rects) {
		t.Fatalf("действующая раскладка %q, ждали %q", r.Key, key)
	}
	if got := edtest.SaveLoad(t, e).Spreads[0].Layout; got != key {
		t.Fatalf("после Save → Load ключ %q", got)
	}
	e.StepLayout(s.ID, "spread", 1)
	if got := edtest.P(e).Spreads[0].Layout; got == key {
		t.Errorf("‹ › не сдвинули раскладку с %q", got)
	}
}

// oldLadderAlbum — альбом 30×20 с одним разворотом из 4 фото и ключом лесенки key.
func oldLadderAlbum(t *testing.T, key string) *album.Editor {
	t.Helper()
	pr := model.New()
	pr.Settings.PageH = 200
	e := edtest.Open(t, pr, "")
	var ph []model.PhotoInfo
	for i, wh := range [][2]int{{200, 300}, {200, 300}, {300, 200}, {200, 300}} {
		id := fmt.Sprint("o", i)
		ph = append(ph, model.PhotoInfo{ID: id, Path: edtest.Abs(id + ".jpg"), W: wh[0], H: wh[1]})
	}
	build(t, e, ph, -1, model.AutoOpts{Spreads: 1, MaxPer: 4})
	e.SetLayout(edtest.P(e).Spreads[0].ID, key)
	if got := edtest.P(e).Spreads[0].Layout; got != key {
		t.Fatalf("ключ %q не встал: %q", key, got)
	}
	return e
}

// whPhotos — фото prefix0, prefix1… с размерами wh (ширина, высота).
func whPhotos(prefix string, wh ...[2]int) []model.PhotoInfo {
	out := make([]model.PhotoInfo, len(wh))
	for i, x := range wh {
		id := fmt.Sprint(prefix, i)
		out[i] = model.PhotoInfo{ID: id, Path: edtest.Abs(id + ".jpg"), W: x[0], H: x[1]}
	}
	return out
}

// choiceKeys — все ключи подборки разворота: то, что интерфейс может прислать в SetLayout.
func choiceKeys(c model.Choices) map[string]bool {
	out := map[string]bool{}
	for _, r := range []*model.ChoiceRows{&c.Whole, c.Left, c.Right} {
		if r == nil {
			continue
		}
		for _, x := range slices.Concat(r.InOrder, r.Rearranged, r.Extra) {
			out[x.Key] = true
		}
	}
	return out
}

// TestSetLayoutSpineOutOfRow (decisions 0.13.1 №23): ключ не из подборки, у которого узкое фото
// лежит на корешке (A(0,1,2,3) на «V 0.74 V V», 30×30), — холостая правка: ни rev, ни шага
// отмены, и у сетки «Вернуть авто» не срабатывает. Тот же ключ с широким фото через корешок
// ставится; ключ из подборки с узким фото на корешке (ряд Layouts корешок только штрафует) — тоже;
// оба переживают Save → Load, отмена снимает их по шагу.
func TestSetLayoutSpineOutOfRow(t *testing.T) {
	const key = "A(0,1,2,3)"
	v, h := [2]int{200, 300}, [2]int{300, 200}
	e := edtest.Open(t, model.New(), "")
	edtest.NewSpread(e, -1, whPhotos("n", v, [2]int{222, 300}, v, v))
	edtest.NewSpread(e, -1, whPhotos("w", h, h, h, [2]int{222, 300}))
	view := edtest.NewSpread(e, -1, whPhotos("r", [2]int{222, 300}, v))
	p := edtest.P(e)
	prm := p.Params()
	ids := []string{view.Spreads[0].ID, view.Spreads[1].ID, view.Spreads[2].ID}

	// Предусловия: ключ строится на обоих, его нет в подборке; на первом узкое фото на корешке.
	for i, spineOK := range []bool{false, true} {
		r, ok := layout.Build(key, p.Aspects(p.Spreads[i].Photos), prm)
		if !ok || forms.SpineOK(r, prm) != spineOK {
			t.Fatalf("разворот %d: Build %v, SpineOK %v (ждали %v) — тест не проверяет", i, ok, forms.SpineOK(r, prm), spineOK)
		}
		if choiceKeys(e.LayoutChoices(ids[i]))[key] {
			t.Fatalf("разворот %d: %s в подборке — тест не проверяет", i, key)
		}
	}
	// Ключ подборки с узким фото на корешке, кроме стоящего (лучшая пишется как "").
	rowKey := ""
	asp := p.Aspects(p.Spreads[2].Photos)
	cur, _, _ := p.Layout(p.Spreads[2])
	for k := range choiceKeys(e.LayoutChoices(ids[2])) {
		if r, ok := layout.Build(k, asp, prm); ok && !forms.SpineOK(r, prm) && k != cur.Key && k > rowKey {
			rowKey = k // наибольший — детерминированно
		}
	}
	if rowKey == "" {
		t.Fatal("в подборке нет ключа с узким фото на корешке — тест не проверяет")
	}

	rev, undo := e.View().Rev, edtest.UndoLen(e)
	if got := e.SetLayout(ids[0], key); got.Rev != rev || edtest.UndoLen(e) != undo || edtest.P(e).Spreads[0].Layout != "" {
		t.Fatalf("ключ с узким фото на корешке встал: %q", edtest.P(e).Spreads[0].Layout)
	}
	edtest.FreezeAt(t, e, 0)
	rev, undo = e.View().Rev, edtest.UndoLen(e)
	if e.SetLayout(ids[0], key).Rev != rev || edtest.UndoLen(e) != undo || len(edtest.P(e).Spreads[0].Grid) == 0 {
		t.Fatal("у сетки ключ с узким фото на корешке снял закрепление")
	}

	e.SetLayout(ids[1], key)
	e.SetLayout(ids[2], rowKey)
	q := edtest.P(e)
	if q.Spreads[1].Layout != key || q.Spreads[2].Layout != rowKey || edtest.UndoLen(e) != undo+2 {
		t.Fatalf("допустимые ключи не встали: %q, %q (ждали %q, %q), шагов %d", q.Spreads[1].Layout, q.Spreads[2].Layout, key, rowKey, edtest.UndoLen(e)-undo)
	}
	if s := edtest.SaveLoad(t, e).Spreads; s[1].Layout != key || s[2].Layout != rowKey {
		t.Fatalf("после Save → Load: %q, %q", s[1].Layout, s[2].Layout)
	}
	e.Undo()
	e.Undo()
	if q := edtest.P(e); q.Spreads[1].Layout != "" || q.Spreads[2].Layout != "" || len(q.Spreads[0].Grid) == 0 {
		t.Fatalf("отмена: %q, %q", q.Spreads[1].Layout, q.Spreads[2].Layout)
	}

	// Ключ уже стоит (как из старого файла): ветка r.Key == s.Layout в foldAllowed — «Вернуть авто»
	// срабатывает, хоть узкое фото на корешке и ключа нет в подборке. Разворот 0 всё ещё сетка.
	edtest.Change(e, func(p *model.Project) { p.Spreads[0].Layout = key })
	edtest.FreezeAt(t, e, 0) // уже закреплён — холостая правка, но проверяет, что сетка есть
	rev, undo = e.View().Rev, edtest.UndoLen(e)
	if e.SetLayout(ids[0], key).Rev == rev || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("стоящий ключ не снял закрепление: шагов %d (ждали %d)", edtest.UndoLen(e), undo+1)
	}
	if q := edtest.P(e); len(q.Spreads[0].Grid) != 0 || q.Spreads[0].Layout != key {
		t.Fatalf("после снятия: сетка %d мест, ключ %q", len(q.Spreads[0].Grid), q.Spreads[0].Layout)
	}
	if got := edtest.SaveLoad(t, e).Spreads[0]; len(got.Grid) != 0 || got.Layout != key {
		t.Fatalf("после Save → Load: сетка %d мест, ключ %q", len(got.Grid), got.Layout)
	}
	e.Undo()
	if q := edtest.P(e); len(q.Spreads[0].Grid) == 0 || edtest.UndoLen(e) != undo {
		t.Fatalf("отмена не вернула сетку: %d мест, шагов %d (ждали %d)", len(q.Spreads[0].Grid), edtest.UndoLen(e), undo)
	}
}
