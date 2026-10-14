package grid

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"reflect"
	"slices"
	"strings"
	"testing"
	"unicode"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

func inUnit(b model.Box) bool {
	return b.W >= model.MinBoxFrac && b.H >= model.MinBoxFrac && b.X >= 0 && b.Y >= 0 && b.X+b.W <= 1+1e-9 && b.Y+b.H <= 1+1e-9
}

func tplOf(scope string, n int) model.Template {
	t := model.Template{ID: model.NewID(), Name: "t", Scope: scope}
	for i := range n {
		t.Boxes = append(t.Boxes, model.Box{X: float64(i) / 20, Y: 0.1, W: 0.04, H: 0.5})
	}
	return t
}

func TestTemplatesFile(t *testing.T) {
	// Размер и разбор.
	if _, _, err := model.ParseTemplates([]byte(strings.Repeat(" ", model.MaxTemplatesFile+1))); err == nil {
		t.Fatal("файл больше 1 МБ принят")
	}
	for _, bad := range []string{"", "{", "[]", `{"templates":[]}`, `{"version":0}`, "null"} {
		if _, _, err := model.ParseTemplates([]byte(bad)); err == nil {
			t.Fatalf("%q принят", bad)
		}
	}
	// Версия новее читается, неизвестные поля не мешают.
	list, ver, err := model.ParseTemplates([]byte(`{"version":2,"extra":1,"templates":[{"id":"x","name":"N","scope":"page","boxes":[{"x":0.1,"y":0.1,"w":0.5,"h":0.5}],"z":0}]}`))
	if err != nil || ver != 2 || len(list) != 1 || list[0].Name != "N" || !model.IsTextID(list[0].ID) {
		t.Fatalf("version 2: %v %d %+v", err, ver, list)
	}

	// Зажим.
	nan, inf := math.NaN(), math.Inf(1)
	many := make([]model.Box, model.MaxPlaces+5)
	for i := range many {
		many[i] = model.Box{X: 0.1, Y: 0.1, W: 0.2, H: 0.2}
	}
	id := model.NewID()
	in := []model.Template{
		{ID: id, Name: "  a\n\tb\x00 ", Scope: model.TplPage, Boxes: []model.Box{{X: nan, Y: 0, W: 0.5, H: 0.5}, {X: 5, Y: -3, W: 0, H: 7}, {X: 0, Y: inf, W: 1, H: 1}}},
		{ID: id, Name: "", Scope: model.TplSpread, Boxes: many}, // повтор ID, имя пустое
		{ID: "ZZ", Name: strings.Repeat("я", 100), Scope: model.TplPage, Boxes: many[:1]},
		{ID: model.NewID(), Name: "без мест", Scope: model.TplPage},
		{ID: model.NewID(), Name: "все NaN", Scope: model.TplPage, Boxes: []model.Box{{X: nan, Y: nan, W: nan, H: nan}}},
		{ID: model.NewID(), Name: "область", Scope: "x", Boxes: many[:1]},
	}
	out := model.ClampTemplates(in)
	if len(out) != 3 {
		t.Fatalf("осталось %d шаблонов: %+v", len(out), out)
	}
	if out[0].Name != "a b" || len(out[0].Boxes) != 1 || !inUnit(out[0].Boxes[0]) || out[0].Boxes[0].W != model.MinBoxFrac || out[0].Boxes[0].H != 1 {
		t.Fatalf("зажим мест/имени: %+v", out[0])
	}
	if out[1].Name != "Шаблон 2" || len(out[1].Boxes) != model.MaxPlaces || out[1].ID == out[0].ID {
		t.Fatalf("повтор ID, лимит мест, имя по умолчанию: %+v", out[1])
	}
	if n := len([]rune(out[2].Name)); n != model.MaxTemplateName || !model.IsTextID(out[2].ID) {
		t.Fatalf("длинное имя %d знаков, ID %q", n, out[2].ID)
	}
	if in[0].Name != "  a\n\tb\x00 " || len(in[0].Boxes) != 3 {
		t.Fatal("вход изменён")
	}

	// Предел числа шаблонов и круг запись → чтение.
	var lots []model.Template
	for range model.MaxTemplates + 10 {
		lots = append(lots, tplOf(model.TplPage, 1))
	}
	if got := model.ClampTemplates(lots); len(got) != model.MaxTemplates {
		t.Fatalf("шаблонов %d", len(got))
	}
	data, err := model.MarshalTemplates(out)
	if err != nil {
		t.Fatal(err)
	}
	back, ver, err := model.ParseTemplates(data)
	if err != nil || ver != 3 { // шаблон на MaxPlaces > 16 мест — версия 3 (027); без углов и ≤ 16 мест — 1 (TestTemplatesVersion3)
		t.Fatalf("круг: %v %d", err, ver)
	}
	a, _ := json.Marshal(out)
	b, _ := json.Marshal(back)
	if string(a) != string(b) {
		t.Fatalf("после записи и чтения:\n%s\n%s", a, b)
	}
	if data, _ = model.MarshalTemplates(nil); !strings.Contains(string(data), `"templates":[]`) {
		t.Fatalf("пустой список пишется как []: %s", data)
	}
}

