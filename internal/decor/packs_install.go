package decor

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"slices"

	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// Установка и удаление наборов (design §4.3, §4.5). Разбор архива — пакет pack (App переносит
// манифест сюда копией: decor не импортирует pack — depguard); проверка содержимого — packs_stage.go,
// здесь запись под ключами и сборка мусора.

var (
	// ErrPackEmpty — ни один предмет набора не прошёл проверку.
	ErrPackEmpty = errors.New("в наборе нечего ставить")
	errPackOlder = errors.New("уже стоит версия набора новее")
	errPackSame  = errors.New("этот набор уже стоит")
	// ErrBadPackID — id набора не по грамматике или имя устройства Windows (con, nul…): ошибка
	// строки каталога или манифеста, а не диска.
	ErrBadPackID = errors.New("недопустимый id набора")
	// ErrLibFull — своё и наборы вместе дошли до MaxLibraryLen.
	ErrLibFull = fmt.Errorf("библиотека заполнена (%d ГБ)", MaxLibraryLen>>30)
)

// PackManifest — проверенный манифест набора (pack.Manifest без пропущенных предметов).
type PackManifest struct {
	ID, Title, About, Author, License, Source string
	Version                                   int
	MinApp                                    string // x.y.z
	Items                                     []PackItem
	Themes                                    []PackTheme // объявления тем (170); решает ли им верить, StagePack не знает — таблица по origin
}

// PackItem — предмет манифеста: имя записи (для расширения и тоста), вид, тема (база), имя, размер,
// Topic — тема набора из его объявлений или встроенная ("" — предмет в базе).
type PackItem struct {
	File, Kind, Theme, Name, Topic string
	Size                           int64
}

// PackSource — предмет i манифеста потоком со сверкой размера и sha256 (pack.Archive.Extract).
// Ошибка — архив битый или подменён: набор не ставится целиком.
type PackSource interface {
	Extract(i int, w io.Writer) error
}

// libBytes — байты библиотеки (свои + наборы, ключ считается один раз); под mu. Насыщается
// за пределом: реестры правят руками.
func (s *Store) libBytes() int64 {
	seen := map[string]bool{}
	var total int64
	add := func(kind, key string, n int64) {
		if !seen[kind+key] {
			seen[kind+key] = true
			total += min(max(n, 0), MaxLibraryLen+1-total)
		}
	}
	for _, e := range s.entries {
		add(e.Kind, e.Key, e.Bytes)
	}
	for _, r := range s.packs {
		for _, e := range r.Items {
			add(e.Kind, e.Key, e.Bytes)
		}
	}
	return total
}

// packStep — крючок теста атомарности: сбой на предмете i (в работе — никогда).
var packStep = func(int) error { return nil }

// InstallPack записывает проверенный набор (design §4.3): файлы под именами от ключей (файл с
// тем же ключом есть — байты те же, временный удаляется), последним — реестр Наборы/<id>.json
// (точка фиксации). Сбой до неё — новые файлы удаляются, библиотека прежняя. Обновление —
// ключи прежней версии, которых нет в новой, уходят сборкой мусора; keep — нужны открытому
// альбому. Квота сверяется заново (ErrLibFull). Временные файлы st после успеха пусты; Close
// всё равно нужен.
func (s *Store) InstallPack(st *Staged, keep func(key string) bool) (PackInfo, error) {
	if st == nil || len(st.items) == 0 {
		return PackInfo{}, ErrPackEmpty
	}
	s.ioMu.Lock()
	defer s.ioMu.Unlock()
	old, err := s.installGate(st, keep)
	if err != nil {
		return PackInfo{}, err
	}
	reg := st.reg(nowMs())
	if old != nil {
		reg.Installed, reg.Base = old.Installed, old.Base // обновление остаётся на своём месте и хранит границу
		if old.Version == st.m.Version && old.Zip != "" && old.Zip != st.zip {
			// Архив подписан, но издатель перепаковал ту же версию: старые ключи уйдут уборкой — след для разбора.
			log.Printf("библиотека: набор %q: та же версия %d, другой архив", st.m.ID, st.m.Version)
		}
	}
	placed, created, err := s.placeItems(st)
	if err == nil {
		err = s.writePackReg(reg)
	}
	if err != nil {
		for _, p := range created {
			_ = os.Remove(p)
		}
		return PackInfo{}, errors.New("набор не записался: " + paths.NoPath(err).Error())
	}
	s.mu.Lock()
	for i, it := range st.items {
		s.indexFile(it.kind, it.key, placed[i])
		if it.kind == KindFrame {
			s.frame.put(it.key, it.win)
		}
	}
	if old != nil {
		s.packs[slices.Index(s.packs, old)] = reg
	} else {
		s.packs = append(s.packs, reg)
	}
	s.rebuildThemes()
	s.mu.Unlock()
	if old != nil {
		have := map[string]bool{}
		for _, e := range reg.Items {
			have[e.Kind+e.Key] = true
		}
		gone := slices.DeleteFunc(slices.Clone(old.Items), func(e packEntry) bool { return have[e.Kind+e.Key] })
		s.collect(gone, keep, nil, false)
	}
	return reg.info(), nil
}

