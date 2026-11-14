package app

// Доправка ревью 153: проход подъёма один раз (отметка LiftDone), ✕ у поднятого места чистит заметку.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

// Проход подъёма — один раз: место 0.35 без диска (Mac, том без номера) при втором и третьем старте
// не поднимается выше и не шумит; ключ без засева (первая запись — «Забыть места») отметку тоже
// получает; файл 0.34 без отметки поднимается и получает её, даже когда поднимать нечего.
func TestPlaceLiftOnce(t *testing.T) {
	a, _, _ := placeApp(t)
	captureLog(t)
	ev := watchEvents(a)
	arch := realDir(t, "2020-архив", "Фото")
	y26 := mkDirs(t, arch, "2026", "Петровы")

	// Ключа не было: первая запись создаёт его сразу с отметкой, место 0.35 без Vol не трогается.
	prefs.ForgetPlaces(a.cfgDir)
	prefs.UpdateAgentPerms(a.cfgDir, func(p *prefs.AgentPerms) {
		p.Places = []prefs.Place{{Dir: y26, How: prefs.PlaceParent}}
	})
	if !prefs.LoadAgentPerms(a.cfgDir).LiftDone {
		t.Fatal("первая запись ключа не поставила отметку")
	}
	raw, _ := os.ReadFile(prefs.ConfigPath(a.cfgDir))
	for range 3 { // три старта
		a.liftLegacyPlaces()
	}
	after, _ := os.ReadFile(prefs.ConfigPath(a.cfgDir))
	if string(after) != string(raw) || ev.n("agents:state") != 0 || ev.n(placesLiftedEvent) != 0 {
		t.Fatalf("место 0.35 тронуто: события %d/%d\n%s", ev.n("agents:state"), ev.n(placesLiftedEvent), after)
	}
	if pl := prefs.LoadAgentPerms(a.cfgDir).Places; len(pl) != 1 || pl[0].Dir != y26 {
		t.Fatalf("места: %+v", pl)
	}

	// Засев тоже ставит отметку.
	b, _, _ := placeApp(t)
	prefs.RememberDir(b.cfgDir, prefs.DirPhotos, y26)
	b.seedPlaces()
	if !prefs.LoadAgentPerms(b.cfgDir).LiftDone {
		t.Error("засев не поставил отметку")
	}

	// Файл 0.34: подъём один раз; дальше то же место не двигается и не шумит.
	prefs.UpdateAgentPerms(a.cfgDir, func(p *prefs.AgentPerms) { p.LiftDone = false })
	a.liftLegacyPlaces()
	if pl := prefs.LoadAgentPerms(a.cfgDir).Places; len(pl) != 1 || pl[0].Dir == y26 || !prefs.LoadAgentPerms(a.cfgDir).LiftDone {
		t.Fatalf("0.34 не поднято или нет отметки: %+v", pl)
	}
	if ev.n(placesLiftedEvent) != 1 {
		t.Fatalf("событий подъёма %d", ev.n(placesLiftedEvent))
	}
	raw, _ = os.ReadFile(prefs.ConfigPath(a.cfgDir))
	a.liftLegacyPlaces()
	a.liftLegacyPlaces()
	if after, _ = os.ReadFile(prefs.ConfigPath(a.cfgDir)); string(after) != string(raw) || ev.n(placesLiftedEvent) != 1 {
		t.Fatalf("повтор изменил файл или поднял событие:\n%s", after)
	}
}

// ✕ у поднятого места: заметка об этом месте уходит той же записью, о другом остаётся; ✕ у места без
// заметки заметки не трогает.
func TestForgetLiftedPlaceNote(t *testing.T) {
	a, ev, arch, chosen, gone := legacyApp(t)
	a.liftLegacyPlaces()
	if l := prefs.LoadAgentPerms(a.cfgDir).Lifted; len(l) != 2 {
		t.Fatalf("lifted до ✕: %+v", l)
	}
	// Заметка о другом месте (не arch): остаётся.
	other := prefs.PlaceLift{From: filepath.Join(chosen, "x"), To: chosen}
	prefs.UpdateAgentPerms(a.cfgDir, func(p *prefs.AgentPerms) { p.Lifted = append(p.Lifted, other) })

	a.ForgetAgentPlace(prefs.PlaceKey(prefs.Place{Dir: gone, How: prefs.PlaceParent})) // места без заметок
	if l := prefs.LoadAgentPerms(a.cfgDir).Lifted; len(l) != 3 {
		t.Fatalf("✕ у места без заметки тронул заметки: %+v", l)
	}

	n := ev.n("agents:state")
	info := a.ForgetAgentPlace(prefs.PlaceKey(prefs.Place{Dir: arch, How: prefs.PlaceParent}))
	l := prefs.LoadAgentPerms(a.cfgDir).Lifted
	if len(l) != 1 || l[0] != other || len(info.Lifted) != 1 || info.Lifted[0].To != chosen {
		t.Fatalf("после ✕ у поднятого: %+v / %+v", l, info.Lifted)
	}
	if ev.n("agents:state") != n+1 {
		t.Errorf("записей не одна: +%d", ev.n("agents:state")-n)
	}
	// Последняя заметка: ключ lifted из файла исчезает.
	a.ForgetAgentPlace(prefs.PlaceKey(prefs.Place{Dir: chosen, How: prefs.PlaceChosen}))
	if data, _ := os.ReadFile(prefs.ConfigPath(a.cfgDir)); strings.Contains(string(data), `"lifted"`) {
		t.Errorf("lifted остался в файле: %s", data)
	}
}
