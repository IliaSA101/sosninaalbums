package exporter

import (
	"context"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Пустое поле Host — паника New с именем поля (шаблон служб: testkit.NewPanics). Сквозные тесты
// выгрузки остаются в internal/app (0.26-126-15): они стоят на опоре App.
func TestExporterNewPanics(t *testing.T) {
	full := Host{
		Editor:      func() *album.Editor { return nil },
		Exists:      func(string) bool { return false },
		Emit:        func(string, ...any) {},
		GoBG:        func(func()) bool { return false },
		StopCtx:     context.Background,
		ColorReport: func(*model.Project) (map[string]int, map[string]int) { return nil, nil },
		Ask:         func(string) (bool, error) { return false, nil },
		PickDir:     func(string) (string, error) { return "", nil },
		RememberDir: func(string) {},
		OpenURL:     func(string) {},
		ShowDir:     func(string) {},
		LiftMemory:  func() func() { return func() {} },
	}
	testkit.NewPanics(t, full, func(h Host) { New(h) }, "exporter.New")
}

// Свежая служба: выгрузка не идёт, отмены и папки нет; Show без папки открывает "" (как было).
func TestExporterFreshState(t *testing.T) {
	var shown []string
	h := Host{
		Editor: func() *album.Editor { return nil }, Exists: func(string) bool { return false },
		Emit: func(string, ...any) {}, GoBG: func(func()) bool { return false }, StopCtx: context.Background,
		ColorReport: func(*model.Project) (map[string]int, map[string]int) { return nil, nil },
		Ask:         func(string) (bool, error) { return false, nil }, PickDir: func(string) (string, error) { return "", nil },
		RememberDir: func(string) {}, OpenURL: func(string) {}, ShowDir: func(d string) { shown = append(shown, d) },
		LiftMemory: func() func() { return func() {} },
	}
	s := New(h)
	if busy, cancelable := s.Busy(); busy || cancelable || s.LastDir() != "" {
		t.Fatalf("новая служба: идёт %v, отмена %v, папка %q", busy, cancelable, s.LastDir())
	}
	s.Cancel() // без выгрузки — ничего
	s.Show()
	if len(shown) != 1 || shown[0] != "" {
		t.Fatalf("Show без выгрузки: %q", shown)
	}
}
