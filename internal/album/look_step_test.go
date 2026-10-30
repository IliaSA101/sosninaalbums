package album_test

import (
	"errors"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// «Поставить на разворот N» (131 design §4, G4b): шаг на текущий лист целиком, как плитка 067.

// lkStepEd — редактор с разворотом из трёх фото на «Авто», названием и своей надписью.
func lkStepEd(t *testing.T) (*album.Editor, string) {
	t.Helper()
	e := lkEditor(t)
	v, err := e.NewSpread(-1, edtest.Photos("pa", "pb", "pc"))
	if err != nil {
		t.Fatal(err)
	}
	sid := v.Spreads[0].ID
	e.SetSpreadName(sid, "Моя глава")
	edtest.AddText(t, e, sid, 0)
	return e, sid
}

func TestApplyLookStep(t *testing.T) {
	e, sid := lkStepEd(t)
	json0, rev, undo := lkJSON(e), e.View().Rev, edtest.UndoLen(e)
	want, err := e.PlanLookStep(sid, 3, lkRefA, lookA(), nil)
	if err != nil || e.View().Rev != rev || lkJSON(e) != json0 {
		t.Fatalf("PlanLookStep: %v, изменил проект", err)
	}
	_, plan, missing, err := e.ApplyLookStep(sid, 3, lkRefA, lookA(), nil)
	if err != nil || plan != want || plan.Left != 1 || missing != 0 {
		t.Fatalf("план %+v, ждали %+v: %v", plan, want, err)
	}
	s := edtest.P(e).Spreads[0]
	if len(s.Grid) != 2 || s.Grid[0].Photo != "pa" || s.Grid[1].Photo != "pb" || s.LookStep != 3 || s.Name != "Моя глава" || len(s.Texts) != 1 {
		t.Fatalf("лист: мест %d, шаг %d, название %q, надписей %d", len(s.Grid), s.LookStep, s.Name, len(s.Texts))
	}
	if edtest.P(e).Look == nil || edtest.P(e).Look.Key != lkKeyA {
		t.Fatal("альбом без макета запоминает тот, чей шаг поставлен")
	}
	if edtest.UndoLen(e) != undo+1 {
		t.Fatalf("шагов отмены %d, ждали %d", edtest.UndoLen(e), undo+1)
	}
	// Повтор — холостая.
	rev, data := e.View().Rev, lkJSON(e)
	if _, p2, _, err := e.ApplyLookStep(sid, 3, lkRefA, lookA(), nil); err != nil || e.View().Rev != rev || lkJSON(e) != data || p2.Left != 0 {
		t.Fatalf("повтор: rev %d → %d, %v", rev, e.View().Rev, err)
	}
	e.Undo()
	if lkJSON(e) != json0 {
		t.Fatal("Undo не вернул лист")
	}
}

// Заготовка шага заменяет надписи листа (как у плитки 067: надписи шаблона есть); шаг чужого макета и «обычная плитка» номера не
// пишут; макет альбома не подменяется.
func TestApplyLookStepRefs(t *testing.T) {
	e, sid := lkStepEd(t)
	if _, _, _, err := e.ApplyLookStep(sid, 1, lkRefA, lookA(), nil); err != nil {
		t.Fatal(err)
	}
	if s := edtest.P(e).Spreads[0]; len(s.Texts) != 1 || !s.Texts[0].Draft || s.LookStep != 1 {
		t.Fatalf("надписей %d (одна заготовка), шаг %d", len(s.Texts), s.LookStep)
	}
	if _, _, _, err := e.ApplyLookStep(sid, 3, lkRefB, lookA(), nil); err != nil {
		t.Fatal(err)
	}
	if s := edtest.P(e).Spreads[0]; s.LookStep != 0 || edtest.P(e).Look.Key != lkKeyA {
		t.Fatalf("чужой макет: шаг %d, макет %q", s.LookStep, edtest.P(e).Look.Key)
	}
	e2, sid2 := lkStepEd(t)
	if _, _, _, err := e2.ApplyLookStep(sid2, 3, model.LookRef{}, lookA(), nil); err != nil {
		t.Fatal(err)
	}
	if s := edtest.P(e2).Spreads[0]; len(s.Grid) != 2 || s.LookStep != 0 || edtest.P(e2).Look != nil {
		t.Fatalf("плитка без ссылки: мест %d, шаг %d, макет %v", len(s.Grid), s.LookStep, edtest.P(e2).Look)
	}
}

// Отказы: номер шага вне макета, обложка (G5), пустой макет — без шага отмены; неизвестный лист —
// холостая; лист 031 берёт только оформление.
func TestApplyLookStepRefused(t *testing.T) {
	e, sid := lkStepEd(t)
	rev, undo, data := e.View().Rev, edtest.UndoLen(e), lkJSON(e)
	for _, c := range []struct {
		id   string
		step int
		tpl  model.AlbumTpl
		want error
	}{
		{sid, 0, lookA(), album.ErrLookCover},
		{model.CoverID, 1, lookA(), album.ErrLookCover},
		{sid, 9, lookA(), album.ErrLookStep},
		{sid, -1, lookA(), album.ErrLookStep},
		{sid, 1, model.AlbumTpl{}, model.ErrTplEmpty},
	} {
		if _, _, _, err := e.ApplyLookStep(c.id, c.step, lkRefA, c.tpl, nil); !errors.Is(err, c.want) {
			t.Errorf("шаг %d: %v, ждали %v", c.step, err, c.want)
		}
		if _, err := e.PlanLookStep(c.id, c.step, lkRefA, c.tpl, nil); !errors.Is(err, c.want) {
			t.Errorf("план, шаг %d: %v, ждали %v", c.step, err, c.want)
		}
	}
	if e.View().Rev != rev || edtest.UndoLen(e) != undo || lkJSON(e) != data {
		t.Fatal("отказ изменил проект")
	}
	if _, _, _, err := e.ApplyLookStep("нет такого", 1, lkRefA, lookA(), nil); err != nil || e.View().Rev != rev || edtest.P(e).Look != nil {
		t.Fatalf("неизвестный лист: %v", err)
	}
	// Лист 031: места строит виньетка, шаг с местами даёт только оформление.
	if _, err := e.AddVignette(-1); err != nil {
		t.Fatal(err)
	}
	last := len(edtest.P(e).Spreads) - 1
	vid := edtest.P(e).Spreads[last].ID
	places := len(edtest.P(e).Spreads[last].Grid)
	if _, _, _, err := e.ApplyLookStep(vid, 3, lkRefA, lookA(), nil); err != nil {
		t.Fatal(err)
	}
	if s := edtest.P(e).Spreads[last]; len(s.Grid) != places || s.BgL != "#fff0f0" {
		t.Fatalf("лист 031: мест %d (было %d), фон %q", len(s.Grid), places, s.BgL)
	}
}
