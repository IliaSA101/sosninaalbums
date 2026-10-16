package model

import (
	"errors"
	"fmt"
	"math"
	"slices"

	"github.com/IliaSA101/sosninaalbums/internal/geom"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
)

// Place — место на развороте в режиме «Сетка» (спека 007). Сетка не надстройка
// над ключом, а замена: пока Spread.Grid не пуст, ключ раскладки не действует.
type Place struct {
	ID    string  `json:"id"` // 12 hex, NewID; уникален в проекте
	X     float64 `json:"x"`  // мм от левого верхнего угла разворота
	Y     float64 `json:"y"`
	W     float64 `json:"w"`
	H     float64 `json:"h"`
	Photo string  `json:"photo,omitempty"` // ID фото; "" — пустое место
	Fit   string  `json:"fit,omitempty"`   // "" — с обрезкой (cover + Frame), FitContain — целиком
	// Rot — градусы по часовой вокруг центра места, (−180, 180], шаг 0.1; X, Y, W, H —
	// неповёрнутый прямоугольник (design-rotation §2.1): поворот меняет одно поле.
	Rot float64 `json:"rot,omitempty"`
	// Role — роль места виньетки (031): RolePupil | RoleTeacher | RoleGroup; "" — ученик.
	// Едет с местом (деление, слои); вне виньетки всегда "" (FixVignettes).
	Role string `json:"role,omitempty"`
	// Shape — форма (044): "" — прямоугольник, ShapeOval — эллипс, вписанный в рамку (круг
	// при W = H). Round — скругление углов прямоугольника, % меньшей стороны, целое 0…50;
	// у ShapeOval всегда 0 (ClampShape). Рамка X, Y, W, H — описанный прямоугольник.
	Shape string  `json:"shape,omitempty"`
	Round float64 `json:"round,omitempty"`
}

// ShapeOval — Place.Shape эллипса (не "ellipse": 200 шаблонов × 64 места с ним не
// укладываются в MaxTemplatesFile, 044 research §2).
const ShapeOval = "oval"

// MaxRound — предел скругления, % меньшей стороны: 50 — у квадрата на вид круг.
const MaxRound = 50.0

// ClampShape — форма и скругление из файла, шаблона или правки: форма не из белого
// списка — прямоугольник; скругление NaN/Inf/< 0 — 0, > MaxRound — MaxRound, целое;
// у эллипса скругления нет. Идемпотентна.
func ClampShape(shape string, round float64) (string, float64) {
	if shape != ShapeOval {
		shape = ""
	}
	if shape == ShapeOval || !(round > 0) || math.IsInf(round, 1) {
		return shape, 0
	}
	return shape, math.Round(min(round, MaxRound))
}

// ShapeRadius — радиус скругления, мм: round % меньшей стороны рамки w×h.
func ShapeRadius(round, w, h float64) float64 {
	return round / 100 * min(w, h)
}

// Outline — форма места для расчётов geom (кадр, «Целиком», подпись, маска экспорта).
func (pl *Place) Outline() geom.Outline {
	return geom.Outline{Oval: pl.Shape == ShapeOval, Radius: ShapeRadius(pl.Round, pl.W, pl.H)}
}

// ClonePlace — копия места целиком (новые поля Place переезжают сами) с новым ID
// и без фото: деление и копия сетки дают пустые места.
func ClonePlace(pl *Place, taken map[string]bool) *Place {
	c := *pl
	c.ID, c.Photo = NewPlaceID(taken), ""
	return &c
}

// FitContain — фото вписано в место целиком, без кадра (логотипы, PNG).
const FitContain = "contain"

// ErrNoPlace — фото некуда сесть: сетка — та же, что на других листах, молча
// добавлять ей место нельзя (design 007 §0.7).
var ErrNoPlace = errors.New("на развороте нет свободных мест")

func ErrTooManyPlaces() error {
	return fmt.Errorf("на развороте не может быть больше %d мест", MaxPlaces)
}

