package agents

import (
	"context"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

// TestOpenAllowKept: у открытия (класс O) «Всегда» службой читается как «Всегда», в отличие от выгрузки (Q).
func TestOpenAllowKept(t *testing.T) {
	r := newRig(t, Options{}, "")
	sp := toolSpec{name: "open_album", perm: prefs.ClassOpen}
	for _, c := range []struct{ set, want string }{
		{prefs.PermAllow, prefs.PermAllow},
		{prefs.PermAsk, prefs.PermAsk},
		{prefs.PermDeny, prefs.PermDeny},
		{"мусор", prefs.PermDeny},
	} {
		r.fx.edit.setPerm("open_album", c.set)
		if mode, _, _ := r.svc.permLevel(sp); mode != c.want {
			t.Errorf("open_album: %q -> %q, ждали %q", c.set, mode, c.want)
		}
	}
	for _, s := range r.svc.Specs() {
		if s.name == "open_album" && s.perm != prefs.ClassOpen {
			t.Errorf("класс open_album в таблице службы: %q", s.perm)
		}
	}
}

// quietAnswer — Host.Ask открытия: тишина, когда служба разрешила её слотом (Always), иначе «да» человека.
func quietAnswer(asked *[]AskReq) func(context.Context, AskReq) (AskRes, error) {
	return func(_ context.Context, q AskReq) (AskRes, error) {
		*asked = append(*asked, q)
		why := WhyAnswered
		if q.Always {
			why = WhyQuiet
		}
		return AskRes{Why: why, Album: AlbumRef{Key: "new", Rev: 3}}, nil
	}
}

// TestOpenHintNoSlot: «Всегда» у open_album — тихий слот только у вызова по ключу. Найденный по name_hint (служба ставит и
// Key, и Hint) вопрос слот не берёт и Always не ставит; тихие вызовы по ключу после стольких же вопросов по словам
// получают все три слота, четвёртый — вопрос (общий предел 3 за 10 минут).
func TestOpenHintNoSlot(t *testing.T) {
	var asked []AskReq
	r, cs := editRig(t, Options{}, func(e *editFake) { e.ask = quietAnswer(&asked) })
	r.fx.mu.Lock()
	r.fx.view.Path = "" // открыт другой альбом: ответа «уже открыт» нет
	r.fx.mu.Unlock()
	r.fx.edit.setPerm("open_album", prefs.PermAllow)
	key := RecentKey(r.fx.recent[0].Path)

	for range 5 { // найденное по словам (Hint и Key), без слотов
		res := okReply(t, tcall(t, cs, "open_album", map[string]any{"name_hint": "ZQ5"}))
		if res.Data["opened"] != true {
			t.Fatalf("по словам: %v", res.Data)
		}
		if q := asked[len(asked)-1]; q.Always || q.Hint != "ZQ5" || q.Key != key {
			t.Fatalf("по словам: Host.Ask %+v", q)
		}
	}
	for i := range 3 { // слоты целы
		okReply(t, tcall(t, cs, "open_album", map[string]any{"album": key}))
		if q := asked[len(asked)-1]; !q.Always || q.Key != key || q.Hint != "" {
			t.Fatalf("по ключу %d: Host.Ask %+v", i+1, q)
		}
	}
	okReply(t, tcall(t, cs, "open_album", map[string]any{"album": key}))
	if q := asked[len(asked)-1]; q.Always {
		t.Errorf("четвёртое тихое за 10 минут: Always = true")
	}
}

// TestOpenPickWhy: вызов без аргументов и name_hint, которого нет среди последних (итог поиска агенту не виден), — выбор
// файла без ключа и слова: Always нет, слот не тратится; при «Спрашивать» то же самое, а ключ без «Всегда» тишины не просит.
func TestOpenPickWhy(t *testing.T) {
	var asked []AskReq
	r, cs := editRig(t, Options{}, func(e *editFake) { e.ask = quietAnswer(&asked) })
	r.fx.mu.Lock()
	r.fx.view.Path = ""
	r.fx.mu.Unlock()
	key := RecentKey(r.fx.recent[0].Path)

	r.fx.edit.setPerm("open_album", prefs.PermAllow)
	for _, args := range []map[string]any{nil, {"name_hint": "Несуществующих"}} {
		okReply(t, tcall(t, cs, "open_album", args))
		if q := asked[len(asked)-1]; q.Always || q.Key != "" || q.Hint != "" || q.Kind != AskOpen {
			t.Errorf("%v: Host.Ask %+v", args, q)
		}
	}
	r.fx.edit.setPerm("open_album", prefs.PermAsk)
	okReply(t, tcall(t, cs, "open_album", map[string]any{"album": key}))
	if q := asked[len(asked)-1]; q.Always || q.Key != key {
		t.Errorf("«Спрашивать», ключ: Host.Ask %+v", q)
	}
	// Слоты за всё это не тратились: три тихих по ключу при «Всегда» проходят.
	r.fx.edit.setPerm("open_album", prefs.PermAllow)
	for i := range 3 {
		okReply(t, tcall(t, cs, "open_album", map[string]any{"album": key}))
		if q := asked[len(asked)-1]; !q.Always {
			t.Fatalf("по ключу %d: слот потрачен раньше: %+v", i+1, q)
		}
	}
}
