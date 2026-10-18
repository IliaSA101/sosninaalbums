package album_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// wireJSON — вид в карту, как его видит фронт.
func wireJSON(t *testing.T, v model.View) map[string]any {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

// wireMerge — слияние фронта (base/core/viewdelta.ts mergeView; векторы — model
// TestViewDeltaParity): шапка дельты, развороты по order — из дельты, иначе из prev.
func wireMerge(t *testing.T, prev, d map[string]any) map[string]any {
	t.Helper()
	byID := func(v any) map[string]any {
		out := map[string]any{}
		l, _ := v.([]any)
		for _, s := range l {
			out[s.(map[string]any)["id"].(string)] = s
		}
		return out
	}
	old, fresh := byID(prev["spreads"]), byID(d["spreads"])
	out := map[string]any{}
	for k, v := range d {
		if k != "base" && k != "order" {
			out[k] = v
		}
	}
	spreads := []any{}
	order, _ := d["order"].([]any)
	for _, id := range order {
		s, ok := fresh[id.(string)]
		if !ok {
			s, ok = old[id.(string)]
		}
		if !ok {
			t.Fatalf("разворота %v нет ни в дельте, ни в прежнем виде", id)
		}
		spreads = append(spreads, s)
	}
	out["spreads"] = spreads
	return out
}

// TestWireDeltaReplay — весь голден-реплей ядра цепочкой дельт: после каждой правки слияние
// прежнего вида с дельтой равно полному виду (всё, что видит фронт, дельта доносит), а
// неизменённые развороты в дельту не идут хотя бы на части шагов (иначе выигрыша нет).
func TestWireDeltaReplay(t *testing.T) {
	r := replayScenario(t)
	if len(r.views) < 50 {
		t.Fatalf("в реплее %d правок: сценарий не тот", len(r.views))
	}
	prev := r.views[0]
	prev.Seq = 1
	have := wireJSON(t, prev)
	partial, idle := 0, 0
	for i, v := range r.views[1:] {
		seq := i + 2
		w, same := model.Wire(&prev, v, seq-1, seq)
		got := wireMerge(t, have, wireJSON(t, w))
		want := v
		want.Seq = seq
		wm := wireJSON(t, want)
		if wm["spreads"] == nil {
			wm["spreads"] = []any{}
		}
		if !reflect.DeepEqual(got, wm) {
			t.Fatalf("шаг %d: слияние дельты не равно полному виду", i+1)
		}
		if same != model.SameView(prev, v) || same && w.Fresh() != 0 {
			t.Fatalf("шаг %d: same %v, изменено %d", i+1, same, w.Fresh())
		}
		if same {
			idle++
		}
		if n := w.Fresh(); n > 0 && n < len(v.Spreads) {
			partial++
		}
		prev, have = v, got
	}
	if partial == 0 || idle == 0 {
		t.Fatalf("частичных дельт %d, холостых %d: ждали и те и другие", partial, idle)
	}
	t.Logf("правок %d: частичных дельт %d, холостых %d", len(r.views)-1, partial, idle)
}
