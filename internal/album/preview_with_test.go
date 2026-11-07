package album_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/model/looktpl"
)

// previewTpl — страница с n местами в два ряда; фон — цвет.
func previewTpl(n int) looktpl.SpreadTpl {
	boxes := make([]model.Box, n)
	for i := range boxes {
		boxes[i] = model.Box{X: 0.05 + 0.3*float64(i%3), Y: 0.05 + 0.3*float64(i/3), W: 0.25, H: 0.25}
	}
	return looktpl.SpreadTpl{V: looktpl.TplV, Pages: 1, Ratio: 281.0 / 284, W: 281, H: 284,
		Page: []looktpl.TplBg{{Color: "#faf0e6"}}, Places: boxes}
}

// previewPhotos — k фото; ID — 12 знаков hex, как у файлов ленты.
func previewPhotos(k int) []model.PhotoInfo {
	out := make([]model.PhotoInfo, k)
	for i := range out {
		id := fmt.Sprintf("%012x", 0xa00+i)
		out[i] = model.PhotoInfo{ID: id, Path: edtest.Abs(id + ".jpg"), W: 3000, H: 2000}
	}
	return out
}

// seated — ID фото на местах листа по порядку Grid (он же порядок FillSeats).
func seated(p *model.Project) []string {
	var out []string
	for _, pl := range p.Spreads[0].Grid {
		out = append(out, pl.Photo)
	}
	return out
}

// sameBytes — сериализация проекта и вида с ID листа и мест, заменёнными порядковыми (они случайны
// на каждый вызов): остальное сравнивается побайтно.
func sameBytes(p *model.Project, v model.View) []byte {
	out := model.Marshal(p)
	b, _ := json.Marshal(v)
	out = append(out, b...)
	s := p.Spreads[0]
	out = bytes.ReplaceAll(out, []byte(s.ID), []byte("SHEET"))
	for i, pl := range s.Grid {
		out = bytes.ReplaceAll(out, []byte(pl.ID), fmt.Appendf(nil, "PLACE%d", i))
	}
	return out
}

// TestPreviewTplWithPhotos: места по FillSeats, лишние фото не мешают, повтор копиями «ID-n»
// закрывает все места, nil и пусто — прежний PreviewTpl побайтно.
func TestPreviewTplWithPhotos(t *testing.T) {
	tpl := previewTpl(4)
	t.Run("места по порядку", func(t *testing.T) { previewInOrder(t, tpl) })
	t.Run("лишние фото", func(t *testing.T) { previewExtra(t, tpl) })
	t.Run("меньше мест", func(t *testing.T) { previewFewer(t, tpl) })
	t.Run("повтор", previewRepeat)
	t.Run("nil и пусто", func(t *testing.T) { previewSameAsPlain(t, tpl) })
	t.Run("ошибка шаблона", func(t *testing.T) {
		if _, _, err := album.PreviewTplWith(looktpl.SpreadTpl{}, 300, 200, nil, previewPhotos(2)); err == nil {
			t.Error("пустой шаблон принят")
		}
	})
}

// requireAllSeated — каждое место вида занято фото.
func requireAllSeated(t *testing.T, v model.View) {
	t.Helper()
	for i, pl := range v.Spreads[0].Places {
		if pl.Photo == "" {
			t.Errorf("место %d в виде пусто", i)
		}
	}
}

func previewInOrder(t *testing.T, tpl looktpl.SpreadTpl) {
	ph := previewPhotos(4)
	p, v, err := album.PreviewTplWith(tpl, 300, 200, nil, ph)
	if err != nil {
		t.Fatal(err)
	}
	// Порядок посадки — FillSeats пустого листа, он же порядок Grid.
	empty, _, err := album.PreviewTpl(tpl, 300, 200, nil)
	if err != nil {
		t.Fatal(err)
	}
	seats := empty.FillSeats(empty.Spreads[0].ID)
	if len(seats) != len(ph) {
		t.Fatalf("мест %d, ждали %d", len(seats), len(ph))
	}
	want := make([]string, len(ph))
	for i, x := range ph {
		want[i] = x.ID
	}
	if got := seated(p); !slices.Equal(got, want) {
		t.Fatalf("на местах %v, ждали %v", got, want)
	}
	requireAllSeated(t, v)
	for _, x := range ph {
		if r := p.Photos[x.ID]; r.Path != x.Path || r.W != x.W || r.H != x.H {
			t.Errorf("сведения %s: %+v", x.ID, r)
		}
	}
}

func previewExtra(t *testing.T, tpl looktpl.SpreadTpl) {
	ph := previewPhotos(9)
	p, _, err := album.PreviewTplWith(tpl, 300, 200, nil, ph)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{ph[0].ID, ph[1].ID, ph[2].ID, ph[3].ID}
	if got := seated(p); !slices.Equal(got, want) {
		t.Fatalf("на местах %v, ждали первые четыре %v", got, want)
	}
	if len(p.Spreads) != 1 {
		t.Errorf("листов %d: лишние фото не должны добавлять листы", len(p.Spreads))
	}
}

func previewFewer(t *testing.T, tpl looktpl.SpreadTpl) {
	p, _, err := album.PreviewTplWith(tpl, 300, 200, nil, previewPhotos(2))
	if err != nil {
		t.Fatal(err)
	}
	if got := seated(p); got[0] == "" || got[1] == "" || got[2] != "" || got[3] != "" {
		t.Fatalf("на местах %v: ждали два фото и два пустых", got)
	}
}

// previewRepeat — 3 фото на 7 мест: повторы приходят копиями «ID-n», все места с фото.
func previewRepeat(t *testing.T) {
	base := previewPhotos(3)
	var ph []model.PhotoInfo
	for i := range 7 {
		x := base[i%3]
		x.ID = model.CopyID(x.ID, i/3+1)
		ph = append(ph, x)
	}
	p, v, err := album.PreviewTplWith(previewTpl(7), 300, 200, nil, ph)
	if err != nil {
		t.Fatal(err)
	}
	got := seated(p)
	if len(got) != 7 {
		t.Fatalf("мест %d", len(got))
	}
	for i, id := range got {
		if id != ph[i].ID {
			t.Errorf("место %d: %q, ждали %q", i, id, ph[i].ID)
		}
	}
	requireAllSeated(t, v)
}

// previewSameAsPlain — nil и пустой список дают то же, что PreviewTpl, побайтно.
func previewSameAsPlain(t *testing.T, tpl looktpl.SpreadTpl) {
	p0, v0, err := album.PreviewTpl(tpl, 300, 200, nil)
	if err != nil {
		t.Fatal(err)
	}
	for name, ph := range map[string][]model.PhotoInfo{"nil": nil, "пусто": {}} {
		p, v, err := album.PreviewTplWith(tpl, 300, 200, nil, ph)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(sameBytes(p, v), sameBytes(p0, v0)) {
			t.Errorf("%s: результат отличается от PreviewTpl", name)
		}
	}
}
