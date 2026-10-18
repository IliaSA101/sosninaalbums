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
// изделия), places.go (Place, сетка, мост к internal/geom), backdrop.go (Backdrop), decor.go
// (узор фона Pattern, его слоты и зажим, ключ своего из библиотеки 048), spreadtpl.go (шаблон готового
// разворота 067: SpreadTpl, якоря, зажим, разбор, ключ),
// text.go (кадр, рамки текстов, ширина «по тексту», кегль подписи, PhotoRef.Copy; имя на фото 030, 043: PhotoRef.Name, PersonFromFile и вольное PersonFromFileLoose, NameView), looksname.go (LooksLikeName — имя из названия файла при броске 060 и стоп-список), seatname.go (табличка места 071: NameAt, PlaceOf, жильцы мест Occupant, ClampSeat), order.go (стек
// слоёв), marks.go (типы отметок), tone.go (ч/б и тон фото 068: Tone, ClampTone, строка CSS), viewtypes.go (View, выбор сеток, PathPair, план окна «Имена» 043),
// templates.go (шаблоны сеток), limits.go (пределы и ошибки; параллельный счёт раскладок), clamp.go (зажимы сетки и оформления, CheckTotals), fields.go (поля подписей 030:
// Vars — поля шапки листа и VarsOf, Person, SheetCaps, их зажим, Expand), cover.go (обложка 032: Cover, CoverSpec,
// пресеты, зажим; Фотосфера 047: форматы, пресеты страницы, шаблон обложки), sheet.go (геометрия листа Geo, Sheet, Sheets, CoverGeo, AreaOf, проект-лист 031
// NewSheet и FoldPages; Settings.PageW — только здесь, ClampSettings и Params; виньетка 031:
// Vignette, роли мест, зажим и FixVignettes).
package model