// installGate — проверки перед записью набора под ioMu: чтение, квота (повторная: между StagePack и
// «Заменить?» могли пройти минуты, а своё и другие наборы — встать за это время) и версия. Возвращает
// стоящий набор с тем же id (nil — нового).
func (s *Store) installGate(st *Staged, keep func(key string) bool) (*packReg, error) {
	s.mu.RLock()
	ro, old, used := s.readOnly, s.packOf(st.m.ID), s.libBytes()
	var need int64 // только новые файлы: тот же ключ на диске placeItems не пишет
	for _, it := range st.items {
		if _, ok := s.files[it.kind][it.key]; !ok {
			need += it.bytes
		}
	}
	freed := s.freedKeys(old, st) // старые ключи, которые уборка удалит после обновления
	s.mu.RUnlock()
	if ro {
		return nil, ErrReadOnly
	}
	for _, e := range freed { // keep — функция App со своими замками: зовётся вне mu
		if keep == nil || !keep(e.Key) {
			used -= e.Bytes
		}
	}
	if used+need > MaxLibraryLen {
		return nil, ErrLibFull
	}
	if old != nil && old.Version == st.m.Version && !st.refresh {
		return nil, errPackSame
	}
	if old != nil && old.Version > st.m.Version {
		return nil, errPackOlder
	}
	return old, nil
}

// freedKeys — записи прежней версии old, которых нет в новой (st) и которых не держит ни своё, ни
// другой набор: ровно то, что удалит уборка (кроме ключей открытого альбома — их отсеивает keep).
// Ключ один раз; под mu.
func (s *Store) freedKeys(old *packReg, st *Staged) []packEntry {
	if old == nil {
		return nil
	}
	have := map[string]bool{}
	for _, it := range st.items {
		have[it.kind+it.key] = true
	}
	var out []packEntry
	for _, e := range old.Items {
		if have[e.Kind+e.Key] || s.heldBy(e.Key, old) {
			continue
		}
		have[e.Kind+e.Key] = true // повтор ключа в реестре считается один раз, как в libBytes
		out = append(out, e)
	}
	return out
}

// placeItems — файлы предметов на места; created — новые (откат), placed[i] — путь предмета i.
func (s *Store) placeItems(st *Staged) (placed, created []string, err error) {
	placed = make([]string, len(st.items))
	for i := range st.items {
		it := &st.items[i]
		if err := packStep(i); err != nil {
			return nil, created, err
		}
		path := s.packPath(it.kind, it.key, it.ext)
		if !library.Under(path, s.root) { // имя от ключа, но проверка дешевле ошибки
			return nil, created, errors.New("недопустимый путь в библиотеке")
		}
		placed[i] = path
		if regularFile(path) {
			if it.tmp != "" {
				_ = os.Remove(it.tmp)
				it.tmp = ""
			}
			continue
		}
		if it.tpl != nil {
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return nil, created, err
			}
			err = library.WriteFileAtomic(path, it.tpl)
		} else {
			err = os.Rename(it.tmp, path)
		}
		if err != nil {
			return nil, created, err
		}
		it.tmp = ""
		created = append(created, path)
	}
	return placed, created, nil
}

// writePackReg — реестр набора атомарно; Raw-поля заполняются из разобранных (прочитанный реестр их
// обнуляет, и без этого запись стёрла бы themes, tv и topic).
func (s *Store) writePackReg(reg *packReg) error {
	reg.syncRaw()
	data, err := json.MarshalIndent(reg, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Join(s.root, packsDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return library.WriteFileAtomic(filepath.Join(dir, reg.ID+".json"), data)
}
