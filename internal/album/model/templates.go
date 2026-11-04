package model

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/IliaSA101/sosninaalbums/internal/geom"
)

const (
	// TemplatesVersion — версия templates.json; больше — только чтение. Пишется
	// наименьшая, понимающая все поля файла (MarshalTemplates): 2 — при угле,
	// 3 — при шаблоне больше чем на v2Places мест (027: 0.9.x режет коробки до 16),
	// 4 — при шаблоне виньетки (038: прежние программы такой файл только читают),
	// 5 — при форме места или виньетки (044: 0.11 читает без форм и не пишет).
	TemplatesVersion = 5
	// v2Places — больше мест в шаблоне программа до 0.10 не держит.
	v2Places = 16
	// MaxTemplates — шаблонов в файле.
	MaxTemplates = 200
	// MaxTemplateName — знаков в имени.
	MaxTemplateName = 60
	// MaxTemplatesFile — больше 1 МБ файл считаем битым.
	MaxTemplatesFile = 1 << 20
	// MinBoxFrac — меньшая сторона места в долях области.
	MinBoxFrac = 0.01
	// tplPrec — доли и угол в файле кратны 1/tplPrec: 1e-5 доли — 0,006 мм на
	// развороте 600 мм, а 200 шаблонов по 64 места укладываются в MaxTemplatesFile (147).
	tplPrec = 1e5
	// TplVignette — Template.Kind своего шаблона виньетки (038).
	TplVignette = "vignette"
	// MinVigAspect, MaxVigAspect — пределы W/H области шаблона виньетки.
	MinVigAspect = 0.1
	MaxVigAspect = 10
)

// Ошибки шаблонов (текст уходит в тост).
var (
	ErrTemplatesNewer = errors.New("шаблоны сохранены более новой версией программы")
	ErrNoPlaces       = errors.New("нет мест")
	ErrTooManyTpl     = fmt.Errorf("не больше %d шаблонов", MaxTemplates)
	// ErrTemplatesFull — запись дала бы файл больше MaxTemplatesFile: любая программа
	// сочла бы его битым и увела в .bad вместе со всеми шаблонами.
	ErrTemplatesFull error = tplError("Шаблонов слишком много: файл больше 1 МБ — удалите ненужные")
	// ErrTplNotFound — шаблона с таким ID нет (удалён в другом окне).
	ErrTplNotFound error = tplError("Шаблон не найден")
)

// tplError — ошибка шаблонов с готовым текстом тоста (с заглавной буквы, как NoPlacesError).
type tplError string

func (e tplError) Error() string { return string(e) }

// Template — сохранённая сетка: места в долях области Scope, порядок = порядок мест.
type Template struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Scope string `json:"scope"`          // TplPage | TplSpread; у виньетки — всегда TplSpread
	Boxes []Box  `json:"boxes"`          // у виньетки — места листа: старая программа видит сетку
	Kind  string `json:"kind,omitempty"` // "" | TplVignette
	// Vig — роли, шапка и стиль; только при Kind == TplVignette.
	Vig *VigTpl `json:"vig,omitempty"`
}

// VigTpl — свой шаблон виньетки сверх мест (038 design §1.1).
type VigTpl struct {
	VigGeo
	Style VigStyle `json:"style"` // оформление, которое приносит выбор шаблона
}

// VigGeo — геометрия своего шаблона без мест (места — Template.Boxes / VigShape.Boxes).
type VigGeo struct {
	Roles  []string `json:"roles"`          // по одной на место: pupil | teacher | group
	Head   *Box     `json:"head,omitempty"` // полоса шапки в долях области; nil — шапки нет
	Pages  int      `json:"pages"`          // 1 — лист (Settings.Sheet), 2 — разворот
	Aspect float64  `json:"aspect"`         // W/H области при сохранении
	Ratio  string   `json:"ratio"`          // портрет: "2:3" | "3:4" | "4:5" | "1:1"
	// W, H — размер области при сохранении, мм; 0 — неизвестен (шаблон до поля). Тот же размер,
	// портрет и числа людей — места встают ровно как сохранены (vignette.BuildOwn).
	W float64 `json:"w,omitempty"`
	H float64 `json:"h,omitempty"`
}

// VigStyle — оформление шаблона виньетки; данные класса (текст шапки, поля) не хранятся.
type VigStyle struct {
	Caps  *SheetCaps `json:"caps,omitempty"`  // стиль подписей листа с шаблоном имени (030)
	TText string     `json:"ttext,omitempty"` // шаблон подписи учителя
	Head  *TextStyle `json:"head,omitempty"`  // стиль надписи-шапки
	// Shape, Round — форма мест учеников и учителей (044, Vignette.Shape/Round).
	Shape string  `json:"shape,omitempty"`
	Round float64 `json:"round,omitempty"`
}

