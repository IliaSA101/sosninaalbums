package app

// Правки агента 0.35 (154): фон альбома, стикеры на обложку, было/стало семейства — на настоящем App.

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/app/agents"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

// bgPair — две плитки фона каталога.
func (r *edRig) bgPair(t *testing.T) (a, b string) {
	t.Helper()
	items, _ := structured(t, r.call(t, "list_decor", map[string]any{"kind": "bg", "limit": 5}))["items"].([]any)
	if len(items) < 2 {
		t.Fatalf("фонов в каталоге %d", len(items))
	}
	return items[0].(map[string]any)["id"].(string), items[1].(map[string]any)["id"].(string)
}

// TestAgentAlbumBg: фон альбома — тост обоих видов, номера cleared, albumBg по фону альбома, холостой повтор,
// «Недавние» не тронуты, undo_last_edit возвращает проект.
func TestAgentAlbumBg(t *testing.T) {
	r := newEdRig34(t)
	a, b := r.bgPair(t)
	recent := prefs.RecentDecor(r.a.cfgDir, "bg")
	json0 := projectJSON(t, r.a)

	out := r.ok(t, "set_background", map[string]any{"item": a, "to": "album"})
	if out["changed"] != float64(2) || out["albumBg"] != true || out["cleared"] != nil || out["skipped"] != nil {
		t.Fatalf("ответ: %v", out)
	}
	if e := r.lastEdited(t); e.Text != "Фон альбома поставлен" || e.Go != "" {
		t.Fatalf("тост: %+v", e)
	}
	if !reflect.DeepEqual(prefs.RecentDecor(r.a.cfgDir, "bg"), recent) {
		t.Fatal("«Недавние» тронуты")
	}
	r.ok(t, "undo_last_edit", nil)
	if projectJSON(t, r.a) != json0 {
		t.Fatal("undo_last_edit не вернул проект")
	}

	// Свой фон каталога на листе 1 снимается: cleared [1], тост с номерами.
	r.ok(t, "set_background", map[string]any{"item": b, "spreads": []any{1}})
	out = r.ok(t, "set_background", map[string]any{"item": a, "to": "album"})
	if !reflect.DeepEqual(out["cleared"], []any{float64(1)}) || out["albumBg"] != true {
		t.Fatalf("со своим фоном: %v", out)
	}
	if e := r.lastEdited(t); e.Text != "Фон альбома — теперь и на 1 развороте: 1" {
		t.Fatalf("тост: %+v", e)
	}
	// Тот же фон, сняты только свои (лист 2): albumBg false.
	r.ok(t, "set_background", map[string]any{"item": b, "spreads": []any{2}})
	out = r.ok(t, "set_background", map[string]any{"item": a, "to": "album"})
	if !reflect.DeepEqual(out["cleared"], []any{float64(2)}) || out["albumBg"] == true || out["changed"] != float64(1) {
		t.Fatalf("тот же фон: %v", out)
	}
	// Повтор — холостой: ни шага, ни тоста.
	rev, toasts := r.a.editor().View().Rev, len(r.of("agents:edited"))
	out = r.ok(t, "set_background", map[string]any{"item": a, "to": "album"})
	if out["changed"] != float64(0) || r.a.editor().View().Rev != rev || len(r.of("agents:edited")) != toasts {
		t.Fatalf("повтор: %v", out)
	}
}

// TestAgentAlbumBgHidden: лист «скрыт» без своего фона — в hidden; «скрыт» со своим узором — в cleared (отметка снята),
// не в hidden; «скрыт» с фото клиента без согласия — в skipped, не в hidden.
func TestAgentAlbumBgHidden(t *testing.T) {
	r := newEdRig34(t)
	a, b := r.bgPair(t)
	sp := r.a.editor().View().Spreads
	r.ok(t, "set_background", map[string]any{"item": b, "spreads": []any{2}}) // узор на листе 2
	r.a.SetNoAlbumBg(sp[0].ID, true)
	r.a.SetNoAlbumBg(sp[1].ID, true)
	out := r.ok(t, "set_background", map[string]any{"item": a, "to": "album"})
	if !reflect.DeepEqual(out["hidden"], []any{float64(1)}) || !reflect.DeepEqual(out["cleared"], []any{float64(2)}) || out["skipped"] != nil {
		t.Fatalf("скрыт без фона и с узором: %v", out)
	}
	// Фото клиента на скрытом листе 1 без согласия: остаётся (skipped), «скрыт» не обещаем.
	r = newEdRig34(t)
	sp = r.a.editor().View().Spreads
	if _, err := r.a.SetBackdrop(sp[0].ID, "spread", r.photoID(4)); err != nil {
		t.Fatal(err)
	}
	r.a.SetNoAlbumBg(sp[0].ID, true)
	out = r.ok(t, "set_background", map[string]any{"item": b, "to": "album"})
	if !reflect.DeepEqual(out["skipped"], []any{float64(1)}) || out["hidden"] != nil {
		t.Fatalf("скрыт с фото клиента: %v", out)
	}
}

