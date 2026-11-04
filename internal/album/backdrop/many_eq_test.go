package backdrop

import (
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Сравнение фона листа для счёта ApplyTo (077) — по значению, как JSON в Editor: кадр nil и
// кадр с другими числами — разные, копии с тем же кадром — равны; Inks nil и пустой — равны.
func TestSameBg(t *testing.T) {
	f := func(s float64) *model.Frame { return &model.Frame{S: s} }
	bd := func(photo string, fr *model.Frame) *model.Backdrop {
		return &model.Backdrop{Photo: photo, W: 100, H: 50, Frame: fr}
	}
	for _, c := range []struct {
		name string
		a, b *model.Backdrop
		same bool
	}{
		{"оба nil", nil, nil, true},
		{"один nil", bd("a", nil), nil, false},
		{"тот же без кадра", bd("a", nil), bd("a", nil), true},
		{"тот же кадр — разные указатели", bd("a", f(2)), bd("a", f(2)), true},
		{"кадр и без кадра", bd("a", f(2)), bd("a", nil), false},
		{"другой кадр", bd("a", f(2)), bd("a", f(3)), false},
		{"другое фото", bd("a", nil), bd("b", nil), false},
		{"другая рамка", bd("a", nil), &model.Backdrop{Photo: "a", W: 100, H: 51}, false},
	} {
		if got := sameBackdrop(c.a, c.b); got != c.same {
			t.Errorf("%s: %v", c.name, got)
		}
	}
	for _, c := range []struct {
		name string
		a, b *model.Pattern
		same bool
	}{
		{"оба nil", nil, nil, true},
		{"один nil", &model.Pattern{Kind: "dots"}, nil, false},
		{"Inks nil и пустой", &model.Pattern{Kind: "dots"}, &model.Pattern{Kind: "dots", Inks: []string{}}, true},
		{"другие цвета", &model.Pattern{Kind: "dots", Inks: []string{"#000000"}}, &model.Pattern{Kind: "dots"}, false},
		{"другая плитка", &model.Pattern{Kind: "dots", Preset: "sa:a"}, &model.Pattern{Kind: "dots", Preset: "sa:b"}, false},
		{"другая прозрачность", &model.Pattern{Kind: "dots", Fade: 0.1}, &model.Pattern{Kind: "dots"}, false},
	} {
		if got := samePattern(c.a, c.b); got != c.same {
			t.Errorf("узор, %s: %v", c.name, got)
		}
	}
}