type templatesFile struct {
	Version   int        `json:"version"`
	Templates []Template `json:"templates"`
}

// ParseTemplates разбирает templates.json: список после зажима и версия файла.
// Больше MaxTemplatesFile или битый JSON — ошибка (вызывающий считает список
// пустым и не затирает файл). Версия новее TemplatesVersion читается так же:
// что понятно — берём, запись вызывающий запрещает. Список укладывается в
// MaxTemplatesFile при записи (fitFile).
func ParseTemplates(data []byte) ([]Template, int, error) {
	if len(data) > MaxTemplatesFile {
		return nil, 0, errors.New("templates.json больше 1 МБ")
	}
	// Сначала только версия: тело новой версии может быть устроено иначе, и это
	// не «битый файл», а «новее программы» (его не затирают, запись запрещена).
	var head struct {
		Version int `json:"version"`
	}
	if err := json.Unmarshal(data, &head); err != nil {
		return nil, 0, err
	}
	if head.Version < 1 {
		return nil, 0, errors.New("templates.json без версии")
	}
	var f templatesFile
	if err := json.Unmarshal(data, &f); err != nil {
		if head.Version > TemplatesVersion {
			return nil, head.Version, nil
		}
		return nil, 0, err
	}
	return fitFile(ClampTemplates(f.Templates)), head.Version, nil
}

// fitFile — начало списка, которое MarshalTemplates уложит в MaxTemplatesFile. Зачем: зажим
// мог вырастить файл (роли по числу мест, имена «Шаблон N»), и Store отказывал бы в любой
// записи, даже в удалении, — из такого списка нельзя было бы ничего убрать. Отбрасываются
// последние шаблоны (в файле они в порядке сохранения), с записью в журнал; сам файл prefs
// перед первой записью уводит в templates.bad-<время>.json, так что отброшенные не пропадают.
func fitFile(list []Template) []Template {
	data, err := MarshalTemplates(list)
	if err != nil || len(data) <= MaxTemplatesFile {
		return list
	}
	// В файле шаблоны идут через запятую и пишутся так же, как поодиночке: размер каждого
	// считается один раз, а не Marshal всего списка на каждый отброшенный (200 × 1 МБ).
	empty, _ := MarshalTemplates(nil)
	n := len(list)
	for over := len(data) - MaxTemplatesFile; n > 0 && over > 0; n-- {
		one, _ := MarshalTemplates(list[n-1 : n])
		over -= len(one) - len(empty) + 1
	}
	log.Printf("templates.json: после зажима больше 1 МБ, отброшено шаблонов: %d из %d", len(list)-n, len(list))
	return list[:n]
}

// MarshalTemplates — содержимое templates.json (после ClampTemplates NaN не бывает).
// Версия — наименьшая, понимающая все поля: без углов 1, чтобы 0.3.0/0.4.0
// продолжали править файл (v2 они только читают); шаблон больше v2Places мест — 3:
// 0.9.x откроет файл только для чтения и не потеряет места при записи. Без отступов,
// доли и угол округлены (RoundTemplates): формат тот же, старые версии читают как раньше.
// Шаблон виньетки — 4: прежние программы файл не пишут и не теряют роли и стиль.
// Форма места или виньетки — 5: 0.11 видит прямоугольники и не стирает формы записью.
func MarshalTemplates(list []Template) ([]byte, error) {
	list = RoundTemplates(list)
	ver := 1
	switch {
	case slices.ContainsFunc(list, tplShaped):
		ver = 5
	case slices.ContainsFunc(list, func(t Template) bool { return t.Kind == TplVignette }):
		ver = 4
	case slices.ContainsFunc(list, func(t Template) bool { return len(t.Boxes) > v2Places }):
		ver = 3
	case slices.ContainsFunc(list, func(t Template) bool {
		return slices.ContainsFunc(t.Boxes, func(b Box) bool { return b.Rot != 0 })
	}):
		ver = 2
	}
	// Без экранирования < > & (файл не HTML): имя из «<» иначе в 6 раз длиннее, и 200
	// шаблонов виньетки с ролями и стилем не укладывались в MaxTemplatesFile (038).
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(templatesFile{Version: ver, Templates: list}); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(buf.Bytes(), []byte("\n")), nil
}

// tplShaped — у шаблона есть форма места или формы виньетки (templates.json v5).
func tplShaped(t Template) bool {
	return slices.ContainsFunc(t.Boxes, func(b Box) bool { return b.Shape != "" || b.Round != 0 }) ||
		t.Vig != nil && (t.Vig.Style.Shape != "" || t.Vig.Style.Round != 0)
}