// TestAgentAlbumBgNoSpreadsUndo: в альбоме без разворотов фон альбома — шаг (ревизия выросла, changed 0):
// запись для undo_last_edit делается по смене ревизии, и отмена возвращает проект.
func TestAgentAlbumBgNoSpreadsUndo(t *testing.T) {
	r := newEdRig34(t)
	a, _ := r.bgPair(t)
	sheet := model.New()
	sheet.Settings.Sheet, sheet.Settings.PageW, sheet.Settings.PageH = true, 204, 304
	r.a.ed.Store(mustEditor(t, sheet, ""))
	json0, rev := projectJSON(t, r.a), r.a.editor().View().Rev
	out := r.ok(t, "set_background", map[string]any{"item": a, "to": "album"})
	if out["changed"] != float64(0) || r.a.editor().View().Rev == rev {
		t.Fatalf("ждали changed 0 и рост ревизии: %v, rev %d→%d", out, rev, r.a.editor().View().Rev)
	}
	r.ok(t, "undo_last_edit", nil)
	if projectJSON(t, r.a) != json0 {
		t.Fatal("undo_last_edit не вернул проект")
	}
}

// TestAgentAlbumBgKeepsClientPhoto: фото клиента без согласия остаётся (skipped); с replace и spreads снято только
// на названном листе.
func TestAgentAlbumBgKeepsClientPhoto(t *testing.T) {
	r := newEdRig34(t)
	a, _ := r.bgPair(t)
	sp := r.a.editor().View().Spreads
	for i, ph := range []string{r.photoID(4), r.photoID(2)} {
		if _, err := r.a.SetBackdrop(sp[i].ID, "spread", ph); err != nil {
			t.Fatal(err)
		}
	}
	out := r.ok(t, "set_background", map[string]any{"item": a, "to": "album"})
	if !reflect.DeepEqual(out["skipped"], []any{float64(1), float64(2)}) || out["cleared"] != nil {
		t.Fatalf("без согласия: %v", out)
	}
	for _, bad := range []map[string]any{
		{"item": a, "to": "album", "replace": true},
		{"item": a, "to": "album", "spreads": []any{2}},
		{"item": a, "to": "album", "scope": "left"},
		{"item": a, "to": "album", "replace": true, "spreads": []any{"cover"}},
		{"item": a, "pages": true, "spreads": []any{1}},
	} {
		r.fail(t, "set_background", bad)
	}
	out = r.ok(t, "set_background", map[string]any{"item": a, "to": "album", "replace": true, "spreads": []any{2}})
	if !reflect.DeepEqual(out["cleared"], []any{float64(2)}) || !reflect.DeepEqual(out["skipped"], []any{float64(1)}) || out["replaced"] != true {
		t.Fatalf("согласие на лист 2: %v", out)
	}
	v := r.a.editor().View()
	own := func(s model.SpreadView) (n int) {
		for _, b := range s.Backdrops {
			if !b.Album {
				n++
			}
		}
		return n
	}
	if own(v.Spreads[0]) != 1 || own(v.Spreads[1]) != 0 {
		t.Fatalf("фото фона: лист 1 — %d, лист 2 — %d", own(v.Spreads[0]), own(v.Spreads[1]))
	}
}

