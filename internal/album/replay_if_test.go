package album_test

// Сцена голден-реплея (replay_test.go): правки агента «если всё ещё то же» (162 М4). У каждого
// …If два шага: X:да (rev вида совпал — правка прошла) и X:нет (rev на 1 меньше — ErrRevChanged,
// проект, rev и отмена прежние); AutoBuildIf:поздно — отменённый заранее ctx (ErrLate), без часов:
// иначе TestEditorReplayStable был бы нестабилен. Идёт в самом конце сценария: прежние строки
// эталона не сдвигаются. MarkUnsaved в реплее нет: dirty без rev — шаг ничего не записал бы.

import (
	"encoding/json"
	"errors"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/layers"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// ifPair — шаги name:да и name:нет. call зовёт правку с условием x и отдаёт её вид и прочий
// результат; «да» обязана изменить альбом (холостая ничего бы не показала), «нет» — отказать
// ErrRevChanged.
func (r *replay) ifPair(name string, call func(x album.Expect) (model.View, any, error)) {
	r.t.Helper()
	rev := r.e.View().Rev
	v, out, err := call(album.Expect{Rev: rev})
	if err != nil || v.Rev <= rev {
		r.t.Fatalf("%s:да: rev %d → %d, %v", name, rev, v.Rev, err)
	}
	data, jerr := json.Marshal(out)
	if jerr != nil {
		r.t.Fatalf("%s:да: %v", name, jerr)
	}
	r.edit(name+":да", v, json.RawMessage(r.norm(data)), nil)
	v, _, err = call(album.Expect{Rev: v.Rev - 1})
	if !errors.Is(err, album.ErrRevChanged) {
		r.t.Fatalf("%s:нет: %v", name, err)
	}
	r.edit(name+":нет", v, nil, err)
}

// replayIf — все одиннадцать …If и чтение Families.
func replayIf(r *replay) {
	e := r.e
	var feed []model.PhotoInfo
	for i, k := range []byte("hvshvh") {
		feed = append(feed, replayPhoto(60+i, k, model.XmpMarks{}))
	}
	o := model.AutoOpts{Spreads: 2, MaxPer: 64}
	r.ifPair("AutoBuildIf", func(x album.Expect) (model.View, any, error) {
		v, err := e.AutoBuildIf(feed, -1, o, x)
		return v, nil, err
	})
	sp := r.v.Spreads[len(r.v.Spreads)-2:]
	ids := []string{sp[0].ID, sp[1].ID}
	v, err := e.AutoBuildIf(feed, -1, o, album.Expect{Rev: r.v.Rev, Ctx: canceled()})
	if !errors.Is(err, album.ErrLate) {
		r.t.Fatalf("AutoBuildIf:поздно: %v", err)
	}
	r.edit("AutoBuildIf:поздно", v, nil, err)

	r.ifPair("VaryLayoutsIf", func(x album.Expect) (model.View, any, error) {
		v, err := e.VaryLayoutsIf(ids, model.VarietyMore, nil, feed, x)
		return v, nil, err
	})
	r.ifPair("SetMarksIf", func(x album.Expect) (model.View, any, error) {
		v, n, err := e.SetMarksIf(feed[:3], model.MarkPatch{Rating: edtest.Ip(4)}, x)
		return v, n, err
	})
	st := r.v.Settings
	st.Gap += 2
	r.ifPair("SetSettingsIf", func(x album.Expect) (model.View, any, error) {
		v, err := e.SetSettingsIf(st, x)
		return v, nil, err
	})
	r.ifPair("ApplyBackgroundToIf", func(x album.Expect) (model.View, any, error) {
		v, out, err := e.ApplyBackgroundToIf(ifTargets("spread", ids...), ifDots("dots"), false, x)
		return v, out, err
	})
	r.ifPair("SetAlbumBgIf", func(x album.Expect) (model.View, any, error) {
		v, out, err := e.SetAlbumBgIf(ifDots("stripes"), false, nil, x)
		return v, out, err
	})
	puts := []album.StickerPut{
		{Spread: ids[0], Page: 1, Src: ifStar, Anchor: layers.AnchorTopRight, Size: "s"},
		{Spread: ids[1], Page: 0, Src: ifStar, Anchor: layers.AnchorBottomLeft, Size: "m"},
	}
	r.ifPair("AddStickersIf", func(x album.Expect) (model.View, any, error) {
		v, got, err := e.AddStickersIf(puts, x)
		return v, got, err
	})

	// Обложка: прежняя снята — создаётся заново, затем фото на лицевую и бросок с макетом.
	if edtest.P(e).Cover != nil {
		r.do("DeleteCover:перед If", e.DeleteCover())
	}
	r.ve("CreateCover:для If")(e.CreateCover("", nil))
	r.ifPair("CoverPhotoIf", func(x album.Expect) (model.View, any, error) {
		v, place, changed, err := e.CoverPhotoIf(feed[3], x)
		return v, []any{place, changed}, err
	})
	pr := r.v.Settings
	pr.PageW, pr.PageH = 300, 200
	preset := model.CoverPresets()[0].ID
	r.ifPair("ApplyPrintIf", func(x album.Expect) (model.View, any, error) {
		v, err := e.ApplyPrintIf(pr, preset, x)
		return v, nil, err
	})
	r.do("DeleteCover:перед CreateCoverLookIf", e.DeleteCover())
	r.ifPair("CreateCoverLookIf", func(x album.Expect) (model.View, any, error) {
		v, look, place, err := e.CreateCoverLookIf("", &feed[4], nil, e.Look(), x)
		return v, []any{look != nil, place}, err
	})
	r.ifPair("UndoIfRev", func(x album.Expect) (model.View, any, error) {
		v, err := e.UndoIfRev(x)
		return v, nil, err
	})

	fams, rev := e.Families(append(ids, "нет"))
	r.read("Families", []any{fams, rev})
}
