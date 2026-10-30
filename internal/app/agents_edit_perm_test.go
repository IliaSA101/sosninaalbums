package app

// Разрешение самого инструмента перечитывается перед записью (150, ревью): запрет другого
// инструмента правку не останавливает, запрет своего — останавливает.

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

func denyTool(a *App, tool string) {
	prefs.UpdateAgentPerms(a.cfgDir, func(p *prefs.AgentPerms) { p.Tools[tool] = prefs.PermDeny })
}

// Автосборка ждёт ленту; за это время человек ставит «Запретить» самой автосборке — правки нет.
// Запрет другого инструмента (vary_layouts) за то же время сборке не мешает.
func TestAutobuildDeniedWhileWaiting(t *testing.T) {
	for _, tc := range []struct {
		name, tool, mode string
		asked, wantOff   bool
	}{
		{"сам инструмент", toolAutobuild, prefs.PermDeny, false, true},
		{"другой инструмент", toolVary, prefs.PermDeny, false, false},
		{"Всегда сменилось на Спрашивать", toolAutobuild, prefs.PermAsk, false, true},
		{"вопрос уже задан, режим Спрашивать", toolAutobuild, prefs.PermAsk, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := newEditRig(t, 8)
			a := r.a
			a.mu.Lock()
			photos, byID := a.photos, a.byID
			a.photos, a.byID, a.scanning = nil, map[string]library.Photo{}, true
			a.mu.Unlock()
			time.AfterFunc(80*time.Millisecond, func() {
				prefs.UpdateAgentPerms(a.cfgDir, func(p *prefs.AgentPerms) { p.Tools[tc.tool] = tc.mode }) // смена приходит до конца чтения
				a.mu.Lock()
				a.photos, a.byID, a.scanning = photos, byID, false
				a.mu.Unlock()
			})
			rev := r.rev()
			q := r.req()
			q.Asked = tc.asked
			q.ScanWaitMs = int(testkit.Limit(5 * time.Second).Milliseconds())
			res, err := a.agentAuto(context.Background(), q)
			if tc.wantOff {
				if !errors.Is(err, errAgentOff) || r.rev() != rev || len(a.editor().View().Spreads) != 0 {
					t.Fatalf("запрет автосборки: %+v, %v, rev %d→%d", res, err, rev, r.rev())
				}
				return
			}
			if err != nil || res.Photos != 8 {
				t.Fatalf("запрет другого инструмента: %+v, %v", res, err)
			}
		})
	}
}

// vary_layouts и undo_last_edit: запрет своего инструмента — отказ без правки; чужого — правка проходит.
func TestVaryUndoDeniedTool(t *testing.T) {
	r := newEditRig(t, 8)
	a := r.a
	if _, err := a.agentAuto(context.Background(), r.req()); err != nil {
		t.Fatal(err)
	}
	v := a.editor().View()
	var ids []string
	for _, sv := range v.Spreads {
		ids = append(ids, sv.ID)
	}
	// Запрещена автосборка — vary и undo идут.
	denyTool(a, toolAutobuild)
	rev, _, err := a.agentVary(context.Background(), r.key(), v.Rev, ids, 2, nil)
	if err != nil {
		t.Fatalf("vary при запрете autobuild: %v", err)
	}
	// Запрещён vary — отказ, rev прежний.
	denyTool(a, toolVary)
	if _, _, err := a.agentVary(context.Background(), r.key(), rev, ids, 2, nil); !errors.Is(err, errAgentOff) || r.rev() != rev {
		t.Fatalf("vary при запрете vary: %v, rev %d→%d", err, rev, r.rev())
	}
	// Запрещён undo — отказ, правка не отменена.
	denyTool(a, toolUndo)
	if _, err := a.agentUndo(context.Background(), r.key(), rev); !errors.Is(err, errAgentOff) || r.rev() != rev {
		t.Fatalf("undo при запрете undo: %v, rev %d→%d", err, rev, r.rev())
	}
	prefs.UpdateAgentPerms(a.cfgDir, func(p *prefs.AgentPerms) { p.Tools[toolUndo] = prefs.PermAllow })
	if _, err := a.agentUndo(context.Background(), r.key(), rev); err != nil {
		t.Fatalf("undo при запрете других: %v", err)
	}
}
