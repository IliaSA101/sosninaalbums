package agents

// Правки 154 на службе: фон альбома (to: album), «cover» у стикеров, было/стало семейства у vary_layouts.

import (
	"context"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

// TestBgAlbumArgs: негодный вход set_background отвергается до Host и до вопроса — в «Всегда» и «Спрашивать»;
// текст отказа точный (replace без spreads — по спеке), человеку вопрос не показывается.
func TestBgAlbumArgs(t *testing.T) {
	yes := true
	for _, mode := range []string{prefs.PermAllow, prefs.PermAsk} {
		r, cs := editRig(t, Options{}, nil)
		r.fx.edit.setPerm("set_background", mode)
		for name, c := range map[string]struct {
			kv   map[string]any
			want *agentErr
		}{
			"scope с album": {map[string]any{"item": "sa:plain", "to": "album", "scope": "left"},
				badArgs("с to: album scope не передают")},
			"replace без spreads": {map[string]any{"item": "sa:plain", "to": "album", "replace": true},
				badArgs("с to: album и replace назовите в spreads номера из skipped, на которые согласился человек")},
			"spreads без replace": {map[string]any{"item": "sa:plain", "to": "album", "spreads": []any{1}},
				badArgs("с to: album spreads передают только с replace: номера из skipped, на которые согласился человек")},
			"cover в листах согласия": {map[string]any{"item": "sa:plain", "to": "album", "replace": true, "spreads": []any{"cover"}},
				errCoverNotTarget("set_background")},
			"pages без album":        {map[string]any{"item": "sa:plain", "pages": yes, "spreads": []any{1}}, badArgs("pages передают только с to: album")},
			"spreads пусты":          {map[string]any{"item": "sa:plain"}, badArgs("spreads: назовите хотя бы один разворот")},
			"to неизвестное (схема)": {map[string]any{"item": "sa:plain", "to": "page", "spreads": []any{1}}, badArgs("поле «to»: допустимые значения: «spreads», «album»")},
			"cover на наборе":        {map[string]any{"item": "sa:plain", "spreads": []any{1, "cover"}}, errCoverNotTarget("set_background")},
			"cover с пробелами":      {map[string]any{"item": "sa:plain", "spreads": []any{" Cover "}}, errCoverNotTarget("set_background")},
		} {
			if rep := tcall(t, cs, "set_background", args34(r, c.kv)); !rep.IsError || rep.Text != c.want.Error() {
				t.Errorf("%s, %s: ждали %q, получили isError=%v %q", mode, name, c.want.Error(), rep.IsError, rep.Text)
			}
		}
		if n := r.fx.edit.nBg.Load(); n != 0 {
			t.Fatalf("%s: Host.Bg вызван %d раз при негодном входе", mode, n)
		}
		if n := r.fx.edit.nAsk.Load(); n != 0 {
			t.Fatalf("%s: человеку показан вопрос %d раз при негодном входе", mode, n)
		}
		okReply(t, tcall(t, cs, "set_background", args34(r, map[string]any{"item": "sa:plain", "to": "album", "pages": true})))
		if g := r.fx.edit.got34().lastBg; !g.Album || !g.Pages || g.Replace || len(g.Spreads) != 0 {
			t.Fatalf("BgReq: %+v", g)
		}
		if want := int32(map[string]int{prefs.PermAllow: 0, prefs.PermAsk: 1}[mode]); r.fx.edit.nAsk.Load() != want {
			t.Fatalf("%s: годный вход — вопросов %d, ждали %d", mode, r.fx.edit.nAsk.Load(), want)
		}
	}
}

// TestStickerCoverRef: «cover» годится целью стикера и уходит в Host с ID обложки в любом регистре и с пробелами;
// без обложки и у альбома-листа — errNoCover до Host и до вопроса, в «Всегда» и «Спрашивать».
func TestStickerCoverRef(t *testing.T) {
	for _, mode := range []string{prefs.PermAllow, prefs.PermAsk} {
		r, cs := editRig(t, Options{}, nil)
		r.fx.edit.setPerm("add_sticker", mode)
		for _, ref := range []string{"cover", "Cover", " COVER "} {
			okReply(t, tcall(t, cs, "add_sticker", args34(r, map[string]any{"stickers": []any{map[string]any{"spread": ref, "item": "sa:star", "page": "left"}}})))
			if g := r.fx.edit.got34().lastStk; len(g.Spreads) != 1 || g.Spreads[0] != "cover" || g.Pages[0] != "left" {
				t.Fatalf("%s, %q: StickerReq: %+v", mode, ref, g)
			}
		}
		for name, edit := range map[string]func(v *model.View){
			"нет обложки": func(v *model.View) { v.Cover = nil },
			"альбом-лист": func(v *model.View) { v.Settings.Sheet = true },
		} {
			r.fx.mu.Lock()
			saved := r.fx.view
			edit(&r.fx.view)
			r.fx.mu.Unlock()
			n, asks := r.fx.edit.nStk.Load(), r.fx.edit.nAsk.Load()
			failText(t, tcall(t, cs, "add_sticker", args34(r, map[string]any{"stickers": []any{map[string]any{"spread": "cover", "item": "sa:star"}}})), errNoCover)
			if r.fx.edit.nStk.Load() != n || r.fx.edit.nAsk.Load() != asks {
				t.Fatalf("%s, %s: отказ дошёл до Host или вопроса", mode, name)
			}
			r.fx.mu.Lock()
			r.fx.view = saved
			r.fx.mu.Unlock()
		}
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
