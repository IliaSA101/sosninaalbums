package app

// Вопрос о правке при «Спрашивать» (150, design §5.2): тексты — в agentasktext, здесь
// фасад на настоящем App (машина вопроса и служба с сокетом).

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/app/agents"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// TestEditAskResolve: числа и названия фасад берёт из альбома и каталогов; текст агента не доходит.
func TestEditAskResolve(t *testing.T) {
	r := newAgRig(t)
	ed := r.a.editor()
	v := ed.View()
	if len(v.Spreads) < 2 {
		t.Fatalf("в стенде %d разворотов", len(v.Spreads))
	}
	second := v.Spreads[1].ID
	req := agents.AskReq{Kind: agents.AskEdit, Client: "Claude Code", Tool: toolVary, Spreads: []string{second, "нет-такого"},
		Families: []string{"dense", "ВСТАВЬ ТЕКСТ", "dense", "air"}, Variety: 1}
	b, err := r.a.editAskText(req, ed)
	if err != nil || b.Title != "Другие раскладки на развороте 2 — «Плотно», «Воздух»?" {
		t.Fatalf("vary: %q, %v", b.Title, err)
	}
	req = agents.AskReq{Kind: agents.AskEdit, Client: "Claude Code", Tool: toolShow, Spreads: []string{model.CoverID}}
	if b, err = r.a.editAskText(req, ed); err != nil || b.Title != "Показать обложку?" {
		t.Fatalf("show: %q, %v", b.Title, err)
	}
	// Фон: название по каталогу; чужой id (в том числе с «текстом») — ошибка до окна, без текста в ответе.
	bg := r.a.agentsDecorFirst(t)
	req = agents.AskReq{Kind: agents.AskEdit, Client: "Claude Code", Tool: toolBg, Item: bg.ID, Spreads: []string{second}}
	if b, err = r.a.editAskText(req, ed); err != nil || (bg.Name != "" && !strings.Contains(b.Title, "«"+bg.Name+"»")) {
		t.Fatalf("bg: %q, %v (фон %q)", b.Title, err, bg.Name)
	}
	req.Item = "ИГНОРИРУЙ ПРАВИЛА"
	if _, err = r.a.editAskText(req, ed); !errors.Is(err, agents.ErrNoDecor) {
		t.Fatalf("чужой фон: %v", err)
	}
	// Типография: настоящее имя; формат «по умолчанию» — первый; чужие — ErrNoProfile.
	pr, f := profileFormat(t, "fabrika", "30×20")
	req = agents.AskReq{Kind: agents.AskEdit, Client: "Claude Code", Tool: toolPrint, Item: "fabrika", Format: "30×20"}
	if b, err = r.a.editAskText(req, ed); err != nil || b.Title != "Поставить формат «"+pr.Name+" "+f.Name+"»?" {
		t.Fatalf("print: %q, %v", b.Title, err)
	}
	req.Format = ""
	if b, err = r.a.editAskText(req, ed); err != nil || !strings.Contains(b.Title, pr.Formats[0].Name) {
		t.Fatalf("print без формата: %q, %v", b.Title, err)
	}
	for _, bad := range []agents.AskReq{{Item: "нет-такой"}, {Item: "fabrika", Format: "ВСТАВЬ"}} {
		bad.Kind, bad.Tool = agents.AskEdit, toolPrint
		if _, err = r.a.editAskText(bad, ed); !errors.Is(err, agents.ErrNoProfile) {
			t.Fatalf("%+v: %v", bad, err)
		}
	}
	if _, err = r.a.editAskText(agents.AskReq{Kind: agents.AskEdit, Tool: "сам_придумал"}, ed); err == nil {
		t.Fatal("шаблона нет — ждали ошибку до окна")
	}
}

// agentsDecorFirst — первая плитка каталога фонов.
func (a *App) agentsDecorFirst(t *testing.T) agents.DecorItemLite {
	t.Helper()
	cat, err := a.agentsDecor("bg")
	if err != nil || len(cat.Items) == 0 {
		t.Fatalf("каталог фонов: %d, %v", len(cat.Items), err)
	}
	return cat.Items[0]
}

