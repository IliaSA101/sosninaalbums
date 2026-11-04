package app

// Фасад правок агента (149, Т3): автосборка как в окне, один шаг отмены, rev и срок через фасад,
// «покажи лист», «занят». Копии — agents_edit_backup_test.go, паритет выбора фото — agents_edit_pick_test.go.

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/app/agents"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// editRig — App с лентой из n фото в одной папке и пустым альбомом; ключ альбома — «k<номер Editor>».
type editRig struct {
	a      *App
	shots  string
	photos []string // ID фото ленты по порядку создания

	mu     sync.Mutex
	events []string
	shown  []AgentShow
}

func newEditRig(t *testing.T, n int) *editRig {
	t.Helper()
	a := newApp(t)
	r := &editRig{a: a}
	a.emit = func(name string, data ...any) {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.events = append(r.events, name)
		if s, ok := data[0].(AgentShow); ok && name == "agents:show" {
			r.shown = append(r.shown, s)
		}
	}
	cache, err := library.NewCache(testkit.TempDir(t))
	if err != nil {
		t.Fatal(err)
	}
	a.cache = cache
	r.shots = filepath.Join(testkit.TempDir(t), "Съёмка")
	if err := os.Mkdir(r.shots, 0o755); err != nil {
		t.Fatal(err)
	}
	for i := range n {
		w, h := 600, 400
		if i%3 == 1 {
			w, h = 400, 600
		}
		r.photos = append(r.photos, library.IDFor(testkit.JPEG(t, r.shots, fmt.Sprintf("Кадр_%02d.jpg", i), w, h)))
	}
	if _, err := a.AddFolderPath(r.shots); err != nil {
		t.Fatal(err)
	}
	waitRead(t, a)
	a.bg.Wait() // хвост чтения (прогрев миниатюр) закончен: он сам сбросил бы scanning
	a.agentEd().opts.Key = func(ed uint64) string { return fmt.Sprintf("k%d", ed) }
	prefs.UpdateAgents(a.cfgDir, func(c *prefs.Agents) { c.Edit = true }) // правки включены: фасад проверяет флажок перед записью
	return r
}

func (r *editRig) key() string { return fmt.Sprintf("k%d", r.a.editor().ID()) }

func (r *editRig) rev() int { return r.a.editor().View().Rev }

// req — запрос автосборки с умолчаниями окна (их подставляет служба).
func (r *editRig) req() agentAutoReq {
	return agentAutoReq{Key: r.key(), Rev: r.rev(), PerAuto: true, Series: true, Variety: 1, Variant: 1}
}

func (r *editRig) eventCount(name string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return editCount(r.events, name)
}

func editCount(list []string, s string) (n int) {
	for _, x := range list {
		if x == s {
			n++
		}
	}
	return n
}

// layoutOf — раскладки разворотов: ID не сравниваются (у каждой сборки свои).
func layoutOf(v model.View) (out [][]string) {
	for _, sv := range v.Spreads {
		out = append(out, append([]string{sv.Layout}, sv.Photos...))
	}
	return out
}

