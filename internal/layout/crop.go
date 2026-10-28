package layout

// Подрезка, чтобы закрыть область (спека 104): сплошной блок получает общий множитель f
// пропорций свободных фото и закрывает страницу или разворот в полях точно. Формула и
// решения — docs/specs/0.24/104-crop-to-fill/design.md §3.

import (
	"math"
	"strconv"
)

const (
	// MaxFitCrop — наибольшая подрезка по стороне (15 %). Не путать с maxCrop = 0.3 «в край».
	MaxFitCrop = 0.15
	// CropK — вес подрезки в оценке растянутого блока (замер design §4: 1,0 почти нейтрален
	// к заполнению и ставит выше из почти равных ту, где режем меньше).
	CropK = 1.0
	// CropPool — сколько лучших без подрезки раскладок ряда до ManyFrom фото растягивается
	// и пересортировывается; префикс ряда сохраняется (набор не зависит от limit).
	CropPool = ManyCap
	// cropEps — блок, чья высота уже в пределах eps мм от области, не трогаем (побитно как при 0).
	cropEps = 1e-6
)

// fitCrop — подрезка p в допустимых пределах: NaN и ≤ 0 дают 0. Params собирает модель,
// но другой проверки Crop в layout нет.
func fitCrop(p Params) float64 {
	if !(p.Crop > 0) {
		return 0
	}
	return math.Min(p.Crop, MaxFitCrop)
}

// plainTree — сплошной блок: фото без m или H/V из сплошных. Такой блок закрывает
// свой прямоугольник без дыр; G, L/A и m оставляют пустоту по замыслу.
func plainTree(t *keyTree) bool {
	if t.leaf >= 0 {
		return !t.small
	}
	if t.op != 'H' && t.op != 'V' {
		return false
	}
	for _, k := range t.kids {
		if !plainTree(k) {
			return false
		}
	}
	return true
}

// coef — высота блока при ширине w и множителе f: (A·w + B1)/f + B0. false — есть фото
// со своим кадром (Lock): его пропорция не растёт с f, формула не линейна.
func coef(t *keyTree, asp []float64, gap float64, lock uint64) (A, B0, B1 float64, ok bool) {
	if t.leaf >= 0 {
		if lock&(1<<uint(t.leaf)) != 0 {
			return 0, 0, 0, false
		}
		return 1 / asp[t.leaf], 0, 0, true
	}
	g := gap * float64(len(t.kids)-1)
	if t.op == 'V' {
		for _, k := range t.kids {
			a, b0, b1, ok := coef(k, asp, gap, lock)
			if !ok {
				return 0, 0, 0, false
			}
			A, B0, B1 = A+a, B0+b0, B1+b1
		}
		return A, B0 + g, B1, true
	}
	var inv, s0, s1 float64
	for _, k := range t.kids {
		a, b0, b1, ok := coef(k, asp, gap, lock)
		if !ok {
			return 0, 0, 0, false
		}
		inv += 1 / a
		s0 += b0 / a
		s1 += b1 / a
	}
	A = 1 / inv
	return A, A * s0, A * (s1 - g), true
}

// stretchTo растягивает сплошной блок kt на box: общий множитель f пропорций свободных
// фото, подрезка по стороне 1 − min(f, 1/f) ≤ fitCrop(p). false — блок уже закрыт, все
// фото со своим кадром или f вне предела; out тогда не годится.
func (g geom) stretchTo(kt *keyTree, asp []float64, box Rect, p Params, out []Rect) (crop float64, depth int, ok bool) {
	return g.stretchWith(kt, asp, box, p, out, true)
}

// stretchWith — stretchTo; formula=false — всегда деление пополам (сверка формулы в тесте).
func (g geom) stretchWith(kt *keyTree, asp []float64, box Rect, p Params, out []Rect, formula bool) (crop float64, depth int, ok bool) {
	t := fitCrop(p)
	A, B0, B1, lin := coef(kt, asp, g.gap, p.Lock)
	var f float64
	if lin && formula {
		if math.Abs(A*box.W+B1+B0-box.H) < cropEps {
			return 0, 0, false
		}
		f = (A*box.W + B1) / (box.H - B0)
		if !(f >= 1-t-1e-12 && f <= 1/(1-t)+1e-12) {
			return 0, 0, false
		}
	} else if f, ok = g.bisectF(kt, asp, box, t, p.Lock); !ok {
		return 0, 0, false
	}
	nd := kt.nodeF(asp, g.gap, f, p.Lock)
	if _, ok := fitTreeXY(nd, box, g.gap, out, 0.5, 0.5); !ok {
		return 0, 0, false
	}
	return 1 - math.Min(f, 1/f), nd.depth, true
}

// scaled — пропорции, где у свободных (без Lock) листьев kt a·f.
func scaled(kt *keyTree, asp []float64, f float64, lock uint64) []float64 {
	tmp := make([]float64, len(asp))
	copy(tmp, asp)
	for _, l := range kt.leaves(nil) {
		if lock&(1<<uint(l)) == 0 {
			tmp[l] = asp[l] * f
		}
	}
	return tmp
}

