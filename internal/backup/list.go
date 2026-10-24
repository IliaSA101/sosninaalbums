package backup

import (
	"cmp"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"time"
)

// Entry — строка списка копий.
type Entry struct {
	Name    string
	Time    time.Time
	Size    int64
	Spreads int
	Photos  int
}

// copyFile — файл копии в папке альбома. raw — время из имени; t — оно же, но не позже now:
// копия «из будущего» (часы шли вперёд) иначе вечно считалась бы самой свежей.
type copyFile struct {
	name string
	raw  time.Time
	t    time.Time
	seq  int
	size int64
}

// tmpRe — остаток записи library.WriteFileAtomic (CreateTemp: <имя>.<число>.tmp).
var tmpRe = regexp.MustCompile(`^\d{8}-\d{6}(?:-\d{1,3})?\.salbum\.\d+\.tmp$`)

// copies — копии в папке альбома, новые (по имени) первыми. Только обычные файлы с именем по
// шаблону: ссылки и чужие файлы не читаются.
func copies(dir string, now time.Time) ([]copyFile, error) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []copyFile
	for _, e := range ents {
		m := nameRe.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		st, err := os.Lstat(filepath.Join(dir, e.Name()))
		if err != nil || !st.Mode().IsRegular() {
			continue
		}
		t, err := time.ParseInLocation(timeLayout, m[1], time.Local)
		if err != nil {
			continue
		}
		seq := 1
		if m[2] != "" {
			seq, _ = strconv.Atoi(m[2])
		}
		c := copyFile{name: e.Name(), raw: t, t: t, seq: seq, size: st.Size()}
		if c.t.After(now) {
			c.t = now
		}
		out = append(out, c)
	}
	// Имена по строкам не сортируются: «…-2.salbum» меньше «….salbum», а она новее.
	slices.SortFunc(out, func(a, b copyFile) int {
		if c := b.raw.Compare(a.raw); c != 0 {
			return c
		}
		return cmp.Compare(b.seq, a.seq)
	})
	return out, nil
}

// List — копии альбома key, новые первыми; папки нет — пусто. Развороты и фото — лёгким
// разбором JSON, без salbum.Load: тот трогал бы фото на диске.
func (s *Store) List(key string) ([]Entry, error) {
	dir := s.Dir(key)
	if dir == "" {
		return []Entry{}, nil
	}
	if _, err := os.Lstat(dir); errors.Is(err, fs.ErrNotExist) {
		return []Entry{}, nil
	}
	if !plainDir(s.Root) || !plainDir(dir) {
		return []Entry{}, nil
	}
	cs, err := copies(dir, s.now())
	if err != nil {
		return nil, err
	}
	out := make([]Entry, 0, len(cs))
	for _, c := range cs {
		e := Entry{Name: c.name, Time: c.t, Size: c.size}
		if data, err := readLimited(filepath.Join(dir, c.name)); err == nil {
			sm := summarize(data)
			e.Spreads, e.Photos = sm.spreads, sm.photos
		}
		out = append(out, e)
	}
	return out, nil
}

type summary struct{ spreads, photos, folders int }

func summarize(data []byte) summary {
	var head struct {
		Spreads []json.RawMessage          `json:"spreads"`
		Photos  map[string]json.RawMessage `json:"photos"`
		Folders []json.RawMessage          `json:"folders"`
	}
	if json.Unmarshal(data, &head) != nil {
		return summary{}
	}
	return summary{len(head.Spreads), len(head.Photos), len(head.Folders)}
}

// HasContent — в файле альбома есть развороты или папки: пустой черновик копировать незачем.
func HasContent(path string) bool {
	data, err := readLimited(path)
	if err != nil {
		return false
	}
	sm := summarize(data)
	return sm.spreads > 0 || sm.folders > 0
}

