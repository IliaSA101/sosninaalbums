package model

import (
	"slices"

	"github.com/IliaSA101/sosninaalbums/internal/library"
)

// Свои отметки фото (спека 004, design §2): живут в Editor рядом с Project, а не в нём —
// иначе 5000 отметок платили бы marshal/unmarshal на каждой правке альбома.

// XmpMarks — отметки из файла (XMP). Псевдоним, а не новый тип: в wailsjs один TS-тип.
type XmpMarks = library.Marks

// Mark — своя отметка фото; nil-поле — «не трогали, действует отметка из файла».
// Значения неизменяемы: правка кладёт новые указатели, иначе снимки отмены поменялись бы следом.
type Mark struct {
	Path   string  `json:"path"` // абсолютный; по нему ID при Load и перепривязке
	Rating *int    `json:"rating,omitempty"`
	Label  *string `json:"label,omitempty"` // "" — метка снята в программе
	Flag   *int    `json:"flag,omitempty"`
	// Bins — ID подборок фото (083) по возрастанию, без повторов; срез не правится на месте.
	Bins []string `json:"bins,omitempty"`
	// alt — запасной путь из файла, если не нашлось ни там, ни там (как PhotoRef.Alt).
	alt string
}

// Alt — запасной путь из файла (полем не экспортировать: JSON).
func (m Mark) Alt() string { return m.alt }

// WithAlt — копия с запасным путём (Load; перепривязка сбрасывает).
func (m Mark) WithAlt(s string) Mark { m.alt = s; return m }

// WithBins — копия с подборками ids (сорт, без повторов, пусто — nil).
func (m Mark) WithBins(ids []string) Mark { m.Bins = normBins(ids); return m }

// MarkPatch — что поменять у выбранных фото. Rating и Step вместе — Step не действует.
type MarkPatch struct {
	Rating *int    `json:"rating,omitempty"` // 0–5
	Step   int     `json:"step,omitempty"`   // ±1 от действующей оценки каждого фото, в пределах 0–5
	Label  *string `json:"label,omitempty"`  // "" | red | yellow | green | blue | purple
	Toggle bool    `json:"toggle,omitempty"` // с Label: у всех уже этот цвет — снять ("")
	Flag   *int    `json:"flag,omitempty"`   // −1 | 0 | 1
}

// MarkView — своя отметка для UI (App.Marks); nil — поле своё не задано.
type MarkView struct {
	Rating *int     `json:"rating,omitempty"`
	Label  *string  `json:"label,omitempty"`
	Flag   *int     `json:"flag,omitempty"`
	Bins   []string `json:"bins,omitempty"` // ID подборок (083)
}

// Clone — копия с собственными указателями: Editor не делит отметки с чужим проектом.
func (m Mark) Clone() Mark {
	m.Rating, m.Label, m.Flag = clonePtr(m.Rating), clonePtr(m.Label), clonePtr(m.Flag)
	m.Bins = slices.Clone(m.Bins)
	return m
}

// None — записи нет смысла хранить: ни своих отметок, ни подборок.
func (m Mark) None() bool {
	return m.Rating == nil && m.Label == nil && m.Flag == nil && len(m.Bins) == 0
}

func (m Mark) View() MarkView {
	return MarkView{Rating: clonePtr(m.Rating), Label: clonePtr(m.Label), Flag: clonePtr(m.Flag), Bins: slices.Clone(m.Bins)}
}

func SameMark(a, b Mark) bool {
	return a.Path == b.Path && a.alt == b.alt &&
		samePtr(a.Rating, b.Rating) && samePtr(a.Label, b.Label) && samePtr(a.Flag, b.Flag) &&
		slices.Equal(a.Bins, b.Bins)
}

func samePtr[T comparable](a, b *T) bool {
	return a == nil && b == nil || a != nil && b != nil && *a == *b
}

func clonePtr[T any](p *T) *T {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}

// Effective — действующая отметка: своя по полю, иначе из файла. Зеркало во фронте — markOf,
// паритет — testdata/parity/marks.json.
func Effective(own Mark, x XmpMarks) XmpMarks {
	if own.Rating != nil {
		x.Rating = *own.Rating
	}
	if own.Label != nil {
		x.Label, x.LabelText = *own.Label, ""
	}
	if own.Flag != nil {
		x.Flag = *own.Flag
	}
	return x
}

func OkRating(v int) bool { return v >= 0 && v <= 5 }

func OkFlag(v int) bool { return v >= -1 && v <= 1 }

// OkLabel — цвета, которые ставит программа; "other" — только из файла.
func OkLabel(v string) bool {
	switch v {
	case "", "red", "yellow", "green", "blue", "purple":
		return true
	}
	return false
}

// KeepIf — поле, если значение допустимо, иначе nil.
func KeepIf[T any](p *T, ok func(T) bool) *T {
	if p == nil || !ok(*p) {
		return nil
	}
	return p
}

// CleanPatch отбрасывает недопустимые поля патча; остальные применяются.
func CleanPatch(pt MarkPatch) MarkPatch {
	rated := pt.Rating != nil // Step не действует и при недопустимом Rating, который keepIf отбросит
	pt.Rating = KeepIf(clonePtr(pt.Rating), OkRating)
	pt.Label = KeepIf(clonePtr(pt.Label), OkLabel)
	pt.Flag = KeepIf(clonePtr(pt.Flag), OkFlag)
	if rated || pt.Step != 1 && pt.Step != -1 {
		pt.Step = 0
	}
	return pt
}

// SetField пишет цель в своё поле. Своего нет, а цель равна значению из файла, — пропуск:
// иначе холостое нажатие заводило бы свою отметку, которую «Обновить» уже не сдвинет.
func SetField[T comparable](own **T, target, file T) {
	if *own == nil && target == file || *own != nil && **own == target {
		return
	}
	*own = &target
}
