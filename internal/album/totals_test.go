package album_test

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// fullTexts — редактор, где тексты заняли общий запас: blocks блоков в runes знаков
// по MaxTexts на разворот (запас не делится на MaxTexts — последний неполный;
// после них разворот пустой, для AddText).
func fullTexts(t *testing.T, blocks, runes int) *album.Editor {
	t.Helper()
	e := edtest.Open(t, model.New(), "")
	spreads := (blocks + model.MaxTexts - 1) / model.MaxTexts
	edtest.Change(e, func(p *model.Project) {
		for i := range spreads {
			s := &model.Spread{ID: model.NewID()}
			for range min(model.MaxTexts, blocks-i*model.MaxTexts) {
				s.Texts = append(s.Texts, &model.TextBlock{ID: model.NewID(), Auto: true, Text: strings.Repeat("я", runes), Style: model.PlainStyle()})
			}
			p.Spreads = append(p.Spreads, s)
		}
		p.Spreads = append(p.Spreads, &model.Spread{ID: model.NewID(), Photos: []string{}})
	})
	if len(edtest.P(e).Spreads) != spreads+1 || len(edtest.P(e).Spreads[0].Texts) != model.MaxTexts {
		t.Fatalf("заготовка: %d разворотов", len(edtest.P(e).Spreads))
	}
	return e
}

func TestTotalTextsCountLimit(t *testing.T) {
	e := fullTexts(t, model.MaxTotalTexts, 1) // ровно в запас
	last := edtest.P(e).Spreads[len(edtest.P(e).Spreads)-1].ID
	rev := edtest.Rev(e)
	v, err := e.AddText(last, 0, "")
	if err == nil || edtest.Rev(e) != rev || len(v.Spreads[len(v.Spreads)-1].Texts) != 0 {
		t.Fatalf("блок сверх запаса: err=%v rev %d→%d", err, rev, edtest.Rev(e))
	}
	// Запас, который Load оставит, не теряется: файл с полным запасом открывается целиком.
	path := filepath.Join(testkit.TempDir(t), "f.salbum")
	if err := e.Save(path); err != nil {
		t.Fatal(err)
	}
	q, err := salbum.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, s := range q.Spreads {
		n += len(s.Texts)
	}
	if n != model.MaxTotalTexts {
		t.Fatalf("после Save+Load %d блоков из %d", n, model.MaxTotalTexts)
	}
	// Освободили место — добавление проходит.
	e.DeleteText(edtest.P(e).Spreads[0].Texts[0].ID)
	if _, err := e.AddText(last, 0, ""); err != nil {
		t.Fatal(err)
	}
}

func TestTotalTextsRunesLimit(t *testing.T) {
	e := fullTexts(t, model.MaxTotalRunes/model.MaxTextRunes, model.MaxTextRunes) // все знаки заняты
	last := edtest.P(e).Spreads[len(edtest.P(e).Spreads)-1].ID
	if _, err := e.AddText(last, 0, ""); err == nil {
		t.Fatal("блок сверх запаса знаков принят")
	}
}

func TestTotalTextsCaptionsCount(t *testing.T) {
	e := fullTexts(t, model.MaxTotalTexts, 1)
	edtest.NewSpread(e, -1, edtest.Photos("a"))
	rev := edtest.Rev(e)
	e.SetCaption("a", "подпись", "")
	if edtest.Rev(e) != rev {
		t.Fatal("подпись сверх запаса принята: Load её выбросил бы")
	}
}

