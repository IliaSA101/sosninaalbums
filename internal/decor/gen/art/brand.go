package art

import (
	"bytes"
	"embed"
	"image/color"
	"image/png"
	"sync"

	"github.com/IliaSA101/sosninaalbums/internal/decor/gen/paint"
)

// Фирменные стикеры (спека 116): готовые PNG владельца, вшитые в программу. Живут как
// процедурные ключи — в альбом уходит Art{Kind}, поэтому альбом открывается и выгружается на
// любом компьютере без файла-источника (решение №99). Растр рисует paint.Canvas.Raster.

//go:embed brand
var brandFS embed.FS

// brandFiles — ключ → файл; порядок — порядок каталога. Ключ и файл выпущенного не меняются.
var brandFiles = []struct{ kind, name, file string }{
	{"brand-albums-black", "sosnina albums, чёрный", "brand/brand-albums-black.png"},
	{"brand-albums-white", "sosnina albums, белый", "brand/brand-albums-white.png"},
	{"brand-films-black", "sosnina films, чёрный", "brand/brand-films-black.png"},
	{"brand-films-white", "sosnina films, белый", "brand/brand-films-white.png"},
}

// brandDefs — стикеры-картинки для All. Пропорция — из заголовка PNG (на каталог файл не
// декодируется целиком); картинка разбирается при первом рисовании.
func brandDefs() []Def { return brandDefsFrom(brandFS.ReadFile) }

// brandDefsFrom — brandDefs с чтением файлов снаружи: тест подсовывает битые данные.
func brandDefsFrom(read func(string) ([]byte, error)) []Def {
	out := make([]Def, 0, len(brandFiles))
	for _, f := range brandFiles {
		data, err := read(f.file)
		if err != nil {
			panic("art: нет вшитого " + f.file)
		}
		cfg, err := png.DecodeConfig(bytes.NewReader(data))
		if err != nil || cfg.Height == 0 {
			panic("art: " + f.file + " не PNG")
		}
		r := sync.OnceValue(func() *paint.Raster {
			img, err := png.Decode(bytes.NewReader(data))
			if err != nil {
				return nil
			}
			return paint.NewRaster(img)
		})
		out = append(out, Def{
			Kind: f.kind, Name: f.name, Aspect: float64(cfg.Width) / float64(cfg.Height), Image: true,
			Draw: func(c *paint.Canvas, _ Spec, _ []color.NRGBA, a float64) { c.Raster(r(), a, 1) },
		})
	}
	return out
}