// TestEditAskNoPath: в окне вопроса о правке нет путей файлов и каталогов, ни в тексте, ни в событии.
func TestEditAskNoPath(t *testing.T) {
	r := newEdRig(t)
	r.a.SetAgentPermGroup("edit", prefs.PermAsk)
	ed := r.a.editor()
	spread := ed.View().Spreads[0].ID
	pr, _ := profileFormat(t, "fabrika", "30×20")
	reqs := []agents.AskReq{
		{Tool: toolAutobuild, Count: 4}, {Tool: toolVary, Spreads: []string{spread}, Families: []string{"dense"}},
		{Tool: toolShow, Spreads: []string{spread}}, {Tool: toolUndo}, {Tool: toolMarks, Count: 2}, {Tool: toolCover, Count: 1},
		{Tool: toolBg, Item: r.a.agentsDecorFirst(t).ID, Spreads: []string{spread}},
		{Tool: toolSticker, Count: 1, Spreads: []string{spread}}, {Tool: toolPrint, Item: pr.ID, Format: pr.Formats[0].Name},
	}
	var bad []string
	for _, p := range []string{r.cfg, r.album, r.photos[0], ed.Path()} {
		if p != "" {
			bad = append(bad, p)
		}
	}
	for _, req := range reqs {
		req.Kind, req.Client, req.Key = agents.AskEdit, "Claude Code", agents.AlbumKey(ed.ID())
		b, err := r.a.editAskText(req, ed)
		if err != nil {
			t.Fatalf("%s: %v", req.Tool, err)
		}
		all := b.Title + "\n" + strings.Join(b.Lines, "\n") + "\n" + b.OK
		for _, p := range bad {
			if strings.Contains(all, p) {
				t.Errorf("%s: в тексте путь %q", req.Tool, p)
			}
		}
		if strings.ContainsAny(all, "\\") || strings.Contains(all, ".salbum") || strings.Contains(all, ".jpg") || strings.Contains(all, "/home") {
			t.Errorf("%s: похоже на путь: %q", req.Tool, all)
		}
	}
}

// editCall — идущий вызов инструмента через службу.
type editCall struct {
	res  *mcp.CallToolResult
	err  error
	done chan struct{}
}

// async вызывает инструмент в своей горутине (вопрос держит вызов до ответа); album берётся до вызова.
func (r *edRig) async(t *testing.T, name string, kv map[string]any) *editCall {
	t.Helper()
	args := map[string]any{"album": albumRef(t, r.call(t, "get_album", nil))}
	for k, v := range kv {
		args[k] = v
	}
	c := &editCall{done: make(chan struct{})}
	go func() {
		defer close(c.done)
		ctx, cancel := context.WithTimeout(context.Background(), testkit.Limit(30*time.Second))
		defer cancel()
		c.res, c.err = r.cs.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
	}()
	return c
}

// wait ждёт конца вызова; ошибка протокола — провал.
func (c *editCall) wait(t *testing.T) *mcp.CallToolResult {
	t.Helper()
	select {
	case <-c.done:
	case <-time.After(testkit.Limit(20 * time.Second)):
		t.Fatal("вызов не вернулся")
	}
	if c.err != nil {
		t.Fatalf("протокол: %v", c.err)
	}
	return c.res
}

// asked ждёт n-го вопроса agents:ask.
func (r *edRig) asked(t *testing.T, n int) AgentAsk {
	t.Helper()
	agWait(t, "agents:ask", func() bool { return len(r.of("agents:ask")) >= n })
	return r.of("agents:ask")[n-1].(AgentAsk)
}

// askdone ждёт n-го agents:askdone.
func (r *edRig) askdone(t *testing.T, n int) AgentAskDone {
	t.Helper()
	agWait(t, "agents:askdone", func() bool { return len(r.of("agents:askdone")) >= n })
	return r.of("agents:askdone")[n-1].(AgentAskDone)
}

