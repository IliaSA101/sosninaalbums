package app

// Правки агента 0.35 (154): фон альбома, стикеры на обложку, было/стало семейства — на настоящем App.

import (
	"reflect"
	"slices"
	"testing"

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

// TestAgentStickersNoCover: «cover» без обложки и у альбома-листа — один отказ errNoCover, и в «Спрашивать» тоже.
func TestAgentStickersNoCover(t *testing.T) {
	r := newEdRig34(t)
	item := r.decorID(t, "sticker")
	put := map[string]any{"stickers": []any{map[string]any{"spread": "cover", "item": item}}}
	json0 := projectJSON(t, r.a)
	if msg := r.fail(t, "add_sticker", put); msg != agents.ErrNoCover.Error() {
		t.Fatalf("нет обложки: %q", msg)
	}
	r.a.SetAgentPermGroup("edit", prefs.PermAsk)
	if msg := r.fail(t, "add_sticker", put); msg != agents.ErrNoCover.Error() {
		t.Fatalf("«Спрашивать», нет обложки: %q", msg)
	}
	if projectJSON(t, r.a) != json0 {
		t.Fatal("альбом изменился")
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
	if b, err = r.a.editAskText(req, ed); err != nil || !slices.Contains(b.Lines, "Фото фона уйдут с разворотов 2: 1 фото") {
		t.Fatalf("согласие: %+v, %v", b, err)
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
