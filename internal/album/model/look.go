package model

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"slices"
	"strings"
)

// Макет альбома (спека 131, design §2): обложка и серия шагов-разворотов из библиотеки (вид album).
// Файл хранится как пришёл, ключ — sha256 его байтов (0.27-131-6), зажим — при каждом чтении:
// поле новее программы не меняет ключ и не теряется в файле.

const (
	// LookV — версия файла макета; больше — макет новее программы.
	LookV = 1
	// MaxLookFile — байтов в файле макета (предел чтения 0.27 — 1 МБ, контракт 0.27-131-30).
	MaxLookFile = 512 << 10
	// MaxLookSteps — шагов в макете: окно «Сохранить альбом как макет» сливает одинаковые до 24.
	MaxLookSteps = 24
)

// Роли шага (0.27-131-3): один раз, по кругу (блок), виньетка 031.
const (
	RoleOnce     = ""
	RoleRepeat   = "repeat"
	RoleVignette = "vignette"
)

var (
	// ErrLookNewer — макет из новой версии программы.
	ErrLookNewer = errors.New("макет альбома из новой версии программы")
	// ErrLookSteps — в файле больше MaxLookSteps шагов: обрезка поменяла бы серию (конец пропал бы).
	ErrLookSteps = errors.New("в макете альбома больше 24 разворотов")
	errLookBig   = errors.New("файл макета альбома слишком большой")
)

// AlbumTpl — макет: обложка и серия. Имя и тема — в записи библиотеки, не в файле.
type AlbumTpl struct {
	V     int        `json:"v"`
	Cover *CoverTpl  `json:"cover,omitempty"`
	Steps []LookStep `json:"steps"` // 1…MaxLookSteps
}

// CoverTpl — оформление обложки: фон всего листа и стороны (доли зоны сторонки 127, Pages = 1).
type CoverTpl struct {
	Bg    *TplBg     `json:"bg,omitempty"`
	Back  *SpreadTpl `json:"back,omitempty"`
	Front *SpreadTpl `json:"front,omitempty"`
}

// LookStep — шаг серии: роль, название главы (Spread.Name 094) и шаблон разворота (Pages 2) или
// страницы (Pages 1 — зеркалится на обе); у виньетки — параметры листа 031.
type LookStep struct {
	Role string    `json:"role,omitempty"`
	Name string    `json:"name,omitempty"`
	Tpl  SpreadTpl `json:"tpl"`
	Vig  *LookVig  `json:"vig,omitempty"`
}

// LookVig — подмножество Vignette: шаблон каталога 031 (своего Own нет — его нет на чужом ПК). В
// design §2 — VigTpl, но это имя занято своим шаблоном виньетки (038).
type LookVig struct {
	Tpl      string `json:"tpl"`
	Teachers int    `json:"teachers"`
	Pupils   int    `json:"pupils"`
	Ratio    string `json:"ratio,omitempty"`
}

// LookKey — ключ макета: my: + 8 байт sha256 байтов файла (как у картинок, 0.27-131-6). Ключ от
// канонического JSON менялся бы с версией программы: поле 0.29 переписало бы ключ в 0.28.
func LookKey(data []byte) string {
	sum := sha256.Sum256(data)
	return libKeyPrefix + hex.EncodeToString(sum[:8])
}

// lookFile — второй проход разбора: шаги и обложка сырыми, чтобы число шагов сверить до разбора
// каждого (память на мусорном входе — по числу шагов), а версии шаблонов — до их разбора.
type lookFile struct {
	V     int               `json:"v"`
	Cover json.RawMessage   `json:"cover,omitempty"`
	Steps []json.RawMessage `json:"steps"`
}

// verHead — только версия: у файла новее программы остальные поля могут быть другого вида, и
// ответ должен быть «новее», а не «испорчен».
type verHead struct {
	V int `json:"v"`
}

