//go:build qa

package app

import (
	"context"
	"errors"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/osx"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
	wr "github.com/wailsapp/wails/v2/pkg/runtime"
)

// Зонд окна для win-qa (спека 144, решение 0.31-144-59): в exe набора QA (тег qa) и только на стенде
// скрипт из SA_QA_PROBE выполняется в окне WebView2, а записи, которые он шлёт событием qaprobe,
// дописываются строками в файл SA_QA_PROBE_OUT. CDP к окну Wails невозможен (go-webview2 затирает
// аргументы браузера, 118-1), UIA не умеет запросов, а проверять нужно именно путь запроса через
// WebView2 и Wails. В выпуске файл не собирается: qaprobe.go — пустая заглушка.

const (
	qaProbeMaxJS   = 256 << 10 // предел скрипта: читается чужой файл
	qaProbeMaxLine = 64 << 10  // предел одной записи
	qaProbeMaxOut  = 4 << 20   // предел всего файла записей
)

func (a *App) qaProbe(ctx context.Context) {
	if !osx.StandOn() {
		return
	}
	src, out := os.Getenv("SA_QA_PROBE"), os.Getenv("SA_QA_PROBE_OUT")
	if src == "" || out == "" {
		return
	}
	js, err := qaProbeScript(src)
	if err != nil || !filepath.IsAbs(out) || paths.CheckLocalPath(out) != nil {
		log.Printf("зонд QA: скрипт или файл записей не годятся")
		return
	}
	sink := &qaProbeSink{path: out}
	wr.EventsOn(ctx, "qaprobe", sink.add)
	wr.WindowExecJS(ctx, js)
	log.Print("зонд QA: скрипт запущен")
}

// qaProbeScript — текст скрипта: локальный обычный файл не больше qaProbeMaxJS.
func qaProbeScript(path string) (string, error) {
	if !filepath.IsAbs(path) {
		return "", errors.New("путь не абсолютный")
	}
	f, _, err := library.OpenLocal(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	b, err := io.ReadAll(io.LimitReader(f, qaProbeMaxJS+1))
	if err != nil {
		return "", err
	}
	if len(b) > qaProbeMaxJS {
		return "", errors.New("скрипт больше предела")
	}
	return string(b), nil
}

// qaProbeSink дописывает записи зонда в файл; события приходят из разных горутин.
type qaProbeSink struct {
	mu   sync.Mutex
	path string
	size int
}

func (s *qaProbeSink) add(data ...any) {
	line := `{"step":"qa-bad-record"}`
	if len(data) > 0 {
		if t, ok := data[0].(string); ok && len(t) <= qaProbeMaxLine && !strings.ContainsAny(t, "\r\n") {
			line = t
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.size+len(line)+1 > qaProbeMaxOut {
		return
	}
	f, err := os.OpenFile(s.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer func() { _ = f.Close() }()
	if _, err := f.WriteString(line + "\n"); err == nil {
		s.size += len(line) + 1
	}
}
