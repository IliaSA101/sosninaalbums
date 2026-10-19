package spreads_test

// Скорость разнообразия (design 041 §10): сборка 1500 фото на 100 разворотов + первый View —
// «Обычное» ≤ 1.3× «Меньше» на холодном кэше; под замком Editor (тёплый кэш) ≤ 100 мс.

import (
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/spreads"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/layout/variety"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

const benchPhotos, benchSpreads = 1500, 100

func benchOpts(v model.Variety) model.AutoOpts {
	return model.AutoOpts{Spreads: benchSpreads, MaxPer: 16, Variety: v}
}

// underLock — тело правки под замком (spreads.AutoBuild) на тёплом кэше: то, что держит e.mu
// сверх прежнего; план и пулы уже посчитаны Prepare (вне замка).
func underLock(tb testing.TB, ph []model.PhotoInfo, v model.Variety) time.Duration {
	tb.Helper()
	p := model.New()
	pre := spreads.Prepare(spreads.PlanOf(p, ph, nil), ph, benchOpts(v))
	return testkit.Fastest(3, func() {
		q := p.Clone()
		if made, err := spreads.AutoBuild(q, pre, -1, nil); err != nil || made != benchSpreads {
			tb.Fatalf("сборка: %d %v", made, err)
		}
	})
}

// TestAutoBuildVarietyLock: под замком «Обычное» на 1500 фото / 100 разворотов (тёплый кэш)
// укладывается в 100 мс (design §10): выбор и запись ключей, без перебора раскладок.
func TestAutoBuildVarietyLock(t *testing.T) {
	if testing.Short() {
		t.Skip("1500 фото — долгий прогрев")
	}
	ph := mixedPhotos("b", benchPhotos)
	if d := underLock(t, ph, model.VarietyNormal); d > testkit.Limit(100*time.Millisecond) {
		t.Fatalf("под замком %v", d)
	}
}

// BenchmarkAutoBuildVariety: холодный кэш — сборка и первый View по уровням; «под замком» —
// тело правки на тёплом кэше.
func BenchmarkAutoBuildVariety(b *testing.B) {
	ph := mixedPhotos("b", benchPhotos)
	for _, c := range []struct {
		name string
		v    model.Variety
	}{{"Меньше", model.VarietyLess}, {"Обычное", model.VarietyNormal}, {"Больше", model.VarietyMore}} {
		b.Run(c.name+"/холодный", func(b *testing.B) {
			for b.Loop() {
				layout.ResetCache()
				variety.ResetCache()
				e := edtest.Open(b, model.New(), "")
				if _, err := e.AutoBuild(ph, -1, benchOpts(c.v)); err != nil {
					b.Fatal(err)
				}
				e.View()
			}
		})
	}
	b.Run("Обычное/под замком", func(b *testing.B) {
		for b.Loop() {
			underLock(b, ph, model.VarietyNormal)
		}
	})
}
