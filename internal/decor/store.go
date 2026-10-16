// Package decor — библиотека фонов и стикеров (спека 048, design §5): реестр своих файлов
// <cfgDir>/Библиотека/library.json, каталог (встроенные пресеты + свои − скрытые), импорт,
// ✎ и ×, искатель файла по ключу для salbum и отдача /media/gen и /media/lib.
//
// Граница безопасности: путь к своему файлу строит только код из ключа my:<16 hex>; путей от
// интерфейса и из .salbum пакет не принимает; запись — только внутри корня библиотеки (tmp +
// rename). Недавние пишет App в prefs: decor их только фильтрует.
package decor

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/imgfile"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// Виды библиотеки (kind в методах App).
const (
	KindBg      = "bg"
	KindSticker = "sticker"
)

// RootName — папка библиотеки в папке данных (решение №79); regFile — реестр.
const (
	RootName   = "Библиотека"
	regFile    = "library.json"
	regVersion = 1
	maxRegSize = 16 << 20 // 2×2000 записей — сотни КБ; больше — чужой или битый файл
)

// Ошибки для интерфейса.
var (
	ErrReadOnly = errors.New("библиотеку изменила более новая версия программы — здесь её можно только смотреть")
	ErrKind     = errors.New("неизвестный вид библиотеки")
	ErrNotFound = errors.New("нет такого элемента в библиотеке")
	ErrBuiltin  = errors.New("встроенный элемент не меняется")
	ErrName     = errors.New("введите название")
)

// entry — запись library.json. Kind чужого вида (оформления 049 и новее) хранится как есть.
type entry struct {
	Key    string `json:"key"`
	Kind   string `json:"kind"`
	Theme  string `json:"theme"`
	Name   string `json:"name"`
	Ext    string `json:"ext"`
	W      int    `json:"w"`
	H      int    `json:"h"`
	Bytes  int64  `json:"bytes"`
	Added  int64  `json:"added"` // unix мс
	Hidden bool   `json:"hidden,omitempty"`
}

type registry struct {
	Version int      `json:"version"`
	Items   []*entry `json:"items"`
}

// Store — библиотека одной папки данных. Методы безопасны из разных горутин: Find и /media
// читают под RLock, правки реестра идут по одной (ioMu), медленное копирование — без mu.
type Store struct {
	root string

	ioMu sync.Mutex // Import, Rename, Hide: чтение-правка-запись реестра по одной

	mu       sync.RWMutex
	entries  []*entry                     // порядок файла; запись сохраняет и чужие виды
	files    map[string]map[string]string // вид → ключ → путь файла на диске (скан + импорт)
	readOnly bool

	media *mediaCache
	svg   svgTokens // проверенные SVG, ждущие PNG от окна (AddPNG)
}

// kindDirs — папка вида; имя файла внутри — только <16 hex>.<ext> от ключа.
var kindDirs = map[string]string{KindBg: "Фоны", KindSticker: "Стикеры"}

// Open открывает библиотеку в <cfgDir>/Библиотека: читает реестр и ищет файлы. Ошибки чтения
// не мешают работе: битый реестр уходит в .bad, библиотека пуста (строка в log.txt).
func Open(cfgDir string) *Store {
	s := &Store{root: filepath.Join(cfgDir, RootName), files: map[string]map[string]string{}, media: newMediaCache(mediaCacheMax)}
	s.scan()
	s.load()
	return s
}

// Root — корень библиотеки.
func (s *Store) Root() string { return s.root }

// ReadOnly — реестр записан более новой версией: импорт, ✎ и × закрыты.
func (s *Store) ReadOnly() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.readOnly
}

// scan строит индекс ключ → файл по папкам видов и убирает остатки оборванного импорта.
func (s *Store) scan() {
	for kind, sub := range kindDirs {
		dir := filepath.Join(s.root, sub)
		list, err := os.ReadDir(dir)
		if err != nil {
			continue // папки ещё нет — своих этого вида нет
		}
		m := map[string]string{}
		for _, de := range list {
			name := de.Name()
			if strings.HasPrefix(name, tmpPrefix) && strings.HasSuffix(name, ".tmp") {
				_ = os.Remove(filepath.Join(dir, name)) // остаток импорта, оборванного выходом
				continue
			}
			key, ok := keyOfFile(name)
			if !ok || !de.Type().IsRegular() {
				continue
			}
			if _, dup := m[key]; !dup { // ReadDir по имени: .jpg раньше .png
				m[key] = filepath.Join(dir, name)
			}
		}
		s.files[kind] = m
	}
}

// keyOfFile — ключ по имени файла <16 hex>.<jpg|png>; иначе ok = false.
func keyOfFile(name string) (string, bool) {
	base, ext, ok := strings.Cut(name, ".")
	if !ok || (ext != "jpg" && ext != "png") {
		return "", false
	}
	key := libPrefix + base
	return key, model.ValidLibKey(key)
}

const libPrefix = "my:"

