package spreads_test

// Спека 027: автосборка ровно на N разворотов (AutoPlan = AutoBuild) и ряд «Авто» на
// 17+ фото порциями по 12 (счётчик «+», листание за конец, «Ещё» в галерее).

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// mixedPhotos — n фото подряд: каждое третье портретное, каждое седьмое квадратное.
func mixedPhotos(prefix string, n int) []model.PhotoInfo {
	out := make([]model.PhotoInfo, n)
	for i := range out {
		w, h := 300, 200
		switch {
		case i%7 == 3:
			w, h = 250, 250
		case i%3 == 0:
			w, h = 200, 300
		}
		id := fmt.Sprint(prefix, i)
		out[i] = model.PhotoInfo{ID: id, Path: edtest.Abs(id + ".jpg"), W: w, H: h}
	}
	return out
}

// TestAutoPlanMatchesBuild: AutoBuild(…, N, m) делает ровно те развороты, что показал
// AutoPlan (число, Min, Max, Avg); фото из сеток и с фонов не считаются ни там, ни там;
// повёрнутое фото альбома считается с действующими пропорциями.
func TestAutoPlanMatchesBuild(t *testing.T) {
	ph := mixedPhotos("p", 30)
	for _, c := range []struct{ n, m, want int }{{1, 64, 1}, {5, 64, 5}, {1, 4, 8}, {30, 64, 30}, {99, 64, 30}, {3, 10, 3}, {7, 0, 30}} {
		e := edtest.Open(t, model.New(), "")
		edtest.NewSpread(e, -1, ph[:2]) // p0, p1 уже в альбоме; p0 повёрнуто
		e.TurnPhotos([]string{"p0"}, 1)
		edtest.NewSpread(e, -1, edtest.Photos("g"))
		edtest.FreezeAt(t, e, 1)
		bg := edtest.NewSpread(e, -1, edtest.Photos("x")).Spreads[2].ID
		edtest.SetBg(t, e, bg, "spread", "bg")
		all := slices.Concat(ph[:10], edtest.Photos("g", "bg"), ph[10:])

		ap, err := e.AutoPlan(all, model.AutoOpts{Spreads: c.n, MaxPer: c.m})
		if err != nil || ap.Photos != 30 || ap.Spreads != c.want {
			t.Fatalf("N=%d m=%d: план %+v, ошибка %v", c.n, c.m, ap, err)
		}
		v, err := e.AutoBuild(all, -1, model.AutoOpts{Spreads: c.n, MaxPer: c.m})
		if err != nil {
			t.Fatal(err)
		}
		var sizes []int
		for _, s := range v.Spreads {
			if len(s.Photos) > 0 && strings.HasPrefix(s.Photos[0], "p") {
				sizes = append(sizes, len(s.Photos))
			}
		}
		total := 0
		for _, k := range sizes {
			total += k
		}
		if len(sizes) != ap.Spreads || slices.Min(sizes) != ap.Min || slices.Max(sizes) != ap.Max || total != 30 ||
			float64(total)/float64(len(sizes)) != ap.Avg {
			t.Fatalf("N=%d m=%d: сборка %v, план %+v", c.n, c.m, sizes, ap)
		}
		if slices.Max(sizes) > max(1, min(c.m, model.MaxPhotos)) {
			t.Fatalf("m=%d: разворот на %d фото", c.m, slices.Max(sizes))
		}
		if !slices.ContainsFunc(edtest.P(e).Spreads, func(s *model.Spread) bool { return slices.Contains(model.GridPhotos(s), "g") }) {
			t.Fatal("фото сетки ушло в автосборку")
		}
		if !slices.Contains(v.BackdropUsed, "bg") {
			t.Fatal("фото фона ушло в автосборку")
		}
	}
}

// AutoPlan без фото — пустой план без ошибки; альбом полон — ошибка про 1000 разворотов.
func TestAutoPlanEdges(t *testing.T) {
	e := edtest.Open(t, model.New(), "")
	if ap, err := e.AutoPlan(nil, model.AutoOpts{Spreads: 5, MaxPer: 64}); err != nil || ap != (model.AutoPlan{}) {
		t.Fatalf("без фото: %+v %v", ap, err)
	}
	edtest.Change(e, func(p *model.Project) {
		for range model.MaxSpreads {
			p.Spreads = append(p.Spreads, &model.Spread{ID: model.NewID(), Photos: []string{}, BgL: "#ffffff"}) // не пустые: пустой хвост сборка заняла бы (035)
		}
	})
	ap, err := e.AutoPlan(edtest.Photos("a", "b"), model.AutoOpts{Spreads: 1, MaxPer: 64})
	if err == nil || !strings.Contains(err.Error(), "1000") || ap.Photos != 2 || ap.MaxSpreads != 0 {
		t.Fatalf("полный альбом: %+v %v", ap, err)
	}
	// Почти полон: верх ползунка — оставшееся место.
	edtest.Change(e, func(p *model.Project) { p.Spreads = p.Spreads[:model.MaxSpreads-3] })
	if ap, err = e.AutoPlan(mixedPhotos("q", 10), model.AutoOpts{Spreads: 10, MaxPer: 64}); err != nil || ap.MaxSpreads != 3 || ap.Spreads != 3 || ap.MinSpreads != 1 {
		t.Fatalf("почти полон: %+v %v", ap, err)
	}
}

