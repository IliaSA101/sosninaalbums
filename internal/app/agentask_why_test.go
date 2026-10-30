package app

// Вопрос о папке с путём и причиной (150, design §5.1): Why при «Всегда» и «Спрашивать», путь
// и «Выбрать другую…», негодная подсказка, подмена ссылки; путь не уходит агенту; черновик агента
// после правок 151 остаётся нетронутым; тихие вызовы не сбрасывают ступени отказов.

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/IliaSA101/sosninaalbums/internal/app/agents"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Причина «не тихо» в вопросе: при «Всегда» — place, remote, big, rate, draft; «Спрашивать» — пусто,
// кроме bad (негодная подсказка — в любом режиме).
func TestFolderAskWhy(t *testing.T) {
	r := newQuietRig(t)
	outside := realPath(t, filepath.Join(testkit.TempDir(t), "Чужая"))
	wide := r.mkdir(t, "много_папок")
	for i := range 501 {
		if err := os.Mkdir(filepath.Join(wide, fmt.Sprint(i)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	missing := filepath.Join(r.place, "Нет такой")
	type row struct {
		name   string
		req    func(always bool) agents.AskReq
		remote bool
		want   string // при «Всегда»
		ask    string // при «Спрашивать»
	}
	add := func(hint string) func(bool) agents.AskReq {
		return func(always bool) agents.AskReq {
			q := r.addReq()
			q.Hint, q.Always = hint, always
			return q
		}
	}
	rows := []row{
		{"place", add(outside), false, whyPlace, ""},
		{"remote", add(r.shoot), true, whyRemote, ""},
		{"big", add(wide), false, whyBig, ""},
		{"rate", func(bool) agents.AskReq { q := r.addReq(); q.Hint = r.shoot; return q }, false, whyRate, ""}, // слот не выдан
		{"draft", func(always bool) agents.AskReq {
			q := r.createReq()
			q.Hint, q.Always = r.shoot, always
			return q
		}, false, whyDraft, ""},
		{"bad", add(missing), false, whyBad, whyBad},
	}
	n := 0
	for _, mode := range []string{prefs.PermAllow, prefs.PermAsk} {
		r.setMode(toolAddFolder, mode)
		r.setMode(toolCreateAlbum, mode)
		for _, w := range rows {
			quietRemote = func(string) bool { return w.remote }
			want := w.want
			if mode == prefs.PermAsk {
				want = w.ask
			}
			n++
			ev := r.refuse(t, r.ask(w.req(mode == prefs.PermAllow)), n)
			if ev.Why != want {
				t.Errorf("%s при %q: Why %q, ждали %q", w.name, mode, ev.Why, want)
			}
		}
	}
	if len(r.folders()) != 1 {
		t.Error("отказанная папка попала в альбом")
	}
}

// Вопрос с путём: настоящий путь, подпись, «Выбрать другую…»; ответ «да» добавляет показанную папку
// без окна ОС; путь не в строках вопроса.
func TestFolderAskPath(t *testing.T) {
	r := newQuietRig(t)
	r.setMode(toolAddFolder, prefs.PermAsk)
	q := r.addReq()
	q.Hint = r.shoot
	c := r.ask(q)
	ev := r.asked(t, 1)
	if ev.Path != r.shoot || ev.Hint != "Петровы" || ev.Alt == "" || ev.Link || ev.Tool != toolAddFolder || ev.Why != "" {
		t.Fatalf("вопрос: %+v", ev)
	}
	if s := strings.Join(append(ev.Lines, ev.Title, ev.OK, ev.Hint), "\n"); strings.Contains(s, r.shoot) {
		t.Errorf("полный путь в текстах вопроса: %s", s)
	}
	if _, err := r.a.AnswerAgent(ev.ID, true); err != nil {
		t.Fatal(err)
	}
	c.wait(t)
	if c.res.Why != agents.WhyAnswered || !contains(r.folders(), r.shoot) {
		t.Fatalf("итог: %+v, %v, папки %v", c.res, c.err, r.folders())
	}
	r.mu.Lock()
	dialogs := len(r.titles)
	r.mu.Unlock()
	if dialogs != 0 {
		t.Errorf("окно ОС открыто при показанном пути: %d", dialogs)
	}
}

// «Выбрать другую…»: окно ОС открывается в родителе показанного пути, добавляется выбранная папка;
// места не пополняются.
func TestFolderAskOther(t *testing.T) {
	r := newQuietRig(t)
	r.setMode(toolAddFolder, prefs.PermAsk)
	places := placeDirs(t, r.a)
	q := r.addReq()
	q.Hint = r.shoot
	c := r.ask(q)
	ev := r.asked(t, 1)
	if _, err := r.a.AnswerAgentOther(ev.ID); err != nil {
		t.Fatal(err)
	}
	c.wait(t)
	r.mu.Lock()
	defs := append([]string(nil), r.defs...)
	r.mu.Unlock()
	if len(defs) != 1 || defs[0] != filepath.Dir(r.shoot) {
		t.Errorf("окно ОС открыто в %v, ждали родителя показанного пути", defs)
	}
	if c.res.Why != agents.WhyAnswered || !contains(r.folders(), r.other) || contains(r.folders(), r.shoot) {
		t.Errorf("добавлена не выбранная папка: %+v, %v", c.res, r.folders())
	}
	if got := placeDirs(t, r.a); fmt.Sprint(got) != fmt.Sprint(places) {
		t.Errorf("«Выбрать другую…» изменила места: %v → %v", places, got)
	}
}

// Негодная подсказка: вопрос 149 без пути и кнопки «Выбрать другую…»; окно ОС открывается в
// «Фото» по умолчанию, ответ добавляет выбранную папку.
func TestFolderAskBadHint(t *testing.T) {
	r := newQuietRig(t)
	for _, hint := range []string{"относительный/путь", filepath.Join(r.place, "Нет такой"), "\x00", strings.Repeat("я", 5000)} {
		r.setMode(toolAddFolder, prefs.PermAsk)
		q := r.addReq()
		q.Hint = hint
		n := len(r.evOf("agents:ask")) + 1 // номер ждём до ask: вопрос мог прийти раньше подсчёта
		c := r.ask(q)
		ev := r.asked(t, n)
		if ev.Path != "" || ev.Hint != "" || ev.Alt != "" || ev.Why != whyBad || ev.Link {
			t.Errorf("подсказка %.20q: %+v", hint, ev)
		}
		if _, err := r.a.AnswerAgent(ev.ID, false); err != nil {
			t.Fatal(err)
		}
		c.wait(t)
	}
	q := r.addReq()
	q.Hint = "относительный/путь"
	n := len(r.evOf("agents:ask")) + 1 // см. выше: счёт до ask
	c := r.ask(q)
	ev := r.asked(t, n)
	if _, err := r.a.AnswerAgent(ev.ID, true); err != nil {
		t.Fatal(err)
	}
	c.wait(t)
	if c.res.Why != agents.WhyAnswered || !contains(r.folders(), r.other) {
		t.Errorf("выбор в окне ОС: %+v, %v", c.res, r.folders())
	}
}

// Подсказка через ссылку: вопрос показывает настоящий путь и «по ссылке»; если показанный путь сам
// стал ссылкой до ответа — папка не добавляется (too_wide).
func TestFolderAskRelinked(t *testing.T) {
	r := newQuietRig(t)
	r.setMode(toolAddFolder, prefs.PermAsk)
	link := filepath.Join(r.place, "Алиас")
	if err := os.Symlink(r.shoot, link); err != nil {
		t.Skip("ссылки недоступны:", err)
	}
	q := r.addReq()
	q.Hint = link
	c := r.ask(q)
	ev := r.asked(t, 1)
	// Имя в заголовке — настоящее, не из подсказки-ссылки «Алиас» (аудит М2).
	if ev.Path != r.shoot || !ev.Link || ev.Hint != "Петровы" {
		t.Fatalf("вопрос по ссылке: %+v", ev)
	}
	outside := realPath(t, filepath.Join(testkit.TempDir(t), "Наружу"))
	if err := os.Rename(r.shoot, r.shoot+".old"); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, r.shoot); err != nil {
		t.Fatal(err)
	}
	if _, err := r.a.AnswerAgent(ev.ID, true); err != nil {
		t.Fatal(err)
	}
	c.wait(t)
	if c.res.Why != agents.WhyTooWide || len(r.folders()) != 1 {
		t.Errorf("подмена показанного пути: %+v, %v, папки %v", c.res, c.err, r.folders())
	}
}

// Путь тихой папки и отказа: агенту ни в ответе, ни в ошибке, ни в журнале и log.txt; человеку —
// в поле Path события, не в Text.
func TestAgentFolderNoEchoQuiet(t *testing.T) {
	logs := captureLog(t)
	r := newQuietRig(t)
	secret := "СекретнаяСъёмкаИванов"
	dir := r.mkdir(t, filepath.Join(secret, "Кадры"), "a.jpg") // имя родителя — секрет; последний элемент в вопросе допустим
	outside := realPath(t, filepath.Join(testkit.TempDir(t), secret+"Чужая", "Кадры"))
	leaks := func(what string, v any) {
		t.Helper()
		b, _ := json.Marshal(v)
		if s := fmt.Sprintf("%+v %s", v, b); strings.Contains(s, secret) || strings.Contains(s, r.place) {
			t.Errorf("%s: путь в %s", what, s)
		}
	}
	c := r.run(t, r.addQuiet(dir))
	if c.res.Why != agents.WhyQuiet {
		t.Fatalf("итог: %+v, %v", c.res, c.err)
	}
	leaks("итог агенту", c.res)
	ev := edited(r.askRig)
	if len(ev) != 1 || ev[0].Path != dir || strings.Contains(ev[0].Text, secret) {
		t.Errorf("agents:edited: путь — в Path, не в Text: %+v", ev)
	}
	c = r.ask(r.addQuiet(outside)) // вопрос: путь только окну (Path)
	ask := r.refuse(t, c, 1)
	if ask.Path != outside || strings.Contains(strings.Join(ask.Lines, " ")+ask.Title+ask.OK, secret) {
		t.Errorf("agents:ask: %+v", ask)
	}
	leaks("итог отказа", c.res)
	if c.err != nil {
		leaks("ошибка агенту", c.err.Error())
	}
	leaks("agents:askdone", r.done(t, 1))
	leaks("журнал", r.a.AgentsInfo().Calls)
	if s := logs.String(); strings.Contains(s, secret) || strings.Contains(s, r.place) {
		t.Errorf("путь в log.txt:\n%s", s)
	}
}

// Черновик агента после его же правки 151 остаётся нетронутым: второй create_album при «Всегда» тихий.
func TestQuietAgentDraftAfter151(t *testing.T) {
	r := newQuietRig(t)
	first := r.a.editor()
	r.a.draftSet(first, first.View().Rev) // как после create_album
	if c := r.run(t, r.createQuiet(r.shoot)); c.res.Why != agents.WhyQuiet {
		t.Fatalf("первый create_album: %+v, %v", c.res, c.err)
	}
	waitRead(t, r.a)
	ids := []string{library.IDFor(filepath.Join(r.shoot, "Кадр_0.jpg")), library.IDFor(filepath.Join(r.shoot, "Кадр_1.jpg"))}
	res, err := r.a.agentsMark(t.Context(), agents.MarkReq{Key: r.key(), Rev: r.rev(), IDs: ids, Flag: "pick", Client: "Claude Code"})
	if err != nil || res.Changed != 2 {
		t.Fatalf("set_marks: %+v, %v", res, err)
	}
	cur := r.a.editor()
	c := r.ask(r.createQuiet(r.shoot))
	waitUntil(t, "итог или вопрос", func() bool { // вопрос «draft» — провал, ждать его таймаута незачем
		select {
		case <-c.done:
			return true
		default:
			return len(r.evOf("agents:ask")) > 0
		}
	})
	if n := len(r.evOf("agents:ask")); n != 0 || c.res.Why != agents.WhyQuiet || r.a.editor() == cur {
		t.Fatalf("второй create_album после set_marks: вопросов %d, %+v, %v", n, c.res, c.err)
	}
	// Правка человека, а не агента, черновик по-прежнему «трогает».
	cur = r.a.editor()
	if _, _, err := cur.AddFolder(r.mkdir(t, "Иванов", "x.jpg")); err != nil {
		t.Fatal(err)
	}
	if ev := r.refuse(t, r.ask(r.createQuiet(r.shoot)), 1); ev.Why != whyDraft {
		t.Errorf("после правки человека: %+v", ev)
	}
}

// Тихие вызовы между отказами не сбрасывают и не растят ступени: после второго отказа подряд —
// тишина, а не пауза первой ступени (через настоящую службу и App).
func TestQuietNoStrikeResetApp(t *testing.T) {
	r := newEdRig(t)
	a := r.a
	if _, err := a.SetAgents(false); err != nil {
		t.Fatal(err)
	}
	a.agents = agents.New(a.agentsHost(), agents.Options{OnCall: a.agentsCalled, OnDenied: a.agentsDenied,
		Cooldown: 300 * time.Millisecond, Mute: time.Hour})
	r.start(t)

	oldRemote, oldShots := quietRemote, quietShots
	quietRemote, quietShots = func(string) bool { return false }, func() []string { return nil }
	t.Cleanup(func() { quietRemote, quietShots = oldRemote, oldShots })
	m := a.askM()
	m.rules = func() *folderRules { x := permissiveRules(); x.disk = diskProbe; return x }
	m.front = func() {}
	place := realPath(t, filepath.Join(testkit.TempDir(t), "Архив"))
	shoot := filepath.Join(place, "Петровы")
	outside := realPath(t, filepath.Join(testkit.TempDir(t), "Чужая"))
	if err := os.Mkdir(shoot, 0o755); err != nil {
		t.Fatal(err)
	}
	testkit.JPEG(t, shoot, "Кадр.jpg", 600, 400)
	prefs.UpdateAgentPerms(a.cfgDir, func(p *prefs.AgentPerms) {
		p.Tools[toolAddFolder] = prefs.PermAllow
		p.Places = prefs.PushPlace(p.Places, prefs.Place{Dir: place, How: prefs.PlaceChosen})
	})

	args := func(hint string) map[string]any {
		return map[string]any{"album": albumRef(t, r.call(t, "get_album", nil)), "folder_hint": hint}
	}
	// refuse — вопрос вне места; человек отказывает.
	refuse := func(n int) {
		t.Helper()
		ar := args(outside)
		done := make(chan *mcp.CallToolResult, 1)
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), testkit.Limit(30*time.Second))
			defer cancel()
			res, _ := r.cs.CallTool(ctx, &mcp.CallToolParams{Name: "add_folder", Arguments: ar})
			done <- res
		}()
		waitUntil(t, "вопрос", func() bool { return len(r.of("agents:ask")) >= n })
		if _, err := a.AnswerAgent(r.of("agents:ask")[n-1].(AgentAsk).ID, false); err != nil {
			t.Fatal(err)
		}
		if res := <-done; res == nil || !res.IsError {
			t.Fatalf("отказ %d: %v", n, res)
		}
	}
	refuse(1)
	cool := r.call(t, "add_folder", args(shoot))
	if !cool.IsError {
		t.Fatal("пауза после отказа не действует")
	}
	// Пауза кончилась: тихая папка проходит.
	waitUntil(t, "конец паузы", func() bool {
		time.Sleep(20 * time.Millisecond)
		return !r.call(t, "add_folder", args(shoot)).IsError
	})
	refuse(2)
	muted := r.call(t, "add_folder", args(shoot))
	if !muted.IsError || agText(muted) == agText(cool) {
		t.Errorf("второй отказ подряд после тихого успеха: ждали тишину, пришло %q (пауза: %q)", agText(muted), agText(cool))
	}
}
