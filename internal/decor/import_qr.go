package decor

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// AddQR — код ссылки стикером в «Мои» под ключом от байтов PNG (my: + sha256[:8], как у файла):
// та же ссылка — та же запись, скрытая возвращается. Только чтение — ErrReadOnly; ошибки ссылки —
// из EncodeQR. Ссылка не открывается и путём не становится.
func (s *Store) AddQR(link string) (Item, error) {
	q, err := EncodeQR(link)
	if err != nil {
		return Item{}, err
	}
	data, err := q.PNG()
	if err != nil {
		return Item{}, err
	}
	sum := sha256.Sum256(data)
	key := libPrefix + hex.EncodeToString(sum[:8])
	s.ioMu.Lock()
	defer s.ioMu.Unlock()
	if s.ReadOnly() {
		return Item{}, ErrReadOnly
	}
	if s.alreadyHave(KindSticker, key) { // тот же код: запись одна; признак QR ставится и старой записи
		s.mu.Lock()
		e := s.entryOf(KindSticker, key)
		mark := !e.QR
		e.QR = true
		it := e.item()
		s.mu.Unlock()
		if mark {
			s.saveLogged()
		}
		return it, nil
	}
	dir := filepath.Join(s.root, kindDirs[KindSticker])
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Item{}, paths.NoPath(err)
	}
	tmp, err := writeTemp(dir, data)
	if err != nil {
		return Item{}, err
	}
	defer func() { _ = os.Remove(tmp) }() // после rename его уже нет
	info, _, err := checkCopy(tmp)
	if err != nil {
		return Item{}, err
	}
	it, changed, err := s.commit(KindSticker, ThemeMine, qrName(q.Text), key, tmp, "png", info)
	if err != nil {
		return Item{}, err
	}
	s.mu.Lock()
	if e := s.entryOf(KindSticker, key); e != nil && !e.QR {
		e.QR, changed = true, true
	}
	s.mu.Unlock()
	if changed {
		s.saveLogged()
	}
	return it, nil
}

// isQR — запись ссылки клиента: признак QR, а у записей до признака (0.23–0.27) — имя «QR · …»
// от qrName у стикера. Переименованной старой записи признак ставит migrateQR при открытии (по PNG).
func (e *entry) isQR() bool {
	return e.QR || e.Kind == KindSticker && strings.HasPrefix(e.Name, qrNamePrefix)
}

// IsQR — ключ key — код ссылки (свой стикер из AddQR): в шаблон разворота и макет не берётся,
// ссылка клиента не должна уйти с макетом другим людям. Чужой и неизвестный ключ — false.
func (s *Store) IsQR(key string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e := s.entryOf(KindSticker, key)
	return e != nil && e.isQR()
}
