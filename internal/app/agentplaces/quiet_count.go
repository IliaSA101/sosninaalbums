package agentplaces

// Подсчёт папки перед тихим добавлением (150, design §4.5): порциями, с флагом остановки и общим
// пределом времени — папка в миллион записей или зависший диск не держат вызов агента. Подмены среды
// (st_dev, сетевой том, пауза, облачная заглушка) приходят параметром QuietHooks: пакет не читает
// переменные App, а вызывающий снимает их в своей горутине (зависшая порция переживает предел).

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

// QuietTally — итог подсчёта.
type QuietTally struct {
	Photos, Entries, Dirs int
	Cloud, Big, Remote    bool // Remote: внутри смонтирован сетевой том (другой st_dev)
	Link                  bool // внутри ссылка (файл или папка): сканер идёт по ссылкам с локальной целью, тихо нельзя (аудит В1)
}

// Over — OverCap с пределом числа фото.
func (t QuietTally) Over() bool { return t.OverCap(true) }

// OverCap — Over; capPhotos = false — предел 5000 фото не действует (открытие альбома, 0.36-160-5).
func (t QuietTally) OverCap(capPhotos bool) bool {
	return t.Big || t.Remote || (capPhotos && t.Photos > quietMaxPhotos) || t.Entries > quietMaxEntries || t.Dirs > quietMaxDirs
}

// QuietHooks — снимок подмен для обхода: читается в вызывающей горутине и передаётся параметром.
// Все функции обязательны: nil вызовет панику в горутине обхода, `Within` её глотает, и подсчёт молча вернёт Big.
type QuietHooks struct {
	Dev    func(string) (uint64, bool)
	Remote func(string) bool
	Pause  func()
	Cloud  func(fs.DirEntry) bool
	// CapPhotos — действует ли предел числа фото (папка агента — да, открытие альбома — нет).
	CapPhotos bool
}

// CountQuiet считает папку root за timeout; не уложились во время или не прочитали — Big.
func CountQuiet(root string, timeout time.Duration, h QuietHooks) QuietTally {
	if paths.CheckLocalPath(root) != nil {
		return QuietTally{Big: true}
	}
	var stop atomic.Bool
	until := time.Now().Add(timeout)
	t, ok := appkit.Within(timeout, func() QuietTally { return walkQuiet(root, &stop, until, h) })
	stop.Store(true) // зависшая порция доживает в горутине и выйдет, не читая дальше
	if !ok {
		return QuietTally{Big: true}
	}
	return t
}

// mountedRemote — подпапка sub на другом томе, чем корень (st_dev), и том сетевой.
func mountedRemote(sub string, rootDev uint64, rootOK bool, h QuietHooks) bool {
	d, ok := h.Dev(sub)
	return rootOK && ok && d != rootDev && h.Remote(sub)
}

// walkQuiet — обход в ширину без ссылок; останавливается на первом превышении, облачной заглушке,
// stop и сроке.
func walkQuiet(root string, stop *atomic.Bool, until time.Time, h QuietHooks) (t QuietTally) {
	queue := []string{root}
	rootDev, devOK := h.Dev(root)
	for len(queue) > 0 {
		dir := queue[0]
		queue = queue[1:]
		t.Dirs++
		f, err := os.Open(dir)
		if err != nil {
			t.Big = true
			return t
		}
		failed := false
		for done := false; !done; {
			h.Pause()
			ents, err := f.ReadDir(quietBatch)
			for _, e := range ents {
				t.Entries++
				switch {
				case e.IsDir():
					sub := filepath.Join(dir, e.Name())
					// Том, смонтированный внутри места (sshfs, NFS, cifs), проба корня не видит (аудит В3):
					// подпапка с другим st_dev — на сетевом томе читать её без вопроса нельзя.
					t.Remote = t.Remote || mountedRemote(sub, rootDev, devOK, h)
					queue = append(queue, sub)
				case e.Type()&fs.ModeSymlink != 0:
					t.Link = true
				case library.IsImage(e.Name()):
					t.Photos++
					t.Cloud = t.Cloud || h.Cloud(e)
					// Заглушка облака — тоже reparse point (Windows): ссылкой считается то, что не заглушка.
					t.Link = t.Link || (!t.Cloud && e.Type()&fs.ModeIrregular != 0)
				}
			}
			if t.OverCap(h.CapPhotos) || t.Cloud || t.Remote || t.Link {
				_ = f.Close()
				return t
			}
			if stop.Load() || time.Now().After(until) {
				_ = f.Close()
				t.Big = true
				return t
			}
			if err != nil {
				done, failed = true, !errors.Is(err, io.EOF)
			}
		}
		_ = f.Close()
		if failed {
			t.Big = true
			return t
		}
	}
	return t
}
