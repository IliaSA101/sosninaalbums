package libpacks

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/decor/fetch"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// Кэш листов образцов (спека 144, design §4, security Н2, Н9, требования 2, 6, 10): файлы
// <папка настроек>/library-previews/<sha256>.jpg. Содержимое недоверенное (папку мог тронуть любой
// процесс пользователя): каждое чтение сверяет тип файла, размер и sha256 с подписанным каталогом.

const (
	previewDirName = "library-previews"
	previewExt     = ".jpg"
	tmpPattern     = ".dl-*.tmp"
)

var (
	// errNoSheet — листа нет, он не тот или не читается: образца нет, остальное работает.
	errNoSheet = errors.New("лист образцов не найден")
	// previewName — имя своего листа: 64 строчных hex и «.jpg»; уборка не трогает другие имена.
	previewName = regexp.MustCompile(`^[0-9a-f]{64}\.jpg$`)
)

// validSHA — 64 строчных hex (имя файла листа).
func validSHA(s string) bool {
	return len(s) == 64 && strings.Trim(s, "0123456789abcdef") == ""
}

// previewDir — папка листов или "" (образцы выключены): нет папки настроек или она не абсолютна
// (при сбое UserConfigDir Join("", …) писал бы в текущую папку процесса — Н2), либо папка листов —
// не обычная папка (ссылка, junction, файл): выключено до конца сеанса, одна строка в log.txt.
// Папку не создаёт — читатели (плитки, клетки) не должны плодить пустую.
func (s *Service) previewDir() string {
	cfg := s.h.CfgDir()
	if cfg == "" || !filepath.IsAbs(cfg) || s.prev.off.Load() {
		return ""
	}
	dir := filepath.Join(cfg, previewDirName)
	st, err := os.Lstat(dir)
	switch {
	case err != nil:
		return dir // нет папки — листов нет, чтение само скажет об этом
	case !st.IsDir():
		if s.prev.off.CompareAndSwap(false, true) {
			log.Printf("библиотека: образцы выключены — папка листов не обычная папка")
		}
		return ""
	}
	return dir
}

// ensureDir — папка листов для задания: создаётся (0700) и проверяется как previewDir.
func (s *Service) ensureDir() string {
	cfg := s.h.CfgDir()
	if cfg == "" || !filepath.IsAbs(cfg) {
		return ""
	}
	if err := os.MkdirAll(filepath.Join(cfg, previewDirName), 0o700); err != nil {
		return ""
	}
	return s.previewDir()
}

// sheetPath — путь листа; "" — папки нет или sha негодный (имя берётся только из 64 hex).
func (s *Service) sheetPath(p *fetch.Preview) string {
	dir := s.previewDir()
	if dir == "" || p == nil || !validSHA(p.SHA256) {
		return ""
	}
	return filepath.Join(dir, p.SHA256+previewExt)
}

// sheetPresent — лист на месте: обычный файл заявленного размера (без sha и разбора: sha
// сверяется при чтении, разбор — при показе).
func (s *Service) sheetPresent(p *fetch.Preview) bool {
	path := s.sheetPath(p)
	if path == "" || p.Size < 1 || p.Size > fetch.MaxPreview {
		return false
	}
	st, err := os.Lstat(path)
	return err == nil && st.Mode().IsRegular() && st.Size() == p.Size
}

// readPreview — байты листа: обычный файл заявленного размера, открытый тот же, что проверенный,
// не больше MaxPreview+1 байт, sha256 как в каталоге. Не сошлось — errNoSheet; неверный sha
// удаляет файл (следующее задание скачает заново).
func (s *Service) readPreview(p *fetch.Preview) ([]byte, error) {
	path := s.sheetPath(p)
	if path == "" || p.Size < 1 || p.Size > fetch.MaxPreview || paths.CheckLocalPath(path) != nil {
		return nil, errNoSheet
	}
	lst, err := os.Lstat(path)
	if err != nil || !lst.Mode().IsRegular() || lst.Size() != p.Size {
		return nil, errNoSheet
	}
	_ = os.SameFile(lst, lst) // Windows читает номер файла лениво по пути
	if s.afterLstat != nil {
		s.afterLstat()
	}
	f, got, err := library.OpenLocal(path)
	if err != nil {
		return nil, errNoSheet
	}
	// Без defer: на Windows открытый файл не удалить, а Remove ниже идёт до выхода из функции.
	if !os.SameFile(lst, got) || !got.Mode().IsRegular() || got.Size() != p.Size {
		_ = f.Close()
		return nil, errNoSheet
	}
	data, err := io.ReadAll(io.LimitReader(f, fetch.MaxPreview+1))
	_ = f.Close() // чтение: ошибка закрытия ничего не теряет
	if err != nil || int64(len(data)) != p.Size {
		return nil, errNoSheet
	}
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != p.SHA256 {
		_ = os.Remove(path)
		return nil, errNoSheet
	}
	return data, nil
}

// gcPreviews — уборка после remember (design §4, Н9): остаются листы действующего списка cat,
// остальные свои имена удаляются. Вне catMu (окно зовёт cached() раз в 2 с) и под prevMu.TryLock:
// занято заданием — пропуск (удаление идущего .dl-*.tmp на Linux проходит, а Rename потом падает).
// Ссылки и папки с подходящим именем не трогаются (Type().IsRegular), цели ссылок не разыменуются.
func (s *Service) gcPreviews(cat fetch.Catalog) {
	dir := s.previewDir()
	if dir == "" || !s.prevMu.TryLock() {
		return
	}
	defer s.prevMu.Unlock()
	keep := map[string]bool{}
	for _, e := range cat.Packs {
		if e.Preview != nil {
			keep[e.Preview.SHA256+previewExt] = true
		}
	}
	list, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range list {
		if n := e.Name(); e.Type().IsRegular() && previewName.MatchString(n) && !keep[n] {
			_ = os.Remove(filepath.Join(dir, n)) // не вышло — доберёт следующая уборка
		}
	}
}

// cleanTmp — остатки .dl-*.tmp прежних заданий (процесс убит посреди загрузки); только в начале
// задания под prevMu.
func cleanTmp(dir string) {
	list, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range list {
		if ok, _ := filepath.Match(tmpPattern, e.Name()); ok && e.Type().IsRegular() {
			_ = os.Remove(filepath.Join(dir, e.Name()))
		}
	}
}

// prevLog — журнал образцов: первая строка и каждая сотая (как logRare в app); без адресов и путей.
func (s *Service) prevLog(format string, args ...any) {
	if n := s.prev.logN.Add(1); n == 1 || n%100 == 0 {
		log.Printf("библиотека: образцы — "+format, args...)
	}
}
