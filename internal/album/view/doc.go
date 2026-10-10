// Package view — сборка View для интерфейса из проекта (023): раскладка разворотов «Авто»
// (параллельно, только чтение), кадры, подписи, тексты, фоны, сетка, слои z, безопасная зона.
//
// BuildView принимает *model.Project и State (поля Editor, которых нет в проекте): замок и
// выбор момента — у Editor.view (ops_view.go ядра). Типы вида (View, SpreadView…) живут в model.
//
// Карта: view.go (State, BuildView, spreadView, safeZone), lay.go (lay, Laid, layAll, воркеры),
// parts.go (сетка, подписи, z, кадры, тексты, фоны, цвет страниц).
package view
