//go:build !dev && !qa

package update

// standEnabled — в выпуске стенда обновления нет (см. stand_dev.go): SA_UPDATE_URL не читается.
const standEnabled = false
