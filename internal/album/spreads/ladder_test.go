package spreads_test

// 055 «Лесенка честно и чаще»: лесенка в автосборке на живых пропорциях stock, чередование
// направлений у «Разнообразить» и ключ лесенки альбомов 0.13, ушедший из ряда.

import (
	"fmt"
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
