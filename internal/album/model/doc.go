// Package model — данные альбома и их инварианты (023): Project, Spread, места, тексты,
// фоны, отметки, типы View для интерфейса, пределы и зажим, данные шаблонов сеток.
//
// Правят модель только через Editor (ядро internal/album): здесь функции и методы, которые
// зовут ядро, области и Load (salbum). Экспортные имена, нужные только им (Normalize, KeepLayouts,
// SyncOrder…), руками вне конвейера Editor не звать. Поля from и loaded — без экспорта,
// через доступы From/Loaded: экспортное поле попало бы в JSON снимков отмены.
//
// Типы биндингов Wails живут здесь: namespace фронта — имя пакета (model.View).
//
// Карта: project.go (Project, Spread, Settings, PhotoRef, Normalize, KeepLayouts, тип
// изделия), places.go (Place, сетка, мост к internal/geom), backdrop.go (Backdrop),
// text.go (кадр, рамки текстов, ширина «по тексту», кегль подписи, PhotoRef.Copy), order.go (стек
// слоёв), marks.go (типы отметок), viewtypes.go (View, выбор сеток, PathPair),
// templates.go (шаблоны сеток), warm.go (параллельный счёт раскладок), limits.go (пределы
// и ошибки), clamp.go (зажимы сетки и оформления, CheckTotals).
package model
