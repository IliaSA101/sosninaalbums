// Package fontlib — библиотека своих шрифтов (спека 082, design §4): копии файлов TTF/OTF в
// <папка данных>/Библиотека/Шрифты/<16 hex>.<ttf|otf> и реестр fonts.json — кэш признаков, чтобы
// старт не читал файлы.
//
// Граница безопасности: файл шрифта — недоверенный ввод, его целиком проверяет text.Inspect до
// записи; имя файла в библиотеке строит только код из ключа (первые 8 байт sha256 содержимого),
// при каждом чтении байты сверяются с ключом (подмена файла на диске — отказ). Пути приходят
// только из диалога App, ImportFontPaths и папки «Шрифты» собранного альбома; в ошибках и журнале
// путей нет (paths.NoPath), в отчёт для тоста идёт лишь имя файла.
package fontlib

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/IliaSA101/sosninaalbums/internal/imgfile"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
	"github.com/IliaSA101/sosninaalbums/internal/text"
)

// Пределы (082-12).
const (
	MaxImport     = 100               // файлов за раз
	MaxFonts      = 500               // файлов в библиотеке, со скрытыми
	MaxFontsBytes = 1 << 30           // вся библиотека шрифтов
	MaxFontBytes  = text.MaxFontBytes // один файл
	MaxAlbumFiles = 64                // файлов из папки «Шрифты» собранного альбома
	maxRegSize    = 4 << 20           // 500 записей — десятки КБ; больше — чужой или битый файл
	regFile       = "fonts.json"      // реестр
	regVersion    = 1
	tmpPrefix     = "import-" // временный файл импорта; остатки убирает Open
)

// Пределы, которые тесты сужают, чтобы не создавать сотни шрифтов.
var (
	maxFonts      = MaxFonts
	maxFontsBytes = int64(MaxFontsBytes)
)

// Ошибки для интерфейса.
var (
	ErrReadOnly = errors.New("библиотеку шрифтов изменила более новая версия программы — здесь её можно только смотреть")
	ErrNotFound = errors.New("нет такого шрифта в библиотеке")
	ErrSha      = errors.New("файл шрифта изменён на диске: не совпал с ключом")
	errTooBig   = errors.New("слишком большой файл")
)

// entry — запись fonts.json: признаки файла из text.Inspect.
type entry struct {
	Key        string `json:"key"`
	Ext        string `json:"ext"`
	Family     string `json:"family"`
	Bold       bool   `json:"bold,omitempty"`
	Italic     bool   `json:"italic,omitempty"`
	Cyr        bool   `json:"cyr,omitempty"`
	Restricted bool   `json:"restricted,omitempty"`
	Name       string `json:"name"` // имя исходного файла без расширения — для сообщений
	Bytes      int64  `json:"bytes"`
	Added      int64  `json:"added"` // unix мс
	Hidden     bool   `json:"hidden,omitempty"`
}

type registry struct {
	Version int      `json:"version"`
	Items   []*entry `json:"items"`
}

// Face — начертание гарнитуры для сбора в папку: путь строится только из ключа внутри корня.
type Face struct {
	Key, Ext     string
	Bold, Italic bool
	Restricted   bool
	Path         string
}

// Store — библиотека шрифтов одной папки. Методы безопасны из разных горутин: правки реестра и
// файлов идут по одной (ioMu), индекс читают под RLock.
type Store struct {
	dir string

	ioMu sync.Mutex // запись в библиотеку (Import, ImportFolder), Hide, Drop: по одной; Inspect — без него

	mu       sync.RWMutex
	entries  []*entry
	readOnly bool
	fresh    map[string]bool // ключи, добавленные в этом сеансе: только их снимает Drop
	// replaced — скрытая запись того же начертания, которую заменил свежий ключ: Drop свежего
	// возвращает её (файл заменённого остаётся на диске), иначе откат окном терял бы старое.
	// Пишется и читается под ioMu.
	replaced map[string]*entry
}

