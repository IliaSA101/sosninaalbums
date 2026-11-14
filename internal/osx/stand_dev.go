//go:build dev || qa

package osx

// DevBuild — сборка для разработки (wails dev собирает с тегом dev) или exe набора QA (тег qa:
// `wails build -tags dev` не собирается — Wails сам добавляет production, и его app_dev.go
// с app_production.go дают двойное объявление; scripts/win/qa-kit.sh). Связь .salbum (112) в ней
// не пишется: dev-exe лежит в папке проекта и увёл бы двойной щелчок у выпуска.
const DevBuild = true

// standEnabled — стенды QA включены только в сборке для разработки. В выпуске переменная
// SA_STAND игнорируется: забытая в системе, она иначе дала бы пустые настройки, а неверная —
// тихий незапуск (stderr у GUI-exe нет).
const standEnabled = DevBuild
