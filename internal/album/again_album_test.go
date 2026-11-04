package album_test

// «Весь альбом заново» (107, сц. 10–11): цепочка «Пересобрать» режима альбома — один шаг отмены,
// пересборка = сборка того же варианта на альбоме до неё, живость по правкам и по режиму цепочки.

import (
	"bytes"
	"errors"
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// albumOpts — настройки режима альбома: «Авто», разнообразие и ч/б — чтобы вариант что-то менял.
func albumOpts(n, variant int, again bool) model.AutoOpts {
	return model.AutoOpts{Spreads: n, Auto: true, Variety: model.VarietyNormal, Gray: 1, Album: true, Variant: variant, Again: again}
}

// albumShots — shots с ID, как у сканера (от пути): только такие альбом переживает Save → Load.
func albumShots(prefix string, n int) []model.PhotoInfo {
	out := shots(prefix, n)
	for i, f := range out {
		lib := edtest.Lib(f.Path, model.XmpMarks{})
		lib.W, lib.H, lib.Sig = f.W, f.H, f.Sig
		out[i] = lib
	}
	return out
}

// albumFix — собранный альбом (голые развороты) с якорями: разворот с надписью и пустой.
// feed — фото альбома по порядку и три свободных; base — проект до пересборки.
func albumFix(t *testing.T) (e *album.Editor, feed []model.PhotoInfo, base *model.Project) {
	t.Helper()
	e = edtest.Open(t, model.New(), "")
	ph := albumShots("a", 40)
	if _, err := e.AutoBuild(ph, -1, model.AutoOpts{Spreads: 8, Auto: true, Variety: model.VarietyNormal}); err != nil {
		t.Fatal(err)
	}
	edtest.Change(e, func(p *model.Project) {
		p.Spreads[2].Name = "Банкет"
		p.Spreads = append(p.Spreads, &model.Spread{ID: "blank"})
	})
	return e, append(ph, albumShots("free", 3)...), edtest.P(e).Clone()
}

// bareIDs — ID голых разворотов (с фото, без названия) по порядку альбома.
func bareIDs(p *model.Project) []string {
	var out []string
	for _, s := range p.Spreads {
		if len(s.Photos) > 0 && s.Name == "" {
			out = append(out, s.ID)
		}
	}
	return out
}

// albumV — пересборка альбома вариантом k (again — цепочкой); ошибка — провал.
func albumV(t *testing.T, e *album.Editor, feed []model.PhotoInfo, n, k int, again bool) model.View {
	t.Helper()
	v, err := e.AutoBuild(feed, -1, albumOpts(n, k, again))
	if err != nil {
		t.Fatalf("вариант %d (again %v): %v", k, again, err)
	}
	return v
}

// TestAutoAlbumOneUndoStep: пересборка альбома — один шаг отмены; Ctrl+Z — альбом до неё (байт в
// байт, включая якоря), Ctrl+Y — пересобранный; после неё пересборка жива.
func TestAutoAlbumOneUndoStep(t *testing.T) {
	e, feed, _ := albumFix(t)
	before, u0 := model.Marshal(edtest.P(e)), edtest.UndoLen(e)
	v := albumV(t, e, feed, 6, 1, false)
	after := model.Marshal(edtest.P(e))
	if !v.Again || edtest.UndoLen(e) != u0+1 || bytes.Equal(before, after) {
		t.Fatalf("again %v, шагов %d (ждали %d), проект изменился: %v", v.Again, edtest.UndoLen(e), u0+1, !bytes.Equal(before, after))
	}
	if v := e.Undo(); !bytes.Equal(model.Marshal(edtest.P(e)), before) || v.Again {
		t.Fatalf("Ctrl+Z: не альбом до пересборки (again %v)", v.Again)
	}
	if v := e.Redo(); !bytes.Equal(model.Marshal(edtest.P(e)), after) || v.Again {
		t.Fatalf("Ctrl+Y: не пересобранный (again %v)", v.Again)
	}
}

// TestAutoAlbumAgainOneStep: «Пересобрать…» ×3 — тот же один шаг; новые развороты каждого варианта
// имеют ID первого; Ctrl+Z — альбом до первой пересборки, Ctrl+Y — последний вариант.
func TestAutoAlbumAgainOneStep(t *testing.T) {
	e, feed, base := albumFix(t)
	before, u0 := model.Marshal(base), edtest.UndoLen(e)
	was := bareIDs(base)
	albumV(t, e, feed, 6, 1, false)
	first := bareIDs(edtest.P(e))
	if len(first) != 6 || !slices.Equal(first, was[:6]) {
		t.Fatalf("ID после первой пересборки %v, прежние голые %v", first, was)
	}
	for k := 2; k <= 4; k++ {
		if v := albumV(t, e, feed, 6, k, true); !v.Again {
			t.Fatalf("вариант %d: пересборка не жива", k)
		}
		if ids := bareIDs(edtest.P(e)); !slices.Equal(ids, first) {
			t.Fatalf("вариант %d: ID %v, ждали %v", k, ids, first)
		}
	}
	last := model.Marshal(edtest.P(e))
	// Тот же вариант — холостая: ни шага, ни rev проекта.
	rev := edtest.Rev(e)
	albumV(t, e, feed, 6, 4, true)
	albumV(t, e, feed, 6, 4, true)
	edtest.Same(t, e, rev, u0+1, "тот же вариант пересборки")
	if !bytes.Equal(model.Marshal(edtest.P(e)), last) {
		t.Fatal("тот же вариант изменил проект")
	}
	if v := e.Undo(); !bytes.Equal(model.Marshal(edtest.P(e)), before) || v.Again {
		t.Fatalf("Ctrl+Z: не альбом до первой пересборки (again %v)", v.Again)
	}
	if v := e.Redo(); !bytes.Equal(model.Marshal(edtest.P(e)), last) || v.Again {
		t.Fatalf("Ctrl+Y: не последний вариант (again %v)", v.Again)
	}
}

// TestAutoAlbumAgainEqualsFresh: пересборка варианта k даёт те же байты (с точностью до случайных
// ID), что сборка варианта k на таком же альбоме с нуля, — в том числе при другом N и после
// Save → Load.
func TestAutoAlbumAgainEqualsFresh(t *testing.T) {
	e, feed, base := albumFix(t)
	albumV(t, e, feed, 6, 1, false)
	for _, c := range []struct{ n, k int }{{6, 2}, {6, 3}, {9, 4}, {4, 5}} {
		albumV(t, e, feed, c.n, c.k, true)
		fresh := edtest.Open(t, base.Clone(), "")
		if _, err := fresh.AutoBuild(feed, -1, albumOpts(c.n, c.k, false)); err != nil {
			t.Fatal(err)
		}
		if a, b := canonIDs(edtest.P(e)), canonIDs(edtest.P(fresh)); !bytes.Equal(a, b) {
			t.Fatalf("N=%d вариант %d: пересборка и свежая сборка разошлись", c.n, c.k)
		}
	}
	// Круг Save → Load: альбом после пересборки записывается и читается тем же.
	if err := edtest.Reopen(edtest.P(e)); err != nil {
		t.Fatal(err)
	}
	if q := edtest.SaveLoad(t, e); len(q.Spreads) != len(edtest.P(e).Spreads) || !bytes.Equal(canonIDs(q), canonIDs(edtest.P(e))) {
		t.Fatal("после Save → Load альбом другой")
	}
}

// TestAutoAlbumStaleAfterEdit: правка (в том числе Ctrl+Z) закрывает цепочку — пересборка альбома
// отвечает ErrAgainStale и проект не меняет; цепочка другого режима тоже не годится.
func TestAutoAlbumStaleAfterEdit(t *testing.T) {
	stale := func(t *testing.T, e *album.Editor, feed []model.PhotoInfo, o model.AutoOpts, what string) {
		t.Helper()
		rev, data := edtest.Rev(e), model.Marshal(edtest.P(e))
		if _, err := e.AutoBuild(feed, -1, o); !errors.Is(err, album.ErrAgainStale) || edtest.Rev(e) != rev || !bytes.Equal(model.Marshal(edtest.P(e)), data) {
			t.Fatalf("%s: ждали ErrAgainStale и целый проект: %v", what, err)
		}
		if _, err := e.AutoPlan(feed, o); !errors.Is(err, album.ErrAgainStale) {
			t.Fatalf("%s: AutoPlan: %v", what, err)
		}
	}
	e, feed, _ := albumFix(t)
	stale(t, e, feed, albumOpts(6, 2, true), "сборки не было") // до первой пересборки
	albumV(t, e, feed, 6, 1, false)
	e.SetSettings(edtest.P(e).Settings) // тот же размер — холостая, цепочка жива
	if _, err := e.AutoBuild(feed, -1, albumOpts(6, 2, true)); err != nil {
		t.Fatalf("холостая правка закрыла цепочку: %v", err)
	}
	edtest.Change(e, func(p *model.Project) { p.Spreads[0].Name = "Правка" })
	stale(t, e, feed, albumOpts(6, 3, true), "после правки")
	e.Undo()
	stale(t, e, feed, albumOpts(6, 3, true), "после Ctrl+Z")

	// Цепочка обычной сборки не пересобирается как альбом, и наоборот.
	e, feed, _ = albumFix(t)
	plain := model.AutoOpts{Spreads: 5, Auto: true}
	if _, err := e.AutoBuild(feed[40:], -1, plain); err != nil {
		t.Fatal(err)
	}
	stale(t, e, feed, albumOpts(6, 2, true), "цепочка не альбома")
	albumV(t, e, feed, 6, 1, false)
	plain.Again = true
	stale(t, e, feed[40:], plain, "цепочка альбома")
}
