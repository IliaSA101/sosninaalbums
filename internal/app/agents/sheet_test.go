package agents

import (
	"bytes"
	"context"
	"fmt"
	"image/jpeg"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// get_contact_sheet (151, Т3): выбор фото, страницы, ids, сетка, фильтры, уровень. Рисунок листа — фасад
// (internal/app/agents_sheet_test.go); здесь Host.Sheet подставной.

// manyPhotos добавляет n годных фото с возрастающим временем съёмки и возвращает их ID по порядку.
func manyPhotos(r *rig, n int) []string {
	r.fx.mu.Lock()
	defer r.fx.mu.Unlock()
	ids := make([]string, n)
	for i := range n {
		ids[i] = fmt.Sprintf("bbbb%012d", i)
		r.fx.photos = append(r.fx.photos, library.Photo{ID: ids[i], Name: fmt.Sprintf("ZQ1_many%03d.jpg", i), Path: fmt.Sprintf("/albums/ZQ2dir/m%d.jpg", i),
			W: 100, H: 100, Shot: 1_800_000_000 + int64(i)})
	}
	return ids
}

func sheetCells(t *testing.T, d map[string]any) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, c := range d["cells"].([]any) {
		out = append(out, c.(map[string]any))
	}
	return out
}

func cellIDs(cells []map[string]any) []string {
	var out []string
	for _, c := range cells {
		out = append(out, c["id"].(string))
	}
	return out
}

// TestContactSheetLevel: картинки только на уровне 3; ниже — отказ словами, Host не зван.
func TestContactSheetLevel(t *testing.T) {
	for _, lv := range []int{1, 2} {
		r, cs := editRig(t, Options{}, nil)
		setLevel(r.cfg, lv)
		failText(t, tcall(t, cs, "get_contact_sheet", nil), errNoImages)
		if n := r.fx.edit.nSheet.Load(); n != 0 {
			t.Fatalf("уровень %d: Host.Sheet вызван %d раз", lv, n)
		}
	}
	r, cs := editRig(t, Options{}, nil)
	setLevel(r.cfg, 3)
	res := pcall(t, cs, "get_contact_sheet", nil)
	if res.IsError || imageOf(res) == nil {
		t.Fatalf("уровень 3: %q", textOf(res))
	}
	setLevel(r.cfg, 2) // уровень берётся на каждый вызов
	failText(t, tcall(t, cs, "get_contact_sheet", nil), errNoImages)
	// Уровень сняли, пока шёл Host.Sheet: картинка не уходит (image читает его заново).
	setLevel(r.cfg, 3)
	r.fx.edit.setSheet(func(ctx context.Context, in SheetReq) ([]byte, []string, error) {
		setLevel(r.cfg, 1)
		b, err := testJPEG(in.Side, in.Side)
		return b, make([]string, len(in.IDs)), err
	})
	failText(t, tcall(t, cs, "get_contact_sheet", nil), errNoImages)
}

