package decor

import (
	"cmp"
	"encoding/json"
	"log"
	"slices"
)

// Таблица тем (спека 170, design §4): встроенные строки themeList плюс темы доверенных наборов,
// объявленные в манифесте (PackManifest.Themes) и хранимые в реестре набора. Строится при открытии
// и при каждой смене наборов; Catalog и проверки «тема есть» читают готовое под mu.

// themeDef — строка таблицы. base == "" — встроенная или служебная строка; иначе тема набора.
type themeDef struct {
	id, name, base string
	order          int
	at             int64 // Installed объявившего набора
}

// baseRows — строки таблицы без наборов: themeList без выборок recent и all (их Catalog ставит сам).
var baseRows = func() []themeDef {
	var out []themeDef
	for _, t := range themeList {
		if t.id != ThemeRecent && t.id != ThemeAll {
			out = append(out, themeDef{id: t.id, name: t.name})
		}
	}
	return out
}()

// PackTheme — объявление темы из манифеста набора (pack.Theme).
type PackTheme struct {
	ID, Name, Base string
	Order          int // 0 — не задан (50)
}

// themeRows — порядок показа тем без выборок; под mu. Пока таблицу не строили — встроенные.
func (s *Store) themeRows() []themeDef {
	if len(s.tab) == 0 {
		return baseRows
	}
	return s.tab
}

// packTopic — тема предмета набора: topic, если это встроенная тема или тема таблицы (кто бы её ни
// объявил: имя — из доверенного набора), иначе база theme (170-23); под mu.
func (s *Store) packTopic(e packEntry) string {
	if e.Topic != "" && (isBaseTheme(e.Topic) || s.kidTheme(e.Topic)) {
		return e.Topic
	}
	return e.Theme
}

// kidTheme — тема набора с таким id есть в таблице; под mu.
func (s *Store) kidTheme(id string) bool {
	_, ok := s.tabIdx[id]
	return ok
}

// isThemeL — тема, в которой может лежать элемент: встроенная или тема набора; под mu. Пакетная
// isTheme — только встроенные (пресеты, packTheme: то, что видят и старые версии).
func (s *Store) isThemeL(id string) bool { return isTheme(id) || s.kidTheme(id) }

// ownThemeL — тема, в которую можно положить своё; под mu.
func (s *Store) ownThemeL(id string) bool { return id != ThemeLegacy && s.isThemeL(id) }

// ownTheme — ownThemeL под своим RLock: Import, SaveSpreadTpl, SaveLook зовут её без mu.
func (s *Store) ownTheme(id string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ownThemeL(id)
}

// validOwnTheme — тема своей записи при чтении library.json: грамматика, не выборка, не brand и не
// legacy. Есть ли тема в таблице, здесь не решается: наборы читаются позже, а значение в файле
// остаётся, пока набор не вернётся (170, §4.4). Показ в «Моих» — Catalog.
func validOwnTheme(id string) bool {
	switch id {
	case ThemeRecent, ThemeAll, ThemeNew, ThemeBrand, ThemeLegacy:
		return false
	}
	return validKind(id)
}