// TestAgentAlbumBgPages: pages не назван — по нынешнему фону альбома; лист — всегда весь.
func TestAgentAlbumBgPages(t *testing.T) {
	r := newEdRig34(t)
	a, b := r.bgPair(t)
	pages := func() bool { return r.a.editor().View().AlbumBg.Pages }
	r.ok(t, "set_background", map[string]any{"item": a, "to": "album", "pages": true})
	r.ok(t, "set_background", map[string]any{"item": b, "to": "album"})
	if !pages() {
		t.Fatal("pages не назван, а фон стал на весь разворот")
	}
	r.ok(t, "set_background", map[string]any{"item": a, "to": "album", "pages": false})
	if pages() {
		t.Fatal("pages false не принят")
	}
	sheet := model.New()
	sheet.Settings.Sheet, sheet.Settings.PageW, sheet.Settings.PageH = true, 204, 304
	r.a.ed.Store(mustEditor(t, sheet, ""))
	rev := r.a.editor().View().Rev
	r.ok(t, "set_background", map[string]any{"item": a, "to": "album", "pages": true})
	if v := r.a.editor().View(); v.Rev == rev || v.AlbumBg == nil || v.AlbumBg.Pages {
		t.Fatalf("альбом-лист без разворотов: rev %d→%d, фон %+v", rev, v.Rev, v.AlbumBg)
	}
}

// TestAgentStickersNoCover: «cover» без обложки и у альбома-листа — один отказ errNoCover, в «Всегда» и «Спрашивать»;
// вопроса человеку нет, альбом не меняется.
func TestAgentStickersNoCover(t *testing.T) {
	r := newEdRig34(t)
	item := r.decorID(t, "sticker")
	put := map[string]any{"stickers": []any{map[string]any{"spread": "cover", "item": item}}}
	sheet := model.New()
	sheet.Settings.Sheet, sheet.Settings.PageW, sheet.Settings.PageH = true, 204, 304
	plain := r.a.editor()
	for _, mode := range []string{prefs.PermAllow, prefs.PermAsk} {
		r.a.SetAgentPermGroup("edit", mode)
		for name, ed := range map[string]*album.Editor{"нет обложки": plain, "альбом-лист": mustEditor(t, sheet, "")} {
			r.a.ed.Store(ed)
			json0, asks := projectJSON(t, r.a), len(r.of("agents:ask"))
			if msg := r.fail(t, "add_sticker", put); msg != agents.ErrNoCover.Error() {
				t.Fatalf("%s, %s: %q", mode, name, msg)
			}
			if projectJSON(t, r.a) != json0 || len(r.of("agents:ask")) != asks {
				t.Fatalf("%s, %s: альбом изменился или показан вопрос", mode, name)
			}
		}
	}
}

// TestAgentStickersCover: стикеры на лицевую и заднюю сторонку обложки одним шагом; тост ведёт к обложке.
func TestAgentStickersCover(t *testing.T) {
	r := newEdRig34(t)
	item := r.decorID(t, "sticker")
	if _, err := r.a.CreateCover("", r.photoID(0)); err != nil {
		t.Fatal(err)
	}
	json0 := projectJSON(t, r.a)
	out := r.ok(t, "add_sticker", map[string]any{"stickers": []any{
		map[string]any{"spread": "cover", "item": item},
		map[string]any{"spread": "cover", "page": "left", "item": item, "place": "top-right", "size": "s"},
	}})
	if out["added"] != float64(2) {
		t.Fatalf("ответ: %v", out)
	}
	if st := r.a.editor().View().Cover.Sheet.Stickers; len(st) != 2 {
		t.Fatalf("стикеров на обложке: %d", len(st))
	}
	if e := r.lastEdited(t); e.Text != "Стикеров добавлено: 2" || e.Go != model.CoverID {
		t.Fatalf("тост: %+v", e)
	}
	r.a.Undo()
	if projectJSON(t, r.a) != json0 {
		t.Fatal("один шаг отмены")
	}
}

