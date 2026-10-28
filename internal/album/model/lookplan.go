package model

// Постановка макета альбома (131 design §4): параметры, режимы и итог сухого прогона. Типы в
// model: они уходят в биндинги Wails (App.ApplyLook, App.PlanLook).

// Режимы постановки макета: серия (новый или пробный альбом), добавить развороты и фоны по
// серии (собранный альбом, G4b).
const (
	LookSeries = "series"
	LookAdd    = "add"
	LookBg     = "bg"
)

// LookOpts — что поставить. Режим при несобранном альбоме определяет ядро (series), поле Mode
// нужно только у собранного: add или bg. Spreads — разворотов серии (series, add); 0 — по
// умолчанию, все шаги макета по разу.
type LookOpts struct {
	Mode    string `json:"mode"`
	Spreads int    `json:"spreads"`
}

// LookPlan — итог постановки или её сухого прогона PlanLook (одна и та же функция на копии):
// вопрос до правки и тост после.
type LookPlan struct {
	Mode     string `json:"mode"`     // series | add | bg — что выйдет; у собранного без выбора — ""
	Trial    bool   `json:"trial"`    // пробный: в альбоме уже макетные развороты, они заменятся
	Touched  int    `json:"touched"`  // тронутых разворотов и обложки (свои места, надписи, стикеры), их правки заменятся
	Full     bool   `json:"full"`     // собранный: есть фото из съёмки, series недоступен
	Spreads  int    `json:"spreads"`  // разворотов макета после постановки
	Places   int    `json:"places"`   // мест в них
	Drafts   int    `json:"drafts"`   // заготовок в них
	Replaced int    `json:"replaced"` // bg: разворотов со своим фоном, который заменится
	// CoverTouched и CoverReplaced — доля обложки в Touched и Replaced (они остаются суммой):
	// текст вопроса «… и обложке» берётся по ним (0.28-131-174).
	CoverTouched  bool   `json:"coverTouched"`
	CoverReplaced bool   `json:"coverReplaced"`
	Anchors       int    `json:"anchors"` // bg: станут якорями 107 (были голыми)
	Cover         bool   `json:"cover"`   // оформление обложки поставлено
	Left          int    `json:"left"`    // фото уйдут в ленту
	Prev          string `json:"prev"`    // имя прежнего макета альбома
	Missing       int    `json:"missing"` // предметов макета без файла на этом ПК
}
