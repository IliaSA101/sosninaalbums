package imgsig

import (
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// stockDir ищет .testphotos/stock вверх от папки теста (фото вне git, лежат в основном дереве); "" — нет.
func stockDir() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		p := filepath.Join(dir, ".testphotos", "stock")
		if st, err := os.Stat(p); err == nil && st.IsDir() {
			return p
		}
		up := filepath.Dir(dir)
		if up == dir {
			return ""
		}
		dir = up
	}
}

// halve берёт каждый второй пиксель: быстрый черновик 1000 px для вырезок (подпись считается
// уже по миниатюре 400 px, как в кэше).
func halve(src image.Image) *image.RGBA {
	b := src.Bounds()
	out := image.NewRGBA(image.Rect(0, 0, b.Dx()/2, b.Dy()/2))
	for y := range out.Rect.Dy() {
		for x := range out.Rect.Dx() {
			r, g, bl, _ := src.At(b.Min.X+2*x, b.Min.Y+2*y).RGBA()
			out.SetRGBA(x, y, color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(bl >> 8), 255})
		}
	}
	return out
}

// shrink вырезает окно v и уменьшает его средним по площади до длинной стороны 400, k — яркость.
func shrink(src *image.RGBA, v view, k float64) *image.RGBA {
	b := src.Bounds()
	x0, y0 := int(v.x*float64(b.Dx())), int(v.y*float64(b.Dy()))
	cw, ch := int(v.w*float64(b.Dx())), int(v.h*float64(b.Dy()))
	w, h := 400, max(1, ch*400/cw)
	if ch > cw {
		w, h = max(1, cw*400/ch), 400
	}
	out := image.NewRGBA(image.Rect(0, 0, w, h))
	for ty := range h {
		for tx := range w {
			sx0, sx1 := x0+tx*cw/w, x0+max((tx+1)*cw/w, tx*cw/w+1)
			sy0, sy1 := y0+ty*ch/h, y0+max((ty+1)*ch/h, ty*ch/h+1)
			var sum [3]float64
			n := 0
			for y := sy0; y < sy1; y++ {
				for x := sx0; x < sx1; x++ {
					p := src.Pix[src.PixOffset(x, y):]
					sum[0], sum[1], sum[2] = sum[0]+float64(p[0]), sum[1]+float64(p[1]), sum[2]+float64(p[2])
					n++
				}
			}
			out.SetRGBA(tx, ty, color.RGBA{
				clamp8(sum[0] / float64(n) * k), clamp8(sum[1] / float64(n) * k), clamp8(sum[2] / float64(n) * k), 255,
			})
		}
	}
	return out
}

// На живых кадрах stock (Pexels, docs/qa/stock-photos.md) цифры research §2: разные кадры не ближе
// порога «Похожие», почти дубли — в своих уровнях. Папки нет (чистый клон) — пропуск.
func TestDistStock(t *testing.T) {
	dir := stockDir()
	if dir == "" {
		t.Skip("нет .testphotos/stock")
	}
	files, _ := filepath.Glob(filepath.Join(dir, "*.jpg"))
	slices.Sort(files)
	n := 30
	if testing.Short() {
		n = 12
	}
	if len(files) < n {
		t.Skipf("в stock %d фото, нужно %d", len(files), n)
	}
	var sigs []Sig
	var dup, near, zoom []int
	for _, f := range files[:n] {
		fh, err := os.Open(f)
		if err != nil {
			t.Fatal(err)
		}
		full, err := jpeg.Decode(fh)
		_ = fh.Close()
		if err != nil {
			t.Fatalf("%s: %v", filepath.Base(f), err)
		}
		src := halve(full)
		a := Of(thumb(t, shrink(src, view{0, 0, 1, 1}, 1), 82))
		sigs = append(sigs, a)
		dup = append(dup, Dist(a, Of(thumb(t, shrink(src, view{0, 0, 1, 1}, 0.85), 70))))
		near = append(near, Dist(a, Of(thumb(t, shrink(src, view{0.03, 0.01, 0.97, 0.97}, 1), 82))))
		zoom = append(zoom, Dist(a, Of(thumb(t, shrink(src, view{0.05, 0.05, 0.9, 0.9}, 1.1), 82))))
	}
	lowest := 128
	for i := range sigs {
		for j := i + 1; j < len(sigs); j++ {
			lowest = min(lowest, Dist(sigs[i], sigs[j]))
		}
	}
	t.Logf("stock %d фото: разные — минимум %d; темнее ≤ %d, сдвиг ≤ %d, зум ≤ %d",
		n, lowest, slices.Max(dup), slices.Max(near), slices.Max(zoom))
	if lowest <= DistSimilar {
		t.Errorf("разные кадры сошлись на Dist %d (≤ %d)", lowest, DistSimilar)
	}
	if m := slices.Max(dup); m > DistDup {
		t.Errorf("темнее на 15 %%: худший Dist %d > %d", m, DistDup)
	}
	if m := slices.Max(near); m > DistNear {
		t.Errorf("сдвиг 3 %%: худший Dist %d > %d", m, DistNear)
	}
	slices.Sort(zoom)
	if m := zoom[len(zoom)/2]; m > DistNear {
		t.Errorf("ближе на 5 %%: медиана Dist %d > %d", m, DistNear)
	}
}
