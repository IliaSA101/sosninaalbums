package faces

import (
	"fmt"
	"image"
)

const (
	// minScore — порог уверенности. На детских портретах и стоке проверен: ниже, в полосе 0,6–0,8,
	// лежат профили и ложные срабатывания (букет — 0,60).
	minScore = 0.8
	// nmsIoU — склейка рамок одного лица, как в cv::FaceDetectorYN.
	nmsIoU = 0.3
	// Середина глаз в настоящем лице — на 0,2…0,7 высоты рамки от её верха (у лиц ≥ 0,8 на стоке:
	// p5 0,32, p95 0,45; лицо, смотрящее вниз, — 0,55). У ложной «рамки без лица» (руки с кольцами,
	// stock-57) — 0,046: фильтр снимает класс ошибок, который порог оценки не ловит.
	eyeMin, eyeMax = 0.2, 0.7
)

// Pt — точка в долях картинки.
type Pt struct{ X, Y float64 }

// Rect — рамка в долях картинки: левый верхний угол и размер.
type Rect struct{ X, Y, W, H float64 }

// Face — найденное лицо. Всё в долях 0…1 картинки, поданной в Detect (рамка у края может выходить
// за 0…1). Eyes[0] — глаз слева на снимке, Mouth[0] — угол рта слева.
type Face struct {
	Score float64
	Box   Rect
	Eyes  [2]Pt
	Nose  Pt
	Mouth [2]Pt
}

// EyeLine — середина между глазами.
func (f Face) EyeLine() Pt {
	return Pt{(f.Eyes[0].X + f.Eyes[1].X) / 2, (f.Eyes[0].Y + f.Eyes[1].Y) / 2}
}

// plausible — фильтр «середина глаз на 0,2…0,7 высоты рамки».
func (f Face) plausible() bool {
	if f.Box.H <= 0 || f.Box.W <= 0 {
		return false
	}
	e := (f.EyeLine().Y - f.Box.Y) / f.Box.H
	return e >= eyeMin && e <= eyeMax
}

// Detect ищет лица на картинке в один поток: порог 0,8, фильтр глаз, склейка 0,3; лица — по
// убыванию оценки. Картинку уменьшают до длинной стороны 640 с усреднением. Пустая, чрезмерно
// большая картинка и сбой сети дают nil. Безопасен для вызова из нескольких горутин: сеть
// после загрузки только читается.
func Detect(img image.Image) []Face {
	fs, _ := detect(img, 1, &memTrack{})
	return fs
}

// DetectN — Detect с делением свёрток между workers потоками; результат тот же до бита.
func DetectN(img image.Image, workers int) []Face {
	fs, _ := detect(img, workers, &memTrack{})
	return fs
}

func detect(img image.Image, workers int, tr *memTrack) ([]Face, error) {
	all, err := candidates(img, workers, tr)
	if err != nil {
		return nil, err
	}
	fs := all[:0]
	for _, f := range all {
		if f.plausible() {
			fs = append(fs, f)
		}
	}
	return nms(fs, nmsIoU), nil
}

// candidates — все срабатывания не ниже порога, ещё без фильтра глаз и склейки.
func candidates(img image.Image, workers int, tr *memTrack) ([]Face, error) {
	if img == nil {
		return nil, nil
	}
	n, err := loadNet()
	if err != nil {
		return nil, err
	}
	in, ok := prepareInput(img, inputLong)
	if !ok {
		return nil, nil
	}
	outs, err := n.run(in.d, in.H, in.W, workers, tr)
	if err != nil {
		return nil, fmt.Errorf("faces: %w", err)
	}
	cs, err := decode(outs, in.W, in.H, minScore)
	if err != nil {
		return nil, err
	}
	fs := make([]Face, len(cs))
	for i, c := range cs {
		fs[i] = c.face(in.w, in.h)
	}
	return fs, nil
}

// face переводит кандидата из пикселей входа в доли картинки (w×h — картинка внутри входа).
func (c cand) face(w, h int) Face {
	fw, fh := float64(w), float64(h)
	pt := func(i int) Pt { return Pt{c.pts[i][0] / fw, c.pts[i][1] / fh} }
	return Face{
		Score: c.score,
		Box:   Rect{c.x / fw, c.y / fh, c.w / fw, c.h / fh},
		Eyes:  [2]Pt{pt(0), pt(1)},
		Nose:  pt(2),
		Mouth: [2]Pt{pt(3), pt(4)},
	}
}
