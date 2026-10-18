//go:build dev

package fetch

// standLibrary — SA_LIBRARY_URL читается только в сборке для разработки (wails dev — тег dev),
// как SA_STAND (osx/stand_dev.go).
const standLibrary = true
