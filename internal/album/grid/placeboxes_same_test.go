package grid_test

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

var updatePlaceBoxes = flag.Bool("update-placeboxes", false, "перезаписать testdata/placeboxes-spread.txt")

// TestPlaceBoxesSpreadSame (132-3): «Сохранить как шаблон» разворота и листа 031 после ветки обложки
// (0.27-132) отдаёт прежние доли, область и тексты ошибок. Эталон снят на коде до неё.
func TestPlaceBoxesSpreadSame(t *testing.T) {
	var lines []string
	add := func(name string, e interface {
		PlaceBoxes(string, string) ([]model.Box, string, error)
	}, sid string, scopes ...string) {
		for _, sc := range scopes {
			boxes, tplScope, err := e.PlaceBoxes(sid, sc)
			b, jerr := json.Marshal(boxes)
			if jerr != nil {
				t.Fatal(jerr)
			}
			lines = append(lines, fmt.Sprintf("%s %q %s %s %v", name, sc, tplScope, b, err))
		}
	}
	scopes := []string{model.ScopeSpread, model.ScopeLeft, model.ScopeRight, model.ScopeSpine, "", "всё"}
	e := edtest.Open(t, model.New(), "")
	sid := edtest.NewSpread(e, -1, edtest.Photos("a", "b", "c")).Spreads[0].ID
	add("разворот", e, sid, scopes...)
	add("нет такого", e, "нет", model.ScopeSpread, model.ScopeLeft)
	edtest.Change(e, func(p *model.Project) { // одно место: на другой странице мест нет
		p.Freeze(p.Spreads[0])
		p.Spreads[0].Grid = p.Spreads[0].Grid[:1]
		p.Spreads[0].Photos = nil
	})
	add("одно место", e, sid, scopes...)
	edtest.Change(e, func(p *model.Project) { p.Spreads[0].Grid = nil })
	add("пустой", e, sid, scopes...)

	p := model.New()
	p.Settings.Sheet, p.Settings.PageW, p.Settings.PageH = true, 304, 203
	e2 := edtest.Open(t, p, "")
	sid2 := edtest.NewSpread(e2, -1, edtest.Photos("a", "b", "c", "d")).Spreads[0].ID
	add("лист 031", e2, sid2, scopes...)

	got := strings.Join(lines, "\n") + "\n"
	const path = "testdata/placeboxes-spread.txt"
	if *updatePlaceBoxes {
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
		t.Errorf("PlaceBoxes разворота изменился:\n%s\nэталон:\n%s", got, want)
	}
}