// trim — пределы папки альбома: не больше Keep копий и albumMax байт, давние удаляются первыми.
// Не трогает самую новую копию и protect (только что записанную: при часах, ушедших назад, она
// по имени старше прежних; и восстанавливаемую). Заодно — остатки оборванной записи старше tmpAge.
func (s *Store) trim(dir string, now time.Time, protect ...string) error {
	cs, err := copies(dir, now)
	if err != nil {
		return err
	}
	n, total := len(cs), int64(0)
	for _, c := range cs {
		total += c.size
	}
	var first error
	for i := len(cs) - 1; i > 0 && (n > Keep || total > s.albumMax); i-- {
		if slices.Contains(protect, cs[i].name) {
			continue
		}
		if err := os.Remove(filepath.Join(dir, cs[i].name)); err != nil {
			if first == nil {
				first = err
			}
			continue
		}
		n--
		total -= cs[i].size
	}
	dropTmp(dir, now)
	return first
}

// dropTmp удаляет остатки library.WriteFileAtomicMode старше tmpAge (выход или сбой посреди
// записи): свежий может быть чужой записью в эту секунду — его не трогаем.
func dropTmp(dir string, now time.Time) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range ents {
		if !tmpRe.MatchString(e.Name()) {
			continue
		}
		p := filepath.Join(dir, e.Name())
		if st, err := os.Lstat(p); err == nil && st.Mode().IsRegular() && now.Sub(st.ModTime()) > tmpAge {
			_ = os.Remove(p) // не вышло — попробуем при следующей записи
		}
	}
}

// album — папка копий альбома для Prune.
type album struct {
	dir    string
	newest time.Time
	size   int64
}

// Prune — общий предел: копии старше MaxAge удаляются, затем самые давние альбомы — пока
// папок больше MaxDirs или всё вместе больше MaxBytes (самый свежий альбом остаётся всегда).
// Часы отстают (now раньше самой новой копии) — по возрасту не удаляем: «старыми» оказались бы
// и свежие копии.
// Трогает только прямые подпапки Root — обычные папки, не ссылки и не junction — и в них
// только файлы копий; чужие файлы и папки остаются.
func (s *Store) Prune() error {
	if s == nil || s.Root == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !plainDir(s.Root) {
		return nil // папки ещё нет (или это не наша папка)
	}
	ents, err := os.ReadDir(s.Root)
	if err != nil {
		return err
	}
	now := s.now()
	type found struct {
		dir string
		cs  []copyFile
	}
	var dirs []found
	var newest time.Time
	for _, e := range ents {
		dir := filepath.Join(s.Root, e.Name())
		if !plainDir(dir) {
			continue
		}
		cs, err := copies(dir, now)
		if err != nil {
			continue
		}
		if len(cs) > 0 {
			newest = maxTime(newest, cs[0].raw)
		}
		dirs = append(dirs, found{dir, cs})
	}
	byAge := !now.Before(newest)
	old := now.Add(-MaxAge)
	var albums []album
	var total int64
	for _, f := range dirs {
		dir, cs := f.dir, f.cs
		a := album{dir: dir}
		for _, c := range cs {
			if byAge && c.t.Before(old) {
				_ = os.Remove(filepath.Join(dir, c.name)) // не вышло — попробуем при следующем запуске
				continue
			}
			a.newest = maxTime(a.newest, c.t)
			a.size += c.size
		}
		if a.newest.IsZero() {
			removeAlbum(dir) // копий не осталось; чужое в папке её удержит
			continue
		}
		albums = append(albums, a)
		total += a.size
	}
	slices.SortFunc(albums, func(a, b album) int { return a.newest.Compare(b.newest) })
	for len(albums) > 1 && (len(albums) > MaxDirs || total > MaxBytes) {
		removeAlbum(albums[0].dir)
		total -= albums[0].size
		albums = albums[1:]
	}
	return nil
}

func maxTime(a, b time.Time) time.Time {
	if b.After(a) {
		return b
	}
	return a
}

// removeAlbum удаляет из папки альбома файлы копий и остатки их записи, затем саму папку,
// если она опустела. RemoveAll не зовём: в папке может оказаться ссылка на чужое.
func removeAlbum(dir string) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range ents {
		if !nameRe.MatchString(e.Name()) && !tmpRe.MatchString(e.Name()) {
			continue
		}
		p := filepath.Join(dir, e.Name())
		if st, err := os.Lstat(p); err == nil && st.Mode().IsRegular() {
			_ = os.Remove(p) // не вышло — останется до следующей чистки
		}
	}
	_ = os.Remove(dir) // непустая (чужие файлы) — остаётся
}
