package variety

import (
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// featCand — синтетическая раскладка генератора: 12 фото, главное big, сырая оценка и Fair.
func featCand(key string, score, fair float64, big int, k Kind) Cand {
	c := cand(key, "feat-"+key, score, false)
	c.Rects = make([]layout.Rect, 12)
	c.Family, c.Big, c.Heroes, c.Fair, c.Kind = Hero, big, 1, fair, k
	return c
}

// rowBest — лучшая ряда «Авто» на 12 фото (оценка 0.9: потолок допуска — heroCap 0.72).
func rowBest() Cand {
	c := cand("best", "b", 0.9, true)
	c.Rects, c.Fair = make([]layout.Rect, 12), 0.9
	return c
}

// TestPickFeatured (решения №30, №34): раскладка с главным фото — по ритму в допуске δ_hero по Fair
// от min(лучшая, 0.72); не после разворота с главным; «даром» — при сырой потере ≤ 0.02; при звезде —
// только если главное она, и тогда в допуске δ★ по Fair и после главного тоже; вид соседа — никогда.
func TestPickFeatured(t *testing.T) {
	due := Ctx{SinceHero: 3} // 4-й без главного — пора у «Обычного»
	for _, c := range []struct {
		name string
		lv   Level
		ctx  Ctx
		alt  Cand
		star int
		want int
	}{
		{"по ритму: Fair ровно δ_hero", Normal, due, featCand("f", 0.4, 0.64, 3, KindPage), -1, 1},
		{"по ритму: Fair чуть ниже", Normal, due, featCand("f", 0.4, 0.6399, 3, KindPage), -1, 0},
		{"Больше: δ_hero 0.12, период 3", More, Ctx{SinceHero: 2}, featCand("f", 0.4, 0.60, 3, KindPage), -1, 1},
		{"Больше: рано (2-й)", More, Ctx{SinceHero: 1}, featCand("f", 0.4, 0.60, 3, KindPage), -1, 0},
		{"рано (3-й) — не по ритму", Normal, Ctx{SinceHero: 2}, featCand("f", 0.4, 0.72, 3, KindPage), -1, 0},
		{"после главного — нет", Normal, Ctx{SinceHero: 3, Hero: true}, featCand("f", 0.4, 0.72, 3, KindPage), -1, 0},
		{"«даром»: сырая потеря 0.02 (лучшая — вид соседа)", Normal, Ctx{Prev: []string{"b"}}, featCand("f", 0.88, 0.95, 3, KindPage), -1, 1},
		{"«даром»: сырая потеря больше", Normal, Ctx{Prev: []string{"b"}}, featCand("f", 0.8799, 0.95, 3, KindPage), -1, 0},
		{"звезда — главное, δ★ по Fair", Normal, Ctx{Hero: true}, featCand("f", 0.3, 0.62, 5, KindJoint), 5, 1},
		{"звезда — чуть дальше δ★", Normal, Ctx{}, featCand("f", 0.3, 0.6199, 5, KindJoint), 5, 0},
		{"звезда есть, главное другое", Normal, due, featCand("f", 0.4, 0.72, 3, KindJoint), 5, 0},
		{"вид предыдущего — даже со звездой", Normal, Ctx{Prev: []string{"feat-f"}}, featCand("f", 0.4, 0.72, 5, KindJoint), 5, 0},
		{"вид следующего", Normal, Ctx{SinceHero: 3, Next: "feat-f"}, featCand("f", 0.4, 0.72, 3, KindPage), -1, 0},
		{"«Меньше» — никогда", Less, due, featCand("f", 0.9, 0.95, 3, KindPage), -1, 0},
	} {
		if got := pickOne(c.lv, c.ctx, Spread{Pool: []Cand{rowBest(), c.alt}, Star: c.star}); got != c.want {
			t.Errorf("%s: выбран %d, ждали %d", c.name, got, c.want)
		}
	}
}

// TestPickHeroAndAir: оба по ритму — воздух, если главное ждёт не дольше; главное, если дольше
// (у «Больше» периоды равны, иначе главное не доставалось бы никогда).
func TestPickHeroAndAir(t *testing.T) {
	air := airCand("a", 0.6, 0.45, 0.2*geomSq.PageH)
	air.Rects = make([]layout.Rect, 12)
	pool := []Cand{rowBest(), air, featCand("f", 0.4, 0.70, 3, KindPage)}
	for _, c := range []struct {
		name string
		ctx  Ctx
		want int
	}{
		{"ждут поровну — воздух", Ctx{Since: 4, SinceHero: 4}, 1},
		{"главное ждёт дольше — главное", Ctx{Since: 4, SinceHero: 5}, 2},
		{"воздух не пора — главное", Ctx{Since: 1, SinceHero: 3}, 2},
		// bonusHero (главное ждёт не дольше воздуха, воздух не пора): бонус есть и сильнее повтора
		// вида позапрошлого (penSame2)
		{"главное не дольше, воздух не пора — главное", Ctx{Since: 3, SinceHero: 3}, 2},
		{"то же, вид как у позапрошлого — главное", Ctx{Since: 3, SinceHero: 3, Prev: []string{"feat-f", "x"}}, 2},
	} {
		if got := pickOne(Normal, c.ctx, Spread{Pool: pool, Star: -1}); got != c.want {
			t.Errorf("%s: выбран %d, ждали %d", c.name, got, c.want)
		}
	}
}

// TestPickHeroChain: на цепочке — главное по ритму, не два подряд, семейство чередуется, виды,
// бывшие раньше позапрошлого, — дороже невиданных.
func TestPickHeroChain(t *testing.T) {
	mk := func() Spread {
		page, joint := featCand("p", 0.4, 0.70, 0, KindPage), featCand("j", 0.4, 0.70, 5, KindJoint)
		return Spread{Pool: []Cand{rowBest(), page, joint}, Star: -1}
	}
	chain := make([]Spread, 9)
	for i := range chain {
		chain[i] = mk()
	}
	got := Pick(chain, geomSq, Normal, Ctx{})
	var heroes []int
	for i, k := range got {
		if chain[i].Pool[k].Kind != NoKind {
			heroes = append(heroes, i)
		}
	}
	// 0–2 — без главного, 3 — главное, 4–6 без, 7 — главное
	if len(heroes) != 2 || heroes[0] != 3 || heroes[1] != 7 {
		t.Fatalf("главное на %v, ждали [3 7] (выбор %v)", heroes, got)
	}
	if chain[3].Pool[got[3]].Kind == chain[7].Pool[got[7]].Kind {
		t.Errorf("семейство главного не сменилось: %v", got)
	}
	// penSeen: лучший вид был раньше позапрошлого, равный по оценке невиданный — берётся он
	seen := Spread{Pool: []Cand{cand("a", "A", 1.0, true), cand("b", "B", 1.0, true)}, Star: -1}
	other := Spread{Pool: []Cand{cand("c", "C", 1.0, true)}, Star: -1}
	if k := Pick([]Spread{seen, other, other, seen}, geomSq, Normal, Ctx{})[3]; k != 1 {
		t.Errorf("вид, бывший раньше, не оштрафован: выбран %d", k)
	}
}

// TestPickSameKey (QA 0.11, 127 живых фото): тот же ключ, что у предыдущего, при другом виде
// (пропорции чуть другие) — штраф: из равных берётся другая схема; в цепочке и от Ctx.PrevKey.
func TestPickSameKey(t *testing.T) {
	mk := func(look string) Spread {
		return Spread{Pool: []Cand{cand("K", look+"1", 1.0, true), cand("L", look+"2", 0.99, true)}, Star: -1}
	}
	if got := pickOne(Normal, Ctx{PrevKey: "K"}, mk("a")); got != 1 {
		t.Errorf("ключ соседа из Ctx: выбран %d", got)
	}
	if got := pickOne(Normal, Ctx{}, mk("a")); got != 0 {
		t.Errorf("без соседа — лучшая: выбран %d", got)
	}
	if got := Pick([]Spread{mk("a"), mk("b")}, geomSq, Normal, Ctx{}); got[0] != 0 || got[1] != 1 {
		t.Errorf("цепочка: %v, ждали [0 1]", got)
	}
	if got := Pick([]Spread{mk("a"), mk("b")}, geomSq, Less, Ctx{PrevKey: "K"}); got[0] != 0 || got[1] != 0 {
		t.Errorf("«Меньше» соседей не смотрит: %v", got)
	}
}
