package model

// Прогрев раскладок (WarmLayouts) на разворотах без «Других вариантов».

import (
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
