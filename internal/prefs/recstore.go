package prefs

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// Общий код файлов записей о выгрузках: shares.json (ключ — путь) и projects.json (ключ — uid)
// читаются и пишутся одинаково — предел размера и числа записей, .bad, «новее — только чтение»,
// атомарная запись 0600. Замок один на процесс — sharesMu (sharesfile.go).

// storeKind — чем файл записей отличается от другого.
type storeKind[R any] struct {
	file string // имя файла: "shares.json"
	// decode/encode — обёртка файла; версия идёт первой (peekShareVersion читает её из начала).
	decode func(data []byte) (version int, items map[string]json.RawMessage, err error)
	encode func(items map[string]json.RawMessage) ([]byte, error)
	// key — ключ записи из файла годен и уже в каноническом виде (иначе запись отбрасывается).
	key   func(string) bool
	clean func(R) (R, bool)
	same  func(a, b R) bool
	// expires — срок записи; истёкший больше sharesKeepGone назад выбрасывается при записи.
	expires func(R) time.Time
	// seed — записи для файла, которого ещё нет (импорт projects.json из shares.json); nil — нет.
	// Ошибка — источник не открылся: записи неизвестны, файл не создаётся (как unread).
	seed func() (map[string]R, error)

	errNewer, errUnread, errFull error
}

// store — записи одного файла в папке данных dir. Без папки (dir == "") живут в памяти до
// выхода. Все методы зовут под sharesMu.
type store[R any] struct {
	kind     storeKind[R]
	dir      string
	now      func() time.Time
	mem      map[string]R // dir == "": записи без файла
	readonly bool         // битый файл не уводится в .bad: shares.json при импорте не трогаем

	qlogged atomic.Bool // отказ отложить битый файл уже в журнале: не повторять при каждом чтении (T3-7)
}

func newStore[R any](dir string, kind storeKind[R]) store[R] {
	return store[R]{kind: kind, dir: dir, now: time.Now, mem: map[string]R{}}
}

// path — файл записей; пусто, если папки данных нет.
func (s *store[R]) path() string {
	if s.dir == "" {
		return ""
	}
	return filepath.Join(s.dir, s.kind.file)
}

// sharesState — как прочитался файл.
type sharesState struct {
	newer  bool  // версия новее программы: пишем нельзя
	unread error // файл есть, но не открылся (замок антивируса): записи неизвестны
}

// load читает записи под sharesMu (вызывающий держит замок). Битый или слишком большой файл
// уходит в <файл>.bad, записей нет; негодные записи отбрасываются.
func (s *store[R]) load() (map[string]R, sharesState) {
	if s.dir == "" {
		return s.mem, sharesState{}
	}
	path := s.path()
	data, err := readLimited(path, maxSharesFile)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return s.absent()
	case err != nil && !errors.Is(err, errSharesTooBig):
		log.Printf("%s не прочитан: %v", s.kind.file, paths.NoPath(err))
		return map[string]R{}, sharesState{unread: err}
	}
	var (
		version int
		items   map[string]json.RawMessage
	)
	if err == nil {
		version, items, err = s.kind.decode(data)
	}
	if err != nil {
		// Не разобрать: если в начале файла версия новее программы, файл не битый, а чужой —
		// его не трогаем (только чтение, записей нет).
		if peekShareVersion(data) > sharesVersion {
			log.Printf("%s версии новее программы не прочитан целиком: только чтение", s.kind.file)
			return map[string]R{}, sharesState{newer: true}
		}
		return s.quarantine(path, err)
	}
	newer := version > sharesVersion
	switch {
	case version < 1:
		return s.quarantine(path, errors.New("нет версии"))
	case len(items) > maxShares && !newer:
		return s.quarantine(path, errors.New("записей больше предела"))
	case len(items) > maxShares: // файл новее: читаем что можно, в .bad не уводим
		keep := slices.Sorted(maps.Keys(items))[:maxShares]
		cut := make(map[string]json.RawMessage, maxShares)
		for _, k := range keep {
			cut[k] = items[k]
		}
		items = cut
	}
	recs := make(map[string]R, len(items))
	for key, raw := range items {
		var rec R
		if !s.kind.key(key) || json.Unmarshal(raw, &rec) != nil {
			continue
		}
		if rec, ok := s.kind.clean(rec); ok {
			recs[key] = rec
		}
	}
	if dropped := len(items) - len(recs); dropped > 0 {
		log.Printf("%s: отброшено записей с негодными полями: %d", s.kind.file, dropped)
	}
	return recs, sharesState{newer: newer}
}