// Сборка агентом = сборка окна с теми же умолчаниями (0.33-149-30): те же фото, раскладки и число
// разворотов; Album и Again — всегда false.
func TestAgentBuildEqualsWindow(t *testing.T) {
	r := newEditRig(t, 14)
	a, ed := r.a, r.a.editor()
	// Окно: N = round(14 / 4) = 4 (3,5 → вверх), «Авто», серии, «Обычное», семейства «Все», вариант 1.
	win := model.AutoOpts{Spreads: 4, MinPer: 1, MaxPer: 64, Auto: true, Series: true, Variety: model.VarietyNormal, Variant: 1}
	v, err := ed.AutoBuild(a.infos(r.photos), 0, win)
	if err != nil {
		t.Fatal(err)
	}
	want := layoutOf(v)
	if len(want) != 4 {
		t.Fatalf("окно собрало %d разворотов", len(want))
	}
	ed.Undo()
	if len(ed.View().Spreads) != 0 {
		t.Fatal("отмена окна не вернула пустой альбом")
	}
	res, err := a.agentAuto(context.Background(), r.req())
	if err != nil {
		t.Fatal(err)
	}
	if got := layoutOf(ed.View()); !slices.EqualFunc(got, want, slices.Equal) {
		t.Fatalf("агент собрал иначе:\n%v\nокно:\n%v", got, want)
	}
	if res.Added != 4 || res.Photos != 14 || res.First != 1 || res.Go != ed.View().Spreads[0].ID || res.Rev != ed.View().Rev {
		t.Fatalf("итог: %+v", res)
	}
	if res.PlanFrom == 0 || res.PlanTo < res.PlanFrom {
		t.Fatalf("«от» и «до» плана: %+v", res)
	}
	// Второй запрос: свободных фото нет — ошибка, альбом и шаги прежние.
	rev := r.rev()
	if _, err := a.agentAuto(context.Background(), r.req()); !errors.Is(err, errAgentNoFree) {
		t.Fatalf("ждали errAgentNoFree: %v", err)
	}
	if r.rev() != rev {
		t.Fatal("rev сменился без правки")
	}
	// Явное N и свой диапазон: «Свой 2–3», N = 5.
	r2 := newEditRig(t, 14)
	q := r2.req()
	q.PerAuto, q.PerMin, q.PerMax, q.Spreads = false, 2, 3, 5
	res, err = r2.a.agentAuto(context.Background(), q)
	if err != nil || res.Added != 5 || res.PlanFrom != 2 || res.PlanTo != 3 {
		t.Fatalf("свой диапазон: %+v %v", res, err)
	}
	// N вне пределов плана зажимается (14 фото по «до» 3 — не меньше 5 разворотов).
	r3 := newEditRig(t, 14)
	q = r3.req()
	q.PerAuto, q.PerMin, q.PerMax, q.Spreads = false, 2, 3, 1
	if res, err = r3.a.agentAuto(context.Background(), q); err != nil || res.Added != 5 {
		t.Fatalf("зажим N: %+v %v", res, err)
	}
}

// Каждая правка агента — ровно один шаг отмены; отмена своей правки через фасад возвращает прежнее.
func TestEditOneUndoStep(t *testing.T) {
	r := newEditRig(t, 12)
	a, ed := r.a, r.a.editor()
	ctx := context.Background()
	base := ed.View()
	res, err := a.agentAuto(ctx, r.req())
	if err != nil || res.Added == 0 {
		t.Fatalf("сборка: %+v %v", res, err)
	}
	built := layoutOf(ed.View())
	ed.Undo() // один шаг
	if v := ed.View(); len(v.Spreads) != 0 || len(v.Folders) != len(base.Folders) {
		t.Fatalf("после одной отмены: разворотов %d, папок %d (было %d)", len(v.Spreads), len(v.Folders), len(base.Folders))
	}
	ed.Redo()
	if got := layoutOf(ed.View()); !slices.EqualFunc(got, built, slices.Equal) {
		t.Fatal("возврат не восстановил сборку")
	}
	// Раскладки: один шаг на все листы; холостая правка шага не ставит.
	ids := make([]string, 0, len(ed.View().Spreads))
	for _, sv := range ed.View().Spreads {
		ids = append(ids, sv.ID)
	}
	rev := r.rev()
	vr, err := a.agentVary(ctx, r.key(), rev, ids, 2, nil)
	if err != nil {
		t.Fatal(err)
	}
	rev2, changed := vr.Album.Rev, vr.Changed
	if changed == 0 {
		t.Skip("на этом наборе фото раскладки не сменились") // не провал: счёт зависит от набора
	}
	if rev2 == rev {
		t.Fatalf("раскладки сменились (%d), а rev прежний", changed)
	}
	after := layoutOf(ed.View())
	back, err := a.agentUndo(ctx, r.key(), rev2) // своя правка — отмена одним шагом
	if err != nil || back == rev2 {
		t.Fatalf("отмена: rev %d→%d %v", rev2, back, err)
	}
	if got := layoutOf(ed.View()); !slices.EqualFunc(got, built, slices.Equal) || slices.EqualFunc(got, after, slices.Equal) {
		t.Fatal("одна отмена не вернула раскладки до правки")
	}
}

