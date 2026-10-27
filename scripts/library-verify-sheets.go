//go:build ignore

// Вторая половина library-verify (см. library-verify.go): рамки путём программы и их лист, мера
// «тянется», общий лист -preview и -pull шаблонов из библиотеки.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"math"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/decor"
	"github.com/IliaSA101/sosninaalbums/internal/decor/gen"
	"github.com/IliaSA101/sosninaalbums/internal/imgfile"
	"github.com/disintegration/imaging"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

// checkFrames — рамки набора путём программы (ServeFramePic, 9-срез) на трёх местах; лист frames.png.
func (v *verifier) checkFrames(info decor.PackInfo) {
	items := v.tiles(decor.KindFrame, info)
	if len(items) != info.Frame {
		v.failf("плиток рамок %d, в наборе %d", len(items), info.Frame)
	}
	var rows []image.Image
	for i, it := range items {
		v.ctx = fmt.Sprintf("%s · рамка %d «%s»", info.ID, i+1, it.Name)
		v.guard("рамка", func() {
			if row := v.frameRow(i+1, it); row != nil && len(rows) < maxFrameRows {
				rows = append(rows, row)
			}
		})
	}
	if v.out != "" && len(rows) > 0 {
		if err := imaging.Save(stack(rows, 8), filepath.Join(v.out, "frames.png")); err != nil {
			v.failf("frames.png: %v", err)
		}
	}
}

// frameRow — строка листа рамок: три места на шахматке и подпись; nil — рамка не читается.
func (v *verifier) frameRow(n int, it decor.Item) image.Image {
	fr, err := v.s.Frame(it.ID)
	if err != nil {
		v.failf("Frame: %v", err)
		return nil
	}
	share := -1.0
	if src, release, err := imgfile.OpenOriented(fr.Photo.Path); err != nil {
		v.failf("картинка не читается: %v", err)
	} else {
		share = stretchShare(src, fr.Cut)
		release()
	}
	note := ""
	if fr.Open {
		note = " open"
	}
	if share > stretchFlag {
		note += " FLAG"
	}
	fmt.Fprintf(v.w, "  рамка %d «%s»: тянется %.0f%%%s\n", n, it.Name, share, note)
	var cells []image.Image
	for _, sp := range spotSizes {
		cell, err := v.frameCell(fr, sp)
		if err != nil {
			v.failf("%s: %v", sp.name, err)
			continue
		}
		cells = append(cells, cell)
	}
	W := 8
	for _, c := range cells {
		W += c.Bounds().Dx() + 8
	}
	row := imaging.New(W, cellH+16, color.NRGBA{0xd9, 0xd9, 0xdc, 255})
	x := 8
	for _, c := range cells {
		row = imaging.Paste(row, c, image.Pt(x, 14))
		x += c.Bounds().Dx() + 8
	}
	label(row, 8, 11, fmt.Sprintf("#%d stretch %.0f%%%s", n, share, note))
	return row
}

// overlay — накладка рамки на месте sp, как её отдаёт программа.
func overlay(fr decor.FrameSrc, sp size) (image.Image, error) {
	q := gen.FramePicQuery(gen.FramePicRequest{W: sp.w, H: sp.h, Scale: 1, Cut: fr.Cut, Open: fr.Open}) + fmt.Sprintf("&px=%d", framePx)
	rec := httptest.NewRecorder()
	decor.ServeFramePic(rec, httptest.NewRequest("GET", "/media/f/x?"+q, nil), fr.Photo.Path)
	if rec.Code != 200 {
		return nil, fmt.Errorf("ServeFramePic: код %d", rec.Code)
	}
	ov, err := png.Decode(rec.Body)
	if err != nil {
		return nil, fmt.Errorf("ответ не PNG: %w", err)
	}
	return ov, nil
}

// onChecker — накладка на шахматке, высотой cellH.
func onChecker(ov image.Image) image.Image {
	b := ov.Bounds()
	cell := checker(b.Dx(), b.Dy())
	draw.Draw(cell, cell.Bounds(), ov, b.Min, draw.Over)
	return imaging.Resize(cell, 0, cellH, imaging.Lanczos)
}

