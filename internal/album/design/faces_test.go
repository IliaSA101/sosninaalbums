package design_test

import (
	"fmt"
	"math"
	"reflect"
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/design"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/geom"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// Подгон по лицам (спека 130, G3). Лица подставлены вместо детектора: ставит их тест, а
// положение лица на листе проверяется по виду (Img, Win), а не той же геометрией, что в теле.

// facesEd — лист-виньетка 204×304 по умолчанию (24 ученика и учитель), n портретов 3:4.
func facesEd(t *testing.T, n int) (*album.Editor, string) {
	t.Helper()
	e := edtest.Open(t, album.NewVignetteSheet(204, 304), "")
	sid := edtest.P(e).Spreads[0].ID
	ph := make([]model.PhotoInfo, n)
	for i := range ph {
		ph[i] = model.PhotoInfo{ID: fmt.Sprintf("f%02d", i), Path: edtest.Abs(fmt.Sprintf("Лицо%02d.jpg", i)), W: 300, H: 400}
	}
	e.FillPlaces(ph, sid)
	return e, sid
}

// faceFor — лицо i-го места: размер 0,18…0,25 кадра, глаза у верхней трети рамки, чуть вбок.
func faceFor(i int) model.FaceHit {
	h := 0.18 + 0.01*float64(i%8)
	box := layout.Rect{X: 0.5 - h*0.375 + 0.01*float64(i%3-1), Y: 0.22, W: h * 0.75, H: h}
	return model.FaceHit{Box: box, Eye: geom.Pt{X: box.X + box.W/2, Y: box.Y + 0.38*h}}
}

// hitsOf — снимок мест листа sid с лицами face(i); i — номер места с фото по порядку сетки.
func hitsOf(e *album.Editor, sid string, face func(i int) model.FaceHit) []model.FaceHit {
	p, _ := e.Snapshot()
	var out []model.FaceHit
	for _, pl := range p.Sheet(sid).Grid {
		if pl.Photo == "" {
			continue
		}
		h := face(len(out))
		h.Place, h.Photo = pl.ID, pl.Photo
		h.Frame = p.Photos[pl.Photo].FrameOrDefault()
		h.Turn, _ = model.SplitR(h.Frame.R)
		out = append(out, h)
	}
	return out
}

// share — доли лица в окне по виду: размер рамки, линия глаз от верха окна и сдвиг центра
// рамки от середины окна (мм). Годится для фото без поворота и зеркала.
func share(v model.View, i int, h model.FaceHit) (size, eye, dx float64) {
	fv := v.Spreads[0].Frames[i]
	w := fv.Win
	return h.Box.H * fv.Img.H / w.H, (fv.Img.Y + h.Eye.Y*fv.Img.H - w.Y) / w.H,
		fv.Img.X + (h.Box.X+h.Box.W/2)*fv.Img.W - w.X - w.W/2
}

// wantShare — лицо места i стоит по норме n.
func wantShare(t *testing.T, v model.View, i int, h model.FaceHit, n geom.FaceNorm) {
	t.Helper()
	size, eye, dx := share(v, i, h)
	if math.Abs(size-n.Size) > 0.002 || math.Abs(eye-n.Eye) > 0.002 || math.Abs(dx) > 0.1 {
		t.Fatalf("место %d: размер %.4f, глаза %.4f, сдвиг %.3f мм; норма %+v", i, size, eye, dx, n)
	}
}

// jsonOf — проект как в файле (для сравнения «до/после»).
func jsonOf(e *album.Editor) string {
	p, _ := e.Snapshot()
	return string(model.Marshal(p))
}

// frames — кадры фото листа по ID фото.
func frames(e *album.Editor) map[string]model.Frame {
	p, _ := e.Snapshot()
	out := map[string]model.Frame{}
	for id, r := range p.Photos {
		out[id] = r.FrameOrDefault()
	}
	return out
}

// total — сумма групп отчёта без «Подогнано».
func total(r model.FaceReport) int {
	return r.Fitted + len(r.Check) + len(r.Short) + r.Contain + r.Manual + r.Changed
}

func TestFitFacesSheet(t *testing.T) {
	e, sid := facesEd(t, 25)
	hits := hitsOf(e, sid, faceFor)
	if len(hits) != 25 {
		t.Fatalf("мест с фото %d", len(hits))
	}
	before, rev, undo := jsonOf(e), edtest.Rev(e), edtest.UndoLen(e)
	v, rep, n := e.FitFaces(sid, hits, "", design.FacesAsk, nil, nil)
	if n != geom.DefaultFaceNorm || rep.Asked || rep.RefErr != "" {
		t.Fatalf("норма %+v, отчёт %+v", n, rep)
	}
	if total(rep) != 25 || rep.Fitted != 25 || len(rep.Set) != 25 {
		t.Fatalf("итог не сошёлся: %+v", rep)
	}
	for i, h := range hits {
		wantShare(t, v, i, h, n)
	}
	if edtest.Rev(e) != rev+1 || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("rev %d→%d, undo %d→%d: нужна одна правка", rev, edtest.Rev(e), undo, edtest.UndoLen(e))
	}
	e.Undo()
	if jsonOf(e) != before {
		t.Fatal("Ctrl+Z не вернул кадры")
	}
}

