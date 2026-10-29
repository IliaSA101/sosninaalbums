package app

// Host службы агентов (143, Г5): замыкания над App, которые отдают службе копии данных открытого
// альбома и библиотеки. Контракт (design §5): ничего не пишут (ни файлов, ни настроек, ни
// событий), не ждут долгих операций под a.mu (загрузка, автосборка, выгрузка), с медленным диском —
// только через проверку тома; ошибки — без путей и имён файлов, с длинными операциями — ctx.

import (
	"context"
	"errors"
	"io/fs"
	"slices"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/model/printprof"
	"github.com/IliaSA101/sosninaalbums/internal/app/agents"
	"github.com/IliaSA101/sosninaalbums/internal/app/colorpick"
	"github.com/IliaSA101/sosninaalbums/internal/decor"
	"github.com/IliaSA101/sosninaalbums/internal/imgfile"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/osx"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
	"github.com/IliaSA101/sosninaalbums/internal/text"
)

// Ошибки картинок для агента: фиксированные тексты, ни пути, ни имени файла (в ошибках
// cache.FileCtx и render имя фото бывает, а текст идёт в log.txt).
var (
	errAgentsImage  = errors.New("картинка фото не получена")
	errAgentsRender = errors.New("разворот не нарисован")
)

// agentsHost — что службе агентов нужно от App. Позднее связывание: cfgDir, cache и editor читаются
// при каждом вызове (cfgDir ставит startup, редактор подменяется при открытии альбома).
func (a *App) agentsHost() agents.Host {
	return agents.Host{
		CfgDir:       func() string { return a.cfgDir },
		Version:      func() string { return appVersion(wailsJSON) },
		Sock:         func() string { return osx.MCPSock(a.data) },
		Changed:      a.agentsChanged,
		View:         a.agentsView,
		Photos:       a.agentsPhotos,
		Marks:        func() map[string]model.MarkView { return a.editor().Marks() },
		Status:       a.agentsStatus,
		Decor:        a.agentsDecor,
		Packs:        a.agentsPacks,
		Prints:       func() []printprof.Info { return printprof.Current().Infos() },
		Recent:       func() []prefs.RecentProject { return prefs.RecentView(a.loadConfig().Recent) },
		FontsMissing: agentsFontsMissing,
		Preview:      a.agentsPreview,
		Render:       a.agentsRender,
	}
}

// agentsView — вид, который видит окно: последний отданный ему (несохранённое тоже), иначе вид
// редактора. edID 0 — альбома ещё нет (startup не закончил). Холодные раскладки считаются вне a.mu.
func (a *App) agentsView() (model.View, uint64) {
	select {
	case <-a.ready:
	default:
		return model.View{}, 0
	}
	ed := a.editor()
	a.mu.Lock()
	if a.wire.sentEd.Value() == ed && a.wire.seq > 0 {
		v := a.wire.sent
		a.mu.Unlock()
		return v, ed.ID()
	}
	a.mu.Unlock()
	return ed.View(), ed.ID()
}

// agentsPhotos — копия ленты фото под a.mu.
func (a *App) agentsPhotos() []library.Photo {
	a.mu.Lock()
	defer a.mu.Unlock()
	return slices.Clone(a.photos)
}

// agentsStatus — пропавшие и нечитаемые фото. К пропавшим добавляются фото альбома, чей том не
// отвечает или путь не локальный (UNC): Render рисует их пустыми, и подпись «unavailable» должна их
// считать (0.31-143-71). Проверка тома — osx.Reachable с кэшем, а не обращение к диску.
func (a *App) agentsStatus() ([]string, map[string]string) {
	a.mu.Lock()
	missing := slices.Clone(a.missing)
	unread := a.unreadAllLocked()
	a.mu.Unlock()
	for id, r := range a.editor().Refs() {
		if r.Lib != "" || r.Path == "" || slices.Contains(missing, id) {
			continue
		}
		if paths.CheckLocalPath(r.Path) != nil || !reachable(r.Path) {
			missing = append(missing, id)
		}
	}
	slices.Sort(missing)
	return missing, unread
}

// agentsDecor — каталог оформления вида kind в узком виде: без адресов картинок и плиток «Можно
// скачать». "look" — макеты альбома, "template" — «Мои шаблоны» (не каталог библиотеки).
func (a *App) agentsDecor(kind string) (agents.DecorCatalogLite, error) {
	if kind == "template" {
		return agentsTemplates(a.templateList()), nil
	}
	if kind == "look" {
		kind = decor.KindAlbum
	}
	s, err := a.decorStore()
	if err != nil {
		return agents.DecorCatalogLite{}, err
	}
	w, h := a.editor().SheetSize()
	cat, err := s.Catalog(kind, nil, w, h)
	if err != nil {
		return agents.DecorCatalogLite{}, err
	}
	out := agents.DecorCatalogLite{New: cat.New}
	for _, t := range cat.Themes {
		out.Themes = append(out.Themes, agents.DecorThemeLite{ID: t.ID, Name: t.Name, Count: t.Count})
	}
	for _, it := range cat.Items {
		out.Items = append(out.Items, agents.DecorItemLite{ID: it.ID, Theme: it.Theme, Name: it.Name, Own: it.Own,
			PackID: it.PackID, Pack: it.Pack, W: it.W, H: it.H, Pages: it.Pages, Places: it.Places,
			Steps: it.Steps, Vig: it.Vig, Cover: it.Cover})
	}
	return out, nil
}

