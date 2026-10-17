package edtest

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/library"
)

// Open — album.NewEditor для тестов: отказ проверки проекта — Fatal.
func Open(t testing.TB, p *model.Project, path string) *album.Editor {
	t.Helper()
	e, err := album.NewEditor(p, path)
	if err != nil {
		t.Fatal(err)
	}
	return e
}

// Abs — локальный абсолютный путь для проекта, собранного в тесте: NewEditor и Restore
// проверяют пути (salbum.Check), а «/a.jpg» на Windows не локальный (нет диска).
func Abs(name string) string {
	return filepath.Join(filepath.VolumeName(os.TempDir())+string(filepath.Separator), name)
}

// BdEd — альбом из двух разворотов (a, b | c) и фото "bg" вне мест.
func BdEd(t *testing.T) (*album.Editor, string, string) {
	t.Helper()
	e, s1, s2 := NewTwo(t)
	return e, s1, s2
}

func SetBg(t *testing.T, e *album.Editor, sp, scope, id string) model.View {
	t.Helper()
	v, err := e.SetBackdrop(sp, scope, Photo(id))
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func BdOf(e *album.Editor, sp, scope string) *model.Backdrop {
	Mu(e).Lock()
	defer Mu(e).Unlock()
	_, s := P(e).Find(sp)
	return *s.Slot(scope)
}

// AutoEd — редактор с одним «Авто»-разворотом из трёх фото разных пропорций.
func AutoEd(t *testing.T) (e *album.Editor, id string) {
	t.Helper()
	e = Open(t, model.New(), "")
	ph := []model.PhotoInfo{
		{ID: "a", Path: Abs("a.jpg"), W: 300, H: 200},
		{ID: "b", Path: Abs("b.jpg"), W: 200, H: 300},
		{ID: "c", Path: Abs("c.jpg"), W: 300, H: 300},
	}
	v := NewSpread(e, -1, ph)
	return e, v.Spreads[0].ID
}

// Gp — место для тестов (ID проставит GridEd).
func Gp(x, y, w, h float64, photo string) *model.Place {
	return &model.Place{X: x, Y: y, W: w, H: h, Photo: photo}
}

// GridEd — редактор с разворотом-сеткой "s1" (страница 281×284, как в design):
// места по порядку, фото 3:2 по местам. extra — следующие развороты «Авто».
func GridEd(t *testing.T, places []*model.Place, extra ...[]string) *album.Editor {
	t.Helper()
	p := model.New()
	p.Settings.PageW, p.Settings.PageH = 281, 284
	s := &model.Spread{ID: "s1", Photos: []string{}, Grid: places}
	ref := func(id string) { p.Photos[id] = model.PhotoRef{Path: Abs(id + ".jpg"), W: 300, H: 200} }
	for i, pl := range places {
		if pl.ID == "" {
			pl.ID = fmt.Sprintf("%012x", i+1)
		}
		if pl.Photo != "" {
			ref(pl.Photo)
			s.Photos = append(s.Photos, pl.Photo)
		}
	}
	if len(places) == 0 {
		s.Grid = nil
	}
	p.Spreads = []*model.Spread{s}
	for i, ids := range extra {
		for _, id := range ids {
			ref(id)
		}
		p.Spreads = append(p.Spreads, &model.Spread{ID: fmt.Sprintf("s%d", i+2), Photos: slices.Clone(ids)})
	}
	e := Open(t, p, "")
	MustGrid(t, e)
	return e
}

// CoverPlace — ID места на лицевой обложки CoverEd.
const CoverPlace = "c0ffee0000c1"

// CoverEd — GridEd с обложкой «тонкие листы» (сторонка — страница 281×284): на лицевой
// место CoverPlace с фото cover и текст; развороты — places и extra, как у GridEd.
func CoverEd(t *testing.T, cover string, places []*model.Place, extra ...[]string) *album.Editor {
	t.Helper()
	e := GridEd(t, places, extra...)
	pr, _ := model.CoverPresetOf(model.CoverDefault)
	Change(e, func(p *model.Project) {
		p.Photos[cover] = model.PhotoRef{Path: Abs(cover + ".jpg"), W: 300, H: 200}
		p.Cover = &model.Cover{Spec: pr.Spec, Sheet: &model.Spread{ID: model.CoverID, Photos: []string{cover},
			Grid:  []*model.Place{{ID: CoverPlace, X: 400, Y: 100, W: 120, H: 80, Photo: cover}},
			Texts: []*model.TextBlock{{ID: "c0ffee0000c2", X: 350, Y: 40, W: 80, Text: "Обложка", Style: model.PlainStyle()}}}}
	})
	MustGrid(t, e)
	return e
}

// PhotosOf — фото мест разворота i по порядку ("" — пустое).
func PhotosOf(e *album.Editor, i int) []string {
	var out []string
	for _, pl := range P(e).Spreads[i].Grid {
		out = append(out, pl.Photo)
	}
	return out
}

// Несимметричная левая страница (большое и широкое место) и место справа.
func Calendar() []*model.Place {
	return []*model.Place{Gp(12, 12, 127, 127, "a"), Gp(12, 142, 200, 130, "c"), Gp(293, 12, 127, 127, "")}
}

func Ip(v int) *int { return &v }

func Sp(v string) *string { return &v }

func SetMarks(t *testing.T, e *album.Editor, ph []model.PhotoInfo, pt model.MarkPatch) model.View {
	t.Helper()
	v, err := e.SetMarks(ph, pt)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

// MarkRows — Marks() в виде, удобном для сравнения: nil-поле — -9 / "-".
func MarkRows(e *album.Editor) map[string][3]any {
	out := map[string][3]any{}
	for id, m := range e.Marks() {
		r := [3]any{-9, "-", -9}
		if m.Rating != nil {
			r[0] = *m.Rating
		}
		if m.Label != nil {
			r[1] = *m.Label
		}
		if m.Flag != nil {
			r[2] = *m.Flag
		}
		out[id] = r
	}
	return out
}

func WantMarks(t *testing.T, e *album.Editor, want map[string][3]any) {
	t.Helper()
	if got := MarkRows(e); !reflect.DeepEqual(got, want) {
		t.Fatalf("отметки %v, ждали %v", got, want)
	}
}

func Photo(id string) model.PhotoInfo {
	return model.PhotoInfo{ID: id, Path: Abs(id + ".jpg"), W: 300, H: 200}
}

func Photos(ids ...string) []model.PhotoInfo {
	out := make([]model.PhotoInfo, len(ids))
	for i, id := range ids {
		out[i] = Photo(id)
	}
	return out
}

// Обёртки для тестов, где отказ не ждут: ошибка — сразу фатальная паника.
func NewSpread(e *album.Editor, at int, ph []model.PhotoInfo) model.View {
	v, err := e.NewSpread(at, ph)
	if err != nil {
		panic(err)
	}
	return v
}

func AddPhotos(e *album.Editor, id string, ph []model.PhotoInfo, idx int) model.View {
	v, err := e.AddPhotos(id, ph, idx)
	if err != nil {
		panic(err)
	}
	return v
}

func AddFolder(e *album.Editor, dir string) model.View {
	v, err := e.AddFolder(dir)
	if err != nil {
		panic(err)
	}
	return v
}

// NewTwo — проект с двумя разворотами: [a b] и [c].
func NewTwo(t *testing.T) (*album.Editor, string, string) {
	t.Helper()
	e := Open(t, model.New(), "")
	NewSpread(e, -1, Photos("a", "b"))
	v := NewSpread(e, -1, Photos("c"))
	if len(v.Spreads) != 2 {
		t.Fatalf("развороты: %d", len(v.Spreads))
	}
	return e, v.Spreads[0].ID, v.Spreads[1].ID
}

// RowsOf — ряд «в порядке ленты» разворота i.
func RowsOf(e *album.Editor, i int) []layout.Result {
	s := P(e).Spreads[i]
	return layout.Cached(P(e).Aspects(s.Photos), P(e).Params(), model.VariantLimit)
}

// PickKey — не лучшая раскладка разворота i, подходящая под условие.
func PickKey(t *testing.T, e *album.Editor, i int, ok func(string) bool) string {
	t.Helper()
	for _, r := range RowsOf(e, i)[1:] {
		if ok(r.Key) {
			return r.Key
		}
	}
	t.Fatalf("нет подходящей раскладки среди %d", len(RowsOf(e, i)))
	return ""
}

func AnyKey(string) bool { return true }

func RectOf(v model.View, spread int, id string) layout.Rect {
	s := v.Spreads[spread]
	return s.Rects[slices.Index(s.Photos, id)]
}

// StyleAlign — блок id с выравниванием align (SetTextStyle сам держит «по тексту» на странице).
func StyleAlign(t *testing.T, e *album.Editor, id, align string) {
	t.Helper()
	st := Block(t, e, id).Style
	st.Align = align
	e.SetTextStyle(id, st)
}

// Block — блок проекта по ID (для проверок полей, которых нет в View).
func Block(t *testing.T, e *album.Editor, id string) *model.TextBlock {
	t.Helper()
	s, i := P(e).FindText(id)
	if s == nil {
		t.Fatalf("блока %s нет", id)
	}
	return s.Texts[i]
}

// AddText — новый блок на странице page разворота sid, его ID.
func AddText(t *testing.T, e *album.Editor, sid string, page int) string {
	t.Helper()
	v, err := e.AddText(sid, page, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range v.Spreads {
		if s.ID == sid {
			return s.Texts[len(s.Texts)-1].ID
		}
	}
	t.Fatalf("разворота %s нет", sid)
	return ""
}

// FreezeAt закрепляет разворот i одной правкой.
func FreezeAt(t *testing.T, e *album.Editor, i int) {
	t.Helper()
	Change(e, func(p *model.Project) { p.Freeze(p.Spreads[i]) })
	if err := GridInvariants(P(e)); err != nil {
		t.Fatal(err)
	}
	if len(P(e).Spreads[i].Grid) == 0 {
		t.Fatal("разворот не закрепился")
	}
}

// PlaceIDsOf — ID мест первого разворота снизу вверх.
func PlaceIDsOf(e *album.Editor) []string {
	var out []string
	for _, pl := range P(e).Spreads[0].Grid {
		out = append(out, pl.ID)
	}
	return out
}

// Lib — фото ленты по абсолютному пути (ID как у сканера).
func Lib(path string, x model.XmpMarks) model.PhotoInfo {
	return model.PhotoInfo{ID: library.IDFor(path), Path: path, W: 300, H: 200, Xmp: x}
}

// StackOf — общий стек первого разворота снизу вверх.
func StackOf(e *album.Editor) []string { return P(e).Spreads[0].Stack() }

// TextAt — блок текста на развороте sid с рамкой у (x, y): перекрытие задаёт тест.
func TextAt(t *testing.T, e *album.Editor, sid string, x, y float64) string {
	t.Helper()
	id := AddText(t, e, sid, 0)
	e.SetTextBox(id, x, y, 80)
	return id
}

func SpreadIDs(v model.View) []string {
	ids := make([]string, len(v.Spreads))
	for i, s := range v.Spreads {
		ids[i] = s.ID
	}
	return ids
}

// SheetOf — ID листа (обложка или разворот), где стоит фото id; "" — нигде.
func SheetOf(e *album.Editor, id string) string {
	for _, s := range P(e).Sheets() {
		if slices.Contains(s.Photos, id) {
			return s.ID
		}
	}
	return ""
}

// Tilt — SetFrameAngle без сессии на листе фото id (057: лист нужен правилу размера).
func Tilt(e *album.Editor, id string, tilt float64) model.View {
	return e.SetFrameAngle(SheetOf(e, id), id, tilt, "")
}
