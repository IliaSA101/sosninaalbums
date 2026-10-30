package app

// Папка выгрузки по просьбе агента (152, design §3.3): имя, корень выгрузок, предложение, соседняя папка
// 121, свои и чужие файлы, сеть и облако. Агент путь не получает: человек видит настоящий путь в вопросе.

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"unicode/utf8"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/app/exporter"
	"github.com/IliaSA101/sosninaalbums/internal/export"
	"github.com/IliaSA101/sosninaalbums/internal/export/printshop"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

// exportCloudRoots — шов тестов: корни облачных папок этой машины.
var exportCloudRoots = func() []string {
	home, _ := os.UserHomeDir()
	return cloudRoots(runtime.GOOS, home, os.Getenv)
}

// exportPlan — что показано в вопросе о папке и что получит ExportWith.
type exportPlan struct {
	shown   string   // настоящий путь для вопроса (у типографии — папка книги); "" — окно ОС
	pick    string   // что отдать ExportWith: у типографии выбранная папка (книга внутри), иначе shown
	create  bool     // shown ещё нет: создаётся после вопросов
	pickNew bool     // pick ещё нет
	nextTo  bool     // путь был в папке фото альбома — показана соседняя (121)
	net     bool     // сетевой диск
	cloud   bool     // синхронизируется с облаком
	foreign bool     // в папке чужие файлы с номерами: выгрузить нельзя
	badHint bool     // подсказка агента негодна (показано «Агент предложил папку, которую программа не берёт без вас»)
	ours    []string // свои файлы, которые заменятся (набор, показанный в вопросе)
}

// approved — путь показан и годится: «Выгрузить» пишет в него без окна ОС.
func (p exportPlan) approved() bool { return p.shown != "" && !p.foreign }

// exportNames — имена выгрузки: job — имя книги от агента для exporter (Job.Name), dir — имя папки в корне
// выгрузок ("" — нет: черновик без подсказки), bad — подсказка-имя негодна.
type exportNames struct {
	job, dir string
	bad      bool
}

