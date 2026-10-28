package app

import (
	"io"
	"log"
	"os"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/library"
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

// setupLog пишет лог в файл и, если есть консоль, в stderr; файл больше 2 МБ уходит в .old.
func setupLog(path string) {
	rotateLog(path, maxLogSize)
	if f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, logFileMode); err == nil {
		log.SetOutput(logWriter{file: f, console: os.Stderr})
	}
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
	st, err := library.StatLocal(path)
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
