package fontlib

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/paths"
	"github.com/IliaSA101/sosninaalbums/internal/text"
)

// Skip — пропущенный файл и причина для тоста.
type Skip struct {
	Name string `json:"name"`
	Why  string `json:"why"`
}

// Result — итог загрузки: гарнитуры, которые добавились или вернулись из скрытых (целиком, со
// всеми начертаниями), ключи новых файлов «<Key>.<ext>» (окно проверяет, что открывает их) и
// пропущенные файлы. Срезы не nil.
type Result struct {
	Added   []text.CustomFamily
	Faces   []string
	Skipped []Skip
}

func newResult() Result {
	return Result{Added: []text.CustomFamily{}, Faces: []string{}, Skipped: []Skip{}}
}

// Причины пропуска.
var (
	errHave  = errors.New("уже есть в библиотеке")
	errQuiet = errors.New("без сообщения") // папка альбома: ненужное и повторы молча
)

// cand — проверенный файл, ещё не в библиотеке.
type cand struct {
	data   []byte
	info   text.FaceInfo
	key    string
	family string
	name   string
}

// inspectFont — полная проверка файла; подменяется в тестах (счёт вызовов, замок во время проверки).
var inspectFont = text.Inspect

// albumBytes — сколько байт всего читается из папки «Шрифты» чужого альбома (аудит 082): 64
// файла по 32 МБ — это 2 ГБ чтения при открытии альбома. Переменная — для тестов.
var albumBytes int64 = 256 << 20

// readBytes читает файл шрифта без проверки. Ошибка — причина для тоста (без пути).
func readBytes(path string) ([]byte, error) {
	if paths.CheckLocalPath(path) != nil {
		return nil, errors.New("файл не на локальном диске")
	}
	b, err := readCapped(path, MaxFontBytes)
	if errors.Is(err, errTooBig) {
		return nil, text.ErrFontTooBig
	}
	if err != nil {
		log.Printf("шрифты: файл не прочитан: %v", paths.NoPath(err))
		return nil, errors.New("файл не читается")
	}
	return b, nil
}

// baseName — имя файла без пути и расширения, одной строкой.
func baseName(path string) string {
	return oneLine(strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)))
}

// familyOr — гарнитура для библиотеки: из файла, без имени в файле — имя файла.
func familyOr(fam, name string) string {
	if f := oneLine(fam); f != "" {
		return f
	}
	if name != "" {
		return name
	}
	return "Шрифт"
}

// checkFont проверяет прочитанный файл. Зовётся без ioMu: Inspect недоверенного файла может
// идти секунды, а замок нужен открытию альбома и окну шрифтов.
func checkFont(b []byte, path string) (cand, error) {
	info, err := inspectFont(b)
	if err != nil {
		return cand{}, err
	}
	name := baseName(path)
	return cand{data: b, info: info, key: keyOf(b), family: familyOr(info.Family, name), name: name}, nil
}

// readFont читает и проверяет один файл.
func readFont(path string) (cand, error) {
	b, err := readBytes(path)
	if err != nil {
		return cand{}, err
	}
	return checkFont(b, path)
}

// addLocked — add под ioMu; проверка файла уже позади.
func (s *Store) addLocked(c cand, want map[string]bool) (string, bool, error) {
	s.ioMu.Lock()
	defer s.ioMu.Unlock()
	return s.add(c, want)
}

// prepare — начало загрузки под ioMu: библиотека открыта на запись, папка есть.
func (s *Store) prepare() error {
	s.ioMu.Lock()
	defer s.ioMu.Unlock()
	if s.ReadOnly() {
		return ErrReadOnly
	}
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return errors.New("папка шрифтов не создаётся: " + paths.NoPath(err).Error())
	}
	return nil
}

// displayName — имя файла для тоста: только последнее звено пути.
func displayName(p string) string {
	n := oneLine(filepath.Base(p))
	if n == "" || n == "." {
		return "файл"
	}
	return n
}

func styleName(bold, italic bool) string {
	switch {
	case bold && italic:
		return "жирный курсив"
	case bold:
		return "жирный"
	case italic:
		return "курсив"
	}
	return "обычный"
}