// Чужой rev, чужой ключ и срок вышел: ошибка, проект и отмена нетронуты.
func TestAgentEditStaleAndLate(t *testing.T) {
	r := newEditRig(t, 8)
	a, ed := r.a, r.a.editor()
	ctx := context.Background()
	if _, err := a.agentAuto(ctx, r.req()); err != nil {
		t.Fatal(err)
	}
	sp := ed.View().Spreads[0].ID
	rev := r.rev()
	frozen := func(what string) {
		t.Helper()
		if ed.View().Rev != rev {
			t.Fatalf("%s: rev сменился", what)
		}
	}
	// rev старше на один: человек успел поправить альбом.
	if _, err := a.agentVary(ctx, r.key(), rev-1, []string{sp}, 1, nil); !errors.Is(err, errAgentAlbumEdited) {
		t.Fatalf("vary: %v", err)
	}
	if _, err := a.agentUndo(ctx, r.key(), rev-1); !errors.Is(err, errAgentAlbumEdited) {
		t.Fatalf("undo: %v", err)
	}
	q := r.req()
	q.Rev = rev - 1
	if _, err := a.agentAuto(ctx, q); !errors.Is(err, errAgentAlbumEdited) {
		t.Fatalf("auto: %v", err)
	}
	frozen("rev")
	// Чужой ключ, пустой ключ, ключ прежнего альбома.
	old := r.key()
	for _, k := range []string{"", "k0", "kx"} {
		if _, err := a.agentVary(ctx, k, rev, []string{sp}, 1, nil); !errors.Is(err, errAgentAlbumChanged) {
			t.Fatalf("ключ %q: %v", k, err)
		}
	}
	a.agentEd().opts.Key = nil // служба не подключена — ни один ключ не годен
	if _, err := a.agentUndo(ctx, old, rev); !errors.Is(err, errAgentAlbumChanged) {
		t.Fatalf("без функции ключа: %v", err)
	}
	a.agentEd().opts.Key = func(ed uint64) string { return fmt.Sprintf("k%d", ed) }
	frozen("ключ")
	// Срок вышел: правка не применяется, хотя rev и ключ верны.
	dead, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := a.agentVary(dead, r.key(), rev, []string{sp}, 2, nil); !errors.Is(err, errAgentLate) {
		t.Fatalf("vary после срока: %v", err)
	}
	if _, err := a.agentUndo(dead, r.key(), rev); !errors.Is(err, errAgentLate) {
		t.Fatalf("undo после срока: %v", err)
	}
	frozen("срок")
	if len(ed.View().Spreads) == 0 {
		t.Fatal("отмена после срока сработала")
	}
	// Другой альбом открыт: ключ прежнего не годится, новый редактор не тронут.
	a.setEditor(blankEditor(""))
	if _, err := a.agentUndo(ctx, old, rev); !errors.Is(err, errAgentAlbumChanged) {
		t.Fatalf("после смены альбома: %v", err)
	}
}

