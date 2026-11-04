package app

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/decor"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Поля плитки 133 (packId, peek) доходят до интерфейса в JSON биндинга: встроенное — sa:builtin
// без peek, набор — его id и /media/lib/p/…, своё — без packId, с peek; у шаблона разворота и макета альбома (JSON, не картинка) peek нет.
func TestDecorCatalogFields(t *testing.T) {
	a := decorApp(t)
	if _, err := a.AddPackPath(d1Zip(t)); err != nil {
		t.Fatal(err)
	}
	importBg(t, a, testkit.JPEG(t, testkit.TempDir(t), "Мой.jpg", 60, 40))
	// Свой макет альбома: плитка с packId пустым, own, но без peek.
	store, err := a.decorStore()
	if err != nil {
		t.Fatal(err)
	}
	tpl := model.SpreadTpl{V: model.TplV, Pages: 2, Ratio: 2, Places: []model.Box{{X: 0.1, Y: 0.1, W: 0.3, H: 0.8}}, Page: []model.TplBg{{Color: "#ffffff"}, {Color: "#ffffff"}}}
	if _, err := store.SaveLook("mine", "Свой макет", model.AlbumTpl{V: model.LookV, Steps: []model.LookStep{{Tpl: tpl}}}); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, kind := range []string{decor.KindBg, decor.KindSticker, decor.KindSpread, decor.KindFrame, decor.KindAlbum} {
		cat, err := a.DecorCatalog(kind)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := json.Marshal(cat)
		if err != nil {
			t.Fatal(err)
		}
		var back struct {
			Items []map[string]any `json:"items"`
		}
		if err := json.Unmarshal(raw, &back); err != nil || len(back.Items) != len(cat.Items) {
			t.Fatalf("%s: %v %d", kind, err, len(back.Items))
		}
		for _, m := range back.Items {
			checkFieldItem(t, kind, m, seen)
		}
	}
	if !seen["own"+decor.KindAlbum] {
		t.Errorf("свой макет не встретился: %v", seen)
	}
	if !seen["builtin"] || !seen["own"+decor.KindBg] || !seen["pack"+decor.KindSpread] {
		t.Errorf("не встретились все виды плиток: %v", seen)
	}
}

// checkFieldItem — одна плитка в JSON по владельцу; seen копит встреченные виды владельцев.
func checkFieldItem(t *testing.T, kind string, m map[string]any, seen map[string]bool) {
	t.Helper()
	packID, _ := m["packId"].(string)
	peek, hasPeek := m["peek"].(string)
	goodPeek := hasPeek && strings.HasPrefix(peek, decor.LibPath+"p/")
	noPeek := kind == decor.KindSpread || kind == decor.KindAlbum // JSON: /media/lib/p дал бы 404
	switch {
	case packID == decor.BuiltinPack:
		seen["builtin"] = true
		if hasPeek {
			t.Errorf("%s: у встроенного peek %v", kind, m)
		}
	case packID == "ivory":
		seen["pack"+kind] = true
		if goodPeek == noPeek || (hasPeek && !goodPeek) {
			t.Errorf("%s: peek набора %v", kind, m)
		}
	case packID == "" && m["own"] == true:
		seen["own"+kind] = true
		if goodPeek == noPeek || (hasPeek && !goodPeek) {
			t.Errorf("%s: peek своего %v", kind, m)
		}
	default:
		t.Errorf("%s: плитка без владельца %v", kind, m)
	}
}

// Поля ступени 2 (133) в JSON биндинга: form у встроенного уголка, new — всегда массив (не null).
func TestDecorCatalogFieldsStep2(t *testing.T) {
	a := decorApp(t)
	cat, err := a.DecorCatalog(decor.KindSticker)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(cat)
	if err != nil {
		t.Fatal(err)
	}
	var back struct {
		New   *[]string `json:"new"`
		Items []struct {
			Name  string `json:"name"`
			Form  string `json:"form"`
			Style string `json:"style"`
		} `json:"items"`
	}
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	if back.New == nil || len(*back.New) != 0 || !strings.Contains(string(raw), `"new":[]`) {
		t.Errorf("new должен быть [] в JSON: %v", back.New)
	}
	found := false
	for _, it := range back.Items {
		if it.Name == "Уголок для фото" {
			found = it.Form == decor.FormCorner && it.Style == ""
		}
		if it.Form != "" && it.Form != decor.FormCorner && it.Form != decor.FormDivider {
			t.Errorf("недопустимый form %q у %q", it.Form, it.Name)
		}
	}
	if !found {
		t.Error("у встроенного «Уголок для фото» нет form=corner")
	}
	// Ошибка вида: пустой, но не nil, new и в ответе об ошибке.
	if c, err := a.DecorCatalog("нет"); err == nil || c.New == nil {
		t.Errorf("ответ об ошибке: new=%v err=%v", c.New, err)
	}
}
