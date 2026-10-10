package salbum

import (
	"maps"
	"slices"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// dedupe: фото стоит в альбоме один раз (остаётся первое вхождение), ID без
// записи в photos убирается, повтор ID разворота получает новый. Место сетки с
// убранным фото (или вторым вхождением) пустеет, но остаётся. Разворот,
// опустевший от этого и пустой (Spread.empty), удаляется.
func dedupe(p *model.Project) {
	seenPhoto, seenSpread := map[string]bool{}, map[string]bool{}
	out := p.Spreads[:0]
	for _, s := range p.Spreads {
		if s.ID == "" || seenSpread[s.ID] {
			s.ID = model.NewID()
		}
		seenSpread[s.ID] = true
		n := len(s.Photos)
		s.Photos = slices.DeleteFunc(s.Photos, func(id string) bool {
			_, ok := p.Photos[id]
			if !ok || seenPhoto[id] {
				return true
			}
			seenPhoto[id] = true
			return false
		})
		model.EmptyGridDupes(s)
		if len(s.Photos) != n {
			s.Layout, s.Variant = "", 0
			if s.Empty() {
				continue
			}
		}
		out = append(out, s)
	}
	p.Spreads = out
}

// Rekey приводит ID фото к idFor(путь) (library.IDFor): альбом переехал — иначе
// лента не узнала бы фото как использованные; а ID, не равный пути, показывал бы
// на экране одно фото, а в печать отдавал другое. Записи с одним путём сливаются:
// остаётся первая в порядке ленты (с её кадром и подписью), ссылки в разворотах
// переводятся на новый ID (и в местах сетки — renameRefs), повтор фото в альбоме —
// первое вхождение (dedupe; слившийся повтор в месте — пустое место).
func Rekey(p *model.Project, idFor func(string) string) {
	order := make([]string, 0, len(p.Photos))
	seen := make(map[string]bool, len(p.Photos))
	for _, s := range p.Spreads {
		for _, id := range s.PhotoIDs() {
			if _, ok := p.Photos[id]; ok && !seen[id] {
				seen[id] = true
				order = append(order, id)
			}
		}
	}
	// Записи вне лент в альбоме не нужны (pruneRefs), но порядок держим стабильным.
	for _, id := range slices.Sorted(maps.Keys(p.Photos)) {
		if !seen[id] {
			order = append(order, id)
		}
	}
	photos := make(map[string]model.PhotoRef, len(p.Photos))
	moved := map[string]string{}
	merged := false
	for _, id := range order {
		ref := p.Photos[id]
		nid := idFor(ref.Path)
		if _, taken := photos[nid]; taken {
			merged = true
		} else {
			photos[nid] = ref
		}
		if nid != id {
			moved[id] = nid
		}
	}
	p.Photos = photos
	p.RenameRefs(moved)
	if merged {
		dedupe(p)
	}
}

// migrate доводит прочитанный проект до текущей версии. v1 → v2: новые поля
// нулевые, папки-строки разобраны в decode; номер варианта (файлы до ключей)
// переводится в ключ той же раскладки. v2 → v3, v3 → v4, v4 → v5: только номер
// версии (у старых файлов нет мест, зеркала, прозрачности фона и своих отметок).
func migrate(p *model.Project) {
	var warm []*model.Spread
	for _, s := range p.Spreads {
		if s.Layout == "" && s.Variant > 0 {
			warm = append(warm, s)
		}
	}
	p.WarmLayouts(warm)
	for _, s := range p.Spreads {
		if s.Layout == "" && s.Variant > 0 {
			if res := layout.Cached(p.Aspects(s.Photos), p.Params(), model.VariantLimit); len(res) > 0 {
				s.Layout = res[min(s.Variant, len(res)-1)].Key
				if s.Layout == res[0].Key {
					s.Layout = ""
				}
			}
		}
		s.Variant = 0
	}
	p.Version = model.FormatVersion
}

// dropNilPlaces — первый шаг decode: `"grid":[null]` даёт nil в []*Place без
// ошибки разбора, а дальше всё, что трогает места, упало бы.
func dropNilPlaces(p *model.Project) {
	for _, s := range p.Spreads {
		s.Grid = slices.DeleteFunc(s.Grid, func(pl *model.Place) bool { return pl == nil })
	}
}

// unionGrid — истина при чтении и места, и Photos: фото, стоящее только в месте,
// дописывается в Photos (иначе выпало бы); фото только в Photos получит место
// при починке (syncGrid(true)). До dedupe: повторы она разбирает уже с местами.
func unionGrid(p *model.Project) {
	for _, s := range p.Spreads {
		if len(s.Grid) == 0 {
			continue
		}
		in := make(map[string]bool, len(s.Photos))
		for _, id := range s.Photos {
			in[id] = true
		}
		for _, pl := range s.Grid {
			if pl.Photo != "" && !in[pl.Photo] {
				in[pl.Photo] = true
				s.Photos = append(s.Photos, pl.Photo)
			}
		}
	}
}