// TestStepLayoutMore: на 40 фото ряд «Авто» — 12 вариантов и «+»; шаг за 12-й досчитывает
// порцию (13 / 24+), и так до ManyCap; с последнего — к первому; назад с первого — на
// последний в посчитанном ряду (у первого варианта это первая порция).
func TestStepLayoutMore(t *testing.T) {
	e := edtest.Open(t, model.New(), "")
	sid := edtest.NewSpread(e, -1, mixedPhotos("p", 40)).Spreads[0].ID
	sv := func() model.SpreadView { return e.View().Spreads[0] }
	if s := sv(); s.Variant != 0 || s.Variants != layout.ManyBatch || !s.More {
		t.Fatalf("начало: %d / %d more=%v", s.Variant, s.Variants, s.More)
	}
	for i := 1; i < layout.ManyCap; i++ {
		s := e.StepLayout(sid, "spread", 1).Spreads[0]
		wantN := (i/layout.ManyBatch + 1) * layout.ManyBatch
		if s.Variant != i || s.Variants != wantN || s.More != (wantN < layout.ManyCap) {
			t.Fatalf("шаг %d: %d / %d more=%v", i, s.Variant, s.Variants, s.More)
		}
	}
	if s := e.StepLayout(sid, "spread", 1).Spreads[0]; s.Variant != 0 || s.Variants != layout.ManyBatch || !s.More || edtest.P(e).Spreads[0].Layout != "" {
		t.Fatalf("с последнего: %d / %d", s.Variant, s.Variants)
	}
	if s := e.StepLayout(sid, "spread", -1).Spreads[0]; s.Variant != layout.ManyBatch-1 || s.Variants != layout.ManyBatch {
		t.Fatalf("назад с первого: %d / %d", s.Variant, s.Variants)
	}
	// Листание — одна серия отмены.
	if v := e.Undo(); len(v.Spreads) != 1 || v.Spreads[0].Variant != 0 {
		t.Fatalf("отмена листания: %+v", v.Spreads)
	}
}

// TestMoreLayouts: галерея на 40 фото — первая порция и More; «Ещё» добавляет по 12 до 48,
// ряд растёт префиксом. До 16 фото — прежний ряд 24 без More.
func TestMoreLayouts(t *testing.T) {
	e := edtest.Open(t, model.New(), "")
	sid := edtest.NewSpread(e, -1, mixedPhotos("p", 40)).Spreads[0].ID
	ch := e.LayoutChoices(sid)
	if len(ch.Whole.InOrder) != layout.ManyBatch || !ch.Whole.More || len(ch.Whole.Rearranged) != 0 || ch.Whole.Block != "many" {
		t.Fatalf("первая порция: %d more=%v block=%q", len(ch.Whole.InOrder), ch.Whole.More, ch.Whole.Block)
	}
	prev := ch.Whole.InOrder
	for have, want := range map[int]int{12: 24, 23: 24, 24: 36, 47: 48, 100: 48} {
		m := e.MoreLayouts(sid, have)
		if len(m.Whole.InOrder) != want || m.Whole.More != (want < layout.ManyCap) || m.Rev != ch.Rev {
			t.Fatalf("have %d: %d more=%v", have, len(m.Whole.InOrder), m.Whole.More)
		}
		for i, c := range prev {
			if m.Whole.InOrder[i].Key != c.Key {
				t.Fatalf("have %d: ряд не префикс на %d", have, i)
			}
		}
	}
	// Текущая раскладка из 3-й порции: галерея показывает ряд до неё.
	key := e.MoreLayouts(sid, 30).Whole.InOrder[30].Key
	e.SetLayout(sid, key)
	if ch = e.LayoutChoices(sid); len(ch.Whole.InOrder) != 36 || !ch.Whole.More {
		t.Fatalf("ряд до текущей: %d more=%v", len(ch.Whole.InOrder), ch.Whole.More)
	}

	e2 := edtest.Open(t, model.New(), "")
	s16 := edtest.NewSpread(e2, -1, mixedPhotos("p", 16)).Spreads[0].ID
	a, b := e2.LayoutChoices(s16), e2.MoreLayouts(s16, 24)
	if len(a.Whole.InOrder) != model.VariantLimit || a.Whole.More || b.Whole.More || len(b.Whole.InOrder) != len(a.Whole.InOrder) {
		t.Fatalf("16 фото: %d/%d more=%v/%v", len(a.Whole.InOrder), len(b.Whole.InOrder), a.Whole.More, b.Whole.More)
	}
	// Счётчик — ряд «Авто» и «Другие варианты» за ним (решение 0.11 №36); у 17+ их нет.
	if s := e2.View().Spreads[0]; s.More || s.Variants != model.VariantLimit+len(a.Whole.Extra) || len(a.Whole.Extra) == 0 {
		t.Fatalf("16 фото: счётчик %d more=%v, других %d", s.Variants, s.More, len(a.Whole.Extra))
	}
	if len(ch.Whole.Extra) != 0 {
		t.Fatalf("40 фото: «Других вариантов» %d", len(ch.Whole.Extra))
	}
}

