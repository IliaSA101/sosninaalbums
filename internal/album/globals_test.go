package album_test

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"log"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/album/vignette"
	"github.com/IliaSA101/sosninaalbums/internal/geom"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// В рабочем коде дерева альбома нет экспортных переменных уровня пакета, кроме ошибок Err*:
// крючок теста, ставший экспортным, дал бы app/export менять поведение ядра в обход Editor
// (023 Ш4з). Подмена в тестах — строчная переменная и доступ через export_test.go, inside
// или сеттер модели (forbidigo пускает его только в тесты). internal/inside — сам механизм
// доступа (поля-функции ставит ядро в init), internal/edtest — только для тестов.
func TestNoExportedGlobals(t *testing.T) {
	skip := map[string]bool{"testdata": true, "inside": true, "edtest": true}
	fset := token.NewFileSet()
	var bad []string
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if skip[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		f, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		for _, name := range exportedVars(f) {
			bad = append(bad, filepath.ToSlash(path)+": "+name)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(bad) > 0 {
		t.Errorf("экспортные переменные пакета (нужна строчная + доступ для тестов):\n%s", strings.Join(bad, "\n"))
	}
}

// TestPageWReaders — страж 032 §1: Settings.PageW в рабочем коде internal/ читают только
// model/sheet.go (Geo), ClampSettings и Params. Остальной код листа берёт ширину и страницы
// из Geo: иначе обложка и лист 031 тихо получили бы размеры разворота. Запись (тесты,
// настройки) и поле литерала — не чтение.
func TestPageWReaders(t *testing.T) {
	allowed := map[string]bool{"album/model/sheet.go": true, "album/model/limits.go:ClampSettings": true, "album/model/project.go:Params": true}
	fset := token.NewFileSet()
	var bad []string
	err := filepath.WalkDir("..", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		rel := strings.TrimPrefix(filepath.ToSlash(path), "../")
		if allowed[rel] {
			return nil
		}
		f, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		for _, at := range pageWReads(fset, f, func(fn string) bool { return allowed[rel+":"+fn] }) {
			bad = append(bad, rel+":"+at)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(bad) > 0 {
		t.Errorf("Settings.PageW читают мимо Geo (model/sheet.go):\n%s", strings.Join(bad, "\n"))
	}
	// Сам сканер не слепой: чтение через поле и через копию настроек находит, запись — нет.
	src := `package x
func f(p *P, s Settings) float64 { st := p.Settings; p.Settings.PageW = 1; return st.PageW + p.Settings.PageW + s.PageW }
func (s Settings) g() float64 { d := model.DefaultSettings(); return s.PageW + d.PageW }`
	sf, err := parser.ParseFile(fset, "x.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got := pageWReads(fset, sf, func(string) bool { return false }); len(got) != 5 {
		t.Fatalf("сканер нашёл %v, ждали пять чтений", got)
	}
}

// pageWReads — чтения X.PageW, где X — настройки проекта: поле Settings, параметр или
// переменная типа Settings, копия `st := p.Settings`. skip — функция из разрешённых.
func pageWReads(fset *token.FileSet, f *ast.File, skip func(fn string) bool) []string {
	var out []string
	for _, decl := range f.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok || fd.Body == nil || skip(fd.Name.Name) {
			continue
		}
		for _, line := range funcPageWReads(fset, fd) {
			out = append(out, fmt.Sprintf("%d %s", line, fd.Name.Name))
		}
	}
	return out
}

// isSettings — выражение даёт настройки проекта: поле или тип Settings, вызов *Settings().
func isSettings(e ast.Expr) bool {
	c, call := e.(*ast.CallExpr) // DefaultSettings(), model.ClampSettings(s)
	if call {
		e = c.Fun
	}
	name := ""
	switch v := e.(type) {
	case *ast.Ident:
		name = v.Name
	case *ast.SelectorExpr:
		name = v.Sel.Name
	}
	return name == "Settings" || call && strings.HasSuffix(name, "Settings")
}

// funcPageWReads — строки чтений X.PageW в функции fd (см. pageWReads).
func funcPageWReads(fset *token.FileSet, fd *ast.FuncDecl) []int {
	vars, lhs := map[string]bool{}, map[ast.Node]bool{}
	fields := fd.Type.Params.List
	if fd.Recv != nil {
		fields = slices.Concat(fields, fd.Recv.List)
	}
	for _, fl := range fields {
		for _, n := range fl.Names {
			vars[n.Name] = isSettings(fl.Type)
		}
	}
	var out []int
	ast.Inspect(fd.Body, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.AssignStmt:
			for i, l := range v.Lhs {
				lhs[l] = true
				if id, ok := l.(*ast.Ident); ok && len(v.Lhs) == len(v.Rhs) {
					vars[id.Name] = isSettings(v.Rhs[i])
				}
			}
		case *ast.ValueSpec:
			for _, id := range v.Names {
				vars[id.Name] = v.Type != nil && isSettings(v.Type)
			}
		case *ast.SelectorExpr:
			id, isID := v.X.(*ast.Ident)
			if v.Sel.Name == "PageW" && !lhs[v] && (isSettings(v.X) && !isID || isID && vars[id.Name]) {
				out = append(out, fset.Position(v.Pos()).Line)
			}
		}
		return true
	})
	return out
}

// exportedVars — экспортные переменные уровня пакета в файле, кроме ошибок Err*.
func exportedVars(f *ast.File) []string {
	var out []string
	for _, decl := range f.Decls {
		g, ok := decl.(*ast.GenDecl)
		if !ok || g.Tok != token.VAR {
			continue
		}
		for _, spec := range g.Specs {
			for _, name := range spec.(*ast.ValueSpec).Names {
				if name.IsExported() && !strings.HasPrefix(name.Name, "Err") {
					out = append(out, name.Name)
				}
			}
		}
	}
	return out
}

// TestVigGroupTpls — страж 031 G5: список шаблонов с групповым фото в model (предел мест
// при Load считается с ним) совпадает с каталогом генератора, умолчание — то же.
func TestVigGroupTpls(t *testing.T) {
	for _, tp := range vignette.Catalog(false) {
		if model.VigGroup(tp.ID) != (tp.Group != vignette.PosNone) {
			t.Errorf("шаблон %s: VigGroup %v, групповое %q", tp.ID, model.VigGroup(tp.ID), tp.Group)
		}
	}
	if def := model.DefaultVignette(); def.Tpl != vignette.DefaultTpl || model.VigGroup("нет") {
		t.Errorf("умолчание %q, у генератора %q", def.Tpl, vignette.DefaultTpl)
	}
}

// TestSheetGeometryInside — страж 031 §3 в паре с TestPageWReaders: в листе 204×304 (страница =
// лист, pageW = ширина листа) ряды «Авто», места, тексты, группа, фон, шаблон, выравнивание
// «по странице», Load и View.safe — внутри листа. Пропущенное 2·PageW дало бы лист вдвое шире,
// а половинный — страницу в пол-листа. Страниц нет: P[…], B[right…], left/right — отказ.
// Холст экспорта и «1п» — TestExportSheetOneFile (internal/app).
func TestSheetGeometryInside(t *testing.T) {
	c := sheetCheck{t: t, e: edtest.Open(t, model.NewSheet(sheetW, sheetH), "")}
	c.sid = c.e.View().Spreads[0].ID
	if _, err := c.e.AddPhotos(c.sid, edtest.Photos("a", "b", "c"), 0); err != nil {
		t.Fatal(err)
	}
	c.auto()
	c.edits()
	c.design()
	c.load()
}

const sheetW, sheetH, sheetEps = 204.0, 304.0, 0.01

// sheetCheck — шаги TestSheetGeometryInside над одним листом.
type sheetCheck struct {
	t   *testing.T
	e   *album.Editor
	sid string
}

// inside — прямоугольник внутри листа; повёрнутое держится вписанным кругом (ClampPlace):
// углы могут выйти, центр — нет.
func (c sheetCheck) inside(what string, r layout.Rect, rot float64) {
	c.t.Helper()
	if rot != 0 {
		r = layout.Rect{X: r.X + r.W/2, Y: r.Y + r.H/2, W: sheetEps, H: sheetEps}
	}
	if r.X < -sheetEps || r.Y < -sheetEps || r.X+r.W > sheetW+sheetEps || r.Y+r.H > sheetH+sheetEps || r.W <= 0 || r.H <= 0 {
		c.t.Errorf("%s вне листа %vx%v: %+v", what, sheetW, sheetH, r)
	}
}

// all — места, тексты и фоны листа редактора e внутри листа; отдаёт лист.
func (c sheetCheck) all(e *album.Editor) *model.Spread {
	c.t.Helper()
	p, v := e.Snapshot()
	s := p.Spreads[0]
	for _, pl := range s.Grid {
		c.inside("место "+pl.ID, model.PlaceAABB(pl), pl.Rot)
	}
	for _, tb := range s.Texts {
		c.inside("текст "+tb.ID, model.BoxOf(tb).AABB(), tb.Rot)
	}
	for _, b := range v.Spreads[0].Backdrops {
		c.inside("фон "+b.Scope, b.Area, 0)
	}
	return s
}

// auto — безопасная зона, предел текста и ряды «Авто» без страниц.
func (c sheetCheck) auto() {
	t := c.t
	v := c.e.View()
	if len(v.Safe) != 1 || v.Safe[0] != (layout.Rect{X: 5, Y: 5, W: sheetW - 10, H: sheetH - 10}) || v.TextMaxW != sheetW-10 {
		t.Fatalf("безопасная зона %+v, предел текста %v", v.Safe, v.TextMaxW)
	}
	ch := c.e.LayoutChoices(c.sid)
	if ch.Left != nil || ch.Right != nil || layout.IsPaged(v.Spreads[0].Layout) {
		t.Fatalf("у листа ряды страниц: %v", v.Spreads[0].Layout)
	}
	for _, r := range slices.Concat(ch.Whole.InOrder, ch.Whole.Rearranged) {
		if layout.IsPaged(r.Key) || strings.HasPrefix(r.Key, "B[right") {
			t.Errorf("ключ страниц %s", r.Key)
		}
		for _, x := range r.Rects {
			c.inside("ряд "+r.Key, x, 0)
		}
	}
}

// edits — места и тексты на «странице 1» (её нет — это лист), выравнивание «по странице» —
// к полю листа, а не к середине; поворот группы.
func (c sheetCheck) edits() {
	t := c.t
	c.e.SetGrid(c.sid, true)
	if _, err := c.e.AddPlace(c.sid, 1); err != nil {
		t.Fatal(err)
	}
	for page := range 2 {
		if _, err := c.e.AddText(c.sid, page, ""); err != nil {
			t.Fatal(err)
		}
	}
	var ids []string
	for _, pl := range c.all(c.e).Grid {
		ids = append(ids, pl.ID)
	}
	c.e.AlignItems(c.sid, ids, "right", geom.AlignPage)
	for _, pl := range c.all(c.e).Grid {
		if r := model.PlaceAABB(pl); math.Abs(r.X+r.W-(sheetW-8)) > sheetEps {
			t.Errorf("«вправо по странице»: место %s до %v, ждали поле листа %v", pl.ID, r.X+r.W, sheetW-8)
		}
	}
	c.e.RotateGroup(c.sid, ids, 30, sheetW-20, sheetH/2)
	c.all(c.e)
}

// design — фон и цвет только на весь лист; шаблон на весь лист — внутри, шаблон страницы
// и область страницы — отказ, в галерее шаблонов страницы нет.
func (c sheetCheck) design() {
	t, e, sid := c.t, c.e, c.sid
	if _, err := e.SetBackdrop(sid, model.ScopeLeft, edtest.Photo("bg")); !errors.Is(err, model.ErrScope) {
		t.Errorf("фон левой страницы листа: %v", err)
	}
	if _, err := e.SetBackdrop(sid, model.ScopeSpread, edtest.Photo("bg")); err != nil {
		t.Fatal(err)
	}
	if bs := e.View().Spreads[0].Backdrops; len(bs) != 1 || bs[0].Area != (layout.Rect{W: sheetW, H: sheetH}) {
		t.Errorf("фон листа %+v", bs)
	}
	if bg := e.SetBackground(sid, model.ScopeRight, "#ff0000").Spreads[0].BgSet; bg != [2]string{} {
		t.Errorf("цвет правой страницы листа: %v", bg)
	}
	half := []model.Box{{W: 0.5, H: 1}, {X: 0.5, W: 0.5, H: 1}}
	if _, _, err := e.ApplyGrid(sid, model.ScopeSpread, model.TplSpread, half); err != nil {
		t.Fatal(err)
	}
	s := c.all(e)
	if r := model.PlaceAABB(s.Grid[len(s.Grid)-1]); r.X+r.W < sheetW/2+10 {
		t.Errorf("правая половина шаблона легла в пол-листа: %+v", r)
	}
	for _, sc := range [][2]string{{model.ScopeSpread, model.TplPage}, {model.ScopeLeft, model.TplPage}, {model.ScopeRight, model.TplSpread}} {
		if _, _, err := e.ApplyGrid(sid, sc[0], sc[1], half); err == nil {
			t.Errorf("шаблон %s на %s листа принят", sc[1], sc[0])
		}
	}
	if g := e.GridChoices(sid, 2, model.ScopeSpread, []model.Template{{ID: "t", Scope: model.TplPage, Boxes: half}}); len(g.Mine) != 0 {
		t.Errorf("шаблон страницы в галерее листа: %+v", g.Mine)
	}
}

// load — Save → Load: лист остаётся листом, обложка из файла снимается (строка в log.txt),
// фон и цвет страницы переходят на весь лист, всё внутри.
func (c sheetCheck) load() {
	t := c.t
	p, _ := c.e.Snapshot()
	sp := p.Spreads[0]
	sp.BgPhoto, sp.BgPhotoL, sp.BgR = nil, sp.BgPhoto, "#00ff00"
	p.Cover = &model.Cover{Sheet: &model.Spread{ID: model.CoverID}}
	path := filepath.Join(testkit.TempDir(t), "лист.salbum")
	if err := salbum.Save(p, path); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	q, err := salbum.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	ls := q.Spreads[0]
	if !q.Settings.Sheet || q.Geo(ls).W != sheetW || q.Cover != nil || !strings.Contains(buf.String(), "обложка") {
		t.Fatalf("Load листа: sheet=%v, ширина %v, обложка %v, лог %q", q.Settings.Sheet, q.Geo(ls).W, q.Cover, buf.String())
	}
	if ls.BgPhoto == nil || ls.BgPhotoL != nil || ls.BgL != "#00ff00" || ls.BgR != "#00ff00" {
		t.Errorf("фон страниц не перешёл на лист: %+v %+v %q %q", ls.BgPhoto, ls.BgPhotoL, ls.BgL, ls.BgR)
	}
	c.all(edtest.Open(t, q, path))
}
