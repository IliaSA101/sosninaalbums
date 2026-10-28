package guidevideo

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/decor/fetch"
)

// List — ролики из сохранённого каталога и состояние файлов; сети нет. Нет каталога или служба
// выключена — пустой список (не nil).
func (s *Service) List() []Item {
	items := []Item{}
	dir := s.dir()
	cat, ok := s.h.Cached()
	if dir == "" || !ok {
		return items
	}
	m := pick(cat, s.h.App(), s.h.OS())
	for _, id := range ids(m) {
		c := m[id]
		it := Item{ID: id, Version: c.video.Version, Seconds: c.video.Seconds, Size: c.file.Size}
		if name, ok := fileName(id, c.video.Version, c.kind); ok {
			st, err := os.Lstat(filepath.Join(dir, name))
			it.Ready = err == nil && st.Mode().IsRegular() && st.Size() == c.file.Size
		}
		items = append(items, it)
	}
	return items
}

// Download — скачивает ролики ids (id статей F1) в кэш. Порядок: служба включена → согласие →
// id по грамматике (мусор — ErrGone без сети) → замок → begin (повторная сверка согласия) → папка →
// свежий каталог → по одному ролику. Первая ошибка обрывает ход: ролики маленькие, второй запрос после
// отказа лишний (урок 137). Итог — весь список; при ошибке он тоже отдаётся (окно обновит строки).
func (s *Service) Download(want []string) ([]Item, error) {
	dir := s.dir()
	if dir == "" {
		return s.List(), ErrUnavailable
	}
	if !s.h.Allowed() {
		return s.List(), ErrNoConsent
	}
	list, ok := cleanIDs(want)
	if !ok {
		return s.List(), ErrGone
	}
	if len(list) == 0 {
		return s.List(), nil
	}
	if !s.mu.TryLock() {
		return s.List(), ErrBusy
	}
	defer s.mu.Unlock()
	ctx, done, err := s.begin()
	if err != nil {
		return s.List(), err
	}
	defer done()
	// Папка раньше сети: папка-ссылка — ErrUnavailable без единого запроса.
	if err := s.prepare(dir); err != nil {
		return s.List(), err
	}
	cat, err := s.h.Fresh(ctx)
	if err != nil {
		logErr("каталог", err)
		return s.List(), human(err, false)
	}
	m := pick(cat, s.h.App(), s.h.OS())
	for _, id := range list {
		if ctx.Err() != nil {
			return s.List(), ErrCancelled
		}
		if err := s.fetchOne(ctx, dir, id, m); err != nil {
			return s.List(), err
		}
	}
	return s.List(), nil
}

// cleanIDs — id от интерфейса без повторов; негодный по грамматике или больше MaxVideos — ok == false.
func cleanIDs(in []string) (out []string, ok bool) {
	seen := make(map[string]bool, min(len(in), fetch.MaxVideos))
	for _, id := range in {
		if !fetch.ValidID(id) {
			return nil, false
		}
		if seen[id] {
			continue
		}
		if seen[id] = true; len(seen) > fetch.MaxVideos {
			return nil, false
		}
		out = append(out, id)
	}
	return out, true
}

// prepare — папка роликов (0700), не ссылка; остатки своих gv-*.tmp убираются (под замком службы).
func (s *Service) prepare(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return ErrVideoDisk
	}
	if st, err := os.Lstat(dir); err != nil || !st.IsDir() {
		return ErrUnavailable
	}
	list, err := os.ReadDir(dir)
	if err != nil {
		return ErrVideoDisk
	}
	for _, e := range list {
		if e.Type().IsRegular() && isTmp(e.Name()) {
			_ = os.Remove(filepath.Join(dir, e.Name())) // не вышло — доберёт следующая загрузка
		}
	}
	return nil
}

// isTmp — имя временного файла службы: «gv-<случайное>.tmp» из CreateTemp.
func isTmp(name string) bool {
	return len(name) > len("gv-.tmp") && strings.HasPrefix(name, "gv-") && strings.HasSuffix(name, ".tmp")
}

