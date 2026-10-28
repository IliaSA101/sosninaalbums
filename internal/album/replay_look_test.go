package album_test

import (
	"reflect"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// replayLook — макет альбома (131 G4a): сухой прогон и отказ на собранном альбоме, затем серия на
// чистом листе (развороты и обложка сняты), повтор, смена макета; в конце Undo возвращает альбом.
func replayLook(r *replay) {
	e := r.e
	a, b := lookA(), lookB()
	o := model.LookOpts{Spreads: 9}
	plan, err := e.PlanLook(lkRefA, a, nil, o, nil)
	r.read("PlanLook:собранный", []any{plan, errText(err)})
	v, plan, err := e.ApplyLook(lkRefA, a, nil, o, nil)
	r.edit("ApplyLook:собранный", v, plan, err)

	ids := make([]string, 0, len(r.v.Spreads))
	for _, s := range r.v.Spreads {
		ids = append(ids, s.ID)
	}
	r.do("DeleteSpreads:макет", e.DeleteSpreads(ids))
	r.do("DeleteCover:макет", e.DeleteCover())
	plan, err = e.PlanLook(lkRefA, a, nil, o, nil)
	r.read("PlanLook", []any{plan, errText(err)})
	v, plan, err = e.ApplyLook(lkRefA, a, nil, o, nil)
	r.edit("ApplyLook", v, plan, err)
	v, plan, err = e.ApplyLook(lkRefA, a, nil, o, nil)
	r.edit("ApplyLook:повтор", v, plan, err)
	v, plan, err = e.ApplyLook(lkRefB, b, lkPrev(a), model.LookOpts{Spreads: 6}, nil)
	r.edit("ApplyLook:смена", v, plan, err)
	for range 4 { // смена, первая постановка, снятие обложки и развороты
		r.do("Undo:макет", e.Undo())
	}
	replayLookFull(r, a, b)
}

// replayLookFull — макет на собранном альбоме (G4b): серией он не ставится, «Фоны по серии» и
// «Добавить», шаг на лист и разворот по серии, смена формата с макетом и без.
func replayLookFull(r *replay, a, b model.AlbumTpl) {
	e := r.e
	if _, err := e.PlanLook(lkRefA, a, nil, model.LookOpts{Mode: model.LookSeries, Spreads: 3}, nil); err != nil {
		r.t.Fatal(err)
	}
	for _, mode := range []string{model.LookBg, model.LookAdd} {
		o := model.LookOpts{Mode: mode, Spreads: 2}
		plan, err := e.PlanLook(lkRefA, a, nil, o, nil)
		r.read("PlanLook:"+mode, []any{plan, errText(err)})
		v, plan, err := e.ApplyLook(lkRefA, a, nil, o, nil)
		r.edit("ApplyLook:"+mode, v, plan, err)
	}
	v, plan, err := e.ApplyLook(lkRefB, b, lkPrev(a), model.LookOpts{Mode: model.LookBg}, nil)
	r.edit("ApplyLook:фоны другого макета", v, plan, err)
	sid := r.v.Spreads[0].ID
	gp, err := e.PlanLookStep(sid, 3, lkRefB, b, nil)
	r.read("PlanLookStep", []any{gp, errText(err)})
	v, gp, miss, err := e.ApplyLookStep(sid, 3, lkRefB, b, nil)
	r.edit("ApplyLookStep", v, []any{gp, miss}, err)
	v, gp, miss, err = e.ApplyLookStep(sid, 0, lkRefB, b, nil)
	r.edit("ApplyLookStep:обложка", v, []any{gp, miss}, err)
	v, err = e.NewLookSpread(b, nil)
	r.edit("NewLookSpread", v, nil, err)
	st := r.v.Settings
	st.PageW = 200
	r.do("SetSettingsLook", e.SetSettingsLook(st, &b))
	st.PageW = 300
	r.do("SetSettingsLook:обратно", e.SetSettingsLook(st, nil))
	// Библиотека макетов (G6a): поиск и «есть на ПК» подключены — Found в виде, SetSettings сам вписывает макет.
	e.SetLookHost(album.LookHost{Find: func(model.LookRef) *model.AlbumTpl { return &b }, Has: func(model.LookRef) bool { return true }})
	r.read("SetLookHost", e.View().Look)
	r.read("Look", e.Look())
	r.read("IsSheet", e.IsSheet())
	v, gp, miss, err = e.ApplyLookStep(sid, 3, lkRefB, b, nil) // места первого разворота — ровно по шагу: нетронутые
	r.edit("ApplyLookStep:перед форматом", v, []any{gp, miss}, err)
	st.PageW = 220
	// Проба: с подключённой библиотекой места макетного разворота вписаны по LookBoxes, без неё растянуты —
	// строка «SetSettings:поиск макета» ниже фиксирует именно первое.
	probe := func(host bool) []model.PlaceView {
		q, _ := e.Snapshot()
		pe, err := album.NewEditor(q, "")
		if err != nil {
			r.t.Fatal(err)
		}
		if host {
			pe.SetLookHost(album.LookHost{Find: func(model.LookRef) *model.AlbumTpl { return &b }})
		}
		return pe.SetSettings(st).Spreads[0].Places
	}
	if with, without := probe(true), probe(false); reflect.DeepEqual(with, without) {
		r.t.Fatal("SetSettings с библиотекой макетов не отличается от растяжения мест")
	}
	r.do("SetSettings:поиск макета", e.SetSettings(st))
	replayLookCover(r)
}

// replayLookCover — обложка по макету (G5): шаг 0 на обложку (план и правка), обложка снята и
// создана заново с оформлением макета.
func replayLookCover(r *replay) {
	e := r.e
	c := lkCoverTpl()
	gp, err := e.PlanLookStep(model.CoverID, 0, lkRefB, c, nil)
	r.read("PlanLookStep:обложка макета", []any{gp, errText(err)})
	v, gp, miss, err := e.ApplyLookStep(model.CoverID, 0, lkRefB, c, nil)
	r.edit("ApplyLookStep:на обложку", v, []any{gp, miss}, err)
	r.do("DeleteCover:перед CreateCoverLook", e.DeleteCover())
	lkHost(e, c)
	v, styled, err := e.CreateCoverLook("", nil, nil, e.Look())
	r.edit("CreateCoverLook", v, styled != nil, err)
}

func errText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
