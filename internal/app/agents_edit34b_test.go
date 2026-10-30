package app

// Доработка 151 по итогам ревью и аудита: обложка с местами и wrap, лист без обложки, замена
// плиток фона без replace, формы id у отметок, отсутствующая типография у изделия.

import (
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/app/agents"
)

// seatPhotos — файлы (BaseID) в местах лицевой обложки по порядку; пустое место — "".
func seatPhotos(r *edRig) (out []string) {
	p, _ := r.a.editor().Snapshot()
	for _, pl := range p.CoverSeats() {
		out = append(out, model.BaseID(pl.Photo))
	}
	return out
}

// TestAgentCoverSeat: у обложки с двумя местами фото встаёт в пустое; пустых нет — вместо кадра в наибольшем,
// прежний кадр уходит в ленту (в альбоме его нет); поле place = seat; тот же кадр повторно — холостая;
// каждый вызов — один шаг; круг Save→Load держит места.
func TestAgentCoverSeat(t *testing.T) {
	r := newEdRig34(t)
	if _, err := r.a.CreateCover("", ""); err != nil {
		t.Fatal(err)
	}
	boxes := []model.Box{{X: 0.05, Y: 0.05, W: 0.5, H: 0.5}, {X: 0.6, Y: 0.6, W: 0.3, H: 0.3}}
	if _, err := r.a.ApplyCoverGrid(model.ScopeRight, boxes); err != nil {
		t.Fatal(err)
	}
	if got := seatPhotos(r); len(got) != 2 || got[0] != "" || got[1] != "" {
		t.Fatalf("места обложки: %q", got)
	}
	for i := range 2 { // пустые места — по порядку, каждое — шаг
		json0 := projectJSON(t, r.a)
		out := r.ok(t, "set_cover_photo", map[string]any{"photo": r.photoID(i)})
		if out["place"] != "seat" || out["changed"] != true || out["created"] == true {
			t.Fatalf("фото %d: %v", i, out)
		}
		if filled := slices.DeleteFunc(seatPhotos(r), func(s string) bool { return s == "" }); len(filled) != i+1 {
			t.Fatalf("после фото %d занято мест %d", i, len(filled))
		}
		if i == 1 {
			continue
		}
		r.a.Undo()
		if projectJSON(t, r.a) != json0 {
			t.Fatal("один шаг отмены не вернул место")
		}
		r.ok(t, "set_cover_photo", map[string]any{"photo": r.photoID(0)})
	}
	// Пустых нет: заменяется кадр в наибольшем месте (первое по площади), прежний — в ленту.
	p, _ := r.a.editor().Snapshot()
	seats := p.CoverSeats()
	big := 0
	if seats[1].W*seats[1].H > seats[0].W*seats[0].H {
		big = 1
	}
	prev, other := seats[big].Photo, model.BaseID(seats[1-big].Photo) // prev — запись (у уже стоящего в альбоме фото это копия «id-N»)
	json1 := projectJSON(t, r.a)
	out := r.ok(t, "set_cover_photo", map[string]any{"photo": r.photoID(2)})
	if out["place"] != "seat" || out["changed"] != true {
		t.Fatalf("замена: %v", out)
	}
	got := seatPhotos(r)
	if got[big] != r.photoID(2) || got[1-big] != other {
		t.Fatalf("после замены места %q, ждали %s в месте %d и %s рядом", got, r.photoID(2), big, other)
	}
	if q, _ := r.a.editor().Snapshot(); q.Placed(prev) || len(q.Photos[prev].Path) > 0 {
		t.Fatalf("прежний кадр %s остался в альбоме: места %q", prev, seatPhotos(r))
	}
	if e := r.lastEdited(t); e.Text != "Фото на обложке" || e.Go != model.CoverID {
		t.Fatalf("тост: %+v", e)
	}
	// Тот же кадр ещё раз — ни шага, ни тоста.
	json2, rev, toasts := projectJSON(t, r.a), r.a.editor().View().Rev, len(r.of("agents:edited"))
	if again := r.ok(t, "set_cover_photo", map[string]any{"photo": r.photoID(2)}); again["changed"] != false || again["place"] != "seat" ||
		projectJSON(t, r.a) != json2 || r.a.editor().View().Rev != rev || len(r.of("agents:edited")) != toasts {
		t.Fatalf("повтор: %v", again)
	}
	r.a.Undo()
	if projectJSON(t, r.a) != json1 {
		t.Fatal("отмена замены вернула не прежний кадр")
	}
}

