package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/decor"
)

// «Сохранить альбом как макет…» (131 G6b): App.SaveLook — снимок альбома в свои макеты, круг
// «сохранил → применил к новому альбому», края и неизменность проекта.

// saveSeries — альбом по макету из пяти разворотов: титул и финал со своим цветом, три повторяемых
// одинаковых; на титуле надпись. Возвращает ID разворотов в порядке альбома.
func saveSeries(t *testing.T, a *App) []string {
	t.Helper()
	id := saveLook(t, a, "Исходный", "#ffffff")
	r, err := a.ApplyLook(id, model.LookOpts{Mode: "series", Spreads: 5})
	if err != nil || len(r.View.Spreads) != 5 {
		t.Fatalf("исходный альбом: %v, %d разворотов", err, len(r.View.Spreads))
	}
	ed := a.editor()
	ids := make([]string, len(r.View.Spreads))
	for i, sp := range r.View.Spreads {
		ids[i] = sp.ID
	}
	ed.SetBackground(ids[0], model.ScopeLeft, "#111111")
	ed.SetBackground(ids[4], model.ScopeRight, "#222222")
	if _, err := ed.AddText(ids[0], 0, ""); err != nil {
		t.Fatal(err)
	}
	v := ed.View()
	if len(v.Spreads[0].Texts) != 1 {
		t.Fatalf("надпись титула не добавилась")
	}
	ed.SetText(v.Spreads[0].Texts[0].ID, "Анна и Михаил", "")
	return ids
}

func fullOpts(ids []string) model.SaveLookOpts {
	return model.SaveLookOpts{Theme: "mine", Name: "Мой макет", Ids: ids, RepFrom: 2, RepTo: len(ids) - 1, Places: true, Drafts: true, Cover: true}
}

// rects — места разворотов проекта в мм, по порядку.
func rects(p *model.Project) [][]model.Box {
	out := make([][]model.Box, len(p.Spreads))
	for i, s := range p.Spreads {
		for _, pl := range s.Grid {
			out[i] = append(out[i], model.Box{X: pl.X, Y: pl.Y, W: pl.W, H: pl.H})
		}
	}
	return out
}

// TestSaveLookRoundTrip: сохранил → открыл из библиотеки → применил к новому альбому: те же места,
// цвета и заготовка; три одинаковых повтора слились в один шаг, исходный альбом цел.
func TestSaveLookRoundTrip(t *testing.T) {
	a := lookApp(t)
	ids := saveSeries(t, a)
	src, _ := a.editor().Snapshot()
	before := projectJSON(t, a)
	rev := a.editor().View().Rev

	it, err := a.SaveLook(fullOpts(ids))
	if err != nil {
		t.Fatal(err)
	}
	if it.Name != "Мой макет" || it.Theme != "mine" || it.Steps != 3 || !strings.HasPrefix(it.ID, "my:") || it.From != 6 {
		t.Fatalf("плитка: %+v", it)
	}
	if projectJSON(t, a) != before || a.editor().View().Rev != rev {
		t.Fatal("SaveLook изменил альбом")
	}
	info, err := a.LookInfo(it.ID)
	if err != nil || len(info.Steps) != 3 || info.Steps[1].Role != "repeat" || info.Steps[0].Role != "" || info.Steps[1].Places != 2 || info.Min != 3 {
		t.Fatalf("LookInfo: %+v %v", info, err)
	}

	// Новый альбом из трёх пустых разворотов, серия из пяти — как у исходного.
	p := model.New()
	p.Spreads = []*model.Spread{{ID: "n1"}, {ID: "n2"}, {ID: "n3"}}
	a.setEditor(mustEditor(t, p, ""))
	r, err := a.ApplyLook(it.ID, model.LookOpts{Mode: "series", Spreads: 5})
	if err != nil || len(r.View.Spreads) != 5 || r.Plan.Drafts != 1 {
		t.Fatalf("ApplyLook: %d, %+v, %v", len(r.View.Spreads), r.Plan, err)
	}
	dst, _ := a.editor().Snapshot()
	want, got := rects(src), rects(dst)
	for i := range want {
		if len(want[i]) != len(got[i]) {
			t.Fatalf("разворот %d: мест %d, было %d", i+1, len(got[i]), len(want[i]))
		}
		for j, w := range want[i] {
			g := got[i][j]
			if math.Abs(g.X-w.X) > 0.05 || math.Abs(g.Y-w.Y) > 0.05 || math.Abs(g.W-w.W) > 0.05 || math.Abs(g.H-w.H) > 0.05 {
				t.Errorf("разворот %d место %d: %+v, было %+v", i+1, j+1, g, w)
			}
		}
		if dst.Spreads[i].BgL != src.Spreads[i].BgL || dst.Spreads[i].BgR != src.Spreads[i].BgR {
			t.Errorf("разворот %d: цвета %q/%q, были %q/%q", i+1, dst.Spreads[i].BgL, dst.Spreads[i].BgR, src.Spreads[i].BgL, src.Spreads[i].BgR)
		}
	}
	if tx := dst.Spreads[0].Texts; len(tx) != 1 || !tx[0].Draft || tx[0].Text != "Анна и Михаил" {
		t.Errorf("заготовка титула: %+v", tx)
	}
	// Макет пережил закрытие хранилища: ключ тот же, файл читается заново.
	s, err := a.decorStore()
	if err != nil {
		t.Fatal(err)
	}
	if tpl, err := s.Look(it.ID); err != nil || len(tpl.Steps) != 3 || tpl.Steps[1].Role != model.RoleRepeat {
		t.Fatalf("макет из библиотеки: %v", err)
	}
	// Тот же макет во второй раз — «уже есть», копии нет.
	a.setEditor(mustEditor(t, src, ""))
	if _, err := a.SaveLook(fullOpts(ids)); err == nil {
		t.Error("тот же макет сохранён дважды")
	}
	if n := countLooks(t, a); n != 2 { // «Исходный» из saveLook и «Мой макет»
		t.Errorf("макетов в библиотеке %d", n)
	}
}