func TestFitFacesUndoOne(t *testing.T) {
	e, sid := facesEd(t, 25)
	before := jsonOf(e)
	_, rep, _ := e.FitFaces(sid, hitsOf(e, sid, faceFor), "", design.FacesAsk, nil, nil)
	if rep.Fitted == 0 || jsonOf(e) == before {
		t.Fatal("подгон ничего не поставил")
	}
	undo := edtest.UndoLen(e)
	e.Undo()
	if edtest.UndoLen(e) != undo-1 || jsonOf(e) != before {
		t.Fatalf("один Ctrl+Z не вернул лист: шагов %d → %d", undo, edtest.UndoLen(e))
	}
	e.Redo()
	if !reflect.DeepEqual(frames(e)["f00"], rep.Set["f00"]) {
		t.Fatalf("после Redo кадр %+v, ждали %+v", frames(e)["f00"], rep.Set["f00"])
	}
}

func TestFitFacesNoop(t *testing.T) {
	e, sid := facesEd(t, 25)
	_, rep, _ := e.FitFaces(sid, hitsOf(e, sid, faceFor), "", design.FacesAsk, nil, nil)
	rev, undo, was := edtest.Rev(e), edtest.UndoLen(e), jsonOf(e)
	// Повтор с кадрами прошлого подгона и со свежим снимком: правка холостая, отчёт тот же.
	v, again, _ := e.FitFaces(sid, hitsOf(e, sid, faceFor), "", design.FacesAsk, nil, rep.Set)
	edtest.Same(t, e, rev, undo, "повторный подгон")
	if jsonOf(e) != was || again.Asked || again.Fitted != 25 || v.Rev != rev {
		t.Fatalf("повтор: %+v", again)
	}
	// Пустое: нет листа, нет мест с фото, пустой список лиц.
	_, none, _ := e.FitFaces("нет", nil, "", design.FacesAsk, nil, nil)
	_, empty, _ := e.FitFaces(sid, nil, "", design.FacesAsk, nil, nil)
	edtest.Same(t, e, rev, undo, "лист без лиц")
	if total(none) != 0 || empty.Changed != 25 || none.Check == nil || none.Short == nil {
		t.Fatalf("нет листа: %+v, пустой список: %+v", none, empty)
	}
}

