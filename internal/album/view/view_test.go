package view_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/view"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// mixedProject — смесь для сравнения: «Авто» лучшая, постраничная, с перестановкой
// разворота (4 фото) и страницы, «в край», «Ручная», пустой разворот; 8–16 фото.
func mixedProject(t *testing.T) *model.Project {
	t.Helper()
	p := bigProject(9, 16)
	trim := func(i, n int) { p.Spreads[i].Photos = p.Spreads[i].Photos[:n] }
	trim(0, 8)
	trim(1, 10)
	trim(3, 12)
	trim(4, 8)
	trim(5, 2)
	trim(6, 11)
	p.Spreads[7].Photos = nil // пустой
	prm := p.Params()
	asp := func(i int) []float64 { return p.Aspects(p.Spreads[i].Photos) }
	pick := func(what string, rs []layout.Result, ok func(string) bool) string {
		for _, r := range rs {
			if ok(r.Key) {
				return r.Key
			}
		}
		t.Fatalf("%s: нет ключа среди %d", what, len(rs))
		return ""
	}
	twoPages := func(k string) bool { return strings.HasPrefix(k, "P[") && !strings.Contains(k, "-") }
	paged := pick("постранично", layout.Cached(asp(1), prm, model.VariantLimit), twoPages)
	p.Spreads[1].Layout = paged
	// Перестановка всего разворота бывает до 7 фото и не у всякого набора: 2:3, 3:2 через одно.
	trim(2, 4)
	for j, id := range p.Spreads[2].Photos {
		p.Photos[id] = model.PhotoRef{Path: edtest.Abs(id + ".jpg"), W: 2000 + 1000*(j%2), H: 3000 - 1000*(j%2)}
	}
	rr, _ := layout.CachedRearranged(asp(2), prm, model.VariantLimit)
	p.Spreads[2].Layout = pick("перестановка", rr, func(k string) bool { return model.IndexOf(layout.Cached(asp(2), prm, model.VariantLimit), k) < 0 })
	// Перестановка внутри страницы: ключ из ряда «с перестановкой» одной из страниц.
	for _, r := range layout.Cached(asp(3), prm, model.VariantLimit) {
		for side := range 2 {
			if !twoPages(r.Key) || p.Spreads[3].Layout != "" {
				continue
			}
			_, prr, _, _ := layout.CachedPages(r.Key, asp(3), prm, side, model.VariantLimit)
			for _, q := range prr {
				if view.PagePermuted(q.Key, side) {
					p.Spreads[3].Layout = q.Key
					break
				}
			}
		}
	}
	if p.Spreads[3].Layout == "" {
		t.Fatal("нет раскладки с перестановкой внутри страницы")
	}
	p.Spreads[4].Layout = "P[E(0)|V(H(1,2),H(3,4),H(5,6,7))]" // левая страница в край
	if _, ok := layout.Build(p.Spreads[4].Layout, asp(4), prm); !ok {
		t.Fatalf("ключ %s не строится", p.Spreads[4].Layout)
	}
	return p
}

