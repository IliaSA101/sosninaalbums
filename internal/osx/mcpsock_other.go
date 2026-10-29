//go:build !windows

package osx

// roamingDir — папка настроек пользователя (Linux — XDG_CONFIG_HOME или ~/.config, Mac —
// ~/Library/Application Support).
func roamingDir() string { return ConfigDir() }
