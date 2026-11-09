package prefs

import (
	"encoding/json"
	"path/filepath"
	"sync"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// sharesMu — shares.json и projects.json читаются и пишутся целиком под одним замком на
// процесс: ход выгрузки и «Изменить» пишут одновременно (разбор №29); импорт projects.json
// читает shares.json под тем же замком.
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
// памяти до выхода. Правила адресов даёт вызывающий (ShareRules). С 0.38 только читаются
// программой (projects.json, 167-4): откат на 0.37 видит свои записи как раньше.
type Shares struct {
	store[ShareRecord]
	rules ShareRules
}

// NewShares — записи выгрузок папки данных dir.
func NewShares(dir string, rules ShareRules) *Shares {
	s := &Shares{rules: rules}
	s.store = newStore(dir, storeKind[ShareRecord]{
		file: "shares.json",
		decode: func(data []byte) (int, map[string]json.RawMessage, error) {
			var f sharesFile
			err := json.Unmarshal(data, &f)
			return f.Version, f.Albums, err
		},
		encode: func(items map[string]json.RawMessage) ([]byte, error) {
			return json.MarshalIndent(sharesFile{Version: sharesVersion, Albums: items}, "", "  ")
		},
		key: func(k string) bool {
			c, err := shareKey(k)
			return err == nil && c == k
		},
		clean:    rules.clean,
		same:     sameRec,
		expires:  func(r ShareRecord) time.Time { return r.ExpiresAt },
		errNewer: ErrSharesNewer, errUnread: ErrSharesUnread, errFull: ErrSharesFull,
	})
	return s
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

func sameRec(a, b ShareRecord) bool {
	return a.Origin == b.Origin && a.ID == b.ID && a.URL == b.URL && a.Title == b.Title &&
		a.Password == b.Password && a.ExpiresAt.Equal(b.ExpiresAt) && a.PublishedAt.Equal(b.PublishedAt) &&
		a.Scene == b.Scene && a.Gone == b.Gone && a.UID == b.UID
}
