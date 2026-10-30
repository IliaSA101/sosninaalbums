package app

// Правки агента 0.34 (151 Т4) на настоящем App: отметки, обложка, фон, стикеры, формат типографии
// через службу и сокет. Один вызов — один шаг Ctrl+Z, холостая — без шага, тост — слова Go.

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/model/printprof"
	"github.com/IliaSA101/sosninaalbums/internal/app/agents"
	"github.com/IliaSA101/sosninaalbums/internal/app/ownprint"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

// newEdRig34 — стенд правок с разрешением «Разрешить» у правящих инструментов: вопрос об изменении
// (AskEdit) в окне App — задача 150, тут проверяются сами правки.
func newEdRig34(t *testing.T) *edRig {
	t.Helper()
	r := newEdRig(t)
	r.a.SetAgentPermGroup("edit", prefs.PermAllow)
	return r
}

// tool вызывает правящий инструмент: album подставляется из get_album, остальное — kv.
func (r *edRig) tool(t *testing.T, name string, kv map[string]any) *mcp.CallToolResult {
	t.Helper()
	args := map[string]any{"album": albumRef(t, r.call(t, "get_album", nil))}
	for k, v := range kv {
		args[k] = v
	}
	return r.call(t, name, args)
}

// ok — вызов без ошибки, ответ как объект.
func (r *edRig) ok(t *testing.T, name string, kv map[string]any) map[string]any {
	t.Helper()
	res := r.tool(t, name, kv)
	if res.IsError {
		t.Fatalf("%s %v: %s", name, kv, agText(res))
	}
	return structured(t, res)
}

// fail — вызов с ошибкой; её текст.
func (r *edRig) fail(t *testing.T, name string, kv map[string]any) string {
	t.Helper()
	res := r.tool(t, name, kv)
	if !res.IsError {
		t.Fatalf("%s %v: ждали отказ, пришло %s", name, kv, agText(res))
	}
	return agText(res)
}

func (r *edRig) photoID(i int) string { return library.IDFor(r.photos[i]) }

// decorID — id первой плитки каталога kind (встроенные идут первыми).
func (r *edRig) decorID(t *testing.T, kind string) string {
	t.Helper()
	items, _ := structured(t, r.call(t, "list_decor", map[string]any{"kind": kind, "limit": 5}))["items"].([]any)
	if len(items) == 0 {
		t.Fatalf("каталог %s пуст", kind)
	}
	return items[0].(map[string]any)["id"].(string)
}

// lastEdited — последний тост agents:edited.
func (r *edRig) lastEdited(t *testing.T) AgentEdited {
	t.Helper()
	evs := r.of("agents:edited")
	if len(evs) == 0 {
		t.Fatal("тоста agents:edited нет")
	}
	return evs[len(evs)-1].(AgentEdited)
}

// TestAgentMarksSameAsHuman: итог set_marks — тот же, что у SetMarks человека; id не из ленты — в skipped;
// один шаг отмены.
func TestAgentMarksSameAsHuman(t *testing.T) {
	r := newEdRig34(t)
	ed := r.a.editor()
	ids := []string{r.photoID(0), r.photoID(1)}
	patch := model.MarkPatch{Rating: new(4), Flag: new(1), Label: new("red")}
	json0 := projectJSON(t, r.a)

	out := r.ok(t, "set_marks", map[string]any{"ids": []any{ids[0], ids[1], "ffffffffffffffff"}, "rating": 4, "flag": "pick", "label": "red"})
	if out["changed"] != float64(2) || !reflect.DeepEqual(out["skipped"], []any{"ffffffffffffffff"}) {
		t.Fatalf("ответ: %v", out)
	}
	viaAgent := ed.Marks()
	r.a.Undo()
	if projectJSON(t, r.a) != json0 {
		t.Fatal("один шаг отмены не вернул проект")
	}
	if _, err := r.a.SetMarks(ids, patch); err != nil {
		t.Fatal(err)
	}
	if human := ed.Marks(); !reflect.DeepEqual(viaAgent, human) {
		t.Fatalf("агент %v, человек %v", viaAgent, human)
	}
	if e := r.lastEdited(t); e.Text != "Отметки: 2 фото" || e.Go != "" {
		t.Fatalf("тост: %+v", e)
	}
}

