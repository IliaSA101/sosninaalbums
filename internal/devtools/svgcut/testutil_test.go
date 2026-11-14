package svgcut

import (
	"encoding/xml"
	"errors"
	"image"
	"image/color"
	"image/draw"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// node — разобранный элемент SVG, который дал сам svgcut: для фальшивого растра в тестах.
type node struct {
	name  string
	attrs map[string]string
	kids  []*node
}

func parseTree(t testing.TB, svg []byte) *node {
	t.Helper()
	dec := xml.NewDecoder(strings.NewReader(string(svg)))
	var root *node
	var stack []*node
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("выход svgcut не XML: %v\n%s", err, svg)
		}
		switch e := tok.(type) {
		case xml.StartElement:
			n := &node{name: e.Name.Local, attrs: map[string]string{}}
			for _, a := range e.Attr {
				n.attrs[a.Name.Local] = a.Value
			}
			if len(stack) > 0 {
				p := stack[len(stack)-1]
				p.kids = append(p.kids, n)
			} else {
				root = n
			}
			stack = append(stack, n)
		case xml.EndElement:
			stack = stack[:len(stack)-1]
		}
	}
	return root
}

// fakeRaster — растр без браузера для SVG svgcut из прямоугольных путей: заливка по центру пикселя
// (как crispEdges), плоский цвет rgb(), градиент — серый 128, mask — стирание чёрных путей маски.
// Достаточно, чтобы проверить сборку документов, попадание точек и вырезы; настоящий Chromium —
// в TestSvgCutChromeKnockout.
type fakeRaster struct {
	t     testing.TB
	mu    sync.Mutex
	calls int
	svgs  []string
}

func (f *fakeRaster) Render(svg []byte, w, h int) (*image.NRGBA, error) {
	f.mu.Lock()
	f.calls++
	f.svgs = append(f.svgs, string(svg))
	f.mu.Unlock()
	root := parseTree(f.t, svg)
	vb := strings.Fields(root.attrs["viewBox"])
	var v [4]float64
	for i := range v {
		v[i], _ = strconv.ParseFloat(vb[i], 64)
	}
	r := &rasterCtx{w: w, h: h, v: v, masks: map[string][]Box{}}
	r.collectMasks(root)
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	r.paint(root, img)
	return img, nil
}

type rasterCtx struct {
	w, h  int
	v     [4]float64
	masks map[string][]Box // id маски → чёрные пути (в координатах листа)
}

func (r *rasterCtx) collectMasks(n *node) {
	if n.name == "mask" {
		for _, k := range n.kids {
			if k.name == "path" && k.attrs["fill"] == "#000" {
				if info, err := scanPath(k.attrs["d"]); err == nil {
					r.masks[n.attrs["id"]] = append(r.masks[n.attrs["id"]], info.box)
				}
			}
		}
	}
	for _, k := range n.kids {
		r.collectMasks(k)
	}
}

// px — прямоугольник листа в пикселях.
func (r *rasterCtx) px(b Box) (x0, y0, x1, y1 int) {
	sx, sy := float64(r.w)/r.v[2], float64(r.h)/r.v[3]
	return int((b.X0-r.v[0])*sx + 0.5), int((b.Y0-r.v[1])*sy + 0.5), int((b.X1-r.v[0])*sx + 0.5), int((b.Y1-r.v[1])*sy + 0.5)
}

func (r *rasterCtx) paint(n *node, dst *image.NRGBA) {
	switch n.name {
	case "defs", "mask":
		return
	case "path":
		col, ok := fillColor(n.attrs["fill"])
		if !ok {
			return
		}
		info, err := scanPath(n.attrs["d"])
		if err != nil {
			return
		}
		x0, y0, x1, y1 := r.px(info.box)
		draw.Draw(dst, image.Rect(x0, y0, x1, y1).Intersect(dst.Bounds()), image.NewUniform(col), image.Point{}, draw.Src)
		return
	case "g":
		if m, ok := n.attrs["mask"]; ok {
			layer := image.NewNRGBA(dst.Bounds())
			for _, k := range n.kids {
				r.paint(k, layer)
			}
			for _, b := range r.masks[strings.TrimSuffix(strings.TrimPrefix(m, "url(#"), ")")] {
				x0, y0, x1, y1 := r.px(b)
				draw.Draw(layer, image.Rect(x0, y0, x1, y1).Intersect(layer.Bounds()), image.Transparent, image.Point{}, draw.Src)
			}
			draw.Draw(dst, dst.Bounds(), layer, image.Point{}, draw.Over)
			return
		}
	}
	for _, k := range n.kids {
		r.paint(k, dst)
	}
}

func fillColor(f string) (color.NRGBA, bool) {
	switch {
	case strings.HasPrefix(f, "url("):
		return color.NRGBA{128, 128, 128, 255}, true
	case f == "#000":
		return color.NRGBA{0, 0, 0, 255}, true
	case strings.HasPrefix(f, "rgb("):
		p := strings.Split(strings.TrimSuffix(strings.TrimPrefix(f, "rgb("), ")"), ",")
		if len(p) != 3 {
			return color.NRGBA{}, false
		}
		var c [3]uint8
		for i := range c {
			v, _ := strconv.Atoi(strings.TrimSpace(p[i]))
			c[i] = uint8(v)
		}
		return color.NRGBA{c[0], c[1], c[2], 255}, true
	}
	return color.NRGBA{}, false
}

// readTestdata читает файл из testdata.
func readTestdata(t testing.TB, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// newFake — фальшивый растр и опции с ним.
func newFake(t testing.TB) (*fakeRaster, Options) {
	f := &fakeRaster{t: t}
	o := DefaultOptions()
	o.Render = f.Render
	return f, o
}

// mustCut режет файл и падает на ошибке.
func mustCut(t testing.TB, data []byte, o Options) *Result {
	t.Helper()
	res, err := Cut(data, o)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

// svgOf собирает лист w×h с белым листом-фоном и заданными путями (прямоугольниками «x0 y0 x1 y1 rgb»).
func svgOf(w, h int, sheetFill string, rects ...string) []byte {
	var b strings.Builder
	b.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 ` + strconv.Itoa(w) + ` ` + strconv.Itoa(h) + `">`)
	b.WriteString(`<path transform="translate(0,0)" fill="` + sheetFill + `" d="M 0 0 L ` + strconv.Itoa(w) + ` 0 L ` + strconv.Itoa(w) + ` ` + strconv.Itoa(h) + ` L 0 ` + strconv.Itoa(h) + ` L 0 0 z"/>`)
	for _, r := range rects {
		f := strings.Fields(r)
		b.WriteString(`<path transform="translate(0,0)" fill="` + f[4] + `" d="M ` + f[0] + ` ` + f[1] + ` L ` + f[2] + ` ` + f[1] + ` L ` + f[2] + ` ` + f[3] + ` L ` + f[0] + ` ` + f[3] + ` z"/>`)
	}
	b.WriteString(`</svg>`)
	return []byte(b.String())
}
