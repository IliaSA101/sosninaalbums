package model

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"
	"testing"
)

// Макет альбома (спека 131, design §2): зажим, разбор, ключ байтов, серия, пропорции мест.

// lookTpl — шаг-разворот с одним местом и цветом страниц.
func lookTpl(places ...Box) SpreadTpl {
	return SpreadTpl{V: TplV, Pages: 2, Ratio: 2, Places: places, Page: []TplBg{{Color: "#ffffff"}, {Color: "#eeeeee"}}}
}

func step(role, name string, places ...Box) LookStep {
	return LookStep{Role: role, Name: name, Tpl: lookTpl(places...)}
}

func vigStep(name string) LookStep {
	return LookStep{Role: RoleVignette, Name: name, Tpl: lookTpl(), Vig: &LookVig{Tpl: "head-t-left", Teachers: 1, Pupils: 24, Ratio: "3:4"}}
}

var box1 = Box{X: 0.1, Y: 0.1, W: 0.3, H: 0.8}

// seriesLook — 0 титул, 1 виньетка, 2 сборы, 3–4 повтор, 5 финал-1, 6 виньетка, 7 финал-2.
func seriesLook() AlbumTpl {
	return AlbumTpl{V: LookV, Steps: []LookStep{
		step("", "Титул", box1), vigStep("Класс"), step("", "Сборы", box1, box1),
		step(RoleRepeat, "", box1), step(RoleRepeat, "", box1, box1, box1),
		step("", "Финал", box1), vigStep("Учителя"), step("", "Конец", box1),
	}}
}

func lookJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestLookClamp(t *testing.T) {
	in := AlbumTpl{V: LookV, Steps: []LookStep{
		{Role: "weird", Name: "  Глава\nпервая  ", Tpl: lookTpl(box1)},
		{Role: RoleRepeat, Tpl: lookTpl(box1)},
		{Role: RoleRepeat, Tpl: lookTpl(box1)},
		{Role: "", Tpl: lookTpl(box1)},
		{Role: RoleRepeat, Tpl: lookTpl(box1)},   // второй блок повтора — «один раз»
		{Role: RoleVignette, Tpl: lookTpl(box1)}, // виньетка без Vig — «один раз»
		{Role: RoleVignette, Tpl: SpreadTpl{V: TplV, Pages: 2, Page: []TplBg{{}, {}}},
			Vig: &LookVig{Tpl: "my:own", Teachers: 99, Pupils: 99, Ratio: "7:5"}}, // пустое оформление — можно
		{Role: "", Tpl: SpreadTpl{V: TplV, Pages: 2, Page: []TplBg{{}, {}}}}, // пустой шаг выбрасывается
		{Role: "", Tpl: SpreadTpl{V: 2, Pages: 2, Page: []TplBg{{}, {}}}},    // шаблон новее — тоже
		{Role: "", Tpl: lookTpl(box1), Vig: &LookVig{Tpl: "x"}},              // Vig не у виньетки снимается
	}}
	c, ok := ClampLook(in)
	if !ok {
		t.Fatal("макет отвергнут")
	}
	roles := make([]string, len(c.Steps))
	for i, s := range c.Steps {
		roles[i] = s.Role
	}
	want := []string{"", RoleRepeat, RoleRepeat, "", "", "", RoleVignette, ""}
	if !slices.Equal(roles, want) {
		t.Fatalf("роли %q, ждали %q", roles, want)
	}
	if c.Steps[0].Name != "Глава первая" {
		t.Errorf("имя %q", c.Steps[0].Name)
	}
	v := c.Steps[6].Vig
	if v == nil || v.Tpl != DefaultVignette().Tpl || v.Teachers != MaxTeachers || v.Ratio != VigRatio ||
		VigPlaces(Vignette{Tpl: v.Tpl, Teachers: v.Teachers, Pupils: v.Pupils}) > MaxPlaces {
		t.Errorf("виньетка %+v", v)
	}
	if c.Steps[7].Vig != nil {
		t.Error("Vig у шага не-виньетки не снят")
	}
	if again, ok := ClampLook(c); !ok || !bytes.Equal(lookJSON(t, again), lookJSON(t, c)) {
		t.Error("зажим не идемпотентен")
	}
	// Отказы: версия, пусто, одни виньетки.
	for name, bad := range map[string]AlbumTpl{
		"v0":       {Steps: []LookStep{step("", "", box1)}},
		"v2":       {V: 2, Steps: []LookStep{step("", "", box1)}},
		"пусто":    {V: LookV},
		"виньетки": {V: LookV, Steps: []LookStep{vigStep("a"), vigStep("b")}},
	} {
		if _, ok := ClampLook(bad); ok {
			t.Errorf("%s: принят", name)
		}
	}
	// Шагов больше 24 — обрезка (из файла такое не доходит: ParseLook отказывает).
	many := AlbumTpl{V: LookV}
	for range MaxLookSteps + 5 {
		many.Steps = append(many.Steps, step("", "", box1))
	}
	if c, ok := ClampLook(many); !ok || len(c.Steps) != MaxLookSteps {
		t.Errorf("обрезка шагов: %d", len(c.Steps))
	}
}