// agentsTemplates — «Мои шаблоны» (templates.json) как плитки каталога: страница или разворот,
// число мест. Имена даёт человек (служба на уровне 1 заменит их общими).
func agentsTemplates(list []model.Template) agents.DecorCatalogLite {
	out := agents.DecorCatalogLite{Themes: []agents.DecorThemeLite{{ID: decor.ThemeAll, Name: "Все", Count: len(list)}}}
	for _, t := range list {
		pages := 2
		if t.Scope == model.TplPage {
			pages = 1
		}
		out.Items = append(out.Items, agents.DecorItemLite{ID: t.ID, Theme: decor.ThemeMine, Name: t.Name, Own: true,
			Pages: pages, Places: len(t.Boxes)})
	}
	return out
}

// agentsPacks — стоящие наборы и сохранённый список «Можно скачать»; в сеть не ходит (libpacks.List).
func (a *App) agentsPacks() (agents.PacksLite, error) {
	l, err := a.libp.List()
	if err != nil {
		return agents.PacksLite{}, err
	}
	out := agents.PacksLite{AvailAt: l.AvailAt, Allowed: l.Net.Allowed, AtStart: l.Net.AtStart}
	for _, p := range l.Packs {
		out.Installed = append(out.Installed, agents.PackLite{ID: p.ID, Title: p.Title, Author: p.Author, License: p.License,
			Origin: p.Origin, Version: p.Version, Bg: p.Bg, Sticker: p.Sticker, Spread: p.Spread, Frame: p.Frame,
			Print: p.Print, Bytes: p.Bytes})
	}
	for _, p := range l.Available {
		out.Available = append(out.Available, agents.PackAvailLite{ID: p.ID, Title: p.Title, Version: p.Version, Bytes: p.Bytes})
	}
	return out, nil
}

// agentsFontsMissing — названия своих гарнитур альбома, которых нет на этом компьютере.
func agentsFontsMissing(used []model.FontUse) []string {
	var out []string
	for _, f := range used {
		if !text.IsCustomID(f.ID) || text.Available(f.ID) {
			continue
		}
		name := f.Name
		if name == "" {
			name = "Свой шрифт"
		}
		out = append(out, name)
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// agentsImageErr приводит ошибку получения картинки к тому, что можно вернуть службе: отмена —
// как есть, нет файла или он не читается — ErrUnavailable, прочее — безымянный текст.
func agentsImageErr(ctx context.Context, err error) error {
	var de *library.DecodeError
	switch {
	case ctx.Err() != nil:
		return ctx.Err()
	case errors.Is(err, fs.ErrNotExist), errors.As(err, &de):
		return agents.ErrUnavailable
	}
	return errAgentsImage
}

// agentsPreview — фото id, уменьшенное до px: источник тот же, что у /media/p (mediaPhoto:
// CheckLocalPath, проверка тома, пропавшие), копия из кэша превью читается через imgfile (предел
// размера и бюджет памяти до декода). Диск без a.mu.
func (a *App) agentsPreview(ctx context.Context, id string, px int) ([]byte, error) {
	if a.cache == nil {
		return nil, errAgentsImage
	}
	// mediaPhoto для фото ленты проверяет только том: UNC (\\сервер\ресурс) не локальный путь, и проба
	// его тома — это сетевое обращение. Лента такого не несёт (AddFolder проверяет путь), но Host не
	// полагается на это и отказывает до пробы (0.31-143-61).
	a.mu.Lock()
	sp, inStrip := a.byID[model.BaseID(id)]
	a.mu.Unlock()
	if inStrip && paths.CheckLocalPath(sp.Path) != nil {
		return nil, agents.ErrUnavailable
	}
	p, ok, gone := a.mediaPhoto(id)
	if !ok || gone {
		return nil, agents.ErrUnavailable
	}
	file, err := a.cache.FileCtx(ctx, p, true)
	if err != nil {
		return nil, agentsImageErr(ctx, err)
	}
	img, release, err := imgfile.OpenOrientedCtx(ctx, file)
	if err != nil {
		return nil, agentsImageErr(ctx, err)
	}
	defer release()
	b, err := agents.EncodePreview(img, px)
	if err != nil {
		return nil, errAgentsImage
	}
	return b, nil
}

// agentsRender — разворот или обложка spreadID видом печати, не шире px: рендер пипетки по превью
// фото (colorpick.Picker.Sheet), JPEG — кодировщик службы. Ошибки render пишут имя файла фото —
// наружу и в log.txt идёт безымянный текст.
func (a *App) agentsRender(ctx context.Context, spreadID string, px int) ([]byte, error) {
	img, err := a.colorPick.Sheet(ctx, spreadID, px)
	switch {
	case errors.Is(err, colorpick.ErrNoSheet):
		return nil, agents.ErrNoSpread
	case ctx.Err() != nil:
		return nil, ctx.Err()
	case err != nil:
		return nil, errAgentsRender // в log.txt его пишет защита вызова (безымянный текст)
	}
	b, err := agents.EncodePreview(img, px)
	if err != nil {
		return nil, errAgentsRender
	}
	return b, nil
}