// exportNamesOf — имя от агента («Петровы») или имя файла альбома. Годное имя — то, что FolderName принимает
// как есть (без `<>:"/\|?*`, без точки и пробела на краях, не CON): иначе подсказка негодна, и программа не
// подменяет её своей (0.35-152-16). Подсказка-путь имени не даёт: имя берётся от файла.
func exportNamesOf(hint string, ed *album.Editor) exportNames {
	hint = strings.TrimSpace(hint)
	file := ""
	if p := ed.Path(); p != "" {
		file = paths.FolderName(strings.TrimSuffix(filepath.Base(p), filepath.Ext(p)), "")
	}
	switch {
	case hint == "" || strings.ContainsAny(hint, `/\`):
		return exportNames{dir: file}
	case !paths.BadRunes(hint) && utf8.ValidString(hint) && paths.FolderName(hint, "") == hint:
		return exportNames{job: hint, dir: hint}
	}
	return exportNames{bad: true}
}

// hitsIn — свои и чужие файлы с номерами в dir (одно чтение каталога под пределом). known = false — не
// успели или не прочитали: строки нет, после щелчка ReplaceQuestion спросит, как в меню.
func hitsIn(r *folderRules, sm exporter.Summary, dir string) (ours []string, foreign, known bool) {
	type res struct {
		ours, foreign []string
		err           error
	}
	got, ok := within(r.timeout, func() res {
		o, f, err := sm.Hits(dir)
		return res{o, f, err}
	})
	if !ok || got.err != nil {
		return nil, false, false
	}
	return got.ours, len(got.foreign) > 0, true
}

// ownIn — свои файлы выгрузок в dir без фильтра плана (одно чтение каталога под пределом); known = false —
// не успели или не прочитали.
func ownIn(r *folderRules, sm exporter.Summary, dir string) (ours []string, known bool) {
	type res struct {
		ours []string
		err  error
	}
	got, ok := within(r.timeout, func() res {
		o, err := sm.Own(dir)
		return res{o, err}
	})
	if !ok || got.err != nil {
		return nil, false
	}
	return got.ours, true
}

// planExport считает папку для вопроса. Источник (первый годный): подсказка-путь; корень выгрузок и имя
// альбома; иначе пути нет — окно ОС. Негодная подсказка агента не подменяется предложением программы.
func (a *App) planExport(r *folderRules, ed *album.Editor, sm exporter.Summary, hint string, nm exportNames) exportPlan {
	p := exportPlan{badHint: nm.bad}
	var base writeDir
	have := false
	switch {
	case r == nil || nm.bad:
	case strings.ContainsAny(hint, `/\`):
		if w, err := r.writeDir(strings.TrimSpace(hint)); err == nil {
			base, have = w, true
		} else {
			p.badHint = true
		}
	case nm.dir != "":
		base, have = a.exportSuggest(r, sm, nm.dir)
	}
	if !have {
		return p
	}
	if base, p.nextTo, have = exportBesidePhotos(r, ed, base); !have {
		return exportPlan{badHint: p.badHint}
	}
	p.pick, p.pickNew = base.path, base.create
	p.shown, p.create = base.path, base.create
	if sm.Print != "" { // папка книги — внутри выбранной
		if base.create {
			p.shown = filepath.Join(base.path, sm.Book)
		} else {
			book, exists, err := printshop.BookDir(base.path, sm.Book)
			if err != nil {
				return exportPlan{badHint: p.badHint}
			}
			p.shown, p.create = book, !exists
		}
	}
	if !p.create {
		ours, foreign, known := hitsIn(r, sm, p.shown)
		if known {
			p.ours, p.foreign = ours, foreign
		}
	}
	p.net = quietRemote(p.shown)
	p.cloud = inAny(r, p.shown, exportCloudRoots())
	return p
}

// exportBesidePhotos — путь внутри папки фото альбома заменяется соседней (121): человеку сразу видно, куда
// запишется, и после щелчка второй вопрос не нужен. have = false — соседней нет или она не годится.
func exportBesidePhotos(r *folderRules, ed *album.Editor, w writeDir) (out writeDir, beside, have bool) {
	folders, _ := ed.Sources() // отдельные файлы (097) не проверяем: сканер их папку не обходит
	root, in := export.AlbumFolderOf(w.path, folders, paths.Linked)
	if !in {
		return w, false, true
	}
	next, ok := export.NextToAlbum(root, folders, paths.Linked)
	if !ok {
		return w, false, false
	}
	dir, _, err := paths.BookDir(filepath.Dir(next), filepath.Base(next), "выгрузки")
	if err != nil {
		return w, false, false
	}
	nw, err := r.writeDir(dir)
	if err != nil {
		return w, false, false
	}
	return nw, true, true
}

// exportSuggest — папка без подсказки-пути: корень выгрузок (куда человек выгружал в прошлый раз) и имя
// альбома. Корень со своими номерными файлами — это папка «на клиента»: берётся её родитель, чтобы следующий
// клиент не лёг внутрь прошлого. Типография: возвращается сам корень, книга ляжет внутри.
func (a *App) exportSuggest(r *folderRules, sm exporter.Summary, name string) (writeDir, bool) {
	root := prefs.ExportRoot(a.loadConfig())
	if root == "" {
		return writeDir{}, false
	}
	w, err := r.writeDir(root)
	if err != nil || w.create {
		return writeDir{}, false
	}
	// Свои файлы считаются без фильтра плана: диапазон или «только обложка» не делают папку клиента чужой.
	if ours, known := ownIn(r, sm, w.path); known && len(ours) > 0 {
		if w, err = r.writeDir(r.parent(w.path)); err != nil || w.create {
			return writeDir{}, false
		}
	}
	if sm.Print != "" {
		return w, true
	}
	t, err := r.writeDir(filepath.Join(w.path, name))
	if err != nil {
		return writeDir{}, false
	}
	return t, true
}