func TestLookClampCover(t *testing.T) {
	page := SpreadTpl{V: TplV, Pages: 1, Ratio: 0.5, Places: []Box{box1}, Page: []TplBg{{Color: "#aabbcc"}}}
	two := lookTpl(box1)
	in := AlbumTpl{V: LookV, Steps: []LookStep{step("", "", box1)}, Cover: &CoverTpl{
		Bg: &TplBg{Color: "#ABCDEF", Lib: "не ключ"}, Back: &two, Front: &page}}
	c, ok := ClampLook(in)
	if !ok || c.Cover == nil || c.Cover.Back != nil || c.Cover.Front == nil || c.Cover.Bg == nil || c.Cover.Bg.Lib != "" {
		t.Fatalf("обложка %+v", c.Cover)
	}
	// Пустая обложка (мусорный фон, сторона-разворот) — nil.
	in.Cover = &CoverTpl{Bg: &TplBg{Color: "мусор"}, Back: &two}
	if c, _ := ClampLook(in); c.Cover != nil {
		t.Errorf("пустая обложка осталась: %+v", c.Cover)
	}
	// Сторона на одну страницу, но шаблон негодный (страниц фона не столько же) — стороны нет.
	bad := SpreadTpl{V: TplV, Pages: 1, Places: []Box{box1}}
	in.Cover = &CoverTpl{Bg: &TplBg{Color: "#aabbcc"}, Front: &bad}
	if c, _ := ClampLook(in); c.Cover == nil || c.Cover.Front != nil || c.Cover.Bg == nil {
		t.Errorf("негодная сторона обложки: %+v", c.Cover)
	}
}

