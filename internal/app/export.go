package app

import (
	"context"
	"errors"
	"runtime"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/app/exporter"
	"github.com/IliaSA101/sosninaalbums/internal/export/printshop"
	"github.com/IliaSA101/sosninaalbums/internal/imgfile"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/osx"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
	wr "github.com/wailsapp/wails/v2/pkg/runtime"
)

// Выгрузка (спеки 013, 047, 073, 081, 121): тело — internal/app/exporter, здесь фасад с прежними
// методами для интерфейса, определённый тип ExportOptions, Host службы и то, что она берёт у App:
// цвет фото из ленты, диалоги и браузер (разрез 126 Ш3).

// ExportOptions — настройки окна экспорта. Тип определён (не псевдоним) от exporter.Options: поля и
// теги те же, а models.ts остаётся в пространстве app (126-2) — `export` ключевое слово TS.
type ExportOptions exporter.Options

// Export собирает развороты по настройкам окна экспорта: спрашивает папку и пишет в фоне.
// Ход работы приходит событиями export:progress и export:done; остановка — CancelExport.
func (a *App) Export(o ExportOptions) (bool, error) {
	if a.shr.Busy() { // экспорт и выгрузка по ссылке вместе не идут (146)
		return false, errors.New("Сейчас идёт выгрузка по ссылке — дождитесь её конца") //nolint:revive,staticcheck // текст уходит пользователю тостом как есть
	}
	return a.exp.Export(exporter.Options(o))
}

// CancelExport — «Остановить»: текущий разворот прерывается (не дописывается), следующие не пишутся, прежние
// файлы папки не удаляются. Экспорта нет — ничего.
func (a *App) CancelExport() { a.exp.Cancel() }

// ShowInExplorer открывает папку последнего экспорта. Путь от интерфейса не принимаем.
func (a *App) ShowInExplorer() { a.exp.Show() }

// PrintPlans — готовность альбома к выгрузке для типографий (081 №13, 123 §1.5); last — последний
// выбор окна ("" — «Обычная»). Проект не меняется.
func (a *App) PrintPlans(last string) []printshop.Plan { return a.exp.PrintPlans(last) }

// OpenPrintSite открывает сайт типографии id в браузере (123 Б1); от интерфейса приходит только id.
func (a *App) OpenPrintSite(id string) { a.exp.OpenPrintSite(id) }

// expHost — что службе выгрузки нужно от App. Поля — замыкания над a: тесты подменяют a.emit,
// a.exists, askYes, pickExportDir и openURL уже после NewApp.
func (a *App) expHost() exporter.Host {
	return exporter.Host{
		Editor:      a.editor,
		Exists:      func(path string) bool { return a.exists(path) },
		Emit:        func(name string, data ...any) { a.emit(name, data...) },
		GoBG:        func(fn func()) bool { return a.goBG(fn) },
		StopCtx:     func() context.Context { return a.stopCtx },
		ColorReport: func(p *model.Project) (map[string]int, map[string]int) { return a.colorReport(p) },
		Ask:         func(msg string) (bool, error) { return askYes(a, msg) },
		PickDir:     func(title string) (string, error) { return pickExportDir(a, title) },
		RememberDir: func(dir string) { a.rememberDir(prefs.DirExport, dir) },
		OpenURL:     func(u string) { openURL(a, u) },
		ShowDir:     showDir,
		LiftMemory:  liftMemoryLimit,
	}
}

// colorReport считает фото альбома с не-sRGB профилем: conv — что пересчитается в sRGB само
// (имя → число: «Adobe RGB», «Display P3» для всех P3 — iPhone и Xiaomi с разными именами, у прочих
// матричных — имя профиля), other — фото с «чужим» профилем (имя профиля → число), их не пересчитываем.
// Цвет берётся из реестра ленты, а если фото там нет (папка закрыта или чтение не дошло) — из
// заголовка файла. Нечитаемые файлы пропускаются: экспорт упадёт на них с понятной ошибкой.
// Счёт — по файлам: копии (098) — тот же файл, без двойного счёта и второго чтения заголовка.
func (a *App) colorReport(p *model.Project) (conv, other map[string]int) {
	conv, other = map[string]int{}, map[string]int{}
	files := make(map[string]model.PhotoRef, len(p.Photos)) // ID файла → любая запись семьи
	for id, ref := range p.Photos {
		files[model.BaseID(id)] = ref
	}
	a.mu.Lock()
	known := make(map[string]library.Photo, len(files))
	for id := range files {
		if ph, ok := a.byID[id]; ok {
			known[id] = ph
		}
	}
	a.mu.Unlock()
	for id, ref := range files {
		col, prof := known[id].Color, known[id].Profile
		if _, ok := known[id]; !ok {
			col, prof = imgfile.ReadColor(ref.Path)
		}
		switch col {
		case imgfile.ColorAdobe:
			conv["Adobe RGB"]++
		case imgfile.ColorP3:
			conv["Display P3"]++
		case imgfile.ColorICC:
			if prof == "" {
				prof = "RGB-профиль"
			}
			conv[prof]++
		case imgfile.ColorOther:
			if prof == "" {
				prof = "неизвестный профиль"
			}
			other[prof]++
		}
	}
	return conv, other
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
	pickExportDir = func(a *App, title string) (string, error) {
		return wr.OpenDirectoryDialog(a.ctx, wr.OpenDialogOptions{
			Title:            title,
			DefaultDirectory: a.dialogDir(prefs.DirExport),
		})
	}
	// openURL — открыть адрес в браузере (подмена в тестах).
	openURL = func(a *App, url string) {
		osx.OpenURL(url, func(u string) { wr.BrowserOpenURL(a.ctx, u) })
	}
)