// frameCell — рамка на месте sp: окно прозрачно, рамка вровень с краем места.
func (v *verifier) frameCell(fr decor.FrameSrc, sp size) (image.Image, error) {
	ov, err := overlay(fr, sp)
	if err != nil {
		return nil, err
	}
	b := ov.Bounds()
	if !fr.Open { // у открытой окно — вся середина, у закрытой центр прозрачен
		if _, _, _, a := ov.At(b.Min.X+b.Dx()/2, b.Min.Y+b.Dy()/2).RGBA(); a>>8 > 8 {
			v.failf("%s: окно непрозрачно в центре", sp.name)
		}
	}
	if box := opaqueBox(ov); box.Min.X > 2 || box.Min.Y > 2 || box.Max.X < b.Dx()-2 || box.Max.Y < b.Dy()-2 {
		v.failf("%s: рамка не вровень с краем места (занимает %v из %v)", sp.name, box, b)
	}
	return onChecker(ov), nil
}

// opaqueBox — рамка непрозрачных (альфа > 16) пикселей в координатах от начала картинки; пустая
// картинка — пустой прямоугольник.
func opaqueBox(img image.Image) image.Rectangle {
	b := img.Bounds()
	x0, y0, x1, y1 := b.Max.X, b.Max.Y, b.Min.X, b.Min.Y
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if _, _, _, a := img.At(x, y).RGBA(); a>>8 > 16 {
				x0, y0, x1, y1 = min(x0, x), min(y0, y), max(x1, x+1), max(y1, y+1)
			}
		}
	}
	if x0 >= x1 || y0 >= y1 {
		return image.Rectangle{}
	}
	return image.Rect(x0, y0, x1, y1).Sub(b.Min)
}

func checker(w, h int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			c := uint8(0xcf)
			if (x/12+y/12)%2 == 1 {
				c = 0xe9
			}
			img.SetNRGBA(x, y, color.NRGBA{c, c, c + 4, 255})
		}
	}
	return img
}

// label — подпись ASCII (basicfont кириллицы не знает).
func label(dst draw.Image, x, y int, s string) {
	d := &font.Drawer{Dst: dst, Src: image.NewUniform(color.NRGBA{0x20, 0x20, 0x24, 255}), Face: basicfont.Face7x13, Dot: fixed.P(x, y)}
	d.DrawString(s)
}

// stack — картинки друг под другом на светло-сером.
func stack(rows []image.Image, gap int) image.Image {
	W, H := 0, gap
	for _, r := range rows {
		W, H = max(W, r.Bounds().Dx()+2*gap), H+r.Bounds().Dy()+gap
	}
	out := imaging.New(W, H, color.NRGBA{0xd9, 0xd9, 0xdc, 255})
	y := gap
	for _, r := range rows {
		out = imaging.Paste(out, r, image.Pt(gap, y))
		y += r.Bounds().Dy() + gap
	}
	return out
}

// stretchShare — доля «тянущегося» рамки, % (0.27-129-20; тот же счёт, что в devtools/sheetcut,
// frame.go: копия, потому что скрипт собирается и в 0.26, где devtools нет). img — готовая обрезанная
// рамка, cut — срез 9-среза, доли l, t, r, b. Непрозрачное — альфа ≥ 128. Центр тянется по обеим осям
// — каждый непрозрачный px в счёт; боковые полосы тянутся вдоль своей стороны — в счёт px, отличающийся
// от медианы своей линии (строка верхней и нижней полосы, столбец левой и правой) по альфе или любому
// каналу больше 20/255. Ровная планка и углы — 0. Делитель — всё непрозрачное. Правка меры — в обоих местах.
func stretchShare(img image.Image, cut [4]float64) float64 {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	src := image.NewNRGBA(image.Rect(0, 0, w, h))
	draw.Draw(src, src.Bounds(), img, b.Min, draw.Src)
	x0, y0 := int(math.Round(cut[0]*float64(w))), int(math.Round(cut[1]*float64(h)))
	x1, y1 := w-int(math.Round(cut[2]*float64(w))), h-int(math.Round(cut[3]*float64(h)))
	if x0 >= x1 || y0 >= y1 {
		return 100
	}
	total := opaqueIn(src, src.Bounds())
	if total == 0 {
		return 0
	}
	n := opaqueIn(src, image.Rect(x0, y0, x1, y1))
	n += unevenAlong(src, image.Rect(x0, 0, x1, y0), true) + unevenAlong(src, image.Rect(x0, y1, x1, h), true)
	n += unevenAlong(src, image.Rect(0, y0, x0, y1), false) + unevenAlong(src, image.Rect(x1, y0, w, y1), false)
	return 100 * float64(n) / float64(total)
}

