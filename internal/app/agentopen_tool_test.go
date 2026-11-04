package app

// Ссылка «Не спрашивать» в вопросе открытия (160 Т5, 160-Т5-8): Tool только там, где «Всегда» сделает вопрос тихим;
// строка «Место … — на диске …» в вопросе открытия с причиной disk.

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/app/agents"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

// TestOpenAskTool: три случая вызова × режим. Tool = open_album только при «Спрашивать», ключе и без name_hint.
// При «Всегда» ссылки нет: вопрос бывает только когда тихо нельзя, «Не спрашивать» там не к чему.
func TestOpenAskTool(t *testing.T) {
	r := newOpenRig(t)
	key := r.req(r.target)
	hinted := key
	hinted.Hint = "Ивановых"
	pick := agents.AskReq{Kind: agents.AskOpen, Client: "Claude Code"}
	for _, c := range []struct {
		name string
		mode string
		req  agents.AskReq
		want string
	}{
		{"ключ, спрашивать", prefs.PermAsk, key, toolOpenAlbum},
		{"слова, спрашивать", prefs.PermAsk, hinted, ""},
		{"выбор файла, спрашивать", prefs.PermAsk, pick, ""},
		{"ключ, всегда", prefs.PermAllow, key, ""},
		{"слова, всегда", prefs.PermAllow, hinted, ""},
		{"выбор файла, всегда", prefs.PermAllow, pick, ""},
	} {
		prefs.UpdateAgentPerms(r.a.cfgDir, func(p *prefs.AgentPerms) { p.Tools["open_album"] = c.mode })
		n := len(r.evOf("agents:ask"))
		call := r.ask(c.req)
		ev := r.asked(t, n+1)
		if ev.Tool != c.want {
			t.Errorf("%s: Tool = %q, ждали %q", c.name, ev.Tool, c.want)
		}
		if _, err := r.a.AgentAnswerOpen(ev.ID, false); err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		call.wait(t)
	}
}

// TestOpenAskDiskNote: при «Всегда» и фото на другом диске вопрос открытия несёт причину disk и строку «Место … —
// на диске …», как вопрос о папке.
func TestOpenAskDiskNote(t *testing.T) {
	r := newOQRig(t)
	root := filepath.Dir(r.place)
	f := &fakeVols{}
	f.set(fakeVol{root, "card1", "Фото-архив"})
	useFakeVols(t, f)
	prefs.UpdateAgentPerms(r.a.cfgDir, func(p *prefs.AgentPerms) {
		p.Places = []prefs.Place{{Dir: r.place, How: prefs.PlaceChosen, Vol: &prefs.PlaceVol{ID: "card1", Label: "Фото-архив", Rel: "Архив"}}}
	})
	f.set(fakeVol{root, "stranger", "Чужая"})
	c := r.ask(r.req(r.target))
	ev := r.asked(t, 1)
	if ev.Why != whyDisk || ev.Tool != "" {
		t.Fatalf("Why = %q, Tool = %q", ev.Why, ev.Tool)
	}
	if !strings.Contains(strings.Join(ev.Lines, "\n"), "Место «"+r.place+"» — на диске «Фото-архив»") {
		t.Errorf("строки вопроса: %q", ev.Lines)
	}
	_, _ = r.a.AnswerAgent(ev.ID, false)
	c.wait(t)
}
