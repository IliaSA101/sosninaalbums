//go:build ignore

// qa-icc делает тестовый набор для QA пересчёта профилей ICC (спека 119): из фото папки stock — JPEG
// с встроенными профилями, в которые цвета переведены верно (sRGB → профиль), поэтому после пересчёта
// программа показывает исходные цвета.
//
//	go run scripts/qa-icc.go -src .testphotos/stock -out .testphotos/icc
//
// Набор: icc-p3-1…3 (Display P3; третий — с именем Xiaomi «sRGB EOTF with DCI-P3 Color Gamut»),
// icc-prophoto-1…2, icc-lut (P3 с пустой таблицей A2B0 — значок «!», не пересчитывается),
// icc-linear (линейный sRGB). PNG с iCCP (спека 125): icc-p3.png, icc-prophoto.png, icc-lut.png и
// icc-p3-alpha.png (P3 с плавным градиентом альфы по ширине) — из первых фото папки. WebP в набор не входит
// (проверяется тестами). Папку вывода скрипт не очищает: пишет только свои icc-*.jpg и icc-*.png и
// отказывается работать «на месте». Профили собирает testkit — скрипт запускается только вручную.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
	"github.com/disintegration/imaging"
)

// Колоранты sRGB в XYZ D50 (столбцы — R, G, B), строками.
var srgbToXYZ = [9]float64{0.4360747, 0.3850649, 0.1430804, 0.2225045, 0.7168786, 0.0606169, 0.0139322, 0.0971045, 0.7141733}

type target struct {
	name    string
	profile []byte
	xyz     [9]float64            // колоранты профиля, rX rY rZ gX …
	encode  func(float64) float64 // линейное → значение канала
}

func srgbOETF(l float64) float64 {
	if l <= 0.0031308 {
		return 12.92 * l
	}
	return 1.055*math.Pow(l, 1/2.4) - 0.055
}