// TestContactSheetPage: листы по страницам выборки, номера сквозные, next ведёт к следующему листу.
func TestContactSheetPage(t *testing.T) {
	r, cs := editRig(t, Options{}, nil)
	setLevel(r.cfg, 3)
	extra := manyPhotos(r, 20) // 5 из fixture + 20: 25 фото, лист 3 × 3 = 9
	schemas := outSchemas(t, cs)
	var seen []string
	offset, pages := 0, 0
	for {
		args := map[string]any{"grid": 3, "size": 768}
		if offset > 0 {
			args["offset"] = offset
		}
		res := pcall(t, cs, "get_contact_sheet", args)
		if res.IsError || imageOf(res) == nil {
			t.Fatalf("offset %d: %q", offset, textOf(res))
		}
		_ = mustOK(t, cs, schemas, "get_contact_sheet", args)
		d := jsonOf(t, res)
		cells := sheetCells(t, d)
		got := r.fx.edit.gotSheet()
		if got.First != offset+1 || got.Grid != 3 || got.Side != 768 || len(got.IDs) != len(cells) {
			t.Fatalf("offset %d: запрос к Host %+v", offset, got)
		}
		if d["total"].(float64) != 25 || int(d["from"].(float64)) != offset+1 || int(d["to"].(float64)) != offset+len(cells) {
			t.Fatalf("offset %d: total/from/to %v", offset, d)
		}
		for i, c := range cells {
			if int(c["n"].(float64)) != offset+i+1 {
				t.Fatalf("номер клетки %d на листе с offset %d: %v", i, offset, c["n"])
			}
			seen = append(seen, c["id"].(string))
		}
		pages++
		next, _ := d["next"].(float64)
		if next == 0 {
			break
		}
		if int(next) != offset+len(cells) {
			t.Fatalf("next %v после offset %d и %d клеток", next, offset, len(cells))
		}
		offset = int(next)
	}
	if pages != 3 || len(seen) != 25 {
		t.Fatalf("листов %d, клеток %d: ждали 3 листа и 25 клеток", pages, len(seen))
	}
	// Порядок list_photos: сначала по времени съёмки, фото без времени в конце.
	if seen[0] != idA || seen[1] != idB || seen[2] != extra[0] || seen[len(seen)-1] != idE {
		t.Fatalf("порядок выборки: %v", seen)
	}
	// Дальше конца и пустая выборка — отказ словами, Host не зван.
	before := r.fx.edit.nSheet.Load()
	failText(t, tcall(t, cs, "get_contact_sheet", map[string]any{"offset": 25}), errBadPage)
	failText(t, tcall(t, cs, "get_contact_sheet", map[string]any{"filter": "marked", "folder": 9}), errBadPage)
	if r.fx.edit.nSheet.Load() != before {
		t.Fatal("отказ дошёл до Host.Sheet")
	}
	// По умолчанию 6 × 6 на 1056.
	okReply(t, tcall(t, cs, "get_contact_sheet", nil))
	if got := r.fx.edit.gotSheet(); got.Grid != 6 || got.Side != 1056 || len(got.IDs) != 25 {
		t.Fatalf("умолчания: %+v", got)
	}
}

// TestContactSheetIDs: лист из названных фото — в их порядке, без повторов; пропавшее — «×»; чужой id и
// смесь с фильтрами — отказ.
func TestContactSheetIDs(t *testing.T) {
	r, cs := editRig(t, Options{}, nil)
	setLevel(r.cfg, 3)
	copyOfB := idB + "-2" // запись-копия на развороте: основа — idB
	d := okReply(t, tcall(t, cs, "get_contact_sheet", map[string]any{"ids": []any{idB, idA, copyOfB, idC, idB}})).Data
	cells := sheetCells(t, d)
	if !reflect.DeepEqual(cellIDs(cells), []string{idB, idA, idC}) {
		t.Fatalf("клетки: %v", cellIDs(cells))
	}
	got := r.fx.edit.gotSheet()
	if got.First != 1 || !reflect.DeepEqual(got.IDs, []string{idB, idA, ""}) {
		t.Fatalf("Host.Sheet: %+v (пропавшее должно идти пустым ID)", got)
	}
	if d["total"].(float64) != 3 || d["from"].(float64) != 1 || d["to"].(float64) != 3 || d["next"].(float64) != 0 {
		t.Fatalf("total/from/to/next: %v", d)
	}
	if cells[2]["status"] != "missing" || !reflect.DeepEqual(d["broken"], []any{3.0}) {
		t.Fatalf("пропавшее фото: клетка %v, broken %v", cells[2], d["broken"])
	}
	// Отметки в таблице: idA — оценка из отметок человека (5), стоит на развороте; idB стоит на обложке.
	if cells[1]["used"] != true || cells[1]["rating"].(float64) != 5 || cells[0]["used"] != true || cells[0]["rating"] != nil {
		t.Fatalf("отметки клеток: %v", cells)
	}
	// Пропавшее вне ленты (только в списке Status) — тоже клетка «×».
	r.fx.mu.Lock()
	r.fx.missing = append(r.fx.missing, "cccc000000000009")
	r.fx.mu.Unlock()
	d = okReply(t, tcall(t, cs, "get_contact_sheet", map[string]any{"ids": []any{"cccc000000000009"}})).Data
	if !reflect.DeepEqual(d["broken"], []any{1.0}) {
		t.Fatalf("пропавшее вне ленты: %v", d)
	}
	before := r.fx.edit.nSheet.Load()
	failText(t, tcall(t, cs, "get_contact_sheet", map[string]any{"ids": []any{idA, "ffff"}}), errNoPhoto)
	for _, extra := range []map[string]any{{"filter": "used"}, {"folder": 1}, {"bin": "b1"}, {"order": "name"}, {"offset": 1}} {
		args := map[string]any{"ids": []any{idA}}
		for k, v := range extra {
			args[k] = v
		}
		if rep := tcall(t, cs, "get_contact_sheet", args); !rep.IsError || !strings.HasPrefix(rep.Text, "неверные аргументы вызова: ") {
			t.Errorf("ids вместе с %v: %+v", extra, rep)
		}
	}
	// Схема: 0 и 65 id, пустой id, длинный id.
	for _, ids := range [][]any{{}, make([]any, 65), {""}, {strings.Repeat("a", 65)}} {
		for i := range ids {
			if ids[i] == nil {
				ids[i] = idA
			}
		}
		if rep := tcall(t, cs, "get_contact_sheet", map[string]any{"ids": ids}); !rep.IsError {
			t.Errorf("ids из %d: ждали отказ схемы", len(ids))
		}
	}
	if r.fx.edit.nSheet.Load() != before {
		t.Fatal("отказ дошёл до Host.Sheet")
	}
}

