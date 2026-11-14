//go:build !dev && !qa

package osx

// DevBuild — в выпуске false (см. stand_dev.go).
const DevBuild = false

// standEnabled — в выпуске стендов нет: SA_STAND не читается.
const standEnabled = DevBuild
