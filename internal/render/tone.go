package render

import (
	"image"
	"math"
)

// Tone — ч/б и тон фото в месте (068); поля — из FrameView (render не видит model, depguard).
// Формула — CSS-фильтры Chromium в порядке строки model.Tone.CSS: grayscale(1) по Rec.709,
// brightness v·(100+Bright)/100, contrast (v−0,5)·(100+Contrast)/100 + 0,5, зажим 0…1 после
// каждого шага. Совпадение строки и пикселей держит TestToneCSSMatchesRender в export.
type Tone struct {
	Gray             bool
	Bright, Contrast int
}

// toneSub — дробная точность входа таблицы: серый в целых теряет до 0,5 уровня, контраст и
// яркость умножают это до 2,25 раза — с 1/16 уровня итог от формулы в float не дальше 1.
const toneSub = 16

// toneTable — таблица шагов 2–3 по входу v·toneSub, v — 0…255.
func toneTable(t Tone) []uint8 {
	kb, kc := float64(100+t.Bright)/100, float64(100+t.Contrast)/100
	tbl := make([]uint8, 255*toneSub+1)
	for i := range tbl {
		v := min(float64(i)/(255*toneSub)*kb, 1)
		v = min(max((v-0.5)*kc+0.5, 0), 1)
		tbl[i] = uint8(math.Round(v * 255))
	}
	return tbl
}

// applyTone меняет цвет пикселей im по тону t; альфа стоит. Нулевой тон — без обхода (прежний
// путь байт-в-байт). im — своя копия пикселей (Crop/Resize/padEdge), чужое не трогаем.
func applyTone(im *image.NRGBA, t Tone) {
	if im == nil || t == (Tone{}) {
		return
	}
	tbl := toneTable(t)
	w := im.Rect.Dx()
	for y := range im.Rect.Dy() {
		row := im.Pix[y*im.Stride : y*im.Stride+4*w]
		for i := 0; i+3 < len(row); i += 4 {
			if t.Gray {
				// Rec.709 в целых с точностью 1/toneSub: (2126R + 7152G + 722B)·16/10000.
				l := (2126*uint32(row[i]) + 7152*uint32(row[i+1]) + 722*uint32(row[i+2])) * toneSub
				g := tbl[(l+5000)/10000]
				row[i], row[i+1], row[i+2] = g, g, g
				continue
			}
			row[i], row[i+1], row[i+2] = tbl[uint32(row[i])*toneSub], tbl[uint32(row[i+1])*toneSub], tbl[uint32(row[i+2])*toneSub]
		}
	}
}
