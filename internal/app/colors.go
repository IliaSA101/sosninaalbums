package app

// Окно выбора цвета (спека 109): «Свои цвета» (config.json → myColors, правила — internal/prefs)
// и «С фото» (SpreadPhotoColors). Альбом методы не меняют: Host.Changed, событий и rev нет.

import (
	"container/list"
	"context"
	"errors"
	"slices"
	"sync"
	"sync/atomic"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/colorspace"
	"github.com/IliaSA101/sosninaalbums/internal/imgfile"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

// MyColors — «Свои цвета» из config.json после проверки; никогда не nil.
func (a *App) MyColors() []string { return prefs.MyColors(a.cfgDir) }

// RememberColor ставит hex первым (без повторов, ≤ 10); мусор — список без изменений.
func (a *App) RememberColor(hex string) []string { return prefs.RememberColor(a.cfgDir, hex) }

// ForgetColor убирает hex; нет такого — список без изменений.
func (a *App) ForgetColor(hex string) []string { return prefs.ForgetColor(a.cfgDir, hex) }

// ImportColors — перенос прежних «Последних» из localStorage: пишет, только если в config.json
// пусто, иначе отдаёт что есть (второе окно не задвоит список).
func (a *App) ImportColors(list []string) []string { return prefs.ImportColors(a.cfgDir, list) }

const (
	maxPhotoColors = 8   // цветов в «С фото»
	photoGrid      = 32  // сетка выборки миниатюры 32×32 = 1024 точки
	pointsCacheLen = 512 // фото в кэше точек: 512 × 1024 × 12 Б ≈ 6 МБ
)

var errNoSheet = errors.New("лист не найден")

// colorState — состояние методов цвета App: кэш точек фото и счётчик промахов для журнала.
type colorState struct {
	pts  pointsCache
	miss atomic.Int64
}

// pointsCache — LRU точек OKLab по пути миниатюры. Миниатюра неизменна: сменилось фото —
// сменился ключ кэша миниатюр и её путь.
type pointsCache struct {
	mu sync.Mutex
	ll *list.List // от свежих к старым; значение — *pointsEntry
	m  map[string]*list.Element
}

type pointsEntry struct {
	key string
	pts []colorspace.Lab
}

func (c *pointsCache) get(key string) ([]colorspace.Lab, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.m[key]
	if !ok {
		return nil, false
	}
	c.ll.MoveToFront(e)
	return e.Value.(*pointsEntry).pts, true
}

func (c *pointsCache) put(key string, pts []colorspace.Lab) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.m == nil {
		c.ll, c.m = list.New(), map[string]*list.Element{}
	}
	if e, ok := c.m[key]; ok {
		e.Value.(*pointsEntry).pts = pts
		c.ll.MoveToFront(e)
		return
	}
	c.m[key] = c.ll.PushFront(&pointsEntry{key: key, pts: pts})
	for c.ll.Len() > pointsCacheLen {
		old := c.ll.Back()
		c.ll.Remove(old)
		delete(c.m, old.Value.(*pointsEntry).key)
	}
}

// sampleThumb читает миниатюру и берёт сетку точек; подменяется в тестах (счёт чтений).
var sampleThumb = func(ctx context.Context, path string) ([]colorspace.Lab, error) {
	img, release, err := imgfile.OpenOrientedCtx(ctx, path)
	if err != nil {
		return nil, err
	}
	defer release()
	return colorspace.Sample(img, photoGrid), nil
}

// viewSheet — лист (разворот или обложка) по ID из вида.
func viewSheet(v model.View, id string) (model.SpreadView, bool) {
	if v.Cover != nil && v.Cover.Sheet.ID == id {
		return v.Cover.Sheet, true
	}
	for _, s := range v.Spreads {
		if s.ID == id {
			return s, true
		}
	}
	return model.SpreadView{}, false
}

// colorPhotoIDs — фото листа для «С фото»: места и фото на фоне, без повторов. Стикеры и своё из
// библиотеки (фоны, узоры) — оформление, а не съёмка (109-3).
func colorPhotoIDs(s model.SpreadView) []string {
	ids := slices.Clone(s.Photos)
	for _, b := range s.Backdrops {
		if b.Lib == "" {
			ids = append(ids, b.Photo)
		}
	}
	seen := make(map[string]bool, len(ids))
	return slices.DeleteFunc(ids, func(id string) bool {
		dup := id == "" || seen[model.BaseID(id)]
		seen[model.BaseID(id)] = true
		return dup
	})
}

// SpreadPhotoColors — до 8 главных цветов фото листа по убыванию доли (109-3). Каждое фото весит
// одинаково: это цвета съёмки, а не площади. Пропавший файл или ошибка миниатюры — фото
// пропускается; нет фото — []. Альбом не меняется.
func (a *App) SpreadPhotoColors(spreadID string) (cols []string, err error) {
	defer guardErr("SpreadPhotoColors", &err)
	sv, ok := viewSheet(a.editor().View(), spreadID)
	if !ok {
		return []string{}, errNoSheet
	}
	if a.cache == nil {
		return []string{}, nil
	}
	var pts []colorspace.Lab
	for _, id := range colorPhotoIDs(sv) {
		p, ok, gone := a.mediaPhoto(id)
		if !ok || gone {
			continue
		}
		file, err := a.cache.FileCtx(a.stopCtx, p, false)
		if err != nil {
			logRare(a.colors.miss.Add(1), "цвета фото: миниатюра: %v", paths.NoPath(err))
			continue
		}
		got, hit := a.colors.pts.get(file)
		if !hit {
			if got, err = sampleThumb(a.stopCtx, file); err != nil {
				logRare(a.colors.miss.Add(1), "цвета фото: чтение: %v", paths.NoPath(err))
				continue
			}
			a.colors.pts.put(file, got)
		}
		pts = append(pts, got...)
	}
	return colorspace.Dominant(pts, maxPhotoColors), nil
}
