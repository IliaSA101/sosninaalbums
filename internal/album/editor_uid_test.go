package album

// Отпечаток проекта (146 T4-9, editor_uid.go): выдача без шага отмены, смена при восстановлении
// копии и «Сохранить как», запись перед выгрузкой.

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// uidEditor — Editor над файлом path с проектом p (сначала p записан как есть). Отпечаток без
// печати получает печать path — как файл, где отпечаток выдан по этому пути (SaveNew).
func uidEditor(t *testing.T, p *model.Project, path string) *Editor {
	t.Helper()
	if model.GoodUID(p.UID) && p.UIDAt() == "" {
		p.SetUIDAt(model.UIDTag(p.UID, path))
	}
	if err := salbum.Save(p, path); err != nil {
		t.Fatal(err)
	}
	q, err := salbum.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	e, err := NewEditor(q, path)
	if err != nil {
		t.Fatal(err)
	}
	return e
}

// fileUID — отпечаток в файле альбома.
func fileUID(t *testing.T, path string) string {
	t.Helper()
	q, err := salbum.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	return q.UID
}

// Альбом до 0.32 (без поля): отпечаток выдан без шага отмены, rev и dirty; в файле его нет,
// пока не записан SaveUID; выданный при открытии без печати — SaveUID выдаёт новый с печатью
// пути; второй SaveUID — холостой. Несколько альбомов — разные отпечатки.
func TestEditorUIDOldFile(t *testing.T) {
	dir := testkit.TempDir(t)
	seen := map[string]bool{}
	for i := range 3 {
		path := filepath.Join(dir, "old"+string(rune('a'+i))+".salbum")
		e := uidEditor(t, model.New(), path)
		path, uid := e.Ident()
		if !model.GoodUID(uid) || seen[uid] {
			t.Fatalf("отпечаток %q (повтор: %v)", uid, seen[uid])
		}
		seen[uid] = true
		if len(e.undo) != 0 || e.dirty || e.rev != 0 {
			t.Fatalf("выдача отпечатка — правка: шагов %d, dirty %v, rev %d", len(e.undo), e.dirty, e.rev)
		}
		if need, err := e.NeedUID(path, uid); !need || err != nil {
			t.Fatalf("NeedUID до записи: %v %v", need, err)
		}
		if got := fileUID(t, path); got != "" {
			t.Fatalf("в файле до записи %q", got)
		}
		pinned, err := e.SaveUID(path, uid)
		if err != nil {
			t.Fatal(err)
		}
		if pinned == uid || !model.GoodUID(pinned) || seen[pinned] || e.p.UID != pinned {
			t.Fatalf("SaveUID без печати: %q (был %q, в альбоме %q)", pinned, uid, e.p.UID)
		}
		seen[pinned], uid = true, pinned
		if need, err := e.NeedUID(path, uid); need || err != nil {
			t.Fatalf("NeedUID после записи: %v %v", need, err)
		}
		if got := fileUID(t, path); got != uid || fileTag(t, path) != model.UIDTag(uid, path) {
			t.Fatalf("в файле %q, ждали %q с печатью пути", got, uid)
		}
		st, _ := os.Stat(path)
		if again, err := e.SaveUID(path, uid); again != uid || err != nil {
			t.Fatal(again, err)
		}
		if st2, _ := os.Stat(path); !st2.ModTime().Equal(st.ModTime()) || st2.Size() != st.Size() {
			t.Error("повторный SaveUID переписал файл")
		}
	}
}