// rebuildThemes — таблица из встроенных строк и объявлений наборов; под mu на запись. Наборы идут
// по возрастанию Installed (порядок s.packs): первый объявивший id владеет именем и местом, удалили
// его — при перестройке тема переходит к следующему (170-5). Верят только наборам с origin net и
// demo — по полю реестра, а не по id: набор из файла с id примера объявлений не получает (170-7).
func (s *Store) rebuildThemes() {
	var kids []themeDef
	idx := map[string]int{}
	names := reservedFolded()
	for _, r := range s.packs {
		if r.Origin != OriginNet && r.Origin != OriginDemo {
			continue
		}
		var sk themeSkips
		for _, d := range r.Themes {
			if _, ok := idx[d.ID]; ok {
				continue // общая тема: имя и место у поставленного раньше
			}
			why := ""
			switch {
			case !validThemeID(d.ID):
				why = whyID
			case names[themeFold(d.Name)]:
				why = whyName
			case !isBaseTheme(d.Base):
				why = whyBase
			case len(kids) >= maxPackThemes:
				why = whyLimit
			}
			if why != "" {
				sk.add(why)
				continue
			}
			idx[d.ID] = len(kids)
			names[themeFold(d.Name)] = true
			kids = append(kids, themeDef{id: d.ID, name: d.Name, base: d.Base, order: d.Order, at: r.Installed})
		}
		s.noteThemes(r.ID, sk)
	}
	slices.SortStableFunc(kids, func(a, b themeDef) int {
		return cmp.Or(cmp.Compare(a.order, b.order), cmp.Compare(a.at, b.at), cmp.Compare(a.id, b.id))
	})
	tab := make([]themeDef, 0, len(baseRows)+len(kids))
	for _, row := range baseRows {
		tab = append(tab, row)
		if isBaseTheme(row.id) { // за встроенной — её темы наборов; пустая база скрывается в Catalog, дети остаются
			for _, k := range kids {
				if k.base == row.id {
					tab = append(tab, k)
				}
			}
		}
	}
	s.tab = tab
	s.tabIdx = make(map[string]int, len(kids))
	for i, d := range tab {
		if d.base != "" {
			s.tabIdx[d.id] = i
		}
	}
}

// noteThemes — одна строка в log.txt за сеанс на набор (170-30); под mu.
func (s *Store) noteThemes(setID string, sk themeSkips) {
	if sk.n == 0 || s.themeLogged[setID] {
		return
	}
	if s.themeLogged == nil {
		s.themeLogged = map[string]bool{}
	}
	s.themeLogged[setID] = true
	log.Print("библиотека: " + sk.note(setID))
}

// Новые поля реестра набора читаются через RawMessage (design §2.2): незнакомый тип поля не должен
// ронять реестр, как ронял бы типизированный разбор, — набор грузится, предметы в базе.

// parseRegThemes разбирает сырые themes, tv и topic после Unmarshal реестра; Raw-поля освобождаются.
func (r *packReg) parseRegThemes() themeSkips {
	var sk themeSkips
	r.Themes, sk = parseThemeDecl(r.ThemesRaw, 0) // без предела байт: размер держит packRegMax (170-29)
	var tv int
	if json.Unmarshal(r.TVRaw, &tv) != nil || tv < 0 {
		tv = 0 // набор один раз перекачается: безвредно
	}
	r.TV = tv
	r.ThemesRaw, r.TVRaw = nil, nil
	return sk
}

// topicOfRaw — topic записи: строка [a-z]{1,16}, иначе "".
func topicOfRaw(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) != nil || !validKind(s) {
		return ""
	}
	return s
}

// syncRaw перед записью реестра заполняет Raw-поля из разобранных: themes и topic — если есть, tv —
// всегда (`tv` ≥ 1 — «писала программа, которая знает темы»).
func (r *packReg) syncRaw() {
	r.ThemesRaw, r.TVRaw = nil, nil
	if len(r.Themes) > 0 {
		r.ThemesRaw, _ = json.Marshal(r.Themes)
	}
	if r.TV > 0 {
		r.TVRaw, _ = json.Marshal(r.TV)
	}
	for i := range r.Items {
		r.Items[i].TopicRaw = nil
		if r.Items[i].Topic != "" {
			r.Items[i].TopicRaw, _ = json.Marshal(r.Items[i].Topic)
		}
	}
}

// topicOf — тема предмета манифеста для реестра: id объявленной (после правила decor) или
// встроенной темы, иначе "" — предмет в базе.
func (st *Staged) topicOf(topic string) string {
	if isBaseTheme(topic) || slices.ContainsFunc(st.decls, func(d themeDecl) bool { return d.ID == topic }) {
		return topic
	}
	return ""
}