// TestAgentMarksLabelTwice: повтор того же цвета его не снимает (Toggle выключен); холостой повтор — без шага и тоста.
func TestAgentMarksLabelTwice(t *testing.T) {
	r := newEdRig34(t)
	id := r.photoID(0)
	r.ok(t, "set_marks", map[string]any{"ids": []any{id}, "label": "red"})
	rev, toasts := r.a.editor().View().Rev, len(r.of("agents:edited"))
	out := r.ok(t, "set_marks", map[string]any{"ids": []any{id}, "label": "red"})
	if out["changed"] != float64(0) || r.a.editor().View().Rev != rev || len(r.of("agents:edited")) != toasts {
		t.Fatalf("повтор: %v, rev %d→%d, тостов %d→%d", out, rev, r.a.editor().View().Rev, toasts, len(r.of("agents:edited")))
	}
	if m := r.a.editor().Marks()[id]; m.Label == nil || *m.Label != "red" {
		t.Fatalf("цвет снят: %+v", m)
	}
	r.ok(t, "set_marks", map[string]any{"ids": []any{id}, "label": "none"})
	if m := r.a.editor().Marks()[id]; m.Label != nil && *m.Label != "" {
		t.Fatalf("none не снял цвет: %+v", m)
	}
}

// TestAgentCoverCreate: обложки нет — создаётся с фото; тост с переходом к обложке; один шаг отмены.
func TestAgentCoverCreate(t *testing.T) {
	r := newEdRig34(t)
	json0 := projectJSON(t, r.a)
	out := r.ok(t, "set_cover_photo", map[string]any{"photo": r.photoID(0)})
	if out["created"] != true || out["changed"] != true || out["product"] == "" {
		t.Fatalf("ответ: %v", out)
	}
	if place, _ := out["place"].(string); place != "seat" && place != "background" {
		t.Fatalf("place: %v", out["place"])
	}
	if v := r.a.editor().View(); v.Cover == nil || len(v.Cover.Sheet.Photos)+len(v.BackdropUsed) == 0 {
		t.Fatalf("обложка без фото: %+v", v.Cover)
	}
	if e := r.lastEdited(t); e.Text != "Обложка создана с фото" || e.Go != model.CoverID {
		t.Fatalf("тост: %+v", e)
	}
	r.a.Undo()
	if projectJSON(t, r.a) != json0 {
		t.Fatal("отмена создания")
	}
}

// TestAgentCoverTwice: обложка есть — другое фото встаёт на лицевую, тот же кадр повторно — changed false
// (TestAgentCoverSameAgain); развороты не тронуты; каждый вызов — шаг.
func TestAgentCoverTwice(t *testing.T) {
	r := newEdRig34(t)
	r.ok(t, "set_cover_photo", map[string]any{"photo": r.photoID(0)})
	spreads := len(r.a.editor().View().Spreads)
	json1 := projectJSON(t, r.a)
	out := r.ok(t, "set_cover_photo", map[string]any{"photo": r.photoID(1)})
	if out["created"] == true || out["changed"] != true || (out["place"] != "seat" && out["place"] != "background") {
		t.Fatalf("второе фото: %v", out)
	}
	if e := r.lastEdited(t); e.Text != "Фото на обложке" || e.Go != model.CoverID {
		t.Fatalf("тост: %+v", e)
	}
	if len(r.a.editor().View().Spreads) != spreads {
		t.Fatal("развороты изменились")
	}
	json2, rev, toasts := projectJSON(t, r.a), r.a.editor().View().Rev, len(r.of("agents:edited"))
	if again := r.ok(t, "set_cover_photo", map[string]any{"photo": r.photoID(1)}); again["changed"] != false ||
		projectJSON(t, r.a) != json2 || r.a.editor().View().Rev != rev || len(r.of("agents:edited")) != toasts {
		t.Fatalf("тот же кадр повторно: %v", again)
	}
	r.a.Undo()
	if projectJSON(t, r.a) != json1 {
		t.Fatal("один шаг отмены не вернул обложку")
	}
}