// viewJSON — View на холодном кэше раскладок, параллельно или по порядку.
func viewJSON(t *testing.T, e *album.Editor, par bool) []byte {
	t.Helper()
	model.SetParallelLayouts(par)
	t.Cleanup(func() { model.SetParallelLayouts(true) })
	layout.ResetCache()
	data, err := json.Marshal(e.View())
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestViewParallelSame(t *testing.T) {
	e := edtest.Open(t, mixedProject(t), "")
	e.SetGrid(edtest.P(e).Spreads[6].ID, true) // «Ручная»
	seq, par := viewJSON(t, e, false), viewJSON(t, e, true)
	if !bytes.Equal(seq, par) {
		t.Fatalf("View параллельно отличается от последовательного:\n%s\n%s", seq, par)
	}
	// Смесь действительно смешанная: иначе сравнение ничего не доказывает.
	v := e.View()
	want := map[string]bool{"лучшая": false, "постранично": false, "перестановка": false,
		"страница с перестановкой": false, "в край": false, "сетка": false, "пусто": false}
	for i, sv := range v.Spreads {
		s := edtest.P(e).Spreads[i]
		switch {
		case len(s.Grid) > 0:
			want["сетка"] = true
		case len(s.Photos) == 0:
			want["пусто"] = true
		case sv.Layout != s.Layout && s.Layout != "":
			t.Fatalf("разворот %d: ключ %s не встал (%s)", i, s.Layout, sv.Layout)
		}
		want["лучшая"] = want["лучшая"] || s.Layout == "" && len(s.Photos) > 0 && len(s.Grid) == 0
		want["постранично"] = want["постранично"] || len(sv.Pages) == 2 && !sv.Rearranged
		want["перестановка"] = want["перестановка"] || sv.Rearranged
		want["страница с перестановкой"] = want["страница с перестановкой"] || len(sv.Pages) == 2 && (sv.Pages[0].Rearranged || sv.Pages[1].Rearranged)
		want["в край"] = want["в край"] || strings.Contains(sv.Layout, "E(") || strings.HasPrefix(sv.Layout, "B[")
	}
	for k, ok := range want {
		if !ok {
			t.Errorf("в смеси нет случая «%s»", k)
		}
	}
}

// TestKeepLayoutsParallelSame — смена настроек: прогрев в keepLayouts не меняет итог.
func TestKeepLayoutsParallelSame(t *testing.T) {
	run := func(par bool) []byte {
		e := edtest.Open(t, mixedProject(t), "")
		e.View()
		st := e.View().Settings
		st.Gap += 2
		model.SetParallelLayouts(par)
		defer func() { model.SetParallelLayouts(true) }()
		layout.ResetCache()
		e.SetSettings(st)
		return model.Marshal(edtest.P(e))
	}
	if seq, par := run(false), run(true); !bytes.Equal(seq, par) {
		t.Fatalf("проект после смены настроек отличается:\n%s\n%s", seq, par)
	}
}

// TestViewParallelRace — View и правки вперемешку из нескольких горутин (смысл — под -race).
func TestViewParallelRace(t *testing.T) {
	e := edtest.Open(t, bigProject(12, 9), "")
	layout.ResetCache()
	ids := make([]string, len(edtest.P(e).Spreads))
	for i, s := range edtest.P(e).Spreads {
		ids[i] = s.ID
	}
	var wg sync.WaitGroup
	for g := range 4 {
		wg.Go(func() {
			for i := range 6 {
				switch (g + i) % 3 {
				case 0:
					e.View()
				case 1:
					st := e.View().Settings
					st.Gap = float64(2 + (g+i)%4)
					e.SetSettings(st)
				default:
					e.StepLayout(ids[(g*7+i)%len(ids)], "spread", 1)
				}
			}
		})
	}
	wg.Wait()
}

// TestViewWorkerPanic — паника в воркере уходит в лог, View собирается целиком:
// упавший разворот считается последовательно, как до 020.
func TestViewWorkerPanic(t *testing.T) {
	e := edtest.Open(t, bigProject(6, 8), "")
	want := viewJSON(t, e, false)
	bad := edtest.P(e).Spreads[3].ID
	old := view.LayWorker()
	view.SetLayWorker(func(p *model.Project, s *model.Spread) view.Laid {
		if s.ID == bad {
			panic("сбой")
		}
		return old(p, s)
	})
	defer func() { view.SetLayWorker(old) }()
	var buf bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(prev)
	got := viewJSON(t, e, true)
	if !bytes.Equal(want, got) {
		t.Fatalf("View после паники воркера отличается")
	}
	if !strings.Contains(buf.String(), "ПАНИКА в раскладке разворота "+strconv.Quote(bad)) { // ID из файла — в кавычках (paths.LogQ)
		t.Fatalf("паники нет в логе: %q", buf.String())
	}
	if strings.Contains(buf.String(), "ещё") {
		t.Fatalf("сводка при одной панике: %q", buf.String())
	}
}

// TestViewWorkerPanicMany — массовая паника: один полный стек и одна сводка «ещё K»,
// View всё равно собирается целиком.
func TestViewWorkerPanicMany(t *testing.T) {
	const n = 6
	e := edtest.Open(t, bigProject(n, 8), "")
	want := viewJSON(t, e, false)
	old := view.LayWorker()
	view.SetLayWorker(func(*model.Project, *model.Spread) view.Laid { panic("сбой") })
	defer func() { view.SetLayWorker(old) }()
	var buf bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(prev)
	got := viewJSON(t, e, true)
	if !bytes.Equal(want, got) {
		t.Fatalf("View после паник воркеров отличается")
	}
	out := buf.String()
	if c := strings.Count(out, "ПАНИКА в раскладке разворота "); c != 1 {
		t.Fatalf("полных стеков %d, нужен 1: %q", c, out)
	}
	if c := strings.Count(out, "[running]:"); c != 1 {
		t.Fatalf("стеков горутин %d, нужен 1", c)
	}
	if !strings.Contains(out, "ПАНИКА в раскладке: ещё 5 разворотов") {
		t.Fatalf("нет сводки: %q", out)
	}
}

// Shown: описанный прямоугольник повёрнутой рамки, обрезанный областью.
func TestBackdropViewShown(t *testing.T) {
	area := layout.Rect{X: 0, Y: 0, W: 300, H: 300}
	cases := []struct {
		name string
		b    model.BackdropView
		want layout.Rect
		ok   bool
	}{
		{"без поворота", model.BackdropView{Area: area, Box: model.BackdropBox{X: 10, Y: 20, W: 100, H: 50}}, layout.Rect{X: 10, Y: 20, W: 100, H: 50}, true},
		{"за край области", model.BackdropView{Area: area, Box: model.BackdropBox{X: 250, Y: 0, W: 100, H: 50}}, layout.Rect{X: 250, W: 50, H: 50}, true},
		{"целиком за областью", model.BackdropView{Area: area, Box: model.BackdropBox{X: 300, Y: 0, W: 100, H: 50}}, layout.Rect{}, false},
		// 90° по часовой вокруг (X, Y): рамка уходит влево и вниз от угла.
		{"поворот 90", model.BackdropView{Area: layout.Rect{X: 0, Y: 0, W: 600, H: 300}, Box: model.BackdropBox{X: 400, Y: 0, W: 100, H: 50, Rot: 90}}, layout.Rect{X: 350, W: 50, H: 100}, true},
	}
	for _, c := range cases {
		r, ok := c.b.Shown()
		if ok != c.ok || math.Abs(r.X-c.want.X) > 1e-9 || math.Abs(r.Y-c.want.Y) > 1e-9 || math.Abs(r.W-c.want.W) > 1e-9 || math.Abs(r.H-c.want.H) > 1e-9 {
			t.Errorf("%s: %+v %v, нужно %+v %v", c.name, r, ok, c.want, c.ok)
		}
	}
}

// z: проход стека, подпись сразу над своим фото; «Авто» — ячейки, затем тексты.
func TestViewZ(t *testing.T) {
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(10, 10, 100, 100, "a"), edtest.Gp(60, 60, 100, 100, "b"), edtest.Gp(200, 10, 50, 50, "")})
	g := edtest.PlaceIDsOf(e)
	e.SetCaption("a", "Первое", "")
	e.SetCaption("b", "Второе", "")
	t1 := edtest.TextAt(t, e, "s1", 70, 70)
	// t1 под b: a, подпись a, t1, b, подпись b, пустое место.
	e.ArrangeLayers("s1", "down", []string{t1})
	if got := edtest.StackOf(e); !slices.Equal(got, []string{g[0], t1, g[1], g[2]}) {
		t.Fatalf("стек %v", got)
	}
	sv := e.View().Spreads[0]
	capZ := map[string]int{}
	for _, c := range sv.Captions {
		capZ[c.Photo] = c.Z
	}
	got := []int{sv.Places[0].Z, capZ["a"], sv.Texts[0].Z, sv.Places[1].Z, capZ["b"], sv.Places[2].Z}
	if !slices.Equal(got, []int{1, 2, 3, 4, 5, 6}) || sv.Frames[0].Z != 1 || sv.Frames[1].Z != 4 {
		t.Fatalf("z %v, ячейки %d %d", got, sv.Frames[0].Z, sv.Frames[1].Z)
	}

	ea, id := edtest.AutoEd(t)
	ea.SetCaption("b", "Подпись", "")
	tx := edtest.AddText(t, ea, id, 0)
	av := ea.View().Spreads[0]
	if av.Photos[1] != "b" {
		t.Fatalf("лента %v", av.Photos)
	}
	z := []int{av.Frames[0].Z, av.Frames[1].Z, av.Captions[0].Z, av.Frames[2].Z, av.Texts[0].Z}
	if !slices.Equal(z, []int{1, 2, 3, 4, 5}) || av.Texts[0].ID != tx {
		t.Fatalf("«Авто» z %v", z)
	}
}