func TestFitFacesSkips(t *testing.T) {
	e, sid := facesEd(t, 22) // три места пустые: в счёт не идут
	p, _ := e.Snapshot()
	var ids []string
	for _, pl := range p.Sheet(sid).Grid {
		if pl.Photo != "" {
			ids = append(ids, pl.ID)
		}
	}
	if len(ids) != 22 {
		t.Fatalf("мест с фото %d", len(ids))
	}
	edtest.Change(e, func(p *model.Project) { p.Sheet(sid).PlaceRef(ids[1]).Role = model.RoleGroup })
	e.SetPlaceFit(sid, []string{ids[2]}, model.FitContain)
	hits := hitsOf(e, sid, func(i int) model.FaceHit {
		h := faceFor(i)
		switch i {
		case 3: // затылок
			h.Box, h.Eye = layout.Rect{}, geom.Pt{}
		case 4: // несколько лиц
			h.Many = true
		case 5: // файла нет
			h.Box, h.Eye, h.Missing = layout.Rect{}, geom.Pt{}, true
		}
		return h
	})
	// После снимка: кадр, фото и поворот меняются — эти места трогать нельзя.
	e.SetFrame(hits[6].Photo, 2, 0.1, 0)
	e.SwapPhotos(hits[7].Photo, hits[8].Photo)
	e.TurnPhotos([]string{hits[9].Photo}, 1)
	before := frames(e)
	_, rep, _ := e.FitFaces(sid, hits, "", design.FacesAll, nil, nil)
	if rep.Fitted != 13 || rep.NoFace != 1 || rep.Many != 1 || rep.Missing != 1 || rep.Contain != 1 || rep.Changed != 4 {
		t.Fatalf("итог %+v", rep)
	}
	// Группового нет в счёте: 21 место, из них «проверьте» — без лица, несколько, нет фото.
	if total(rep) != 21 || !sameSet(rep.Check, ids[3:6]) {
		t.Fatalf("сумма %d, проверьте %v, ждали %v", total(rep), rep.Check, ids[3:6])
	}
	after := frames(e)
	for _, i := range []int{1, 2, 3, 5, 6, 7, 8, 9} { // групповое, «Целиком», без лица, нет файла, сменили
		if id := hits[i].Photo; !reflect.DeepEqual(after[id], before[id]) {
			t.Fatalf("место %d тронуто: %+v → %+v", i, before[id], after[id])
		}
	}
	if after[hits[4].Photo] == before[hits[4].Photo] {
		t.Fatal("кадр у места с несколькими лицами не поставлен")
	}
	if _, ok := rep.Set[hits[3].Photo]; ok || len(rep.Set) != 14 { // 13 подогнанных и место с несколькими лицами
		t.Fatalf("Set: %d кадров", len(rep.Set))
	}
}

func TestFitFacesShort(t *testing.T) {
	e, sid := facesEd(t, 25)
	hits := hitsOf(e, sid, func(i int) model.FaceHit {
		h := faceFor(i)
		switch i {
		case 0: // мелкое лицо: упор в 400 %
			h.Box = layout.Rect{X: 0.47, Y: 0.3, W: 0.04, H: 0.05}
			h.Eye = geom.Pt{X: 0.49, Y: 0.32}
		case 1: // огромное лицо: S уже 1
			h.Box = layout.Rect{X: 0.1, Y: 0.1, W: 0.8, H: 0.8}
			h.Eye = geom.Pt{X: 0.5, Y: 0.35}
		case 2: // у левого края фото
			h.Box = layout.Rect{X: 0.0, Y: 0.3, W: 0.15, H: 0.18}
			h.Eye = geom.Pt{X: 0.075, Y: 0.37}
		}
		return h
	})
	_, rep, _ := e.FitFaces(sid, hits, "", design.FacesAsk, nil, nil)
	if rep.Small != 1 || rep.Big != 1 || rep.Edge != 1 || rep.Fitted != 22 || total(rep) != 25 {
		t.Fatalf("итог %+v", rep)
	}
	if !sameSet(rep.Short, []string{hits[0].Place, hits[1].Place, hits[2].Place}) {
		t.Fatalf("не по норме: %v", rep.Short)
	}
	if f := rep.Set[hits[0].Photo]; f.S != model.MaxScale {
		t.Fatalf("мелкое лицо: S = %v, ждали предел %v", f.S, model.MaxScale)
	}
	if f := rep.Set[hits[1].Photo]; f.S != 1 {
		t.Fatalf("огромное лицо: S = %v", f.S)
	}
}

