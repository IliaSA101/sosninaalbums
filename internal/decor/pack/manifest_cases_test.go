package pack

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// Паритет с scripts/pack-check.mjs (спека 143, ступень 1): те же случаи testdata/pack-manifest-cases.json
// читает scripts/pack-check.test.mjs. Расходятся итоги — автор набора увидит в скрипте не то, что скажет программа.

// manifestCase — случай файла: вход (manifest, raw, gen, padTo), версия программы и ожидаемый итог (подмножество verdict).
type manifestCase struct {
	Name     string          `json:"name"`
	App      string          `json:"app"`
	Manifest json.RawMessage `json:"manifest"`
	Raw      *string         `json:"raw"`
	Gen      *struct {
		Count int    `json:"count"`
		Kind  string `json:"kind"`
		Ext   string `json:"ext"`
		Size  int64  `json:"size"`
	} `json:"gen"`
	PadTo int            `json:"padTo"`
	Want  map[string]any `json:"want"`
}

// caseInput — байты манифеста случая.
func caseInput(t *testing.T, c manifestCase) []byte {
	t.Helper()
	if c.Raw != nil {
		return []byte(*c.Raw)
	}
	var m map[string]any
	if err := json.Unmarshal(c.Manifest, &m); err != nil {
		t.Fatalf("%s: manifest: %v", c.Name, err)
	}
	if g := c.Gen; g != nil {
		items := make([]any, g.Count)
		for i := range items {
			items[i] = map[string]any{"file": fmt.Sprintf("f%d.%s", i, g.Ext), "kind": g.Kind, "theme": "basic", "name": "n",
				"sha256": strings.Repeat("0", 64), "size": g.Size}
		}
		m["items"] = items
	}
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if n := c.PadTo - len(data); n > 0 {
		data = append(data, bytes.Repeat([]byte(" "), n)...)
	}
	return data
}

// manifestCode — класс отказа, как code в pack-check.mjs.
func manifestCode(err error) string {
	switch {
	case errors.Is(err, ErrNewer):
		return "newer"
	case errors.Is(err, ErrAppOld):
		return "appOld"
	case errors.Is(err, ErrEmpty):
		return "empty"
	case errors.Is(err, ErrDamaged):
		return "damaged"
	case errors.Is(err, ErrManifest):
		return "manifest"
	}
	return "other"
}

// manifestVerdict — итог ParseManifest в форме скрипта (ключи — как в want).
func manifestVerdict(data []byte, app string) map[string]any {
	m, err := ParseManifest(data, app)
	if err != nil {
		return map[string]any{"ok": false, "code": manifestCode(err), "error": err.Error()}
	}
	items := make([]any, 0, len(m.Items))
	for _, it := range m.Items {
		items = append(items, map[string]any{"file": it.File, "kind": it.Kind, "theme": it.Theme, "name": it.Name, "size": it.Size})
	}
	skipped := make([]any, 0, len(m.Skipped))
	for _, s := range m.Skipped {
		skipped = append(skipped, map[string]any{"file": s.File, "why": s.Why})
	}
	return map[string]any{"ok": true, "code": "", "error": "", "title": m.Title, "about": m.About, "author": m.Author,
		"license": m.License, "source": m.Source, "items": items, "skipped": skipped}
}

func TestManifestCases(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "pack-manifest-cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var file struct {
		Cases []manifestCase `json:"cases"`
	}
	if err := json.Unmarshal(data, &file); err != nil {
		t.Fatal(err)
	}
	if len(file.Cases) < 150 {
		t.Fatalf("случаев %d: файл урезан", len(file.Cases))
	}
	names := map[string]bool{}
	for _, c := range file.Cases {
		if names[c.Name] {
			t.Errorf("случай «%s» повторяется", c.Name)
		}
		names[c.Name] = true
		if len(c.Want) == 0 {
			t.Errorf("%s: нет want", c.Name)
			continue
		}
		got := manifestVerdict(caseInput(t, c), c.App)
		// через JSON: числа и срезы приводятся к виду файла
		raw, _ := json.Marshal(got)
		var gotAny map[string]any
		if err := json.Unmarshal(raw, &gotAny); err != nil {
			t.Fatal(err)
		}
		for k, want := range c.Want {
			have, known := gotAny[k]
			if !known {
				t.Errorf("%s: ключ want «%s» не бывает в итоге", c.Name, k)
			} else if !reflect.DeepEqual(have, want) {
				t.Errorf("%s: %s = %v, ждали %v", c.Name, k, have, want)
			}
		}
	}
}
