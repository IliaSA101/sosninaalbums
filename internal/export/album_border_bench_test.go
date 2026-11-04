package export

import (
	"strconv"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/render"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// frameKinds — виды встроенных рамочек каталога (gen.Frames; export пакет decor не импортирует).
var frameKinds = []string{"line", "double", "mat", "polaroid", "stitch", "corners", "gold", "scallop", "soft"}

// vignetteProject — sheets листов по 64 места (предел мест на лист); фото — общие файлы.
func vignetteProject(sheets int, paths []string) *model.Project {
	p := model.New()
	for s := range sheets {
		sp := &model.Spread{ID: "s" + strconv.Itoa(s)}
		for i := range model.MaxPlaces {
			id := "p" + strconv.Itoa(s) + "-" + strconv.Itoa(i)
			p.Photos[id] = model.PhotoRef{Path: paths[i], W: 30, H: 20}
			sp.Photos = append(sp.Photos, id)
			sp.Grid = append(sp.Grid, &model.Place{ID: model.NewID(), Photo: id,
				X: float64(5 + i%8*45), Y: float64(5 + i/8*25), W: 40, H: 22})
		}
		p.Spreads = append(p.Spreads, sp)
	}
	return p
}

// borderAll — рамочка каждому месту: виды каталога по кругу, со сдвигом по листу.
func borderAll(b *testing.B, e *album.Editor, p *model.Project) {
	for s, sp := range p.Spreads {
		byKind := make(map[string][]string)
		for i, pl := range sp.Grid {
			k := frameKinds[(i+s)%len(frameKinds)]
			byKind[k] = append(byKind[k], pl.ID)
		}
		for k, ids := range byKind {
			if _, err := e.ApplyBorder(sp.ID, ids, model.BorderSrc{Kind: k}); err != nil {
				b.Fatal(err)
			}
		}
	}
}

// BenchmarkExportVignetteBorder — цена рамочек в экспорте (079 design §12): 30 листов по 64 места
// (виньетка: предел мест на лист), рамочки всех видов каталога против тех же листов без рамочек.
// Предела нет — число сверяют до и после правок рендера рамочек. Один заход — 30 листов 300 dpi:
// View → FromView → Render каждого.
func BenchmarkExportVignetteBorder(b *testing.B) {
	dir := testkit.TempDir(b)
	paths := make([]string, model.MaxPlaces)
	for i := range paths {
		paths[i] = testkit.JPEG(b, dir, "v"+strconv.Itoa(i)+".jpg", 30, 20)
	}
	for _, withBorder := range []bool{false, true} {
		name := "без рамочек"
		if withBorder {
			name = "с рамочками"
		}
		b.Run(name, func(b *testing.B) {
			p := vignetteProject(30, paths)
			e := mustEditor(b, p, "")
			if withBorder {
				borderAll(b, e, p)
			}
			pp, v := e.Snapshot()
			opt := render.Options{PageW: pp.Settings.PageW, PageH: pp.Settings.PageH, DPI: 300, Quality: 95}
			for b.Loop() {
				for _, es := range FromView(pp, v) {
					if _, err := render.Render(es, opt); err != nil {
						b.Fatal(err)
					}
				}
			}
		})
	}
}