// Import копирует файлы в библиотеку. Пути — только из диалога App или ImportFontPaths.
// Ошибка — когда библиотека закрыта целиком (ReadOnly) или папка не создаётся; пропуск отдельных
// файлов с причиной — в Result.Skipped.
func (s *Store) Import(files []string) (Result, error) {
	res := newResult()
	if err := s.prepare(); err != nil {
		return res, err
	}
	var ids []string
	for i, p := range files {
		name := displayName(p)
		if i >= MaxImport {
			res.Skipped = append(res.Skipped, Skip{name, fmt.Sprintf("за раз — не больше %d файлов", MaxImport)})
			continue
		}
		c, err := readFont(p)
		if err == nil {
			var id string
			var written bool
			if id, written, err = s.addLocked(c, nil); err == nil {
				ids = appendOnce(ids, id)
				if written {
					res.Faces = append(res.Faces, c.key+"."+c.info.Ext)
				}
			}
		}
		if err != nil {
			res.Skipped = append(res.Skipped, Skip{name, err.Error()})
		}
	}
	s.ioMu.Lock()
	defer s.ioMu.Unlock()
	s.finish(&res, ids)
	return res, nil
}

// ImportFolder — шрифты папки «Шрифты» собранного альбома (082-8): только файлы самой папки,
// *.ttf и *.otf, не больше MaxAlbumFiles; в библиотеку попадает лишь начертание, чей
// FamilyID(Family) есть в want (иначе ни копии, ни записи). Доступность тома (osx.Reachable)
// проверяет App до вызова. Итог — без «уже есть» и без ненужных файлов.
func (s *Store) ImportFolder(dir string, want map[string]bool) (Result, error) {
	res := newResult()
	if len(want) == 0 {
		return res, nil
	}
	if err := paths.CheckLocalPath(dir); err != nil {
		return res, paths.NoPath(err)
	}
	list, err := os.ReadDir(dir)
	if err != nil {
		log.Printf("шрифты: папка альбома не читается: %v", paths.NoPath(err))
		return res, errors.New("папка шрифтов не читается")
	}
	var files []os.DirEntry
	for _, de := range list {
		ext := strings.ToLower(filepath.Ext(de.Name()))
		if de.Type().IsRegular() && (ext == ".ttf" || ext == ".otf") { // вложенные папки и ссылки — нет
			files = append(files, de)
		}
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Name() < files[j].Name() })
	if len(files) > MaxAlbumFiles {
		log.Printf("шрифты: в папке альбома %d файлов — берутся первые %d", len(files), MaxAlbumFiles)
		files = files[:MaxAlbumFiles]
	}
	if s.ReadOnly() {
		return res, ErrReadOnly
	}
	if len(files) == 0 {
		return res, nil
	}
	if err := s.prepare(); err != nil {
		return res, err
	}
	var ids []string
	var used int64
	for i, de := range files {
		n := de.Name()
		if fi, err := de.Info(); err == nil && used+fi.Size() > albumBytes {
			log.Printf("шрифты: папка альбома больше %d МБ — пропущено файлов: %d", albumBytes>>20, len(files)-i)
			break
		}
		path := filepath.Join(dir, n)
		b, err := readBytes(path)
		used += int64(len(b))
		var c cand
		if err == nil {
			c, err = wanted(b, path, want)
		}
		if err == nil {
			var id string
			var written bool
			if id, written, err = s.addLocked(c, want); err == nil {
				ids = appendOnce(ids, id)
				if written {
					res.Faces = append(res.Faces, c.key+"."+c.info.Ext)
				}
			}
		}
		if err != nil && !errors.Is(err, errQuiet) {
			res.Skipped = append(res.Skipped, Skip{displayName(n), err.Error()})
		}
	}
	s.ioMu.Lock()
	defer s.ioMu.Unlock()
	s.finish(&res, ids)
	return res, nil
}

// wanted проверяет файл папки альбома, только если его гарнитура в want: имя берётся из одной
// таблицы name, полный Inspect — лишь для нужных (аудит 082). Неразобранный файл проверяется
// целиком: Inspect откажет до обхода глифов и назовёт причину для тоста.
func wanted(b []byte, path string, want map[string]bool) (cand, error) {
	if fam, ok := text.PeekFamily(b); ok && !want[text.FamilyID(familyOr(fam, baseName(path)))] {
		return cand{}, errQuiet
	}
	return checkFont(b, path)
}