func countLooks(t *testing.T, a *App) int {
	t.Helper()
	cat, err := a.DecorCatalog(decor.KindAlbum)
	if err != nil {
		t.Fatal(err)
	}
	return len(cat.Items)
}

// TestSaveLookEdges: один разворот, повторяемых нет, нет обложки, неизвестная тема, пустое имя,
// чужие id, 25 разных, лист 031 — ошибка без правки.
func TestSaveLookEdges(t *testing.T) {
	a := lookApp(t)
	ids := saveSeries(t, a)
	before := projectJSON(t, a)

	// Один разворот без повторяемых (RepFrom > RepTo), обложки в альбоме нет, тема неизвестная → «Мои».
	one := model.SaveLookOpts{Theme: "несуществующая", Name: "Один", Ids: ids[:1], RepFrom: 1, RepTo: 0, Places: true, Cover: true}
	it, err := a.SaveLook(one)
	if err != nil || it.Steps != 1 || it.Theme != decor.ThemeMine || it.Cover {
		t.Fatalf("один разворот: %+v %v", it, err)
	}
	info, err := a.LookInfo(it.ID)
	if err != nil || info.Min != 1 || info.Max != 1 || info.Steps[0].Role != "" {
		t.Fatalf("серия без повтора: %+v %v", info, err)
	}

	// Огромное имя режется до предела названия, а не ломает запись.
	long := model.SaveLookOpts{Name: strings.Repeat("Я", 100_000), Ids: ids[:2], Places: true}
	if it, err = a.SaveLook(long); err != nil || len([]rune(it.Name)) > 60 {
		t.Fatalf("длинное имя: %d рун, %v", len([]rune(it.Name)), err)
	}

	bad := map[string]model.SaveLookOpts{
		"пустое имя":    {Name: "  ", Ids: ids, Places: true},
		"нет id":        {Name: "x", Places: true},
		"чужой id":      {Name: "x", Ids: []string{"нет такого"}, Places: true},
		"обложка в ids": {Name: "x", Ids: []string{model.CoverID}, Places: true},
		"дубль id":      {Name: "x", Ids: []string{ids[0], ids[0]}, Places: true},
	}
	for name, o := range bad {
		if _, err := a.SaveLook(o); err == nil {
			t.Errorf("%s: ошибки нет", name)
		}
	}
	if !errors.Is(func() error { _, err := a.SaveLook(bad["пустое имя"]); return err }(), decor.ErrName) {
		t.Error("пустое имя: не ErrName")
	}
	if projectJSON(t, a) != before {
		t.Error("ошибочные SaveLook изменили альбом")
	}

	// Пустой разворот без мест и оформления снять нечего.
	a.setEditor(mustEditor(t, func() *model.Project { p := model.New(); p.Spreads = []*model.Spread{{ID: "e1"}}; return p }(), ""))
	if _, err := a.SaveLook(model.SaveLookOpts{Name: "Пустой", Ids: []string{"e1"}, Places: true, Drafts: true}); err == nil || !strings.Contains(err.Error(), "нечего сохранять") {
		t.Errorf("пустой разворот: %v", err)
	}

	// 25 разных разворотов — ошибка для тоста, 24 — макет.
	p := model.New()
	for i := range 25 {
		p.Spreads = append(p.Spreads, &model.Spread{ID: fmt.Sprintf("m%02d", i), BgL: fmt.Sprintf("#0000%02x", i+1)})
	}
	a.setEditor(mustEditor(t, p, ""))
	all := make([]string, len(p.Spreads))
	for i, s := range p.Spreads {
		all[i] = s.ID
	}
	_, err = a.SaveLook(model.SaveLookOpts{Name: "Много", Ids: all})
	if err == nil || err.Error() != "Не больше 24 разных разворотов — выделите нужные в ленте" {
		t.Errorf("25 разных: %v", err)
	}
	if it, err = a.SaveLook(model.SaveLookOpts{Name: "Двадцать четыре", Ids: all[:24]}); err != nil || it.Steps != 24 {
		t.Errorf("24 разных: %+v %v", it, err)
	}
	// 25 одинаковых повторяемых сливаются в один шаг.
	same := model.New()
	for i := range 25 {
		same.Spreads = append(same.Spreads, &model.Spread{ID: fmt.Sprintf("q%02d", i), BgL: "#123456"})
	}
	a.setEditor(mustEditor(t, same, ""))
	all = all[:0]
	for _, s := range same.Spreads {
		all = append(all, s.ID)
	}
	if it, err = a.SaveLook(model.SaveLookOpts{Name: "Одинаковые", Ids: all, RepFrom: 1, RepTo: 25}); err != nil || it.Steps != 1 {
		t.Errorf("25 одинаковых повторяемых: %+v %v", it, err)
	}
}

