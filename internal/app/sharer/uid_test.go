package sharer

// Отпечаток проекта (146 T4-9): запись о ссылке другого проекта на том же пути — чужая; поздняя
// запись Change после «Сохранить как» не пишется; метки del стираются повтором.

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/api/albums/albumstest"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

// П.1: пока шёл PATCH, на путь сохранили другой проект — ни запись правки, ни пометка «нет на
// сервере» (404) прежнюю запись не возвращают; принятая сервисом правка — ошибка «альбом сменился»
// (новый пароль уже на сервере); флаг правки после этого снят.
func TestShareChangeSavedDuringPatch(t *testing.T) {
	for _, status := range []int{0, http.StatusNotFound} {
		r := newRig(t)
		r.connect()
		r.must(Options{Days: 30})
		path := r.curJob().Path
		r.setFault(func(q *http.Request) *albumstest.Fault {
			if q.Method != http.MethodPatch {
				return nil
			}
			r.svc.OnSaved(path, true)
			if status != 0 {
				return &albumstest.Fault{Status: status}
			}
			return nil
		})
		_, err := r.svc.Change(Change{Days: 90})
		// Правка принята (200): сервис уже знает новый срок, а запись не сохранена — ошибка, не тишина.
		if status == 0 && (err == nil || !strings.Contains(err.Error(), "альбом за это время сменился")) {
			t.Fatalf("правка принята, запись не сохранена: %v", err)
		}
		if status != 0 && err == nil {
			t.Fatalf("ответ %d без ошибки", status)
		}
		if rec, ok := r.svc.look(path); ok {
			t.Fatalf("ответ %d: запись прежнего альбома вернулась: %+v", status, rec)
		}
		r.svc.mu.Lock()
		net, stop := r.svc.net, r.svc.netStop
		r.svc.mu.Unlock()
		if net || stop {
			t.Fatalf("ответ %d: флаг правки %v, стоп %v", status, net, stop)
		}
	}
}

// restart — служба заново над теми же папками и хранилищем (перезапуск программы).
func (r *rig) restart() {
	s := New(r.svc.h)
	s.gap, s.wait = 0, r.svc.wait
	r.svc = s
}

// П.2: запись другого проекта на этом пути — окно без ссылки, «Обновить» (Days:0) — отказ,
// новая ссылка — только после вопроса, запросов по чужой записи нет; «Да» — новая ссылка
// с отпечатком текущего проекта, «Нет» — чужая запись цела.
func TestShareForeignRecord(t *testing.T) {
	for _, tc := range foreignCases {
		t.Run(tc.name, func(t *testing.T) {
			r := newRig(t)
			r.connect()
			r.must(Options{Days: 30})
			old, _ := r.svc.look(r.curJob().Path)
			if old.UID != r.curJob().UID {
				t.Fatalf("оснастка: отпечаток записи %q", old.UID)
			}
			tc.swap(r)
			foreignRefused(t, r, old.ID)
			foreignNewLink(t, r, old.ID)
		})
	}
}

// foreignCases — как запись пути становится чужой текущему проекту.
var foreignCases = []struct {
	name string
	swap func(r *rig)
}{
	{"файл заменён другим проектом", func(r *rig) {
		j := r.curJob()
		j.UID = uidOf("другой проект")
		r.setJob(j)
	}},
	{"перезапуск, отпечаток в файле другой", func(r *rig) {
		r.restart()
		j := r.curJob()
		j.UID = uidOf("после перезапуска")
		r.setJob(j)
	}},
	{"печать не этого пути (копия шаблона, файл с другого пути)", func(r *rig) {
		j := r.curJob() // отпечаток тот же, что в записи: чужой её делает только печать
		j.Unsealed = true
		r.setJob(j)
		r.mu.Lock()
		r.pinUID = uidOf("выдан при выгрузке")
		r.mu.Unlock()
	}},
	{"запись без отпечатка", func(r *rig) {
		rec, _ := r.svc.look(r.curJob().Path)
		rec.UID = ""
		if err := r.svc.put(r.curJob().Path, rec); err != nil {
			r.t.Fatal(err)
		}
		r.restart()
	}},
	{"запись с негодным отпечатком", func(r *rig) {
		p := prefs.SharesPath(r.cfg)
		b, err := os.ReadFile(p)
		if err != nil {
			r.t.Fatal(err)
		}
		b = []byte(strings.Replace(string(b), r.curJob().UID, strings.ToUpper(r.curJob().UID), 1))
		if err := os.WriteFile(p, b, 0o600); err != nil {
			r.t.Fatal(err)
		}
		r.restart()
	}},
}

