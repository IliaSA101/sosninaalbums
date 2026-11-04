package decor

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Макеты альбома в библиотеке и наборах (спека 131, design §7).

// lookOf — макет: титул, повтор, финал; фон страниц — bg, стикер — st (пусто — без них).
func lookOf(bg, st string) model.AlbumTpl {
	page := []model.TplBg{{Color: "#ffffff", Lib: bg}, {Color: "#ffffff"}}
	tpl := model.SpreadTpl{V: model.TplV, Pages: 2, Ratio: 2, Places: []model.Box{{X: 0.1, Y: 0.1, W: 0.3, H: 0.8}}, Page: page}
	if st != "" {
		tpl.Stickers = []model.TplItem{{W: 0.1, H: 0.1, Lib: st}}
	}
	return model.AlbumTpl{V: model.LookV, Steps: []model.LookStep{
		{Name: "Титул", Tpl: tpl}, {Role: model.RoleRepeat, Tpl: tpl}, {Name: "Финал", Tpl: tpl},
	}}
}

func lookBytes(t *testing.T, l model.AlbumTpl, extra string) []byte {
	t.Helper()
	b, err := json.Marshal(l)
	if err != nil {
		t.Fatal(err)
	}
	if extra != "" { // поле новее программы: файл хранится как пришёл
		b = append(b[:len(b)-1], []byte(","+extra+"}")...)
	}
	return b
}

func findItem(t *testing.T, s *Store, kind, id string) (Item, bool) {
	t.Helper()
	cat, err := s.Catalog(kind, nil, 300, 300)
	if err != nil {
		t.Fatal(err)
	}
	i := slices.IndexFunc(cat.Items, func(it Item) bool { return it.ID == id })
	if i < 0 {
		return Item{}, false
	}
	return cat.Items[i], true
}

func TestDecorAlbumKind(t *testing.T) {
	s, cfg := openTemp(t)
	if cat, err := s.Catalog(KindAlbum, nil, 300, 300); err != nil || len(cat.Items) != 0 {
		t.Fatalf("пустой каталог макетов: %v %+v", err, cat)
	}
	if _, err := s.Import(KindAlbum, "mine", nil); !errors.Is(err, ErrKind) {
		t.Errorf("Import макета: %v", err)
	}
	l := lookOf("", "")
	l.Steps = append(l.Steps, model.LookStep{Role: model.RoleVignette, Tpl: l.Steps[0].Tpl, Vig: &model.LookVig{Tpl: "head-t-left", Teachers: 1, Pupils: 24}})
	it, err := s.SaveLook("wedding", "  Свадьба  ", l)
	if err != nil {
		t.Fatal(err)
	}
	want := Item{ID: it.ID, Theme: "wedding", Name: "Свадьба", URL: LookURL(it.ID), Own: true, Steps: 4, From: 3 + 25, Vig: true}
	if it != want {
		t.Fatalf("плитка %+v, ждали %+v", it, want)
	}
	if _, err := s.SaveLook("wedding", "Ещё раз", l); !errors.Is(err, errHave) {
		t.Errorf("повтор: %v", err)
	}
	for name, bad := range map[string]model.AlbumTpl{"пусто": {V: model.LookV}, "версия": {V: 2, Steps: l.Steps}} {
		if _, err := s.SaveLook("mine", name, bad); err == nil {
			t.Errorf("%s: сохранён", name)
		}
	}
	if _, err := s.SaveLook("mine", " ", l); !errors.Is(err, ErrName) {
		t.Errorf("пустое имя: %v", err)
	}
	// Файл — Макеты/<hex>.json, ключ — sha256 байтов; после перезапуска плитка та же, Look читается.
	path := filepath.Join(cfg, RootName, "Макеты", strings.TrimPrefix(it.ID, "my:")+".json")
	data, err := os.ReadFile(path)
	if err != nil || model.LookKey(data) != it.ID {
		t.Fatalf("файл макета: %v, ключ %s ≠ %s", err, model.LookKey(data), it.ID)
	}
	s2 := Open(cfg)
	if got, ok := findItem(t, s2, KindAlbum, it.ID); !ok || got != want {
		t.Fatalf("после перезапуска: %+v", got)
	}
	if lt, err := s2.Look(it.ID); err != nil || len(lt.Steps) != 4 {
		t.Errorf("Look: %v %+v", err, lt)
	}
	for _, id := range []string{"my:0000000000000000", "../x", "sa:tpl-ivory-hero", ""} {
		if _, err := s2.Look(id); !errors.Is(err, ErrNotFound) {
			t.Errorf("Look(%q): %v", id, err)
		}
	}
	// ✎ и × — как у других видов; скрытый без плитки, файл остаётся.
	if err := s2.Rename(KindAlbum, it.ID, "Свадьба 2"); err != nil {
		t.Fatal(err)
	}
	if err := s2.Hide(KindAlbum, it.ID); err != nil {
		t.Fatal(err)
	}
	if _, ok := findItem(t, s2, KindAlbum, it.ID); ok {
		t.Error("скрытый макет в каталоге")
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("файл скрытого удалён: %v", err)
	}
}