// TestAgentCoverBackdrop: у обложки без мест фото идёт фоном лицевой.
func TestAgentCoverBackdrop(t *testing.T) {
	r := newEdRig34(t)
	out := r.ok(t, "set_cover_photo", map[string]any{"photo": r.photoID(0)})
	if out["place"] != "background" {
		t.Fatalf("обложка по умолчанию с местами (%v): места проверяет TestAgentCoverSeat", out["place"])
	}
	v := r.a.editor().View()
	if !containsID(v.BackdropUsed, r.photoID(0)) {
		t.Fatalf("фото не на фоне лицевой: %v", v.BackdropUsed)
	}
}

func containsID(list []string, id string) bool {
	for _, s := range list {
		if s == id {
			return true
		}
	}
	return false
}

// TestAgentBgSpreads: фон из каталога на два листа — один шаг, тост со словом «разворота», переход к первому.
// Листы с фото фона клиента без replace пропускаются (TestAgentBgKeepsPhotoBg), с replace — заменяются.
func TestAgentBgSpreads(t *testing.T) {
	r := newEdRig34(t)
	item := r.decorID(t, "bg")
	json0 := projectJSON(t, r.a)
	v0 := r.a.editor().View()
	out := r.ok(t, "set_background", map[string]any{"item": item, "spreads": []any{1, 2}})
	if out["changed"] != float64(2) || out["skipped"] != nil {
		t.Fatalf("ответ: %v", out)
	}
	if e := r.lastEdited(t); e.Text != "Фон поставлен на 2 разворота" || e.Go != v0.Spreads[0].ID {
		t.Fatalf("тост: %+v", e)
	}
	r.a.Undo()
	if projectJSON(t, r.a) != json0 {
		t.Fatal("один шаг отмены не вернул фон")
	}
}

func TestAgentBgKeepsPhotoBg(t *testing.T) {
	r := newEdRig34(t)
	item := r.decorID(t, "bg")
	sp := r.a.editor().View().Spreads
	if _, err := r.a.SetBackdrop(sp[1].ID, "spread", r.photoID(4)); err != nil {
		t.Fatal(err)
	}
	json1 := projectJSON(t, r.a)
	out := r.ok(t, "set_background", map[string]any{"item": item, "spreads": []any{1, 2}})
	if out["changed"] != float64(1) || !reflect.DeepEqual(out["skipped"], []any{float64(2)}) || out["replaced"] == true {
		t.Fatalf("без replace: %v", out)
	}
	if e := r.lastEdited(t); e.Text != "Фон поставлен на 1 разворот" || e.Go != sp[0].ID {
		t.Fatalf("тост: %+v", e)
	}
	r.a.Undo()
	if projectJSON(t, r.a) != json1 {
		t.Fatal("отмена")
	}
	out = r.ok(t, "set_background", map[string]any{"item": item, "spreads": []any{1, 2}, "replace": true})
	if out["changed"] != float64(2) || out["skipped"] != nil || out["replaced"] != true {
		t.Fatalf("replace: %v", out)
	}
}

// TestAgentBgNotCover: обложка — не цель фона.
func TestAgentBgNotCover(t *testing.T) {
	r := newEdRig34(t)
	json0 := projectJSON(t, r.a)
	if msg := r.fail(t, "set_background", map[string]any{"item": r.decorID(t, "bg"), "spreads": []any{"cover"}}); msg == "" {
		t.Fatal("пустой отказ")
	}
	if projectJSON(t, r.a) != json0 {
		t.Fatal("альбом изменился")
	}
}

