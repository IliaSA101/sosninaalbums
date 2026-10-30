package app

// Подсчёт папки перед тихим добавлением (150, design §4.5): порциями, с флагом остановки и общим
// пределом времени — папка в миллион записей или зависший диск не держат вызов агента.

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// Пределы тихой папки.
const (
	quietMaxPhotos  = 5000
	quietMaxEntries = 20000
	quietMaxDirs    = 500
	quietBatch      = 256
)

var (
	quietTimeout = 1500 * time.Millisecond
	quietPause   = func() {}  // перед каждой порцией (тест: зависший диск)
	quietCloud   = entryCloud // запись — облачная заглушка (по ОС)
)

// quietTally — итог подсчёта.
type quietTally struct {
	photos, entries, dirs int
	cloud, big, remote    bool // remote: внутри смонтирован сетевой том (другой st_dev)
}

func (t quietTally) over() bool {
	return t.big || t.remote || t.photos > quietMaxPhotos || t.entries > quietMaxEntries || t.dirs > quietMaxDirs
}

// countQuiet считает папку root; не уложились во время или не прочитали — big.
func countQuiet(root string) quietTally {
	if paths.CheckLocalPath(root) != nil {
		return quietTally{big: true}
	}
	var stop atomic.Bool
	until := time.Now().Add(quietTimeout)
	t, ok := within(quietTimeout, func() quietTally { return walkQuiet(root, &stop, until) })
	stop.Store(true) // зависшая порция доживает в горутине и выйдет, не читая дальше
	if !ok {
		return quietTally{big: true}
	}
	return t
}

// mountedRemote — подпапка sub на другом томе, чем корень (st_dev), и том сетевой.
func mountedRemote(sub string, rootDev uint64, rootOK bool) bool {
	d, ok := placeDev(sub)
	return rootOK && ok && d != rootDev && quietRemote(sub)
}

// walkQuiet — обход в ширину без ссылок; останавливается на первом превышении, облачной заглушке,
// stop и сроке.
func walkQuiet(root string, stop *atomic.Bool, until time.Time) (t quietTally) {
	queue := []string{root}
	rootDev, devOK := placeDev(root)
	for len(queue) > 0 {
		dir := queue[0]
		queue = queue[1:]
		t.dirs++
		f, err := os.Open(dir)
		if err != nil {
			t.big = true
			return t
		}
		failed := false
		for done := false; !done; {
			quietPause()
			ents, err := f.ReadDir(quietBatch)
			for _, e := range ents {
				t.entries++
				switch {
				case e.IsDir():
					sub := filepath.Join(dir, e.Name())
					// Том, смонтированный внутри места (sshfs, NFS, cifs), проба корня не видит (аудит В3):
					// подпапка с другим st_dev — на сетевом томе читать её без вопроса нельзя.
					t.remote = t.remote || mountedRemote(sub, rootDev, devOK)
					queue = append(queue, sub)
				case library.IsImage(e.Name()):
					t.photos++
					t.cloud = t.cloud || quietCloud(e)
				}
			}
			if t.over() || t.cloud || t.remote {
				_ = f.Close()
				return t
			}
			if stop.Load() || time.Now().After(until) {
				_ = f.Close()
				t.big = true
				return t
			}
			if err != nil {
				done, failed = true, !errors.Is(err, io.EOF)
			}
		}
		_ = f.Close()
		if failed {
			t.big = true
			return t
		}
	}
	return t
}
