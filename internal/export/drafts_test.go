package export

import (
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/text"
)

// Заготовки (131 G2, 0.27-131-5): выгрузка их не рисует, плитка и пипетка рисуют.

// draftAlbum — три разворота; на первом заготовка и обычная надпись, на втором только заготовка,
// на третьем пустое место; обложка с заготовкой и обычной надписью. Редактор — для правок теста.
func draftAlbum(t *testing.T) (*album.Editor, *model.Project, model.View) {
	t.Helper()
	st := model.DefaultCaptionStyle()
	p := model.New()
	p.Settings.PageW, p.Settings.PageH = 100, 100
	tb := func(id, txt string, draft bool) *model.TextBlock {
		return &model.TextBlock{ID: id, X: 10, Y: 20, W: 60, Text: txt, Style: st, Draft: draft}
	}
	p.Spreads = []*model.Spread{
		{ID: "s1", Texts: []*model.TextBlock{tb("t1", "[Имена]", true), tb("t2", "Привет", false)}},
		{ID: "s2", Texts: []*model.TextBlock{tb("t3", "[Дата]", true)}},
		{ID: "s3", Grid: []*model.Place{{ID: "m1", X: 5, Y: 5, W: 50, H: 50}}},
	}
	e := mustEditor(t, p, "")
	if _, err := e.CreateCover("", nil); err != nil {
		t.Fatal(err)
	}
	e.SetCoverSpineBg("#102030")
	for _, c := range []struct {
		txt   string
		draft bool
	}{{"[Обложка]", true}, {"Свадьба", false}} {
		v, err := e.AddText(model.CoverID, 0, "")
		if err != nil {
			t.Fatal(err)
		}
		id := v.Cover.Sheet.Texts[len(v.Cover.Sheet.Texts)-1].ID
		e.SetText(id, c.txt, "")
		if c.draft {
			e.SetTextDraft(model.CoverID, id, true)
		}
	}
	pp, v := e.Snapshot()
	return e, pp, v
}

func texts(ts []model.TextView) (out []string) {
	for _, t := range ts {
		out = append(out, t.Text)
	}
	return out
}

func sceneTexts(s []string) string { return strings.Join(s, "|") }

// TestExportSkipsDraft — FromView и NewCoverJob не берут заготовки; вид не меняется;
// SheetScene, CoverScene (пипетка) и плитка их рисуют; Blank*/PlanFonts заготовки не считают.
func TestExportSkipsDraft(t *testing.T) {
	_, p, v := draftAlbum(t)
	if v.Drafts != 3 || len(v.Spreads[0].Texts) != 2 {
		t.Fatalf("вид: заготовок %d, текстов на первом листе %d", v.Drafts, len(v.Spreads[0].Texts))
	}
	out := FromView(p, v)
	if len(out) != 3 || len(out[0].Texts) != 1 || len(out[1].Texts) != 0 || len(out[2].Texts) != 0 {
		t.Fatalf("печатаемых текстов %d/%d/%d, ждали 1/0/0", len(out[0].Texts), len(out[1].Texts), len(out[2].Texts))
	}
	if out[0].Texts[0].Text != "Привет" {
		t.Fatalf("осталась не та надпись: %q", out[0].Texts[0].Text)
	}
	if len(v.Spreads[0].Texts) != 2 || !v.Spreads[0].Texts[0].Draft {
		t.Fatal("printable изменил вид")
	}
	job := NewCoverJob(p, v, 20, 90)
	if got := sceneTexts(texts(v.Cover.Sheet.Texts)); got != "[Обложка]|Свадьба" {
		t.Fatalf("тексты обложки в виде: %q", got)
	}
	if len(job.Spread.Texts) != 1 || job.Spread.Texts[0].Text != "Свадьба" || len(job.Spread.Plates) != 1 {
		t.Fatalf("обложка к печати: текстов %d, плашек %d", len(job.Spread.Texts), len(job.Spread.Plates))
	}
	// Пипетка и плитка: заготовки на месте, плашка корешка — тоже.
	if sc := SheetScene(p, v.Spreads[0]); len(sc.Texts) != 2 {
		t.Fatalf("SheetScene: текстов %d, ждали 2", len(sc.Texts))
	}
	cs, ok := CoverScene(p, v)
	if !ok || len(cs.Texts) != 2 || len(cs.Plates) != 1 {
		t.Fatalf("CoverScene: %v, текстов %d, плашек %d", ok, len(cs.Texts), len(cs.Plates))
	}
	if _, ok := CoverScene(p, model.View{}); ok {
		t.Fatal("обложки нет, а сцена есть")
	}
	// Лист только с заготовкой — пустой для печати; обложка с обычной надписью — нет.
	pl := NewPlan(nil, 3, false)
	if got := BlankSpreads(v, &pl); !slices.Equal(got, []int{2}) {
		t.Fatalf("пустые для печати: %v, ждали [2]", got)
	}
	if BlankCover(v) {
		t.Fatal("обложка с обычной надписью пуста")
	}
	w := v
	cv := *v.Cover
	cv.Sheet.Texts = slices.DeleteFunc(slices.Clone(cv.Sheet.Texts), func(t model.TextView) bool { return !t.Draft })
	cv.SpineBg = ""
	w.Cover = &cv
	if !BlankCover(w) {
		t.Fatal("обложка только с заготовкой должна быть пустой для печати")
	}
}

