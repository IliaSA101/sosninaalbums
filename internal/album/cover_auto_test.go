package album_test

// Автосборка и обложка (145 Ч1, решение 0.31-145-2): сборка не берёт кадры обложки (места и фоны
// лицевой, корешка, задней) и не меняет Project.Cover — «как сейчас» закреплено тестом.

import (
	"bytes"
	"encoding/json"
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// caCover — JSON Project.Cover (байты до и после сборки сравниваются как есть).
func caCover(e *album.Editor) []byte {
	edtest.Mu(e).Lock()
	defer edtest.Mu(e).Unlock()
	b, _ := json.Marshal(edtest.P(e).Cover)
	return b
}

// caSpreadsHave — сколько фото лежит в разворотах и стоит ли в них кадр id.
func caSpreadsHave(p *model.Project, id string) (n int, has bool) {
	for _, s := range p.Spreads {
		n += len(s.Photos)
		has = has || slices.Contains(s.Photos, id)
		for _, pl := range s.Grid {
			has = has || pl.Photo == id
		}
	}
	return n, has
}

// caCount — сколько раз кадр id стоит в разворотах (в Photos листов).
func caCount(p *model.Project, id string) (n int) {
	for _, s := range p.Spreads {
		if slices.Contains(s.Photos, id) {
			n++
		}
	}
	return n
}

// caStage — по одному кадру обложки на каждый вид места: что именно занимает кадр.
var caStage = map[string]func(t *testing.T, e *album.Editor, ph model.PhotoInfo){
	"место лицевой": func(t *testing.T, e *album.Editor, ph model.PhotoInfo) {
		if _, _, err := e.ApplyCoverGrid(model.ScopeRight, []model.Box{lkBox(0.1, 0.1, 0.8, 0.8)}); err != nil {
			t.Fatal(err)
		}
		if _, left, _ := e.FillPlaces([]model.PhotoInfo{ph}, model.CoverID); left != 0 {
			t.Fatalf("кадр не встал в место: left %d", left)
		}
	},
	"фон лицевой": func(t *testing.T, e *album.Editor, ph model.PhotoInfo) { caBackdrop(t, e, model.ScopeRight, ph) },
	"фон задней":  func(t *testing.T, e *album.Editor, ph model.PhotoInfo) { caBackdrop(t, e, model.ScopeLeft, ph) },
	"фон корешка": func(t *testing.T, e *album.Editor, ph model.PhotoInfo) { caBackdrop(t, e, model.ScopeSpine, ph) },
}

func caBackdrop(t *testing.T, e *album.Editor, scope string, ph model.PhotoInfo) {
	t.Helper()
	if _, err := e.SetBackdrop(model.CoverID, scope, ph); err != nil {
		t.Fatal(err)
	}
}

// caEditor — редактор с шестью пустыми разворотами и обложкой «both»; кадр cov стоит на ней так,
// как велит stage. Пустые развороты нужны, чтобы сборка встала на них: число листов (а с ним
// ширина корешка и сдвиг правой стороны обложки) не меняется, и обложка остаётся байт в байт.
func caEditor(t *testing.T, stage func(*testing.T, *album.Editor, model.PhotoInfo), cov model.PhotoInfo) *album.Editor {
	t.Helper()
	e := edtest.Open(t, model.New(), "")
	edtest.Change(e, func(p *model.Project) {
		for range 6 {
			p.Spreads = append(p.Spreads, &model.Spread{ID: model.NewID()})
		}
	})
	if _, err := e.CreateCover("", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := e.SetCoverLayout(model.CoverBoth); err != nil { // сторонки и корешок
		t.Fatal(err)
	}
	stage(t, e, cov)
	return e
}

// caReadOnly — план и предпросмотр: кадр обложки в них не считается, проект и rev целы.
func caReadOnly(t *testing.T, e *album.Editor, feed []model.PhotoInfo, o model.AutoOpts, want int) {
	t.Helper()
	rev, before := edtest.Rev(e), caCover(e)
	plan, err := e.AutoPlan(feed, o)
	if err != nil || plan.Photos != want {
		t.Fatalf("AutoPlan: фото %d (ждали %d), %v", plan.Photos, want, err)
	}
	pv, err := e.AutoPreview(feed, -1, o)
	if err != nil {
		t.Fatal(err)
	}
	sum := 0
	for _, s := range pv.Spreads {
		sum += s.Photos
	}
	if sum != want || edtest.Rev(e) != rev || !bytes.Equal(caCover(e), before) {
		t.Fatalf("AutoPreview: фото %d (ждали %d), rev %d → %d", sum, want, rev, edtest.Rev(e))
	}
}

// TestAutoSkipsCoverPhotos: кадр обложки (место или фон любой сторонки) лежит и в ленте. План,
// предпросмотр, сборка, пересборка, «Весь альбом заново» и его пересборка его в развороты не
// берут, а Project.Cover после каждого шага байт в байт прежний.
func TestAutoSkipsCoverPhotos(t *testing.T) {
	for name, stage := range caStage {
		t.Run(name, func(t *testing.T) {
			cov, free := edtest.Photo("cover"), shots("f", 20)
			e := caEditor(t, stage, cov)
			feed := append(slices.Clone(free), cov)
			before := caCover(e)
			if !bytes.Contains(before, []byte(`"cover"`)) {
				t.Fatalf("кадр не попал на обложку: %s", before)
			}
			o := model.AutoOpts{Spreads: 6, Auto: true, Variety: model.VarietyNormal}
			caReadOnly(t, e, feed, o, len(free))

			ao := albumOpts(6, 0, false)
			againO, againA := o, ao
			againO.Again, againO.Variant = true, 2
			againA.Again, againA.Variant = true, 2
			// Сборка, пересборка вариантом 2, «Весь альбом заново» поверх неё и его пересборка.
			for _, step := range []struct {
				what string
				o    model.AutoOpts
			}{{"AutoBuild", o}, {"Again", againO}, {"Album", ao}, {"Album+Again", againA}} {
				if _, err := e.AutoBuild(feed, -1, step.o); err != nil {
					t.Fatalf("%s: %v", step.what, err)
				}
				if n, has := caSpreadsHave(edtest.P(e), cov.ID); has || n != len(free) {
					t.Fatalf("%s: кадр обложки в развороте %v, фото в разворотах %d (ждали %d)", step.what, has, n, len(free))
				}
				if after := caCover(e); !bytes.Equal(after, before) {
					t.Fatalf("%s: обложка изменилась:\n%s\n%s", step.what, before, after)
				}
			}
		})
	}
}

// TestAutoAlbumKeepsCover: кадр поставлен на обложку после сборки и стоит в развороте (копией в
// месте или тем же файлом на фоне). «Весь альбом заново» обложку не меняет, кадр в развороте
// остаётся — поведение «как сейчас» (Ф2): сборка пересобирает развороты, не сверяя их с обложкой.
func TestAutoAlbumKeepsCover(t *testing.T) {
	stage := map[string]func(t *testing.T, e *album.Editor, ph model.PhotoInfo){
		"копия в месте лицевой": func(t *testing.T, e *album.Editor, ph model.PhotoInfo) {
			if _, _, err := e.ApplyCoverGrid(model.ScopeRight, []model.Box{lkBox(0.1, 0.1, 0.8, 0.8)}); err != nil {
				t.Fatal(err)
			}
			ph.ID = model.CopyReq(ph.ID)
			if _, _, _, err := e.PlacePhotos(model.CoverID, edtest.P(e).Cover.Sheet.Grid[0].ID, []model.PhotoInfo{ph}); err != nil {
				t.Fatal(err)
			}
		},
		"фон лицевой": func(t *testing.T, e *album.Editor, ph model.PhotoInfo) { caBackdrop(t, e, model.ScopeRight, ph) },
	}
	for name, st := range stage {
		t.Run(name, func(t *testing.T) {
			e, feed, _ := albumFix(t)
			if _, err := e.CreateCover("", nil); err != nil {
				t.Fatal(err)
			}
			in := edtest.P(e).Spreads[0].Photos[0] // кадр, уже стоящий в развороте
			st(t, e, feed[slices.IndexFunc(feed, func(f model.PhotoInfo) bool { return f.ID == in })])
			before := caCover(e)
			n := len(bareIDs(edtest.P(e))) // столько же голых разворотов: листов и корешок прежние
			albumV(t, e, feed, n, 1, false)
			if c := caCount(edtest.P(e), in); c != 1 {
				t.Fatalf("кадр в разворотах %d раз, ждали 1 (дубль остаётся в развороте, не множится)", c)
			}
			if after := caCover(e); !bytes.Equal(after, before) {
				t.Fatalf("обложка изменилась:\n%s\n%s", before, after)
			}
			albumV(t, e, feed, n, 2, true)
			if after := caCover(e); !bytes.Equal(after, before) {
				t.Fatalf("после пересборки обложка изменилась:\n%s\n%s", before, after)
			}
		})
	}
}