// Битый, пустой, огромный файл и файл новее программы под ключом — без плитки и без паники.
func TestDecorAlbumBrokenFile(t *testing.T) {
	for name, body := range map[string]string{"битый": "{", "новее": `{"v":2,"steps":[]}`, "пусто": "", "огромный": strings.Repeat(" ", model.MaxLookFile+1)} {
		_, cfg3 := openTemp(t)
		key := "my:00000000000000a1"
		p := filepath.Join(cfg3, RootName, "Макеты", "00000000000000a1.json")
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		reg := `{"version":1,"items":[{"key":"` + key + `","kind":"album","theme":"mine","name":"М"}]}`
		if err := os.WriteFile(filepath.Join(cfg3, RootName, regFile), []byte(reg), 0o644); err != nil {
			t.Fatal(err)
		}
		s3 := Open(cfg3)
		if _, ok := findItem(t, s3, KindAlbum, key); ok {
			t.Errorf("%s: плитка есть", name)
		}
		if _, err := s3.Look(key); err == nil {
			t.Errorf("%s: Look без ошибки", name)
		}
	}
}

// Предмет album набора: байты как пришли (поле новее программы — тоже), ключ — их sha256, путь
// записи в реестре; обновление набора — новый ключ по тому же пути, прежний файл уходит.
func TestPackAlbumItem(t *testing.T) {
	s, cfg := openTemp(t)
	bg := renderPNG(t, 40, 30)
	v1 := lookBytes(t, lookOf(libKey(bg), ""), `"auto":{"rule":"x"}`)
	m, src := testPack("wedding-layouts", 1, pit{"bg/a.png", bg}, pit{"looks/main.json", v1})
	info := install(t, s, m, src, nil)
	if info.Bg != 1 || info.Bytes != int64(len(bg)+len(v1)) {
		t.Errorf("строка набора %+v", info)
	}
	key1 := model.LookKey(v1)
	if got, ok := s.LookByItem("wedding-layouts", "looks/main.json"); !ok || got != key1 {
		t.Fatalf("LookByItem: %s %v, ждали %s", got, ok, key1)
	}
	disk, err := os.ReadFile(filepath.Join(cfg, RootName, "Макеты", strings.TrimPrefix(key1, "my:")+".json"))
	if err != nil || !bytes.Equal(disk, v1) {
		t.Fatalf("файл не как пришёл: %v", err)
	}
	reg, err := os.ReadFile(filepath.Join(cfg, RootName, packsDir, "wedding-layouts.json"))
	if err != nil || !strings.Contains(string(reg), `"item": "looks/main.json"`) {
		t.Fatalf("путь записи в реестре: %v\n%s", err, reg)
	}
	it, ok := findItem(t, s, KindAlbum, key1)
	if !ok || it.Pack != "Набор wedding-layouts" || it.Steps != 3 || it.From != 3 || it.URL != LookURL(key1) {
		t.Fatalf("плитка набора %+v", it)
	}
	for _, q := range [][2]string{{"wedding-layouts", "looks/other.json"}, {"nope", "looks/main.json"}, {"wedding-layouts", "../x"}, {"wedding-layouts", ""}} {
		if _, ok := s.LookByItem(q[0], q[1]); ok {
			t.Errorf("LookByItem%v найден", q)
		}
	}
	// Обновление: тот же путь, другие байты — новый ключ; LookByItem — новый, прежний файл удалён.
	l2 := lookOf(libKey(bg), "")
	l2.Steps[0].Name = "Начало"
	v2 := lookBytes(t, l2, "")
	m2, src2 := testPack("wedding-layouts", 2, pit{"bg/a.png", bg}, pit{"looks/main.json", v2})
	st, err := s.StagePack(m2, src2, OriginFile, strings.Repeat("cd", 32))
	if err != nil || st.Ask != AskUpdate {
		t.Fatalf("обновление: %v %+v", err, st)
	}
	if _, err := s.InstallPack(st, nil); err != nil {
		t.Fatal(err)
	}
	st.Close()
	key2 := model.LookKey(v2)
	s2 := Open(cfg)
	if got, ok := s2.LookByItem("wedding-layouts", "looks/main.json"); !ok || got != key2 {
		t.Errorf("после обновления: %s %v", got, ok)
	}
	if _, err := s2.Look(key1); !errors.Is(err, ErrNotFound) {
		t.Errorf("прежний макет остался: %v", err)
	}
	if _, ok := s2.Find(libKey(bg)); !ok {
		t.Error("фон, нужный новому макету, удалён")
	}
}