// Open открывает библиотеку в папке dir (App передаёт <данные>/Библиотека/Шрифты): убирает
// остатки оборванной записи, читает реестр; нет реестра или он битый — пересобирает по файлам.
// Ошибки чтения не мешают работе (строка в log.txt).
func Open(dir string) *Store {
	s := &Store{dir: dir, fresh: map[string]bool{}, replaced: map[string]*entry{}}
	s.cleanTmp()
	s.load()
	return s
}

// ReadOnly — реестр записан более новой версией: импорт, × и откат закрыты.
func (s *Store) ReadOnly() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.readOnly
}

// cleanTmp убирает остатки оборванного импорта и записи реестра.
func (s *Store) cleanTmp() {
	list, err := os.ReadDir(s.dir)
	if err != nil {
		return
	}
	for _, de := range list {
		n := de.Name()
		if strings.HasSuffix(n, ".tmp") && (strings.HasPrefix(n, tmpPrefix) || strings.HasPrefix(n, regFile+".")) {
			_ = os.Remove(filepath.Join(s.dir, n))
		}
	}
}

// load читает fonts.json. Нет файла или битый (разбор, версия, размер) — пересборка по файлам
// папки (битый уходит в .bad); не читается (доступ, блокировка антивирусом) — реестр не трогаем,
// сеанс только чтение: пересобранное остаётся в памяти. Версия новее — только чтение.
func (s *Store) load() {
	path := filepath.Join(s.dir, regFile)
	data, err := readCapped(path, maxRegSize)
	switch {
	case errors.Is(err, os.ErrNotExist):
		s.rebuild(false)
		return
	case err != nil && !errors.Is(err, errTooBig):
		log.Printf("шрифты: реестр не читается (%v) — только чтение", paths.NoPath(err))
		s.readOnly = true
		s.rebuild(true)
		return
	}
	var reg registry
	if err == nil {
		err = json.Unmarshal(data, &reg)
	}
	if err == nil && reg.Version < 1 {
		err = errors.New("нет версии")
	}
	if err != nil {
		log.Printf("шрифты: реестр не читается (%v), отложен в fonts.json.bad", paths.NoPath(err))
		if rerr := os.Rename(path, path+".bad"); rerr != nil {
			log.Printf("шрифты: %v", paths.NoPath(rerr))
		}
		s.rebuild(false)
		return
	}
	if reg.Version > regVersion {
		log.Printf("шрифты: реестр версии %d — только чтение", reg.Version)
		s.readOnly = true
	}
	dirty := false
	for _, e := range reg.Items {
		if len(s.entries) >= MaxFonts {
			dirty = true
			break
		}
		if !s.accept(e) {
			dirty = true
			continue
		}
		s.entries = append(s.entries, e)
	}
	if dirty && !s.readOnly { // записи без файла выброшены
		if err := s.save(); err != nil {
			log.Printf("шрифты: реестр не записан: %v", paths.NoPath(err))
		}
	}
}

// accept проверяет запись реестра (его правят руками): ключ, расширение, имя, файл на диске,
// повтор ключа и начертания. Годная запись чистится на месте.
func (s *Store) accept(e *entry) bool {
	if e == nil || !validKey(e.Key) || (e.Ext != "ttf" && e.Ext != "otf") {
		return false
	}
	if e.Family = oneLine(e.Family); e.Family == "" {
		return false
	}
	e.Name = oneLine(e.Name)
	e.Bytes = min(max(e.Bytes, 0), MaxFontBytes)
	if st, err := os.Lstat(s.path(e.Key, e.Ext)); err != nil || !st.Mode().IsRegular() {
		return false
	}
	id := text.FamilyID(e.Family)
	for _, o := range s.entries {
		if o.Key == e.Key || text.FamilyID(o.Family) == id && o.Bold == e.Bold && o.Italic == e.Italic {
			return false
		}
	}
	return true
}

// path — файл начертания: только от проверенного ключа и расширения, внутри корня.
func (s *Store) path(key, ext string) string { return filepath.Join(s.dir, key+"."+ext) }

// validKey — 16 знаков hex в нижнем регистре.
func validKey(k string) bool { return text.IsCustomID("my:" + k) }