// Файл с отпечатком: тот же после открытия, в записи нет нужды; негодный в файле — отброшен,
// альбом получает новый. Новый альбом без файла: сверка отказывает (пути нет).
func TestEditorUIDFromFile(t *testing.T) {
	dir := testkit.TempDir(t)
	p := model.New()
	p.UID = model.NewUID()
	path := filepath.Join(dir, "a.salbum")
	e := uidEditor(t, p, path)
	if _, uid := e.Ident(); uid != p.UID {
		t.Fatalf("отпечаток %q, в файле %q", uid, p.UID)
	}
	if need, err := e.NeedUID(path, p.UID); need || err != nil {
		t.Fatalf("NeedUID: %v %v", need, err)
	}
	for _, bad := range []string{"ABCDEF00000000000000000000000000", "short", strings.Repeat("g", 32), `"},"x":"`} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		data = []byte(strings.Replace(string(data), p.UID, strings.ReplaceAll(bad, `"`, `\"`), 1))
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
		q, err := salbum.Load(path)
		if err != nil {
			t.Fatalf("%q: %v", bad, err)
		}
		if q.UID != "" {
			t.Fatalf("негодный %q прочитан как %q", bad, q.UID)
		}
		p.UID = bad // следующий круг ищет в файле этот
	}
	n, err := NewEditor(model.New(), "")
	if err != nil {
		t.Fatal(err)
	}
	if _, uid := n.Ident(); !model.GoodUID(uid) {
		t.Fatalf("новый альбом без отпечатка: %q", uid)
	}
	if _, err := n.NeedUID("", n.p.UID); !errors.Is(err, ErrUIDChanged) {
		t.Fatalf("сверка без пути: %v", err)
	}
}

// Восстановление копии: копия другого проекта — её отпечаток, Ctrl+Z — прежний, Ctrl+Y — снова
// копии; копия без отпечатка — новый. Устаревший отпечаток в NeedUID и SaveUID — отказ, файл цел.
func TestEditorUIDRestore(t *testing.T) {
	dir := testkit.TempDir(t)
	p := model.New()
	p.UID = model.NewUID()
	path := filepath.Join(dir, "a.salbum")
	e := uidEditor(t, p, path)
	other := model.New()
	other.UID = model.NewUID()
	other.SetUIDAt(model.UIDTag(other.UID, path)) // копия файла этого пути (папка копий: его байты)
	other.Settings.Gap++
	if _, err := e.Restore(other); err != nil {
		t.Fatal(err)
	}
	if e.p.UID != other.UID {
		t.Fatalf("после восстановления %q, ждали %q", e.p.UID, other.UID)
	}
	if need, err := e.NeedUID(path, other.UID); !need || err != nil {
		t.Fatalf("NeedUID после восстановления: %v %v", need, err)
	}
	if _, err := e.NeedUID(path, p.UID); !errors.Is(err, ErrUIDChanged) {
		t.Fatalf("устаревший отпечаток: %v", err)
	}
	if _, err := e.SaveUID(path, p.UID); !errors.Is(err, ErrUIDChanged) || fileUID(t, path) != p.UID {
		t.Fatalf("SaveUID устаревшего: %v, в файле %q", err, fileUID(t, path))
	}
	if _, err := e.NeedUID(filepath.Join(dir, "b.salbum"), other.UID); !errors.Is(err, ErrUIDChanged) {
		t.Fatalf("другой путь: %v", err)
	}
	e.Undo()
	if e.p.UID != p.UID {
		t.Fatalf("Ctrl+Z: %q, ждали %q", e.p.UID, p.UID)
	}
	e.Redo()
	if e.p.UID != other.UID {
		t.Fatalf("Ctrl+Y: %q, ждали %q", e.p.UID, other.UID)
	}
	bare := model.New()
	if _, err := e.Restore(bare); err != nil {
		t.Fatal(err)
	}
	if u := e.p.UID; !model.GoodUID(u) || u == p.UID || u == other.UID {
		t.Fatalf("копия без отпечатка: %q", u)
	}
	// Холостое восстановление той же копии — без шага.
	n := len(e.undo)
	same := e.p.Clone()
	same.SetUIDAt(model.UIDTag(same.UID, path)) // та же копия из папки копий этого файла
	if _, err := e.Restore(same); err != nil {
		t.Fatal(err)
	}
	if len(e.undo) != n {
		t.Errorf("восстановление того же — шаг: %d → %d", n, len(e.undo))
	}
}