// TestSaveLookCover: обложка альбома входит в макет при флажке и не входит без него.
func TestSaveLookCover(t *testing.T) {
	a := lookApp(t)
	ed := a.editor()
	if _, err := ed.CreateCover(model.CoverDefault, nil); err != nil {
		t.Fatal(err)
	}
	ed.SetBackground(model.CoverID, model.ScopeLeft, "#334455")
	ed.SetBackground("s1", model.ScopeLeft, "#667788")
	before := projectJSON(t, a)
	o := model.SaveLookOpts{Name: "С обложкой", Ids: []string{"s1"}, Cover: true}
	it, err := a.SaveLook(o)
	if err != nil || !it.Cover {
		t.Fatalf("с обложкой: %+v %v", it, err)
	}
	s, _ := a.decorStore()
	tpl, err := s.Look(it.ID)
	if err != nil || tpl.Cover == nil || tpl.Cover.Back == nil || tpl.Cover.Back.Page[0].Color != "#334455" {
		t.Fatalf("обложка макета: %+v %v", tpl.Cover, err)
	}
	o.Name, o.Cover = "Без обложки", false
	if it, err = a.SaveLook(o); err != nil || it.Cover || it.ID == "" {
		t.Fatalf("без обложки: %+v %v", it, err)
	}
	if projectJSON(t, a) != before {
		t.Error("SaveLook изменил альбом")
	}
}

// TestSaveLookSheet031: лист для печати — errLookSheet, библиотека не пополняется.
func TestSaveLookSheet031(t *testing.T) {
	a := lookApp(t)
	a.setEditor(mustEditor(t, album.NewVignetteSheet(300, 400), ""))
	n := countLooks(t, a)
	ids := []string{a.editor().View().Spreads[0].ID}
	if _, err := a.SaveLook(model.SaveLookOpts{Name: "Лист", Ids: ids, Places: true}); !errors.Is(err, errLookSheet) {
		t.Errorf("лист 031: %v", err)
	}
	if countLooks(t, a) != n {
		t.Error("макет сохранён из листа 031")
	}
}

