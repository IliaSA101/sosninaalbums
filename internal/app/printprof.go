package app

import (
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/model/printprof"
	"github.com/IliaSA101/sosninaalbums/internal/app/ownprint"
	wr "github.com/wailsapp/wails/v2/pkg/runtime"
)

// Свои типографии (123 Б1, Б3, Б4; разрез 126 Ш5): тело — internal/app/ownprint, здесь фасад с
// прежними методами для интерфейса, типы ответов и Host службы. Плоские типы определены (не
// псевдонимы) от типов службы: поля и теги те же, а models.ts остаётся в пространстве app
// (126-2). Вложенные — PrintEdit и PrintEditCheck — написаны в app заново; пару стережёт
// TestDTOPairs. PrintEdit — ещё и вход CheckPrintProfile/SavePrintProfile.

// dirPrint — своя последняя папка у диалогов файла типографии.
const dirPrint = "print"

var printFilter = []wr.FileFilter{{DisplayName: "Типография (*" + ownprint.Ext + ")", Pattern: "*" + ownprint.Ext}}

// PrintEdit — поля окна «Своя типография». Числа — как ввёл пользователь; id у своей всегда новый.
type PrintEdit struct {
	Name    string            `json:"name"`
	Base    string            `json:"base"` // id типографии-основы (изделия, корешки, пределы); "" — без обложки
	DPI     int               `json:"dpi"`
	Layflat bool              `json:"layflat"`
	Safe    float64           `json:"safe"`  // безопасная зона страницы, мм
	Bleed   float64           `json:"bleed"` // обрез, мм
	Formats []PrintEditFormat `json:"formats"`
	Spreads int               `json:"spreads"` // число разворотов, при котором введены px обложки (корешок); 0 — как в альбоме
	Prefix  string            `json:"prefix"`  // начало имён файлов выгрузки: «1-»
	Quality int               `json:"quality"` // JPEG при выгрузке: 100 | 95 | 0 («как выбрано в окне»)
	URL     string            `json:"url"`     // сайт типографии, https; "" — без кнопки
}

// PrintEditFormat — строка «подпись · разворот px · обложка px».
type PrintEditFormat ownprint.EditFormat

// PrintEditCheck — расчёт окна: чистая функция полей, проект и файлы не трогаются.
type PrintEditCheck struct {
	OK      bool              `json:"ok"`      // можно сохранять
	Errs    map[string]string `json:"errs"`    // поле → текст под полем
	Formats []PrintCheckRow   `json:"formats"` // по строке формата
	Spine   float64           `json:"spine"`   // корешок основы при Spreads, мм; 0 — нет
	Lines   []string          `json:"lines"`   // «Сейчас альбом даёт …»
	Names   string            `json:"names"`   // пример имён: «1-00.jpg — обложка, 1-01.jpg…1-12.jpg»
}

// PrintCheckRow — итог строки формата: страница и сторонка в мм из px либо ошибка под строкой.
type PrintCheckRow ownprint.CheckRow

// PrintSaved — итог «Сохранить»: id новой типографии и вид альбома после применения.
type PrintSaved ownprint.Saved

// PrintImport — итог «Добавить из файла».
type PrintImport ownprint.Import

// PrintShare — итог «Поделиться файлом»: имя записанного файла (без пути).
type PrintShare ownprint.Share

// mapSlice — срез с преобразованием элементов; nil остаётся nil (в JSON null и [] не путаются).
func mapSlice[A, B any](in []A, f func(A) B) []B {
	if in == nil {
		return nil
	}
	out := make([]B, len(in))
	for i, v := range in {
		out[i] = f(v)
	}
	return out
}

func editIn(e PrintEdit) ownprint.Edit {
	return ownprint.Edit{Name: e.Name, Base: e.Base, DPI: e.DPI, Layflat: e.Layflat, Safe: e.Safe, Bleed: e.Bleed,
		Formats: mapSlice(e.Formats, func(f PrintEditFormat) ownprint.EditFormat { return ownprint.EditFormat(f) }),
		Spreads: e.Spreads, Prefix: e.Prefix, Quality: e.Quality, URL: e.URL}
}

func editOut(e ownprint.Edit) PrintEdit {
	return PrintEdit{Name: e.Name, Base: e.Base, DPI: e.DPI, Layflat: e.Layflat, Safe: e.Safe, Bleed: e.Bleed,
		Formats: mapSlice(e.Formats, func(f ownprint.EditFormat) PrintEditFormat { return PrintEditFormat(f) }),
		Spreads: e.Spreads, Prefix: e.Prefix, Quality: e.Quality, URL: e.URL}
}

