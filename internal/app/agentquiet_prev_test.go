package app

// «Открыть прежний» и уход от открытого альбома (150, design §6): свой файл с несохранёнными
// правками и правленный человеком черновик закрепляются копией, файл человека не переписывается.

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/app/agents"
	"github.com/IliaSA101/sosninaalbums/internal/backup"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// savedAs сохраняет открытый альбом в новый файл и возвращает путь с байтами файла.
func (r *quietRig) savedAs(t *testing.T, name string) (file string, data []byte) {
	t.Helper()
	file = filepath.Join(testkit.TempDir(t), name+projectExt)
	if err := r.a.editor().Save(file); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	return file, data
}

func sameBytes(t *testing.T, file string, want []byte) bool {
	t.Helper()
	got, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	return bytes.Equal(got, want)
}

// Прежний альбом по виду: свой файл без правок, закреплённая копия, черновик агента, правленный
// человеком, свой файл с несохранёнными правками.
func TestAgentOpenPrev(t *testing.T) {
	for _, c := range []struct {
		name string
		run  func(*testing.T)
	}{
		{"файл", prevFile},
		{"закреплённая копия", prevPinnedCopy},
		{"черновик агента закреплён до ухода", prevAgentDraftPinned},
		{"новый альбом, правленный человеком", prevHumanEdited},
		{"свой файл с несохранёнными правками", prevOwnFileDirty},
		{"текущий свой файл с правками при «Открыть прежний»", prevCurrentFileDirty},
		{"копии нет — файл не тронут и альбом остаётся", prevNoStore},
	} {
		t.Run(c.name, c.run)
	}
}

func prevFile(t *testing.T) {
	r := newQuietRig(t)
	file, data := r.savedAs(t, "Свадьба")
	if c := r.run(t, r.createQuiet(r.shoot)); c.res.Why != agents.WhyQuiet {
		t.Fatalf("итог: %+v, %v", c.res, c.err)
	}
	if got := r.a.askM().prev; got != file {
		t.Fatalf("прежний: %q, ждали файл %q", got, file)
	}
	if _, err := r.a.AgentOpenPrev(); err != nil || r.a.editor().Path() != file {
		t.Fatalf("AgentOpenPrev: %v, путь %q", err, r.a.editor().Path())
	}
	if !sameBytes(t, file, data) {
		t.Error("файл без правок переписан")
	}
	if _, err := r.a.AgentOpenPrev(); err == nil {
		t.Error("прежний открыт дважды")
	}
}

func prevPinnedCopy(t *testing.T) {
	r := newQuietRig(t)
	ed := r.a.editor()
	r.a.draftSet(ed, ed.View().Rev) // как после create_album
	if c := r.run(t, r.createQuiet(r.shoot)); c.res.Why != agents.WhyQuiet {
		t.Fatalf("итог: %+v, %v", c.res, c.err)
	}
	prev := r.a.askM().prev
	if prev == "" || !r.a.backups.Inside(prev) {
		t.Fatalf("прежний не копия: %q", prev)
	}
	if _, err := r.a.AgentOpenPrev(); err != nil || r.a.editor() == ed {
		t.Fatalf("AgentOpenPrev: %v", err)
	}
	if got := r.a.editor().View().Folders; len(got) != 1 {
		t.Errorf("в копии папок %v, ждали «Съёмку»", got)
	}
}

func prevAgentDraftPinned(t *testing.T) {
	r := newQuietRig(t)
	ed := r.a.editor()
	r.a.draftSet(ed, ed.View().Rev)
	dir := filepath.Join(r.a.backups.Root, backup.DraftKey)
	if n := len(copiesIn(t, dir)); n != 0 {
		t.Fatalf("копий до ухода %d", n)
	}
	if c := r.run(t, r.createQuiet(r.shoot)); c.res.Why != agents.WhyQuiet {
		t.Fatalf("итог: %+v, %v", c.res, c.err)
	}
	before := copiesIn(t, dir)
	if len(before) == 0 {
		t.Fatal("копии черновика агента нет")
	}
	if err := r.a.backups.Prune(); err != nil {
		t.Fatal(err)
	}
	if after := copiesIn(t, dir); len(after) != len(before) {
		t.Errorf("чистка тронула закреплённую копию: %d → %d", len(before), len(after))
	}
	if _, err := os.Stat(r.a.askM().prev); err != nil {
		t.Errorf("копия для «Открыть прежний»: %v", err)
	}
}