func TestFitFacesKeepsManual(t *testing.T) {
	e, sid := facesEd(t, 25)
	e.SetFrame("f03", 2.5, 0.3, -0.2) // «на вкус»: лицо заметно крупнее нормы
	hits := hitsOf(e, sid, faceFor)
	manual := frames(e)["f03"]
	rev, undo, was := edtest.Rev(e), edtest.UndoLen(e), jsonOf(e)

	_, ask, _ := e.FitFaces(sid, hits, "", design.FacesAsk, nil, nil)
	edtest.Same(t, e, rev, undo, "вопрос о правленых")
	if !ask.Asked || ask.Manual != 1 || ask.Fitted != 0 || jsonOf(e) != was {
		t.Fatalf("«спросить»: %+v", ask)
	}

	_, keep, _ := e.FitFaces(sid, hits, "", design.FacesKeep, nil, nil)
	if keep.Asked || keep.Manual != 1 || keep.Fitted != 24 || frames(e)["f03"] != manual || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("«keep»: %+v, кадр %+v", keep, frames(e)["f03"])
	}
	if _, in := keep.Set["f03"]; in || total(keep) != 25 {
		t.Fatalf("правленый попал в Set или сумма: %+v", keep)
	}

	e.Undo()
	_, all, _ := e.FitFaces(sid, hits, "", design.FacesAll, nil, nil)
	if all.Manual != 0 || all.Fitted != 25 || frames(e)["f03"] == manual {
		t.Fatalf("«all»: %+v", all)
	}
	// Кадр, поставленный подгоном, — не правленый: его пересчитывают без вопроса.
	_, again, _ := e.FitFaces(sid, hitsOf(e, sid, faceFor), "", design.FacesAsk, nil, all.Set)
	if again.Asked || again.Fitted != 25 {
		t.Fatalf("после подгона: %+v", again)
	}
}

// TestFitFacesManualThresholds — «правленый» меряется порогами 12 % и 1 мм; поворот и наклон не в счёт.
func TestFitFacesManualThresholds(t *testing.T) {
	cases := []struct {
		name   string
		frame  func(f model.Frame) model.Frame
		manual bool
	}{
		{"в пределах", func(f model.Frame) model.Frame { f.S += 0.01; f.X += 0.01; return f }, false},
		{"крупнее на 20 %", func(f model.Frame) model.Frame { f.S *= 1.2; return f }, true},
		{"сдвиг больше 1 мм по X", func(f model.Frame) model.Frame { f.X += 0.2; return f }, true},
		{"сдвиг больше 1 мм по Y", func(f model.Frame) model.Frame { f.Y -= 0.2; return f }, true},
		{"только наклон", func(model.Frame) model.Frame { return model.Frame{S: 1, R: 7.5} }, false},
		{"только четверть", func(model.Frame) model.Frame { return model.Frame{S: 1, R: 90} }, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e, sid := facesEd(t, 1)
			_, first, _ := e.FitFaces(sid, hitsOf(e, sid, faceFor), "", design.FacesAsk, nil, nil)
			f := c.frame(first.Set["f00"])
			edtest.Change(e, func(p *model.Project) {
				r := p.Photos["f00"]
				r.Frame = model.ClampFrame(&f)
				p.Photos["f00"] = r
			})
			_, rep, _ := e.FitFaces(sid, hitsOf(e, sid, faceFor), "", design.FacesAsk, nil, nil)
			if rep.Asked != c.manual {
				t.Fatalf("Asked = %v, ждали %v: %+v", rep.Asked, c.manual, rep)
			}
			if !c.manual && total(rep) != 1 {
				t.Fatalf("без вопроса место не подогнано: %+v", rep)
			}
		})
	}
}

