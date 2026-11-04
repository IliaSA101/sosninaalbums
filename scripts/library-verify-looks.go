//go:build ignore

// Проверка макетов альбома (вид album, спека 131, design §7, приёмка C1) для library-verify: третий
// файл того же package main. Без него (сборка для 0.26/0.27, где вида album нет) lookCheck пуст и
// макеты не проверяются. Для каждого макета набора:
//   - missing 0 (ключей библиотеки нет), LowDPI 0, места внутри области — по шагам и сторонам
//     обложки на страницах 20×30, 30×30 и 40×30 (места — Store.Look + model.LookTpl, как их
//     поставит альбом); у шага-страницы место не уходит через сгиб (область — страница);
//   - пропорция: меньше 80 % мест шага, где кадр 3:2 / 2:3 режется ≤ 15 % (на эталоне Ratio шага) —
//     WARN (решение 0.27-131-24), не провал; места-фигуры, шаги-виньетки и места навылет (сторона на
//     краю листа: пропорция у них любая) не считаются;
//   - набор с макетами — id `<имя>-layouts` и minApp не ниже 0.28.0 (правило выпуска, 0.28-131-24);
//   - шаг-виньетка (164): без Texts и Drafts (надпись встала бы на обе страницы) в любом наборе; со
//     style или htext — minApp не ниже 0.36.0 (0.28–0.35 поля пропускают и ставят лист без оформления);
//   - подгонка по страницам (165): шаг с Fit — только в наборе с minApp не ниже 0.37.0 (старые версии
//     поле пропускают и ставят шаг на разворот одним масштабом); в наборе, где есть Fit, каждый предмет
//     двухстраничного шага (по снимку Editor) целиком в одной странице, центр на сгибе ±0,5 мм допустим
//     только у шага с Fit = "" (наборы без Fit по этому правилу не проверяются).
package main

import (
	"fmt"
	"image"
	"math"
	"path/filepath"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/decor"
)

func init() { lookCheck, lookRows = checkLooks, lookSheetRows }

const (
	lookKind   = "album"
	lookSuffix = "-layouts"
	lookMinApp = "0.28.0"
	// lookVigMinApp — первая версия, понимающая style и htext шага-виньетки (164).
	lookVigMinApp = "0.36.0"
	// lookFitMinApp — первая версия, понимающая SpreadTpl.Fit (165).
	lookFitMinApp = "0.37.0"
	// spineTol — допуск предмета у сгиба, мм (design 165 §5); spineCenter — полоса «центр на сгибе», мм.
	spineTol    = 0.011
	spineCenter = 0.5
)

// checkLooks — все макеты набора; каждый независим, паника одного не прерывает остальных.
func checkLooks(v *verifier, path string, info decor.PackInfo) {
	if v.albums == 0 {
		return
	}
	if !strings.HasSuffix(info.ID, lookSuffix) {
		v.failf("макеты альбома в наборе %s: id должен кончаться на %s", info.ID, lookSuffix)
	}
	if verNum(v.minApp) < verNum(lookMinApp) {
		v.failf("макеты альбома: minApp %s ниже %s", v.minApp, lookMinApp)
	}
	tiles := v.tiles(lookKind, info)
	if len(tiles) != v.albums {
		v.failf("плиток макетов %d, в наборе %d", len(tiles), v.albums)
	}
	v.setFit = false
	for _, it := range tiles {
		if t, err := v.s.Look(it.ID); err == nil {
			for _, st := range t.Steps {
				v.setFit = v.setFit || st.Tpl.Fit != ""
			}
		}
	}
	for _, it := range tiles {
		v.ctx = filepath.Base(path) + " · " + it.Name
		v.guard("макет", func() { v.checkLook(it) })
	}
}

// verNum — x.y.z -> число для сравнения версий.
func verNum(s string) int {
	var a, b, c int
	fmt.Sscanf(s, "%d.%d.%d", &a, &b, &c)
	return (a*1000+b)*1000 + c
}

