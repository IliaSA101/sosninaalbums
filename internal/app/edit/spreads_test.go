package edit

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Перестановка и перенос фото: рабочий случай меняет состав и шлёт вид, отказ (неизвестное фото,
// переполненный разворот) ничего не меняет; Undo и Redo проходят по шагам.
func TestPhotoMoves(t *testing.T) {
	a, views := spreadsMethods(t, []string{"a", "b", "c"}, []string{"d"}, []string{"e"})
	rev := func() int { return a.editor().View().Rev }

	r := rev()
	v := a.SwapPhotos("a", "d")
	if !equalComposition(photosOf(v), []string{"d", "b", "c"}, []string{"a"}, []string{"e"}) || v.Rev <= r {
		t.Fatalf("SwapPhotos: %v, rev %d→%d", photosOf(v), r, v.Rev)
	}
	// Неизвестное фото — холостая правка: иначе фото встало бы дважды.
	r = rev()
	if v = a.SwapPhotos("a", "нет"); v.Rev != r || !equalComposition(photosOf(v), []string{"d", "b", "c"}, []string{"a"}, []string{"e"}) {
		t.Fatalf("SwapPhotos неизвестного: %v, rev %d→%d", photosOf(v), r, v.Rev)
	}

	// Перенос в другой разворот на позицию 1; опустевший прежний разворот исчезает.
	v, err := a.MovePhoto("e", "s1", 1)
	if err != nil || !equalComposition(photosOf(v), []string{"d", "e", "b", "c"}, []string{"a"}) {
		t.Fatalf("MovePhoto: %v, %v", photosOf(v), err)
	}
	// Неизвестное фото — не ошибка и не правка.
	r = rev()
	if v, err = a.MovePhoto("нет", "s1", 0); err != nil || v.Rev != r {
		t.Fatalf("MovePhoto неизвестного: err %v, rev %d→%d", err, r, v.Rev)
	}

	// Разворот уже полон: перенос в него — ошибка, проект как был.
	full := make([]string, model.MaxPhotos)
	for i := range full {
		full[i] = fmt.Sprintf("F%d", i) // 'A'+i на 64 фото дошло бы до «z»
	}
	b, _ := spreadsMethods(t, full, []string{"z"})
	before := b.editor().View()
	v, err = b.MovePhoto("z", "s1", -1)
	if err == nil || !strings.Contains(err.Error(), "не может быть больше") {
		t.Fatalf("переполнение: err %v", err)
	}
	if v.Rev != before.Rev || !equalComposition(photosOf(v), full, []string{"z"}) {
		t.Fatalf("после отказа: %v, rev %d→%d", photosOf(v), before.Rev, v.Rev)
	}

	// Удаление фото из альбома: уходит из разворота (неизвестный ID пропускается).
	v = a.RemovePhotos([]string{"b", "нет"})
	if !equalComposition(photosOf(v), []string{"d", "e", "c"}, []string{"a"}) {
		t.Fatalf("RemovePhotos: %v", photosOf(v))
	}

	// Каждая успешная правка и каждый ответ — событие view с тем же rev, что вернул метод.
	last := views()
	if len(last) == 0 || last[len(last)-1].Rev != v.Rev {
		t.Fatalf("событие view не совпало с ответом: %d событий", len(last))
	}

	// Откат всех шагов возвращает исходный состав, Redo — вперёд.
	for a.editor().View().CanUndo {
		a.undo()
	}
	if got := photosOf(a.editor().View()); !equalComposition(got, []string{"a", "b", "c"}, []string{"d"}, []string{"e"}) {
		t.Fatalf("после отката: %v", got)
	}
	if v = a.redo(); !equalComposition(photosOf(v), []string{"d", "b", "c"}, []string{"a"}, []string{"e"}) || !v.CanUndo {
		t.Fatalf("Redo: %v", photosOf(v))
	}
}

// Redo без шагов вперёд — вид без изменений.
func TestRedoNothing(t *testing.T) {
	a, _ := spreadsMethods(t, []string{"a"})
	before := a.editor().View()
	if v := a.redo(); v.Rev != before.Rev || v.CanRedo {
		t.Fatalf("Redo без шагов: rev %d→%d, canRedo %v", before.Rev, v.Rev, v.CanRedo)
	}
}

// Разделить и склеить развороты, переставить и удалить: границы и отказы не меняют проект.
func TestSplitMergeMoveDelete(t *testing.T) {
	a, _ := spreadsMethods(t, []string{"a", "b", "c"}, []string{"d"})
	rev := func() int { return a.editor().View().Rev }

	// Деление: at вне 1..len−1 и неизвестный разворот — ничего.
	r := rev()
	for _, c := range []struct {
		id string
		at int
	}{{"s1", 0}, {"s1", 3}, {"s1", -1}, {"нет", 1}} {
		if v, err := a.SplitSpread(c.id, c.at); err != nil || v.Rev != r || len(v.Spreads) != 2 {
			t.Fatalf("SplitSpread(%q, %d) изменил проект: rev %d→%d", c.id, c.at, r, v.Rev)
		}
	}
	v, err := a.SplitSpread("s1", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(v.Spreads) != 3 || !equalComposition(photosOf(v), []string{"a"}, []string{"b", "c"}, []string{"d"}) {
		t.Fatalf("SplitSpread: %v", photosOf(v))
	}
	tailID := v.Spreads[1].ID

	// Склейка возвращает состав; у последнего разворота «следующего» нет — отказ.
	r = rev()
	if v = a.MergeSpreads("s2"); v.Rev != r {
		// s2 — исходный «d»: следующего нет
		t.Fatalf("MergeSpreads последнего: rev %d→%d", r, v.Rev)
	}
	if v = a.MergeSpreads("нет"); v.Rev != r {
		t.Fatalf("MergeSpreads неизвестного: rev %d→%d", r, v.Rev)
	}
	v = a.MergeSpreads("s1")
	if !equalComposition(photosOf(v), []string{"a", "b", "c"}, []string{"d"}) {
		t.Fatalf("MergeSpreads: %v", photosOf(v))
	}
	if slices.ContainsFunc(v.Spreads, func(s model.SpreadView) bool { return s.ID == tailID }) {
		t.Error("хвостовой разворот остался после склейки")
	}

	// Вместе больше MaxPhotos — не склеивается.
	big := make([]string, model.MaxPhotos-1)
	for i := range big {
		big[i] = fmt.Sprintf("k%d", i)
	}
	b, _ := spreadsMethods(t, big, []string{"y", "z"})
	rb := b.editor().View().Rev
	if v = b.MergeSpreads("s1"); v.Rev != rb || len(v.Spreads) != 2 {
		t.Fatalf("склейка сверх лимита прошла: rev %d→%d, разворотов %d", rb, v.Rev, len(v.Spreads))
	}

	// Перемещение: за край зажимается, неизвестный — ничего.
	v = a.MoveSpread("s1", 99)
	if v.Spreads[1].ID != "s1" {
		t.Fatalf("MoveSpread в конец: %v", photosOf(v))
	}
	v = a.MoveSpread("s1", -5)
	if v.Spreads[0].ID != "s1" {
		t.Fatalf("MoveSpread в начало: %v", photosOf(v))
	}
	r = rev()
	if v = a.MoveSpread("нет", 0); v.Rev != r {
		t.Fatalf("MoveSpread неизвестного изменил проект")
	}

	// Удаление: неизвестный — ничего, известный уходит и откатывается.
	if v = a.DeleteSpread("нет"); v.Rev != r || len(v.Spreads) != 2 {
		t.Fatalf("DeleteSpread неизвестного: rev %d→%d", r, v.Rev)
	}
	if v = a.DeleteSpread("s1"); len(v.Spreads) != 1 || v.Spreads[0].ID != "s2" {
		t.Fatalf("DeleteSpread: %v", photosOf(v))
	}
	if v = a.undo(); len(v.Spreads) != 2 {
		t.Fatalf("Undo удаления: %d разворотов", len(v.Spreads))
	}
}

// Раскладки: галерея — только чтение (без правки и события), SetLayout/StepLayout меняют ключ,
// битый ключ, тот же ключ, чужой scope и неизвестный разворот — холостые.
func TestLayoutMethods(t *testing.T) {
	a, views := spreadsMethods(t, []string{"a", "b", "c"})
	spread := func(v model.View) model.SpreadView { return v.Spreads[0] }

	events := len(views())
	ch := a.LayoutChoices("s1")
	if ch.Spread != "s1" || ch.Rev != a.editor().View().Rev || len(ch.Whole.InOrder) < 2 {
		t.Fatalf("LayoutChoices: разворот %q, rev %d, раскладок %d", ch.Spread, ch.Rev, len(ch.Whole.InOrder))
	}
	if len(views()) != events {
		t.Error("LayoutChoices прислал событие view")
	}
	if none := a.LayoutChoices("нет"); none.Spread != "нет" || len(none.Whole.InOrder) != 0 || none.Left != nil || none.Right != nil {
		t.Errorf("LayoutChoices неизвестного разворота: %+v", none)
	}

	cur := spread(a.editor().View()).Layout
	var key string
	for _, c := range ch.Whole.InOrder {
		if c.Key != cur {
			key = c.Key
			break
		}
	}
	if key == "" {
		t.Fatal("в галерее нет другой раскладки")
	}
	r := a.editor().View().Rev
	v := a.SetLayout("s1", key)
	if spread(v).Layout != key || v.Rev <= r {
		t.Fatalf("SetLayout: ключ %q (ждали %q), rev %d→%d", spread(v).Layout, key, r, v.Rev)
	}
	// Тот же ключ, битый ключ, неизвестный разворот.
	r = v.Rev
	for _, c := range [][2]string{{"s1", key}, {"s1", "битый"}, {"нет", key}} {
		if v = a.SetLayout(c[0], c[1]); v.Rev != r {
			t.Errorf("SetLayout(%q, %q) изменил проект", c[0], c[1])
		}
	}

	// Листание вперёд меняет ключ, назад — возвращает; delta 0, scope «не тот» и неизвестный разворот — холостые.
	v = a.StepLayout("s1", "spread", 1)
	if spread(v).Layout == key {
		t.Fatal("StepLayout вперёд не сменил раскладку")
	}
	if v = a.StepLayout("s1", "spread", -1); spread(v).Layout != key {
		t.Fatalf("StepLayout назад: %q, ждали %q", spread(v).Layout, key)
	}
	r = v.Rev
	for _, c := range []struct {
		id, scope string
		delta     int
	}{{"s1", "spread", 0}, {"s1", "всё", 1}, {"нет", "spread", 1}} {
		if v = a.StepLayout(c.id, c.scope, c.delta); v.Rev != r {
			t.Errorf("StepLayout(%q, %q, %d) изменил проект", c.id, c.scope, c.delta)
		}
	}
}

// Настройки: значения зажимаются в допустимые пределы, цвет нормализуется, Undo возвращает прежние.
func TestSetSettings(t *testing.T) {
	a, _ := spreadsMethods(t, []string{"a", "b"})
	before := a.editor().View()

	s := before.Settings
	s.PageW, s.PageH, s.Margin, s.Gap, s.DPI, s.Bg = 1e9, 250, 1e9, -4, 0, "#ABCDEF"
	r := before.Rev
	v := a.SetSettings(s)
	want := model.ClampSettings(s)
	if v.Settings != want || v.Rev <= r {
		t.Fatalf("SetSettings: %+v (ждали %+v), rev %d→%d", v.Settings, want, r, v.Rev)
	}
	if v.Settings.PageW >= 1e9 || v.Settings.Gap != 0 || v.Settings.DPI == 0 {
		t.Errorf("пределы не применены: %+v", v.Settings)
	}
	// Те же настройки — холостая правка.
	if v2 := a.SetSettings(v.Settings); v2.Rev != v.Rev {
		t.Errorf("повторные настройки — не холостая правка: rev %d→%d", v.Rev, v2.Rev)
	}
	if v = a.undo(); v.Settings != before.Settings {
		t.Errorf("Undo настроек: %+v, было %+v", v.Settings, before.Settings)
	}
}

// «Разрезать» на пределе MaxSpreads (075-7): ошибка с пределом, вид не рассылается, проект прежний.
func TestSplitSpreadLimit(t *testing.T) {
	p := model.New()
	for i := range model.MaxSpreads {
		id := fmt.Sprintf("p%d", i)
		p.Spreads = append(p.Spreads, &model.Spread{ID: fmt.Sprintf("s%d", i), Photos: []string{id}})
		p.Photos[id] = model.PhotoRef{Path: `C:\` + id + ".jpg", W: 300, H: 200}
	}
	p.Spreads[0].Photos = append(p.Spreads[0].Photos, "x")
	p.Photos["x"] = model.PhotoRef{Path: `C:\x.jpg`, W: 300, H: 200}
	a, views := newMethods(t, p)
	rev := a.editor().View().Rev
	v, err := a.SplitSpread("s0", 1)
	if err == nil || !strings.Contains(err.Error(), "200 разворотов") {
		t.Fatalf("ждали отказ по пределу: %v", err)
	}
	if v.Rev != rev || len(v.Spreads) != model.MaxSpreads || len(views()) != 0 {
		t.Fatalf("после отказа: rev %d→%d, %d разворотов, видов %d", rev, v.Rev, len(v.Spreads), len(views()))
	}
}

// «Убрать из альбома» (075-13): место, фон и свой стикер — один шаг и одна рассылка вида;
// RemovePhotos фон и стикер фото из места оставляет. Undo возвращает всё.
func TestPurgePhotosMethod(t *testing.T) {
	a, views := spreadsMethods(t, []string{"a", "b"}, []string{"c"})
	ed := a.editor()
	pa := model.PhotoInfo{ID: "a", Path: `C:\a.jpg`, W: 300, H: 200}
	if _, err := ed.SetBackdrop("s2", model.ScopeSpread, pa); err != nil {
		t.Fatal(err)
	}
	if _, id, err := ed.AddSticker("s2", 0, model.StickerSrc{Photo: pa}); err != nil || id == "" {
		t.Fatalf("стикер: %q, %v", id, err)
	}
	if v := a.RemovePhotos([]string{"a"}); len(v.Spreads[1].Backdrops) != 1 || len(v.Spreads[1].Stickers) != 1 {
		t.Fatalf("RemovePhotos снял фон или стикер: %+v", v.Spreads[1])
	}
	a.undo()
	n, rev := len(views()), ed.View().Rev
	v := a.PurgePhotos([]string{"a"})
	if v.Rev != rev+1 || len(views()) != n+1 {
		t.Fatalf("rev %d→%d, видов %d→%d", rev, v.Rev, n, len(views()))
	}
	if !equalComposition(photosOf(v), []string{"b"}, []string{"c"}) || len(v.Spreads[1].Backdrops) != 0 || len(v.Spreads[1].Stickers) != 0 {
		t.Fatalf("PurgePhotos: %v %+v", photosOf(v), v.Spreads[1])
	}
	if v = a.undo(); !equalComposition(photosOf(v), []string{"a", "b"}, []string{"c"}) || len(v.Spreads[1].Backdrops) != 1 || len(v.Spreads[1].Stickers) != 1 {
		t.Fatalf("Undo: %v %+v", photosOf(v), v.Spreads[1])
	}
}

// PhotosFor (092): строка журнала «не найдено K из N» без ID и путей; повтор ID считается один раз;
// пустой список — тишина; все найдены — строки нет; ни одного — ErrNoPhoto.
func TestPhotosForLog(t *testing.T) {
	var buf bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(prev) })

	k := twoSpreads(t)
	k.lentPhoto("лента-секрет")
	get := k.h.InfosOrRefs
	check := func(name string, wantLog string) {
		t.Helper()
		got := buf.String()
		buf.Reset()
		if wantLog == "" && got != "" || wantLog != "" && !strings.Contains(got, wantLog) {
			t.Errorf("%s: журнал %q, ждали %q", name, got, wantLog)
		}
		for _, bad := range []string{`C:\`, "секрет", "призрак"} {
			if strings.Contains(got, bad) {
				t.Errorf("%s: в журнале %q есть %q", name, got, bad)
			}
		}
	}

	infos, err := PhotosFor("AddPhotos", []string{"лента-секрет", "призрак-1"}, get)
	if err != nil || len(infos) != 1 {
		t.Fatalf("часть: err %v, %d сведений", err, len(infos))
	}
	check("часть", "не найдено 1 из 2")

	// Повторы во входе: один ID — один счёт.
	if _, err = PhotosFor("NewSpread", []string{"призрак-1", "призрак-1", "лента-секрет"}, get); err != nil {
		t.Fatal(err)
	}
	check("повторы", "не найдено 1 из 2")

	if infos, err = PhotosFor("AddPhotos", []string{"призрак-1", "призрак-2"}, get); !errors.Is(err, ErrNoPhoto) || infos != nil {
		t.Fatalf("ни одного: err %v, %v", err, infos)
	}
	check("ни одного", "не найдено 2 из 2")

	// Фото альбома без ленты — найдено по проекту, строки нет.
	if infos, err = PhotosFor("AddPhotos", []string{"a"}, get); err != nil || len(infos) != 1 {
		t.Fatalf("альбом: err %v, %v", err, infos)
	}
	check("все найдены", "")
	// Пустой список — не просьба: ни ошибки, ни строки.
	if infos, err = PhotosFor("NewSpread", nil, get); err != nil || infos != nil {
		t.Fatalf("пустой: err %v, %v", err, infos)
	}
	check("пустой", "")
}
