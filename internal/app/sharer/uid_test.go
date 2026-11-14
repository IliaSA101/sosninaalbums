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
	"time"

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
		if rec, ok := r.svc.look(r.curJob().UID); !ok || rec.Gone || time.Until(rec.ExpiresAt) > 31*24*time.Hour {
			t.Fatalf("ответ %d: поздняя запись правки легла: %+v", status, rec)
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

// Другой проект на том же пути (другой отпечаток) записи прежнего не видит: окно без ссылки, новая
// ссылка без вопросов и без секретов прежней, запись прежнего цела (она по uid, 167-4).
func TestShareForeignRecord(t *testing.T) {
	r := newRig(t)
	r.connect()
	r.must(Options{Days: 30, Password: "old-1234", Title: "Клиент А"})
	old, _ := r.svc.look(r.curJob().UID)
	a := r.curJob()
	j := a
	j.UID, j.Refs = uidOf("другой проект"), nil
	r.setJob(j)
	if v := r.state(); v.Link != nil {
		t.Fatalf("окно показывает ссылку другого проекта: %+v", v.Link)
	}
	if _, err := r.svc.Text(); err == nil {
		t.Error("«Скопировать» отдало ссылку другого проекта")
	}
	if _, err := r.svc.Change(Change{Days: 90}); err == nil {
		t.Error("правка ссылки другого проекта прошла")
	}
	if ok, err := r.svc.Share(Options{}); ok || err == nil {
		t.Fatalf("Days:0 без записи: %v %v", ok, err)
	}
	r.must(Options{Days: 30})
	if len(r.asks) != 0 {
		t.Errorf("вопросы: %q", r.asks)
	}
	got, ok := r.svc.look(j.UID)
	if !ok || got.ID == old.ID || got.Password != "" || got.Title == "Клиент А" {
		t.Fatalf("запись другого проекта: %+v", got)
	}
	if again, ok := r.svc.look(a.UID); !ok || again.ID != old.ID {
		t.Fatalf("запись прежнего проекта тронута: %+v", again)
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

// П.3: записи, что не стёрлись (projects.json не пишется), стираются повтором: look — своя,
// удачный putIf — все сразу.
func TestShareDelRetried(t *testing.T) {
	r := newRig(t)
	r.connect()
	dir := filepath.Dir(r.curJob().Path)
	var list []string // отпечатки
	for i, name := range []string{"a.salbum", "b.salbum", "c.salbum"} {
		r.setJob(testJob(t, filepath.Join(dir, name), 1, uint8(i)))
		r.must(Options{Days: 30})
		list = append(list, r.curJob().UID)
	}
	keep, _ := r.svc.look(list[0])
	file := prefs.ProjectsPath(r.cfg)
	saved, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	freezeShares(t, r.cfg)
	for _, u := range list {
		if r.svc.drop(u) == nil {
			t.Fatal("оснастка: стирание в замороженном файле прошло")
		}
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
	onDisk := func(u string) bool {
		_, ok := prefs.NewProjects(r.cfg, r.svc.rules()).Get(u)
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
	other := uidOf("d.salbum")
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
