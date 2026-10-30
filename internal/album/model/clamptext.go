package model

import (
	"errors"
	"fmt"
	"maps"
	"math"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/IliaSA101/sosninaalbums/internal/geom"
)

// ClampDesign — зажим оформления прочитанного проекта: кадры, подписи, тон фото, фон,
// блоки текста (лишние и пустые отбрасываются, плохой или повторный ID — новый).
// Координаты блоков и ширину «по тексту» потом держит FitTexts. Запас текста — spendTexts.
// Стикеры (048) — после текстов: их ID не совпадают с ID текстов и мест.
func (p *Project) ClampDesign() {
	p.Settings.Bg = BgColor(p.Settings.Bg)
	p.spendTexts(true)
	p.clampStickers()
	// Тон (068), прозрачность (079) и свой кадр (098) — всем фото, не только ленты: запаса текста не
	// тратят, мусор из файла зажимается.
	for id, r := range p.Photos {
		if r.Adj != nil || r.Fade != 0 || r.Asp != 0 {
			r.Adj, r.Fade, r.Asp = ClampTone(r.Adj), ClampPhotoFade(r.Fade), ClampAsp(r.Asp)
			p.Photos[id] = r
		}
	}
}

// clampSheetTexts — блоки листа s при Load: лишние и не влезшие в запас отбрасываются,
// величины зажимаются от размера листа (гигантские x/y/w дали бы Inf в FitTexts).
func (p *Project) clampSheetTexts(s *Spread, b *textBudget, seen map[string]bool) {
	g := p.Geo(s)
	lim := 2 * math.Max(g.W, g.H)
	out := s.Texts[:0]
	for _, t := range s.Texts {
		if t == nil || len(out) >= MaxTexts {
			continue
		}
		t.Text = CleanText(t.Text, MaxTextRunes)
		if strings.TrimSpace(t.Text) == "" || !b.take(t.Text) {
			continue
		}
		if !IsTextID(t.ID) || seen[t.ID] {
			t.ID = NewID()
		}
		seen[t.ID] = true
		t.Style = ClampStyle(t.Style)
		t.W = geom.ClampF(Finite(t.W, 0.2*g.W), 0, g.W)
		t.X, t.Y = geom.ClampF(Finite(t.X, 0), -lim, lim), geom.ClampF(Finite(t.Y, 0), -lim, lim)
		t.Rot = geom.NormDeg(t.Rot)
		out = append(out, t)
	}
	clear(s.Texts[len(out):])
	s.Texts = out
	if len(s.Texts) == 0 {
		s.Texts = nil
	}
}

// spendTexts — единый обход общего запаса текста для ClampDesign (fix: лишнее отбрасывается,
// поля зажимаются) и CheckTotals (!fix: false — запас превышен). Порядок один — поля шапки
// всех листов (Spread.Vars), листы (обложка первой: шаблон подписей листа, подпись учителя
// виньетки, блоки, таблички мест 071), подписи и имена фото в порядке ленты; поля шапки и имена — только знаки
// (решение 61). Поля шапки — отдельным первым проходом: шаблон обложки раскрывается полями
// первой виньетки (VarsOf), и её поля должны быть уже зажаты. Разный счёт дал бы правку,
// которую Load молча обрежет (TestTotalsMatchLoad). Шаблоны листа списываются столько раз,
// сколько показываются, по раскрытой длине (tplCost; decisions 79): иначе шаблон на 64 местах
// тысячи листов раздул бы вид до сотен МБ.
func (p *Project) spendTexts(fix bool) bool {
	b := textBudget{blocks: MaxTotalTexts, runes: MaxTotalRunes}
	sheets := p.Sheets()
	for _, s := range sheets {
		if fix {
			s.Vars = clampVarsIn(s.Vars, &b)
		} else if s.Vars != nil && !b.takeRunes(s.Vars.runes()) {
			return false
		}
	}
	seen, varsOf := map[string]bool{}, p.SheetVars() // поля уже зажаты: дальше не меняются
	for _, s := range sheets {
		v := varsOf(s)
		if fix {
			s.BgL, s.BgR = BgColor(s.BgL), BgColor(s.BgR)
			s.Name = CleanName(s.Name)
			p.clampBackdrops(s)
			clampPatterns(s)
			s.Caps = clampCapsIn(s.Caps, func(t string) int { return tplCost(v, t, capShows(s)) }, &b)
			s.Vig = clampVigIn(s, func(t string) int { return tplCost(v, t, teacherShows(s)) }, &b)
			p.clampSheetTexts(s, &b, seen)
			s.spendSeats(&b, true)
			continue
		}
		if !s.spendSheet(v, &b) {
			return false
		}
	}
	for _, id := range p.ribbonPhotos() {
		r := p.Photos[id]
		if fix {
			r.Frame = ClampFrame(r.Frame)
			r.Caption = clampCaption(r.Caption, &b)
			r.Person = clampPersonIn(r.Person, &b)
			p.Photos[id] = r
			continue
		}
		if c := r.Caption; c != nil && strings.TrimSpace(c.Text) != "" && !b.take(c.Text) {
			return false
		}
		if r.Person != nil && !b.takeRunes(r.Person.runes()) {
			return false
		}
	}
	return true
}

