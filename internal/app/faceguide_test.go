package app

import (
	"encoding/json"
	"math"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/geom"
)

// Силуэт под лицо в App (silhouette.md, G9): норма листа и окно места, чтение без правки.

// guideTop — макушка силуэта: самая верхняя точка контура.
func guideTop(g model.FaceGuide) float64 {
	top := math.Inf(1)
	for _, p := range g.Path {
		top = min(top, p[1])
	}
	return top
}

// wantGuide сверяет силуэт с нормой n: окно, линия глаз и макушка (глаза − 0,75 F).
func wantGuide(t *testing.T, g model.FaceGuide, w, h float64, n geom.FaceNorm) {
	t.Helper()
	if !g.On || math.Abs(g.W-w) > 1e-9 || math.Abs(g.H-h) > 1e-9 || len(g.Path) != 85 {
		t.Fatalf("силуэт %v×%v, точек %d, on=%v; ждали окно %v×%v", g.W, g.H, len(g.Path), g.On, w, h)
	}
	if want := n.Eye * h; math.Abs(g.EyeY-want) > 1e-9 {
		t.Errorf("линия глаз %.4f, ждали %.4f (норма %+v)", g.EyeY, want, n)
	}
	if want := (n.Eye - 0.75*n.Size) * h; math.Abs(guideTop(g)-want) > 1e-9 {
		t.Errorf("макушка %.4f, ждали %.4f (норма %+v)", guideTop(g), want, n)
	}
}

func TestAppFaceGuide(t *testing.T) {
	e := newFacesEnv(t, 25)
	p, _ := e.ed.Snapshot()
	pl := p.Sheet(e.sid).PlaceRef(e.placeOf(t, 3))
	photo := e.ids[3]

	// Начальная норма 0,35 / 0,32; лиц не искали — детектор не звали.
	calls := e.calls.Load()
	wantGuide(t, e.a.FaceGuide(e.sid, photo), pl.W, pl.H, geom.DefaultFaceNorm)
	if e.calls.Load() != calls {
		t.Fatal("силуэт звал детектор лиц")
	}

	// Все места листа: окно своё у каждого, силуэт есть везде.
	for i := range e.ids {
		if g := e.a.FaceGuide(e.sid, e.ids[i]); !g.On {
			t.Fatalf("место %d: силуэта нет", i)
		}
	}

	// «Остальные — как это лицо»: норма образца, на всех местах листа сразу.
	e.fit(t, e.placeOf(t, 7), "")
	e.a.faceSt.mu.Lock()
	n := e.a.faceSt.norm[e.sid]
	e.a.faceSt.mu.Unlock()
	if n == geom.DefaultFaceNorm || n.Size <= 0 {
		t.Fatalf("норма листа после образца %+v", n)
	}
	for _, i := range []int{0, 3, 24} {
		q := p.Sheet(e.sid).PlaceRef(e.placeOf(t, i))
		wantGuide(t, e.a.FaceGuide(e.sid, e.ids[i]), q.W, q.H, n)
	}

	// Чтение: проект, rev и отмена не тронуты (кроме самого подгона до этого).
	rev := e.rev()
	for range 3 {
		e.a.FaceGuide(e.sid, photo)
	}
	if e.rev() != rev {
		t.Fatalf("rev %d → %d", rev, e.rev())
	}

	// Память другого альбома не отдаётся: для него нормы нет.
	other := newFacesEnv(t, 3)
	if got := e.a.faceSt.normOf(other.ed, e.sid); got != nil {
		t.Fatalf("норма чужого альбома: %+v", got)
	}
	if got := e.a.faceSt.normOf(e.ed, e.sid); got == nil || *got != n {
		t.Fatalf("норма своего альбома: %v, ждали %+v", got, n)
	}
	// Другой альбом открыт в App: его лист снова на начальной норме.
	e.a.ed.Store(other.ed)
	op, _ := other.ed.Snapshot()
	opl := op.Sheet(other.sid).PlaceRef(other.placeOf(t, 0))
	wantGuide(t, e.a.FaceGuide(other.sid, other.ids[0]), opl.W, opl.H, geom.DefaultFaceNorm)
}

func TestAppFaceGuideOff(t *testing.T) {
	e := newFacesEnv(t, 5)
	rev := e.rev()
	// Лист не найден, фото чужое, ID пустой — On = false, остальное пусто, ошибки нет.
	for _, c := range []struct{ name, sid, photo string }{
		{"чужой лист", "нет-такого", e.ids[0]},
		{"чужое фото", e.sid, "нет-такого"},
		{"пустой ID", e.sid, ""},
		{"оба пустые", "", ""},
	} {
		if g := e.a.FaceGuide(c.sid, c.photo); g.On || g.W != 0 || g.H != 0 || g.EyeY != 0 || len(g.Path) != 0 {
			t.Errorf("%s: %+v", c.name, g)
		}
	}
	// «Целиком» и групповое — силуэта нет.
	e.ed.SetPlaceFit(e.sid, []string{e.placeOf(t, 1)}, model.FitContain)
	if g := e.a.FaceGuide(e.sid, e.ids[1]); g.On {
		t.Error("«Целиком»: силуэт есть")
	}
	if g := e.a.FaceGuide(e.sid, e.ids[2]); !g.On {
		t.Error("соседнее место потеряло силуэт")
	}
	if e.rev() != rev+1 { // одна правка — сама SetPlaceFit
		t.Fatalf("rev %d → %d", rev, e.rev())
	}
}

// TestAppFaceGuideJSON: ключи ответа — как ждёт интерфейс (контракт: on, w, h, eyeY, path).
func TestAppFaceGuideJSON(t *testing.T) {
	e := newFacesEnv(t, 2)
	js, err := json.Marshal(e.a.FaceGuide(e.sid, e.ids[0]))
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{`"on":true`, `"w":`, `"h":`, `"eyeY":`, `"path":[[`} {
		if !strings.Contains(string(js), k) {
			t.Errorf("в JSON нет %s: %.80s", k, js)
		}
	}
	off, _ := json.Marshal(e.a.FaceGuide("нет-такого", ""))
	if !strings.Contains(string(off), `"on":false`) || !strings.Contains(string(off), `"path":[]`) {
		t.Errorf("пустой ответ: %s", off)
	}
}
