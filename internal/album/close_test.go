package album_test

import (
	"bytes"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// closeEd — альбом [a b] [c] с отменой и возвратом, своей отметкой, подборкой и листом своего шаблона «Свой»
// (лист собран мимо Editor: SyncOwnNames нужен только Vig.Own), записанный в path.
func closeEd(t *testing.T, path string) (*album.Editor, string) {
	t.Helper()
	e, s1, _ := edtest.NewTwo(t)
	edtest.SetMarks(t, e, edtest.Photos("a"), model.MarkPatch{Rating: edtest.Ip(3)})
	if _, err := e.NewBin("Обязательно", edtest.Photos("a")); err != nil {
		t.Fatal(err)
	}
	e.SetSpreadName(s1, "раз")
	e.SetSpreadName(s1, "два")
	e.Undo()
	sp := edtest.P(e).Spreads[1]
	sp.Kind = model.KindVignette
	sp.Vig = &model.Vignette{Tpl: model.VigOwnPrefix + "t1", Own: &model.VigShape{Name: "Свой"}}
	if err := e.Save(path); err != nil {
		t.Fatal(err)
	}
	return e, s1
}

// После Close правки шести входов (087-5) — холостые без ошибки: альбом, rev, dirty, отмена и
// отметки прежние, в log.txt строка на каждую; запись и чтения работают, второй Close холостой.
func TestEditorClose(t *testing.T) {
	dir := testkit.TempDir(t)
	e, s1 := closeEd(t, filepath.Join(dir, "a.salbum"))
	// Второй такой же, не закрытый: те же правки его меняют — значит, холостыми их делает Close.
	open, _ := closeEd(t, filepath.Join(dir, "b.salbum"))
	rename := []model.Template{{ID: "t1", Name: "Другой", Kind: model.TplVignette}}

	var logged bytes.Buffer
	log.SetOutput(&logged)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	data, rev, undo, redo := model.Marshal(edtest.P(e)), edtest.Rev(e), edtest.UndoLen(e), edtest.RedoLen(e)
	marks, bins := e.Marks(), e.View().Bins
	if undo == 0 || redo == 0 || len(marks) == 0 || edtest.Dirty(e) {
		t.Fatalf("заготовка: отмена %d, возврат %d, отметок %d, dirty %v", undo, redo, len(marks), edtest.Dirty(e))
	}
	e.Close()
	e.Close() // повторный — холостой, не виснет
	// Порядок — чтобы каждая правка меняла открытый: имена шаблонов до правки через settle (лист
	// собран мимо Editor), возврат сразу после отмены.
	edits := []struct {
		name string
		do   func(ed *album.Editor) (model.View, error)
	}{
		{"SyncOwnNames", func(ed *album.Editor) (model.View, error) {
			v, changed := ed.SyncOwnNames(rename)
			if changed != (ed == open) {
				t.Errorf("SyncOwnNames: changed %v (закрыт %v)", changed, ed == e)
			}
			return v, nil
		}},
		{"Undo", func(ed *album.Editor) (model.View, error) { return ed.Undo(), nil }},
		{"Redo", func(ed *album.Editor) (model.View, error) { return ed.Redo(), nil }},
		{"SetSpreadName", func(ed *album.Editor) (model.View, error) { return ed.SetSpreadName(s1, "поздно"), nil }},
		{"SetMarks", func(ed *album.Editor) (model.View, error) {
			return ed.SetMarks(edtest.Photos("b"), model.MarkPatch{Rating: edtest.Ip(5)})
		}},
		{"ResetMarks", func(ed *album.Editor) (model.View, error) { return ed.ResetMarks([]string{"a"}), nil }},
		// Подборки (083): ID у двух альбомов свои — берётся из вида (чтение работает и после Close).
		{"NewBin", func(ed *album.Editor) (model.View, error) { return ed.NewBin("Запас", edtest.Photos("b")) }},
		{"SetBin", func(ed *album.Editor) (model.View, error) {
			return ed.SetBin(ed.View().Bins[0].ID, edtest.Photos("c"), true)
		}},
		{"RenameBin", func(ed *album.Editor) (model.View, error) { return ed.RenameBin(ed.View().Bins[0].ID, "Другое") }},
		{"DeleteBin", func(ed *album.Editor) (model.View, error) { return ed.DeleteBin(ed.View().Bins[0].ID), nil }},
		{"Restore", func(ed *album.Editor) (model.View, error) { return ed.Restore(model.New()) }},
	}
	for _, ed := range edits {
		name, edit := ed.name, ed.do
		openRev := edtest.Rev(open)
		v, err := edit(e)
		if err != nil || v.Rev != rev || v.Dirty {
			t.Errorf("%s после Close: rev %d→%d, dirty %v, ошибка %v", name, rev, v.Rev, v.Dirty, err)
		}
		if _, err := edit(open); err != nil || edtest.Rev(open) == openRev {
			t.Errorf("%s открытого: rev %d→%d, ошибка %v — правка не меняет альбом, проверка пуста", name, openRev, edtest.Rev(open), err)
		}
	}
	if !bytes.Equal(model.Marshal(edtest.P(e)), data) || edtest.Rev(e) != rev || edtest.Dirty(e) ||
		edtest.UndoLen(e) != undo || edtest.RedoLen(e) != redo || !reflect.DeepEqual(e.Marks(), marks) ||
		!reflect.DeepEqual(e.View().Bins, bins) {
		t.Fatalf("закрытый альбом изменился: rev %d→%d, dirty %v, отмена %d→%d, возврат %d→%d",
			rev, edtest.Rev(e), edtest.Dirty(e), undo, edtest.UndoLen(e), redo, edtest.RedoLen(e))
	}
	if got := strings.Count(logged.String(), "правка прежнего альбома после смены отклонена"); got != len(edits) {
		t.Errorf("строк в журнале %d, правок %d:\n%s", got, len(edits), logged.String())
	}

	// Запись и чтения закрытого: Save в новый путь — тот же файл, что до Close (круг Save → Load).
	if v := e.View(); v.Rev != rev || len(v.Spreads) != 2 {
		t.Fatalf("View: rev %d, разворотов %d", v.Rev, len(v.Spreads))
	}
	if q, _ := e.Snapshot(); q == nil || len(q.Spreads) != 2 {
		t.Fatal("Snapshot закрытого пуст")
	}
	again := filepath.Join(dir, "again.salbum")
	if err := e.Save(again); err != nil {
		t.Fatal(err)
	}
	before, err1 := os.ReadFile(filepath.Join(dir, "a.salbum"))
	after, err2 := os.ReadFile(again)
	// Печать отпечатка переносится как есть (146 T4-9): файл байт в байт.
	if err1 != nil || err2 != nil || !bytes.Equal(before, after) {
		t.Fatalf("Save после Close пишет другое (%v, %v):\n%s\n---\n%s", err1, err2, before, after)
	}
	q, err := salbum.Load(again)
	if err != nil {
		t.Fatal(err)
	}
	if len(q.Spreads) != 2 || q.Spreads[0].Name != "раз" || len(q.Loaded().Marks) != 1 {
		t.Fatalf("Save → Load: разворотов %d, имя %q, отметок %d", len(q.Spreads), q.Spreads[0].Name, len(q.Loaded().Marks))
	}
}

// Close ждёт правку, которая держит замок (и очередь к нему перед собой): иначе правка,
// начатая до смены альбома, ушла бы мимо записи.
func TestEditorCloseWaitsEdit(t *testing.T) {
	e, s1, _ := edtest.NewTwo(t)
	mu := edtest.Mu(e)
	mu.Lock()
	done := make(chan struct{})
	go func() {
		e.Close()
		close(done)
	}()
	// Замок держим до конца проверки: Close должен встать на нём (стоящая горутина — доказательство
	// «ждёт», без паузы-угадайки); вернувшийся Close — регресс.
	if !testkit.WaitBlocked(t, "(*Editor).Close", testkit.Limit(3*time.Second)) {
		mu.Unlock()
		t.Fatal("Close не встал на замок правки (вернулся, не дождавшись)")
	}
	select {
	case <-done:
		mu.Unlock()
		t.Fatal("Close не ждёт замок правки")
	default:
	}
	mu.Unlock()
	select {
	case <-done:
	case <-time.After(testkit.Limit(5 * time.Second)):
		t.Fatal("Close не вернулся после отпускания замка")
	}
	if v := e.SetSpreadName(s1, "после"); v.Spreads[0].Name != "" {
		t.Fatalf("правка после Close применена: %q", v.Spreads[0].Name)
	}
}