func TestFitFacesEmptyFitted(t *testing.T) {
	e, sid := facesEd(t, 25)
	face := func(i int) model.FaceHit {
		h := faceFor(i)
		if i == 2 { // крохотное лицо: подгон упрётся в 400 %, лицо останется мимо нормы
			h.Box, h.Eye = layout.Rect{X: 0.47, Y: 0.3, W: 0.04, H: 0.05}, geom.Pt{X: 0.49, Y: 0.32}
		}
		return h
	}
	_, first, _ := e.FitFaces(sid, hitsOf(e, sid, face), "", design.FacesAsk, nil, nil)
	if first.Small != 1 || first.Asked {
		t.Fatalf("первый подгон: %+v", first)
	}
	// Перезапуск: память fitted пуста. Спросит только место мимо нормы, остальные в пределах
	// порогов — «наши» и пересчитываются без вопроса.
	_, cold, _ := e.FitFaces(sid, hitsOf(e, sid, face), "", design.FacesAsk, nil, nil)
	if !cold.Asked || cold.Manual != 1 {
		t.Fatalf("после перезапуска: %+v", cold)
	}
	// С памятью сессии вопроса нет и правка холостая.
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	_, warm, _ := e.FitFaces(sid, hitsOf(e, sid, face), "", design.FacesAsk, nil, first.Set)
	edtest.Same(t, e, rev, undo, "повтор с памятью")
	if warm.Asked || warm.Small != 1 || warm.Fitted != 24 {
		t.Fatalf("с памятью: %+v", warm)
	}
}

func TestFitFacesChangedPlace(t *testing.T) {
	e, sid := facesEd(t, 6)
	hits := hitsOf(e, sid, faceFor)
	e.SetFrame("f01", 1.5, 0, 0)     // кадр
	e.SetFlip([]string{"f02"}, "h")  // зеркало кадра не меняет: место не «изменили»
	e.SwapPhotos("f03", "f04")       // два места с другим фото
	e.TurnPhotos([]string{"f05"}, 1) // другие четверти
	_, rep, _ := e.FitFaces(sid, hits, "", design.FacesAll, nil, nil)
	if rep.Changed != 4 || rep.Fitted != 2 || total(rep) != 6 {
		t.Fatalf("итог %+v", rep)
	}
	if _, in := rep.Set["f01"]; in {
		t.Fatal("кадр изменённого места затёрт")
	}
	if f := frames(e)["f01"]; f != (model.Frame{S: 1.5}) {
		t.Fatalf("ручной кадр во время поиска: %+v", f)
	}
}

// TestFitFacesFlip — зеркало: центр лица и глаза на линии по тому, что видно на листе.
func TestFitFacesFlip(t *testing.T) {
	e, sid := facesEd(t, 1)
	e.SetFlip([]string{"f00"}, "h")
	h := hitsOf(e, sid, faceFor)
	v, rep, _ := e.FitFaces(sid, h, "", design.FacesAsk, nil, nil)
	if rep.Fitted != 1 {
		t.Fatalf("итог %+v", rep)
	}
	fv := v.Spreads[0].Frames[0]
	// В файле зеркало внутри Img: лицо на (1 − u).
	cx := fv.Img.X + (1-(h[0].Box.X+h[0].Box.W/2))*fv.Img.W - fv.Win.X - fv.Win.W/2
	eye := (fv.Img.Y + h[0].Eye.Y*fv.Img.H - fv.Win.Y) / fv.Win.H
	if math.Abs(cx) > 0.1 || math.Abs(eye-geom.DefaultFaceNorm.Eye) > 0.002 {
		t.Fatalf("зеркало: сдвиг %.3f мм, глаза %.4f", cx, eye)
	}
}

