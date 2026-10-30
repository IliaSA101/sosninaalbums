package album_test

// Правки агента «если всё ещё то же» (149, design §2.1): rev и срок сверяются под замком Editor,
// в одном месте с записью. Отказ не трогает проект, не ставит шаг и не меняет rev.

import (
	"bytes"
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// snap — состояние для проверки «отказ ничего не тронул».
type snap struct {
	data []byte
	rev  int
	undo int
}

func take(e *album.Editor) snap {
	return snap{model.Marshal(edtest.P(e)), edtest.Rev(e), edtest.UndoLen(e)}
}

func (s snap) same(t *testing.T, e *album.Editor, what string) {
	t.Helper()
	if got := take(e); got.rev != s.rev || got.undo != s.undo || !bytes.Equal(got.data, s.data) {
		t.Fatalf("%s: rev %d → %d, шагов %d → %d, проект изменился: %v", what, s.rev, got.rev, s.undo, got.undo, !bytes.Equal(got.data, s.data))
	}
}

func canceled() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}

// TestAutoBuildIfOK: совпавший rev — та же сборка, что у AutoBuild (с точностью до ID), один шаг;
// Expect без Ctx срока не имеет.
func TestAutoBuildIfOK(t *testing.T) {
	e1, ph, _ := againAlbum(t)
	e2, _, _ := againAlbum(t)
	want := autoV(t, e1, ph, -1, 1, false)
	o := againOpts
	o.Variant = 1
	u := edtest.UndoLen(e2)
	got, err := e2.AutoBuildIf(ph, -1, o, album.Expect{Rev: edtest.Rev(e2)})
	if err != nil || len(got.Spreads) != len(want.Spreads) || edtest.UndoLen(e2) != u+1 {
		t.Fatalf("AutoBuildIf: %v, разворотов %d (ждали %d), шагов %d → %d", err, len(got.Spreads), len(want.Spreads), u, edtest.UndoLen(e2))
	}
	if !bytes.Equal(canonIDs(edtest.P(e1)), canonIDs(edtest.P(e2))) {
		t.Fatal("AutoBuildIf собрал не то, что AutoBuild")
	}
}

// TestAutobuildRevUnderLock: человек правит альбом между планом (вне замка) и записью — сверка
// под замком отказывает, правка человека остаётся, шага агента нет.
func TestAutobuildRevUnderLock(t *testing.T) {
	e, ph, _ := againAlbum(t)
	rev := edtest.Rev(e)
	edtest.SetBuildGap(func() {
		e.MoveSpread(edtest.P(e).Spreads[0].ID, 0)
		e.SetSettings(model.Settings{PageW: 300, PageH: 300, DPI: 300, Gap: 5, Margin: 10})
	})
	t.Cleanup(func() { edtest.SetBuildGap(nil) })
	before, n := take(e), len(edtest.P(e).Spreads)
	_, err := e.AutoBuildIf(ph, -1, againOpts, album.Expect{Rev: rev})
	if !errors.Is(err, album.ErrRevChanged) {
		t.Fatalf("правка за время плана: %v, ждали ErrRevChanged", err)
	}
	got := take(e)
	if got.undo <= before.undo || got.rev <= rev {
		t.Fatalf("правка человека не видна: шагов %d → %d, rev %d → %d", before.undo, got.undo, rev, got.rev)
	}
	if len(edtest.P(e).Spreads) != n || edtest.UndoLen(e) > before.undo+2 {
		t.Fatalf("сборка агента записана: разворотов %d → %d, шагов %d → %d", n, len(edtest.P(e).Spreads), before.undo, got.undo)
	}
}

