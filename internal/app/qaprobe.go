//go:build !qa

package app

import "context"

// qaProbe — зонд win-qa есть только в exe набора QA (qaprobe_qa.go); в выпуске и в dev ничего не делает.
func (a *App) qaProbe(context.Context) {}