// checkLookLibs (то же правило, что checkTplLibs): ссылка на картинку своего набора или стоящую в
// библиотеке; чужая — пропуск. Незнакомый узор, больше 24 шагов, битый, новее — пропуск с причиной.
func TestCheckLookLibs(t *testing.T) {
	s, _ := openTemp(t)
	donor := renderPNG(t, 41, 30)
	own := renderPNG(t, 42, 30)
	stk := renderPNG(t, 30, 42)
	put(t, s, "donor", pit{"bg/d.png", donor})
	many := lookOf("", "")
	for len(many.Steps) <= model.MaxLookSteps {
		many.Steps = append(many.Steps, many.Steps[0])
	}
	unknownPat := lookOf("", "")
	unknownPat.Steps[2].Tpl.Page[1].Pat = &model.Pattern{Kind: "future-pattern"}
	coverBg := lookOf("", "")
	coverBg.Cover = &model.CoverTpl{Bg: &model.TplBg{Lib: "my:00000000000000ff"}}
	m, src := testPack("set-layouts", 1,
		pit{"bg/own.png", own},
		pit{"stickers/st.png", stk},
		pit{"looks/donor.json", lookBytes(t, lookOf(libKey(donor), ""), "")},
		pit{"looks/own.json", lookBytes(t, lookOf(libKey(own), libKey(stk)), "")},
		pit{"looks/missing.json", lookBytes(t, lookOf("my:0123456789abcdef", ""), "")},
		pit{"looks/cover.json", lookBytes(t, coverBg, "")},
		pit{"looks/many.json", lookBytes(t, many, "")},
		pit{"looks/pattern.json", lookBytes(t, unknownPat, "")},
		pit{"looks/newer.json", []byte(`{"v":2,"steps":[]}`)},
		pit{"looks/broken.json", []byte(`{"v":1,"steps":[`)},
		pit{"looks/a/b/c.json", lookBytes(t, lookOf("", ""), "")},
	)
	st, err := s.StagePack(m, src, OriginFile, strings.Repeat("ef", 32))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	var whys []string
	for _, sk := range st.Skipped {
		whys = append(whys, sk.Name+": "+sk.Why)
	}
	want := []string{
		"looks/a/b/c.json: недопустимое имя записи макета",
		"looks/broken.json: макет альбома не читается",
		"looks/many.json: в макете больше 24 разворотов",
		"looks/newer.json: макет из новой версии программы",
		"looks/pattern.json: макет из новой версии программы",
		"looks/missing.json: макет ссылается на картинку не из набора",
		"looks/cover.json: макет ссылается на картинку не из набора",
	}
	slices.Sort(whys[:5])
	if !slices.Equal(whys, want) {
		t.Fatalf("пропуски:\n%s", strings.Join(whys, "\n"))
	}
	if len(st.items) != 4 { // две картинки и два макета
		t.Fatalf("предметов %d", len(st.items))
	}
}

// put — набор id версии 1 целиком.
func put(t *testing.T, s *Store, id string, items ...pit) {
	t.Helper()
	m, src := testPack(id, 1, items...)
	install(t, s, m, src, nil)
}