// RoundTemplates — копия списка с долями и углом, кратными 1/tplPrec, как их пишет
// MarshalTemplates (повторно — то же); вход не меняется. Угол после ClampBoxes и так
// с шагом 0.1, округление лишь убирает хвост двоичной дроби. У виньетки — и шапка,
// и пропорция области. nil — пустой список.
func RoundTemplates(list []Template) []Template {
	out := make([]Template, len(list))
	for i, t := range list {
		boxes := make([]Box, len(t.Boxes))
		for j, b := range t.Boxes {
			boxes[j] = roundBox(b)
		}
		t.Boxes = boxes
		if t.Vig != nil {
			v := *t.Vig
			v.Roles = slices.Clone(v.Roles)
			if v.Head != nil {
				h := roundBox(*v.Head)
				v.Head = &h
			}
			v.Aspect, v.W, v.H = roundTpl(v.Aspect), roundTpl(v.W), roundTpl(v.H)
			t.Vig = &v
		}
		out[i] = t
	}
	return out
}

func roundBox(b Box) Box {
	return Box{X: roundTpl(b.X), Y: roundTpl(b.Y), W: roundTpl(b.W), H: roundTpl(b.H), Rot: roundTpl(b.Rot), Shape: b.Shape, Round: b.Round}
}

// roundTpl — v до 1/tplPrec; -0 пишется как 0.
func roundTpl(v float64) float64 {
	if r := math.Round(v*tplPrec) / tplPrec; r != 0 {
		return r
	}
	return 0
}

// ClampTemplates — зажим недоверенного списка (файл, интерфейс): область не из
// page/spread, шаблон без мест и сверх MaxTemplates выбрасываются; доли — ClampBoxes;
// имя — CleanTemplateName; плохой или повторный ID — новый. Виньетка — clampVigTpl,
// без vig — обычная сетка; чужой Kind — шаблон выбрасывается (версия файла не новее
// нашей, значит, он испорчен). Вход не меняется.
func ClampTemplates(in []Template) []Template {
	out := make([]Template, 0, min(len(in), MaxTemplates))
	seen := map[string]bool{}
	for _, t := range in {
		if len(out) >= MaxTemplates {
			break
		}
		if t.Scope != TplPage && t.Scope != TplSpread || t.Kind != "" && t.Kind != TplVignette {
			continue
		}
		var boxes []Box
		var vig *VigTpl
		if t.Kind == TplVignette && t.Vig != nil {
			if boxes, vig = clampVigTpl(t.Boxes, *t.Vig); vig == nil {
				continue
			}
			t.Scope = TplSpread
		} else {
			boxes = ClampBoxes(t.Boxes)
		}
		if len(boxes) == 0 {
			continue
		}
		for !IsTextID(t.ID) || seen[t.ID] {
			t.ID = NewID()
		}
		seen[t.ID] = true
		nt := Template{ID: t.ID, Name: CleanTemplateName(t.Name, len(out)+1), Scope: t.Scope, Boxes: boxes}
		if vig != nil {
			nt.Kind, nt.Vig = TplVignette, vig
		}
		out = append(out, nt)
	}
	return out
}

// clampVigTpl — места и шаблон виньетки из файла; nil — шаблон выбрасывается.
func clampVigTpl(in []Box, v VigTpl) ([]Box, *VigTpl) {
	boxes, g, ok := clampVigGeo(in, v.VigGeo)
	if !ok {
		return nil, nil
	}
	st := VigStyle{TText: CleanText(v.Style.TText, MaxCaptionRunes)}
	st.Shape, st.Round = ClampShape(v.Style.Shape, v.Style.Round)
	if strings.TrimSpace(st.TText) == "" {
		st.TText = ""
	}
	if v.Style.Caps != nil {
		c := ClampSheetCaps(*v.Style.Caps)
		st.Caps = &c
	}
	if v.Style.Head != nil {
		h := ClampStyle(*v.Style.Head)
		st.Head = &h
	}
	return boxes, &VigTpl{VigGeo: g, Style: st}
}

