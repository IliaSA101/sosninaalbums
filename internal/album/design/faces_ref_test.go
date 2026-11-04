package design_test

import (
	"math"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/design"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/geom"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// «Остальные — как это лицо» (0.27-130-17): норма по образцу, образец не трогается.

func TestFitFacesByRef(t *testing.T) {
	e, sid := facesEd(t, 25)
	e.SetFrame("f03", 1.9, 0.05, -0.2)
	hits := hitsOf(e, sid, faceFor)
	v0 := e.View()
	size0, eye0, _ := share(v0, 3, hits[3])
	sample := hits[3].Place
	was := frames(e)["f03"]

	v, rep, n := e.FitFaces(sid, hits, sample, design.FacesAsk, nil, nil)
	if rep.RefErr != "" || rep.Asked || total(rep) != 24 {
		t.Fatalf("итог %+v", rep)
	}
	if math.Abs(n.Size-size0) > 0.005 || math.Abs(n.Eye-eye0) > 0.005 {
		t.Fatalf("норма %+v, доли образца %.4f / %.4f", n, size0, eye0)
	}
	for i, h := range hits {
		if i == 3 {
			continue
		}
		size, eye, dx := share(v, i, h)
		if math.Abs(size-size0) > 0.005 || math.Abs(eye-eye0) > 0.005 || math.Abs(dx) > 0.1 {
			t.Fatalf("место %d: размер %.4f, глаза %.4f, сдвиг %.3f; образец %.4f / %.4f", i, size, eye, dx, size0, eye0)
		}
	}
	if frames(e)["f03"] != was {
		t.Fatalf("образец тронут: %+v → %+v", was, frames(e)["f03"])
	}
	if rep.Set["f03"] != was {
		t.Fatalf("кадр образца в Set: %+v, ждали %+v", rep.Set["f03"], was)
	}
}

func TestFitFacesKeepsRefNorm(t *testing.T) {
	e, sid := facesEd(t, 25)
	e.SetFrame("f03", 1.9, 0.05, -0.2) // образец: лицо не по центру окна
	hits := hitsOf(e, sid, faceFor)
	_, first, norm := e.FitFaces(sid, hits, hits[3].Place, design.FacesAsk, nil, nil)
	others := frames(e)
	sample := others["f03"]
	delete(others, "f03")

	// Кнопка карточки дальше держит норму образца: окна нет (образец — «наш» кадр), остальные стоят.
	hits = hitsOf(e, sid, faceFor)
	_, again, used := e.FitFaces(sid, hits, "", design.FacesAsk, &norm, first.Set)
	if again.Asked || used != norm || total(again) != 25 {
		t.Fatalf("кнопка: %+v, норма %+v", again, used)
	}
	now := frames(e)
	for id, f := range others {
		if now[id] != f {
			t.Fatalf("фото %s сдвинулось: %+v → %+v", id, f, now[id])
		}
	}
	// Сам образец встал по центру нормы: сдвиг по X — не больше миллиметров.
	if f := now["f03"]; f.S != sample.S || math.Abs(f.X-sample.X) > 0.1 {
		t.Fatalf("образец: %+v → %+v", sample, f)
	}
	// Без нормы листа та же кнопка молча вернула бы всех к 0,35 / 0,32 — ради этого норму и помнят.
	v := e.View()
	_, lost, _ := e.FitFaces(sid, hits, "", design.FacesAsk, nil, again.Set)
	if lost.Asked || e.View().Rev == v.Rev {
		t.Fatalf("без нормы листа кадры остались прежними: %+v", lost)
	}
}

func TestFitFacesRefSkipsSample(t *testing.T) {
	e, sid := facesEd(t, 25)
	e.SetFrame("f03", 2.4, 0.3, -0.2) // образец с ручным кадром
	e.SetFrame("f05", 2.5, -0.3, 0.2) // и ещё одно правленое место
	hits := hitsOf(e, sid, faceFor)
	was := frames(e)
	// Образец вне счёта правленых: спрашивать нужно только про f05.
	_, ask, _ := e.FitFaces(sid, hits, hits[3].Place, design.FacesAsk, nil, nil)
	if !ask.Asked || ask.Manual != 1 {
		t.Fatalf("правленых %d (ждали одно): %+v", ask.Manual, ask)
	}
	_, keep, _ := e.FitFaces(sid, hits, hits[3].Place, design.FacesKeep, nil, nil)
	now := frames(e)
	if now["f03"] != was["f03"] || now["f05"] != was["f05"] || keep.Manual != 1 || total(keep) != 24 {
		t.Fatalf("keep: %+v, f03 %+v, f05 %+v", keep, now["f03"], now["f05"])
	}
	if keep.Set["f03"] != was["f03"] || len(keep.Set) != 24 { // 23 подогнанных, кадр образца; f05 оставлен
		t.Fatalf("Set: %d кадров, образец %+v", len(keep.Set), keep.Set["f03"])
	}
}

func TestFitFacesRefErrors(t *testing.T) {
	// setup правит снимок hits (что нашёл поиск) и/или лист (что сделали после) и называет образец.
	cases := []struct {
		name, code string
		setup      func(e *album.Editor, sid string, hits []model.FaceHit) string
	}{
		{"нет лица", model.RefNoFace, func(_ *album.Editor, _ string, h []model.FaceHit) string {
			h[3].Box, h[3].Eye = layout.Rect{}, geom.Pt{}
			return h[3].Place
		}},
		{"несколько лиц", model.RefMany, func(_ *album.Editor, _ string, h []model.FaceHit) string {
			h[3].Many = true
			return h[3].Place
		}},
		{"нет файла", model.RefNoFile, func(_ *album.Editor, _ string, h []model.FaceHit) string {
			h[3].Missing, h[3].Box = true, layout.Rect{}
			return h[3].Place
		}},
		{"«Целиком»", model.RefContain, func(e *album.Editor, sid string, h []model.FaceHit) string {
			e.SetPlaceFit(sid, []string{h[3].Place}, model.FitContain)
			return h[3].Place
		}},
		{"четверти сменили", model.RefChanged, func(e *album.Editor, _ string, h []model.FaceHit) string {
			e.TurnPhotos([]string{h[3].Photo}, 1)
			return h[3].Place
		}},
		{"фото заменили", model.RefChanged, func(e *album.Editor, _ string, h []model.FaceHit) string {
			e.SwapPhotos(h[3].Photo, h[4].Photo)
			return h[3].Place
		}},
		{"нет такого места", model.RefNoPlace, func(*album.Editor, string, []model.FaceHit) string { return "нет-места" }},
		{"пустое место", model.RefNoPlace, func(e *album.Editor, sid string, _ []model.FaceHit) string {
			p, _ := e.Snapshot()
			for _, pl := range p.Sheet(sid).Grid {
				if pl.Photo == "" {
					return pl.ID
				}
			}
			t.Fatal("пустых мест нет")
			return ""
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e, sid := facesEd(t, 22) // три места пустые
			hits := hitsOf(e, sid, faceFor)
			ref := c.setup(e, sid, hits)
			rev, undo, was := edtest.Rev(e), edtest.UndoLen(e), jsonOf(e)
			norm := &geom.FaceNorm{Size: 0.4, Eye: 0.3}
			_, rep, n := e.FitFaces(sid, hits, ref, design.FacesAll, norm, nil)
			edtest.Same(t, e, rev, undo, "ошибка образца")
			if rep.RefErr != c.code || model.RefErrText(rep.RefErr) == "" || total(rep) != 0 || jsonOf(e) != was {
				t.Fatalf("код %q, ждали %q: %+v", rep.RefErr, c.code, rep)
			}
			if n != *norm {
				t.Fatalf("норма при ошибке сменилась: %+v", n)
			}
		})
	}
}

// TestNormFrom — норма по образцу: доли лица по виду; лицо вне окна зажимается; образец не меняется.
func TestNormFrom(t *testing.T) {
	e, sid := facesEd(t, 3)
	e.SetFrame("f01", 1.7, 0.1, -0.3)
	hits := hitsOf(e, sid, faceFor)
	size, eye, _ := share(e.View(), 1, hits[1])
	p, _ := e.Snapshot()
	pl := p.Sheet(sid).Grid[1]
	n, code := design.NormFrom(p, pl, hits[1])
	if code != "" || math.Abs(n.Size-size) > 0.001 || math.Abs(n.Eye-eye) > 0.001 {
		t.Fatalf("норма %+v (%q), по виду %.4f / %.4f", n, code, size, eye)
	}
	// Рамка почти точка: размер зажат снизу, чтобы норма не стала нулём.
	tiny := hits[1]
	tiny.Box = layout.Rect{X: 0.5, Y: 0.3, W: 0.001, H: 0.001}
	if n, _ := design.NormFrom(p, pl, tiny); n.Size != 0.05 {
		t.Fatalf("крохотное лицо: размер %v", n.Size)
	}
	// Рамка на весь кадр при большом приближении: размер зажат сверху.
	huge := hits[1]
	huge.Box = layout.Rect{X: 0, Y: 0, W: 1, H: 1}
	if n, _ := design.NormFrom(p, pl, huge); n.Size != 1 {
		t.Fatalf("огромное лицо: размер %v", n.Size)
	}
	nan := hits[1]
	nan.Eye.Y = math.NaN()
	if _, code := design.NormFrom(p, pl, nan); code != model.RefNoFace {
		t.Fatalf("NaN в глазах: %q", code)
	}
	if model.RefErrText("") != "" || model.RefErrText("чужой") != "" {
		t.Fatal("для пустого и чужого кода текста нет")
	}
	if _, code := design.NormFrom(p, nil, hits[1]); code != model.RefNoPlace {
		t.Fatalf("без места: %q", code)
	}
}
