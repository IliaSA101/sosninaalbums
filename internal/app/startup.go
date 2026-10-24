package app

import (
	"errors"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/osx"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// Выбор проекта при запуске: разрезано из app.go (предел размера файла).

// loadStartup открывает альбом из аргумента запуска (двойной щелчок, 112), а если его нет —
// последний проект, а если и его нет — черновик; иначе пустой. Альбом аргумента становится
// последним и попадает в «Последние»; не открылся — запуск идёт как без него (с заметкой).
// Файл, который есть, но не открылся, не должен быть затёрт пустым проектом:
// чужой .salbum остаётся как есть (path пуст, автосохранение пишет только черновик),
// а сломанный черновик уходит в «черновик.bad-<время>.salbum» рядом. Черновик более новой
// версии (ErrNewer) не сломан: его не трогаем, а автосохранение черновика на сессию выключаем.
// Файл и черновик новее программы — не заметка, а State.Newer (startNewer).
func (a *App) loadStartup(last string) (p *model.Project, path, notice string) {
	var notes []string
	arg := a.takeArg()
	switch {
	case arg == "":
	case paths.SamePath(arg, last):
		last = "" // тот же файл: открывается как аргумент
	case paths.SamePath(arg, a.draftPath()):
		arg = "" // черновик открывается своим путём
	}
	for _, cand := range []string{arg, last, a.draftPath()} {
		if cand == "" {
			continue
		}
		if cand != a.draftPath() && !reachable(cand) {
			// Отключённый сетевой диск: Load ждал бы таймаута, окно — пустое. LastProject не трогаем:
			// со следующим запуском при сети альбом откроется.
			if cand == arg {
				notes = append(notes, "Диск "+filepath.VolumeName(cand)+" с альбомом недоступен.")
			} else {
				notes = append(notes, "Диск "+filepath.VolumeName(cand)+" с последним альбомом недоступен. Открыт черновик.")
			}
			continue
		}
		loaded, err := a.loadCand(cand)
		if err == nil {
			if cand == a.draftPath() {
				cand = ""
			}
			if cand == arg {
				a.rememberProject(cand)
			}
			return loaded, cand, strings.Join(notes, " ")
		}
		if note := a.startFailure(cand, arg, last, err); note != "" {
			notes = append(notes, note)
		}
	}
	return model.New(), "", strings.Join(notes, " ")
}

// loadCand читает кандидата запуска: черновик и копию — как есть, альбом — вместе со шрифтами его папки.
func (a *App) loadCand(cand string) (*model.Project, error) {
	if cand == a.draftPath() || a.backups.Inside(cand) {
		return salbum.Load(cand)
	}
	loaded, af, err := a.loadAlbum(cand)
	a.keepAlbumFonts(af) // шрифты папки собранного альбома (082-8): State.Info, не ошибка
	return loaded, err
}

// startFailure — заметка о кандидате cand, который не открылся (arg и last — кандидаты запуска);
// сломанный черновик уходит в .bad, черновик новее программы остаётся на месте.
func (a *App) startFailure(cand, arg, last string, err error) string {
	if errors.Is(err, os.ErrNotExist) {
		switch cand {
		case arg:
			return "Альбом не найден: " + filepath.Base(cand) + "."
		case last:
			return "Последний проект не найден: " + cand + ". Открыт черновик."
		}
		return ""
	}
	log.Printf("не открылся проект: %v", paths.NoPath(err))
	if errors.Is(err, salbum.ErrNewer) {
		// Окно «Альбом из новой версии» (112) вместо тоста: первый GetState отдаёт State.Newer.
		a.startNewer(cand, cand == a.draftPath(), err)
		if cand == a.draftPath() {
			a.draftNewer.Store(true)
		}
		return ""
	}
	if cand != a.draftPath() {
		return "Не открылся проект: " + newerText(err, "альбом") + "."
	}
	bad := a.badDraftPath(time.Now())
	if rerr := a.rename(cand, bad); rerr != nil {
		log.Printf("черновик не удалось сохранить отдельно: %v", paths.NoPath(rerr))
		a.draftLocked.Store(true)
		return "Не открылся черновик (" + err.Error() + ") и не сохранён отдельно: автосохранение отключено до перезапуска."
	}
	return "Не открылся черновик (" + err.Error() + "). Копия сохранена: " + bad + "."
}

// startEditor — редактор для проекта loadStartup; возвращает путь и уведомление. Load отдаёт
// проверенный проект, и отказ NewEditor сюда не доходит (Load ⊆ Check, FuzzLoad). Если всё же —
// пустой альбом, а черновик до перезапуска не пишется (draftLocked). Замок ставится и когда
// отвергнут LastProject при целом черновике: иначе автосохранение пустого альбома затёрло бы
// черновик; «Новый» потом переименует черновик в .bad.
// В log.txt — без текста ошибки: в нём путь.
func (a *App) startEditor(p *model.Project, path, notice string) (string, string) {
	ed, err := album.NewEditor(p, path)
	if err == nil {
		a.ed.Store(ed)
		return path, notice
	}
	log.Printf("проект при запуске не прошёл проверку путей")
	a.draftLocked.Store(true)
	a.ed.Store(blankEditor(""))
	return "", strings.TrimSpace(notice + " Не открылся проект: " + err.Error() + ". Автосохранение отключено до перезапуска.")
}

// launchPlace — где лежит запущенный exe; ошибка os.Executable — молча «обычное место».
func launchPlace() osx.Place {
	exe, err := os.Executable()
	if err != nil {
		return osx.PlaceNormal
	}
	return osx.LaunchPlace(exe, os.TempDir())
}

// joinLaunchNotice дописывает к тосту старта подсказку о месте запуска (через пустую строку,
// если текст уже есть) и пишет её в log.txt; для обычного места возвращает notice как есть.
func joinLaunchNotice(notice string, place osx.Place) string {
	extra := place.Notice()
	if extra == "" {
		return notice
	}
	log.Printf("запуск не из установленного места: %s", place)
	if notice == "" {
		return extra
	}
	return notice + "\n\n" + extra
}
