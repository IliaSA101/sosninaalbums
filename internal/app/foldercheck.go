package app

import (
	"sync"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/library"
)

// Сверка папок проекта с диском (088): интерфейс зовёт CheckFolders при фокусе окна, по таймеру
// и при ошибке миниатюры; часто — решает Go. Обход без чтения заголовков сравнивается с отпечатком
// последнего чтения (library.Scanner.Changed); изменилось — тихое перечитывание (rescanWith).
// Editor не трогается: ни шага отмены, ни rev, ни «несохранено».

// checkMinGap — проверки не чаще этого; и не чаще checkTookMul длительностей прошлого обхода:
// сетевой обход за секунду иначе занимал бы диск постоянно (088-8). checkMaxGap — потолок
// множителя: обход, повисший на SMB на минуту, не должен глушить сверку на десять.
const (
	checkMinGap  = 2 * time.Second
	checkMaxGap  = time.Minute
	checkTookMul = 10
)

// checkGap — пауза после обхода длительностью took.
func checkGap(took time.Duration) time.Duration {
	return max(checkMinGap, min(checkTookMul*took, checkMaxGap))
}

// folderCheck — ворота сверки: одна за раз и порог частоты. Нулевое значение — свободны.
type folderCheck struct {
	mu       sync.Mutex
	running  bool
	pendFull bool          // полную отбросил порог: следующая будет полной
	end      time.Time     // конец прошлой проверки; нулевое — проверок не было
	took     time.Duration // длительность прошлого обхода
}

// enter пускает проверку в момент now; full — с ней и пересчёт недостающих. Отказ запоминает
// full, пропуск отдаёт его вместе со своим (eff).
func (g *folderCheck) enter(full bool, now time.Time) (eff, ok bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.running || !g.end.IsZero() && now.Sub(g.end) < checkGap(g.took) {
		g.pendFull = g.pendFull || full
		return false, false
	}
	eff, g.pendFull, g.running = full || g.pendFull, false, true
	return eff, true
}

// leave отмечает конец проверки с обходом длительностью took.
func (g *folderCheck) leave(took time.Duration, now time.Time) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.running, g.end, g.took = false, now, took
}

// CheckFolders сверяет папки проекта с диском и сразу возвращается; работа — в фоне. full —
// ещё и пересчёт недостающих фото альбома (фокус, показ окна). Изменилось — лента перечитывается
// тихо (новые встают на места, пропавшие уходят); нет — ни событий, ни записей в журнал.
// Во время чтения папок и до запуска программы не начинается.
func (a *App) CheckFolders(full bool) { a.beginCheck(full) }

// beginCheck — CheckFolders; false — проверка не начата (порог, идёт другая, идёт чтение).
func (a *App) beginCheck(full bool) bool {
	defer guard("CheckFolders")
	select {
	case <-a.ready:
	default:
		return false // startup ещё открывает альбом и сам прочтёт папки
	}
	if a.stopCtx.Err() != nil {
		return false
	}
	a.mu.Lock()
	scanning := a.scanning
	a.mu.Unlock()
	if scanning {
		return false // идущее чтение и так свежее; недостающие пересчитает оно же
	}
	eff, ok := a.check.enter(full, time.Now())
	if !ok {
		return false
	}
	a.bg.Go(func() {
		var took time.Duration
		defer func() { a.check.leave(took, time.Now()) }()
		defer guard("сверка папок")
		took = a.checkFolders(eff)
	})
	return true
}

// checkFolders — тело проверки; отдаёт длительность обхода.
func (a *App) checkFolders(full bool) time.Duration {
	src := a.sources()
	live, _ := splitLive(src.Folders) // недоступный том не обходим: ждали бы таймаута SMB
	liveFiles, _ := splitLive(src.Files)
	changed, took, err := a.scanner.Changed(a.stopCtx, library.Sources{Folders: live, Files: liveFiles})
	if err != nil {
		return took // остановка программы
	}
	if changed {
		a.rescanWith(scanQuiet) // он же тихо пересчитает недостающие
	} else if full {
		a.refreshMissingQuiet()
	}
	return took
}
