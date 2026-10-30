package libpacks

import (
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/decor/fetch"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// ServePreview — GET|HEAD /media/lp/<packID>/<kind>/<i>; tail — хвост адреса после префикса
// (r.URL.Path уже раскодирован: «%2e%2e» пришло как «..», «a%2Fb» — как два сегмента). Коды: 405 — не
// GET/HEAD; 404 — всё остальное неизвестное (хвост не из трёх сегментов, id вне грамматики, набор
// уже стоит или его нет в списке, согласия на сеть нет, вид не из пяти, индекс не «0…n-1» без знака и нулей, лист не
// лежит или не годится); иначе 200 с клеткой, перекодированной Go. Сети и путей из адреса нет:
// набор, ряд и лист берёт Cell из сохранённого списка. `?v=` и `px` не читаются. Заголовки: без
// ServeContent — If-* и Range игнорируются, 304 не бывает (Wails на WebView2 превращает 304 в 500).
func (s *Service) ServePreview(w http.ResponseWriter, r *http.Request, tail string) {
	// Паника разбора — 404 (общий guard медиа ответил бы пустым 200, который WebView принял бы за
	// картинку). Заголовки пишутся после получения клетки, поэтому до них 404 ещё возможен.
	defer func() {
		if rec := recover(); rec != nil {
			// Напрямую в журнал: у prevLog общий с заданием счётчик, и паника после строки задания пропала бы.
			log.Printf("библиотека: образцы — раздача клетки: сбой (%T)", rec)
			http.NotFound(w, r)
		}
	}()
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "метод не поддерживается", http.StatusMethodNotAllowed)
		return
	}
	// Без согласия плиток нет — и клеток тоже; согласие из памяти службы (SetNet обновляет сразу).
	var id, kind string
	var i int
	ok := s.allowed()
	if ok {
		id, kind, i, ok = s.parseTail(tail)
	}
	if !ok {
		s.missLog()
		http.NotFound(w, r)
		return
	}
	data, err := s.Cell(r.Context(), id, kind, i)
	switch {
	case err == nil:
	case errors.Is(err, ErrNoCell):
		s.missLog()
		http.NotFound(w, r)
		return
	default:
		http.NotFound(w, r) // запрос ушёл (плитка покинула экран): иначе net/http отдал бы пустой 200
		return
	}
	h := w.Header()
	h.Set("Content-Type", "image/jpeg")
	h.Set("Content-Length", strconv.Itoa(len(data)))
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_, _ = w.Write(data) // обрыв клиента — не сбой службы
	}
}

// parseTail — <id>/<kind>/<i>: ровно три сегмента, id по грамматике каталога и не имя устройства,
// набор не стоит в библиотеке (плитка его уже не покажет), вид из пяти, индекс — десятичный без
// знака и нулей (-1, +1, 01 — отказ).
func (s *Service) parseTail(tail string) (id, kind string, i int, ok bool) {
	parts := strings.Split(tail, "/")
	if len(parts) != 3 {
		return "", "", 0, false
	}
	id, kind = parts[0], parts[1]
	if !fetch.ValidID(id) || paths.Device(id) || tileLimit[kind] == 0 {
		return "", "", 0, false
	}
	n, err := strconv.Atoi(parts[2])
	if err != nil || n < 0 || strconv.Itoa(n) != parts[2] {
		return "", "", 0, false
	}
	if st, err := s.h.Store(); err == nil {
		for _, p := range st.Packs() {
			if p.ID == id {
				return "", "", 0, false
			}
		}
	}
	return id, kind, n, true
}

// missLog — промах раздачи в журнал: первый и каждый сотый по своему счётчику (промахов бывают
// сотни), без адреса; общий с заданием счётчик prevLog не трогает.
func (s *Service) missLog() {
	if n := s.lpMiss.Add(1); n == 1 || n%100 == 0 {
		log.Printf("библиотека: образцы — раздача клетки: нет такой (%d)", n)
	}
}
