//go:build ignore

// Темы наборов в library-verify (спека 170, design §6; тот же package main, см. library-verify.go): набор ставится
// как из каталога (OriginNet, SetPrintCheck — как App), а здесь проверяется то, что из этого получит клиент.
// Провал: объявление тем не принято (pack.ThemeSkips), topic предмета сброшен в базу (нет в themes своего манифеста),
// тема не попала в таблицу программы (имя-двойник чужого набора, встроенный id, предел) или предметы темы в каталоге
// не сошлись с манифестом. Печатается таблица «тема → бг / ст / рам / разв / макет» по предметам набора.
package main

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strings"
	"unicode"

	"github.com/IliaSA101/sosninaalbums/internal/app/ownprint"
	"github.com/IliaSA101/sosninaalbums/internal/decor"
	"github.com/IliaSA101/sosninaalbums/internal/decor/pack"
	"github.com/IliaSA101/sosninaalbums/internal/library"
)

// themeKinds — виды плиток таблицы и их подписи в столбцах.
var themeKinds = []struct{ kind, col string }{
	{decor.KindBg, "бг"}, {decor.KindSticker, "ст"}, {decor.KindFrame, "рам"}, {decor.KindSpread, "разв"}, {decor.KindAlbum, "макет"},
}

// lastMan — манифест последнего поставленного набора (запоминает themesFill, читает themesCheck).
var lastMan pack.Manifest

func init() {
	themesFill = func(v *verifier, dm *decor.PackManifest, m pack.Manifest) {
		v.s.SetPrintCheck(ownprint.CheckData) // типографии набора проверяются так же, как у клиента
		lastMan = m
		for _, t := range m.Themes {
			dm.Themes = append(dm.Themes, decor.PackTheme{ID: t.ID, Name: t.Name, Base: t.Base, Order: t.Order})
		}
		for i, it := range m.Items {
			dm.Items[i].Topic = it.Topic
		}
	}
	themesCheck = checkThemes
}

// rawTopics — topic предметов как записан в manifest.json: файл → сырое значение. Размер записи — до чтения.
func rawTopics(path string) (map[string]json.RawMessage, error) {
	f, _, err := library.OpenLocal(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	zr, err := zip.NewReader(f, st.Size())
	if err != nil {
		return nil, err
	}
	for _, zf := range zr.File {
		if zf.Name != pack.ManifestName {
			continue
		}
		if zf.UncompressedSize64 > uint64(pack.MaxManifest) {
			return nil, fmt.Errorf("%s больше %d КБ", pack.ManifestName, pack.MaxManifest>>10)
		}
		rc, err := zf.Open()
		if err != nil {
			return nil, err
		}
		defer rc.Close()
		data, err := io.ReadAll(io.LimitReader(rc, pack.MaxManifest+1))
		if err != nil || int64(len(data)) > pack.MaxManifest {
			return nil, fmt.Errorf("%s не читается", pack.ManifestName)
		}
		var m struct {
			Items []struct {
				File  string
				Topic json.RawMessage
			}
		}
		if err := json.Unmarshal(data, &m); err != nil {
			return nil, err
		}
		out := make(map[string]json.RawMessage, len(m.Items))
		for _, it := range m.Items {
			out[it.File] = it.Topic
		}
		return out, nil
	}
	return nil, fmt.Errorf("в архиве нет %s", pack.ManifestName)
}

// checkThemes — проверка тем только что поставленного набора info; провалы копятся, блоки не прерывают друг друга.
func checkThemes(v *verifier, path string, info decor.PackInfo) {
	m := lastMan
	if m.ID != info.ID {
		return
	}
	if note := m.ThemeSkips.Note(m.ID); note != "" {
		v.failf("объявления тем не приняты: %s", note)
	}
	raw, err := rawTopics(path)
	if err != nil {
		v.failf("topic предметов не прочитан: %v", err)
	}
	want := map[string]int{} // тема → предметов в манифесте (видимые виды)
	for _, it := range m.Items {
		if r := strings.TrimSpace(string(raw[it.File])); r != "" && r != "null" && r != `""` && it.Topic == "" {
			v.failf("topic %s у «%s» сброшен в базу (%s): нет объявления в themes этого набора или значение не строка", clip(r), it.File, it.Theme)
		}
		if it.Topic != "" && slices.ContainsFunc(themeKinds, func(k struct{ kind, col string }) bool { return k.kind == it.Kind }) {
			want[it.Topic]++
		}
	}
	rows, order, names := themeTable(v, info)
	for _, t := range m.Themes {
		name, ok := names[t.ID]
		switch {
		case !ok:
			v.failf("тема %q не попала в таблицу программы: имя-двойник, служебный id или предел тем", t.ID)
		case name != t.Name:
			v.failf("тема %q уже объявлена другим набором с именем «%s», а здесь «%s»: строка одна", t.ID, name, t.Name)
		}
		if got := sumRow(rows[t.ID]); got != want[t.ID] {
			v.failf("тема %q: в каталоге предметов набора %d, в манифесте %d", t.ID, got, want[t.ID])
		}
	}
	fmt.Fprintln(v.w, "  темы набора (тема → бг / ст / рам / разв / макет):")
	for _, id := range order {
		c := rows[id]
		fmt.Fprintf(v.w, "    %s → %d / %d / %d / %d / %d\n", themeLabel(names, id), c[0], c[1], c[2], c[3], c[4])
	}
}

// clip — значение из манифеста для строки провала: коротко и без управляющих знаков (строку лога не подделать).
func clip(s string) string {
	r := []rune(s)
	if len(r) > 40 {
		r = r[:40]
	}
	return strings.Map(func(c rune) rune {
		if unicode.IsControl(c) {
			return '?'
		}
		return c
	}, string(r))
}

// themeTable — предметы набора по темам каталога: тема → число по видам (themeKinds); order — порядок показа тем
// набора; names — имена всех строк таблицы программы.
func themeTable(v *verifier, info decor.PackInfo) (rows map[string][5]int, order []string, names map[string]string) {
	rows, names = map[string][5]int{}, map[string]string{}
	for i, k := range themeKinds {
		cat, err := v.s.Catalog(k.kind, nil, 300, 300)
		if err != nil {
			v.failf("каталог %s: %v", k.kind, err)
			continue
		}
		for _, t := range cat.Themes {
			if _, ok := names[t.ID]; !ok {
				names[t.ID] = t.Name
				order = append(order, t.ID)
			}
		}
		for _, it := range cat.Items {
			if it.Pack == info.Title {
				c := rows[it.Theme]
				c[i]++
				rows[it.Theme] = c
			}
		}
	}
	return rows, slices.DeleteFunc(order, func(id string) bool { _, ok := rows[id]; return !ok }), names
}

// themeLabel — «Имя (id)» для таблицы.
func themeLabel(names map[string]string, id string) string {
	if name, ok := names[id]; ok {
		return fmt.Sprintf("%s (%s)", name, id)
	}
	return id
}

func sumRow(c [5]int) (n int) {
	for _, x := range c {
		n += x
	}
	return n
}
