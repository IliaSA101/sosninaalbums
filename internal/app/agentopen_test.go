package app

// Открытие альбома по просьбе агента (152 Т4): вопрос с ключом и без, окно ОС, занятость, битый и новый
// файл, тост «Открыть прежний». Служебная часть (схема, name_hint) — internal/app/agents/write_tools_test.go.

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	wr "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/app/agents"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// openRig — стенд вопросов с двумя сохранёнными альбомами в «Последних»; открыт сохранённый «Прежний».
type openRig struct {
	*askRig
	dir, prev, target string
	dialog            func(wr.OpenDialogOptions) (string, error)
	dialogs           []wr.OpenDialogOptions
}

func newOpenRig(t *testing.T) *openRig {
	t.Helper()
	r := &openRig{askRig: newAskRig(t), dir: testkit.TempDir(t)}
	r.prev = filepath.Join(r.dir, "Прежний.salbum")
	r.target = filepath.Join(r.dir, "Ивановы.salbum")
	for _, p := range []string{r.target, r.prev} { // последний открыт
		if err := blankEditor("album").Save(p); err != nil {
			t.Fatal(err)
		}
		r.a.rememberProject(p)
	}
	if _, err := r.a.openPath(r.prev); err != nil {
		t.Fatal(err)
	}
	old := openFileDialog
	t.Cleanup(func() { openFileDialog = old })
	openFileDialog = func(_ context.Context, o wr.OpenDialogOptions) (string, error) {
		r.dialogs = append(r.dialogs, o)
		if r.dialog != nil {
			return r.dialog(o)
		}
		return "", nil
	}
	return r
}

func (r *openRig) req(path string) agents.AskReq {
	return agents.AskReq{Kind: agents.AskOpen, Client: "Claude Code", Key: agents.RecentKey(path)}
}

// TestAgentOpenKeyLikeRecent: ключ проверяется как в «Файл ▸ Последние»: нет в последних, UNC, не альбом — отказ
// до окна, без пути в тексте.
func TestAgentOpenKeyLikeRecent(t *testing.T) {
	r := newOpenRig(t)
	unc, notAlbum := "//srv/share/Чужой.salbum", filepath.Join(r.dir, "заметки.txt")
	r.a.updateConfig(func(c *prefs.Config) { c.Recent = append(c.Recent, unc, notAlbum) })
	for _, c := range []struct {
		name string
		req  agents.AskReq
		want error
	}{
		{"нет ключа", agents.AskReq{Kind: agents.AskOpen, Key: "ffffffffffff"}, agents.ErrNoRecent},
		{"UNC", r.req(unc), agents.ErrOpenRemote},
		{"не альбом", r.req(notAlbum), agents.ErrOpenFailed},
	} {
		call := r.ask(c.req)
		call.wait(t)
		if !errors.Is(call.err, c.want) {
			t.Errorf("%s: %v, ждали %v", c.name, call.err, c.want)
		}
	}
	if n := len(r.evOf("agents:ask")); n != 0 {
		t.Errorf("вопросов %d", n)
	}
}

// Ключ → вопрос с полным путём и «Агент назвал», «Открыть» → альбом открыт, прежний закреплён в «Открыть
// прежний», тост без «Вернуть», undo агента не отменяет открытие.
func TestAgentOpenFlow(t *testing.T) {
	r := newOpenRig(t)
	a := r.a
	prev := a.editor()
	req := r.req(r.target)
	req.Hint = "Ивановых"
	c := r.ask(req)
	ev := r.asked(t, 1)
	if ev.Kind != agents.AskOpen || ev.Path != r.target || ev.OK != askOKOpen || ev.Alt != askAltOpen || ev.Tool != "" ||
		!strings.Contains(ev.Title, "«Ивановы»") || strings.Join(ev.Lines, "\n") == "" ||
		!strings.Contains(strings.Join(ev.Lines, "\n"), "Агент назвал: «Ивановых»") ||
		!strings.Contains(strings.Join(ev.Lines, "\n"), "«Прежний» сохранится в свой файл") {
		t.Errorf("agents:ask: %+v", ev)
	}
	st, err := a.AgentAnswerOpen(ev.ID, false)
	if err != nil || st.Cancelled {
		t.Fatalf("AgentAnswerOpen: %v, %+v", err, st.Cancelled)
	}
	c.wait(t)
	if c.err != nil || c.res.Why != agents.WhyAnswered || c.res.Album.Key != r.key() || c.res.Album.Rev != r.rev() {
		t.Fatalf("итог: %+v, %v", c.res, c.err)
	}
	if a.editor() == prev || a.editor().Path() != r.target || st.View.Path != r.target {
		t.Errorf("открыт не тот альбом: %q", a.editor().Path())
	}
	if len(r.dialogs) != 0 {
		t.Error("окно ОС при показанном альбоме")
	}
	edited := r.evOf("agents:edited")
	if len(edited) != 1 {
		t.Fatalf("тостов %d", len(edited))
	}
	if e := edited[0].(AgentEdited); e.Client != "Claude Code" || e.Text != "Открыт альбом «Ивановы»" || !e.Prev || e.Rev >= 0 {
		t.Errorf("тост: %+v", e)
	}
	if got, err := a.AgentOpenPrev(); err != nil || got.View.Path != r.prev {
		t.Errorf("Открыть прежний: %v, %q", err, got.View.Path)
	}
}

