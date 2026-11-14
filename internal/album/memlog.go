package album

import (
	"log"
	"runtime"
)

// LogMem пишет в журнал память процесса на этапе stage (089 G3): «Go» — всё, что взято у системы,
// «куча» — занято живыми и ещё не собранными объектами. Строку читает e2e-отчёт; пути в неё не
// попадают. Зовётся на редких этапах (чтение папок, прогрев, автосборка, экспорт): ReadMemStats
// на миг останавливает мир.
func LogMem(stage string) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	log.Printf("память: %s — Go %d МБ, куча %d МБ", stage, m.Sys>>20, m.HeapInuse>>20)
}
