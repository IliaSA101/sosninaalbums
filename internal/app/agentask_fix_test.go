package app

// Доправка 149 по аудиту и ревью: копия черновика до создания альбома, Cancelled у несостоявшегося
// создания, одни правила для подсказки и выбора, флажок после ожидания, имя папки с RLO в тосте.

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/app/agents"
	"github.com/IliaSA101/sosninaalbums/internal/backup"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Несохранённый черновик, хранилища копий нет: создание отклонено (ErrBackup), альбом и черновик на месте.
func TestCreateDraftNoStoreRefused(t *testing.T) {
	for name, store := range map[string]*backup.Store{"nil": nil, "пустой Root": backup.New("")} {
		r := newAskRig(t)
		a := r.a
		a.backups = store
		prev, rev := a.editor(), r.rev()
		c := r.ask(r.createReq())
		st, err := a.AgentAnswerCreate(r.asked(t, 1).ID)
		if err != nil {
			t.Fatal(err)
		}
		c.wait(t)
		if !errors.Is(c.err, agents.ErrBackup) || !st.Cancelled {
			t.Errorf("%s: err=%v cancelled=%v", name, c.err, st.Cancelled)
		}
		if a.editor() != prev || r.rev() != rev {
			t.Errorf("%s: прежний альбом тронут", name)
		}
		if _, err := os.Stat(a.draftPath()); err != nil {
			t.Errorf("%s: черновик удалён: %v", name, err)
		}
	}
}

// Копия есть: она закреплена и переживает чистку; новый альбом создан без Cancelled.
func TestCreateDraftCopyPinned(t *testing.T) {
	r := newAskRig(t)
	a := r.a
	c := r.ask(r.createReq())
	st, err := a.AgentAnswerCreate(r.asked(t, 1).ID)
	if err != nil {
		t.Fatal(err)
	}
	c.wait(t)
	if c.err != nil || c.res.Why != agents.WhyAnswered || st.Cancelled {
		t.Fatalf("итог: %+v, %v, cancelled=%v", c.res, c.err, st.Cancelled)
	}
	root := a.backups.Root
	before := copiesIn(t, filepath.Join(root, backup.DraftKey))
	if len(before) == 0 {
		t.Fatal("копии черновика нет")
	}
	if err := a.backups.Prune(); err != nil {
		t.Fatal(err)
	}
	if after := copiesIn(t, filepath.Join(root, backup.DraftKey)); len(after) != len(before) {
		t.Errorf("чистка тронула закреплённую копию: было %d, стало %d", len(before), len(after))
	}
}

// Создание кончилось ничем — Cancelled: отмена окна ОС, негодная папка, флажок выключен, черновик занят.
func TestAgentAnswerCreateCancelled(t *testing.T) {
	cases := map[string]func(r *askRig){
		"окно отменено": func(r *askRig) { r.picked = "" },
		"папка негодна": func(r *askRig) {
			if runtime.GOOS == "windows" {
				r.picked = `C:\Windows`
			} else {
				r.picked = "/etc/Секрет"
			}
			r.a.askM().rules = strictRules
		},
		"флажок выключен": func(r *askRig) {
			r.a.askM().pickDir = func(string, string) (string, error) {
				r.a.SetAgentsEdit(false) // снят, пока открыто окно ОС
				return r.other, nil
			}
		},
		"черновик занят": func(r *askRig) {
			r.a.draftLocked.Store(true)
			r.a.rename = func(string, string) error { return os.ErrPermission }
		},
	}
	for name, setup := range cases {
		r := newAskRig(t)
		setup(r)
		prev := r.a.editor()
		c := r.ask(r.createReq())
		st, err := r.a.AgentAnswerCreate(r.asked(t, 1).ID)
		c.wait(t)
		if err != nil || !st.Cancelled {
			t.Errorf("%s: err=%v cancelled=%v", name, err, st.Cancelled)
		}
		if r.a.editor() != prev {
			t.Errorf("%s: альбом заменён", name)
		}
	}
}

// Подсказка проверяется теми же правилами, что и выбранная папка: папка данных и папка миниатюр отброшены.
func TestHintUsesAskRules(t *testing.T) {
	r := newAskRig(t)
	a := r.a
	_, thumbs := dataDirs(a.data)
	rules := permissiveRules()
	rules.ours = []string{a.cfgDir, filepath.Join(testkit.TempDir(t), "thumbs")}
	if thumbs != "" {
		rules.ours = append(rules.ours, thumbs)
	}
	a.askM().rules = func() *folderRules { return rules }
	good := filepath.Join(testkit.TempDir(t), "Свадьба")
	if err := os.Mkdir(good, 0o755); err != nil {
		t.Fatal(err)
	}
	for i, c := range []struct{ hint, label string }{
		{good, "Свадьба"}, {filepath.Join(a.cfgDir, "x"), ""}, {rules.ours[1], ""},
	} {
		req := r.addReq()
		req.Hint = c.hint
		call := r.ask(req)
		ev := r.asked(t, i+1)
		if ev.Hint != c.label {
			t.Errorf("подсказка %d: %q, ждали %q", i, ev.Hint, c.label)
		}
		if _, err := a.AnswerAgent(ev.ID, false); err != nil {
			t.Fatal(err)
		}
		call.wait(t)
	}
}

// Имя добавленной папки с RLO в тост не попадает: вместо него «папка».
func TestAgentFolderToastBadRunes(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("имя с RLO на Windows недопустимо")
	}
	r := newAskRig(t)
	a := r.a
	bad := filepath.Join(testkit.TempDir(t), "a\u202Eb")
	if err := os.Mkdir(bad, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, _, err := a.editor().AddFolder(bad); err != nil {
		t.Skip("редактор не принял папку с RLO:", err)
	}
	n := len(a.editor().View().Folders)
	a.agentFolderEdited("Claude", agents.AskRes{Album: agents.AlbumRef{Key: r.key(), Rev: r.rev()}, Folder: n})
	got := r.evOf("agents:edited")
	if len(got) != 1 || got[0].(AgentEdited).Text != "Добавлена папка «папка»" {
		t.Errorf("тост: %+v", got)
	}
}

// Флажок снят, пока правка ждала ленту: сборки нет, rev прежний; то же у раскладок и отмены.
func TestAgentFlagOffAfterWait(t *testing.T) {
	r := newEditRig(t, 6)
	a := r.a
	rev, key := r.rev(), r.key()
	prefsOff := func() { a.SetAgentsEdit(false) }
	prefsOff()
	if _, err := a.agentAuto(context.Background(), r.req()); !errors.Is(err, errAgentOff) {
		t.Errorf("agentAuto: %v", err)
	}
	if _, _, err := a.agentVary(context.Background(), key, rev, nil, 1, nil); !errors.Is(err, errAgentOff) {
		t.Errorf("agentVary: %v", err)
	}
	if _, err := a.agentUndo(context.Background(), key, rev); !errors.Is(err, errAgentOff) {
		t.Errorf("agentUndo: %v", err)
	}
	if r.rev() != rev || len(a.editor().View().Spreads) != 0 {
		t.Error("альбом изменился при выключенном флажке")
	}
	if got := agentHostErr(errAgentOff); !errors.Is(got, agents.ErrNoEdits) {
		t.Errorf("перевод ошибки: %v", got)
	}
}