// Холодный счёт на 7 фото (пять наборов пропорций параллельно): спека — ≤ 400 мс.
// Размер страницы меняется на каждом шаге, поэтому кэш layout не помогает.
func BenchmarkGridChoices7Cold(b *testing.B) {
	asp := []float64{1.5, 0.667, 1, 1.78, 0.75, 1.5, 0.8}
	for i := 0; b.Loop(); i++ {
		prm := layout.Params{PageW: 281 + float64(i)*0.001, PageH: 284, Margin: 10, Gap: 3, Layflat: true}
		if got := builtinGrids(prm, asp, 7); len(got) == 0 {
			b.Fatal("пусто")
		}
	}
}

// fracMM — доли шаблона в мм через fracBoxes (то, чем пользуется ApplyGrid).
func fracMM(t *testing.T, p *model.Project, frac []model.Box, tplScope, scope string) []layout.Rect {
	t.Helper()
	out, err := FracBoxes(p, nil, tplScope, scope, frac)
	if err != nil {
		t.Fatalf("fracBoxes(%s, %s): %v", tplScope, scope, err)
	}
	rs := make([]layout.Rect, len(out))
	for i, b := range out {
		rs[i] = rectOfBox(b)
	}
	return rs
}

func rectOfBox(b model.Box) layout.Rect { return layout.Rect{X: b.X, Y: b.Y, W: b.W, H: b.H} }

func TestFracBoxesPageOnSpread(t *testing.T) {
	p := model.New()
	p.Settings.PageW, p.Settings.PageH = 200, 100
	b := []model.Box{{X: 0.1, Y: 0.2, W: 0.5, H: 0.4}}
	got := fracMM(t, p, b, model.TplPage, model.ScopeSpread)
	want := []layout.Rect{{X: 20, Y: 20, W: 100, H: 40}, {X: 220, Y: 20, W: 100, H: 40}}
	if len(got) != 2 || !nearMM(got[0], want[0]) || !nearMM(got[1], want[1]) {
		t.Fatalf("шаблон страницы на разворот: %v", got)
	}
	if g := fracMM(t, p, b, model.TplPage, model.ScopeRight); len(g) != 1 || !nearMM(g[0], want[1]) {
		t.Fatalf("на правую: %v", g)
	}
	if g := fracMM(t, p, b, model.TplSpread, model.ScopeSpread); len(g) != 1 || !nearMM(g[0], layout.Rect{X: 40, Y: 20, W: 200, H: 40}) {
		t.Fatalf("шаблон разворота: %v", g)
	}
	if _, err := FracBoxes(p, nil, model.TplSpread, model.ScopeLeft, b); err == nil {
		t.Fatal("шаблон разворота на страницу принят")
	}
}

// clampBoxes и ClampBoxes — одно правило (минимум стороны, 0..1), различие только в NaN.
func TestClampBoxesSameRule(t *testing.T) {
	in := []model.Box{{X: -1, Y: 2, W: 0, H: 5}, {X: 0.5, Y: 0.5, W: 0.8, H: 0.001}}
	got, err := model.ClampBoxesStrict(in)
	if err != nil || !slices.Equal(got, model.ClampBoxes(in)) {
		t.Fatalf("%v %v ≠ %v", err, got, model.ClampBoxes(in))
	}
	if _, err := model.ClampBoxesStrict([]model.Box{{W: 0.5, H: math.NaN()}}); err == nil {
		t.Fatal("NaN принят")
	}
	if _, err := model.ClampBoxesStrict(nil); !errors.Is(err, model.ErrNoPlaces) {
		t.Fatalf("пусто: %v", err)
	}
}

