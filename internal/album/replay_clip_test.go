package album_test

// Сцена голден-реплея (replay_test.go): буфер обмена (099), фон альбома (098) и «где стоит» (098).
// Идёт в самом конце сценария: прежние строки эталона не сдвигаются.

import (
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// replayClipBg — новый «Авто»-разворот из трёх свежих фото (ленты ещё не знают их), закреплённый
// местом с кадром: «Вернуть авто» со своим кадром, вырезать/вставить/дубль через буфер,
// «где стоит» и сведения о файле, затем фон альбома — узор, своё фото, прозрачность серией,
// «Не показывать здесь» и снятие.
func replayClipBg(r *replay) {
	e := r.e
	fresh := []model.PhotoInfo{replayPhoto(31, 'h', model.XmpMarks{}), replayPhoto(32, 'v', model.XmpMarks{}), replayPhoto(33, 's', model.XmpMarks{})}
	r.ve("NewSpread:буфер")(e.NewSpread(-1, fresh))
	sid := r.v.Spreads[len(r.v.Spreads)-1].ID
	a, b := fresh[0].ID, fresh[1].ID

	// Место фото a закрепляет разворот (ручной); свой кадр — то, что «Вернуть авто» может сохранить.
	r.do("SetPlaces:буфер", e.SetPlaces(sid, []model.PlaceBox{{ID: a, X: 20, Y: 20, W: 136.5, H: 81}}, ""))
	r.do("SetFrame:буфер", e.SetFrame(a, 1.3, 0.2, -0.1))
	n := e.CropsToKeep(sid)
	if n < 1 {
		r.t.Fatalf("CropsToKeep: %d, ждали правленое фото", n)
	}
	r.read("CropsToKeep", []any{n, e.CropsToKeep("нет")})
	r.ve("BackToAuto")(e.BackToAuto(sid, "", true))
	r.ve("BackToAuto:холостая")(e.BackToAuto(sid, "", true))
	r.do("SetGrid:буфер", e.SetGrid(sid, true))

	// Буфер: копировать — чтение, вырезать (фото уходит с места), вставить обратно, дубль.
	sel := model.ClipSel{Refs: []string{a}}
	c, ok := e.TakeClip(sid, sel)
	if !ok {
		r.t.Fatal("TakeClip: нечего копировать")
	}
	_, none := e.TakeClip(sid, model.ClipSel{})
	r.read("TakeClip", []any{c.Src == sid, len(c.Places), len(c.Auto), ok, none})
	v, cut, ok, err := e.CutClip(sid, sel)
	if !ok || err != nil {
		r.t.Fatalf("CutClip: %v %v", ok, err)
	}
	// ID листа-источника случаен — в шаг идёт только «источник — этот лист».
	r.edit("CutClip", v, []any{cut.Src == sid, ok}, err)
	v, _, ok, err = e.CutClip(sid, model.ClipSel{})
	r.edit("CutClip:холостая", v, ok, err)
	r.voe("PasteClip")(e.PasteClip(&cut, model.PasteReq{Sheet: sid}))
	r.voe("PasteClip:повтор")(e.PasteClip(&cut, model.PasteReq{Sheet: sid})) // фото уже на месте — вопрос без правки
	r.voe("DuplicateClip")(e.DuplicateClip(sid, model.ClipSel{Refs: []string{b}}))

	// «Где стоит» и сведения о файле: фото на месте, дубль (копия), неизвестный ID.
	r.read("WhereUsed", e.WhereUsed([]string{a, b, "нет"}))
	for _, id := range []string{a, fresh[2].ID, "нет"} {
		ref, found := e.FileRef(id)
		r.read("FileRef", []any{ref, found})
	}

	// Фон альбома: узор на разворот, своё фото на страницы, серия прозрачности, «без фона» и снятие.
	dots := model.BgSource{Pattern: &model.Pattern{Kind: "dots", Preset: "sa:dots"}, Color: "#f5efe6"}
	r.ve("SetAlbumBg")(e.SetAlbumBg(dots, false))
	r.ve("SetAlbumBg:холостая")(e.SetAlbumBg(dots, false))
	const key = "my:0a0b0c0d0e0f1011"
	own := model.PhotoInfo{ID: model.LibPhotoID(key), Path: qPath("Библиотека", "Фоны", "0a0b0c0d0e0f1011.jpg"), Lib: key, W: 3000, H: 2000, Size: 77}
	r.ve("SetAlbumBg:фото")(e.SetAlbumBg(model.BgSource{Photo: own}, true))
	r.do("SetAlbumBgFade", e.SetAlbumBgFade(0.3, "ab1"))
	r.do("SetAlbumBgFade:слияние", e.SetAlbumBgFade(0.5, "ab1"))
	r.do("SetNoAlbumBg", e.SetNoAlbumBg(sid, true))
	r.do("SetNoAlbumBg:показать", e.SetNoAlbumBg(sid, false))
	r.do("ClearAlbumBg", e.ClearAlbumBg())
	r.do("ClearAlbumBg:холостая", e.ClearAlbumBg())
}
