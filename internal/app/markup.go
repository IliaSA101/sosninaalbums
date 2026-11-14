package app

import (
	"net/http"
	"path/filepath"

	wr "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/app/ownprint"
	"github.com/IliaSA101/sosninaalbums/internal/library"
)

// Разметка типографии (032, 066): выбор файла обложки и разворотов и раздача картинки.
// Путь — только из диалога Go и из проекта, от интерфейса и из запроса не принимается.
// Кэш превью и тело раздачи — ownprint/markup.go (здесь остались Wails, Editor и проверка тома).

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
	return a.setGuide(path, dirCoverGuide, ownprint.MarkupCover, (*album.Editor).SetCoverGuide)
}

// setSpreadGuide — проверка выбранного файла разметки разворотов и правка.
func (a *App) setSpreadGuide(path string) (model.View, error) {
	return a.setGuide(path, dirSpreadGuide, ownprint.MarkupSpread, (*album.Editor).SetSpreadGuide)
}

// setGuide — файл проверяется library.MarkupFileExplicit (PSB, повреждённый PSD, не картинка — ошибка,
// правки нет), в проект идёт путь, по которому файл читается (у ссылки — её цель). Слот кэша
// превью сбрасывается при каждом выборе, в том числе повторном: разовый сбой чтения иначе держал бы
// 404 до изменения файла.
func (a *App) setGuide(path, kind string, slot int, set func(*album.Editor, string) (model.View, error)) (model.View, error) {
	a.markups.Reset(slot)
	m, err := library.MarkupFileExplicit(path) // явный выбор: заглушку OneDrive читает со скачиванием
	if err != nil {
		return a.editor().View(), err
	}
	v, err := a.changedOrErr(set(a.editor(), m.Path))
	if err == nil {
		a.rememberDir(kind, filepath.Dir(m.Path))
	}
	return v, err
}

// serveMarkup отдаёт /media/cover-guide и /media/spread-guide; false — адрес не этот.
func (a *App) serveMarkup(w http.ResponseWriter, r *http.Request) bool {
	switch r.URL.Path {
	case "/media/cover-guide":
		a.serveGuide(w, r, ownprint.MarkupCover, a.editor().CoverGuidePath())
	case "/media/spread-guide":
		a.serveGuide(w, r, ownprint.MarkupSpread, a.editor().SpreadGuidePath())
	default:
		return false
	}
	return true
}

// serveGuide — файл-разметка path из проекта: проверка тома здесь (osx), остальное — ownprint.Markups.Serve.
func (a *App) serveGuide(w http.ResponseWriter, r *http.Request, slot int, path string) {
	if path == "" || !reachable(path) {
		http.NotFound(w, r)
		return
	}
	a.markups.Serve(w, r, slot, path)
}
