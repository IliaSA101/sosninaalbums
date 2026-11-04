package model

import (
	"strconv"
	"strings"
)

// Tone — ч/б и тон фото в месте (068): едет с фото, как Frame и зеркало. nil — без тона.
// Формула — CSS-фильтры в порядке строки CSS: grayscale(1) (Rec.709), brightness, contrast;
// пиксели экспорта считает render.applyTone, совпадение держит тест в export.
type Tone struct {
	Gray     bool `json:"gray,omitempty"`
	Bright   int  `json:"b,omitempty"` // −MaxTone…MaxTone, %: множитель (100+Bright)/100
	Contrast int  `json:"c,omitempty"` // −MaxTone…MaxTone, %: (v−0,5)·(100+Contrast)/100 + 0,5
}

// MaxTone — предел яркости и контраста по модулю (decisions 0.16 №3).
const MaxTone = 50

// ClampTone — зажим −MaxTone…MaxTone; все нули — nil: «без тона» в файле и снимке один,
// поэтому холостая правка видна сравнением JSON. Возвращает свою копию.
func ClampTone(t *Tone) *Tone {
	if t == nil {
		return nil
	}
	c := Tone{Gray: t.Gray, Bright: clampLevel(t.Bright), Contrast: clampLevel(t.Contrast)}
	if c == (Tone{}) {
		return nil
	}
	return &c
}

func clampLevel(v int) int { return min(max(v, -MaxTone), MaxTone) }

// CSS — строка filter для <img>: "grayscale(1) brightness(120%) contrast(80%)", нулевые
// части опущены; nil и нули — "". Та же строка — у живого предпросмотра (toneCss в UI).
func (t *Tone) CSS() string {
	if t == nil {
		return ""
	}
	var parts []string
	if t.Gray {
		parts = append(parts, "grayscale(1)")
	}
	if t.Bright != 0 {
		parts = append(parts, "brightness("+strconv.Itoa(100+t.Bright)+"%)")
	}
	if t.Contrast != 0 {
		parts = append(parts, "contrast("+strconv.Itoa(100+t.Contrast)+"%)")
	}
	return strings.Join(parts, " ")
}
