// Package colorpick — пипетка и «С фото» окна выбора цвета (спека 109): цвет точки листа из
// пробного кадра и главные цвета фото листа. Альбом методы не меняют: Host.Changed, событий и rev
// нет. Вынесен из internal/app; методы App для JS (PickColorAt, SpreadPhotoColors) — тонкие
// обёртки с guardErr, поэтому набор методов и биндинги прежние.
package colorpick

import (
	"context"
	"errors"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/library"
)

// Host — что окну цвета нужно от App; поля задаёт App (Rare можно не задавать).
type Host struct {
	Editor    func() *album.Editor
	Photo     func(id string) (p library.Photo, ok, gone bool) // App.mediaPhoto
	Cache     func() *library.Cache                            // кэш миниатюр; nil до запуска
	Ctx       func() context.Context                           // отмена при закрытии программы
	Reachable func(path string) bool                           // отвечает ли том пути
	Rare      func(n int64, format string, args ...any)        // редкая строка журнала по счётчику
}

// Picker — состояние окна цвета: кэш точек фото и кэш кадров пипетки.
type Picker struct {
	h      Host
	colors colorState
	pick   pickCache
}

// New — окно цвета поверх h. Пустое обязательное поле Host — паника сразу и по имени, а не
// nil-вызов посреди щелчка пользователя.
func New(h Host) *Picker {
	switch {
	case h.Editor == nil:
		panic("colorpick.New: не задано поле Host.Editor")
	case h.Photo == nil:
		panic("colorpick.New: не задано поле Host.Photo")
	case h.Cache == nil:
		panic("colorpick.New: не задано поле Host.Cache")
	case h.Ctx == nil:
		panic("colorpick.New: не задано поле Host.Ctx")
	case h.Reachable == nil:
		panic("colorpick.New: не задано поле Host.Reachable")
	}
	if h.Rare == nil {
		h.Rare = func(int64, string, ...any) {}
	}
	return &Picker{h: h}
}

// ErrNoSheet — листа нет (удалили, пока открыто окно).
var ErrNoSheet = errors.New("лист не найден")

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
