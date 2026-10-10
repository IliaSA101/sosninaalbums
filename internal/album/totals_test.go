package album_test

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/grid"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// fullTexts — редактор, где тексты заняли общий запас: n разворотов по MaxTexts
// блоков в runes знаков (последний разворот — пустой, для AddText).
func fullTexts(t *testing.T, spreads, runes int) *album.Editor {
	t.Helper()
	e := edtest.Open(t, model.New(), "")
	edtest.Change(e, func(p *model.Project) {
		for range spreads {
			s := &model.Spread{ID: model.NewID()}
			for range model.MaxTexts {
				s.Texts = append(s.Texts, &model.TextBlock{ID: model.NewID(), Auto: true, Text: strings.Repeat("я", runes), Style: model.PlainStyle()})
			}
			p.Spreads = append(p.Spreads, s)
		}
		p.Spreads = append(p.Spreads, &model.Spread{ID: model.NewID(), Photos: []string{}})
	})
	if len(edtest.P(e).Spreads) != spreads+1 || len(edtest.P(e).Spreads[0].Texts) != model.MaxTexts {
		t.Fatalf("заготовка: %d разворотов", len(edtest.P(e).Spreads))
	}
	return e
}

func TestTotalTextsCountLimit(t *testing.T) {
	e := fullTexts(t, model.MaxTotalTexts/model.MaxTexts, 1) // ровно в запас
	last := edtest.P(e).Spreads[len(edtest.P(e).Spreads)-1].ID
	rev := edtest.Rev(e)
	v, err := e.AddText(last, 0, "")
	if err == nil || edtest.Rev(e) != rev || len(v.Spreads[len(v.Spreads)-1].Texts) != 0 {
		t.Fatalf("блок сверх запаса: err=%v rev %d→%d", err, rev, edtest.Rev(e))
	}
	// Запас, который Load оставит, не теряется: файл с полным запасом открывается целиком.
	path := filepath.Join(testkit.TempDir(t), "f.salbum")
	if err := e.Save(path); err != nil {
		t.Fatal(err)
	}
	q, err := salbum.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, s := range q.Spreads {
		n += len(s.Texts)
	}
	if n != model.MaxTotalTexts {
		t.Fatalf("после Save+Load %d блоков из %d", n, model.MaxTotalTexts)
	}
	// Освободили место — добавление проходит.
	e.DeleteText(edtest.P(e).Spreads[0].Texts[0].ID)
	if _, err := e.AddText(last, 0, ""); err != nil {
		t.Fatal(err)
	}
}

func TestTotalTextsRunesLimit(t *testing.T) {
	e := fullTexts(t, model.MaxTotalRunes/model.MaxTextRunes/model.MaxTexts, model.MaxTextRunes) // все знаки заняты
	last := edtest.P(e).Spreads[len(edtest.P(e).Spreads)-1].ID
	if _, err := e.AddText(last, 0, ""); err == nil {
		t.Fatal("блок сверх запаса знаков принят")
	}
}

func TestTotalTextsCaptionsCount(t *testing.T) {
	e := fullTexts(t, model.MaxTotalTexts/model.MaxTexts, 1)
	edtest.NewSpread(e, -1, edtest.Photos("a"))
	rev := edtest.Rev(e)
	e.SetCaption("a", "подпись", "")
	if edtest.Rev(e) != rev {
		t.Fatal("подпись сверх запаса принята: Load её выбросил бы")
	}
}

func TestTotalTextsDuplicateGridDesign(t *testing.T) {
	e := edtest.Open(t, model.New(), "")
	edtest.NewSpread(e, -1, edtest.Photos("a", "b"))
	sid := edtest.P(e).Spreads[0].ID
	edtest.FreezeAt(t, e, 0)
	edtest.Change(e, func(p *model.Project) {
		for range model.MaxTexts {
			p.Spreads[0].Texts = append(p.Spreads[0].Texts, &model.TextBlock{ID: model.NewID(), Auto: true, Text: "т", Style: model.PlainStyle()})
		}
	})
	rev := edtest.Rev(e)
	// 20 копий × 20 блоков = 400 + 20 — в запасе; 100 копий × 20 = 2020 — сверх.
	if _, err := e.DuplicateGrid(sid, grid.MaxDuplicate, true); err == nil || edtest.Rev(e) != rev {
		t.Fatalf("размножение сверх запаса: err=%v", err)
	}
	if _, err := e.DuplicateGrid(sid, 20, true); err != nil {
		t.Fatal(err)
	}
}