// ParseLook — макет из файла (библиотека, набор): не больше MaxLookFile, неизвестные поля
// пропускаются; V файла, шаблона шага или стороны обложки новее — ErrLookNewer (шаг не выпадает
// молча: серия без него была бы другой); шагов больше MaxLookSteps — ErrLookSteps, затем ClampLook;
// нечего ставить — ErrTplEmpty.
func ParseLook(data []byte) (AlbumTpl, error) {
	if len(data) > MaxLookFile {
		return AlbumTpl{}, errLookBig
	}
	var head verHead
	if err := json.Unmarshal(data, &head); err != nil {
		return AlbumTpl{}, err
	}
	if head.V > LookV {
		return AlbumTpl{}, ErrLookNewer
	}
	var f lookFile
	if err := json.Unmarshal(data, &f); err != nil {
		return AlbumTpl{}, err
	}
	if len(f.Steps) > MaxLookSteps {
		return AlbumTpl{}, ErrLookSteps
	}
	t := AlbumTpl{V: f.V, Steps: make([]LookStep, len(f.Steps))}
	if len(f.Cover) > 0 {
		var side struct {
			Back  *verHead `json:"back"`
			Front *verHead `json:"front"`
		}
		_ = json.Unmarshal(f.Cover, &side) // не разобралось — ошибку даст полный разбор ниже
		if side.Back != nil && side.Back.V > TplV || side.Front != nil && side.Front.V > TplV {
			return AlbumTpl{}, ErrLookNewer
		}
		if err := json.Unmarshal(f.Cover, &t.Cover); err != nil {
			return AlbumTpl{}, err
		}
	}
	for i, raw := range f.Steps {
		var st struct {
			Tpl verHead `json:"tpl"`
		}
		_ = json.Unmarshal(raw, &st) // так же: битый шаг отвергнет полный разбор
		if st.Tpl.V > TplV {
			return AlbumTpl{}, ErrLookNewer
		}
		if err := json.Unmarshal(raw, &t.Steps[i]); err != nil {
			return AlbumTpl{}, err
		}
	}
	c, ok := ClampLook(t)
	if !ok {
		return AlbumTpl{}, ErrTplEmpty
	}
	return c, nil
}

// ClampLook — макет к допустимому; копия. Отказ (false): V не LookV или нет ни одного шага-не-
// виньетки. Шаги — ClampSpreadTpl (пустой выбрасывается; у виньетки оформление может быть пустым),
// не больше MaxLookSteps; роль вне списка и виньетка без Vig — «один раз»; повтор — только первый
// сплошной блок; имя — CleanName; Vig — ClampVignette, свой шаблон — умолчание, мест ≤ MaxPlaces.
// Обложка: фон — зажим фона шаблона, стороны — шаблоны страницы; пусто — nil.
func ClampLook(t AlbumTpl) (AlbumTpl, bool) {
	if t.V != LookV {
		return AlbumTpl{}, false
	}
	c := AlbumTpl{V: LookV, Cover: clampCoverTpl(t.Cover), Steps: []LookStep{}}
	for _, st := range t.Steps {
		if len(c.Steps) == MaxLookSteps {
			break
		}
		if cs, ok := clampLookStep(st); ok {
			c.Steps = append(c.Steps, cs)
		}
	}
	oneRepeatBlock(c.Steps)
	if !slices.ContainsFunc(c.Steps, func(s LookStep) bool { return s.Role != RoleVignette }) {
		return AlbumTpl{}, false
	}
	return c, true
}

// clampLookStep — шаг: роль, имя, шаблон, виньетка. false — ставить нечего.
func clampLookStep(st LookStep) (LookStep, bool) {
	c := LookStep{Name: CleanName(st.Name)}
	switch st.Role {
	case RoleRepeat:
		c.Role = RoleRepeat
	case RoleVignette:
		if st.Vig != nil {
			c.Role, c.Vig = RoleVignette, clampLookVig(*st.Vig)
		}
	}
	tpl, ok := ClampSpreadTpl(st.Tpl)
	if !ok && (c.Role != RoleVignette || tpl.V != TplV) {
		return LookStep{}, false // у виньетки места строит лист 031: оформление может быть пустым
	}
	c.Tpl = tpl
	return c, true
}

// clampLookVig — параметры виньетки шага: ClampVignette; свой шаблон (VigOwnPrefix) — умолчание
// каталога; учеников меньше, если мест больше MaxPlaces (как FixVignettes при Load).
func clampLookVig(v LookVig) *LookVig {
	cv := ClampVignette(Vignette{Tpl: v.Tpl, Teachers: v.Teachers, Pupils: v.Pupils, Ratio: v.Ratio})
	if strings.HasPrefix(cv.Tpl, VigOwnPrefix) || cv.Tpl == "" {
		cv.Tpl = DefaultVignette().Tpl
	}
	if over := VigPlaces(cv) - MaxPlaces; over > 0 {
		cv.Pupils = max(0, cv.Pupils-over)
	}
	return &LookVig{Tpl: cv.Tpl, Teachers: cv.Teachers, Pupils: cv.Pupils, Ratio: cv.Ratio}
}

