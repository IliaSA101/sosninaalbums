package assoc

import (
	"bytes"
	"errors"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/app/appkit"
	"github.com/IliaSA101/sosninaalbums/internal/osx"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Паника внутри OpenInNewer — ErrInternal, а не nil (GuardErr, не Guard): иначе окно «Альбом из
// новой версии» решило бы, что новая версия запущена и программа вот-вот закроется.
func TestOpenInNewerPanicGivesError(t *testing.T) {
	var b bytes.Buffer
	log.SetOutput(&b)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	quit := false
	s := New(Host{
		CfgDir:    func() string { return "" },
		Data:      func() string { return osx.AppName },
		Version:   func() string { return "0.25.0" },
		Notice:    func(string) {},
		GoBG:      func(func()) bool { return false },
		Exists:    func(string) bool { return true },
		FlushSave: func() { panic("бум: FlushSave") },
		Quit:      func() { quit = true },
		Reg:       func() Registry { return nil },
		Place:     func() osx.Place { return osx.PlaceNormal },
		Temp:      func() string { return filepath.Join(testkit.TempDir(t), "tmp") },
		Start:     func(string, ...string) error { return nil },
		DevBuild:  func() bool { return false },
	})
	// Найденная новая версия: путь проходит exeOK и доходит до сохранения альбома.
	exe := filepath.Join(testkit.TempDir(t), "sosninaalbums-0.26.exe")
	s.nmu.Lock()
	s.exe = exe
	s.nmu.Unlock()
	err := s.OpenInNewer()
	if !errors.Is(err, appkit.ErrInternal) {
		t.Fatalf("после паники ждали ErrInternal, получили %v", err)
	}
	if quit {
		t.Fatal("после паники программа закрылась")
	}
	if got := b.String(); !strings.Contains(got, "ПАНИКА в OpenInNewer: бум") {
		t.Fatalf("паника не записана: %q", got)
	}
}
