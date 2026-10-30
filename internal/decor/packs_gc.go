package decor

import (
	"bytes"
	"encoding/json"
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
	if err := s.keepBase(reg); err != nil {
		return PackRemove{}, errors.New("набор не удалился: " + paths.NoPath(err).Error())
	}
	if err := os.Remove(filepath.Join(s.root, packsDir, id+".json")); err != nil && !errors.Is(err, os.ErrNotExist) {
		return PackRemove{}, errors.New("набор не удалился: " + paths.NoPath(err).Error())
	}
	s.mu.Lock()
	s.packs = slices.DeleteFunc(s.packs, func(r *packReg) bool { return r == reg })
	s.mu.Unlock()
	return s.collect(reg.Items, keep, nil, false), nil
}

// keepBase — до удаления набора gone дописывает текущую границу первой партии оставшимся наборам, у
// которых она не записана или позже (133 §4): иначе удаление самого раннего сделало бы «первой партией»
// следующие. Наборы, чья дата и есть граница, в записи не нуждаются. Под ioMu; сбой записи — удаление
// не начинается (повторный вызов дописывает остальное). Файл соседа правит writeBase (только base).
func (s *Store) keepBase(gone *packReg) error {
	s.mu.RLock()
	base, found := s.packBase(nil)
	rest, _ := s.packBase(gone)
	var todo []*packReg
	for _, r := range s.packs {
		// rest == base: удаляемый границу не держит, она не сдвинется — писать нечего.
		if r != gone && found && rest != base && dateOK(r.Installed) && r.Installed > base && (r.Base == 0 || r.Base > base) {
			todo = append(todo, r)
		}
	}
	s.mu.RUnlock()
	for _, r := range todo {
		written, err := s.writeBase(r.ID, base)
		if err != nil {
			return err
		}
		if !written {
			continue // реестра нет или он чужой — не трогаем, граница там считается по Installed
		}
		s.mu.Lock()
		r.Base = base
		s.mu.Unlock()
	}
	return nil
}

// writeBase — меняет в реестре набора id на диске только поле base: остальное (записи незнакомого вида,
// поля новее программы) переписывается как есть, а не из разобранной структуры, которая это зажала бы.
// Читается текущий файл, не память со старта. Нет файла, он больше packRegMax, не JSON или v новее
// программы — не пишет (written=false, не ошибка; кроме «нет файла» — след в журнале). Ошибка —
// чтение не по содержимому или сбой записи. Ключи, различающиеся только регистром, после сортировки
// могут поменять, какой из них прочтёт readPackReg, — бывает лишь при ручной правке, не чиним.
func (s *Store) writeBase(id string, base int64) (written bool, err error) {
	path := filepath.Join(s.root, packsDir, id+".json")
	data, err := readCapped(path, packRegMax)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if errors.Is(err, errTooBig) {
		return false, skipBase(id, "слишком большой файл")
	}
	if err != nil {
		return false, err
	}
	var m map[string]json.RawMessage
	if json.Unmarshal(data, &m) != nil {
		return false, skipBase(id, "не JSON-объект")
	}
	var v int
	if json.Unmarshal(m["v"], &v) != nil || v < 1 || v > packRegV {
		return false, skipBase(id, "версия формата не наша") // новее программы (или нет вовсе): чужое не перезаписываем
	}
	if m["base"], err = json.Marshal(base); err != nil {
		return false, err
	}
	// Без экранирования <, >, &: чужие значения остаются теми же символами, а не \u003c.
	var out bytes.Buffer
	enc := json.NewEncoder(&out)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(m); err != nil {
		return false, err
	}
	return true, library.WriteFileAtomic(path, bytes.TrimRight(out.Bytes(), "\n"))
}

// skipBase — след в журнале: границу в реестре не записали из-за его содержимого (не ошибка удаления).
func skipBase(id, why string) error {
	log.Printf("библиотека: граница набора %s не записана (%s)", id, why)
	return nil
}

