package model

import (
	"slices"
	"strings"
	"sync/atomic"

	"github.com/IliaSA101/sosninaalbums/internal/album/model/printprof"
)

// Таблицы типографий из профилей (123 (В)): форматы, изделия обложки, кнопки страницы, группы и
// пределы строятся из снимка printprof.Current() — встроенные JSON, позже слои. Логика (совпадение
// формата, шаблон обложки, пределы) остаётся в Go и читает эти таблицы.

// printTables — производные таблицы одного снимка профилей. Только для чтения: наружу отдаются
// копии (CoverPresets, PagePresets… — «функция, а не переменная»).
type printTables struct {
	snap    *printprof.Snapshot
	groups  []templateGroup // шаблонные группы по порядку профилей
	covers  []CoverPreset   // изделия без CoverCustom
	pages   []PagePreset    // ряды кнопок типографий (общий ряд — в PagePresets)
	pgroups []PrintGroup
	limits  []PrintLimit
	// spineBy — таблица корешка изделия для формата вместо общей: изделие → подпись формата → строки.
	spineBy map[string]map[string][]SpineStep
}

var tablesCache atomic.Pointer[printTables]

// tables — таблицы действующего снимка. Пересчёт по смене указателя снимка: Install и UseForTest
// публикуют новый, поэтому устаревшей таблицы не бывает; два одновременных пересчёта одного снимка
// дают равные таблицы. Операция, которой нужно несколько справок (CoverPresetFor, LimitOf,
// CoverFixText…), берёт t := tables() один раз: иначе между двумя вызовами снимок мог бы смениться
// и пресет, формат и предел вышли бы из разных профилей.
func tables() *printTables {
	snap := printprof.Current()
	if t := tablesCache.Load(); t != nil && t.snap == snap {
		return t
	}
	t := buildTables(snap)
	tablesCache.Store(t)
	return t
}

func buildTables(snap *printprof.Snapshot) *printTables {
	t := &printTables{snap: snap, spineBy: map[string]map[string][]SpineStep{}}
	for _, p := range snap.Profiles() {
		t.addFormats(p)
		t.addProducts(p)
		if p.Default != "" {
			t.pgroups = append(t.pgroups, PrintGroup{Name: p.Name, Gen: p.Gen, Default: p.Default, NameDefault: p.NameDefault})
		}
	}
	return t
}

func (t *printTables) addFormats(p printprof.Profile) {
	var tpl []PrintFormat
	for _, f := range p.Formats {
		safe, bleed := f.SafeIn(p), f.BleedIn(p)
		t.pages = append(t.pages, PagePreset{Label: f.Name, Group: p.Name, W: f.Page[0], H: f.Page[1], DPI: p.Page.DPI, Layflat: p.Page.Layflat,
			Safe: safe, Bleed: bleed, Hint: strings.ReplaceAll(p.Page.Hint, "{safe}", MmText(safe))})
		if f.Template() {
			tpl = append(tpl, PrintFormat{ID: p.ID + "-" + strings.ReplaceAll(f.Name, "×", "x"), Name: f.Name, PageW: f.Page[0], PageH: f.Page[1],
				Safe: safe, Bleed: bleed, SideW: f.Side[0], SideH: f.Side[1], Wrap: f.Wrap, Px: pxOf(f.Px), Guide: guideOf(p, f)})
		}
	}
	if len(tpl) > 0 {
		t.groups = append(t.groups, templateGroup{Group: p.Name, Gen: p.Gen, list: tpl, Formats: func() []PrintFormat { return slices.Clone(tpl) }})
	}
}

func (t *printTables) addProducts(p printprof.Profile) {
	for _, pr := range p.Products {
		spec := CoverSpec{Preset: pr.ID, Wrap: pr.Wrap, Hinge: pr.Hinge, A: pr.A, B: pr.B, Steps: stepsOf(pr.Spine)}
		if c := p.Cover; c != nil {
			if spec.Wrap == 0 {
				spec.Wrap = c.Wrap
			}
			spec.Safe, spec.SpineSafe = c.Safe, c.SpineSafe
		}
		t.covers = append(t.covers, CoverPreset{ID: pr.ID, Name: pr.Name, Group: p.Name, Spec: spec})
		for format, flat := range pr.SpineBy {
			if t.spineBy[pr.ID] == nil {
				t.spineBy[pr.ID] = map[string][]SpineStep{}
			}
			t.spineBy[pr.ID][format] = stepsOf(flat)
		}
		for _, l := range pr.Limits {
			top := l.Max
			if top == 0 { // верх таблицы корешка этого формата — один источник с корешком
				if st := stepsOf(pr.SpineFor(l.Format)); len(st) > 0 {
					top = st[len(st)-1].Upto
				}
			}
			t.limits = append(t.limits, PrintLimit{Product: pr.ID, Format: l.Format, Min: l.Min, Max: top, Pages: l.Pages})
		}
	}
}

// guideOf — направляющие формата f профиля p; профиль без guide — нулевые (прежние линии).
func guideOf(p printprof.Profile, f printprof.PageFormat) PrintGuide {
	var g PrintGuide
	if f.Guide == nil {
		return g
	}
	if b := f.Guide.Bleed; b != nil {
		g.BleedX, g.BleedY = b[0], b[1]
	}
	if d := f.Guide.Fold; d != nil {
		g.FoldX, g.FoldY = d[0], d[1]
		if c := p.Cover; c != nil {
			g.FoldSafe, g.SpineSafe = c.FoldSafe, c.GuideSpineSafe
		}
	}
	return g
}

// pxOf — цель px формата профиля; nil — нет.
func pxOf(p *printprof.PxPair) [2]int {
	if p == nil {
		return [2]int{}
	}
	return [2]int{p[0], p[1]}
}

// stepsOf — плоская таблица профиля («до N, мм») в строки корешка; пусто — nil.
func stepsOf(flat []float64) []SpineStep {
	if len(flat) == 0 {
		return nil
	}
	out := make([]SpineStep, 0, len(flat)/2)
	for _, s := range printprof.Steps(flat) {
		out = append(out, SpineStep{Upto: s.Upto, Mm: s.Mm})
	}
	return out
}

// cloneCover — пресет с собственной таблицей корешка: вызывающий вправе её менять.
func cloneCover(c CoverPreset) CoverPreset {
	c.Spec.Steps = slices.Clone(c.Spec.Steps)
	return c
}

// customPreset — «Свои размеры»: пресет вне типографий, всегда последний.
func customPreset() CoverPreset {
	return CoverPreset{ID: CoverCustom, Name: "Свои размеры", Spec: CoverSpec{Preset: CoverCustom, Wrap: 15, A: 10}}
}

// stepsFor — таблица корешка изделия id для формата page: своя таблица формата (Фабрика 30×20,
// прослойка 2 мм) или base.
func (t *printTables) stepsFor(id, page string, base []SpineStep) []SpineStep {
	if st, ok := t.spineBy[id][page]; ok {
		return slices.Clone(st)
	}
	return base
}

// templateGroups — шаблонные группы снимка (форматы с сторонкой); остальные пресеты сторонку
// задают сами. Копия списка для обходов и тестов.
func templateGroups() []templateGroup { return slices.Clone(tables().groups) }