// TestStepLayoutEdges: листание и галерея там, где ряда нет или текущего ключа в нём нет —
// неизвестный и пустой разворот, страничный ключ при листании «spread», целый ключ при
// листании страницы: проект не меняется, лишних паник нет.
func TestStepLayoutEdges(t *testing.T) {
	e := edtest.Open(t, model.New(), "")
	empty := edtest.NewSpread(e, -1, nil).Spreads[0].ID
	rev := e.View().Rev
	for _, id := range []string{"нет-такого", empty} {
		if v := e.StepLayout(id, "spread", 1); v.Rev != rev {
			t.Fatalf("%q: листание изменило проект", id)
		}
		c := e.LayoutChoices(id)
		if c.Spread != id || c.Whole.InOrder == nil || c.Whole.Rearranged == nil || len(c.Whole.InOrder) != 0 || c.Left != nil || c.Right != nil {
			t.Fatalf("%q: галерея: %+v", id, c)
		}
	}

	ph := []model.PhotoInfo{
		{ID: "a", Path: edtest.Abs("a.jpg"), W: 200, H: 300}, edtest.Photo("b"),
		{ID: "c", Path: edtest.Abs("c.jpg"), W: 200, H: 300}, edtest.Photo("d"),
	}
	id := edtest.NewSpread(e, -1, ph).Spreads[1].ID
	whole := e.LayoutChoices(id).Whole.InOrder
	// Целого ключа нет в ряду страницы: вперёд — первый вариант страницы, назад — последний.
	if v := e.StepLayout(id, "left", 1); !strings.HasPrefix(v.Spreads[1].Layout, "P[") {
		t.Fatalf("страница вперёд: %q", v.Spreads[1].Layout)
	}
	e.SetLayout(id, "")
	if v := e.StepLayout(id, "right", -1); !strings.HasPrefix(v.Spreads[1].Layout, "P[") {
		t.Fatalf("страница назад: %q", v.Spreads[1].Layout)
	}
	e.SetLayout(id, "")
	// Ключ глубже ряда «Авто» (дальше VariantLimit) ни в одном ряду не стоит: вперёд — первый
	// вариант ряда, назад — последний; ряд листания — «Авто» и «Другие варианты» (№36).
	whole = slices.Concat(whole, e.LayoutChoices(id).Whole.Extra)
	p := edtest.P(e)
	deep := layout.Cached(p.Aspects(p.Spreads[1].Photos), p.Params(), 4*model.VariantLimit)
	i := slices.IndexFunc(deep, func(r layout.Result) bool {
		return !slices.ContainsFunc(whole, func(c model.Choice) bool { return c.Key == r.Key }) && !layout.IsRearranged(r.Key)
	})
	if i < 0 {
		t.Fatalf("у 4 фото нет вариантов в порядке ленты глубже ряда из %d", len(whole))
	}
	deepKey := deep[i].Key
	e.SetLayout(id, deepKey)
	if v := e.StepLayout(id, "spread", 1); v.Spreads[1].Layout != whole[0].Key {
		t.Fatalf("вперёд: %q, ждали %q", v.Spreads[1].Layout, whole[0].Key)
	}
	e.SetLayout(id, deepKey)
	if v := e.StepLayout(id, "spread", -1); v.Spreads[1].Layout != whole[len(whole)-1].Key {
		t.Fatalf("назад: %q, ждали %q", v.Spreads[1].Layout, whole[len(whole)-1].Key)
	}
}
