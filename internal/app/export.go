package app

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/export"
	"github.com/IliaSA101/sosninaalbums/internal/imgfile"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/osx"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
	"github.com/IliaSA101/sosninaalbums/internal/render"
	wr "github.com/wailsapp/wails/v2/pkg/runtime"
)

// colorReport считает фото альбома с не-sRGB профилем: adobe — сколько пересчитается в sRGB
// само, other — сколько фото с «чужим» профилем (имя профиля → число), их не пересчитываем.
// Цвет берётся из реестра ленты, а если фото там нет (папка закрыта или чтение не дошло) — из
// заголовка файла. Нечитаемые файлы пропускаются: экспорт упадёт на них с понятной ошибкой.
func (a *App) colorReport(p *model.Project) (adobe int, other map[string]int) {
	other = map[string]int{}
	a.mu.Lock()
	known := make(map[string]library.Photo, len(p.Photos))
	for id := range p.Photos {
		if ph, ok := a.byID[id]; ok {
			known[id] = ph
		}
	}
	a.mu.Unlock()
	for id, ref := range p.Photos {
		col, prof := known[id].Color, known[id].Profile
		if _, ok := known[id]; !ok {
			col, prof = imgfile.ReadColor(ref.Path)
		}
		switch col {
		case imgfile.ColorAdobe:
			adobe++
		case imgfile.ColorOther:
			if prof == "" {
				prof = "неизвестный профиль"
			}
			other[prof]++
		}
	}
	return adobe, other
}

// askOptions — окно вопроса «Да / Нет». На Mac Wails рисует кнопки строками Buttons как есть
// (и возвращает нажатую), на Windows — системные кнопки на языке ОС с ответом "Yes"/"No".
// Первая кнопка — «да», вторая (и по умолчанию, и по Esc) — «нет».
func askOptions(goos, msg string) wr.MessageDialogOptions {
	yes, no := "Yes", "No"
	if goos == "darwin" {
		yes, no = "Да", "Нет"
	}
	return wr.MessageDialogOptions{
		Type:          wr.QuestionDialog,
		Title:         "Экспорт",
		Message:       msg,
		Buttons:       []string{yes, no},
		DefaultButton: no,
		CancelButton:  no,
	}
}

// Диалоги экспорта — переменные, чтобы тесты подменяли нативные окна.
var (
	askYes = func(a *App, msg string) (bool, error) {
		opts := askOptions(runtime.GOOS, msg)
		ans, err := wr.MessageDialog(a.ctx, opts)
		return err == nil && ans == opts.Buttons[0], err
	}
	pickExportDir = func(a *App) (string, error) {
		return wr.OpenDirectoryDialog(a.ctx, wr.OpenDialogOptions{
			Title:            "Куда выгрузить",
			DefaultDirectory: a.dialogDir(prefs.DirExport),
		})
	}
)