// z в View уникальны и образуют 1..N: на этом порядке фронт строит стек под курсором (спека 015).
func TestViewZUnique(t *testing.T) {
	// zs — z мест (или ячеек «Авто»), подписей и текстов разворота; frames сетки
	// повторяют z своих мест, поэтому в сетке их не считаем.
	zs := func(sv model.SpreadView) []int {
		var out []int
		if sv.Grid {
			for _, p := range sv.Places {
				out = append(out, p.Z)
			}
		} else {
			for _, f := range sv.Frames {
				out = append(out, f.Z)
			}
		}
		for _, c := range sv.Captions {
			out = append(out, c.Z)
		}
		for _, x := range sv.Texts {
			out = append(out, x.Z)
		}
		for _, x := range sv.Stubs { // заглушки подписей (030) — в том же стеке
			out = append(out, x.Z)
		}
		slices.Sort(out)
		return out
	}
	want := func(n int) []int {
		w := make([]int, n)
		for i := range w {
			w[i] = i + 1
		}
		return w
	}

	rot := edtest.Gp(60, 60, 100, 100, "b")
	rot.Rot = 30
	e := edtest.GridEd(t, []*model.Place{edtest.Gp(10, 10, 100, 100, "a"), rot, edtest.Gp(200, 10, 50, 50, "")})
	e.SetCaption("a", "Первое", "")
	e.SetCaption("b", "Второе", "")
	t1, t2 := edtest.TextAt(t, e, "s1", 70, 70), edtest.TextAt(t, e, "s1", 20, 150)
	e.ArrangeLayers("s1", "down", []string{t1})
	sv := e.View().Spreads[0]
	if len(sv.Places) != 3 || len(sv.Captions) != 2 || len(sv.Texts) != 2 || sv.Texts[1].ID != t2 {
		t.Fatalf("сетка: мест %d, подписей %d, текстов %d", len(sv.Places), len(sv.Captions), len(sv.Texts))
	}
	if got := zs(sv); !slices.Equal(got, want(7)) {
		t.Fatalf("сетка: z %v", got)
	}
	// Шаблон листа: заглушка пустого места — сразу над ним.
	sv = e.SetSheetCaps("s1", true, model.SheetCaps{Style: model.DefaultCaptionStyle(), Pad: 1, Text: "{Фамилия}"}).Spreads[0]
	if got := zs(sv); len(sv.Stubs) != 1 || !slices.Equal(got, want(8)) {
		t.Fatalf("сетка с заглушкой: заглушек %d, z %v", len(sv.Stubs), got)
	}
	stubAbovePlace(t, sv)

	ea, id := edtest.AutoEd(t)
	ea.SetCaption("b", "Подпись", "")
	edtest.AddText(t, ea, id, 0)
	av := ea.SetSheetCaps(id, true, model.SheetCaps{Style: model.DefaultCaptionStyle(), Pad: 1, Text: "{Фамилия}"}).Spreads[0]
	if got := zs(av); len(av.Stubs) != 2 || !slices.Equal(got, want(len(av.Frames)+len(av.Captions)+len(av.Texts)+len(av.Stubs))) {
		t.Fatalf("«Авто»: заглушек %d, z %v", len(av.Stubs), got)
	}
	maxCell := 0
	for _, f := range av.Frames {
		maxCell = max(maxCell, f.Z)
	}
	for _, x := range av.Texts {
		if x.Z <= maxCell {
			t.Fatalf("«Авто»: текст z=%d не выше ячеек (до %d)", x.Z, maxCell)
		}
	}
}

