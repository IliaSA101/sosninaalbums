package app

// Доправка 150 по ревью и аудиту: уход от альбома (закрепление копии без окна, один уход за раз),
// смена альбома за время пробы, провал добавления после нового альбома, причины «не тихо».

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/app/agentplaces"
	"github.com/IliaSA101/sosninaalbums/internal/app/agents"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// dirtyOwnFile — открытый свой файл с несохранёнными правками (одна добавленная папка).
func (r *quietRig) dirtyOwnFile(t *testing.T) (file string, data []byte) {
	t.Helper()
	file, data = r.savedAs(t, "Свадьба")
	if _, _, err := r.a.editor().AddFolder(r.mkdir(t, "Иванов", "x.jpg")); err != nil {
		t.Fatal(err)
	}
	if !r.a.editor().View().Dirty {
		t.Fatal("правка не оставила несохранённого")
	}
	return file, data
}

// В1: пока шла проба папки, человек открыл другой альбом — тихий create_album его не уводит.
func TestQuietCreateAlbumSwitchedDuringProbe(t *testing.T) {
	r := newQuietRig(t)
	var human *album.Editor
	first := r.a.editor()
	r.a.draftSet(first, first.View().Rev) // черновик агента: тихо заменяется
	rules, once := r.a.askM().rules, sync.Once{}
	reached, proceed := make(chan struct{}), make(chan struct{})
	r.a.askM().rules = func() *agentplaces.Rules { // из quietApply (после плана): ждём, пока человек откроет другой альбом
		if calledFrom("quietApply") {
			once.Do(func() { close(reached); <-proceed })
		}
		return rules()
	}
	c := r.ask(r.createQuiet(r.shoot))
	select {
	case <-reached:
	case <-time.After(5 * time.Second):
		t.Fatalf("применение не началось; вопросы: %+v", r.evOf("agents:ask"))
	}
	human = blankEditor("album")
	r.a.startNew(human)
	if _, _, err := human.AddFolder(r.mkdir(t, "Иванов", "x.jpg")); err != nil {
		t.Fatal(err)
	}
	close(proceed)
	c.wait(t)
	if !errors.Is(c.err, agents.ErrAlbumChanged) {
		t.Fatalf("итог: %+v, %v; ждали ErrAlbumChanged", c.res, c.err)
	}
	if r.a.editor() != human || len(human.View().Folders) != 1 {
		t.Errorf("альбом человека тронут: текущий тот же %v, папок %d", r.a.editor() == human, len(human.View().Folders))
	}
}

// В4: правка человека между копией и Close дозакрепляется — «Открыть прежний» её содержит.
func TestPinLeavingCatchesLateEdit(t *testing.T) {
	r := newQuietRig(t)
	file, data := r.dirtyOwnFile(t)
	cur := r.a.editor()
	lv, err := r.a.pinLeaving(cur)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := cur.AddFolder(r.mkdir(t, "Сидоров", "y.jpg")); err != nil { // человек правит после копии
		t.Fatal(err)
	}
	r.a.startNew(blankEditor("album"))
	lv.release()
	if lv.prev == "" || !r.a.backups.Inside(lv.prev) {
		t.Fatalf("прежний не копия: %q", lv.prev)
	}
	r.a.setPrev(lv.prev)
	if _, err := r.a.AgentOpenPrev(); err != nil {
		t.Fatal(err)
	}
	if got := len(r.a.editor().View().Folders); got != 3 {
		t.Errorf("в копии папок %d, ждали 3 (с правкой между копией и Close)", got)
	}
	if !sameBytes(t, file, data) {
		t.Error("файл человека переписан")
	}
}

