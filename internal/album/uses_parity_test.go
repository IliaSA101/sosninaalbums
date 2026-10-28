package album_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Паритет «сколько раз использовано» (098 §9, decisions 098-6): число в ленте (интерфейс считает
// по виду, base/core/usedindex.ts) = «Использовано N раз» карточки (Go, view.Uses). Вектор
// testdata/parity/uses.json: листы вида (обложка первой) только с полями, по которым считает
// интерфейс, фон альбома и ответ Go по каждому файлу.

// usesSheet — лист вида в векторе: имена полей — как в JSON SpreadView.
type usesSheet struct {
	ID        string        `json:"id"`
	Photos    []string      `json:"photos"`
	Places    []usesPlace   `json:"places"`
	Backdrops []usesPhoto   `json:"backdrops"`
	Stickers  []usesSticker `json:"stickers"`
}

type usesPlace struct {
	Photo  string     `json:"photo"`
	Border *usesPhoto `json:"border,omitempty"`
}

type usesPhoto struct {
	Photo string `json:"photo"`
	Album bool   `json:"album,omitempty"`
}

type usesSticker struct {
	Photo string `json:"photo,omitempty"`
}

// usesFile — ответ Go по файлу: вхождения (число в ленте и на карточке) и места (бледность,
// «Неиспользованные», «Перенести сюда» — по местам).
type usesFile struct {
	ID     string `json:"id"`
	Uses   int    `json:"uses"`
	Places int    `json:"places"`
}

type usesVector struct {
	AlbumBg string      `json:"albumBg"` // View.AlbumBg.Photo
	Sheets  []usesSheet `json:"sheets"`
	Files   []usesFile  `json:"files"`
}

// Файлы сцены: ID файла по номеру; L1, L2 — своё из библиотеки.
const (
	fileA = "00000000000000a1"
	fileB = "00000000000000b1"
	fileC = "00000000000000c1"
	fileD = "00000000000000d1"
	fileX = "00000000000000ff" // в альбоме нет
)

// usesScene — копии на обложке и листах, фото ленты фоном, свои стикер, рамочки и фон альбома:
// A — место листа 1, копия 2 на обложке, копия 3 на листе 2 и сама A фоном листа 2; B — одно
// место; C — только копией на месте сетки листа 3 (рамочка L2 и у пустого места); D — только фон
// лицевой обложки; L1 — фон альбома (рисуется на листах 1 и 3 — их не считать); L2 — стикер листа
// 1 и две рамочки листа 3.
func usesScene(t *testing.T) usesVector {
	t.Helper()
	l1, l2 := model.LibPhotoID("my:00000000000000aa"), model.LibPhotoID("my:00000000000000bb")
	p := model.New()
	ref := func(name string) model.PhotoRef { return model.PhotoRef{Path: edtest.Abs(name), W: 300, H: 200} }
	for id, name := range map[string]string{fileA: "a.jpg", model.CopyID(fileA, 3): "a.jpg", fileB: "b.jpg", model.CopyID(fileC, 2): "c.jpg", fileD: "d.jpg"} {
		p.Photos[id] = ref(name)
	}
	p.Photos[l1] = model.PhotoRef{Lib: "my:00000000000000aa", Path: edtest.Abs("l1.png"), W: 100, H: 100}
	p.Photos[l2] = model.PhotoRef{Lib: "my:00000000000000bb", Path: edtest.Abs("l2.png"), W: 100, H: 100}
	border := func() *model.Border { return &model.Border{Photo: l2, Cut: [4]float64{0.1, 0.1, 0.1, 0.1}} }
	p.Spreads = []*model.Spread{
		{ID: "000000000001", Photos: []string{fileA, fileB},
			Stickers: []*model.Sticker{{ID: "0000000000e1", Photo: l2, X: 10, Y: 10, W: 30, H: 30}}},
		{ID: "000000000002", Photos: []string{model.CopyID(fileA, 3)}, BgPhoto: &model.Backdrop{Photo: fileA, W: 300, H: 200}},
		{ID: "000000000003", Photos: []string{model.CopyID(fileC, 2)}, Grid: []*model.Place{
			{ID: "0000000000b1", X: 10, Y: 10, W: 100, H: 100, Photo: model.CopyID(fileC, 2), Border: border()},
			{ID: "0000000000b2", X: 150, Y: 10, W: 100, H: 100, Border: border()},
		}},
	}
	p.AlbumBg = &model.AlbumBg{Photo: l1}
	e := edtest.Open(t, p, "")
	d := model.PhotoInfo{ID: fileD, Path: edtest.Abs("d.jpg"), W: 300, H: 200}
	if _, err := e.CreateCover("", &d); err != nil {
		t.Fatal(err)
	}
	edtest.Change(e, func(p *model.Project) { // у обложки только места сетки
		a2 := model.CopyID(fileA, 2)
		p.Photos[a2] = p.Photos[fileA] // запись без места снесла прежняя правка (PruneRefs)
		p.Cover.Sheet.Grid = []*model.Place{{ID: "0000000000b3", X: 300, Y: 50, W: 100, H: 100, Photo: a2}}
		p.Cover.Sheet.Photos = []string{a2}
	})
	if err := edtest.GridInvariants(edtest.P(e)); err != nil {
		t.Fatal(err)
	}

	v := e.View()
	var out usesVector
	if v.AlbumBg != nil {
		out.AlbumBg = v.AlbumBg.Photo
	}
	sheets := v.Spreads
	if v.Cover != nil {
		sheets = append([]model.SpreadView{v.Cover.Sheet}, sheets...)
	}
	for _, s := range sheets {
		us := usesSheet{ID: s.ID, Photos: s.Photos, Places: []usesPlace{}, Backdrops: []usesPhoto{}, Stickers: []usesSticker{}}
		for _, pl := range s.Places {
			up := usesPlace{Photo: pl.Photo}
			if pl.Border != nil && pl.Border.Photo != "" {
				up.Border = &usesPhoto{Photo: pl.Border.Photo}
			}
			us.Places = append(us.Places, up)
		}
		for _, b := range s.Backdrops {
			us.Backdrops = append(us.Backdrops, usesPhoto{Photo: b.Photo, Album: b.Album})
		}
		for _, st := range s.Stickers {
			us.Stickers = append(us.Stickers, usesSticker{Photo: st.Photo})
		}
		out.Sheets = append(out.Sheets, us)
	}
	for _, id := range []string{fileA, fileB, fileC, fileD, l1, l2, fileX} {
		uses, _ := e.PhotoUses(id)
		out.Files = append(out.Files, usesFile{ID: id, Uses: len(uses), Places: e.WhereUsed([]string{id})[0].Places})
	}
	return out
}

// countByView — счёт интерфейса по вектору (то, что повторяет usedindex.ts): фон альбома — один
// раз, его копии на листах (album) не в счёт; места, фоны, стикеры и рамочки — по ID файла.
func countByView(v usesVector, id string) (uses, places int) {
	base := model.BaseID(id)
	if v.AlbumBg != "" && model.BaseID(v.AlbumBg) == base {
		uses++
	}
	for _, s := range v.Sheets {
		for _, x := range s.Photos {
			if model.BaseID(x) == base {
				uses++
				places++
			}
		}
		for _, b := range s.Backdrops {
			if !b.Album && model.BaseID(b.Photo) == base {
				uses++
			}
		}
		for _, st := range s.Stickers {
			if st.Photo != "" && model.BaseID(st.Photo) == base {
				uses++
			}
		}
		for _, pl := range s.Places {
			if pl.Border != nil && model.BaseID(pl.Border.Photo) == base {
				uses++
			}
		}
	}
	return uses, places
}

// TestUsesParity — счёт по виду совпадает с view.Uses и WhereUsed по каждому файлу сцены; вектор
// пишется с -update и сверяется каждый прогон (правка правил без перезаписи роняет тест).
func TestUsesParity(t *testing.T) {
	v := usesScene(t)
	want := map[string][2]int{fileA: {4, 3}, fileB: {1, 1}, fileC: {1, 1}, fileD: {1, 0}, fileX: {0, 0}}
	for _, f := range v.Files {
		if w, ok := want[f.ID]; ok && (f.Uses != w[0] || f.Places != w[1]) {
			t.Errorf("Go %s: вхождений %d, мест %d; ждали %v", f.ID, f.Uses, f.Places, w)
		}
		if u, pl := countByView(v, f.ID); u != f.Uses || pl != f.Places {
			t.Errorf("по виду %s: вхождений %d, мест %d; Go — %d, %d", f.ID, u, pl, f.Uses, f.Places)
		}
	}
	if l1, l2 := v.Files[4], v.Files[5]; l1.Uses != 1 || l2.Uses != 3 {
		t.Errorf("своё: фон альбома %+v, стикер и рамочки %+v", l1, l2)
	}
	path := filepath.Join("..", "..", "testdata", "parity", "uses.json")
	if *edtest.Update { // флаг -update общий (edtest): marks.json, replay.json.gz, uses.json
		data, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var got usesVector
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, v) {
		t.Fatal("uses.json устарел (перезапись: go test ./internal/album -run UsesParity -update)")
	}
}
