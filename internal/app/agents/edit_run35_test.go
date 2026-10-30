package agents

// Правки 154 на службе: фон альбома (to: album), «cover» у стикеров, было/стало семейства у vary_layouts.

import (
	"context"
	"testing"
)

// TestBgAlbumArgs: проверки входа to: album идут до Host; Host не вызван.
func TestBgAlbumArgs(t *testing.T) {
	r, cs := editRig(t, Options{}, nil)
	for name, kv := range map[string]map[string]any{
		"scope с album":           {"item": "sa:plain", "to": "album", "scope": "left"},
		"replace без spreads":     {"item": "sa:plain", "to": "album", "replace": true},
		"spreads без replace":     {"item": "sa:plain", "to": "album", "spreads": []any{1}},
		"cover в листах согласия": {"item": "sa:plain", "to": "album", "replace": true, "spreads": []any{"cover"}},
		"pages без album":         {"item": "sa:plain", "pages": true, "spreads": []any{1}},
		"spreads пусты":           {"item": "sa:plain"},
		"to неизвестное":          {"item": "sa:plain", "to": "page", "spreads": []any{1}},
	} {
		if rep := tcall(t, cs, "set_background", args34(r, kv)); !rep.IsError {
			t.Errorf("%s: вход принят", name)
		}
	}
	if n := r.fx.edit.nBg.Load(); n != 0 {
		t.Fatalf("Host.Bg вызван %d раз при негодном входе", n)
	}
	okReply(t, tcall(t, cs, "set_background", args34(r, map[string]any{"item": "sa:plain", "to": "album", "pages": true})))
	if g := r.fx.edit.got34().lastBg; !g.Album || !g.Pages || g.Replace || len(g.Spreads) != 0 {
		t.Fatalf("BgReq: %+v", g)
	}
}

// TestStickerCoverRef: «cover» годится целью стикера и уходит в Host с ID обложки.
func TestStickerCoverRef(t *testing.T) {
	r, cs := editRig(t, Options{}, nil)
	okReply(t, tcall(t, cs, "add_sticker", args34(r, map[string]any{"stickers": []any{map[string]any{"spread": "cover", "item": "sa:star", "page": "left"}}})))
	if g := r.fx.edit.got34().lastStk; len(g.Spreads) != 1 || g.Spreads[0] != "cover" || g.Pages[0] != "left" {
		t.Fatalf("StickerReq: %+v", g)
	}
}

// TestVaryFamiliesOut: пара «до/после» превращается в families с номерами; нечестная пара — поля нет.
func TestVaryFamiliesOut(t *testing.T) {
	r, cs := editRig(t, Options{}, nil)
	r.fx.edit.vary = func(_ context.Context, in VaryReq) (VaryRes, error) {
		return VaryRes{Album: r.fx.bump(), Changed: 1, Spreads: in.Spreads, Before: []string{"air"}, After: []string{"dense"}}, nil
	}
	rep := okReply(t, tcall(t, cs, "vary_layouts", args34(r, map[string]any{"spreads": []any{1}})))
	fams, _ := rep.Data["families"].([]any)
	if len(fams) != 1 {
		t.Fatalf("ответ: %s", rep.Raw)
	}
	if f := fams[0].(map[string]any); f["before"] != "air" || f["after"] != "dense" || f["spread"] != float64(1) {
		t.Fatalf("families: %v", f)
	}
	r.fx.edit.vary = func(_ context.Context, in VaryReq) (VaryRes, error) {
		return VaryRes{Album: r.fx.bump(), Changed: 1, Spreads: in.Spreads, Before: []string{"air"}}, nil
	}
	if rep = okReply(t, tcall(t, cs, "vary_layouts", args34(r, map[string]any{"spreads": []any{1}}))); rep.Data["families"] != nil {
		t.Fatalf("пара разной длины отдана: %s", rep.Raw)
	}
}
