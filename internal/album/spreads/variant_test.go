package spreads_test

// Спека 078 (сц. 8): «Вариант ‹ k ›» — вариант 1 = 0.19, номер повторяем, варианты 2–6 другие,
// правила разнообразия не хуже варианта 1.

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/layout/variety"
)

// variantPaths — пути плана: «Авто», свой диапазон, ровно (без разброса).
var variantPaths = []model.AutoOpts{{Spreads: 25, Auto: true}, {Spreads: 22, MinPer: 4, MaxPer: 14}, {Spreads: 20, MaxPer: 12}}

// buildVariant — альбом из ph, собранный с опциями o и вариантом k, в виде canon.
func buildVariant(t *testing.T, ph []model.PhotoInfo, o model.AutoOpts, k int) *model.Project {
	t.Helper()
	o.Variant = k
	e := edtest.Open(t, model.New(), "")
	build(t, e, ph, -1, o)
	return edtest.P(e)
}

// variantDigest19 — variantDigest сборки 0.19.0 (bf0d1082: тот же расчёт временным тестом на
// дереве 0.19.0): раскладки по путям variantPaths × уровням levels на mixedPhotos("o", 120).
const variantDigest19 = "065d5f8b6866229edd302742759fb6334c21f28946c99d6dbb9ee460a84da66e"

