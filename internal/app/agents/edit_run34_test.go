package agents

import (
	"context"
	"slices"
	"sync/atomic"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/model/printprof"
)

// Правящие инструменты 0.34 (151 Т4), служебная часть: вход, ссылки агента → настоящие, маски уровня 1,
// запись для undo_last_edit. Сам Editor и тосты — у фасада (internal/app/agents_edit34_test.go).

// fake34 — подставные Mark…Print: по умолчанию «правка прошла» (ревизия растёт), тесты подменяют ответ.
type fake34 struct {
	mark   func(MarkReq) (MarkRes, error)
	cover  func(CoverReq) (CoverRes, error)
	bg     func(BgReq) (BgRes, error)
	stk    func(StickerReq) (StickerRes, error)
	print_ func(PrintReq) (PrintRes, error)
	fams   func(ids []string) []string // Host.Families; nil — пустые

	lastMark  MarkReq
	lastCover CoverReq
	lastBg    BgReq
	lastStk   StickerReq
	lastPrint PrintReq

	nMark, nCover, nBg, nStk, nPrint atomic.Int32
}

func (f *fake34) writes34() int32 {
	return f.nMark.Load() + f.nCover.Load() + f.nBg.Load() + f.nStk.Load() + f.nPrint.Load()
}

func (f *fake34) install(e *editFake, h *Host) {
	h.Mark = func(_ context.Context, in MarkReq) (MarkRes, error) {
		f.nMark.Add(1)
		e.mu.Lock()
		f.lastMark = in
		fn := f.mark
		e.mu.Unlock()
		if fn != nil {
			return fn(in)
		}
		return MarkRes{Album: e.f.bump(), Changed: len(in.IDs)}, nil
	}
	h.Cover = func(_ context.Context, in CoverReq) (CoverRes, error) {
		f.nCover.Add(1)
		e.mu.Lock()
		f.lastCover = in
		fn := f.cover
		e.mu.Unlock()
		if fn != nil {
			return fn(in)
		}
		return CoverRes{Album: e.f.bump(), Place: "seat", Changed: true}, nil
	}
	h.Bg = func(_ context.Context, in BgReq) (BgRes, error) {
		f.nBg.Add(1)
		e.mu.Lock()
		f.lastBg = in
		fn := f.bg
		e.mu.Unlock()
		if fn != nil {
			return fn(in)
		}
		return BgRes{Album: e.f.bump(), Changed: len(in.Spreads)}, nil
	}
	h.Stickers = func(_ context.Context, in StickerReq) (StickerRes, error) {
		f.nStk.Add(1)
		e.mu.Lock()
		f.lastStk = in
		fn := f.stk
		e.mu.Unlock()
		if fn != nil {
			return fn(in)
		}
		ids := make([]string, len(in.Items))
		for i := range ids {
			ids[i] = "stk" + string(rune('a'+i))
		}
		return StickerRes{Album: e.f.bump(), Added: len(ids), IDs: ids}, nil
	}
	h.Print = func(_ context.Context, in PrintReq) (PrintRes, error) {
		f.nPrint.Add(1)
		e.mu.Lock()
		f.lastPrint = in
		fn := f.print_
		e.mu.Unlock()
		if fn != nil {
			return fn(in)
		}
		return PrintRes{Album: e.f.bump(), PageW: 300, PageH: 200, Changed: true}, nil
	}
}

func (e *editFake) set34(fn func(f *fake34)) {
	e.mu.Lock()
	fn(&e.fake34)
	e.mu.Unlock()
}

func (e *editFake) got34() fake34 {
	e.mu.Lock()
	defer e.mu.Unlock()
	return fake34{lastMark: e.lastMark, lastCover: e.lastCover, lastBg: e.lastBg, lastStk: e.lastStk, lastPrint: e.lastPrint}
}

func album34(r *rig) map[string]any { return map[string]any{"album": r.fx.refArgs()} }

// args34 — вход инструмента: album + остальное.
func args34(r *rig, kv map[string]any) map[string]any {
	a := album34(r)
	for k, v := range kv {
		a[k] = v
	}
	return a
}