func TestParseTemplatesNewerVersionBody(t *testing.T) {
	// Тело новой версии устроено иначе: это «новее программы», а не битый файл.
	for _, body := range []string{`{"version":5,"templates":{"a":1}}`, `{"version":9,"templates":"x"}`} {
		list, ver, err := model.ParseTemplates([]byte(body))
		if err != nil || ver <= model.TemplatesVersion || len(list) != 0 {
			t.Fatalf("%s: %v %d %v", body, err, ver, list)
		}
	}
	// Версия текущая, а тело битое — по-прежнему ошибка.
	if _, _, err := model.ParseTemplates([]byte(`{"version":1,"templates":{"a":1}}`)); err == nil {
		t.Fatal("битое тело версии 1 принято")
	}
}

func TestBuiltinGridsPanicInWorker(t *testing.T) {
	prm := model.New().Params()
	good := len(builtinGrids(prm, nil, 3))
	if good == 0 {
		t.Fatal("нет встроенных сеток")
	}
	old := layoutsImpl
	defer func() { layoutsImpl = old }()
	layoutsImpl = func(a []float64, p layout.Params, limit int) []layout.Result {
		if a[0] == 1 { // набор «квадраты» падает, остальные живы
			panic("тест")
		}
		return old(a, p, limit)
	}
	got := builtinGrids(prm, nil, 3) // без recover процесс тестов упал бы
	if len(got) == 0 || len(got) > good {
		t.Fatalf("сеток %d, было %d", len(got), good)
	}
}

func TestTemplatesRot(t *testing.T) {
	flat := []model.Template{{ID: model.NewID(), Name: "a", Scope: model.TplPage, Boxes: []model.Box{{X: 0.1, Y: 0.1, W: 0.5, H: 0.5}}}}
	data, err := model.MarshalTemplates(flat)
	if err != nil || !bytes.Contains(data, []byte(`"version":1`)) || bytes.Contains(data, []byte(`"rot"`)) {
		t.Fatalf("без угла — версия 1 без rot: %v\n%s", err, data)
	}
	rot := slices.Clone(flat)
	rot[0].Boxes = []model.Box{{X: 0.1, Y: 0.1, W: 0.5, H: 0.5}, {X: 0.6, Y: 0.6, W: 0.2, H: 0.3, Rot: 30}}
	if data, err = model.MarshalTemplates(rot); err != nil || !bytes.Contains(data, []byte(`"version":2`)) || !bytes.Contains(data, []byte(`"rot":30`)) {
		t.Fatalf("с углом — версия 2: %v\n%s", err, data)
	}
	back, ver, err := model.ParseTemplates(data)
	if err != nil || ver != 2 || ver > model.TemplatesVersion || len(back) != 1 || !slices.Equal(back[0].Boxes, rot[0].Boxes) {
		t.Fatalf("круг v2: %v %d %+v", err, ver, back)
	}
	// v1 (0.3.0/0.4.0) читается, угол 0.
	v1 := []byte(`{"version":1,"templates":[{"id":"0123456789ab","name":"Н","scope":"page","boxes":[{"x":0.1,"y":0.2,"w":0.3,"h":0.4}]}]}`)
	if list, ver, err := model.ParseTemplates(v1); err != nil || ver != 1 || len(list) != 1 || list[0].Boxes[0] != (model.Box{X: 0.1, Y: 0.2, W: 0.3, H: 0.4}) {
		t.Fatalf("v1: %v %d %+v", err, ver, list)
	}
	// Выброшенная коробка уходит со своим углом; угол — geom.NormDeg; у повёрнутой внутри
	// только центр, у ровной — вся.
	nan := math.NaN()
	got := model.ClampBoxes([]model.Box{{X: nan, W: 0.5, H: 0.5, Rot: 10}, {X: -0.3, Y: 0.9, W: 0.4, H: 0.4, Rot: 370}, {X: -0.3, W: 0.4, H: 0.4}, {W: 0.2, H: 0.2, Rot: nan}})
	want := []model.Box{{X: -0.2, Y: 0.8, W: 0.4, H: 0.4, Rot: 10}, {X: 0, W: 0.4, H: 0.4}}
	if !slices.Equal(got, want) {
		t.Fatalf("ClampBoxes: %+v ≠ %+v", got, want)
	}
	many := make([]model.Box, model.MaxPlaces+3)
	for i := range many {
		many[i] = model.Box{W: 0.1, H: 0.1, Rot: float64(i + 1)}
	}
	if got := model.ClampBoxes(many); len(got) != model.MaxPlaces || got[model.MaxPlaces-1].Rot != model.MaxPlaces {
		t.Fatalf("сверх MaxPlaces: %d, последний угол %v", len(got), got[len(got)-1].Rot)
	}
}