// oneLine — имя одной строкой без лишних пробелов, не длиннее 60 знаков.
func oneLine(s string) string {
	s = strings.Join(strings.Fields(text.CleanText(s)), " ")
	if r := []rune(s); len(r) > 60 {
		s = strings.TrimSpace(string(r[:60]))
	}
	return s
}

// readCapped читает обычный файл не больше limit байт (больше — errTooBig).
func readCapped(path string, limit int64) ([]byte, error) {
	f, err := imgfile.OpenRegular(path)
	if err != nil {
		return nil, err
	}
	defer f.Close() //nolint:errcheck // чтение: ошибка закрытия ничего не теряет
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err == nil && int64(len(data)) > limit {
		err = errTooBig
	}
	return data, err
}

// save пишет реестр атомарно. Зовётся под ioMu.
func (s *Store) save() error {
	s.mu.RLock()
	data, err := json.MarshalIndent(registry{Version: regVersion, Items: s.entries}, "", "  ")
	s.mu.RUnlock()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return err
	}
	return writeAtomic(filepath.Join(s.dir, regFile), data)
}

// writeAtomic — tmp в той же папке + rename: оборванная запись не оставляет полфайла.
func writeAtomic(path string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
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
		err = os.Rename(tmp, path)
	}
	if err != nil {
		_ = os.Remove(tmp) // остаток убирает следующий Open
	}
	return err
}

// styleRank — порядок начертаний в гарнитуре: обычный, жирный, курсив, жирный курсив.
func styleRank(bold, italic bool) int {
	r := 0
	if bold {
		r++
	}
	if italic {
		r += 2
	}
	return r
}