// П6: два ухода не идут вместе — второй ждёт release первого, holdSave не затирается.
func TestPinLeavingSerial(t *testing.T) {
	r := newQuietRig(t)
	r.dirtyOwnFile(t)
	cur := r.a.editor()
	lv1, err := r.a.pinLeaving(cur)
	if err != nil {
		t.Fatal(err)
	}
	got := make(chan *leaving, 1)
	var wg sync.WaitGroup
	wg.Go(func() {
		lv2, _ := r.a.pinLeaving(cur)
		got <- lv2
	})
	select {
	case <-got:
		t.Fatal("второй уход начался, не дождавшись первого")
	case <-time.After(150 * time.Millisecond):
	}
	if r.a.holdSave.Load() != cur {
		t.Error("запрет записи первого ухода снят чужим уходом")
	}
	lv1.release()
	select {
	case lv2 := <-got:
		lv2.release()
	case <-time.After(5 * time.Second):
		t.Fatal("второй уход не дождался")
	}
	wg.Wait()
	if r.a.holdSave.Load() != nil || r.a.onRetire.Load() != nil {
		t.Error("после обоих уходов запрет записи или хук не сняты")
	}
}

// П5: вопрос «Создать альбом» при своём файле с несохранёнными правками называет копию, а не файл,
// и оставляет «Открыть прежний».
func TestCreateAskDirtyText(t *testing.T) {
	r := newQuietRig(t)
	r.dirtyOwnFile(t)
	r.setMode(toolCreateAlbum, prefs.PermAsk)
	c := r.ask(r.createReq())
	ev := r.asked(t, 1)
	joined := strings.Join(ev.Lines, "\n")
	if !strings.Contains(joined, "Несохранённые правки «Свадьба» сохранятся копией.") || strings.Contains(joined, "сохранится в свой файл") {
		t.Fatalf("текст вопроса: %q", joined)
	}
	if _, err := r.a.AgentAnswerCreate(ev.ID, false); err != nil {
		t.Fatal(err)
	}
	c.wait(t)
	if prev := r.a.askM().prev; prev == "" || !r.a.backups.Inside(prev) {
		t.Errorf("«Открыть прежний» не предложен: %q", prev)
	}
}

// П9: новый альбом создан, а папка не добавилась — прежний остаётся доступен и человек видит тост.
func TestQuietCreateAddFails(t *testing.T) {
	r := newQuietRig(t)
	r.dirtyOwnFile(t)
	old := quietAddFolder
	quietAddFolder = func(ed *album.Editor, dir string) (model.View, model.FolderAdd, error) {
		return model.View{}, model.FolderAdd{}, errors.New("сбой")
	}
	t.Cleanup(func() { quietAddFolder = old })
	c := r.run(t, r.createQuiet(r.shoot))
	if c.err != nil || c.res.Why != agents.WhyTooWide {
		t.Fatalf("итог: %+v, %v", c.res, c.err)
	}
	if prev := r.a.askM().prev; prev == "" {
		t.Error("прежний забыт")
	}
	toasts := r.evOf("agents:edited")
	if len(toasts) == 0 || !toasts[len(toasts)-1].(AgentEdited).Prev {
		t.Errorf("тоста с «Открыть прежний» нет: %v", toasts)
	}
}