// TestTemplatesVersion3: шаблон больше чем на 16 мест пишется версией 3 (0.9.x с
// TemplatesVersion = 2 откроет файл только для чтения и не обрежет места до 16);
// до 16 мест — как раньше, 1 или 2. Версия 3 читается целиком и с записью.
func TestTemplatesVersion3(t *testing.T) {
	boxes := func(n int, rot float64) []model.Box {
		out := make([]model.Box, n)
		for i := range out {
			out[i] = model.Box{X: float64(i%8) / 8, Y: float64(i/8) / 8, W: 0.1, H: 0.1, Rot: rot}
		}
		return out
	}
	tpl := func(b []model.Box) []model.Template {
		return []model.Template{{ID: model.NewID(), Name: "a", Scope: model.TplSpread, Boxes: b}}
	}
	for _, c := range []struct {
		n   int
		rot float64
		ver int
	}{{16, 0, 1}, {16, 30, 2}, {17, 0, 3}, {17, 30, 3}, {model.MaxPlaces, 0, 3}} {
		data, err := model.MarshalTemplates(tpl(boxes(c.n, c.rot)))
		if err != nil || !bytes.Contains(data, fmt.Appendf(nil, `"version":%d`, c.ver)) {
			t.Fatalf("%d мест, угол %v: ждали версию %d: %v\n%.80s", c.n, c.rot, c.ver, err, data)
		}
		back, ver, err := model.ParseTemplates(data)
		if err != nil || ver != c.ver || ver > model.TemplatesVersion || len(back) != 1 || len(back[0].Boxes) != c.n {
			t.Fatalf("%d мест: круг %v %d %+v", c.n, err, ver, back)
		}
	}
	if model.TemplatesVersion != 4 { // 4 — шаблон виньетки (038, TestTemplatesVignetteRoundTrip)
		t.Fatalf("TemplatesVersion = %d", model.TemplatesVersion)
	}
	// Прежний код: версия файла больше его TemplatesVersion = 2 — только чтение.
	data, _ := model.MarshalTemplates(tpl(boxes(17, 0)))
	if _, ver, _ := model.ParseTemplates(data); ver <= 2 {
		t.Fatalf("0.9.x счёл бы версию %d своей", ver)
	}
}

