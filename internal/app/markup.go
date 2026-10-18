package app

import (
	"errors"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	wr "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// Разметка типографии (032, 066): выбор файла обложки и разворотов и раздача картинки.
// Путь — только из диалога Go и из проекта, от интерфейса и из запроса не принимается.

// Своя последняя папка у каждого диалога «Разметка типографии».
const (
	dirCoverGuide  = "coverGuide"
	dirSpreadGuide = "spreadGuide"
)

// markupFilter — фильтр обоих диалогов разметки.
var markupFilter = []wr.FileFilter{{DisplayName: "Разметка (*.psd, *.jpg, *.jpeg, *.png)", Pattern: "*.psd;*.jpg;*.jpeg;*.png"}}

// PickCoverGuide — «Выбрать файл…» разметки обложки: диалог в Go; файл проверяется так же
// строго, как свой фон (PSD — разбором заголовка). Отказ от диалога — вид без изменений.
func (a *App) PickCoverGuide() (v model.View, err error) {
	defer a.guardEdit("PickCoverGuide", &v, &err)
	return a.pickGuide(dirCoverGuide, a.setCoverGuide)
}

// PickSpreadGuide — «Выбрать файл…» разметки разворотов (066): как у обложки, своя папка диалога.
func (a *App) PickSpreadGuide() (v model.View, err error) {
	defer a.guardEdit("PickSpreadGuide", &v, &err)
	return a.pickGuide(dirSpreadGuide, a.setSpreadGuide)
}

// pickGuide — диалог «Разметка типографии» с папкой kind и правка set.
func (a *App) pickGuide(kind string, set func(path string) (model.View, error)) (model.View, error) {
	path, err := openFileDialog(a.ctx, wr.OpenDialogOptions{
		Title:            "Разметка типографии",
		DefaultDirectory: a.dialogDir(kind),
		Filters:          markupFilter,
	})
	if err != nil || path == "" {
		return a.editor().View(), err
	}
	return set(path)
}

// setCoverGuide — проверка выбранного файла разметки обложки и правка.
func (a *App) setCoverGuide(path string) (model.View, error) {
	return a.setGuide(path, dirCoverGuide, markupCoverSlot, (*album.Editor).SetCoverGuide)
}

// setSpreadGuide — проверка выбранного файла разметки разворотов и правка.
func (a *App) setSpreadGuide(path string) (model.View, error) {
	return a.setGuide(path, dirSpreadGuide, markupSpreadSlot, (*album.Editor).SetSpreadGuide)
}

// setGuide — файл проверяется library.MarkupFile (PSB, повреждённый PSD, не картинка — ошибка,
// правки нет), в проект идёт путь, по которому файл читается (у ссылки — её цель). Слот кэша
// превью сбрасывается при каждом выборе, в том числе повторном: разовый сбой чтения иначе держал бы
// 404 до изменения файла.
func (a *App) setGuide(path, kind string, slot int, set func(*album.Editor, string) (model.View, error)) (model.View, error) {
	a.markups[slot].reset()
	m, err := library.MarkupFile(path)
	if err != nil {
		return a.editor().View(), err
	}
	v, err := a.changedOrErr(set(a.editor(), m.Path))
	if err == nil {
		a.rememberDir(kind, filepath.Dir(m.Path))
	}
	return v, err
}

// maxGuideBytes — предел JPEG/PNG разметки для раздачи: WebView декодирует его сам.
const maxGuideBytes = 100 << 20

// Слоты кэша превью в App.markups.
const (
	markupCoverSlot = iota
	markupSpreadSlot
)

// serveMarkup отдаёт /media/cover-guide и /media/spread-guide; false — адрес не этот.
func (a *App) serveMarkup(w http.ResponseWriter, r *http.Request) bool {
	switch r.URL.Path {
	case "/media/cover-guide":
		a.serveGuide(w, r, markupCoverSlot, a.editor().CoverGuidePath())
	case "/media/spread-guide":
		a.serveGuide(w, r, markupSpreadSlot, a.editor().SpreadGuidePath())
	default:
		return false
	}
	return true
}

// serveGuide — файл-разметка path из проекта (из запроса пути нет: ?r=<rev> лишь сбивает кэш
// WebView). Каждый раз заново: том отвечает, файл проходит MarkupFile. JPEG/PNG отдаются
// файлом, PSD — PNG-превью из кэша слота. Ошибка — 404 и строка в log.txt без пути.
func (a *App) serveGuide(w http.ResponseWriter, r *http.Request, slot int, path string) {
	if path == "" || !reachable(path) {
		http.NotFound(w, r)
		return
	}
	m, err := library.MarkupFile(path)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-cache") // файл на диске могут заменить под тем же путём
	if !m.IsPSD() {
		if m.Size > maxGuideBytes {
			http.NotFound(w, r)
			return
		}
		ct := "image/jpeg"
		if strings.EqualFold(filepath.Ext(m.Path), ".png") {
			ct = "image/png"
		}
		w.Header().Set("Content-Type", ct)
		http.ServeFile(w, r, m.Path) //nolint:gosec // m.Path — путь разметки из проекта после MarkupFile, не из запроса
		return
	}
	data, err := a.markups[slot].get(m.Path)
	if err == nil && len(data) == 0 {
		err = library.ErrNoMarkupImage
	}
	if err != nil {
		// PSD без сведённого — обычный случай (видны только направляющие): 404 без строки в
		// log.txt, иначе каждое обновление подложки засоряет журнал.
		if !errors.Is(err, library.ErrNoMarkupImage) {
			log.Printf("разметка: превью PSD не построено: %v", paths.NoPath(err))
		}
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	_, _ = w.Write(data)
}

// markupPreview — построение превью PSD; подменяется в тестах (счёт обращений).
var markupPreview = library.MarkupPreview

// markupKey — превью действительно, пока у файла те же путь, размер и время записи.
type markupKey struct {
	path string
	size int64
	mod  int64 // ModTime в нс: time.Time через == сравнивать нельзя
}

// markupCache — одна запись на вид разметки. Замок держится и на время построения: второй
// запрос того же файла ждёт готовый результат, а не строит его заново. Замок Editor здесь
// не берётся — путь уже прочитан.
type markupCache struct {
	mu   sync.Mutex
	key  markupKey
	data []byte
	err  error
}

// reset — забыть запись: следующий get строит превью заново.
func (c *markupCache) reset() {
	c.mu.Lock()
	c.key, c.data, c.err = markupKey{}, nil, nil
	c.mu.Unlock()
}

// get — превью PSD path: из кэша, если файл не менялся, иначе заново (и ошибка тоже
// запоминается: битый файл не разбирается на каждый запрос).
func (c *markupCache) get(path string) ([]byte, error) {
	st, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	k := markupKey{path: path, size: st.Size(), mod: st.ModTime().UnixNano()}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.key == k {
		return c.data, c.err
	}
	data, _, err := markupPreview(path)
	c.key, c.data, c.err = k, data, err
	return data, err
}