// oneRepeatBlock — повтор только у первого сплошного блока: «по кругу» — один блок (design §2).
func oneRepeatBlock(steps []LookStep) {
	_, rep, _ := lookBlocks(steps)
	if len(rep) == 0 {
		return
	}
	for i := rep[len(rep)-1] + 1; i < len(steps); i++ {
		if steps[i].Role == RoleRepeat {
			steps[i].Role = RoleOnce
		}
	}
}

// OwnBgs — свой фон страницы задней и лицевой сторон обложки (Page[0] шаблона стороны; нет стороны — пусто).
func (c *CoverTpl) OwnBgs() (out [2]TplBg) {
	for i, side := range []*SpreadTpl{c.Back, c.Front} {
		if side != nil && len(side.Page) > 0 {
			out[i] = side.Page[0]
		}
	}
	return out
}

// HasBg — макет обложки задаёт фон: фон листа или цвет, узор, фото стороны.
func (c *CoverTpl) HasBg() bool {
	own := c.OwnBgs()
	return c.Bg != nil || own[0] != (TplBg{}) || own[1] != (TplBg{})
}

// CoverPageBg — фон страницы стороны обложки, каким его ставит программа (131 design §5): цвет —
// свой стороны, иначе цвет фона листа bg; узор и фото — фона листа, а если у него их нет — свои
// стороны. Так узор и фото ложатся на лист один раз, а сторона добавляет только цвет (решение 135).
// Превью и library-verify берут фон этой же функцией.
func CoverPageBg(bg *TplBg, own TplBg) TplBg {
	var b TplBg
	if bg != nil {
		b = *bg
	}
	out := TplBg{Color: own.Color}
	if out.Color == "" {
		out.Color = b.Color
	}
	if b.Pat != nil || b.Lib != "" {
		out.Pat, out.Lib, out.Fade = b.Pat, b.Lib, b.Fade
	} else {
		out.Pat, out.Lib, out.Fade = own.Pat, own.Lib, own.Fade
	}
	return out
}

// SideTpl — сторона k обложки (0 задняя, 1 лицевая) как страница: фон — CoverPageBg, места — LookBoxes
// на области с отношением r (W/H; r ≤ 0 — места как есть). Стороны нет или k вне 0…1 — пустая страница
// с одним фоном. Плитка макета и library-verify строят сторону только этим: превью не расходятся.
func (c *CoverTpl) SideTpl(k int, r float64) SpreadTpl {
	t := SpreadTpl{V: TplV, Pages: 1, Ratio: 1}
	if c == nil {
		return t
	}
	var own TplBg
	switch {
	case k == 0 && c.Back != nil:
		t = *c.Back // копия значения: меняются только Page и Places, чужие срезы не трогаются
	case k == 1 && c.Front != nil:
		t = *c.Front
	}
	if k == 0 || k == 1 {
		own = c.OwnBgs()[k]
	}
	t.Page = []TplBg{CoverPageBg(c.Bg, own)}
	t.Places = LookBoxes(t, r)
	return t
}

// SheetBgTpl — фон обложки как шаблон на весь лист: страницы сторон по CoverPageBg; узор и фото
// фона листа Bg — на весь лист (Whole), тогда у страниц их нет.
func (c *CoverTpl) SheetBgTpl() SpreadTpl {
	own := c.OwnBgs()
	t := SpreadTpl{V: TplV, Pages: 2, Ratio: 1, Page: []TplBg{CoverPageBg(c.Bg, own[0]), CoverPageBg(c.Bg, own[1])}}
	if c.Bg != nil && (c.Bg.Pat != nil || c.Bg.Lib != "") {
		w := TplBg{Pat: c.Bg.Pat, Lib: c.Bg.Lib, Fade: c.Bg.Fade}
		t.Whole = &w
		for i := range t.Page {
			t.Page[i].Pat, t.Page[i].Lib, t.Page[i].Fade = nil, "", 0
		}
	}
	return t
}

// clampCoverTpl — обложка макета: фон листа и стороны-страницы; пустое — nil.
func clampCoverTpl(c *CoverTpl) *CoverTpl {
	if c == nil {
		return nil
	}
	out := &CoverTpl{}
	if c.Bg != nil {
		if bg := clampTplBg(*c.Bg); bg != (TplBg{}) {
			out.Bg = &bg
		}
	}
	out.Back, out.Front = clampCoverSide(c.Back), clampCoverSide(c.Front)
	if out.Bg == nil && out.Back == nil && out.Front == nil {
		return nil
	}
	return out
}