// GridPhotos — фото непустых мест по порядку сетки: это и есть Photos разворота-сетки.
func GridPhotos(s *Spread) []string {
	out := make([]string, 0, len(s.Grid))
	for _, pl := range s.Grid {
		if pl.Photo != "" {
			out = append(out, pl.Photo)
		}
	}
	return out
}

// Put — единственный способ посадить фото в место: фото уходит отовсюду (его
// прежнее место пустеет), прежний хозяин места покидает разворот, Photos
// пересобирается по местам. pl должен принадлежать s.
func (p *Project) Put(s *Spread, pl *Place, id string) {
	p.Detach([]string{id})
	pl.Photo = id
	s.Photos = GridPhotos(s)
}

// DropOn — первое брошенное фото id на место pl листа s (design 007 §3, 042): то же фото —
// ничего; занятое место и фото из альбома (inAlbum — до Remember) — обмен; иначе перенос, а
// прежний хозяин занятого места уходит в ленту (out). Общее у PlacePhotos и броска 042.
func (p *Project) DropOn(s *Spread, pl *Place, id string, inAlbum bool) (out bool) {
	switch {
	case pl.Photo == id:
	case pl.Photo != "" && inAlbum:
		p.Swap(id, pl.Photo)
	default:
		out = pl.Photo != ""
		p.Put(s, pl, id)
	}
	return out
}

// NextEmpty — первое пустое место после позиции at, потом с начала; нет — nil.
func (s *Spread) NextEmpty(at int) *Place {
	n := len(s.Grid)
	for k := 1; k <= n; k++ {
		if pl := s.Grid[((at+k)%n+n)%n]; pl.Photo == "" {
			return pl
		}
	}
	return nil
}

// SyncGrid — страховка после правки, а не механизм переноса (design 007 §0.3):
// место с фото не из Photos пустеет (и второе место того же фото), фото без
// места садится в первое пустое; Photos = непустые места по порядку. Пустых нет:
// repair (чтение файла, фото терять нельзя) — новое место, иначе errNoPlace.
// Идемпотентна: холостая правка остаётся холостой.
func (p *Project) SyncGrid(repair bool) error {
	var taken map[string]bool // ID мест проекта — только для починки, один раз на проход
	if repair {
		taken = p.PlaceIDs()
	}
	for _, s := range p.Sheets() {
		if len(s.Grid) == 0 {
			s.Grid = nil
			continue
		}
		if err := p.syncSpread(s, repair, taken); err != nil {
			return err
		}
	}
	// Пустые Photos обложки — nil, как их читает FixCoverSheet: иначе "photos":[] после
	// Save → Load стал бы null и правка, круг записи, не совпала бы по JSON.
	if c := p.Cover; c != nil && c.Sheet != nil && len(c.Sheet.Photos) == 0 {
		c.Sheet.Photos = nil
	}
	return nil
}

func (p *Project) syncSpread(s *Spread, repair bool, taken map[string]bool) error {
	in := make(map[string]bool, len(s.Photos))
	for _, id := range s.Photos {
		in[id] = true
	}
	seated := make(map[string]bool, len(s.Grid))
	for _, pl := range s.Grid {
		if pl.Photo == "" {
			continue
		}
		if !in[pl.Photo] || seated[pl.Photo] {
			pl.Photo = ""
			continue
		}
		seated[pl.Photo] = true
	}
	next := 0 // пустые места ищем по порядку, не возвращаясь к занятым
	for _, id := range s.Photos {
		if seated[id] {
			continue
		}
		for next < len(s.Grid) && s.Grid[next].Photo != "" {
			next++
		}
		var pl *Place
		switch {
		case next < len(s.Grid):
			pl = s.Grid[next]
		case !repair:
			return ErrNoPlace
		default:
			var err error
			if pl, err = p.AddPlace(s, 0, p.Aspects([]string{id})[0], taken); err != nil {
				// После clampGrid так не бывает (фото и мест ≤ MaxPlaces); если всё же
				// случилось — разворот уходит в «Авто», но фото не теряются.
				s.Grid = nil
				return nil
			}
		}
		pl.Photo = id
		seated[id] = true
	}
	s.Photos = GridPhotos(s)
	return nil
}