// TestAgentMarks: вход set_marks — копия «id-2» сводится к файлу, повторы убираются, rating 0 задан, чужие id
// возвращаются очищенными; пустой набор полей и выход за пределы схемы — отказ до Host.
func TestAgentMarks(t *testing.T) {
	r, cs := editRig(t, Options{}, func(e *editFake) {
		e.set34(func(f *fake34) {
			f.mark = func(in MarkReq) (MarkRes, error) {
				return MarkRes{Album: AlbumRef{Key: albumKey(7), Rev: 5}, Changed: 1, Skipped: []string{"zz/..\\bad"}}, nil
			}
		})
	})
	rep := okReply(t, tcall(t, cs, "set_marks", args34(r, map[string]any{"ids": []any{idB, idB + "-2", " " + idA + " ", idB}, "rating": 0, "flag": "reject", "label": "none"})))
	got := r.fx.edit.got34().lastMark
	if !slices.Equal(got.IDs, []string{idB, idA}) || !got.SetRating || got.Rating != 0 || got.Flag != "reject" || got.Label != "none" || got.Client != "Claude Code" {
		t.Fatalf("MarkReq: %+v", got)
	}
	if sk, _ := rep.Data["skipped"].([]any); len(sk) != 1 || sk[0] != "zz_.._bad" || rep.Data["changed"] != float64(1) {
		t.Fatalf("ответ: %s", rep.Raw)
	}
	n := r.fx.edit.nMark.Load()
	for _, bad := range []map[string]any{
		{"ids": []any{idB}},                                 // ни одного поля
		{"ids": []any{idB}, "rating": 6},                    // выше предела
		{"ids": []any{idB}, "rating": -1},                   // ниже предела
		{"ids": []any{idB}, "flag": "up"},                   // не из перечисления
		{"ids": []any{idB}, "label": "pink"},                // не из перечисления
		{"ids": []any{}, "flag": "pick"},                    // пусто
		{"ids": manyIDs(maxMarkIDs + 1), "flag": "pick"},    // больше предела
		{"ids": []any{idB, 5}, "flag": "pick"},              // не строка
		{"ids": []any{idB}, "flag": "pick", "extra": "yes"}, // лишнее поле
	} {
		if rp := tcall(t, cs, "set_marks", args34(r, bad)); !rp.IsError {
			t.Errorf("вход %v принят", bad)
		}
	}
	if r.fx.edit.nMark.Load() != n {
		t.Fatal("Host.Mark вызван при негодном входе")
	}
	// Ровно предел — проходит.
	okReply(t, tcall(t, cs, "set_marks", args34(r, map[string]any{"ids": manyIDs(maxMarkIDs), "flag": "pick"})))
	if got := r.fx.edit.got34().lastMark; len(got.IDs) != maxMarkIDs {
		t.Fatalf("на пределе передано %d id", len(got.IDs))
	}
}

func manyIDs(n int) []any {
	out := make([]any, n)
	for i := range out {
		out[i] = "ph" + string(rune('a'+i%26)) + string(rune('a'+i/26%26)) + string(rune('a'+i/676))
	}
	return out
}

// TestAgentCoverArgs: фото — из ленты и читаемое; пропавшее и нечитаемое — отказ до Host; изделие созданной
// обложки на уровне 1 скрыто так же, как в get_album.
func TestAgentCoverArgs(t *testing.T) {
	r, cs := editRig(t, Options{}, func(e *editFake) {
		e.set34(func(f *fake34) {
			f.cover = func(CoverReq) (CoverRes, error) {
				return CoverRes{Album: AlbumRef{Key: albumKey(7), Rev: 9}, Created: true, Product: "ZQ24prof-album", Place: "seat", Changed: true}, nil
			}
		})
	})
	failText(t, tcall(t, cs, "set_cover_photo", args34(r, map[string]any{"photo": "nope"})), errNoPhoto)
	failText(t, tcall(t, cs, "set_cover_photo", args34(r, map[string]any{"photo": idC})), errPhotoGone)
	failText(t, tcall(t, cs, "set_cover_photo", args34(r, map[string]any{"photo": idD})), errPhotoGone)
	if r.fx.edit.nCover.Load() != 0 {
		t.Fatal("Host.Cover вызван для негодного фото")
	}
	rep := okReply(t, tcall(t, cs, "set_cover_photo", args34(r, map[string]any{"photo": idB + "-3"})))
	if g := r.fx.edit.got34().lastCover; g.Photo != idB {
		t.Fatalf("CoverReq: %+v", g)
	}
	if rep.Data["created"] != true || rep.Data["place"] != "seat" || rep.Data["changed"] != true {
		t.Fatalf("ответ: %s", rep.Raw)
	}
	if p, _ := rep.Data["product"].(string); p != ownPrintID("ZQ24prof") {
		t.Fatalf("изделие на уровне 1 не скрыто: %q", p)
	}
}