// Поля подписей (030) у границы общего запаса знаков: правка (CheckTotals) пускает ровно
// то, что Load (ClampDesign) оставит, — после Save+Load ничего не пропало, сверх — отказ.
func TestTotalsMatchLoad(t *testing.T) {
	e := fullTexts(t, model.MaxTotalRunes/model.MaxTextRunes-1, model.MaxTextRunes) // свободно MaxTextRunes знаков
	last := edtest.P(e).Spreads[len(edtest.P(e).Spreads)-1].ID
	edtest.AddPhotos(e, last, edtest.Photos("a"), 0)
	ya := func(n int) string { return strings.Repeat("я", n) }
	rev := edtest.Rev(e)
	e.SetCaption("a", ya(40), "")
	e.SetSheetCaps(last, true, model.SheetCaps{Style: model.DefaultCaptionStyle(), Pad: 1, Text: ya(model.MaxCaptionRunes)})
	v := model.Vars{Title: ya(model.MaxVarRunes), Subtitle: ya(model.MaxVarRunes), Year: ya(model.MaxVarRunes), Lead: ya(model.MaxVarRunes)}
	setVars := func(v model.Vars) { edtest.Change(e, func(p *model.Project) { p.Spreads[0].Vars = &v }) } // окно виньетки — 031 Т2
	setVars(v)
	who := model.Person{Last: ya(model.MaxPersonRunes), First: ya(model.MaxPersonRunes), Middle: ya(model.MaxPersonRunes)}
	e.SetPerson("a", who, "")
	if used := 40 + model.MaxCaptionRunes + 4*model.MaxVarRunes + 3*model.MaxPersonRunes; used != model.MaxTextRunes || edtest.Rev(e) != rev+4 {
		t.Fatalf("заготовка: знаков %d из %d, правок %d", used, model.MaxTextRunes, edtest.Rev(e)-rev)
	}
	// Запас кончился: знак сверх — отказ, в каком бы поле он ни был.
	rev = edtest.Rev(e)
	e.SetCaption("a", ya(41), "")
	if edtest.Rev(e) != rev {
		t.Fatal("подпись сверх запаса принята")
	}
	over := v
	over.Lead = ya(model.MaxVarRunes - 1) // поля листа: тот же запас, знак сверх — отказ
	setVars(over)
	over.Lead += "яя"
	if setVars(over); edtest.Rev(e) != rev+1 {
		t.Fatalf("поле шапки сверх запаса: правок %d, ждали 1", edtest.Rev(e)-rev)
	}
	setVars(v)
	e.SetPerson("a", model.Person{Last: "Я"}, "")
	e.SetPerson("a", who, "") // вернули — снова ровно в запас
	path := filepath.Join(testkit.TempDir(t), "f.salbum")
	if err := e.Save(path); err != nil {
		t.Fatal(err)
	}
	q, err := salbum.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	s := q.Spreads[len(q.Spreads)-1]
	if len(s.Photos) != 1 {
		t.Fatalf("фото на листе: %v", s.Photos)
	}
	r := q.Photos[s.Photos[0]] // ID фото Load выводит из пути заново
	if qv := q.Spreads[0].Vars; qv == nil || *qv != v || r.Person == nil || *r.Person != who || s.Caps == nil ||
		s.Caps.Text != ya(model.MaxCaptionRunes) || r.Caption == nil || r.Caption.Text != ya(40) {
		t.Fatalf("после Save+Load поля потерялись: vars %v, имя %v, лист %v, подпись %v",
			q.Spreads[0].Vars != nil, r.Person != nil, s.Caps != nil, r.Caption != nil)
	}
}