func checkOut(c ownprint.EditCheck) PrintEditCheck {
	return PrintEditCheck{OK: c.OK, Errs: c.Errs,
		Formats: mapSlice(c.Formats, func(r ownprint.CheckRow) PrintCheckRow { return PrintCheckRow(r) }),
		Spine:   c.Spine, Lines: c.Lines, Names: c.Names}
}

// PrintProfiles — типографии реестра по порядку: встроенные, библиотечные, свои. Для списка
// «Типография» и окна «Своя типография».
func (a *App) PrintProfiles() []printprof.Info {
	defer guard("PrintProfiles")
	return printprof.Current().Infos()
}

// DeletePrintProfile удаляет свою типографию (альбомы со ссылкой на неё не меняются; шага отмены нет).
func (a *App) DeletePrintProfile(id string) (v model.View, err error) {
	defer guardErr("DeletePrintProfile", &err)
	return a.ownp.Delete(id)
}

// ImportPrintProfile — «Добавить из файла…»: диалог *.saprint в Go. Отказ от диалога — Canceled.
func (a *App) ImportPrintProfile() (res PrintImport, err error) {
	defer guardErr("ImportPrintProfile", &err)
	r, err := a.ownp.ImportFile()
	return PrintImport(r), err
}

// ConfirmImportPrintProfile — «Заменить» на вопрос ImportPrintProfile.
func (a *App) ConfirmImportPrintProfile(token string) (res PrintImport, err error) {
	defer guardErr("ConfirmImportPrintProfile", &err)
	r, err := a.ownp.ConfirmImport(token)
	return PrintImport(r), err
}

// SharePrintProfile — «Поделиться файлом…»: своя типография в <название>.saprint (диалог Go).
func (a *App) SharePrintProfile(id string) (res PrintShare, err error) {
	defer guardErr("SharePrintProfile", &err)
	r, err := a.ownp.ShareFile(id)
	return PrintShare(r), err
}

// PrintProfileEdit — заготовка окна на основе типографии base; "" — пустая заготовка.
func (a *App) PrintProfileEdit(base string) (e PrintEdit, err error) {
	defer guardErr("PrintProfileEdit", &err)
	r, err := a.ownp.Draft(base)
	return editOut(r), err
}

// CheckPrintProfile — живой расчёт окна: ошибки полей, мм на строку формата, корешок, пример имён.
func (a *App) CheckPrintProfile(e PrintEdit) (c PrintEditCheck) {
	defer guard("CheckPrintProfile")
	return checkOut(a.ownp.Check(editIn(e)))
}

// SavePrintProfile записывает новую свою типографию; apply — ещё и поставить её альбому.
func (a *App) SavePrintProfile(e PrintEdit, apply bool) (res PrintSaved, err error) {
	defer guardErr("SavePrintProfile", &err)
	r, err := a.ownp.Save(editIn(e), apply)
	return PrintSaved(r), err
}

// printsHost — что службе типографий нужно от App. Поля — замыкания над a: тесты подменяют
// a.emit, openFileDialog и saveFileDialog уже после NewApp.
func (a *App) printsHost() ownprint.Host {
	return ownprint.Host{
		Store:    a.decorStore,
		Emit:     func(name string, data ...any) { a.emit(name, data...) },
		Version:  func() string { return appVersion(wailsJSON) },
		Snapshot: func() *model.Project { p, _ := a.editor().SnapshotSide(); return p },
		View:     func() model.View { return a.editor().View() },
		Changed:  func(v model.View) model.View { return a.changed(v) },
		Out:      func(v model.View) model.View { return a.out(v) },
		SetSettings: func(s model.Settings) model.View {
			return a.editor().SetSettings(s)
		},
		ApplyPrint: func(s model.Settings, preset string) (model.View, error) {
			return a.editor().ApplyPrint(s, preset)
		},
		PickOpen: func() (string, error) {
			return openFileDialog(a.ctx, wr.OpenDialogOptions{
				Title:            "Файл типографии",
				DefaultDirectory: a.dialogDir(dirPrint),
				Filters:          printFilter,
			})
		},
		PickSave: func(name string) (string, error) {
			return saveFileDialog(a.ctx, wr.SaveDialogOptions{
				Title:            "Поделиться типографией",
				DefaultDirectory: a.dialogDir(dirPrint),
				DefaultFilename:  name,
				Filters:          printFilter,
			})
		},
		RememberDir: func(dir string) { a.rememberDir(dirPrint, dir) },
	}
}