func appendOnce(list []string, v string) []string {
	for _, o := range list {
		if o == v {
			return list
		}
	}
	return append(list, v)
}

// finish записывает реестр и заполняет Added гарнитурами ids. Ошибка записи — в журнал: файлы
// на месте, гарнитуры в памяти есть до перезапуска.
func (s *Store) finish(res *Result, ids []string) {
	if len(ids) == 0 {
		return
	}
	if err := s.save(); err != nil {
		log.Printf("шрифты: реестр не записан: %v", paths.NoPath(err))
	}
	for _, f := range s.Families() {
		for _, id := range ids {
			if f.ID == id {
				res.Added = append(res.Added, f)
			}
		}
	}
}

// add кладёт проверенный файл в библиотеку (под ioMu). id — гарнитура, written — файл записан
// заново (а не только возвращён из скрытых). want != nil — режим папки альбома: ненужные
// гарнитуры и повторы возвращают errQuiet.
func (s *Store) add(c cand, want map[string]bool) (id string, written bool, err error) {
	id = text.FamilyID(c.family)
	if want != nil && !want[id] {
		return id, false, errQuiet
	}
	s.mu.RLock()
	var same, clash *entry
	var total int64
	for _, e := range s.entries {
		total += e.Bytes
		switch {
		case e.Key == c.key:
			same = e
		case text.FamilyID(e.Family) == id && e.Bold == c.info.Bold && e.Italic == c.info.Italic:
			clash = e
		}
	}
	n := len(s.entries)
	s.mu.RUnlock()

	final := s.path(c.key, c.info.Ext)
	if same != nil {
		if st, serr := os.Lstat(final); serr == nil && st.Mode().IsRegular() { // файл на месте
			if !same.Hidden {
				return id, false, quiet(want, errHave)
			}
			s.reveal(id)
			return id, false, nil
		}
	}
	if clash != nil && same == nil {
		if !clash.Hidden {
			return id, false, quiet(want, fmt.Errorf("уже есть „%s“ (%s) — сначала уберите прежний", c.family, styleName(c.info.Bold, c.info.Italic)))
		}
		n-- // скрытый с тем же начертанием заменяется
		total -= clash.Bytes
	}
	if same == nil {
		if n >= maxFonts {
			return id, false, fmt.Errorf("шрифтов уже %d — больше не помещается", maxFonts)
		}
		if total+int64(len(c.data)) > maxFontsBytes {
			return id, false, fmt.Errorf("библиотека шрифтов заполнена (%d МБ)", maxFontsBytes>>20)
		}
	}
	if err := s.writeFile(final, c.data); err != nil {
		log.Printf("шрифты: файл не записан: %v", paths.NoPath(err))
		return id, false, errors.New("не удалось записать файл в библиотеку")
	}
	s.mu.Lock()
	if same == nil {
		if clash != nil { // старый файл остаётся на диске, но из реестра уходит — до отката Drop
			s.entries = deleteEntry(s.entries, clash)
			s.replaced[c.key] = clash
		}
		same = &entry{Key: c.key, Ext: c.info.Ext, Added: nowMs()}
		s.entries = append(s.entries, same)
	}
	same.Family, same.Bold, same.Italic = c.family, c.info.Bold, c.info.Italic
	same.Cyr, same.Restricted, same.Name, same.Bytes = c.info.Cyr, c.info.Restricted, c.name, int64(len(c.data))
	s.fresh[c.key] = true
	s.mu.Unlock()
	s.reveal(id)
	return id, true, nil
}

// quiet — в режиме папки альбома причина не показывается.
func quiet(want map[string]bool, err error) error {
	if want != nil {
		return errQuiet
	}
	return err
}

// reveal возвращает гарнитуру из скрытых целиком: загрузка любого её файла — намерение
// пользователя пользоваться ею.
func (s *Store) reveal(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, e := range s.entries {
		if text.FamilyID(e.Family) == id {
			e.Hidden = false
		}
	}
}

// writeFile — файл библиотеки: временный в той же папке, затем rename в имя от ключа.
func (s *Store) writeFile(final string, data []byte) error {
	f, err := os.CreateTemp(s.dir, tmpPrefix+"*.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	_, err = f.Write(data)
	if err == nil {
		err = f.Sync()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(tmp, final)
	}
	if err != nil {
		_ = os.Remove(tmp)
	}
	return err
}