// TestAgentAnswerOpen: вопрос без ключа — «Выбрать файл…» и окно ОС (имя клиента в заголовке); «Выбрать
// другой…» у показанного — окно в его папке; отмена окна — «остановлено», вопрос закрыт, альбом тот же.
func TestAgentAnswerOpen(t *testing.T) {
	r := newOpenRig(t)
	a := r.a
	prev := a.editor()
	r.dialog = func(wr.OpenDialogOptions) (string, error) { return r.target, nil }
	c := r.ask(agents.AskReq{Kind: agents.AskOpen, Client: "Claude Code"})
	ev := r.asked(t, 1)
	if ev.OK != askOKOpenPick || ev.Path != "" || ev.Alt != "" || strings.Contains(ev.Title, "«") {
		t.Errorf("вопрос без ключа: %+v", ev)
	}
	if _, err := a.AgentAnswerOpen(ev.ID, false); err != nil {
		t.Fatal(err)
	}
	c.wait(t)
	if c.res.Why != agents.WhyAnswered || a.editor().Path() != r.target {
		t.Fatalf("итог %+v, путь %q", c.res, a.editor().Path())
	}
	if len(r.dialogs) != 1 || !strings.Contains(r.dialogs[0].Title, "Claude Code") || !strings.HasSuffix(r.dialogs[0].Filters[0].Pattern, projectExt) {
		t.Errorf("окно ОС: %+v", r.dialogs)
	}

	// «Выбрать другой…» у показанного альбома: окно в его папке; отмена окна — WhyStopped.
	r.dialog = func(wr.OpenDialogOptions) (string, error) { return "", nil }
	c = r.ask(r.req(r.prev))
	ev = r.asked(t, 2)
	cur := a.editor()
	st, err := a.AgentAnswerOpen(ev.ID, true)
	c.wait(t)
	if err != nil || !st.Cancelled || c.res.Why != agents.WhyStopped || c.err != nil {
		t.Errorf("отмена окна: %v, cancelled %v, %+v, %v", err, st.Cancelled, c.res, c.err)
	}
	if got := r.dialogs[1].DefaultDirectory; got != r.dir {
		t.Errorf("окно открыто в %q, ждали %q", got, r.dir)
	}
	if a.editor() != cur || cur == prev {
		t.Error("альбом сменился после отмены окна")
	}
	if d := r.done(t, 2); d.Why != agents.WhyStopped {
		t.Errorf("agents:askdone: %+v", d)
	}
	if m := a.askM(); m.cur != nil {
		t.Error("вопрос остался открытым")
	}
}

// TestOpenAlbumBusy: идёт выгрузка — вопроса нет, «занято»; выгрузка началась, пока висел вопрос, — отказ
// после «Открыть», альбом цел; второй вопрос при открытом первом — ErrAskBusy.
func TestOpenAlbumBusy(t *testing.T) {
	r := newOpenRig(t)
	a := r.a
	prev := a.editor()
	busy := true
	old := exportRunning
	t.Cleanup(func() { exportRunning = old })
	exportRunning = func(*App) bool { return busy }

	c := r.ask(r.req(r.target))
	c.wait(t)
	if !errors.Is(c.err, agents.ErrExporting) || errors.Is(c.err, agents.ErrBusy) || len(r.evOf("agents:ask")) != 0 {
		t.Fatalf("при выгрузке: %v, событий %d", c.err, len(r.evOf("agents:ask")))
	}

	busy = false
	c = r.ask(r.req(r.target))
	ev := r.asked(t, 1)
	second := r.ask(r.req(r.prev))
	second.wait(t)
	if !errors.Is(second.err, agents.ErrAskBusy) {
		t.Errorf("второй вопрос: %v", second.err)
	}
	busy = true
	st, err := a.AgentAnswerOpen(ev.ID, false)
	c.wait(t)
	if err != nil || !st.Cancelled || !errors.Is(c.err, agents.ErrExporting) || a.editor() != prev {
		t.Errorf("после «Открыть» при выгрузке: %v, %v, %v, тот же %v", err, st.Cancelled, c.err, a.editor() == prev)
	}
}

