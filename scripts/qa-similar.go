//go:build ignore

// qa-similar делает тестовый набор для QA «похожих кадров» (078): из n фото папки stock — по четыре
// файла серии sim-NN-a…d.jpg, которые лежат рядом в порядке имён.
//
//	go run scripts/qa-similar.go -src .testphotos/stock -out .testphotos/similar -n 20
//
// a — оригинал, b — сдвиг рамки на 3 %, c — ближе на 5 % с каждой стороны и ярче на 10 %,
// d — темнее на 15 % (пересжатие q70). Это те же виды почти дублей, на которых меряли пороги
// уровней (research §2). По итогу печатается расстояние imgsig.Dist от оригинала до каждого.
// Папку вывода скрипт не очищает: пишет только свои sim-*.jpg и отказывается работать «на месте».
package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/library/imgsig"
	"github.com/disintegration/imaging"
)

func main() {
	src := flag.String("src", ".testphotos/stock", "папка с исходными фото")
	out := flag.String("out", ".testphotos/similar", "куда писать набор")
	n := flag.Int("n", 20, "сколько исходных фото (серий)")
	flag.Parse()
	if err := run(*src, *out, *n); err != nil {
		fmt.Fprintln(os.Stderr, "qa-similar:", err)
		os.Exit(1)
	}
}

func run(srcDir, outDir string, n int) error {
	src, err := filepath.Abs(srcDir)
	if err != nil {
		return err
	}
	out, err := filepath.Abs(outDir)
	if err != nil {
		return err
	}
	if src == out {
		return fmt.Errorf("-src и -out — одна папка")
	}
	if n < 1 || n > 99 {
		return fmt.Errorf("-n от 1 до 99 (имя серии sim-NN)")
	}
	names, err := photoNames(src)
	if err != nil {
		return err
	}
	if len(names) < n {
		return fmt.Errorf("в %s фото %d, а нужно %d", src, len(names), n)
	}
	if err := os.MkdirAll(out, 0o750); err != nil {
		return err
	}
	for i, name := range names[:n] {
		img, err := imaging.Open(filepath.Join(src, name), imaging.AutoOrientation(true))
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		variants := []struct {
			suffix string
			img    image.Image
			q      int
		}{
			{"a", img, 92},
			{"b", crop(img, 0.03, 0.01, 0, 0.02), 92},
			{"c", scale(crop(img, 0.05, 0.05, 0.05, 0.05), 1.10), 92},
			{"d", scale(img, 0.85), 70},
		}
		var base imgsig.Sig
		fmt.Printf("sim-%02d (%s):", i+1, name)
		for k, v := range variants {
			f := filepath.Join(out, fmt.Sprintf("sim-%02d-%s.jpg", i+1, v.suffix))
			if err := imaging.Save(v.img, f, imaging.JPEGQuality(v.q)); err != nil {
				return err
			}
			s := imgsig.Of(imaging.Fit(v.img, 400, 400, imaging.Linear))
			if k == 0 {
				base = s
				continue
			}
			fmt.Printf(" %s=%d", v.suffix, imgsig.Dist(base, s))
		}
		fmt.Println()
	}
	fmt.Printf("%s: %d серий по 4 фото\n", out, n)
	return nil
}

// photoNames — JPEG в папке (без подпапок) по порядку имён.
func photoNames(dir string) ([]string, error) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range ents {
		switch strings.ToLower(filepath.Ext(e.Name())) {
		case ".jpg", ".jpeg":
			if !e.IsDir() {
				names = append(names, e.Name())
			}
		}
	}
	sort.Strings(names)
	return names, nil
}

// crop отрезает от краёв доли l, t, r, b ширины/высоты (разрешение остаётся прежним).
func crop(img image.Image, l, t, r, b float64) image.Image {
	bb := img.Bounds()
	w, h := float64(bb.Dx()), float64(bb.Dy())
	return imaging.Crop(img, image.Rect(int(l*w), int(t*h), int((1-r)*w), int((1-b)*h)))
}

// scale умножает яркость на k.
func scale(img image.Image, k float64) image.Image {
	mul := func(v uint8) uint8 { return uint8(min(255, float64(v)*k)) }
	return imaging.AdjustFunc(img, func(c color.NRGBA) color.NRGBA {
		return color.NRGBA{mul(c.R), mul(c.G), mul(c.B), c.A}
	})
}