// TestAgentBgArgs: плитка — из каталога фонов; листы — номера или id, обложка не цель; Skipped — номера.
func TestAgentBgArgs(t *testing.T) {
	r, cs := editRig(t, Options{}, func(e *editFake) {
		e.set34(func(f *fake34) {
			f.bg = func(in BgReq) (BgRes, error) {
				return BgRes{Album: AlbumRef{Key: albumKey(7), Rev: 6}, Changed: 1, Skipped: []string{"sp2"}, Replaced: in.Replace}, nil
			}
		})
	})
	failText(t, tcall(t, cs, "set_background", args34(r, map[string]any{"item": "sa:nope", "spreads": []any{1}})), errNoDecor)
	failText(t, tcall(t, cs, "set_background", args34(r, map[string]any{"item": "sa:plain", "spreads": []any{"cover"}})), errCoverNotTarget("set_background"))
	failText(t, tcall(t, cs, "set_background", args34(r, map[string]any{"item": "sa:plain", "spreads": []any{9}})), errNoSpreadFlat)
	if r.fx.edit.nBg.Load() != 0 {
		t.Fatal("Host.Bg вызван при негодном входе")
	}
	rep := okReply(t, tcall(t, cs, "set_background", args34(r, map[string]any{"item": "sa:plain", "spreads": []any{1, "sp2", 2}, "replace": true})))
	g := r.fx.edit.got34().lastBg
	if g.Item != "sa:plain" || !slices.Equal(g.Spreads, []string{"sp1", "sp2"}) || g.Scope != "spread" || !g.Replace {
		t.Fatalf("BgReq: %+v", g)
	}
	if sk, _ := rep.Data["skipped"].([]any); len(sk) != 1 || sk[0] != float64(2) || rep.Data["replaced"] != true {
		t.Fatalf("ответ: %s", rep.Raw)
	}
}

// TestDecorIDNotGet: плитка из «Можно скачать» (Get) и чужого вида в каталог правок не попадает.
func TestDecorIDNotGet(t *testing.T) {
	r, cs := editRig(t, Options{}, nil)
	failText(t, tcall(t, cs, "add_sticker", args34(r, map[string]any{"stickers": []any{map[string]any{"spread": 1, "item": "get:1"}}})), errNoDecor)
	failText(t, tcall(t, cs, "add_sticker", args34(r, map[string]any{"stickers": []any{map[string]any{"spread": 1, "item": "sa:plain"}}})), errNoDecor) // фон — не стикер
	failText(t, tcall(t, cs, "set_background", args34(r, map[string]any{"item": "get:1", "spreads": []any{1}})), errNoDecor)
	failText(t, tcall(t, cs, "set_background", args34(r, map[string]any{"item": "sa:star", "spreads": []any{1}})), errNoDecor) // стикер — не фон
	if n := r.fx.edit.writes34(); n != 0 {
		t.Fatalf("Host дошёл до правки: %d", n)
	}
}

// TestAgentStickerArgs: параллельные срезы — по стикеру, умолчания page/place/size, предел 12, обложка — отказ.
func TestAgentStickerArgs(t *testing.T) {
	r, cs := editRig(t, Options{}, nil)
	st := func(m map[string]any) map[string]any { return m }
	rep := okReply(t, tcall(t, cs, "add_sticker", args34(r, map[string]any{"stickers": []any{
		st(map[string]any{"spread": 1, "item": "sa:star"}),
		st(map[string]any{"spread": "sp2", "page": "left", "item": "sa:star", "place": "top-left", "size": "l"}),
	}})))
	g := r.fx.edit.got34().lastStk
	if !slices.Equal(g.Spreads, []string{"sp1", "sp2"}) || !slices.Equal(g.Pages, []string{"right", "left"}) || !slices.Equal(g.Items, []string{"sa:star", "sa:star"}) ||
		!slices.Equal(g.Places, []string{"center", "top-left"}) || !slices.Equal(g.Sizes, []string{"m", "l"}) {
		t.Fatalf("StickerReq: %+v", g)
	}
	if rep.Data["added"] != float64(2) {
		t.Fatalf("ответ: %s", rep.Raw)
	}
	n := r.fx.edit.nStk.Load()
	// «cover» с 154 — цель: пачка идёт в Host с ID обложки.
	okReply(t, tcall(t, cs, "add_sticker", args34(r, map[string]any{"stickers": []any{st(map[string]any{"spread": "cover", "item": "sa:star"})}})))
	if g := r.fx.edit.got34().lastStk; !slices.Equal(g.Spreads, []string{model.CoverID}) || r.fx.edit.nStk.Load() != n+1 {
		t.Fatalf("StickerReq на обложку: %+v", g)
	}
	n = r.fx.edit.nStk.Load()
	failText(t, tcall(t, cs, "add_sticker", args34(r, map[string]any{"stickers": []any{st(map[string]any{"spread": 1, "item": "sa:star"}), st(map[string]any{"spread": 77, "item": "sa:star"})}})), errNoSpreadFlat)
	many := make([]any, MaxStickers+1)
	for i := range many {
		many[i] = st(map[string]any{"spread": 1, "item": "sa:star"})
	}
	for _, bad := range []map[string]any{
		{"stickers": many}, {"stickers": []any{}},
		{"stickers": []any{st(map[string]any{"spread": 1, "item": "sa:star", "size": "xl"})}},
		{"stickers": []any{st(map[string]any{"spread": 1, "item": "sa:star", "place": "middle"})}},
		{"stickers": []any{st(map[string]any{"spread": 1, "item": "sa:star", "page": "both"})}},
		{"stickers": []any{st(map[string]any{"spread": 1, "item": "sa:star", "x": 1})}},
	} {
		if rp := tcall(t, cs, "add_sticker", args34(r, bad)); !rp.IsError {
			t.Errorf("вход принят: %.80v", bad)
		}
	}
	if r.fx.edit.nStk.Load() != n {
		t.Fatal("Host.Stickers вызван при негодном входе")
	}
	full := many[:MaxStickers] // ровно 12 — проходит
	okReply(t, tcall(t, cs, "add_sticker", args34(r, map[string]any{"stickers": full})))
	if g := r.fx.edit.got34().lastStk; len(g.Items) != MaxStickers {
		t.Fatalf("на пределе передано %d", len(g.Items))
	}
}