// TestAgentVaryFamilies: families — до и после по листам запроса; чужая правка сразу после — пары нет.
func TestAgentVaryFamilies(t *testing.T) {
	r := newEdRig34(t)
	ids := []string{}
	for _, s := range r.a.editor().View().Spreads {
		ids = append(ids, s.ID)
	}
	wantBefore, _ := r.a.editor().Families(ids)
	rev := r.a.editor().View().Rev
	res, err := r.a.agentVary(t.Context(), albumRef(t, r.call(t, "get_album", nil))["key"].(string), rev, ids, 2, nil)
	if err != nil {
		t.Fatal(err)
	}
	wantAfter, _ := r.a.editor().Families(ids)
	if !slices.Equal(res.Spreads, ids) || !slices.Equal(res.Before, wantBefore) || !slices.Equal(res.After, wantAfter) {
		t.Fatalf("пара: %+v, до %v, после %v", res, wantBefore, wantAfter)
	}
	// Человек поправил альбом между правкой и вторым чтением семейств (ревизия r1 ушла вперёд) — пары нет.
	for _, c := range []struct {
		r1, rev int
		pair    bool
	}{{7, 7, true}, {8, 7, false}, {6, 7, false}} {
		sp, before, after := varyPair(ids, wantBefore, wantAfter, c.r1, c.rev)
		if got := sp != nil || before != nil || after != nil; got != c.pair {
			t.Errorf("r1 %d, rev %d: пара %v, ждали %v", c.r1, c.rev, got, c.pair)
		}
		if c.pair && (!slices.Equal(sp, ids) || !slices.Equal(before, wantBefore) || !slices.Equal(after, wantAfter)) {
			t.Errorf("честная пара искажена: %v %v %v", sp, before, after)
		}
	}
	out := r.ok(t, "vary_layouts", map[string]any{"spreads": []any{1, 2}, "variety": 2})
	if fams, _ := out["families"].([]any); len(fams) != 2 || fams[0].(map[string]any)["spread"] != float64(1) {
		t.Fatalf("ответ: %v", out)
	}
}

// TestAskTextBgAlbum: вопрос про фон альбома — числа по виду: свои фоны, фото клиента вне согласия, фото на согласии.
func TestAskTextBgAlbum(t *testing.T) {
	r := newAgRig(t)
	ed, bg := r.a.editor(), r.a.agentsDecorFirst(t)
	sp := ed.View().Spreads
	req := agents.AskReq{Kind: agents.AskEdit, Client: "Claude Code", Tool: toolBg, Item: bg.ID, AlbumBg: true}
	if b, err := r.a.editAskText(req, ed); err != nil || len(b.Lines) != 4 || b.Lines[1] != "Он ляжет на все развороты без своего фона." {
		t.Fatalf("без своих фонов: %+v, %v", b, err)
	}
	if _, err := r.a.SetBackdrop(sp[1].ID, "spread", library.IDFor(r.photos[4])); err != nil {
		t.Fatal(err)
	}
	b, err := r.a.editAskText(req, ed)
	if err != nil || !slices.Contains(b.Lines, "Фото клиента на фоне останутся на разворотах: 2") {
		t.Fatalf("фото клиента: %+v, %v", b, err)
	}
	req.Replace, req.Spreads = true, []string{sp[1].ID}
	if b, err = r.a.editAskText(req, ed); err != nil || !slices.Contains(b.Lines, "Фото фона уйдут с разворота 2: 1 фото") {
		t.Fatalf("согласие: %+v, %v", b, err)
	}
}

