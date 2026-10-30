package app

// Применение выбора человека в вопросе агента (149, design §3): папка, создание альбома,
// отмена одним шагом, путь не утекает.

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/app/agents"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// strictRules — правила с запретами Linux/macOS (как в таблице agentpath_test.go) без диска.
func strictRules() *folderRules { return unixRules("linux", diskOK) }

// Папка не прошла правило: too_wide или ours, тост человеку, правки нет; текст агенту без пути.
func TestAskFolderRejected(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("правила Unix; таблица Windows — TestAgentFolderRules")
	}
	r := newAskRig(t)
	r.a.askM().rules = strictRules
	rev := r.rev()
	for i, c := range []struct{ dir, why string }{
		{"/etc/Секрет", agents.WhyTooWide},
		{"/home/u/.config/Секрет", agents.WhyOurs},
	} {
		r.mu.Lock()
		r.picked = c.dir
		r.mu.Unlock()
		call := r.ask(r.addReq())
		if _, err := r.a.AnswerAgent(r.asked(t, i+1).ID, true); err != nil {
			t.Fatal(err)
		}
		call.wait(t)
		if call.res.Why != c.why || call.err != nil {
			t.Fatalf("%s: %+v, %v", c.dir, call.res, call.err)
		}
		if d := r.done(t, i+1); d.Why != c.why {
			t.Errorf("agents:askdone: %+v", d)
		}
	}
	if n := len(r.evOf("notice")); n != 2 {
		t.Errorf("тостов человеку: %d", n)
	}
	if r.rev() != rev || len(r.a.editor().View().Folders) != 1 {
		t.Error("отклонённая папка попала в альбом")
	}
}

// Путь выбранной папки не доходит ни до агента, ни до журнала, ни до log.txt, ни в события вопроса.
func TestAgentFolderNoEcho(t *testing.T) {
	logs := captureLog(t)
	r := newAskRig(t)
	secret := "СекретнаяСъёмкаИванов"
	parent := filepath.Dir(r.other)
	dir := filepath.Join(parent, secret)
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	r.picked = dir

	leaks := func(what string, v any) {
		t.Helper()
		b, _ := json.Marshal(v)
		s := fmt.Sprintf("%+v %s", v, b)
		if strings.Contains(s, secret) || strings.Contains(s, parent) {
			t.Errorf("%s: путь в %s", what, s)
		}
	}
	c := r.ask(r.addReq())
	ev := r.asked(t, 1)
	leaks("agents:ask", ev)
	if _, err := r.a.AnswerAgent(ev.ID, true); err != nil {
		t.Fatal(err)
	}
	c.wait(t)
	if c.res.Why != agents.WhyAnswered {
		t.Fatalf("итог: %+v, %v", c.res, c.err)
	}
	leaks("итог агенту", c.res)
	leaks("agents:askdone", r.done(t, 1))
	leaks("журнал", r.a.AgentsInfo())

	if runtime.GOOS != "windows" { // отказ по правилу: тост и ошибка без пути
		r.a.askM().rules = strictRules
		r.picked = "/etc/" + secret
		c = r.ask(r.addReq())
		if _, err := r.a.AnswerAgent(r.asked(t, 2).ID, true); err != nil {
			t.Fatal(err)
		}
		c.wait(t)
		leaks("отказ агенту", c.res)
		if c.err != nil {
			leaks("ошибка агенту", c.err.Error())
		}
		leaks("тост", r.evOf("notice"))
	}
	if s := logs.String(); strings.Contains(s, secret) || strings.Contains(s, parent) {
		t.Errorf("путь в log.txt:\n%s", s)
	}
}

