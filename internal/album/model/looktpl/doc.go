// Package looktpl — шаблон готового разворота (067) и макет альбома (131, 164): данные файла,
// разбор, зажим и пределы. Отдельный подпакет model: группа замкнута (в неё ходит из model
// только константа MaxLookSteps и однострочный finite), а model разрослась; пакет зависит от model,
// model о нём не знает. Типы биндингов Wails здесь не живут: SaveLookOpts и LookPlan остаются
// в model (lookplan.go), MaxLookSteps — в lookref.go (его держит и Spread.LookStep).
//
// Карта: spreadtpl.go (SpreadTpl, якоря, зажим, ParseSpreadTpl, ключ TplKey), spreadtpl_borders.go
// (рамочки мест шаблона), look.go (AlbumTpl, шаги LookStep, ParseLook, ClampLook,
// ключ LookKey, пределы файла), look_vigstyle.go (оформление шага-виньетки 164), lookfit.go (подгонка
// шага под формат), looksave.go (MergeLookSteps — серия макета из шагов разворотов при «Сохранить альбом как макет»).
package looktpl
