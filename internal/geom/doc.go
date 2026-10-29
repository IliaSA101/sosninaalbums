// Package geom — чистая геометрия разворота в мм: рамка текста, повёрнутое место,
// кадр фото, перенос группы, выравнивание. Без Project и Editor: модельные типы
// приходят значениями через мост album/model/places.go. Зеркала — frontend/src/base/geom
// (textbox, rotbox, frame) и base/groupgeom (groupxf, grouprot, groupaxes), паритет держат векторы testdata/parity
// (`go test ./internal/geom -update` перезаписывает).
package geom
