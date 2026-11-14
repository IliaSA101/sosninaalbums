package app

// Тихий путь: разрешение между пробой и записью, бюджет копии, черновики, «Открыть прежний»,
// причины Why в вопросе (150, design §4.1; стенд — agentquiet_test.go).

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/app/agents"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Разрешение сменили, пока шла проба тома: «Запретить» — ничего не записано, «Спрашивать» — вопрос.
func TestQuietPermFlipDuringProbe(t *testing.T) {
	r := newQuietRig(t)
	quietRemote = func(string) bool { r.setMode(toolAddFolder, prefs.PermDeny); return false }
	c := r.run(t, r.addQuiet(r.shoot))
	if c.res.Why != agents.WhyOff || len(r.folders()) != 1 || len(edited(r.askRig)) != 0 {
		t.Fatalf("deny во время пробы: %+v, папки %v", c.res, r.folders())
	}
	r.setMode(toolAddFolder, prefs.PermAllow)
	quietRemote = func(string) bool { r.setMode(toolAddFolder, prefs.PermAsk); return false }
	c2 := r.ask(r.addQuiet(r.shoot))
	if ev := r.refuse(t, c2, 1); ev.Why != "" || ev.Path != r.shoot {
		t.Errorf("ask во время пробы: вопрос без причины, got %+v", ev)
	}
	if len(r.folders()) != 1 {
		t.Error("папка добавлена вопреки «Спрашивать»")
	}
}

// Копии нет — правки нет: ошибка агенту, папка не добавлена.
func TestQuietBackup(t *testing.T) {
	r := newQuietRig(t)
	r.a.backups = nil
	// Несохранённый непустой альбом (папка из стенда): копия нужна, а хранилища нет.
	c := r.run(t, r.addQuiet(r.shoot))
	if c.err == nil || c.res.Why == agents.WhyQuiet || len(r.folders()) != 1 {
		t.Errorf("без хранилища копий: %+v, %v, папки %v", c.res, c.err, r.folders())
	}
}

// Альбом, который правил человек, тихо не заменяется; пустой и сохранённый — заменяются.
func TestQuietDraft(t *testing.T) {
	r := newQuietRig(t)
	c := r.ask(r.createQuiet(r.shoot)) // в стенде открыт черновик с папкой «Съёмка», его никто не создавал через агента
	if ev := r.refuse(t, c, 1); ev.Why != whyDraft || ev.Kind != "create" || ev.Tool != toolCreateAlbum {
		t.Errorf("черновик человека: %+v", ev)
	}
	// Сохранённый в файл — уходит в свой файл, тихо; «Открыть прежний» вернёт его.
	file := filepath.Join(testkit.TempDir(t), "Свадьба"+projectExt)
	if err := r.a.editor().Save(file); err != nil {
		t.Fatal(err)
	}
	c = r.run(t, r.createQuiet(r.shoot))
	if c.res.Why != agents.WhyQuiet || c.err != nil {
		t.Fatalf("сохранённый: %+v, %v", c.res, c.err)
	}
	if got := edited(r.askRig); len(got) != 1 || !got[0].Prev || got[0].Text != "Новый альбом из" || got[0].Path != r.shoot {
		t.Errorf("тост: %+v", got)
	}
	if st, err := r.a.AgentOpenPrev(); err != nil || r.a.editor().Path() != file || st.Cancelled {
		t.Errorf("AgentOpenPrev: %v, путь %q", err, r.a.editor().Path())
	}
	if _, err := r.a.AgentOpenPrev(); err == nil {
		t.Error("прежний открыт дважды")
	}
}

// Нетронутый черновик агента заменяется тихо с закреплённой копией; тронутый человеком — вопрос.
func TestQuietAgentDraft(t *testing.T) {
	r := newQuietRig(t)
	ed := r.a.editor()
	r.a.draftSet(ed, ed.View().Rev) // как после create_album
	other := r.mkdir(t, "Иванов", "x.jpg")
	// Человек правит: ревизия ушла вперёд.
	if _, _, err := ed.AddFolder(other); err != nil {
		t.Fatal(err)
	}
	c := r.ask(r.createQuiet(r.shoot))
	if ev := r.refuse(t, c, 1); ev.Why != whyDraft {
		t.Fatalf("правленный человеком: %+v", ev)
	}
	// Правка агента (переходник Host) сдвигает отметку вместе с ревизией: снова нетронутый.
	r.a.draftSet(ed, ed.View().Rev)
	rev := ed.View().Rev
	if _, _, err := ed.AddFolder(r.shoot); err != nil {
		t.Fatal(err)
	}
	r.a.agentDraftStep(r.key(), rev, ed.View().Rev)
	c = r.run(t, r.createQuiet(r.shoot))
	if c.res.Why != agents.WhyQuiet || r.a.editor() == ed {
		t.Fatalf("черновик агента: %+v, %v", c.res, c.err)
	}
	// Прежний — закреплённая копия; «Открыть прежний» открывает её, а текущий (несохранённый) закрепляет.
	pinned := r.a.askM().prev
	if pinned == "" || !r.a.backups.Inside(pinned) {
		t.Fatalf("копия прежнего: %q", pinned)
	}
	if _, err := os.Stat(pinned); err != nil {
		t.Fatal(err)
	}
	cur := r.a.editor()
	if _, err := r.a.AgentOpenPrev(); err != nil || r.a.editor() == cur {
		t.Fatalf("AgentOpenPrev: %v", err)
	}
	if got := len(r.a.editor().View().Folders); got != 3 {
		t.Errorf("в открытой копии папок %d, ждали 3 (Съёмка, Иванов, Петровы)", got)
	}
}
