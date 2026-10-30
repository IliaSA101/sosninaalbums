package app

// Разрешение самого инструмента перечитывается перед записью и у правок 0.34 (150, хвост): пока
// правка ждала ленту, человек мог поставить «Запретить» именно ей. Запрет другого инструмента не мешает.

import (
	"errors"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/app/agents"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

// agentPermLevel (Host.PermLevel) читает config.json один раз на вызов, а значения те же, что у
// agentPerm и уровня из настроек: режим, причина blocked, уровень.
func TestAgentPermLevelWiring(t *testing.T) {
	a := newApp(t)
	if a.agentsHost().PermLevel == nil {
		t.Fatal("Host.PermLevel не подключён")
	}
	a.SetAgentsLevel(3)
	prefs.UpdateAgentPerms(a.cfgDir, func(p *prefs.AgentPerms) {
		p.Tools[toolVary] = prefs.PermDeny
		p.Tools[toolMarks] = prefs.PermAllow
		p.Tools["list_photos"] = prefs.PermAsk
	})
	for _, elevated := range []bool{false, true} {
		a.askM().elevated = func() bool { return elevated }
		for _, tool := range []string{toolVary, toolMarks, "list_photos", toolAutobuild, "get_album", "нет-такого"} {
			wantMode, wantBlocked := a.agentPerm(tool)
			before := prefs.ConfigReads()
			mode, blocked, level := a.agentsHost().PermLevel(tool)
			if n := prefs.ConfigReads() - before; n != 1 {
				t.Errorf("%s: чтений config.json %d, ждали 1", tool, n)
			}
			if mode != wantMode || blocked != wantBlocked || level != 3 || (blocked != "") != elevated {
				t.Errorf("%s elevated=%v: %q, %q, %d; agentPerm даёт %q, %q", tool, elevated, mode, blocked, level, wantMode, wantBlocked)
			}
		}
	}
	// Значение берётся из настройки, а не из умолчания.
	a.askM().elevated = func() bool { return false }
	if m, _, _ := a.agentPermLevel(toolVary); m != prefs.PermDeny {
		t.Errorf("vary_layouts: %q, ждали deny", m)
	}
}

func TestAgentEdit34DeniedTool(t *testing.T) {
	// Каждый инструмент — вызов Host с актуальным rev.
	calls := map[string]func(r *edRig, t *testing.T) error{
		toolMarks: func(r *edRig, t *testing.T) error {
			_, err := r.a.agentsHost().Mark(t.Context(), agents.MarkReq{Key: key(t, r), Rev: rev0(r), IDs: []string{r.photoID(0)}, Flag: "pick", Client: "К"})
			return err
		},
		toolCover: func(r *edRig, t *testing.T) error {
			_, err := r.a.agentsHost().Cover(t.Context(), agents.CoverReq{Key: key(t, r), Rev: rev0(r), Photo: r.photoID(0), Client: "К"})
			return err
		},
		toolBg: func(r *edRig, t *testing.T) error {
			sp := r.a.editor().View().Spreads
			_, err := r.a.agentsHost().Bg(t.Context(), agents.BgReq{Key: key(t, r), Rev: rev0(r), Item: r.decorID(t, "bg"), Spreads: []string{sp[0].ID}, Scope: "spread", Client: "К"})
			return err
		},
		toolSticker: func(r *edRig, t *testing.T) error {
			sp := r.a.editor().View().Spreads
			_, err := r.a.agentsHost().Stickers(t.Context(), agents.StickerReq{Key: key(t, r), Rev: rev0(r), Client: "К", Spreads: []string{sp[0].ID},
				Pages: []string{"right"}, Items: []string{r.decorID(t, "sticker")}, Places: []string{"center"}, Sizes: []string{"m"}})
			return err
		},
		toolPrint: func(r *edRig, t *testing.T) error {
			_, err := r.a.agentsHost().Print(t.Context(), agents.PrintReq{Key: key(t, r), Rev: rev0(r), Profile: "fabrika", Format: "30×20", Client: "К"})
			return err
		},
	}
	for tool, call := range calls {
		t.Run(tool+" сам запрещён", func(t *testing.T) {
			r := newEdRig34(t)
			denyTool(r.a, tool)
			rev := rev0(r)
			if err := call(r, t); !errors.Is(err, agents.ErrNoEdits) || rev0(r) != rev {
				t.Fatalf("запрет своего: %v, rev %d→%d", err, rev, rev0(r))
			}
		})
		t.Run(tool+" запрещён другой", func(t *testing.T) {
			r := newEdRig34(t)
			for other := range calls { // все остальные — «Запретить»
				if other != tool {
					denyTool(r.a, other)
				}
			}
			rev := rev0(r)
			if err := call(r, t); err != nil || rev0(r) == rev {
				t.Fatalf("запрет других: %v, rev %d→%d", err, rev, rev0(r))
			}
		})
		t.Run(tool+" причина blocked", func(t *testing.T) {
			r := newEdRig34(t)
			r.a.askM().elevated = func() bool { return true }
			rev := rev0(r)
			if err := call(r, t); !errors.Is(err, agents.ErrNoEdits) || rev0(r) != rev {
				t.Fatalf("blocked: %v, rev %d→%d", err, rev, rev0(r))
			}
		})
	}
}

func rev0(r *edRig) int { return r.a.editor().View().Rev }

// key — ключ альбома, как его видит агент (get_album).
func key(t *testing.T, r *edRig) string {
	t.Helper()
	return albumRef(t, r.call(t, "get_album", nil))["key"].(string)
}
