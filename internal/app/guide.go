package app

// Справочник F1: отметки «Изучил» (спека 140). Список id лежит в config.json (правила —
// internal/prefs/guide.go), общий для всех альбомов. Альбом методы не меняют: Host.Changed,
// событий, rev и undo нет — это настройки программы, как «Свои цвета» и туры.

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/app/appkit"
	"github.com/IliaSA101/sosninaalbums/internal/app/guidevideo"
	"github.com/IliaSA101/sosninaalbums/internal/decor/fetch"
	"github.com/IliaSA101/sosninaalbums/internal/imgfile"
	"github.com/IliaSA101/sosninaalbums/internal/osx"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

// GuideLearned — отмеченные «Изучил» id из config.json после чистки; никогда не nil.
func (a *App) GuideLearned() []string { return prefs.GuideLearned(a.cfgDir) }

// MarkGuide ставит (on) или снимает отметку с ids одной записью (интерфейс шлёт id статьи и её
// прежние id из RENAMED); возвращает весь список из файла. Негодные id пропускаются.
func (a *App) MarkGuide(ids []string, on bool) []string { return prefs.MarkGuide(a.cfgDir, ids, on) }

// Ролики справочника (спека 140, C): тело — internal/app/guidevideo, здесь фасад для интерфейса,
// определённый тип ответа (models.ts в пространстве app, 126-2) и Host службы.

// GuideVideo — ролик в списке (guidevideo.Item): версия, длина, размер, файл готов.
type GuideVideo guidevideo.Item

func guideVideos(in []guidevideo.Item) []GuideVideo {
	out := make([]GuideVideo, len(in))
	for i, it := range in {
		out[i] = GuideVideo(it)
	}
	return out
}

// GuideVideos — ролики из сохранённого каталога и состояние файлов; без сети, нет каталога — [].
func (a *App) GuideVideos() []GuideVideo { return guideVideos(a.guidev.List()) }

// DownloadGuideVideos скачивает ролики ids; ответ — весь список. Ошибка — готовый русский текст для
// тоста (нет согласия, ролика нет в библиотеке, уже качается, сеть, диск). Ход — событие guide:video.
func (a *App) DownloadGuideVideos(ids []string) (res []GuideVideo, err error) {
	defer appkit.GuardErr("DownloadGuideVideos", &err)
	list, err := a.guidev.Download(ids)
	return guideVideos(list), err
}

// CancelGuideVideos — «Отменить»: идущее скачивание роликов обрывается.
func (a *App) CancelGuideVideos() { a.guidev.Cancel() }

// RevealGuideVideo — «Показать файл»: проводник с роликом id. От окна только id, путь собирает Go.
func (a *App) RevealGuideVideo(id string) (err error) {
	defer appkit.GuardErr("RevealGuideVideo", &err)
	return a.guidev.Reveal(id)
}

// serveGuideVideo отдаёт /media/guide-video/<id>; false — адрес другой.
func (a *App) serveGuideVideo(w http.ResponseWriter, r *http.Request) bool {
	tail, ok := strings.CutPrefix(r.URL.Path, "/media/guide-video/")
	if !ok {
		return false
	}
	a.guidev.Serve(w, r, tail)
	return true
}

// guideCacheDir — <кэш пользователя>/<имя данных>/guide-videos; "" — кэша нет (UserCacheDir без
// HOME или LOCALAPPDATA отдал бы путь от текущей папки: служба роликов тогда выключена, 140 Н4).
// У стенда своя папка (name — osx.DataName). Не dataDirs: своя подпапка guide-videos, а dataDirs
// отдаёт пару настройки и миниатюры.
func guideCacheDir(name string) string {
	base := osx.CacheDir()
	if base == "" {
		return ""
	}
	return filepath.Join(base, name, "guide-videos")
}

// guideHost — что службе роликов нужно от App. Замыкания над a (позднее связывание): наборы
// библиотеки берутся из a.libp на каждом вызове, подмена службы в тестах действует.
func (a *App) guideHost() guidevideo.Host {
	return guidevideo.Host{
		CacheDir:    func() string { return guideCacheDir(a.data) },
		App:         func() string { return appVersion(wailsJSON) },
		OS:          func() string { return runtime.GOOS },
		Allowed:     func() bool { return prefs.LibraryNetOf(a.cfgDir).Allowed },
		StopCtx:     func() context.Context { return a.stopCtx },
		Cached:      func() (fetch.Catalog, bool) { return a.libp.Cached() },
		Fresh:       func(ctx context.Context) (fetch.Catalog, error) { return a.libp.Fresh(ctx) },
		Client:      func() *fetch.Client { return fetch.New(fetch.Default(osx.StandOn())) },
		Emit:        func(name string, data ...any) { a.emit(name, data...) },
		Reveal:      func(path string) error { return osx.Reveal(path, true) },
		OpenRegular: imgfile.OpenRegular,
		Free:        osx.FreeBytes,
		Rename:      os.Rename,
	}
}
