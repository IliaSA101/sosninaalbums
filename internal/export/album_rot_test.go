package export

import (
	"image"
	"image/color"
	"math"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/geom"
	"github.com/IliaSA101/sosninaalbums/internal/render"
	"github.com/IliaSA101/sosninaalbums/internal/text"
)

// Угол блока из View доходит до экспорта, подписи не крутятся, а чернила на холсте лежат
// внутри описанного прямоугольника повёрнутой рамки View — так же, как рисует экран.
func TestExportSpreadsRotation(t *testing.T) {
	st := text.Style{Font: "montserrat", Size: 14, Color: "#000000", Align: "left"}
	p := model.New()
	p.Settings.Bg = "#ffffff"
	p.Photos["a"] = model.PhotoRef{Path: `C:\a.jpg`, W: 300, H: 200,
		Caption: &model.Caption{Text: "Подпись", Style: model.DefaultCaptionStyle()}}
	p.Spreads = []*model.Spread{{ID: "s1", Photos: []string{"a"},
		Texts: []*model.TextBlock{{ID: "t1", X: 150, Y: 120, W: 70, Text: "Привет, мир", Rot: 30, Style: st}}}}
	pp, v := mustEditor(t, p, "").Snapshot()

	got := FromView(pp, v)[0]
	if len(got.Texts) != 2 || got.Texts[0].Rot != 0 || got.Texts[1].Rot != 30 {
		t.Fatalf("углы в задании: %+v", got.Texts)
	}

	// Рисуем только блок (фото по несуществующему пути не открыть).
	tv := v.Spreads[0].Texts[0]
	img, err := render.Render(render.Spread{Texts: got.Texts[1:]}, render.Options{
		PageW: v.Settings.PageW, PageH: v.Settings.PageH, DPI: 100})
	if err != nil {
		t.Fatal(err)
	}
	// Описанный прямоугольник рамки из View (поворот — вокруг (x, y)), мм → px.
	s, c := math.Sincos(tv.Rot * math.Pi / 180)
	k := 100 / 25.4
	x0, y0, x1, y1 := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, q := range [4][2]float64{{0, 0}, {tv.W, 0}, {0, tv.H}, {tv.W, tv.H}} {
		X, Y := tv.X+q[0]*c-q[1]*s, tv.Y+q[0]*s+q[1]*c
		x0, y0, x1, y1 = math.Min(x0, X), math.Min(y0, Y), math.Max(x1, X), math.Max(y1, Y)
	}
	frame := image.Rect(int(x0*k)-2, int(y0*k)-2, int(math.Ceil(x1*k))+2, int(math.Ceil(y1*k))+2)
	var ink image.Rectangle
	for y := 0; y < img.Bounds().Dy(); y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			if img.RGBAAt(x, y) != (color.RGBA{255, 255, 255, 255}) {
				ink = ink.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	if ink.Empty() || !ink.In(frame) {
		t.Fatalf("чернила %v вне описанного прямоугольника рамки %v", ink, frame)
	}
	// Повёрнутый текст не горизонтален: по высоте чернила занимают заметно больше одной строки.
	if float64(ink.Dy())/k < tv.H*0.9 {
		t.Fatalf("чернила %v не повёрнуты (высота рамки %.1f мм)", ink, tv.H)
	}
}

// Поворот места, поворот фото и слои из View доходят до экспорта: текст под местом в Order —
// с меньшим Z, чем фото.
func TestExportSpreadsRotZ(t *testing.T) {
	st := text.Style{Font: "montserrat", Size: 14, Color: "#000000", Align: "left"}
	p := model.New()
	p.Photos["a"] = model.PhotoRef{Path: `C:\a.jpg`, W: 300, H: 200, Frame: &model.Frame{S: 1, R: 100}}
	p.Photos["b"] = model.PhotoRef{Path: `C:\b.jpg`, W: 300, H: 200}
	p.Spreads = []*model.Spread{{ID: "s1",
		Grid: []*model.Place{
			{ID: "p1", X: 40, Y: 40, W: 60, H: 60, Photo: "a", Rot: 30},
			{ID: "p2", X: 150, Y: 40, W: 60, H: 40, Photo: "b"},
		},
		Texts: []*model.TextBlock{{ID: "t1", X: 50, Y: 50, W: 40, Text: "Под фото", Style: st}},
		Order: []string{"t1", "p1", "p2"},
	}}
	pp, v := mustEditor(t, p, "").Snapshot()
	sv := v.Spreads[0]
	got := FromView(pp, v)[0]
	if len(got.Photos) != 2 || len(got.Texts) != 1 {
		t.Fatalf("задание: %+v", got)
	}
	for i, ph := range got.Photos {
		f := sv.Frames[i]
		if ph.Rot != f.CellRot || ph.ImgRot != f.R || ph.Z != f.Z || ph.Img != f.Img {
			t.Fatalf("фото %d: %+v, во View %+v", i, ph, f)
		}
	}
	if a := got.Photos[0]; a.Rot != 30 || a.ImgRot != 100 {
		t.Fatalf("углы места и фото: %v, %v", a.Rot, a.ImgRot)
	}
	if tz := got.Texts[0].Z; tz != sv.Texts[0].Z || tz >= got.Photos[0].Z {
		t.Fatalf("Z текста %d, фото %d, во View %d", tz, got.Photos[0].Z, sv.Texts[0].Z)
	}
}

// Наклон фото фона (045) доходит до экспорта из View: ImgRot = Frame.R, Img — уже увеличенный
// кадр View; фон без наклона — ImgRot 0 (прежний путь рендера).
func TestExportBackdropTilt(t *testing.T) {
	p := model.New()
	p.Photos["a"] = model.PhotoRef{Path: `C:\a.jpg`, W: 300, H: 200}
	p.Photos["l"] = model.PhotoRef{Path: `C:\l.jpg`, W: 6000, H: 4000}
	p.Photos["r"] = model.PhotoRef{Path: `C:\r.jpg`, W: 6000, H: 4000}
	p.Spreads = []*model.Spread{{ID: "s1", Photos: []string{"a"},
		BgPhotoL: &model.Backdrop{Photo: "l", W: 300, H: 300, Rot: 10, Frame: &model.Frame{S: 1, R: 4.5}},
		BgPhotoR: &model.Backdrop{Photo: "r", X: 300, W: 300, H: 300, Frame: &model.Frame{S: 1.5}}}}
	pp, v := mustEditor(t, p, "").Snapshot()
	sv := v.Spreads[0]
	got := FromView(pp, v)[0].Backdrops
	if len(got) != 2 || len(sv.Backdrops) != 2 {
		t.Fatalf("фоны: %+v", got)
	}
	for i, b := range got {
		bv := sv.Backdrops[i]
		if b.ImgRot != bv.Frame.R || b.Img != bv.Img || b.Rot != bv.Box.Rot {
			t.Fatalf("фон %d: %+v, во View %+v", i, b, bv)
		}
	}
	if got[0].ImgRot != 4.5 || got[0].Rot != 10 || got[1].ImgRot != 0 {
		t.Fatalf("наклон фонов: %v, %v; рамка %v", got[0].ImgRot, got[1].ImgRot, got[0].Rot)
	}
}

// Форма места (044) из View доходит до задания: круг, скругление в мм, прямоугольник — нулевая.
func TestExportSpreadsShape(t *testing.T) {
	p := model.New()
	for _, id := range []string{"a", "b", "c"} {
		p.Photos[id] = model.PhotoRef{Path: `C:\` + id + `.jpg`, W: 300, H: 200}
	}
	p.Spreads = []*model.Spread{{ID: "s1", Photos: []string{"a", "b", "c"}, Grid: []*model.Place{
		{ID: "000000000001", X: 10, Y: 10, W: 80, H: 80, Photo: "a", Shape: model.ShapeOval},
		{ID: "000000000002", X: 100, Y: 10, W: 100, H: 60, Photo: "b", Round: 10},
		{ID: "000000000003", X: 210, Y: 10, W: 60, H: 60, Photo: "c"},
	}}}
	pp, v := mustEditor(t, p, "").Snapshot()
	got := FromView(pp, v)[0].Photos
	if len(got) != 3 || !got[0].Outline.Oval || got[0].Outline.Radius != 0 ||
		got[1].Outline.Oval || got[1].Outline.Radius != 6 || got[2].Outline != (geom.Outline{}) {
		t.Fatalf("формы в задании: %+v", got)
	}
}