// TestAgentPrintArgs: «×» и регистр в имени формата не важны; нет формата — первый; неизвестные профиль и
// формат — errNoProfile без Host. Уровень 1: не встроенные — по маске own-<хэш> и «Формат <хэш>», настоящие id и имена не годны.
func TestAgentPrintArgs(t *testing.T) {
	r, cs := editRig(t, Options{}, nil)
	r.fx.mu.Lock()
	r.fx.prints[0].Formats = append(r.fx.prints[0].Formats, printprof.InfoFormat{Name: "30×20", PageW: 300, PageH: 200})
	r.fx.mu.Unlock()
	for _, f := range []string{"30x20", "30X20", " 30 х 20 ", "30×20"} {
		okReply(t, tcall(t, cs, "apply_print", args34(r, map[string]any{"profile": "b1", "format": f})))
		if g := r.fx.edit.got34().lastPrint; g.Profile != "b1" || g.Format != "30×20" || g.Client != "Claude Code" {
			t.Fatalf("формат %q: %+v", f, g)
		}
	}
	okReply(t, tcall(t, cs, "apply_print", args34(r, map[string]any{"profile": "b1"})))
	if g := r.fx.edit.got34().lastPrint; g.Format != "30x30" {
		t.Fatalf("первый формат: %+v", g)
	}
	n := r.fx.edit.nPrint.Load()
	failText(t, tcall(t, cs, "apply_print", args34(r, map[string]any{"profile": "zzz"})), errNoProfile)
	failText(t, tcall(t, cs, "apply_print", args34(r, map[string]any{"profile": "b1", "format": "99x99"})), errNoProfile)
	failText(t, tcall(t, cs, "apply_print", args34(r, map[string]any{"profile": "own1"})), errNoProfile)                                      // настоящий id скрыт
	failText(t, tcall(t, cs, "apply_print", args34(r, map[string]any{"profile": ownPrintID("own1"), "format": "ZQ18 формат"})), errNoProfile) // настоящее имя скрыто
	if r.fx.edit.nPrint.Load() != n {
		t.Fatal("Host.Print вызван при негодном входе")
	}
	okReply(t, tcall(t, cs, "apply_print", args34(r, map[string]any{"profile": ownPrintID("own1"), "format": ownFormatName("own1", "ZQ18 формат")})))
	if g := r.fx.edit.got34().lastPrint; g.Profile != "own1" || g.Format != "ZQ18 формат" {
		t.Fatalf("маска own-2 → %+v", g)
	}
	r.fx.edit.set34(func(f *fake34) {
		f.print_ = func(PrintReq) (PrintRes, error) {
			return PrintRes{Album: r.fx.bump(), PageW: 200, PageH: 200, Cover: "own1-album", CoverChanged: true, Changed: true, OverLimit: true}, nil
		}
	})
	rep := okReply(t, tcall(t, cs, "apply_print", args34(r, map[string]any{"profile": ownPrintID("own1")})))
	if c, _ := rep.Data["cover"].(string); c == "" || c == "own1-album" || rep.Data["overLimit"] != true || rep.Data["coverChanged"] != true {
		t.Fatalf("ответ (изделие своей типографии должно быть скрыто): %s", rep.Raw)
	}
}