func TestParseLook(t *testing.T) {
	good := lookJSON(t, seriesLook())
	if _, err := ParseLook(good); err != nil {
		t.Fatal(err)
	}
	good1 := string(lookJSON(t, step("", "", box1)))
	// Тот же файл с шаблоном шага и сторон обложки v 1 (и null-обложкой) читается: проверка версий не
	// отвергает своё.
	for _, ok := range []string{
		`{"v":1,"cover":{"back":{"v":1,"pages":1,"ratio":1,"page":[{"color":"#ffffff"}],"places":[{"x":0,"y":0,"w":1,"h":1}]}},"steps":[` + good1 + `]}`,
		`{"v":1,"cover":null,"steps":[` + good1 + `]}`,
	} {
		if c, err := ParseLook([]byte(ok)); err != nil || len(c.Steps) != 1 {
			t.Errorf("%s: %v", ok, err)
		}
	}
	big := append(slices.Clone(good[:len(good)-1]), []byte(`,"pad":"`+strings.Repeat("x", MaxLookFile)+`"}`)...)
	many := AlbumTpl{V: LookV}
	for range MaxLookSteps + 1 {
		many.Steps = append(many.Steps, step("", "", box1))
	}
	cases := []struct {
		name string
		data []byte
		want error // nil — любая ошибка
	}{
		{"пусто", nil, nil},
		{"пустой объект", []byte(`{}`), ErrTplEmpty},
		{"усечён", good[:len(good)/2], nil},
		{"мусор", []byte("\x00\xff{"), nil},
		{"не объект", []byte(`[1,2]`), nil},
		{"больше предела", big, errLookBig},
		{"новее", []byte(`{"v":2,"steps":[]}`), ErrLookNewer},
		{"шагов больше 24", lookJSON(t, many), ErrLookSteps},
		{"шаг не объект", []byte(`{"v":1,"steps":[1]}`), nil},
		{"без шагов", []byte(`{"v":1,"steps":[]}`), ErrTplEmpty},
		{"одни виньетки", lookJSON(t, AlbumTpl{V: LookV, Steps: []LookStep{vigStep("")}}), ErrTplEmpty},
		// Версия — первым разбором: поля новее другого вида не делают файл «испорченным».
		{"новее, поля другого вида", []byte(`{"v":2,"steps":{"auto":true},"cover":"x"}`), ErrLookNewer},
		{"v не число", []byte(`{"v":"2","steps":[]}`), nil},
		{"шаги не список", []byte(`{"v":1,"steps":5}`), nil}, // версия разобралась, остальное — нет
		// Шаблон шага или стороны обложки новее — весь макет новее, шаг не выпадает молча.
		{"шаг новее", []byte(`{"v":1,"steps":[` + good1 + `,{"tpl":{"v":2,"pages":2,"page":[{},{}],"places":[{"x":0,"y":0,"w":1,"h":1}]}}]}`), ErrLookNewer},
		{"шаг новее, поля другого вида", []byte(`{"v":1,"steps":[` + good1 + `,{"tpl":{"v":2,"places":"новое"}}]}`), ErrLookNewer},
		{"оборот обложки новее", []byte(`{"v":1,"cover":{"back":{"v":2,"pages":1,"page":[{}]}},"steps":[` + good1 + `]}`), ErrLookNewer},
		{"лицо обложки новее", []byte(`{"v":1,"cover":{"front":{"v":3,"page":"новое"}},"steps":[` + good1 + `]}`), ErrLookNewer},
		{"обложка не объект", []byte(`{"v":1,"cover":[1],"steps":[` + good1 + `]}`), nil},
	}
	for _, c := range cases {
		_, err := ParseLook(c.data)
		if err == nil || c.want != nil && !errors.Is(err, c.want) {
			t.Errorf("%s: %v, ждали %v", c.name, err, c.want)
		}
	}
}

// Поле, которое добавит 0.29, не меняет ключ в 0.28: ключ — от байтов файла, не от разбора.
func TestLookKeyStableUnknownField(t *testing.T) {
	base := lookJSON(t, seriesLook())
	withNew := append(slices.Clone(base[:len(base)-1]), []byte(`,"auto":{"rule":"same-kind"}}`)...)
	a, err := ParseLook(base)
	if err != nil {
		t.Fatal(err)
	}
	b, err := ParseLook(withNew)
	if err != nil {
		t.Fatalf("поле новее программы: %v", err)
	}
	if !bytes.Equal(lookJSON(t, a), lookJSON(t, b)) {
		t.Error("незнакомое поле изменило разбор")
	}
	if LookKey(withNew) == LookKey(base) || !ValidLibKey(LookKey(withNew)) {
		t.Error("ключ не от байтов файла или не по грамматике my:<16 hex>")
	}
	// Сверка с заранее посчитанным sha256 (sha256sum): первые 8 байт, hex.
	for in, want := range map[string]string{
		`{"v":1,"steps":[]}`: "my:9cb58a31694d0f54",
		"":                   "my:e3b0c44298fc1c14",
	} {
		if got := LookKey([]byte(in)); got != want {
			t.Errorf("LookKey(%q) = %s, ждали %s", in, got, want)
		}
	}
}

