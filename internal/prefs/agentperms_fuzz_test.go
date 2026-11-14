package prefs

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

// FuzzAgentPermsConfig: любой config.json (чужие данные) даёт сверенный вид без паники: значения
// из трёх, ≤ 10 мест, все инструменты этой версии на месте, «Спрашивать» только где оно есть.
func FuzzAgentPermsConfig(f *testing.F) {
	for _, s := range []string{
		``, `null`, `{}`, `{"agentPerms":5}`, `{"agentPerms":{"tools":{"autobuild":"allow"},"edit":true}}`,
		`{"agents":{"edit":true},"agentPerms":{"places":[{"dir":"/a","how":"chosen"}]}}`,
		`{"agentPerms":{"tools":{"x":1},"places":[null,{"dir":"\u0000"}]}}`,
		`{"agentPerms":{"places":[{"dir":"/a","how":"parent"},{"dir":"/a/b","how":"chosen"}],"lifted":[{"from":"/a/2026","to":"/a"},null,{"from":5}]}}`,
		`{"agentPerms":{"places":[{"dir":"/a","how":"chosen","vol":{"id":"uuid:x","label":"A\u0000","rel":"b/../../c"}},{"dir":"/a","how":"chosen","vol":{"id":"","rel":"/etc"}},{"dir":"/d","how":"parent","vol":5}]}}`,
		`{"agentPerms":{"places":[{"dir":"/a","how":"chosen","vol":{"id":"win:1:NTFS","rel":""}},{"dir":"/b","how":"chosen","vol":{"id":"win:1:NTFS","rel":"x/y"}}]}}`,
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
		if len(p.Places) > MaxPlaces || len(p.Lifted) > MaxPlaces {
			t.Fatalf("мест %d, lifted %d", len(p.Places), len(p.Lifted))
		}
		checkFuzzPlaces(t, p.Places)
		for _, l := range p.Lifted {
			if l.From == l.To || len(l.From) > maxPlaceBytes || len(l.To) > maxPlaceBytes {
				t.Fatalf("lifted негоден: %+v", l)
			}
		}
		for _, k := range AgentToolNames() {
			if p.Tools[k] == "" {
				t.Fatalf("нет значения у %s", k)
			}
		}
	})
}

// checkFuzzPlaces: ключи мест не повторяются, привязка к диску годна (пределы, UTF-8, без «..»).
func checkFuzzPlaces(t *testing.T, places []Place) {
	t.Helper()
	seen := map[string]bool{}
	for _, pl := range places {
		if seen[pl.Key()] {
			t.Fatalf("два места с одним ключом: %+v", pl)
		}
		seen[pl.Key()] = true
		if v := pl.Vol; v != nil {
			if v.ID == "" || len(v.ID) > maxVolID || len(v.Label) > maxVolLabel || len(v.Rel) > maxVolRel ||
				!noControl(v.ID) || !noControl(v.Rel) || !noControl(v.Label) || v.Rel == ".." || strings.HasPrefix(v.Rel, "../") || strings.HasPrefix(v.Rel, "/") {
				t.Fatalf("vol негоден: %+v", v)
			}
		}
	}
}