// TestAgentOpenBroken: битый файл — ошибка человеку (как в меню) и ErrOpenFailed агенту без пути, альбом тот же;
// файл из новой версии — окно «Альбом из новой версии» (State.Newer) и ErrOpenNewer.
func TestAgentOpenBroken(t *testing.T) {
	r := newOpenRig(t)
	a := r.a
	prev := a.editor()
	broken := filepath.Join(r.dir, "Битый.salbum")
	if err := os.WriteFile(broken, []byte("не альбом"), 0o644); err != nil {
		t.Fatal(err)
	}
	a.rememberProject(broken)
	c := r.ask(r.req(broken))
	_, err := a.AgentAnswerOpen(r.asked(t, 1).ID, false)
	c.wait(t)
	if err == nil || !errors.Is(c.err, agents.ErrOpenFailed) || a.editor() != prev {
		t.Fatalf("битый: ошибка человеку %v, агенту %v, тот же %v", err, c.err, a.editor() == prev)
	}
	if strings.Contains(c.err.Error(), r.dir) {
		t.Errorf("путь в ошибке агенту: %v", c.err)
	}
}

// TestAgentOpenNewer: файл новее программы — окно «Альбом из новой версии» у человека (State.Newer), агенту
// ErrOpenNewer без пути; прежний альбом на месте, вопрос закрыт, «Открыть прежний» не предлагается.
func TestAgentOpenNewer(t *testing.T) {
	r := newOpenRig(t)
	a := r.a
	prev := a.editor()
	newer := filepath.Join(r.dir, "Новый.salbum")
	if err := os.WriteFile(newer, []byte(fmt.Sprintf(`{"version":%d,"spreads":"не список"}`, model.FormatVersion+1)), 0o644); err != nil {
		t.Fatal(err)
	}
	a.rememberProject(newer)
	c := r.ask(r.req(newer))
	ev := r.asked(t, 1)
	st, err := a.AgentAnswerOpen(ev.ID, false)
	c.wait(t)
	if err != nil || !st.Cancelled || st.Newer == nil {
		t.Fatalf("человеку: %v, cancelled %v, окно новой версии %v", err, st.Cancelled, st.Newer)
	}
	if !errors.Is(c.err, agents.ErrOpenNewer) || strings.Contains(c.err.Error(), r.dir) || strings.Contains(c.err.Error(), "Новый") {
		t.Errorf("агенту: %v", c.err)
	}
	m := a.askM()
	m.mu.Lock()
	cur, pv := m.cur, m.prev
	m.mu.Unlock()
	if a.editor() != prev || pv != "" || cur != nil || len(r.evOf("agents:edited")) != 0 {
		t.Errorf("альбом тронут: тот же %v, прежний %q, вопрос %v, тостов %d", a.editor() == prev, pv, cur != nil, len(r.evOf("agents:edited")))
	}
}

// TestAgentOpenOffAndDenied: разрешение «Запретить» снято до вопроса — итог off без окна; пока вопрос висел — off
// при «Открыть», альбом не тронут.
func TestAgentOpenOffAndDenied(t *testing.T) {
	r := newOpenRig(t)
	a := r.a
	prev := a.editor()
	prefs.UpdateAgentPerms(a.cfgDir, func(p *prefs.AgentPerms) { p.Tools["open_album"] = prefs.PermDeny })
	c := r.ask(r.req(r.target))
	c.wait(t)
	if c.res.Why != agents.WhyOff || len(r.evOf("agents:ask")) != 0 {
		t.Fatalf("при запрете: %+v, %v", c.res, c.err)
	}
	prefs.UpdateAgentPerms(a.cfgDir, func(p *prefs.AgentPerms) { p.Tools["open_album"] = prefs.PermAsk })
	c = r.ask(r.req(r.target))
	ev := r.asked(t, 1)
	prefs.UpdateAgentPerms(a.cfgDir, func(p *prefs.AgentPerms) { p.Tools["open_album"] = prefs.PermDeny })
	if st, err := a.AgentAnswerOpen(ev.ID, false); err != nil || !st.Cancelled {
		t.Errorf("AgentAnswerOpen после запрета: %v, %v", err, st.Cancelled)
	}
	c.wait(t)
	if c.res.Why != agents.WhyOff || a.editor() != prev {
		t.Errorf("итог %+v, тот же %v", c.res, a.editor() == prev)
	}
}

// Отказ «Отмена» закрывает вопрос открытия; ответ другого вида («да» через AnswerAgent) — ошибка, вопрос цел.
func TestAgentOpenDecline(t *testing.T) {
	r := newOpenRig(t)
	c := r.ask(r.req(r.target))
	ev := r.asked(t, 1)
	if _, err := r.a.AnswerAgent(ev.ID, true); !errors.Is(err, errAskClosed) {
		t.Errorf("«да» не тем методом: %v", err)
	}
	if _, err := r.a.AnswerAgent(ev.ID, false); err != nil {
		t.Fatal(err)
	}
	c.wait(t)
	if c.res.Why != agents.WhyDeclined {
		t.Errorf("итог %+v", c.res)
	}
}