// Пока лента читается, автосборка ждёт её конца (не больше срока), а не собирает из половины.
func TestAutobuildWaitsScan(t *testing.T) {
	r := newEditRig(t, 8)
	a := r.a
	a.mu.Lock()
	photos, byID := a.photos, a.byID
	a.photos, a.byID, a.scanning = nil, map[string]library.Photo{}, true // чтение идёт, ленты ещё нет
	a.mu.Unlock()
	done := func() {
		a.mu.Lock()
		a.photos, a.byID, a.scanning = photos, byID, false
		a.mu.Unlock()
	}
	// Не дождались: ошибка, альбом прежний.
	q := r.req()
	q.ScanWaitMs = 30
	rev := r.rev()
	if _, err := a.agentAuto(context.Background(), q); !errors.Is(err, errAgentScanning) {
		t.Fatalf("ждали errAgentScanning: %v", err)
	}
	// Срок вызова вышел раньше срока ожидания — тоже «читается».
	dead, cancel := context.WithCancel(context.Background())
	cancel()
	q.ScanWaitMs = 5000
	if _, err := a.agentAuto(dead, q); !errors.Is(err, errAgentScanning) {
		t.Fatalf("ждали errAgentScanning при отмене: %v", err)
	}
	if r.rev() != rev {
		t.Fatal("альбом тронут без ленты")
	}
	// Дождались: чтение кончилось в ходе ожидания, собраны все 8 фото.
	time.AfterFunc(80*time.Millisecond, done)
	q.ScanWaitMs = int(testkit.Limit(5 * time.Second).Milliseconds())
	res, err := a.agentAuto(context.Background(), q)
	if err != nil || res.Photos != 8 {
		t.Fatalf("после ожидания: %+v %v", res, err)
	}
}

// show_spread: ключ и лист проверяются, событие уходит, альбом и отмена не меняются; «занят» ждёт
// снятия, но не дольше срока, и гаснет через BusyTTL.
func TestShowSpreadBusy(t *testing.T) {
	r := newEditRig(t, 8)
	a, ed := r.a, r.a.editor()
	if _, err := a.agentAuto(context.Background(), r.req()); err != nil {
		t.Fatal(err)
	}
	v := ed.View()
	rev, ev := v.Rev, r.eventCount("view")
	n, err := a.agentShow(r.key(), v.Spreads[1].ID)
	if err != nil || n != 2 {
		t.Fatalf("show: %d %v", n, err)
	}
	if got := r.shown; len(got) != 1 || got[0].Spread != v.Spreads[1].ID {
		t.Fatalf("события agents:show: %v", got)
	}
	if r.rev() != rev || r.eventCount("view") != ev {
		t.Fatal("show изменил альбом или разослал вид")
	}
	if _, err := a.agentShow(r.key(), "нет-такого"); !errors.Is(err, errAgentNoSpread) {
		t.Fatalf("неизвестный лист: %v", err)
	}
	if _, err := a.agentShow(r.key(), model.CoverID); !errors.Is(err, errAgentNoSpread) { // обложки нет
		t.Fatalf("обложка без обложки: %v", err)
	}
	if _, err := a.agentShow("kx", v.Spreads[0].ID); !errors.Is(err, errAgentAlbumChanged) {
		t.Fatalf("чужой ключ: %v", err)
	}
	if len(r.shown) != 1 {
		t.Fatalf("отказ послал событие: %v", r.shown)
	}

	// «Занят»: свободен — сразу; поднят — ждёт до конца срока и отвечает true.
	if a.agentBusy(context.Background()) {
		t.Fatal("свободен, а ответ «занят»")
	}
	a.AgentsBusy(true)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Millisecond)
	defer cancel()
	t0 := time.Now()
	if !a.agentBusy(ctx) || time.Since(t0) < 50*time.Millisecond {
		t.Fatalf("занят: ответ раньше срока (%v)", time.Since(t0))
	}
	// Снят в ходе ожидания — ждать дальше не нужно.
	time.AfterFunc(40*time.Millisecond, func() { a.AgentsBusy(false) })
	a.AgentsBusy(true)
	ctx2, cancel2 := context.WithTimeout(context.Background(), testkit.Limit(5*time.Second))
	defer cancel2()
	t0 = time.Now()
	if a.agentBusy(ctx2) || time.Since(t0) > 2*time.Second {
		t.Fatalf("снят в ходе ожидания, а ответ «занят» (%v)", time.Since(t0))
	}
}