func TestLookSeries(t *testing.T) {
	l := seriesLook() // pre [0 1 2], rep [3 4], post [5 6 7]
	cases := []struct {
		n    int
		want []int
	}{
		{-5, []int{0, 1, 2, 3, 5, 6, 7}}, // меньше наименьшего — один повторяемый
		{0, []int{0, 1, 2, 3, 5, 6, 7}},
		{7, []int{0, 1, 2, 3, 5, 6, 7}},
		{8, []int{0, 1, 2, 3, 4, 5, 6, 7}},
		{11, []int{0, 1, 2, 3, 4, 3, 4, 3, 5, 6, 7}},
	}
	for _, c := range cases {
		if got := Series(l, c.n); !slices.Equal(got, c.want) {
			t.Errorf("n=%d: %v, ждали %v", c.n, got, c.want)
		}
	}
	if got := Series(l, 1e6); len(got) != MaxSpreads || got[len(got)-1] != 7 || got[0] != 0 {
		t.Errorf("предел %d: %d, края %d…%d", MaxSpreads, len(got), got[0], got[len(got)-1])
	}
	noRep := AlbumTpl{V: LookV, Steps: []LookStep{step("", "", box1), vigStep(""), step("", "", box1)}}
	for _, n := range []int{0, 1, 3, 50} {
		if got := Series(noRep, n); !slices.Equal(got, []int{0, 1, 2}) {
			t.Errorf("без повтора n=%d: %v", n, got)
		}
	}
	onlyRep := AlbumTpl{V: LookV, Steps: []LookStep{step(RoleRepeat, "", box1), step(RoleRepeat, "", box1)}}
	if got := Series(onlyRep, 5); !slices.Equal(got, []int{0, 1, 0, 1, 0}) {
		t.Errorf("только повтор: %v", got)
	}
}

func TestLookSeriesFit(t *testing.T) {
	l := seriesLook() // pre′ [0 2], rep [3 4], post′ [5 7]; виньетки 1 и 6
	noRep := AlbumTpl{V: LookV, Steps: []LookStep{step("", "", box1), step("", "", box1), vigStep(""), step("", "", box1)}}
	onlyVig := AlbumTpl{V: LookV, Steps: []LookStep{vigStep(""), vigStep("")}} // ClampLook такое отвергает
	cases := []struct {
		name string
		l    AlbumTpl
		m    int
		want []int
	}{
		{"m=0", l, 0, []int{}},
		{"m<0", l, -3, []int{}},
		{"m=1", l, 1, []int{0}},
		{"m=2", l, 2, []int{0, 7}},
		{"m=3 < pre+post", l, 3, []int{0, 2, 7}},
		{"m=4 = pre+post", l, 4, []int{0, 2, 5, 7}},
		{"m=7 ≥ pre+post", l, 7, []int{0, 2, 3, 4, 3, 5, 7}},
		{"без повтора — все не-виньетки по кругу", noRep, 6, []int{0, 1, 3, 0, 1, 3}},
		{"без повтора, m < всех", noRep, 2, []int{0, 3}},
		{"одни виньетки", onlyVig, 3, []int{}},
	}
	for _, c := range cases {
		got := SeriesFit(c.l, c.m)
		if !slices.Equal(got, c.want) {
			t.Errorf("%s: %v, ждали %v", c.name, got, c.want)
		}
		if got == nil {
			t.Errorf("%s: nil вместо пустого", c.name)
		}
	}
	for m := range 30 { // ровно m, без виньеток, края — первый шаг начала и последний конца
		got := SeriesFit(l, m)
		if len(got) != m || slices.ContainsFunc(got, func(i int) bool { return l.Steps[i].Role == RoleVignette }) {
			t.Fatalf("m=%d: %v", m, got)
		}
		if m >= 2 && (got[0] != 0 || got[m-1] != 7) {
			t.Fatalf("m=%d: края %v", m, got)
		}
	}
	if got := SeriesFit(l, 1e6); len(got) != MaxSpreads {
		t.Errorf("предел: %d", len(got))
	}
	if VigStep(l) != 1 || VigStep(AlbumTpl{Steps: []LookStep{step("", "", box1)}}) != -1 {
		t.Error("VigStep")
	}
}

// mmBox — место в мм области w×h.
func mmBox(b Box, w, h float64) (x, y, bw, bh float64) { return b.X * w, b.Y * h, b.W * w, b.H * h }