// П7: настоящая причина важнее лимита «Всегда, а слот не выдан».
func TestQuietWhyRateLast(t *testing.T) {
	r := newQuietRig(t)
	outside := filepath.Join(testkit.TempDir(t), "Вне")
	if err := os.Mkdir(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	q := r.addQuiet(outside)
	q.Always = false // слот не выдан
	c := r.ask(q)
	if ev := r.refuse(t, c, 1); ev.Why != whyPlace {
		t.Errorf("вне места при исчерпанном лимите: %q, ждали %q", ev.Why, whyPlace)
	}
	q = r.addQuiet(r.shoot)
	q.Always = false
	c = r.ask(q)
	if ev := r.refuse(t, c, 2); ev.Why != whyRate {
		t.Errorf("только лимит: %q, ждали %q", ev.Why, whyRate)
	}
}

// В3: сетевой том, смонтированный внутри места, виден по st_dev подпапки — подсчёт не читает его молча.
func TestQuietMountInside(t *testing.T) {
	r := newQuietRig(t)
	mnt := filepath.Join(r.shoot, "Сеть")
	if err := os.Mkdir(mnt, 0o755); err != nil {
		t.Fatal(err)
	}
	oldDev, oldRemote := placeDev, quietRemote
	t.Cleanup(func() { placeDev, quietRemote = oldDev, oldRemote })
	placeDev = func(p string) (uint64, bool) {
		if strings.HasSuffix(p, "Сеть") {
			return 2, true
		}
		return 1, true
	}
	quietRemote = func(p string) bool { return strings.HasSuffix(p, "Сеть") }
	c := r.ask(r.addQuiet(r.shoot))
	if ev := r.refuse(t, c, 1); ev.Why != whyRemote {
		t.Errorf("том внутри места: %q, ждали %q", ev.Why, whyRemote)
	}
	if len(r.folders()) != 1 {
		t.Error("папка с сетевым томом добавлена молча")
	}
}

// М3: остатки временных копий ухода после падения процесса убираются при старте.
func TestSweepPinTemps(t *testing.T) {
	dir := testkit.TempDir(t)
	stale := filepath.Join(dir, pinTmpPrefix+"123"+projectExt)
	keep := filepath.Join(dir, "config.json")
	for _, f := range []string{stale, keep} {
		if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	sweepPinTemps(dir)
	sweepPinTemps("")
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Errorf("остаток копии не убран: %v", err)
	}
	if _, err := os.Stat(keep); err != nil {
		t.Errorf("чужой файл тронут: %v", err)
	}
}

// М5: вопрос об автосборке называет фактически свободные фото, а не число из входа агента.
func TestEditAskCountsFree(t *testing.T) {
	r := newEditRig(t, 8)
	ed := r.a.editor()
	req := agents.AskReq{Kind: agents.AskEdit, Client: "К", Tool: toolAutobuild, Count: 300,
		Photos: []string{r.photos[0], r.photos[0], r.photos[1], "нет-такого"}}
	b, err := r.a.editAskText(req, ed)
	if err != nil || b.Title != "Собрать развороты из 2 свободных фото в конец альбома?" {
		t.Fatalf("дубли и чужие id: %q, %v", b.Title, err)
	}
	req.Photos, req.Count, req.Build = nil, 0, 3 // фото не названы — все свободные ленты; агент просит 3 разворота
	if b, err = r.a.editAskText(req, ed); err != nil || b.Title != "Собрать 3 разворота из 8 свободных фото в конец альбома?" {
		t.Errorf("без названных фото: %q, %v", b.Title, err)
	}
}

// calledFrom — среди вызывающих есть функция с name в имени (шов теста между планом и применением).
func calledFrom(name string) bool {
	pcs := make([]uintptr, 16)
	frames := runtime.CallersFrames(pcs[:runtime.Callers(2, pcs)])
	for {
		f, more := frames.Next()
		if strings.Contains(f.Function, name) {
			return true
		}
		if !more {
			return false
		}
	}
}

// Q3: копия с правками человека, открытая из «Открыть прежний», помечена «не сохранено».
func TestAgentOpenPrevCopyIsDirty(t *testing.T) {
	r := newQuietRig(t)
	r.dirtyOwnFile(t)
	if c := r.run(t, r.createQuiet(r.shoot)); c.res.Why != agents.WhyQuiet {
		t.Fatalf("итог: %+v, %v", c.res, c.err)
	}
	if _, err := r.a.AgentOpenPrev(); err != nil {
		t.Fatal(err)
	}
	if v := r.a.editor().View(); v.Path != "" || !v.Dirty {
		t.Errorf("копия открыта: path %q, dirty %v; ждали без пути и «не сохранено»", v.Path, v.Dirty)
	}
}
