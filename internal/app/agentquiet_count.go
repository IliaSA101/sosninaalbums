package app

// Подсчёт папки перед тихим добавлением (150, design §4.5): порциями, с флагом остановки и общим
// пределом времени — папка в миллион записей или зависший диск не держат вызов агента.

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/app/appkit"
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
	link                  bool // внутри ссылка (файл или папка): сканер идёт по ссылкам с локальной целью, тихо нельзя (аудит В1)
}

func (t quietTally) over() bool { return t.overCap(true) }

// overCap — over; capPhotos = false — предел 5000 фото не действует (открытие альбома, 0.36-160-5).
func (t quietTally) overCap(capPhotos bool) bool {
	return t.big || t.remote || (capPhotos && t.photos > quietMaxPhotos) || t.entries > quietMaxEntries || t.dirs > quietMaxDirs
}

// countQuiet считает папку root; не уложились во время или не прочитали — big.
func countQuiet(root string) quietTally { return countQuietCap(root, true, quietTimeout) }

// countQuietCap — countQuiet; capPhotos = false — без предела числа фото (открытие альбома); timeout — срок обхода.
func countQuietCap(root string, capPhotos bool, timeout time.Duration) quietTally {
	if paths.CheckLocalPath(root) != nil {
		return quietTally{big: true}
	}
	var stop atomic.Bool
	until := time.Now().Add(timeout)
	// Подмены читаем до горутины: зависшая порция переживает предел, а тест восстановит их раньше.
	h := quietHooks{dev: placeDev, remote: quietRemote, pause: quietPause, cloud: quietCloud, capPhotos: capPhotos}
	t, ok := appkit.Within(timeout, func() quietTally { return walkQuiet(root, &stop, until, h) })
	stop.Store(true) // зависшая порция доживает в горутине и выйдет, не читая дальше
	if !ok {
		return quietTally{big: true}
	}
	return t
}

// quietHooks — снимок подмен для обхода: читается в вызывающей горутине и передаётся параметром.
type quietHooks struct {
	dev    func(string) (uint64, bool)
	remote func(string) bool
	pause  func()
	cloud  func(fs.DirEntry) bool
	// capPhotos — действует ли предел числа фото (папка агента — да, открытие альбома — нет).
	capPhotos bool
}

// mountedRemote — подпапка sub на другом томе, чем корень (st_dev), и том сетевой.
func mountedRemote(sub string, rootDev uint64, rootOK bool, h quietHooks) bool {
	d, ok := h.dev(sub)
	return rootOK && ok && d != rootDev && h.remote(sub)
}

// walkQuiet — обход в ширину без ссылок; останавливается на первом превышении, облачной заглушке,
// stop и сроке.
func walkQuiet(root string, stop *atomic.Bool, until time.Time, h quietHooks) (t quietTally) {
	queue := []string{root}
	rootDev, devOK := h.dev(root)
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
			h.pause()
			ents, err := f.ReadDir(quietBatch)
			for _, e := range ents {
				t.entries++
				switch {
				case e.IsDir():
					sub := filepath.Join(dir, e.Name())
					// Том, смонтированный внутри места (sshfs, NFS, cifs), проба корня не видит (аудит В3):
					// подпапка с другим st_dev — на сетевом томе читать её без вопроса нельзя.
					t.remote = t.remote || mountedRemote(sub, rootDev, devOK, h)
					queue = append(queue, sub)
				case e.Type()&fs.ModeSymlink != 0:
					t.link = true
				case library.IsImage(e.Name()):
					t.photos++
					t.cloud = t.cloud || h.cloud(e)
					// Заглушка облака — тоже reparse point (Windows): ссылкой считается то, что не заглушка.
					t.link = t.link || (!t.cloud && e.Type()&fs.ModeIrregular != 0)
				}
			}
			if t.overCap(h.capPhotos) || t.cloud || t.remote || t.link {
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