// TestFitFacesBorderWindow — лицо ставится по окну рамочки, а не по месту (решение 0.27-130-11).
func TestFitFacesBorderWindow(t *testing.T) {
	e, sid := facesEd(t, 3)
	p, _ := e.Snapshot()
	pl := p.Sheet(sid).Grid[0]
	if _, err := e.ApplyBorder(sid, []string{pl.ID}, model.BorderSrc{Kind: "line", Preset: "sa:line"}); err != nil {
		t.Fatal(err)
	}
	e.SetBorder(sid, []string{pl.ID}, model.BorderPatch{Width: 8}, "")
	hits := hitsOf(e, sid, faceFor)
	v, rep, n := e.FitFaces(sid, hits, "", design.FacesAsk, nil, nil)
	if w := v.Spreads[0].Frames[0].Win; !(w.W < pl.W-1) || rep.Fitted != 3 {
		t.Fatalf("окно рамочки %+v у места %v×%v, итог %+v", w, pl.W, pl.H, rep)
	}
	for i, h := range hits {
		wantShare(t, v, i, h, n)
	}
}

func TestFitFacesContract(t *testing.T) {
	e, sid := facesEd(t, 3)
	hits := hitsOf(e, sid, faceFor)
	rev, undo, was := edtest.Rev(e), edtest.UndoLen(e), jsonOf(e)
	// Негодный режим — холостая; негодная норма — как нет нормы; NaN в лице — «без лица».
	_, rep, _ := e.FitFaces(sid, hits, "", "all!", nil, nil)
	edtest.Same(t, e, rev, undo, "негодный режим")
	if total(rep) != 0 {
		t.Fatalf("негодный режим: %+v", rep)
	}
	bad := &geom.FaceNorm{Size: math.NaN(), Eye: 0.3}
	_, rep, n := e.FitFaces(sid, hits, "", design.FacesAsk, bad, nil)
	if n != geom.DefaultFaceNorm || rep.Fitted != 3 {
		t.Fatalf("негодная норма: %+v, %+v", n, rep)
	}
	e.Undo()
	nan := slices.Clone(hits)
	nan[0].Box.H = math.NaN()
	nan[1].Eye.Y = math.Inf(1)
	nan[2].Box = layout.Rect{W: -1, H: -1}
	_, rep, _ = e.FitFaces(sid, nan, "", design.FacesAsk, nil, nil)
	if rep.NoFace != 3 || jsonOf(e) != was {
		t.Fatalf("NaN и пустые рамки: %+v", rep)
	}
}

func TestFitFacesThenSort(t *testing.T) {
	e, sid := facesEd(t, 25)
	for i, who := range []string{"Яблоков", "Абрамов", "Мишин"} {
		e.SetPerson(fmt.Sprintf("f%02d", i), model.Person{Last: who, First: "И"}, "")
	}
	_, rep, _ := e.FitFaces(sid, hitsOf(e, sid, faceFor), "", design.FacesAsk, nil, nil)
	e.SortVignette(sid)
	// Кадр едет с фото, а не с местом: после сортировки у каждого фото прежний кадр.
	now := frames(e)
	for id, f := range rep.Set {
		if now[id] != f {
			t.Fatalf("фото %s: кадр %+v → %+v", id, f, now[id])
		}
	}
	// Фото разошлись по местам, лица у них прежние: повтор по новому снимку холостой.
	snap := hitsOf(e, sid, func(int) model.FaceHit { return model.FaceHit{} })
	for i := range snap {
		f := faceFor(photoNo(snap[i].Photo))
		snap[i].Box, snap[i].Eye = f.Box, f.Eye
	}
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	_, again, _ := e.FitFaces(sid, snap, "", design.FacesAsk, nil, rep.Set)
	edtest.Same(t, e, rev, undo, "повтор после сортировки")
	if again.Asked || again.Fitted != 25 {
		t.Fatalf("после сортировки: %+v", again)
	}
}

// sameSet — те же строки в любом порядке.
func sameSet(a, b []string) bool {
	a, b = slices.Sorted(slices.Values(a)), slices.Sorted(slices.Values(b))
	return slices.Equal(a, b)
}

// photoNo — номер фото "fNN".
func photoNo(id string) int {
	var n int
	_, _ = fmt.Sscanf(id, "f%d", &n)
	return n
}
