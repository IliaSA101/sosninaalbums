package app

// Переходник правок агента к Host службы (149, Т3b2): Host заполнен, ключ единый, ошибки фасада
// переводятся таблицей, agents:edited уходит после правки с текстом из Go, отмена его не шлёт.

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/IliaSA101/sosninaalbums/internal/app/agents"
	"github.com/IliaSA101/sosninaalbums/internal/backup"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Host настоящего App несёт все девять замыканий правок: иначе служба не зарегистрирует инструменты.
func TestAgentsHostEditFilled(t *testing.T) {
	a := newApp(t)
	h := a.agentsHost()
	fields := map[string]bool{"Perm": h.Perm != nil, "Scanning": h.Scanning != nil, "Busy": h.Busy != nil,
		"Backup": h.Backup != nil, "Ask": h.Ask != nil, "Auto": h.Auto != nil, "Vary": h.Vary != nil,
		"Show": h.Show != nil, "Undo": h.Undo != nil}
	for name, ok := range fields {
		if !ok {
			t.Errorf("Host.%s не заполнен", name)
		}
	}
	// Ключ по умолчанию — тот, что отдаёт get_album.
	if kf := a.agentEd().opts.Key; kf == nil || kf(7) != agents.AlbumKey(7) || kf(7) == "" {
		t.Error("ключ по умолчанию не agents.AlbumKey")
	}
}

// Таблица ошибок: каждая строка, в том числе через обёртку %w; неизвестная и nil — как есть.
func TestAgentHostErrTable(t *testing.T) {
	rows := []struct{ from, to error }{
		{errAgentNotReady, agents.ErrNotReady},
		{errAgentNoAlbum, agents.ErrNoAlbum},
		{errAgentAlbumChanged, agents.ErrAlbumChanged},
		{errAgentAlbumEdited, agents.ErrAlbumEdited},
		{errAgentScanning, agents.ErrScanning},
		{errAgentNoFree, agents.ErrNoFree},
		{errAgentNoSpread, agents.ErrNoSpread},
		{errAgentLate, agents.ErrLate},
		{errAgentBackup, agents.ErrBackup},
		{errAgentOff, agents.ErrNoEdits},
	}
	if len(rows) != len(agentErrTable) {
		t.Fatalf("в таблице %d строк, в тесте %d", len(agentErrTable), len(rows))
	}
	for _, r := range rows {
		if got := agentHostErr(r.from); got != r.to {
			t.Errorf("%v → %v, ждали %v", r.from, got, r.to)
		}
		if got := agentHostErr(fmt.Errorf("слой: %w", r.from)); got != r.to {
			t.Errorf("обёрнутая %v → %v, ждали %v", r.from, got, r.to)
		}
	}
	other := errors.New("что-то своё")
	if agentHostErr(other) != other || agentHostErr(nil) != nil {
		t.Error("неизвестная ошибка или nil изменились")
	}
}

// edRig — стенд службы с захватом событий вместе с данными поверх agRig.
type edRig struct {
	*agRig
	mu2 sync.Mutex
	evs []pubEvent
}

func (r *edRig) of(name string) (out []any) {
	r.mu2.Lock()
	defer r.mu2.Unlock()
	for _, e := range r.evs {
		if e.name == name {
			out = append(out, e.data)
		}
	}
	return out
}

func newEdRig(t *testing.T) *edRig {
	t.Helper()
	r := &edRig{agRig: newAgRig(t)}
	r.a.emit = func(name string, data ...any) {
		var d any
		if len(data) > 0 {
			d = data[0]
		}
		r.mu2.Lock()
		r.evs = append(r.evs, pubEvent{name, d})
		r.mu2.Unlock()
	}
	r.a.backups = backup.New(filepath.Join(testkit.TempDir(t), "backups")) // копия перед первой правкой
	setAgentsEdit(r.a, true)
	r.start(t)
	return r
}

// albumRef — {key, rev} из ответа инструмента с полем album.
func albumRef(t *testing.T, res *mcp.CallToolResult) map[string]any {
	t.Helper()
	if res.IsError {
		t.Fatalf("ошибка инструмента: %s", agText(res))
	}
	m := structured(t, res)["album"].(map[string]any)
	return map[string]any{"key": m["key"], "rev": m["rev"]}
}

