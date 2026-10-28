package decor

import "github.com/IliaSA101/sosninaalbums/internal/album/model"

// Ссылки альбома на макеты (131 design §3, §8): запись Project.Look ↔ ключ плитки в индексе.
// Пути в ФС отсюда не берутся: только ключи индекса и точное совпадение записи манифеста.

// LookRefOf — запись макета для альбома по ключу плитки id: ключ, имя (своё или набора), а у
// макета набора — id набора и путь записи (по ним альбом найдёт макет после обновления набора).
// Нет файла в индексе — false.
func (s *Store) LookRefOf(id string) (model.LookRef, bool) {
	if !model.ValidLibKey(id) {
		return model.LookRef{}, false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if _, ok := s.files[KindAlbum][id]; !ok {
		return model.LookRef{}, false
	}
	ref := model.LookRef{Key: id}
	if e := s.entryOf(KindAlbum, id); e != nil {
		ref.Name = e.Name
	}
	for _, r := range s.packs {
		for _, e := range r.Items {
			if e.Kind != KindAlbum || e.Key != id {
				continue
			}
			if ref.Name == "" {
				ref.Name = e.Name
			}
			if ref.Pack == "" && ValidItemPath(e.Item) {
				ref.Pack, ref.Item = r.ID, e.Item
			}
		}
	}
	return ref, true
}

// LookKeyOf — ключ макета в индексе по записи альбома: тот же ключ, а если файла под ним нет
// (набор обновился) — запись набора по точному совпадению с манифестом. Нет — false.
func (s *Store) LookKeyOf(ref model.LookRef) (string, bool) {
	if model.ValidLibKey(ref.Key) {
		s.mu.RLock()
		_, ok := s.files[KindAlbum][ref.Key]
		s.mu.RUnlock()
		if ok {
			return ref.Key, true
		}
	}
	if ref.Pack != "" && ref.Item != "" {
		return s.LookByItem(ref.Pack, ref.Item)
	}
	return "", false
}

// HasLook — макет записи ref есть в библиотеке этого ПК (только индекс: файл не читается).
func (s *Store) HasLook(ref model.LookRef) bool {
	_, ok := s.LookKeyOf(ref)
	return ok
}
