package edit

import (
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// SetTextScale: новый кегль в ответе, неизвестный блок — вид без ошибки.
func TestSetTextScale(t *testing.T) {
	p := model.New()
	p.Spreads = []*model.Spread{{ID: "s1", Photos: []string{"a"}}}
	p.Photos["a"] = model.PhotoRef{Path: `C:\a.jpg`, W: 300, H: 200}
	a, _ := newMethods(t, p)
	v, err := a.AddText("s1", 0, "")
	if err != nil {
		t.Fatal(err)
	}
	id, size := v.Spreads[0].Texts[0].ID, v.Spreads[0].Texts[0].Style.Size
	x0, y0 := v.Spreads[0].Texts[0].X, v.Spreads[0].Texts[0].Y
	v, err = a.SetTextScale(id, size+6, x0-3, y0-2)
	if tv := v.Spreads[0].Texts[0]; err != nil || tv.Style.Size != size+6 || tv.X != x0-3 || tv.Y != y0-2 {
		t.Fatalf("кегль %v, x %v y %v, ошибка %v", tv.Style.Size, tv.X, tv.Y, err)
	}
	if v, err = a.SetTextScale("нет", 20, 0, 0); err != nil || len(v.Spreads) != 1 {
		t.Fatalf("неизвестный блок: %v", err)
	}
}

// Методы над кадром, текстами и подписями: каждый возвращает вид с применённой правкой
// (ревизия растёт), а Undo откатывает её.
func TestDesignSurface(t *testing.T) {
	a := gridMethods(t)
	rev := func() int { return a.editor().View().Rev }
	grew := func(name string, before int, v model.View) {
		t.Helper()
		if v.Rev <= before {
			t.Errorf("%s: правка не применена (rev %d → %d)", name, before, v.Rev)
		}
	}

	// Кадр и поворот фото b (разворот «Авто»).
	r := rev()
	v := a.SetFrame("b", 2, 0.5, -0.5)
	grew("SetFrame", r, v)
	if f := v.Spreads[1].Frames[0]; f.S != 2 || f.X != 0.5 || f.Y != -0.5 {
		t.Errorf("кадр: %+v", f)
	}
	r = rev()
	v = a.ResetFrame("b")
	grew("ResetFrame", r, v)
	if f := v.Spreads[1].Frames[0]; f.S != 1 || f.X != 0 || f.Y != 0 {
		t.Errorf("сброс кадра: %+v", f)
	}
	r = rev()
	v = a.SetFrameAngle("s2", "b", 10, "")
	grew("SetFrameAngle", r, v)
	if f := v.Spreads[1].Frames[0]; f.Tilt != 10 {
		t.Errorf("наклон: %+v", f)
	}
	r = rev()
	v = a.TurnPhotos([]string{"b"}, 1)
	grew("TurnPhotos", r, v)
	if f := v.Spreads[1].Frames[0]; f.Turn != 1 {
		t.Errorf("поворот: %+v", f)
	}
	r = rev()
	v = a.SetFlip([]string{"b"}, "h")
	grew("SetFlip", r, v)
	if !v.Spreads[1].Frames[0].FlipH {
		t.Error("зеркало не включено")
	}
	r = rev()
	v = a.SetBackground("s2", "left", "#112233")
	grew("SetBackground", r, v)
	if v.Spreads[1].BgSet[0] != "#112233" {
		t.Errorf("фон: %v", v.Spreads[1].BgSet)
	}

	// Название разворота.
	r = rev()
	v = a.SetSpreadName("s2", "Церемония")
	grew("SetSpreadName", r, v)
	if v.Spreads[1].Name != "Церемония" {
		t.Errorf("название: %q", v.Spreads[1].Name)
	}

	// Текстовый блок: добавить, править, двигать, повернуть, стиль, удалить.
	r = rev()
	v, err := a.AddText("s2", 0, "")
	if err != nil {
		t.Fatal(err)
	}
	grew("AddText", r, v)
	tid := v.Spreads[1].Texts[len(v.Spreads[1].Texts)-1].ID
	text := func(v model.View) model.TextView { return textOf(t, v.Spreads[1], tid) }
	r = rev()
	v = a.SetText(tid, "Привет", "")
	grew("SetText", r, v)
	if text(v).Text != "Привет" {
		t.Errorf("текст: %q", text(v).Text)
	}
	r = rev()
	v = a.SetTextBox(tid, 20, 30, 80)
	grew("SetTextBox", r, v)
	if tv := text(v); tv.Auto || tv.W != 80 {
		t.Errorf("рамка: %+v", tv)
	}
	r = rev()
	v = a.SetTextAngle(tid, 15)
	grew("SetTextAngle", r, v)
	if text(v).Rot != 15 {
		t.Errorf("угол: %v", text(v).Rot)
	}
	r = rev()
	v = a.SetTextAuto(tid)
	grew("SetTextAuto", r, v)
	if !text(v).Auto {
		t.Error("ширина по тексту не включена")
	}
	st := text(v).Style
	st.Size += 2
	r = rev()
	v = a.SetTextStyle(tid, st)
	grew("SetTextStyle", r, v)
	if text(v).Style.Size != st.Size {
		t.Errorf("кегль: %v", text(v).Style.Size)
	}
	r = rev()
	v, err = a.SetTextScale(tid, st.Size+4, 25, 35)
	if err != nil {
		t.Fatal(err)
	}
	grew("SetTextScale", r, v)
	r = rev()
	v = a.DeleteText(tid)
	grew("DeleteText", r, v)
	if len(v.Spreads[1].Texts) != 0 {
		t.Errorf("тексты после удаления: %d", len(v.Spreads[1].Texts))
	}

	// Подпись к фото.
	r = rev()
	v = a.SetCaption("b", "Под снимком", "")
	grew("SetCaption", r, v)
	if len(v.Spreads[1].Captions) == 0 || v.Spreads[1].Captions[0].Text != "Под снимком" {
		t.Fatalf("подпись: %+v", v.Spreads[1].Captions)
	}
	r = rev()
	v = a.SetCaptionStyle("b", v.Spreads[1].Captions[0].Style, true, 1)
	grew("SetCaptionStyle", r, v)
	if !v.Spreads[1].Captions[0].Over {
		t.Error("подпись не на фото")
	}

	// Каждая правка — шаг отмены: после всех Undo имя разворота и кадр вернулись.
	for a.editor().View().CanUndo {
		a.undo()
	}
	v = a.editor().View()
	if v.Spreads[1].Name != "" || len(v.Spreads[1].Captions) != 0 || v.Spreads[1].Frames[0].S != 1 {
		t.Errorf("после отмены всего: %+v", v.Spreads[1])
	}
}

// textOf — текстовый блок разворота по ID; пропал — провал теста.
func textOf(t *testing.T, sv model.SpreadView, id string) model.TextView {
	t.Helper()
	for _, tv := range sv.Texts {
		if tv.ID == id {
			return tv
		}
	}
	t.Fatal("текст пропал")
	return model.TextView{}
}
