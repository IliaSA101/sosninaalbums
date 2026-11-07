package model

import "github.com/IliaSA101/sosninaalbums/internal/layout"

// Подрезка, чтобы закрыть лист (104): предел подрезки по стороне, целый процент.

// cropSteps — допустимые значения Settings.Crop, %: 0 — фото целиком.
var cropSteps = [...]int{0, 5, 10, 15}

// ClampCrop — ближайший шаг cropSteps (равное удаление — к меньшему); меньше 0 — 0, больше 15 — 15.
// Значение формы = значение файла: интерфейс дроби не сравнивает.
func ClampCrop(c int) int {
	// Края до abs: у MinInt разность c-s переполняется и даёт 15 вместо 0.
	if c <= 0 {
		return 0
	}
	if c >= 15 {
		return 15
	}
	best := cropSteps[0]
	for _, s := range cropSteps[1:] {
		if abs(c-s) < abs(c-best) {
			best = s
		}
	}
	return best
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// OwnFrame — у какого из фото ids свой кадр (098, запись с Asp > 0): такую ячейку подрезка не трогает.
func (p *Project) OwnFrame(ids []string) []bool {
	own := make([]bool, len(ids))
	for i, id := range ids {
		own[i] = p.Photos[id].Asp > 0
	}
	return own
}

// LockBits — биты своих кадров для layout.Params.Lock: первые 64 (MaxPhotos = 64).
func LockBits(own []bool) uint64 {
	var m uint64
	for i, o := range own[:min(len(own), 64)] {
		if o {
			m |= 1 << uint(i)
		}
	}
	return m
}

// LayoutIn — вход раскладки разворота: пропорции ячеек «Авто» и Params с Lock по своим кадрам.
// Единственный способ собрать пару для layout.Build/Cached/…: Lock без Crop не действует, поэтому
// при Crop = 0 он нулевой и ключи кэша прежние (TestLayoutInOnly держит это в исходниках).
func (p *Project) LayoutIn(ids []string) ([]float64, layout.Params) {
	prm := p.Params()
	if prm.Crop > 0 {
		prm.Lock = LockBits(p.OwnFrame(ids))
	}
	return p.Aspects(ids), prm
}
