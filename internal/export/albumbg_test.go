package export

import (
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/render"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Фон альбома в выгрузке (098 design §4.2): рисуется из вида листа тем же путём, что свой фон.

// albumBgSheets — n пустых разворотов s1…sn.
func albumBgSheets(n int) *model.Project {
	p := model.New()
	for i := range n {
		p.Spreads = append(p.Spreads, &model.Spread{ID: "s" + strconv.Itoa(i+1)})
	}
	return p
}

// sheetPix — пиксели листа задания выгрузки (20 dpi).
func sheetPix(t *testing.T, pp *model.Project, es render.Spread) string {
	t.Helper()
	img, err := render.Render(es, render.Options{PageW: pp.Settings.PageW, PageH: pp.Settings.PageH, DPI: 20})
	if err != nil {
		t.Fatal(err)
	}
	return string(img.Pix)
}

// TestAlbumBgExport — лист с фоном альбома побайтно равен листу со своим таким же фоном (узор и
// фото); лист с «Не показывать здесь» — без него; обложка — без него.
func TestAlbumBgExport(t *testing.T) {
	ph := model.PhotoInfo{ID: "bg", Path: testkit.JPEG(t, testkit.TempDir(t), "bg.jpg", 60, 40), W: 60, H: 40}
	for _, c := range []struct {
		name string
		src  model.BgSource
	}{
		{"узор", model.BgSource{Pattern: &model.Pattern{Kind: "stripes", Inks: []string{"#203040"}}}},
		{"фото", model.BgSource{Photo: ph}},
	} {
		t.Run(c.name, func(t *testing.T) {
			e := mustEditor(t, albumBgSheets(3), "")
			if _, err := e.CreateCover("", nil); err != nil {
				t.Fatal(err)
			}
			if _, err := e.SetAlbumBg(c.src, false); err != nil {
				t.Fatal(err)
			}
			e.SetNoAlbumBg("s2", true)
			if _, _, err := e.ApplyBackgroundTo([]model.GridTarget{{Spread: "s3", Scope: model.ScopeSpread}}, c.src); err != nil {
				t.Fatal(err)
			}
			pp, v := e.Snapshot()
			got := FromView(pp, v)
			if len(got[0].Patterns)+len(got[0].Backdrops) != 1 || len(got[1].Patterns)+len(got[1].Backdrops) != 0 {
				t.Fatalf("задание: s1 %+v %+v, s2 %+v %+v", got[0].Patterns, got[0].Backdrops, got[1].Patterns, got[1].Backdrops)
			}
			album, hidden, own := sheetPix(t, pp, got[0]), sheetPix(t, pp, got[1]), sheetPix(t, pp, got[2])
			if album != own {
				t.Fatal("фон альбома не равен своему такому же фону")
			}
			if album == hidden {
				t.Fatal("фон альбома не нарисован")
			}
			if cv := v.Cover.Sheet; len(cv.Patterns)+len(cv.Backdrops) != 0 {
				t.Fatalf("обложка с фоном альбома: %+v %+v", cv.Patterns, cv.Backdrops)
			}
		})
	}
}

// TestBlankIgnoresAlbumBg — лист только с фоном альбома пустой для вопроса перед выгрузкой: фон
// альбома не содержимое листа; свой такой же фон — содержимое.
func TestBlankIgnoresAlbumBg(t *testing.T) {
	e := mustEditor(t, albumBgSheets(3), "")
	src := model.BgSource{Pattern: &model.Pattern{Kind: "stripes"}}
	if _, err := e.SetAlbumBg(src, true); err != nil {
		t.Fatal(err)
	}
	if _, _, err := e.ApplyBackgroundTo([]model.GridTarget{{Spread: "s2", Scope: model.ScopeLeft}}, src); err != nil {
		t.Fatal(err)
	}
	ph := model.PhotoInfo{ID: "bg", Path: testkit.JPEG(t, testkit.TempDir(t), "bg.jpg", 6, 4), W: 6, H: 4}
	_, v := e.Snapshot()
	pl := NewPlan(nil, len(v.Spreads), false)
	if got := BlankSpreads(v, &pl); !slices.Equal(got, []int{1, 3}) {
		t.Fatalf("узор альбома: пустые %v", got)
	}
	if _, err := e.SetAlbumBg(model.BgSource{Photo: ph}, false); err != nil {
		t.Fatal(err)
	}
	_, v = e.Snapshot()
	if got := BlankSpreads(v, &pl); !slices.Equal(got, []int{1, 3}) || len(v.Spreads[0].Backdrops) != 1 {
		t.Fatalf("фото альбома: пустые %v", got)
	}
}

// TestMissingAlbumBg — фото фона альбома попадает в проверку пропавших файлов перед выгрузкой
// (SelectedPhotos берёт его из вида листа): файл есть — тишина, нет — имя в ошибке, своего из
// библиотеки нет — «нет в библиотеке»; фон, скрытый на всех листах или прозрачный на 100 %, не
// рисуется и файла не требует.
func TestMissingAlbumBg(t *testing.T) {
	path := testkit.JPEG(t, testkit.TempDir(t), "bg.jpg", 60, 40)
	const key = "my:00000000000000ef"
	cases := []struct {
		name string
		ph   model.PhotoInfo
		want string // "" — ошибки нет, пока файл на месте
	}{
		{"файл ленты", model.PhotoInfo{ID: "bg", Path: path, W: 60, H: 40}, "bg.jpg"},
		{"своё из библиотеки", model.PhotoInfo{ID: model.LibPhotoID(key), Lib: key, W: 60, H: 40}, "нет в библиотеке"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := mustEditor(t, albumBgSheets(3), "")
			if _, err := e.SetAlbumBg(model.BgSource{Photo: c.ph}, false); err != nil {
				t.Fatal(err)
			}
			pp, v := e.Snapshot()
			sel := NewPlan(nil, len(v.Spreads), false).SelectedPhotos(v, pp.Settings.PageW)
			if !sel[c.ph.ID] || len(sel) != 1 {
				t.Fatalf("выбранные фото %v, ждали только %s", sel, c.ph.ID)
			}
			gone := func(string) bool { return false }
			if err := MissingPhotos(pp, sel, gone); err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("без файла: %v, ждали %q", err, c.want)
			}
			if c.ph.Lib == "" {
				if err := MissingPhotos(pp, sel, func(string) bool { return true }); err != nil {
					t.Fatalf("файл на месте: %v", err)
				}
			}
			e.SetAlbumBgFade(1, "")
			pp, v = e.Snapshot()
			if sel := NewPlan(nil, len(v.Spreads), false).SelectedPhotos(v, pp.Settings.PageW); len(sel) != 0 {
				t.Fatalf("прозрачный на 100 %%: выбрано %v", sel)
			}
			e.SetAlbumBgFade(0, "")
			for _, s := range v.Spreads {
				e.SetNoAlbumBg(s.ID, true)
			}
			pp, v = e.Snapshot()
			if sel := NewPlan(nil, len(v.Spreads), false).SelectedPhotos(v, pp.Settings.PageW); len(sel) != 0 {
				t.Fatalf("скрыт на всех листах: выбрано %v", sel)
			}
		})
	}
}

