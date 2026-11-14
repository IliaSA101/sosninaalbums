//go:build darwin

package mcpbridge

import "path/filepath"

// spawnProgram — внутри <имя>.app/Contents/MacOS/ запускает пакет через /usr/bin/open (окно
// встаёт как при двойном щелчке и не привязано к клиенту), иначе — exe напрямую.
func spawnProgram(exe string, env []string) error {
	if bundle := appBundle(exe); bundle != "" {
		return runDetached("/usr/bin/open", []string{bundle}, "/", env)
	}
	return runDetached(exe, nil, filepath.Dir(exe), env)
}
