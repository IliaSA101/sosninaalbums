package decor

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// bigLook — зажатый макет больше model.MaxLookFile: шаги с предельными надписями (64 × 1000 рун).
func bigLook(t *testing.T) model.AlbumTpl {
	t.Helper()
	var texts []model.TplText
	for range model.MaxTexts {
		texts = append(texts, model.TplText{W: 0.5, Text: strings.Repeat("Ы", model.MaxTextRunes), Style: model.TextStyle{Size: 20, Color: "#222222", Align: "left"}})
	}
	tpl := model.SpreadTpl{V: model.TplV, Pages: 1, Ratio: 1, Page: []model.TplBg{{Color: "#ffffff"}}, Texts: texts}
	var l model.AlbumTpl
	l.V = model.LookV
	for range 12 {
		l.Steps = append(l.Steps, model.LookStep{Tpl: tpl})
	}
	c, ok := model.ClampLook(l)
	if !ok {
		t.Fatal("макет не зажался")
	}
	if data, err := json.Marshal(c); err != nil || len(data) <= model.MaxLookFile {
		t.Fatalf("макет не больше предела: %d байт, %v", len(data), err)
	}
	return c
}

// TestCheckLookMatchesSave: CheckLook отказывает так же, как SaveLook, и ничего не пишет; на годном
// отдаёт зажатый макет, пустом и чужой версии — ErrTplEmpty.
func TestCheckLookMatchesSave(t *testing.T) {
	s, _ := openTemp(t)
	big := bigLook(t)
	_, cerr := CheckLook(big)
	_, serr := s.SaveLook("mine", "Огромный", big)
	if cerr == nil || serr == nil || cerr.Error() != serr.Error() || !strings.Contains(cerr.Error(), "макет больше 512 КБ") {
		t.Fatalf("больше предела: CheckLook %v, SaveLook %v", cerr, serr)
	}
	for name, bad := range map[string]model.AlbumTpl{"пусто": {V: model.LookV}, "версия": {V: 2, Steps: big.Steps}} {
		if _, err := CheckLook(bad); !errors.Is(err, model.ErrTplEmpty) {
			t.Errorf("%s: %v", name, err)
		}
	}
	ok := lookOf("", "")
	c, err := CheckLook(ok)
	if err != nil || len(c.Steps) != len(ok.Steps) {
		t.Errorf("годный: %d шагов, %v", len(c.Steps), err)
	}
}
