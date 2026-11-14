package album_test

// Сцена голден-реплея (replay_test.go): «Весь альбом заново» (107). Идёт в самом конце сценария:
// прежние строки эталона не сдвигаются.

import (
	"fmt"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// replayAlbum107 — три новых «Авто»-разворота из свежих фото и одно свободное в ленте; план
// режима альбома (чтение), пересборка всего альбома (якоря сценария остаются на местах) и её
// пересборка другим вариантом тем же шагом отмены; Undo возвращает альбом до первой пересборки.
func replayAlbum107(r *replay) {
	e := r.e
	var feed []model.PhotoInfo
	for i, k := range []byte("hvshvhsvh") {
		feed = append(feed, replayPhoto(41+i, k, model.XmpMarks{}))
	}
	r.ve("NewSpread:альбом")(e.NewSpread(-1, feed[0:3]))
	r.ve("NewSpread:альбом 2")(e.NewSpread(-1, feed[3:6]))
	r.ve("NewSpread:альбом 3")(e.NewSpread(-1, feed[6:8])) // feed[8] — свободное
	o := model.AutoOpts{Spreads: 1, Auto: true, Variety: model.VarietyNormal, Album: true}
	pl, err := e.AutoPlan(feed, o)
	if err != nil || pl.Album == nil || pl.Album.Spreads < 3 || pl.Album.Free != 1 {
		r.t.Fatalf("AutoPlan:альбом: %+v %v", pl, err)
	}
	r.read("AutoPlan:альбом", []any{pl, fmt.Sprint(err)})
	o.Spreads = pl.Album.Spreads
	r.ve("AutoBuild:альбом")(e.AutoBuild(feed, -1, o))
	o.Again, o.Variant = true, 2
	r.ve("AutoBuild:альбом-again")(e.AutoBuild(feed, -1, o))
	r.do("Undo:альбом", e.Undo())
}
