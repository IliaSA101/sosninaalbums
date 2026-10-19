package similar

import (
	"fmt"
	"image"
	"math/bits"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/library/imgsig"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// near — подпись, у которой Dist до near(0) равен k (k младших бит dHash).
func near(k int) imgsig.Sig {
	return imgsig.Sig{D: uint64(1)<<k - 1, Ok: true, Sharp: 10}
}

// far — «чужая» подпись номер k: далеко (Dist > 40) от любой другой far и от near.
func far(t *testing.T, k int) imgsig.Sig {
	t.Helper()
	x := uint64(k+1) * 0x9e3779b97f4a7c15
	mix := func(z uint64) uint64 {
		z = (z ^ z>>30) * 0xbf58476d1ce4e5b9
		z = (z ^ z>>27) * 0x94d049bb133111eb
		return z ^ z>>31
	}
	s := imgsig.Sig{D: mix(x), P: mix(x + 1), Ok: true, Sharp: 10}
	if d := imgsig.Dist(s, near(0)); d < 40 {
		t.Fatalf("far(%d) слишком близко к near(0): %d", k, d)
	}
	return s
}

// shot — фото номер i в одной папке, 3:2.
func shot(i int, s imgsig.Sig) model.PhotoInfo {
	return model.PhotoInfo{ID: fmt.Sprintf("p%02d", i), Path: fmt.Sprintf("/съёмка/a/%02d.jpg", i), W: 300, H: 200, Sig: s}
}

// shots — подписи подряд как фото 0…n-1.
func shots(sigs ...imgsig.Sig) []model.PhotoInfo {
	out := make([]model.PhotoInfo, len(sigs))
	for i, s := range sigs {
		out[i] = shot(i, s)
	}
	return out
}

// pairs — Drops как пары «убран → на кого похож».
func pairs(d []Drop) map[int]int {
	out := map[int]int{}
	for _, x := range d {
		out[x.I] = x.Like
	}
	return out
}

func wantPairs(t *testing.T, got []Drop, want map[int]int) {
	t.Helper()
	if g := pairs(got); !reflect.DeepEqual(g, want) || len(got) != len(want) {
		t.Fatalf("убрано %v, ждали %v", got, want)
	}
}

// TestDropsLevels — пороги уровней точно по imgsig: 12 / 24 / 34 включительно; 0 и меньше — отбора нет;
// выше 3 зажимается; один кадр и пусто — ничего.
func TestDropsLevels(t *testing.T) {
	for _, c := range []struct {
		dist, lv int
		drop     bool
	}{
		{12, 1, true}, {13, 1, false}, {24, 2, true}, {25, 2, false}, {13, 2, true},
		{34, 3, true}, {35, 3, false}, {25, 3, true}, {0, 1, true}, {0, 3, true},
		{20, 1, false}, {30, 2, false}, {40, 3, false},
	} {
		got := Drops(shots(near(0), near(c.dist)), c.lv, nil, Marks{})
		if (len(got) == 1) != c.drop {
			t.Errorf("Dist %d, уровень %d: убрано %v, ждали убрать=%t", c.dist, c.lv, got, c.drop)
		}
	}
	pair := shots(near(0), near(0))
	for _, lv := range []int{0, -1, -100} {
		if got := Drops(pair, lv, nil, Marks{}); got != nil {
			t.Errorf("уровень %d: %v, ждали nil", lv, got)
		}
	}
	if got := Drops(shots(near(0), near(34)), 99, nil, Marks{}); len(got) != 1 {
		t.Errorf("уровень 99 должен зажаться в 3: %v", got)
	}
	if Drops(nil, 3, nil, Marks{}) != nil || Drops(shots(near(0)), 3, nil, Marks{}) != nil {
		t.Error("пусто и один кадр: ждали nil")
	}
}

// TestDropsKeepsBest — из пары остаётся лучший по порядку: флажок, звёзды, уже в альбоме, резкость,
// раньше; каждый довод решает, когда выше стоящих нет.
func TestDropsKeepsBest(t *testing.T) {
	a, b := shot(0, near(0)), shot(1, near(3))
	for _, c := range []struct {
		name string
		mark Marks
		tune func(x, y *model.PhotoInfo)
		keep int // кого оставить; второй уходит
	}{
		{"раньше", Marks{}, nil, 0},
		{"резче", Marks{}, func(x, y *model.PhotoInfo) { y.Sig.Sharp = 20 }, 1},
		{"резкость пустяк — раньше", Marks{}, func(x, y *model.PhotoInfo) { x.Sig.Sharp, y.Sig.Sharp = 5, 5.0004 }, 0},
		{"в альбоме важнее резкости", Marks{InAlbum: []bool{false, true}}, func(x, y *model.PhotoInfo) { x.Sig.Sharp = 99 }, 1},
		{"звёзды важнее альбома", Marks{Rate: []int{0, 3}, InAlbum: []bool{true, false}}, nil, 1},
		{"флажок важнее звёзд", Marks{Rate: []int{3, 0}, Pick: []bool{false, true}}, nil, 1},
		{"звёзды 1 против 2", Marks{Rate: []int{2, 1}}, nil, 0},
	} {
		x, y := a, b
		if c.tune != nil {
			c.tune(&x, &y)
		}
		got := Drops([]model.PhotoInfo{x, y}, 1, nil, c.mark)
		lose := 1 - c.keep
		if len(got) != 1 || got[0].I != lose || got[0].Like != c.keep || got[0].Dist != 3 {
			t.Errorf("%s: %v, ждали убрать %d (похож на %d, Dist 3)", c.name, got, lose, c.keep)
		}
	}
	// Одно фото из серии в пять: остаётся ровно лучшее по резкости, остальные ссылаются на него.
	in := shots(near(0), near(1), near(2), near(3), near(4))
	in[3].Sig.Sharp = 50
	got := Drops(in, 1, nil, Marks{})
	if len(got) != 4 {
		t.Fatalf("убрано %d из 5: %v", len(got), got)
	}
	for _, d := range got {
		if d.I == 3 || d.Like != 3 {
			t.Errorf("убран %d, похож на %d: ждали, что останется только 3", d.I, d.Like)
		}
	}
}

// TestDropsWindowAndTime — сравниваются кадры не дальше 12 позиций и (при времени у обоих) 10 минут.
func TestDropsWindowAndTime(t *testing.T) {
	sigs := []imgsig.Sig{near(0)}
	for k := 1; k <= 12; k++ {
		sigs = append(sigs, far(t, k))
	}
	for _, c := range []struct {
		gap  int // позиций между двойниками
		drop bool
	}{{12, true}, {13, false}} {
		in := shots(append(slices.Clone(sigs[:c.gap]), near(0))...) // двойник — через gap позиций от первого
		if got := Drops(in, 1, nil, Marks{}); (len(got) == 1) != c.drop {
			t.Errorf("через %d позиций: %v, ждали убрать=%t", c.gap, got, c.drop)
		}
	}
	// Время: шаг 400 с — соседние (400) сравниваются, через одного (800) нет; без времени у одного — по окну.
	timed := func(taken ...int64) []model.PhotoInfo {
		in := shots(near(0), far(t, 1), near(0), far(t, 2), near(0))
		for i, ts := range taken {
			in[i].Taken = ts
		}
		return in
	}
	const t0 = 1_700_000_000
	if got := Drops(timed(t0, t0+400, t0+800, t0+1200, t0+1600), 1, nil, Marks{}); len(got) != 0 {
		t.Errorf("двойники через 800 с: %v, ждали без отбора", got)
	}
	if got := Drops(timed(t0, t0+100, t0+600, t0+700, t0+1300), 1, nil, Marks{}); len(got) != 1 || got[0].I != 2 || got[0].Like != 0 {
		t.Errorf("двойники через 600 с (граница, в окне): %v", got)
	}
	if got := Drops(timed(t0, t0+100, 0, t0+700, t0+1300), 1, nil, Marks{}); len(got) == 0 {
		t.Error("без времени у одного из пары сравнивают по окну позиций")
	}
}

// TestDropsSeriesWall — через границу серии (смена папки) не сравнивают; внутри папки — да;
// несколько серий сразу.
func TestDropsSeriesWall(t *testing.T) {
	in := shots(near(0), near(1), near(0), near(1))
	for i := 2; i < 4; i++ {
		in[i].Path = fmt.Sprintf("/съёмка/b/%02d.jpg", i)
	}
	// 0,1 — папка a; 2,3 — папка b: пары внутри папок, между ними стена. Лучшие — 0 и 2.
	wantPairs(t, Drops(in, 1, nil, Marks{}), map[int]int{1: 0, 3: 2})
	// Те же кадры в одной папке: из четырёх остаётся один.
	if got := Drops(shots(near(0), near(1), near(0), near(1)), 1, nil, Marks{}); len(got) != 3 {
		t.Errorf("одна папка: убрано %d из 4, ждали 3", len(got))
	}
	// Долгая пауза внутри папки — тоже стена (сила 1 от 20 минут).
	in = shots(near(0), near(0), near(0), near(0))
	for i, s := range []int64{0, 10, 20 + 3000, 30 + 3000} {
		in[i].Taken = 1_700_000_000 + s
	}
	wantPairs(t, Drops(in, 3, nil, Marks{}), map[int]int{1: 0, 3: 2})
}

// TestDropsOrientation — вертикальный и горизонтальный кадры друг с другом не сравниваются;
// пропорции Marks.Asp (с поворотом) главнее W/H.
func TestDropsOrientation(t *testing.T) {
	in := shots(near(0), near(2))
	in[1].W, in[1].H = 200, 300
	if got := Drops(in, 3, nil, Marks{}); len(got) != 0 {
		t.Errorf("горизонталь и вертикаль: %v", got)
	}
	in[1].W, in[1].H = 300, 200 // оба горизонтальные по файлу…
	if got := Drops(in, 3, nil, Marks{Asp: []float64{1.5, 1 / 1.5}}); len(got) != 0 {
		t.Errorf("второй повёрнут: %v", got)
	}
	if got := Drops(in, 3, nil, Marks{Asp: []float64{0.67, 0.5}}); len(got) != 1 {
		t.Errorf("обе вертикальные: %v", got)
	}
	// Квадрат — «не шире»: сравнивается с вертикалью, не с горизонталью.
	sq := shots(near(0), near(2))
	sq[0].W, sq[0].H = 100, 100
	sq[1].W, sq[1].H = 200, 300
	if got := Drops(sq, 3, nil, Marks{}); len(got) != 1 {
		t.Errorf("квадрат и вертикаль: %v", got)
	}
}

// TestDropsPickFlag — кадр с флажком «беру» не убирается, даже в паре с таким же; а его
// похожий без флажка уходит.
func TestDropsPickFlag(t *testing.T) {
	in := shots(near(0), near(1), near(2))
	wantPairs(t, Drops(in, 1, nil, Marks{Pick: []bool{true, true, false}}), map[int]int{2: 1}) // ближайший образец — 1
	// Флажок у позднего кадра с меньшей резкостью: он лучший, ранний уходит.
	in[0].Sig.Sharp = 99
	wantPairs(t, Drops(in, 1, nil, Marks{Pick: []bool{false, false, true}}), map[int]int{0: 2, 1: 2})
}

// TestDropsSkipsLibAndAlpha — свой из библиотеки и кадр без подписи (прозрачный PNG, не прочитан) не убираются
// и не служат образцом.
func TestDropsSkipsLibAndAlpha(t *testing.T) {
	in := shots(near(0), near(1), near(2))
	in[0].Lib = "my:0123456789abcdef"
	if got := Drops(in, 1, nil, Marks{}); !reflect.DeepEqual(pairs(got), map[int]int{2: 1}) {
		t.Errorf("свой из библиотеки: %v, ждали 2→1", got)
	}
	in = shots(near(0), near(1), near(2))
	in[1].Sig = imgsig.Sig{}
	if got := Drops(in, 1, nil, Marks{}); !reflect.DeepEqual(pairs(got), map[int]int{2: 0}) {
		t.Errorf("без подписи посередине: %v, ждали 2→0 (окно считает позиции)", got)
	}
}

// TestDropsNoChain — цепочки A ~ B ~ C при A ≁ C: уходит один B, C остаётся (убранный не образец).
func TestDropsNoChain(t *testing.T) {
	a := near(0)
	b := imgsig.Sig{D: 1<<10 - 1, Ok: true, Sharp: 10} // Dist(a, b) = 10
	c := imgsig.Sig{D: 1<<20 - 1, Ok: true, Sharp: 10} // Dist(b, c) = 10, Dist(a, c) = 20
	in := shots(a, b, c)
	wantPairs(t, Drops(in, 1, nil, Marks{}), map[int]int{1: 0})
	// Уровень 2 (≤ 24) склеил бы все три: A остаётся, B и C — похожи на A (C не на B).
	wantPairs(t, Drops(in, 2, nil, Marks{}), map[int]int{1: 0, 2: 0})
	// Длинная медленно меняющаяся серия (шаг 8): на уровне 1 не схлопывается в один кадр.
	var sigs []imgsig.Sig
	for k := range 10 {
		sigs = append(sigs, near(8*k))
	}
	got := Drops(shots(sigs...), 1, nil, Marks{})
	if left := len(sigs) - len(got); left < 4 {
		t.Errorf("серия шаг 8: осталось %d из 10, ждали ≥ 4 (без цепочек)", left)
	}
}

// TestDropsKeepList — возвращённые (keep) остаются; пара из двух возвращённых остаётся обоими;
// возвращённый образцом не становится: прежний образец остаётся, второй дубль — при нём (078-19).
func TestDropsKeepList(t *testing.T) {
	in := shots(near(0), near(1), near(2))
	wantPairs(t, Drops(in, 1, map[string]bool{"p01": true}, Marks{}), map[int]int{2: 0})
	if got := Drops(in, 1, map[string]bool{"p00": true, "p01": true, "p02": true}, Marks{}); len(got) != 0 {
		t.Errorf("все возвращены: %v", got)
	}
	// Незнакомый ID в keep безвреден.
	wantPairs(t, Drops(in, 1, map[string]bool{"нет такого": true}, Marks{}), map[int]int{1: 0, 2: 0})
}

// TestDropsKeepMonotone — «Вернуть» монотонно (078-19): возвращённые вычёркиваются из прежнего
// списка, остальные записи не меняются (тот же образец и Dist), никто из взятых не уходит;
// вернуть все — пусто за один раз. Несколько серий сразу, цепочки, звёзды и флажок.
func TestDropsKeepMonotone(t *testing.T) {
	var sigs []imgsig.Sig
	for g := range 5 {
		base := far(t, g*5)
		base.D &^= 1<<24 - 1
		for k := range 4 { // шаг 6 бит: на уровне 1 соседи похожи, крайние — нет (цепочка)
			s := base
			s.D |= uint64(1)<<(6*k) - 1
			sigs = append(sigs, s)
		}
		sigs = append(sigs, far(t, g*5+3))
	}
	in := shots(sigs...)
	m := Marks{Rate: make([]int, len(in)), Pick: make([]bool, len(in))}
	m.Rate[2], m.Pick[11] = 4, true // звезда и флажок посреди серий
	ids := func(ds []Drop) map[string]bool {
		out := map[string]bool{}
		for _, d := range ds {
			out[in[d.I].ID] = true
		}
		return out
	}
	for lv := 1; lv <= MaxLevel; lv++ {
		base := Drops(in, lv, nil, m)
		if len(base) < 5 {
			t.Fatalf("lv%d: отброшено %d — набор не проверяет возврат", lv, len(base))
		}
		if got := Drops(in, lv, ids(base), m); len(got) != 0 {
			t.Errorf("lv%d: вернули все %d — осталось %v", lv, len(base), got)
		}
		// Один, первые k и через один: ровно прежний список без возвращённых.
		var sets [][]Drop
		for k := range base {
			sets = append(sets, base[k:k+1], base[:k+1])
		}
		var odd []Drop
		for k := 0; k < len(base); k += 2 {
			odd = append(odd, base[k])
		}
		sets = append(sets, odd)
		for _, back := range sets {
			keep := ids(back)
			keep["нет такого"] = true
			keep[in[4].ID] = true // и взятый (чужой кадр между сериями): ничего не меняет
			var want []Drop
			for _, d := range base {
				if !keep[in[d.I].ID] {
					want = append(want, d)
				}
			}
			if got := Drops(in, lv, keep, m); !reflect.DeepEqual(got, want) {
				t.Errorf("lv%d: вернули %v — отброшены %v, ждали %v", lv, back, got, want)
			}
		}
	}
}

// TestDropsStarsNeverDropped — 4–5★ остаются всегда (в том числе обе в паре); 3★ — нет.
func TestDropsStarsNeverDropped(t *testing.T) {
	for _, stars := range []int{4, 5} {
		in := shots(near(0), near(0), near(0))
		wantPairs(t, Drops(in, 3, nil, Marks{Rate: []int{stars, stars, 0}}), map[int]int{2: 0})
	}
	in := shots(near(0), near(0))
	wantPairs(t, Drops(in, 3, nil, Marks{Rate: []int{3, 3}}), map[int]int{1: 0})
	// 5★ и 4★ рядом с обычным: обычный уходит, оба остаются.
	wantPairs(t, Drops(shots(near(0), near(0), near(0)), 3, nil, Marks{Rate: []int{4, 0, 5}}), map[int]int{1: 0}) // ничья Dist — раньше
}

// TestDropsNoSig — кадры без подписи не сравниваются, не убираются; если подписей нет вовсе — пусто.
func TestDropsNoSig(t *testing.T) {
	if got := Drops(shots(imgsig.Sig{}, imgsig.Sig{}, imgsig.Sig{}), 3, nil, Marks{}); len(got) != 0 {
		t.Errorf("без подписей: %v", got)
	}
	// Нулевые хеши без Ok не считаются дубликатами друг друга.
	in := shots(near(0), imgsig.Sig{D: 0, P: 0})
	if got := Drops(in, 3, nil, Marks{}); len(got) != 0 {
		t.Errorf("Ok=false: %v", got)
	}
}

// TestDropsManySeries — несколько серий сразу: тройки дублей среди чужих кадров; результат по порядку ленты,
// без повторов, детерминированный, вход не меняется.
func TestDropsManySeries(t *testing.T) {
	var sigs []imgsig.Sig
	wantDrops := 0
	for g := range 6 {
		base := far(t, g*7)
		base.D = base.D &^ (1<<12 - 1) // младшие биты свободны для сдвигов
		for k := range 3 {
			s := base
			s.D |= uint64(1)<<k - 1 // Dist внутри тройки ≤ 2
			sigs = append(sigs, s)
		}
		wantDrops += 2
		sigs = append(sigs, far(t, g*7+3)) // чужой кадр между тройками
	}
	in := shots(sigs...)
	snapshot := slices.Clone(in)
	got := Drops(in, 1, nil, Marks{})
	if len(got) != wantDrops {
		t.Fatalf("убрано %d, ждали %d: %v", len(got), wantDrops, got)
	}
	if !slices.IsSortedFunc(got, func(a, b Drop) int { return a.I - b.I }) {
		t.Errorf("не по порядку ленты: %v", got)
	}
	seen := map[int]bool{}
	for _, d := range got {
		if seen[d.I] || d.I == d.Like || d.Dist > 12 || bits.OnesCount64(in[d.I].Sig.D^in[d.Like].Sig.D) != d.Dist {
			t.Errorf("плохая запись %+v", d)
		}
		seen[d.I] = true
		if seen[d.Like] {
			t.Errorf("образец %d сам убран (%+v)", d.Like, d)
		}
	}
	if !reflect.DeepEqual(got, Drops(in, 1, nil, Marks{})) || !reflect.DeepEqual(in, snapshot) {
		t.Error("повторный вызов дал другое или вход изменён")
	}
}

// TestDropsShortMarks — короткие и nil-срезы Marks читаются как «нет».
func TestDropsShortMarks(t *testing.T) {
	in := shots(near(0), near(1), near(2))
	m := Marks{Rate: []int{5}, Pick: []bool{}, InAlbum: []bool{false}, Asp: []float64{1.5}}
	wantPairs(t, Drops(in, 1, nil, m), map[int]int{1: 0, 2: 0})
}

// scene — тестовая «сцена» 200×133: фон-градиент и три прямоугольника; shift сдвигает их по x,
// bright добавляет яркости (экспозиция), seed меняет раскладку (другая сцена).
func scene(seed, shift, bright int) *image.Gray {
	img := image.NewGray(image.Rect(0, 0, 200, 133))
	for y := range 133 {
		for x := range 200 {
			img.Pix[y*img.Stride+x] = uint8(40 + (x+y)/4)
		}
	}
	for k := range 3 {
		x0 := (seed*37+k*61)%120 + shift
		y0 := (seed*23+k*41)%80 + 10
		for y := y0; y < y0+35; y++ {
			for x := x0; x < x0+50 && x < 200; x++ {
				img.Pix[y*img.Stride+x] = uint8(200 - 60*k)
			}
		}
	}
	for i := range img.Pix {
		img.Pix[i] = uint8(min(255, int(img.Pix[i])+bright))
	}
	return img
}

// TestDropsRealSigs — с подписями от imgsig.Of: пересвеченный и сдвинутый дубль уходит, другая сцена остаётся.
func TestDropsRealSigs(t *testing.T) {
	a, b, c, d := imgsig.Of(scene(1, 0, 0)), imgsig.Of(scene(1, 0, 25)), imgsig.Of(scene(1, 3, 0)), imgsig.Of(scene(2, 0, 0))
	in := shots(a, b, c, d)
	got := Drops(in, 1, nil, Marks{})
	t.Logf("Dist: a-b %d, a-c %d, a-d %d; убрано %v", imgsig.Dist(a, b), imgsig.Dist(a, c), imgsig.Dist(a, d), got)
	if p := pairs(got); p[1] != 0 || p[2] != 0 || len(got) != 2 {
		t.Errorf("убрано %v: ждали 1→0 и 2→0, а другая сцена (3) остаётся", got)
	}
}

// TestDropsSpeed — отбор на 3200 кадрах (серии по 4 дубля) укладывается в десятки миллисекунд:
// план после него дороже в сотни раз (спека 078 §5).
func TestDropsSpeed(t *testing.T) {
	in := make([]model.PhotoInfo, 3200)
	for i := range in {
		g := uint64(i / 4)
		s := imgsig.Sig{D: g * 0x9e3779b97f4a7c15, P: g * 0xbf58476d1ce4e5b9, Ok: true, Sharp: float32(i % 7)}
		s.D |= uint64(1)<<(i%4) - 1
		in[i] = shot(i, s)
	}
	var n int
	d := testkit.Fastest(3, func() { n = len(Drops(in, 3, nil, Marks{})) })
	t.Logf("3200 кадров: %v, убрано %d", d, n)
	if n < 2000 {
		t.Errorf("убрано %d из 3200, ждали ≥ 2000 (три из четырёх в серии)", n)
	}
	if lim := testkit.Limit(50 * time.Millisecond); d > lim {
		t.Errorf("Drops %v > %v", d, lim)
	}
}
