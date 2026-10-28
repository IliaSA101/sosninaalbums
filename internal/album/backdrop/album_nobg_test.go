package backdrop_test

import (
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/backdrop"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// AllToAlbum без фона альбома холостая: свой фон и «Не показывать здесь» разворотов остаются.
func TestAllToAlbumWithoutAlbumBg(t *testing.T) {
	p := model.New()
	p.Spreads = []*model.Spread{{ID: "a", Photos: []string{}, NoAlbumBg: true}, {ID: "b", Photos: []string{}, BgL: "#102030"}}
	before := string(model.Marshal(p))
	backdrop.AllToAlbum(p)
	if string(model.Marshal(p)) != before {
		t.Fatal("без фона альбома AllToAlbum изменила проект")
	}
}