// load читает library.json. Нет файла — пустой реестр; битый (разбор, версия, размер) — в .bad;
// не читается (доступ, блокировка антивирусом или облаком) — реестр не трогаем, сеанс только
// чтение: иначе исправный реестр ушёл бы в .bad, а первый импорт записал бы пустой поверх.
// Версия новее — только чтение.
func (s *Store) load() {
	path := filepath.Join(s.root, regFile)
	data, err := readCapped(path, maxRegSize)
	if errors.Is(err, os.ErrNotExist) {
		return
	}
	if err != nil && !errors.Is(err, errTooBig) {
		log.Printf("библиотека: реестр не читается (%v) — только чтение", paths.NoPath(err))
		s.readOnly = true
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
		log.Printf("библиотека: реестр не читается (%v), отложен в library.json.bad", paths.NoPath(err))
		if rerr := os.Rename(path, path+".bad"); rerr != nil {
			log.Printf("библиотека: %v", paths.NoPath(rerr))
		}
		return
	}
	if reg.Version > regVersion {
		log.Printf("библиотека: реестр версии %d — только чтение", reg.Version)
		s.readOnly = true
	}
	seen := map[string]bool{}
	for _, e := range reg.Items {
		if e == nil || !model.ValidLibKey(e.Key) || !validKind(e.Kind) || seen[e.Kind+"\x00"+e.Key] {
			continue
		}
		seen[e.Kind+"\x00"+e.Key] = true
		if !isTheme(e.Theme) {
			e.Theme = ThemeMine
		}
		if e.Name = cleanName(e.Name); e.Name == "" {
			e.Name = noName
		}
		// Реестр правят руками: размеры — в пределах файла, иначе сумма байт библиотеки
		// (usage) переполнилась бы и пустила импорт сверх MaxLibraryLen.
		e.W, e.H = min(max(e.W, 0), imgfile.MaxPixels), min(max(e.H, 0), imgfile.MaxPixels)
		e.Bytes = min(max(e.Bytes, 0), MaxBgBytes)
		s.entries = append(s.entries, e)
	}
}

// errTooBig — файл больше предела readCapped: это содержимое, а не ошибка чтения.
var errTooBig = errors.New("слишком большой файл")

// readCapped читает файл не больше limit байт (больше — errTooBig).
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

// validKind — грамматика вида [a-z]{1,16}: чужие виды хранятся, свои — kindDirs.
func validKind(k string) bool {
	if k == "" || len(k) > 16 {
		return false
	}
	for i := range len(k) {
		if k[i] < 'a' || k[i] > 'z' {
			return false
		}
	}
	return true
}

// save пишет реестр атомарно (tmp + rename). Зовётся под ioMu.
func (s *Store) save() error {
	s.mu.RLock()
	data, err := json.MarshalIndent(registry{Version: regVersion, Items: s.entries}, "", "  ")
	s.mu.RUnlock()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(s.root, 0o755); err != nil {
		return err
	}
	return library.WriteFileAtomic(filepath.Join(s.root, regFile), data)
}

// entryOf — запись вида kind с ключом key; зовётся под mu.
func (s *Store) entryOf(kind, key string) *entry {
	for _, e := range s.entries {
		if e.Kind == kind && e.Key == key {
			return e
		}
	}
	return nil
}

// Find — путь своего файла по ключу (искатель salbum.LibFinder): по индексу, без реестра —
// скопированный руками файл тоже находится. Фон важнее стикера: у фона байты исходника.
func (s *Store) Find(key string) (string, bool) {
	if !model.ValidLibKey(key) {
		return "", false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, kind := range []string{KindBg, KindSticker} {
		if p, ok := s.files[kind][key]; ok {
			return p, true
		}
	}
	return "", false
}

// mutate — правка одной записи своего (✎, ×): проверки, fn под mu, запись реестра;
// не записался — правка откатывается.
func (s *Store) mutate(kind, id string, fn func(e *entry) bool) error {
	if _, ok := kindDirs[kind]; !ok {
		return ErrKind
	}
	if !model.ValidLibKey(id) {
		if strings.HasPrefix(id, presetPrefix) {
			return ErrBuiltin
		}
		return ErrNotFound
	}
	s.ioMu.Lock()
	defer s.ioMu.Unlock()
	s.mu.Lock()
	if s.readOnly {
		s.mu.Unlock()
		return ErrReadOnly
	}
	e := s.entryOf(kind, id)
	if e == nil {
		s.mu.Unlock()
		return ErrNotFound
	}
	old := *e
	changed := fn(e)
	s.mu.Unlock()
	if !changed {
		return nil
	}
	if err := s.save(); err != nil {
		s.mu.Lock()
		*e = old
		s.mu.Unlock()
		return errors.New("библиотека не записалась: " + paths.NoPath(err).Error())
	}
	return nil
}

// Rename — ✎ своего: новое имя для показа (чистится, ≤ 60 знаков).
func (s *Store) Rename(kind, id, name string) error {
	name = cleanName(name)
	if name == "" {
		return ErrName
	}
	return s.mutate(kind, id, func(e *entry) bool {
		if e.Name == name {
			return false
		}
		e.Name = name
		return true
	})
}

// Hide — × своего: запись скрывается, файл остаётся (решение №80) — альбомы, где он стоит,
// не меняются; повторная загрузка того же файла возвращает его.
func (s *Store) Hide(kind, id string) error {
	return s.mutate(kind, id, func(e *entry) bool {
		if e.Hidden {
			return false
		}
		e.Hidden = true
		return true
	})
}

// nowMs — время записи; подменяется в тестах порядка.
var nowMs = func() int64 { return time.Now().UnixMilli() }
