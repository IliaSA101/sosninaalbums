package decor

import (
	"errors"
	"slices"
)

// ErrNoItem — предмета нет в наборе (пропущен при установке или не прошёл проверку формата входа).
var ErrNoItem = errors.New("этого предмета в наборе нет — выберите другой")

// ValidSrc — ровно 16 строчных hex: sha256[:16] записи манифеста (packEntry.Src).
func ValidSrc(s string) bool {
	if len(s) != 16 {
		return false
	}
	for i := range len(s) {
		if c := s[i]; c < '0' || c > '9' && c < 'a' || c > 'f' {
			return false
		}
	}
	return true
}

// PackItem — плитка предмета набора packID, который «Можно скачать» только что поставил (144). Вид,
// id и src проверяются до поиска; искать в архиве нечего — только реестр этого набора. Картинка и
// макет находятся по ключу ("my:" + src: ключ — sha байт), разворот — по packEntry.Src (ключ
// разворота — TplKey, с sha файла не совпадает; испорченный Src в реестре не находится, без
// ошибки). w×h — лист для плитки узора, как у Catalog.
func (s *Store) PackItem(packID, kind, src string, w, h float64) (Item, error) {
	if !slices.Contains([]string{KindBg, KindSticker, KindFrame, KindSpread, KindAlbum}, kind) ||
		!validPackID(packID) || !ValidSrc(src) {
		return Item{}, ErrNoItem
	}
	key := ""
	s.mu.RLock()
	if r := s.packOf(packID); r != nil {
		for _, e := range r.Items {
			if e.Kind != kind {
				continue
			}
			if _, ok := s.files[kind][e.Key]; !ok {
				continue
			}
			if kind == KindSpread && e.Src == src || kind != KindSpread && e.Key == libPrefix+src {
				key = e.Key
				break
			}
		}
	}
	s.mu.RUnlock()
	if key == "" {
		return Item{}, ErrNoItem
	}
	cat, err := s.Catalog(kind, nil, w, h)
	if err != nil {
		return Item{}, err
	}
	// Ключ мог достаться более старому набору (общая картинка): плитка одна, по ключу.
	for _, it := range cat.Items {
		if it.ID == key {
			return it, nil
		}
	}
	return Item{}, ErrNoItem
}
