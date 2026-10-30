package grid_test

import (
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

var updateChoices = flag.Bool("update-choices", false, "перезаписать testdata/gridchoices-spread.sha")

// TestGridChoicesSpreadSame (127-11): галерея сеток разворота и листа 031 после появления рядов
// сторонки обложки (127) отдаёт прежние ряды. Эталон — хеш JSON по случаям, снят на коде до
// выделения builtinSets из builtinGrids.
func TestGridChoicesSpreadSame(t *testing.T) {
	tpls := []model.Template{
		{ID: "p3", Name: "стр 3", Scope: model.TplPage, Boxes: []model.Box{{X: 0.05, Y: 0.05, W: 0.9, H: 0.5}, {X: 0.05, Y: 0.6, W: 0.43, H: 0.35}, {X: 0.52, Y: 0.6, W: 0.43, H: 0.35}}},
		{ID: "s3", Name: "разв 3", Scope: model.TplSpread, Boxes: []model.Box{{X: 0.05, Y: 0.05, W: 0.4, H: 0.9}, {X: 0.5, Y: 0.05, W: 0.2, H: 0.9}, {X: 0.75, Y: 0.05, W: 0.2, H: 0.9}}},
	}
	var lines []string
	add := func(name string, e *album.Editor, sid string, scopes ...string) {
		for _, sc := range scopes {
			for n := 0; n <= 8; n++ {
				b, err := json.Marshal(e.GridChoices(sid, n, sc, tpls))
				if err != nil {
					t.Fatal(err)
				}
				lines = append(lines, fmt.Sprintf("%s %s n=%d %x", name, sc, n, sha256.Sum256(b)))
			}
		}
	}
	e := edtest.Open(t, model.New(), "")
	sid := edtest.NewSpread(e, -1, edtest.Photos("a", "b", "c")).Spreads[0].ID
	add("разворот", e, sid, model.ScopeSpread, model.ScopeLeft, model.ScopeRight)
	p := model.New()
	p.Settings.Sheet, p.Settings.PageW, p.Settings.PageH = true, 304, 203
	e2 := edtest.Open(t, p, "")
	sid2 := edtest.NewSpread(e2, -1, edtest.Photos("a", "b", "c", "d")).Spreads[0].ID
	add("лист 031", e2, sid2, model.ScopeSpread)

	got := strings.Join(lines, "\n") + "\n"
	const path = "testdata/gridchoices-spread.sha"
	if *updateChoices {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(want) != got {
		t.Errorf("ряды галереи разворота изменились:\n%s\nэталон:\n%s", got, want)
	}
}
