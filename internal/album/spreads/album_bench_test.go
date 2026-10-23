package spreads_test

// Стоимость «Весь альбом заново» (107, design §8): пересборка 200 разворотов по 16 фото с
// оформленными якорями — не дольше 1,2 × обычной сборки тех же фото в пустой альбом (холодный кэш
// раскладок в обоих случаях): стены только сужают перебор, лишнего счёта нет.

import (
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/layout/variety"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

const albumBenchSpreads, albumBenchPer, albumBenchAnchors = 200, 16, 10

func albumBenchOpts(n int, album bool) model.AutoOpts {
	return model.AutoOpts{Spreads: n, MaxPer: albumBenchPer, Variety: model.VarietyNormal, Album: album}
}

// albumBenchBase — собранный альбом 200 × 16 с оформленными разворотами (каждый 20-й) и фото
// его голых разворотов (пул) по порядку.
func albumBenchBase(tb testing.TB) (base []byte, feed, pool []model.PhotoInfo) {
	tb.Helper()
	feed = mixedPhotos("b", albumBenchSpreads*albumBenchPer)
	e := edtest.Open(tb, model.New(), "")
	if _, err := e.AutoBuild(feed, -1, albumBenchOpts(albumBenchSpreads, false)); err != nil {
		tb.Fatal(err)
	}
	edtest.Change(e, func(p *model.Project) {
		for i := 0; i < len(p.Spreads); i += albumBenchSpreads / albumBenchAnchors {
			p.Spreads[i].Name = "Оформлен"
		}
	})
	p := edtest.P(e)
	in := map[string]bool{}
	for _, s := range p.Spreads {
		if s.Name == "" {
			for _, id := range s.Photos {
				in[id] = true
			}
		}
	}
	for _, ph := range feed {
		if in[ph.ID] {
			pool = append(pool, ph)
		}
	}
	return model.Marshal(p), feed, pool
}

// albumBenchRun — одна холодная сборка: пересборка альбома из base (album) или обычная сборка
// пула в пустой альбом; время — только AutoBuild.
func albumBenchRun(tb testing.TB, base []byte, feed []model.PhotoInfo, rebuild bool) time.Duration {
	tb.Helper()
	layout.ResetCache()
	variety.ResetCache()
	var e *album.Editor
	n := albumBenchSpreads - albumBenchAnchors
	if rebuild {
		e = edtest.Open(tb, model.Unmarshal(base), "")
	} else {
		e = edtest.Open(tb, model.New(), "")
	}
	start := time.Now()
	v, err := e.AutoBuild(feed, -1, albumBenchOpts(n, rebuild))
	d := time.Since(start)
	if err != nil {
		tb.Fatal(err)
	}
	if got := len(v.Spreads); (rebuild && got != albumBenchSpreads) || (!rebuild && got != n) {
		tb.Fatalf("разворотов %d (пересборка %v)", got, rebuild)
	}
	return d
}

// albumBenchPair — пара соседних по времени прогонов: сборка и пересборка альбома. Прогоны
// чередуются, так что всплеск нагрузки на машине попадает в оба времени пары, а не в один ряд.
type albumBenchPair struct{ build, rebuild time.Duration }

func albumBenchPairs(tb testing.TB, base []byte, feed, pool []model.PhotoInfo) []albumBenchPair {
	tb.Helper()
	pairs := make([]albumBenchPair, 5)
	for i := range pairs {
		pairs[i].build = albumBenchRun(tb, nil, pool, false)
		pairs[i].rebuild = albumBenchRun(tb, base, feed, true)
	}
	return pairs
}

// TestAutoAlbumCost: пересборка альбома ≤ 1,2 × сборки тех же фото (порог растёт с занятостью
// машины, testkit.Limit). Сравнивается внутри пары, а не «лучшее с лучшим»: на двухъядерном
// раннере CI тихая секунда доставалась только сборке, и лучшее время пересборки уже не
// догоняло (1,28×); настоящий лишний счёт медленнее 1,2× в каждой паре.
func TestAutoAlbumCost(t *testing.T) {
	if testing.Short() {
		t.Skip("3200 фото на холодном кэше — не быстрый уровень")
	}
	base, feed, pool := albumBenchBase(t)
	if len(pool) != (albumBenchSpreads-albumBenchAnchors)*albumBenchPer {
		t.Fatalf("пул %d фото", len(pool))
	}
	pairs := albumBenchPairs(t, base, feed, pool)
	best := pairs[0]
	for _, p := range pairs {
		if lim := testkit.Limit(p.build * 12 / 10); p.rebuild <= lim {
			t.Logf("сборка %v, пересборка альбома %v (%.2f×)", p.build, p.rebuild, float64(p.rebuild)/float64(p.build))
			return
		}
		if float64(p.rebuild)/float64(p.build) < float64(best.rebuild)/float64(best.build) {
			best = p
		}
	}
	t.Fatalf("пересборка альбома дольше порога в каждой из %d пар; лучшая: пересборка %v, сборка %v, порог %v",
		len(pairs), best.rebuild, best.build, testkit.Limit(best.build*12/10))
}

// BenchmarkAutoAlbum: сборка пула в пустой альбом и пересборка альбома 200 × 16 с 10 якорями.
func BenchmarkAutoAlbum(b *testing.B) {
	base, feed, pool := albumBenchBase(b)
	for _, c := range []struct {
		name    string
		rebuild bool
		feed    []model.PhotoInfo
		base    []byte
	}{{"сборка", false, pool, nil}, {"альбом", true, feed, base}} {
		b.Run(c.name, func(b *testing.B) {
			var sum time.Duration
			for b.Loop() {
				sum += albumBenchRun(b, c.base, c.feed, c.rebuild)
			}
			b.ReportMetric(float64(sum.Milliseconds())/float64(b.N), "мс/AutoBuild")
		})
	}
}
