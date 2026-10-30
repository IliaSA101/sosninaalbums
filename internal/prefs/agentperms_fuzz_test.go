package prefs

import (
	"encoding/json"
	"slices"
	"testing"
)

// FuzzAgentPermsConfig: любой config.json (чужие данные) даёт сверенный вид без паники: значения
// из трёх, ≤ 10 мест, все инструменты этой версии на месте, «Спрашивать» только где оно есть.
func FuzzAgentPermsConfig(f *testing.F) {
	for _, s := range []string{
		``, `null`, `{}`, `{"agentPerms":5}`, `{"agentPerms":{"tools":{"autobuild":"allow"},"edit":true}}`,
		`{"agents":{"edit":true},"agentPerms":{"places":[{"dir":"/a","how":"chosen"}]}}`,
		`{"agentPerms":{"tools":{"x":1},"places":[null,{"dir":"\u0000"}]}}`,
	} {
		f.Add([]byte(s))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		var c Config
		_ = json.Unmarshal(data, &c)
		p := AgentPermsOf(c)
		for k, v := range p.Tools {
			if v != PermAllow && v != PermAsk && v != PermDeny {
				t.Fatalf("%s = %q", k, v)
			}
			if v == PermAsk && !slices.Contains(PermCan(k), PermAsk) {
				t.Fatalf("%s: «Спрашивать» там, где его нет", k)
			}
		}
		if len(p.Places) > MaxPlaces {
			t.Fatalf("мест %d", len(p.Places))
		}
		for _, k := range AgentToolNames() {
			if p.Tools[k] == "" {
				t.Fatalf("нет значения у %s", k)
			}
		}
	})
}
