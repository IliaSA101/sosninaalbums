package decor

import (
	"errors"
	"log"
	"os"
	"path/filepath"
	"slices"

	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// Удаление наборов и сборка мусора по ключам (design §4.5, решение 10).

// PackRemove — числа для вопроса «Удалить набор?» и итог удаления.
type PackRemove struct {
	Files int   `json:"files"` // удаляются с диска
	Kept  int   `json:"kept"`  // остаются: нужны своим, другому набору или открытому альбому
	Bytes int64 `json:"bytes"` // освобождается
}

// PlanRemovePack — что удалит RemovePack, без изменений.
func (s *Store) PlanRemovePack(id string, keep func(key string) bool) (PackRemove, error) {
	if id == BuiltinPack {
		return PackRemove{}, ErrBuiltin
	}
	s.ioMu.Lock()
	defer s.ioMu.Unlock()
	s.mu.RLock()
	reg := s.packOf(id)
	s.mu.RUnlock()
	if reg == nil {
		return PackRemove{}, ErrNotFound
	}
	return s.collect(reg.Items, keep, reg, true), nil
}

// RemovePack — удаление набора (design §4.5, решение 10): сначала реестр (точка фиксации),
// затем файлы его ключей, кроме нужных своим (и скрытым), другому набору и открытому альбому.
func (s *Store) RemovePack(id string, keep func(key string) bool) (PackRemove, error) {
	if id == BuiltinPack {
		return PackRemove{}, ErrBuiltin
	}
	s.ioMu.Lock()
	defer s.ioMu.Unlock()
	s.mu.RLock()
	ro, reg := s.readOnly, s.packOf(id)
	s.mu.RUnlock()
	if ro {
		return PackRemove{}, ErrReadOnly
	}
	if reg == nil {
		return PackRemove{}, ErrNotFound
	}
	if err := os.Remove(filepath.Join(s.root, packsDir, id+".json")); err != nil && !errors.Is(err, os.ErrNotExist) {
		return PackRemove{}, errors.New("набор не удалился: " + paths.NoPath(err).Error())
	}
	s.mu.Lock()
	s.packs = slices.DeleteFunc(s.packs, func(r *packReg) bool { return r == reg })
	s.mu.Unlock()
	return s.collect(reg.Items, keep, nil, false), nil
}

// collect — сборка мусора по ключам (под ioMu): файл удаляется, если ключ не держат свои,
// наборы кроме except, шаблоны (свои и наборов кроме except) и keep; dry — только посчитать.
// Удаление — только внутри папки вида.
func (s *Store) collect(items []packEntry, keep func(key string) bool, except *packReg, dry bool) PackRemove {
	var res PackRemove
	refs := s.tplRefs(except)
	for _, e := range items {
		s.mu.RLock()
		path, ok := s.files[e.Kind][e.Key]
		held := refs[e.Key] || s.heldBy(e.Key, except)
		s.mu.RUnlock()
		if !ok {
			continue // файла нет — удалять нечего
		}
		if held || keep != nil && keep(e.Key) {
			res.Kept++
			continue
		}
		fi, err := os.Stat(path)
		if err != nil || !library.Under(path, filepath.Join(s.root, kindDirs[e.Kind])) {
			continue
		}
		if !dry {
			if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
				log.Printf("библиотека: файл набора не удалён: %v", paths.NoPath(err))
				continue
			}
			s.mu.Lock()
			delete(s.files[e.Kind], e.Key)
			s.mu.Unlock()
		}
		res.Files++
		res.Bytes += fi.Size()
	}
	return res
}

// tplRefs — ключи картинок, на которые ссылаются шаблоны разворотов: свои (и скрытые — как
// heldBy) и наборов, кроме except. Иначе удаление набора стёрло бы фон своего шаблона, ради
// которого фотограф его и сохранял. Разбор — через кэш плиток (ключ — хэш содержимого).
func (s *Store) tplRefs(except *packReg) map[string]bool {
	var keys []string
	s.mu.RLock()
	for _, e := range s.entries {
		if e.Kind == KindSpread {
			keys = append(keys, e.Key)
		}
	}
	for _, r := range s.packs {
		if r == except {
			continue
		}
		for _, e := range r.Items {
			if e.Kind == KindSpread {
				keys = append(keys, e.Key)
			}
		}
	}
	s.mu.RUnlock()
	refs := map[string]bool{}
	for _, key := range keys {
		for _, ref := range s.tplLookup(key).libs {
			refs[ref.key] = true
		}
	}
	return refs
}
