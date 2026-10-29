package packprep

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"strings"
)

// Листы проверки _check/ (design §2.4): плитки по семействам размером с плитку программы (168 px),
// те же предметы по 512 px и углы исходных листов в 100 %.
const (
	TileSide    = 168 // наибольшая плитка окон по умолчанию (DecorDialog.svelte: фоны 168, стикеры 120)
	BigSide     = 512
	tilesPerRow = 8
	bigPerRow   = 4
	bigPerPage  = 12
	tilePad     = 8
	cornerSide  = 200 // угол листа: 200×200, четыре угла — блок 400×400
)

var (
	sheetDark  = color.NRGBA{0x1e, 0x1e, 0x22, 0xff} // #1e1e22
	sheetColor = color.NRGBA{0x4a, 0x7a, 0x6e, 0xff} // #4a7a6e
	textLight  = color.NRGBA{0xee, 0xee, 0xf2, 0xff}
	textDim    = color.NRGBA{0xa8, 0xa8, 0xb4, 0xff}
	textWarn   = color.NRGBA{0xff, 0xb3, 0x47, 0xff}
)

// edgeBlock — четыре угла исходного листа 200×200 в 100 % (мнимые надписи Mystic у краёв).
type edgeBlock struct {
	label string
	img   *image.NRGBA
}

// cornerBlock собирает блок 400×400: углы листа на своих местах, середина вырезана.
func cornerBlock(src image.Image) *image.NRGBA {
	b := src.Bounds()
	blk := image.NewNRGBA(image.Rect(0, 0, 2*cornerSide, 2*cornerSide))
	draw.Draw(blk, blk.Bounds(), image.NewUniform(color.NRGBA{0x80, 0x80, 0x80, 0xff}), image.Point{}, draw.Src)
	cw, ch := min(cornerSide, b.Dx()), min(cornerSide, b.Dy())
	for i, o := range [4]image.Point{{0, 0}, {b.Dx() - cw, 0}, {0, b.Dy() - ch}, {b.Dx() - cw, b.Dy() - ch}} {
		dst := image.Pt((i%2)*cornerSide, (i/2)*cornerSide)
		draw.Draw(blk, image.Rect(dst.X, dst.Y, dst.X+cw, dst.Y+ch), src, b.Min.Add(o), draw.Src)
	}
	return blk
}

func overlayName(idx int) string { return fmt.Sprintf("whites-%02d.png", idx+1) }

// writeOverlay пишет лист svgcut с подсветкой белых путей: _check/whites-NN.png (NN — номер предмета рецепта).
func (r *runner) writeOverlay(idx int, img *image.NRGBA) error {
	dir := filepath.Join(r.cfg.Out, CheckDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return savePNG(filepath.Join(dir, overlayName(idx)), img)
}

func savePNG(file string, img image.Image) error {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return err
	}
	return os.WriteFile(file, buf.Bytes(), 0o644)
}

// writeCheck пишет листы и report.json в _check/.
func (r *runner) writeCheck() error {
	dir := filepath.Join(r.cfg.Out, CheckDir)
	for _, s := range []struct {
		name string
		bg   color.NRGBA
	}{{"items-dark", sheetDark}, {"items-color", sheetColor}} {
		pages := tileSheets(r.outs, TileSide, tilesPerRow, 0, s.bg)
		for i, pg := range pages {
			if err := savePNG(filepath.Join(dir, pageName(s.name, i)), pg); err != nil {
				return err
			}
		}
	}
	for i, pg := range tileSheets(r.outs, BigSide, bigPerRow, bigPerPage, sheetDark) {
		if err := savePNG(filepath.Join(dir, pageName("items-big-dark", i)), pg); err != nil {
			return err
		}
	}
	if len(r.edges) > 0 {
		if err := savePNG(filepath.Join(dir, "edges.png"), edgesSheet(r.edges)); err != nil {
			return err
		}
	}
	js, err := r.rep.JSON()
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, ReportName), append(js, '\n'), 0o644)
}

// pageName — имя листа: первая страница без номера, остальные -2, -3 …
func pageName(base string, i int) string {
	if i == 0 {
		return base + ".png"
	}
	return fmt.Sprintf("%s-%d.png", base, i+1)
}

// tileRow — ряд плиток; head — заголовок семейства над первым рядом группы.
type tileRow struct {
	head string
	outs []*Output
}

// familyRows раскладывает предметы рядами по семействам (в пределах вида, в порядке первого появления):
// у каждого семейства свой ряд, длинное семейство переносится.
func familyRows(outs []*Output, cols int) []tileRow {
	type key struct{ kind, family string }
	var order []key
	groups := map[key][]*Output{}
	for _, o := range outs {
		k := key{o.Kind, o.Family}
		if _, ok := groups[k]; !ok {
			order = append(order, k)
		}
		groups[k] = append(groups[k], o)
	}
	var rows []tileRow
	for _, k := range order {
		g := groups[k]
		for i := 0; i < len(g); i += cols {
			row := tileRow{outs: g[i:min(len(g), i+cols)]}
			if i == 0 {
				row.head = fmt.Sprintf("%s  (%s, %d)", k.family, k.kind, len(g))
			}
			rows = append(rows, row)
		}
	}
	return rows
}

