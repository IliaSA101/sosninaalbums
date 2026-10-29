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
	"sync"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// sharesMu — shares.json читается и пишется целиком под одним замком на процесс: ход
// выгрузки и «Изменить» пишут одновременно (разбор №29).
var sharesMu sync.Mutex

// SharesPath — shares.json в папке данных dir; пустая dir — папки нет, путь пуст.
func SharesPath(dir string) string {
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "shares.json")
}

// sharesFile — содержимое shares.json.
type sharesFile struct {
	Version int                        `json:"version"`
	Albums  map[string]json.RawMessage `json:"albums"`
}

// Shares — записи выгрузок в папке данных dir. Без папки данных (dir == "") записи живут в
// памяти до выхода. Правила адресов даёт вызывающий (ShareRules).
type Shares struct {
	dir   string
	rules ShareRules
	now   func() time.Time
	mem   map[string]ShareRecord // dir == "": записи без файла
}

// NewShares — записи выгрузок папки данных dir.
func NewShares(dir string, rules ShareRules) *Shares {
	return &Shares{dir: dir, rules: rules, now: time.Now, mem: map[string]ShareRecord{}}
}

// shareKey — ключ записи: путь альбома после Clean и paths.Fold. Нелокальный путь — ошибка.
func shareKey(album string) (string, error) {
	if paths.CheckLocalPath(album) != nil {
		return "", ErrSharePath
	}
	return paths.Fold(filepath.Clean(album)), nil
}

// Get — запись альбома; нет записи (или путь негоден) — false.
func (s *Shares) Get(album string) (ShareRecord, bool) {
	key, err := shareKey(album)
	if err != nil {
		return ShareRecord{}, false
	}
	sharesMu.Lock()
	defer sharesMu.Unlock()
	recs, _ := s.load()
	rec, ok := recs[key]
	return rec, ok
}

// Put пишет запись альбома (заменяет прежнюю). Запись не по правилам — ErrShareRecord, файл
// не тронут.
func (s *Shares) Put(album string, rec ShareRecord) error {
	key, err := shareKey(album)
	if err != nil {
		return err
	}
	rec, ok := s.rules.clean(rec)
	if !ok {
		return ErrShareRecord
	}
	return s.modify(func(recs map[string]ShareRecord) error {
		if _, had := recs[key]; !had && len(recs) >= maxShares {
			return ErrSharesFull
		}
		recs[key] = rec
		return nil
	})
}

// Update меняет запись альбома под замком: fn получает копию, результат проверяется по
// правилам и пишется. Записи нет — ok == false, fn не зовётся, файл не пишется.
func (s *Shares) Update(album string, fn func(rec *ShareRecord)) (rec ShareRecord, ok bool, err error) {
	key, err := shareKey(album)
	if err != nil {
		return ShareRecord{}, false, err
	}
	err = s.modify(func(recs map[string]ShareRecord) error {
		cur, had := recs[key]
		if !had {
			return nil
		}
		fn(&cur)
		var good bool
		if cur, good = s.rules.clean(cur); !good {
			return ErrShareRecord
		}
		recs[key], rec, ok = cur, cur, true
		return nil
	})
	if err != nil {
		return ShareRecord{}, false, err
	}
	return rec, ok, nil
}

// Delete убирает запись альбома; записи нет — не ошибка, файл не пишется.
func (s *Shares) Delete(album string) error {
	key, err := shareKey(album)
	if err != nil {
		return err
	}
	return s.modify(func(recs map[string]ShareRecord) error {
		delete(recs, key)
		return nil
	})
}

// modify — чтение, правка и запись под замком. Файл новее программы — ErrSharesNewer без
// записи; не открылся — ошибка (его не затирают пустым). Изменений нет — файл не пишется.
func (s *Shares) modify(fn func(recs map[string]ShareRecord) error) error {
	sharesMu.Lock()
	defer sharesMu.Unlock()
	recs, st := s.load()
	switch {
	case st.newer:
		return ErrSharesNewer
	case st.unread != nil:
		return ErrSharesUnread
	}
	before := len(recs)
	snap := make(map[string]ShareRecord, before)
	for k, v := range recs {
		snap[k] = v
	}
	if err := fn(recs); err != nil {
		return err
	}
	s.expire(recs)
	if sameRecs(snap, recs) {
		return nil
	}
	return s.save(recs)
}

// expire выбрасывает записи, чей срок истёк больше sharesKeepGone назад (разбор №29).
func (s *Shares) expire(recs map[string]ShareRecord) {
	now := s.now()
	for k, r := range recs {
		if !r.ExpiresAt.IsZero() && now.Sub(r.ExpiresAt) > sharesKeepGone {
			delete(recs, k)
		}
	}
}

func sameRecs(a, b map[string]ShareRecord) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if w, ok := b[k]; !ok || !sameRec(v, w) {
			return false
		}
	}
	return true
}