// nodeF — kt.node(scaled(kt, asp, f, lock)) без копии пропорций: блок сплошной (без m),
// горячий путь Build при подрезке.
func (t *keyTree) nodeF(asp []float64, gap, f float64, lock uint64) *node {
	if t.leaf >= 0 {
		a := asp[t.leaf]
		if lock&(1<<uint(t.leaf)) == 0 {
			a *= f
		}
		return &node{leaf: t.leaf, a: 1 / a, key: strconv.Itoa(t.leaf)}
	}
	kids := make([]*node, len(t.kids))
	for i, k := range t.kids {
		kids[i] = k.nodeF(asp, gap, f, lock)
	}
	return compose(t.op == 'H', kids, gap)
}

// bisectF — множитель делением пополам по ln f на [ln(1−t), −ln(1−t)]: высота блока при
// ширине box.W убывает с ростом f. false — все фото с Lock или концы не дают знака.
func (g geom) bisectF(kt *keyTree, asp []float64, box Rect, t float64, lock uint64) (float64, bool) {
	free := false
	for _, l := range kt.leaves(nil) {
		free = free || lock&(1<<uint(l)) == 0
	}
	if !free {
		return 0, false
	}
	excess := func(f float64) float64 {
		n := kt.node(scaled(kt, asp, f, lock), g.gap)
		return n.a*box.W + n.b - box.H
	}
	if math.Abs(excess(1)) < cropEps {
		return 0, false
	}
	lo, hi := math.Log(1-t), -math.Log(1-t)
	if excess(math.Exp(hi)) > cropEps || excess(math.Exp(lo)) < -cropEps {
		return 0, false
	}
	for range 60 {
		mid := (lo + hi) / 2
		if excess(math.Exp(mid)) > 0 {
			lo = mid
		} else {
			hi = mid
		}
	}
	return math.Exp((lo + hi) / 2), true
}

// buildCrop — Build при Crop > 0 (canon=false — ключ записан самим layout, запись не сверяем): сплошной блок разворота (T без модификатора) и сплошные
// страницы без модификатора растягиваются, остальное — как build0 (побитно).
func buildCrop(key string, aspects []float64, p Params, canon bool) (Result, bool) {
	p.Crop = fitCrop(p)
	k, ok := checkKey(key, aspects, p, canon)
	if !ok {
		return Result{}, false
	}
	g := newGeom(p)
	switch k.kind {
	case 'T':
		if k.mod != 0 || !p.Layflat || !plainTree(k.tree) {
			break
		}
		rects := make([]Rect, len(aspects))
		crop, depth, ok := g.stretchTo(k.tree, aspects, g.spread, p, rects)
		// Сгиб — на растянутом блоке: подрезка не должна пустить лицо на корешок.
		if !ok || foldOn(p) && !FoldOK(rects, nil, p) {
			break
		}
		return Result{Rects: rects, Score: score(rects, g.spread, g.spine, depth) - CropK*crop, Key: key, Crop: crop}, true
	case 'P':
		return g.buildPagesCrop(k, key, aspects, p)
	}
	return buildParsed(k, key, aspects, p)
}

// buildPagesCrop — 'P' при Crop > 0: у каждой страницы либо растяжение, либо прежний page.
func (g geom) buildPagesCrop(k parsedKey, key string, aspects []float64, p Params) (Result, bool) {
	rects := make([]Rect, len(aspects))
	var pg [2]pageGeo
	empty, maxc := false, 0.0
	for side, kt := range k.pages {
		if kt == nil {
			empty = true
			continue
		}
		if k.mods[side] == 0 && plainTree(kt) {
			if c, d, ok := g.stretchTo(kt, aspects, g.pages[side], p, rects); ok {
				pg[side] = pageGeo{h: g.pages[side].H, depth: d}
				maxc = math.Max(maxc, c)
				continue
			}
		}
		var ok bool
		if pg[side], ok = g.page(side, k.mods[side], kt, aspects, rects); !ok {
			return Result{}, false
		}
	}
	return Result{Rects: rects, Score: pagedScore(rects, g, pg, empty) - CropK*maxc, Key: key, Crop: maxc}, true
}

// layoutsCrop — ряд «Авто» при Crop > 0. До ManyFrom фото: лучшие CropPool без подрезки,
// каждая растягивается через Build и пересортировывается — набор не зависит от limit,
// так что ряд на 12 остаётся префиксом ряда на 24 и 48. С ManyFrom порядок прежний
// (пул 48 там +67 % времени, а префикс ряда держится только без пересортировки).
func layoutsCrop(aspects []float64, p Params, limit int) []Result {
	many := len(aspects) >= ManyFrom
	wide := limit
	if limit > 0 && limit <= CropPool && !many {
		wide = CropPool
	}
	raw := Cached(aspects, noCrop(p), wide)
	out := make([]Result, 0, len(raw))
	for _, r := range raw {
		if c, ok := buildCrop(r.Key, aspects, p, false); ok {
			out = append(out, c)
		}
	}
	if many {
		return out
	}
	return topUnique(out, limit, tieOrder)
}

// cropRows — ряды, посчитанные без подрезки (по noCrop), с Rects, Score и Crop из Build(p);
// порядок прежний. Нерастягиваемый ключ даёт те же Rects и Score.
func cropRows(rows []Result, aspects []float64, p Params) []Result {
	out := make([]Result, len(rows))
	for i, r := range rows {
		out[i] = r
		if c, ok := buildCrop(r.Key, aspects, p, false); ok {
			out[i] = c
		}
	}
	return out
}

// noCrop — p без подрезки: ряды считаются по нему, растяжение — после (cropRows).
func noCrop(p Params) Params {
	p.Crop, p.Lock = 0, 0
	return p
}