// absent — файла нет: записей нет, кроме посеянных (импорт). Посеянное ложится на диск сразу;
// не легло — остаётся в памяти этого чтения и повторится при следующем. Источник не открылся —
// unread: запись не создаст файл и не оборвёт импорт.
func (s *store[R]) absent() (map[string]R, sharesState) {
	if s.kind.seed == nil {
		return map[string]R{}, sharesState{}
	}
	recs, err := s.kind.seed()
	if err != nil {
		return map[string]R{}, sharesState{unread: err}
	}
	if len(recs) == 0 {
		return map[string]R{}, sharesState{}
	}
	if err := s.save(recs); err != nil {
		log.Printf("%s: импорт не записан: %v", s.kind.file, err)
	}
	return recs, sharesState{}
}

// quarantine уводит битый файл в <файл>.bad и отдаёт то же, что при отсутствии файла (absent:
// пусто или импорт). Занятое имя .bad не затирается (прежняя поломка — улика): тогда файл идёт в
// <файл>.bad.<ГГГГММДД-ччммсс>. Не вышло переименовать —
// файл не трогаем и записи не разрешаем (unread): иначе первая запись затёрла бы его без копии.
// Только чтение (readonly) — файл остаётся как есть, записей нет.
func (s *store[R]) quarantine(path string, cause error) (map[string]R, sharesState) {
	if s.readonly {
		if s.qlogged.CompareAndSwap(false, true) {
			log.Printf("%s не разобран (%v): не трогаем", s.kind.file, paths.NoPath(cause))
		}
		return map[string]R{}, sharesState{}
	}
	dst := s.badName(path)
	if rerr := renameFile(path, dst); rerr != nil {
		// Файл на месте и разбирается заново при каждом чтении: две строки журнала — один раз.
		if s.qlogged.CompareAndSwap(false, true) {
			log.Printf("%s не разобран (%v) и не отложен: %v", s.kind.file, paths.NoPath(cause), paths.NoPath(rerr))
		}
		return map[string]R{}, sharesState{unread: rerr}
	}
	s.qlogged.Store(false)
	log.Printf("%s не разобран (%v): отложен в %s", s.kind.file, paths.NoPath(cause), strings.TrimPrefix(dst, path))
	recs, st := s.absent() // файла теперь нет: 167-27 — импорт, как при первом запуске
	if s.kind.seed != nil && st.unread == nil {
		log.Printf("%s: после карантина повторный импорт, записей: %d", s.kind.file, len(recs))
	}
	return recs, st
}

// badName — свободное имя для битого файла: <файл>.bad, а если занято — со временем и, при
// совпадении, номером.
func (s *store[R]) badName(path string) string {
	dst := path + ".bad"
	if !pathTaken(dst) {
		return dst
	}
	base := path + ".bad." + s.now().Format("20060102-150405")
	dst = base
	for n := 2; pathTaken(dst); n++ {
		dst = base + "-" + strconv.Itoa(n)
	}
	return dst
}

// pathTaken — по пути что-то лежит (в том числе битая ссылка); ошибка, кроме «нет», тоже считается занятым.
func pathTaken(p string) bool {
	_, err := os.Lstat(p)
	return !errors.Is(err, os.ErrNotExist)
}