// TestAgentStickers: три стикера на два листа одним шагом; тост со счётом; переход к первому листу.
func TestAgentStickers(t *testing.T) {
	r := newEdRig34(t)
	item := r.decorID(t, "sticker")
	sp := r.a.editor().View().Spreads
	json0 := projectJSON(t, r.a)
	out := r.ok(t, "add_sticker", map[string]any{"stickers": []any{
		map[string]any{"spread": 1, "item": item},
		map[string]any{"spread": 1, "item": item, "place": "top-right", "size": "s"},
		map[string]any{"spread": 2, "page": "left", "item": item, "place": "bottom-left", "size": "l"},
	}})
	if out["added"] != float64(3) || len(out["ids"].([]any)) != 3 {
		t.Fatalf("ответ: %v", out)
	}
	v := r.a.editor().View()
	if len(v.Spreads[0].Stickers) != 2 || len(v.Spreads[1].Stickers) != 1 {
		t.Fatalf("стикеров: %d и %d", len(v.Spreads[0].Stickers), len(v.Spreads[1].Stickers))
	}
	if e := r.lastEdited(t); e.Text != "Стикеров добавлено: 3" || e.Go != sp[0].ID {
		t.Fatalf("тост: %+v", e)
	}
	r.a.Undo()
	if projectJSON(t, r.a) != json0 {
		t.Fatal("один шаг отмены")
	}
}

// TestAgentStickersNotCover: обложка не цель; пачка с негодным листом отказывает целиком.
func TestAgentStickersNotCover(t *testing.T) {
	r := newEdRig34(t)
	item := r.decorID(t, "sticker")
	json0 := projectJSON(t, r.a)
	r.fail(t, "add_sticker", map[string]any{"stickers": []any{map[string]any{"spread": 1, "item": item}, map[string]any{"spread": "cover", "item": item}}})
	if projectJSON(t, r.a) != json0 {
		t.Fatal("часть пачки применилась")
	}
}

// TestAgentStickerTotal: на листе уже MaxStickers — пачка отказывает целиком, шага и тоста нет.
func TestAgentStickerTotal(t *testing.T) {
	r := newEdRig34(t)
	item := r.decorID(t, "sticker")
	id := r.a.editor().View().Spreads[0].ID
	for len(r.a.editor().View().Spreads[0].Stickers) < model.MaxStickers {
		if _, err := r.a.AddSticker(id, 1, item); err != nil {
			t.Fatal(err)
		}
	}
	json0, toasts := projectJSON(t, r.a), len(r.of("agents:edited"))
	msg := r.fail(t, "add_sticker", map[string]any{"stickers": []any{map[string]any{"spread": 2, "item": item}, map[string]any{"spread": 1, "item": item}}})
	if msg != "на листе или в альбоме нет места для стольких стикеров" {
		t.Fatalf("отказ: %q", msg)
	}
	if projectJSON(t, r.a) != json0 || len(r.of("agents:edited")) != toasts {
		t.Fatal("пачка применилась или был тост")
	}
}

// profileFormat — настоящие профиль и формат типографии id по имени кнопки.
func profileFormat(t *testing.T, id, name string) (printprof.Profile, printprof.PageFormat) {
	t.Helper()
	pr, ok := printprof.Current().ByID(id)
	if !ok {
		t.Fatalf("нет профиля %s", id)
	}
	for _, f := range pr.Formats {
		if f.Name == name {
			return pr, f
		}
	}
	t.Fatalf("у %s нет формата %s", id, name)
	return pr, printprof.PageFormat{}
}

// TestAgentPrint: apply_print = кнопка формата (SetSettings с полями ownprint.PrintSettings); «30x20» = «30×20»;
// тост «Формат: …»; один шаг отмены; повтор — холостая.
func TestAgentPrint(t *testing.T) {
	r := newEdRig34(t)
	pr, f := profileFormat(t, "fabrika", "30×20")
	cur := r.a.editor().View().Settings
	want := ownprint.PrintSettings(pr, f, cur)
	json0 := projectJSON(t, r.a)

	out := r.ok(t, "apply_print", map[string]any{"profile": "fabrika", "format": "30x20"}) // TestAgentPrintFormatX
	page := out["page"].(map[string]any)
	got := r.a.editor().View().Settings
	if out["changed"] != true || page["w"] != got.PageW || page["h"] != got.PageH || got.PageW != want.PageW || got.PageH != want.PageH {
		t.Fatalf("ответ %v, настройки %+v, ждали %+v", out, got, want)
	}
	if e := r.lastEdited(t); e.Text != "Формат: "+pr.Name+" 30×20" || e.Go != "" {
		t.Fatalf("тост: %+v", e)
	}
	r.a.Undo()
	if projectJSON(t, r.a) != json0 {
		t.Fatal("один шаг отмены")
	}
	// Паритет с кнопкой формата: тот же проект после SetSettings(PrintSettings).
	r.a.SetSettings(want)
	human := projectJSON(t, r.a)
	r.a.Undo()
	r.ok(t, "apply_print", map[string]any{"profile": "fabrika", "format": "30×20"})
	if projectJSON(t, r.a) != human {
		t.Fatal("итог агента не совпал с кнопкой формата")
	}
	toasts := len(r.of("agents:edited"))
	if again := r.ok(t, "apply_print", map[string]any{"profile": "fabrika", "format": "30×20"}); again["changed"] != false || len(r.of("agents:edited")) != toasts {
		t.Fatalf("повтор: %v", again)
	}
}

