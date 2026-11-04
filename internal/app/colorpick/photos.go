package colorpick

// «С фото» (спека 109 G2): главные цвета фото листа по миниатюрам, кэш точек OKLab.

import (
	"container/list"
	"context"
	"slices"
	"sync"
	"sync/atomic"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/colorspace"
	"github.com/IliaSA101/sosninaalbums/internal/imgfile"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

const (
	maxPhotoColors = 8   // цветов в «С фото»
	photoGrid      = 32  // сетка выборки миниатюры 32×32 = 1024 точки
	pointsCacheLen = 512 // фото в кэше точек: 512 × 1024 × 12 Б ≈ 6 МБ
)

// colorState — состояние «С фото»: кэш точек фото и счётчик промахов для журнала.
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

// PhotoColors — до 8 главных цветов фото листа по убыванию доли (109-3). Каждое фото весит
// одинаково: это цвета съёмки, а не площади. Пропавший файл или ошибка миниатюры — фото
// пропускается; нет фото — []. Альбом не меняется.
func (p *Picker) PhotoColors(spreadID string) ([]string, error) {
	sv, ok := viewSheet(p.h.Editor().View(), spreadID)
	if !ok {
		return []string{}, ErrNoSheet
	}
	cache := p.h.Cache()
	if cache == nil {
		return []string{}, nil
	}
	ctx := p.h.Ctx()
	var pts []colorspace.Lab
	for _, id := range colorPhotoIDs(sv) {
		ph, ok, gone := p.h.Photo(id)
		if !ok || gone {
			continue
		}
		file, err := cache.FileCtx(ctx, ph, false)
		if err != nil {
			p.h.Rare(p.colors.miss.Add(1), "цвета фото: миниатюра: %v", paths.NoPath(err))
			continue
		}
		got, hit := p.colors.pts.get(file)
		if !hit {
			if got, err = sampleThumb(ctx, file); err != nil {
				p.h.Rare(p.colors.miss.Add(1), "цвета фото: чтение: %v", paths.NoPath(err))
				continue
			}
			p.colors.pts.put(file, got)
		}
		pts = append(pts, got...)
	}
	return colorspace.Dominant(pts, maxPhotoColors), nil
}