// stubAbovePlace — заглушка пустого места сетки лежит сразу над ним.
func stubAbovePlace(t *testing.T, sv model.SpreadView) {
	t.Helper()
	for _, pv := range sv.Places {
		if pv.ID == sv.Stubs[0].Place && sv.Stubs[0].Z != pv.Z+1 {
			t.Fatalf("заглушка z=%d, место z=%d", sv.Stubs[0].Z, pv.Z)
		}
	}
}

func TestViewSafeZone(t *testing.T) {
	e := edtest.Open(t, model.New(), "")
	st := edtest.P(e).Settings
	st.PageW, st.PageH, st.Layflat = 281, 284, true
	v := e.SetSettings(st)
	if len(v.Safe) != 1 || v.Safe[0] != (layout.Rect{X: 8, Y: 8, W: 2*281 - 16, H: 284 - 16}) {
		t.Fatalf("layflat: %+v", v.Safe)
	}
	st.Layflat = false
	v = e.SetSettings(st)
	want := []layout.Rect{{X: 8, Y: 8, W: 265, H: 268}, {X: 289, Y: 8, W: 265, H: 268}}
	if len(v.Safe) != 2 || v.Safe[0] != want[0] || v.Safe[1] != want[1] {
		t.Fatalf("без layflat: %+v", v.Safe)
	}
	// Пустой проект тоже отдаёт зону: UI рисует её, не дожидаясь фото.
	if got := edtest.Open(t, model.New(), "").View().Safe; len(got) != 1 {
		t.Fatalf("новый проект: %+v", got)
	}
}