// foreignRefused — по чужой записи ни ссылки в окне, ни правки, ни «Обновить»; «Нет» на вопрос
// о новой ссылке — без запросов, запись цела.
func foreignRefused(t *testing.T, r *rig, oldID string) {
	t.Helper()
	if v := r.state(); v.Link != nil {
		t.Fatalf("окно показывает ссылку другого проекта: %+v", v.Link)
	}
	if _, err := r.svc.Text(); err == nil {
		t.Error("«Скопировать» отдало ссылку другого проекта")
	}
	if _, err := r.svc.Change(Change{Days: 90}); err == nil {
		t.Error("правка ссылки другого проекта прошла")
	}
	h := &headerLog{}
	r.setFault(h.fault)
	if ok, err := r.svc.Share(Options{}); ok || err == nil {
		t.Fatalf("Days:0 по чужой записи: %v %v", ok, err)
	}
	r.mu.Lock()
	r.answer = false
	pinsBefore := len(r.pins)
	r.mu.Unlock()
	if ok, err := r.svc.Share(Options{Days: 30}); ok || err != nil {
		t.Fatalf("«Нет» на вопрос: %v %v", ok, err)
	}
	if r.asks[len(r.asks)-1] != foreignQuestion {
		t.Fatalf("вопрос: %q", r.asks[len(r.asks)-1])
	}
	if h.n != 0 {
		t.Errorf("по чужой записи ушло %d запросов", h.n)
	}
	// «Нет» — до выдачи отпечатка: Pin переписывает файл альбома, а отказ не должен его трогать
	r.mu.Lock()
	pinned := len(r.pins) - pinsBefore
	r.mu.Unlock()
	if pinned != 0 {
		t.Errorf("после «Нет» Pin вызван %d раз", pinned)
	}
	if rec, ok := r.svc.look(r.curJob().Path); !ok || rec.ID != oldID {
		t.Fatalf("«Нет» тронуло запись: %+v %v", rec, ok)
	}
	r.setFault(nil)
}

// foreignNewLink — «Да»: новая ссылка с отпечатком текущего проекта.
func foreignNewLink(t *testing.T, r *rig, oldID string) {
	t.Helper()
	r.mu.Lock()
	r.answer = true
	r.mu.Unlock()
	if msg, stopped := r.run(Options{Days: 30}); msg != "" || stopped {
		t.Fatalf("новая ссылка: %q %v", msg, stopped)
	}
	rec, ok := r.svc.look(r.curJob().Path)
	if !ok || rec.ID == oldID || rec.UID != r.curJob().UID {
		t.Fatalf("запись после новой ссылки: %+v (прежний id %s)", rec, oldID)
	}
	if v := r.state(); v.Link == nil || v.Link.State != "ok" {
		t.Errorf("новая ссылка не видна: %+v", v.Link)
	}
}

// Pin: отпечаток уходит в файл до выгрузки; ошибка записи останавливает её до запросов,
// записи о ссылке нет, флаг выгрузки снят.
func TestSharePinFails(t *testing.T) {
	r := newRig(t)
	r.connect()
	r.mu.Lock()
	r.pinErr = errors.New("диск занят")
	r.mu.Unlock()
	h := &headerLog{}
	r.setFault(h.fault)
	ok, err := r.svc.Share(Options{Days: 30})
	if ok || err == nil || !strings.Contains(err.Error(), "диск занят") {
		t.Fatalf("Share при ошибке Pin: %v %v", ok, err)
	}
	if h.n != 0 || r.svc.Busy() {
		t.Fatalf("запросов %d, выгрузка идёт: %v", h.n, r.svc.Busy())
	}
	if _, ok := r.rec(); ok {
		t.Fatal("запись о ссылке без отпечатка в файле")
	}
	r.mu.Lock()
	r.pinErr = nil
	r.mu.Unlock()
	r.must(Options{Days: 30})
	uid := r.curJob().UID
	r.mu.Lock()
	pins := slices.Clone(r.pins)
	r.mu.Unlock()
	if len(pins) != 2 || pins[0] != uid || pins[1] != uid {
		t.Fatalf("Pin: %v, ждали дважды %s", pins, uid)
	}
}

// П.3: записи, что не стёрлись (shares.json не пишется), стираются повтором: look — своя,
// удачный putIf — все сразу.
func TestShareDelRetried(t *testing.T) {
	r := newRig(t)
	r.connect()
	dir := filepath.Dir(r.curJob().Path)
	var list []string
	for i, name := range []string{"a.salbum", "b.salbum", "c.salbum"} {
		r.setJob(testJob(t, filepath.Join(dir, name), 1, uint8(i)))
		r.must(Options{Days: 30})
		list = append(list, r.curJob().Path)
	}
	keep, _ := r.svc.look(list[0])
	file := prefs.SharesPath(r.cfg)
	saved, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	freezeShares(t, r.cfg)
	for _, p := range list {
		r.svc.OnSaved(p, true)
	}
	r.svc.mu.Lock()
	n := len(r.svc.del)
	r.svc.mu.Unlock()
	if n != len(list) {
		t.Fatalf("меток del %d, ждали %d", n, len(list))
	}
	if err := os.WriteFile(file, saved, 0o600); err != nil { // диск снова пишется
		t.Fatal(err)
	}
	onDisk := func(p string) bool {
		_, ok := prefs.NewShares(r.cfg, r.svc.rules()).Get(p)
		return ok
	}
	if _, ok := r.svc.look(list[0]); ok || onDisk(list[0]) {
		t.Fatal("look не стёр свою запись")
	}
	r.svc.mu.Lock()
	n = len(r.svc.del)
	r.svc.mu.Unlock()
	if n != len(list)-1 || !onDisk(list[1]) {
		t.Fatalf("look тронул чужие метки: %d, b на диске %v", n, onDisk(list[1]))
	}
	other := filepath.Join(dir, "d.salbum")
	if wrote, err := r.svc.putIf(other, keep, func() bool { return true }); !wrote || err != nil {
		t.Fatalf("putIf: %v %v", wrote, err)
	}
	r.svc.mu.Lock()
	n = len(r.svc.del)
	r.svc.mu.Unlock()
	if n != 0 || onDisk(list[1]) || onDisk(list[2]) || !onDisk(other) {
		t.Fatalf("после удачной записи: меток %d, b %v, c %v, d %v", n, onDisk(list[1]), onDisk(list[2]), onDisk(other))
	}
}