// collect — сборка мусора по ключам (под ioMu): файл удаляется, если ключ не держат свои,
// наборы кроме except, шаблоны (свои и наборов кроме except), записи незнакомого вида и keep;
// dry — только посчитать. Незнакомое не прочиталось до конца — не удаляется ничего.
// Удаление — только внутри папки вида.
//
// Макеты альбома — первыми: их ссылки не считаются (drop), если макет удаляется. Держат макет только
// свои, наборы и keep — то же правило, что у lookDropPaths (refs — ссылки на картинки, макет ими не
// держится). Макет из drop выжил (не удалился) — ссылки пересчитываются до удаления картинок.
func (s *Store) collect(items []packEntry, keep func(key string) bool, except *packReg, dry bool) PackRemove {
	var res PackRemove
	if len(items) == 0 {
		return res // нечего собирать — полный проход по диску не нужен
	}
	drop := s.lookDropPaths(items, keep, except)
	refs, complete := s.tplRefs(except, drop)
	if !complete {
		log.Print("библиотека: незнакомое прочитано не до конца — сборка мусора ничего не удаляет")
	}
	stay := map[string]bool{} // макеты из drop, которые остались на диске
	for _, e := range items {
		if e.Kind != KindAlbum {
			continue
		}
		if path, gone := s.collectOne(e, nil, complete, keep, except, dry, &res); !gone && drop[path] {
			stay[path] = true
		}
	}
	if len(stay) > 0 && complete {
		for p := range stay {
			delete(drop, p)
		}
		if refs, complete = s.tplRefs(except, drop); !complete {
			log.Print("библиотека: незнакомое прочитано не до конца — сборка мусора ничего не удаляет")
		}
	}
	for _, e := range items {
		if e.Kind != KindAlbum {
			s.collectOne(e, refs, complete, keep, except, dry, &res)
		}
	}
	return res
}

// collectOne — один файл сборки мусора: путь из индекса и удалён ли он (dry — удалился бы). refs у
// макета не смотрятся (nil): ссылки — на картинки.
func (s *Store) collectOne(e packEntry, refs map[string]bool, complete bool, keep func(key string) bool, except *packReg, dry bool, res *PackRemove) (string, bool) {
	s.mu.RLock()
	path, ok := s.files[e.Kind][e.Key]
	held := !complete || refs[e.Key] || s.heldBy(e.Key, except)
	s.mu.RUnlock()
	if !ok {
		return "", false // файла нет — удалять нечего
	}
	path = filepath.Clean(path)
	if held || keep != nil && keep(e.Key) {
		res.Kept++
		return path, false
	}
	fi, err := os.Stat(path)
	if err != nil || !library.Under(path, filepath.Join(s.root, kindDirs[e.Kind])) {
		return path, false
	}
	if !dry {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			log.Printf("библиотека: файл набора не удалён: %v", paths.NoPath(err))
			return path, false
		}
		s.mu.Lock()
		delete(s.files[e.Kind], e.Key)
		s.mu.Unlock()
	}
	res.Files++
	res.Bytes += fi.Size()
	return path, true
}

// tplRefs — ключи картинок, на которые ссылаются шаблоны разворотов: свои (и скрытые — как
// heldBy) и наборов, кроме except. Иначе удаление набора стёрло бы фон своего шаблона, ради
// которого фотограф его и сохранял. Разбор — через кэш плиток (ключ — хэш содержимого).
// К ссылкам известного разбора добавлены ключи из сырых байт шаблонов и из записей незнакомого вида
// (unknownRefs) и из файлов макетов альбома (131) — сырые и по разбору (lookRefs), кроме drop — файлов
// макетов, которые удалит эта же сборка; complete = false — незнакомое прочитано не до конца.
func (s *Store) tplRefs(except *packReg, drop map[string]bool) (refs map[string]bool, complete bool) {
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
	unknown := s.unknownRefsDrop(except, drop)
	refs = unknown.keys
	for _, key := range keys {
		info := s.tplLookup(key)
		if info.unread {
			unknown.complete = false // шаблон есть, но не прочитан — его ссылок не знаем
		}
		for _, ref := range info.libs {
			refs[ref.key] = true
		}
		for _, k := range info.raw {
			refs[k] = true
		}
	}
	if unknown.complete { // не до конца — и так ничего не удаляется, разбор макетов не нужен
		unknown.complete = s.lookRefs(refs, drop)
	}
	return refs, unknown.complete
}