// checkLook — один макет: шаги и стороны обложки независимо друг от друга.
func (v *verifier) checkLook(it decor.Item) {
	t, err := v.s.Look(it.ID)
	if err != nil {
		v.failf("макет не читается: %v", err)
		return
	}
	cover := 0
	if t.Cover != nil {
		for _, side := range []*model.SpreadTpl{t.Cover.Back, t.Cover.Front} {
			if side != nil {
				cover++
			}
		}
	}
	fmt.Fprintf(v.w, "  %s: шагов %d, сторон обложки %d, фон обложки %v\n", it.Name, len(t.Steps), cover, t.Cover != nil && t.Cover.Bg != nil)
	if t.Cover != nil {
		if t.Cover.Bg != nil {
			v.lookMissing("обложка, фон", model.SpreadTpl{Page: []model.TplBg{*t.Cover.Bg}})
		}
		for i, c := range []struct {
			name string
			side *model.SpreadTpl
		}{{"обложка, оборот", t.Cover.Back}, {"обложка, лицо", t.Cover.Front}} {
			if c.side != nil {
				// Сторона — как её строит программа (SideTpl): фон по CoverPageBg, картинка стороны при
				// узоре или фото фона листа не ставится и не проверяется. Места ставит lookTpl по размерам.
				side := t.Cover.SideTpl(i, 0)
				v.guard(c.name, func() { v.lookTpl(slug(it.Name), c.name, side, false) })
			}
		}
	}
	for i, st := range t.Steps {
		label := fmt.Sprintf("шаг %d", i+1)
		v.guard(label, func() {
			tpl := st.Tpl
			v.checkLookFit(label, tpl)
			if st.Role == model.RoleVignette {
				v.checkLookVig(label, st)
				// Места виньетки строит лист 031; от шага — только оформление (фон, стикеры).
				if tpl.Pages != 1 && tpl.Pages != 2 {
					tpl.Pages = 2
				}
				if len(tpl.Stickers) == 0 && len(tplKeys(tpl)) == 0 {
					return
				}
				v.lookTpl(slug(it.Name), label, tpl, false) // оформление на обе страницы: корешка нет
				return
			}
			v.lookTpl(slug(it.Name), label, tpl, true)
			v.lookAspect(label, tpl)
		})
	}
}

// checkLookVig — правила шага-виньетки (164): надписей нет (шаг ставится на обе страницы листа, vigPage
// зеркалит, надпись встала бы дважды — шапка и подписи идут через htext и style); style и htext —
// только в наборе с minApp не ниже lookVigMinApp. Оба провала независимы.
func (v *verifier) checkLookVig(label string, st model.LookStep) {
	if n := len(st.Tpl.Texts) + len(st.Tpl.Drafts); n > 0 {
		v.failf("%s: у шага-виньетки надписей %d (texts %d, drafts %d): они встанут на обе страницы, шапку и подписи задают htext и style",
			label, n, len(st.Tpl.Texts), len(st.Tpl.Drafts))
	}
	if st.Vig != nil && (st.Vig.Style != nil || st.Vig.HText != "") && verNum(v.minApp) < verNum(lookVigMinApp) {
		v.failf("%s: шаг-виньетка со style или htext, а minApp %s ниже %s", label, v.minApp, lookVigMinApp)
	}
}

// checkLookFit — Fit шага: только в наборе с minApp не ниже lookFitMinApp (165).
func (v *verifier) checkLookFit(label string, t model.SpreadTpl) {
	if t.Fit != "" && verNum(v.minApp) < verNum(lookFitMinApp) {
		v.failf("%s: у шага fit %q, а minApp %s ниже %s", label, t.Fit, v.minApp, lookFitMinApp)
	}
}

// lookSpine — правило корешка по снимку Editor (165 §5): у двухстраничного шага набора с Fit каждый
// предмет целиком в одной странице (рамка с поворотом, допуск spineTol); центр на сгибе ±spineCenter
// допустим только при Fit = "" (симметричное украшение через корешок, как в 0.36). Возвращает, был ли провал.
func (v *verifier) lookSpine(name string, fit string, sv model.SpreadView) bool {
	before := len(v.fails)
	for i, st := range sv.Stickers {
		c, s := math.Abs(math.Cos(st.Rot*math.Pi/180)), math.Abs(math.Sin(st.Rot*math.Pi/180))
		bw := st.W*c + st.H*s
		cx := st.X + st.W/2
		l, r := cx-bw/2, cx+bw/2
		if r <= sv.Mid+spineTol || l >= sv.Mid-spineTol {
			continue // целиком в одной странице
		}
		if fit == "" && math.Abs(cx-sv.Mid) <= spineCenter {
			continue
		}
		v.failf("%s: стикер %d (центр %.1f мм, ширина %.1f мм) лежит через корешок (%.1f мм), fit %q", name, i+1, cx, bw, sv.Mid, fit)
	}
	return len(v.fails) > before
}

// lookMissing — ключи библиотеки шаблона, которых нет на этом ПК; провал один на шаблон.
func (v *verifier) lookMissing(label string, t model.SpreadTpl) (map[string]model.PhotoInfo, bool) {
	libs, missing := v.libsOf(t)
	if missing > 0 {
		v.failf("%s: missing %d (ключей библиотеки нет)", label, missing)
	}
	return libs, missing == 0
}

