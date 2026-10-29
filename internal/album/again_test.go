package album_test

// «Пересобрать» (078 §8, сц. 9–14): цепочка сборок — один шаг отмены, живость по правкам
// проекта (звёзды и метки не в счёт), пересборка = сборка того же варианта на альбоме до неё.

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/library/imgsig"
)

// shots — n фото со смешанными пропорциями и подписью (цветность разная): похожих нет, ч/б есть из чего выбрать.
func shots(prefix string, n int) []model.PhotoInfo {
	out := make([]model.PhotoInfo, n)
	for i := range out {
		w, h := 300, 200
		switch {
		case i%7 == 3:
			w, h = 250, 250
		case i%3 == 0:
			w, h = 200, 300
		}
		id := fmt.Sprint(prefix, i)
		sig := imgsig.Sig{D: uint64(i) * 0x9e3779b97f4a7c15, P: ^uint64(i) * 0xbf58476d1ce4e5b9, Color: float32(5 + i%40), Sharp: 1, Ok: true}
		out[i] = model.PhotoInfo{ID: id, Path: edtest.Abs(id + ".jpg"), W: w, H: h, Sig: sig}
	}
	return out
}

// againOpts — настройки сборки цепочки: «Авто», ч/б и разнообразие — чтобы вариант что-то менял.
var againOpts = model.AutoOpts{Spreads: 8, Auto: true, Variety: model.VarietyNormal, Gray: 1}

// againAlbum — альбом с одним разворотом своих фото и фото сборки; base — копия до сборки.
func againAlbum(t *testing.T) (e *album.Editor, ph []model.PhotoInfo, base *model.Project) {
	t.Helper()
	e = edtest.Open(t, model.New(), "")
	edtest.NewSpread(e, -1, shots("a", 4))
	return e, shots("n", 60), edtest.P(e).Clone()
}

// autoV — сборка или пересборка варианта k; ошибка — провал.
func autoV(t *testing.T, e *album.Editor, ph []model.PhotoInfo, at, k int, again bool) model.View {
	t.Helper()
	o := againOpts
	o.Variant, o.Again = k, again
	v, err := e.AutoBuild(ph, at, o)
	if err != nil {
		t.Fatalf("сборка варианта %d (again %v): %v", k, again, err)
	}
	return v
}

// canonIDs — JSON проекта с ID разворотов по порядку: варианты сравниваются без случайных ID.
func canonIDs(p *model.Project) []byte {
	data := model.Marshal(p)
	for i, s := range p.Spreads {
		data = bytes.ReplaceAll(data, []byte(s.ID), fmt.Appendf(nil, "S%03d", i))
	}
	return data
}

// TestAutoAgainOneUndoStep: «Собрать → Пересобрать…» ×3 — один шаг отмены (один снимок — альбом
// до сборки); Ctrl+Z — альбом до сборки, Ctrl+Y — последний вариант.
func TestAutoAgainOneUndoStep(t *testing.T) {
	e, ph, _ := againAlbum(t)
	before, u0 := model.Marshal(edtest.P(e)), edtest.UndoLen(e)
	autoV(t, e, ph, -1, 1, false)
	for k := 2; k <= 4; k++ {
		if v := autoV(t, e, ph, -1, k, true); !v.Again {
			t.Fatalf("вариант %d: пересборка не жива", k)
		}
	}
	last := model.Marshal(edtest.P(e))
	if edtest.UndoLen(e) != u0+1 {
		t.Fatalf("шагов отмены %d, ждали %d", edtest.UndoLen(e), u0+1)
	}
	if v := e.Undo(); !bytes.Equal(model.Marshal(edtest.P(e)), before) || v.Again {
		t.Fatalf("Ctrl+Z: не альбом до сборки (again %v)", v.Again)
	}
	if v := e.Redo(); !bytes.Equal(model.Marshal(edtest.P(e)), last) || v.Again {
		t.Fatalf("Ctrl+Y: не последний вариант (again %v)", v.Again)
	}
}

// TestAutoAgainAfterMarks: звезда после сборки пересборку не закрывает, но шаг отметок стоит
// между: пересборка — новый шаг; Ctrl+Z — прошлый вариант, затем отметка, затем альбом до сборки.
func TestAutoAgainAfterMarks(t *testing.T) {
	e, ph, _ := againAlbum(t)
	before := model.Marshal(edtest.P(e))
	autoV(t, e, ph, -1, 1, false)
	v1, u1 := model.Marshal(edtest.P(e)), edtest.UndoLen(e)
	v, err := e.SetMarks(ph[10:11], model.MarkPatch{Rating: edtest.Ip(5)})
	if err != nil || !v.Again {
		t.Fatalf("после звезды пересборка не жива: %v", err)
	}
	autoV(t, e, ph, -1, 2, true)
	autoV(t, e, ph, -1, 3, true) // слита с прошлой пересборкой
	if edtest.UndoLen(e) != u1+2 {
		t.Fatalf("шагов %d, ждали %d", edtest.UndoLen(e), u1+2)
	}
	e.Undo()
	if !bytes.Equal(model.Marshal(edtest.P(e)), v1) || len(e.Marks()) != 1 {
		t.Fatal("Ctrl+Z: не вариант 1 со звездой")
	}
	e.Undo()
	if len(e.Marks()) != 0 {
		t.Fatal("второй Ctrl+Z: звезда осталась")
	}
	e.Undo()
	if !bytes.Equal(model.Marshal(edtest.P(e)), before) {
		t.Fatal("третий Ctrl+Z: не альбом до сборки")
	}
}

