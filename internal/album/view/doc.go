// Package view — сборка View для интерфейса из проекта (023): раскладка разворотов «Авто»
// (параллельно, только чтение), кадры, подписи, тексты, фоны, сетка, слои z, безопасная зона.
//
// BuildView принимает *model.Project и State (поля Editor, которых нет в проекте): замок и
// выбор момента — у Editor.view (ops_salbum.go ядра). Типы вида (View, SpreadView…) живут в model.
//
// Карта: view.go (State, BuildView, spreadView, safeZone), lay.go (lay, Laid, layAll, воркеры),
// parts.go (сетка, z, кадры, тексты, фоны, цвет страниц), sticker.go (стикеры 048), captions.go (подписи 030: текст
// своей подписи или шаблона листа, стиль листа, поля через model.Shown, ужимание, отступ,
// заглушки; State.Print — вид печати для экспорта без заглушек и «[Школа]»).
package view