// PlaceIDs — ID всех мест проекта: новое место не должно совпасть с чужим.
func (p *Project) PlaceIDs() map[string]bool {
	ids := map[string]bool{}
	for _, s := range p.Sheets() {
		for _, pl := range s.Grid {
			ids[pl.ID] = true
		}
	}
	return ids
}

// NewPlaceID — NewID, которого ещё нет среди мест проекта.
func NewPlaceID(taken map[string]bool) string {
	for {
		if id := NewID(); !taken[id] {
			taken[id] = true
			return id
		}
	}
}

// AddPlace добавляет пустое место на страницу page листа (0 — левая, 1 — правая; Geo.Pages):
// по центру, сторона 1/3 меньшей стороны страницы, пропорции aspect (не > 0 — 1:1);
// каждое следующее сдвинуто на 5 мм вниз-вправо (лесенка), чтобы новые места не
// ложились друг на друга. Больше MaxPlaces — ошибка. taken — ID мест проекта
// (placeIDs), пополняется новым.
func (p *Project) AddPlace(s *Spread, page int, aspect float64, taken map[string]bool) (*Place, error) {
	if len(s.Grid) >= MaxPlaces {
		return nil, ErrTooManyPlaces()
	}
	if !(aspect > 0) || math.IsInf(aspect, 0) {
		aspect = 1
	}
	pg := p.Geo(s).Page(page)
	pw, ph := pg.W, pg.H
	side := min(pw, ph) / 3
	w, h := side, side/aspect
	if aspect < 1 {
		w, h = side*aspect, side
	}
	w, h = max(w, geom.MinPlace), max(h, geom.MinPlace)
	room := min((pw-w)/2, (ph-h)/2)
	steps := max(1, int(room/ladderStep)+1)
	d := float64(len(s.Grid)%steps) * ladderStep
	pl := &Place{ID: NewPlaceID(taken), W: w, H: h, X: pg.X + (pw-w)/2 + d, Y: pg.Y + (ph-h)/2 + d}
	p.ClampPlace(s, pl)
	s.Grid = append(s.Grid, pl)
	return pl, nil
}

// ladderStep — сдвиг каждого следующего нового места, мм.
const ladderStep = 5.0

// Freeze («Закрепить») переводит разворот в «Сетку» без изменения картинки:
// место i = ячейка i действующей раскладки, фото то же. У обычной ячейки
// пропорции фото, и cover при S = 1 — фото целиком; у ячейки «в край» cover по
// центру — прежний Fill. Layout не трогается: он нужен «Вернуть авто».
func (p *Project) Freeze(s *Spread) {
	if len(s.Grid) > 0 {
		return
	}
	cur, _, ok := p.Layout(s)
	if !ok || len(cur.Rects) != len(s.Photos) {
		return
	}
	taken := p.PlaceIDs()
	grid := make([]*Place, len(cur.Rects))
	for i, r := range cur.Rects {
		grid[i] = &Place{ID: NewPlaceID(taken), X: r.X, Y: r.Y, W: r.W, H: r.H, Photo: s.Photos[i]}
		p.ClampPlace(s, grid[i])
	}
	s.Grid = grid
}

// Unfreeze («Вернуть авто»): места снимаются, ключ ставится через setLayout —
// сохранённый, если Build строит его для нынешних фото, иначе лучшая. Лучшая
// хранится как "", иначе её выбор в галерее перестал бы быть холостым.
// Кадры фото остаются: в «Авто» они тоже действуют.
func (p *Project) Unfreeze(s *Spread) {
	if len(s.Grid) == 0 {
		return
	}
	s.Grid = nil
	key := s.Layout
	if key != "" {
		if _, ok := layout.Build(key, p.Aspects(s.Photos), p.Params()); !ok {
			key = ""
		}
	}
	p.SetLayout(s, key)
}