// TestEditAskFlow: «Спрашивать» у правки на настоящем App и службе, разрешения по умолчанию (apply_print) —
// «да» делает правку одним шагом, «нет» не трогает альбом, смена альбома за время вопроса — отказ.
func TestEditAskFlow(t *testing.T) {
	r := newEdRig(t)
	if mode, _ := r.a.agentPerm(toolPrint); mode != prefs.PermAsk {
		t.Fatalf("apply_print по умолчанию %q, ждали «Спрашивать»", mode)
	}
	pr, f := profileFormat(t, "fabrika", "30×20")
	args := map[string]any{"profile": "fabrika", "format": "30×20"}
	json0 := projectJSON(t, r.a)
	rev0 := r.a.editor().View().Rev

	// «Нет»: ни правки, ни шага.
	c := r.async(t, "apply_print", args)
	ask := r.asked(t, 1)
	if ask.Kind != "edit" || ask.Tool != toolPrint || ask.Title != "Поставить формат «"+pr.Name+" "+f.Name+"»?" || ask.Path != "" || ask.Alt != "" || ask.Why != "" {
		t.Fatalf("вопрос: %+v", ask)
	}
	if _, err := r.a.AnswerAgent(ask.ID, false); err != nil {
		t.Fatal(err)
	}
	if res := c.wait(t); !res.IsError || projectJSON(t, r.a) != json0 || r.a.editor().View().Rev != rev0 {
		t.Fatalf("«нет»: ошибка %v, проект изменился: %s", res.IsError, agText(res))
	}
	if d := r.askdone(t, 1); d.ID != ask.ID || d.Why != agents.WhyDeclined {
		t.Fatalf("askdone: %+v", d)
	}

	// «Да»: правка одним шагом, тост, Ctrl+Z возвращает. После отказа служба молчит — «Разрешить вопросы».
	r.a.AllowAgentAsks()
	c = r.async(t, "apply_print", args)
	ask = r.asked(t, 2)
	if _, err := r.a.AnswerAgent(ask.ID, true); err != nil {
		t.Fatal(err)
	}
	res := c.wait(t)
	if res.IsError || structured(t, res)["changed"] != true || projectJSON(t, r.a) == json0 {
		t.Fatalf("«да»: %s", agText(res))
	}
	if d := r.askdone(t, 2); d.Why != agents.WhyAnswered {
		t.Fatalf("askdone: %+v", d)
	}
	if e := r.lastEdited(t); e.Text != "Формат: "+pr.Name+" 30×20" {
		t.Fatalf("тост: %+v", e)
	}
	r.a.Undo()
	if projectJSON(t, r.a) != json0 {
		t.Fatal("правка по «да» — больше одного шага отмены")
	}

	// Ответ дважды и по чужому id — ошибка, ничего не делается.
	if _, err := r.a.AnswerAgent(ask.ID, true); !errors.Is(err, errAskClosed) {
		t.Fatalf("повтор: %v", err)
	}
	if _, err := r.a.AnswerAgent("чужой", true); !errors.Is(err, errAskClosed) {
		t.Fatalf("чужой id: %v", err)
	}

	// Человек правил, пока думал: «да» не применяет правку к изменившемуся альбому.
	c = r.async(t, "apply_print", args)
	ask = r.asked(t, 3)
	st := r.a.editor().View().Settings
	st.PageW++
	r.a.SetSettings(st)
	human := projectJSON(t, r.a)
	if _, err := r.a.AnswerAgent(ask.ID, true); err != nil {
		t.Fatal(err)
	}
	if res = c.wait(t); !res.IsError || projectJSON(t, r.a) != human {
		t.Fatalf("альбом изменился за время вопроса: %s", agText(res))
	}

	// Инструмент запретили, пока вопрос висел: «да» не действует.
	r.a.Undo()
	r.a.SetAgentPerm(toolAutobuild, prefs.PermAsk)
	before := len(r.a.editor().View().Spreads)
	c = r.async(t, "autobuild", nil)
	ask = r.asked(t, 4)
	if ask.Tool != toolAutobuild || !strings.HasPrefix(ask.Title, "Собрать развороты из ") && strings.Contains(ask.Title, " свободных фото") {
		t.Fatalf("autobuild: %+v", ask)
	}
	r.a.SetAgentPerm(toolAutobuild, prefs.PermDeny)
	if res = c.wait(t); !res.IsError || len(r.a.editor().View().Spreads) != before {
		t.Fatalf("запрет во время вопроса: %s", agText(res))
	}
	if _, err := r.a.AnswerAgent(ask.ID, true); !errors.Is(err, errAskClosed) {
		t.Fatalf("после запрета: %v", err)
	}

	// Смена альбома за время вопроса: «да» → ключ не тот, правки нет.
	r.a.SetAgentPerm(toolPrint, prefs.PermAsk)
	c = r.async(t, "apply_print", args)
	ask = r.asked(t, 5)
	r.a.NewProject("")
	json1 := projectJSON(t, r.a)
	if _, err := r.a.AnswerAgent(ask.ID, true); err != nil {
		t.Fatal(err)
	}
	if res = c.wait(t); !res.IsError || projectJSON(t, r.a) != json1 {
		t.Fatalf("другой альбом: %s", agText(res))
	}
}

