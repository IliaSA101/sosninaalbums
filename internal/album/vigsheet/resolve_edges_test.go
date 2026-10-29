package vigsheet

import (
	"errors"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// ResolveOwn: чужие ссылки и пустой список — «не найден» без следа; неизвестный разворот — своя ошибка;
// не «my:…» — nil без ошибки (каталог).
func TestResolveOwnEdges(t *testing.T) {
	p := model.New()
	p.Spreads = []*model.Spread{{ID: "s1", Photos: []string{}}}
	before := string(model.Marshal(p))
	if o, err := ResolveOwn(p, "s1", "catalog:x", nil); o != nil || err != nil {
		t.Errorf("каталожный: %v %v", o, err)
	}
	if _, err := ResolveOwn(p, "нет", model.VigOwnPrefix+"a", nil); !errors.Is(err, errVigSpread) {
		t.Errorf("неизвестный разворот: %v", err)
	}
	other := []model.Template{
		{ID: "z", Kind: model.TplVignette, Vig: &model.VigTpl{}}, // другой id
		{ID: "a", Kind: "", Vig: &model.VigTpl{}},                // не виньеточный
		{ID: "a", Kind: model.TplVignette},                       // без Vig
	}
	if _, err := ResolveOwn(p, "s1", model.VigOwnPrefix+"a", other); !errors.Is(err, model.ErrTplNotFound) {
		t.Errorf("шаблона в списке нет: %v", err)
	}
	if string(model.Marshal(p)) != before {
		t.Fatal("отказ оставил след")
	}
}