// lookFile — байты файла макета key в библиотеке (как лежат на диске).
func lookFile(t *testing.T, s *decor.Store, key string) []byte {
	t.Helper()
	name := strings.TrimPrefix(key, "my:") + ".json"
	var data []byte
	_ = filepath.WalkDir(s.Root(), func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && d.Name() == name {
			data, _ = os.ReadFile(p)
		}
		return nil
	})
	if len(data) == 0 {
		t.Fatalf("файла макета %s нет", key)
	}
	return data
}

// TestSaveLookNoClientData: путь тестового фото, имя на табличке места и код ссылки клиента (QR) в файл
// макета не попадают; обычный стикер без ключа остаётся.
func TestSaveLookNoClientData(t *testing.T) {
	a := lookApp(t)
	ids := saveSeries(t, a)
	ed := a.editor()
	const secret = "/photos/секретная-свадьба/IMG_9999.jpg"
	if _, err := ed.AddPhotos(ids[1], []model.PhotoInfo{{ID: "ph-secret", Path: secret, W: 300, H: 200}}, 0); err != nil {
		t.Fatal(err)
	}
	v := ed.View()
	place := v.Spreads[1].Places[0].ID
	ed.SetPlacePerson(ids[1], place, model.Person{Last: "Иванов-секрет", First: "Пётр"}, "")
	if _, _, err := func() (model.View, string, error) {
		return ed.AddSticker(ids[2], 0, model.StickerSrc{Art: &model.Art{Kind: "star"}})
	}(); err != nil {
		t.Fatal(err)
	}
	qr, err := a.AddQRSticker(ids[2], 1, "https://secret.example/client-album")
	if err != nil {
		t.Fatal(err)
	}
	s, _ := a.decorStore()
	qrKey := ownStickers(t, a)[0].ID
	if !s.IsQR(qrKey) || qr.ID == "" {
		t.Fatalf("QR не распознан: %q", qrKey)
	}
	o := fullOpts(ids)
	o.RepFrom, o.RepTo = 0, -1 // каждый разворот — шаг, слияния нет: стикеры третьего видны
	it, err := a.SaveLook(o)
	if err != nil {
		t.Fatal(err)
	}
	data := lookFile(t, s, it.ID)
	for _, bad := range []string{"IMG_9999", "секретная", "Иванов", "Пётр", qrKey, strings.TrimPrefix(qrKey, "my:"), "ph-secret", "secret.example", `"person"`, `"photo"`} {
		if bytes.Contains(data, []byte(bad)) {
			t.Errorf("в файле макета %q", bad)
		}
	}
	tpl, err := s.Look(it.ID)
	if err != nil || len(tpl.Steps) != 5 || len(tpl.Steps[2].Tpl.Stickers) != 1 || tpl.Steps[2].Tpl.Stickers[0].Art == nil {
		t.Fatalf("стикеры третьего шага: %+v %v", tpl.Steps[2].Tpl.Stickers, err)
	}
}

// TestSaveSpreadTplSkipsQR: «Сохранить разворот…» тоже не берёт код ссылки клиента; только QR на
// листе — шаблон без стикеров (мест нет — «нечего»).
func TestSaveSpreadTplSkipsQR(t *testing.T) {
	a := lookApp(t)
	ids := saveSeries(t, a)
	if _, err := a.AddQRSticker(ids[2], 0, "https://secret.example/client-album"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := a.editor().AddSticker(ids[2], 0, model.StickerSrc{Art: &model.Art{Kind: "star"}}); err != nil {
		t.Fatal(err)
	}
	it, err := a.SaveSpreadTpl(ids[2], model.ScopeSpread, "mine", "С кодом", true, false)
	if err != nil {
		t.Fatal(err)
	}
	s, _ := a.decorStore()
	tpl, err := s.SpreadTpl(it.ID)
	if err != nil || len(tpl.Stickers) != 1 || tpl.Stickers[0].Art == nil || len(tpl.Places) != 2 {
		t.Fatalf("шаблон: %+v %v", tpl, err)
	}
	data, _ := json.Marshal(tpl)
	if bytes.Contains(data, []byte(strings.TrimPrefix(ownStickers(t, a)[0].ID, "my:"))) {
		t.Error("ключ кода в шаблоне")
	}
}
