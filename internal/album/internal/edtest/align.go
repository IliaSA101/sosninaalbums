package edtest

import "github.com/IliaSA101/sosninaalbums/internal/geom"

// AlignOps и AlignTo — допустимые операция и опора Editor.AlignItems для случайных
// сценариев. Копия списков ядра (ops_layers.go): ядро держит их закрытыми, а тесты
// других пакетов (grid) до них иначе не дотянутся. Устаревший список не ломает тест,
// а лишь реже меняет проект (неизвестная операция — холостая правка).
var (
	AlignTo  = []string{geom.AlignSel, geom.AlignPage, geom.AlignSpread}
	AlignOps = []string{"left", "hcenter", "right", "top", "vcenter", "bottom", "distH", "distV"}
)