// TestAutobuildAlbumSwapped: Editor закрыт (открыли другой альбом) — сборка, раскладки и отмена
// отказывают ErrRevChanged, а не молча «успех» (shut отдаёт вид без ошибки).
func TestAutobuildAlbumSwapped(t *testing.T) {
	e, ph, _ := againAlbum(t)
	autoV(t, e, ph, -1, 1, false)
	x := album.Expect{Rev: edtest.Rev(e)}
	ids := []string{edtest.P(e).Spreads[0].ID}
	e.Close()
	s := take(e)
	if _, err := e.AutoBuildIf(ph, -1, againOpts, x); !errors.Is(err, album.ErrRevChanged) {
		t.Fatalf("AutoBuildIf: %v", err)
	}
	if _, err := e.VaryLayoutsIf(ids, model.VarietyMore, nil, ph, x); !errors.Is(err, album.ErrRevChanged) {
		t.Fatalf("VaryLayoutsIf: %v", err)
	}
	if _, err := e.UndoIfRev(x); !errors.Is(err, album.ErrRevChanged) {
		t.Fatalf("UndoIfRev: %v", err)
	}
	s.same(t, e, "закрытый Editor")
}

// TestEditLateNoApply: срок вышел до замка или за время плана — ErrLate, проект, rev и шаги прежние.
func TestEditLateNoApply(t *testing.T) {
	e, ph, _ := againAlbum(t)
	rev := edtest.Rev(e)
	s := take(e)
	x := album.Expect{Rev: rev, Ctx: canceled()}
	if _, err := e.AutoBuildIf(ph, -1, againOpts, x); !errors.Is(err, album.ErrLate) {
		t.Fatalf("AutoBuildIf: %v", err)
	}
	s.same(t, e, "срок до сборки")
	// Срок вышел, пока считался план: тело ещё живо, запись — нет.
	ctx, cancel := context.WithCancel(context.Background())
	edtest.SetBuildGap(cancel)
	t.Cleanup(func() { edtest.SetBuildGap(nil) })
	if _, err := e.AutoBuildIf(ph, -1, againOpts, album.Expect{Rev: rev, Ctx: ctx}); !errors.Is(err, album.ErrLate) {
		t.Fatalf("срок за время плана: %v", err)
	}
	s.same(t, e, "срок за время плана")
	edtest.SetBuildGap(nil)
	// Срок важнее rev: опоздавшая правка по старому rev — «поздно».
	if _, err := e.AutoBuildIf(ph, -1, againOpts, album.Expect{Rev: rev - 1, Ctx: canceled()}); !errors.Is(err, album.ErrLate) {
		t.Fatalf("срок и rev: %v", err)
	}
	// Отмена тоже не применяется после срока.
	autoV(t, e, ph, -1, 1, false)
	s = take(e)
	if _, err := e.UndoIfRev(album.Expect{Rev: s.rev, Ctx: canceled()}); !errors.Is(err, album.ErrLate) {
		t.Fatalf("UndoIfRev: %v", err)
	}
	s.same(t, e, "отмена после срока")
}

