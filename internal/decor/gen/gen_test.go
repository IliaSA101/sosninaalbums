package gen

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
	"golang.org/x/image/math/f64"
)

// TestGenCatalog — ключи уникальны и по грамматике, у каждого имя и правильные цвета
// по умолчанию; Patterns/Lookup отдают копии.
func TestGenCatalog(t *testing.T) {
	ps := Patterns()
	if len(ps) < 14 {
		t.Fatalf("узоров %d, ждали ≥ 14 (research §2)", len(ps))
	}
	seen := map[string]bool{}
	for _, m := range append(ps, Arts()...) {
		if seen[m.Kind] || !ValidKind(m.Kind) || !Known(m.Kind) || m.Name == "" {
			t.Errorf("%+v: повтор, грамматика, Known или имя", m)
		}
		seen[m.Kind] = true
		if len(m.Inks) > MaxInks || (len(m.Inks) == 0) != m.Paper {
			t.Errorf("%s: цветов %d (текстура %v)", m.Kind, len(m.Inks), m.Paper)
		}
		for _, v := range m.Inks {
			if c, ok := normInk(v); !ok || c != v {
				t.Errorf("%s: цвет %q", m.Kind, v)
			}
		}
		if IsArt(m.Kind) != (m.Aspect > 0) {
			t.Errorf("%s: стикер — с пропорцией, узор — без", m.Kind)
		}
	}
	ps[0].Inks[0] = "#000000"
	if m, _ := Lookup(ps[0].Kind); m.Inks[0] == "#000000" {
		t.Error("Patterns отдал реестр, а не копию")
	}
	m, _ := Lookup("dots")
	m.Inks[0] = "#000000"
	if m2, _ := Lookup("dots"); m2.Inks[0] == "#000000" {
		t.Error("Lookup отдал реестр, а не копию")
	}
}

// TestGenUnknown — неизвестный ключ: Known = false, Lookup пуст, рисование не трогает холст;
// правильный ключ хранится Clamp (решение №91), неправильный — пустой Spec.
func TestGenUnknown(t *testing.T) {
	for _, k := range []string{"dots2", "zz-new", "paper-9"} {
		if Known(k) || IsArt(k) {
			t.Errorf("%s: Known", k)
		}
		if _, ok := Lookup(k); ok {
			t.Errorf("%s: Lookup", k)
		}
	}
	for k, want := range map[string]bool{
		"dots": true, "a": true, "a-1": true, strings.Repeat("a", 32): true,
		"": false, "Dots": false, "dots_2": false, "точки": false, strings.Repeat("a", 33): false, "a b": false,
	} {
		if ValidKind(k) != want {
			t.Errorf("ValidKind(%q) = %v", k, !want)
		}
	}
	dst := image.NewRGBA(image.Rect(0, 0, 40, 30))
	draw.Draw(dst, dst.Rect, image.NewUniform(color.RGBA{10, 20, 30, 255}), image.Point{}, draw.Src)
	before := bytes.Clone(dst.Pix)
	DrawPattern(dst, dst.Rect, Spec{Kind: "dots2"}, 1, 0)
	DrawArt(dst, f64.Aff3{40, 0, 0, 0, 30, 0}, Spec{Kind: "dots2"}, 0)
	DrawArt(dst, f64.Aff3{40, 0, 0, 0, 30, 0}, Spec{Kind: "dots"}, 0) // узор — не стикер
	if !bytes.Equal(before, dst.Pix) {
		t.Error("неизвестный ключ или узор как стикер что-то нарисовал")
	}
	got := Clamp(Spec{Kind: "zz-new", Inks: []string{"#AABBCC", "#112233", "плохой", "#445566"}, Seed: 5, Scale: 2, Aspect: 3})
	want := Spec{Kind: "zz-new", Inks: []string{"#aabbcc", "#112233"}, Seed: 5, Scale: 2}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Clamp неизвестного = %+v, ждали %+v", got, want)
	}
	if got := Clamp(Spec{Kind: "Плохой", Inks: []string{"#000000"}}); !reflect.DeepEqual(got, Spec{}) {
		t.Errorf("ключ не по грамматике: %+v", got)
	}
}

