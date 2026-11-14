package app

import (
	"slices"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Пересчёт того же или старшего поколения, чем снимок, том закрыть не может: он мог начаться до
// оживления. Следующее поколение закрывает; число — без фото, которых в списке больше нет.
func TestTakeBackNeedsNewerGen(t *testing.T) {
	f := newVolFix(t)
	a := f.a
	refs := map[string]string{"a": "/x/a.jpg", "b": "/x/b.jpg", "c": "/x/c.jpg"}
	a.mu.Lock()
	a.missRefs = refs
	a.missBack = map[string]*backSnap{"E:": {gen: 4, paths: map[string]string{"a": "/x/a.jpg", "b": "/x/b.jpg", "c": "/x/old.jpg"}}}
	got4 := a.takeBack(4, refs, []string{"b"})
	_, kept := a.missBack["E:"]
	got5 := a.takeBack(5, refs, []string{"b"})
	left := len(a.missBack)
	a.mu.Unlock()
	if len(got4) != 0 || !kept {
		t.Fatalf("поколение снимка закрыло том: %+v (снимок цел: %v)", got4, kept)
	}
	// a — нашлось, b — осталось недостающим, c — путь сменился.
	if !slices.Equal(got5, []driveBack{{"E", 1}}) {
		t.Fatalf("следующее поколение: %+v", got5)
	}
	if left != 0 {
		t.Fatalf("снимок не закрыт: %d", left)
	}
}

// Проверка при открытии назвала фото тома пропавшими, но итог не прислала: том ожил — пересчёт
// нужен и при пустом снимке; тоста нет (полосы пользователь не видел).
func TestVolumeBackRecountWhenPending(t *testing.T) {
	f := newVolFix(t)
	f.seed(t, 0)
	entered, release := f.holdFirst()
	defer release()
	f.a.refreshMissingQuiet() // итог ещё не пришёл
	<-entered
	before := f.gen()
	f.a.volumeBack("E:")
	deadline := time.Now().Add(testkit.Limit(10 * time.Second))
	for f.gen() == before {
		if time.Now().After(deadline) {
			t.Fatal("пересчёт не начат при висящем")
		}
		time.Sleep(time.Millisecond)
	}
	release()
	f.waitChecked(t)
	if len(f.seen("drive:back")) != 0 {
		t.Fatalf("тост без полосы: %v", f.backs())
	}
}
