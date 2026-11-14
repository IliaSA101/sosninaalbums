package spreads_test

// Спека 078 (сц. 2–5): похожие кадры через Editor — план, список, предпросмотр и сборка видят
// один отбор; отброшенные не трогаются; «Вернуть», флажок «беру» и звёзды их удерживают.

import (
	"bytes"
	"fmt"
	"math/bits"
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/library/imgsig"
)

// sceneSizes — кадров в сцене по кругу: одиночные, пары и тройки дублей.
var sceneSizes = []int{1, 2, 3, 1, 2}

// mix — splitmix64: «случайные» хеши сцен без генератора.
func mix(x uint64) uint64 {
	x += 0x9e3779b97f4a7c15
	x = (x ^ (x >> 30)) * 0xbf58476d1ce4e5b9
	x = (x ^ (x >> 27)) * 0x94d049bb133111eb
	return x ^ (x >> 31)
}

// scenes — n фото сценами по sceneSizes: кадры одной сцены — дубли (кадр f отличается от первого
// на 3f бит), разные сцены далеки (~64 бита). Первый кадр сцены резче остальных; ориентация
// чередуется по сценам; каждая четвёртая сцена — исходно ч/б (Color 2), остальные цветные.
// first — номер первого кадра каждой сцены (он остаётся при любом уровне).
func scenes(prefix string, n int) (ph []model.PhotoInfo, first []int) {
	for sc := 0; len(ph) < n; sc++ {
		d, p := mix(uint64(sc)*2+1), mix(uint64(sc)*2+2)
		color := float32(20 + sc%30)
		if sc%4 == 0 {
			color = 2
		}
		for f := 0; f < sceneSizes[sc%len(sceneSizes)] && len(ph) < n; f++ {
			id := fmt.Sprintf("%s%d", prefix, len(ph))
			w, h := 300, 200
			if sc%2 == 1 {
				w, h = 200, 300
			}
			if f == 0 {
				first = append(first, len(ph))
			}
			sig := imgsig.Sig{D: d ^ (1<<(3*f) - 1), P: p, Color: color, Sharp: float32(100 - f), Ok: true}
			ph = append(ph, model.PhotoInfo{ID: id, Path: edtest.Abs(id + ".jpg"), W: w, H: h, Sig: sig})
		}
	}
	return ph, first
}

// placedIDs — фото всех разворотов по порядку альбома.
func placedIDs(p *model.Project) []string {
	var out []string
	for _, s := range p.Spreads {
		out = append(out, s.Photos...)
	}
	return out
}

// TestAutoSimilarPlanMatchesBuild: на всех уровнях AutoSimilar, AutoPlan, «Что соберётся» и
// сборка видят один отбор: Similar = число отброшенных, Photos — без них, собраны ровно
// оставленные по порядку ленты (группы — как в предпросмотре), отброшенных в проекте нет.
func TestAutoSimilarPlanMatchesBuild(t *testing.T) {
	ph, first := scenes("s", 60)
	for lv := 1; lv <= model.MaxAutoLevel; lv++ {
		for _, o := range []model.AutoOpts{{Spreads: 6, Auto: true, Similar: lv, Variety: model.VarietyNormal},
			{Spreads: 5, MinPer: 2, MaxPer: 9, Similar: lv, Gray: 2, Variant: 3}} {
			name := fmt.Sprintf("lv%d/%+v", lv, o)
			e := edtest.Open(t, model.New(), "")
			drops, err := e.AutoSimilar(ph, o)
			if err != nil || len(drops) != len(ph)-len(first) {
				t.Fatalf("%s: отброшено %d, ждали %d (%v)", name, len(drops), len(ph)-len(first), err)
			}
			gone := map[string]bool{}
			for _, d := range drops {
				gone[d.ID] = true
			}
			var kept []string
			for _, i := range first {
				kept = append(kept, ph[i].ID)
			}
			ap, err := e.AutoPlan(ph, o)
			if err != nil || ap.Similar != len(drops) || ap.Photos != len(kept) {
				t.Fatalf("%s: план %+v (%v)", name, ap, err)
			}
			pv, err := e.AutoPreview(ph, -1, o)
			if err != nil || len(pv.Spreads) != ap.Spreads {
				t.Fatalf("%s: предпросмотр %d разворотов, план %d (%v)", name, len(pv.Spreads), ap.Spreads, err)
			}
			build(t, e, ph, -1, o)
			p := edtest.P(e)
			if got := placedIDs(p); !slices.Equal(got, kept) {
				t.Fatalf("%s: собраны %v, ждали %v", name, got, kept)
			}
			for i, s := range p.Spreads {
				if len(s.Photos) != pv.Spreads[i].Photos {
					t.Errorf("%s: разворот %d — %d фото, в предпросмотре %d", name, i, len(s.Photos), pv.Spreads[i].Photos)
				}
			}
			for id := range gone {
				if _, ok := p.Photos[id]; ok {
					t.Errorf("%s: отброшенный %s попал в проект", name, id)
				}
			}
		}
	}
}