// TestAgentCoverWrap: у обложки wrap фото — фон через корешок (BgPhoto листа), а не правая сторонка; другое
// фото заменяет его, повтор — холостая.
func TestAgentCoverWrap(t *testing.T) {
	r := newEdRig34(t)
	if _, err := r.a.CreateCover("", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := r.a.SetCoverLayout(model.CoverWrap); err != nil {
		t.Fatal(err)
	}
	bg := func() *model.Backdrop {
		p, _ := r.a.editor().Snapshot()
		if s := p.Cover.Sheet; s.BgPhotoL != nil || s.BgPhotoR != nil || s.BgPhotoS != nil {
			t.Fatalf("фон не через корешок: L %v R %v S %v", s.BgPhotoL, s.BgPhotoR, s.BgPhotoS)
		}
		return p.Cover.Sheet.BgPhoto
	}
	json0 := projectJSON(t, r.a)
	for i := range 2 {
		out := r.ok(t, "set_cover_photo", map[string]any{"photo": r.photoID(i)})
		if out["place"] != "background" || out["changed"] != true {
			t.Fatalf("фото %d: %v", i, out)
		}
		if b := bg(); b == nil || model.BaseID(b.Photo) != r.photoID(i) {
			t.Fatalf("фон после фото %d: %+v", i, b)
		}
	}
	rev, toasts := r.a.editor().View().Rev, len(r.of("agents:edited"))
	if again := r.ok(t, "set_cover_photo", map[string]any{"photo": r.photoID(1)}); again["changed"] != false ||
		r.a.editor().View().Rev != rev || len(r.of("agents:edited")) != toasts {
		t.Fatalf("повтор: %v", again)
	}
	r.a.Undo()
	r.a.Undo()
	if projectJSON(t, r.a) != json0 {
		t.Fatal("два вызова — два шага отмены")
	}
}

// TestAgentCoverSheet: у альбома-листа обложки нет — отказ с понятным текстом, правки и тоста нет.
func TestAgentCoverSheet(t *testing.T) {
	r := newEdRig34(t)
	sheet := model.New() // лента фото остаётся: NewVignetteSheet её сбросил бы, и отказ дал бы errNoPhoto
	sheet.Settings.Sheet, sheet.Settings.PageW, sheet.Settings.PageH = true, 204, 304
	r.a.ed.Store(mustEditor(t, sheet, ""))
	json0, toasts := projectJSON(t, r.a), len(r.of("agents:edited"))
	if msg := r.fail(t, "set_cover_photo", map[string]any{"photo": r.photoID(0)}); msg != agents.ErrNoCover.Error() {
		t.Fatalf("отказ: %q", msg)
	}
	if projectJSON(t, r.a) != json0 || len(r.of("agents:edited")) != toasts {
		t.Fatal("правка или тост у альбома-листа")
	}
}

// TestAgentCoverStaleAlbum: альбом сменился (Editor закрыт) до правки — отказ, а не «created/changed: true»;
// ни тоста, ни записи для undo_last_edit (отмена «не своя»).
func TestAgentCoverStaleAlbum(t *testing.T) {
	r := newEdRig34(t)
	old := albumRef(t, r.call(t, "get_album", nil))
	r.a.NewProject("album")
	toasts := len(r.of("agents:edited"))
	res := r.call(t, "set_cover_photo", map[string]any{"album": old, "photo": r.photoID(0)})
	if !res.IsError {
		t.Fatalf("устаревший ключ принят: %s", agText(res))
	}
	if v := r.a.editor().View(); v.Cover != nil || len(r.of("agents:edited")) != toasts {
		t.Fatalf("обложка у нового альбома %v или тост", v.Cover)
	}
	if res := r.call(t, "undo_last_edit", map[string]any{"album": albumRef(t, r.call(t, "get_album", nil))}); !res.IsError {
		t.Fatalf("отмена после отказа: %s", agText(res))
	}
}

// TestAgentBgReplacesDecor: плитка набора или своя заменяется другой без replace (клиентские фото фона не в счёт);
// тот же фон повторно — без шага и тоста; переход в тосте — к первому листу, где вид правда изменился.
func TestAgentBgReplacesDecor(t *testing.T) {
	r := newEdRig34(t)
	items, _ := structured(t, r.call(t, "list_decor", map[string]any{"kind": "bg", "limit": 5}))["items"].([]any)
	if len(items) < 2 {
		t.Fatalf("фонов в каталоге %d", len(items))
	}
	a, b := items[0].(map[string]any)["id"].(string), items[1].(map[string]any)["id"].(string)
	sp := r.a.editor().View().Spreads
	r.ok(t, "set_background", map[string]any{"item": a, "spreads": []any{1}})
	json1 := projectJSON(t, r.a)
	out := r.ok(t, "set_background", map[string]any{"item": b, "spreads": []any{1}})
	if out["changed"] != float64(1) || out["skipped"] != nil || out["replaced"] == true || projectJSON(t, r.a) == json1 {
		t.Fatalf("замена плитки без replace: %v", out)
	}
	// Тот же фон ещё раз: ни шага, ни тоста.
	json2, rev, toasts := projectJSON(t, r.a), r.a.editor().View().Rev, len(r.of("agents:edited"))
	if again := r.ok(t, "set_background", map[string]any{"item": b, "spreads": []any{1}}); again["changed"] != float64(0) ||
		projectJSON(t, r.a) != json2 || r.a.editor().View().Rev != rev || len(r.of("agents:edited")) != toasts {
		t.Fatalf("повтор: %v", again)
	}
	// Свой фон тоже заменяется плиткой каталога без replace.
	own := importBg(t, r.a, r.photos[0])
	r.ok(t, "set_background", map[string]any{"item": own[0], "spreads": []any{1}})
	if out := r.ok(t, "set_background", map[string]any{"item": a, "spreads": []any{1}}); out["changed"] != float64(1) || out["skipped"] != nil {
		t.Fatalf("свой фон без replace: %v", out)
	}
	// Лист 1 уже с плиткой a, лист 2 — без: «перейти» ведёт к листу 2, где вид изменился.
	out = r.ok(t, "set_background", map[string]any{"item": a, "spreads": []any{1, 2}})
	if e := r.lastEdited(t); out["changed"] == float64(0) || e.Go != sp[1].ID {
		t.Fatalf("переход %q (ждали лист 2, %q), ответ %v", e.Go, sp[1].ID, out)
	}
}

// TestAgentMarksCopyReq: «id-new» (запрос копии) и пути — в skipped; отметка под ключом «-new» не пишется,
// отметка настоящего фото не затрагивается; пачка из одних негодных — без шага и тоста.
func TestAgentMarksCopyReq(t *testing.T) {
	r := newEdRig34(t)
	id, other := r.photoID(0), r.photoID(1)
	if _, err := r.a.SetMarks([]string{other}, model.MarkPatch{Label: new("blue")}); err != nil {
		t.Fatal(err)
	}
	out := r.ok(t, "set_marks", map[string]any{"ids": []any{id, id + "-new", other + "-new", "../" + id, "/etc/passwd"}, "label": "red"})
	if out["changed"] != float64(1) {
		t.Fatalf("ответ: %v", out)
	}
	sk, _ := out["skipped"].([]any)
	for _, want := range []string{id + "-new", other + "-new"} {
		if !slices.Contains(sk, any(want)) {
			t.Errorf("%s нет в skipped %v", want, sk)
		}
	}
	for k, m := range r.a.editor().Marks() {
		if strings.Contains(k, "-new") || strings.Contains(k, "/") {
			t.Fatalf("отметка под ключом %q: %+v", k, m)
		}
	}
	if m := r.a.editor().Marks()[other]; m.Label == nil || *m.Label != "blue" {
		t.Fatalf("отметка настоящего фото затёрта: %+v", m)
	}
	rev, toasts := r.a.editor().View().Rev, len(r.of("agents:edited"))
	out = r.ok(t, "set_marks", map[string]any{"ids": []any{other + "-new"}, "label": "red"})
	if out["changed"] != float64(0) || r.a.editor().View().Rev != rev || len(r.of("agents:edited")) != toasts {
		t.Fatalf("одни негодные: %v", out)
	}
	if m := r.a.editor().Marks()[other]; m.Label == nil || *m.Label != "blue" {
		t.Fatalf("отметка настоящего фото затёрта запросом копии: %+v", m)
	}
	// Фасад сам не принимает запрос копии, даже если служба его пропустила (второй рубеж).
	ref := albumRef(t, r.call(t, "get_album", nil))
	res, err := r.a.agentsHost().Mark(t.Context(), agents.MarkReq{Key: ref["key"].(string), Rev: int(ref["rev"].(float64)),
		IDs: []string{id + "-new"}, Label: "red"})
	if err != nil || res.Changed != 0 || !slices.Equal(res.Skipped, []string{id + "-new"}) {
		t.Fatalf("Host.Mark с запросом копии: %+v, %v", res, err)
	}
	for k := range r.a.editor().Marks() {
		if strings.Contains(k, "-new") {
			t.Fatalf("отметка под ключом %q", k)
		}
	}
}

// ghostCover — альбом с обложкой, чьё изделие принадлежит типографии, которой нет на этом ПК (файл с чужого
// компьютера после Load): id годной грамматики, реестр его не знает — model.CoverMissing.
func ghostCover(t *testing.T, r *edRig) {
	t.Helper()
	if _, err := r.a.CreateCover("", r.photoID(0)); err != nil {
		t.Fatal(err)
	}
	p, _ := r.a.editor().Snapshot()
	p.Cover.Spec.Preset = "ghostprint-album"
	r.a.ed.Store(mustEditor(t, p, ""))
	if got := model.CoverMissing(r.a.editor().View().Cover.Preset); got == "" {
		t.Fatalf("изделие %q не «отсутствующее»", r.a.editor().View().Cover.Preset)
	}
}
