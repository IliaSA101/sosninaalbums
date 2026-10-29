package app

import (
	"bytes"
	"encoding/json"
	"math"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Копировать и вставить на уровне App (099 G4): буфер, ответы, отмена. Тела правок проверяют
// internal/album и grid; здесь — привязка буфера к альбому, пути, холостые вызовы и контракт ответов.

// clipApp — App с ручным листом s1 (фото «a» и «b» в местах, пустое место, надпись, стикер),
// «Авто» s2 из «c» и пустым ручным s3; emits считает рассылки вида.
func clipApp(t *testing.T) (a *App, emits *atomic.Int32, text, sticker string) {
	t.Helper()
	a = switchApp(t)
	emits = new(atomic.Int32)
	a.emit = func(name string, _ ...any) {
		if name == "view" {
			emits.Add(1)
		}
	}
	text, sticker = "0000000000a1", "0000000000b1"
	p := model.New()
	p.Spreads = []*model.Spread{
		{ID: "s1", Photos: []string{"a", "b"},
			Grid: []*model.Place{
				{ID: "00000000000a", X: 12, Y: 12, W: 100, H: 100, Photo: "a"},
				{ID: "00000000000b", X: 12, Y: 150, W: 100, H: 100},
				{ID: "00000000000c", X: 300, Y: 12, W: 100, H: 100, Photo: "b"},
				{ID: "00000000000e", X: 150, Y: 150, W: 100, H: 100}},
			Texts:    []*model.TextBlock{{ID: text, X: 150, Y: 20, W: 80, Text: "Привет"}},
			Stickers: []*model.Sticker{{ID: sticker, Art: &model.Art{Kind: "star"}, X: 200, Y: 200, W: 40, H: 40}}},
		{ID: "s2", Photos: []string{"c"}},
		{ID: "s3", Photos: []string{}, Grid: []*model.Place{{ID: "00000000000d", X: 12, Y: 12, W: 100, H: 100}}},
	}
	for _, id := range []string{"a", "b", "c"} {
		p.Photos[id] = model.PhotoRef{Path: testkit.AbsPath(id + ".jpg"), W: 300, H: 200}
	}
	a.ed.Store(mustEditor(t, p, ""))
	a.changed(a.editor().View()) // первый вид уходит в цепочку: дальше событие — только от правки
	emits.Store(0)
	return a, emits, text, sticker
}

// sheetOf — лист id из вида.
func sheetOf(t *testing.T, v model.View, id string) model.SpreadView {
	t.Helper()
	for _, s := range v.Spreads {
		if s.ID == id {
			return s
		}
	}
	t.Fatalf("листа %s нет в виде", id)
	return model.SpreadView{}
}

// TestClipRoundTrip: копия → вставка на другой лист → Save→Load → отмена одним шагом; буфер
// переживает вставку, отмену и вырезание (как системный), несколько объектов разом.
func TestClipRoundTrip(t *testing.T) {
	a, emits, text, sticker := clipApp(t)
	sel := model.ClipSel{Refs: []string{"00000000000b", "00000000000e"}, Texts: []string{text}, Stickers: []string{sticker}}
	info := a.CopyItems("s1", sel)
	if info != (model.ClipInfo{Places: 2, Texts: 1, Stickers: 1, Sheet: "s1"}) {
		t.Fatalf("сводка копирования: %+v", info)
	}
	if emits.Load() != 0 || a.editor().View().CanUndo {
		t.Fatal("копирование не правка: событие или шаг отмены")
	}
	// Нечего копировать (пусто, чужие ID, неизвестный лист) — буфер прежний.
	for _, tc := range []struct {
		sheet string
		sel   model.ClipSel
	}{{"s1", model.ClipSel{}}, {"s1", model.ClipSel{Refs: []string{"нет"}}}, {"нет", sel}} {
		if got := a.CopyItems(tc.sheet, tc.sel); got != info {
			t.Fatalf("%s %+v: буфер стал %+v", tc.sheet, tc.sel, got)
		}
	}

	rev := a.editor().View().Rev
	r, err := a.PasteItems(model.PasteReq{Sheet: "s3"})
	if err != nil {
		t.Fatal(err)
	}
	if r.View.Rev != rev+1 || !r.View.CanUndo || len(r.Repeat) != 0 || r.Clip != info {
		t.Fatalf("вставка: rev %d→%d, повтор %v, буфер %+v", rev, r.View.Rev, r.Repeat, r.Clip)
	}
	if len(r.Refs) != 2 || len(r.Texts) != 1 || len(r.Stickers) != 1 {
		t.Fatalf("выделить после вставки: %+v", r)
	}
	if s := sheetOf(t, r.View, "s3"); len(s.Places) != 3 || len(s.Texts) != 1 || len(s.Stickers) != 1 {
		t.Fatalf("s3: места %d, надписи %d, стикеры %d", len(s.Places), len(s.Texts), len(s.Stickers))
	}
	if emits.Load() != 1 {
		t.Fatalf("рассылок вида %d, нужна 1", emits.Load())
	}

	// Круг Save→Load: вставленное записывается и читается.
	path := filepath.Join(testkit.TempDir(t), "clip.salbum")
	if err := a.editor().Save(path); err != nil {
		t.Fatal(err)
	}
	q, err := salbum.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if s := q.Sheet("s3"); s == nil || len(s.Grid) != 3 || len(s.Texts) != 1 || len(s.Stickers) != 1 {
		t.Fatalf("после Save→Load s3: %+v", s)
	}

	// Отмена — один шаг; буфер остаётся, повторная вставка работает.
	if u := a.Undo(); len(sheetOf(t, u, "s3").Places) != 1 {
		t.Fatal("отмена вставки не вернула s3")
	}
	if a.ClipState() != info {
		t.Fatal("отмена сбросила буфер")
	}
	if r, err = a.PasteItems(model.PasteReq{Sheet: "s2"}); err != nil || !r.Froze || len(sheetOf(t, r.View, "s2").Places) == 0 {
		t.Fatalf("вставка в «Авто»: %v, froze %v", err, r.Froze)
	}
}

// TestClipCutKeepsBuffer: вырезание — один шаг, буфер получает снимок; отмена возвращает
// элементы, буфер остаётся; нечего вырезать — холостая без шага и события.
func TestClipCutKeepsBuffer(t *testing.T) {
	a, emits, text, sticker := clipApp(t)
	rev := a.editor().View().Rev
	cr, err := a.CutItems("s1", model.ClipSel{Refs: []string{"a"}, Texts: []string{text}, Stickers: []string{sticker}})
	if err != nil || cr.View.Rev != rev+1 || cr.Clip.Photos != 1 || cr.Clip.Texts != 1 || cr.Clip.Stickers != 1 {
		t.Fatalf("вырезать: %v, rev %d→%d, буфер %+v", err, rev, cr.View.Rev, cr.Clip)
	}
	if s := sheetOf(t, cr.View, "s1"); len(s.Texts) != 0 || len(s.Stickers) != 0 || len(s.Places) != 3 {
		t.Fatalf("после вырезания s1: %d надписей, %d стикеров, %d мест", len(s.Texts), len(s.Stickers), len(s.Places))
	}
	u := a.Undo()
	if s := sheetOf(t, u, "s1"); len(s.Texts) != 1 || len(s.Stickers) != 1 || len(s.Places) != 4 || u.CanUndo {
		t.Fatal("отмена вырезания не вернула элементы одним шагом")
	}
	if a.ClipState() != cr.Clip {
		t.Fatal("отмена изменила буфер")
	}
	emits.Store(0)
	for _, sel := range []model.ClipSel{{}, {Refs: []string{"нет"}, Texts: []string{"нет"}}} {
		r, err := a.CutItems("s1", sel)
		if err != nil || r.View.Rev != u.Rev || r.Clip != cr.Clip {
			t.Fatalf("холостое вырезание %+v: %v, rev %d, буфер %+v", sel, err, r.View.Rev, r.Clip)
		}
	}
	if emits.Load() != 0 {
		t.Fatal("холостое вырезание разослало вид")
	}
}

// TestClipDuplicateKeepsBuffer: дубль — один шаг, сдвиг на том же листе, буфер не трогается.
func TestClipDuplicateKeepsBuffer(t *testing.T) {
	a, _, text, sticker := clipApp(t)
	info := a.CopyItems("s1", model.ClipSel{Refs: []string{"00000000000b"}})
	rev := a.editor().View().Rev
	r, err := a.DuplicateItems("s1", model.ClipSel{Texts: []string{text}, Stickers: []string{sticker}})
	if err != nil || r.View.Rev != rev+1 || len(r.Texts) != 1 || len(r.Stickers) != 1 || r.Clip != info {
		t.Fatalf("дубль: %v, rev %d→%d, %+v", err, rev, r.View.Rev, r)
	}
	if s := sheetOf(t, r.View, "s1"); len(s.Texts) != 2 || len(s.Stickers) != 2 {
		t.Fatalf("после дубля: %d надписей, %d стикеров", len(s.Texts), len(s.Stickers))
	}
	if r.Texts[0] == text || r.Stickers[0] == sticker {
		t.Fatal("дубль получил старый ID")
	}
	if u := a.Undo(); len(sheetOf(t, u, "s1").Texts) != 1 || u.CanUndo {
		t.Fatal("отмена дубля — не один шаг")
	}
	// Холостой: нечего дублировать.
	rev = a.editor().View().Rev
	if r, err = a.DuplicateItems("s1", model.ClipSel{}); err != nil || r.View.Rev != rev || len(r.Refs)+len(r.Texts)+len(r.Stickers) != 0 {
		t.Fatalf("пустой дубль: %v, %+v", err, r)
	}
}

// TestClipPasteEmpty: пустой буфер и неизвестный лист — холостая вставка: ни шага, ни события.
func TestClipPasteEmpty(t *testing.T) {
	a, emits, text, _ := clipApp(t)
	rev := a.editor().View().Rev
	r, err := a.PasteItems(model.PasteReq{Sheet: "s1"})
	if err != nil || r.View.Rev != rev || r.Clip != (model.ClipInfo{}) {
		t.Fatalf("пустой буфер: %v, rev %d→%d, %+v", err, rev, r.View.Rev, r.Clip)
	}
	a.CopyItems("s1", model.ClipSel{Texts: []string{text}})
	if r, err = a.PasteItems(model.PasteReq{Sheet: "нет"}); err != nil || r.View.Rev != rev {
		t.Fatalf("неизвестный лист: %v, rev %d", err, r.View.Rev)
	}
	if emits.Load() != 0 || a.editor().View().CanUndo {
		t.Fatal("холостая вставка: событие или шаг отмены")
	}
}

// TestClipRepeatAsk: фото буфера уже стоит в месте — ответ с Repeat без правки, события и шага нет;
// вид прежний (rev тот же).
func TestClipRepeatAsk(t *testing.T) {
	a, emits, _, _ := clipApp(t)
	a.CopyItems("s1", model.ClipSel{Refs: []string{"a"}})
	rev := a.editor().View().Rev
	r, err := a.PasteItems(model.PasteReq{Sheet: "s3"})
	if err != nil || !slices.Equal(r.Repeat, []string{"a"}) || r.View.Rev != rev || len(r.Refs) != 0 {
		t.Fatalf("вопрос о повторе: %v, %+v", err, r)
	}
	if emits.Load() != 0 || a.editor().View().CanUndo {
		t.Fatal("вопрос о повторе: событие или шаг отмены")
	}
	// Repeat вне белого списка чистится до вопроса — тот же ответ.
	if r, err = a.PasteItems(model.PasteReq{Sheet: "s3", Repeat: "всё"}); err != nil || len(r.Repeat) != 1 {
		t.Fatalf("чужой repeat: %v, %v", err, r.Repeat)
	}
}

// TestClipClearedOnSwitch: смена альбома сбрасывает буфер — вставка в новый альбом холостая,
// сводка пустая; в новом альбоме можно снять свой снимок.
func TestClipClearedOnSwitch(t *testing.T) {
	a, _, text, _ := clipApp(t)
	if a.CopyItems("s1", model.ClipSel{Texts: []string{text}}).Texts != 1 || a.ClipState().Texts != 1 {
		t.Fatal("буфер не заполнился")
	}
	next := switchAlbum(t, "")
	a.retire(next, nil)
	if got := a.ClipState(); got != (model.ClipInfo{}) {
		t.Fatalf("после смены альбома буфер %+v", got)
	}
	rev := next.View().Rev
	r, err := a.PasteItems(model.PasteReq{Sheet: "s1"})
	if err != nil || r.View.Rev != rev || next.View().CanUndo || len(r.Texts) != 0 {
		t.Fatalf("вставка из чужого буфера: %v, rev %d→%d", err, rev, r.View.Rev)
	}
	if a.clip.c != nil || a.clip.ed != nil {
		t.Fatal("снимок прежнего альбома остался в памяти")
	}
	// В новом альбоме буфер работает заново.
	if info := a.CopyItems("s2", model.ClipSel{Refs: []string{"b"}}); info.Photos != 1 || info.Sheet != "s2" {
		t.Fatalf("копирование в новом альбоме: %+v", info)
	}
}

// TestClipNoPaths: из ответов буфера не уходит ни пути, ни ссылки: ID и числа не содержат
// «/», «\» и «:» (вид View несёт /media-адреса и проверяется не здесь).
func TestClipNoPaths(t *testing.T) {
	a, _, text, sticker := clipApp(t)
	var out []any
	out = append(out, a.CopyItems("s1", model.ClipSel{Refs: []string{"00000000000b"}, Texts: []string{text}, Stickers: []string{sticker}}))
	ask, err := a.PasteItems(model.PasteReq{Sheet: "s3"}) // в буфере нет фото в местах — вставка
	if err != nil || len(ask.Refs) == 0 {
		t.Fatal(err)
	}
	a.CopyItems("s1", model.ClipSel{Refs: []string{"a"}})
	rep, err := a.PasteItems(model.PasteReq{Sheet: "s3"}) // фото уже стоит: ответ с Repeat
	if err != nil || len(rep.Repeat) != 1 {
		t.Fatalf("вопрос о повторе: %v %v", err, rep.Repeat)
	}
	rep.View, ask.View = model.View{}, model.View{}
	out = append(out, rep, ask)
	cr, err := a.CutItems("s1", model.ClipSel{Refs: []string{"00000000000c"}})
	if err != nil {
		t.Fatal(err)
	}
	pr, err := a.PasteItems(model.PasteReq{Sheet: "s3", Repeat: "copy"})
	if err != nil {
		t.Fatal(err)
	}
	dr, err := a.DuplicateItems("s1", model.ClipSel{Texts: []string{text}})
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range []*PasteResult{&pr, &dr} {
		r.View = model.View{}
	}
	cr.View = model.View{}
	out = append(out, cr, pr, dr, a.ClipState())
	for _, v := range out {
		data, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		var tree any
		if err := json.Unmarshal(data, &tree); err != nil {
			t.Fatal(err)
		}
		walkStrings(tree, func(s string) {
			if s != "" && (strings.ContainsAny(s, `/\:`) || strings.Contains(s, "jpg")) {
				t.Errorf("%T: в ответе путь или ссылка %q", v, s)
			}
		})
	}
	// Срезы ответа — всегда [], не null.
	data, _ := json.Marshal(PasteResult{})
	for _, k := range []string{`"refs":[]`, `"texts":[]`, `"stickers":[]`, `"repeat":[]`} {
		if !bytes.Contains(data, []byte(k)) {
			t.Errorf("в пустом PasteResult нет %s", k)
		}
	}
}

// walkStrings обходит JSON-дерево, кроме поля view (вид несёт адреса /media): строки значений.
func walkStrings(v any, f func(string)) {
	switch x := v.(type) {
	case string:
		f(x)
	case []any:
		for _, e := range x {
			walkStrings(e, f)
		}
	case map[string]any:
		for k, e := range x {
			if k != "view" {
				walkStrings(e, f)
			}
		}
	}
}

// TestPasteNaNAt: NaN и Inf в At и Hover — «точки нет», как у запроса без них: итог тот же,
// а не NaN в координатах проекта.
func TestPasteNaNAt(t *testing.T) {
	nan, inf := math.NaN(), math.Inf(1)
	run := func(r model.PasteReq) model.SpreadView {
		a, _, text, sticker := clipApp(t)
		a.CopyItems("s1", model.ClipSel{Texts: []string{text}, Stickers: []string{sticker}})
		r.Sheet = "s3"
		res, err := a.PasteItems(r)
		if err != nil {
			t.Fatal(err)
		}
		return sheetOf(t, res.View, "s3")
	}
	want := run(model.PasteReq{})
	for name, r := range map[string]model.PasteReq{
		"At NaN":      {At: &model.Pt{X: nan, Y: 10}},
		"At Inf":      {At: &model.Pt{X: 10, Y: inf}},
		"Hover NaN":   {Hover: &model.Pt{X: nan, Y: nan}},
		"Hover -Inf":  {Hover: &model.Pt{X: math.Inf(-1), Y: 0}},
		"оба смешаны": {At: &model.Pt{X: nan, Y: nan}, Hover: &model.Pt{X: inf, Y: inf}},
	} {
		got := run(r)
		// ID у вставленного новые: сравниваются положения.
		if len(got.Texts) != 1 || len(got.Stickers) != 1 || got.Texts[0].X != want.Texts[0].X || got.Texts[0].Y != want.Texts[0].Y ||
			got.Stickers[0].X != want.Stickers[0].X || got.Stickers[0].Y != want.Stickers[0].Y {
			t.Errorf("%s: надписи %+v, нужны %+v", name, got.Texts, want.Texts)
		}
		for _, st := range got.Stickers {
			if math.IsNaN(st.X) || math.IsNaN(st.Y) || math.IsInf(st.X, 0) || math.IsInf(st.Y, 0) {
				t.Errorf("%s: стикер с NaN/Inf: %+v", name, st)
			}
		}
	}
}
