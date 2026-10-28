package guidevideo

import (
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/IliaSA101/sosninaalbums/internal/decor/fetch"
)

// contentTypes — тип по виду файла: у Go нет .webm во встроенной таблице, а угадывать по байтам
// недоверенного файла нельзя.
var contentTypes = map[string]string{fetch.KindWebM: "video/webm", fetch.KindMP4: "video/mp4"}

// Serve — GET|HEAD /media/guide-video/<id>; tail — хвост адреса после префикса. Коды: 405 — не
// GET/HEAD; 404 — служба выключена, хвост не один id по грамматике, id нет в каталоге или файла
// нет/он не тот; иначе 200 с целым файлом (Range и If-* игнорируются). Путь собирает Go из id и
// версии каталога — из адреса берётся только id.
func (s *Service) Serve(w http.ResponseWriter, r *http.Request, tail string) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "метод не поддерживается", http.StatusMethodNotAllowed)
		return
	}
	dir := s.dir()
	if dir == "" || !fetch.ValidID(tail) {
		http.NotFound(w, r)
		return
	}
	cat, ok := s.h.Cached()
	c, found := pick(cat, s.h.App(), s.h.OS())[tail]
	if !ok || !found {
		http.NotFound(w, r)
		return
	}
	name, ok := fileName(tail, c.video.Version, c.kind)
	if !ok {
		http.NotFound(w, r)
		return
	}
	path := filepath.Join(dir, name)
	lst, err := os.Lstat(path)
	if err != nil || !lst.Mode().IsRegular() || lst.Size() != c.file.Size {
		if err == nil || !os.IsNotExist(err) { // не скачан — не сбой
			s.logMiss("ролик: файл не годен")
		}
		http.NotFound(w, r)
		return
	}
	_ = os.SameFile(lst, lst) // Windows читает номер файла лениво по пути: сверка открытого с проверенным
	select {
	case s.slots <- struct{}{}:
		defer func() { <-s.slots }()
	case <-r.Context().Done():
		return
	}
	f, err := s.h.OpenRegular(path)
	if err != nil {
		s.logMiss("ролик: файл не открылся")
		http.NotFound(w, r)
		return
	}
	defer f.Close() //nolint:errcheck // чтение: ошибка закрытия ничего не теряет
	if st, err := f.Stat(); err != nil || !os.SameFile(lst, st) || !st.Mode().IsRegular() || st.Size() != c.file.Size {
		s.logMiss("ролик: файл сменился")
		http.NotFound(w, r)
		return
	}
	// Всегда 200 и весь файл (спайк Т0, 28.10): WebKitGTK по Range падает, окно берёт ролик через
	// fetch целиком; Range и If-* не читаются вовсе, 304 и 206 невозможны (Wails на WebView2 304
	// превращает в 500). Ролик ≤ 16 МБ, нормальный — до 2 МБ.
	h := w.Header()
	h.Set("Content-Type", contentTypes[c.kind])
	h.Set("Content-Length", strconv.FormatInt(c.file.Size, 10))
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodHead {
		return
	}
	_, _ = io.Copy(w, io.LimitReader(f, c.file.Size)) // обрыв клиента — не сбой службы
}

// logMiss — отказ раздачи в журнал: первый и каждый сотый, без пути (промахов бывают сотни).
func (s *Service) logMiss(msg string) {
	if n := s.miss.Add(1); n == 1 || n%100 == 0 {
		log.Printf("[x%d] %s", n, msg)
	}
}

// RevealPath — путь готового файла ролика id для проводника. Путь собирает Go, от окна только
// id; файл — обычный (не ссылка) и текущей версии, иначе ErrNoFile и проводник не зовётся.
func (s *Service) RevealPath(id string) (string, error) {
	dir := s.dir()
	if dir == "" {
		return "", ErrUnavailable
	}
	cat, ok := s.h.Cached()
	c, found := pick(cat, s.h.App(), s.h.OS())[id]
	if !fetch.ValidID(id) || !ok || !found {
		return "", ErrNoFile
	}
	name, ok := fileName(id, c.video.Version, c.kind)
	if !ok {
		return "", ErrNoFile
	}
	path := filepath.Join(dir, name)
	if st, err := os.Lstat(path); err != nil || !st.Mode().IsRegular() {
		return "", ErrNoFile
	}
	return path, nil
}

// Reveal — «Показать файл»: проводник с выделенным роликом. Ошибка проводника — без пути в тексте.
func (s *Service) Reveal(id string) error {
	path, err := s.RevealPath(id)
	if err != nil {
		return err
	}
	if err := s.h.Reveal(path); err != nil {
		log.Printf("ролики: проводник не открылся (%T)", err)
		return ErrReveal
	}
	return nil
}