// TestAutoSimilarDroppedUntouched: отброшенный кадр, уже стоящий в альбоме, остаётся на своём
// развороте (сборка его не снимает); свободный отброшенный в проект не попадает; уровень 0 —
// сборка как без отбора, побитово.
func TestAutoSimilarDroppedUntouched(t *testing.T) {
	ph, _ := scenes("u", 30)
	e := edtest.Open(t, model.New(), "")
	edtest.NewSpread(e, -1, ph[1:3]) // сцена 1: оба кадра в альбоме, резче — ph[1]
	old := edtest.P(e).Spreads[0].ID
	build(t, e, ph, -1, model.AutoOpts{Spreads: 3, Auto: true, Similar: 1})
	p := edtest.P(e)
	_, s := p.Find(old)
	if s == nil || !slices.Equal(s.Photos, []string{ph[2].ID}) {
		t.Fatalf("разворот с отброшенным: %+v", s)
	}
	if _, ok := p.Photos[ph[4].ID]; ok { // сцена 2: ph[4] — второй кадр, свободный
		t.Fatal("свободный отброшенный попал в проект")
	}
	for _, sp := range p.Spreads[1:] {
		if slices.Contains(sp.Photos, ph[2].ID) || !slices.Contains(placedIDs(p), ph[1].ID) {
			t.Fatalf("собранный разворот %v: отброшенный взят или оставленный не взят", sp.Photos)
		}
	}

	a, b := edtest.Open(t, model.New(), ""), edtest.Open(t, model.New(), "")
	build(t, a, ph, -1, model.AutoOpts{Spreads: 3, Auto: true})
	build(t, b, ph, -1, model.AutoOpts{Spreads: 3, Auto: true, Keep: []string{ph[0].ID}})
	if !bytes.Equal(canon(edtest.P(a)), canon(edtest.P(b))) {
		t.Fatal("Similar 0 с Keep — сборка не как без отбора")
	}
}