// TestUndoLastNewTools: правка с шагом любого из новых инструментов запоминается для undo_last_edit, холостая
// (changed false / 0) — нет: отмена после неё «не своя».
func TestUndoLastNewTools(t *testing.T) {
	calls := []struct {
		name string
		args func(*rig) map[string]any
		noop func(*fake34)
	}{
		{"set_marks", func(r *rig) map[string]any { return args34(r, map[string]any{"ids": []any{idB}, "flag": "pick"}) },
			func(f *fake34) { f.mark = func(MarkReq) (MarkRes, error) { return MarkRes{Album: AlbumRef{}}, nil } }},
		{"set_cover_photo", func(r *rig) map[string]any { return args34(r, map[string]any{"photo": idB}) },
			func(f *fake34) { f.cover = func(CoverReq) (CoverRes, error) { return CoverRes{Place: "seat"}, nil } }},
		{"set_background", func(r *rig) map[string]any {
			return args34(r, map[string]any{"item": "sa:plain", "spreads": []any{1}})
		}, func(f *fake34) { f.bg = func(BgReq) (BgRes, error) { return BgRes{}, nil } }},
		{"add_sticker", func(r *rig) map[string]any {
			return args34(r, map[string]any{"stickers": []any{map[string]any{"spread": 1, "item": "sa:star"}}})
		}, func(f *fake34) { f.stk = func(StickerReq) (StickerRes, error) { return StickerRes{}, nil } }},
		{"apply_print", func(r *rig) map[string]any { return args34(r, map[string]any{"profile": "b1"}) },
			func(f *fake34) { f.print_ = func(PrintReq) (PrintRes, error) { return PrintRes{}, nil } }},
	}
	for _, c := range calls {
		t.Run(c.name, func(t *testing.T) {
			r, cs := editRig(t, Options{}, nil)
			rep := okReply(t, tcall(t, cs, c.name, c.args(r)))
			ref := rep.Data["album"].(map[string]any)
			rev := int(ref["rev"].(float64))
			if got := okReply(t, tcall(t, cs, "undo_last_edit", map[string]any{"album": ref})); int(got.Data["album"].(map[string]any)["rev"].(float64)) != rev+1 {
				t.Fatalf("отмена не дошла до Host: %s", got.Raw)
			}
			// Холостая: записи нет.
			r.fx.edit.set34(c.noop)
			rep = okReply(t, tcall(t, cs, c.name, c.args(r)))
			failText(t, tcall(t, cs, "undo_last_edit", map[string]any{"album": r.fx.refArgs()}), errUndoNotOwn)
			_ = rep
		})
	}
}

// TestSpreadFamilyOut: family в list_spreads и get_spread — из Host.Families; чужое слово отбрасывается;
// у обложки поля нет; Host без Families (старый) — поля нет.
func TestSpreadFamilyOut(t *testing.T) {
	var asked [][]string
	_, cs := editRig(t, Options{}, func(e *editFake) {
		e.set34(func(f *fake34) {
			f.fams = func(ids []string) []string {
				asked = append(asked, slices.Clone(ids))
				out := make([]string, len(ids))
				for i, id := range ids {
					out[i] = map[string]string{"sp1": "dense", "sp2": "weird"}[id]
				}
				return out
			}
		})
	})
	list := okReply(t, tcall(t, cs, "list_spreads", nil))
	items := list.Data["items"].([]any)
	if items[0].(map[string]any)["family"] != "dense" {
		t.Fatalf("list_spreads[0]: %v", items[0])
	}
	if _, has := items[1].(map[string]any)["family"]; has {
		t.Fatalf("чужое слово семейства вышло наружу: %v", items[1])
	}
	if len(asked) != 1 || !slices.Equal(asked[0], []string{"sp1", "sp2"}) {
		t.Fatalf("Families спрошен: %v", asked)
	}
	if sp := okReply(t, tcall(t, cs, "get_spread", map[string]any{"spread": 1})); sp.Data["family"] != "dense" {
		t.Fatalf("get_spread: %s", sp.Raw)
	}
	if sp := okReply(t, tcall(t, cs, "get_spread", map[string]any{"spread": "cover"})); sp.Data["family"] != nil {
		t.Fatalf("у обложки семейство: %s", sp.Raw)
	}
}