// Шрифт заготовки не нужен печати: вопрос о нехватающем шрифте её не касается (на листе и на обложке).
func TestPlanFontsSkipsDraft(t *testing.T) {
	lato := text.FamilyID("Lato")
	tv := func(s string, draft bool) model.TextView {
		return model.TextView{Text: s, Draft: draft, Style: model.TextStyle{Font: lato}}
	}
	names := []model.FontUse{{ID: lato, Name: "Lato"}}
	pl := NewPlan(nil, 1, false).WithCover(&CoverJob{})
	v := model.View{AlbumFonts: names,
		Spreads: []model.SpreadView{{Texts: []model.TextView{tv("[Имена]", true)}}},
		Cover:   &model.CoverView{Sheet: model.SpreadView{Texts: []model.TextView{tv("[Обложка]", true)}}}}
	if got := PlanFonts(v, &pl); len(got) != 0 {
		t.Fatalf("шрифт заготовки: %v", got)
	}
	v.Spreads[0].Texts = append(v.Spreads[0].Texts, tv("Анна", false))
	if got := PlanFonts(v, &pl); !slices.Equal(got, []string{"Lato"}) {
		t.Fatalf("обычная надпись тем же шрифтом: %v", got)
	}
}

// TestTplTileDrawsDraft — плитка шаблона (TplScene) рисует заготовки; экспорт той же вида — нет.
func TestTplTileDrawsDraft(t *testing.T) {
	_, p, v := draftAlbum(t)
	tile := model.View{Spreads: v.Spreads[:1], Rev: v.Rev}
	sc, ok := TplScene(p, tile)
	if !ok || len(sc.Texts) != 2 {
		t.Fatalf("плитка: %v, текстов %d, ждали 2 (с заготовкой)", ok, len(sc.Texts))
	}
	if es := FromView(p, tile); len(es[0].Texts) != 1 {
		t.Fatalf("экспорт того же вида: текстов %d, ждали 1", len(es[0].Texts))
	}
}

