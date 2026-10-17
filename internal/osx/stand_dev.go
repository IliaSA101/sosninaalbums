//go:build dev

package osx

// standEnabled — стенды QA включены только в сборке для разработки (wails dev собирает с тегом
// dev). В выпуске переменная SA_STAND игнорируется: забытая в системе, она иначе дала бы
// пустые настройки, а неверная — тихий незапуск (stderr у GUI-exe нет).
const standEnabled = true