// Круг через службу на настоящем App: инструменты видны, автосборка шлёт тост с go, vary без
// изменений — нет, show шлёт только agents:show, отмена — только view.
func TestAgentsEditCircleViaService(t *testing.T) {
	r := newEdRig(t)
	tools, err := r.cs.ListTools(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, tl := range tools.Tools {
		names = append(names, tl.Name)
	}
	for _, want := range []string{"autobuild", "vary_layouts", "show_spread", "undo_last_edit"} {
		if !slices.Contains(names, want) {
			t.Errorf("нет инструмента %s: %v", want, names)
		}
	}
	ed := r.a.editor()
	ref := albumRef(t, r.call(t, "get_album", nil))
	if ref["key"] != agents.AlbumKey(ed.ID()) {
		t.Fatalf("ключ get_album %v ≠ agents.AlbumKey", ref["key"])
	}

	// Автосборка: «Собрано разворотов: N», go — ID первого нового разворота, rev — после правки.
	before := len(ed.View().Spreads)
	res := r.call(t, "autobuild", map[string]any{"album": ref})
	ref2 := albumRef(t, res)
	v := ed.View()
	added := len(v.Spreads) - before
	got := r.of("agents:edited")
	if added < 1 || len(got) != 1 {
		t.Fatalf("собрано %d, событий agents:edited %d: %s", added, len(got), agText(res))
	}
	ev := got[0].(AgentEdited)
	if ev.Text != fmt.Sprintf("Собрано разворотов: %d", added) || ev.Client != "тест" || ev.Rev != v.Rev ||
		ev.Go != v.Spreads[before].ID || int(ref2["rev"].(float64)) != v.Rev {
		t.Errorf("agents:edited: %+v (rev %d, первый новый %s)", ev, v.Rev, v.Spreads[before].ID)
	}

	// vary без листов: менять нечего, rev прежний, события нет (служба пустой список отвергает
	// раньше, поэтому — через Host напрямую).
	h := r.a.agentsHost()
	out, changed, err := h.Vary(context.Background(), agents.VaryReq{Key: ref["key"].(string), Rev: v.Rev, Variety: 1})
	if err != nil || changed != 0 || out.Rev != v.Rev || len(r.of("agents:edited")) != 1 {
		t.Fatalf("vary без листов: %+v %d %v, событий %d", out, changed, err, len(r.of("agents:edited")))
	}

	// vary по настоящим листам: тост «Раскладки изменены: N» только если что-то сменилось.
	res = r.call(t, "vary_layouts", map[string]any{"album": ref2, "spreads": []any{1, 2, 3}, "variety": 2})
	n := int(structured(t, res)["changed"].(float64))
	evs := r.of("agents:edited")
	switch {
	case n == 0 && len(evs) != 1:
		t.Errorf("changed 0, а событий %d", len(evs))
	case n > 0 && (len(evs) != 2 || evs[1].(AgentEdited).Text != fmt.Sprintf("Раскладки изменены: %d", n) || evs[1].(AgentEdited).Go != ""):
		t.Errorf("changed %d, события %+v", n, evs)
	}

	// show_spread: только agents:show.
	nEdited := len(r.of("agents:edited"))
	r.call(t, "show_spread", map[string]any{"album": map[string]any{"key": ref["key"]}, "spread": 2})
	if s := r.of("agents:show"); len(s) != 1 || s[0].(AgentShow).Spread != ed.View().Spreads[1].ID {
		t.Errorf("agents:show: %+v", s)
	}
	if len(r.of("agents:edited")) != nEdited {
		t.Error("show_spread прислал agents:edited")
	}

	// Отмена своей правки: view есть, agents:edited нет.
	views := len(r.of("view"))
	res = r.call(t, "undo_last_edit", map[string]any{"album": albumRef(t, r.call(t, "get_album", nil))})
	if res.IsError {
		t.Fatalf("undo_last_edit: %s", agText(res))
	}
	if len(r.of("agents:edited")) != nEdited {
		t.Error("отмена прислала agents:edited")
	}
	if len(r.of("view")) <= views {
		t.Error("отмена не прислала view")
	}
}

// Ошибки через Host: чужой ключ и устаревшая ревизия — ошибки службы, альбом не меняется.
func TestAgentsHostEditErrors(t *testing.T) {
	r := newEditRig(t, 6)
	h := r.a.agentsHost()
	ctx := context.Background()
	if _, err := h.Auto(ctx, agents.AutoReq{Key: "чужой", Rev: r.rev(), PerAuto: true}); !errors.Is(err, agents.ErrAlbumChanged) {
		t.Errorf("Auto чужой ключ: %v", err)
	}
	if _, err := h.Auto(ctx, agents.AutoReq{Key: r.key(), Rev: r.rev() + 5, PerAuto: true}); !errors.Is(err, agents.ErrAlbumEdited) {
		t.Errorf("Auto чужая ревизия: %v", err)
	}
	if _, _, err := h.Vary(ctx, agents.VaryReq{Key: "чужой", Rev: r.rev(), Variety: 1}); !errors.Is(err, agents.ErrAlbumChanged) {
		t.Errorf("Vary чужой ключ: %v", err)
	}
	if _, err := h.Show(r.key(), "нет-такого"); !errors.Is(err, agents.ErrNoSpread) {
		t.Errorf("Show нет листа: %v", err)
	}
	if _, err := h.Undo(ctx, agents.AlbumRef{Key: "чужой", Rev: r.rev()}); !errors.Is(err, agents.ErrAlbumChanged) {
		t.Errorf("Undo чужой ключ: %v", err)
	}
	if err := h.Backup("чужой"); !errors.Is(err, agents.ErrAlbumChanged) {
		t.Errorf("Backup чужой ключ: %v", err)
	}
	if n := len(r.a.editor().View().Spreads); n != 0 {
		t.Errorf("ошибки изменили альбом: %d разворотов", n)
	}
}

// Тост про папку: add + answered → «Добавлена папка «имя»» без пути и без go; слияние и отказ
// тоста не дают. Неизвестный клиент называется «Агент».
func TestAgentsAskFolderToast(t *testing.T) {
	r := newAskRig(t)
	h := r.a.agentsHost()
	ch := make(chan agents.AskRes, 1)
	ask := func(req agents.AskReq) { go func() { res, _ := h.Ask(context.Background(), req); ch <- res }() }

	req := r.addReq()
	req.Client = ""
	ask(req)
	if _, err := r.a.AnswerAgent(r.asked(t, 1).ID, true); err != nil {
		t.Fatal(err)
	}
	res := <-ch
	if res.Why != agents.WhyAnswered {
		t.Fatalf("итог: %+v", res)
	}
	got := r.evOf("agents:edited")
	if len(got) != 1 {
		t.Fatalf("agents:edited: %+v", got)
	}
	e := got[0].(AgentEdited)
	want := "Добавлена папка «" + filepath.Base(r.other) + "»"
	if e.Text != want || e.Client != "Агент" || e.Go != "" || e.Rev != res.Album.Rev || e.Rev != r.rev() {
		t.Errorf("agents:edited: %+v, ждали текст %q", e, want)
	}
	if strings.Contains(fmt.Sprintf("%+v", e), filepath.Dir(r.other)) {
		t.Errorf("путь в событии: %+v", e)
	}

	// Слияние: та же папка уже в альбоме — шага отмены нет, тоста нет.
	ask(r.addReq())
	if _, err := r.a.AnswerAgent(r.asked(t, 2).ID, true); err != nil {
		t.Fatal(err)
	}
	if res = <-ch; !res.Merged || len(r.evOf("agents:edited")) != 1 {
		t.Errorf("слияние: %+v, событий %d", res, len(r.evOf("agents:edited")))
	}

	// Отказ человека: тоста нет.
	ask(r.addReq())
	if _, err := r.a.AnswerAgent(r.asked(t, 3).ID, false); err != nil {
		t.Fatal(err)
	}
	if res = <-ch; res.Why != agents.WhyDeclined || len(r.evOf("agents:edited")) != 1 {
		t.Errorf("отказ: %+v, событий %d", res, len(r.evOf("agents:edited")))
	}
}
