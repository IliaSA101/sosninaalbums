package spreads_test

// Замена фото на «Авто»-развороте (168): ячейки не двигаются, прежнее фото уходит в ленту.

import (
	"fmt"
	"math"
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Размеры файлов по букве набора: альбомное, книжное, квадрат, широкое (через сгиб), почти 3:2.
var rpSizes = map[byte][2]int{'L': {6000, 4000}, 'P': {4000, 6000}, 'S': {4000, 4000}, 'W': {6000, 3000}, 'N': {6000, 4020}}

// rpPhoto — фото ленты id по букве размера.
func rpPhoto(id string, kind byte) model.PhotoInfo {
	sz := rpSizes[kind]
	return model.PhotoInfo{ID: id, Path: edtest.Abs(id + ".jpg"), W: sz[0], H: sz[1]}
}

// rpEd — «Авто»-разворот из фото набора set (p0, p1…) с подрезкой crop %, раскладкой variant из
// ряда «в порядке ленты» и своим кадром 1 у фото own (−1 — ни у кого).
func rpEd(t testing.TB, set string, crop, variant, own int) (*album.Editor, string) {
	t.Helper()
	p := model.New()
	p.Settings.Crop = crop
	e := edtest.Open(t, p, "")
	ph := make([]model.PhotoInfo, len(set))
	for i := range set {
		ph[i] = rpPhoto(fmt.Sprintf("p%d", i), set[i])
	}
	sid := edtest.NewSpread(e, -1, ph).Spreads[0].ID
	if own >= 0 {
		edtest.Change(e, func(p *model.Project) {
			r := p.Photos[ph[own].ID]
			r.Asp = 1
			p.Photos[ph[own].ID] = r
		})
	}
	if rows := edtest.RowsOf(e, 0); variant > 0 && variant < len(rows) {
		e.SetLayout(sid, rows[variant].Key)
	}
	return e, sid
}

// rpShape — действующий ключ и прямоугольники листа sid по виду.
func rpShape(t testing.TB, e *album.Editor, sid string) (string, []layout.Rect) {
	t.Helper()
	for _, s := range e.View().Spreads {
		if s.ID == sid {
			return s.Layout, s.Rects
		}
	}
	t.Fatalf("листа %s нет", sid)
	return "", nil
}

// rpTol — допуск сдвига ячеек, мм: свой кадр хранится с округлением ClampAsp до 1e-4, у фото с
// «некруглой» пропорцией (N) это даёт до ≈ 0,013 мм на листе в сотни мм.
const rpTol = 0.02

// rpDiff — наибольший сдвиг сторон ячеек, мм; число ячеек другое — +Inf.
func rpDiff(a, b []layout.Rect) float64 {
	if len(a) != len(b) {
		return math.Inf(1)
	}
	m := 0.0
	for i := range a {
		for _, d := range []float64{a[i].X - b[i].X, a[i].Y - b[i].Y, a[i].W - b[i].W, a[i].H - b[i].H} {
			m = math.Max(m, math.Abs(d))
		}
	}
	return m
}

// rpKeeps — замена ячейки i набора set всеми видами нового фото: ячейки и ключ прежние, Undo —
// проект байт в байт. Возвращает число случаев и наибольший сдвиг, мм.
func rpKeeps(t *testing.T, set string, crop, variant, i, own int) (cases int, worst float64) {
	t.Helper()
	e, sid := rpEd(t, set, crop, variant, own)
	key0, rects0 := rpShape(t, e, sid)
	before := model.Marshal(edtest.P(e))
	for _, kind := range []byte("LPSWN") {
		_, left, _, err := e.ReplacePhoto(sid, fmt.Sprintf("p%d", i), rpPhoto("new", kind))
		if err != nil || left != 1 {
			t.Fatalf("crop %d %s var %d i %d: left %d, ошибка %v", crop, set, variant, i, left, err)
		}
		key1, rects1 := rpShape(t, e, sid)
		d := rpDiff(rects0, rects1)
		worst = math.Max(worst, d)
		if d > rpTol || key0 != key1 {
			t.Fatalf("crop %d %s var %d i %d own %d новое %c: сдвиг %.4f мм, ключ %q → %q", crop, set, variant, i, own, kind, d, key0, key1)
		}
		cases++
		e.Undo() // тот же редактор — следующий вид нового фото
		if got := model.Marshal(edtest.P(e)); string(got) != string(before) {
			t.Fatalf("crop %d %s i %d: Undo вернул не тот проект", crop, set, i)
		}
	}
	return cases, worst
}

// TestReplacePhotoKeepsRects — таблица замера 168 (design §2): набор, подрезка 0 и 10 %, раскладка
// лучшая и 4-я, свой кадр у прежнего фото есть и нет, новое фото любых пропорций, любая ячейка.
// Ячейки до и после совпадают ≤ 0,02 мм (rpTol), ключ раскладки тот же.
func TestReplacePhotoKeepsRects(t *testing.T) {
	sets := []string{"LLP", "LPLPL", "PPPLL", "LLLLPPPP", "PL", "W", "WL", "LPLPLPLPLPLP"}
	// «N» (6000×4020) — пропорция, не делящаяся на 1e-4: ClampAsp округляет её, и ячейки сдвигаются.
	nsets := []string{"NNL", "NLPN", "SNP", "NNNNPP", "N"}
	if testkit.Lite() {
		sets = append(sets[:4], nsets[:2]...)
	} else {
		sets = append(sets, nsets...)
	}
	total, worst := 0, 0.0
	for _, crop := range []int{0, 10} {
		for _, set := range sets {
			for _, variant := range []int{0, 3} {
				for i := range set {
					for _, own := range []int{-1, i} {
						n, w := rpKeeps(t, set, crop, variant, i, own)
						total, worst = total+n, math.Max(worst, w)
					}
				}
			}
		}
	}
	t.Logf("случаев %d, наибольший сдвиг %.5f мм", total, worst)
}

// TestReplacePhotoOldToStrip — прежнее фото в ленте (нет в Photos), новое на том же индексе, один шаг
// отмены; Undo — проект байт в байт, Redo — снова. 64 фото на листе: замена проходит.
func TestReplacePhotoOldToStrip(t *testing.T) {
	for _, n := range []int{4, 64} {
		set := ""
		for i := range n {
			set += string("LP"[i%2])
		}
		e, sid := rpEd(t, set, 0, 0, -1)
		i := n / 2
		old := fmt.Sprintf("p%d", i)
		before := model.Marshal(edtest.P(e))
		rev, undo := edtest.Rev(e), edtest.UndoLen(e)
		v, left, named, err := e.ReplacePhoto(sid, old, rpPhoto("new", 'P'))
		if err != nil || left != 1 || len(named) != 0 {
			t.Fatalf("%d фото: left %d, named %v, ошибка %v", n, left, named, err)
		}
		p := edtest.P(e)
		s := p.Spreads[0]
		if edtest.HasRef(p, old) || len(s.Photos) != n || s.Photos[i] != "new" || !edtest.HasRef(p, "new") {
			t.Fatalf("%d фото: прежнее в проекте (%v) или новое не на месте: %v", n, edtest.HasRef(p, old), s.Photos)
		}
		if v.Rev != rev+1 || edtest.Rev(e) != rev+1 || edtest.UndoLen(e) != undo+1 {
			t.Fatalf("%d фото: rev %d → %d, шагов %d → %d: ждали один шаг", n, rev, edtest.Rev(e), undo, edtest.UndoLen(e))
		}
		after := model.Marshal(p)
		e.Undo()
		if got := model.Marshal(edtest.P(e)); string(got) != string(before) || edtest.UndoLen(e) != undo {
			t.Fatalf("%d фото: Undo — не тот проект или шаг", n)
		}
		e.Redo()
		if got := model.Marshal(edtest.P(e)); string(got) != string(after) {
			t.Fatalf("%d фото: Redo — не тот проект", n)
		}
	}
}

// TestReplacePhotoOwnCrop — свой кадр нового фото: пропорции ячейки, если они другие; 0, если те
// же; при подрезке и своём кадре у прежнего — пропорции ячейки и при равных (замок подрезки).
func TestReplacePhotoOwnCrop(t *testing.T) {
	for _, crop := range []int{0, 10} {
		for _, own := range []bool{false, true} {
			e, sid := rpEd(t, "LPL", crop, 0, map[bool]int{false: -1, true: 1}[own])
			_, rects := rpShape(t, e, sid)
			cell := rects[1].W / rects[1].H
			// Другие пропорции: свой кадр — всегда пропорции ячейки.
			if _, _, _, err := e.ReplacePhoto(sid, "p1", rpPhoto("wide", 'W')); err != nil {
				t.Fatal(err)
			}
			if got := edtest.P(e).Photos["wide"].Asp; got != model.ClampAsp(cell) {
				t.Fatalf("crop %d own %v: Asp %v, ждали пропорции ячейки %v", crop, own, got, model.ClampAsp(cell))
			}
			e.Undo()
			// Те же пропорции: свой кадр не нужен, кроме замка подрезки.
			same := model.PhotoInfo{ID: "same", Path: edtest.Abs("same.jpg"), W: int(math.Round(cell * 1e4)), H: 1e4}
			if _, _, _, err := e.ReplacePhoto(sid, "p1", same); err != nil {
				t.Fatal(err)
			}
			want := 0.0
			if own && crop > 0 {
				want = model.ClampAsp(cell)
			}
			if got := edtest.P(e).Photos["same"].Asp; got != want {
				t.Fatalf("crop %d own %v: те же пропорции — Asp %v, ждали %v", crop, own, got, want)
			}
			// «Сбросить кадр» отдаёт фото целиком.
			if want > 0 {
				e.ResetFrame("same")
				if got := edtest.P(e).Photos["same"].Asp; got != 0 {
					t.Fatalf("Сбросить кадр оставил Asp %v", got)
				}
			}
		}
	}
}

// swapMix — ручной лист s1 (m1…m3 — места) и два «Авто»: s2 (a1…a4, разные пропорции) и s3 (b1…b3).
func swapMix(t *testing.T) *album.Editor {
	t.Helper()
	pls := []*model.Place{edtest.Gp(10, 10, 80, 60, "m1"), edtest.Gp(100, 10, 60, 80, "m2"), edtest.Gp(170, 10, 90, 60, "m3")}
	e := edtest.GridEd(t, pls, []string{"a1", "a2", "a3", "a4"}, []string{"b1", "b2", "b3"})
	edtest.Change(e, func(p *model.Project) {
		for id, sz := range map[string][2]int{"a2": {4000, 6000}, "a4": {6000, 3000}, "b1": {4000, 6000}, "b3": {4000, 4000}, "m2": {4000, 6000}} {
			r := p.Photos[id]
			r.W, r.H = sz[0], sz[1]
			p.Photos[id] = r
		}
	})
	return e
}

// swapInfo — сведения о фото id проекта e: так их отдаёт App (infosOrRefs).
func swapInfo(e *album.Editor, id string) model.PhotoInfo {
	r := edtest.P(e).Photos[id]
	return model.PhotoInfo{ID: id, Path: r.Path, W: r.W, H: r.H}
}

// swapCheck — после замены (left = 0, один шаг отмены) ячейки листов ids те же, что до неё;
// Undo возвращает проект байт в байт. do — сама замена.
func swapCheck(t *testing.T, e *album.Editor, ids []string, do func()) {
	t.Helper()
	keys, rects := make([]string, len(ids)), make([][]layout.Rect, len(ids))
	for i, id := range ids {
		keys[i], rects[i] = rpShape(t, e, id)
	}
	before, undo := model.Marshal(edtest.P(e)), edtest.UndoLen(e)
	do()
	if edtest.UndoLen(e) != undo+1 {
		t.Fatalf("шагов %d → %d", undo, edtest.UndoLen(e))
	}
	for i, id := range ids {
		if key, got := rpShape(t, e, id); rpDiff(rects[i], got) > 0.01 || key != keys[i] {
			t.Fatalf("лист %s: ячейки сдвинулись на %.4f мм (%q → %q)", id, rpDiff(rects[i], got), keys[i], key)
		}
	}
	if err := edtest.GridInvariants(edtest.P(e)); err != nil {
		t.Fatal(err)
	}
	e.Undo()
	if string(model.Marshal(edtest.P(e))) != string(before) {
		t.Fatal("Undo вернул не тот проект")
	}
}

// TestReplacePhotoFromAlbumSwaps — фото уже в альбоме: обмен; ячейки обоих листов «Авто» прежние,
// и когда фото с другого листа, и когда с того же.
func TestReplacePhotoFromAlbumSwaps(t *testing.T) {
	e := swapMix(t)
	swapCheck(t, e, []string{"s2", "s3"}, func() {
		if _, left, _, err := e.ReplacePhoto("s2", "a2", swapInfo(e, "b1")); err != nil || left != 0 {
			t.Fatalf("left %d, ошибка %v", left, err)
		}
		p := edtest.P(e)
		if p.Spreads[1].Photos[1] != "b1" || p.Spreads[2].Photos[0] != "a2" {
			t.Fatalf("обмен не встал: %v, %v", p.Spreads[1].Photos, p.Spreads[2].Photos)
		}
	})
	swapCheck(t, e, []string{"s2"}, func() {
		if _, left, _, err := e.ReplacePhoto("s2", "a2", swapInfo(e, "a4")); err != nil || left != 0 {
			t.Fatalf("left %d, ошибка %v", left, err)
		}
		if got := edtest.P(e).Spreads[1].Photos; !slices.Equal(got, []string{"a1", "a4", "a3", "a2"}) {
			t.Fatalf("порядок %v", got)
		}
	})
}

// TestReplacePhotoFromManualPlace — фото с ручного места: обмен, ушедшему в место Asp = 0, места
// ручного листа те же, ячейки «Авто» прежние.
func TestReplacePhotoFromManualPlace(t *testing.T) {
	e := swapMix(t)
	edtest.Change(e, func(p *model.Project) { // у прежнего фото свой кадр: на ручном месте он не действует
		r := p.Photos["a2"]
		r.Asp = 0.7
		p.Photos["a2"] = r
	})
	boxes := func() (out []model.Place) {
		for _, pl := range edtest.P(e).Spreads[0].Grid {
			c := *pl
			c.Photo = ""
			out = append(out, c)
		}
		return out
	}
	was := boxes()
	swapCheck(t, e, []string{"s2"}, func() {
		if _, left, _, err := e.ReplacePhoto("s2", "a2", swapInfo(e, "m2")); err != nil || left != 0 {
			t.Fatalf("left %d, ошибка %v", left, err)
		}
		p := edtest.P(e)
		if p.Spreads[0].Grid[1].Photo != "a2" || p.Spreads[1].Photos[1] != "m2" || p.Photos["a2"].Asp != 0 {
			t.Fatalf("обмен с ручным: место %q, Asp ушедшего %v", p.Spreads[0].Grid[1].Photo, p.Photos["a2"].Asp)
		}
		if !reflect.DeepEqual(was, boxes()) {
			t.Fatal("места ручного листа сдвинулись")
		}
	})
}

// TestReplacePhotoCopyReq — запрос копии «<id>-new»: новая запись на месте target, прежнее в ленте,
// исходное фото остаётся на своём листе.
func TestReplacePhotoCopyReq(t *testing.T) {
	e := swapMix(t)
	_, rects0 := rpShape(t, e, "s2")
	r := edtest.P(e).Photos["b1"]
	req := model.PhotoInfo{ID: model.CopyReq("b1"), Path: r.Path, W: r.W, H: r.H}
	_, left, _, err := e.ReplacePhoto("s2", "a3", req)
	if err != nil || left != 1 {
		t.Fatalf("left %d, ошибка %v", left, err)
	}
	p := edtest.P(e)
	cp := p.Spreads[1].Photos[2]
	if cp == "b1" || cp == "a3" || p.Photos[cp].Path != r.Path || !slices.Contains(p.Spreads[2].Photos, "b1") {
		t.Fatalf("копия не встала: на месте %q, лист s3 %v", cp, p.Spreads[2].Photos)
	}
	if edtest.HasRef(p, "a3") {
		t.Fatal("прежнее фото осталось в проекте")
	}
	if _, rects := rpShape(t, e, "s2"); rpDiff(rects0, rects) > 0.01 {
		t.Fatalf("ячейки сдвинулись на %.4f мм", rpDiff(rects0, rects))
	}
}

// TestReplacePhotoNoop — холостые: неизвестные лист и target, то же фото, ручной лист, обложка,
// фото не этого листа — rev, шаги отмены и проект прежние.
func TestReplacePhotoNoop(t *testing.T) {
	e := swapMix(t)
	edtest.Change(e, func(p *model.Project) { // обложка с фото в месте
		p.Cover = &model.Cover{Sheet: &model.Spread{ID: model.CoverID, Grid: []*model.Place{edtest.Gp(10, 10, 50, 50, "c1")}, Photos: []string{"c1"}}}
		p.Photos["c1"] = model.PhotoRef{Path: edtest.Abs("c1.jpg"), W: 300, H: 200}
	})
	cases := []struct {
		what, sid, target, id string
	}{
		{"лист неизвестен", "нет", "a1", "x1"},
		{"target неизвестен", "s2", "нет", "x1"},
		{"target с другого листа", "s2", "b1", "x1"},
		{"то же фото", "s2", "a1", "a1"},
		{"ручной лист", "s1", "m1", "x1"},
		{"обложка", model.CoverID, "c1", "x1"},
		{"пустой лист", "", "", "x1"},
	}
	for _, c := range cases {
		rev, undo, before := edtest.Rev(e), edtest.UndoLen(e), model.Marshal(edtest.P(e))
		v, left, named, err := e.ReplacePhoto(c.sid, c.target, rpPhoto(c.id, 'L'))
		if err != nil || left != 0 || len(named) != 0 {
			t.Fatalf("%s: left %d, named %v, ошибка %v", c.what, left, named, err)
		}
		edtest.Same(t, e, rev, undo, c.what)
		if v.Rev != rev || string(model.Marshal(edtest.P(e))) != string(before) {
			t.Fatalf("%s: проект или rev вида изменились", c.what)
		}
	}
}

// TestReplacePhotoSurvives — дальнейшие правки замену не сбрасывают: правка другого листа и текст
// на этом листе не двигают ячейки и Asp; круг Save → Load тоже. Добавление фото на лист перестраивает
// его как обычное «Авто», а свой кадр замены остаётся.
func TestReplacePhotoSurvives(t *testing.T) {
	e := swapMix(t)
	_, rects0 := rpShape(t, e, "s2")
	if _, _, _, err := e.ReplacePhoto("s2", "a2", rpPhoto("n1", 'W')); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := e.ReplacePhoto("s3", "b2", rpPhoto("n2", 'P')); err != nil {
		t.Fatal(err)
	}
	asp := edtest.P(e).Photos["n1"].Asp
	if asp == 0 {
		t.Fatal("n1: свой кадр не встал")
	}
	edtest.AddPhotos(e, "s3", edtest.Photos("z1"), -1)
	edtest.AddText(t, e, "s2", 0)
	if _, rects := rpShape(t, e, "s2"); rpDiff(rects0, rects) > 0.01 || edtest.P(e).Photos["n1"].Asp != asp {
		t.Fatalf("после правок: сдвиг %.4f мм, Asp %v → %v", rpDiff(rects0, rects), asp, edtest.P(e).Photos["n1"].Asp)
	}
	// Save → Load.
	path := filepath.Join(testkit.TempDir(t), "r.salbum")
	if err := e.Save(path); err != nil {
		t.Fatal(err)
	}
	q, err := album.LoadBase(path, filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	got := edtest.Open(t, q, path)
	n1 := idAt(t, edtest.P(got), "n1.jpg") // Load переключает ID на ключ по пути (Rekey)
	if a := edtest.P(got).Photos[n1].Asp; a != asp {
		t.Fatalf("после Load Asp %v, было %v", a, asp)
	}
	if _, rects := rpShape(t, got, "s2"); rpDiff(rects0, rects) > 0.01 {
		t.Fatalf("после Load ячейки сдвинулись на %.4f мм", rpDiff(rects0, rects))
	}
	// Добавили фото на лист: другое число фото — «Авто» строит заново, кадр n1 остаётся.
	edtest.AddPhotos(got, "s2", edtest.Photos("z2"), -1)
	if edtest.P(got).Photos[n1].Asp != asp || len(edtest.P(got).Spreads[1].Photos) != 5 {
		t.Fatalf("после добавления: Asp %v, фото %v", edtest.P(got).Photos[n1].Asp, edtest.P(got).Spreads[1].Photos)
	}
	if err := edtest.GridInvariants(edtest.P(got)); err != nil {
		t.Fatal(err)
	}
}

// idAt — ID записи проекта с файлом name (после Load ID — ключ по пути).
func idAt(t *testing.T, p *model.Project, name string) string {
	t.Helper()
	for id, r := range p.Photos {
		if r.Path == edtest.Abs(name) {
			return id
		}
	}
	t.Fatalf("записи с файлом %s нет", name)
	return ""
}

// TestReplacePhotoNamed — лист с шаблоном подписи с полем имени: новое фото получает имя из
// названия файла (named), как у PlacePhotos; Undo снимает замену вместе с именем одним шагом.
func TestReplacePhotoNamed(t *testing.T) {
	e := swapMix(t)
	edtest.Change(e, func(p *model.Project) {
		p.Spreads[1].Caps = &model.SheetCaps{Style: model.NewCaptionStyle(p.Settings.PageH), Text: "{Фамилия} {Имя}"}
	})
	undo := edtest.UndoLen(e)
	ph := model.PhotoInfo{ID: "who", Path: edtest.Abs("01_иванов_петя.jpg"), W: 300, H: 200}
	_, left, named, err := e.ReplacePhoto("s2", "a1", ph)
	if err != nil || left != 1 || !slices.Equal(named, []string{"who"}) {
		t.Fatalf("left %d, named %v, ошибка %v", left, named, err)
	}
	if pv, _ := e.Person("who"); pv.Last != "Иванов" {
		t.Fatalf("имя %+v", pv)
	}
	if edtest.UndoLen(e) != undo+1 {
		t.Fatalf("шагов %d → %d: замена и имя — один шаг", undo, edtest.UndoLen(e))
	}
	e.Undo()
	if edtest.HasRef(edtest.P(e), "who") {
		t.Fatal("Undo оставил новое фото")
	}
}
