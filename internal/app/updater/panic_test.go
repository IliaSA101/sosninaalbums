package updater

import (
	"bytes"
	"context"
	"errors"
	"log"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/app/appkit"
	"github.com/IliaSA101/sosninaalbums/internal/update"
)

// panicHost — Host, где падает одно место: Emit, StopCtx, GOOS и OpenURL бросают панику, когда
// метод до них дошёл. Остальные поля — безобидные заглушки.
func panicHost(t *testing.T, goos string) Host {
	t.Helper()
	return Host{
		CfgDir:     t.TempDir,
		Version:    func() string { return "0.1.0" },
		Emit:       func(string, ...any) {},
		GoBG:       func(func()) bool { return false },
		StopCtx:    func() context.Context { panic("бум: StopCtx") },
		Quit:       func() { panic("бум: Quit") },
		OpenURL:    func(string) { panic("бум: OpenURL") },
		StartTimes: func() (time.Duration, time.Duration) { return 0, 0 },
		Client:     func() (*update.Client, bool) { return nil, false },
		GOOS:       func() string { return goos },
		Exe:        func() (string, error) { return `C:\Prog\SosninaAlbums.exe`, nil },
		Dir:        func() string { return "" },
		Free:       func(string) (uint64, error) { return 0, nil },
		Start:      func(string, string) error { return nil },
	}
}

// logBuf — журнал в буфер на время теста.
func logBuf(t *testing.T) *bytes.Buffer {
	t.Helper()
	var b bytes.Buffer
	log.SetOutput(&b)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	return &b
}

// Паника внутри метода с ошибкой — ErrInternal, а не nil и не падение программы (GuardErr, не
// Guard): иначе интерфейс счёл бы обновление удавшимся. Каждый метод — свой defer, свой случай.
func TestUpdaterPanicGivesError(t *testing.T) {
	ready := &update.Offer{Version: "0.99.0", Setup: &update.File{}}
	cases := []struct {
		name, log string
		run       func(*Service) error
	}{
		{"Check", "CheckUpdate", func(s *Service) error { _, err := s.Check(); return err }},
		{"Download", "DownloadUpdate", func(s *Service) error {
			s.offer = ready // есть что качать: путь доходит до begin → StopCtx
			_, err := s.Download()
			return err
		}},
		{"Install", "InstallUpdate", func(s *Service) error {
			s.offer, s.phase, s.path = ready, phaseReady, "setup.exe"
			s.h.GOOS = func() string { panic("бум: GOOS") }
			return s.Install()
		}},
		{"OpenPage", "OpenUpdatePage", func(s *Service) error { return s.OpenPage() }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			b := logBuf(t)
			s := New(panicHost(t, "windows"))
			err := c.run(s)
			if !errors.Is(err, appkit.ErrInternal) {
				t.Fatalf("после паники ждали ErrInternal, получили %v", err)
			}
			if got := b.String(); !strings.Contains(got, "ПАНИКА в "+c.log+": бум") {
				t.Fatalf("паника не записана под именем %s: %q", c.log, got)
			}
		})
	}
}
