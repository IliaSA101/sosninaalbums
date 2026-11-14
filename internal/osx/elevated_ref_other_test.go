//go:build !windows

package osx

import (
	"os"
	"testing"
)

// refElevated — на Unix повышенный процесс — это действующий uid 0 по определению ОС;
// независимого признака нет, тест сверяет Elevated с этим определением.
func refElevated(*testing.T) bool { return os.Geteuid() == 0 }
