package sharer

import (
	"net/http"
	"path/filepath"
	"testing"

	"github.com/IliaSA101/sosninaalbums/api/albums/albumstest"
)

// «Обновить этот альбом» с другого пути (167-18, design §4.4): привязка ложится в файл только после
// последнего putIf, когда запись уже переехала на путь файла. Срыв раньше (после Update) не должен
// оставить привязку в файле при записи на старом пути.

// movedChanged — альбом перенесён в name, и сцена изменена (страницы придётся грузить); отпечаток
// прежний, привязок в файле нет. Прежний id и путь записи возвращаются.
func movedChanged(t *testing.T, r *rig, name string) (id, uid, oldPath string) {
	t.Helper()
	id, uid = r.upload("", "")
	old := r.curJob()
	oldPath = old.Path
	sp := old.Spreads
	for i := range sp { // все страницы другие: сервис просит догрузить шесть
		sp[i].Bg[0].B, sp[i].Bg[1].B = 250-uint8(i), 240-uint8(i)
	}
	nj := jobOf(t, old.Path, sp)
	nj.Path = filepath.Join(filepath.Dir(old.Path), name+".salbum")
	nj.Title = name
	nj.UID, nj.Refs = uid, nil
	r.setJob(nj)
	r.mu.Lock()
	r.binds = nil
	r.mu.Unlock()
	if v := r.state(); v.Link == nil || v.Link.State != "moved" {
		t.Fatalf("до выгрузки: %+v", v.Link)
	}
	return id, uid, oldPath
}

// noBind — привязки в файле нет, Bind не звали, запись осталась на старом пути.
func noBind(t *testing.T, r *rig, id, uid, oldPath string) {
	t.Helper()
	r.mu.Lock()
	binds, refs := append([]string(nil), r.binds...), append([]string(nil), r.job.Refs...)
	cur := r.job.Path
	r.mu.Unlock()
	if len(binds) != 0 || len(refs) != 0 {
		t.Errorf("привязка легла в файл: Bind %v, привязки файла %v", binds, refs)
	}
	rec, ok := r.svc.look(uid)
	if !ok || rec.ID != id || rec.Path != oldPath || rec.Own(cur) {
		t.Errorf("запись должна остаться на старом пути %q: %v %+v", oldPath, ok, rec)
	}
}

func TestShareMovedUpdateFailNoBind(t *testing.T) {
	r := newRig(t)
	id, uid, oldPath := movedChanged(t, r, "b")
	var updates int
	r.setFault(func(q *http.Request) *albumstest.Fault {
		if isPublish(q) { // Update прошёл и страницы загружены, публикация срывается
			updates++
			return &albumstest.Fault{Status: http.StatusBadRequest}
		}
		return nil
	})
	msg, stopped := r.run(Options{Choice: "update"})
	if msg == "" || stopped || updates == 0 {
		t.Fatalf("итог %q stopped=%v, publish звали %d раз", msg, stopped, updates)
	}
	noBind(t, r, id, uid, oldPath)
}

func TestShareMovedUpdateStopNoBind(t *testing.T) {
	r := newRig(t)
	id, uid, oldPath := movedChanged(t, r, "b")
	stopAfterTwo(r) // «Остановить» во время выгрузки страниц
	msg, stopped := r.run(Options{Choice: "update"})
	if msg != "" || !stopped {
		t.Fatalf("итог %q stopped=%v", msg, stopped)
	}
	noBind(t, r, id, uid, oldPath)
}

// Контроль: без срыва привязка ложится один раз и запись переезжает на путь файла.
func TestShareMovedUpdateBindsOnSuccess(t *testing.T) {
	r := newRig(t)
	id, uid, _ := movedChanged(t, r, "b")
	r.must(Options{Choice: "update"})
	r.mu.Lock()
	binds, refs, cur := append([]string(nil), r.binds...), append([]string(nil), r.job.Refs...), r.job.Path
	r.mu.Unlock()
	if len(binds) != 1 || binds[0] != id || len(refs) != 1 {
		t.Errorf("Bind %v, привязки файла %v", binds, refs)
	}
	if rec, _ := r.svc.look(uid); rec.ID != id || !rec.Own(cur) {
		t.Errorf("запись не переехала: %+v", rec)
	}
}

// Пункт A: запись найдена на сервисе (другой компьютер) — даты выгрузки нет, название есть.
func TestShareFindPublishedLine(t *testing.T) {
	r := newRig(t)
	_, uid := r.upload("Иванов", "pass-1234")
	if v := r.state(); v.Link == nil || v.Link.Published == "" {
		t.Fatalf("своя запись без строки «Выгружено»: %+v", v.Link)
	}
	r.secondComputer(uid)
	v, err := r.svc.Find()
	if err != nil || v.Link == nil || v.Link.State != "moved" {
		t.Fatalf("Find: %v, %+v", err, v.Link)
	}
	const want = "Выгружено как «Иванов»"
	if v.Link.Published != want {
		t.Errorf("Find: Published = %q, ждали %q", v.Link.Published, want)
	}
	if got := r.state().Link; got == nil || got.Published != want {
		t.Errorf("State: %+v", got)
	}
}
