//go:build !windows

package osx

// sysBackend — вне Windows реестра нет: связь .salbum ставит пакет (.deb) или Info.plist, а
// Supported() == false и остальное — no-op (спека 112, §3).
func sysBackend() regBackend { return nil }