// builtIDs — ID разворотов, которых нет в base.
func builtIDs(p, base *model.Project) []string {
	var out []string
	for _, s := range p.Spreads {
		if _, old := base.Find(s.ID); old == nil {
			out = append(out, s.ID)
		}
	}
	return out
}

// TestAutoAgainReusesIDs: пересборка берёт ID разворотов прошлого варианта по порядку, лишним — новые.
func TestAutoAgainReusesIDs(t *testing.T) {
	e, ph, base := againAlbum(t)
	autoV(t, e, ph, -1, 1, false)
	ids1 := builtIDs(edtest.P(e), base)
	autoV(t, e, ph, -1, 2, true)
	if ids2 := builtIDs(edtest.P(e), base); !slices.Equal(ids1, ids2) || len(ids1) != againOpts.Spreads {
		t.Fatalf("ID варианта 1 %v, варианта 2 %v", ids1, ids2)
	}
	o := againOpts
	o.Spreads, o.Variant, o.Again = 10, 3, true
	if _, err := e.AutoBuild(ph, -1, o); err != nil {
		t.Fatal(err)
	}
	ids3 := builtIDs(edtest.P(e), base)
	if len(ids3) != 10 || !slices.Equal(ids3[:8], ids1) || slices.Contains(ids1, ids3[9]) {
		t.Fatalf("10 разворотов вместо 8: %v (было %v)", ids3, ids1)
	}
}

// TestViewAgainFlag: View.again — жива ли пересборка: нет до сборки; да после сборки, звезды и
// флажка; нет после правки альбома, Ctrl+Z и Ctrl+Y; новая сборка — снова да.
func TestViewAgainFlag(t *testing.T) {
	e, ph, _ := againAlbum(t)
	check := func(step string, want bool) {
		t.Helper()
		if got := e.View().Again; got != want {
			t.Fatalf("%s: again %v, ждали %v", step, got, want)
		}
	}
	check("до сборки", false)
	autoV(t, e, ph, -1, 1, false)
	check("после сборки", true)
	if _, err := e.SetMarks(ph[:2], model.MarkPatch{Rating: edtest.Ip(3), Flag: edtest.Ip(1)}); err != nil {
		t.Fatal(err)
	}
	check("после звезды и флажка", true)
	e.Undo() // шаг только отметок: проект тот же
	check("отмена отметки", true)
	e.MoveSpread(edtest.P(e).Spreads[0].ID, 2)
	check("после правки", false)
	autoV(t, e, ph[:30], -1, 1, false)
	check("новая сборка", true)
	e.Undo()
	check("Ctrl+Z", false)
	e.Redo()
	check("Ctrl+Y", false)
}

// TestAutoAgainEqualsFresh: сборка v1 → пересборка v3 (в конец и в середину) — тот же альбом,
// что сборка v3 на альбоме до сборки (с точностью до ID разворотов).
func TestAutoAgainEqualsFresh(t *testing.T) {
	for _, at := range []int{-1, 0} {
		e, ph, base := againAlbum(t)
		autoV(t, e, ph, at, 1, false)
		autoV(t, e, ph, at, 3, true)
		f := edtest.Open(t, base, "")
		autoV(t, f, ph, at, 3, false)
		if !bytes.Equal(canonIDs(edtest.P(e)), canonIDs(edtest.P(f))) {
			t.Fatalf("at %d: пересборка v3 не равна сборке v3 на базе", at)
		}
	}
}

