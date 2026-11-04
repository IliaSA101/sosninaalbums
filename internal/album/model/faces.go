package model

import (
	"github.com/IliaSA101/sosninaalbums/internal/geom"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// Подгон кадров по лицам (спека 130, design §5). Типы обмена между App (детектор, потоки,
// кэш) и телом правки design.FitFaces: лицо приходит готовыми долями, пакета faces здесь нет.

// FaceHit — снимок одного места на момент поиска и найденное лицо.
type FaceHit struct {
	Place, Photo string
	// Turn — четверти поворота фото в снимке (детектор видел картинку уже повёрнутой).
	Turn int
	// Frame — кадр фото в снимке: правка во время поиска меняет его (место «изменили»).
	Frame Frame
	// Box — рамка выбранного лица, Eye — середина глаз, в долях кадра после четвертей.
	// Пустая рамка (H ≤ 0) — лица нет.
	Box layout.Rect
	Eye geom.Pt
	// Many — лиц было несколько, Box — выбранное (самое крупное, при похожих — ближайшее к центру).
	Many bool
	// Missing — файла нет или он не читается: лица искать негде.
	Missing bool
}

// Коды ошибки образца «Остальные — как это лицо» (FaceReport.RefErr): правки нет.
const (
	RefNoPlace = "noplace" // нет такого места с фото
	RefNoFace  = "noface"  // на фото нет лица
	RefMany    = "many"    // на фото несколько лиц
	RefContain = "contain" // фото стоит «Целиком»: кадра нет
	RefNoFile  = "nofile"  // файла нет
	RefChanged = "changed" // фото или поворот изменили во время поиска
)

// RefErrText — текст тоста-ошибки по коду образца; "" для "" и неизвестного кода.
func RefErrText(code string) string {
	switch code {
	case RefNoPlace:
		return "Нет такого места с фото"
	case RefNoFace:
		return "На этом фото не нашлось лица"
	case RefMany:
		return "На этом фото несколько лиц"
	case RefContain:
		return "Фото стоит «Целиком» — кадра нет"
	case RefNoFile:
		return "Нет файла этого фото"
	case RefChanged:
		return "Фото изменили во время поиска"
	}
	return ""
}

// FaceReport — итог подгона. Одна причина на место: Fitted + len(Check) + len(Short) + Contain
// + Manual + Changed = мест с фото (без группового и образца). Check — места «проверьте»
// (без лица, несколько лиц, нет фото), Short — «не по норме» (мелкое, крупное, у края); оба —
// ID мест. У Check и Short кадр поставлен (кроме «без лица» и «нет фото»), в Fitted их нет.
type FaceReport struct {
	Fitted  int `json:"fitted"`
	NoFace  int `json:"noFace"`
	Many    int `json:"many"`
	Missing int `json:"missing"`
	Small   int `json:"small"`
	Big     int `json:"big"`
	Edge    int `json:"edge"`
	Contain int `json:"contain"`
	Manual  int `json:"manual"`
	Changed int `json:"changed"`

	Check []string `json:"check"`
	Short []string `json:"short"`
	// Asked — есть правленые кадры, а mode пуст: правки нет, вызывающий спрашивает (Manual = N).
	Asked bool `json:"asked"`
	// RefErr — код ошибки образца (Ref*); непусто — правки нет.
	RefErr string `json:"refErr,omitempty"`
	// Set — кадры, которые поставила правка, и кадр образца (фото → кадр): App помнит их как
	// fitted, они не «правленые». В биндинги не идёт.
	Set map[string]Frame `json:"-"`
}

// NewFaceReport — пустой итог со списками не nil (в JSON «[]», не null).
func NewFaceReport() FaceReport {
	return FaceReport{Check: []string{}, Short: []string{}}
}

// FaceGuide — силуэт под лицо для одного места (silhouette.md): ответ App.FaceGuide. Единицы — мм
// окна места, начало — его левый верхний угол. On = false — силуэта нет, остальные поля пусты.
// Path — замкнутый контур (последняя точка не равна первой), может выходить за окно по бокам.
type FaceGuide struct {
	On   bool         `json:"on"`
	W    float64      `json:"w"`
	H    float64      `json:"h"`
	EyeY float64      `json:"eyeY"`
	Path [][2]float64 `json:"path"`
}
