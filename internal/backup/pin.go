package backup

// Закрепление копии (149, 0.33-149-19): копия перед правкой агента не должна уйти в чистку до
// выхода из программы. Trim (по Keep и размеру) и Prune (по возрасту, числу и размеру папок)
// закреплённые файлы не удаляют; закрепление живёт в памяти Store, на диск оно не пишется.

import (
	"errors"
	"slices"
)

// ErrNoStore — копий нет: хранилище не создано или у него пустой Root (том не нашёлся).
var ErrNoStore = errors.New("хранилище резервных копий недоступно")

// Pin закрепляет копию name альбома key: имя по шаблону, обычный файл в папке альбома.
func (s *Store) Pin(key, name string) error {
	if s == nil || s.Root == "" {
		return ErrNoStore
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.pinLocked(key, name)
}

// pinLocked — Pin под s.mu.
func (s *Store) pinLocked(key, name string) error {
	if _, err := s.File(key, name); err != nil {
		return err
	}
	if s.pins[key] == nil {
		s.pins[key] = map[string]bool{}
	}
	s.pins[key][name] = true
	return nil
}

// ForcePin — Force (копия без учёта Every) и закрепление: name — копия с содержимым path (новая
// или самая новая, совпавшая с ним байт в байт). Файла path нет — копировать нечего: "" без
// ошибки. Любая причина, по которой копии нет (нет хранилища, ключ, диск), — ошибка: правка агента
// без копии не делается. Ошибка чистки после записи копии не в счёт — копия есть и закреплена.
func (s *Store) ForcePin(path, key string) (name string, err error) {
	if s == nil || s.Root == "" {
		return "", ErrNoStore
	}
	if !validKey(key) {
		return "", ErrNotFound
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	name, err = s.copyLocked(path, key, true, "")
	if name == "" {
		return "", err
	}
	return name, s.pinLocked(key, name)
}

// pinned — закреплённые копии альбома key. Под s.mu.
func (s *Store) pinned(key string) []string {
	out := make([]string, 0, len(s.pins[key]))
	for n := range s.pins[key] {
		out = append(out, n)
	}
	slices.Sort(out)
	return out
}