// TestAgentPrintKeepsProduct и TestAgentPrintOtherProduct: изделие своей типографии остаётся (шаблон идёт за
// форматом), чужое, пустое и «типографии нет на этом ПК» (TestAgentPrintMissingProduct) — заменяется на умолчание.
func TestAgentPrintProducts(t *testing.T) {
	cases := []struct {
		name, preset string
		keep         bool
	}{
		{"KeepsProduct", "fabrika-photo-1", true},
		{"Custom", model.CoverCustom, true},
		{"OtherProduct", "fsfera-thin", false},
		{"MissingProduct", "", false}, // настоящее «Типография не найдена»: ghostCover
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := newEdRig34(t)
			if c.preset == "" {
				ghostCover(t, r)
			} else {
				if _, err := r.a.CreateCover("", r.photoID(0)); err != nil {
					t.Fatalf("обложка не создана: %v", err)
				}
				if v, err := r.a.ApplyCoverPreset(c.preset); err != nil || v.Cover == nil || v.Cover.Preset != c.preset {
					t.Fatalf("изделие %s не поставилось: %v", c.preset, err)
				}
			}
			pr, _ := profileFormat(t, "fabrika", "30×20")
			before := r.a.editor().View().Cover.Preset
			out := r.ok(t, "apply_print", map[string]any{"profile": "fabrika", "format": "30×20"})
			after := r.a.editor().View().Cover.Preset
			if c.keep && (after != before || out["coverChanged"] == true) {
				t.Fatalf("изделие %s→%s, ответ %v", before, after, out)
			}
			if !c.keep && (after != pr.Default || out["coverChanged"] != true) {
				t.Fatalf("изделие %s→%s, ждали умолчание %s; ответ %v", before, after, pr.Default, out)
			}
		})
	}
}

// TestAgentEditsOneStep: каждый из пяти инструментов — ровно один шаг Ctrl+Z.
func TestAgentEditsOneStep(t *testing.T) {
	calls := []struct {
		name string
		kv   func(*edRig, *testing.T) map[string]any
	}{
		{"set_marks", func(r *edRig, _ *testing.T) map[string]any {
			return map[string]any{"ids": []any{r.photoID(0), r.photoID(1)}, "rating": 3}
		}},
		{"set_cover_photo", func(r *edRig, _ *testing.T) map[string]any { return map[string]any{"photo": r.photoID(0)} }},
		{"set_background", func(r *edRig, t *testing.T) map[string]any {
			return map[string]any{"item": r.decorID(t, "bg"), "spreads": []any{1, 2}}
		}},
		{"add_sticker", func(r *edRig, t *testing.T) map[string]any {
			return map[string]any{"stickers": []any{map[string]any{"spread": 1, "item": r.decorID(t, "sticker")}, map[string]any{"spread": 2, "item": r.decorID(t, "sticker")}}}
		}},
		{"apply_print", func(r *edRig, _ *testing.T) map[string]any {
			return map[string]any{"profile": "fabrika", "format": "30×20"}
		}},
	}
	for _, c := range calls {
		t.Run(c.name, func(t *testing.T) {
			r := newEdRig34(t)
			json0 := stateOf(t, r.a)
			r.ok(t, c.name, c.kv(r, t))
			if stateOf(t, r.a) == json0 {
				t.Fatal("правки нет")
			}
			r.a.Undo()
			if stateOf(t, r.a) != json0 {
				t.Fatal("одна отмена не вернула альбом")
			}
			// undo_last_edit агента тоже снимает именно этот шаг.
			r.ok(t, c.name, c.kv(r, t))
			r.ok(t, "undo_last_edit", nil)
			if stateOf(t, r.a) != json0 {
				t.Fatal("undo_last_edit не вернул альбом")
			}
		})
	}
}