func prevHumanEdited(t *testing.T) {
	r := newQuietRig(t)
	first := r.a.editor()
	r.a.draftSet(first, first.View().Rev)
	if c := r.run(t, r.createQuiet(r.shoot)); c.res.Why != agents.WhyQuiet {
		t.Fatalf("итог: %+v, %v", c.res, c.err)
	}
	cur := r.a.editor()
	if _, _, err := cur.AddFolder(r.mkdir(t, "Иванов", "x.jpg")); err != nil { // человек правит
		t.Fatal(err)
	}
	dir := filepath.Join(r.a.backups.Root, backup.DraftKey)
	before := len(copiesIn(t, dir))
	if _, err := r.a.AgentOpenPrev(); err != nil || r.a.editor() == cur {
		t.Fatalf("AgentOpenPrev: %v", err)
	}
	if after := len(copiesIn(t, dir)); after <= before {
		t.Errorf("правленный человеком альбом не закреплён: копий %d → %d", before, after)
	}
}

func prevOwnFileDirty(t *testing.T) {
	r := newQuietRig(t)
	file, data := r.savedAs(t, "Свадьба")
	ed := r.a.editor()
	if _, _, err := ed.AddFolder(r.mkdir(t, "Иванов", "x.jpg")); err != nil { // правка без сохранения
		t.Fatal(err)
	}
	if !ed.View().Dirty {
		t.Fatal("правка не оставила несохранённого")
	}
	if c := r.run(t, r.createQuiet(r.shoot)); c.res.Why != agents.WhyQuiet {
		t.Fatalf("итог: %+v, %v", c.res, c.err)
	}
	if !sameBytes(t, file, data) {
		t.Fatal("файл человека переписан при уходе")
	}
	prev := r.a.askM().prev
	if prev == file || prev == "" || !r.a.backups.Inside(prev) {
		t.Fatalf("прежний должен быть копией с правками, а не файлом: %q", prev)
	}
	if _, err := r.a.AgentOpenPrev(); err != nil || r.a.editor() == ed {
		t.Fatalf("AgentOpenPrev: %v", err)
	}
	if got := len(r.a.editor().View().Folders); got != 2 {
		t.Errorf("в копии папок %d, ждали 2 (с правкой человека)", got)
	}
	if !sameBytes(t, file, data) {
		t.Error("файл человека переписан после «Открыть прежний»")
	}
	if r.a.holdSave.Load() != nil {
		t.Error("запрет записи не снят")
	}
}

func prevCurrentFileDirty(t *testing.T) {
	r := newQuietRig(t)
	first, firstData := r.savedAs(t, "Первый")
	if c := r.run(t, r.createQuiet(r.shoot)); c.res.Why != agents.WhyQuiet {
		t.Fatalf("итог: %+v, %v", c.res, c.err)
	}
	cur, curData := r.savedAs(t, "Второй")
	if _, _, err := r.a.editor().AddFolder(r.mkdir(t, "Иванов", "x.jpg")); err != nil {
		t.Fatal(err)
	}
	if _, err := r.a.AgentOpenPrev(); err != nil || r.a.editor().Path() != first {
		t.Fatalf("AgentOpenPrev: %v, путь %q", err, r.a.editor().Path())
	}
	if !sameBytes(t, cur, curData) || !sameBytes(t, first, firstData) {
		t.Error("файл переписан")
	}
}

func prevNoStore(t *testing.T) {
	r := newQuietRig(t)
	file, data := r.savedAs(t, "Свадьба")
	ed := r.a.editor()
	if _, _, err := ed.AddFolder(r.mkdir(t, "Иванов", "x.jpg")); err != nil {
		t.Fatal(err)
	}
	r.a.backups = nil
	c := r.run(t, r.createQuiet(r.shoot))
	if c.err == nil || r.a.editor() != ed || !sameBytes(t, file, data) {
		t.Errorf("без хранилища: %+v, %v, альбом заменён %v", c.res, c.err, r.a.editor() != ed)
	}
	if r.a.holdSave.Load() != nil {
		t.Error("запрет записи не снят")
	}
}

// Вопрос «Создать альбом» (askApplyCreate): свой файл с несохранёнными правками тоже копия.
func TestCreateAskKeepsOwnFile(t *testing.T) {
	r := newQuietRig(t)
	file, data := r.savedAs(t, "Свадьба")
	ed := r.a.editor()
	if _, _, err := ed.AddFolder(r.mkdir(t, "Иванов", "x.jpg")); err != nil {
		t.Fatal(err)
	}
	r.setMode(toolCreateAlbum, prefs.PermAsk)
	c := r.ask(r.createReq())
	st, err := r.a.AgentAnswerCreate(r.asked(t, 1).ID, false)
	if err != nil {
		t.Fatal(err)
	}
	c.wait(t)
	if st.Cancelled || r.a.editor() == ed || c.err != nil {
		t.Fatalf("создание: cancelled=%v err=%v", st.Cancelled, c.err)
	}
	if !sameBytes(t, file, data) {
		t.Error("файл человека переписан")
	}
	pins := copiesIn(t, filepath.Join(r.a.backups.Root, backup.Key(file)))
	if len(pins) == 0 {
		t.Error("копии правок человека нет")
	}
}