// EmptyGridDupes — у места, чьё фото не в Photos разворота, или второго места
// того же фото, Photo = "": геометрия сетки остаётся, фото не стоит дважды.
func EmptyGridDupes(s *Spread) {
	if len(s.Grid) == 0 {
		return
	}
	in := make(map[string]bool, len(s.Photos))
	for _, id := range s.Photos {
		in[id] = true
	}
	for _, pl := range s.Grid {
		if pl.Photo != "" && !in[pl.Photo] {
			pl.Photo = ""
		}
		delete(in, pl.Photo)
	}
}

// RenameRefs переводит ID фото по карте «старый → новый» и в Photos листов (с обложкой),
// и в местах: одной картой, иначе syncGrid пересадил бы фото по первым пустым местам.
func (p *Project) RenameRefs(moved map[string]string) {
	if len(moved) == 0 {
		return
	}
	for _, s := range p.Sheets() {
		for i, id := range s.Photos {
			if nid, ok := moved[id]; ok {
				s.Photos[i] = nid
			}
		}
		for _, pl := range s.Grid {
			if nid, ok := moved[pl.Photo]; ok {
				pl.Photo = nid
			}
		}
		for _, b := range s.Backdrops() {
			if nid, ok := moved[b.Photo]; ok {
				b.Photo = nid
			}
		}
		for _, st := range s.Stickers {
			if nid, ok := moved[st.Photo]; ok && st.Photo != "" {
				st.Photo = nid
			}
		}
	}
}

// GridMerge — ключ слияния серии правок мест (стрелки при одном выделении).
func GridMerge(session string) string {
	if !ValidSession(session) {
		return ""
	}
	return "grid:" + session
}

// PlaceRef — место по ссылке ref: сначала ID места, потом место с этим фото.
// Чужое и неизвестное — nil; пустое место находится только по своему ID.
func (s *Spread) PlaceRef(ref string) *Place {
	if _, pl := s.FindPlace(ref); pl != nil {
		return pl
	}
	if ref == "" {
		return nil
	}
	for _, pl := range s.Grid {
		if pl.Photo == ref {
			return pl
		}
	}
	return nil
}

// WithGrid — правка мест у любого разворота: «Авто» сначала закрепляется
// (freeze: картинка та же), затем fn. Если закрепить нельзя или fn ничего не
// изменила в закреплённых местах, «Авто» остаётся «Авто» (Grid = nil): холостая
// правка не оставляет следа. У сетки — просто fn. Закрепление и fn идут в одном
// try, поэтому это один шаг undo; ошибка fn откатывает всё.
func (p *Project) WithGrid(s *Spread, fn func() error) error {
	if len(s.Grid) > 0 {
		return fn()
	}
	p.Freeze(s)
	if len(s.Grid) == 0 {
		s.Grid = nil
		return nil
	}
	base := make([]Place, len(s.Grid))
	for i, pl := range s.Grid {
		base[i] = *pl
	}
	if err := fn(); err != nil {
		return err
	}
	if slices.EqualFunc(base, s.Grid, func(a Place, b *Place) bool { return a == *b }) {
		s.Grid = nil
	}
	return nil
}

// DeletePlaces — тело DeletePlaces (и DeleteSelection) внутри change.
func (p *Project) DeletePlaces(spreadID string, ids []string) {
	s := p.GridSpread(spreadID)
	if s == nil {
		return
	}
	gone := make(map[string]bool, len(ids))
	for _, ref := range ids {
		if pl := s.PlaceRef(ref); pl != nil {
			gone[pl.ID] = true
		}
	}
	var out []string
	all := true
	for _, pl := range s.Grid {
		switch {
		case !gone[pl.ID]:
			all = false
		case pl.Photo != "":
			out = append(out, pl.Photo)
		}
	}
	p.Detach(out)
	if all {
		p.Unfreeze(s)
		return
	}
	s.Grid = slices.DeleteFunc(s.Grid, func(pl *Place) bool { return gone[pl.ID] })
}

var (
	ErrScope = errors.New("неизвестная область раскладки")
)

