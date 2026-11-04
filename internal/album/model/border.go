package model

import (
	"errors"
	"math"
	"slices"

	"github.com/IliaSA101/sosninaalbums/internal/decor/gen"
	"github.com/IliaSA101/sosninaalbums/internal/geom"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// Рамочки мест и прозрачность фото (спека 079, design §1–2, v14).

// Border — рамочка места (079): встроенная (Kind — ключ вида decor/gen) или картинка из
// библиотеки (Photo — ID записи Project.Photos, обычно с Lib); ровно одно. Лежит внутри рамки
// места, фото — в её окне. У каждого места свой указатель (079-23): правка — только заменой
// pl.Border, копии мест — через Clone.
type Border struct {
	Kind   string     `json:"kind,omitempty"`   // [a-z0-9-]{1,32}, заморожен (как Art.Kind)
	Photo  string     `json:"photo,omitempty"`  // картинка: ID в Project.Photos
	Inks   []string   `json:"inks,omitempty"`   // встроенная: 0–2 цвета #rrggbb; пусто — цвета вида
	Width  float64    `json:"w,omitempty"`      // встроенная: толщина, мм, 0,2…20, шаг 0,1; 0 — по умолчанию вида
	Scale  float64    `json:"scale,omitempty"`  // картинка: размер 0,5…2, шаг 0,01; 0 — 1
	Cut    [4]float64 `json:"cut,omitzero"`     // картинка: 9-срез l, t, r, b — доли ширины/высоты картинки
	Open   bool       `json:"open,omitempty"`   // картинка без замкнутого окна: фото на всё место
	Preset string     `json:"preset,omitempty"` // плитка sa:<…>, поставившая рамочку (как Pattern.Preset)
}

// ErrBorder — рамочка не поставлена: вид не по грамматике, ключ библиотеки своей неверен или
// заданы и вид, и картинка.
var ErrBorder = errors.New("рамочка не поставлена: неверный вид или ключ")

// Пределы размера картинки-рамочки.
const (
	MinBorderScale = 0.5
	MaxBorderScale = 2.0
)

// Clone — копия рамочки (Inks не общий); nil — nil.
func (b *Border) Clone() *Border {
	if b == nil {
		return nil
	}
	c := *b
	c.Inks = slices.Clone(b.Inks)
	return &c
}

// ClampBorder — рамочка из файла или правки; новая копия или nil (снять): ровно одно из Kind и
// Photo; ключ не по грамматике — nil, правильный неизвестный хранится (файл новее); записи Photo
// нет в Photos — nil. Цвета и толщина — gen.ClampFrameInks/ClampFrameWidth, размер — 0 или
// 0,5…2 шаг 0,01 (ровно 1 — 0: одна запись на один вид), срез — gen.ClampCut (один зажим с
// разбором PNG, 079-25). Поля картинки у встроенной и наоборот снимаются, Preset — только у
// встроенной и по грамматике. Идемпотентна.
func ClampBorder(p *Project, b *Border) *Border {
	if b == nil || (b.Kind == "") == (b.Photo == "") {
		return nil
	}
	if b.Photo != "" {
		if _, ok := p.Photos[b.Photo]; !ok {
			return nil
		}
		return &Border{Photo: b.Photo, Scale: clampBorderScale(b.Scale), Cut: gen.ClampCut(b.Cut), Open: b.Open}
	}
	if !gen.ValidKind(b.Kind) {
		return nil
	}
	out := &Border{Kind: b.Kind, Inks: gen.ClampFrameInks(b.Kind, b.Inks), Width: gen.ClampFrameWidth(b.Width)}
	if ValidPreset(b.Preset) {
		out.Preset = b.Preset
	}
	return out
}

func clampBorderScale(v float64) float64 {
	if !(v > 0) || math.IsInf(v, 1) {
		return 0
	}
	if v = geom.Round2(min(max(v, MinBorderScale), MaxBorderScale)); v == 1 {
		return 0
	}
	return v
}

// ClampPhotoFade — прозрачность фото места (PhotoRef.Fade): NaN и Inf — 0, 0…1, шаг 0,01.
func ClampPhotoFade(v float64) float64 {
	return geom.Round2(geom.ClampF(Finite(v, 0), 0, 1))
}

// BorderSrc — что ставится рамочкой (ApplyBorder): встроенная (Kind, цвета, толщина, плитка) или
// картинка Photo (ID — LibPhotoID ключа) со срезом и Open из библиотеки. Пусто — снять.
type BorderSrc struct {
	Kind   string
	Inks   []string
	Width  float64
	Preset string
	Photo  PhotoInfo
	Cut    [4]float64
	Open   bool
}

// Border — рамочка из источника до зажима (ClampBorder — после Remember записи картинки); пустой
// источник — nil.
func (s BorderSrc) Border() *Border {
	switch {
	case s.Kind != "":
		return &Border{Kind: s.Kind, Inks: slices.Clone(s.Inks), Width: s.Width, Preset: s.Preset}
	case s.Photo.ID != "":
		return &Border{Photo: s.Photo.ID, Cut: s.Cut, Open: s.Open}
	}
	return nil
}

// BorderPatch — ручная правка рамочки (SetBorder): Inks nil — не менять, [] — цвета вида; 0 у
// чисел — не менять. Правка снимает Preset. Ключи JSON — как шлёт UI (frameapi.ts): поле без
// ключа приходит nil/0 — «не менять».
type BorderPatch struct {
	Inks  []string `json:"inks,omitempty"`
	Width float64  `json:"width,omitempty"`
	Scale float64  `json:"scale,omitempty"`
}

// BorderIns — отступы окна рамочки места pl, мм в осях места (design §2), уже зажатые
// geom.ClampInset. Встроенная — Ins вида × толщина (у овала все четыре — наибольший), накладка и
// неизвестный вид — 0. Картинка — срез × k, k = min(W/iw, H/ih) × размер; Open и картинка без
// размеров (079-24: k = +Inf дал бы NaN) — 0, окно = место.
func (p *Project) BorderIns(pl *Place) [4]float64 {
	var ins [4]float64
	b := pl.Border
	switch {
	case b == nil:
		return ins
	case b.Kind != "":
		m, ok := gen.FrameLookup(b.Kind)
		if !ok {
			return ins
		}
		w := b.Width
		if w == 0 {
			w = m.Width
		}
		for i, k := range m.Ins {
			ins[i] = k * w
		}
		if pl.Shape == ShapeOval {
			e := max(ins[0], ins[1], ins[2], ins[3])
			ins = [4]float64{e, e, e, e}
		}
	default:
		r, ok := p.Photos[b.Photo]
		if !ok || b.Open || r.W <= 0 || r.H <= 0 {
			return ins
		}
		iw, ih := float64(r.W), float64(r.H)
		s := b.Scale
		if s == 0 {
			s = 1
		}
		k := min(pl.W/iw, pl.H/ih) * s
		ins = [4]float64{k * b.Cut[0] * iw, k * b.Cut[1] * ih, k * b.Cut[2] * iw, k * b.Cut[3] * ih}
	}
	return geom.ClampInset(ins, pl.W, pl.H)
}

// BorderWindow — окно фото места pl (мм разворота, до поворота места) и его форма; без рамочки —
// само место и его форма.
func (p *Project) BorderWindow(pl *Place) (layout.Rect, geom.Outline) {
	return geom.InsetBox(layout.Rect{X: pl.X, Y: pl.Y, W: pl.W, H: pl.H}, pl.Outline(), p.BorderIns(pl))
}