func TestTotalSpreadsLimit(t *testing.T) {
	e := edtest.Open(t, model.New(), "")
	edtest.Change(e, func(p *model.Project) {
		for range model.MaxSpreads {
			p.Spreads = append(p.Spreads, &model.Spread{ID: model.NewID(), Photos: []string{}})
		}
	})
	rev := edtest.Rev(e)
	if _, err := e.NewSpread(-1, edtest.Photos("a")); err == nil {
		t.Fatal("разворот сверх MaxSpreads создан")
	}
	e.AutoBuild(edtest.Photos("a", "b"), -1, 2)
	if len(edtest.P(e).Spreads) != model.MaxSpreads || edtest.Rev(e) != rev {
		t.Fatalf("AutoBuild сверх MaxSpreads: %d разворотов", len(edtest.P(e).Spreads))
	}
	// Проект с ровно MaxSpreads: деление дало бы лишний разворот — отказ.
	edtest.Change(e, func(p *model.Project) { p.Spreads = p.Spreads[:model.MaxSpreads-1] })
	v := edtest.NewSpread(e, -1, edtest.Photos("a", "b"))
	sid := v.Spreads[len(v.Spreads)-1].ID
	rev = edtest.Rev(e)
	e.SplitSpread(sid, 1)
	if len(edtest.P(e).Spreads) != model.MaxSpreads || edtest.Rev(e) != rev {
		t.Fatalf("SplitSpread сверх MaxSpreads: %d разворотов", len(edtest.P(e).Spreads))
	}
}

// Гигантские числа в блоке текста не дают Inf/NaN: View сериализуется (design 008 §2.3).
func TestLoadHugeTextBoxView(t *testing.T) {
	for _, body := range []string{
		`{"x":1.7e308,"y":1.7e308,"w":1.7e308,"rot":45,"auto":true,"text":"т"}`,
		`{"x":-1.7e308,"y":1.7e308,"w":1.7e308,"rot":30,"text":"т"}`,
		`{"x":1e300,"y":-1e300,"w":1e300,"rot":90,"text":"т"}`,
	} {
		path := filepath.Join(testkit.TempDir(t), "f.salbum")
		data := `{"version":3,"spreads":[{"id":"s","texts":[` + body + `]}]}`
		if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
		p, err := salbum.Load(path)
		if err != nil {
			t.Fatalf("%s: %v", body, err)
		}
		for _, tb := range p.Spreads[0].Texts {
			for _, v := range []float64{tb.X, tb.Y, tb.W} {
				if math.IsInf(v, 0) || math.IsNaN(v) {
					t.Fatalf("%s: блок %+v", body, tb)
				}
			}
		}
		if _, err := json.Marshal(edtest.Open(t, p, "").View()); err != nil {
			t.Fatalf("%s: View не сериализуется: %v", body, err)
		}
	}
}

// Переполненный проект не «замерзает»: удаление разворота и правки, не
// добавляющие разворотов, проходят; рост числа разворотов — отказ.
func TestCheckTotalsOverfullNotFrozen(t *testing.T) {
	e := edtest.Open(t, model.New(), "")
	edtest.P(e).Spreads = nil
	for range model.MaxSpreads + 3 {
		edtest.P(e).Spreads = append(edtest.P(e).Spreads, &model.Spread{ID: model.NewID(), Photos: []string{}})
	}
	if _, err := e.NewSpread(-1, edtest.Photos("a")); err == nil {
		t.Fatal("рост сверх предела должен отказать")
	}
	rev := edtest.Rev(e)
	first := edtest.P(e).Spreads[0].ID
	v := e.DeleteSpread(first)
	if len(v.Spreads) != model.MaxSpreads+2 || edtest.Rev(e) == rev {
		t.Fatalf("DeleteSpread в переполненном проекте не прошёл: %d разворотов", len(v.Spreads))
	}
	// Правка без изменения числа разворотов тоже проходит.
	s := model.DefaultSettings()
	s.PageW++
	rev = edtest.Rev(e)
	e.SetSettings(s)
	if edtest.Rev(e) == rev {
		t.Fatal("SetSettings в переполненном проекте не прошёл")
	}
}
