package app

// Что остаётся от открытого альбома, когда агент (или «Открыть прежний») уводит к другому
// (150, design §6): несохранённое закрепляется копией, файл человека молча не переписывается.

import (
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/backup"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// leaving — уход от альбома: prev — файл для «Открыть прежний» ("" — прежнего нет), release — снять
// запрет записи и освободить уход (звать один раз после смены альбома; повтор безвреден).
type leaving struct {
	prev    string
	release func()
}

// pinLeaving готовит уход от cur и занимает его на себя (leaveMu): два ухода подряд не затрут holdSave
// и onRetire друг друга. Свой файл без правок — prev = файл, закрепления нет; свой файл с правками и
// черновик — закреплённая копия, а запись cur в его файл на время ухода запрещена (holdSave). Правка
// человека между копией и Close дозакрепляется в retire (onRetire) — без окна. Копии нет —
// errAgentBackup, уход освобождён, альбом не трогаем.
func (a *App) pinLeaving(cur *album.Editor) (*leaving, error) {
	a.leaveMu.Lock()
	lv := &leaving{}
	var once sync.Once
	lv.release = func() {
		once.Do(func() {
			a.onRetire.Store(nil)
			a.holdSave.CompareAndSwap(cur, nil)
			a.leaveMu.Unlock()
		})
	}
	switch a.askDocOf(cur) {
	case askDocSaved, askDocDirty:
		if !cur.View().Dirty {
			lv.prev = cur.Path()
			return lv, nil
		}
		// Запись в файл запрещена до копии: автосохранение между проверкой и копией файл бы переписало.
		a.holdSave.Store(cur)
		rev := cur.View().Rev // до копии: правка после неё сдвинет rev и вызовет дозакрепление
		name, perr := a.pinDirtyFile(cur)
		if perr != nil || name == "" {
			lv.release()
			return nil, errAgentBackup
		}
		lv.prev, _ = a.backups.File(backupKey(cur.Path()), name)
		fix := func(closed *album.Editor) {
			if closed != cur || closed.View().Rev == rev {
				return
			}
			n, e := a.pinDirtyFile(closed) // Editor закрыт: новых правок уже не будет
			if e != nil {
				log.Printf("правки перед уходом не закреплены: %v", paths.NoPath(e))
				return
			}
			if p, e := a.backups.File(backupKey(closed.Path()), n); e == nil {
				lv.prev = p
			}
		}
		a.onRetire.Store(&fix)
	case askDocDraft:
		if !a.unlockDraft() {
			return lv, nil // startNew откажет сам (draft_locked)
		}
		name, perr := a.agentPinned(cur)
		if perr != nil || name == "" {
			lv.release()
			return nil, errAgentBackup
		}
		lv.prev, _ = a.backups.File(backup.DraftKey, name)
	}
	return lv, nil
}

// pinTmpPrefix — имя временного файла копии в папке настроек (pinDirtyFile).
const pinTmpPrefix = "копия-"

// sweepPinTemps убирает остатки pinDirtyFile после падения процесса: файл с данными альбома не
// должен жить в папке настроек вечно. Зовёт startup до первого ухода.
func sweepPinTemps(dir string) {
	if dir == "" {
		return
	}
	list, _ := filepath.Glob(filepath.Join(dir, pinTmpPrefix+"*"+projectExt))
	for _, f := range list {
		_ = os.Remove(f)
	}
}

// pinDirtyFile закрепляет копией текущее состояние альбома с файлом, не трогая сам файл: проект
// пишется во временный файл рядом с настройками и копируется в хранилище под ключом альбома.
func (a *App) pinDirtyFile(ed *album.Editor) (string, error) {
	if a.backups == nil || a.cfgDir == "" {
		return "", errAgentBackup
	}
	tmp, err := os.CreateTemp(a.cfgDir, pinTmpPrefix+"*"+projectExt)
	if err != nil {
		return "", errAgentBackup
	}
	name := tmp.Name()
	_ = tmp.Close()
	defer func() { _ = os.Remove(name) }()
	if err := ed.WriteCopy(name); err != nil {
		return "", errAgentBackup
	}
	return a.backups.ForcePin(name, backupKey(ed.Path()))
}