// stateOf — проект и свои отметки (они лежат рядом с проектом, в его JSON не входят).
func stateOf(t *testing.T, a *App) string {
	t.Helper()
	m, err := json.Marshal(a.editor().Marks())
	if err != nil {
		t.Fatal(err)
	}
	return projectJSON(t, a) + string(m)
}

// TestAgentNoRecent: плитки, поставленные агентом, в «Недавние» окна не попадают (0.34-151-12).
func TestAgentNoRecent(t *testing.T) {
	r := newEdRig34(t)
	r.ok(t, "set_background", map[string]any{"item": r.decorID(t, "bg"), "spreads": []any{1}})
	r.ok(t, "add_sticker", map[string]any{"stickers": []any{map[string]any{"spread": 1, "item": r.decorID(t, "sticker")}}})
	for _, kind := range []string{"bg", "sticker"} {
		if got := prefs.RecentDecor(r.a.cfgDir, kind); len(got) != 0 {
			t.Errorf("недавние %s: %v", kind, got)
		}
	}
}

// TestAgentToastsText: тексты тостов всех пяти инструментов — из Go, без пути и имён; клиент подставлен.
func TestAgentToastsText(t *testing.T) {
	r := newEdRig34(t)
	sp := r.a.editor().View().Spreads
	pr, _ := profileFormat(t, "fabrika", "30×20")
	want := []struct {
		name string
		kv   map[string]any
		text string
		goTo string
	}{
		{"set_marks", map[string]any{"ids": []any{r.photoID(0)}, "flag": "pick"}, "Отметки: 1 фото", ""},
		{"set_cover_photo", map[string]any{"photo": r.photoID(0)}, "Обложка создана с фото", model.CoverID},
		{"set_background", map[string]any{"item": r.decorID(t, "bg"), "spreads": []any{1}}, "Фон поставлен на 1 разворот", sp[0].ID},
		{"add_sticker", map[string]any{"stickers": []any{map[string]any{"spread": 2, "item": r.decorID(t, "sticker")}}}, "Стикеров добавлено: 1", sp[1].ID},
		{"apply_print", map[string]any{"profile": "fabrika", "format": "30×20"}, "Формат: " + pr.Name + " 30×20", ""},
	}
	for _, w := range want {
		r.ok(t, w.name, w.kv)
		if e := r.lastEdited(t); e.Text != w.text || e.Go != w.goTo || e.Client != "тест" || e.Rev != r.a.editor().View().Rev {
			t.Errorf("%s: тост %+v", w.name, e)
		}
		if e := r.lastEdited(t); strings.ContainsAny(e.Text, "/\\") || strings.Contains(e.Text, ".jpg") || strings.Contains(e.Text, "Иванов") {
			t.Errorf("%s: в тосте путь или имя файла: %q", w.name, e.Text)
		}
	}
}

// TestAgentsHost34Filled: Host настоящего App несёт все семь замыканий 0.34 (иначе инструментов нет).
func TestAgentsHost34Filled(t *testing.T) {
	h := newApp(t).agentsHost()
	for name, ok := range map[string]bool{"Sheet": h.Sheet != nil, "Families": h.Families != nil, "Mark": h.Mark != nil, "Cover": h.Cover != nil,
		"Bg": h.Bg != nil, "Stickers": h.Stickers != nil, "Print": h.Print != nil} {
		if !ok {
			t.Errorf("Host.%s не заполнен", name)
		}
	}
	// Ошибки Host: чужой ключ и устаревшая ревизия — ошибки службы, до Editor дело не доходит.
	if _, err := h.Mark(t.Context(), agents.MarkReq{Key: "чужой", IDs: []string{"a"}, Flag: "pick"}); err == nil {
		t.Error("чужой ключ принят")
	}
}