// Export собирает развороты по настройкам окна экспорта: спрашивает папку и пишет в фоне.
// Ход работы приходит событиями export:progress и export:done.
func (a *App) Export(o ExportOptions) (bool, error) {
	// Флаг ставим до диалога: пока он открыт, второй вызов (из браузера) не пройдёт.
	a.mu.Lock()
	if a.exporting {
		a.mu.Unlock()
		return false, fmt.Errorf("экспорт уже идёт")
	}
	a.exporting = true
	a.mu.Unlock()
	started := false
	defer func() {
		if !started {
			a.mu.Lock()
			a.exporting = false
			a.mu.Unlock()
		}
	}()

	// Всё, что может отказать, — до диалога папки и удаления прежних файлов: иначе отказ пришёл бы после потерь.
	p, v := a.editor().Snapshot()
	o.Quality, o.DPI = export.ClampOptions(o.Quality, o.DPI, p.Settings.DPI)
	n := len(v.Spreads)
	// Обложка (032): нет её или проект-лист — флаги игнорируются; «только обложка» включает обложку.
	cover := export.NewCoverJob(p, v, o.DPI, o.Quality)
	if p.Settings.Sheet || (!o.Cover && !o.CoverOnly) {
		cover = nil
	}
	o.Cover, o.CoverOnly = cover != nil, o.CoverOnly && cover != nil
	if n == 0 && cover == nil {
		return false, fmt.Errorf("в альбоме нечего экспортировать")
	}
	o.Pages = o.Pages && !p.Settings.Sheet // у листа 031 «отдельными страницами» нет: один файл на лист
	g := p.Geo(nil)                        // холст — половина листа (032 §7): у разворота страница, у листа 031 — W/2
	opt := render.Options{PageW: g.W / 2, PageH: g.H, DPI: o.DPI, Quality: o.Quality}
	var pl export.Plan // «только обложка»: пустой план, не Full — хвосты разворотов не удаляются
	if !o.CoverOnly {
		units, err := exportUnits(o.Range, n, p.Settings.Sheet)
		if err != nil {
			return false, err
		}
		pl = export.NewPlan(units, n, o.Pages)
		if err := export.CheckCanvases(opt, o.Pages); err != nil {
			return false, err
		}
	}
	if cover != nil {
		if err := export.CheckCanvas(cover.Opt); err != nil {
			return false, err
		}
		pl = pl.WithCover(cover)
	}
	sel := pl.SelectedPhotos(v, opt.PageW)
	if err := export.MissingPhotos(p, sel, a.exists); err != nil {
		return false, err
	}
	// Профили проверяем тоже до диалога папки: ответ «нет» не должен стоить выбора папки.
	adobe, other := a.colorReport(export.PickPhotos(p, sel))
	if text := export.ColorWarning(other); text != "" {
		if ok, err := askYes(a, text); !ok {
			return false, err
		}
	}
	// Пустые развороты — тоже до папки, по тому же доводу (спека 013).
	white := p.Settings.Bg == "" || p.Settings.Bg == "#ffffff"
	blankCover := cover != nil && export.BlankCover(v)
	if text := export.BlankWarningCover(export.BlankSpreads(v, &pl), blankCover, white); text != "" {
		if ok, err := askYes(a, text); !ok {
			return false, err
		}
	}
	// Пустые поля подписей (030) — тоже до папки: в печати они пусты. Только листы плана.
	if text := export.FieldsWarning(export.PlanFields(v, &pl)); text != "" {
		if ok, err := askYes(a, text); !ok {
			return false, err
		}
	}
	dir, err := pickExportDir(a)
	if err != nil || dir == "" {
		return false, err
	}
	a.rememberDir(prefs.DirExport, dir)
	// Пустые места после выбора папки, до вопроса о перезаписи: «нет» ничего не удаляет.
	if text := export.EmptyWarning(export.EmptySummaryFor(v, &pl, opt.PageW)); text != "" {
		if ok, err := askYes(a, text); !ok {
			return false, err
		}
	}

	ours, foreign, err := export.PlanDir(dir)
	if err != nil {
		return false, fmt.Errorf("не удалось прочитать папку экспорта: %w", err)
	}
	ours, foreign = export.AddCoverFile(pl, dir, ours, foreign) // 00.jpg: наш заменяется, чужой — отказ
	if foreign = export.HitFiles(pl, foreign); len(foreign) > 0 {
		return false, fmt.Errorf("В папке есть файлы с номерами разворотов (или 00.jpg) не из Sosnina Albums (%d шт., например %s). Выберите другую папку или уберите их.", len(foreign), foreign[0].Name) //nolint:revive,staticcheck // текст уходит пользователю тостом как есть
	}
	var stale []string
	if hit := export.HitFiles(pl, ours); len(hit) > 0 {
		if ok, err := askYes(a, export.ReplaceQuestion(pl, hit)); !ok {
			return false, err
		}
		stale = export.StaleNames(hit, pl.Names(n)) // удалятся после записи новых
	}

	spreads := export.FromView(p, v)
	started = true
	a.mu.Lock()
	a.exportDir = dir
	a.mu.Unlock()
	a.bg.Go(func() {
		defer guard("экспорт")
		defer func() {
			a.mu.Lock()
			a.exporting = false
			a.mu.Unlock()
		}()
		start := time.Now()
		err := export.RunPlan(a.stopCtx, spreads, pl, opt, dir, stale,
			func(done, total int) { a.emit("export:progress", done, total) })
		msg, note := "", ""
		if err != nil {
			msg = err.Error()
			log.Printf("экспорт: %v", paths.NoPath(err))
		} else {
			note = export.ColorNote(adobe)
			log.Printf("экспорт: %d разворотов в %s за %v", len(pl.Jobs), filepath.Base(dir), time.Since(start).Round(time.Second))
			if note != "" {
				log.Print(note)
			}
		}
		a.emit("export:done", dir, msg, note)
	})
	return true, nil
}

// exportUnits — цели диапазона окна экспорта; "" — nil (все развороты). У листа 031 «л»/«п» — ошибка.
func exportUnits(rng string, n int, sheet bool) ([]export.Unit, error) {
	if strings.TrimSpace(rng) == "" {
		return nil, nil
	}
	units, err := export.ParseRange(rng, n)
	switch {
	case err != nil && sheet:
		// ParseRange общий с альбомом и говорит «разворота 7 нет»; у листа 031 единица — лист.
		err = errors.New(strings.Replace(err.Error(), "разворота", "листа", 1))
	case err == nil && sheet:
		err = export.CheckSheet(units)
	}
	return units, err
}

// ShowInExplorer открывает папку последнего экспорта. Путь от интерфейса не принимаем:
// иначе любая страница в окне смогла бы запускать explorer с произвольным аргументом.
func (a *App) ShowInExplorer() {
	a.mu.Lock()
	dir := a.exportDir
	a.mu.Unlock()
	if st, err := os.Stat(dir); dir == "" || err != nil || !st.IsDir() {
		return
	}
	_ = osx.OpenDir(dir) // не открылась папка — не страшно, ошибку показать нечем
}

// ExportOptions — настройки окна экспорта. Живёт в internal/app, а не в internal/export: Wails берёт
// пространство имён models.ts из имени пакета, а `export` — ключевое слово TS.
type ExportOptions struct {
	Range   string `json:"range"`   // "" — все развороты; «3-5, 8, 7п» (export.ParseRange)
	Quality int    `json:"quality"` // 100 | 95 | 80; другое — 95
	DPI     int    `json:"dpi"`     // 72..1200; 0 — из настроек альбома
	Pages   bool   `json:"pages"`   // по страницам: левая и правая отдельными файлами
	// Cover — выгрузить обложку 00.jpg (спека 032); обложки нет или проект-лист — игнорируется.
	// CoverOnly — только обложка, развороты и Range не выгружаются (включает Cover).
	Cover     bool `json:"cover"`
	CoverOnly bool `json:"coverOnly"`
}
