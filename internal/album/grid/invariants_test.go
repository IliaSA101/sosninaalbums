package grid_test

import (
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"path/filepath"
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// TestGridInvariants — инварианты сетки (design §1) после случайных правок,
// отмен и возвратов: фото не дважды, Photos по местам, места внутри разворота.
func TestGridInvariants(t *testing.T) {
	r := rand.New(rand.NewPCG(7, 2026))
	e := edtest.Open(t, model.New(), "")
	// Пути абсолютные, ID — по пути (как у ленты): круг Encode → Decode их не меняет (Rekey).
	dir := testkit.TempDir(t)
	pool := make([]model.PhotoInfo, 14)
	for i := range pool {
		path := filepath.Join(dir, fmt.Sprintf("p%02d.jpg", i))
		pool[i] = model.PhotoInfo{ID: library.IDFor(path), Path: path, W: 200 + 50*(i%4), H: 300 - 40*(i%3)}
	}
	pick := func(n int) []model.PhotoInfo {
		out := make([]model.PhotoInfo, 0, n)
		for range n {
			out = append(out, pool[r.IntN(len(pool))])
		}
		return out
	}
	edtest.NewSpread(e, -1, pool[:3])
	scopes := []string{model.ScopeLeft, model.ScopeRight, model.ScopeSpread}
	ops := []string{"set", "add", "move", "swap", "remove", "new", "place", "fill", "addPlace", "setPlaces", "delete",
		"arrange", "layers", "group", "fit", "shape", "mirror", "copy", "apply", "dup", "settings", "layout", "split", "merge", "auto", "undo", "redo", "angle", "angle", "align", "align", "grot", "gaxes", "cover", "vignette", "fields", "drop", "drop", "border", "border", "clip", "clip", "copies"}
	arr := []string{"same", "sameW", "fitPhoto", "splitH", "splitV", "front", "back"}
	vigTpls := []string{"grid", "head-t-left", "g-top", "t-top-g", "split-tg", "нет такого"}
	withGrid, seated, turned, ordered := 0, 0, 0, 0 // тест не должен выродиться в прогон одного «Авто»
	onCover, vigs := 0, 0                           // шагов над местами обложки (032), виньеток (031)
	bordered := 0                                   // мест с рамочкой по шагам (079)
	clips := 0                                      // удачных вставок и дублей (099)
	shifts := 0                                     // сдвигов своего фото по местам ручного листа (114)
	frSrcs := []model.BorderSrc{{}, {Kind: "line", Width: 2}, {Kind: "polaroid", Inks: []string{"#EEEEEE"}}, {Kind: "future-kind"},
		{Photo: model.PhotoInfo{ID: model.LibPhotoID(frKey), Path: filepath.Join(dir, "frame.png"), Lib: frKey, W: 3000, H: 4000}, Cut: [4]float64{0.1, 0.2, 0.1, 0.3}},
		{Kind: "Плохой"}}
	// Свои рамочка и стикер — по ключу библиотеки: в файл путь не пишется (048), при чтении его даёт
	// искатель; без искателя круг записи теряет путь любой своей записи, не только вставленной.
	ownKey := "my:00000000000000aa"
	own := model.PhotoInfo{ID: model.LibPhotoID(ownKey), Path: filepath.Join(dir, "logo.png"), Lib: ownKey, W: 400, H: 200}
	edtest.LibFinder(t, map[string]string{frKey: frSrcs[4].Photo.Path, ownKey: own.Path})
	steps := testkit.Steps(1500, 500) // -short: префикс той же последовательности, пороги ниже пропорционально
	need := func(n int) int { return n * steps / 1500 }
	for step := range steps {
		n := len(edtest.P(e).Spreads)
		if n == 0 {
			edtest.NewSpread(e, -1, pick(2))
			continue
		}
		s := edtest.P(e).Spreads[r.IntN(n)]
		if c := edtest.P(e).Cover; c != nil && r.IntN(5) == 0 {
			s = c.Sheet // 032: правки мест, текстов и группы — и на обложке
			onCover += len(s.Grid)
		}
		id := s.ID
		var places []string
		for _, pl := range s.Grid {
			places = append(places, pl.ID)
		}
		place := ""
		if len(places) > 0 {
			place = places[r.IntN(len(places))]
		}
		// ref — ID места, ID фото разворота или мусор; у «Авто» мест нет, только фото.
		refs := slices.Clone(places)
		refs = append(refs, s.Photos...)
		ref := "нет такого"
		if len(refs) > 0 && r.IntN(8) > 0 {
			ref = refs[r.IntN(len(refs))]
		}
		op := ops[r.IntN(len(ops))]
		// 114: каждый 5-й шаг — своё фото листа-сетки в щель (без r: прежняя последовательность не сдвигается).
		if _, own := edtest.P(e).Find(id); step%5 == 0 && own != nil && len(own.Grid) > 0 && len(own.Photos) > 1 {
			before := slices.Clone(own.Photos)
			e.MovePhoto(before[step/5%len(before)], id, step/3%(len(before)+2)-1)
			if _, now := edtest.P(e).Find(id); now != nil && !slices.Equal(before, now.Photos) {
				shifts++
			}
		}
		switch op {
		case "set":
			e.SetGrid(id, r.IntN(3) > 0)
		case "add":
			e.AddPhotos(id, pick(1+r.IntN(3)), r.IntN(4)-1)
		case "move":
			e.MovePhoto(pool[r.IntN(len(pool))].ID, id, r.IntN(3)-1)
		case "swap":
			e.SwapPhotos(pool[r.IntN(len(pool))].ID, pool[r.IntN(len(pool))].ID)
		case "remove":
			e.RemovePhotos([]string{pool[r.IntN(len(pool))].ID})
		case "new":
			e.NewSpread(r.IntN(n+1), pick(r.IntN(3)))
		case "place":
			e.PlacePhotos(id, place, pick(1+r.IntN(3)))
		case "fill":
			e.FillPlaces(pick(3), id)
		case "drop": // 042: бросок на лист или место, с вопросом и любым ответом
			_, _, _ = e.DropPhotos(id, []string{"", place}[r.IntN(2)], pick(1+r.IntN(5)), r.Float64()*1.4-0.2, []string{"", "add", "skip"}[r.IntN(3)])
		case "addPlace":
			e.AddPlace(id, r.IntN(2), []string{"", model.ShapeOval, model.ShapeRounded, "нет такой"}[r.IntN(4)])
		case "setPlaces":
			e.SetPlaces(id, []model.PlaceBox{{ID: place, X: r.Float64()*700 - 50, Y: r.Float64()*350 - 20, W: r.Float64() * 300, H: r.Float64() * 300}}, "m")
			e.SetPlaces(id, []model.PlaceBox{{ID: ref, X: r.Float64() * 400, Y: r.Float64() * 200, W: 20 + r.Float64()*200, H: 20 + r.Float64()*200}}, "")
		case "delete":
			e.DeletePlaces(id, []string{place, ref})
		case "arrange":
			e.ArrangePlaces(id, arr[r.IntN(len(arr))], refs)
		case "layers":
			if len(s.Texts) < 3 {
				e.AddText(id, r.IntN(2), "")
			}
			lr := slices.Clone(refs)
			for _, tb := range s.Texts {
				lr = append(lr, tb.ID)
			}
			r.Shuffle(len(lr), func(i, j int) { lr[i], lr[j] = lr[j], lr[i] })
			e.ArrangeLayers(id, []string{"front", "up", "down", "back"}[r.IntN(4)], lr[:r.IntN(len(lr)+1)])
		case "group":
			if len(s.Texts) < 2 {
				e.AddText(id, r.IntN(2), "")
			}
			gr := slices.Clone(refs)
			for _, tb := range s.Texts {
				gr = append(gr, tb.ID)
			}
			r.Shuffle(len(gr), func(i, j int) { gr[i], gr[j] = gr[j], gr[i] })
			e.TransformGroup(id, gr[:r.IntN(len(gr)+1)], 0.2+r.Float64()*2.5, r.Float64()*562, r.Float64()*284,
				r.Float64()*200-100, r.Float64()*200-100, []string{"", "g"}[r.IntN(2)])
		case "grot", "gaxes": // спека 018: поворот группы и масштаб по осям
			gr := slices.Clone(refs)
			for _, tb := range s.Texts {
				gr = append(gr, tb.ID)
			}
			r.Shuffle(len(gr), func(i, j int) { gr[i], gr[j] = gr[j], gr[i] })
			gr = gr[:r.IntN(len(gr)+1)]
			if op == "grot" {
				e.RotateGroup(id, gr, r.Float64()*720-360, r.Float64()*600-20, r.Float64()*320-20)
			} else {
				e.ScaleGroupAxes(id, gr, 0.1+r.Float64()*3, 0.1+r.Float64()*3, r.Float64()*360-180, r.Float64()*562, r.Float64()*284)
			}
		case "angle":
			e.SetPlaceAngle(id, ref, []float64{0, 1, 30, 90, -45, 145, 180, r.Float64()*800 - 400}[r.IntN(8)])
		case "align":
			if len(s.Texts) < 2 {
				e.AddText(id, r.IntN(2), "")
			}
			ar := slices.Clone(refs)
			for _, tb := range s.Texts {
				ar = append(ar, tb.ID)
			}
			r.Shuffle(len(ar), func(i, j int) { ar[i], ar[j] = ar[j], ar[i] })
			e.AlignItems(id, ar[:r.IntN(len(ar)+1)], edtest.AlignOps[r.IntN(len(edtest.AlignOps))], edtest.AlignTo[r.IntN(len(edtest.AlignTo))])
		case "fit":
			e.SetPlaceFit(id, []string{ref}, []string{"", model.FitContain}[r.IntN(2)])
		case "shape": // 044: форма и скругление, и у «Авто»
			if r.IntN(2) == 0 {
				e.SetPlaceShape(id, []string{ref, place}, []string{"", model.ShapeOval, model.ShapeRounded, "нет такой"}[r.IntN(4)])
			} else {
				e.SetPlaceRound(id, []string{ref, place}, []float64{0, 10, 12.4, 50, 70, -5, math.NaN()}[r.IntN(7)])
			}
		case "mirror":
			e.MirrorGrid(id, scopes[r.IntN(3)])
		case "copy":
			to := edtest.P(e).Spreads[r.IntN(n)].ID
			if r.IntN(4) == 0 { // 049 Ф2: оформление без мест (шапка виньетки цели стоит)
				_, _ = e.CopyDecor(id, scopes[r.IntN(3)], []model.GridTarget{{Spread: to, Scope: scopes[r.IntN(3)]}}, r.IntN(2) == 0, r.IntN(2) == 0)
				break
			}
			e.CopyGrid(id, scopes[r.IntN(3)], []model.GridTarget{{Spread: to, Scope: scopes[r.IntN(3)]}}, r.IntN(2) == 0, r.IntN(2) == 0)
		case "apply":
			if r.IntN(4) == 0 { // 067: готовый разворот — снимок случайного листа на случайную область
				from := edtest.P(e).Spreads[r.IntN(n)].ID
				if tp, err := e.SpreadTplOf(from, scopes[r.IntN(3)], r.IntN(3) > 0, r.IntN(2) == 0); err == nil {
					_, _, _, _ = e.ApplySpreadTpl(id, scopes[r.IntN(3)], tp, nil)
				}
				break
			}
			e.ApplyGrid(id, scopes[r.IntN(3)], []string{model.TplPage, model.TplSpread}[r.IntN(2)],
				[]model.Box{{X: r.Float64(), Y: r.Float64(), W: r.Float64(), H: r.Float64(), Rot: float64(r.IntN(3)-1) * 35}, {X: 0.5, Y: 0, W: 0.5, H: 0.5}})
		case "dup":
			if n < 12 {
				e.DuplicateGrid(id, 1, r.IntN(2) == 0)
			}
		case "settings":
			st := edtest.P(e).Settings
			st.PageW, st.PageH, st.Layflat = 200+float64(r.IntN(200)), 200+float64(r.IntN(200)), r.IntN(2) == 0
			e.SetSettings(st)
		case "layout":
			if rows := layout.Cached(edtest.P(e).Aspects(s.Photos), edtest.P(e).Params(), model.VariantLimit); len(rows) > 0 {
				e.SetLayout(id, rows[r.IntN(len(rows))].Key)
			}
		case "split":
			e.SplitSpread(id, 1)
		case "merge":
			e.MergeSpreads(id)
		case "auto":
			e.AutoBuild(pick(4), -1, model.AutoOpts{Spreads: 2, MaxPer: 64})
		case "cover": // 032: обложка, места на ней и пересчёт по зонам при смене сторонки
			if edtest.P(e).Cover == nil {
				_, _ = e.CreateCover("", nil)
				break
			}
			_, _ = e.AddPlace(model.CoverID, r.IntN(2), "")
			sp := edtest.P(e).Cover.Spec
			sp.W, sp.Hinge = float64(r.IntN(2)*(150+r.IntN(300))), float64(r.IntN(10))
			_, _ = e.SetCoverSpec(sp)
		case "vignette": // 031: «Применить» (и пересборка при смене настроек — шаг «settings»)
			switch r.IntN(5) {
			case 0, 1:
				_, _, _ = e.SetVignette(id, model.Vignette{Tpl: vigTpls[r.IntN(len(vigTpls))], Teachers: r.IntN(4),
					Pupils: r.IntN(20), Ratio: []string{"", "1:1", "2:3"}[r.IntN(3)]}, nil)
			case 2:
				e.SortVignette(id)
			case 3:
				e.ClearVignette(id)
			default:
				if n < 12 {
					_, _ = e.AddVignette(r.IntN(n + 1))
				}
			}
		case "fields": // 030: имя, подпись с полями и отступом, стиль листа — круг записи их не меняет
			ph := pool[r.IntN(len(pool))].ID
			switch r.IntN(4) {
			case 0:
				e.SetPerson(ph, model.Person{Last: "Иванова", First: []string{"", "Анна"}[r.IntN(2)]}, "")
			case 1:
				e.SetCaption(ph, []string{"", "{Фамилия} {Имя}", "{Школа}, {Год}"}[r.IntN(3)], "")
			case 2:
				e.SetCaptionStyle(ph, model.DefaultCaptionStyle(), r.IntN(2) == 0, []float64{0, 1, 2.5, 30}[r.IntN(4)])
			default:
				e.SetSheetCaps(id, r.IntN(3) > 0, model.SheetCaps{Style: model.DefaultCaptionStyle(), Pad: r.Float64() * 25,
					Over: r.IntN(2) == 0, Wrap: r.IntN(2) == 0, Text: []string{"", "{Имя}", "{Фамилия}\n{Имя}"}[r.IntN(3)]})
			}
		case "border": // 079: рамочка на выделение (и у «Авто»), правка серией, прозрачность фото
			switch r.IntN(3) {
			case 0:
				_, _ = e.ApplyBorder(id, refs[:r.IntN(len(refs)+1)], frSrcs[r.IntN(len(frSrcs))])
			case 1:
				e.SetBorder(id, []string{ref, place}, model.BorderPatch{Width: r.Float64() * 25, Scale: r.Float64() * 3,
					Inks: [][]string{nil, {}, {"#123456", "#abcdef", "#000000"}}[r.IntN(3)]}, []string{"", "b"}[r.IntN(2)])
			default:
				e.SetPhotoFade([]string{ref}, r.Float64()*1.4-0.2, "")
			}
		case "clip":
			clips += clipOp(e, r, s, refs, own)
		case "copies": // 098: запрос копии во всех входах; копия — своя запись того же файла
			cp := pick(1 + r.IntN(2))
			for i := range cp {
				if r.IntN(3) > 0 {
					cp[i].ID = model.CopyReq(cp[i].ID)
				}
			}
			switch r.IntN(4) {
			case 0:
				e.AddPhotos(id, cp, -1)
			case 1:
				e.NewSpread(r.IntN(n+1), cp)
			case 2:
				e.PlacePhotos(id, place, cp)
			default:
				_, _, _ = e.DropPhotos(id, []string{"", place}[r.IntN(2)], cp, -1, []string{"", "add", "skip"}[r.IntN(3)])
			}
		case "undo":
			e.Undo()
		case "redo":
			e.Redo()
		}
		if err := errors.Join(edtest.GridInvariants(edtest.P(e)), edtest.FamilyInvariant(edtest.P(e))); err != nil {
			t.Fatalf("шаг %d (%s): %v", step, op, err)
		}
		// Повторное открытие (Decode(Encode)) — каждый третий шаг: JSON туда и обратно — две
		// трети времени теста.
		if step%3 == 2 {
			if err := edtest.Reopen(edtest.P(e)); err != nil {
				t.Fatalf("шаг %d (%s): %v", step, op, err)
			}
		}
		for _, sp := range edtest.P(e).Spreads {
			if sp.Order != nil {
				ordered++
			}
			if sp.IsVignette() && len(sp.Grid) > 0 {
				vigs++
			}
		}
		v := e.View() // вид строится без паники
		for _, sv := range v.Spreads {
			if sv.Grid {
				withGrid++
				seated += len(sv.Photos)
			}
			for _, pv := range sv.Places {
				if pv.Rot != 0 {
					turned++
				}
				if pv.Border != nil {
					bordered++
				}
			}
		}
	}
	if err := edtest.Reopen(edtest.P(e)); err != nil {
		t.Fatalf("итог: %v", err)
	}
	if onCover < need(100) {
		t.Fatalf("обложку почти не правили: %d мест по шагам", onCover)
	}
	if bordered < need(500) {
		t.Fatalf("рамочек почти не было: %d мест по шагам", bordered)
	}
	if clips < need(25) {
		t.Fatalf("вставок буфера почти не было: %d", clips)
	}
	if shifts < need(10) {
		t.Fatalf("сдвигов фото по местам почти не было: %d", shifts)
	}
	if vigs < need(100) {
		t.Fatalf("виньеток почти не было: %d по шагам", vigs)
	}
	if withGrid < need(500) || seated < need(500) || turned < need(500) || ordered < need(500) {
		t.Fatalf("сеток мало: %d разворотов-сеток по шагам, %d фото в местах, %d повёрнутых мест, %d с общим порядком",
			withGrid, seated, turned, ordered)
	}
	t.Logf("сеток %d, фото в местах %d, повёрнутых мест %d, с общим порядком %d, виньеток %d, с рамочкой %d, вставок %d, сдвигов %d, разворотов %d",
		withGrid, seated, turned, ordered, vigs, bordered, clips, shifts, len(edtest.P(e).Spreads))
}

// clipOp — шаг буфера (099) на листе s: выделение — случайная часть мест (фото), надписей и
// стикеров с мусором (стикер, свой или рисунок, ставится, если их меньше двух); вырезать и
// вставить на случайный лист, копировать и вставить с любым ответом о повторе (и в точку, и под
// курсор), дубль. Итог — 1, если вставка встала.
func clipOp(e *album.Editor, r *rand.Rand, s *model.Spread, refs []string, own model.PhotoInfo) int {
	if len(s.Stickers) < 2 { // стикеров другие шаги не ставят: свой (запись по ключу) или рисунок
		src := []model.StickerSrc{{Photo: own}, {Art: &model.Art{Kind: "star"}}}[r.IntN(2)]
		if _, _, err := e.AddSticker(s.ID, r.IntN(2), src); err == nil {
			s = edtest.P(e).Sheet(s.ID)
		}
	}
	sel := model.ClipSel{Refs: append(slices.Clone(refs), "нет такого")}
	for _, tb := range s.Texts {
		sel.Texts = append(sel.Texts, tb.ID)
	}
	for _, st := range s.Stickers {
		sel.Stickers = append(sel.Stickers, st.ID)
	}
	part := func(ids []string) []string {
		r.Shuffle(len(ids), func(i, j int) { ids[i], ids[j] = ids[j], ids[i] })
		return ids[:r.IntN(len(ids)+1)]
	}
	sel.Refs, sel.Texts, sel.Stickers = part(sel.Refs), part(sel.Texts), part(sel.Stickers)
	if r.IntN(3) == 0 {
		_, out, err := e.DuplicateClip(s.ID, sel)
		return okPaste(out, err)
	}
	var c model.Clip
	if r.IntN(2) == 0 {
		_, c, _, _ = e.CutClip(s.ID, sel)
	} else {
		c, _ = e.TakeClip(s.ID, sel)
	}
	sheets := []string{"нет такого"}
	for _, sh := range edtest.P(e).Sheets() {
		sheets = append(sheets, sh.ID)
	}
	pt := func() *model.Pt {
		return []*model.Pt{nil, {X: r.Float64()*700 - 50, Y: r.Float64()*350 - 20}, {X: math.NaN(), Y: 1}}[r.IntN(3)]
	}
	req := model.PasteReq{Sheet: sheets[r.IntN(len(sheets))], Hover: pt(), At: pt(),
		Repeat: []string{"", model.RepeatCopy, model.RepeatMove, "нет такого"}[r.IntN(4)]}
	_, out, err := e.PasteClip(&c, req)
	return okPaste(out, err)
}

// okPaste — 1, если вставка что-то поставила.
func okPaste(out model.PasteOut, err error) int {
	if err != nil || len(out.Refs)+len(out.Texts)+len(out.Stickers) == 0 {
		return 0
	}
	return 1
}

// seatSet — таблички мест всего альбома (с обложкой) как мультимножество.
func seatSet(p *model.Project) []string {
	var out []string
	for _, s := range p.Sheets() {
		for _, pl := range s.Grid {
			if pl.Person != (model.Person{}) {
				out = append(out, fmt.Sprintf("%+v", pl.Person))
			}
		}
	}
	slices.Sort(out)
	return out
}

// seatRun — прогон TestSeatNamesInvariant: случайный лист, место и фото на шаг.
type seatRun struct {
	t     *testing.T
	e     *album.Editor
	r     *rand.Rand
	pool  []model.PhotoInfo
	names []model.Person
	step  int
	// Цель шага: лист, его места и ссылки (место или фото).
	s       *model.Spread
	place   string
	refs    []string
	rebuilt int // повторов пересборки виньетки с табличками
	scopes  []string
}

func (x *seatRun) pick(n int) []model.PhotoInfo {
	out := make([]model.PhotoInfo, 0, n)
	for range n {
		out = append(out, x.pool[x.r.IntN(len(x.pool))])
	}
	return out
}

func (x *seatRun) photo() string { return x.pool[x.r.IntN(len(x.pool))].ID }

func (x *seatRun) scope() string { return x.scopes[x.r.IntN(len(x.scopes))] }

// aim — цель шага на листе s: случайное место (или мусор) и ссылки.
func (x *seatRun) aim(s *model.Spread) {
	x.s, x.place = s, "нет такого"
	var places []string
	for _, pl := range s.Grid {
		places = append(places, pl.ID)
	}
	if len(places) > 0 {
		x.place = places[x.r.IntN(len(places))]
	}
	x.refs = append(places, s.Photos...)
}

// photoOp — действия с фото: таблички — поле места, код их не трогает.
func (x *seatRun) photoOp(op string) bool {
	e, id, r := x.e, x.s.ID, x.r
	switch op {
	case "place":
		e.PlacePhotos(id, x.place, x.pick(1+r.IntN(3)))
	case "fill":
		e.FillPlaces(x.pick(3), id)
	case "drop": // 042: и плитка новых мест, и рост виньетки
		_, _, _ = e.DropPhotos(id, []string{"", x.place}[r.IntN(2)], x.pick(1+r.IntN(4)), r.Float64(), []string{"", "add", "skip"}[r.IntN(3)])
	case "swap":
		e.SwapPhotos(x.photo(), x.photo())
	case "move":
		e.MovePhoto(x.photo(), id, r.IntN(3)-1)
	case "remove":
		e.RemovePhotos([]string{x.photo()})
	case "add":
		e.AddPhotos(id, x.pick(1), -1)
	case "sort":
		e.SortVignette(id)
	default:
		return false
	}
	return true
}

// placeOp — движение, форма, слои, деление и копии мест: таблички те же.
func (x *seatRun) placeOp(op string) bool {
	e, id, r := x.e, x.s.ID, x.r
	switch op {
	case "setPlaces":
		e.SetPlaces(id, []model.PlaceBox{{ID: x.place, X: r.Float64() * 400, Y: r.Float64() * 200, W: 20 + r.Float64()*200, H: 20 + r.Float64()*200}}, "")
	case "arrange":
		e.ArrangePlaces(id, []string{"same", "sameW", "fitPhoto", "splitH", "splitV", "front", "back"}[r.IntN(7)], x.refs)
	case "layers":
		e.ArrangeLayers(id, []string{"front", "up", "down", "back"}[r.IntN(4)], x.refs[:r.IntN(len(x.refs)+1)])
	case "angle":
		e.SetPlaceAngle(id, x.place, []float64{0, 30, 90, -45}[r.IntN(4)])
	case "shape":
		e.SetPlaceShape(id, []string{x.place}, []string{"", model.ShapeOval, model.ShapeRounded}[r.IntN(3)])
	case "fit":
		e.SetPlaceFit(id, []string{x.place}, []string{"", model.FitContain}[r.IntN(2)])
	case "mirror":
		e.MirrorGrid(id, x.scope())
	case "dup":
		if len(edtest.P(e).Spreads) < 8 {
			e.DuplicateGrid(id, 1, r.IntN(2) == 0)
		}
	case "addPlace":
		e.AddPlace(id, r.IntN(2), "")
	case "clear":
		e.ClearVignette(id)
	case "settings": // пересборка нетронутых виньеток: слотов столько же — все жильцы садятся
		st := edtest.P(e).Settings
		st.PageW, st.PageH = 200+float64(r.IntN(200)), 200+float64(r.IntN(200))
		e.SetSettings(st)
	default:
		return false
	}
	return true
}

// rebuildOp — правки, которые законно меняют таблички: своя табличка, удаление и пересборка
// мест, «Вернуть авто», раскладка, вставка списка имён, отмена.
func (x *seatRun) rebuildOp(op string) {
	e, id, r, p := x.e, x.s.ID, x.r, edtest.P(x.e)
	switch op {
	case "seat":
		if n := len(x.s.Grid); n > 0 {
			who, k := x.names[r.IntN(len(x.names))], r.IntN(n)
			edtest.Change(e, func(p *model.Project) { _, sp := p.Find(id); sp.Grid[k].Person = who })
		}
	case "delete":
		e.DeletePlaces(id, []string{x.place})
	case "apply":
		e.ApplyGrid(id, x.scope(), []string{model.TplPage, model.TplSpread}[r.IntN(2)],
			[]model.Box{{X: 0, Y: 0, W: 0.5, H: 1}, {X: 0.5, Y: 0, W: 0.5, H: 0.5}, {X: 0.5, Y: 0.5, W: 0.5, H: 0.5}}[:1+r.IntN(3)])
	case "copy":
		to := p.Spreads[r.IntN(len(p.Spreads))].ID
		e.CopyGrid(id, x.scope(), []model.GridTarget{{Spread: to, Scope: x.scope()}}, r.IntN(2) == 0, false)
	case "unfreeze", "freeze":
		e.SetGrid(id, op == "freeze")
	case "layout":
		if rows := layout.Cached(p.Aspects(x.s.Photos), p.Params(), model.VariantLimit); len(rows) > 0 {
			e.SetLayout(id, rows[r.IntN(len(rows))].Key)
		}
	case "vignette":
		if r.IntN(3) == 0 && len(p.Spreads) < 8 {
			_, _ = e.AddVignette(r.IntN(len(p.Spreads) + 1))
			return
		}
		_, _, _ = e.SetVignette(id, model.Vignette{Tpl: []string{"grid", "t-left", "g-top"}[r.IntN(3)], Teachers: r.IntN(3), Pupils: r.IntN(12)}, nil)
	case "reset":
		x.reset()
	case "paste":
		x.paste()
	case "undo":
		e.Undo()
	case "redo":
		e.Redo()
	}
}

// reset — «Вернуть авто» виньетки (с местами, если есть: иначе повторов почти нет) и повтор:
// пересборка с табличками идемпотентна — повтор холостой.
func (x *seatRun) reset() {
	for _, v := range edtest.P(x.e).Spreads {
		if v.IsVignette() && len(v.Grid) > 0 && x.r.IntN(2) == 0 {
			x.aim(v)
		}
	}
	if _, _, err := x.e.ResetVignette(x.s.ID); err != nil || !x.s.IsVignette() || len(x.s.Grid) == 0 {
		return
	}
	x.rebuilt++
	rev, undo := edtest.Rev(x.e), edtest.UndoLen(x.e)
	_, _, _ = x.e.ResetVignette(x.s.ID)
	edtest.Same(x.t, x.e, rev, undo, fmt.Sprintf("шаг %d: повтор «Вернуть авто» виньетки", x.step))
}

// paste — окно «Имена» (071 G4): список ложится табличками мест (и пустых), «Имена файлов» и
// столбец «Файл» снимают табличку места, где имя легло в фото; повтор той же вставки холостой.
func (x *seatRun) paste() {
	o := []model.NamesOpts{{Text: "Фамилия\tИмя\tПредмет\nСидорова\tОльга\tФизика\nИванов\tПетя\nПетров"},
		{Text: "А Б\nВ Г\nД Е\nЖ З\nИ К\nЛ М\nН О"}, {From: "files"},
		{Text: "Файл\tФИО\nDSC_01.jpg\tОрлов Олег\nDSC_05.jpg\tКозлов Иван"}}[x.r.IntN(4)]
	if _, _, err := x.e.PasteNames(x.s.ID, o); err != nil {
		x.t.Fatalf("шаг %d: вставка имён: %v", x.step, err)
	}
	rev, undo := edtest.Rev(x.e), edtest.UndoLen(x.e)
	_, _, _ = x.e.PasteNames(x.s.ID, o)
	edtest.Same(x.t, x.e, rev, undo, fmt.Sprintf("шаг %d: повтор вставки имён", x.step))
}

// check — после шага: таблички правок keep те же, холостая правка их не меняет, сетка цела,
// круг записи (каждый пятый шаг) их не меняет.
func (x *seatRun) check(op string, kept bool, was []string, rev int) []string {
	got := seatSet(edtest.P(x.e))
	if kept && !slices.Equal(got, was) {
		x.t.Fatalf("шаг %d (%s): таблички %v → %v", x.step, op, was, got)
	}
	if edtest.Rev(x.e) == rev && !slices.Equal(got, was) {
		x.t.Fatalf("шаг %d (%s): холостая правка сменила таблички", x.step, op)
	}
	if err := edtest.GridInvariants(edtest.P(x.e)); err != nil {
		x.t.Fatalf("шаг %d (%s): %v", x.step, op, err)
	}
	if x.step%5 == 4 {
		if err := edtest.Reopen(edtest.P(x.e)); err != nil {
			x.t.Fatalf("шаг %d (%s): %v", x.step, op, err)
		}
	}
	return got
}

// TestSeatNamesInvariant — таблички мест (071) после случайных правок: действия с фото, движение,
// форма, слои, деление и сортировка таблички не теряют и не размножают; меняют их только удаление
// и пересборка мест, «Вернуть авто», правка имени, окно «Имена» и отмена. Повтор пересборки — холостая правка.
func TestSeatNamesInvariant(t *testing.T) {
	dir := testkit.TempDir(t)
	x := &seatRun{t: t, r: rand.New(rand.NewPCG(71, 2026)), pool: make([]model.PhotoInfo, 12),
		names: []model.Person{{Last: "Иванов", First: "Петя"}, {Last: "Сидорова", First: "Ольга", Subject: "Математика"},
			{Subject: "Физика"}, {Last: "Петров"}, {}},
		scopes: []string{model.ScopeLeft, model.ScopeRight, model.ScopeSpread}}
	for i := range x.pool {
		path := filepath.Join(dir, fmt.Sprintf("DSC_%02d.jpg", i))
		x.pool[i] = model.PhotoInfo{ID: library.IDFor(path), Path: path, W: 200 + 50*(i%4), H: 300 - 40*(i%3)}
	}
	x.e = edtest.Open(t, model.New(), "")
	edtest.NewSpread(x.e, -1, x.pool[:4])
	_, _ = x.e.AddVignette(-1)
	ops := []string{"place", "fill", "drop", "swap", "move", "remove", "add", "sort", // фото
		"setPlaces", "arrange", "layers", "angle", "shape", "fit", "mirror", "dup", "addPlace", "clear", "settings", // места
		"seat", "seat", "seat", "delete", "apply", "copy", "unfreeze", "freeze", "layout", "vignette", "reset", "paste", "undo", "redo"}
	withSeats := 0 // тест не должен выродиться в прогон без табличек
	steps := testkit.Steps(900, 300)
	for x.step = range steps {
		p := edtest.P(x.e)
		if len(p.Spreads) == 0 {
			edtest.NewSpread(x.e, -1, x.pick(3))
			continue
		}
		x.aim(p.Spreads[x.r.IntN(len(p.Spreads))])
		was, rev := seatSet(p), edtest.Rev(x.e)
		op := ops[x.r.IntN(len(ops))]
		kept := x.photoOp(op) || x.placeOp(op)
		if !kept {
			x.rebuildOp(op)
		}
		if len(x.check(op, kept, was, rev)) > 0 {
			withSeats++
		}
	}
	if withSeats < steps/2 || x.rebuilt < steps/100 {
		t.Fatalf("табличек мало: шагов с ними %d из %d, пересборок %d", withSeats, steps, x.rebuilt)
	}
	t.Logf("шагов с табличками %d из %d, повторов пересборки %d", withSeats, steps, x.rebuilt)
}
