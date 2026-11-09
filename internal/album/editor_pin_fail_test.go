package album

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// 167 T3b: запись не удалась — отпечаток, привязки и dirty прежние (откат выдачи отпечатка).

// blockPath — путь записи, куда файл не ляжет: на месте файла стоит непустая папка (rename на неё
// отказывает на любой ОС, права не нужны).
func blockPath(t *testing.T, path string) {
	t.Helper()
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(path, "x"), 0o755); err != nil {
		t.Fatal(err)
	}
}

// pinState — то, что отказ записи менять не вправе.
type pinState struct {
	uid   string
	refs  int
	dirty bool
}

func pinStateOf(e *Editor) pinState {
	_, uid := e.Ident()
	return pinState{uid, len(e.ShareRefs()), e.dirty}
}

func TestSaveFileUIDWriteFailKeeps(t *testing.T) {
	path := filepath.Join(testkit.TempDir(t), "a.salbum")
	e := uidEditor(t, model.New(), path)
	path, uid := e.Ident()
	e.dirty = true // несохранённая правка: отказ записи её не снимает
	before := pinStateOf(e)
	blockPath(t, path)
	if _, err := e.SaveFileUID(path, uid); err == nil {
		t.Fatal("запись в занятый путь прошла")
	}
	if got := pinStateOf(e); got != before {
		t.Errorf("после отказа %+v, было %+v", got, before)
	}
	if need, err := e.NeedFileUID(path, uid); !need || err != nil {
		t.Errorf("запись по-прежнему нужна: %v %v", need, err)
	}
}

func TestSaveFreshWriteFailKeeps(t *testing.T) {
	path := filepath.Join(testkit.TempDir(t), "a.salbum")
	e := uidEditor(t, model.New(), path)
	path, uid := e.Ident()
	if err := e.SetRef(path, uid, model.LinkRef(uid, "alb")); err != nil {
		t.Fatal(err)
	}
	e.dirty = true // несохранённая правка: отказ записи её не снимает
	before := pinStateOf(e)
	blockPath(t, path)
	if nu, err := e.SaveFresh(path, uid); err == nil || nu != "" {
		t.Fatalf("запись в занятый путь прошла: %q %v", nu, err)
	}
	if got := pinStateOf(e); got != before || before.refs != 1 {
		t.Errorf("после отказа %+v, было %+v", got, before)
	}
	// Откат не оставил следов: после починки пути тот же вызов проходит.
	if err := os.RemoveAll(path); err != nil {
		t.Fatal(err)
	}
	if nu, err := e.SaveFresh(path, uid); err != nil || nu == uid {
		t.Errorf("после починки: %q %v", nu, err)
	}
}

func TestSaveOverWriteFailKeeps(t *testing.T) {
	dir := testkit.TempDir(t)
	a := filepath.Join(dir, "a.salbum")
	e := uidEditor(t, model.New(), a)
	a, uid := e.Ident()
	if err := e.SetRef(a, uid, model.LinkRef(uid, "alb")); err != nil {
		t.Fatal(err)
	}
	e.dirty = true // несохранённая правка: отказ записи её не снимает
	before := pinStateOf(e)
	// Другой проект: цель — непустая папка; выданный отпечаток откатывается.
	target := filepath.Join(dir, "t.salbum")
	if err := os.MkdirAll(filepath.Join(target, "x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if fresh, err := e.SaveOver(target); err == nil || fresh {
		t.Fatalf("другой проект: запись в занятый путь прошла: %v %v", fresh, err)
	}
	if got := pinStateOf(e); got != before {
		t.Errorf("другой проект: %+v, было %+v", got, before)
	}
	// Тот же путь: обычная запись, ошибка наружу, отпечаток и привязки на месте.
	blockPath(t, a)
	if fresh, err := e.SaveOver(a); err == nil || fresh {
		t.Fatalf("тот же путь: запись в занятый путь прошла: %v %v", fresh, err)
	}
	if got := pinStateOf(e); got != before {
		t.Errorf("тот же путь: %+v, было %+v", got, before)
	}
}

// Устаревший отпечаток или чужой путь: NeedFileUID и SaveFileUID отказывают ErrUIDChanged, файл цел.
func TestFileUIDStaleIdentRefused(t *testing.T) {
	dir := testkit.TempDir(t)
	path := filepath.Join(dir, "a.salbum")
	e := uidEditor(t, model.New(), path)
	path, uid := e.Ident()
	for _, c := range []struct{ path, uid string }{
		{path, model.NewUID()},
		{filepath.Join(dir, "other.salbum"), uid},
		{"", uid},
	} {
		if _, err := e.NeedFileUID(c.path, c.uid); !errors.Is(err, ErrUIDChanged) {
			t.Errorf("NeedFileUID(%q): %v", c.path, err)
		}
		if _, err := e.SaveFileUID(c.path, c.uid); !errors.Is(err, ErrUIDChanged) {
			t.Errorf("SaveFileUID(%q): %v", c.path, err)
		}
	}
	if fileUID(t, path) == uid {
		t.Error("отказ записал отпечаток в файл")
	}
}

// Отпечаток уже в файле: SaveFileUID ничего не пишет (файл не переписывается).
func TestSaveFileUIDAlreadyOnDisk(t *testing.T) {
	path := filepath.Join(testkit.TempDir(t), "a.salbum")
	e := uidEditor(t, model.New(), path)
	path, uid := e.Ident()
	if _, err := e.SaveFileUID(path, uid); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := e.SaveFileUID(path, uid); err != nil || got != uid {
		t.Fatalf("повторная запись: %q %v", got, err)
	}
	if after, _ := os.ReadFile(path); string(after) != string(before) {
		t.Error("повторная SaveFileUID переписала файл")
	}
}