// tileSheets рисует листы плиток размером tile; perPage > 0 — не больше плиток на странице.
func tileSheets(outs []*Output, tile, cols, perPage int, bg color.NRGBA) []*image.NRGBA {
	sc := 1
	if tile > TileSide {
		sc = 2
	}
	capH := 5*lineAdv*sc + 2
	headH := lineAdv*sc + 6
	var pages [][]tileRow
	var cur []tileRow
	n := 0
	for _, row := range familyRows(outs, cols) {
		if perPage > 0 && n > 0 && n+len(row.outs) > perPage {
			pages = append(pages, cur)
			cur, n = nil, 0
		}
		cur = append(cur, row)
		n += len(row.outs)
	}
	pages = append(pages, cur)
	var res []*image.NRGBA
	for _, rows := range pages {
		h := tilePad
		for _, row := range rows {
			if row.head != "" {
				h += headH
			}
			h += tile + capH + tilePad
		}
		img := image.NewNRGBA(image.Rect(0, 0, cols*(tile+tilePad)+tilePad, max(h, tilePad)))
		draw.Draw(img, img.Bounds(), image.NewUniform(bg), image.Point{}, draw.Src)
		y := tilePad
		for _, row := range rows {
			if row.head != "" {
				drawText(img, tilePad, y+2, row.head, sc, textDim)
				y += headH
			}
			for i, o := range row.outs {
				drawTile(img, image.Pt(tilePad+i*(tile+tilePad), y), tile, sc, o)
			}
			y += tile + capH + tilePad
		}
		res = append(res, img)
	}
	return res
}

// drawTile рисует предмет в квадрате tile×tile и подпись: №, размер, см при 300 dpi, имя, метки флагов.
func drawTile(dst *image.NRGBA, at image.Point, tile, sc int, o *Output) {
	t := o.thumb
	if tile < max(t.Bounds().Dx(), t.Bounds().Dy()) {
		t = fit(t, tile)
	}
	x := at.X + (tile-t.Bounds().Dx())/2
	y := at.Y + (tile-t.Bounds().Dy())/2
	draw.Draw(dst, image.Rect(x, y, x+t.Bounds().Dx(), y+t.Bounds().Dy()), t, image.Point{}, draw.Over)
	width := tile / (glyphAdv * sc)
	ty := at.Y + tile + 2
	drawText(dst, at.X, ty, fmt.Sprintf("#%d %dx%d PX", o.N, o.W, o.H), sc, textLight)
	drawText(dst, at.X, ty+lineAdv*sc, fmt.Sprintf("%.1fx%.1f CM", o.CmW, o.CmH), sc, textDim)
	for i, l := range wrapText(o.Name, width, 2) {
		drawText(dst, at.X, ty+(2+i)*lineAdv*sc, l, sc, textLight)
	}
	if tags := flagTags(o); tags != "" {
		drawText(dst, at.X, ty+4*lineAdv*sc, wrapText(tags, width, 1)[0], sc, textWarn)
	}
}

// flagTags — короткие метки флагов для плитки: начало каждого флага до «:» или «(».
func flagTags(o *Output) string {
	var tags []string
	for _, f := range o.Flags {
		if strings.HasPrefix(f, "файл байт в байт") || strings.HasPrefix(f, "белые пути") {
			continue
		}
		if i := strings.IndexAny(f, ":("); i > 0 {
			f = f[:i]
		}
		tags = append(tags, strings.TrimSpace(f))
	}
	return strings.Join(tags, "; ")
}

// edgesSheet — блоки углов исходных листов пятью колонками, под каждым имя файла.
func edgesSheet(blocks []edgeBlock) *image.NRGBA {
	const cols, side, lab = 5, 2 * cornerSide, 14
	rows := (len(blocks) + cols - 1) / cols
	img := image.NewNRGBA(image.Rect(0, 0, cols*(side+tilePad)+tilePad, rows*(side+lab+tilePad)+tilePad))
	draw.Draw(img, img.Bounds(), image.NewUniform(sheetDark), image.Point{}, draw.Src)
	for i, b := range blocks {
		x, y := tilePad+(i%cols)*(side+tilePad), tilePad+(i/cols)*(side+lab+tilePad)
		draw.Draw(img, image.Rect(x, y, x+side, y+side), b.img, image.Point{}, draw.Src)
		drawText(img, x, y+side+3, fmt.Sprintf("%d %s", i+1, b.label), 1, textLight)
	}
	return img
}
