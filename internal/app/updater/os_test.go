package updater

import (
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/update"
)

// Стенд: «уже показан» — поле службы на сеанс; без недоступной версии тоста нет, новая версия —
// снова, прежняя — молчит.
func TestToldOnStand(t *testing.T) {
	s := New(panicHost(t, "linux"))
	blocked := func(v string) update.Result { return update.Result{Blocked: v} }
	steps := []struct {
		res  update.Result
		want bool
	}{
		{update.Result{}, false},
		{blocked("0.26.0"), true},
		{blocked("0.26.0"), false},
		{blocked("0.27.0"), true},
		{update.Result{Blocked: "0.28.0", Offer: &update.Offer{Version: "0.26.0", Page: true}}, false}, // со ступенью — update:offer
		{blocked("0.27.0"), false},
	}
	for i, st := range steps {
		if got := s.toldOnStand(st.res); got != st.want {
			t.Errorf("шаг %d: %v, ждали %v", i, got, st.want)
		}
	}
}