// DraftSummaryFor: пустые места и заготовки; цель-страница и обложка вне плана; пустые входы.
func TestDraftSummary(t *testing.T) {
	_, _, v := draftAlbum(t)
	all := NewPlan(nil, 3, false)
	d := DraftSummaryFor(v, &all, 100)
	if !slices.Equal(d.Spreads, []int{1, 2, 3}) || d.Empty != 1 || d.Drafts != 2 || d.Cover {
		t.Fatalf("весь альбом без обложки: %+v", d)
	}
	withCover := all.WithCover(&CoverJob{})
	d = DraftSummaryFor(v, &withCover, 100)
	if d.Drafts != 3 || !d.Cover {
		t.Fatalf("с обложкой: %+v", d)
	}
	if d = DraftSummaryFor(v, nil, 100); d.Drafts != 3 || !d.Cover || len(d.Spreads) != 3 {
		t.Fatalf("nil-план: %+v", d)
	}
	// Страница: заготовка на левой странице (x 10…70 заходит на правую, 100 — граница) — на обе.
	pages := NewPlan([]Unit{{1, 1}}, 3, true) // левая страница разворота 1
	if d = DraftSummaryFor(v, &pages, 100); d.Drafts != 1 || !slices.Equal(d.Spreads, []int{1}) {
		t.Fatalf("левая страница: %+v", d)
	}
	right := NewPlan([]Unit{{1, 2}}, 3, true)
	if d = DraftSummaryFor(v, &right, 100); d.Drafts != 0 || len(d.Spreads) != 0 {
		t.Fatalf("правая страница: %+v", d)
	}
	if d = DraftSummaryFor(model.View{}, nil, 100); d.Drafts != 0 || d.Empty != 0 || len(d.Spreads) != 0 || EmptyDraftWarning(d) != "" {
		t.Fatalf("пустой вид: %+v", d)
	}
}

// TestEmptyDraftWarning — текст вопроса: без заготовок прежний EmptyWarning, с ними — новый.
func TestEmptyDraftWarning(t *testing.T) {
	if got, want := EmptyDraftWarning(DraftSummary{Spreads: []int{1, 3}, Empty: 3}), EmptyWarning([]int{1, 3}, 3); got != want || got == "" {
		t.Fatalf("без заготовок: %q, ждали %q", got, want)
	}
	for _, c := range []struct {
		d    DraftSummary
		want string
	}{
		{DraftSummary{Spreads: []int{1, 3}, Empty: 3, Drafts: 2},
			"На разворотах 1, 3 пустые места: 3, заготовки: 2. Заготовки не печатаются, на месте пустых мест будет фон. Выгрузить?"},
		{DraftSummary{Spreads: []int{4}, Drafts: 1}, "На развороте 4 заготовки: 1. Заготовки не печатаются. Выгрузить?"},
		{DraftSummary{Drafts: 1, Cover: true}, "На обложке заготовки: 1. Заготовки не печатаются. Выгрузить?"},
		{DraftSummary{Spreads: []int{2}, Empty: 1, Drafts: 2, Cover: true},
			"На обложке и развороте 2 пустые места: 1, заготовки: 2. Заготовки не печатаются, на месте пустых мест будет фон. Выгрузить?"},
		// Лист и обложка только с заготовками (TakeBlank, 0.28-131-19): что выйдет пустым.
		{DraftSummary{Spreads: []int{2}, Drafts: 1, Blank: []int{2}},
			"На развороте 2 заготовки: 1. Заготовки не печатаются. Разворот 2 выйдет пустым. Выгрузить?"},
		{DraftSummary{Spreads: []int{2, 5}, Drafts: 3, Blank: []int{2, 5}},
			"На разворотах 2, 5 заготовки: 3. Заготовки не печатаются. Развороты 2, 5 выйдут пустыми. Выгрузить?"},
		{DraftSummary{Drafts: 1, Cover: true, BlankCover: true}, "На обложке заготовки: 1. Заготовки не печатаются. Обложка выйдет пустой. Выгрузить?"},
		{DraftSummary{Spreads: []int{1, 2}, Empty: 1, Drafts: 2, Cover: true, Blank: []int{2}, BlankCover: true},
			"На обложке и разворотах 1, 2 пустые места: 1, заготовки: 2. Заготовки не печатаются, на месте пустых мест будет фон. Обложка и разворот 2 выйдут пустыми. Выгрузить?"},
		{DraftSummary{Spreads: []int{3, 4}, Drafts: 2, Cover: true, Blank: []int{3, 4}, BlankCover: true},
			"На обложке и разворотах 3, 4 заготовки: 2. Заготовки не печатаются. Обложка и развороты 3, 4 выйдут пустыми. Выгрузить?"},
	} {
		if got := EmptyDraftWarning(c.d); got != c.want {
			t.Errorf("%+v:\n got %q\nwant %q", c.d, got, c.want)
		}
	}
	many := make([]int, 12)
	for i := range many {
		many[i] = i + 1
	}
	if got := EmptyDraftWarning(DraftSummary{Spreads: many, Drafts: 12}); !strings.Contains(got, "1, 2, 3, 4, 5, 6, 7, 8, 9, 10 …и ещё 2 заготовки: 12") {
		t.Errorf("длинный список: %q", got)
	}
}