// Подпись учителя виньетки (031, Vig.TText) — в общем запасе знаков тем же обходом, что у
// Load: правка пускает ровно то, что Load оставит; знак сверх — отказ. Шаблоны листа
// списываются на каждый показ (decisions 79): шаблон учеников — на каждое место, подпись
// учителя — на место учителя.
func TestTotalsVignetteTText(t *testing.T) {
	e := fullTexts(t, 200, 997) // свободно 600 знаков
	v, err := e.AddVignette(-1)
	if err != nil {
		t.Fatal(err)
	}
	sv := v.Spreads[len(v.Spreads)-1]
	n := utf8.RuneCountInString
	places := len(edtest.P(e).Spreads[len(v.Spreads)-1].Grid)
	if places != 25 || sv.Vig.Teachers != 1 {
		t.Fatalf("виньетка по умолчанию: мест %d, учителей %d", places, sv.Vig.Teachers)
	}
	free := 600 - places*n(model.PupilCaption) - n(model.TeacherCaption) - n(model.VigHeadText)
	if free <= 0 || n(model.TeacherCaption)+free+1 > model.MaxCaptionRunes { // иначе зажим CleanText спрятал бы отказ
		t.Fatalf("заготовка: свободно %d знаков", free)
	}
	vig := sv.Vig.Vignette
	vig.TText = strings.Repeat("я", n(model.TeacherCaption)+free+1)
	rev := edtest.Rev(e)
	if _, _, err := e.SetVignette(sv.ID, vig, nil); err == nil || edtest.Rev(e) != rev {
		t.Fatalf("подпись учителя сверх запаса принята: %v", err)
	}
	vig.TText = vig.TText[len("я"):]
	if _, _, err := e.SetVignette(sv.ID, vig, nil); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(testkit.TempDir(t), "f.salbum")
	if err := e.Save(path); err != nil {
		t.Fatal(err)
	}
	q, err := salbum.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if s := q.Spreads[len(q.Spreads)-1]; !s.IsVignette() || s.Vig.TText != vig.TText {
		t.Fatalf("после Save+Load подпись учителя потерялась: %+v", s.Vig)
	}
}

// Шаблон подписей листа показывается у каждого фото — и списывается из запаса на каждое
// (decisions 79): правка пускает ровно запас, фото сверх на лист с шаблоном — отказ; файл,
// где шаблон не влез, открывается без шаблона, а не раздувает вид, и правится дальше.
func TestTotalsSheetTemplatePerPhoto(t *testing.T) {
	e := fullTexts(t, 200, 998) // свободно 400 знаков
	ids := []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"}
	v := edtest.NewSpread(e, -1, edtest.Photos(ids...))
	sid := v.Spreads[len(v.Spreads)-1].ID
	ya := func(n int) string { return strings.Repeat("я", n) }
	caps := func(n int) model.SheetCaps {
		return model.SheetCaps{Style: model.DefaultCaptionStyle(), Pad: 1, Text: ya(n)}
	}
	rev := edtest.Rev(e)
	if e.SetSheetCaps(sid, true, caps(41)); edtest.Rev(e) != rev { // 41 × 10 > 400
		t.Fatal("шаблон сверх запаса на 10 фото принят")
	}
	if e.SetSheetCaps(sid, true, caps(40)); edtest.Rev(e) != rev+1 { // ровно 400
		t.Fatal("шаблон ровно в запас не принят")
	}
	rev = edtest.Rev(e)
	if _, err := e.AddPhotos(sid, edtest.Photos("k"), -1); err == nil || edtest.Rev(e) != rev {
		t.Fatalf("11-е фото на лист с шаблоном сверх запаса: %v", err)
	}
	path := filepath.Join(testkit.TempDir(t), "f.salbum")
	if err := e.Save(path); err != nil {
		t.Fatal(err)
	}
	q, err := salbum.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if s := q.Spreads[len(q.Spreads)-1]; s.Caps == nil || s.Caps.Text != ya(40) {
		t.Fatalf("после Save+Load шаблон в запасе потерялся: %+v", s.Caps)
	}
	// Файл с шаблоном на знак длиннее: Load снимает шаблон (стиль остаётся), проект правится.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	over := strings.Replace(string(data), `"`+ya(40)+`"`, `"`+ya(41)+`"`, 1) // ровно 40 «я» в кавычках — только шаблон
	if over == string(data) {
		t.Fatal("шаблон в файле не найден")
	}
	if err := os.WriteFile(path, []byte(over), 0o644); err != nil {
		t.Fatal(err)
	}
	if q, err = salbum.Load(path); err != nil {
		t.Fatal(err)
	}
	if s := q.Spreads[len(q.Spreads)-1]; s.Caps == nil || s.Caps.Text != "" {
		t.Fatalf("шаблон сверх запаса пережил Load: %+v", s.Caps)
	}
	e2 := edtest.Open(t, q, "")
	rev = edtest.Rev(e2)
	if e2.SetSheetCaps(sid, true, caps(40)); edtest.Rev(e2) != rev+1 {
		t.Fatal("после Load шаблон в запас не вернуть")
	}
}

