//go:build dev || qa

package update

// standEnabled — SA_UPDATE_URL читается только в сборке для разработки (тег dev) и в exe набора QA (тег qa), как SA_LIBRARY_URL.
const standEnabled = true