// TestGenClamp — края и пустые значения зажима; Clamp идемпотентен.
func TestGenClamp(t *testing.T) {
	cases := []struct {
		name string
		in   Spec
		want Spec
	}{
		{"пусто", Spec{Kind: "dots"}, Spec{Kind: "dots"}},
		{"сид у неслучайного", Spec{Kind: "dots", Seed: 9}, Spec{Kind: "dots"}},
		{"сид у случайного", Spec{Kind: "confetti", Seed: 9}, Spec{Kind: "confetti", Seed: 9}},
		{"масштаб 1 — по умолчанию", Spec{Kind: "dots", Scale: 1}, Spec{Kind: "dots"}},
		{"масштаб NaN", Spec{Kind: "dots", Scale: math.NaN()}, Spec{Kind: "dots"}},
		{"масштаб +Inf", Spec{Kind: "dots", Scale: math.Inf(1)}, Spec{Kind: "dots"}},
		{"масштаб < 0", Spec{Kind: "dots", Scale: -2}, Spec{Kind: "dots"}},
		{"масштаб мал", Spec{Kind: "dots", Scale: 0.01}, Spec{Kind: "dots", Scale: 0.25}},
		{"масштаб велик", Spec{Kind: "dots", Scale: 99}, Spec{Kind: "dots", Scale: 4}},
		{"шаг 0,01", Spec{Kind: "dots", Scale: 1.234}, Spec{Kind: "dots", Scale: 1.23}},
		{"цвета: регистр и лишние", Spec{Kind: "dots", Inks: []string{"#ABCDEF", "#000000"}}, Spec{Kind: "dots", Inks: []string{"#abcdef"}}},
		{"плохой цвет — цвет генератора", Spec{Kind: "bands", Inks: []string{"red", "#000000"}}, Spec{Kind: "bands", Inks: []string{"#e5d5c1", "#000000"}}},
		{"больше цветов, чем у генератора", Spec{Kind: "confetti", Inks: []string{"#000000", "#111111", "#222222", "#333333", "#444444"}},
			Spec{Kind: "confetti", Inks: []string{"#000000", "#111111", "#222222", "#333333"}}},
		{"Aspect у узора", Spec{Kind: "dots", Aspect: 2}, Spec{Kind: "dots"}},
		{"пустой список цветов", Spec{Kind: "dots", Inks: []string{}}, Spec{Kind: "dots"}},
		{"хвост по умолчанию", Spec{Kind: "waves", Inks: []string{"#010203", "#A9CCE1", "#7fb2d1"}}, Spec{Kind: "waves", Inks: []string{"#010203"}}},
		{"все по умолчанию", Spec{Kind: "waves", Inks: wavesInks()}, Spec{Kind: "waves"}},
		{"по умолчанию в середине", Spec{Kind: "waves", Inks: append(wavesInks()[:2], "#000000")}, Spec{Kind: "waves", Inks: append(wavesInks()[:2], "#000000")}},
		{"плохой цвет в хвосте", Spec{Kind: "waves", Inks: []string{"#010203", "red"}}, Spec{Kind: "waves", Inks: []string{"#010203"}}},
	}
	for _, c := range cases {
		got := Clamp(c.in)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: %+v, ждали %+v", c.name, got, c.want)
		}
		if again := Clamp(got); !reflect.DeepEqual(again, got) {
			t.Errorf("%s: не идемпотентен: %+v → %+v", c.name, got, again)
		}
	}
}

// wavesInks — цвета волн по умолчанию (копия).
func wavesInks() []string {
	m, _ := Lookup("waves")
	return m.Inks
}

// TestGenResolve — Scale 0 → 1, цвета дополнены генератором; у неизвестного ok = false.
func TestGenResolve(t *testing.T) {
	s, ok := Resolve(Spec{Kind: "waves", Inks: []string{"#010203"}})
	want := Spec{Kind: "waves", Inks: []string{"#010203", "#a9cce1", "#7fb2d1"}, Scale: 1}
	if !ok || !reflect.DeepEqual(s, want) {
		t.Errorf("Resolve = %+v %v, ждали %+v", s, ok, want)
	}
	if _, ok := Resolve(Spec{Kind: "dots2"}); ok {
		t.Error("неизвестный: ok")
	}
}