// fetchOne — один ролик: id нет в свежем каталоге — ErrGone; готовый (размер и sha256) пропуск;
// иначе временный файл → Client.Download → Rename → чистка прежних версий этого id.
func (s *Service) fetchOne(ctx context.Context, dir, id string, m map[string]choice) error {
	c, ok := m[id]
	if !ok {
		return ErrGone
	}
	name, ok := fileName(id, c.video.Version, c.kind)
	// Предел и sha256 повторяем здесь: каталог уже проверен fetch, но служба не должна от этого зависеть.
	if !ok || c.file.Size < 1 || c.file.Size > fetch.MaxVideoFile || !validSum(c.file.SHA256) {
		return ErrGone
	}
	final := filepath.Join(dir, name)
	if s.verified(final, c.file) {
		s.cleanOld(dir, id, name)
		return nil
	}
	_ = os.Remove(final)                                                  // битый, подмененный или чужого вида: заменяем (папка непустая — Rename скажет)
	if n, err := s.h.Free(dir); err == nil && n < uint64(c.file.Size)*2 { //nolint:gosec // размер ≤ 16 МБ
		return ErrVideoDisk
	}
	f, err := os.CreateTemp(dir, "gv-*.tmp")
	if err != nil {
		return ErrVideoDisk
	}
	tmp := f.Name()
	// Дескриптор закрыт до Download: тот открывает путь заново.
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return ErrVideoDisk
	}
	defer func() { _ = os.Remove(tmp) }() // после удачного Rename файла уже нет
	e := fetch.Entry{Title: id, Size: c.file.Size, SHA256: c.file.SHA256, URL: c.file.URL}
	if err := s.h.Client().Download(ctx, e, tmp, s.progress(id, c.file.Size)); err != nil {
		logErr(id, err)
		return human(err, true)
	}
	if err := s.h.Rename(tmp, final); err != nil {
		return ErrVideoDisk
	}
	s.cleanOld(dir, id, name)
	return nil
}

// verified — файл по пути сходится с записью каталога: обычный (не ссылка), размер и sha256
// (как update.Verify; открытие — через Host, без ожидания на канале).
func (s *Service) verified(path string, f fetch.VideoFile) bool {
	lst, err := os.Lstat(path)
	if err != nil || !lst.Mode().IsRegular() || lst.Size() != f.Size {
		return false
	}
	_ = os.SameFile(lst, lst) // Windows читает номер файла лениво по пути
	r, err := s.h.OpenRegular(path)
	if err != nil {
		return false
	}
	defer r.Close() //nolint:errcheck // чтение: ошибка закрытия ничего не теряет
	if st, err := r.Stat(); err != nil || !os.SameFile(lst, st) {
		return false
	}
	h := sha256.New()
	if n, err := io.Copy(h, io.LimitReader(r, f.Size+1)); err != nil || n != f.Size {
		return false
	}
	return hex.EncodeToString(h.Sum(nil)) == f.SHA256
}

// cleanOld — прежние версии этого id: только обычные файлы с разбором имени, чей id равен своему
// (cover-export — другая статья, security Н1). Ошибка удаления (Windows: старый ролик отдаётся)
// не ошибка загрузки — остаток доберёт следующая.
func (s *Service) cleanOld(dir, id, keep string) {
	list, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range list {
		pid, _, _, ok := parseName(e.Name())
		if ok && pid == id && e.Name() != keep && e.Type().IsRegular() {
			_ = os.Remove(filepath.Join(dir, e.Name()))
		}
	}
}

// progress — guide:video: первый кусок и конец — сразу, ход байтов — не чаще progressGap.
func (s *Service) progress(id string, total int64) func(done int64) {
	var last time.Time
	return func(done int64) {
		now := time.Now()
		if last.IsZero() || done >= total || now.Sub(last) >= progressGap {
			last = now
			s.h.Emit("guide:video", Progress{ID: id, Done: done, Total: total})
		}
	}
}

// logErr — строка log.txt о неудаче скачивания (что делали и ошибка fetch; в ней нет ни путей,
// ни адресов, как у наборов — libpacks.logNetErr). Отмена пользователем не ошибка.
func logErr(what string, err error) {
	if errors.Is(err, fetch.ErrCancelled) || errors.Is(err, ErrCancelled) || errors.Is(err, context.Canceled) {
		return
	}
	log.Printf("ролики: %s — %v", what, err)
}

// human — ошибка fetch словами про ролик: тексты fetch говорят о наборах (security Н10). file —
// ошибка скачивания файла: 404 на адресе ролика значит «ролик ещё не выложен».
func human(err error, file bool) error {
	switch {
	case errors.Is(err, fetch.ErrCancelled):
		return ErrCancelled
	case errors.Is(err, fetch.ErrTLS): // тексты fetch говорят «обновление отменено», а адрес в них не нужен
		return ErrVideoTLS
	case errors.Is(err, fetch.ErrAddress):
		return ErrVideoAddr
	case !file:
		return err
	case errors.Is(err, fetch.ErrBroken):
		return ErrVideoBroken
	case errors.Is(err, fetch.ErrDisk):
		return ErrVideoDisk
	case errors.Is(err, fetch.ErrSlow):
		return ErrVideoSlow
	case errors.Is(err, fetch.ErrNotFound):
		return ErrGone
	}
	return err
}
