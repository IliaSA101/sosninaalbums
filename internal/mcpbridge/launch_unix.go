//go:build !windows && !darwin

package mcpbridge

import "path/filepath"

// spawnProgram — запуск своей программы без аргументов, рабочая папка — папка exe.
func spawnProgram(exe string, env []string) error {
	return runDetached(exe, nil, filepath.Dir(exe), env)
}
