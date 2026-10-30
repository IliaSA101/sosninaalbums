package assoc

import (
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/osx"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Пустое поле Host — паника New с именем поля (шаблон служб: testkit.NewPanics). Сквозные тесты
// связи остаются в internal/app (0.26-126-15): они стоят на опоре App.
func TestAssocNewPanics(t *testing.T) {
	full := Host{
		CfgDir:    func() string { return "" },
		Data:      func() string { return osx.AppName },
		Version:   func() string { return "dev" },
		Notice:    func(string) {},
		GoBG:      func(func()) bool { return false },
		Exists:    func(string) bool { return false },
		FlushSave: func() {},
		Quit:      func() {},
		Reg:       func() Registry { return nil },
		Place:     func() osx.Place { return osx.PlaceNormal },
		Temp:      func() string { return "" },
		Start:     func(string, ...string) error { return nil },
		DevBuild:  func() bool { return false },
	}
	testkit.NewPanics(t, full, func(h Host) { New(h) }, "assoc.New")
}
