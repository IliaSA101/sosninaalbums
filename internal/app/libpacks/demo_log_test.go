package libpacks

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/decor"
	"github.com/IliaSA101/sosninaalbums/internal/decor/pack"
)

// Файл примера новее программы (minApp выше версии): в log.txt причина словами, без пути на
// диске; пример не встаёт, событий нет.
func TestPlaceDemoLogsAppTooOld(t *testing.T) {
	e := newDemoEnv(t, true, func(h *Host) { h.Version = func() string { return "0.25.0" } }) // minApp набора — 0.26.0
	e.svc.PlaceDemo(t.Context())
	e.mustNotStand(demoPackID)
	out := e.log.String()
	if !strings.Contains(out, "пример: файл не принят — нужна версия Sosnina Albums 0.26.0 или новее") {
		t.Errorf("log.txt без причины: %q", out)
	}
	if strings.Contains(out, e.dir) || strings.Contains(out, "errorString") || len(e.ev.of("library:changed")) != 0 {
		t.Errorf("путь или тип в log.txt, либо событие: %q", out)
	}
}

// logDemoErr: известные отказы разбора и квота — словами, ошибка диска с путём — только типом.
func TestLogDemoErrNoPaths(t *testing.T) {
	buf := catchLog(t)
	logDemoErr("не поставлен", &os.PathError{Op: "open", Path: "/home/user/секрет/x", Err: os.ErrPermission})
	logDemoErr("не поставлен", fmt.Errorf("%w: нет «a.png»", pack.ErrDamaged))
	logDemoErr("не поставлен", decor.ErrLibFull)
	logDemoErr("файл не принят", nil)
	out := buf.String()
	for _, want := range []string{"(*fs.PathError)", "архив набора повреждён: нет «a.png»", "библиотека заполнена", "файл не принят (<nil>)"} {
		if !strings.Contains(out, want) {
			t.Errorf("нет %q: %q", want, out)
		}
	}
	if strings.Contains(out, "секрет") {
		t.Errorf("путь в log.txt: %q", out)
	}
}