// TestGenQuery — круг Query → Parse для каждого узора; мусор, лишнее, края px — отказ.
func TestGenQuery(t *testing.T) {
	for _, m := range Patterns() {
		s := Spec{Kind: m.Kind, Inks: []string{"#102030"}, Scale: 1.5, Seed: 42}
		u := Query(s, 305.123, 210)
		path, raw, _ := strings.Cut(u, "?")
		req, ok := Parse(path, raw+"&px=320")
		if !ok || req.Art || !reflect.DeepEqual(req.Spec, Clamp(s)) || req.W != 305.12 || req.H != 210 || req.Px != 320 {
			t.Errorf("%s: %s → %+v %v", m.Kind, u, req, ok)
		}
		if w, h := req.Size(); w != 320 || h != 220 {
			t.Errorf("%s: размер %d×%d", m.Kind, w, h)
		}
	}
	if u := Query(Spec{Kind: "dots", Scale: 1}, 100, 50); u != "/media/gen/pat?k=dots&w=100&h=50&v=1" {
		t.Errorf("по умолчанию пишется лишнее: %s", u)
	}
	good := "k=dots&i=aabbcc&s=2&w=100&h=50&v=1&px=64"
	if _, ok := Parse(patPath, good); !ok {
		t.Fatal("правильный запрос отклонён")
	}
	bad := map[string]string{
		"чужой путь":          "/media/gen/x?" + good,
		"стикер вместо узора": artPath + "?" + good,
		"лишний параметр":     good + "&z=1",
		"повтор":              good + "&px=64",
		"нет px":              "k=dots&w=100&h=50&v=1",
		"нет v":               "k=dots&w=100&h=50&px=64",
		"нет w":               "k=dots&h=50&v=1&px=64",
		"px мал":              "k=dots&w=100&h=50&v=1&px=15",
		"px велик":            "k=dots&w=100&h=50&v=1&px=4097",
		"высота > 4096":       "k=dots&w=10&h=1000&v=1&px=1000",
		"плотнее 100 px/мм":   "k=dots&w=20&h=20&v=1&px=2001",
		"w = 0,01, px 4096":   "k=waves&w=0.01&h=0.01&s=4&v=1&px=4096",
		"NaN":                 "k=dots&s=NaN&w=100&h=50&v=1&px=64",
		"Inf":                 "k=dots&w=Inf&h=50&v=1&px=64",
		"w = 0":               "k=dots&w=0&h=50&v=1&px=64",
		"цвет":                "k=dots&i=zzzzzz&w=100&h=50&v=1&px=64",
		"пять цветов":         "k=dots&i=000000,000000,000000,000000,000000&w=100&h=50&v=1&px=64",
		"пустой цвет":         "k=dots&i=&w=100&h=50&v=1&px=64",
		"сид":                 "k=confetti&r=-1&w=100&h=50&v=1&px=64",
		"неизвестный":         "k=dots2&w=100&h=50&v=1&px=64",
		"битая строка":        "k=dots&w=100&h=50&v=1&px=64&%zz",
	}
	for name, q := range bad {
		path, raw := patPath, q
		if p, r, ok := strings.Cut(q, "?"); ok {
			path, raw = p, r
		}
		if req, ok := Parse(path, raw); ok {
			t.Errorf("%s: принят %+v", name, req)
		}
	}
}

// FuzzGenQuery — любой запрос: без паники; принятый — в пределах px и рисуется.
func FuzzGenQuery(f *testing.F) {
	f.Add(patPath, "k=dots&i=aabbcc&s=2&w=100&h=50&v=1&px=64")
	f.Add(patPath, "k=confetti&r=4294967295&w=0.01&h=0.01&v=0&px=16")
	f.Add(artPath, "k=dots&px=64&v=1")
	f.Fuzz(func(t *testing.T, path, raw string) {
		req, ok := Parse(path, raw)
		if !ok {
			return
		}
		w, h := req.Size()
		if w < 1 || h < 1 || w > MaxPx || h > MaxPx {
			t.Fatalf("%s?%s: размер %d×%d", path, raw, w, h)
		}
		if w*h <= 256*256 {
			req.Draw()
		}
	})
}

// FuzzParseDraw — принятый запрос рисуется целиком за разумное время: мусорная плотность
// (w 0,01 мм при px 4096) или маска бумаги в сотни МБ отклоняются ещё в Parse. В обычном
// go test — только затравки; поиск — вручную: go test -fuzz FuzzParseDraw -fuzztime=20s.
func FuzzParseDraw(f *testing.F) {
	f.Add(patPath, "k=dots&w=100&h=50&s=0.25&v=1&px=512")
	f.Add(patPath, "k=waves&w=5.12&h=5.12&s=4&v=1&px=512") // ровно 100 px/мм
	f.Add(patPath, "k=waves&w=0.01&h=0.01&s=4&v=1&px=4096")
	f.Add(patPath, "k=paper-1&w=20&h=20&v=1&px=4096")
	f.Add(patPath, "k=watercolor&w=2500&h=300&s=0.25&v=1&px=512")
	f.Add(artPath, "k=dots&px=64&v=1")
	f.Add(artPath, "k=wave-band&a=500&v=1&px=4096") // стикеры: самые вытянутые рамки и самые тяжёлые
	f.Add(artPath, "k=plate&a=0.002&v=1&px=4096")
	f.Add(artPath, "k=confetti-pile&r=9&v=1&px=4096")
	f.Add(artPath, "k=balloons&v=1&px=4096")
	limit := testkit.Limit(5 * time.Second)
	f.Fuzz(func(t *testing.T, path, raw string) {
		req, ok := Parse(path, raw)
		if !ok {
			return
		}
		if !req.Art && float64(req.Px) > MaxPxPerMM*req.W {
			t.Fatalf("%s?%s: принят %d px на %v мм", path, raw, req.Px, req.W)
		}
		start := time.Now()
		img := req.Draw()
		if d := time.Since(start); d > limit {
			t.Fatalf("%s?%s: рисование %v", path, raw, d)
		}
		if w, h := req.Size(); img.Rect.Dx() != w || img.Rect.Dy() != h {
			t.Fatalf("%s?%s: картинка %v, ждали %d×%d", path, raw, img.Rect, w, h)
		}
	})
}
