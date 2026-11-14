package testkit

import "testing"

// ExportMarker — текст COM-метки выгрузки (imgfile.ExportMarker): testkit не импортирует пакеты проекта.
const ExportMarker = "Sosnina Albums"

// ExportJPEG пишет JPEG w×h, каким его оставляет выгрузка программы: после SOI те же JFIF-сегмент
// и COM-метка, что у export.saveJPEG (библиотека не может импортировать export — цикл).
// Совпадение с настоящим файлом выгрузки сверяет TestMarkerInHeader.
func ExportJPEG(t testing.TB, dir, name string, w, h int) string {
	t.Helper()
	jfif := Segment(0xE0, []byte{'J', 'F', 'I', 'F', 0, 1, 1, 1, 0, 0, 0, 0, 0, 0})
	return JPEGSegments(t, dir, name, w, h, jfif, Segment(0xFE, []byte(ExportMarker)))
}
