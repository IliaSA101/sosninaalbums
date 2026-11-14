package app

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	wr "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/imgfile"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

// Папка данных и перенос своих шаблонов файлом (086 design §2).

// dirTemplates — своя последняя папка у обоих диалогов шаблонов.
const dirTemplates = "templates"

// saveFileDialog — нативный диалог сохранения; в тестах подменяется.
var saveFileDialog = wr.SaveFileDialog

// Тексты отказов — в тост (первую букву поднимает интерфейс), без пути: ошибки файловой
// системы несут его целиком.
var (
	errDataDir      = errors.New("папка данных недоступна")
	errDataDirOpen  = errors.New("не удалось открыть папку данных")
	errTplNone      = errors.New("своих шаблонов пока нет — сохранять нечего")
	errTplSaveLocal = errors.New("сохраните файл на диске этого компьютера")
	errTplSave      = errors.New("не удалось сохранить файл шаблонов")
	errTplFile      = errors.New("это не файл шаблонов Sosnina Albums")
	errTplLocal     = errors.New("файл шаблонов не на локальном диске")
	errTplMissing   = errors.New("файл шаблонов не найден")
	errTplBig       = errors.New("файл шаблонов больше 1 МБ")
	errTplNewerFile = errors.New("файл шаблонов из более новой версии программы — обновите Sosnina Albums")
	errTplEmpty     = errors.New("в файле нет шаблонов")
	errTplAllSame   = errors.New("все шаблоны из файла уже есть")
	errTplAllOver   = fmt.Errorf("шаблонов уже %d — удалите ненужные", model.MaxTemplates)
)

// TplSave — итог «Мои шаблоны ▸ Сохранить в файл…».
type TplSave struct {
	Canceled bool   `json:"canceled"`
	Saved    int    `json:"saved"`
	Text     string `json:"text"` // текст тоста: «Сохранено шаблонов: 14»
}

// TplLoad — итог «Мои шаблоны ▸ Загрузить из файла…».
type TplLoad struct {
	Canceled bool   `json:"canceled"`
	Added    int    `json:"added"`
	Same     int    `json:"same"`
	Over     int    `json:"over"`
	Text     string `json:"text"` // текст тоста: «Добавлено шаблонов: 12 · уже были: 3»
}

// tplFilter — фильтр обоих диалогов: файл переноса — ровно templates.json.
var tplFilter = []wr.FileFilter{{DisplayName: "Шаблоны Sosnina Albums (*.json)", Pattern: "*.json"}}

// ShowDataFolder открывает папку данных (настройки, шаблоны, библиотека, черновик, log.txt)
// в проводнике; нет папки — сначала создаёт. Ошибка без пути: её видит человек.
func (a *App) ShowDataFolder() (err error) {
	defer guardErr("ShowDataFolder", &err)
	// Пустой путь — не «текущая папка»: проводник открыл бы что попало.
	if a.cfgDir == "" || os.MkdirAll(a.cfgDir, 0o700) != nil {
		return errDataDir
	}
	if openDir(a.cfgDir) != nil {
		return errDataDirOpen // в журнал не пишем: текст проводника несёт путь с именем пользователя
	}
	return nil
}

// ExportTemplates — весь список «Мои шаблоны» в файл формата templates.json (диалог Go).
// Пустой список — ошибка до диалога; отказ от диалога — Canceled.
func (a *App) ExportTemplates() (res TplSave, err error) {
	defer guardErr("ExportTemplates", &err)
	list := a.templateList()
	if len(list) == 0 {
		return res, errTplNone
	}
	path, err := saveFileDialog(a.ctx, wr.SaveDialogOptions{
		Title:            "Сохранить мои шаблоны",
		DefaultDirectory: a.dialogDir(dirTemplates),
		DefaultFilename:  "Мои шаблоны.json",
		Filters:          tplFilter,
	})
	if err != nil {
		return res, err
	}
	if path == "" {
		return TplSave{Canceled: true}, nil
	}
	added := !strings.EqualFold(filepath.Ext(path), ".json")
	if added {
		path += ".json"
	}
	if paths.CheckLocalPath(path) != nil {
		return res, errTplSaveLocal
	}
	// Замену выбранного файла подтвердил диалог, а файла с дописанным «.json» он не видел:
	// молча перезаписать его нельзя. Имя без папки — путь в тосте не показываем.
	if added {
		if _, err := os.Lstat(path); err == nil {
			return res, fmt.Errorf("файл уже есть: %s", filepath.Base(path))
		}
	}
	data, err := model.MarshalTemplates(list)
	if err == nil {
		err = library.WriteFileAtomic(path, data)
	}
	if err != nil {
		log.Printf("файл шаблонов: %v", paths.NoPath(err))
		return res, errTplSave
	}
	a.rememberDir(dirTemplates, filepath.Dir(path))
	return TplSave{Saved: len(list), Text: fmt.Sprintf("Сохранено шаблонов: %d", len(list))}, nil
}