// Зона и предел «по тексту» — от Settings.Safe, а не от константы (028 А).
func TestSafeZoneFromSettings(t *testing.T) {
	e := edtest.Open(t, model.New(), "")
	st := edtest.P(e).Settings
	st.PageW, st.PageH, st.Layflat, st.Safe = 281, 284, true, 12
	v := e.SetSettings(st)
	if len(v.Safe) != 1 || v.Safe[0] != (layout.Rect{X: 12, Y: 12, W: 2*281 - 24, H: 284 - 24}) {
		t.Fatalf("layflat: %+v", v.Safe)
	}
	if v.TextMaxW != 281-24 || v.Settings.Safe != 12 {
		t.Fatalf("TextMaxW %v, safe %v", v.TextMaxW, v.Settings.Safe)
	}
	st.Layflat = false
	v = e.SetSettings(st)
	want := []layout.Rect{{X: 12, Y: 12, W: 257, H: 260}, {X: 293, Y: 12, W: 257, H: 260}}
	if len(v.Safe) != 2 || v.Safe[0] != want[0] || v.Safe[1] != want[1] || v.TextMaxW != 281-24 {
		t.Fatalf("без layflat: %+v, TextMaxW %v", v.Safe, v.TextMaxW)
	}
	// 0 — без зоны: контур по краю страниц, «по тексту» — вся страница.
	st.Safe = 0
	v = e.SetSettings(st)
	if len(v.Safe) != 2 || v.Safe[0] != (layout.Rect{W: 281, H: 284}) || v.TextMaxW != 281 {
		t.Fatalf("без зоны: %+v, TextMaxW %v", v.Safe, v.TextMaxW)
	}
}

