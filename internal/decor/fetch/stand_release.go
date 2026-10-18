//go:build !dev

package fetch

// standLibrary — в выпуске стенда библиотеки нет (см. stand_dev.go): SA_LIBRARY_URL не читается.
const standLibrary = false
