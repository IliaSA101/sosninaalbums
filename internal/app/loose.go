package app

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
	wr "github.com/wailsapp/wails/v2/pkg/runtime"
)

// Отдельные фото (097): добавление файлов в ленту без папки. Папки — api.go (ChooseFolder и др.),
// бросок в окно — drop.go.

// looseStat — os.Lstat проверки отдельного файла; в тесте подменяется, чтобы убедиться, что до отказа
// CheckLocalPath и по расширению к диску (на UNC-пути — к сети) никто не обращается.
var looseStat = os.Lstat

// FolderAdded — событие `folders:added`: что вышло из добавления папки Dir. Within: "" — папка новая;
// == Dir — уже была; иначе — путь папки, в которую Dir входит. Merged — сколько папок списка вошли в
// Dir; LooseIn — сколько отдельных фото теперь видны через неё.
type FolderAdded struct {
	Dir     string `json:"dir"`
	Within  string `json:"within"`
	Merged  int    `json:"merged"`
	LooseIn int    `json:"looseIn"`
}

// LooseResult — вид после добавления отдельных фото и его итог (поля как у model.LooseAdded).
// Batch — ID нового добавления, "" — ничего не добавлено.
type LooseResult struct {
	View     model.View `json:"view"`
	Batch    string     `json:"batch"`
	Added    int        `json:"added"`
	InFolder int        `json:"inFolder"`
	Folder   string     `json:"folder"`
	Dup      int        `json:"dup"`
	Over     int        `json:"over"`
	Skipped  int        `json:"skipped"`
	Bad      int        `json:"bad"`
}

// looseTooMany — отказ списку длиннее предела: проверяется до любого обращения к диску.
func looseTooMany(n int) error {
	return fmt.Errorf("Отдельных фото не больше %s — не добавлено %d", spaced(model.MaxLooseFiles), n-model.MaxLooseFiles) //nolint:staticcheck // текст уходит пользователю тостом как есть
}

// spaced — число с пробелом тысяч: 2000 → «2 000».
func spaced(n int) string {
	if n < 1000 {
		return fmt.Sprint(n)
	}
	return fmt.Sprintf("%d %03d", n/1000, n%1000)
}

// ChooseFiles спрашивает фото (несколько файлов, JPEG и PNG) и добавляет их в проект одним добавлением.
// Диалог открывается в папке прошлого выбора, а при первом — как «Добавить папку».
func (a *App) ChooseFiles() (r LooseResult, err error) {
	defer guardErr("ChooseFiles", &err)
	r.View = a.editor().View()
	start := a.dialogDir(prefs.DirPhotos)
	if a.loadConfig().LastDirs[prefs.DirLoose] != "" {
		start = a.dialogDir(prefs.DirLoose)
	}
	files, err := openFilesDialog(a.ctx, wr.OpenDialogOptions{
		Title:            "Фото для альбома",
		DefaultDirectory: start,
		Filters:          []wr.FileFilter{{DisplayName: "Фото (*.jpg, *.jpeg, *.png)", Pattern: "*.jpg;*.jpeg;*.png"}},
	})
	if err != nil || len(files) == 0 {
		return r, err
	}
	r, err = a.AddFilePaths(files)
	if err == nil {
		a.rememberDir(prefs.DirLoose, filepath.Dir(files[0]))
	}
	return r, err
}

// AddFilePaths добавляет файлы одним добавлением (путь для проверки из браузера и для ChooseFiles).
// Список длиннее MaxLooseFiles — ошибка looseTooMany без обращения к диску (бросок в окно проверяет
// тот же предел по всем путям до Lstat, dropPaths). Что не добавилось, сказано в счётах итога.
func (a *App) AddFilePaths(ps []string) (r LooseResult, err error) {
	defer a.guardEdit("AddFilePaths", &r.View, &err)
	return a.addLoose(ps)
}

// RemoveLoose убирает добавление id (path == "") или один его файл. Ничего не читает с диска.
func (a *App) RemoveLoose(id, path string) (v model.View) {
	defer a.guardEdit("RemoveLoose", &v, nil)
	v = a.changed(a.editor().RemoveLoose(id, path))
	a.rescan()
	return v
}

// addLoose — тело AddFilePaths и броска файлов. Порядок проверок каждого пути как у dropPaths:
// CheckLocalPath до любого Stat → расширение → обычный файл → ссылка только с локальной целью.
// В log.txt — только счёты (в пути имя пользователя и название альбома).
func (a *App) addLoose(ps []string) (r LooseResult, err error) {
	ed := a.editor()
	r.View = ed.View()
	if len(ps) > model.MaxLooseFiles {
		return r, looseTooMany(len(ps))
	}
	var ok []string
	for _, p := range ps {
		switch looseVerdict(p) {
		case looseOK:
			ok = append(ok, p)
		case looseBad:
			r.Bad++
		default:
			r.Skipped++
		}
	}
	v, got, err := ed.AddLoose(ok, time.Now().Unix())
	if err != nil {
		return r, err
	}
	r.Batch, r.Added, r.InFolder, r.Folder, r.Dup, r.Over = got.Batch, got.Added, got.InFolder, got.Folder, got.Dup, got.Over
	r.View = a.changed(v)
	if r.Added > 0 {
		a.rescan()
	}
	log.Printf("отдельные фото: добавлено %d, в папке альбома %d, повторов %d, сверх предела %d, не фото %d, нелокальных %d",
		r.Added, r.InFolder, r.Dup, r.Over, r.Skipped, r.Bad)
	return r, nil
}

type looseKind int

const (
	looseOK      looseKind = iota
	looseBad               // нелокальный путь
	looseSkipped           // не фото, не файл, нет на диске
)

// looseVerdict — годится ли путь в отдельные фото.
func looseVerdict(p string) looseKind {
	if paths.CheckLocalPath(p) != nil {
		return looseBad
	}
	name := filepath.Base(p)
	if !library.IsImage(name) || paths.DeviceElem(name) {
		return looseSkipped
	}
	st, err := looseStat(p)
	if err != nil {
		return looseSkipped
	}
	if st.Mode()&(fs.ModeSymlink|fs.ModeIrregular) != 0 {
		var linked bool
		if st, linked = library.StatLink(p); !linked {
			return looseSkipped
		}
	}
	if !st.Mode().IsRegular() {
		return looseSkipped
	}
	return looseOK
}

// srcSet — снимок источников ленты: Undo, Redo и открытие копии перечитывают библиотеку, только
// если он сменился.
type srcSet struct{ folders, files []string }

func sourcesOf(ed *album.Editor) srcSet {
	f, l := ed.Sources()
	return srcSet{f, l}
}

func (s srcSet) differs(o srcSet) bool {
	return !slices.Equal(s.folders, o.folders) || !slices.Equal(s.files, o.files)
}

// folderAdded шлёт `folders:added`: окно покажет тост по итогу добавления папки dir. Within,
// равный папке с точностью до регистра и разделителей, отдаётся как dir (UI сравнивает строки).
func (a *App) folderAdded(dir string, r model.FolderAdd) {
	within := r.Within
	// Ключ DirKey, как у самой проверки повтора (spreads.AddFolder): SamePath на Linux различает
	// регистр, и Dir с Within расходились бы.
	if within != "" && paths.DirKey(within) == paths.DirKey(dir) {
		within = dir
	}
	a.emit("folders:added", FolderAdded{Dir: dir, Within: within, Merged: r.Merged, LooseIn: r.LooseIn})
}
