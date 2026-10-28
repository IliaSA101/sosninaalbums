package app

import (
	"log"
	"path/filepath"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/app/assoc"
)

// noticeBackup — тост о копии старого формата, сделанной последней записью ed (Save, WriteDraft):
// «Сохранена копия для версии 0.22: «Свадьба (для 0.22).salbum»», один раз (Editor.TakeBackup
// забывает имя). Звать после успешной записи. В log.txt — без имени: в нём имя альбома.
func (a *App) noticeBackup(ed *album.Editor) {
	bak := ed.TakeBackup()
	if bak == "" {
		return
	}
	log.Print("сохранена копия файла для прежней версии программы")
	a.emit("notice", backupText(bak))
}

// backupText — текст тоста о копии bak: версия — из хвоста имени « (для 0.22)» (BackupName).
func backupText(bak string) string {
	name := filepath.Base(bak)
	if ver := tailVer(name); ver != "" {
		return "Сохранена копия для версии " + ver + ": «" + name + "»"
	}
	return "Сохранена копия для прежней версии: «" + name + "»"
}

// tailVer — версия из последнего « (для X.Y)» в имени; нет — "".
func tailVer(name string) string {
	const head = " (для "
	i := strings.LastIndex(name, head)
	if i < 0 {
		return ""
	}
	ver, _, ok := strings.Cut(name[i+len(head):], ")")
	if !ok || assoc.ShortVer(ver) != ver {
		return ""
	}
	return ver
}
