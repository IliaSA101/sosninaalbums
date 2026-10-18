package app

import (
	"log"
	"os"

	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// dropStat — os.Stat перетаскивания; в тесте подменяется, чтобы убедиться, что до отказа
// CheckLocalPath к файловой системе (на UNC-пути — к сети) никто не обращается.
var dropStat = os.Stat

// dropPaths добавляет брошенные в окно папки. Путь сначала проверяет CheckLocalPath и только
// потом Stat: UNC-папка иначе вешала бы окно на сетевом запросе. Тост получает текст с путём,
// log.txt — без него (в пути имя пользователя и название альбома).
func (a *App) dropPaths(ps []string) {
	fail := func(err error) {
		log.Printf("перетаскивание: %v", paths.NoPath(err))
		a.emit("notice", err.Error())
	}
	for _, p := range ps {
		if err := paths.CheckLocalPath(p); err != nil {
			fail(err)
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
	a.rescan()
}
