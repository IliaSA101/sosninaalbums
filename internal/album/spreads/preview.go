package spreads

// Ряд блока «Раскладки» окна автосборки (065, design §3): «Варианты» групп плана и «Что
// соберётся» — та же сборка на копии проекта. Проект Editor не трогается: правится копия.

import (
	"cmp"
	"slices"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/layout/forms"
	"github.com/IliaSA101/sosninaalbums/internal/layout/variety"
)

// Preview — операция над копией проекта c (Clone, не проект Editor): сборка как у AutoBuild
// с тем же входом pre и позицией at, затем ряды. Тот же вход — тот же детерминированный Pick,
// поэтому «Что соберётся» совпадает со сборкой. Фото нет — пустые ряды без ошибки.
func Preview(c *model.Project, pre Prep, at int, rate Rating) (model.AutoPreview, error) {
	pv := model.AutoPreview{Samples: []model.SampleRow{}, Spreads: []model.PreviewSpread{}}
	b, err := build(c, pre, at, rate, nil)
	if err != nil {
		return pv, err
	}
	g := c.Geo(&model.Spread{})
	pv.W, pv.H = g.W, g.H
	prm := c.Params()
	seen := map[int]bool{}
	for i, s := range b.spreads(c) {
		r, _, _ := c.Layout(s) // у собранного разворота 1…64 фото — раскладка есть всегда
		ps := model.PreviewSpread{Choice: describeOne(r, prm), Photos: len(s.Photos), Fallback: i < len(b.fb) && b.fb[i],
			Gray: i < len(b.gray) && b.gray[i]}
		if ps.Fallback {
			pv.Fallback++
		}
		pv.Spreads = append(pv.Spreads, ps)
		if n := len(s.Photos); !seen[n] {
			seen[n] = true
			asp, sp := c.LayoutIn(s.Photos)
			pv.Samples = append(pv.Samples, sampleRow(asp, sp, i))
		}
	}
	// Размеры групп разные (seen), так что порядок однозначен.
	slices.SortFunc(pv.Samples, func(a, b model.SampleRow) int { return cmp.Compare(a.Photos, b.Photos) })
	return pv, nil
}

// sampleRow — ряды подборки разворота с лучшей раскладкой (Layout ""): ряд «Авто» — первая
// порция (на 17+ фото без «Ещё»), «Другие варианты» — как у LayoutChoices.
func sampleRow(asp []float64, prm layout.Params, group int) model.SampleRow {
	row, _ := model.MoreRow(asp, prm, 0) // AutoRow с "" на 17+ досчитал бы ряд до ManyCap
	return model.SampleRow{Photos: len(asp), Group: group, InOrder: choiceRows(row, nil, "", prm).InOrder,
		Extra: extraChoices(asp, prm)}
}

// describeOne — плитка одной раскладки с семейством и формой, как в подборке (describeRows);
// похожих в ряду «Что соберётся» нет: Twin = −1. Семья — плитки (TileFamily, 104): как в галерее
// и в фильтре окна (variety.ByFamily), «Плотно» по заполнению считается и здесь.
func describeOne(r layout.Result, prm layout.Params) model.Choice {
	f := forms.Of(r.Key, r.Rects, prm)
	return model.Choice{Key: r.Key, Rects: r.Rects, Family: string(variety.TileFamily(f, r.Rects, prm)),
		FormName: forms.Name(f), Twin: -1, Crop: cropPct(r.Crop)}
}

// FamilyOf — семейство плитки действующей раскладки разворота s (TileFamily, как в галерее и
// фильтре 065) для чтения агентом (151). Сетка (ручные места), виньетка, лист без фото и
// раскладка, которой нет, — "". Проект не меняется.
func FamilyOf(p *model.Project, s *model.Spread) string {
	if s == nil || len(s.Grid) > 0 || s.IsVignette() || len(s.Photos) == 0 {
		return ""
	}
	r, _, ok := p.Layout(s)
	if !ok {
		return ""
	}
	_, prm := p.LayoutIn(s.Photos)
	return string(variety.TileFamily(forms.Of(r.Key, r.Rects, prm), r.Rects, prm))
}