// TestContactSheetIDsGrid: сетка для ids — ⌈√n⌉ в пределах 3…8; заданная меньше n — отказ.
func TestContactSheetIDsGrid(t *testing.T) {
	r, cs := editRig(t, Options{}, nil)
	setLevel(r.cfg, 3)
	extra := manyPhotos(r, 64)
	for _, c := range []struct{ n, grid int }{{1, 3}, {9, 3}, {10, 4}, {16, 4}, {17, 5}, {37, 7}, {49, 7}, {50, 8}, {64, 8}} {
		ids := make([]any, c.n)
		for i := range ids {
			ids[i] = extra[i]
		}
		okReply(t, tcall(t, cs, "get_contact_sheet", map[string]any{"ids": ids}))
		if got := r.fx.edit.gotSheet(); got.Grid != c.grid || len(got.IDs) != c.n {
			t.Errorf("%d фото: сетка %d, ждали %d", c.n, got.Grid, c.grid)
		}
	}
	ids := make([]any, 10)
	for i := range ids {
		ids[i] = extra[i]
	}
	before := r.fx.edit.nSheet.Load()
	failText(t, tcall(t, cs, "get_contact_sheet", map[string]any{"ids": ids, "grid": 3}), errBadPage)
	if r.fx.edit.nSheet.Load() != before {
		t.Fatal("отказ дошёл до Host.Sheet")
	}
	okReply(t, tcall(t, cs, "get_contact_sheet", map[string]any{"ids": ids, "grid": 8}))
	if got := r.fx.edit.gotSheet(); got.Grid != 8 || len(got.IDs) != 10 {
		t.Fatalf("заданная сетка: %+v", got)
	}
	// Повторы не считаются: 70 строк из 20 разных фото — 20 клеток, сетка 5.
	dup := make([]any, 0, 64)
	for i := range 64 {
		dup = append(dup, extra[i%20])
	}
	okReply(t, tcall(t, cs, "get_contact_sheet", map[string]any{"ids": dup}))
	if got := r.fx.edit.gotSheet(); got.Grid != 5 || len(got.IDs) != 20 {
		t.Fatalf("повторы: %+v", got)
	}
}

