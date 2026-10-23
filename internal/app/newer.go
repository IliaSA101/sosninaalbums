package app

import (
	"errors"
	"log"
	"path/filepath"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
)

// Версия программы в файле (098 design §6): текст «файл новее программы» с версией записавшей
// программы и тост о копии старого формата. Отдельно от files.go: тот у предела строк.

// newerText — текст ошибки открытия err для what (первое слово как есть: «альбом» в ошибку для
// фронта — со строчной, как ошибки Go; «Черновик» в начало уведомления): файл новее программы с
// версией из поля app — «альбом сохранён в версии 0.25. Эта программа — 0.23: обновите её»;
// версии в файле нет — «… сохранён более новой версией программы»; не ErrNewer — err.Error().
// Без точки в конце.
func newerText(err error, what string) string { return newerTextFor(err, what, appVersion(wailsJSON)) }

// newerTextFor — newerText с версией этой программы mine (тесты подставляют свою).
func newerTextFor(err error, what, mine string) string {
	var ne *salbum.NewerError
	if !errors.As(err, &ne) {
		if errors.Is(err, salbum.ErrNewer) {
			return what + " сохранён более новой версией программы"
		}
		return err.Error()
	}
	theirs := shortVer(ne.App)
	switch {
	case theirs == "":
		return what + " сохранён более новой версией программы"
	case shortVer(mine) == "":
		return what + " сохранён в версии " + theirs + ": обновите программу"
	}
	return what + " сохранён в версии " + theirs + ". Эта программа — " + shortVer(mine) + ": обновите её"
}

// shortVer — «0.25.1-rc2» → «0.25»: в тексте важны major.minor. Не версия (dev, пусто) — "".
func shortVer(v string) string {
	major, rest, ok := strings.Cut(v, ".")
	if !ok || !digits(major) {
		return ""
	}
	minor := rest
	if i := strings.IndexFunc(rest, func(c rune) bool { return c < '0' || c > '9' }); i >= 0 {
		minor = rest[:i]
	}
	if minor == "" {
		return ""
	}
	return major + "." + minor
}

// digits — непустая строка цифр.
func digits(s string) bool { return s != "" && strings.Trim(s, "0123456789") == "" }

// openErr — ошибка открытия альбома для ответа фронту: файл новее программы — с версией.
func openErr(err error) error {
	if errors.Is(err, salbum.ErrNewer) {
		return errors.New(newerText(err, "альбом"))
	}
	return err
}

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
	if !ok || shortVer(ver) != ver {
		return ""
	}
	return ver
}