// SpineEps — место, лишь касающееся корешка, его не пересекает (как в layout).
const SpineEps = 0.01

// CrossesSpine — место листа s пересекает шов (для «!» в интерфейсе при выключенном
// layflat). У повёрнутого — по описанному прямоугольнику: для вертикали он точен.
// Нет шва (лист 031) — false.
func (p *Project) CrossesSpine(s *Spread, pl *Place) bool {
	w, a := p.Geo(s).Mid, PlaceAABB(pl)
	return w > 0 && a.X < w-SpineEps && a.X+a.W > w+SpineEps
}

// FindPlace — место разворота по ID.
func (s *Spread) FindPlace(id string) (int, *Place) {
	for i, pl := range s.Grid {
		if pl.ID == id {
			return i, pl
		}
	}
	return -1, nil
}

// GridSpread — лист (и обложка) в режиме «Сетка» по ID; «Авто» и неизвестный — nil
// (правки мест у «Авто» холостые: ID мест неоткуда взять).
func (p *Project) GridSpread(id string) *Spread {
	if s := p.Sheet(id); s != nil && len(s.Grid) > 0 {
		return s
	}
	return nil
}

// Swap меняет два фото местами во всех листах (с обложкой) — и в Photos, и в местах
// (сетка ↔ «Авто» тоже).
func (p *Project) Swap(a, b string) {
	other := map[string]string{a: b, b: a}
	for _, s := range p.Sheets() {
		for i, id := range s.Photos {
			if o, ok := other[id]; ok {
				s.Photos[i] = o
			}
		}
		for _, pl := range s.Grid {
			if o, ok := other[pl.Photo]; ok {
				pl.Photo = o
			}
		}
	}
}

// Мост к internal/geom. Модельные типы (Place, Frame, Settings) лежат в .salbum и
// в models.ts, поэтому они не алиасы geom: сюда приходят значениями, расчёт — в geom.

// Rect — место как geom.RotRect (неповёрнутый прямоугольник и угол вокруг центра).
func (pl *Place) Rect() geom.RotRect {
	return geom.RotRect{X: pl.X, Y: pl.Y, W: pl.W, H: pl.H, Rot: pl.Rot}
}

// setRect пишет итог расчёта обратно; прочие поля места (ID, Photo, Fit) стоят.
func (pl *Place) setRect(r geom.RotRect) {
	pl.X, pl.Y, pl.W, pl.H, pl.Rot = r.X, r.Y, r.W, r.H, r.Rot
}

func PlaceBoxOf(pl *Place) geom.TextBox { return pl.Rect().Box() }
func PlaceAABB(pl *Place) layout.Rect   { return pl.Rect().AABB() }
func Hold(pl *Place) layout.Rect        { return pl.Rect().Hold() }

// clampPlaceIn — clampPlace в развороте sw×ph (geom.ClampPlaceIn).
func clampPlaceIn(pl *Place, sw, ph float64) {
	r := pl.Rect()
	geom.ClampPlaceIn(&r, sw, ph)
	pl.setRect(r)
}

// ClampMoveIn — зажим переноса повёрнутого места (geom.ClampMoveIn); prev — до правки.
func ClampMoveIn(pl *Place, prev Place, sw, ph float64) {
	r := pl.Rect()
	geom.ClampMoveIn(&r, prev.Rect(), sw, ph)
	pl.setRect(r)
}

// PlaceImage — фото целиком (мм) для ячейки cell и кадра f (geom.PlaceImage).
func PlaceImage(cell layout.Rect, aspect float64, f Frame) layout.Rect {
	return geom.PlaceImage(cell, aspect, geom.Crop{S: f.S, X: f.X, Y: f.Y, R: f.R})
}

// PlaceImageIn — PlaceImage для места формы o (geom.PlaceImageIn): фото закрывает форму.
func PlaceImageIn(cell layout.Rect, aspect float64, f Frame, o geom.Outline) layout.Rect {
	return geom.PlaceImageIn(cell, aspect, geom.Crop{S: f.S, X: f.X, Y: f.Y, R: f.R}, o)
}