// TestContactSheetFilters: фильтры как у list_photos, но без missing и unreadable.
func TestContactSheetFilters(t *testing.T) {
	r, cs := editRig(t, Options{}, nil)
	setLevel(r.cfg, 3)
	ids := func(args map[string]any) []string {
		t.Helper()
		okReply(t, tcall(t, cs, "get_contact_sheet", args))
		return r.fx.edit.gotSheet().IDs
	}
	list := func(args map[string]any) []string { // та же выборка у list_photos
		t.Helper()
		var out []string
		for _, it := range okReply(t, tcall(t, cs, "list_photos", args)).Data["items"].([]any) {
			out = append(out, it.(map[string]any)["id"].(string))
		}
		return out
	}
	for _, f := range []string{"all", "used", "unused", "marked"} {
		args := map[string]any{"filter": f}
		want := list(args)
		got := ids(args)
		// Пропавшие и нечитаемые идут в лист пустым ID («×»): сверяем остальное.
		var kept []string
		for i, id := range got {
			if id != "" {
				kept = append(kept, id)
			} else if want[i] != idC && want[i] != idD {
				t.Errorf("%s: пустой ID на месте %s", f, want[i])
			}
		}
		if len(got) != len(want) {
			t.Errorf("%s: %d клеток, у list_photos %d фото", f, len(got), len(want))
		}
		for _, id := range kept {
			if !contains(want, id) {
				t.Errorf("%s: лишнее фото %s", f, id)
			}
		}
	}
	// unused не берёт ни стоящее на развороте и обложке, ни фото фона; used — только стоящее.
	free := manyPhotos(r, 3)
	got := ids(map[string]any{"filter": "unused"})
	if contains(got, idA) || contains(got, idB) || contains(got, idE) || !contains(got, free[0]) || !contains(got, free[2]) {
		t.Errorf("unused: %v", got)
	}
	if got := ids(map[string]any{"filter": "used"}); !reflect.DeepEqual(got, []string{idA, idB}) {
		t.Errorf("used: %v", got)
	}
	if got := ids(map[string]any{"filter": "marked"}); !contains(got, idA) || contains(got, idB) || contains(got, free[0]) {
		t.Errorf("marked: %v", got)
	}
	if got := ids(map[string]any{"bin": "b1"}); !reflect.DeepEqual(got, []string{idA}) {
		t.Errorf("bin: %v", got)
	}
	if got := ids(map[string]any{"folder": 1}); !contains(got, idA) || contains(got, idE) {
		t.Errorf("folder: %v", got)
	}
	// order=name на уровне 3 (имена доступны) — по имени без регистра; shot — по времени.
	if got := ids(map[string]any{"order": "name", "filter": "unused"}); len(got) < 2 {
		t.Errorf("order name: %v", got)
	}
}

// TestContactSheetFilterArgs: missing и unreadable лист не берёт (лист вышел бы из одних заглушек); прочие
// поля входа держат пределы схемы.
func TestContactSheetFilterArgs(t *testing.T) {
	r, cs := editRig(t, Options{}, nil)
	setLevel(r.cfg, 3)
	before := r.fx.edit.nSheet.Load()
	for _, f := range []string{"missing", "unreadable", "bogus"} {
		if rep := tcall(t, cs, "get_contact_sheet", map[string]any{"filter": f}); !rep.IsError || !strings.HasPrefix(rep.Text, "неверные аргументы вызова: ") {
			t.Errorf("filter %s: %+v", f, rep)
		}
	}
	if r.fx.edit.nSheet.Load() != before {
		t.Fatal("отказ схемы дошёл до Host.Sheet")
	}
	// Все остальные поля входа — в пределах схемы.
	for _, bad := range []map[string]any{{"grid": 2}, {"grid": 9}, {"size": 767}, {"size": 1569}, {"offset": -1}, {"folder": 0}, {"order": "x"}, {"bogus": 1}} {
		if rep := tcall(t, cs, "get_contact_sheet", bad); !rep.IsError {
			t.Errorf("вход %v принят", bad)
		}
	}
}

func contains(l []string, s string) bool {
	for _, x := range l {
		if x == s {
			return true
		}
	}
	return false
}

