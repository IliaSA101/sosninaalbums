package app

import (
	"fmt"
	"log"
	"os"

	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// dropStat — os.Stat перетаскивания; в тесте подменяется, чтобы убедиться, что до отказа
// CheckLocalPath к файловой системе (на UNC-пути — к сети) никто не обращается.
var dropStat = os.Stat

// dropPaths добавляет брошенные в окно папки. Путь сначала проверяет CheckLocalPath и только
// потом Stat: UNC-папка иначе вешала бы окно на сетевом запросе. Отказы CheckLocalPath копятся:
// один путь — тост с путём, два и больше — один тост со счётом (бросок с сетевого диска иначе
// засыпал бы тостами, 075-10). log.txt — без путей (в пути имя пользователя и название альбома).
func (a *App) dropPaths(ps []string) {
	fail := func(err error) {
		log.Printf("перетаскивание: %v", paths.NoPath(err))
		a.emit("notice", err.Error())
	}
	var bad []error // отказы CheckLocalPath
	for _, p := range ps {
		if err := paths.CheckLocalPath(p); err != nil {
			bad = append(bad, err)
			continue
		}
		st, err := dropStat(p)
		if err != nil || !st.IsDir() {
			continue // не папка (файл, пропавший путь) — как прежде, молча
		}
		v, err := a.editor().AddFolder(p)
		if err != nil {
			fail(err)
			continue
		}
		a.changed(v)
	}
	switch n := len(bad); {
	case n == 1:
		fail(bad[0])
	case n > 1:
		log.Printf("перетаскивание: %d %s отклонено", n, plural(n, "путь", "пути", "путей"))
		a.emit("notice", fmt.Sprintf(`Не добавлено путей: %d — поддерживаются только локальные пути вида C:\папка`, n))
	}
	a.rescan()
}
