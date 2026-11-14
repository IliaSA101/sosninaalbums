//go:build dev || qa

package fetch

// standLibrary — SA_LIBRARY_URL читается только в сборке для разработки (wails dev — тег dev)
// и в exe набора QA (тег qa), как SA_STAND (osx/stand_dev.go).
const standLibrary = true
