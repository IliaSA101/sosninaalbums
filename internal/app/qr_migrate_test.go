package app

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/decor"
)

// TestSaveSpreadTplSkipsRenamedOldQR: код ссылки 0.23–0.27, переименованный фотографом (в реестре ни
// признака, ни «QR · » в имени), после открытия библиотеки узнаётся по содержимому и в шаблон
// разворота не уходит.
func TestSaveSpreadTplSkipsRenamedOldQR(t *testing.T) {
	a := lookApp(t)
	ids := saveSeries(t, a)
	if _, err := a.AddQRSticker(ids[2], 0, "https://secret.example/client-album"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := a.editor().AddSticker(ids[2], 0, model.StickerSrc{Art: &model.Art{Kind: "star"}}); err != nil {
		t.Fatal(err)
	}
	qrKey := ownStickers(t, a)[0].ID
	// Реестр, каким его оставила бы 0.27 после ✎: имя другое, признака нет.
	path := filepath.Join(a.cfgDir, decor.RootName, "library.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var reg map[string]any
	if err := json.Unmarshal(data, &reg); err != nil {
		t.Fatal(err)
	}
	delete(reg, "qrScan")
	for _, it := range reg["items"].([]any) {
		e := it.(map[string]any)
		delete(e, "qr")
		e["name"] = "Галерея Ивановых"
	}
	data, _ = json.Marshal(reg)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	a.openDecor()
	s, _ := a.decorStore()
	if !s.IsQR(qrKey) {
		t.Fatal("старый код не узнан при открытии библиотеки")
	}
	it, err := a.SaveSpreadTpl(ids[2], model.ScopeSpread, "mine", "С кодом", true, false)
	if err != nil {
		t.Fatal(err)
	}
	tpl, err := s.SpreadTpl(it.ID)
	if err != nil || len(tpl.Stickers) != 1 || tpl.Stickers[0].Art == nil {
		t.Fatalf("шаблон: %+v %v", tpl, err)
	}
	out, _ := json.Marshal(tpl)
	if bytes.Contains(out, []byte(strings.TrimPrefix(qrKey, "my:"))) {
		t.Error("ключ старого кода в шаблоне")
	}
}