// TestAutoSimilarList: список отброшенных по порядку ленты, Like — первый кадр сцены, Dist — по
// подписям; «Вернуть» (Keep), флажок «беру» (своя отметка) и 4★ (XMP) удерживают кадр; уровень 0
// — пустой список, не nil.
func TestAutoSimilarList(t *testing.T) {
	ph, first := scenes("l", 24)
	ph[4].Xmp.Rating = 4 // сцена 2, второй кадр
	e := edtest.Open(t, model.New(), "")
	if _, err := e.SetMarks(ph[5:6], model.MarkPatch{Flag: edtest.Ip(1)}); err != nil { // третий кадр сцены 2
		t.Fatal(err)
	}
	o := model.AutoOpts{Spreads: 3, Auto: true, Similar: 1, Keep: []string{ph[2].ID}} // второй кадр сцены 1
	drops, err := e.AutoSimilar(ph, o)
	if err != nil {
		t.Fatal(err)
	}
	// Флажок и звёзды идут первыми, и резкий первый кадр их сцены уходит сам: сцена 2 — ph[3] ближе
	// к ph[4] (3 бита), чем к ph[5] (6). Возвращённый образцом не становится (078-19): в сцене 1
	// ph[1] остаётся, ph[2] вычеркнут из списка.
	like := map[int]int{3: 4}
	keep := map[int]bool{2: true, 4: true, 5: true}
	for k, i := range first {
		end := len(ph)
		if k+1 < len(first) {
			end = first[k+1]
		}
		for j := i + 1; j < end; j++ {
			if _, ok := like[j]; !ok && !keep[j] && i > 5 {
				like[j] = i
			}
		}
	}
	var want []model.SimilarDrop
	for i := range ph {
		if l, ok := like[i]; ok {
			want = append(want, model.SimilarDrop{ID: ph[i].ID, Like: ph[l].ID, Dist: imgsig.Dist(ph[i].Sig, ph[l].Sig)})
		}
	}
	if !slices.Equal(drops, want) {
		t.Fatalf("отброшенные %v, ждали %v", drops, want)
	}
	if want[0].Dist != bits.OnesCount64(1<<3-1) {
		t.Fatalf("Dist второго кадра %d", want[0].Dist)
	}
	none, err := e.AutoSimilar(ph, model.AutoOpts{Spreads: 3, Auto: true})
	if err != nil || none == nil || len(none) != 0 {
		t.Fatalf("уровень 0: %v %v", none, err)
	}
	ap, _ := e.AutoPlan(ph, o)
	if ap.Similar != len(want) {
		t.Fatalf("AutoPlan.Similar %d, ждали %d", ap.Similar, len(want))
	}
}

// TestAutoSimilarKeepAdds: «Вернуть» через Editor монотонно (078-19) — вернуть N кадров даёт ровно
// +N фото плана и прежний список без них; «Вернуть все» — 0 отброшенных за один раз, сборка
// берёт все фото. Все уровни, несколько сцен сразу.
func TestAutoSimilarKeepAdds(t *testing.T) {
	ph, _ := scenes("k", 60)
	e := edtest.Open(t, model.New(), "")
	for lv := 1; lv <= model.MaxAutoLevel; lv++ {
		o := model.AutoOpts{Spreads: 6, Auto: true, Similar: lv}
		base, err := e.AutoSimilar(ph, o)
		if err != nil || len(base) < 4 {
			t.Fatalf("lv%d: отброшено %d (%v)", lv, len(base), err)
		}
		plan0, _ := e.AutoPlan(ph, o)
		for _, back := range [][]model.SimilarDrop{base[1:2], base[:3], base} {
			o.Keep = nil
			for _, d := range back {
				o.Keep = append(o.Keep, d.ID)
			}
			got, err := e.AutoSimilar(ph, o)
			want := slices.DeleteFunc(slices.Clone(base), func(d model.SimilarDrop) bool { return slices.Contains(o.Keep, d.ID) })
			if err != nil || !slices.Equal(got, want) {
				t.Fatalf("lv%d: вернули %d — отброшены %v, ждали %v (%v)", lv, len(back), got, want, err)
			}
			ap, _ := e.AutoPlan(ph, o)
			if ap.Photos != plan0.Photos+len(back) || ap.Similar != len(want) {
				t.Fatalf("lv%d: вернули %d — фото %d → %d, отброшено %d", lv, len(back), plan0.Photos, ap.Photos, ap.Similar)
			}
		}
		b := edtest.Open(t, model.New(), "")
		build(t, b, ph, -1, o) // o.Keep — все отброшенные
		if got := len(placedIDs(edtest.P(b))); got != len(ph) {
			t.Fatalf("lv%d: вернули все — собрано %d из %d", lv, got, len(ph))
		}
	}
}

// canon — JSON проекта с ID разворотов по порядку (S0, S1…): развороты сборки получают новые ID,
// а сравниваются раскладки и фото.
func canon(p *model.Project) []byte {
	data := model.Marshal(p)
	for i, s := range p.Spreads {
		data = bytes.ReplaceAll(data, []byte(s.ID), fmt.Appendf(nil, "S%03d", i))
	}
	return data
}

// fresh — новый редактор с копией проекта e: сборка «с нуля» на той же базе.
func fresh(t *testing.T, e *album.Editor) *album.Editor {
	t.Helper()
	return edtest.Open(t, edtest.P(e).Clone(), "")
}