// Композиция шага целиком (0.27-131-20): на 20×30, 30×30, 40×30 отношения мест прежние, общие края
// и промежутки общие и равные (± 0,01 мм), место левой страницы не уходит через сгиб.
func TestLookPlacesKeepComposition(t *testing.T) {
	main := Box{X: 0.11, Y: 0.08, W: 0.28, H: 0.84, Rot: 3, Shape: ShapeOval}
	top := Box{X: 0.60, Y: 0.08, W: 0.30, H: 0.40}
	bottom := Box{X: 0.60, Y: 0.52, W: 0.30, H: 0.40}
	tpl := lookTpl(main, top, bottom) // нарисован на 30×30: разворот 600×300, Ratio 2
	before := slices.Clone(tpl.Places)
	const tol = 0.01
	near := func(a, b float64) bool { return math.Abs(a-b) <= tol }
	for _, f := range []struct {
		name         string
		w, h         float64 // мм разворота
		compW, compH float64 // композиция по design §2
	}{{"20×30", 400, 300, 400, 200}, {"30×30", 600, 300, 600, 300}, {"40×30", 800, 300, 600, 300}} {
		got := LookBoxes(tpl, f.w/f.h)
		if len(got) != 3 {
			t.Fatalf("%s: мест %d", f.name, len(got))
		}
		mx, my, mw, mh := mmBox(got[0], f.w, f.h)
		tx, ty, tw, th := mmBox(got[1], f.w, f.h)
		bx, by, bw, bh := mmBox(got[2], f.w, f.h)
		if !near(mw/mh, 600*0.28/(300*0.84)) || !near(tw/th, 1.5) || !near(bw/bh, 1.5) {
			t.Errorf("%s: отношения %.4f %.4f %.4f", f.name, mw/mh, tw/th, bw/bh)
		}
		if !near(my, ty) || !near(my+mh, by+bh) || !near(tx, bx) || !near(tx+tw, bx+bw) {
			t.Errorf("%s: общие края разошлись", f.name)
		}
		if !near(by-(ty+th), (300*0.04)*f.compH/300) {
			t.Errorf("%s: промежуток %.3f", f.name, by-(ty+th))
		}
		if mx+mw > f.w/2+tol {
			t.Errorf("%s: место левой страницы через сгиб: %.2f > %.2f", f.name, mx+mw, f.w/2)
		}
		// Композиция по центру: её рамка (0…1 шаблона) — compW×compH посередине области.
		left, right := (f.w-f.compW)/2, (f.w+f.compW)/2
		if !near(mx, left+0.11*f.compW) || !near(tx+tw, right-0.10*f.compW) || !near(my, (f.h-f.compH)/2+0.08*f.compH) {
			t.Errorf("%s: композиция не по центру: %.2f %.2f %.2f", f.name, mx, tx+tw, my)
		}
		if got[0].Rot != 3 || got[0].Shape != ShapeOval {
			t.Errorf("%s: угол и форма потеряны: %+v", f.name, got[0])
		}
	}
	if !slices.Equal(tpl.Places, before) {
		t.Error("LookBoxes изменила шаблон")
	}
	// Страница (Pages = 1): своя пропорция; мусорное r — места как есть.
	page := SpreadTpl{V: TplV, Pages: 1, Ratio: 1, Places: []Box{{X: 0, Y: 0, W: 0.5, H: 1}}, Page: []TplBg{{}}}
	if got := LookBoxes(page, 0.5); !near(got[0].Y*300, 75) || !near(got[0].H*300, 150) || got[0].X != 0 || got[0].W != 0.5 {
		t.Errorf("страница: %+v", got[0])
	}
	for _, r := range []float64{0, -1, math.NaN(), math.Inf(1)} {
		if got := LookBoxes(tpl, r); !slices.Equal(got, before) {
			t.Errorf("r=%v: %+v", r, got)
		}
	}
	if got := LookBoxes(SpreadTpl{Ratio: 2}, 1); len(got) != 0 {
		t.Errorf("без мест: %+v", got)
	}
	// Рамочка места (в памяти) едет с местом.
	bd := &Border{}
	withBorder := lookTpl(Box{X: 0.1, Y: 0.1, W: 0.2, H: 0.2, Border: bd})
	if got := LookBoxes(withBorder, 1); got[0].Border != bd {
		t.Error("рамочка места потеряна")
	}
}