// LowDPI по разворотам (028 В): мыльный кадр и мыльный фон дают 2, у соседнего
// (резкие кадр и фон, фото без размера) — 0, у View — сумма.
func TestViewLowDPIPerSpread(t *testing.T) {
	e := edtest.Open(t, model.New(), "")
	sharp := func(id string) model.PhotoInfo {
		return model.PhotoInfo{ID: id, Path: edtest.Abs(id + ".jpg"), W: 30000, H: 20000}
	}
	edtest.NewSpread(e, -1, []model.PhotoInfo{sharp("a"), edtest.Photo("soft")})
	v := edtest.NewSpread(e, -1, []model.PhotoInfo{sharp("b"), {ID: "z", Path: edtest.Abs("z.jpg")}})
	s0, s1 := v.Spreads[0].ID, v.Spreads[1].ID
	edtest.SetBg(t, e, s0, model.ScopeLeft, "bg") // 300×200 px на страницу — мыльный
	v, err := e.SetBackdrop(s1, model.ScopeSpread, sharp("bgsharp"))
	if err != nil {
		t.Fatal(err)
	}
	if v.Spreads[0].LowDPI != 2 || v.Spreads[1].LowDPI != 0 || v.LowDPI != 2 {
		t.Fatalf("развороты %d и %d, всего %d", v.Spreads[0].LowDPI, v.Spreads[1].LowDPI, v.LowDPI)
	}
	if b := v.Spreads[1].Backdrops[0]; b.DPI < v.MinDPI {
		t.Fatalf("резкий фон: %d dpi при пороге %d", b.DPI, v.MinDPI)
	}
}

// bigProject — n разворотов «Авто» по k фото: свадебная смесь 3:2, 2:3, 1:1 и 16:9
// (детерминированно), так что составы разворотов почти не повторяются.
func bigProject(n, k int) *model.Project {
	p := model.New()
	x := uint64(n*1000 + k)
	for i := range n {
		s := &model.Spread{ID: fmt.Sprintf("s%03d", i)}
		for j := range k {
			x = x*6364136223846793005 + 1442695040888963407
			w, h := 3000, 2000
			switch r := x >> 33 % 100; {
			case r < 30:
				w, h = 2000, 3000
			case r < 35:
				w, h = 2000, 2000
			case r < 40:
				w, h = 3200, 1800
			}
			id := fmt.Sprintf("p%03d-%02d", i, j)
			p.Photos[id] = model.PhotoRef{Path: edtest.Abs(id + ".jpg"), W: w, H: h}
			s.Photos = append(s.Photos, id)
		}
		p.Spreads = append(p.Spreads, s)
	}
	return p
}

// seqPar — бенчмарк дважды: по порядку (как до 020) и параллельно.
func seqPar(b *testing.B, f func(b *testing.B)) {
	for _, par := range []bool{false, true} {
		name := "seq"
		if par {
			name = "par"
		}
		b.Run(name, func(b *testing.B) {
			model.SetParallelLayouts(par)
			defer func() { model.SetParallelLayouts(true) }()
			f(b)
		})
	}
}

// benchViewCold — View на холодном кэше раскладок (кэш текста остаётся тёплым).
func benchViewCold(b *testing.B, n, k int) {
	seqPar(b, func(b *testing.B) {
		e := edtest.Open(b, bigProject(n, k), "")
		for b.Loop() {
			layout.ResetCache()
			e.View()
		}
	})
}

func BenchmarkViewCold40x16(b *testing.B) { benchViewCold(b, 40, 16) }

// BenchmarkViewCold200x16 — худший случай при пределе разворотов (073: 200 × 16, перебор на 16
// фото — самый долгий): наблюдение, без порога; прогрев с ходом — BenchmarkWarmView200x16.
func BenchmarkViewCold200x16(b *testing.B) { benchViewCold(b, 200, 16) }

// BenchmarkViewCold40x64 — 40 разворотов по 64 фото (быстрые раскладки, спека 027): цель ≤ 1 с.
func BenchmarkViewCold40x64(b *testing.B) { benchViewCold(b, 40, 64) }

func BenchmarkViewCold90x7(b *testing.B) { benchViewCold(b, 90, 7) }

// BenchmarkViewWarm40x16 — тот же альбом, кэш тёплый: цена View на каждую правку.
func BenchmarkViewWarm40x16(b *testing.B) {
	seqPar(b, func(b *testing.B) {
		e := edtest.Open(b, bigProject(40, 16), "")
		e.View()
		for b.Loop() {
			e.View()
		}
	})
}