// worstFile — файл у предела (decisions 79, A4): MaxSpreads листов по MaxPlaces пустых мест
// (заглушка у каждого) с шаблоном листа tpl; первый — виньетка с полями шапки по 120 знаков,
// остальные берут их у неё (VarsOf): одни поля на весь альбом — худший случай, как прежние
// поля альбома.
func worstFile(t *testing.T, tpl string) []byte {
	t.Helper()
	p := model.New()
	long := strings.Repeat("я", model.MaxVarRunes)
	vars := &model.Vars{Title: long, Subtitle: long, Year: long, Lead: long}
	caps := model.SheetCaps{Style: model.DefaultCaptionStyle(), Pad: 1, Text: tpl}
	k := 0
	for range model.MaxSpreads {
		s := &model.Spread{ID: model.NewID(), Photos: []string{}, Caps: &caps}
		for i := range model.MaxPlaces {
			k++
			s.Grid = append(s.Grid, &model.Place{ID: fmt.Sprintf("%012x", k), X: float64(i%8)*70 + 5, Y: float64(i/8)*35 + 3, W: 60, H: 25})
		}
		p.Spreads = append(p.Spreads, s)
	}
	vig := model.DefaultVignette()
	p.Spreads[0].Kind, p.Spreads[0].Vig, p.Spreads[0].Vars = model.KindVignette, &vig, vars
	data, err := salbum.Encode(p, "")
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// Худший вид из файла у предела (A4): шаблон листа размножается по местам, но списывается
// на каждый показ по раскрытой длине, а раскрытие режется до подписи. До 0.10 тот же файл
// давал вид 243 МБ за 3 с («{Год}»×60), 63 МБ («{Год}»); теперь ≈ 8,5 МБ за 0,1 с — почти
// весь объём — 64 000 мест. «{Год}» — самый большой вид на знак запаса (5 знаков → 120),
// «{Год}»×60 — самый длинный шаблон.
func TestWorstCaseView(t *testing.T) {
	if testing.Short() {
		t.Skip("долгий: тысяча листов")
	}
	const maxMB = 16
	for _, tpl := range []string{"{Год}", strings.Repeat("{Год}", model.MaxCaptionRunes/5)} {
		p, err := salbum.Decode(worstFile(t, tpl), testkit.TempDir(t))
		if err != nil {
			t.Fatal(err)
		}
		if v := p.VarsOf(p.Spreads[len(p.Spreads)-1]); v == nil || v.Year == "" {
			t.Fatalf("поля шапки виньетки не дошли до последнего листа: %+v", v)
		}
		e := edtest.Open(t, p, "")
		var size, stubs, runes int
		d := testkit.Fastest(2, func() {
			v := e.View()
			b, err := json.Marshal(v)
			if err != nil {
				t.Fatal(err)
			}
			size, stubs, runes = len(b), 0, 0
			for _, sv := range v.Spreads {
				for _, st := range sv.Stubs {
					stubs++
					n := utf8.RuneCountInString(st.Text)
					runes += n
					if n > model.MaxCaptionRunes {
						t.Fatalf("заглушка длиной %d", n)
					}
				}
			}
		})
		t.Logf("шаблон %d знаков: заглушек %d, знаков в них %d, вид %.1f МБ за %v", utf8.RuneCountInString(tpl), stubs, runes, float64(size)/(1<<20), d)
		if runes > model.MaxTotalRunes || size > maxMB<<20 || d > testkit.Limit(1500*time.Millisecond) {
			t.Fatalf("вид у предела: заглушек %d, %.1f МБ за %v", stubs, float64(size)/(1<<20), d)
		}
	}
}

func TestTotalTextsDuplicateGridDesign(t *testing.T) {
	e := edtest.Open(t, model.New(), "")
	edtest.NewSpread(e, -1, edtest.Photos("a", "b"))
	sid := edtest.P(e).Spreads[0].ID
	edtest.FreezeAt(t, e, 0)
	edtest.Change(e, func(p *model.Project) {
		for range model.MaxTexts {
			p.Spreads[0].Texts = append(p.Spreads[0].Texts, &model.TextBlock{ID: model.NewID(), Auto: true, Text: "т", Style: model.PlainStyle()})
		}
	})
	rev := edtest.Rev(e)
	// Граница точная: 31 копия + исходный = 32 × 64 = 2048 — сверх MaxTotalTexts; 30 + 1 = 1984 — в запасе.
	// Так ловится и проверка, забывшая исходный разворот.
	if _, err := e.DuplicateGrid(sid, 31, true); err == nil || edtest.Rev(e) != rev {
		t.Fatalf("размножение сверх запаса: err=%v", err)
	}
	if _, err := e.DuplicateGrid(sid, 30, true); err != nil {
		t.Fatal(err)
	}
}

func TestTotalSpreadsLimit(t *testing.T) {
	e := edtest.Open(t, model.New(), "")
	edtest.Change(e, func(p *model.Project) {
		for range model.MaxSpreads {
			p.Spreads = append(p.Spreads, &model.Spread{ID: model.NewID(), Photos: []string{}, BgL: "#ffffff"}) // не пустые: пустой хвост сборка заняла бы (035)
		}
	})
	rev := edtest.Rev(e)
	if _, err := e.NewSpread(-1, edtest.Photos("a")); err == nil {
		t.Fatal("разворот сверх MaxSpreads создан")
	}
	e.AutoBuild(edtest.Photos("a", "b"), -1, model.AutoOpts{Spreads: 1, MaxPer: 64})
	if len(edtest.P(e).Spreads) != model.MaxSpreads || edtest.Rev(e) != rev {
		t.Fatalf("AutoBuild сверх MaxSpreads: %d разворотов", len(edtest.P(e).Spreads))
	}
	// Проект с ровно MaxSpreads: деление дало бы лишний разворот — отказ.
	edtest.Change(e, func(p *model.Project) { p.Spreads = p.Spreads[:model.MaxSpreads-1] })
	v := edtest.NewSpread(e, -1, edtest.Photos("a", "b"))
	sid := v.Spreads[len(v.Spreads)-1].ID
	rev = edtest.Rev(e)
	e.SplitSpread(sid, 1)
	if len(edtest.P(e).Spreads) != model.MaxSpreads || edtest.Rev(e) != rev {
		t.Fatalf("SplitSpread сверх MaxSpreads: %d разворотов", len(edtest.P(e).Spreads))
	}
}

// Гигантские числа в блоке текста не дают Inf/NaN: View сериализуется (design 008 §2.3).
func TestLoadHugeTextBoxView(t *testing.T) {
	for _, body := range []string{
		`{"x":1.7e308,"y":1.7e308,"w":1.7e308,"rot":45,"auto":true,"text":"т"}`,
		`{"x":-1.7e308,"y":1.7e308,"w":1.7e308,"rot":30,"text":"т"}`,
		`{"x":1e300,"y":-1e300,"w":1e300,"rot":90,"text":"т"}`,
	} {
		path := filepath.Join(testkit.TempDir(t), "f.salbum")
		data := `{"version":3,"spreads":[{"id":"s","texts":[` + body + `]}]}`
		if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
		p, err := salbum.Load(path)
		if err != nil {
			t.Fatalf("%s: %v", body, err)
		}
		for _, tb := range p.Spreads[0].Texts {
			for _, v := range []float64{tb.X, tb.Y, tb.W} {
				if math.IsInf(v, 0) || math.IsNaN(v) {
					t.Fatalf("%s: блок %+v", body, tb)
				}
			}
		}
		if _, err := json.Marshal(edtest.Open(t, p, "").View()); err != nil {
			t.Fatalf("%s: View не сериализуется: %v", body, err)
		}
	}
}

// Переполненный проект не «замерзает»: удаление разворота и правки, не
// добавляющие разворотов, проходят; рост числа разворотов — отказ.
func TestCheckTotalsOverfullNotFrozen(t *testing.T) {
	e := edtest.Open(t, model.New(), "")
	edtest.P(e).Spreads = nil
	for range model.MaxSpreads + 3 {
		edtest.P(e).Spreads = append(edtest.P(e).Spreads, &model.Spread{ID: model.NewID(), Photos: []string{}})
	}
	if _, err := e.NewSpread(-1, edtest.Photos("a")); err == nil {
		t.Fatal("рост сверх предела должен отказать")
	}
	rev := edtest.Rev(e)
	first := edtest.P(e).Spreads[0].ID
	v := e.DeleteSpread(first)
	if len(v.Spreads) != model.MaxSpreads+2 || edtest.Rev(e) == rev {
		t.Fatalf("DeleteSpread в переполненном проекте не прошёл: %d разворотов", len(v.Spreads))
	}
	// Правка без изменения числа разворотов тоже проходит.
	s := model.DefaultSettings()
	s.PageW++
	rev = edtest.Rev(e)
	e.SetSettings(s)
	if edtest.Rev(e) == rev {
		t.Fatal("SetSettings в переполненном проекте не прошёл")
	}
}

// Безопасная зона (028 А) правится SetSettings одним шагом: авто-текст переносится
// по новому пределу (FitTexts в normalize), Undo возвращает 8 и прежнюю ширину.
func TestSetSettingsSafeUndo(t *testing.T) {
	e, s1, _ := edtest.NewTwo(t)
	id := edtest.AddText(t, e, s1, 0)
	e.SetText(id, strings.Repeat("слово ", 80), "")
	e.SetTextAuto(id)
	w0, lines0 := edtest.Block(t, e, id).W, len(textOf(e, id).Lines)
	if st := edtest.P(e).Settings; st.Safe != 8 || w0 != edtest.P(e).TextMaxW(nil) {
		t.Fatalf("safe %v, ширина %v при пределе %v", st.Safe, w0, edtest.P(e).TextMaxW(nil))
	}
	undo := edtest.UndoLen(e)
	st := e.View().Settings
	st.Safe = 30
	v := e.SetSettings(st)
	if edtest.UndoLen(e) != undo+1 || v.Settings.Safe != 30 || v.TextMaxW != st.PageW-60 {
		t.Fatalf("шагов %d (было %d), safe %v, TextMaxW %v", edtest.UndoLen(e), undo, v.Settings.Safe, v.TextMaxW)
	}
	if b := edtest.Block(t, e, id); b.W != v.TextMaxW || len(textOf(e, id).Lines) <= lines0 {
		t.Fatalf("авто-текст не перенесён: ширина %v, строк %d (было %d)", b.W, len(textOf(e, id).Lines), lines0)
	}
	v = e.Undo()
	if v.Settings.Safe != 8 || edtest.Block(t, e, id).W != w0 || len(textOf(e, id).Lines) != lines0 {
		t.Fatalf("Undo: safe %v, ширина %v (было %v)", v.Settings.Safe, edtest.Block(t, e, id).W, w0)
	}
}

// textOf — текст из View (переносы считает Go).
func textOf(e *album.Editor, id string) model.TextView {
	for _, s := range e.View().Spreads {
		for _, x := range s.Texts {
			if x.ID == id {
				return x
			}
		}
	}
	return model.TextView{}
}