// buildFamilies — гарнитуры по записям: имя берётся у первой записи, скрыта — если скрыты все
// начертания, «без кириллицы» — если её нет ни в одном. Порядок — по имени без регистра.
func buildFamilies(entries []*entry) []text.CustomFamily {
	byID := map[string]*text.CustomFamily{}
	var order []string
	for _, e := range entries {
		id := text.FamilyID(e.Family)
		f := byID[id]
		if f == nil {
			f = &text.CustomFamily{ID: id, Name: e.Family, Hidden: true, NoCyr: true}
			byID[id] = f
			order = append(order, id)
		}
		f.Faces = append(f.Faces, text.CustomFace{Key: e.Key, Ext: e.Ext, Bold: e.Bold, Italic: e.Italic})
		f.Hidden = f.Hidden && e.Hidden
		f.NoCyr = f.NoCyr && !e.Cyr
	}
	out := make([]text.CustomFamily, 0, len(order))
	for _, id := range order {
		f := byID[id]
		sort.SliceStable(f.Faces, func(i, j int) bool {
			return styleRank(f.Faces[i].Bold, f.Faces[i].Italic) < styleRank(f.Faces[j].Bold, f.Faces[j].Italic)
		})
		out = append(out, *f)
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := strings.ToLower(out[i].Name), strings.ToLower(out[j].Name)
		if a != b {
			return a < b
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// Families — все свои гарнитуры, скрытые с Hidden (скрытая остаётся зарегистрированной:
// тексты альбомов рисуются ею же).
func (s *Store) Families() []text.CustomFamily {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return buildFamilies(s.entries)
}

// Faces — начертания гарнитуры для сбора в папку; нет такой — nil.
func (s *Store) Faces(familyID string) []Face {
	if !text.IsCustomID(familyID) {
		return nil
	}
	s.mu.RLock()
	var out []Face
	for _, e := range s.entries {
		if text.FamilyID(e.Family) == familyID {
			out = append(out, Face{Key: e.Key, Ext: e.Ext, Bold: e.Bold, Italic: e.Italic,
				Restricted: e.Restricted, Path: s.path(e.Key, e.Ext)})
		}
	}
	s.mu.RUnlock()
	sort.SliceStable(out, func(i, j int) bool {
		return styleRank(out[i].Bold, out[i].Italic) < styleRank(out[j].Bold, out[j].Italic)
	})
	return out
}

// Load — байты файла по ключу (загрузчик для text.SetCustom). Читает только файл записи
// реестра и сверяет sha256 с ключом: подмена на диске — ErrSha.
func (s *Store) Load(key string) ([]byte, error) {
	if !validKey(key) {
		return nil, ErrNotFound
	}
	s.mu.RLock()
	ext := ""
	for _, e := range s.entries {
		if e.Key == key {
			ext = e.Ext
			break
		}
	}
	s.mu.RUnlock()
	if ext == "" {
		return nil, ErrNotFound
	}
	b, err := readCapped(s.path(key, ext), MaxFontBytes)
	if errors.Is(err, errTooBig) {
		return nil, text.ErrFontTooBig
	}
	if err != nil {
		return nil, paths.NoPath(err)
	}
	if keyOf(b) != key {
		return nil, ErrSha
	}
	return b, nil
}

// keyOf — ключ файла: первые 8 байт sha256 в hex.
func keyOf(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:8])
}

// Hide — × гарнитуры: все начертания скрыты, файлы и тексты альбомов остаются; загрузка того
// же файла возвращает гарнитуру. Не записался реестр — правка откатывается.
func (s *Store) Hide(familyID string) error {
	if !text.IsCustomID(familyID) {
		return ErrNotFound
	}
	s.ioMu.Lock()
	defer s.ioMu.Unlock()
	if s.ReadOnly() {
		return ErrReadOnly
	}
	s.mu.Lock()
	var changed []*entry
	found := false
	for _, e := range s.entries {
		if text.FamilyID(e.Family) == familyID {
			found = true
			if !e.Hidden {
				e.Hidden = true
				changed = append(changed, e)
			}
		}
	}
	s.mu.Unlock()
	if !found {
		return ErrNotFound
	}
	if len(changed) == 0 {
		return nil
	}
	if err := s.save(); err != nil {
		s.mu.Lock()
		for _, e := range changed {
			e.Hidden = false
		}
		s.mu.Unlock()
		return errors.New("библиотека шрифтов не записалась: " + paths.NoPath(err).Error())
	}
	return nil
}

// Drop — откат начертаний, которые не открыло окно программы (082-14): удаляет файл и запись
// только у ключей, добавленных в этом сеансе; прочие пропускает. Ключ — «<16 hex>» или
// «<16 hex>.<ttf|otf>»; удалённые возвращаются в той же записи, что пришли.
func (s *Store) Drop(keys []string) ([]string, error) {
	s.ioMu.Lock()
	defer s.ioMu.Unlock()
	if s.ReadOnly() {
		return nil, ErrReadOnly
	}
	removed := []string{}
	for _, k := range keys {
		key, _, _ := strings.Cut(k, ".")
		if !validKey(key) {
			continue
		}
		s.mu.RLock()
		var e *entry
		if s.fresh[key] {
			for _, o := range s.entries {
				if o.Key == key {
					e = o
					break
				}
			}
		}
		s.mu.RUnlock()
		if e == nil || k != key && k != key+"."+e.Ext {
			continue
		}
		if err := os.Remove(s.path(key, e.Ext)); err != nil && !errors.Is(err, os.ErrNotExist) {
			log.Printf("шрифты: откат начертания не удался: %v", paths.NoPath(err))
			continue
		}
		old := s.replaced[key]
		back := old != nil && s.fileHere(old)
		s.mu.Lock()
		s.entries = deleteEntry(s.entries, e)
		if back {
			s.entries = append(s.entries, old)
		}
		delete(s.fresh, key)
		delete(s.replaced, key)
		s.mu.Unlock()
		removed = append(removed, k)
	}
	if len(removed) > 0 {
		if err := s.save(); err != nil {
			log.Printf("шрифты: реестр не записан: %v", paths.NoPath(err))
		}
	}
	return removed, nil
}

// fileHere — файл записи на месте и это обычный файл (не ссылка).
func (s *Store) fileHere(e *entry) bool {
	st, err := os.Lstat(s.path(e.Key, e.Ext))
	return err == nil && st.Mode().IsRegular()
}

func deleteEntry(list []*entry, e *entry) []*entry {
	for i, o := range list {
		if o == e {
			return append(list[:i:i], list[i+1:]...)
		}
	}
	return list
}