func TestLookFit(t *testing.T) {
	l := seriesLook()
	// Места: титул 1, виньетки по 25, сборы 2, повтор 1 и 3, финал 1, конец 1 — при n = 7: 56.
	if p := seriesPlaces(l, Series(l, 7)); p != 56 {
		t.Fatalf("мест при n=7: %d", p)
	}
	cases := []struct{ k, want int }{
		{0, 7}, {56, 7}, {57, 8}, {59, 8}, {60, 9}, {63, 10}, {64, 11}, {1e6, MaxSpreads},
	}
	for _, c := range cases {
		n := LookFit(l, c.k)
		if n != c.want {
			t.Errorf("k=%d: n=%d, ждали %d", c.k, n, c.want)
		}
		if n < MaxSpreads && seriesPlaces(l, Series(l, n)) < c.k {
			t.Errorf("k=%d: при n=%d мест меньше", c.k, n)
		}
		if n > 7 && seriesPlaces(l, Series(l, n-1)) >= c.k {
			t.Errorf("k=%d: n=%d не наименьшее", c.k, n)
		}
	}
	noRep := AlbumTpl{V: LookV, Steps: []LookStep{step("", "", box1), step("", "", box1)}}
	emptyRep := AlbumTpl{V: LookV, Steps: []LookStep{step("", "", box1), {Role: RoleRepeat, Tpl: SpreadTpl{V: TplV, Pages: 2, Page: []TplBg{{Color: "#ffffff"}, {}}}}}}
	if LookFit(noRep, 100) != 2 || LookFit(emptyRep, 100) != 2 {
		t.Errorf("без повтора %d, повтор без мест %d", LookFit(noRep, 100), LookFit(emptyRep, 100))
	}
	page := AlbumTpl{V: LookV, Steps: []LookStep{{Tpl: SpreadTpl{V: TplV, Pages: 1, Places: []Box{box1}, Page: []TplBg{{}}}}}}
	if LookPlaces(page, 0) != 2 || LookPlaces(page, 1) != 0 || LookPlaces(page, -1) != 0 {
		t.Error("LookPlaces: страница × 2, вне шагов — 0")
	}
}

func TestLookCard(t *testing.T) {
	l := seriesLook()
	front := SpreadTpl{V: TplV, Pages: 1, Places: []Box{box1, box1}, Page: []TplBg{{}}}
	l.Cover = &CoverTpl{Front: &front}
	c := CardOf(l)
	if c != (LookCard{Steps: 8, From: 59 + 2, Vig: true, Cover: true}) {
		t.Errorf("карточка %+v", c)
	}
	plain := AlbumTpl{V: LookV, Steps: []LookStep{step("", "", box1)}}
	if c := CardOf(plain); c != (LookCard{Steps: 1, From: 1}) {
		t.Errorf("простая %+v", c)
	}
}

