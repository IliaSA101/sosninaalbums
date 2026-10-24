package decor

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/imgfile"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// Пределы импорта (решение №93).
const (
	MaxImport     = 100       // файлов за раз
	MaxOwn        = 2000      // своих на вид, со скрытыми: их файлы на диске
	MaxBgBytes    = 200 << 20 // фон — байты как есть
	MaxLibraryLen = 2 << 30   // вся библиотека: Roaming в доменном профиле (решение №79)
)

// tmpPrefix — временный файл импорта в папке вида; остатки убирает scan.
const tmpPrefix = "import-"

// Skip — пропущенный файл и причина для тоста.
type Skip struct {
	Name string `json:"name"`
	Why  string `json:"why"`
}

// ImportResult — итог «Загрузить своё…»: добавленные (и возвращённые из скрытых), пропущенные и
// проверенные SVG, которые окну осталось нарисовать (Svg; запись добавит AddPNG).
type ImportResult struct {
	Added   []Item   `json:"added"`
	Skipped []Skip   `json:"skipped"`
	Svg     []SvgJob `json:"svg"`
}

// Import копирует файлы в библиотеку вида kind, в тему theme (выборки и неизвестная — «Мои»).
// Пути — только из диалога открытия в App. Ключ — от байтов исходника: тот же файл второй раз —
// «уже есть», скрытый — возвращается. Ошибка — только когда библиотека закрыта целиком.
func (s *Store) Import(kind, theme string, files []string) (ImportResult, error) {
	res := ImportResult{Added: []Item{}, Skipped: []Skip{}, Svg: []SvgJob{}}
	if _, ok := kindDirs[kind]; !ok || kind == KindSpread || kind == KindPrint { // шаблон разворота — SaveSpreadTpl, типография — SavePrint
		return res, ErrKind
	}
	if !isTheme(theme) {
		theme = ThemeMine
	}
	s.ioMu.Lock()
	defer s.ioMu.Unlock()
	if s.ReadOnly() {
		return res, ErrReadOnly
	}
	dirty := false
	for i, p := range files {
		name := displayName(p)
		if i >= MaxImport {
			res.Skipped = append(res.Skipped, Skip{name, fmt.Sprintf("за раз — не больше %d файлов", MaxImport)})
			continue
		}
		var (
			it      Item
			changed bool
			err     error
		)
		if kind == KindSticker || kind == KindFrame {
			var job *SvgJob
			if kind == KindFrame {
				it, job, changed, err = s.importFrame(theme, name, p)
			} else {
				it, job, changed, err = s.importSticker(theme, name, p)
			}
			if job != nil { // SVG чист: PNG нарисует окно, запись появится в AddPNG
				res.Svg = append(res.Svg, *job)
			}
		} else {
			it, changed, err = s.importOne(kind, theme, name, p)
		}
		dirty = dirty || changed
		if err != nil {
			res.Skipped = append(res.Skipped, Skip{name, err.Error()})
			continue
		}
		if it.ID != "" {
			res.Added = append(res.Added, it)
		}
	}
	if dirty {
		s.saveLogged()
	}
	return res, nil
}

// saveLogged пишет реестр; ошибка — в log.txt: файлы уже на месте, искатель их находит, в
// каталоге они — до перезапуска. Зовётся под ioMu.
func (s *Store) saveLogged() {
	if err := s.save(); err != nil {
		log.Printf("библиотека: реестр не записан: %v", paths.NoPath(err))
	}
}

// errHave — тот же файл уже в библиотеке (видимый).
var errHave = errors.New("уже есть в библиотеке")

// importOne — один файл: проверка, копия во временный, ключ, запись под именем от ключа.
// changed — реестр изменился (добавлен или возвращён).
func (s *Store) importOne(kind, theme, name, src string) (Item, bool, error) {
	if paths.CheckLocalPath(src) != nil {
		return Item{}, false, errors.New("файл не на локальном диске")
	}
	ph, err := library.BackdropFile(src) // локальный, без .., обычный файл, JPEG/PNG, MaxPixels
	if err != nil {
		return Item{}, false, paths.NoPath(err)
	}
	if ph.Size > MaxBgBytes {
		return Item{}, false, fmt.Errorf("файл больше %d МБ", MaxBgBytes>>20)
	}
	dir := filepath.Join(s.root, kindDirs[kind])
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Item{}, false, paths.NoPath(err)
	}
	tmp, key, err := copyHashed(ph.Path, dir, MaxBgBytes)
	if err != nil {
		return Item{}, false, err
	}
	defer func() { _ = os.Remove(tmp) }() // после rename его уже нет
	info, ext, err := checkCopy(tmp)
	if err != nil {
		return Item{}, false, err
	}
	return s.commit(kind, theme, name, key, tmp, ext, info)
}

