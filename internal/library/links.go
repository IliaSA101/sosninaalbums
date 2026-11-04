package library

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sync/atomic"

	"github.com/IliaSA101/sosninaalbums/internal/imgfile"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// Ссылки на файлы и папки (0.22-sec-4): одна проверка для обхода папок, отдельных фото, броска в
// окно и чужих файлов (PlainFile). До проверки цели на диск по ней не ходим: ссылка на UNC иначе
// открыла бы сетевую шару (запрос SMB с NTLM-хешем пользователя).

// ErrLinkNotLocal — какое-то звено цепочки ссылок ведёт за пределы локального диска (или цепочка
// длиннее paths.MaxLinkHops).
var ErrLinkNotLocal = errors.New("ссылка ведёт за пределы локального диска")

// readLinkHook — подмена чтения сырой цели ссылки (SetReadLink); nil — os.Readlink.
var readLinkHook atomic.Pointer[func(p string) (string, error)]

// readLink — сырая цель ссылки p.
func readLink(p string) (string, error) {
	if f := readLinkHook.Load(); f != nil {
		return (*f)(p)
	}
	return os.Readlink(p)
}

// SetReadLink — только для тестов: rl читает цель ссылки вместо os.Readlink (на Windows без прав
// ссылку не создать); restore возвращает прежнее. Атомарно: фоновое сканирование читает его же.
func SetReadLink(rl func(p string) (string, error)) (restore func()) {
	old := readLinkHook.Swap(&rl)
	return func() { readLinkHook.Store(old) }
}

// LinkTarget — цель ссылки p и сведения о ней: каждое звено цепочки (до paths.MaxLinkHops) проверяется
// по сырой записи (linkLocal), затем EvalSymlinks и CheckLocalPath итога, и только потом Stat.
// Не ссылка — сам p (другие точки повторной обработки Readlink не читает — решает Stat).
// Ошибка — ErrLinkNotLocal или ошибка диска (битая ссылка).
func LinkTarget(p string) (string, fs.FileInfo, error) {
	if !linkLocal(p) {
		return "", nil, ErrLinkNotLocal
	}
	target, err := filepath.EvalSymlinks(p)
	if err != nil {
		return "", nil, err
	}
	if paths.CheckLocalPath(target) != nil {
		return "", nil, ErrLinkNotLocal
	}
	st, err := os.Stat(target) //nolint:gosec // G703: target прошёл linkLocal по цепочке и CheckLocalPath выше — локальный путь после ссылок
	if err != nil {
		return "", nil, err
	}
	return target, st, nil
}

// linkLocal — у ссылки p каждая цель цепочки локальна (paths.LinkChainLocal, как при обходе папок).
func linkLocal(p string) bool { return paths.LinkChainLocal(p, readLink) }

// statLink — сведения о файле, на который указывает ссылка p (LinkTarget); ok = false для папок,
// битых ссылок и ссылок, хоть одно звено которых нелокально (UNC, `\\?\`, устройство).
func statLink(p string) (fs.FileInfo, bool) {
	_, st, err := LinkTarget(p)
	if err != nil || st.IsDir() {
		return nil, false
	}
	return st, true
}

// localTarget — цель ссылки лежит на локальном диске (paths.LinkTargetLocal).
func localTarget(dir, target string) bool { return paths.LinkTargetLocal(dir, target) }

// StatLocal — os.Stat без хода по нелокальной ссылке (101, техдолг 0.22): Lstat; ссылка или
// точка повторной обработки — LinkTarget (каждое звено локально, до Stat цели) и сведения цели;
// иначе — сведения самого файла. Путь сначала проходит paths.CheckLocalPath (пустой, относительный, UNC, устройство —
// отказ с ошибкой paths, её узнаёт paths.NoPath). Ошибка — ErrLinkNotLocal или ошибка диска.
func StatLocal(p string) (fs.FileInfo, error) {
	if err := paths.CheckLocalPath(p); err != nil {
		return nil, err
	}
	st, err := os.Lstat(p) //nolint:gosec // G703: путь прошёл CheckLocalPath выше; Lstat не идёт по ссылке, цель ссылки — LinkTarget
	if err != nil || st.Mode()&(fs.ModeSymlink|fs.ModeIrregular) == 0 {
		return st, err
	}
	_, st, err = LinkTarget(p)
	return st, err
}

// openLocalHook — только для тестов: зовётся между проверкой и открытием в OpenLocal (подмена файла).
var openLocalHook func(p string)

// ErrFileChanged — файл подменили между проверкой и открытием.
var ErrFileChanged = errors.New("файл сменился во время открытия")

// OpenLocal — открыть для чтения обычный файл по пути от пользователя (аудит 123 №8): та же
// проверка, что StatLocal, затем открывается проверенная цель (у ссылки — цель LinkTarget, а не
// сама ссылка, которую успели бы перенаправить на \\host), без ожидания на канале и на Windows без хода по
// ссылке в последнем звене (imgfile.OpenNoReparse), и os.SameFile сверяет открытое с проверенным. Сведения — открытого файла.
func OpenLocal(p string) (*os.File, fs.FileInfo, error) {
	if err := paths.CheckLocalPath(p); err != nil {
		return nil, nil, err
	}
	st, err := os.Lstat(p) //nolint:gosec // G703: путь прошёл CheckLocalPath выше; Lstat не идёт по ссылке, цель ссылки — LinkTarget
	if err != nil {
		return nil, nil, err
	}
	target := p
	if st.Mode()&(fs.ModeSymlink|fs.ModeIrregular) != 0 {
		if target, st, err = LinkTarget(p); err != nil {
			return nil, nil, err
		}
	}
	// На Windows облачная заглушка (OneDrive «только в сети») для Go — ModeIrregular: окончательно решает дескриптор
	// (imgfile.OpenNoReparse принимает только заглушку, точку соединения и прочее отклоняет).
	if !imgfile.MaybeRegular(st.Mode()) {
		return nil, nil, errors.New("не обычный файл")
	}
	// На Windows Lstat/Stat обычного файла номер файла не читают: SameFile дочитал бы его лениво
	// по пути уже после открытия и сравнил подменённый файл сам с собой. Читаем номер сейчас.
	_ = os.SameFile(st, st)
	if openLocalHook != nil {
		openLocalHook(target)
	}
	f, err := imgfile.OpenNoReparse(target)
	if err != nil {
		return nil, nil, err
	}
	got, err := f.Stat()
	if err != nil || !os.SameFile(st, got) {
		_ = f.Close()
		return nil, nil, ErrFileChanged
	}
	return f, got, nil
}

// RegularOrCloud — запись p со сведениями st — обычный файл или облачная заглушка OneDrive (imgfile.RegularOrCloud);
// для пакетов, которым imgfile недоступен по depguard (prefs).
func RegularOrCloud(p string, st fs.FileInfo) bool { return imgfile.RegularOrCloud(p, st) }