// TestAutoAgainPlanFromBase: подсказка, предпросмотр и список похожих пересборки считаются от
// альбома до сборки — как у сборки на нём.
func TestAutoAgainPlanFromBase(t *testing.T) {
	e, ph, base := againAlbum(t)
	ph = append(ph, ph[5]) // повтор отбрасывает Dedupe
	ph[8].Sig = ph[7].Sig  // дубль одной ориентации: похожие есть
	autoV(t, e, ph, -1, 1, false)
	f := edtest.Open(t, base, "")
	o := againOpts
	o.Variant, o.Similar = 2, 1
	a := o
	a.Again = true
	pa, ea := e.AutoPlan(ph, a)
	pf, ef := f.AutoPlan(ph, o)
	if ea != nil || ef != nil || pa != pf || pa.Similar != 1 {
		t.Fatalf("план пересборки %+v (%v), на базе %+v (%v)", pa, ea, pf, ef)
	}
	va, ea := e.AutoPreview(ph, -1, a)
	vf, ef := f.AutoPreview(ph, -1, o)
	if ea != nil || ef != nil || !reflect.DeepEqual(va, vf) {
		t.Fatalf("предпросмотр пересборки разошёлся с базой: %v %v", ea, ef)
	}
	sa, ea := e.AutoSimilar(ph, a)
	sf, ef := f.AutoSimilar(ph, o)
	if ea != nil || ef != nil || !slices.Equal(sa, sf) || len(sa) != 1 {
		t.Fatalf("похожие пересборки %v, на базе %v", sa, sf)
	}
}

// staleAll — все вызовы с again отвечают ErrAgainStale, проект и rev прежние.
func staleAll(t *testing.T, e *album.Editor, ph []model.PhotoInfo, step string) {
	t.Helper()
	data, rev := model.Marshal(edtest.P(e)), e.View().Rev
	o := againOpts
	o.Again, o.Variant = true, 2
	_, e1 := e.AutoPlan(ph, o)
	pv, e2 := e.AutoPreview(ph, -1, o)
	sd, e3 := e.AutoSimilar(ph, o)
	v, e4 := e.AutoBuild(ph, -1, o)
	for i, err := range []error{e1, e2, e3, e4} {
		if !errors.Is(err, album.ErrAgainStale) {
			t.Fatalf("%s: вызов %d — %v, ждали ErrAgainStale", step, i, err)
		}
	}
	if pv.Spreads == nil || sd == nil || v.Rev != rev || !bytes.Equal(model.Marshal(edtest.P(e)), data) {
		t.Fatalf("%s: отказ поменял проект или отдал nil", step)
	}
}

// TestAutoAgainStaleAfterEdit: без сборки и после правки альбома пересборки нет — ошибка, альбом прежний.
func TestAutoAgainStaleAfterEdit(t *testing.T) {
	e, ph, _ := againAlbum(t)
	staleAll(t, e, ph, "без сборки")
	autoV(t, e, ph, -1, 1, false)
	e.SetSettings(model.Settings{PageW: 300, PageH: 300, DPI: 300, Gap: 5, Margin: 10})
	staleAll(t, e, ph, "после правки")
}

// TestAutoAgainStaleDuringBuild: правка между снимком входа пересборки и её замком (план считается
// вне замка) — пересборка отказывает, правка остаётся.
func TestAutoAgainStaleDuringBuild(t *testing.T) {
	e, ph, _ := againAlbum(t)
	autoV(t, e, ph, -1, 1, false)
	edtest.SetAgainGap(func() { e.MoveSpread(edtest.P(e).Spreads[0].ID, 2) })
	t.Cleanup(func() { edtest.SetAgainGap(nil) })
	data := model.Marshal(edtest.P(e))
	o := againOpts
	o.Again, o.Variant = true, 2
	if _, err := e.AutoBuild(ph, -1, o); !errors.Is(err, album.ErrAgainStale) {
		t.Fatalf("правка за время счёта: %v, ждали ErrAgainStale", err)
	}
	if bytes.Equal(model.Marshal(edtest.P(e)), data) || e.View().Again {
		t.Fatal("правка за время счёта потерялась или пересборка жива")
	}
}

// TestAutoAgainStaleAfterUndo: Ctrl+Z после сборки и Ctrl+Y закрывают пересборку.
func TestAutoAgainStaleAfterUndo(t *testing.T) {
	e, ph, _ := againAlbum(t)
	autoV(t, e, ph, -1, 1, false)
	autoV(t, e, ph, -1, 2, true)
	e.Undo()
	staleAll(t, e, ph, "Ctrl+Z")
	e.Redo()
	staleAll(t, e, ph, "Ctrl+Y")
}

// TestAutoAgainNoop: пересборка тем же вариантом — холостая правка: rev, шаги и проект те же,
// пересборка жива (и после звезды).
func TestAutoAgainNoop(t *testing.T) {
	e, ph, _ := againAlbum(t)
	autoV(t, e, ph, -1, 1, false)
	if _, err := e.SetMarks(ph[3:4], model.MarkPatch{Rating: edtest.Ip(2)}); err != nil {
		t.Fatal(err)
	}
	data, rev, u := model.Marshal(edtest.P(e)), e.View().Rev, edtest.UndoLen(e)
	v := autoV(t, e, ph, -1, 1, true)
	if v.Rev != rev || edtest.UndoLen(e) != u || !bytes.Equal(model.Marshal(edtest.P(e)), data) || !v.Again {
		t.Fatalf("тот же вариант: rev %d → %d, шагов %d → %d, again %v", rev, v.Rev, u, edtest.UndoLen(e), v.Again)
	}
}
