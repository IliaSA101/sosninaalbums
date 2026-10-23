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

// albumBenchBest — лучшее из пяти времён сборки и пересборки, прогоны чередуются: всплеск нагрузки
// на машине попадает в оба ряда, а не в один.
func albumBenchBest(tb testing.TB, base []byte, feed, pool []model.PhotoInfo) (build, rebuild time.Duration) {
	tb.Helper()
	for i := range 5 {
		if d := albumBenchRun(tb, nil, pool, false); i == 0 || d < build {
			build = d
		}
		if d := albumBenchRun(tb, base, feed, true); i == 0 || d < rebuild {
			rebuild = d
		}
	}
	return build, rebuild
}

// TestAutoAlbumCost: пересборка альбома ≤ 1,2 × сборки тех же фото (порог растёт с занятостью
// машины, testkit.Limit).
func TestAutoAlbumCost(t *testing.T) {
	if testing.Short() {
		t.Skip("3200 фото на холодном кэше — не быстрый уровень")
	}
	base, feed, pool := albumBenchBase(t)
	if len(pool) != (albumBenchSpreads-albumBenchAnchors)*albumBenchPer {
		t.Fatalf("пул %d фото", len(pool))
	}
	build, rebuild := albumBenchBest(t, base, feed, pool)
	if lim := testkit.Limit(build * 12 / 10); rebuild > lim {
		t.Fatalf("пересборка альбома %v, сборка тех же фото %v, порог %v", rebuild, build, lim)
	}
	t.Logf("сборка %v, пересборка альбома %v (%.2f×)", build, rebuild, float64(rebuild)/float64(build))
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