// variantDigest — хеш фото, сохранённого и действующего ключа каждого разворота по всем путям и
// уровням: от формата файла не зависит (v14 и далее его меняют), от раскладок — да.
func variantDigest(t *testing.T, ph []model.PhotoInfo, k int) string {
	t.Helper()
	h := sha256.New()
	for _, v := range levels {
		for _, o := range variantPaths {
			o.Variety = v
			p := buildVariant(t, ph, o, k)
			for _, s := range p.Spreads {
				r, _, _ := p.Layout(s)
				fmt.Fprintf(h, "%d|%d|%s|%s|%s\n", v, o.Spreads, strings.Join(s.Photos, ","), s.Layout, r.Key)
			}
		}
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

// TestVariantOneIsOld: вариант 1 (и 0 — его зажим) собирает побитово то же, что 0.19.0, на всех
// уровнях и путях: сверка с хешем, снятым на дереве до 078.
func TestVariantOneIsOld(t *testing.T) {
	ph := mixedPhotos("o", 120)
	for _, k := range []int{0, 1} {
		if got := variantDigest(t, ph, k); got != variantDigest19 {
			t.Fatalf("вариант %d: раскладки не как в 0.19.0 (хеш %s)", k, got)
		}
	}
}

// TestVariantDeterministic: тот же номер на тех же фото и настройках — тот же альбом.
func TestVariantDeterministic(t *testing.T) {
	ph := mixedPhotos("d", 120)
	for _, v := range levels {
		for _, k := range []int{2, 7, model.MaxVariant} {
			o := variantPaths[0]
			o.Variety = v
			if a, b := buildVariant(t, ph, o, k), buildVariant(t, ph, o, k); !bytes.Equal(canon(a), canon(b)) {
				t.Fatalf("уровень %d, вариант %d: две сборки разные", v, k)
			}
		}
	}
}

// spreadKeys — фото и действующий ключ каждого разворота: «другой разворот» — другая группа или ключ.
func spreadKeys(p *model.Project) []string {
	out := make([]string, len(p.Spreads))
	for i, s := range p.Spreads {
		r, _, _ := p.Layout(s)
		out[i] = fmt.Sprint(s.Photos, r.Key)
	}
	return out
}

// TestVariantsDiffer: варианты 2–6 на 200 фото — не меньше 30 % разворотов другие (группа или
// раскладка) по сравнению с вариантом 1 на каждом уровне и пути с разбросом.
func TestVariantsDiffer(t *testing.T) {
	ph := mixedPhotos("v", 200)
	for _, v := range levels {
		for _, o := range variantPaths[:2] {
			o.Variety = v
			base := spreadKeys(buildVariant(t, ph, o, 1))
			diff, all := 0, 0
			for k := 2; k <= 6; k++ {
				got := spreadKeys(buildVariant(t, ph, o, k))
				for i := range got {
					all++
					if i >= len(base) || got[i] != base[i] {
						diff++
					}
				}
			}
			share := float64(diff) / float64(all)
			t.Logf("уровень %d %+v: отличается %.0f%%", v, o, 100*share)
			if share < 0.30 {
				t.Errorf("уровень %d %+v: варианты 2–6 отличаются у %.0f%% разворотов, ждали ≥ 30%%", v, o, 100*share)
			}
		}
	}
}

// rules — метрики разнообразия собранного альбома: доля соседей с одним видом и средняя потеря
// раскладки к лучшей своего разворота.
func rules(p *model.Project) (repeat, loss float64) {
	prm := p.Params()
	var prev string
	for i, s := range p.Spreads {
		r, best, _ := p.Layout(s)
		look := variety.Look(r.Rects, prm)
		if i > 0 && look == prev {
			repeat++
		}
		prev = look
		loss += best[0].Score - r.Score
	}
	n := float64(len(p.Spreads))
	return repeat / max(n-1, 1), loss / n
}

// TestVariantKeepsRules: у вариантов 2–6 повторов вида у соседей и средней потери не больше, чем
// у варианта 1, × 1,1 (с малым допуском на абсолют около нуля) — на «Обычном» и «Больше».
func TestVariantKeepsRules(t *testing.T) {
	ph := mixedPhotos("r", 200)
	for _, v := range []model.Variety{model.VarietyNormal, model.VarietyMore} {
		for _, o := range variantPaths[:2] {
			o.Variety = v
			var rep, loss []float64
			for k := 1; k <= 6; k++ {
				r, l := rules(buildVariant(t, ph, o, k))
				rep, loss = append(rep, r), append(loss, l)
			}
			t.Logf("уровень %d %+v: повторы %v, потеря %v", v, o, rep, loss)
			for k := 1; k < len(rep); k++ {
				if rep[k] > rep[0]*1.1+0.02 || loss[k] > loss[0]*1.1+0.002 {
					t.Errorf("уровень %d %+v, вариант %d: повторы %.3f (вариант 1: %.3f), потеря %.4f (%.4f)",
						v, o, k+1, rep[k], rep[0], loss[k], loss[0])
				}
			}
		}
	}
}

// groupsOf — фото по разворотам: границы сборки.
func groupsOf(p *model.Project) string {
	out := make([][]string, len(p.Spreads))
	for i, s := range p.Spreads {
		out[i] = s.Photos
	}
	return fmt.Sprint(out)
}

// TestVariantMovesBounds: на путях с разбросом каждый вариант 2–6 сдвигает границы разворотов
// (волна размеров), путь «ровно» — нет.
func TestVariantMovesBounds(t *testing.T) {
	ph := mixedPhotos("b", 200)
	for i, o := range variantPaths {
		base := groupsOf(buildVariant(t, ph, o, 1))
		for k := 2; k <= 6; k++ {
			if moved := groupsOf(buildVariant(t, ph, o, k)) != base; moved != (i < 2) {
				t.Errorf("%+v, вариант %d: границы сдвинуты %v", o, k, moved)
			}
		}
	}
}

// TestVariantOnLess: на «Меньше» вариант тоже меняет раскладки (из ряда «Авто» почти без потери),
// границы — у путей с разбросом.
func TestVariantOnLess(t *testing.T) {
	ph := mixedPhotos("m", 120)
	o := variantPaths[2] // ровно: границы те же, меняются только ключи
	o.Variety = model.VarietyLess
	a, b := buildVariant(t, ph, o, 1), buildVariant(t, ph, o, 3)
	ka, kb := spreadKeys(a), spreadKeys(b)
	if len(ka) != len(kb) || slices.Equal(ka, kb) {
		t.Fatalf("«Меньше», вариант 3 — та же сборка")
	}
	for i := range a.Spreads {
		if !slices.Equal(a.Spreads[i].Photos, b.Spreads[i].Photos) {
			t.Fatalf("путь «ровно»: вариант сдвинул границу разворота %d", i)
		}
	}
}