func sameRec(a, b ShareRecord) bool {
	return a.Origin == b.Origin && a.ID == b.ID && a.URL == b.URL && a.Title == b.Title &&
		a.Password == b.Password && a.ExpiresAt.Equal(b.ExpiresAt) && a.PublishedAt.Equal(b.PublishedAt) &&
		a.Scene == b.Scene && a.Gone == b.Gone
}

// sharesState — как прочитался файл.
type sharesState struct {
	newer  bool  // версия новее программы: пишем нельзя
	unread error // файл есть, но не открылся (замок антивируса): записи неизвестны
}

// load читает записи под sharesMu (вызывающий держит замок). Битый или слишком большой файл
// уходит в shares.json.bad, записей нет; негодные записи отбрасываются.
func (s *Shares) load() (map[string]ShareRecord, sharesState) {
	if s.dir == "" {
		return s.mem, sharesState{}
	}
	path := SharesPath(s.dir)
	data, err := readLimited(path, maxSharesFile)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return map[string]ShareRecord{}, sharesState{}
	case err != nil && !errors.Is(err, errSharesTooBig):
		log.Printf("shares.json не прочитан: %v", paths.NoPath(err))
		return map[string]ShareRecord{}, sharesState{unread: err}
	}
	var f sharesFile
	if err == nil {
		err = json.Unmarshal(data, &f)
	}
	if err != nil {
		// Не разобрать: если в начале файла версия новее программы, файл не битый, а чужой —
		// его не трогаем (только чтение, записей нет).
		if peekShareVersion(data) > sharesVersion {
			log.Printf("shares.json версии новее программы не прочитан целиком: только чтение")
			return map[string]ShareRecord{}, sharesState{newer: true}
		}
		return s.quarantine(path, err)
	}
	newer := f.Version > sharesVersion
	switch {
	case f.Version < 1:
		return s.quarantine(path, errors.New("нет версии"))
	case len(f.Albums) > maxShares && !newer:
		return s.quarantine(path, errors.New("записей больше предела"))
	case len(f.Albums) > maxShares: // файл новее: читаем что можно, в .bad не уводим
		keep := slices.Sorted(maps.Keys(f.Albums))[:maxShares]
		cut := make(map[string]json.RawMessage, maxShares)
		for _, k := range keep {
			cut[k] = f.Albums[k]
		}
		f.Albums = cut
	}
	recs := make(map[string]ShareRecord, len(f.Albums))
	for key, raw := range f.Albums {
		var rec ShareRecord
		if k, kerr := shareKey(key); kerr != nil || k != key || json.Unmarshal(raw, &rec) != nil {
			continue
		}
		if rec, ok := s.rules.clean(rec); ok {
			recs[key] = rec
		}
	}
	if dropped := len(f.Albums) - len(recs); dropped > 0 {
		log.Printf("shares.json: отброшено записей с негодными полями: %d", dropped)
	}
	return recs, sharesState{newer: newer}
}

// quarantine уводит битый shares.json в shares.json.bad и отдаёт пустой список. Не вышло
// переименовать — файл не трогаем и записи не разрешаем (unread): иначе первая запись затёрла бы
// его без копии.
func (s *Shares) quarantine(path string, cause error) (map[string]ShareRecord, sharesState) {
	log.Printf("shares.json не разобран (%v): отложен в shares.json.bad", paths.NoPath(cause))
	if rerr := renameFile(path, path+".bad"); rerr != nil {
		log.Printf("shares.json: не отложен: %v", paths.NoPath(rerr))
		return map[string]ShareRecord{}, sharesState{unread: rerr}
	}
	return map[string]ShareRecord{}, sharesState{}
}

// renameFile — os.Rename; подменяется в тестах (отказ переименования иначе не изобразить).
var renameFile = os.Rename

// reShareVersion — поле version в начале файла: пишется первым (sharesFile).
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

var errSharesTooBig = errors.New("shares.json больше предела")

// readLimited читает файл не больше limit байт; больше — errSharesTooBig.
func readLimited(path string, limit int64) ([]byte, error) {
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

// save пишет записи атомарно, 0600. Файл больше предела не пишется (ErrSharesFull): при
// следующем чтении он ушёл бы в .bad. Без папки данных записи остаются в памяти.
func (s *Shares) save(recs map[string]ShareRecord) error {
	if s.dir == "" {
		s.mem = recs
		return nil
	}
	f := sharesFile{Version: sharesVersion, Albums: make(map[string]json.RawMessage, len(recs))}
	for k, r := range recs {
		raw, err := json.Marshal(r) //nolint:gosec // пароль клиента лежит открытым по решению владельца: «скопировать снова» (design §6 п. 6)
		if err != nil {
			return fmt.Errorf("запись о выгрузке: %w", err)
		}
		f.Albums[k] = raw
	}
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return fmt.Errorf("shares.json: %w", err)
	}
	if len(data) > maxSharesFile {
		return ErrSharesFull
	}
	if err := library.WriteFileAtomicMode(SharesPath(s.dir), data, privateMode); err != nil {
		log.Printf("shares.json: %v", paths.NoPath(err))
		return errors.New("не удалось сохранить записи о выгрузках")
	}
	return nil
}
