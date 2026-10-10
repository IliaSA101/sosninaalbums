// Package salbum — формат файла альбома .salbum (023): чтение с миграцией и починкой,
// запись, свои отметки в файле, перепривязка путей фото и папок.
//
// Импортирует только model (и нижние пакеты): Load держит инварианты модели через её
// функции. Правки живого альбома — только через Editor ядра: Relink, RelinkDrive и Refs
// (глубокая копия сведений о фото) здесь — тела его обёрток (ops_salbum.go), напрямую из
// app их не звать.
//
// Карта: load.go (Load, Decode, Encode, Save, пути и CheckRel; Check — пути проекта из памяти
// для NewEditor/Restore), fix.go (migrate и
// починки при чтении: dedupe, Rekey, dropNilPlaces, unionGrid), write.go (атомарная
// запись, копия файла старой версии), marks.go (секция отметок), relink.go (перепривязка).
// Голдены старых версий — testdata/.
package salbum