// ImportTemplates — диалог выбора файла шаблонов и ImportTemplatesPath; отказ — Canceled.
func (a *App) ImportTemplates() (res TplLoad, err error) {
	defer guardErr("ImportTemplates", &err)
	path, err := openFileDialog(a.ctx, wr.OpenDialogOptions{
		Title:            "Загрузить шаблоны",
		DefaultDirectory: a.dialogDir(dirTemplates),
		Filters:          tplFilter,
	})
	if err != nil {
		return res, err
	}
	if path == "" {
		return TplLoad{Canceled: true}, nil
	}
	res, err = a.ImportTemplatesPath(path)
	if err == nil {
		a.rememberDir(dirTemplates, filepath.Dir(path))
	}
	return res, err
}

// ImportTemplatesPath добавляет шаблоны из файла в конец «Моих шаблонов» (как AddPackPath: путь
// и для проверки из браузера — только локальный обычный файл .json ≤ 1 МБ, только чтение).
// Тот же шаблон по содержимому пропускается, совпавший ID заменяется (prefs.MergeTemplates).
// Любой отказ — ошибка, templates.json не тронут.
func (a *App) ImportTemplatesPath(path string) (res TplLoad, err error) {
	defer guardErr("ImportTemplatesPath", &err)
	add, err := readTplFile(path)
	if err != nil {
		return res, err
	}
	a.tplMu.Lock()
	defer a.tplMu.Unlock()
	if err := a.tplReady(); err != nil {
		return res, err
	}
	if a.tpls.Newer() {
		return res, model.ErrTemplatesNewer
	}
	out, m := prefs.MergeTemplates(a.tpls.List(), add, model.NewID)
	if m.Added == 0 {
		if m.Over > 0 {
			return res, errTplAllOver
		}
		return res, errTplAllSame
	}
	if err := a.storeTemplates(out); err != nil {
		return res, err
	}
	return TplLoad{Added: m.Added, Same: m.Same, Over: m.Over, Text: tplLoadText(m)}, nil
}

// tplLoadText — текст тоста загрузки без нулевых частей.
func tplLoadText(m prefs.MergeResult) string {
	s := fmt.Sprintf("Добавлено шаблонов: %d", m.Added)
	if m.Same > 0 {
		s += fmt.Sprintf(" · уже были: %d", m.Same)
	}
	if m.Over > 0 {
		s += fmt.Sprintf(" · не вошли: %d (не больше %d)", m.Over, model.MaxTemplates)
	}
	return s
}

// readTplFile — шаблоны из чужого файла: недоверенный ввод, тот же разбор с зажимами, что при
// старте. Размер проверяется и по пути, и по открытому дескриптору (файл могли подменить), и
// чтением не больше предела + 1 байт.
func readTplFile(path string) ([]model.Template, error) {
	if paths.CheckLocalPath(path) != nil {
		return nil, errTplLocal
	}
	p, size, err := library.PlainFile(path, ".json")
	switch {
	case errors.Is(err, os.ErrNotExist):
		return nil, errTplMissing
	case err != nil:
		return nil, errTplFile
	case size > model.MaxTemplatesFile:
		return nil, errTplBig
	}
	f, err := imgfile.OpenRegular(p)
	if err != nil {
		return nil, errTplFile
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(io.LimitReader(f, model.MaxTemplatesFile+1))
	if err != nil {
		log.Printf("файл шаблонов: %v", paths.NoPath(err))
		return nil, errTplFile
	}
	if len(data) > model.MaxTemplatesFile {
		return nil, errTplBig
	}
	list, ver, err := model.ParseTemplates(data)
	switch {
	case err != nil:
		return nil, errTplFile
	case ver > model.TemplatesVersion:
		// Поля новой версии потерялись бы при записи в свой templates.json.
		return nil, errTplNewerFile
	case len(list) == 0:
		return nil, errTplEmpty
	}
	return list, nil
}