// Удаление наборов с макетами: картинку донора держит макет дополнения; макет, который удаляется той же
// сборкой, своих картинок не держит; свой макет держит картинку набора.
func TestRemovePackKeepsAlbumRefs(t *testing.T) {
	setup := lookDonor
	t.Run("донор первым — картинка остаётся", func(t *testing.T) {
		s, bg, look := setup(t)
		if res, err := s.RemovePack("donor", nil); err != nil || res.Files != 0 || res.Kept != 1 {
			t.Fatalf("удаление донора: %v %+v", err, res)
		}
		if _, ok := s.Find(bg); !ok {
			t.Fatal("картинку макета удалили")
		}
		if _, err := s.Look(look); err != nil {
			t.Fatalf("макет пропал: %v", err)
		}
	})
	t.Run("дополнение первым — потом донор целиком", func(t *testing.T) {
		s, bg, look := setup(t)
		if res, err := s.RemovePack("donor-layouts", nil); err != nil || res.Files != 1 {
			t.Fatalf("удаление дополнения: %v %+v", err, res)
		}
		if _, err := s.Look(look); !errors.Is(err, ErrNotFound) {
			t.Fatalf("макет остался: %v", err)
		}
		if res, err := s.RemovePack("donor", nil); err != nil || res.Files != 1 || res.Kept != 0 {
			t.Fatalf("удаление донора: %v %+v", err, res)
		}
		if _, ok := s.Find(bg); ok {
			t.Error("картинка осталась без ссылок")
		}
	})
	t.Run("макет и картинка в одном наборе — уходят вместе", func(t *testing.T) {
		s, _ := openTemp(t)
		bg := renderPNG(t, 44, 30)
		put(t, s, "solo-layouts", pit{"bg/s.png", bg}, pit{"looks/s.json", lookBytes(t, lookOf(libKey(bg), ""), "")})
		plan, err := s.PlanRemovePack("solo-layouts", nil)
		if err != nil || plan.Files != 2 || plan.Kept != 0 {
			t.Fatalf("план: %v %+v", err, plan)
		}
		if res, err := s.RemovePack("solo-layouts", nil); err != nil || res != plan {
			t.Fatalf("удаление: %v %+v, план %+v", err, res, plan)
		}
	})
}

// lookDonor — донор с фоном и дополнение с макетом на нём; ключи фона и макета.
func lookDonor(t *testing.T) (*Store, string, string) {
	t.Helper()
	s, _ := openTemp(t)
	bg := renderPNG(t, 43, 30)
	put(t, s, "donor", pit{"bg/d.png", bg})
	data := lookBytes(t, lookOf(libKey(bg), ""), "")
	put(t, s, "donor-layouts", pit{"looks/main.json", data})
	return s, libKey(bg), model.LookKey(data)
}

// Свой макет держит картинку набора; макет, оставленный keep (нужен открытому альбому), — тоже.
func TestRemovePackOwnLookAndKeep(t *testing.T) {
	s, bg, _ := lookDonor(t)
	if _, err := s.SaveLook("mine", "Мой", lookOf(bg, bg)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RemovePack("donor-layouts", nil); err != nil {
		t.Fatal(err)
	}
	if res, err := s.RemovePack("donor", nil); err != nil || res.Kept != 1 || res.Files != 0 {
		t.Fatalf("донор: %v %+v", err, res)
	}
	s2, _, look := lookDonor(t)
	keep := func(k string) bool { return k == look }
	if res, err := s2.RemovePack("donor-layouts", keep); err != nil || res.Kept != 1 {
		t.Fatalf("keep: %v %+v", err, res)
	}
	if res, _ := s2.PlanRemovePack("donor", nil); res.Kept != 1 {
		t.Errorf("макет, оставленный keep, не держит картинку: %+v", res)
	}
}

// Контракт 0.27-131-30: ключи картинок файла макета — строки как есть, сырой разбор находит ровно их.
func TestLookFileKeysRaw(t *testing.T) {
	l := lookOf("my:00000000000000b1", "my:00000000000000b2")
	front := model.SpreadTpl{V: model.TplV, Pages: 1, Page: []model.TplBg{{Lib: "my:00000000000000b3"}}}
	l.Cover = &model.CoverTpl{Bg: &model.TplBg{Lib: "my:00000000000000b4"}, Front: &front}
	l.Steps[1].Name = `Глава "my:" и \ кавычки`
	// Рамочка места (borders[].lib, G1) — тоже ключ картинки (0.28-131-27 п. 3).
	l.Steps[2].Tpl.Borders = []model.TplBorder{{I: 0, Lib: "my:00000000000000b5"}}
	s, cfg := openTemp(t)
	it, err := s.SaveLook("mine", "Ключи", l)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(cfg, RootName, "Макеты", strings.TrimPrefix(it.ID, "my:")+".json"))
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := model.ParseLook(data)
	if err != nil {
		t.Fatal(err)
	}
	var want []string
	for _, ref := range lookLibs(parsed) {
		if !slices.Contains(want, ref.key) {
			want = append(want, ref.key)
		}
	}
	got := rawLibKeys(data)
	slices.Sort(want)
	slices.Sort(got)
	if len(want) != 5 || !slices.Equal(got, want) || !slices.Contains(want, "my:00000000000000b5") {
		t.Fatalf("сырые ключи %v, по разбору %v", got, want)
	}
}
