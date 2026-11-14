//go:build !windows

package update

import "errors"

// errNoStart — запуск установщика есть только в Windows (на Mac и Linux обновление — страница).
var errNoStart = errors.New("установщик запускается только в Windows")

// Start — на этой ОС не запускает ничего.
func Start(_, _ string) error { return errNoStart }