// TestUndoIfRevAtomic: rev совпал — отменён ровно один шаг агента; человек успел править — отказ,
// его правка и история целы; два одновременных «Ctrl+Z» человека и отмена агента не отменяют
// лишнего (проверяется под -race).
func TestUndoIfRevAtomic(t *testing.T) {
	e, ph, base := againAlbum(t)
	u0 := edtest.UndoLen(e)
	autoV(t, e, ph, -1, 1, false)
	rev := edtest.Rev(e)
	if _, err := e.UndoIfRev(album.Expect{Rev: rev}); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(model.Marshal(edtest.P(e)), model.Marshal(base)) || edtest.UndoLen(e) != u0 || !e.View().CanRedo {
		t.Fatal("отмена агента не вернула альбом до сборки одним шагом")
	}
	// Человек правит после агента: rev ушёл.
	e, ph, _ = againAlbum(t)
	autoV(t, e, ph, -1, 1, false)
	rev = edtest.Rev(e)
	e.MoveSpread(edtest.P(e).Spreads[0].ID, 1)
	s := take(e)
	if _, err := e.UndoIfRev(album.Expect{Rev: rev}); !errors.Is(err, album.ErrRevChanged) {
		t.Fatalf("после правки человека: %v", err)
	}
	s.same(t, e, "отказ отмены")
	// Гонка: человек жмёт Ctrl+Z, агент отменяет свою правку по тому же rev.
	for range 50 {
		e, ph, _ = againAlbum(t)
		e.SetSettings(model.Settings{PageW: 300, PageH: 300, DPI: 300, Gap: 5, Margin: 10})
		u0 = edtest.UndoLen(e)
		autoV(t, e, ph, -1, 1, false)
		rev = edtest.Rev(e)
		var agentErr error
		var wg sync.WaitGroup
		wg.Add(2)
		go func() { defer wg.Done(); e.Undo() }()
		go func() { defer wg.Done(); _, agentErr = e.UndoIfRev(album.Expect{Rev: rev}) }()
		wg.Wait()
		// Агент успел первым — человек отменил ещё и шаг до сборки; человек первым — агент отказан.
		if n := edtest.UndoLen(e); (agentErr == nil && n != u0-1) || (agentErr != nil && (n != u0 || !errors.Is(agentErr, album.ErrRevChanged))) {
			t.Fatalf("гонка: ошибка агента %v, шагов %d (до сборки %d)", agentErr, n, u0)
		}
	}
}

// TestVaryLayoutsIf: раскладки со сверкой — результат VaryLayouts, один шаг; нечего менять —
// холостая без ошибки; чужой rev и срок — отказ без записи.
func TestVaryLayoutsIf(t *testing.T) {
	e1, ph, _ := againAlbum(t)
	e2, _, _ := againAlbum(t)
	autoV(t, e1, ph, -1, 1, false)
	autoV(t, e2, ph, -1, 1, false)
	ids := func(e *album.Editor) []string {
		var out []string
		for _, s := range edtest.P(e).Spreads {
			out = append(out, s.ID)
		}
		return out
	}
	// Одинаковые ID у двух альбомов нет — сравниваем число изменённых разворотов через тот же вход.
	rev, u := edtest.Rev(e2), edtest.UndoLen(e2)
	v, err := e2.VaryLayoutsIf(ids(e2), model.VarietyMore, []string{"air"}, ph, album.Expect{Rev: rev})
	if err != nil {
		t.Fatal(err)
	}
	e1.VaryLayouts(ids(e1), model.VarietyMore, []string{"air"}, ph)
	if !bytes.Equal(canonIDs(edtest.P(e1)), canonIDs(edtest.P(e2))) {
		t.Fatal("VaryLayoutsIf дал не то, что VaryLayouts")
	}
	if v.Rev == rev || edtest.UndoLen(e2) != u+1 {
		t.Fatalf("раскладки не изменены или шагов не один: rev %d → %d, шагов %d → %d", rev, v.Rev, u, edtest.UndoLen(e2))
	}
	// Холостая: пустой список разворотов — менять нечего.
	s := take(e2)
	if _, err := e2.VaryLayoutsIf(nil, model.VarietyMore, nil, ph, album.Expect{Rev: s.rev}); err != nil {
		t.Fatal(err)
	}
	s.same(t, e2, "холостая")
	// Чужой rev и вышедший срок.
	if _, err := e2.VaryLayoutsIf(ids(e2), model.VarietyMore, nil, ph, album.Expect{Rev: s.rev - 1}); !errors.Is(err, album.ErrRevChanged) {
		t.Fatalf("чужой rev: %v", err)
	}
	if _, err := e2.VaryLayoutsIf(ids(e2), model.VarietyMore, nil, ph, album.Expect{Rev: s.rev, Ctx: canceled()}); !errors.Is(err, album.ErrLate) {
		t.Fatalf("срок: %v", err)
	}
	s.same(t, e2, "отказы")
}
