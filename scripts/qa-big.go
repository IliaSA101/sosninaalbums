//go:build ignore

// qa-big делает тестовый набор для QA ступени 2 крупного просмотра (спека 128): кадры длинной стороной
// 4000 px — без них оригинал не включается (превью 2048, предел зума считается по ширине фото).
// Без внешних загрузок: из фото папки stock увеличением (Lanczos).
//
//	go run scripts/qa-big.go -src .testphotos/stock -out .testphotos/big
//
// Набор: big-srgb-1…3 (sRGB, без профиля, пропорции исходника), big-srgb-4-portrait (кадр 3:4 по высоте 4000 —
// «другие пропорции» при листании), big-p3 (Display P3) и big-prophoto (ProPhoto): у них оригинал не просят.
// Папку вывода скрипт не очищает: пишет только свои big-*.jpg и отказывается работать «на месте».
// Профили собирает testkit — скрипт запускается только вручную.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"image"
	"image/jpeg"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
	"github.com/disintegration/imaging"
)

const longSide = 4000

// Колоранты sRGB в XYZ D50, строками.
var srgbToXYZ = [9]float64{0.4360747, 0.3850649, 0.1430804, 0.2225045, 0.7168786, 0.0606169, 0.0139322, 0.0971045, 0.7141733}

type target struct {
	profile []byte
	xyz     [9]float64
	encode  func(float64) float64
}

func srgbOETF(l float64) float64 {
	if l <= 0.0031308 {
		return 12.92 * l
	}
	return 1.055*math.Pow(l, 1/2.4) - 0.055
}

func main() {
	src := flag.String("src", ".testphotos/stock", "папка с исходными фото")
	out := flag.String("out", ".testphotos/big", "куда писать набор")
	flag.Parse()
	if err := run(*src, *out); err != nil {
		fmt.Fprintln(os.Stderr, "qa-big:", err)
		os.Exit(1)
	}
}

func run(srcDir, outDir string) error {
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
	names, err := photoNames(src)
	if err != nil {
		return err
	}
	if len(names) < 5 {
		return fmt.Errorf("в %s фото %d, а нужно 5", src, len(names))
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	desc := testkit.DescV2
	p3 := &target{testkit.ParaProfile(testkit.XYZP3Exact, 3, testkit.SRGBPara3, desc("Display P3")), testkit.XYZP3Exact, srgbOETF}
	pro := &target{testkit.ParaProfile(testkit.XYZProPhoto, 0, []float64{1.8}, desc("ProPhoto RGB")), testkit.XYZProPhoto,
		func(l float64) float64 { return math.Pow(l, 1/1.8) }}
	set := []struct {
		file     string
		t        *target // nil — sRGB без профиля
		portrait bool
	}{
		{"big-srgb-1.jpg", nil, false}, {"big-srgb-2.jpg", nil, false}, {"big-srgb-3.jpg", nil, false},
		{"big-srgb-4-portrait.jpg", nil, true}, {"big-p3.jpg", p3, false}, {"big-prophoto.jpg", pro, false},
	}
	for i, s := range set {
		from := names[i%len(names)]
		if err := write(filepath.Join(src, from), filepath.Join(out, s.file), s.t, s.portrait); err != nil {
			return err
		}
		fmt.Println(s.file, "←", from)
	}
	return nil
}

func photoNames(dir string) ([]string, error) {
	es, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range es {
		if n := strings.ToLower(e.Name()); !e.IsDir() && (strings.HasSuffix(n, ".jpg") || strings.HasSuffix(n, ".jpeg")) {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	return out, nil
}

// write увеличивает фото до 4000 px по длинной стороне (portrait — кадр 3:4 высотой 4000), при профиле
// переводит цвета в его пространство и пишет JPEG q92 (с профилем в APP2).
func write(from, to string, t *target, portrait bool) error {
	img, err := imaging.Open(from, imaging.AutoOrientation(true))
	if err != nil {
		return err
	}
	if portrait {
		img = imaging.Fill(img, 3000, longSide, imaging.Center, imaging.Lanczos)
	} else if b := img.Bounds(); b.Dx() >= b.Dy() {
		img = imaging.Resize(img, longSide, 0, imaging.Lanczos)
	} else {
		img = imaging.Resize(img, 0, longSide, imaging.Lanczos)
	}
	var dst image.Image = img
	if t != nil {
		dst = convert(img, t)
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: 92}); err != nil {
		return err
	}
	data := buf.Bytes()
	if t != nil {
		data = append(append(append([]byte{}, data[:2]...), testkit.ICCSegment(1, 1, t.profile)...), data[2:]...)
	}
	return os.WriteFile(to, data, 0o644)
}

// convert переводит sRGB в пространство профиля t (приём тот же, что в qa-icc.go).
func convert(img image.Image, t *target) *image.NRGBA {
	src := imaging.Clone(img)
	inv := inv3(t.xyz)
	lut := func(v uint8) float64 {
		c := float64(v) / 255
		if c <= 0.04045 {
			return c / 12.92
		}
		return math.Pow((c+0.055)/1.055, 2.4)
	}
	dst := image.NewNRGBA(src.Bounds())
	for i := 0; i+3 < len(src.Pix); i += 4 {
		l := [3]float64{lut(src.Pix[i]), lut(src.Pix[i+1]), lut(src.Pix[i+2])}
		var xyz [3]float64
		for r := range 3 {
			xyz[r] = srgbToXYZ[r*3]*l[0] + srgbToXYZ[r*3+1]*l[1] + srgbToXYZ[r*3+2]*l[2]
		}
		for r := range 3 {
			v := inv[r*3]*xyz[0] + inv[r*3+1]*xyz[1] + inv[r*3+2]*xyz[2]
			o := math.Max(0, math.Min(1, t.encode(math.Max(0, math.Min(1, v)))))
			dst.Pix[i+r] = uint8(math.Round(o * 255))
		}
		dst.Pix[i+3] = 255
	}
	return dst
}

// inv3 — обратная матрица 3×3 (колоранты заданы как rX rY rZ gX …, матрица профиля — столбцами).
func inv3(a [9]float64) [9]float64 {
	c := [9]float64{a[0], a[3], a[6], a[1], a[4], a[7], a[2], a[5], a[8]}
	d := c[0]*(c[4]*c[8]-c[5]*c[7]) - c[1]*(c[3]*c[8]-c[5]*c[6]) + c[2]*(c[3]*c[7]-c[4]*c[6])
	return [9]float64{
		(c[4]*c[8] - c[5]*c[7]) / d, (c[2]*c[7] - c[1]*c[8]) / d, (c[1]*c[5] - c[2]*c[4]) / d,
		(c[5]*c[6] - c[3]*c[8]) / d, (c[0]*c[8] - c[2]*c[6]) / d, (c[2]*c[3] - c[0]*c[5]) / d,
		(c[3]*c[7] - c[4]*c[6]) / d, (c[1]*c[6] - c[0]*c[7]) / d, (c[0]*c[4] - c[1]*c[3]) / d,
	}
}
