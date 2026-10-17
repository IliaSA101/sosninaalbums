//go:build !dev

package osx

// standEnabled — в выпуске стендов нет (см. stand_dev.go): SA_STAND не читается.
const standEnabled = false