// Создание альбома: прежний сохранён в свой файл, новый пуст и получает выбранную папку.
func TestCreateAlbumFlow(t *testing.T) {
	r := newAskRig(t)
	a := r.a
	prev := a.editor()
	oldKey := r.key()
	file := filepath.Join(testkit.TempDir(t), "Прежний.salbum")
	if err := prev.Save(file); err != nil {
		t.Fatal(err)
	}
	c := r.ask(r.createReq())
	ev := r.asked(t, 1)
	if ev.Kind != agents.AskCreate || len(ev.Lines) < 3 || !strings.Contains(ev.Lines[1], "«Прежний»") ||
		!strings.Contains(ev.Lines[1], "сохранится в свой файл") {
		t.Errorf("agents:ask: %+v", ev)
	}
	st, err := a.AgentAnswerCreate(ev.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	c.wait(t)
	if c.err != nil || c.res.Why != agents.WhyAnswered || c.res.Folder != 1 || c.res.Merged {
		t.Fatalf("итог: %+v, %v", c.res, c.err)
	}
	next := a.editor()
	if next == prev || c.res.Album.Key != r.key() || c.res.Album.Key == oldKey || c.res.Album.Rev != next.View().Rev {
		t.Errorf("новый альбом: ключ %q (был %q), rev %d", c.res.Album.Key, oldKey, c.res.Album.Rev)
	}
	if got := st.View.Folders; len(got) != 1 || got[0] != r.other {
		t.Errorf("State: папки %v", got)
	}
	if st.View.Path != "" {
		t.Errorf("новый альбом с файлом: %q", st.View.Path)
	}
	if fi, err := os.Stat(file); err != nil || fi.Size() == 0 {
		t.Errorf("прежний альбом не записан: %v", err)
	}
	if len(r.titles) != 1 || !strings.Contains(r.titles[0], "Claude Code") {
		t.Errorf("окно ОС: %v", r.titles)
	}
}

// Черновик занят: startNew оставил прежний альбом — папка в него не добавляется, итог draft_locked.
func TestCreateAlbumDraftLocked(t *testing.T) {
	r := newAskRig(t)
	a := r.a
	a.draftLocked.Store(true)
	a.rename = func(string, string) error { return os.ErrPermission }
	prev := a.editor()
	rev := r.rev()
	c := r.ask(r.createReq())
	id := r.asked(t, 1).ID
	if _, err := a.AgentAnswerCreate(id, false); err != nil {
		t.Fatal(err)
	}
	c.wait(t)
	if c.res.Why != agents.WhyDraftLocked || c.err != nil {
		t.Fatalf("итог: %+v, %v", c.res, c.err)
	}
	if a.editor() != prev || r.rev() != rev || len(prev.View().Folders) != 1 {
		t.Errorf("прежний альбом тронут: тот же %v, rev %d→%d, папок %d", a.editor() == prev, rev, r.rev(), len(prev.View().Folders))
	}
	if n := len(r.evOf("notice")); n != 1 {
		t.Errorf("тостов: %d", n)
	}
	if d := r.done(t, 1); d.Why != agents.WhyDraftLocked {
		t.Errorf("agents:askdone: %+v", d)
	}
}

// После создания ревизия из ответа годна для сборки; ключ и ревизия прежнего альбома — нет.
func TestCreateThenBuildRev(t *testing.T) {
	r := newAskRig(t)
	a := r.a
	for i := range 6 {
		testkit.JPEG(t, r.other, fmt.Sprintf("Кадр_%d.jpg", i), 600, 400)
	}
	oldKey, oldRev := r.key(), r.rev()
	c := r.ask(r.createReq())
	if _, err := a.AgentAnswerCreate(r.asked(t, 1).ID, false); err != nil {
		t.Fatal(err)
	}
	c.wait(t)
	waitRead(t, a)
	a.bg.Wait()
	req := agentAutoReq{Key: c.res.Album.Key, Rev: c.res.Album.Rev, PerAuto: true, Series: true, Variety: 1, Variant: 1}
	if got := len(a.photos); got != 6 {
		t.Fatalf("лента: %d фото", got)
	}
	stale := req
	stale.Rev--
	if _, err := a.agentAuto(t.Context(), stale); !errors.Is(err, errAgentAlbumEdited) {
		t.Errorf("не та ревизия: %v", err)
	}
	old := req
	old.Key, old.Rev = oldKey, oldRev
	if _, err := a.agentAuto(t.Context(), old); !errors.Is(err, errAgentAlbumChanged) {
		t.Errorf("ключ прежнего альбома: %v", err)
	}
	res, err := a.agentAuto(t.Context(), req)
	if err != nil || res.Added == 0 || res.Rev <= req.Rev {
		t.Fatalf("сборка по ревизии из ответа: %+v, %v", res, err)
	}
}

// Создание не отменяется через ключ прежнего альбома: он закрыт, прежний альбом цел.
func TestCreateNoUndo(t *testing.T) {
	r := newAskRig(t)
	a := r.a
	prev := a.editor()
	oldKey, oldRev := r.key(), r.rev()
	c := r.ask(r.createReq())
	if _, err := a.AgentAnswerCreate(r.asked(t, 1).ID, false); err != nil {
		t.Fatal(err)
	}
	c.wait(t)
	newRev := r.rev()
	if _, err := a.agentUndo(t.Context(), oldKey, oldRev); !errors.Is(err, errAgentAlbumChanged) {
		t.Errorf("отмена по ключу прежнего альбома: %v", err)
	}
	if a.editor() == prev || r.rev() != newRev || prev.View().Rev != oldRev || len(prev.View().Folders) != 1 {
		t.Error("отмена тронула альбомы")
	}
}

// add одним шагом отмены: одна отмена убирает ровно папку агента.
func TestAddFolderUndo(t *testing.T) {
	r := newAskRig(t)
	a := r.a
	ed := a.editor()
	c := r.ask(r.addReq())
	if _, err := a.AnswerAgent(r.asked(t, 1).ID, true); err != nil {
		t.Fatal(err)
	}
	c.wait(t)
	if got := ed.View().Folders; len(got) != 2 {
		t.Fatalf("папки: %v", got)
	}
	rev2, err := a.agentUndo(t.Context(), c.res.Album.Key, c.res.Album.Rev)
	if err != nil {
		t.Fatal(err)
	}
	v := ed.View()
	if len(v.Folders) != 1 || v.Folders[0] != r.shots || v.Rev != rev2 || !v.CanUndo {
		t.Fatalf("после одной отмены: папки %v, rev %d, можно отменить ещё %v", v.Folders, v.Rev, v.CanUndo)
	}
	if _, err := a.agentUndo(t.Context(), c.res.Album.Key, c.res.Album.Rev); !errors.Is(err, errAgentAlbumEdited) {
		t.Errorf("повторная отмена по старой ревизии: %v", err)
	}
}
