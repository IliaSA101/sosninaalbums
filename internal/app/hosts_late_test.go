package app

import (
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Позднее связывание Host (126-25): в программе cfgDir ставит startup, assocReg — Run, а тесты
// меняют emit, exists и время старта; все они после NewApp. Host службы читает их при каждом
// вызове — значение, снятое при сборке Host, увидело бы старое (пустой cfgDir, nil-реестр).
// Хосты берём сразу после NewApp и только потом меняем App.
func TestHostsLateBound(t *testing.T) {
	a := newApp(t)
	uh, ah, eh := a.updHost(), a.assocHost(), a.expHost()

	// Подмены после сборки Host.
	dir := testkit.TempDir(t)
	reg := &fakeReg{}
	var events []string
	a.cfgDir = dir
	a.data = "late-data"
	a.assocReg = reg
	a.emit = func(name string, _ ...any) { events = append(events, name) }
	asked := map[string]int{}
	a.exists = func(p string) bool { asked[p]++; return p == "/late/yes" }
	defer setStart(7*time.Second, 9*time.Hour)()
	ed := mustEditor(t, model.New(), "")
	a.ed.Store(ed)

	if got := uh.CfgDir(); got != dir {
		t.Errorf("updater CfgDir = %q, ждали %q", got, dir)
	}
	if got := ah.CfgDir(); got != dir {
		t.Errorf("assoc CfgDir = %q, ждали %q", got, dir)
	}
	if got := ah.Data(); got != "late-data" {
		t.Errorf("assoc Data = %q", got)
	}
	if got := ah.Reg(); got != reg {
		t.Errorf("assoc Reg = %v, ждали подменённый реестр", got)
	}
	if d, e := uh.StartTimes(); d != 7*time.Second || e != 9*time.Hour {
		t.Errorf("StartTimes = %v %v, ждали 7s 9h", d, e)
	}

	uh.Emit("u")
	eh.Emit("e")
	ah.Notice("n")
	if want := []string{"u", "e", "notice"}; len(events) != 3 || events[0] != want[0] || events[1] != want[1] || events[2] != want[2] {
		t.Errorf("события = %v, ждали %v", events, want)
	}

	if !ah.Exists("/late/yes") || ah.Exists("/late/no") {
		t.Error("assoc Exists не зовёт подменённую a.exists")
	}
	if !eh.Exists("/late/yes") || eh.Exists("/late/no") {
		t.Error("exporter Exists не зовёт подменённую a.exists")
	}
	if asked["/late/yes"] != 2 || asked["/late/no"] != 2 {
		t.Errorf("a.exists вызвана %v", asked)
	}

	if eh.Editor() != ed {
		t.Error("exporter Editor не текущий редактор App")
	}
	ed2 := mustEditor(t, model.New(), "")
	a.ed.Store(ed2)
	if eh.Editor() != ed2 {
		t.Error("exporter Editor закэширован при сборке Host")
	}
}