const (
	opaqueMin = 128 // непрозрачное: альфа не меньше
	unevenTol = 20  // ≈ 8 %: допуск альфы и цвета до «отличается от медианы линии»
)

// opaqueIn — непрозрачных px в прямоугольнике.
func opaqueIn(src *image.NRGBA, rc image.Rectangle) (n int) {
	for y := rc.Min.Y; y < rc.Max.Y; y++ {
		for x := rc.Min.X; x < rc.Max.X; x++ {
			if src.Pix[y*src.Stride+x*4+3] >= opaqueMin {
				n++
			}
		}
	}
	return n
}

// unevenAlong — непрозрачные px полосы rc, отличающиеся от медианы своей линии: rows — линии идут
// по строкам (полоса тянется по X), иначе по столбцам. Медиана альфы — по всем px линии, цвета — по
// непрозрачным (нижняя медиана: первое значение, с которого накоплено не меньше половины).
func unevenAlong(src *image.NRGBA, rc image.Rectangle, rows bool) int {
	if rc.Empty() {
		return 0
	}
	lines, length := rc.Dx(), rc.Dy()
	if rows {
		lines, length = rc.Dy(), rc.Dx()
	}
	at := func(line, k int) []uint8 {
		x, y := rc.Min.X+k, rc.Min.Y+line
		if !rows {
			x, y = rc.Min.X+line, rc.Min.Y+k
		}
		o := y*src.Stride + x*4
		return src.Pix[o : o+4]
	}
	count := 0
	for line := range lines {
		var hist [4][256]int
		all, op := 0, 0
		for k := range length {
			p := at(line, k)
			all++
			hist[3][p[3]]++
			if p[3] >= opaqueMin {
				op++
				for c := range 3 {
					hist[c][p[c]]++
				}
			}
		}
		median := func(c, n int) int {
			acc := 0
			for v, k := range hist[c] {
				if acc += k; acc*2 >= n {
					return v
				}
			}
			return 0
		}
		ma := median(3, all)
		var mc [3]int
		if op > 0 {
			mc = [3]int{median(0, op), median(1, op), median(2, op)}
		}
		for k := range length {
			p := at(line, k)
			if p[3] < opaqueMin {
				continue
			}
			far := math.Abs(float64(p[3])-float64(ma)) > unevenTol
			for c := range 3 {
				far = far || math.Abs(float64(p[c])-float64(mc[c])) > unevenTol
			}
			if far {
				count++
			}
		}
	}
	return count
}

// previewSheet — общий лист проверенных наборов: фоны, стикеры, рамки (место 20×30), развороты
// (страница 30×30), строками по виду; плиток каждого вида не больше maxTiles.
func (v *verifier) previewSheet(path string) {
	var bgs, sts, frs, tps []image.Image
	for _, info := range v.done {
		for _, it := range v.tiles(decor.KindBg, info) {
			bgs = appendTile(bgs, v.fileTile(it.ID, false))
		}
		for _, it := range v.tiles(decor.KindSticker, info) {
			sts = appendTile(sts, v.fileTile(it.ID, true))
		}
		for _, it := range v.tiles(decor.KindFrame, info) {
			if fr, err := v.s.Frame(it.ID); err == nil {
				if ov, err := overlay(fr, spotSizes[1]); err == nil {
					frs = appendTile(frs, onChecker(ov))
				}
			}
		}
		for _, it := range v.tiles(decor.KindSpread, info) {
			tps = appendTile(tps, v.tplTile(it))
		}
	}
	if len(bgs)+len(sts)+len(frs)+len(tps) == 0 {
		v.failf("лист %s: нет ни одной плитки", filepath.Base(path))
		return
	}
	var rows []image.Image
	for _, sec := range [][]image.Image{bgs, sts, frs, tps} {
		if len(sec) > 0 {
			rows = append(rows, flow(sec, flowW, 8))
		}
	}
	if err := imaging.Save(stack(rows, 16), path); err != nil {
		v.failf("лист %s: %v", filepath.Base(path), err)
	}
}

func appendTile(list []image.Image, t image.Image) []image.Image {
	if t == nil || len(list) >= maxTiles {
		return list
	}
	return append(list, t)
}

