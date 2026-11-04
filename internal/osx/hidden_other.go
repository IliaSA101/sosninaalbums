//go:build !windows

package osx

// Hidden — атрибута «скрытый» вне Windows нет: точку в начале имени проверяет правило папки.
func Hidden(string) bool { return false }
