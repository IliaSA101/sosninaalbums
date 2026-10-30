package app

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

// dropStat — os.Lstat перетаскивания: ссылка не проходится до проверки всей её цепочки
// (library.LinkTarget, 0.22-sec-3). В тесте подменяется, чтобы убедиться, что до отказа
// CheckLocalPath и предела числа путей к файловой системе (на UNC-пути — к сети) никто не обращается.
var dropStat = os.Lstat

// dropPaths добавляет брошенные в окно папки и фото (файлы — одним добавлением, 097). Путь сначала проверяет CheckLocalPath и только
// потом Lstat: UNC-папка иначе вешала бы окно на сетевом запросе. Отказы CheckLocalPath копятся:
// один путь — тост с путём, два и больше — один тост со счётом (бросок с сетевого диска иначе
// засыпал бы тостами, 075-10). log.txt — без путей (в пути имя пользователя и название альбома).
// Путей больше MaxLooseFiles — отказ looseTooMany до любого обращения к диску (0.22-sec-5).
func (a *App) dropPaths(ps []string) {
	fail := func(err error) {
		log.Printf("перетаскивание: %v", paths.NoPath(err))
		a.emit("notice", err.Error())
	}
	if len(ps) > model.MaxLooseFiles {
		fail(looseTooMany(len(ps)))
		return
	}
	var bad []error      // отказы CheckLocalPath
	var files []string   // брошенные файлы: уходят одним добавлением
	var parents []string // родители добавленных папок: места съёмок агента (agentplaces_app.go)
	for _, p := range ps {
		if err := paths.CheckLocalPath(p); err != nil {
			bad = append(bad, err)
			continue
		}
		st, err := dropStat(p)
		if err != nil {
			continue // пропавший путь — молча
		}
		if st.Mode()&(fs.ModeSymlink|fs.ModeIrregular) != 0 {
			// Ссылка: папка она или файл — по цели, если вся цепочка локальна. Нелокальная или
			// битая — в файлы: looseVerdict посчитает её пропущенной, без обращения к цели.
			if _, st, err = library.LinkTarget(p); err != nil {
				files = append(files, p)
				continue
			}
		}
		if !st.IsDir() {
			files = append(files, p)
			continue
		}
		v, r, err := a.editor().AddFolder(p)
		if err != nil {
			fail(err)
			continue
		}
		a.changed(v)
		a.folderAdded(p, r)
		parents = append(parents, filepath.Dir(p))
	}
	if len(parents) > 0 {
		a.pushPlaces(prefs.PlaceParent, parents...)
	}
	switch n := len(bad); {
	case n == 1:
		fail(bad[0])
	case n > 1:
		log.Printf("перетаскивание: %d %s отклонено", n, plural(n, "путь", "пути", "путей"))
		a.emit("notice", fmt.Sprintf(`Не добавлено путей: %d — поддерживаются только локальные пути вида C:\папка`, n))
	}
	a.dropFiles(files)
	a.rescan()
}

// dropFiles — брошенные файлы одним добавлением; итог уходит событием `loose:added` (ответа у броска нет).
func (a *App) dropFiles(files []string) {
	if len(files) == 0 {
		return
	}
	r, err := a.addLoose(files)
	if err != nil {
		log.Printf("перетаскивание: %v", paths.NoPath(err))
		a.emit("notice", err.Error())
		return
	}
	a.emit("loose:added", model.LooseAdded{Batch: r.Batch, Added: r.Added, InFolder: r.InFolder, Folder: r.Folder,
		Dup: r.Dup, Skipped: r.Skipped, Bad: r.Bad, Over: r.Over})
}
