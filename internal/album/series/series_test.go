package series

import (
	"fmt"
	"math"
	"path/filepath"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

const t0 = 1_700_000_000 // произвольное «сегодня» в секундах Unix

// shot — фото в папке dir, снятое через at секунд после t0 (at < 0 — времени нет).
func shot(dir string, i int, at int64) model.PhotoInfo {
	ph := model.PhotoInfo{ID: fmt.Sprintf("%s%d", dir, i), Path: filepath.Join("/съёмка", dir, fmt.Sprintf("%d.jpg", i))}
	if at >= 0 {
		ph.Taken = t0 + at
	}
	return ph
}

// run — k фото одной папки с шагом step секунд, первое — в момент from.
func run(dir string, k int, from, step int64) []model.PhotoInfo {
	out := make([]model.PhotoInfo, k)
	for i := range out {
		out[i] = shot(dir, i, from+int64(i)*step)
	}
	return out
}

// nonzero — индексы границ с ненулевой силой.
func nonzero(c []float64) []int {
	var out []int
	for j, v := range c {
		if v > 0 {
			out = append(out, j)
		}
	}
	return out
}

func TestSeriesCutsNone(t *testing.T) {
	steady := run("a", 30, 0, 10)
	noTime := run("a", 30, -1, 0)
	for name, in := range map[string][]model.PhotoInfo{
		"пусто":     nil,
		"одно фото": steady[:1],
		"одна папка, ровный темп": steady,
		"одна папка, без времени": noTime,
	} {
		if c := Cuts(in); c != nil {
			t.Errorf("%s: %v", name, c)
		}
	}
	// очередь короче 120 с не дробится, даже если она втрое длиннее соседних разрывов
	q := run("a", 20, 0, 10)
	for i := 10; i < 20; i++ {
		q[i].Taken += 100 - 10
	}
	if c := Cuts(q); c != nil {
		t.Errorf("пауза 100 с: %v", nonzero(c))
	}
	// регистр в пути — та же папка на Windows и macOS, на Linux — другая (model.SamePath, 101)
	in := []model.PhotoInfo{{ID: "1", Path: filepath.Join("/съёмка", "Венчание", "1.jpg")}, {ID: "2", Path: filepath.Join("/съёмка", "венчание", "2.jpg")}}
	if c := Cuts(in); (c == nil) != model.SamePath("Венчание", "венчание") {
		t.Errorf("папка в другом регистре: %v", c)
	}
}

func TestSeriesCutsFolder(t *testing.T) {
	in := append(run("утро", 6, 0, 10), run("церемония", 6, 70, 10)...)
	c := Cuts(in)
	if len(c) != 13 || c[0] != 0 || c[12] != 0 {
		t.Fatalf("длина и края: %v", c)
	}
	if got := nonzero(c); len(got) != 1 || got[0] != 6 || c[6] != 1 {
		t.Errorf("граница папки: %v", c)
	}
	// нет времени вовсе: папка одна — сигнал
	in = append(run("а", 4, -1, 0), run("б", 4, -1, 0)...)
	if got := nonzero(Cuts(in)); len(got) != 1 || got[0] != 4 {
		t.Errorf("папки без времени: %v", got)
	}
	// свои из библиотеки в папках не лежат: смена «папки» около них не сигнал
	in = run("а", 4, -1, 0)
	in[2].Lib = "my:0123456789abcdef"
	in[3].Path = filepath.Join("/другая", "3.jpg")
	if c := Cuts(in); c != nil {
		t.Errorf("Lib: %v", nonzero(c))
	}
}

func TestSeriesCutsPause(t *testing.T) {
	// шаг 30 с, T₀ = 120 с; пауза ×32 (960 с) — полная сила
	in := append(run("а", 10, 0, 30), run("а", 10, 300+960, 30)...)
	c := Cuts(in)
	if got := nonzero(c); len(got) != 1 || got[0] != 10 || c[10] < 0.999 {
		t.Errorf("пауза ×32: %v", c)
	}
	// пауза ×8 (240 с = 2·T₀): ln 2 / ln 8 = 1/3
	in = append(run("а", 10, 0, 30), run("а", 10, 270+240, 30)...)
	if c = Cuts(in); len(c) != 21 || math.Abs(c[10]-1.0/3) > 1e-9 {
		t.Errorf("пауза 240 с: %v", c)
	}
	// от 20 минут — граница, как бы ни был медленен темп
	in = append(run("а", 10, 0, 600), run("а", 10, 5400+1200, 600)...)
	if c = Cuts(in); c == nil || c[10] != 1 {
		t.Errorf("пауза 20 минут: %v", c)
	}
}

// Темп местный: тот же разрыв — граница среди быстрой съёмки и не граница среди медленной.
func TestSeriesCutsLocalTempo(t *testing.T) {
	gap := int64(1000)
	fast := append(run("а", 15, 0, 20), run("а", 15, 280+gap, 20)...)
	if c := Cuts(fast); c == nil || c[15] < 0.999 {
		t.Errorf("быстрая съёмка: %v", c)
	}
	slow := append(run("а", 15, 0, 300), run("а", 15, 4200+gap, 300)...)
	if c := Cuts(slow); c != nil {
		t.Errorf("медленная съёмка: %v", nonzero(c))
	}
}

func TestSeriesCutsTwoCameras(t *testing.T) {
	// две камеры вперемешку, часы второй уходят вперёд: время то растёт, то откатывается
	for _, shift := range []int64{45, 400, 3600, 7200} {
		var in []model.PhotoInfo
		for i := range int64(20) {
			in = append(in, shot("а", int(i), i*20), shot("б", int(i), i*20+shift))
		}
		// папка меняется на каждом фото — но это одна лента: сравниваем только время
		for i := range in {
			in[i].Path = filepath.Join("/съёмка", "общая", fmt.Sprint(i, ".jpg"))
		}
		if c := Cuts(in); c != nil {
			t.Errorf("сдвиг %d с: границы %v", shift, nonzero(c))
		}
	}
	// у части фото времени нет: пара без времени сигнала не даёт
	in := run("а", 10, 0, 10)
	in[5].Taken = 0
	in[6].Taken += 5000
	if c := Cuts(in); c != nil {
		t.Errorf("пропавшее время: %v", nonzero(c))
	}
}

func TestSeriesCutsClose(t *testing.T) {
	// граница папки на j = 5 и пауза на j = 6: остаётся сильная — папка
	in := append(run("а", 5, 0, 30), run("б", 1, 150+400, 30)...)
	in = append(in, run("б", 10, 580+960, 30)...)
	for i := range in[5:] { // «б» одна папка для всех, кроме первого
		in[5+i].Path = filepath.Join("/съёмка", "б", fmt.Sprint(i, ".jpg"))
	}
	c := Cuts(in)
	if got := nonzero(c); len(got) != 1 || got[0] != 5 {
		t.Errorf("папка и пауза рядом: %v", c)
	}
	// две равные границы подряд: остаётся ранняя
	in = []model.PhotoInfo{shot("а", 0, -1), shot("б", 1, -1), shot("в", 2, -1), shot("в", 3, -1)}
	if got := nonzero(Cuts(in)); len(got) != 1 || got[0] != 1 {
		t.Errorf("равные подряд: %v", got)
	}
	// ближе нет — обе остаются: развороты по 2 фото допустимы
	in = []model.PhotoInfo{shot("а", 0, -1), shot("а", 1, -1), shot("б", 2, -1), shot("б", 3, -1), shot("в", 4, -1), shot("в", 5, -1)}
	if got := nonzero(Cuts(in)); len(got) != 2 || got[0] != 2 || got[1] != 4 {
		t.Errorf("через одно фото: %v", got)
	}
}

// Слабая пауза на j = 5, сразу за ней смена папки на j = 6: остаётся более сильная, поздняя.
func TestSeriesCutsCloseLaterStronger(t *testing.T) {
	in := run("а", 6, 0, 30)
	in[5].Taken = t0 + 120 + 240 // пауза 240 с при темпе 30 с: сила ≈ 0,33
	in = append(in, shot("б", 6, 120+240+30))
	c := Cuts(in)
	if got := nonzero(c); len(got) != 1 || got[0] != 6 || c[6] != 1 {
		t.Errorf("слабая пауза и папка следом: %v", c)
	}
	// то же на самих силах: из двух ближе seriesMinDist остаётся сильная, остальные не трогаются
	cuts := []float64{0, 0.3, 0.8, 0, 0, 0.5, 0}
	thinCuts(cuts)
	if got := nonzero(cuts); len(got) != 2 || got[0] != 2 || got[1] != 5 || cuts[2] != 0.8 {
		t.Errorf("thinCuts: %v", cuts)
	}
}

// Вход не меняется, повтор даёт то же, на большой ленте (900 фото, 30 серий) — без лишнего.
func TestSeriesCutsStable(t *testing.T) {
	var in []model.PhotoInfo
	for s := range 30 {
		in = append(in, run(fmt.Sprint("п", s%3), 30, int64(s)*5000, 15)...)
	}
	before := fmt.Sprint(in)
	a, b := Cuts(in), Cuts(in)
	if fmt.Sprint(in) != before || fmt.Sprint(a) != fmt.Sprint(b) || len(a) != 901 {
		t.Fatal("вход изменён или результат нестабилен")
	}
	if got := len(nonzero(a)); got < 29 || got > 30 {
		t.Errorf("серий найдено %d из 30", got+1)
	}
	for j := 30; j < 900; j += 30 {
		if a[j] < 0.5 {
			t.Errorf("граница серии на %d слабая: %v", j, a[j])
		}
	}
}

// Любая лента (папки, Lib, Taken вплоть до MinInt64/MaxInt64): len n+1, края 0, значения в [0,1],
// без NaN, двух соседних ненулевых нет.
func FuzzSeriesCuts(f *testing.F) {
	special := []int64{math.MinInt64, -1, 0, 1, math.MaxInt64, 1 << 62}
	f.Add([]byte{})
	f.Add([]byte{0, 0, 0, 0, 1, 1, 1, 1, 0, 2, 2, 2})
	f.Add([]byte{0x10, 3, 0x11, 4, 0x20, 5, 0x30, 6, 0x41, 7, 0x00, 5}) // смесь Lib, папок и особых Taken
	f.Fuzz(func(t *testing.T, data []byte) {
		// запись — 2 байта: [папка 0–3 | 0x10 — Lib | 0x20 — особое Taken | 0x40 — произвольное Taken, 0x80 — вдвое реже] и время
		var in []model.PhotoInfo
		for i := 0; i+1 < len(data) && i < 600; i += 2 {
			ph := model.PhotoInfo{ID: fmt.Sprint(i), Path: filepath.Join("/съёмка", fmt.Sprint(data[i]&3), "a.jpg")}
			if data[i]&0x10 != 0 {
				ph.Lib = "my:0123456789abcdef"
			}
			if data[i]&0x20 != 0 {
				ph.Taken = special[int(data[i+1])%len(special)]
			} else if data[i]&0x40 != 0 { // произвольной величины: байт, сдвинутый на 0–56 бит
				ph.Taken = int64(data[i+1]) << (data[i] >> 2 & 7 * 8)
			} else {
				ph.Taken = t0 + int64(data[i+1])*int64(data[i]>>7+1)*40
			}
			in = append(in, ph)
		}
		c := Cuts(in)
		if c == nil {
			return
		}
		if len(c) != len(in)+1 || c[0] != 0 || c[len(in)] != 0 {
			t.Fatalf("длина %d при %d фото или края не 0", len(c), len(in))
		}
		for j, v := range c {
			if math.IsNaN(v) || v < 0 || v > 1 {
				t.Fatalf("[%d] = %v", j, v)
			}
			if j > 0 && v > 0 && c[j-1] > 0 {
				t.Fatalf("соседние границы %d и %d", j-1, j)
			}
		}
	})
}