// BenchmarkSetSettingsCold — смена промежутка на 90 × 7: прежние настройки в кэше
// (альбом только что показан), новые — нет; keepLayouts и View заново.
func BenchmarkSetSettingsCold(b *testing.B) {
	seqPar(b, func(b *testing.B) {
		e := edtest.Open(b, bigProject(90, 7), "")
		st := e.View().Settings
		gaps := [2]float64{st.Gap, st.Gap + 1}
		i := 0
		for b.Loop() {
			b.StopTimer()
			layout.ResetCache()
			e.View()
			b.StartTimer()
			i++
			st.Gap = gaps[i%2]
			e.SetSettings(st)
		}
	})
}

func TestCaptionPlacement(t *testing.T) {
	c := &model.Caption{Text: "Первый танец", Style: model.DefaultCaptionStyle()}
	rects := []layout.Rect{{X: 10, Y: 10, W: 100, H: 100}, {X: 60, Y: 115, W: 100, H: 100}, {X: 200, Y: 10, W: 50, H: 50}}
	capOf := func(id string, c *model.Caption, rects []layout.Rect, i int, pageH float64) model.CaptionView {
		return view.CaptionViewOf(view.CaptionIn{Photo: id, Text: c.Text, Look: view.LookOf(c, nil), Rects: rects, I: i, PageH: pageH})
	}
	cv := capOf("a", c, rects, 0, 300)
	if cv.X != 10 || cv.W != 100 || cv.Y != 111 || cv.H <= 0 || cv.Photo != "a" || cv.ID != "" {
		t.Fatalf("под фото: %+v", cv)
	}
	// Ниже с перекрытием стоит фото с 115: предел 114, подпись ~4 мм не влезает.
	if want := math.Round((111+cv.H-114)*10) / 10; cv.Overflow != want || want <= 0 {
		t.Fatalf("нехватка %.2f, ждали %.2f", cv.Overflow, want)
	}
	if len(cv.Lines) != 1 || cv.Lines[0].Y <= cv.Y || cv.Lines[0].Y > cv.Y+cv.H {
		t.Fatalf("строки: %+v", cv.Lines)
	}
	// Справа ничего ниже нет — до края страницы без обрезной зоны.
	if cv = capOf("c", c, rects, 2, 300); cv.Overflow != 0 || cv.Y != 61 {
		t.Fatalf("свободное место: %+v", cv)
	}
	if cv = capOf("c", c, rects, 2, 62); cv.Overflow <= 0 {
		t.Fatalf("у края страницы: %+v", cv)
	}
	// На фото: у нижнего края внутри ячейки; в низкой ячейке — нехватка.
	c.Over = true
	if cv = capOf("a", c, rects, 0, 300); cv.Overflow != 0 || math.Abs(cv.Y+cv.H-(110-3*view.CaptionPad)) > 1e-6 || !cv.Over {
		t.Fatalf("на фото: %+v", cv)
	}
	short := []layout.Rect{{X: 0, Y: 0, W: 100, H: 4}}
	if cv = capOf("a", c, short, 0, 300); cv.Overflow <= 0 {
		t.Fatalf("на низком фото: %+v", cv)
	}
	// Через Editor: подпись видна в View, стиль по умолчанию.
	e, _, _ := edtest.NewTwo(t)
	v := e.SetCaption("a", "Подпись", "")
	if cs := v.Spreads[0].Captions; len(cs) != 1 || cs[0].Photo != "a" || cs[0].Style != model.DefaultCaptionStyle() || cs[0].Text != "Подпись" {
		t.Fatalf("подписи: %+v", cs)
	}
	v = e.SetCaptionStyle("a", model.TextStyle{Font: "ptserif", Size: math.NaN(), Align: "x"}, true, 1)
	if cs := v.Spreads[0].Captions[0]; !cs.Over || cs.Style.Font != "ptserif" || cs.Style.Size != 14 || cs.Style.Align != "center" {
		t.Fatalf("стиль подписи: %+v", cs)
	}
	rev := v.Rev
	if v = e.SetCaptionStyle("b", model.DefaultCaptionStyle(), false, 1); v.Rev != rev {
		t.Fatal("стиль без подписи — холостая правка")
	}
}