// modify — чтение, правка и запись под замком. Файл новее программы — errNewer без записи; не
// открылся — errUnread (его не затирают пустым). Изменений нет — файл не пишется.
func (s *store[R]) modify(fn func(recs map[string]R) error) error {
	sharesMu.Lock()
	defer sharesMu.Unlock()
	recs, st := s.load()
	switch {
	case st.newer:
		return s.kind.errNewer
	case st.unread != nil:
		return s.kind.errUnread
	}
	snap := maps.Clone(recs)
	s.expire(recs) // до fn: давно истёкшую запись fn не видит занятой (PutIfAbsent)
	if err := fn(recs); err != nil {
		return err
	}
	if s.sameRecs(snap, recs) {
		return nil
	}
	return s.save(recs)
}

// expire выбрасывает записи, чей срок истёк больше sharesKeepGone назад (разбор №29).
func (s *store[R]) expire(recs map[string]R) {
	now := s.now()
	for k, r := range recs {
		if at := s.kind.expires(r); !at.IsZero() && now.Sub(at) > sharesKeepGone {
			delete(recs, k)
		}
	}
}

func (s *store[R]) sameRecs(a, b map[string]R) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if w, ok := b[k]; !ok || !s.kind.same(v, w) {
			return false
		}
	}
	return true
}

// renameFile — os.Rename; подменяется в тестах (отказ переименования иначе не изобразить).
var renameFile = os.Rename

// reShareVersion — поле version в начале файла: пишется первым.
var reShareVersion = regexp.MustCompile(`^\s*\{\s*"version"\s*:\s*(\d{1,9})`)

// peekShareVersion — версия из первых 4 КБ файла, который не разобрать целиком (больше предела
// или усечён); 0 — не нашлась.
func peekShareVersion(data []byte) int {
	data = data[:min(len(data), 4096)]
	m := reShareVersion.FindSubmatch(data)
	if m == nil {
		return 0
	}
	n, _ := strconv.Atoi(string(m[1]))
	return n
}

var errSharesTooBig = errors.New("файл записей больше предела")

// readFault — шов только для тестов: ошибка открытия файла записей вместо настоящего чтения.
// Замок антивируса не изобразить одинаково на всех ОС (chmod 0 на Windows чтение не запрещает).
// Атомарный указатель: чтения идут из горутин других тестов.
var readFault atomic.Pointer[func(path string) error]

// setReadFault — только для тестов: fault(path) != nil — файл path «не открылся»; restore
// возвращает прежнее.
func setReadFault(fault func(path string) error) (restore func()) {
	old := readFault.Swap(&fault)
	return func() { readFault.Store(old) }
}

// readLimited читает файл не больше limit байт; больше — errSharesTooBig.
func readLimited(path string, limit int64) ([]byte, error) {
	if fault := readFault.Load(); fault != nil {
		if err := (*fault)(path); err != nil {
			return nil, err
		}
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err == nil && int64(len(data)) > limit {
		err = errSharesTooBig
	}
	return data, err
}

// save пишет записи атомарно, 0600. Файл больше предела не пишется (errFull): при следующем
// чтении он ушёл бы в .bad. Без папки данных записи остаются в памяти.
func (s *store[R]) save(recs map[string]R) error {
	if s.dir == "" {
		s.mem = recs
		return nil
	}
	items := make(map[string]json.RawMessage, len(recs))
	for k, r := range recs {
		raw, err := json.Marshal(r) //nolint:gosec // пароль клиента лежит открытым по решению владельца: «скопировать снова» (design §6 п. 6)
		if err != nil {
			return fmt.Errorf("запись о выгрузке: %w", err)
		}
		items[k] = raw
	}
	data, err := s.kind.encode(items)
	if err != nil {
		return fmt.Errorf("%s: %w", s.kind.file, err)
	}
	if len(data) > maxSharesFile {
		return s.kind.errFull
	}
	if err := library.WriteFileAtomicMode(s.path(), data, privateMode); err != nil {
		log.Printf("%s: %v", s.kind.file, paths.NoPath(err))
		return errors.New("не удалось сохранить записи о выгрузках")
	}
	return nil
}
