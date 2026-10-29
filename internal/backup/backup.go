// Package backup — резервные копии файла альбома (спека 019 E): перед перезаписью прежний
// .salbum копируется байт в байт в %LOCALAPPDATA%\SosninaAlbums\backups\<ключ альбома>\.
// Ни один путь не приходит из интерфейса: ключ строится по пути альбома, имя копии
// проверяется шаблоном, ссылки и junction в папке копий не читаются и не удаляются.
package backup

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/osx"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// Пределы (решение владельца 2026-10-09).
const (
	Keep     = 10                   // копий на альбом
	Every    = 10 * time.Minute     // не чаще, после первой копии за сеанс
	MaxDirs  = 30                   // папок альбомов
	MaxBytes = 300 << 20            // всего
	MaxAge   = 180 * 24 * time.Hour // копии старше удаляются
)

// Пределы, которые держатся и между запусками (чистка при старте — не единственная).
const (
	albumMax   = MaxBytes / 3 // байт в папке одного альбома
	pruneEvery = 20           // записей за сеанс до фоновой Prune
	tmpAge     = time.Hour    // остаток оборванной записи старше — удаляется
	dirMode    = 0o700        // копии — только владельцу (вне Windows)
	fileMode   = 0o600
)

// DraftKey — папка копий черновика: у него нет своего имени.
const DraftKey = "черновик"

// maxFile — предел чтения, как у salbum.Load: больше альбом не бывает.
const maxFile = 50 << 20

const (
	ext        = ".salbum"
	timeLayout = "20060102-150405"
	maxName    = 40 // рун в имени папки альбома
)

// nameRe — имя копии: ГГГГММДД-ччммсс[-N].salbum.
var nameRe = regexp.MustCompile(`^(\d{8}-\d{6})(?:-(\d{1,3}))?\.salbum$`)

// ErrNotFound — копии с таким именем нет (или имя не по шаблону).
var ErrNotFound = errors.New("копия не найдена")

// Store — папка копий. Пустой Root — копий нет (методы ничего не делают).
type Store struct {
	Root string
	now  func() time.Time

	mu       sync.Mutex
	last     map[string]time.Time // ключ → время последней копии в этом сеансе
	writes   int                  // записей с прошлой PruneDue
	albumMax int64                // предел папки альбома (тесты уменьшают)
}

// New — хранилище копий в root.
func New(root string) *Store {
	return &Store{Root: root, now: time.Now, last: map[string]time.Time{}, albumMax: albumMax}
}

// DefaultRoot — %LOCALAPPDATA%\<name>\backups (как osx.WebViewDir; name — osx.DataName, у
// стенда QA своя): копии не ездят с перемещаемым профилем. На Linux — $XDG_DATA_HOME
// (~/.local/share): кэш, где они лежали бы иначе, чистят «уборщики» (101, design §3.1).
// "" — папки данных нет.
func DefaultRoot(name string) string {
	base := os.Getenv("LOCALAPPDATA")
	if !filepath.IsAbs(base) { // относительный путь копии «в текущую папку» не кладёт
		base = ""
	}
	if base == "" && runtime.GOOS == "linux" {
		base = xdgDataHome()
	}
	if base == "" {
		base = osx.CacheDir() // "" при ошибке или относительном пути — общая функция папки данных
	}
	if base == "" {
		return ""
	}
	return filepath.Join(base, name, "backups")
}

// Key — папка копий альбома: чистое имя файла и 8 hex от sha256 полного пути (paths.Fold)
// (два «Альбом.salbum» в разных папках не смешиваются).
func Key(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	sum := sha256.Sum256([]byte(paths.Fold(filepath.Clean(abs))))
	return cleanName(strings.TrimSuffix(filepath.Base(abs), filepath.Ext(abs))) + "-" + hex.EncodeToString(sum[:4])
}

// xdgDataHome — $XDG_DATA_HOME или ~/.local/share; относительный путь XDG не годится (спецификация
// XDG велит его пропустить). "" — нет ни того, ни домашней папки.
func xdgDataHome() string {
	if d := os.Getenv("XDG_DATA_HOME"); filepath.IsAbs(d) {
		return d
	}
	if home, err := os.UserHomeDir(); err == nil && filepath.IsAbs(home) {
		return filepath.Join(home, ".local", "share")
	}
	return ""
}