// clampCoverSide — сторона обложки: шаблон страницы (Pages = 1); иначе или пусто — nil.
func clampCoverSide(t *SpreadTpl) *SpreadTpl {
	if t == nil || t.Pages != 1 {
		return nil
	}
	c, ok := ClampSpreadTpl(*t)
	if !ok {
		return nil
	}
	return &c
}

// lookBlocks — индексы шагов до первого повтора (pre), первого сплошного блока повтора (rep) и после
// него (post). Повтор после блока (незажатый вход) идёт в post как «один раз».
func lookBlocks(steps []LookStep) (pre, rep, post []int) {
	for i, s := range steps {
		switch {
		case len(post) == 0 && s.Role == RoleRepeat && (len(rep) == 0 || rep[len(rep)-1] == i-1):
			rep = append(rep, i)
		case len(rep) == 0:
			pre = append(pre, i)
		default:
			post = append(post, i)
		}
	}
	return pre, rep, post
}

// Series — индексы шагов новой серии из n разворотов (пустой альбом, «Добавить»; обложки не
// касается): pre, затем rep по кругу, затем post. n зажимается в [|pre| + |post| + 1, MaxSpreads];
// без повтора — ровно все шаги по порядку. Виньетки — на своих местах в pre и post.
func Series(t AlbumTpl, n int) []int {
	pre, rep, post := lookBlocks(t.Steps)
	if len(rep) == 0 {
		return append(pre, post...)
	}
	mid := max(1, min(n, MaxSpreads)-len(pre)-len(post))
	out := make([]int, 0, len(pre)+mid+len(post))
	out = append(out, pre...)
	for i := range mid {
		out = append(out, rep[i%len(rep)])
	}
	return append(out, post...)
}

// SeriesFit — шаги для m разворотов-не-виньеток собранного альбома («Фоны по серии», design §2,
// К1-7), ровно m (0…MaxSpreads). Блоки pre′, post′ — без виньеток. Хватает места — pre′, середина
// по кругу из rep (нет rep — из всех шагов-не-виньеток), post′. Не хватает — первый шаг начала и
// последний шаг конца на краях, между ними — остальные по порядку, обрезанные до m.
func SeriesFit(t AlbumTpl, m int) []int {
	m = max(0, min(m, MaxSpreads))
	if m == 0 {
		return []int{}
	}
	notVig := func(ids []int) []int {
		return slices.DeleteFunc(slices.Clone(ids), func(i int) bool { return t.Steps[i].Role == RoleVignette })
	}
	pre, rep, post := lookBlocks(t.Steps)
	pre, post = notVig(pre), notVig(post)
	if m < len(pre)+len(post) {
		all := append(slices.Clone(pre), post...)
		if m == 1 {
			return all[:1]
		}
		out := append([]int{all[0]}, all[1 : len(all)-1][:m-2]...)
		return append(out, all[len(all)-1])
	}
	if len(rep) == 0 {
		rep = append(slices.Clone(pre), post...)
	}
	if len(rep) == 0 {
		return []int{} // одни виньетки (ClampLook такого не пропускает)
	}
	out := slices.Clone(pre)
	for i := range m - len(pre) - len(post) {
		out = append(out, rep[i%len(rep)])
	}
	return append(out, post...)
}

// VigStep — индекс первого шага-виньетки (его берут развороты-виньетки «Фонов по серии»); нет — -1.
func VigStep(t AlbumTpl) int {
	return slices.IndexFunc(t.Steps, func(s LookStep) bool { return s.Role == RoleVignette })
}

// LookBoxes — места шага на области с отношением сторон r (W/H; у страницы — страница): композиция
// шага (нарисована при t.Ratio) целиком вписывается одним масштабом в наибольший прямоугольник с
// отношением t.Ratio по центру области (0.27-131-20). Общие края и промежутки остаются общими и
// равными, место левой страницы не уходит через сгиб. r не число или ≤ 0 — места как есть (копия).
func LookBoxes(t SpreadTpl, r float64) []Box {
	out := slices.Clone(t.Places)
	big := t.Ratio
	if !finite(r) || r <= 0 || !finite(big) || big <= 0 || r == big {
		return out
	}
	for i := range out {
		b := &out[i]
		if r < big { // область уже: композиция во всю ширину, по высоте — часть
			k := r / big
			b.Y, b.H = (1-k)/2+b.Y*k, b.H*k
		} else {
			k := big / r
			b.X, b.W = (1-k)/2+b.X*k, b.W*k
		}
	}
	return out
}

