package osx

// Remote — том пути не локальный (150, design §4.4): тихая папка агента берётся только с локального
// тома. Проба под пределом ProbeTimeout и одна за раз: пока первая висит (мёртвый NFS), вторая
// сразу отвечает «сетевой», а не копит горутины. Ошибка, предел и незнакомый тип — «сетевой».

import (
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"time"
)

// remoteProbe — проба одного пути на этой ОС (remote_<ос>.go): remote = true — не локальный.
type remoteProbe struct {
	check   func(path string) (remote bool, err error)
	timeout time.Duration
	busy    atomic.Bool
}

var stdRemote = &remoteProbe{check: remoteOS, timeout: ProbeTimeout}

// Remote — путь на сетевом (нелокальном) томе. Относительный и пустой путь — тоже: тома у него нет.
func Remote(path string) bool { return stdRemote.remote(path) }

func (p *remoteProbe) remote(path string) bool {
	if path == "" || !filepath.IsAbs(path) && !strings.HasPrefix(path, `\\`) {
		return true
	}
	if !p.busy.CompareAndSwap(false, true) {
		return true // проба уже идёт и, судя по всему, висит
	}
	type result struct {
		remote bool
		err    error
	}
	ch := make(chan result, 1)
	go func() {
		r := result{remote: true}
		defer func() {
			_ = recover() // чужой том не должен валить программу
			p.busy.Store(false)
			ch <- r
		}()
		r.remote, r.err = p.check(path)
	}()
	t := time.NewTimer(p.timeout)
	defer t.Stop()
	select {
	case r := <-ch:
		return r.remote || r.err != nil
	case <-t.C:
		return true // проба доживает в своей горутине и сама снимет busy
	}
}

// Типы macOS: локальны, если том отмечен MNT_LOCAL и тип из списка (design §4.4).
var darwinLocalFS = []string{"apfs", "hfs", "msdos", "exfat"}

const mntLocal = 0x1000 // MNT_LOCAL в f_flags statfs

// darwinRemote — решение по flags и f_fstypename; чистая функция, чтобы тест шёл на любой ОС.
func darwinRemote(flags uint32, fstype string) bool {
	return flags&mntLocal == 0 || !slices.Contains(darwinLocalFS, fstype)
}