// cleanName — имя без запрещённых в Windows знаков и управляющих символов, без точек и
// пробелов по краям, не длиннее maxName рун; пусто — «альбом».
func cleanName(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r == utf8.RuneError || unicode.IsControl(r) || strings.ContainsRune(`<>:"/\|?*`, r) {
			continue
		}
		b.WriteRune(r)
	}
	trim := func(s string) string { return strings.Trim(s, ". ") }
	out := trim(b.String())
	if utf8.RuneCountInString(out) > maxName {
		out = trim(string([]rune(out)[:maxName]))
	}
	if out == "" {
		return "альбом"
	}
	return out
}

// validKey — ключ одной папкой внутри Root: без разделителей, «..» и устройств.
func validKey(key string) bool {
	return key != "" && !strings.ContainsAny(key, `/\:`) && filepath.IsLocal(key)
}

// Before — перед перезаписью path: копия прежнего файла, если в этом сеансе для key её ещё не
// было или с прошлой прошло не меньше Every; затем в папке альбома остаётся Keep последних.
// Время ставится и при сбое: иначе ошибка повторялась бы на каждом автосохранении.
// Файл совпал с самой новой копией — не пишется и время не ставится (дубль ничего не сохраняет).
func (s *Store) Before(path, key string) error { return s.copy(path, key, false, "") }

// Force — копия без учёта Every (черновик перед удалением, файл перед «Сохранить как» поверх него).
func (s *Store) Force(path, key string) error { return s.copy(path, key, true, "") }

// ForceKeep — Force, только чистка не удаляет копию keep: её восстанавливают, и после Ctrl+Z
// она должна остаться в списке, даже если была самой давней из Keep.
func (s *Store) ForceKeep(path, key, keep string) error { return s.copy(path, key, true, keep) }

// PruneDue — с прошлого вызова записано pruneEvery копий: пора Prune (в фоне у вызывающего),
// иначе за долгий сеанс общий предел держала бы только чистка при старте.
func (s *Store) PruneDue() bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.writes < pruneEvery {
		return false
	}
	s.writes = 0
	return true
}

