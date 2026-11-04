//go:build qa

package osx

import (
	"io/fs"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Шов win-qa (спека 157): SA_QA_VOLDELAY=Q:4000 — первая проба тома Q: отвечает через 4000 мс
// («сонный диск»). Только в exe набора QA (тег qa) и только на стенде (StandOn); выпуск файл не
// собирает, а страж строк release.yml не даёт имени переменной попасть в бинарник.

const (
	qaVolDelayEnv = "SA_QA_VOLDELAY"
	qaVolDelayMax = 30 * time.Second
)

// qaVolDelay разбирает «Q:4000» → («Q:», 4 с). Неверное — ok=false: шов выключен, путь не строится.
func qaVolDelay(spec string) (vol string, d time.Duration, ok bool) {
	l, ms, found := strings.Cut(spec, ":")
	if !found || len(l) != 1 || !(l[0] >= 'A' && l[0] <= 'Z' || l[0] >= 'a' && l[0] <= 'z') {
		return "", 0, false
	}
	n, err := strconv.Atoi(ms)
	if err != nil || n <= 0 || n > int(qaVolDelayMax/time.Millisecond) {
		return "", 0, false
	}
	return strings.ToUpper(l) + ":", time.Duration(n) * time.Millisecond, true
}

// qaSleep — ожидание задержки; подменяется в тесте.
var qaSleep = time.Sleep

// qaStat оборачивает stat корня тома: первая проба буквы из SA_QA_VOLDELAY ждёт задержку.
func qaStat(stat func(string) (fs.FileInfo, error)) func(string) (fs.FileInfo, error) {
	if !StandOn() {
		return stat
	}
	vol, d, ok := qaVolDelay(os.Getenv(qaVolDelayEnv))
	if !ok {
		return stat
	}
	var once sync.Once
	return func(p string) (fs.FileInfo, error) {
		if strings.ToUpper(strings.TrimRight(p, `\/`)) == vol {
			once.Do(func() { qaSleep(d) })
		}
		return stat(p)
	}
}