// TakeBlank: лист и обложка только с заготовкой плана уходят из вопроса «пустые» в вопрос о
// заготовках (0.28-131-19); пустой лист без заготовок и лист с заготовкой вне цели-страницы
// остаются; пустые входы; повторный вызов на исходном списке его не меняет.
func TestTakeBlank(t *testing.T) {
	_, _, v := draftAlbum(t)
	cv := *v.Cover
	cv.Sheet.Texts = slices.DeleteFunc(slices.Clone(cv.Sheet.Texts), func(t model.TextView) bool { return !t.Draft })
	cv.SpineBg = ""
	v.Cover = &cv
	v.Spreads = append(v.Spreads, model.SpreadView{}, model.SpreadView{Texts: v.Spreads[1].Texts}) // 4 — пуст, 5 — только заготовка
	all := NewPlan(nil, 5, false).WithCover(&CoverJob{})
	blank := BlankSpreads(v, &all)
	if !slices.Equal(blank, []int{2, 4, 5}) || !BlankCover(v) {
		t.Fatalf("пустые для печати: %v, обложка %v", blank, BlankCover(v))
	}
	d := DraftSummaryFor(v, &all, 100)
	rest, cover := d.TakeBlank(blank, true)
	if !slices.Equal(rest, []int{4}) || cover || !slices.Equal(d.Blank, []int{2, 5}) || !d.BlankCover || !slices.Equal(blank, []int{2, 4, 5}) {
		t.Fatalf("остались %v, обложка %v; забрал %v, обложка %v; вход %v", rest, cover, d.Blank, d.BlankCover, blank)
	}
	want := "На обложке и разворотах 1, 2, 3, 5 пустые места: 1, заготовки: 4. Заготовки не печатаются, на месте пустых мест будет фон. Обложка и развороты 2, 5 выйдут пустыми. Выгрузить?"
	if got := EmptyDraftWarning(d); got != want {
		t.Fatalf("вопрос:\n got %q\nwant %q", got, want)
	}
	if got := BlankWarningCover(rest, cover, true); got != "Разворот 4 пустой — выгрузить белым?" {
		t.Fatalf("вопрос о пустых: %q", got)
	}
	// Обложка вне плана: её заготовки не считаются, пустой она не спрашивается.
	noCover := NewPlan(nil, 5, false)
	d = DraftSummaryFor(v, &noCover, 100)
	if rest, cover := d.TakeBlank(BlankSpreads(v, &noCover), false); !slices.Equal(rest, []int{4}) || cover || d.BlankCover {
		t.Fatalf("без обложки: %v %v %+v", rest, cover, d)
	}
	// Правая страница листа 2: заготовка (x 10…70) на неё не заходит — вопрос «пустой» остаётся.
	right := NewPlan([]Unit{{2, 2}}, 5, true)
	d = DraftSummaryFor(v, &right, 100)
	if rest, _ := d.TakeBlank(BlankSpreads(v, &right), false); !slices.Equal(rest, []int{2}) || len(d.Blank) != 0 {
		t.Fatalf("правая страница: %v, забрал %v", rest, d.Blank)
	}
	// Пустые входы: ничего не забирает и не придумывает.
	var z DraftSummary
	if rest, cover := z.TakeBlank(nil, false); len(rest) != 0 || cover || len(z.Blank) != 0 || z.BlankCover || EmptyDraftWarning(z) != "" {
		t.Fatalf("пусто: %v %v %+v", rest, cover, z)
	}
	if rest, cover := z.TakeBlank([]int{1, 2}, true); !slices.Equal(rest, []int{1, 2}) || !cover {
		t.Fatalf("без заготовок: %v %v", rest, cover)
	}
}