// TestContactSheetStates: состояния клеток от Host (pending, broken, чужое слово) попадают в ответ; пропавшие
// и нечитаемые по Status служба метит сама, не отдавая Host их ID.
func TestContactSheetStates(t *testing.T) {
	r, cs := editRig(t, Options{}, nil)
	setLevel(r.cfg, 3)
	r.fx.edit.setSheet(func(_ context.Context, in SheetReq) ([]byte, []string, error) {
		b, err := testJPEG(in.Side, in.Side)
		st := make([]string, len(in.IDs))
		for i := range st {
			st[i] = CellOK
		}
		st[1], st[2] = CellPending, "что-то-новое"
		return b, st, err
	})
	d := okReply(t, tcall(t, cs, "get_contact_sheet", map[string]any{"ids": []any{idA, idB, idE, idC, idD}})).Data
	if !reflect.DeepEqual(d["pending"], []any{2.0}) || !reflect.DeepEqual(d["broken"], []any{3.0, 4.0, 5.0}) {
		t.Fatalf("pending/broken: %v / %v", d["pending"], d["broken"])
	}
	if got := r.fx.edit.gotSheet().IDs; !reflect.DeepEqual(got, []string{idA, idB, idE, "", ""}) {
		t.Fatalf("в Host ушли ID: %q", got)
	}
	cells := sheetCells(t, d)
	if cells[3]["status"] != "missing" || cells[4]["status"] != "unreadable" || cells[0]["status"] != nil {
		t.Fatalf("status клеток: %v", cells)
	}
	// Host ответил не по клетке на фото — внутренняя ошибка, картинка не уходит.
	r.fx.edit.setSheet(func(_ context.Context, in SheetReq) ([]byte, []string, error) {
		b, err := testJPEG(in.Side, in.Side)
		return b, []string{CellOK}, err
	})
	if rep := tcall(t, cs, "get_contact_sheet", map[string]any{"ids": []any{idA, idB}}); !rep.IsError || rep.Text != errInternal.Error() {
		t.Fatalf("лишние состояния: %+v", rep)
	}
}

// TestContactSheetJPEG: ответ Host проходит через кодировщик службы — метаданные, хвост и не-JPEG
// наружу не идут; размер не больше заказанного.
func TestContactSheetJPEG(t *testing.T) {
	r, cs := editRig(t, Options{}, nil)
	setLevel(r.cfg, 3)
	const secret = "ZQSECRETEXIF"
	r.fx.edit.setSheet(func(_ context.Context, in SheetReq) ([]byte, []string, error) {
		b, err := testJPEG(in.Side, in.Side)
		com := append([]byte{0xFF, 0xFE, 0, byte(len(secret) + 2)}, secret...)
		b = append(append(append([]byte{}, b[:2]...), com...), b[2:]...)
		return append(b, []byte("ZQTAIL")...), make([]string, len(in.IDs)), err
	})
	res := pcall(t, cs, "get_contact_sheet", map[string]any{"size": 800})
	img := imageOf(res)
	if img == nil {
		t.Fatalf("нет картинки: %q", textOf(res))
	}
	if bytes.Contains(img, []byte(secret)) || bytes.Contains(img, []byte("ZQTAIL")) {
		t.Fatal("метаданные или хвост Host дошли до агента")
	}
	cleanOutput(t, img)
	cfg, err := jpeg.DecodeConfig(bytes.NewReader(img))
	d := jsonOf(t, res)
	if err != nil || cfg.Width != 800 || d["w"].(float64) != 800 || d["h"].(float64) != 800 {
		t.Fatalf("размер: %v %+v, ответ %v×%v", err, cfg, d["w"], d["h"])
	}
	for name, fake := range map[string]func(SheetReq) ([]byte, error){
		"не JPEG":        func(SheetReq) ([]byte, error) { return []byte("не картинка"), nil },
		"крупнее заказа": func(in SheetReq) ([]byte, error) { return testJPEG(in.Side+100, in.Side+100) },
		"пусто":          func(SheetReq) ([]byte, error) { return nil, nil },
		"больше предела": func(SheetReq) ([]byte, error) { return make([]byte, maxJPEG+1), nil },
	} {
		r.fx.edit.setSheet(func(_ context.Context, in SheetReq) ([]byte, []string, error) {
			b, err := fake(in)
			return b, make([]string, len(in.IDs)), err
		})
		rep := tcall(t, cs, "get_contact_sheet", map[string]any{"size": 800})
		// Плохой ответ Host — отказ без картинки (внутренняя ошибка в log.txt, не текст Host).
		if !rep.IsError || name == "больше предела" && rep.Text != errTooBigOne.Error() {
			t.Errorf("%s: %+v", name, rep)
		}
	}
}

