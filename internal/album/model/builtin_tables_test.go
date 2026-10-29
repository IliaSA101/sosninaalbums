package model

import (
	"cmp"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"
)

// Эталон встроенных таблиц типографий (0.26-123 (В)): снят с кода до переноса таблиц в JSON
// (internal/album/model/printprof/builtin) и после переноса проходит без -update. Файл —
// строки «секция {json}»: построчный diff читается, а числа таблиц видны глазами.
const builtinTablesPath = "testdata/builtin-tables.txt"

// TestBuiltinTablesSnapshot сверяет всё, что model выводит из таблиц Фотосферы, Нетпринта и Фабрики:
// форматы, изделия обложки, кнопки страницы, пределы и группы; шаблон обложки и совпадение формата
// на каждой странице-пробе; ответы LimitFor по всем изделиям и подписям кнопок. Меняется таблица —
// сначала новая версия JSON и страж TestBuiltinVersionHash, потом -update с объяснением.
func TestBuiltinTablesSnapshot(t *testing.T) {
	got := builtinTablesLines(t)
	if *update {
		if err := os.WriteFile(builtinTablesPath, []byte(strings.Join(got, "\n")+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("эталон перезаписан: %s (%d строк)", builtinTablesPath, len(got))
		return
	}
	data, err := os.ReadFile(builtinTablesPath)
	if err != nil {
		t.Fatalf("нет эталона (%v): go test ./internal/album/model -run BuiltinTablesSnapshot -update", err)
	}
	want := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	if slices.Equal(want, got) {
		return
	}
	var diff []string
	for i := 0; i < max(len(want), len(got)) && len(diff) < 12; i++ {
		w, g := "", ""
		if i < len(want) {
			w = want[i]
		}
		if i < len(got) {
			g = got[i]
		}
		if w != g {
			diff = append(diff, fmt.Sprintf("строка %d:\n  эталон: %s\n  сейчас: %s", i+1, w, g))
		}
	}
	t.Fatalf("таблицы разошлись с эталоном (строк %d и %d):\n%s", len(want), len(got), strings.Join(diff, "\n"))
}

// probePages — страницы-пробы: размер каждой кнопки страницы (с её зоной, обрезом, dpi), те же
// размеры в стиле «безопасная зона 10» (как у альбомов без кнопки), чужие размеры, допуск PageTol и лист 031.
func probePages() []struct {
	Name string
	S    Settings
} {
	type page = struct {
		Name string
		S    Settings
	}
	var out []page
	seen := map[string]bool{}
	add := func(name string, s Settings) {
		k := fmt.Sprintf("%v", s)
		if !seen[k] {
			seen[k] = true
			out = append(out, page{name, s})
		}
	}
	for _, pr := range PagePresets() {
		add(pr.Group+"/"+pr.Label, Settings{PageW: pr.W, PageH: pr.H, DPI: cmp.Or(pr.DPI, 300), Layflat: pr.Layflat, Safe: pr.Safe, Bleed: pr.Bleed})
		add(pr.Group+"/"+pr.Label+"/safe10", Settings{PageW: pr.W, PageH: pr.H, DPI: 300, Layflat: true, Safe: 10})
	}
	for _, w := range [][2]float64{{300, 200}, {303.95, 202.95}, {200.1, 203}, {201, 203.2}, {120, 90}, {400, 400}, {304.3, 203}, {250, 250}} {
		add(fmt.Sprintf("чужая/%vx%v", w[0], w[1]), Settings{PageW: w[0], PageH: w[1], DPI: 300, Layflat: true, Safe: 10})
	}
	add("лист 031", Settings{PageW: 304, PageH: 203, DPI: 300, Layflat: true, Safe: 5, Sheet: true})
	return out
}

func builtinTablesLines(t *testing.T) []string {
	t.Helper()
	var out []string
	put := func(section string, v any) {
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, section+" "+string(b))
	}
	for _, f := range FsferaFormats() {
		put("fsferaFormat", f)
	}
	for _, f := range FabrikaFormats() {
		put("fabrikaFormat", f)
	}
	// Направляющие (123 Б5) вне JSON формата (json:"-"), поэтому закреплены отдельной секцией.
	for _, group := range []string{CoverGroupFsfera, CoverGroupFabrika} {
		for _, f := range GroupFormats(group) {
			if f.Guide == (PrintGuide{}) { // у форматов без направляющих строки нет: добавленные видны в diff
				continue
			}
			put("formatGuide", struct {
				Group, Name string
				Guide       PrintGuide
			}{group, f.Name, f.Guide})
		}
	}
	for _, c := range CoverPresets() {
		put("coverPreset", c)
	}
	for _, p := range PagePresets() {
		put("pagePreset", p)
	}
	for _, g := range PrintGroups() {
		put("printGroup", g)
	}
	// Порядок строк предела не значим (LimitFor ищет по product+format) — снимок в каноническом.
	lims := PrintLimits()
	slices.SortFunc(lims, func(a, b PrintLimit) int {
		return cmp.Or(cmp.Compare(a.Product, b.Product), cmp.Compare(a.Format, b.Format))
	})
	for _, l := range lims {
		put("printLimit", l)
	}
	groups := []string{CoverGroupFsfera, CoverGroupNetprint, CoverGroupFabrika, "", "Нет такой"}
	for _, pg := range probePages() {
		row := map[string]any{"page": pg.Name}
		for _, g := range groups {
			f, ok := FormatOf(g, pg.S)
			n := FormatNear(g, pg.S)
			pp, okp := PagePresetOf(pg.S)
			pi, oki := PagePresetIn(pg.S, g)
			row[g] = fmt.Sprintf("of=%s/%v near=%s in=%s/%v fits=%v of=%s/%v/%s", f.Name, ok, n.Name, pi.Label, oki, PageFitsGroup(pg.S, g), pp.Label, okp, pp.Group)
		}
		put("page", row)
	}
	// Шаблон обложки по странице: сторонка, заворот, корешок — на каждом изделии и странице-пробе.
	// Шаблон зависит только от размера страницы и листа 031 — зону и обрез в пробах не повторяем.
	var sizes []Settings
	for _, pg := range probePages() {
		s := Settings{PageW: pg.S.PageW, PageH: pg.S.PageH, Sheet: pg.S.Sheet}
		if !slices.Contains(sizes, s) {
			sizes = append(sizes, s)
		}
	}
	for _, c := range CoverPresets() {
		for _, s := range sizes {
			pr, ok := CoverPresetFor(c.ID, s)
			put("coverFor", []any{c.ID, s.PageW, s.PageH, s.Sheet, ok, pr})
		}
	}
	var labels []string
	for _, p := range PagePresets() {
		if !slices.Contains(labels, p.Label) {
			labels = append(labels, p.Label)
		}
	}
	labels = append(labels, "", "0×0")
	for _, c := range CoverPresets() {
		var b strings.Builder
		b.WriteString(c.ID)
		for _, l := range labels {
			lim, ok := LimitFor(c.ID, l)
			fmt.Fprintf(&b, " [%s]=%v/%q/%d-%d/%v", l, ok, lim.Format, lim.Min, lim.Max, lim.Pages)
		}
		out = append(out, "limitFor "+b.String())
	}
	return out
}
