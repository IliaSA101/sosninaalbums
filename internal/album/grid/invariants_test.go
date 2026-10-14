package grid_test

import (
	"fmt"
	"math/rand/v2"
	"path/filepath"
	"slices"
	"testing"

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
		"arrange", "layers", "group", "fit", "mirror", "copy", "apply", "dup", "settings", "layout", "split", "merge", "auto", "undo", "redo", "angle", "angle", "align", "align", "grot", "gaxes", "cover", "vignette", "fields", "drop", "drop"}
	arr := []string{"same", "sameW", "fitPhoto", "splitH", "splitV", "front", "back"}
	vigTpls := []string{"grid", "head-t-left", "g-top", "t-top-g", "split-tg", "нет такого"}
	withGrid, seated, turned, ordered := 0, 0, 0, 0 // тест не должен выродиться в прогон одного «Авто»
	onCover, vigs := 0, 0                           // шагов над местами обложки (032), виньеток (031)
	for step := range 1500 {
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
			e.AddPlace(id, r.IntN(2))
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
		case "mirror":
			e.MirrorGrid(id, scopes[r.IntN(3)])
		case "copy":
			to := edtest.P(e).Spreads[r.IntN(n)].ID
			e.CopyGrid(id, scopes[r.IntN(3)], []model.GridTarget{{Spread: to, Scope: scopes[r.IntN(3)]}}, r.IntN(2) == 0, r.IntN(2) == 0)
		case "apply":
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
			_, _ = e.AddPlace(model.CoverID, r.IntN(2))
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
		case "undo":
			e.Undo()
		case "redo":
			e.Redo()
		}
		if err := edtest.GridInvariants(edtest.P(e)); err != nil {
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
			}
		}
	}
	if err := edtest.Reopen(edtest.P(e)); err != nil {
		t.Fatalf("итог: %v", err)
	}
	if onCover < 100 {
		t.Fatalf("обложку почти не правили: %d мест по шагам", onCover)
	}
	if vigs < 100 {
		t.Fatalf("виньеток почти не было: %d по шагам", vigs)
	}
	if withGrid < 500 || seated < 500 || turned < 500 || ordered < 500 {
		t.Fatalf("сеток мало: %d разворотов-сеток по шагам, %d фото в местах, %d повёрнутых мест, %d с общим порядком",
			withGrid, seated, turned, ordered)
	}
	t.Logf("сеток %d, фото в местах %d, повёрнутых мест %d, с общим порядком %d, виньеток %d, разворотов %d",
		withGrid, seated, turned, ordered, vigs, len(edtest.P(e).Spreads))
}
