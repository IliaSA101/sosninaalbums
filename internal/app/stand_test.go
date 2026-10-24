package app

import (
	"encoding/json"
	"strings"
	"testing"
)

// State.Stand: ставится из признака стенда в GetState и в ответах, что идут через него
// (новый альбом); в JSON без стенда поля нет (omitempty).
func TestStateStand(t *testing.T) {
	old := standOn
	t.Cleanup(func() { standOn = old })
	a := newApp(t)

	standOn = func() bool { return true }
	if !a.GetState().Stand {
		t.Error("стенд: GetState().Stand=false")
	}
	if st := a.NewProject("album"); !st.Stand {
		t.Error("стенд: State из NewProject без Stand")
	}
	b, err := json.Marshal(a.GetState())
	if err != nil || !strings.Contains(string(b), `"stand":true`) {
		t.Errorf("стенд: JSON без stand:true (%v)", err)
	}

	standOn = func() bool { return false }
	if a.GetState().Stand {
		t.Error("без стенда (выпуск): Stand=true")
	}
	b, _ = json.Marshal(a.GetState())
	if strings.Contains(string(b), `"stand"`) {
		t.Error("без стенда поле stand не должно попадать в JSON")
	}
}
