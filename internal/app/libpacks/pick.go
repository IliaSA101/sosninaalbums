package libpacks

import (
	"slices"

	"github.com/IliaSA101/sosninaalbums/internal/decor"
	"github.com/IliaSA101/sosninaalbums/internal/decor/fetch"
)

// Строки «Можно скачать» и состав «Скачать» (спека 137, design §3, критика Д11, П4): чистая
// логика над сохранённым или свежим каталогом, без сети и диска.

// whyApp — причина серой кнопки: набору нужна программа новее.
func whyApp(e fetch.Entry) string {
	return "нужна версия Sosnina Albums " + e.MinApp + " или новее"
}

// catIndex — каталог для обхода: строки по id и дополнения по id донора (в порядке каталога),
// собранные одним проходом.
type catIndex struct {
	byID map[string]fetch.Entry
	kids map[string][]fetch.Entry
}

func indexOf(cat fetch.Catalog) catIndex {
	ix := catIndex{byID: make(map[string]fetch.Entry, len(cat.Packs)), kids: map[string][]fetch.Entry{}}
	for _, e := range cat.Packs {
		ix.byID[e.ID] = e
		if e.Needs != "" {
			ix.kids[e.Needs] = append(ix.kids[e.Needs], e)
		}
	}
	return ix
}

func haveOf(st *decor.Store) map[string]decor.PackInfo {
	have := map[string]decor.PackInfo{}
	for _, p := range st.Packs() {
		have[p.ID] = p
	}
	return have
}

// closure — что поставит «Скачать» для id: сам набор, его нестоящие доноры вверх по needs и
// годные дополнения вниз. Вниз не идут стоящие, удалённые пользователем (skip — кроме нажатого),
// наборы с недопустимым id и те, что просят программу новее. Посещённые считаются по set, поэтому
// цикл needs (a↔b) конечен. up — доноры вверх (их пропуск «Скачать» тоже снимает после удачной
// установки), ents — состав в порядке каталога.
type closure struct {
	set  map[string]bool
	up   []string
	ents []fetch.Entry
}

func (ix catIndex) closure(cat fetch.Catalog, id string, have map[string]decor.PackInfo, skip []string, app string) closure {
	c := closure{set: map[string]bool{id: true}}
	// Вверх: донор, которого нет в библиотеке и ещё нет в составе.
	for cur := ix.byID[id]; cur.Needs != ""; {
		d, ok := ix.byID[cur.Needs]
		_, stands := have[cur.Needs]
		if !ok || stands || c.set[d.ID] {
			break
		}
		c.set[d.ID] = true
		c.up = append(c.up, d.ID)
		cur = d
	}
	// Вниз: обход в ширину по kids от всех, кто уже в составе.
	queue := append([]string{id}, c.up...)
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, k := range ix.kids[cur] {
			_, stands := have[k.ID]
			if c.set[k.ID] || stands || slices.Contains(skip, k.ID) || k.NeedsApp(app) {
				continue
			}
			c.set[k.ID] = true
			queue = append(queue, k.ID)
		}
	}
	for _, e := range cat.Packs {
		if c.set[e.ID] {
			c.ents = append(c.ents, e)
		}
	}
	return c
}

// availRows — строки раздела по порядку каталога. Набор в библиотеке (любого происхождения,
// встроенный тоже) строки не имеет. Дополнение к стоящему донору — строка с donor; дополнение к
// нестоящему приходит с донором и своей строки не имеет. Bytes и дополнения — по составу closure
// (то, что «Скачать» действительно поставит).
func availRows(cat fetch.Catalog, have map[string]decor.PackInfo, skip []string, app string) []Avail {
	ix := indexOf(cat)
	rows := []Avail{}
	for _, e := range cat.Packs {
		if _, stands := have[e.ID]; stands {
			continue
		}
		donor := ""
		if e.Needs != "" {
			d, ok := have[e.Needs]
			if !ok {
				continue
			}
			donor = d.Title
		}
		c := ix.closure(cat, e.ID, have, skip, app)
		a := Avail{ID: e.ID, Title: e.Title, About: e.About, License: e.License, Version: e.Version,
			Bg: e.Counts.Bg, Sticker: e.Counts.Sticker, Spread: e.Counts.Spread, Frame: e.Counts.Frame, Print: e.Counts.Print,
			Donor: donor, AddonTitles: []string{}, Removed: slices.Contains(skip, e.ID)}
		for _, m := range c.ents {
			a.Bytes += m.Size
			if m.ID != e.ID {
				a.AddonTitles = append(a.AddonTitles, m.Title)
			}
		}
		if e.NeedsApp(app) {
			a.Why = whyApp(e)
		}
		rows = append(rows, a)
	}
	return rows
}

// downloadable — строки, которые ставит «Скачать все»: не серые и не «удалён вами».
func downloadable(rows []Avail) []Avail {
	return slices.DeleteFunc(slices.Clone(rows), func(a Avail) bool { return a.Why != "" || a.Removed })
}

// bytesOf — сумма Bytes строк (дополнения уже внутри).
func bytesOf(rows []Avail) (n int64) {
	for _, a := range rows {
		n += a.Bytes
	}
	return n
}

// shownPlan — «Скачать все»: строки без серой кнопки и без «удалён вами», id их составов
// (объединение closure — план DownloadAll), число строк и байты состава. Одна функция считает и
// подпись, и план, и available в итогах (критика П1, Д4).
func shownPlan(cat fetch.Catalog, have map[string]decor.PackInfo, skip []string, app string) (ids map[string]bool, all Offer) {
	ix := indexOf(cat)
	ids = map[string]bool{}
	for _, r := range downloadable(availRows(cat, have, skip, app)) {
		all.Count++
		for _, m := range ix.closure(cat, r.ID, have, skip, app).ents {
			if !ids[m.ID] {
				ids[m.ID] = true
				all.Bytes += m.Size
			}
		}
	}
	return ids, all
}

// updateWant — id для «Обновить»: стоящие (версии и причины «из файла»/«встроенный») и
// дополнения к стоящему донору, которых нет. Новые наборы сюда не входят.
func updateWant(cat fetch.Catalog, have map[string]decor.PackInfo) map[string]bool {
	want := map[string]bool{}
	for _, e := range cat.Packs {
		_, stands := have[e.ID]
		_, donor := have[e.Needs]
		if stands || e.Needs != "" && donor {
			want[e.ID] = true
		}
	}
	return want
}
