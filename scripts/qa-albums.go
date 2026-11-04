//go:build ignore

// qa-albums делает набор для проверки exe на раннере Windows и на ноутбуке (спека 118): синтетические
// фото (без личных данных) и два альбома рядом с ними.
//
//	go run scripts/qa-albums.go -out <папка> [-photos 12] [-spreads 6]
//
// В папке: photos/ — JPEG 1600×1067, у каждого свой цвет и крупный номер; Alpha.salbum — папка
// photos + автосборка (-spreads разворотов); Beta.salbum — тот же альбом без двух последних
// разворотов. Пути в альбомах только относительные (WriteRel): папку можно перенести на другой
// диск и на другую ОС. Проект собирается публичным API Editor (AddFolder, AutoBuild,
// DeleteSpreads), файл читается обратно тем же загрузчиком, что и программа. Печатает
// alpha=<путь> и beta=<путь>. Сборка под Windows: GOOS=windows CGO_ENABLED=0 go build -o
// tools/qa-albums.exe scripts/qa-albums.go
package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"os"
	"path/filepath"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/library"
)

const (
	photoW, photoH = 1600, 1067
	// cutSpreads — на сколько разворотов Beta короче Alpha.
	cutSpreads = 2
)

func main() {
	out := flag.String("out", "", "папка набора (создаётся)")
	n := flag.Int("photos", 12, "фото")
	sp := flag.Int("spreads", 6, "разворотов в Alpha (не меньше 3)")
	flag.Parse()
	if *out == "" {
		fmt.Fprintln(os.Stderr, "qa-albums: нужен -out <папка>")
		os.Exit(2)
	}
	alpha, beta, err := run(*out, *n, *sp)
	if err != nil {
		fmt.Fprintln(os.Stderr, "qa-albums:", err)
		os.Exit(1)
	}
	fmt.Printf("alpha=%s\nbeta=%s\n", alpha, beta)
}

// run делает фото и оба альбома; возвращает пути Alpha и Beta.
func run(out string, photos, spreads int) (alpha, beta string, err error) {
	if photos < spreads || photos > model.MaxPhotos*spreads || spreads < cutSpreads+1 || spreads > model.MaxSpreads {
		return "", "", fmt.Errorf("фото %d, разворотов %d: нужно %d..%d разворотов и не меньше фото, чем разворотов",
			photos, spreads, cutSpreads+1, model.MaxSpreads)
	}
	if out, err = filepath.Abs(out); err != nil {
		return "", "", err
	}
	dir := filepath.Join(out, "photos")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", "", err
	}
	for i := range photos {
		if err := writePhoto(filepath.Join(dir, fmt.Sprintf("photo-%02d.jpg", i+1)), i); err != nil {
			return "", "", err
		}
	}
	e, err := album.NewEditor(model.New(), "")
	if err != nil {
		return "", "", err
	}
	if _, _, err := e.AddFolder(dir); err != nil {
		return "", "", err
	}
	infos, err := readInfos(dir)
	if err != nil {
		return "", "", err
	}
	// Поровну на развороты: «до» — округление вверх, «от» — вниз; Series выключен (одна папка).
	per := (photos + spreads - 1) / spreads
	v, err := e.AutoBuild(infos, -1, model.AutoOpts{Spreads: spreads, MaxPer: per, MinPer: photos / spreads})
	if err != nil {
		return "", "", fmt.Errorf("автосборка: %w", err)
	}
	if len(v.Spreads) != spreads {
		return "", "", fmt.Errorf("автосборка дала %d разворотов, ждали %d", len(v.Spreads), spreads)
	}
	alpha, beta = filepath.Join(out, "Alpha.salbum"), filepath.Join(out, "Beta.salbum")
	if err := saveRel(e, alpha, out); err != nil {
		return "", "", err
	}
	ids := make([]string, 0, cutSpreads)
	for _, s := range v.Spreads[spreads-cutSpreads:] {
		ids = append(ids, s.ID)
	}
	e.DeleteSpreads(ids)
	if err := saveRel(e, beta, out); err != nil {
		return "", "", err
	}
	return alpha, beta, verify(alpha, spreads, beta, spreads-cutSpreads)
}

// saveRel пишет проект Editor в path с относительными путями внутри base. Editor.Save добавил бы
// рядом абсолютные пути этой машины: на Windows-раннере им нечего делать.
func saveRel(e *album.Editor, path, base string) error {
	p, side := e.SnapshotSide()
	return salbum.WriteRel(p, path, base, side)
}

