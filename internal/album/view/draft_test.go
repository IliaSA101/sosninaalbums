package view_test

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Заготовки и макет в виде (131 design §6, §9): TextView.Draft, View.Drafts по разворотам и
// обложке, SpreadView.LookStep, View.Look; поля внутри заготовки — не в Unfilled.
func TestViewDrafts(t *testing.T) {
	e := edtest.CoverEd(t, "cv", []*model.Place{edtest.Gp(10, 10, 100, 80, "p1")}, []string{"a"}, []string{"b"})
	// Без заготовок и макета: Drafts 0, look в JSON нет, шагов нет.
	v := e.View()
	if v.Drafts != 0 || v.Look != nil || v.Spreads[0].LookStep != 0 {
		t.Fatalf("пустой альбом: заготовок %d, макет %+v", v.Drafts, v.Look)
	}
	if b, _ := json.Marshal(v); strings.Contains(string(b), `"look"`) || strings.Contains(string(b), `"lookStep"`) ||
		strings.Contains(string(b), `"draft"`) || !strings.Contains(string(b), `"drafts":0`) {
		t.Fatal("в JSON вида без макета лишние поля")
	}
	edtest.Change(e, func(p *model.Project) {
		p.Look = &model.LookRef{Key: "my:00112233445566ff", Name: "Классика", Pack: "wedding-layouts", Item: "layouts/classic.json"}
		s0, s1 := p.Spreads[0], p.Spreads[1]
		s0.LookStep, s1.LookStep, s1.BgL = 2, 3, "#ffeeee"
		s0.Texts = append(s0.Texts,
			&model.TextBlock{ID: "0a0b0c0d0e01", X: 20, Y: 20, W: 60, Auto: true, Text: "[Имена]", Style: model.PlainStyle(), Draft: true},
			&model.TextBlock{ID: "0a0b0c0d0e02", X: 20, Y: 40, W: 60, Auto: true, Text: "{Школа}", Style: model.PlainStyle(), Draft: true})
		s1.Texts = append(s1.Texts, &model.TextBlock{ID: "0a0b0c0d0e03", X: 20, Y: 20, W: 60, Auto: true, Text: "{Год}", Style: model.PlainStyle()})
		p.Cover.Sheet.Texts[0].Draft = true
	})
	v = e.View()
	if v.Drafts != 3 {
		t.Fatalf("заготовок %d, ждали 3 (две на развороте, одна на обложке)", v.Drafts)
	}
	if lv := v.Look; lv == nil || lv.Key != "my:00112233445566ff" || lv.Name != "Классика" || lv.Found {
		t.Fatalf("макет в виде: %+v", lv)
	}
	if v.Spreads[0].LookStep != 2 || v.Spreads[1].LookStep != 3 || v.Spreads[2].LookStep != 0 || v.Cover.Sheet.LookStep != 0 {
		t.Fatalf("шаги: %d %d %d, обложка %d", v.Spreads[0].LookStep, v.Spreads[1].LookStep, v.Spreads[2].LookStep, v.Cover.Sheet.LookStep)
	}
	flags := func(sv model.SpreadView) []bool {
		var out []bool
		for _, tv := range sv.Texts {
			out = append(out, tv.Draft)
		}
		return out
	}
	if !slices.Equal(flags(v.Spreads[0]), []bool{true, true}) || !slices.Equal(flags(v.Spreads[1]), []bool{false}) ||
		!slices.Equal(flags(v.Cover.Sheet), []bool{true}) {
		t.Fatalf("признаки: %v %v, обложка %v", flags(v.Spreads[0]), flags(v.Spreads[1]), flags(v.Cover.Sheet))
	}
	// {Школа} — только в заготовке: ни в вид, ни в лист; {Год} — в обычной надписи.
	if !slices.Equal(v.Unfilled, []string{"Год"}) || len(v.Spreads[0].Unfilled) != 0 || !slices.Equal(v.Spreads[1].Unfilled, []string{"Год"}) {
		t.Fatalf("пустые поля: вид %v, листы %v %v", v.Unfilled, v.Spreads[0].Unfilled, v.Spreads[1].Unfilled)
	}
	// Подпись заготовкой не бывает; холст подставляет поле и в заготовке.
	if tv := v.Spreads[0].Texts[1]; tv.Src != "{Школа}" || tv.Text == "{Школа}" {
		t.Fatalf("поле в заготовке: %q / %q", tv.Text, tv.Src)
	}
	// Вид печати несёт те же числа: по нему спрашивает экспорт.
	if _, pv := e.Snapshot(); pv.Drafts != 3 || pv.Look == nil || pv.Spreads[1].LookStep != 3 {
		t.Fatalf("вид печати: заготовок %d, макет %+v", pv.Drafts, pv.Look)
	}
	if b, _ := json.Marshal(v); !strings.Contains(string(b), `"look":{"key":"my:00112233445566ff","name":"Классика","found":false}`) ||
		!strings.Contains(string(b), `"lookStep":2`) || !strings.Contains(string(b), `"draft":true`) {
		t.Fatalf("JSON вида: нет полей макета")
	}
}
