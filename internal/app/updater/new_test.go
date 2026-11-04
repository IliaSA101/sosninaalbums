package updater

import (
	"context"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
	"github.com/IliaSA101/sosninaalbums/internal/update"
)

// Пустое поле Host — паника New с именем поля (шаблон служб: testkit.NewPanics). Сквозные тесты
// обновления остаются в internal/app (0.26-126-15): они стоят на опоре App.
func TestUpdaterNewPanics(t *testing.T) {
	full := Host{
		CfgDir:     func() string { return "" },
		Version:    func() string { return "dev" },
		Emit:       func(string, ...any) {},
		GoBG:       func(func()) bool { return false },
		StopCtx:    context.Background,
		Quit:       func() {},
		OpenURL:    func(string) {},
		StartTimes: func() (time.Duration, time.Duration) { return 0, 0 },
		Client:     func() (*update.Client, bool) { return nil, false },
		GOOS:       func() string { return "linux" },
		Exe:        func() (string, error) { return "", nil },
		Dir:        func() string { return "" },
		Free:       func(string) (uint64, error) { return 0, nil },
		Start:      func(string, string) error { return nil },
	}
	testkit.NewPanics(t, full, func(h Host) { New(h) }, "updater.New")
}
