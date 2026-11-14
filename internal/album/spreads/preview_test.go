package spreads_test

// Спека 065: ряд окна автосборки (Editor.AutoPreview) — «Что соберётся» совпадает со сборкой,
// «Варианты» — подборка разворота, проект не меняется.

import (
	"bytes"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// previewAlbum — альбом: два разворота с фото (часть фото уедет в сборку), звёзды 5★ у двух
// новых фото, пустой хвост из двух разворотов. Возвращает фото сборки.
func previewAlbum(t *testing.T) (*album.Editor, []model.PhotoInfo) {
	t.Helper()
	e := edtest.Open(t, model.New(), "")
	old := mixedPhotos("a", 9)
	edtest.NewSpread(e, -1, old[:4])
	edtest.NewSpread(e, -1, old[4:])
	edtest.NewSpread(e, -1, nil)
	edtest.NewSpread(e, -1, nil)
	ph := slices.Concat(old[2:4], old[7:], mixedPhotos("n", 40))
	ph[5].Xmp.Rating, ph[17].Xmp.Rating = 5, 5
	return e, ph
}

// built — новые развороты после сборки (ID, которых не было) по порядку альбома.
func built(p *model.Project, was map[string]bool) []*model.Spread {
	var out []*model.Spread
	for _, s := range p.Spreads {
		if !was[s.ID] {
			out = append(out, s)
		}
	}
	return out
}

// TestAutoPreviewMatchesBuild: «Что соберётся» == AutoBuild с теми же аргументами — число,
// фото и действующий ключ каждого разворота, запасные — на всех уровнях, с семействами и без,
// в конец (пустой хвост) и в середину, с фото из альбома и звёздами.
func TestAutoPreviewMatchesBuild(t *testing.T) {
	for _, v := range levels {
		for _, fs := range [][]string{nil, {"dense"}, {"strip", "figure"}} {
			for _, at := range []int{-1, 1} {
				name := fmt.Sprintf("v%d/%v/at%d", v, fs, at)
				e, ph := previewAlbum(t)
				o := model.AutoOpts{Spreads: 8, MinPer: 3, MaxPer: 9, Variety: v, Families: fs}
				pv, err := e.AutoPreview(ph, at, o)
				if err != nil {
					t.Fatalf("%s: %v", name, err)
				}
				was := map[string]bool{}
				for _, s := range edtest.P(e).Spreads {
					was[s.ID] = true
				}
				build(t, e, ph, at, o)
				samePreview(t, name, edtest.P(e), built(edtest.P(e), was), pv, fs)
			}
		}
	}
	// 078: похожие, ч/б и вариант — то же совпадение, и пометки ч/б как у сборки.
	for _, o := range []model.AutoOpts{
		{Spreads: 8, MinPer: 2, MaxPer: 9, Variety: model.VarietyNormal, Similar: 2, Gray: 2, Variant: 4},
		{Spreads: 8, Auto: true, Variety: model.VarietyMore, Similar: 3, Gray: 3, Variant: 2, Families: []string{"dense"}},
	} {
		for _, at := range []int{-1, 1} {
			previewMatches078(t, o, at)
		}
	}
}

// previewMatches078 — «Что соберётся» == сборка с похожими, ч/б и вариантом (078) из at, и пометки
// ч/б — как у собранных разворотов.
func previewMatches078(t *testing.T, o model.AutoOpts, at int) {
	t.Helper()
	name := fmt.Sprintf("078 %+v/at%d", o, at)
	e, ph := previewAlbum(t)
	sig, _ := scenes("x", len(ph))
	for i := range ph {
		ph[i].Sig = sig[i].Sig
	}
	pv, err := e.AutoPreview(ph, at, o)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	if ap, _ := e.AutoPlan(ph, o); ap.Similar == 0 || ap.Gray == 0 {
		t.Fatalf("%s: отбор не сработал — план %+v", name, ap)
	}
	was := map[string]bool{}
	for _, s := range edtest.P(e).Spreads {
		was[s.ID] = true
	}
	build(t, e, ph, at, o)
	got := built(edtest.P(e), was)
	samePreview(t, name, edtest.P(e), got, pv, o.Families)
	for i, s := range got {
		gray := true
		for _, id := range s.Photos {
			gray = gray && edtest.P(e).Photos[id].Adj != nil && edtest.P(e).Photos[id].Adj.Gray
		}
		if gray != pv.Spreads[i].Gray {
			t.Errorf("%s: разворот %d ч/б %v, в предпросмотре %v", name, i, gray, pv.Spreads[i].Gray)
		}
	}
}

// samePreview — собранные развороты got == «Что соберётся» pv: число, фото, действующий ключ,
// семейство и форма; запасной — лучшая и не из разрешённых fs; счётчик запасных сходится.
func samePreview(t *testing.T, name string, p *model.Project, got []*model.Spread, pv model.AutoPreview, fs []string) {
	t.Helper()
	if len(got) != len(pv.Spreads) || len(got) != 8 {
		t.Fatalf("%s: собрано %d, предпросмотр %d", name, len(got), len(pv.Spreads))
	}
	fb := 0
	for i, s := range got {
		r, best, _ := p.Layout(s)
		w := pv.Spreads[i]
		if r.Key != w.Key || len(s.Photos) != w.Photos || w.Twin != -1 || w.FormName == "" || w.Family != famOf(p, r) {
			t.Errorf("%s: разворот %d — собран %s (%d фото), показан %+v", name, i, r.Key, len(s.Photos), w.Choice)
		}
		if w.Fallback {
			fb++
			if r.Key != best[0].Key || slices.Contains(fs, w.Family) {
				t.Errorf("%s: запасной %d — %s, не лучшая или разрешённая", name, i, r.Key)
			}
		}
	}
	if fb != pv.Fallback || (fs == nil && fb != 0) {
		t.Errorf("%s: запасных %d, Fallback %d", name, fb, pv.Fallback)
	}
}

// TestAutoPreviewSamples: по образцу на каждое число фото плана, по возрастанию; образец — первая
// группа этого размера; его ряды == подборка (LayoutChoices) собранного разворота из тех же фото,
// в том числе на 17+ фото (первая порция ряда, без «Ещё»).
func TestAutoPreviewSamples(t *testing.T) {
	cases := []struct {
		ph []model.PhotoInfo
		o  model.AutoOpts
	}{
		{mixedPhotos("s", 50), model.AutoOpts{Spreads: 9, MinPer: 2, MaxPer: 10, Variety: model.VarietyLess}},
		{mixedPhotos("m", 40), model.AutoOpts{Spreads: 2, MaxPer: 20, Variety: model.VarietyLess}},
	}
	for ci, c := range cases {
		e := edtest.Open(t, model.New(), "")
		pv, err := e.AutoPreview(c.ph, -1, c.o)
		if err != nil {
			t.Fatal(err)
		}
		first := map[int]int{}
		for i, s := range pv.Spreads {
			if _, ok := first[s.Photos]; !ok {
				first[s.Photos] = i
			}
		}
		if len(pv.Samples) != len(first) || len(pv.Samples) == 0 {
			t.Fatalf("случай %d: образцов %d, размеров %d", ci, len(pv.Samples), len(first))
		}
		if ci == 0 && len(first) < 2 {
			t.Fatalf("случай 0: один размер групп %v — сценарий не проверяет порядок", first)
		}
		ids := edtest.SpreadIDs(build(t, e, c.ph, -1, c.o))
		for k, smp := range pv.Samples {
			if k > 0 && smp.Photos <= pv.Samples[k-1].Photos {
				t.Errorf("случай %d: образцы не по возрастанию: %d после %d", ci, smp.Photos, pv.Samples[k-1].Photos)
			}
			if first[smp.Photos] != smp.Group {
				t.Errorf("случай %d: образец %d фото — группа %d, первая такая %d", ci, smp.Photos, smp.Group, first[smp.Photos])
			}
			ch := e.LayoutChoices(ids[smp.Group])
			if !reflect.DeepEqual(smp.InOrder, ch.Whole.InOrder) || !reflect.DeepEqual(smp.Extra, ch.Whole.Extra) {
				t.Errorf("случай %d: образец %d фото: ряды %d+%d ≠ подборка %d+%d", ci, smp.Photos,
					len(smp.InOrder), len(smp.Extra), len(ch.Whole.InOrder), len(ch.Whole.Extra))
			}
		}
	}
}

// TestAutoPreviewReadOnly: предпросмотр не меняет проект, rev, отмену и возврат, «изменён»;
// нет фото — пустые ряды без ошибки (массивы, не null); места нет — ошибка, как у AutoPlan.
func TestAutoPreviewReadOnly(t *testing.T) {
	e, ph := previewAlbum(t)
	edtest.NewSpread(e, -1, mixedPhotos("u", 2))
	e.Undo() // непустой возврат
	snap := model.Marshal(edtest.P(e))
	rev, undo, redo, dirty := edtest.Rev(e), edtest.UndoLen(e), edtest.RedoLen(e), edtest.Dirty(e)
	for _, o := range []model.AutoOpts{
		{Spreads: 6, MaxPer: 12, Variety: model.VarietyNormal},
		{Spreads: 6, MaxPer: 12, Variety: model.VarietyLess, Families: []string{"strip"}},
		{Spreads: 6, Auto: true, Variety: model.VarietyMore, Families: []string{"dense", "hero"}},
	} {
		for _, at := range []int{-1, 0, 2} {
			if pv, err := e.AutoPreview(ph, at, o); err != nil || len(pv.Spreads) == 0 || pv.W <= 0 || pv.H <= 0 {
				t.Fatalf("%+v at %d: %v, %d разворотов, лист %v×%v", o, at, err, len(pv.Spreads), pv.W, pv.H)
			}
		}
	}
	if !bytes.Equal(model.Marshal(edtest.P(e)), snap) || edtest.Rev(e) != rev || edtest.UndoLen(e) != undo ||
		edtest.RedoLen(e) != redo || edtest.Dirty(e) != dirty {
		t.Fatalf("предпросмотр изменил альбом: rev %d → %d, отмена %d → %d, возврат %d → %d", rev, edtest.Rev(e), undo,
			edtest.UndoLen(e), redo, edtest.RedoLen(e))
	}

	pv, err := e.AutoPreview(nil, -1, model.AutoOpts{Spreads: 3, MaxPer: 12})
	if err != nil || pv.Samples == nil || pv.Spreads == nil || len(pv.Samples)+len(pv.Spreads)+pv.Fallback != 0 {
		t.Errorf("без фото: %+v %v", pv, err)
	}
	full := fullWithTail(t, 0, 0)
	_, planErr := full.AutoPlan(mixedPhotos("f", 4), model.AutoOpts{Spreads: 1, MaxPer: 64})
	if _, err := full.AutoPreview(mixedPhotos("f", 4), -1, model.AutoOpts{Spreads: 1, MaxPer: 64}); err == nil ||
		planErr == nil || err.Error() != planErr.Error() || !strings.Contains(err.Error(), "разворотов") {
		t.Errorf("полный альбом: %v, у плана %v", err, planErr)
	}
}

// TestAutoPreviewCost (design §3): 300 фото / 60 разворотов на тёплом кэше — в пределах
// 150 мс: копия проекта, план и выбор, без перебора раскладок.
func TestAutoPreviewCost(t *testing.T) {
	testkit.CostTest(t)
	e, ph := previewCost(t)
	o := model.AutoOpts{Spreads: 60, MaxPer: 9, Variety: model.VarietyNormal, Families: []string{"dense", "hero"}}
	if _, err := e.AutoPreview(ph, -1, o); err != nil { // прогрев кэша рядов
		t.Fatal(err)
	}
	if d := testkit.Fastest(3, func() { _, _ = e.AutoPreview(ph, -1, o) }); d > testkit.Limit(150*time.Millisecond) {
		t.Fatalf("предпросмотр на тёплом кэше %v", d)
	}
}

// previewCost — альбом из 40 разворотов по 5 фото и 300 новых фото.
func previewCost(tb testing.TB) (*album.Editor, []model.PhotoInfo) {
	tb.Helper()
	e := edtest.Open(tb, model.New(), "")
	old := mixedPhotos("c", 200)
	for i := 0; i < len(old); i += 5 {
		edtest.NewSpread(e, -1, old[i:i+5])
	}
	return e, mixedPhotos("p", 300)
}

// BenchmarkAutoPreview: копия проекта (Clone) и весь предпросмотр на тёплом кэше.
func BenchmarkAutoPreview(b *testing.B) {
	e, ph := previewCost(b)
	o := model.AutoOpts{Spreads: 60, MaxPer: 9, Variety: model.VarietyNormal}
	if _, err := e.AutoPreview(ph, -1, o); err != nil {
		b.Fatal(err)
	}
	b.Run("Clone", func(b *testing.B) {
		p := edtest.P(e)
		for b.Loop() {
			p.Clone()
		}
	})
	b.Run("AutoPreview", func(b *testing.B) {
		for b.Loop() {
			if _, err := e.AutoPreview(ph, -1, o); err != nil {
				b.Fatal(err)
			}
		}
	})
}
