package backdrop_test

import (
	"bytes"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/backdrop"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Тела правок фона зовут и мимо обёрток Editor (те проверяют область сами): неизвестная
// область и лист проект не трогают, а на живой области те же вызовы правят его.
func TestBackdropBodiesBadArea(t *testing.T) {
	e, s1, _ := edtest.BdEd(t)
	edtest.SetBg(t, e, s1, "left", "bg")
	edtest.SetBg(t, e, s1, "right", "bg2")
	p := edtest.P(e).Clone()
	before := model.Marshal(p)

	edits := map[string]func(sid, scope string){
		"рамка":        func(sid, scope string) { backdrop.SetBackdropBox(p, sid, scope, 10, 20, 100, 80, 15) },
		"поворот":      func(sid, scope string) { backdrop.TurnBackdrop(p, sid, scope, 90) },
		"кадр":         func(sid, scope string) { backdrop.SetBackdropFrame(p, sid, scope, 2, 0.5, -0.5) },
		"наклон":       func(sid, scope string) { backdrop.SetBackdropAngle(p, sid, scope, 7, nil) },
		"выпрямить":    func(sid, scope string) { backdrop.StraightenBackdrop(p, sid, scope, 80, 5) },
		"зеркало":      func(sid, scope string) { backdrop.FlipBackdrop(p, sid, scope, "h") },
		"прозрачность": func(sid, scope string) { backdrop.SetBackdropFade(p, sid, scope, 0.4) },
	}
	for name, edit := range edits {
		for _, bad := range [][2]string{{s1, "top"}, {s1, ""}, {"нет", "left"}} {
			edit(bad[0], bad[1])
			if !bytes.Equal(model.Marshal(p), before) {
				t.Fatalf("%s (%q, %q): проект изменён", name, bad[0], bad[1])
			}
		}
		// Контроль: на живой области правка не холостая — иначе проверка выше ничего не доказывает.
		edit(s1, "left")
		if bytes.Equal(model.Marshal(p), before) {
			t.Fatalf("%s: на области left правка ничего не изменила", name)
		}
		p = edtest.P(e).Clone()
	}
}

// BaseFrame: кадр фона в снимке начала сессии наклона — копия; снимка, листа, области или фона
// нет — nil (база — текущий кадр); у фона без кадра — кадр по умолчанию {S: 1}, не nil.
func TestBackdropBaseFrame(t *testing.T) {
	e, s1, _ := edtest.BdEd(t)
	edtest.SetBg(t, e, s1, "left", "bg")
	edtest.SetBg(t, e, s1, "right", "bg2")
	e.SetBackdropFrame(s1, "left", 1.5, 0.3, -0.2)
	p0 := edtest.P(e).Clone()

	for name, f := range map[string]*model.Frame{
		"нет снимка":  backdrop.BaseFrame(nil, s1, "left"),
		"нет листа":   backdrop.BaseFrame(p0, "нет", "left"),
		"чужая обл.":  backdrop.BaseFrame(p0, s1, "top"),
		"нет фона":    backdrop.BaseFrame(p0, s1, "spread"),
		"пустой лист": backdrop.BaseFrame(p0, "", "left"),
	} {
		if f != nil {
			t.Errorf("%s: ждали nil, получили %+v", name, f)
		}
	}
	got := backdrop.BaseFrame(p0, s1, "left")
	if got == nil || *got != (model.Frame{S: 1.5, X: 0.3, Y: -0.2}) {
		t.Fatalf("кадр левой: %+v", got)
	}
	if def := backdrop.BaseFrame(p0, s1, "right"); def == nil || *def != (model.Frame{S: 1}) {
		t.Fatalf("фон без кадра: %+v, ждали {S: 1}", def)
	}
	// Копия: правка результата снимок не меняет.
	got.S = 4
	if f := p0.Sheet(s1).BgPhotoL.Frame; f.S != 1.5 {
		t.Fatalf("BaseFrame отдал не копию: кадр в снимке %+v", f)
	}
}
