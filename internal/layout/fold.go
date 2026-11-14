package layout

// Правило сгиба (064, владелец 2026-10-17): одно для всех путей подбора раскладки.
// Ставится в генераторах до отсечения лимита, а не в Build: стоящий ключ вне правила
// (свёрстанный раньше) обязан строиться дальше (decisions 0.15 №1).

const (
	// FoldAspect — через корешок идёт только фото не уже этого W/H видимой части.
	FoldAspect = 1.2
	// FoldPart — и не меньше этой доли ширины фото с каждой стороны корешка (меньше — узкая
	// полоска, лицо почти на сгибе).
	FoldPart = 0.2
)

// FoldOK1 — фото x с пропорцией кадра a (≤ 0 — по рамке) может так лежать: не пересекает
// корешок дальше spineEps, или FoldAny, или min(W/H, a) ≥ FoldAspect и доля ≥ FoldPart с
// каждой стороны. Кадр важен «в край»: рамка шире кадра, и сгиб режет середину сюжета (№13).
// Сравнения — в квантах tieEps: FMA на arm64 не должен решать, пройдёт ли раскладка.
// Без Layflat пересекающее фото не проходит никогда; у листа корешка нет.
func FoldOK1(x Rect, a float64, p Params) bool {
	spine := p.PageW
	if p.Sheet || !(x.X < spine-spineEps && x.X+x.W > spine+spineEps) {
		return true
	}
	if !p.Layflat {
		return false
	}
	if p.FoldAny {
		return true
	}
	w := x.W / x.H
	if a > 0 {
		w = min(w, a)
	}
	part := min(spine-x.X, x.X+x.W-spine) / x.W
	return tieQ(w) >= tieQ(FoldAspect) && tieQ(part) >= tieQ(FoldPart)
}

// FoldOK — каждое фото раскладки проходит FoldOK1; asp[i] — кадр rects[i], nil — по рамкам
// (у деревьев рамка листа = кадр). Без Layflat — ни одно фото не пересекает корешок.
func FoldOK(rects []Rect, asp []float64, p Params) bool {
	for i, x := range rects {
		a := 0.0
		if asp != nil {
			a = asp[i]
		}
		if !FoldOK1(x, a, p) {
			return false
		}
	}
	return true
}

// foldOn — фильтр сгиба что-то отсекает: layflat-разворот (не лист) с правилом «только
// широкие». Иначе генераторы его не зовут: «любые» — прежнее поведение бит-в-бит (№3),
// без layflat блоки на разворот и так не строятся.
func foldOn(p Params) bool { return p.Layflat && !p.FoldAny && !p.Sheet }

// foldOf — p для фильтра блоков bestBlocksIn: nil, когда правило ничего не отсекает.
func foldOf(p Params) *Params {
	if !foldOn(p) {
		return nil
	}
	return &p
}
