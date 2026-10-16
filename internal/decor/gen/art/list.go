// Package art — стикеры программы (спека 048, 052 §2): рисуются заливками на холсте paint в
// «дизайн-пространстве» (0..a)×(0..1) — высота рамки 1, ширина её пропорция a, поэтому круг
// остаётся кругом при любой рамке. Выпущенный стикер не меняется (эталоны gen/testdata/golden.json);
// правка — новый ключ. Реестр, Spec и Draw* — в пакете gen.
package art

import (
	"image/color"

	"github.com/IliaSA101/sosninaalbums/internal/decor/gen/paint"
)

type pt = paint.Pt

// Spec — что стикеру нужно из gen.Spec: ключ и сид для своего ГСЧ (paint.CellRand).
type Spec struct {
	Kind string
	Seed uint32
}

// Def — стикер программы: строка каталога и рисование в пространстве (0..a)×(0..1); у
// растяжимых (Stretch) a — пропорция рамки в модели, у остальных — Aspect.
type Def struct {
	Kind, Name      string
	Inks            []string // цвета по умолчанию; len — сколько пикеров
	Aspect          float64  // ширина/высота (у растяжимого — для первой вставки)
	Random, Stretch bool     // есть «Перемешать»; рисуется заново в рамке
	Draw            func(c *paint.Canvas, s Spec, ink []color.NRGBA, a float64)
}

// All — стикеры в порядке каталога (выпущенный ключ не меняется; новый — в конец).
func All() []Def {
	return []Def{
		{Kind: "star", Name: "Звезда", Inks: []string{"#f2c14e"}, Aspect: 1, Draw: drawStar},
		{Kind: "burst", Name: "Взрыв", Inks: []string{"#f06a5b", "#ffd166"}, Aspect: 1, Draw: drawBurst},
		{Kind: "sparkle", Name: "Искра", Inks: []string{"#f2c14e", "#f9e2a0"}, Aspect: 1, Draw: drawSparkle},
		{Kind: "heart", Name: "Сердце", Inks: []string{"#e8607a", "#f7b6c2"}, Aspect: 1, Draw: drawHeart},
		{Kind: "hearts-pair", Name: "Пара сердец", Inks: []string{"#e8607a", "#f2a0b4"}, Aspect: 1.2, Draw: drawHeartsPair},
		{Kind: "circle", Name: "Круг", Inks: []string{"#e9c8b8"}, Aspect: 1, Draw: drawCircle},
		{Kind: "blob", Name: "Пятно", Inks: []string{"#cfe0d6"}, Aspect: 1.2, Random: true, Draw: drawBlob},
		{Kind: "plate", Name: "Плашка", Inks: []string{"#fffaf0", "#d9b779"}, Aspect: 3, Stretch: true, Draw: drawPlate},
		{Kind: "banner", Name: "Лента-баннер", Inks: []string{"#e8607a", "#b8435a"}, Aspect: 3, Draw: drawBanner},
		{Kind: "wave-band", Name: "Волны", Inks: []string{"#d3e6f1", "#a9cce1", "#7fb2d1"}, Aspect: 4, Stretch: true, Draw: drawWaveBand},
		{Kind: "rainbow", Name: "Радуга", Inks: []string{"#ef7b7b", "#f6c177", "#9ccc9c", "#8fb8e0"}, Aspect: 2, Draw: drawRainbow},
		{Kind: "arches", Name: "Бохо-арки", Inks: []string{"#e7c9b4", "#d4a98f", "#b98466"}, Aspect: 0.85, Draw: drawArches},
		{Kind: "corner", Name: "Уголок для фото", Inks: []string{"#c9b79c"}, Aspect: 1, Draw: drawCorner},
		{Kind: "brush-stroke", Name: "Мазок", Inks: []string{"#f2c6b4"}, Aspect: 3, Random: true, Draw: drawStroke},
		{Kind: "arrow", Name: "Стрелка", Inks: []string{"#6b6b6b"}, Aspect: 2, Draw: drawArrow},
		{Kind: "confetti-pile", Name: "Горсть конфетти", Inks: []string{"#f2b134", "#e85d75", "#4fb0c6", "#7cc47f"}, Aspect: 1, Random: true, Draw: drawConfettiPile},
		{Kind: "flower", Name: "Цветок", Inks: []string{"#f09bb0", "#f6d365"}, Aspect: 1, Draw: drawFlower},
		{Kind: "daisy", Name: "Ромашка", Inks: []string{"#f8ecd8", "#f6c445"}, Aspect: 1, Draw: drawDaisy},
		{Kind: "leaf", Name: "Лист", Inks: []string{"#7fb685", "#4f8f5f"}, Aspect: 0.6, Draw: drawLeaf},
		{Kind: "branch", Name: "Ветка", Inks: []string{"#7a8f6b", "#a8c69f"}, Aspect: 0.7, Draw: drawBranch},
		{Kind: "sun", Name: "Солнце", Inks: []string{"#f6b93b", "#f9d77e"}, Aspect: 1, Draw: drawSun},
		{Kind: "cloud", Name: "Облако", Inks: []string{"#cfe3f2", "#e6f0f8"}, Aspect: 1.6, Draw: drawCloud},
		{Kind: "balloon", Name: "Шар", Inks: []string{"#e85d75", "#9a9a9a"}, Aspect: 0.8, Draw: drawBalloon},
		{Kind: "balloons", Name: "Связка шаров", Inks: []string{"#e85d75", "#f2b134", "#4fb0c6"}, Aspect: 1, Draw: drawBalloons},
		{Kind: "cap", Name: "Шапка выпускника", Inks: []string{"#2f3a56", "#e0a93d"}, Aspect: 1.4, Draw: drawCap},
		{Kind: "pencil", Name: "Карандаш", Inks: []string{"#f2b134", "#f0d3a8", "#3d3d3d"}, Aspect: 4, Draw: drawPencil},
		{Kind: "book", Name: "Книга", Inks: []string{"#fffdf6", "#6b8fb8"}, Aspect: 1.4, Draw: drawBook},
		{Kind: "apple", Name: "Яблоко", Inks: []string{"#e24b4b", "#5f9e5f"}, Aspect: 0.95, Draw: drawApple},
		{Kind: "bell", Name: "Колокольчик", Inks: []string{"#e0b04a", "#8a6a2a"}, Aspect: 0.95, Draw: drawBell},
		{Kind: "rings", Name: "Кольца", Inks: []string{"#d9b779", "#c9a24e"}, Aspect: 1.5, Draw: drawRings},
	}
}
