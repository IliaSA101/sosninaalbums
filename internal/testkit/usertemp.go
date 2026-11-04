package testkit

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// userTempPrefix — имя временных папок UserTemp в домашней папке; по нему же их подметают.
const userTempPrefix = "sa-testtmp-"

// userTempMaxAge — старше этого срока папка UserTemp считается остатком убитого теста.
const userTempMaxAge = 24 * time.Hour

var sweepOnce sync.Once

// UserTemp — временная папка, которую правила путей агента принимают за «обычную»: на macOS os.TempDir() лежит в
// /private/var/folders — системном дереве (внутри /private и /var), и папка там отвергается правилом по замыслу
// (162-7: менять нужно тест, а не границу). Имя без точки: скрытые элементы пути правило записи отвергает на любой Unix-ОС
// (hiddenWrite), поэтому `.sa-test-*` из design не годится. Поэтому на darwin папка создаётся в $HOME как sa-testtmp-*, на прочих ОС —
// обычный TempDir. Удаляется как TempDir. При первом вызове процесса подметает остатки старше суток: тест, убитый по
// сроку, Cleanup не выполняет, и они копились бы в домашней папке. Пустой HOME — t.Fatal: MkdirTemp("") взял бы
// os.TempDir, а склейка "" + "/x" в других местах — корень диска.
func UserTemp(t testing.TB) string {
	t.Helper()
	return userTempFor(t, runtime.GOOS)
}

// userTempFor — UserTemp для ОС goos (тест проверяет ветку darwin на любой ОС).
func userTempFor(t testing.TB, goos string) string {
	t.Helper()
	if goos != "darwin" {
		return TempDir(t)
	}
	dir, err := userTempIn(os.Getenv("HOME"), time.Now(), &sweepOnce)
	if err != nil {
		t.Fatalf("UserTemp: %v", err)
	}
	h := hooksOf(t)
	t.Cleanup(func() {
		h.run(t)
		if err := removeAll(dir, removeTimeout); err != nil {
			t.Errorf("UserTemp RemoveAll cleanup: %v", err)
		}
	})
	return dir
}

// userTempIn создаёт sa-testtmp-* в home; перед первым созданием (once) подметает остатки старше userTempMaxAge.
func userTempIn(home string, now time.Time, once *sync.Once) (string, error) {
	if home == "" {
		return "", errors.New("HOME пуст: папку в домашней не создать")
	}
	if !filepath.IsAbs(home) {
		return "", fmt.Errorf("HOME не абсолютный путь: %q", home)
	}
	once.Do(func() { sweepUserTemp(home, now) })
	return os.MkdirTemp(home, userTempPrefix+"*")
}

// sweepUserTemp удаляет в home папки sa-testtmp-* старше userTempMaxAge; ошибки не важны (чужой процесс мог удалить
// сам, а остаток подметёт следующий запуск). Файлы и ссылки не трогает: только настоящие папки с нашим именем.
func sweepUserTemp(home string, now time.Time) {
	list, err := os.ReadDir(home)
	if err != nil {
		return
	}
	for _, e := range list {
		if !strings.HasPrefix(e.Name(), userTempPrefix) || !e.Type().IsDir() {
			continue
		}
		if info, err := e.Info(); err == nil && now.Sub(info.ModTime()) > userTempMaxAge {
			_ = os.RemoveAll(filepath.Join(home, e.Name()))
		}
	}
}
