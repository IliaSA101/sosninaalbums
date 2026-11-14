package app

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/osx"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

// AgentPlace.Bindable считает Go (160): окно не гадает по ОС. Условие — подмена placeCanBind
// (macOS: только под /Volumes), и в AgentsInfo, и в AgentPlacesLive, где путь привязанного места
// берётся по номеру диска, а не из config.json.
func TestAgentPlaceBindable(t *testing.T) {
	s := newVolStand(t)
	old := placeCanBind
	placeCanBind = func(p string) bool { return strings.HasPrefix(p, s.E) }
	t.Cleanup(func() { placeCanBind = old })

	bound := prefs.Place{Dir: filepath.Join(s.F, "Клиент"), How: prefs.PlaceParent, Vol: &prefs.PlaceVol{ID: "card1", Label: "старая", Rel: "Клиент"}} // буква была F, диск теперь на E
	onE := prefs.Place{Dir: filepath.Join(s.E, "Клиент", "Свадьба"), How: prefs.PlaceChosen}
	onF := prefs.Place{Dir: filepath.Join(s.F, "Клиент", "Свадьба"), How: prefs.PlaceChosen}
	prefs.UpdateAgentPerms(s.a.cfgDir, func(p *prefs.AgentPerms) { p.Places = []prefs.Place{bound, onE, onF} })

	want := func(list []AgentPlace, label string, exp map[prefs.Place]bool) {
		t.Helper()
		got := map[string]AgentPlace{}
		for _, p := range list {
			got[p.Key] = p
		}
		if len(got) != len(exp) {
			t.Fatalf("%s: мест %d, ждали %d: %+v", label, len(got), len(exp), list)
		}
		for pl, b := range exp {
			if v := got[prefs.PlaceKey(pl)]; v.Bindable != b {
				t.Errorf("%s: %s Bindable=%v, ждали %v", label, pl.Dir, v.Bindable, b)
			}
		}
	}
	// По config.json: путь привязанного — прежняя буква F.
	want(s.a.AgentsInfo().Places, "AgentsInfo", map[prefs.Place]bool{bound: false, onE: true, onF: false})
	// Вживую: привязанное место нашлось на E.
	want(s.a.AgentPlacesLive(), "AgentPlacesLive", map[prefs.Place]bool{bound: true, onE: true, onF: false})

	// Без подмены — по ОС процесса: на Windows и Linux привязать можно любое место.
	placeCanBind = osx.CanBind
	for _, p := range s.a.AgentsInfo().Places {
		if p.Bindable != osx.CanBind(p.Dir) {
			t.Errorf("настоящий CanBind: %s Bindable=%v", p.Dir, p.Bindable)
		}
	}
}

// Мест нет — пустой список без паники, у DTO поле по умолчанию ложно.
func TestAgentPlaceBindableEmpty(t *testing.T) {
	s := newVolStand(t)
	if got := s.a.AgentsInfo().Places; got == nil || len(got) != 0 {
		t.Fatalf("без мест: %#v", got)
	}
	if got := s.a.AgentPlacesLive(); len(got) != 0 {
		t.Fatalf("без мест вживую: %+v", got)
	}
	if (AgentPlace{}).Bindable {
		t.Error("пустое место не привязываемо")
	}
}
