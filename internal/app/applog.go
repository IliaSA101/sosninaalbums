package app

import (
	"io"
	"log"
	"os"
	"strings"
	"sync"
)

// Лог в файл: запись, ротация, прореживание повторов.

const maxLogSize = 2 << 20

// Права на Linux и Mac (аудит 0.23-sec): в папке данных и log.txt — пути и имена из альбомов,
// чужим пользователям машины они ни к чему. Существующие файлы и папки не трогаем. На Windows
// биты прав не действуют.
const (
	dataDirMode os.FileMode = 0o700
	logFileMode os.FileMode = 0o600
)

// makeDataDir создаёт папку данных (и недостающих родителей) с правами dataDirMode.
func makeDataDir(dir string) error { return os.MkdirAll(dir, dataDirMode) }

// setupLog пишет лог в файл и, если есть консоль, в stderr; файл больше 2 МБ уходит в .old —
// при старте и по ходу сессии (rotatingLog).
func setupLog(path string) {
	rotateLog(path, maxLogSize)
	if rl, err := newRotatingLog(path, maxLogSize); err == nil {
		log.SetOutput(logWriter{file: rl, console: os.Stderr})
	}
}

// openLogFile открывает log.txt на дозапись с правами logFileMode.
func openLogFile(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, logFileMode)
}

// rotatingLog — файл лога, который сам уходит в .old, когда вырос больше limit: в долгой
// сессии rotateLog при старте не хватает. Мьютекс свой: log.Logger сериализует Write, но
// писателю это знать не обязано (ему же переоткрывать файл).
type rotatingLog struct {
	mu       sync.Mutex
	path     string
	limit    int64
	rotateAt int64 // размер, после которого пора ротировать (после неудачной ротации сдвигается)
	size     int64
	f        *os.File                       // nil — файл не открыт, пробуем снова на следующей записи
	open     func(string) (*os.File, error) // подмена в тестах
}

func newRotatingLog(path string, limit int64) (*rotatingLog, error) {
	l := &rotatingLog{path: path, limit: limit, rotateAt: limit, open: openLogFile}
	if err := l.reopen(); err != nil {
		return nil, err
	}
	return l, nil
}

// reopen открывает файл и берёт его размер как начальный счёт байт.
func (l *rotatingLog) reopen() error {
	f, err := l.open(l.path)
	if err != nil {
		return err
	}
	l.f, l.size = f, 0
	if st, err := f.Stat(); err == nil {
		l.size = st.Size()
	}
	return nil
}

// rotate закрывает файл до переименования (открытый файл на Windows не переименовать),
// ротирует и открывает заново. Ошибки не возвращает: текущая запись всё равно пойдёт дальше —
// в новый файл, а если открыть не вышло, в консоль (logWriter).
func (l *rotatingLog) rotate() {
	if l.f != nil {
		_ = l.f.Close()
		l.f = nil
	}
	rotateLog(l.path, l.limit)
	// Одна повторная попытка: чаще всего сбой открытия — антивирус, держащий файл на миг.
	err := l.reopen()
	if err != nil {
		err = l.reopen()
	}
	if err != nil {
		return
	}
	l.rotateAt = l.limit
	if l.size > l.limit {
		// Переименовать не вышло: не молотить файл на каждой строке, повторим через limit байт.
		l.rotateAt = l.size + l.limit
	}
}

func (l *rotatingLog) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.f == nil || l.size > l.rotateAt {
		l.rotate()
	}
	if l.f == nil {
		return 0, os.ErrClosed
	}
	n, err := l.f.Write(p)
	l.size += int64(n)
	return n, err
}

// logWriter пишет сначала в файл, потом в консоль, и ошибку консоли не возвращает.
// io.MultiWriter(os.Stderr, f) не годится: у собранного exe (GUI без консоли) запись
// в stderr падает, MultiWriter на этом останавливается — и log.txt оставался пустым
// (так было в 0.2.0 у Марины).
type logWriter struct {
	file    io.Writer
	console io.Writer
}

func (w logWriter) Write(p []byte) (int, error) {
	n, err := w.file.Write(p)
	_, _ = w.console.Write(p)
	return n, err
}

// rotateLog переименовывает log.txt в log.old.txt, если он больше limit байт.
func rotateLog(path string, limit int64) {
	// Свой файл программы: папка данных может лежать на UNC (перенаправленный %APPDATA%), StatLocal её отверг бы и лог не ротировался.
	st, err := os.Lstat(path)
	if err != nil || st.Size() <= limit {
		return
	}
	old := strings.TrimSuffix(path, ".txt") + ".old.txt"
	// Ротация — удобство: при сбое лог просто продолжит расти до следующего запуска.
	_ = os.Remove(old)
	_ = os.Rename(path, old)
}

// logRare пишет в лог первое событие счётчика и затем каждое сотое:
// на повторяющихся ошибках (например, 404 от /media) файл не раздувается.
func logRare(n int64, format string, args ...any) {
	if n == 1 || n%100 == 0 {
		log.Printf("[x%d] "+format, append([]any{n}, args...)...) //nolint:gosec // G706: текст ошибок ОС и имя файла без путей, в лог-файл разработчику
	}
}
