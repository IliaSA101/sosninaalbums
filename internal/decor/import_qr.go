package decor

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"

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
	if s.alreadyHave(KindSticker, key) { // тот же код: запись одна, реестр не меняется
		s.mu.RLock()
		defer s.mu.RUnlock()
		return s.entryOf(KindSticker, key).item(), nil
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
	if changed {
		s.saveLogged()
	}
	return it, nil
}