// verify читает оба альбома загрузчиком программы: число разворотов и что фото на месте.
func verify(alpha string, na int, beta string, nb int) error {
	for _, c := range []struct {
		path string
		n    int
	}{{alpha, na}, {beta, nb}} {
		p, err := salbum.Load(c.path)
		if err != nil {
			return fmt.Errorf("проверочное чтение %s: %w", filepath.Base(c.path), err)
		}
		if len(p.Spreads) != c.n {
			return fmt.Errorf("%s: разворотов %d, ждали %d", filepath.Base(c.path), len(p.Spreads), c.n)
		}
		for _, s := range p.Spreads {
			if len(s.Photos) == 0 {
				return fmt.Errorf("%s: пустой разворот", filepath.Base(c.path))
			}
		}
	}
	return nil
}

// readInfos — сведения о фото папки по порядку имён (так их видит лента).
func readInfos(dir string) ([]model.PhotoInfo, error) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var list []model.PhotoInfo
	for _, en := range ents {
		if en.IsDir() || !library.IsImage(en.Name()) {
			continue
		}
		path := filepath.Join(dir, en.Name())
		ph, err := library.ReadInfo(path)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", en.Name(), err)
		}
		list = append(list, model.PhotoInfo{ID: library.IDFor(path), Path: path, W: ph.W, H: ph.H, Size: ph.Size})
	}
	return list, nil
}

// writePhoto рисует кадр i: свой цвет по кругу оттенков, светлая рамка и номер цифрами 5×7.
func writePhoto(path string, i int) error {
	img := image.NewRGBA(image.Rect(0, 0, photoW, photoH))
	bg := hue(i)
	draw.Draw(img, img.Bounds(), &image.Uniform{bg}, image.Point{}, draw.Src)
	frame := color.RGBA{250, 250, 250, 255}
	for t := range 12 {
		for x := range photoW {
			img.Set(x, t, frame)
			img.Set(x, photoH-1-t, frame)
		}
		for y := range photoH {
			img.Set(t, y, frame)
			img.Set(photoW-1-t, y, frame)
		}
	}
	num := fmt.Sprintf("%d", i+1)
	const scale = 36
	w := len(num)*6*scale - scale
	x0, y0 := (photoW-w)/2, (photoH-7*scale)/2
	for k, ch := range num {
		drawDigit(img, x0+k*6*scale, y0, int(ch-'0'), scale, frame)
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := jpeg.Encode(f, img, &jpeg.Options{Quality: 88}); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// hue — цвет кадра i: оттенок шагом 47° (соседние кадры не похожи), средняя насыщенность.
func hue(i int) color.RGBA {
	h := float64(i*47%360) / 60
	const s, v = 0.55, 0.62
	c := v * s
	x := c * (1 - abs(mod2(h)-1))
	m := v - c
	var r, g, b float64
	switch int(h) {
	case 0:
		r, g, b = c, x, 0
	case 1:
		r, g, b = x, c, 0
	case 2:
		r, g, b = 0, c, x
	case 3:
		r, g, b = 0, x, c
	case 4:
		r, g, b = x, 0, c
	default:
		r, g, b = c, 0, x
	}
	return color.RGBA{uint8((r + m) * 255), uint8((g + m) * 255), uint8((b + m) * 255), 255}
}

func mod2(f float64) float64 { return f - 2*float64(int(f/2)) }

func abs(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}

// digits — цифры 5×7, строки по пять бит слева направо.
var digits = [10][7]uint8{
	{0b01110, 0b10001, 0b10011, 0b10101, 0b11001, 0b10001, 0b01110},
	{0b00100, 0b01100, 0b00100, 0b00100, 0b00100, 0b00100, 0b01110},
	{0b01110, 0b10001, 0b00001, 0b00010, 0b00100, 0b01000, 0b11111},
	{0b11111, 0b00010, 0b00100, 0b00010, 0b00001, 0b10001, 0b01110},
	{0b00010, 0b00110, 0b01010, 0b10010, 0b11111, 0b00010, 0b00010},
	{0b11111, 0b10000, 0b11110, 0b00001, 0b00001, 0b10001, 0b01110},
	{0b00110, 0b01000, 0b10000, 0b11110, 0b10001, 0b10001, 0b01110},
	{0b11111, 0b00001, 0b00010, 0b00100, 0b01000, 0b01000, 0b01000},
	{0b01110, 0b10001, 0b10001, 0b01110, 0b10001, 0b10001, 0b01110},
	{0b01110, 0b10001, 0b10001, 0b01111, 0b00001, 0b00010, 0b01100},
}

func drawDigit(img *image.RGBA, x0, y0, d, scale int, c color.RGBA) {
	for row, bits := range digits[d] {
		for col := range 5 {
			if bits>>(4-col)&1 == 0 {
				continue
			}
			r := image.Rect(x0+col*scale, y0+row*scale, x0+(col+1)*scale, y0+(row+1)*scale)
			draw.Draw(img, r, &image.Uniform{c}, image.Point{}, draw.Src)
		}
	}
}