// 147: запись без отступов, доли и угол до 1e-5, вход не меняется; -0 пишется как 0;
// угол, округлившийся в 0, не пишется. RoundTemplates даёт ту же запись (повтор — то же).
func TestMarshalTemplatesRound(t *testing.T) {
	in := []model.Template{
		{ID: "0123456789ab", Name: "a", Scope: model.TplPage, Boxes: []model.Box{
			{X: 0.123456789, Y: -0.000001, W: 0.3333333333333333, H: 0.999999}, {X: -0.0123456, Y: 0.5, W: 0.2, H: 0.2, Rot: 1e-7},
		}},
		{ID: "0123456789ac", Name: "b", Scope: model.TplSpread, Boxes: []model.Box{{X: 0.1, Y: 0.2, W: 0.3, H: 0.4, Rot: -12.345678901}}},
	}
	keep := slices.Clone(in[0].Boxes)
	data, err := model.MarshalTemplates(in)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"version":2,"templates":[{"id":"0123456789ab","name":"a","scope":"page","boxes":[` +
		`{"x":0.12346,"y":0,"w":0.33333,"h":1},{"x":-0.01235,"y":0.5,"w":0.2,"h":0.2}]},` +
		`{"id":"0123456789ac","name":"b","scope":"spread","boxes":[{"x":0.1,"y":0.2,"w":0.3,"h":0.4,"rot":-12.34568}]}]}`
	if string(data) != want {
		t.Fatalf("запись:\n%s\nждали\n%s", data, want)
	}
	if !slices.Equal(in[0].Boxes, keep) {
		t.Fatalf("вход изменён: %+v", in[0].Boxes)
	}
	r := model.RoundTemplates(in)
	if again, _ := model.MarshalTemplates(model.RoundTemplates(r)); string(again) != want {
		t.Fatalf("повторное округление:\n%s", again)
	}
	if r := model.RoundTemplates(nil); r == nil || len(r) != 0 {
		t.Fatalf("nil: %#v", r)
	}
}

// vigTplOf — шаблон виньетки на n мест (доли кратны 1e-5): первое — учитель, последнее —
// групповое, остальные ученики; шапка, стиль подписей, подпись учителя и стиль шапки.
func vigTplOf(n, pages int) model.Template {
	t := model.Template{ID: model.NewID(), Name: "Класс", Scope: model.TplSpread, Kind: model.TplVignette}
	roles := make([]string, n)
	for i := range n {
		t.Boxes = append(t.Boxes, model.Box{X: float64(i%8) / 8, Y: 0.2 + float64(i/8)/12, W: 0.11111, H: 0.07407})
		roles[i] = model.RolePupil
	}
	roles[0], roles[n-1] = model.RoleTeacher, model.RoleGroup
	caps := model.VigCaps(304)
	caps.Text = "{Имя}"
	head := model.ClampStyle(model.TextStyle{Font: "cormorant", Size: 26, Color: "#223344", Align: "center"})
	t.Vig = &model.VigTpl{
		VigGeo: model.VigGeo{Roles: roles, Head: &model.Box{X: 0.05, Y: 0.01, W: 0.9, H: 0.12345}, Pages: pages, Aspect: 0.67105, Ratio: "2:3"},
		Style:  model.VigStyle{Caps: &caps, TText: "{Имя} {Отчество}", Head: &head},
	}
	return model.RoundTemplates([]model.Template{t})[0]
}

// 038: шаблон виньетки переживает запись и чтение без потерь (роли, шапка, страницы,
// пропорции, стиль); версия файла 4 только при нём, без него — прежние 1/2/3; запись без
// отступов; программа до 038 (TemplatesVersion 3) файл только читает и видит сетку из
// тех же мест; старые файлы v1–v3 читаются обычными шаблонами.
func TestTemplatesVignetteRoundTrip(t *testing.T) {
	vig, sheet, grid := vigTplOf(26, 2), vigTplOf(64, 1), tplOf(model.TplPage, 3)
	list := []model.Template{vig, grid, sheet}
	if got := model.ClampTemplates(list); !reflect.DeepEqual(got, list) {
		t.Fatalf("чистый список изменён зажимом:\n%+v\n%+v", got, list)
	}
	data, err := model.MarshalTemplates(list)
	if err != nil || !bytes.HasPrefix(data, []byte(`{"version":4,`)) || bytes.ContainsAny(data, "\n\t") ||
		bytes.Count(data, []byte(`"kind":"vignette"`)) != 2 || bytes.Count(data, []byte(`"vig":`)) != 2 {
		t.Fatalf("запись: %v\n%.300s", err, data)
	}
	back, ver, err := model.ParseTemplates(data)
	if err != nil || ver != 4 || !reflect.DeepEqual(back, list) {
		t.Fatalf("круг: %v, версия %d\n%+v\n%+v", err, ver, back, list)
	}
	// Без шаблона виньетки — прежние версии; 64 места — 3, как у сетки.
	for _, c := range []struct {
		list []model.Template
		ver  int
	}{{[]model.Template{grid}, 1}, {[]model.Template{grid, sheet}, 4}, {nil, 1}} {
		if data, _ := model.MarshalTemplates(c.list); !bytes.HasPrefix(data, fmt.Appendf(nil, `{"version":%d,`, c.ver)) {
			t.Fatalf("%d шаблонов: %.40s, ждали версию %d", len(c.list), data, c.ver)
		}
	}
	// Программа до 038: версия новее её TemplatesVersion = 3 — только чтение; поля 038 ей
	// незнакомы, места читаются обычной сеткой.
	var old struct {
		Templates []struct {
			Scope string      `json:"scope"`
			Boxes []model.Box `json:"boxes"`
		} `json:"templates"`
	}
	if ver <= 3 || json.Unmarshal(data, &old) != nil || len(old.Templates) != 3 || len(old.Templates[2].Boxes) != 64 ||
		old.Templates[0].Scope != model.TplSpread {
		t.Fatalf("старая программа: версия %d, %+v", ver, old.Templates)
	}
	for v := 1; v <= 3; v++ {
		body := fmt.Appendf(nil, `{"version":%d,"templates":[{"id":"0123456789ab","name":"Старый","scope":"page","boxes":[{"x":0.1,"y":0.1,"w":0.5,"h":0.5}]}]}`, v)
		if l, got, err := model.ParseTemplates(body); err != nil || got != v || len(l) != 1 || l[0].Kind != "" || l[0].Vig != nil {
			t.Fatalf("v%d: %v %d %+v", v, err, got, l)
		}
	}
	// Округление шапки и пропорции — в копии; вход не меняется, повтор — то же.
	vig.Vig.Head.H, vig.Vig.Aspect = 0.123456789, 1.234567891
	r := model.RoundTemplates([]model.Template{vig})
	if h := r[0].Vig.Head.H; h != 0.12346 || r[0].Vig.Aspect != 1.23457 || vig.Vig.Head.H != 0.123456789 || vig.Vig.Aspect != 1.234567891 {
		t.Fatalf("округление: шапка %v, пропорция %v; вход %v %v", h, r[0].Vig.Aspect, vig.Vig.Head.H, vig.Vig.Aspect)
	}
	if again := model.RoundTemplates(r); !reflect.DeepEqual(again, r) {
		t.Fatal("повторное округление меняет шаблон")
	}
}

// 038: зажим шаблонов виньетки из файла (недоверенный вход).
func TestClampTemplatesVignette(t *testing.T) {
	nan := math.NaN()
	with := func(f func(*model.Template)) model.Template {
		tp := vigTplOf(5, 2)
		f(&tp)
		return tp
	}
	in := []model.Template{
		with(func(tp *model.Template) { tp.Kind = "calendar" }),                          // 0: чужой вид — прочь
		with(func(tp *model.Template) { tp.Vig = nil }),                                  // 1: без vig — сетка
		with(func(tp *model.Template) { tp.Kind = "" }),                                  // 2: vig без вида — сетка
		with(func(tp *model.Template) { tp.Vig.Roles = tp.Vig.Roles[:2] }),               // 3: ролей меньше
		with(func(tp *model.Template) { tp.Vig.Roles = append(tp.Vig.Roles, "x", "y") }), // 4: ролей больше
		with(func(tp *model.Template) { // 5: мусор в ролях, NaN у учителя, шапке и пропорции
			tp.Vig.Roles = []string{model.RoleTeacher, "boss", "", model.RoleGroup, model.RoleTeacher}
			tp.Boxes[0].X = nan
			tp.Vig.Head = &model.Box{X: nan, W: 1, H: 1}
			tp.Vig.Aspect, tp.Vig.Ratio, tp.Scope = nan, "7:3", model.TplPage
		}),
		with(func(tp *model.Template) { tp.Vig.Roles = slices.Repeat([]string{model.RoleGroup}, 5) }), // 6: только групповые — прочь
		with(func(tp *model.Template) { tp.Vig.Pages = 3 }),                                           // 7: страниц не 1/2 — прочь
		with(func(tp *model.Template) { tp.Vig.Pages = 0 }),                                           // 8: прочь
		with(func(tp *model.Template) { tp.Boxes = nil }),                                             // 9: без мест — прочь
		with(func(tp *model.Template) { // 10: крайние пропорции, шапка за краем, грязный стиль
			tp.Vig.Aspect = 100
			tp.Vig.Head = &model.Box{X: 0.95, Y: -1, W: 0.5, H: 0.001}
			tp.Vig.Style.TText = "  \n "
			tp.Vig.Style.Caps.Style.Size, tp.Vig.Style.Caps.Pad, tp.Vig.Style.Caps.Text = nan, -5, strings.Repeat("я", 400)
			tp.Vig.Style.Head.Size, tp.Vig.Style.Head.Color = 1e9, "url(x)"
		}),
		with(func(tp *model.Template) { // 11: мест больше предела — пары место-роль режутся вместе
			tp.Boxes = slices.Repeat(tp.Boxes[:1], model.MaxPlaces+6)
			tp.Vig.Roles = slices.Repeat([]string{model.RoleTeacher}, model.MaxPlaces+6)
		}),
		with(func(tp *model.Template) { tp.Vig.Aspect = 0 }),           // 12
		with(func(tp *model.Template) { tp.Vig.Aspect = math.Inf(1) }), // 13: не конечная — 1
		with(func(tp *model.Template) { // 14: углы мест и шапки — 0 (BuildOwn их не знает), место у края — внутри
			tp.Boxes[1].Rot, tp.Boxes[2].Rot, tp.Boxes[3].Rot = 30, nan, 45
			tp.Boxes[3].X = 0.99
			tp.Vig.Head.Rot = 15
		}),
	}
	keep := slices.Clone(in[5].Vig.Roles)
	out := model.ClampTemplates(in)
	if !slices.Equal(in[5].Vig.Roles, keep) || !math.IsNaN(in[5].Vig.Aspect) {
		t.Fatal("вход изменён")
	}
	byID := map[string]model.Template{}
	for _, tp := range out {
		byID[tp.ID] = tp
	}
	g14 := byID[in[14].ID]
	for i, b := range append(slices.Clone(g14.Boxes), *g14.Vig.Head) {
		if b.Rot != 0 || !inUnit(b) || len(g14.Boxes) != 5 {
			t.Errorf("шаблон 14: место %d %+v, мест %d", i, b, len(g14.Boxes))
		}
	}
	// Копия на листе (Vig.Own) — тот же зажим.
	own := model.ClampVignette(model.Vignette{Tpl: "my:x", Own: &model.VigShape{Name: "x", Boxes: in[14].Boxes, VigGeo: in[14].Vig.VigGeo}}).Own
	if own == nil || own.Boxes[1].Rot != 0 || own.Boxes[3].Rot != 0 || own.Head.Rot != 0 || !inUnit(own.Boxes[3]) {
		t.Errorf("Own с углами: %+v", own)
	}
	for _, i := range []int{0, 6, 7, 8, 9} {
		if _, ok := byID[in[i].ID]; ok {
			t.Errorf("шаблон %d не выброшен", i)
		}
	}
	if len(out) != len(in)-5 {
		t.Fatalf("осталось %d", len(out))
	}
	for _, i := range []int{1, 2} {
		if g := byID[in[i].ID]; g.Kind != "" || g.Vig != nil || len(g.Boxes) != 5 {
			t.Errorf("шаблон %d — не обычная сетка: %+v", i, g)
		}
	}
	pupil, teacher, group := model.RolePupil, model.RoleTeacher, model.RoleGroup
	for i, want := range map[int][]string{
		3:  {teacher, pupil, pupil, pupil, pupil},
		4:  {teacher, pupil, pupil, pupil, group},
		5:  {pupil, pupil, group, teacher}, // место учителя с NaN ушло вместе с ролью
		11: slices.Repeat([]string{teacher}, model.MaxPlaces),
	} {
		g := byID[in[i].ID]
		if g.Kind != model.TplVignette || g.Vig == nil || !slices.Equal(g.Vig.Roles, want) || len(g.Boxes) != len(want) {
			t.Errorf("шаблон %d: роли %v, мест %d, ждали %v", i, g.Vig.Roles, len(g.Boxes), want)
		}
	}
	g5 := byID[in[5].ID].Vig
	if g5.Head != nil || g5.Aspect != 1 || g5.Ratio != model.VigRatio || byID[in[5].ID].Scope != model.TplSpread {
		t.Errorf("шаблон 5: шапка %+v, пропорция %v, портрет %q, область %q", g5.Head, g5.Aspect, g5.Ratio, byID[in[5].ID].Scope)
	}
	g10 := byID[in[10].ID].Vig
	if h := g10.Head; g10.Aspect != model.MaxVigAspect || h == nil || !inUnit(*h) || h.H != model.MinBoxFrac {
		t.Errorf("шаблон 10: пропорция %v, шапка %+v", g10.Aspect, h)
	}
	st := g10.Style
	if st.TText != "" || st.Caps == nil || math.IsNaN(st.Caps.Style.Size) || st.Caps.Pad < 0 ||
		len([]rune(st.Caps.Text)) != model.MaxCaptionRunes || st.Head == nil || st.Head.Size > 1000 || st.Head.Color == "url(x)" {
		t.Errorf("шаблон 10: стиль %+v caps %+v head %+v", st, st.Caps, st.Head)
	}
	if a, b := byID[in[12].ID].Vig.Aspect, byID[in[13].ID].Vig.Aspect; a != model.MinVigAspect || b != 1 {
		t.Errorf("пропорция 0 → %v, +Inf → %v", a, b)
	}
	if again := model.ClampTemplates(out); !reflect.DeepEqual(again, out) {
		t.Fatal("повторный зажим меняет список")
	}
}

// Аудит 038: файл в пределе, но после зажима (роли по числу мест, стиль по умолчанию) больше
// 1 МБ — ParseTemplates отбрасывает с конца ровно столько шаблонов, сколько нужно: иначе Store
// отказывал бы любой записи, даже удалению. Тексты ошибок шаблонов — с заглавной (тост).
func TestParseTemplatesFitsFile(t *testing.T) {
	list := make([]model.Template, model.MaxTemplates)
	for i := range list {
		tp := vigTplOf(model.MaxPlaces, 1)
		tp.Name = strings.Repeat("я", model.MaxTemplateName)
		tp.Vig.Roles = nil
		for j := range tp.Boxes {
			tp.Boxes[j].X += 1e-5 // все знаки доли: файл ближе к пределу
		}
		tp.Vig.Style = model.VigStyle{TText: strings.Repeat("я", model.MaxCaptionRunes),
			Caps: &model.SheetCaps{Text: strings.Repeat("я", model.MaxCaptionRunes)}, Head: &model.TextStyle{}}
		list[i] = tp
	}
	data, _ := model.MarshalTemplates(list)
	clamped := model.ClampTemplates(list)
	grown, _ := model.MarshalTemplates(clamped)
	if len(data) > model.MaxTemplatesFile || len(grown) <= model.MaxTemplatesFile {
		t.Fatalf("файл %d байт, после зажима %d: случай не воспроизведён", len(data), len(grown))
	}
	got, _, err := model.ParseTemplates(data)
	n := len(got)
	if err != nil || n == 0 || n == len(list) || got[n-1].ID != list[n-1].ID {
		t.Fatalf("прочитано %d из %d: %v", n, len(list), err)
	}
	fits, _ := model.MarshalTemplates(got)
	more, _ := model.MarshalTemplates(clamped[:n+1])
	if len(fits) > model.MaxTemplatesFile || len(more) <= model.MaxTemplatesFile {
		t.Fatalf("%d шаблонов — %d байт, %d — %d: отброшено не ровно до предела", n, len(fits), n+1, len(more))
	}
	for _, e := range []error{model.ErrTemplatesFull, model.ErrTplNotFound} {
		if r := []rune(e.Error()); !unicode.IsUpper(r[0]) {
			t.Errorf("текст ошибки со строчной: %q", e)
		}
	}
}

// Паника в раскладках сеток попадает в журнал со стеком.
func TestSafeLayoutsLogsPanic(t *testing.T) {
	var buf bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(prev) })
	old := layoutsImpl
	layoutsImpl = func([]float64, layout.Params, int) []layout.Result { panic("тест-паника") }
	t.Cleanup(func() { layoutsImpl = old })
	if rs := safeLayouts([]float64{1}, model.New().Params()); rs != nil {
		t.Fatal("после паники ждали nil")
	}
	if out := buf.String(); !strings.Contains(out, "тест-паника") || !strings.Contains(out, "goroutine") {
		t.Fatalf("в журнале нет паники со стеком: %q", out)
	}
}

// Запись шаблонов не молчит о доле, которой нет в JSON: NaN — ошибка без данных, а не обрезанный файл;
// чистый список и пустой пишутся.
func TestMarshalTemplatesError(t *testing.T) {
	bad := model.Template{ID: "0123456789ab", Name: "a", Scope: model.TplPage, Boxes: []model.Box{{X: 0.1, Y: 0.1, W: 0.2, H: math.NaN()}}}
	good := tplOf(model.TplPage, 2)
	for name, list := range map[string][]model.Template{"NaN одним": {bad}, "NaN среди двух": {good, bad}} {
		if data, err := model.MarshalTemplates(list); err == nil || data != nil {
			t.Errorf("%s: ждали ошибку без данных, получили %q, %v", name, data, err)
		}
	}
	for name, list := range map[string][]model.Template{"два": {good, tplOf(model.TplSpread, 1)}, "пусто": nil} {
		if data, err := model.MarshalTemplates(list); err != nil || len(data) == 0 {
			t.Errorf("%s: %q, %v", name, data, err)
		}
	}
}

// vigHead: пустой ID и ссылка на несуществующую надпись — nil, своя надпись — именно она, не первая.
func TestVigHeadLookup(t *testing.T) {
	a, b := &model.TextBlock{ID: "aaaaaaaaaaaa"}, &model.TextBlock{ID: "bbbbbbbbbbbb"}
	s := &model.Spread{Texts: []*model.TextBlock{a, b}}
	if vigHead(s, "") != nil || vigHead(s, "cccccccccccc") != nil || vigHead(&model.Spread{}, a.ID) != nil {
		t.Fatal("пустой ID или чужая ссылка дали надпись")
	}
	if vigHead(s, a.ID) != a || vigHead(s, b.ID) != b {
		t.Fatal("надпись найдена не по ID")
	}
}

func nearMM(a, b layout.Rect) bool {
	const eps = 0.01
	return math.Abs(a.X-b.X) <= eps && math.Abs(a.Y-b.Y) <= eps && math.Abs(a.W-b.W) <= eps && math.Abs(a.H-b.H) <= eps
}