// LookPlaces — мест у шага idx: у виньетки — места листа (учителя, ученики, групповое), у шаблона
// страницы — × 2 (зеркалится на обе страницы); нет шага — 0.
func LookPlaces(t AlbumTpl, idx int) int {
	if idx < 0 || idx >= len(t.Steps) {
		return 0
	}
	s := t.Steps[idx]
	if s.Role == RoleVignette && s.Vig != nil {
		return VigPlaces(Vignette{Tpl: s.Vig.Tpl, Teachers: s.Vig.Teachers, Pupils: s.Vig.Pupils})
	}
	if s.Tpl.Pages == 1 {
		return 2 * len(s.Tpl.Places)
	}
	return len(s.Tpl.Places)
}

// seriesPlaces — мест у серии (индексы шагов).
func seriesPlaces(t AlbumTpl, ids []int) int {
	n := 0
	for _, i := range ids {
		n += LookPlaces(t, i)
	}
	return n
}

// LookFit — наименьшее n серии, у которой мест не меньше k («Подобрать под 64 выбранных фото»),
// не больше MaxSpreads. Без повтора или с повтором без мест — n по умолчанию (все шаги).
func LookFit(t AlbumTpl, k int) int {
	pre, rep, post := lookBlocks(t.Steps)
	lo := len(pre) + len(post) + min(1, len(rep))
	if len(rep) == 0 || seriesPlaces(t, rep) == 0 {
		return max(lo, len(t.Steps))
	}
	have := seriesPlaces(t, pre) + seriesPlaces(t, post)
	n := len(pre) + len(post)
	for i := 0; n < MaxSpreads; i++ {
		have += LookPlaces(t, rep[i%len(rep)])
		n++
		if have >= k {
			break
		}
	}
	return n
}

// LookCard — сведения плитки макета (DecorItem): разворотов и мест при n по умолчанию (каждый шаг
// один раз) с обложкой, есть ли шаг-виньетка и оформление обложки.
type LookCard struct {
	Steps int
	From  int
	Vig   bool
	Cover bool
}

// CardOf — плитка макета: n по умолчанию — все шаги по разу (Series(t, len(Steps))).
func CardOf(t AlbumTpl) LookCard {
	ids := Series(t, len(t.Steps))
	c := LookCard{Steps: len(ids), From: seriesPlaces(t, ids), Vig: VigStep(t) >= 0, Cover: t.Cover != nil}
	if t.Cover != nil {
		for _, side := range []*SpreadTpl{t.Cover.Back, t.Cover.Front} {
			if side != nil {
				c.From += len(side.Places)
			}
		}
	}
	return c
}

// LookBlockSizes — сколько шагов в начале (pre), в блоке повтора (rep) и в конце (post): ядро
// сопоставляет макетные развороты альбома блокам серии (начало — с головы, конец — с хвоста).
func LookBlockSizes(t AlbumTpl) (pre, rep, post int) {
	a, b, c := lookBlocks(t.Steps)
	return len(a), len(b), len(c)
}

// LookBlocks — индексы шагов начала (pre), блока повтора (rep) и конца (post): ядро находит по ним
// хвост серии в альбоме и круг повторяемых шагов.
func LookBlocks(t AlbumTpl) (pre, rep, post []int) { return lookBlocks(t.Steps) }

// AddSteps — шаги n добавляемых разворотов («Добавить», 131 design §4): повторяемые по кругу,
// начиная со следующего за шагом after (индекс шага разворота, перед которым встанет первый новый;
// -1 или шаг не из повтора — с первого повторяемого). Повтора нет — последний шаг, который не
// виньетка. n зажимается в 1…MaxSpreads.
func AddSteps(t AlbumTpl, after, n int) []int {
	n = max(1, min(n, MaxSpreads))
	_, rep, _ := lookBlocks(t.Steps)
	out := make([]int, n)
	if len(rep) == 0 {
		for i := len(t.Steps) - 1; i >= 0; i-- {
			if t.Steps[i].Role != RoleVignette {
				for j := range out {
					out[j] = i
				}
				break
			}
		}
		return out
	}
	start := 0
	if at := slices.Index(rep, after); at >= 0 {
		start = at + 1
	}
	for i := range out {
		out[i] = rep[(start+i)%len(rep)]
	}
	return out
}