// spendSheet — счёт листа s для CheckTotals в порядке зажима Load (spendTexts): шаблон
// подписей, подпись учителя, блоки, таблички мест; false — запас превышен.
func (s *Spread) spendSheet(v *Vars, b *textBudget) bool {
	if s.Caps != nil && !b.takeRunes(tplCost(v, s.Caps.Text, capShows(s))) {
		return false
	}
	if s.Kind == KindVignette && s.Vig != nil && !b.takeRunes(tplCost(v, s.Vig.TText, teacherShows(s))) {
		return false
	}
	for _, t := range s.Texts {
		if t != nil && strings.TrimSpace(t.Text) != "" && !b.take(t.Text) {
			return false
		}
	}
	return s.spendSeats(b, false)
}

// tplCost — знаков запаса на шаблон листа tpl, показанный shows раз: на каждый показ — длина
// исходника или раскрытого с полями листа v (VarsOf, как у заглушки, ShownCaption), что больше.
// Иначе «{Год}» (5 знаков) при годе в 120 знаков у каждого места дал бы вид в 24 раза больше
// списанного. Поля шапки Load зажимает раньше (первый проход spendTexts), поэтому счёт при
// Load и в CheckTotals один.
func tplCost(v *Vars, tpl string, shows int) int {
	if tpl == "" {
		return 0
	}
	shown, _ := ShownCaption(v, tpl, nil, FieldStub)
	return max(runes(tpl), runes(shown)) * shows
}

// capShows — сколько раз шаблон подписей листа s может показаться: у каждого фото и у пустого
// места (заглушка). С сеткой фото сидят в местах, поэтому показов ≤ max(фото, мест); не меньше
// одного (как до 0.10). После ClampDesign Load фото и мест не добавляет сверх этого (ClampGrid
// режет, SyncGrid сажает фото в пустые места или ставит место на фото без места) — счёт
// CheckTotals после Load не больше списанного при Load.
func capShows(s *Spread) int { return max(1, len(s.Photos), len(s.Grid)) }

// teacherShows — мест учителя у виньетки s (подпись Vig.TText), не меньше одного.
func teacherShows(s *Spread) int {
	n := 0
	for _, pl := range s.Grid {
		if pl != nil && pl.Role == RoleTeacher {
			n++
		}
	}
	return max(1, n)
}

// ribbonPhotos — ID фото проекта: сначала в порядке листов (обложка первой), затем
// остальные по возрастанию — лишнее отбрасывается предсказуемо.
func (p *Project) ribbonPhotos() []string {
	ids := make([]string, 0, len(p.Photos))
	done := make(map[string]bool, len(p.Photos))
	for _, s := range p.Sheets() {
		for _, id := range s.Photos {
			if _, ok := p.Photos[id]; ok && !done[id] {
				done[id] = true
				ids = append(ids, id)
			}
		}
	}
	for _, id := range slices.Sorted(maps.Keys(p.Photos)) {
		if !done[id] {
			ids = append(ids, id)
		}
	}
	return ids
}

// textBudget — общий запас на тексты и подписи проекта: файл с тысячами блоков
// не должен раздувать раскладку текста и экспорт.
type textBudget struct{ blocks, runes int }

// takeRunes списывает только знаки (поля 030: переменные, имена, шаблон листа —
// не блоки текста); false — запас знаков кончился.
func (b *textBudget) takeRunes(n int) bool {
	if n > b.runes {
		return false
	}
	b.runes -= n
	return true
}

// take списывает блок из s; false — запас кончился, блок надо отбросить.
func (b *textBudget) take(s string) bool {
	n := utf8.RuneCountInString(s)
	if b.blocks <= 0 || n > b.runes {
		return false
	}
	b.blocks--
	b.runes -= n
	return true
}

// CheckTotals — итог правки не должен превышать то, что Load оставит: общий запас
// текстов и подписей, MaxSpreads и стикеры (048). Иначе после Save+Load лишнее молча пропало бы.
// Развороты — отказ, только если их стало больше, чем в before: иначе переполненный
// проект «замёрз» бы, не проходила бы даже правка с удалением.
func (p *Project) CheckTotals(before *Project) error {
	if len(p.Spreads) > MaxSpreads && len(p.Spreads) > len(before.Spreads) {
		return fmt.Errorf("в альбоме не может быть больше %d разворотов", MaxSpreads)
	}
	if !p.spendTexts(false) {
		return errTextBudget()
	}
	return p.checkStickers(before)
}

// ErrTextBudget — правка вышла за общий запас текстов (CheckTotals); errors.Is верно и для
// errTextBudget(). По нему ядро повторяет бросок без автоимён (0.14 decisions 31).
var ErrTextBudget = errors.New("слишком много текста в проекте")

// textBudgetError — ErrTextBudget с пределами в тексте для пользователя.
type textBudgetError struct{}

func (textBudgetError) Error() string {
	return fmt.Sprintf("слишком много текста в проекте: не больше %d блоков и подписей и %d знаков", MaxTotalTexts, MaxTotalRunes)
}

func (textBudgetError) Unwrap() error { return ErrTextBudget }

func errTextBudget() error { return textBudgetError{} }

func clampCaption(c *Caption, b *textBudget) *Caption {
	if c == nil {
		return nil
	}
	t := CleanText(c.Text, MaxCaptionRunes)
	if strings.TrimSpace(t) == "" || !b.take(t) {
		return nil
	}
	return &Caption{Text: t, Style: ClampStyle(c.Style), Over: c.Over, Pad: ClampCaptionPad(c.Pad)}
}
