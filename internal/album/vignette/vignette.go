package vignette

import "math"

// Rect — прямоугольник в мм разворота (лист — тот же разворот, 031 §3).
type Rect struct {
	X, Y, W, H float64
}

// Роли мест виньетки (Place.Role в модели; там "" — тоже ученик).
const (
	RolePupil   = "pupil"
	RoleTeacher = "teacher"
	RoleGroup   = "group"
)

// Input — всё, что генератору нужно знать о листе и классе.
type Input struct {
	// Regions — лист: одна область, разворот: левая и правая страницы. Каждая уже без
	// max(поля, безопасная зона), у корешка тоже: места не выходят за них и на сгиб не попадают.
	Regions []Rect
	Gap     float64 // промежуток, мм: по горизонтали max(Gap, 2), по вертикали max(Gap, Pad)
	// Pad — отступ подписи от фото, мм: подпись кончается на Pad выше портрета ряда ниже
	// (030 §5), иначе при Pad > Gap «!» стоял бы у всех подписей.
	Pad      float64
	Teachers int
	Pupils   int
	Ratio    float64 // портрет w/h; ≤ 0 — 3:4
	HS       float64 // полоса подписи под портретом ученика (и групповым), мм
	HST      float64 // полоса подписи учителя, мм
}

// Slot — место виньетки: рамка портрета (полоса подписи — под ней) и роль.
type Slot struct {
	Box  Rect
	Role string
}

// Plan — итог генератора. Порядок слотов: ученики → учителя → групповое (031 design §2):
// «Заполнить пустые» и вставка имён идут по классу.
type Plan struct {
	Slots    []Slot
	Head     *Rect   // полоса шапки; nil — у шаблона шапки нет
	PupilW   float64 // ширина портрета ученика, мм (0 — учеников нет)
	TeacherW float64 // ширина портрета учителя, мм (0 — учителей нет); у PosLead — руководителя
	// LeadW — ширина руководителя (PosLead, 049), остальные учителя — в общей сетке; 0 — не lead.
	LeadW float64
	// HeadCell — шапка стоит в свободных ячейках последнего ряда (PosCell): кегль шапки
	// считается по её ширине, а не по высоте полосы (049 §3.2).
	HeadCell bool
}

// DefaultRatio — портрет по умолчанию: типовой школьный кадр (031, развилка 8).
const DefaultRatio = "3:4"

// ParseRatio — пропорция w/h из подписи поля «Портрет»; чужое — 3:4.
func ParseRatio(s string) float64 {
	switch s {
	case "2:3":
		return 2.0 / 3
	case "4:5":
		return 0.8
	case "1:1":
		return 1
	}
	return 0.75
}

// clean зажимает вход: числа и размеры из UI и файла могут быть любыми.
func clean(in Input) Input {
	fin := func(v float64) float64 {
		if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 {
			return 0
		}
		return v
	}
	if in.Ratio <= 0 || math.IsNaN(in.Ratio) || math.IsInf(in.Ratio, 0) {
		in.Ratio = 0.75
	}
	in.Ratio = math.Min(math.Max(in.Ratio, 0.2), 5)
	in.Gap, in.Pad, in.HS, in.HST = fin(in.Gap), fin(in.Pad), fin(in.HS), fin(in.HST)
	in.Teachers, in.Pupils = max(in.Teachers, 0), max(in.Pupils, 0)
	rs := make([]Rect, len(in.Regions))
	for i, r := range in.Regions {
		rs[i] = Rect{r.X, r.Y, fin(r.W), fin(r.H)}
		if math.IsNaN(r.X) || math.IsInf(r.X, 0) {
			rs[i].X = 0
		}
		if math.IsNaN(r.Y) || math.IsInf(r.Y, 0) {
			rs[i].Y = 0
		}
	}
	in.Regions = rs
	return in
}
