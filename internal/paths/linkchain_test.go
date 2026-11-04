package paths

import (
	"errors"
	"path/filepath"
	"runtime"
	"testing"
)

// LinkChainLocal: цепочка по сырым записям — не ссылка (ошибка чтения) — конец, true; звено на сетевой путь в любом
// месте цепочки — false (ещё до обращения к диску); цикл длиннее MaxLinkHops — false.
func TestLinkChainLocal(t *testing.T) {
	root := filepath.Join(string(filepath.Separator), "tmp")
	if runtime.GOOS == "windows" {
		root = `C:\tmp`
	}
	unc := "//nosuchhost/x.jpg"
	if runtime.GOOS == "windows" {
		unc = `\\nosuchhost\share\x.jpg`
	}
	a, b := filepath.Join(root, "a"), filepath.Join(root, "b")
	links := map[string]string{}
	rl := func(p string) (string, error) {
		if v, ok := links[p]; ok {
			return v, nil
		}
		return "", errors.New("не ссылка")
	}
	if !LinkChainLocal(a, rl) {
		t.Error("не ссылка: конец цепочки должен быть локальным")
	}
	links[a] = b
	if !LinkChainLocal(a, rl) {
		t.Error("ссылка на локальный не-ссылку")
	}
	links[b] = unc
	if LinkChainLocal(a, rl) {
		t.Error("второе звено цепочки на сетевой путь пропущено")
	}
	links[b] = a // цикл
	if LinkChainLocal(a, rl) {
		t.Error("цикл ссылок длиннее MaxLinkHops пропущен")
	}
}
