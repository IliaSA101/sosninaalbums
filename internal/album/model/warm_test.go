package model

// Прогрев раскладок (WarmLayouts) на разворотах без «Других вариантов».

import (
	"io"
	"log"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/layout/variety"
)

// WarmLayouts греет ряд «Авто» (variety.Row) и у разворотов на 17+ фото: «Других вариантов» там нет
// (ExtraRow пуст), и без прогрева холодный ряд считается под замком при первом View. Несколько
// разворотов сразу, пустой разворот не мешает, выключенный параллелизм не греет ничего.
func TestWarmLayoutsRow(t *testing.T) {
	p := New()
	p.Photos = map[string]PhotoRef{}
	var ss []*Spread
	for _, n := range []int{0, 6, 17, 30, 40} {
		s := &Spread{ID: NewID()}
		for i := range n {
			id := NewID()
			w, h := 300, 200+n // другой состав у каждого: ряды не делят кэш
			if i%4 == 0 {
				w, h = 200, 300
			}
			p.Photos[id] = PhotoRef{W: w, H: h}
			s.Photos = append(s.Photos, id)
		}
		ss = append(ss, s)
	}
	warm := func() []bool {
		var out []bool
		for _, s := range ss[2:] {
			out = append(out, variety.RowCached(p.Aspects(s.Photos), p.Params(), RowLen(len(s.Photos))))
		}
		return out
	}
	t.Cleanup(func() { SetParallelLayouts(true) })
	for _, par := range []bool{false, true} {
		SetParallelLayouts(par)
		variety.ResetCache()
		p.WarmLayouts(ss)
		for i, got := range warm() {
			if got != par {
				t.Fatalf("параллельно=%v: ряд разворота на %d фото прогрет=%v", par, len(ss[i+2].Photos), got)
			}
		}
	}
}

// WarmAspects с ходом (073): ход 1, 2, …, n по одному на разворот, без повторов и шагов назад, даже
// когда воркеры кончают вперемешку; после него ряды прогреты, как у WarmLayouts. Пустой список —
// ни одного вызова; без параллельности — ни прогрева, ни хода; done == nil — тот же прогрев.
func TestWarmAspectsProgress(t *testing.T) {
	p := New()
	p.Photos = map[string]PhotoRef{}
	var ss []*Spread
	for k := range 12 {
		s := &Spread{ID: NewID()}
		for i := range 3 + k%5 {
			id := NewID()
			w, h := 300, 200+k*7+i // свой состав у каждого: ряды не делят кэш
			if i%3 == 0 {
				w, h = 200, 300
			}
			p.Photos[id] = PhotoRef{W: w, H: h}
			s.Photos = append(s.Photos, id)
		}
		ss = append(ss, s)
	}
	t.Cleanup(func() { SetParallelLayouts(true); variety.ResetCache() })
	prm := p.Params()
	warmed := func(s *Spread) bool { return variety.RowCached(p.Aspects(s.Photos), prm, RowLen(len(s.Photos))) }
	run := func(spreads []*Spread) []int {
		var got []int
		WarmAspects(p.WarmJobs(spreads), p.Params(), func(n int) { got = append(got, n) })
		return got
	}
	if got := run(nil); len(got) != 0 {
		t.Fatalf("пустой список: ход %v", got)
	}
	SetParallelLayouts(false)
	variety.ResetCache()
	if got := run(ss); len(got) != 0 || warmed(ss[0]) {
		t.Fatalf("без параллельности: ход %v или прогрев", got)
	}
	SetParallelLayouts(true)
	for _, warm := range []bool{false, true} { // холодный и тёплый кэш
		got := run(ss)
		if len(got) != len(ss) {
			t.Fatalf("тёплый=%v: ход %v, ждали 1…%d", warm, got, len(ss))
		}
		for i, n := range got {
			if n != i+1 {
				t.Fatalf("тёплый=%v: ход %v не по порядку", warm, got)
			}
		}
		for _, s := range ss {
			if !warmed(s) {
				t.Fatalf("тёплый=%v: ряд разворота на %d фото не прогрет", warm, len(s.Photos))
			}
		}
	}
	variety.ResetCache()
	WarmAspects(p.WarmJobs(ss), p.Params(), nil)
	if !warmed(ss[5]) {
		t.Fatal("done == nil: ряд не прогрет")
	}
}

// TestWarmAspectsProgressPanicInDone: паника в done уходит в лог, как паника раскладки: воркер
// жив, остальные события приходят по порядку, прогрев доходит до конца. Паника на нескольких
// событиях подряд и на последнем.
func TestWarmAspectsProgressPanicInDone(t *testing.T) {
	prev := log.Writer()
	log.SetOutput(io.Discard)
	t.Cleanup(func() { log.SetOutput(prev); variety.ResetCache() })
	p := New()
	p.Photos = map[string]PhotoRef{}
	var ss []*Spread
	for k := range 10 {
		s := &Spread{ID: NewID()}
		for i := range 3 {
			id := NewID()
			p.Photos[id] = PhotoRef{W: 300, H: 200 + k*5 + i}
			s.Photos = append(s.Photos, id)
		}
		ss = append(ss, s)
	}
	variety.ResetCache()
	var got []int
	WarmAspects(p.WarmJobs(ss), p.Params(), func(n int) {
		got = append(got, n)
		if n == 2 || n == 3 || n == len(ss) {
			panic("сбой хода")
		}
	})
	if len(got) != len(ss) {
		t.Fatalf("событий %d из %d: %v", len(got), len(ss), got)
	}
	for i, n := range got {
		if n != i+1 {
			t.Fatalf("ход %v не по порядку", got)
		}
	}
}