// FuzzParseLook — разбор чужого файла (набор, библиотека): ошибка вместо паники; принятый макет
// маршалится и читается тем же; серия и места не выходят за пределы.
func FuzzParseLook(f *testing.F) {
	l := seriesLook()
	front := SpreadTpl{V: TplV, Pages: 1, Places: []Box{box1}, Page: []TplBg{{Lib: "my:0123456789abcdef"}}}
	l.Cover = &CoverTpl{Bg: &TplBg{Color: "#ffffff"}, Front: &front}
	seed, _ := json.Marshal(l)
	f.Add(seed)
	f.Add([]byte(`{"v":1,"steps":[{"role":"repeat","tpl":{"v":1,"pages":1,"ratio":0.2,"page":[{}],"places":[{"x":0,"y":0,"w":1,"h":1}]}}]}`))
	f.Add([]byte(`{"v":1,"steps":[{"role":"vignette","vig":{"tpl":"my:x","teachers":-1,"pupils":1e9},"tpl":{"v":1,"pages":2,"page":[{},{}]}}]}`))
	f.Add([]byte(`{"v":2}`))
	f.Add([]byte(`{"v":1,"steps":[` + strings.Repeat(`{},`, 30) + `{}]}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		lt, err := ParseLook(data)
		if err != nil {
			return
		}
		checkLook(t, lt)
	})
}

// checkLook — свойства принятого макета: круг записи, пределы шагов, серии и мест.
func checkLook(t *testing.T, lt AlbumTpl) {
	t.Helper()
	{
		b, err := json.Marshal(lt)
		if err != nil {
			t.Fatalf("принятый макет не маршалится: %v", err)
		}
		again, err := ParseLook(b)
		if err != nil || !bytes.Equal(lookJSON(t, again), b) {
			t.Fatalf("круг записи изменил макет: %v\n%s", err, b)
		}
		if len(lt.Steps) < 1 || len(lt.Steps) > MaxLookSteps {
			t.Fatalf("шагов %d", len(lt.Steps))
		}
		for _, n := range []int{-1, 0, 1, 7, 1000} {
			for _, i := range Series(lt, n) {
				if i < 0 || i >= len(lt.Steps) {
					t.Fatalf("Series(%d): индекс %d", n, i)
				}
			}
			if got := SeriesFit(lt, n); len(got) != max(0, min(n, MaxSpreads)) {
				t.Fatalf("SeriesFit(%d): %d", n, len(got))
			}
		}
		if n := LookFit(lt, 1000); n < 1 || n > MaxSpreads {
			t.Fatalf("LookFit: %d", n)
		}
		for _, s := range lt.Steps {
			for _, r := range []float64{0.2, 1, 5} {
				for _, bx := range LookBoxes(s.Tpl, r) {
					if bx.Rot == 0 && (bx.X < -1e-9 || bx.Y < -1e-9 || bx.X+bx.W > 1+1e-9 || bx.Y+bx.H > 1+1e-9) {
						t.Fatalf("место вне области: %+v (r=%v)", bx, r)
					}
				}
			}
		}
		_ = fmt.Sprint(CardOf(lt))
	}
}

// CoverTpl: OwnBgs и HasBg различают стороны; CoverPageBg — цвет свой, узор и фото листа главнее.
func TestCoverTplBgs(t *testing.T) {
	side := func(bg TplBg) *SpreadTpl { return &SpreadTpl{V: TplV, Pages: 1, Ratio: 1, Page: []TplBg{bg}} }
	if (&CoverTpl{}).HasBg() || (&CoverTpl{Back: side(TplBg{})}).HasBg() {
		t.Fatal("пустой макет без фона")
	}
	c := &CoverTpl{Back: side(TplBg{Color: "#111111"})}
	if own := c.OwnBgs(); !c.HasBg() || own[0].Color != "#111111" || own[1] != (TplBg{}) {
		t.Fatalf("задняя: %+v", c.OwnBgs())
	}
	c = &CoverTpl{Front: side(TplBg{Lib: "my:0123456789abcdef"})}
	if own := c.OwnBgs(); !c.HasBg() || own[1].Lib == "" || own[0] != (TplBg{}) {
		t.Fatalf("лицевая: %+v", c.OwnBgs())
	}
	if c = (&CoverTpl{Bg: &TplBg{Color: "#222222"}}); !c.HasBg() {
		t.Fatal("фон листа")
	}
	bg := &TplBg{Color: "#aaaaaa", Pat: &Pattern{Kind: "dots"}, Fade: 0.3}
	own := TplBg{Color: "#bbbbbb", Lib: "my:0123456789abcdef", Fade: 0.5}
	if got := CoverPageBg(bg, own); got.Color != "#bbbbbb" || got.Pat == nil || got.Lib != "" || got.Fade != 0.3 {
		t.Fatalf("узор листа главнее картинки стороны: %+v", got)
	}
	if got := CoverPageBg(&TplBg{Color: "#aaaaaa"}, own); got.Color != "#bbbbbb" || got.Lib != own.Lib || got.Fade != 0.5 {
		t.Fatalf("у листа нет узора: картинка стороны: %+v", got)
	}
	if got := CoverPageBg(bg, TplBg{}); got.Color != "#aaaaaa" {
		t.Fatalf("цвет листа: %+v", got)
	}
	if got := CoverPageBg(nil, TplBg{Color: "#cccccc"}); got.Color != "#cccccc" {
		t.Fatalf("без листа: %+v", got)
	}
}