// fileTile — плитка фона или стикера rowH высотой: файл как есть, стикер вписан на тёмную подложку
// (кайма и пятна пустот видны).
func (v *verifier) fileTile(key string, sticker bool) image.Image {
	info, ok := v.s.LibInfo(key)
	if !ok {
		return nil
	}
	src, release, err := imgfile.OpenOriented(info.Path)
	if err != nil {
		v.failf("лист: %s не читается: %v", key, err)
		return nil
	}
	defer release()
	if !sticker {
		return imaging.Resize(src, 0, rowH, imaging.Lanczos)
	}
	tile := imaging.New(rowH, rowH, color.NRGBA{0x1e, 0x1e, 0x22, 255})
	fit := imaging.Fit(src, rowH-16, rowH-16, imaging.Lanczos)
	return imaging.OverlayCenter(tile, fit, 1)
}

// tplTile — плитка шаблона: страница 30×30, как строит окно «Развороты».
func (v *verifier) tplTile(it decor.Item) image.Image {
	t, err := v.s.SpreadTpl(it.ID)
	if err != nil {
		return nil
	}
	libs, _ := v.libsOf(t)
	sz := pageSizes[1]
	p, view, err := album.PreviewTpl(t, sz.w*float64(max(t.Pages, 1)), sz.h, libs)
	if err != nil {
		return nil
	}
	sv := view.Spreads[0]
	area := sv.W
	if t.Pages == 1 {
		area = sv.W / 2
	}
	img, err := tplImage(t, p, view, int(float64(rowH)*area/sv.H))
	if err != nil {
		v.failf("лист: плитка «%s»: %v", it.Name, err)
		return nil
	}
	return imaging.Resize(img, 0, rowH, imaging.Lanczos)
}

// flow — плитки строками шириной до maxW.
func flow(tiles []image.Image, maxW, gap int) image.Image {
	var lines []image.Image
	var cur []image.Image
	w := 0
	flush := func() {
		if len(cur) == 0 {
			return
		}
		line := imaging.New(max(w, 1), rowH, color.NRGBA{0xd9, 0xd9, 0xdc, 255})
		x := 0
		for _, t := range cur {
			line = imaging.Paste(line, t, image.Pt(x, 0))
			x += t.Bounds().Dx() + gap
		}
		lines = append(lines, line)
		cur, w = nil, 0
	}
	for _, t := range tiles {
		if w > 0 && w+t.Bounds().Dx() > maxW {
			flush()
		}
		cur = append(cur, t)
		w += t.Bounds().Dx() + gap
	}
	flush()
	return stack(lines, gap)
}

// pullTpls забирает из Развороты/ библиотеки шаблоны, все ключи которых из набора packID
// (правки Марины, design §5): JSON в out. Библиотека — папка данных или её «Библиотека».
func pullTpls(w io.Writer, lib, packID, out string) error {
	cfg := lib
	if filepath.Base(lib) == decor.RootName {
		cfg = filepath.Dir(lib)
	}
	s := decor.Open(cfg)
	title := ""
	for _, p := range s.Packs() {
		if p.ID == packID && p.Removable {
			title = p.Title
		}
	}
	if title == "" {
		return fmt.Errorf("набора %q нет в библиотеке", packID)
	}
	keys := map[string]bool{}
	for _, kind := range []string{decor.KindBg, decor.KindSticker} {
		cat, err := s.Catalog(kind, nil, 300, 300)
		if err != nil {
			return err
		}
		for _, it := range cat.Items {
			if it.Pack == title {
				keys[it.ID] = true
			}
		}
	}
	cat, err := s.Catalog(decor.KindSpread, nil, 300, 300)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	n := 0
	for _, it := range cat.Items {
		if it.Pack != "" || !strings.HasPrefix(it.ID, "my:") {
			continue
		}
		t, err := s.SpreadTpl(it.ID)
		if err != nil {
			fmt.Fprintf(w, "  «%s»: пропущен: %v\n", it.Name, err)
			continue
		}
		used := tplKeys(t)
		if len(used) == 0 || slices.ContainsFunc(used, func(k string) bool { return !keys[k] }) {
			continue
		}
		data, err := json.MarshalIndent(t, "", "  ")
		if err != nil {
			return errors.New("шаблон не записывается")
		}
		name := fmt.Sprintf("%02d-%s.json", n+1, cmpOr(slug(it.Name), "razvorot"))
		if err := os.WriteFile(filepath.Join(out, name), data, 0o644); err != nil {
			return err
		}
		fmt.Fprintf(w, "  «%s» → %s\n", it.Name, name)
		n++
	}
	fmt.Fprintf(w, "забрано шаблонов: %d\n", n)
	return nil
}
