//go:build !windows

package osx

import "testing"

// Вне Windows проводника нет — атрибутов командной строки быть не должно.
func TestExplorerSelectAttr(t *testing.T) {
	if attr := explorerSelectAttr(`/a, b/c.jpg`); attr != nil {
		t.Fatal("вне Windows атрибутов быть не должно")
	}
}