// lookTpl — шаблон шага или стороны на трёх страницах: места через LookTpl, как их ставит альбом;
// look — имя макета для файла снимка при провале; spine — шаг-разворот (правило корешка, если в наборе есть Fit).
func (v *verifier) lookTpl(look, label string, t model.SpreadTpl, spine bool) {
	libs, _ := v.lookMissing(label, t)
	bgs := 0 // фонов, чьи файлы есть: на листе должны лечь
	for _, b := range append(append([]model.TplBg{}, t.Page...), derefBg(t.Whole)) {
		if _, ok := libs[b.Lib]; ok && b.Lib != "" {
			bgs++
		}
	}
	pages := 1.0
	if t.Pages == 2 {
		pages = 2
	}
	for _, sz := range pageSizes {
		wmm := sz.w * pages
		tpl := model.LookTpl(t, wmm/sz.h) // шаг с Fit по страницам места не трогает, панорама сбрасывает Fit (165)
		p, view, err := album.PreviewTpl(tpl, wmm, sz.h, libs)
		if err != nil {
			v.failf("%s · %s: PreviewTpl: %v", label, sz.name, err)
			continue
		}
		name := label + " · " + sz.name
		bad := v.checkView(name, tpl, view.Spreads[0], view.MinDPI, bgs)
		if spine && v.setFit && t.Pages == 2 {
			bad = v.lookSpine(name, t.Fit, view.Spreads[0]) || bad
		}
		if bad && v.out != "" {
			v.snapshot(tpl, p, view, filepath.Join(v.out, "fail-"+look+"-"+slug(name)+".png"))
		}
	}
}

// lookAspect — предупреждение о пропорции мест шага на эталоне Ratio (после ClampSpreadTpl всегда 0,2…5;
// без ratio в файле — 1, такой макет library-pack не пропускает); места-фигуры не считаются.
func (v *verifier) lookAspect(label string, t model.SpreadTpl) {
	ratio := t.Ratio
	n, ok := 0, 0
	for _, b := range t.Places {
		if b.Shape != "" || b.W <= 0 || b.H <= 0 || bleeds(b) {
			continue
		}
		n++
		if a := b.W * ratio / b.H; min(cropOf(a, 1.5), cropOf(a, 2.0/3)) <= cropMax+1e-9 {
			ok++
		}
	}
	if n > 0 && float64(ok) < fitShare*float64(n)-1e-9 {
		v.warnf("%s: %d из %d мест не 3:2 / 2:3 (подходят %d%%, порог %d%%)", label, n-ok, n, ok*100/n, int(fitShare*100))
	}
}

// bleeds — место навылет: сторона без наклона на краю композиции (допуск model.FoldEps, как у
// прилипания в grid); кадр до края листа законно любой пропорции.
func bleeds(b model.Box) bool {
	if b.Rot != 0 {
		return false
	}
	const e = model.FoldEps
	return b.X <= e || b.Y <= e || b.X+b.W >= 1-e || b.Y+b.H >= 1-e
}

// lookSheetRows — строка листа превью на каждый макет набора: «Обложка, 1…N» (оборот и лицо обложки,
// затем шаги по одному разу в порядке серии, страница 30×30). Фон стороны — model.CoverPageBg (цвет
// свой, узор и фото листа, а при их отсутствии — свои); места — как их ставит альбом: LookTpl на области страницы или разворота 30×30 (у шага с
// Ratio ≠ числа страниц композиция вписана по центру). Шаг-виньетка — только оформление: места листа строит 031.
func lookSheetRows(v *verifier, info decor.PackInfo) []image.Image {
	var rows []image.Image
	for _, it := range v.tiles(lookKind, info) {
		t, err := v.s.Look(it.ID)
		if err != nil {
			continue // провал уже записал checkLook
		}
		var tiles []image.Image
		if c := t.Cover; c != nil {
			for j := range 2 {
				// Сторона и её места — SideTpl на области страницы 30×30 (r = 1), как строит плитка программы.
				tpl := c.SideTpl(j, 1)
				tiles = appendTile(tiles, v.tileOf(it.Name+": "+[]string{"обложка, оборот", "обложка, лицо"}[j], tpl))
			}
		}
		for i, idx := range model.Series(t, len(t.Steps)) {
			tiles = appendTile(tiles, v.tileOf(fmt.Sprintf("%s: шаг %d", it.Name, i+1), lookFit(t.Steps[idx].Tpl)))
		}
		if len(tiles) > 0 {
			rows = append(rows, flow(tiles, flowW, 8))
		}
	}
	return rows
}

// lookFit — шаблон с местами, как их ставит альбом на области 30×30 (страница или разворот).
func lookFit(t model.SpreadTpl) model.SpreadTpl {
	return model.LookTpl(t, float64(t.Pages))
}