// TestAskTextBgAlbumOwn: «Свои фоны из каталога» называют только листы с таким фоном; лист согласия, где одно фото
// клиента, в эту строку не попадает (иначе вопрос дважды называет лист); склонение — по числу листов.
func TestAskTextBgAlbumOwn(t *testing.T) {
	r := newAgRig(t)
	ed, bg := r.a.editor(), r.a.agentsDecorFirst(t)
	sp := ed.View().Spreads
	req := agents.AskReq{Kind: agents.AskEdit, Client: "Claude Code", Tool: toolBg, Item: bg.ID, AlbumBg: true, Replace: true}
	// Лист 1 — только фото клиента (на согласии), лист 2 — узор каталога.
	if _, err := r.a.SetBackdrop(sp[0].ID, "spread", library.IDFor(r.photos[4])); err != nil {
		t.Fatal(err)
	}
	if _, err := r.a.ApplyBackground(sp[1].ID, "spread", bg.ID); err != nil {
		t.Fatal(err)
	}
	req.Spreads = []string{sp[0].ID}
	b, err := r.a.editAskText(req, ed)
	if err != nil || !slices.Contains(b.Lines, "Свои фоны из каталога уйдут с разворота: 2") ||
		!slices.Contains(b.Lines, "Фото фона уйдут с разворота 1: 1 фото") {
		t.Fatalf("фото клиента на согласии и узор на листе 2: %+v, %v", b.Lines, err)
	}
	// Лист 1 со своим узором и фото клиента: в обеих строках — своё называет и «Свои фоны», и «Фото фона».
	if _, err = r.a.ApplyBackground(sp[0].ID, "left", bg.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = r.a.SetBackdrop(sp[0].ID, "right", library.IDFor(r.photos[4])); err != nil {
		t.Fatal(err)
	}
	b, err = r.a.editAskText(req, ed)
	if err != nil || !slices.Contains(b.Lines, "Свои фоны из каталога уйдут с разворотов: 1, 2") ||
		!slices.Contains(b.Lines, "Фото фона уйдут с разворота 1: 1 фото") {
		t.Fatalf("узор и фото на листе 1: %+v, %v", b.Lines, err)
	}
	// Альбом-лист: «листа/листов».
	body := editAsk{Client: "Claude Code", Tool: toolBg, AlbumBg: true, Sheet: true, OwnCat: []int{3, 7}, TakeClient: []int{5}, Photos: 2}.albumBgLines()
	want := []string{"Свои фоны из каталога уйдут с листов: 3, 7", "Фото фона уйдут с листа 5: 2 фото"}
	if !slices.Equal(body, want) {
		t.Fatalf("альбом-лист: %q, ждали %q", body, want)
	}
}

// TestAskTextBgReplace: фон на набор с replace — «Фото фона заменятся: K» по плану редактора с учётом области
// (AskReq.Scope): K на левой странице и на правой разный.
func TestAskTextBgReplace(t *testing.T) {
	r := newAgRig(t)
	ed, bg := r.a.editor(), r.a.agentsDecorFirst(t)
	sp := ed.View().Spreads
	if _, err := r.a.SetBackdrop(sp[0].ID, "left", library.IDFor(r.photos[4])); err != nil {
		t.Fatal(err)
	}
	if _, err := r.a.SetBackdrop(sp[1].ID, "spread", library.IDFor(r.photos[3])); err != nil {
		t.Fatal(err)
	}
	base := agents.AskReq{Kind: agents.AskEdit, Client: "Claude Code", Tool: toolBg, Item: bg.ID, Spreads: []string{sp[0].ID, sp[1].ID}, Replace: true}
	for scope, want := range map[string]string{"": "Фото фона заменятся: 2", "spread": "Фото фона заменятся: 2", "left": "Фото фона заменятся: 2", "right": "Фото фона заменятся: 1"} {
		req := base
		req.Scope = scope
		b, err := r.a.editAskText(req, ed)
		if err != nil || !slices.Contains(b.Lines, want) {
			t.Errorf("scope %q: ждали %q, получили %+v, %v", scope, want, b.Lines, err)
		}
	}
	// Без replace и без листов строки нет: вопрос не обещает замены, которой не будет.
	for name, req := range map[string]agents.AskReq{
		"без replace": {Kind: base.Kind, Client: base.Client, Tool: base.Tool, Item: base.Item, Spreads: base.Spreads},
		"без листов":  {Kind: base.Kind, Client: base.Client, Tool: base.Tool, Item: base.Item, Replace: true},
	} {
		if b, err := r.a.editAskText(req, ed); err != nil || slices.ContainsFunc(b.Lines, func(l string) bool { return strings.Contains(l, "Фото фона заменятся") }) {
			t.Errorf("%s: %+v, %v", name, b.Lines, err)
		}
	}
}

// TestNumsText: диапазоны от трёх подряд, больше двенадцати — «и ещё N».
func TestNumsText(t *testing.T) {
	long := make([]int, 20)
	for i := range long {
		long[i] = i*2 + 2
	}
	for _, c := range []struct {
		in   []int
		want string
	}{{nil, ""}, {[]int{7, 3, 18}, "3, 7, 18"}, {[]int{3, 5, 6, 7, 8}, "3, 5-8"}, {[]int{1, 2}, "1, 2"}, {long, "2, 4, 6, 8, 10, 12, 14, 16, 18, 20, 22, 24, и ещё 8"}} {
		if got := numsText(c.in); got != c.want {
			t.Errorf("%v: %q, ждали %q", c.in, got, c.want)
		}
	}
}
