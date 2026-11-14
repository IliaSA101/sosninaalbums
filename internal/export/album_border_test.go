package export

import (
	"image/color"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/render"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Рамочки мест в экспорте (079 G4): окно фото, слой рамочки, маска окна и файлы рамочек-картинок
// в плане (пропавшая своя останавливает выгрузку; рамочка пустого места не печатается).

const (
	brKey   = "my:00112233445566ff"
	placeA  = "000000000001"
	placeB  = "000000000002"
	brSheet = "s1"
)

// borderAlbum — разворот с двумя местами: A с фото, B пустое. Стили файлов — фиктивные пути.
func borderAlbum(rot float64) *model.Project {
	p := model.New()
	p.Photos["a"] = model.PhotoRef{Path: testkit.AbsPath("a.jpg"), W: 300, H: 200}
	p.Spreads = []*model.Spread{{ID: brSheet, Photos: []string{"a"}, Grid: []*model.Place{
		{ID: placeA, X: 20, Y: 10, W: 60, H: 40, Photo: "a", Rot: rot},
		{ID: placeB, X: 100, Y: 10, W: 60, H: 40},
	}}}
	return p
}

// ownFrame — запись своей рамочки-картинки для ApplyBorder.
func ownFrame(path string) model.PhotoInfo {
	return model.PhotoInfo{ID: model.LibPhotoID(brKey), Path: path, Lib: brKey, W: 3000, H: 4000}
}

// Встроенная рамочка: окно — Cell фото (Img посчитан для него), место — Pivot, слой рамочки — Z
// места, цвета и толщина действующие; пустое место — без слоя; мягкий край — маска без слоя.
func TestExportBorderBuiltin(t *testing.T) {
	p := borderAlbum(0)
	e := mustEditor(t, p, "")
	if _, err := e.ApplyBorder(brSheet, []string{placeA, placeB}, model.BorderSrc{Kind: "line"}); err != nil {
		t.Fatal(err)
	}
	pp, v := e.Snapshot()
	es := FromView(pp, v)[0]
	sv := v.Spreads[0]
	if len(es.Photos) != 1 || len(es.Borders) != 1 {
		t.Fatalf("фото %d, рамочек %d (пустое место без рамочки): %+v", len(es.Photos), len(es.Borders), es)
	}
	ph, b := es.Photos[0], es.Borders[0]
	place := layout.Rect{X: 20, Y: 10, W: 60, H: 40}
	if !nearRect(ph.Cell, sv.Frames[0].Win) || nearRect(ph.Cell, place) || ph.Pivot != place || ph.Img != sv.Frames[0].Img {
		t.Fatalf("окно %v, место %v, Pivot %v, Img %v", ph.Cell, place, ph.Pivot, ph.Img)
	}
	// Линия 1,5 мм: окно уже места на 1,5 мм со всех сторон.
	if !nearRect(ph.Cell, layout.Rect{X: 21.5, Y: 11.5, W: 57, H: 37}) {
		t.Errorf("окно линии: %v", ph.Cell)
	}
	if b.Box != place || b.Kind != "line" || b.Width != 1.5 || len(b.Inks) != 1 || b.Pic || b.Z != sv.Frames[0].Z || b.Z != ph.Z {
		t.Errorf("рамочка: %+v, кадр Z %d", b, sv.Frames[0].Z)
	}
	if ph.Win != nil {
		t.Errorf("маска у вида без маски: %+v", ph.Win)
	}

	// Мягкий край — маска у фото, слоя рамочки нет.
	e = mustEditor(t, borderAlbum(25), "")
	if _, err := e.ApplyBorder(brSheet, []string{placeA}, model.BorderSrc{Kind: "soft"}); err != nil {
		t.Fatal(err)
	}
	pp, v = e.Snapshot()
	es = FromView(pp, v)[0]
	w := es.Photos[0].Win
	if len(es.Borders) != 0 || w == nil || w.Kind != "soft" || w.Width != 8 || w.Rot != 25 || w.Box != place || es.Photos[0].Rot != 25 {
		t.Errorf("мягкий край: слоёв %d, маска %+v", len(es.Borders), w)
	}
}

// Неизвестный вид (файл новее): окно = место, слоя нет; фото без рамочки — задание прежнее.
func TestExportBorderUnknownKind(t *testing.T) {
	p := borderAlbum(0)
	p.Spreads[0].Grid[0].Border = &model.Border{Kind: "zz-future"}
	pp, v := mustEditor(t, p, "").Snapshot()
	es := FromView(pp, v)[0]
	place := layout.Rect{X: 20, Y: 10, W: 60, H: 40}
	if len(es.Borders) != 0 || es.Photos[0].Cell != place || es.Photos[0].Win != nil || es.Photos[0].Pivot != (layout.Rect{}) {
		t.Errorf("неизвестный вид: %+v", es)
	}
}

// Рамочка-картинка: путь записи альбома, срез и размер — из альбома; Open — без маски окна.
func TestExportBorderPicture(t *testing.T) {
	file := filepath.Join(testkit.TempDir(t), "fr.png")
	cut := [4]float64{0.14, 0.2, 0.14, 0.1}
	for _, open := range []bool{false, true} {
		e := mustEditor(t, borderAlbum(0), "")
		if _, err := e.ApplyBorder(brSheet, []string{placeA}, model.BorderSrc{Photo: ownFrame(file), Cut: cut, Open: open}); err != nil {
			t.Fatal(err)
		}
		pp, v := e.Snapshot()
		es := FromView(pp, v)[0]
		if len(es.Borders) != 1 {
			t.Fatalf("open=%v: слоёв %d", open, len(es.Borders))
		}
		b, ph := es.Borders[0], es.Photos[0]
		wantCut := pp.Spreads[0].Grid[0].Border.Cut
		if wantCut != cut {
			t.Fatalf("срез в альбоме %v, ждали %v", wantCut, cut)
		}
		if !b.Pic || b.Path != file || b.Cut != wantCut || b.Scale != 1 || b.Box != ph.Pivot {
			t.Errorf("open=%v: рамочка %+v", open, b)
		}
		if open != (ph.Win == nil) {
			t.Errorf("open=%v: маска окна %+v", open, ph.Win)
		}
		if ph.Win != nil && (ph.Win.Path != file || ph.Win.Cut != cut || ph.Win.Box != b.Box) {
			t.Errorf("маска: %+v", ph.Win)
		}
	}
}

// Круг: от задания до пикселей — полоса цвета рамочки на месте, фото в окне, прозрачность фото.
func TestExportBorderRender(t *testing.T) {
	dir := testkit.TempDir(t)
	p := borderAlbum(0)
	p.Photos["a"] = model.PhotoRef{Path: testkit.JPEG(t, dir, "a.jpg", 300, 200), W: 300, H: 200}
	e := mustEditor(t, p, "")
	if _, err := e.ApplyBorder(brSheet, []string{placeA}, model.BorderSrc{Kind: "line", Inks: []string{"#c8201e"}, Width: 4}); err != nil {
		t.Fatal(err)
	}
	e.SetPhotoFade([]string{"a"}, 0.5, "")
	pp, v := e.Snapshot()
	opt := render.Options{PageW: pp.Settings.PageW, PageH: pp.Settings.PageH, DPI: 25}
	img, err := render.Render(FromView(pp, v)[0], opt)
	if err != nil {
		t.Fatal(err)
	}
	k := 25 / 25.4
	at := func(x, y float64) color.RGBA { return img.RGBAAt(int(x*k), int(y*k)) }
	if c := at(22, 30); !nearC(c, color.RGBA{0xc8, 0x20, 0x1e, 255}, 3) {
		t.Errorf("полоса: %v", c)
	}
	bgc := render.ParseHex(v.Spreads[0].Bg[0])
	c := at(50, 30)
	if nearC(c, bgc, 6) || nearC(c, color.RGBA{0xc8, 0x20, 0x1e, 255}, 6) {
		t.Errorf("окно: %v — ждали фото, смешанное со страницей %v", c, bgc)
	}
	if c := at(10, 30); !nearC(c, bgc, 2) {
		t.Errorf("снаружи: %v", c)
	}
}

func nearC(a, b color.RGBA, tol int) bool {
	d := func(x, y uint8) int { return max(int(x)-int(y), int(y)-int(x)) }
	return d(a.R, b.R) <= tol && d(a.G, b.G) <= tol && d(a.B, b.B) <= tol
}

// Файл рамочки-картинки в плане: пропавшая своя останавливает выгрузку со своим текстом, пустое
// место рамочки не печатает (файл не нужен), цель-страница берёт рамочку по описанной рамке.
func TestExportBorderMissing(t *testing.T) {
	const pageW = 300.0
	frame := func(photo string, x, rot float64, border bool, ph string) model.PlaceView {
		pv := model.PlaceView{ID: "p" + photo, X: x, Y: 10, W: 60, H: 40, Rot: rot, Photo: ph}
		if border {
			pv.Border = &model.BorderView{Pic: true, Photo: photo, Known: true}
		}
		return pv
	}
	v := model.View{Spreads: []model.SpreadView{
		{Grid: true, Photos: []string{"x1"}, Rects: []layout.Rect{{X: 10, Y: 10, W: 60, H: 40}},
			Places: []model.PlaceView{frame("fl", 10, 0, true, "x1"), frame("fe", 100, 0, true, "")}}, // левая; пустое место
		{Grid: true, Photos: []string{"x2"}, Rects: []layout.Rect{{X: 280, Y: 10, W: 60, H: 40}},
			Places: []model.PlaceView{frame("fc", 280, 0, true, "x2")}}, // через шов
		{Grid: true, Photos: []string{"x3"}, Rects: []layout.Rect{{X: 400, Y: 10, W: 60, H: 40}},
			Places: []model.PlaceView{frame("fr", 400, 0, true, "x3")}}, // правая
	}}
	keys := func(m map[string]bool) []string {
		var out []string
		for k := range m {
			out = append(out, k)
		}
		slices.Sort(out)
		return out
	}
	pl := func(spec string) Plan {
		u, err := ParseRange(spec, 3)
		if err != nil {
			t.Fatal(err)
		}
		return NewPlan(u, 3, true)
	}
	for _, c := range []struct {
		name string
		plan Plan
		want []string
	}{
		{"все", NewPlan(nil, 3, false), []string{"fc", "fl", "fr", "x1", "x2", "x3"}},
		{"1л", pl("1л"), []string{"fl", "x1"}},
		{"2л", pl("2л"), []string{"fc", "x2"}},
		{"3п", pl("3п"), []string{"fr", "x3"}},
	} {
		if got := keys(c.plan.SelectedPhotos(v, pageW)); !slices.Equal(got, c.want) {
			t.Errorf("%s: %v, ждали %v", c.name, got, c.want)
		}
	}
	if sel := NewPlan(nil, 3, false).SelectedPhotos(v, pageW); sel["fe"] {
		t.Error("рамочка пустого места попала в план")
	}

	// Пропавшая своя рамочка (в библиотеке нет, путь пуст) и пустое место с такой же рамочкой.
	p := borderAlbum(0)
	e := mustEditor(t, p, "")
	if _, err := e.ApplyBorder(brSheet, []string{placeA, placeB}, model.BorderSrc{Photo: ownFrame("")}); err != nil {
		t.Fatal(err)
	}
	pp, vv := e.Snapshot()
	sel := NewPlan(nil, 1, false).SelectedPhotos(vv, pageW)
	id := model.LibPhotoID(brKey)
	if !sel[id] {
		t.Fatalf("рамочка места с фото не в плане: %v", sel)
	}
	err := MissingPhotos(pp, sel, func(string) bool { return true })
	if err == nil || !strings.Contains(err.Error(), "нет в библиотеке этого компьютера: 1 (своя рамочка); загрузите те же файлы в галерею рамочек") {
		t.Errorf("пропавшая своя рамочка: %v", err)
	}
	// Снять фото с места A: остаётся рамочка только пустого места — файл не нужен.
	pp.Spreads[0].Grid[0].Photo = ""
	pp.Spreads[0].Photos = nil
	_, vv = mustEditor(t, pp, "").Snapshot()
	if sel := NewPlan(nil, 1, false).SelectedPhotos(vv, pageW); sel[id] {
		t.Errorf("рамочка пустого места держит выгрузку: %v", sel)
	}
}

// То же на обложке: рамочка места обложки с фото — в плане с обложкой, без обложки — нет.
func TestExportBorderMissingCover(t *testing.T) {
	cv := model.SpreadView{Grid: true, Photos: []string{"c1"}, Rects: []layout.Rect{{X: 10, Y: 10, W: 60, H: 40}},
		Places: []model.PlaceView{
			{ID: "pc", X: 10, Y: 10, W: 60, H: 40, Photo: "c1", Border: &model.BorderView{Pic: true, Photo: "fc", Known: true}},
			{ID: "pe", X: 100, Y: 10, W: 60, H: 40, Border: &model.BorderView{Pic: true, Photo: "fe", Known: true}}}}
	v := model.View{Cover: &model.CoverView{Sheet: cv}}
	with := NewPlan(nil, 0, false).WithCover(&CoverJob{}).SelectedPhotos(v, 300)
	if !with["fc"] || !with["c1"] || with["fe"] {
		t.Errorf("с обложкой: %v", with)
	}
	if without := NewPlan(nil, 0, false).SelectedPhotos(v, 300); len(without) != 0 {
		t.Errorf("без обложки: %v", without)
	}
	// Пропавшая своя рамочка обложки стоит выгрузку и называется рамочкой.
	p := model.New()
	id := model.LibPhotoID(brKey)
	p.Photos["c1"] = model.PhotoRef{Path: testkit.AbsPath("c.jpg"), W: 4, H: 3}
	p.Photos[id] = model.PhotoRef{Lib: brKey, W: 3000, H: 4000}
	p.Cover = &model.Cover{Sheet: &model.Spread{ID: model.CoverID, Photos: []string{"c1"}, Grid: []*model.Place{
		{ID: "000000000009", X: 10, Y: 10, W: 60, H: 40, Photo: "c1", Border: &model.Border{Photo: id}}}}}
	err := MissingPhotos(p, map[string]bool{id: true}, func(string) bool { return true })
	if err == nil || !strings.Contains(err.Error(), "(своя рамочка)") {
		t.Errorf("обложка: %v", err)
	}
}

// Пустое место с рамочкой: слоя в задании нет (как само место), остальное задание прежнее.
func TestExportEmptyPlaceNoBorder(t *testing.T) {
	p := borderAlbum(0)
	e := mustEditor(t, p, "")
	if _, err := e.ApplyBorder(brSheet, []string{placeB}, model.BorderSrc{Kind: "double"}); err != nil {
		t.Fatal(err)
	}
	pp, v := e.Snapshot()
	if v.Spreads[0].Places[1].Border == nil {
		t.Fatal("в View нет рамочки пустого места")
	}
	es := FromView(pp, v)[0]
	if len(es.Borders) != 0 || es.Photos[0].Cell != (layout.Rect{X: 20, Y: 10, W: 60, H: 40}) || es.Photos[0].Win != nil {
		t.Errorf("рамочка пустого места попала в задание: %+v", es)
	}
}

// Несколько видов своего пропали сразу: тексты в общем виде («свои фоны, стикеры и рамочки»).
func TestMissingPhotosLibBorderMix(t *testing.T) {
	p := selectProject()
	k := map[string]string{"bg": "my:0123456789abcdef", "st": "my:fedcba9876543210", "br": "my:00000000000000aa", "br2": "my:00000000000000bb"}
	for id, key := range k {
		p.Photos[id] = model.PhotoRef{Lib: key, W: 4, H: 3}
	}
	p.Spreads = []*model.Spread{{ID: "s1", BgPhoto: &model.Backdrop{Photo: "bg"},
		Stickers: []*model.Sticker{{ID: "st1", Photo: "st"}},
		Grid: []*model.Place{{ID: "000000000001", Photo: "a", Border: &model.Border{Photo: "br"}},
			{ID: "000000000002", Photo: "a", Border: &model.Border{Photo: "br2"}}}}}
	for _, c := range []struct {
		ids  []string
		want string
	}{
		{[]string{"br"}, "(своя рамочка); загрузите те же файлы в галерею рамочек, они"},
		{[]string{"br", "br2"}, "(свои рамочки); загрузите те же файлы в галерею рамочек, они"},
		{[]string{"bg", "br"}, "(свои фоны и рамочки); загрузите те же файлы в галереи фонов и рамочек"},
		{[]string{"st", "br"}, "(свои стикеры и рамочки); загрузите те же файлы в галереи стикеров и рамочек"},
		{[]string{"bg", "st", "br"}, "(свои фоны, стикеры и рамочки); загрузите те же файлы в галереи фонов, стикеров и рамочек"},
	} {
		sel := map[string]bool{}
		for _, id := range c.ids {
			sel[id] = true
		}
		if err := MissingPhotos(p, sel, func(string) bool { return true }); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%v: %v", c.ids, err)
		}
	}
}

// Прозрачность фото места доезжает до задания и без рамочки; рамочки нет — окно = место, слоя нет.
func TestExportPhotoFade(t *testing.T) {
	e := mustEditor(t, borderAlbum(0), "")
	e.SetPhotoFade([]string{"a"}, 0.3, "")
	pp, v := e.Snapshot()
	es := FromView(pp, v)[0]
	if len(es.Borders) != 0 || es.Photos[0].Fade != 0.3 || es.Photos[0].Win != nil || es.Photos[0].Pivot != (layout.Rect{}) ||
		es.Photos[0].Cell != (layout.Rect{X: 20, Y: 10, W: 60, H: 40}) {
		t.Errorf("прозрачность: %+v", es.Photos[0])
	}
}