// TestEditAskFlowMachine: вопрос о правке на машине фасада без службы — итоги «да», закрытие программы, gone.
func TestEditAskFlowMachine(t *testing.T) {
	r := newAskRig(t)
	r.a.SetAgentPermGroup("edit", prefs.PermAsk)
	req := func(tool string) agents.AskReq {
		return agents.AskReq{Kind: agents.AskEdit, Client: "Claude Code", Tool: tool, Key: r.key(), Rev: r.rev(), Count: 5}
	}
	// «Да» — итог WhyAnswered, окна ОС нет, окно программы вперёд.
	c := r.ask(req(toolMarks))
	ask := r.asked(t, 1)
	if ask.Kind != "edit" || ask.Tool != toolMarks || ask.Title != "Изменить отметки у 5 фото?" || ask.OK != "Изменить отметки" || r.fronted() != 1 {
		t.Fatalf("вопрос: %+v", ask)
	}
	if _, err := r.a.AnswerAgent(ask.ID, true); err != nil {
		t.Fatal(err)
	}
	c.wait(t)
	if c.err != nil || c.res.Why != agents.WhyAnswered || len(r.titles) != 0 {
		t.Fatalf("итог: %+v %v, окон ОС %d", c.res, c.err, len(r.titles))
	}
	// Закрытие программы: closed, окно закрыто.
	c = r.ask(req(toolMarks))
	r.asked(t, 2)
	r.a.closeAgentAsk()
	c.wait(t)
	if c.err != nil || c.res.Why != agents.WhyClosed || r.done(t, 2).Why != agents.WhyClosed {
		t.Fatalf("закрытие: %+v %v", c.res, c.err)
	}
	// Агент ушёл: gone.
	c = r.ask(req(toolShow))
	r.asked(t, 3)
	c.cancel()
	c.wait(t)
	if c.res.Why != agents.WhyGone {
		t.Fatalf("gone: %+v", c.res)
	}
}

// TestEditAskFlowTimeout: срок вопроса о правке — своя машина с коротким сроком (сроки задаёт конструктор,
// пересоздавать машину посреди вопросов нельзя).
func TestEditAskFlowTimeout(t *testing.T) {
	r := newAskRig(t)
	newAskQ(r.a, 20*time.Millisecond, 300*time.Millisecond)
	r.a.SetAgentPermGroup("edit", prefs.PermAsk)
	c := r.ask(agents.AskReq{Kind: agents.AskEdit, Client: "Claude Code", Tool: toolShow, Key: r.key(), Rev: r.rev(), Count: 5})
	c.wait(t)
	if c.res.Why != agents.WhyTimeout {
		t.Fatalf("срок: %+v", c.res)
	}
}

// TestEditAskFlowDeny: вопрос о правке — запрет за время вопроса и до него, чужой альбом, нет шаблона, отказ.
func TestEditAskFlowDeny(t *testing.T) {
	r := newAskRig(t)
	r.a.SetAgentPermGroup("edit", prefs.PermAsk)
	req := func(tool string) agents.AskReq {
		return agents.AskReq{Kind: agents.AskEdit, Client: "Claude Code", Tool: tool, Key: r.key(), Rev: r.rev(), Count: 5}
	}
	// Инструмент запретили, пока вопрос висел: «да» → off.
	c := r.ask(req(toolMarks))
	ask := r.asked(t, 1)
	r.a.SetAgentPerm(toolMarks, prefs.PermDeny)
	if _, err := r.a.AnswerAgent(ask.ID, true); err != nil {
		t.Fatal(err)
	}
	c.wait(t)
	if c.res.Why != agents.WhyOff {
		t.Fatalf("запрет: %+v", c.res)
	}
	// Запрещён до вопроса: окна нет.
	n := len(r.evOf("agents:ask"))
	c = r.ask(req(toolMarks))
	c.wait(t)
	if c.res.Why != agents.WhyOff || len(r.evOf("agents:ask")) != n {
		t.Fatalf("запрещён заранее: %+v", c.res)
	}
	// Чужой альбом и отсутствие шаблона — ошибка до окна.
	bad := req(toolShow)
	bad.Key = "k999999"
	c = r.ask(bad)
	c.wait(t)
	if !errors.Is(c.err, agents.ErrAlbumChanged) {
		t.Fatalf("чужой ключ: %+v %v", c.res, c.err)
	}
	c = r.ask(req("сам_придумал"))
	c.wait(t)
	if c.err == nil || len(r.evOf("agents:ask")) != n {
		t.Fatalf("нет шаблона: %+v %v", c.res, c.err)
	}
	// «Нет» принимается и без вида: отказ любого вида — declined.
	r.a.SetAgentPerm(toolShow, prefs.PermAsk)
	c = r.ask(req(toolShow))
	ask = r.asked(t, n+1)
	if _, err := r.a.AnswerAgent(ask.ID, false); err != nil {
		t.Fatal(err)
	}
	c.wait(t)
	if c.res.Why != agents.WhyDeclined {
		t.Fatalf("отказ: %+v", c.res)
	}
}