func main() {
	src := flag.String("src", ".testphotos/stock", "папка с исходными фото")
	out := flag.String("out", ".testphotos/icc", "куда писать набор")
	flag.Parse()
	if err := run(*src, *out); err != nil {
		fmt.Fprintln(os.Stderr, "qa-icc:", err)
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
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	names, err := photoNames(src)
	if err != nil {
		return err
	}
	desc := testkit.DescV2
	p3 := target{"p3", testkit.ParaProfile(testkit.XYZP3Exact, 3, testkit.SRGBPara3, desc("Display P3")), testkit.XYZP3Exact, srgbOETF}
	xiaomi := target{"p3", testkit.ProfileXiaomiP3(false), testkit.XYZP3Exact, srgbOETF}
	pro := target{"prophoto", testkit.ParaProfile(testkit.XYZProPhoto, 0, []float64{1.8}, desc("ProPhoto RGB")), testkit.XYZProPhoto,
		func(l float64) float64 { return math.Pow(l, 1/1.8) }}
	lut := target{"lut", testkit.LUTProfile(testkit.XYZP3Exact, desc("Display P3 LUT")), testkit.XYZP3Exact, srgbOETF}
	lin := target{"linear", testkit.LinearProfile(testkit.XYZLinearSRGB, desc("Linear sRGB")), testkit.XYZLinearSRGB, func(l float64) float64 { return l }}
	set := []struct {
		file string
		t    target
	}{
		{"icc-p3-1.jpg", p3}, {"icc-p3-2.jpg", p3}, {"icc-p3-3.jpg", xiaomi},
		{"icc-prophoto-1.jpg", pro}, {"icc-prophoto-2.jpg", pro},
		{"icc-lut.jpg", lut}, {"icc-linear.jpg", lin},
	}
	if len(names) < len(set) {
		return fmt.Errorf("в %s фото %d, а нужно %d", src, len(names), len(set))
	}
	for i, s := range set {
		if err := write(filepath.Join(src, names[i]), filepath.Join(out, s.file), s.t); err != nil {
			return err
		}
		fmt.Println(s.file, "←", names[i], "("+s.t.name+")")
	}
	// PNG: те же приёмы, что у JPEG, профиль в iCCP сразу после IHDR.
	pngSet := []struct {
		file  string
		t     target
		alpha bool
	}{
		{"icc-p3.png", p3, false}, {"icc-prophoto.png", pro, false}, {"icc-lut.png", lut, false}, {"icc-p3-alpha.png", p3, true},
	}
	for i, s := range pngSet {
		if err := writePNG(filepath.Join(src, names[i]), filepath.Join(out, s.file), s.t, s.alpha); err != nil {
			return err
		}
		fmt.Println(s.file, "←", names[i], "("+s.t.name+")")
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

// write переводит sRGB-фото в пространство профиля и пишет JPEG q95 с профилем в APP2.
func write(from, to string, t target) error {
	dst, err := convert(from, t)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: 95}); err != nil {
		return err
	}
	data := buf.Bytes()
	res := append(append(append([]byte{}, data[:2]...), testkit.ICCSegment(1, 1, t.profile)...), data[2:]...)
	return os.WriteFile(to, res, 0o644)
}

// writePNG — то же в PNG с чанком iCCP; alpha — непрозрачность растёт слева направо (альфа 30…255, слева почти прозрачно)
// (цвет не премножен: альфа программе не должна меняться).
func writePNG(from, to string, t target, alpha bool) error {
	dst, err := convert(from, t)
	if err != nil {
		return err
	}
	if alpha {
		w := dst.Bounds().Dx()
		for y := range dst.Bounds().Dy() {
			for x := range w {
				dst.Pix[y*dst.Stride+x*4+3] = uint8(30 + 225*x/max(w-1, 1))
			}
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, dst); err != nil {
		return err
	}
	res := testkit.PNGWithChunks(buf.Bytes(), testkit.PNGChunk("iCCP", testkit.PNGICCData(t.profile)))
	return os.WriteFile(to, res, 0o644)
}

// convert переводит sRGB-фото в пространство профиля t (до 1600 px по ширине), альфа 255.
func convert(from string, t target) (*image.NRGBA, error) {
	img, err := imaging.Open(from, imaging.AutoOrientation(true))
	if err != nil {
		return nil, err
	}
	if b := img.Bounds(); b.Dx() > 1600 {
		img = imaging.Resize(img, 1600, 0, imaging.Lanczos) // для QA хватает, файлы легче
	}
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
		var xyz, o [3]float64
		for r := range 3 {
			xyz[r] = srgbToXYZ[r*3]*l[0] + srgbToXYZ[r*3+1]*l[1] + srgbToXYZ[r*3+2]*l[2]
		}
		for r := range 3 {
			v := inv[r*3]*xyz[0] + inv[r*3+1]*xyz[1] + inv[r*3+2]*xyz[2]
			o[r] = math.Max(0, math.Min(1, t.encode(math.Max(0, math.Min(1, v)))))
			dst.Pix[i+r] = uint8(math.Round(o[r] * 255))
		}
		dst.Pix[i+3] = 255
	}
	return dst, nil
}

// inv3 — обратная матрица 3×3 (колоранты невырожденные).
func inv3(a [9]float64) [9]float64 {
	// колоранты заданы как rX rY rZ gX …, матрица профиля — столбцами: C[k][i] = a[i*3+k]
	c := [9]float64{a[0], a[3], a[6], a[1], a[4], a[7], a[2], a[5], a[8]}
	d := c[0]*(c[4]*c[8]-c[5]*c[7]) - c[1]*(c[3]*c[8]-c[5]*c[6]) + c[2]*(c[3]*c[7]-c[4]*c[6])
	return [9]float64{
		(c[4]*c[8] - c[5]*c[7]) / d, (c[2]*c[7] - c[1]*c[8]) / d, (c[1]*c[5] - c[2]*c[4]) / d,
		(c[5]*c[6] - c[3]*c[8]) / d, (c[0]*c[8] - c[2]*c[6]) / d, (c[2]*c[3] - c[0]*c[5]) / d,
		(c[3]*c[7] - c[4]*c[6]) / d, (c[1]*c[6] - c[0]*c[7]) / d, (c[0]*c[4] - c[1]*c[3]) / d,
	}
}