// BenchmarkExportAlbumBgPhoto — фото-фон альбома на 40 листах против своего того же фото на 40
// листах («Фоны → все развороты»): путь один, ждём равенства ±10 % (098 design §4.2). Кэша
// декодированных картинок между листами нет — файл читается на каждом листе.
func BenchmarkExportAlbumBgPhoto(b *testing.B) {
	const sheets = 40
	path := testkit.JPEG(b, testkit.TempDir(b), "bg.jpg", 2400, 1600)
	ph := model.PhotoInfo{ID: "bg", Path: path, W: 2400, H: 1600}
	for _, albumBg := range []bool{false, true} {
		name := "свой фон"
		if albumBg {
			name = "фон альбома"
		}
		b.Run(name, func(b *testing.B) {
			e := mustEditor(b, albumBgSheets(sheets), "")
			setBg(b, e, ph, albumBg)
			pp, v := e.Snapshot()
			opt := render.Options{PageW: pp.Settings.PageW, PageH: pp.Settings.PageH, DPI: 100, Quality: 95}
			for b.Loop() {
				for _, es := range FromView(pp, v) {
					if len(es.Backdrops) != 1 {
						b.Fatal("лист без фона")
					}
					if _, err := render.Render(es, opt); err != nil {
						b.Fatal(err)
					}
				}
			}
		})
	}
}

// setBg — фото ph фоном всех листов: фоном альбома или своим на каждом («все развороты»).
func setBg(b *testing.B, e *album.Editor, ph model.PhotoInfo, albumBg bool) {
	var err error
	if albumBg {
		_, err = e.SetAlbumBg(model.BgSource{Photo: ph}, false)
	} else {
		var ts []model.GridTarget
		for _, s := range e.View().Spreads {
			ts = append(ts, model.GridTarget{Spread: s.ID, Scope: model.ScopeSpread})
		}
		_, _, err = e.ApplyBackgroundTo(ts, model.BgSource{Photo: ph})
	}
	if err != nil {
		b.Fatal(err)
	}
}
