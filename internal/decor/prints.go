package decor

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Свои типографии (123 Б1, design §4): вид print, файл Типографии/<16 hex>.saprint — профиль JSON.
// Decor хранит байты и ничего не знает о формате: разбор и проверку делает printprof в App.
// Ключ случайный (не хэш): «Заменить» меняет содержимое под тем же ключом.

// printExt — расширение файла типографии.
const printExt = "saprint"

// MaxPrintFile — предел файла типографии, байт (printprof.MaxSize; decor пакет профилей не тянет).
const MaxPrintFile = 64 << 10

// MaxPrints — своих типографий (printprof.MaxOwn).
const MaxPrints = 64

// ErrPrintLimit — своих уже MaxPrints.
var ErrPrintLimit = fmt.Errorf("своих типографий уже %d — удалите ненужные", MaxPrints)

// PrintFile — своя типография из хранилища: ключ, название и байты профиля.
type PrintFile struct {
	Key  string
	Name string
	Data []byte
}

// Prints — свои типографии по порядку добавления. Нечитаемый, пустой или слишком большой файл
// пропускается: профиль проверяет вызывающий, а общий размер читаемого ограничен MaxPrints × MaxPrintFile.
func (s *Store) Prints() []PrintFile {
	s.mu.RLock()
	type ref struct{ key, name, path string }
	var refs []ref
	for _, e := range s.entries {
		if e.Kind != KindPrint || e.Hidden {
			continue
		}
		if path, ok := s.files[KindPrint][e.Key]; ok {
			refs = append(refs, ref{e.Key, e.Name, path})
		}
	}
	s.mu.RUnlock()
	var out []PrintFile
	for _, r := range refs {
		if len(out) >= MaxPrints {
			break
		}
		data, err := readCapped(r.path, MaxPrintFile)
		if err != nil || len(data) == 0 {
			continue
		}
		out = append(out, PrintFile{Key: r.key, Name: r.name, Data: data})
	}
	return out
}

// SavePrint пишет профиль data под названием name: key "" — новый файл, иначе замена содержимого
// под тем же ключом (импорт «Заменить»). Размер и число проверяются до записи.
func (s *Store) SavePrint(key, name string, data []byte) (string, error) {
	if len(data) == 0 || len(data) > MaxPrintFile {
		return "", errors.New("файл типографии пуст или больше 64 КБ")
	}
	if name = cleanName(name); name == "" {
		return "", ErrName
	}
	s.ioMu.Lock()
	defer s.ioMu.Unlock()
	if s.ReadOnly() {
		return "", ErrReadOnly
	}
	s.mu.RLock()
	e := s.entryOf(KindPrint, key)
	n, _ := s.usage(KindPrint)
	s.mu.RUnlock()
	switch {
	case key != "" && (e == nil || !model.ValidLibKey(key)):
		return "", ErrNotFound
	case e == nil && n >= MaxPrints:
		return "", ErrPrintLimit
	}
	if key == "" {
		var b [8]byte
		if _, err := rand.Read(b[:]); err != nil {
			return "", err
		}
		key = libPrefix + hex.EncodeToString(b[:])
	}
	final := filepath.Join(s.root, kindDirs[KindPrint], strings.TrimPrefix(key, libPrefix)+"."+printExt)
	if err := writeTpl(s.root, final, data); err != nil {
		return "", err
	}
	s.mu.Lock()
	if s.files[KindPrint] == nil {
		s.files[KindPrint] = map[string]string{}
	}
	s.files[KindPrint][key] = final
	if e == nil {
		e = &entry{Key: key, Kind: KindPrint, Theme: ThemeMine}
		s.entries = append(s.entries, e)
	}
	e.Name, e.Ext, e.Bytes, e.Hidden, e.Added = name, printExt, int64(len(data)), false, nowMs()
	s.mu.Unlock()
	s.saveLogged()
	return key, nil
}

// DeletePrint удаляет свою типографию: файл и запись. Нет такой — ErrNotFound.
func (s *Store) DeletePrint(key string) error {
	s.ioMu.Lock()
	defer s.ioMu.Unlock()
	if s.ReadOnly() {
		return ErrReadOnly
	}
	s.mu.Lock()
	e := s.entryOf(KindPrint, key)
	path := s.files[KindPrint][key]
	if e == nil {
		s.mu.Unlock()
		return ErrNotFound
	}
	s.entries = removeEntry(s.entries, e)
	delete(s.files[KindPrint], key)
	s.mu.Unlock()
	if path != "" {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return errors.New("файл типографии не удалён")
		}
	}
	s.saveLogged()
	return nil
}

func removeEntry(list []*entry, e *entry) []*entry {
	out := list[:0:0]
	for _, x := range list {
		if x != e {
			out = append(out, x)
		}
	}
	return out
}