// checkCopy — копия проверяется заново (исходник мог смениться между проверкой и чтением):
// заголовок, пределы, расширение по сигнатуре.
func checkCopy(tmp string) (library.Photo, string, error) {
	info, err := library.ReadInfo(tmp)
	if err == nil {
		err = imgfile.CheckSize(tmp)
	}
	if err != nil {
		return info, "", errors.New("файл не читается как JPEG или PNG")
	}
	ext, err := sniffExt(tmp)
	return info, ext, err
}

// commit — проверенная копия tmp под ключом key: повтор, возврат скрытого, пределы, rename
// в имя от ключа, запись реестра в памяти.
func (s *Store) commit(kind, theme, name, key, tmp, ext string, info library.Photo) (Item, bool, error) {
	s.mu.RLock()
	e := s.entryOf(kind, key)
	have, onDisk := s.files[kind][key]
	n, total := s.usage(kind)
	s.mu.RUnlock()
	if onDisk { // стёртый руками файл загрузка того же исходника возвращает (сц. 8)
		_, err := os.Stat(have)
		onDisk = err == nil
	}
	if e != nil && onDisk {
		if !e.Hidden {
			return Item{}, false, errHave
		}
		s.mu.Lock()
		e.Hidden = false
		s.mu.Unlock()
		return e.item(), true, nil
	}
	if e == nil && n >= MaxOwn {
		return Item{}, false, fmt.Errorf("своих уже %d — больше не помещается", MaxOwn)
	}
	if total+info.Size > MaxLibraryLen {
		return Item{}, false, fmt.Errorf("библиотека заполнена (%d ГБ)", MaxLibraryLen>>30)
	}
	final := filepath.Join(filepath.Dir(tmp), strings.TrimPrefix(key, libPrefix)+"."+ext)
	if !library.Under(final, s.root) { // имя от ключа, но проверка дешевле ошибки
		return Item{}, false, errors.New("недопустимый путь в библиотеке")
	}
	if err := os.Rename(tmp, final); err != nil {
		return Item{}, false, paths.NoPath(err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.files[kind] == nil {
		s.files[kind] = map[string]string{}
	}
	s.files[kind][key] = final
	if e == nil { // запись без файла (стёрт руками) остаётся со своим именем и темой
		e = &entry{Key: key, Kind: kind, Theme: theme, Name: name, Added: nowMs()}
		s.entries = append(s.entries, e)
	}
	e.Ext, e.W, e.H, e.Bytes, e.Hidden = ext, info.W, info.H, info.Size, false
	return e.item(), true, nil
}

// usage — своих вида kind и байты всей библиотеки — свои и наборы (libBytes): квота 2 ГБ общая
// (design §4.2), иначе набор у предела и своё сверху проходили бы мимо неё; под mu.
func (s *Store) usage(kind string) (int, int64) {
	n := 0
	for _, e := range s.entries {
		if e.Kind == kind {
			n++
		}
	}
	return n, s.libBytes()
}

// copyHashed копирует src во временный файл папки dir, считая sha256 по дороге: исходник
// читается один раз. Ключ — my: + 8 байт хэша (решение №78).
func copyHashed(src, dir string, limit int64) (tmp, key string, err error) {
	in, err := imgfile.OpenRegular(src)
	if err != nil {
		return "", "", paths.NoPath(err)
	}
	defer in.Close() //nolint:errcheck // чтение: ошибка закрытия ничего не теряет
	out, err := os.CreateTemp(dir, tmpPrefix+"*.tmp")
	if err != nil {
		return "", "", paths.NoPath(err)
	}
	tmp = out.Name()
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(out, h), io.LimitReader(in, limit+1))
	if err == nil && n > limit {
		err = fmt.Errorf("файл больше %d МБ", limit>>20)
	}
	if err == nil {
		err = out.Sync()
	}
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		_ = os.Remove(tmp)
		return "", "", paths.NoPath(err)
	}
	sum := h.Sum(nil)
	return tmp, libPrefix + hex.EncodeToString(sum[:8]), nil
}

// sniffExt — расширение по сигнатуре копии, а не по имени исходника.
func sniffExt(path string) (string, error) {
	f, err := imgfile.OpenRegular(path)
	if err != nil {
		return "", paths.NoPath(err)
	}
	defer f.Close() //nolint:errcheck // чтение: ошибка закрытия ничего не теряет
	head := make([]byte, 8)
	if _, err := io.ReadFull(f, head); err != nil {
		return "", errors.New("файл не читается как JPEG или PNG")
	}
	switch {
	case bytes.Equal(head, []byte("\x89PNG\r\n\x1a\n")):
		return "png", nil
	case head[0] == 0xFF && head[1] == 0xD8:
		return "jpg", nil
	}
	return "", errors.New("файл не читается как JPEG или PNG")
}

// displayName — имя для показа из имени файла без расширения.
func displayName(p string) string {
	base := filepath.Base(p)
	if n := cleanName(strings.TrimSuffix(base, filepath.Ext(base))); n != "" {
		return n
	}
	return noName
}