// TestContactSheetTimeout: Host не уложился в срок вызова — «время вышло», место и рендер свободны.
func TestContactSheetTimeout(t *testing.T) {
	r, cs := editRig(t, Options{CallTimeout: testkit.Limit(250 * time.Millisecond)}, nil)
	setLevel(r.cfg, 3)
	release := make(chan struct{})
	r.fx.edit.setSheet(func(ctx context.Context, in SheetReq) ([]byte, []string, error) {
		select {
		case <-ctx.Done():
		case <-release:
		}
		return nil, nil, ctx.Err()
	})
	failText(t, tcall(t, cs, "get_contact_sheet", nil), errTimeout)
	close(release)
	r.fx.edit.setSheet(nil)
	waitFor(t, "лист снова доступен", func() bool { // зависшее тело отпускает рендер по своему концу
		rep, err := tcallG(cs, "get_contact_sheet", nil)
		return err == nil && !rep.IsError
	})
}

// TestContactSheetBusy: лист и рендер разворота идут по одному — второй отвечает «занято».
func TestContactSheetBusy(t *testing.T) {
	r, cs := editRig(t, Options{}, nil)
	setLevel(r.cfg, 3)
	in, release := make(chan struct{}), make(chan struct{})
	r.fx.edit.setSheet(func(ctx context.Context, req SheetReq) ([]byte, []string, error) {
		close(in)
		<-release
		b, err := testJPEG(req.Side, req.Side)
		return b, make([]string, len(req.IDs)), err
	})
	done := make(chan reply, 1)
	go func() {
		rep, err := tcallG(cs, "get_contact_sheet", nil)
		if err != nil {
			rep = reply{IsError: true, Text: err.Error()}
		}
		done <- rep
	}()
	<-in
	failText(t, tcall(t, cs, "get_spread_preview", map[string]any{"spread": 1}), errCallBusy)
	failText(t, tcall(t, cs, "get_contact_sheet", nil), errCallBusy)
	close(release)
	if rep := <-done; rep.IsError {
		t.Fatalf("первый лист: %q", rep.Text)
	}
}

// toolNames — имена инструментов службы над Host, который собирает mutate из полного набора fixture.
func toolNames(t *testing.T, mutate func(*Host)) map[string]bool {
	t.Helper()
	r := newRig(t, Options{}, "")
	setLevel(r.cfg, 1)
	h := r.fx.host(r.cfg, r.sock, func() {})
	mutate(&h)
	r.svc = New(h, Options{})
	t.Cleanup(r.svc.Close)
	r.start(t)
	res, err := client(t, r.sock, "x").ListTools(callCtx(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]bool{}
	for _, tl := range res.Tools {
		out[tl.Name] = true
	}
	return out
}

// TestContactSheetHostOptional: get_contact_sheet появляется от одного Host.Sheet, правящие инструменты 0.34 —
// только когда заполнены правки 149 и все семь замыканий 3б.
func TestContactSheetHostOptional(t *testing.T) {
	edit34 := []string{"set_marks", "set_cover_photo", "set_background", "add_sticker", "apply_print"}
	got := toolNames(t, func(h *Host) { h.Undo = nil }) // нет правок 149
	if !got["get_contact_sheet"] || len(got) != 12 {
		t.Errorf("без правок 149 ждали 11 чтения и лист, получили %d: %v", len(got), got)
	}
	got = toolNames(t, func(h *Host) { h.Sheet = nil })
	if got["get_contact_sheet"] || !got["autobuild"] || got["set_marks"] || len(got) != 18 {
		t.Errorf("без Sheet: %d инструментов: %v", len(got), got)
	}
	for name, drop := range map[string]func(*Host){"Families": func(h *Host) { h.Families = nil }, "Mark": func(h *Host) { h.Mark = nil },
		"Cover": func(h *Host) { h.Cover = nil }, "Bg": func(h *Host) { h.Bg = nil }, "Stickers": func(h *Host) { h.Stickers = nil },
		"Print": func(h *Host) { h.Print = nil }} {
		got := toolNames(t, drop)
		if !got["get_contact_sheet"] || !got["autobuild"] {
			t.Errorf("без %s: пропали лист или правки 149", name)
		}
		for _, n := range edit34 {
			if got[n] {
				t.Errorf("без %s есть %s", name, n)
			}
		}
	}
	h := newFixture().host("", "", func() {})
	if !h.canEdit34() || len(toolNames(t, func(*Host) {})) != 24 {
		t.Fatal("полный набор не даёт 24 инструмента")
	}
}
