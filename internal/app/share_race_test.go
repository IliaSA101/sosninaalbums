package app

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// 167 T3b: Bind и pinShare идут рядом со сменой альбома (retire) и восстановлением копии. Привязка
// записи не правка: файл пишет только автосохранение под saveMu, поэтому восстановленное
// содержимое на диске остаётся целым, а привязка прежнего альбома не попадает в чужой файл.

func spreadsInFile(t *testing.T, path string) int {
	t.Helper()
	p, err := salbum.Load(path)
	if err != nil {
		t.Fatalf("файл %s не читается: %v", filepath.Base(path), err)
	}
	return len(p.Spreads)
}

// Восстановление копии одновременно с привязками и записью отпечатка: в файле — содержимое копии.
func TestBindRacesRestore(t *testing.T) {
	a, file, _ := backupApp(t, 2)
	uid := mustUID(t, a)
	addSpread(t, a)
	a.flushSave() // копия: файл с двумя разворотами; в альбоме три
	list := backups(t, a)
	if len(list) != 1 || list[0].Spreads != 2 {
		t.Fatalf("копии: %+v", list)
	}
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Go(func() {
			_ = a.bindShare(file, uid, fmt.Sprintf("alb-%d", i)) // отказ допустим: альбом мог смениться
			if i%2 == 0 {
				_, _ = a.pinShare(file, uid, false)
			}
			a.autosave()
		})
	}
	wg.Go(func() {
		if _, err := a.RestoreBackup(list[0].Name); err != nil {
			t.Errorf("RestoreBackup: %v", err)
		}
	})
	wg.Wait()
	a.flushSave()
	a.saveTimer.Stop()
	if got := spreadsInFile(t, file); got != 2 {
		t.Errorf("в файле %d разворотов, ждали 2 (содержимое копии)", got)
	}
	if v := a.editor().View(); len(v.Spreads) != 2 {
		t.Errorf("в альбоме %d разворотов, ждали 2", len(v.Spreads))
	}
	for _, ref := range fileRefsOf(t, file) {
		if !model.GoodLinkRef(ref) {
			t.Errorf("негодная привязка в файле: %q", ref)
		}
	}
}

// Смена альбома (retire) одновременно с привязками прежнего: прежний файл записан целым, привязка
// прежнего отпечатка в файл нового не ложится.
func TestBindRacesRetire(t *testing.T) {
	a, file, _ := backupApp(t, 2)
	uid := mustUID(t, a)
	other := filepath.Join(testkit.TempDir(t), "Другой"+projectExt)
	ed := mustEditor(t, model.New(), "")
	if err := ed.Save(other); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Go(func() {
			_ = a.bindShare(file, uid, fmt.Sprintf("alb-%d", i))
			a.autosave()
		})
	}
	wg.Go(func() {
		if _, err := a.openPath(other); err != nil {
			t.Errorf("openPath: %v", err)
		}
	})
	wg.Wait()
	a.flushSave()
	a.saveTimer.Stop()
	if got := spreadsInFile(t, file); got != 2 {
		t.Errorf("прежний файл: %d разворотов, ждали 2", got)
	}
	if got := a.editor().Path(); got != other {
		t.Fatalf("текущий альбом %q, ждали %q", got, other)
	}
	if refs := fileRefsOf(t, other); len(refs) != 0 {
		t.Errorf("привязки прежнего альбома легли в чужой файл: %v", refs)
	}
}

// pinShare берёт saveMu: пока автосохранение или смена альбома держат его, запись отпечатка не
// начинается (ни в файле, ни в Editor ничего не меняется); а барьер выхода проверяется уже после
// замка — выход, наступивший за время ожидания, запись отменяет.
func TestPinShareUnderSaveMu(t *testing.T) {
	for _, stopWhileWaiting := range []bool{false, true} {
		a, file, _ := backupApp(t, 1)
		uid := mustUID(t, a)
		before, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		type res struct {
			uid string
			err error
		}
		got := make(chan res, 1)
		a.saveMu.Lock()
		go func() {
			nu, err := a.pinShare(file, uid, true)
			got <- res{nu, err}
		}()
		if !testkit.WaitBlocked(t, "(*App).pinShare", testkit.Limit(10*time.Second)) {
			a.saveMu.Unlock()
			t.Fatal("pinShare не встал на saveMu")
		}
		select {
		case r := <-got:
			a.saveMu.Unlock()
			t.Fatalf("pinShare прошёл под чужим saveMu: %+v", r)
		default:
		}
		if _, now := a.editor().Ident(); now != uid || diskUID(t, file) != uid {
			a.saveMu.Unlock()
			t.Fatalf("отпечаток сменился до замка: в альбоме %q, в файле %q", now, diskUID(t, file))
		}
		if stopWhileWaiting {
			a.stopped.Store(true)
		}
		a.saveMu.Unlock()
		r, ok := testkit.WaitCh(t, got, testkit.Limit(10*time.Second), "pinShare", nil)
		if !ok {
			t.Fatal("pinShare не завершился после отпускания saveMu")
		}
		after, _ := os.ReadFile(file)
		if stopWhileWaiting {
			if r.err == nil || string(after) != string(before) {
				t.Errorf("после выхода: err %v, файл изменён %v", r.err, string(after) != string(before))
			}
			continue
		}
		if _, now := a.editor().Ident(); r.err != nil || r.uid == uid || now != r.uid || diskUID(t, file) != r.uid {
			t.Errorf("после замка: %+v, в альбоме %q, в файле %q", r, now, diskUID(t, file))
		}
	}
}
