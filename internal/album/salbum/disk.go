package salbum

import (
	"io/fs"
	"os"
	"sync/atomic"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/imgfile"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// Disk — всё, к чему разбор файла обращается на диске и в глобальных искателях (160, design §3). Перечень
// полный: другого обращения у Decode нет. Методы неэкспортируемые — реализовать Disk снаружи нельзя,
// получить его можно только отсюда: OSDisk, Bounded, Spy.
type Disk interface {
	reachable(path string) bool                  // том отвечает (мёртвый сетевой — «файла нет»)
	statLocal(path string) (fs.FileInfo, error)  // Stat с локальной целью ссылки
	readInfo(path string) (library.Photo, error) // заголовок фото
	linked(path string) bool                     // папка — ссылка или junction (FoldSources)
	lib(key string) (string, bool)               // путь своего элемента библиотеки
}

// Подмены тестов (глобальные, как reachable): страж «сухой разбор не трогает диск» ставит на них t.Fatal.
var (
	statLocalFn = library.StatLocal
	readInfoFn  = library.ReadInfoLocal // одно открытие проверенной цели, без второго обращения по пути (аудит Н3)
	linkedFn    = paths.Linked
)

// osDisk — настоящий диск и искатель библиотеки (libFinder).
type osDisk struct{}

func (osDisk) reachable(path string) bool                  { return reachable(path) }
func (osDisk) statLocal(path string) (fs.FileInfo, error)  { return statLocalFn(path) }
func (osDisk) readInfo(path string) (library.Photo, error) { return readInfoFn(path) }
func (osDisk) linked(path string) bool                     { return linkedFn(path) }
func (osDisk) lib(key string) (string, bool) {
	fp := libFinder.Load()
	if fp == nil {
		return "", false
	}
	return (*fp)(key)
}

// OSDisk — настоящий диск (то, что берёт Decode).
func OSDisk() Disk { return osDisk{} }

// noDisk — сухой разбор: файлов нет, ссылок нет, библиотека ничего не находит; размеры не
// перечитываются, запасной путь остаётся в Alt. Глобальных переменных не трогает (вызовы параллельны).
type noDisk struct{}

func (noDisk) reachable(string) bool                  { return false }
func (noDisk) statLocal(string) (fs.FileInfo, error)  { return nil, fs.ErrNotExist }
func (noDisk) readInfo(string) (library.Photo, error) { return library.Photo{}, fs.ErrNotExist }
func (noDisk) linked(string) bool                     { return false }
func (noDisk) lib(string) (string, bool)              { return "", false }

// Budget — срок и число обращений полного разбора (160, аудит В3): по исчерпании Bounded отвечает «нет», а
// Exceeded() становится истиной — вызывающий отвечает big. nil — без предела.
type Budget struct {
	until time.Time
	max   int64
	ops   atomic.Int64
	over  atomic.Bool
}

// NewBudget — срок until (нулевой — без срока) и не больше maxOps обращений (0 — без числа).
func NewBudget(until time.Time, maxOps int) *Budget { return &Budget{until: until, max: int64(maxOps)} }

// spend считает обращение; false — бюджет исчерпан.
func (b *Budget) spend() bool {
	if b == nil {
		return true
	}
	if b.over.Load() || (!b.until.IsZero() && !time.Now().Before(b.until)) || (b.max > 0 && b.ops.Add(1) > b.max) {
		b.over.Store(true)
		return false
	}
	return true
}

// Exceeded — бюджет был исчерпан во время разбора: часть путей «не нашлась» не по правде.
func (b *Budget) Exceeded() bool { return b != nil && b.over.Load() }

// lstatFn — Lstat для Bounded; подмена тестов.
var lstatFn = os.Lstat

// plainFile — по пути не ссылка и не reparse point: тихое открытие ссылок-файлов не знает (аудит В1).
func plainFile(path string) bool {
	st, err := lstatFn(path)
	return err == nil && st.Mode()&(fs.ModeSymlink|fs.ModeIrregular) == 0
}

// bounded — настоящий диск, но только по путям, которые разрешил allowed; вне них — «файла нет»,
// а диск не трогается. Ссылка вместо файла или папки — тоже «нет». Папка, которую нельзя проверить,
// считается ссылкой (не сворачивается в корень).
type bounded struct {
	allowed func(path string) bool
	b       *Budget
}

// Bounded — настоящий диск по путям, где allowed(path) истинно (160: проверенные корни и родители), в пределах
// бюджета b (nil — без предела).
func Bounded(allowed func(path string) bool, b *Budget) Disk { return bounded{allowed, b} }

func (d bounded) ok(path string) bool { return d.allowed(path) && d.b.spend() }

func (d bounded) reachable(path string) bool { return d.ok(path) && reachable(path) }
func (d bounded) statLocal(path string) (fs.FileInfo, error) {
	if !d.ok(path) || !plainFile(path) {
		return nil, fs.ErrNotExist
	}
	return statLocalFn(path)
}
func (d bounded) readInfo(path string) (library.Photo, error) {
	if !d.ok(path) || !plainFile(path) {
		return library.Photo{}, fs.ErrNotExist
	}
	return readInfoFn(path)
}
func (d bounded) linked(path string) bool { return !d.ok(path) || linkedFn(path) }
func (d bounded) lib(key string) (string, bool) {
	path, ok := osDisk{}.lib(key)
	if !ok || !d.ok(path) {
		return "", false
	}
	return path, true
}

// spy — Disk с сообщением о каждом обращении: op и путь. Для тестов стражей (счётчик касаний диска).
type spy struct {
	d  Disk
	fn func(op, path string)
}

// Spy оборачивает d: fn(op, path) зовётся перед каждым обращением к диску.
func Spy(d Disk, fn func(op, path string)) Disk { return spy{d, fn} }

func (s spy) reachable(path string) bool {
	s.fn("reachable", path)
	return s.d.reachable(path)
}
func (s spy) statLocal(path string) (fs.FileInfo, error) {
	s.fn("stat", path)
	return s.d.statLocal(path)
}
func (s spy) readInfo(path string) (library.Photo, error) {
	s.fn("info", path)
	return s.d.readInfo(path)
}
func (s spy) linked(path string) bool {
	s.fn("linked", path)
	return s.d.linked(path)
}
func (s spy) lib(key string) (string, bool) {
	s.fn("lib", key)
	return s.d.lib(key)
}

// fileSizeOn — размер обычного файла path на диске d; ссылка — только с локальной целью.
func fileSizeOn(d Disk, path string) (int64, bool) {
	if !d.reachable(path) {
		return 0, false
	}
	st, err := d.statLocal(path)
	if err != nil || !imgfile.RegularOrCloud(path, st) { // заглушка OneDrive — файл на месте
		return 0, false
	}
	return st.Size(), true
}

// isDirOn — path — папка на диске d (ссылка — как fileSizeOn).
func isDirOn(d Disk, path string) bool {
	if !d.reachable(path) {
		return false
	}
	st, err := d.statLocal(path)
	return err == nil && st.IsDir()
}

// isFileOn — обычный файл на отвечающем томе.
func isFileOn(d Disk, path string) bool {
	_, ok := fileSizeOn(d, path)
	return ok
}

// probeDimsOn — размеры из заголовка фото с учётом поворота EXIF.
func probeDimsOn(d Disk, path string) (w, h int, ok bool) {
	if path == "" || paths.CheckLocalPath(path) != nil || !d.reachable(path) {
		return 0, 0, false
	}
	// Открывать только то, что прошло StatLocal (обычный файл с локальной целью): иначе ссылка на UNC отдала бы
	// хеш NTLM чужому SMB (аудит В2).
	if st, err := d.statLocal(path); err != nil || !imgfile.RegularOrCloud(path, st) {
		return 0, 0, false
	}
	ph, err := d.readInfo(path)
	if err != nil {
		return 0, 0, false
	}
	return ph.W, ph.H, true
}