func (s *Store) copy(path, key string, force bool, keep string) error {
	if s == nil || s.Root == "" {
		return nil
	}
	if !validKey(key) {
		return fmt.Errorf("недопустимый ключ копии %q", key)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	// Часы ушли назад (now раньше прошлой копии) — пора: иначе копий не было бы до тех пор.
	if t, ok := s.last[key]; ok && !force && !now.Before(t) && now.Sub(t) < Every {
		return nil
	}
	data, err := readLimited(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil // перезаписывать нечего
	}
	dir := filepath.Join(s.Root, key)
	if err == nil {
		err = s.ensureDir(dir)
	}
	if err == nil && sameAsNewest(dir, data, now) {
		return nil
	}
	s.last[key] = now
	if err != nil {
		return err
	}
	name, err := freeName(dir, now)
	if err != nil {
		return err
	}
	if err := library.WriteFileAtomicMode(filepath.Join(dir, name), data, fileMode); err != nil {
		return err
	}
	s.writes++
	return s.trim(dir, now, name, keep)
}

// sameAsNewest — data байт в байт совпадает с самой новой копией альбома.
func sameAsNewest(dir string, data []byte, now time.Time) bool {
	cs, err := copies(dir, now)
	if err != nil || len(cs) == 0 || cs[0].size != int64(len(data)) {
		return false
	}
	old, err := readLimited(filepath.Join(dir, cs[0].name))
	return err == nil && bytes.Equal(old, data)
}

// ensureDir создаёт Root и папку альбома; существующая папка — только обычная (не ссылка и
// не junction: запись через неё ушла бы мимо папки копий).
func (s *Store) ensureDir(dir string) error {
	if err := os.MkdirAll(s.Root, dirMode); err != nil {
		return err
	}
	if !plainDir(s.Root) {
		return errors.New("папка копий — не обычная папка") // путь не пишем: в нём имя пользователя
	}
	if err := os.Mkdir(dir, dirMode); err != nil && !errors.Is(err, fs.ErrExist) {
		return err
	}
	if !plainDir(dir) {
		return fmt.Errorf("папка копий альбома — не обычная папка: %s", filepath.Base(dir))
	}
	return nil
}

// freeName — имя копии по местному времени; занято — следующий номер после самого большого
// (-2, -3…): освободившийся после чистки меньший номер сделал бы новую копию «старше» прежних.
func freeName(dir string, now time.Time) (string, error) {
	stamp := now.Format(timeLayout)
	ents, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	last := 0
	for _, e := range ents {
		m := nameRe.FindStringSubmatch(e.Name())
		if m == nil || m[1] != stamp {
			continue
		}
		seq := 1
		if m[2] != "" {
			seq, _ = strconv.Atoi(m[2])
		}
		last = max(last, seq)
	}
	switch {
	case last == 0:
		return stamp + ext, nil
	case last < 999:
		return fmt.Sprintf("%s-%d%s", stamp, last+1, ext), nil
	}
	return "", fmt.Errorf("нет свободного имени копии на %s", stamp)
}

// plainDir — обычная папка (Lstat: ссылка и junction не считаются).
func plainDir(p string) bool {
	st, err := os.Lstat(p)
	return err == nil && st.Mode().Type() == fs.ModeDir
}

// readLimited читает файл не больше maxFile.
func readLimited(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close() //nolint:errcheck // чтение: ошибка закрытия ничего не теряет
	data, err := io.ReadAll(io.LimitReader(f, maxFile+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxFile {
		return nil, fmt.Errorf("файл больше %d МБ", maxFile>>20)
	}
	return data, nil
}

// Inside — path лежит в папке копий: строкой (без регистра, после Abs и Clean) или тем же
// каталогом под другим именем — \\?\, \\localhost\C$, subst, junction строкой не поймать.
func (s *Store) Inside(path string) bool {
	if s == nil || s.Root == "" {
		return false
	}
	root, err1 := filepath.Abs(s.Root)
	p, err2 := filepath.Abs(path)
	if err1 != nil || err2 != nil {
		return false
	}
	lr, lp := paths.Fold(filepath.Clean(root)), paths.Fold(filepath.Clean(p))
	if lp == lr || strings.HasPrefix(lp, strings.TrimSuffix(lr, string(filepath.Separator))+string(filepath.Separator)) {
		return true
	}
	rst, err := os.Stat(root)
	if err != nil {
		return false // папки копий нет — внутри неё ничего не лежит
	}
	for d, i := p, 0; i < maxDepth; i++ {
		if st, err := os.Stat(d); err == nil && os.SameFile(rst, st) {
			return true
		}
		up := filepath.Dir(d)
		if up == d {
			break
		}
		d = up
	}
	return false
}

// maxDepth — предел подъёма по родителям в Inside (страховка от зацикливания на странном пути).
const maxDepth = 256

// Dir — папка копий альбома key ("" — ключ негодный).
func (s *Store) Dir(key string) string {
	if s == nil || s.Root == "" || !validKey(key) {
		return ""
	}
	return filepath.Join(s.Root, key)
}

// File — путь копии name альбома key: имя по шаблону, путь внутри Root, обычный файл.
func (s *Store) File(key, name string) (string, error) {
	dir := s.Dir(key)
	if dir == "" || !nameRe.MatchString(name) {
		return "", ErrNotFound
	}
	if rel, err := filepath.Rel(s.Root, filepath.Join(dir, name)); err != nil || !filepath.IsLocal(rel) {
		return "", ErrNotFound
	}
	if !plainDir(s.Root) || !plainDir(dir) {
		return "", ErrNotFound
	}
	p := filepath.Join(dir, name)
	if st, err := os.Lstat(p); err != nil || !st.Mode().IsRegular() {
		return "", ErrNotFound
	}
	return p, nil
}
