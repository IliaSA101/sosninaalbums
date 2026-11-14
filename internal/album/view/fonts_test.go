package view_test

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
	"github.com/IliaSA101/sosninaalbums/internal/text"
)

// TestViewAlbumFonts — View.AlbumFonts (082-24): свои гарнитуры альбома из текстов и подписей фото
// с названием (зарегистрированной — из реестра, нет на компьютере — из файла, неизвестное — пусто),
// по названию без регистра; круг Save → Load; правка убирает ненужную; без своих — поля нет.
func TestViewAlbumFonts(t *testing.T) {
	lato := edtest.UseFont(t, "Lato", "Oswald-Regular.ttf")
	bodoni, anon := text.FamilyID("Bodoni"), text.FamilyID("Без имени")
	style := func(id string) model.TextStyle {
		return model.TextStyle{Font: id, Size: 20, Color: "#222222", Align: "left"}
	}
	block := func(id, font string) *model.TextBlock {
		return &model.TextBlock{ID: id, X: 10, Y: 10, W: 50, Text: "Анна", Style: style(font)}
	}
	p := model.New()
	p.Photos["ph"] = model.PhotoRef{Path: filepath.Join(testkit.TempDir(t), "a.jpg"), W: 60, H: 40,
		Caption: &model.Caption{Text: "подпись", Style: style(anon)}}
	p.Spreads = []*model.Spread{
		{ID: "s1", Photos: []string{"ph"}, Texts: []*model.TextBlock{block("0a0b0c0d0e01", lato), block("0a0b0c0d0e02", bodoni)}},
		{ID: "s2", Texts: []*model.TextBlock{block("0a0b0c0d0e03", lato)}},
	}
	p.Fonts = map[string]model.FontRef{bodoni: {Name: "bodoni"}}
	e := edtest.Open(t, p, "")
	e.SetText("0a0b0c0d0e03", "Павел", "") // правка: SyncFonts в конвейере заполняет таблицу
	want := []model.FontUse{{ID: anon}, {ID: bodoni, Name: "bodoni"}, {ID: lato, Name: "Lato"}}
	if got := e.View().AlbumFonts; !reflect.DeepEqual(got, want) {
		t.Fatalf("AlbumFonts %+v, ждали %+v", got, want)
	}

	if got := edtest.Open(t, edtest.SaveLoad(t, e), "").View().AlbumFonts; !reflect.DeepEqual(got, want) {
		t.Fatalf("после Load: %+v", got)
	}

	e.DeleteText("0a0b0c0d0e02")
	if got := e.View().AlbumFonts; len(got) != 2 || got[0].ID != anon || got[1].ID != lato {
		t.Fatalf("после удаления Bodoni: %+v", got)
	}
	e.Undo()
	if got := e.View().AlbumFonts; !reflect.DeepEqual(got, want) {
		t.Fatalf("Undo: %+v", got)
	}

	plain := edtest.Open(t, model.New(), "")
	if v := plain.View(); v.AlbumFonts != nil {
		t.Fatalf("без своих: %+v", v.AlbumFonts)
	}
	raw, err := json.Marshal(plain.View())
	if err != nil || strings.Contains(string(raw), "albumFonts") {
		t.Fatalf("в JSON вида без своих шрифтов есть albumFonts: %v", err)
	}
}
