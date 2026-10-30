package prefs

// Привязка места агента к диску (0.35/153, design §4.1): Place.Vol — номер тома и путь внутри него.
// Здесь только хранение и строение (диск не трогаем); том читает internal/osx, сверяет internal/app.
// Ключ места — «номер тома + путь внутри»: буква диска в него не входит, поэтому «Убрать» и «Привязать»
// находят место и после смены буквы.

import (
	"crypto/sha256"
	"encoding/hex"
	"path"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// Пределы PlaceVol: ID и метка — как в osx.Vol, путь внутри тома — как у Dir.
const (
	maxVolID    = 256
	maxVolLabel = 256
	maxVolRel   = 1024
)

// PlaceVol — диск места: ID не меняется при смене буквы, Rel — путь внутри тома от корня ФС через «/»
// (пусто — корень тома), Label — только для показа.
type PlaceVol struct {
	ID    string `json:"id"`
	Label string `json:"label,omitempty"`
	Rel   string `json:"rel"`
}

// noControl — строка без управляющих знаков, годный UTF-8.
func noControl(s string) bool {
	if !utf8.ValidString(s) {
		return false
	}
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			return false
		}
	}
	return true
}

// cleanVol — годный PlaceVol или nil (негодный привязку теряет, само место остаётся): ID непустой,
// путь внутри без «..» и начального «/»; метка чистится, а не отвергается — она не решает ничего.
func cleanVol(v *PlaceVol) *PlaceVol {
	if v == nil || v.ID == "" || len(v.ID) > maxVolID || !noControl(v.ID) || len(v.Rel) > maxVolRel || !noControl(v.Rel) {
		return nil
	}
	rel := path.Clean(v.Rel)
	if rel == "." {
		rel = ""
	}
	if strings.HasPrefix(rel, "/") || rel == ".." || strings.HasPrefix(rel, "../") {
		return nil
	}
	label := strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, strings.ToValidUTF8(v.Label, ""))
	if len(label) > maxVolLabel {
		label = strings.ToValidUTF8(label[:maxVolLabel], "")
	}
	return &PlaceVol{ID: v.ID, Label: label, Rel: rel}
}

// Key — ключ места для сравнения: том и путь внутри него (регистр по правилу ОС) либо, без тома,
// paths.DirKey(Dir). Два места с одним Dir и разными ID — два места (две карты под одной буквой).
func (p Place) Key() string {
	if p.Vol == nil {
		return paths.DirKey(p.Dir)
	}
	return p.Vol.ID + "\x00" + paths.Fold(p.Vol.Rel)
}

// PlaceKey — ключ места наружу (интерфейс): первые 12 hex sha256 от Key. Путь и номер тома в окно
// не идут; «Убрать» и «Привязать» находят место по ключу, добавить путь окно не может.
func PlaceKey(p Place) string {
	sum := sha256.Sum256([]byte(p.Key()))
	return hex.EncodeToString(sum[:])[:12]
}

// relDirKey — путь внутри тома как ключ каталога: «/a/b/», корень — «/».
func relDirKey(rel string) string {
	if rel == "" {
		return "/"
	}
	return "/" + paths.Fold(rel) + "/"
}

// PlaceInside — место inner лежит внутри места outer (или совпадает): отдельным оно ничего не
// добавляет. Только на одном диске: оба привязаны к тому же ID (сравнение внутри тома) или оба без
// привязки (по пути); привязанное и непривязанное, а также разные ID — не внутри (0.35-153-14).
func PlaceInside(inner, outer Place) bool {
	switch {
	case inner.Vol == nil && outer.Vol == nil:
		return strings.HasPrefix(paths.DirKey(inner.Dir), paths.DirKey(outer.Dir))
	case inner.Vol != nil && outer.Vol != nil && inner.Vol.ID == outer.Vol.ID:
		return strings.HasPrefix(relDirKey(inner.Vol.Rel), relDirKey(outer.Vol.Rel))
	}
	return false
}

// BindPlaces привязывает к диску места без привязки: ключ binds — PlaceKey места. Места, ставшие
// внутренними (на том же диске), убираются. Возвращает число привязанных; нечего менять — список
// не трогается.
func (p *AgentPerms) BindPlaces(binds map[string]PlaceVol) int {
	list := slices.Clone(p.Places)
	n := 0
	for i, v := range list {
		if v.Vol != nil {
			continue
		}
		if b, ok := binds[PlaceKey(v)]; ok {
			if vol := cleanVol(&b); vol != nil {
				list[i].Vol = vol
				n++
			}
		}
	}
	if n > 0 {
		p.Places = normalizePlaces(list, MaxPlaces)
	}
	return n
}

// ForgetPlace убирает одно место по ключу PlaceKey; false — такого нет (холостое).
func (p *AgentPerms) ForgetPlace(key string) bool {
	i := slices.IndexFunc(p.Places, func(v Place) bool { return PlaceKey(v) == key })
	if i < 0 {
		return false
	}
	gone := p.Places[i]
	p.Places = slices.Delete(slices.Clone(p.Places), i, i+1)
	// Заметка «было → стало» об этом месте теряет смысл вместе с ним (✕ у поднятого места).
	if len(p.Lifted) > 0 {
		to := paths.DirKey(gone.Dir)
		p.Lifted = slices.DeleteFunc(slices.Clone(p.Lifted), func(l PlaceLift) bool { return paths.DirKey(l.To) == to })
		if len(p.Lifted) == 0 {
			p.Lifted = nil
		}
	}
	return true
}
