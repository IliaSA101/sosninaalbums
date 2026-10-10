package grid

import (
	"bytes"
	"encoding/json"
	"errors"
	"log"
	"math"
	"slices"
	"strings"
	"testing"

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
	if err != nil || ver != 1 { // без углов — версия 1: 0.3.0/0.4.0 правят файл
		t.Fatalf("круг: %v %d", err, ver)
	}
	a, _ := json.Marshal(out)
	b, _ := json.Marshal(back)
	if string(a) != string(b) {
		t.Fatalf("после записи и чтения:\n%s\n%s", a, b)
	}
	if data, _ = model.MarshalTemplates(nil); !strings.Contains(string(data), `"templates": []`) {
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
	out, err := FracBoxes(p, tplScope, scope, frac)
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
	if _, err := FracBoxes(p, model.TplSpread, model.ScopeLeft, b); err == nil {
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
	for _, body := range []string{`{"version":3,"templates":{"a":1}}`, `{"version":9,"templates":"x"}`} {
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
	if err != nil || !bytes.Contains(data, []byte(`"version": 1`)) || bytes.Contains(data, []byte(`"rot"`)) {
		t.Fatalf("без угла — версия 1 без rot: %v\n%s", err, data)
	}
	rot := slices.Clone(flat)
	rot[0].Boxes = []model.Box{{X: 0.1, Y: 0.1, W: 0.5, H: 0.5}, {X: 0.6, Y: 0.6, W: 0.2, H: 0.3, Rot: 30}}
	if data, err = model.MarshalTemplates(rot); err != nil || !bytes.Contains(data, []byte(`"version": 2`)) || !bytes.Contains(data, []byte(`"rot": 30`)) {
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

func nearMM(a, b layout.Rect) bool {
	const eps = 0.01
	return math.Abs(a.X-b.X) <= eps && math.Abs(a.Y-b.Y) <= eps && math.Abs(a.W-b.W) <= eps && math.Abs(a.H-b.H) <= eps
}