// Флаг «занят» жив BusyTTL от последнего сигнала: упавший интерфейс агента не блокирует навсегда.
func TestAgentBusyTTL(t *testing.T) {
	a := newApp(t)
	var mu sync.Mutex
	now := time.Unix(1000, 0)
	s := a.agentEd()
	s.opts.Now = func() time.Time { mu.Lock(); defer mu.Unlock(); return now }
	s.opts.BusyWait = 20 * time.Millisecond
	step := func(d time.Duration) { mu.Lock(); now = now.Add(d); mu.Unlock() }
	ctx := context.Background()
	a.AgentsBusy(true)
	if !a.agentBusy(ctx) {
		t.Fatal("только что поднят, а не занят")
	}
	step(29 * time.Second)
	if !a.agentBusy(ctx) {
		t.Fatal("29 с — флаг ещё жив")
	}
	a.AgentsBusy(true) // повтор раз в 10 с продлевает
	step(29 * time.Second)
	if !a.agentBusy(ctx) {
		t.Fatal("повтор не продлил флаг")
	}
	step(2 * time.Second)
	if a.agentBusy(ctx) {
		t.Fatal("30 с без сигнала — флаг должен погаснуть")
	}
	a.AgentsBusy(true)
	a.AgentsBusy(false)
	if a.agentBusy(ctx) {
		t.Fatal("снят явно")
	}
}

// Программа ещё запускается: правок нет; флажок и причина «от администратора».
func TestAgentEditState(t *testing.T) {
	a := NewApp() // startup не вызывался: ready не закрыт
	a.agentEd().opts.Key = func(uint64) string { return "k" }
	if _, err := a.agentShow("k", "x"); !errors.Is(err, errAgentNotReady) {
		t.Fatalf("до startup: %v", err)
	}
	b := newApp(t)
	allowed, blocked := b.agentEditState()
	if allowed || blocked != "" || b.AgentsInfo().EditBlocked != "" {
		t.Fatalf("умолчание: %v %q", allowed, blocked)
	}
	prefs.UpdateAgents(b.cfgDir, func(c *prefs.Agents) { c.Edit = true })
	if allowed, blocked = b.agentEditState(); !allowed || blocked != "" {
		t.Fatalf("флажок включён: %v %q", allowed, blocked)
	}
	prefs.UpdateAgents(b.cfgDir, func(c *prefs.Agents) { c.Edit = false })
	if allowed, _ = b.agentEditState(); allowed {
		t.Fatal("флажок выключен, а правки разрешены")
	}
}

// Процесс от администратора: правки закрыты при любом флажке, вопрос агента получает off без окна.
func TestAgentEditElevated(t *testing.T) {
	r := newAskRig(t) // флажок правок включён
	if on, blocked := r.a.agentEditState(); !on || blocked != "" {
		t.Fatalf("до повышения: %v %q", on, blocked)
	}
	r.a.askM().elevated = func() bool { return true }
	if on, blocked := r.a.agentEditState(); on || blocked != agentBlockedElevated {
		t.Fatalf("администратор: %v %q", on, blocked)
	}
	if info := r.a.AgentsInfo(); !info.Edit || info.EditBlocked != agentBlockedElevated {
		t.Fatalf("AgentsInfo: edit=%v blocked=%q", info.Edit, info.EditBlocked)
	}
	c := r.ask(r.addReq())
	c.wait(t)
	if c.res.Why != agents.WhyOff || c.err != nil || len(r.evOf("agents:ask")) != 0 || r.fronted() != 0 {
		t.Fatalf("вопрос под администратором: %+v %v", c.res, c.err)
	}
	if got := r.a.editor().View().Folders; len(got) != 1 {
		t.Errorf("папка добавлена под администратором: %v", got)
	}
}