// clampVigGeo — места с ролями и геометрия шаблона виньетки (файл шаблонов, Load листа):
// место с NaN/Inf уходит вместе со своей ролью, ролей — по числу мест (недостающие и
// чужие — ученик), угол — 0, доли — ClampBoxes; шапка — ClampBoxes одного места без угла; пропорция области
// конечная в MinVigAspect…MaxVigAspect (иначе 1), портрет — известный (иначе VigRatio);
// размер области — okArea у обеих сторон, иначе 0.
// ok=false — страниц не 1 и не 2 или нет мест учеников и учителей: строить нечего.
func clampVigGeo(in []Box, g VigGeo) ([]Box, VigGeo, bool) {
	if g.Pages != 1 && g.Pages != 2 {
		return nil, VigGeo{}, false
	}
	boxes := make([]Box, 0, min(len(in), MaxPlaces))
	roles := make([]string, 0, min(len(in), MaxPlaces))
	for i, b := range in {
		if len(boxes) >= MaxPlaces {
			break
		}
		b.Rot = 0 // BuildOwn угла не знает: под углом зажим держал бы внутри только центр
		if !finiteBox(b) {
			continue
		}
		r := RolePupil
		if i < len(g.Roles) && (g.Roles[i] == RoleTeacher || g.Roles[i] == RoleGroup) {
			r = g.Roles[i]
		}
		boxes, roles = append(boxes, b), append(roles, r)
	}
	boxes = ClampBoxes(boxes) // все конечные и не больше MaxPlaces: пары с ролями не сдвигаются
	if !slices.ContainsFunc(roles, func(r string) bool { return r != RoleGroup }) {
		return nil, VigGeo{}, false
	}
	out := VigGeo{Roles: roles, Pages: g.Pages, Aspect: 1, Ratio: clampRatio(g.Ratio)}
	if !math.IsNaN(g.Aspect) && !math.IsInf(g.Aspect, 0) {
		out.Aspect = geom.ClampF(g.Aspect, MinVigAspect, MaxVigAspect)
	}
	if okArea(g.W) && okArea(g.H) {
		out.W, out.H = g.W, g.H
	}
	if g.Head != nil {
		hb := *g.Head
		hb.Rot = 0
		if h := ClampBoxes([]Box{hb}); len(h) == 1 {
			out.Head = &h[0]
		}
	}
	return boxes, out, true
}

// okArea — сторона области шаблона из файла: конечная, от страницы до двух страниц
// наибольшего формата (ClampSettings). Иначе размер неизвестен — шаблон без точного режима.
func okArea(v float64) bool { return v >= minPage && v <= 2*maxPage }

// Groups — групповых мест в шаблоне.
func (g VigGeo) Groups() int {
	n := 0
	for _, r := range g.Roles {
		if r == RoleGroup {
			n++
		}
	}
	return n
}

// equal — та же геометрия (без мест).
func (g VigGeo) equal(o VigGeo) bool {
	return slices.Equal(g.Roles, o.Roles) && g.Pages == o.Pages && g.Aspect == o.Aspect && g.Ratio == o.Ratio && g.W == o.W && g.H == o.H &&
		(g.Head == nil) == (o.Head == nil) && (g.Head == nil || *g.Head == *o.Head)
}

// ClampBoxes — доли места в 0..1: NaN/Inf выбрасывает место, W, H ≥ minBoxFrac,
// место целиком внутри области; не больше MaxPlaces мест. Rot — geom.NormDeg; у
// повёрнутого внутри только центр: положение в мм зажмёт clampPlace (hold) —
// иначе копия места у края под углом съезжала бы внутрь.
func ClampBoxes(in []Box) []Box {
	out := make([]Box, 0, min(len(in), MaxPlaces))
	for _, b := range in {
		if len(out) >= MaxPlaces {
			break
		}
		if !finiteBox(b) {
			continue
		}
		b.W = geom.ClampF(b.W, MinBoxFrac, 1)
		b.H = geom.ClampF(b.H, MinBoxFrac, 1)
		b.Shape, b.Round = ClampShape(b.Shape, b.Round)
		if b.Rot = geom.NormDeg(b.Rot); b.Rot == 0 {
			b.X = geom.ClampF(b.X, 0, 1-b.W)
			b.Y = geom.ClampF(b.Y, 0, 1-b.H)
		} else {
			b.X = geom.ClampF(b.X, -b.W/2, 1-b.W/2)
			b.Y = geom.ClampF(b.Y, -b.H/2, 1-b.H/2)
		}
		out = append(out, b)
	}
	return out
}

func finiteBox(b Box) bool {
	for _, v := range [...]float64{b.X, b.Y, b.W, b.H, b.Rot} {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return false
		}
	}
	return true
}

// CleanTemplateName — имя без управляющих знаков и лишних пробелов, ≤ MaxTemplateName;
// пустое — «Шаблон N».
func CleanTemplateName(s string, n int) string {
	s = strings.Join(strings.Fields(CleanText(s, 4*MaxTemplateName)), " ")
	if utf8.RuneCountInString(s) > MaxTemplateName {
		s = strings.TrimSpace(string([]rune(s)[:MaxTemplateName]))
	}
	if s == "" {
		return fmt.Sprintf("Шаблон %d", n)
	}
	return s
}

// NoPlacesError — ErrNoPlaces с текстом по области; errors.Is(…, ErrNoPlaces) верно.
type NoPlacesError string

func (e NoPlacesError) Error() string { return string(e) }

func (NoPlacesError) Unwrap() error { return ErrNoPlaces }