// «Сохранить как» другим проектом: новый отпечаток в проекте, стеках отмены и возврата и снимке
// пересборки (снимки чужой копии — свои); запись не удалась — всё прежнее.
func TestEditorSaveNew(t *testing.T) {
	dir := testkit.TempDir(t)
	p := model.New()
	p.UID = model.NewUID()
	path := filepath.Join(dir, "a.salbum")
	e := uidEditor(t, p, path)
	old := p.UID
	st := e.p.Settings
	st.Gap++
	e.SetSettings(st)
	st.Gap++
	e.SetSettings(st)
	e.Undo() // один шаг в redo
	foreign := model.New()
	foreign.UID = model.NewUID()
	e.undo = append([]Snap{{P: foreign}}, e.undo...) // снимок до восстановления чужой копии
	e.auto.base = e.p.Clone()
	if len(e.undo) != 2 || len(e.redo) != 1 {
		t.Fatalf("оснастка: %d/%d", len(e.undo), len(e.redo))
	}

	bad := filepath.Join(path, "b.salbum") // папка — файл альбома: запись не пройдёт
	if err := e.SaveNew(bad); err == nil {
		t.Fatal("запись внутрь файла прошла")
	}
	if e.p.UID != old || e.undo[1].P.UID != old || e.redo[0].P.UID != old || e.auto.base.UID != old || e.path != path {
		t.Fatalf("после неудачи: %q %q %q %q %q", e.p.UID, e.undo[1].P.UID, e.redo[0].P.UID, e.auto.base.UID, e.path)
	}

	to := filepath.Join(dir, "b.salbum")
	if err := e.SaveNew(to); err != nil {
		t.Fatal(err)
	}
	uid := e.p.UID
	if !model.GoodUID(uid) || uid == old {
		t.Fatalf("новый отпечаток %q", uid)
	}
	if e.undo[1].P.UID != uid || e.redo[0].P.UID != uid || e.auto.base.UID != uid {
		t.Fatalf("стеки: %q %q, пересборка %q", e.undo[1].P.UID, e.redo[0].P.UID, e.auto.base.UID)
	}
	if e.undo[0].P.UID != foreign.UID {
		t.Fatalf("снимок чужой копии сменил отпечаток: %q", e.undo[0].P.UID)
	}
	if fileUID(t, to) != uid || fileUID(t, path) != old {
		t.Fatalf("в файлах: %q %q", fileUID(t, to), fileUID(t, path))
	}
	if need, err := e.NeedUID(to, uid); need || err != nil {
		t.Fatalf("NeedUID после SaveNew: %v %v", need, err)
	}
	e.Undo()
	if e.p.UID != uid {
		t.Fatalf("Ctrl+Z после «Сохранить как» вернул прежний отпечаток %q", e.p.UID)
	}
}

// GoodUID на краях: только 32 знака [0-9a-f]; верхний регистр — негодный (NewUID пишет нижний).
func TestGoodUID(t *testing.T) {
	for s, want := range map[string]bool{
		"":                                    false,
		strings.Repeat("a", 31):               false,
		strings.Repeat("a", 33):               false,
		strings.Repeat("0", 32):               true,
		"0123456789abcdef0123456789abcdef":    true,
		"0123456789ABCDEF0123456789abcdef":    false,
		"0123456789abcdeg0123456789abcdef":    false,
		strings.Repeat("я", 16):               false, // 32 байта, не hex
		"0123456789abcdef0123456789abcde\x00": false,
	} {
		if got := model.GoodUID(s); got != want {
			t.Errorf("GoodUID(%q) = %v", s, got)
		}
	}
	a, b := model.NewUID(), model.NewUID()
	if !model.GoodUID(a) || a == b {
		t.Fatalf("NewUID: %q %q", a, b)
	}
}
