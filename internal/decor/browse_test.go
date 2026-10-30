package decor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Общий вектор вида и стикера с packprep (design §5): пилот и края.
type formCase struct {
	Name  string `json:"name"`
	W     int    `json:"w"`
	H     int    `json:"h"`
	Form  string `json:"form"`
	Style string `json:"style"`
	Note  string `json:"note"`
}

func formVector(t *testing.T) []formCase {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "parity", "decorform.json"))
	if err != nil {
		t.Fatal(err)
	}
	var v struct {
		Cases []formCase `json:"cases"`
	}
	if err := json.Unmarshal(data, &v); err != nil || len(v.Cases) < 144 {
		t.Fatalf("вектор: %v, случаев %d", err, len(v.Cases))
	}
	return v.Cases
}

func TestFormOf(t *testing.T) {
	for _, c := range formVector(t) {
		if got := formOf(c.Name, c.W, c.H); got != c.Form {
			t.Errorf("%q %d×%d (%s): вид %q, ждали %q", c.Name, c.W, c.H, c.Note, got, c.Form)
		}
	}
}

func TestStyleOf(t *testing.T) {
	styles := map[string]int{}
	for _, c := range formVector(t) {
		// Стиль берёт только обычный стикер: у уголка и разделителя его нет, как в каталоге.
		got := ""
		if formOf(c.Name, c.W, c.H) == "" {
			got = styleOf(c.Name)
		}
		if got != c.Style {
			t.Errorf("%q (%s): стиль %q, ждали %q", c.Name, c.Note, got, c.Style)
		}
		if got != "" {
			styles[got]++
		}
	}
	// Пилот: школа — Значки 36, Рисунки 12; сад — Рисунки 36, Акварель 12.
	if styles["Значки"] < 36 || styles["Рисунки"] < 48 || styles["Акварель"] < 12 {
		t.Errorf("стили пилота: %v", styles)
	}
}

// Свои стикеры, фоны и рамочки не размечаются; наборы и встроенные — да; поля доходят до JSON.
func TestCatalogFormStyle(t *testing.T) {
	s, _ := openTemp(t)
	round := renderPNG(t, 20, 20)
	wide := renderPNG(t, 80, 20) // 4:1 — у набора разделитель
	m, src := testPack("marks", 1, pit{"stickers/a.png", round}, pit{"stickers/b.png", wide}, pit{"stickers/c.png", renderPNG(t, 22, 22)},
		pit{"bg/x.png", renderPNG(t, 70, 30)}, pit{"stickers/d.png", renderPNG(t, 84, 21)})
	m.Items[0].Name, m.Items[1].Name, m.Items[2].Name = "Значки: Колокольчик", "Лента", "Уголки для фото"
	m.Items[3].Name = "Уголок: фон"
	m.Items[4].Name = "Разделитель: яблоко"
	install(t, s, m, src, nil)
	dir := testkit.TempDir(t)
	own, err := s.Import(KindSticker, "mine", []string{writeFile(t, dir, "own.png", renderPNG(t, 120, 20))}) // 6:1, «Уголок» в имени файла не нужен
	if err != nil || len(own.Added) != 1 {
		t.Fatalf("свой: %v %+v", err, own)
	}
	cat, err := s.Catalog(KindSticker, nil, 300, 300)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string][2]string{}
	for _, it := range cat.Items {
		got[it.Name] = [2]string{it.Form, it.Style}
		if it.Own && (it.Form != "" || it.Style != "") {
			t.Errorf("свой размечен: %+v", it)
		}
	}
	for name, want := range map[string][2]string{
		"Значки: Колокольчик": {"", "Значки"}, "Лента": {FormDivider, ""}, "Разделитель: яблоко": {FormDivider, ""}, "Уголки для фото": {FormCorner, ""},
		"Уголок для фото": {FormCorner, ""}, "Лента-баннер": {"", ""}, // встроенные: размера нет — по имени
	} {
		if g, ok := got[name]; !ok || g != want {
			t.Errorf("%q: %v (есть=%v), ждали %v", name, g, ok, want)
		}
	}
	raw, _ := json.Marshal(cat.Items)
	if !strings.Contains(string(raw), `"form":"divider"`) || strings.Contains(string(raw), `"form":""`) {
		t.Errorf("JSON: form только непустой")
	}
	bg, err := s.Catalog(KindBg, nil, 300, 300)
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range bg.Items {
		if it.Form != "" || it.Style != "" {
			t.Errorf("у фона вид: %+v", it)
		}
	}
}

// setNow подставляет часы библиотеки: возвращает указатель на «сейчас».
func setNow(t *testing.T, start int64) *int64 {
	t.Helper()
	now := start
	old := nowMs
	nowMs = func() int64 { return now }
	t.Cleanup(func() { nowMs = old })
	return &now
}

const (
	t0  = int64(1800000000000) // 2027-01-15: после minInstalled
	day = int64(24 * 3600 * 1000)
)

// installAt ставит набор из одного стикера в момент at; возвращает ID предмета.
func installAt(t *testing.T, s *Store, now *int64, id string, at int64, seed int) string {
	t.Helper()
	*now = at
	data := renderPNG(t, 16+seed, 16+seed)
	m, src := testPack(id, 1, pit{"stickers/a.png", data})
	install(t, s, m, src, nil)
	return libKey(data)
}

func newOf(t *testing.T, s *Store, kind string) Catalog {
	t.Helper()
	cat, err := s.Catalog(kind, nil, 300, 300)
	if err != nil {
		t.Fatal(err)
	}
	if cat.New == nil {
		t.Fatal("New = nil: в JSON будет null")
	}
	if len(cat.New) > 0 != (len(cat.Themes) > 0 && cat.Themes[0].ID == ThemeNew) {
		t.Errorf("«Новые» первой строкой только при предметах: %v %+v", cat.New, cat.Themes)
	}
	if len(cat.New) > 0 && cat.Themes[0].Count != len(cat.New) {
		t.Errorf("число строки %d, предметов %d", cat.Themes[0].Count, len(cat.New))
	}
	return cat
}

// TestCatalogNewFirstBatch — нет наборов и один набор — первая партия.
func TestCatalogNewFirstBatch(t *testing.T) {
	s, _ := openTemp(t)
	now := setNow(t, t0)
	if c := newOf(t, s, KindSticker); len(c.New) != 0 {
		t.Fatalf("без наборов %v", c.New)
	}
	installAt(t, s, now, "first", t0, 1)
	if c := newOf(t, s, KindSticker); len(c.New) != 0 {
		t.Errorf("единственный набор — первая партия: %v", c.New)
	}

}

// TestCatalogNewWindow — порядок каталога, окно 30 суток включительно, первые сутки.
func TestCatalogNewWindow(t *testing.T) {
	s, _ := openTemp(t)
	now := setNow(t, t0)
	installAt(t, s, now, "aaa", t0, 1)
	installAt(t, s, now, "bbb", t0+day, 2)         // ровно 24 ч: ещё первая партия
	kc := installAt(t, s, now, "ccc", t0+day+1, 3) // позже 24 ч — новый
	kd := installAt(t, s, now, "ddd", t0+5*day, 4) // новее
	*now = t0 + 5*day
	if c := newOf(t, s, KindSticker); !slices.Equal(c.New, []string{kd, kc}) {
		t.Errorf("новые %v, ждали [ddd ccc] — новый выше, bbb (ровно 24 ч) нет", c.New)
	}
	*now = t0 + day + 1 + 30*day // ccc ровно 30 суток
	if c := newOf(t, s, KindSticker); !slices.Equal(c.New, []string{kd, kc}) {
		t.Errorf("30 суток включительно: %v", c.New)
	}
	*now = t0 + day + 2 + 30*day // 30 суток и 1 мс
	if c := newOf(t, s, KindSticker); slices.Contains(c.New, kc) {
		t.Errorf("ccc старше 30 суток остался: %v", c.New)
	}
	*now = t0 + 40*day
	if c := newOf(t, s, KindSticker); len(c.New) != 0 {
		t.Errorf("через 40 суток новых нет: %v", c.New)
	}

}

// TestCatalogNewTieAndKinds — один Installed у двух наборов, набор только с фонами, 124 плитки.
func TestCatalogNewTieAndKinds(t *testing.T) {
	s, _ := openTemp(t)
	now := setNow(t, t0)
	installAt(t, s, now, "aaa", t0, 1)
	installAt(t, s, now, "bbb", t0, 2) // тот же момент — первая партия
	if c := newOf(t, s, KindSticker); len(c.New) != 0 {
		t.Errorf("два набора в один момент: %v", c.New)
	}
	*now = t0 + 3*day
	m, src := testPack("only-bg", 1, pit{"bg/q.png", renderPNG(t, 70, 31)})
	install(t, s, m, src, nil)
	if c := newOf(t, s, KindSticker); len(c.New) != 0 {
		t.Errorf("набор с фонами создал «Новые» у стикеров: %v", c.New)
	}
	if c := newOf(t, s, KindBg); len(c.New) != 1 {
		t.Errorf("фоны: %v", c.New)
	}
	var items []pit
	for i := range 124 {
		items = append(items, pit{"stickers/s" + string(rune('a'+i%26)) + string(rune('a'+i/26)) + ".png", renderPNG(t, 20+i, 20+i)})
	}
	*now = t0 + 4*day
	mm, ss := testPack("big", 1, items...)
	install(t, s, mm, ss, nil)
	if c := newOf(t, s, KindSticker); len(c.New) != 124 {
		t.Errorf("124 плитки: %d", len(c.New))
	}

}

// TestCatalogNewRemoveKeepsBase — удаление не сдвигает границу, обновление хранит её, переустановка — новый.
func TestCatalogNewRemoveKeepsBase(t *testing.T) {
	s, cfg := openTemp(t)
	now := setNow(t, t0)
	installAt(t, s, now, "wedding", t0, 1) // день 0
	ks := installAt(t, s, now, "school", t0+60*day, 2)
	kg := installAt(t, s, now, "garden", t0+61*day, 3)
	*now = t0 + 62*day
	if _, err := s.RemovePack("wedding", nil); err != nil {
		t.Fatal(err)
	}
	if c := newOf(t, s, KindSticker); !slices.Equal(c.New, []string{kg, ks}) {
		t.Errorf("после удаления самого раннего: %v", c.New)
	}
	if c := newOf(t, Open(cfg), KindSticker); !slices.Equal(c.New, []string{kg, ks}) { // круг записи и чтения
		t.Errorf("после перезапуска: %v", c.New)
	}
	// Обновление garden после окна: дата и граница прежние, новым не становится.
	*now = t0 + 100*day
	m2, s2 := testPack("garden", 2, pit{"stickers/b.png", renderPNG(t, 50, 50)})
	st, err := s.StagePack(m2, s2, OriginFile, "")
	if err != nil || st.Ask != AskUpdate {
		t.Fatalf("вопрос обновления: %v %+v", err, st)
	}
	defer st.Close()
	if _, err := s.InstallPack(st, nil); err != nil {
		t.Fatal(err)
	}
	if c := newOf(t, s, KindSticker); len(c.New) != 0 {
		t.Errorf("обновление сделало набор новым: %v", c.New)
	}
	reg, err := readPackReg(filepath.Join(cfg, RootName, packsDir, "garden.json"), "garden")
	if err != nil || reg.Base != t0 || reg.Installed != t0+61*day {
		t.Errorf("обновление: Base %d Installed %d, ждали %d и %d (%v)", reg.Base, reg.Installed, t0, t0+61*day, err)
	}
	// Переустановка: удалён и поставлен снова — новый.
	if _, err := s.RemovePack("school", nil); err != nil {
		t.Fatal(err)
	}
	k2 := installAt(t, s, now, "school", t0+101*day, 2)
	if c := newOf(t, s, KindSticker); !slices.Equal(c.New, []string{k2}) {
		t.Errorf("переустановка: %v", c.New)
	}
	// Удалены все — следующий набор снова первая партия.
	for _, p := range s.Packs()[1:] {
		if _, err := s.RemovePack(p.ID, nil); err != nil {
			t.Fatal(err)
		}
	}
	installAt(t, s, now, "again", t0+102*day, 6)
	if c := newOf(t, s, KindSticker); len(c.New) != 0 {
		t.Errorf("после удаления всех: %v", c.New)
	}

}

// TestCatalogNewBadDates — Installed 0, до 2025 и из будущего.
func TestCatalogNewBadDates(t *testing.T) {
	s, cfg := openTemp(t)
	now := setNow(t, t0)
	installAt(t, s, now, "aaa", t0, 1)
	k := installAt(t, s, now, "bbb", t0+10*day, 2)
	regs := filepath.Join(cfg, RootName, packsDir)
	for id, inst := range map[string]string{"zero": "0", "old": "1000", "future": "9999999999999"} {
		body := `{"v":1,"id":"` + id + `","version":1,"title":"` + id + `","installed":` + inst + `,"items":[]}`
		if err := os.WriteFile(filepath.Join(regs, id+".json"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	s = Open(cfg)
	*now = t0 + 11*day
	// Пустые наборы без предметов в New не попадают; границу 0 и 1000 не сдвигают: bbb остаётся новым.
	if c := newOf(t, s, KindSticker); !slices.Equal(c.New, []string{k}) {
		t.Errorf("мусорные даты сдвинули границу: %v", c.New)
	}
	reg, err := readPackReg(filepath.Join(regs, "zero.json"), "zero")
	if err != nil || reg.Installed != 0 {
		t.Fatal(err, reg)
	}
	// Дата из будущего — «сейчас»: набор новый, пока есть предметы.
	s2, _ := openTemp(t)
	now2 := setNow(t, t0)
	installAt(t, s2, now2, "aaa", t0, 1)
	kf := installAt(t, s2, now2, "fut", t0+5*day, 2)
	*now2 = t0 + 2*day // часы переведены назад: Installed набора fut позже «сейчас»
	if c := newOf(t, s2, KindSticker); !slices.Equal(c.New, []string{kf}) {
		t.Errorf("дата из будущего: %v", c.New)
	}
	*now2 = t0 + day/2 // «сейчас» в первых сутках: будущая дата считается за «сейчас» — первая партия
	if c := newOf(t, s2, KindSticker); len(c.New) != 0 {
		t.Errorf("будущая дата в первых сутках: %v", c.New)
	}

}

// TestCatalogNewNotTheme — своё и «new» как тема.
func TestCatalogNewNotTheme(t *testing.T) {
	if isTheme(ThemeNew) {
		t.Error("new — выборка, не тема")
	}
	s, _ := openTemp(t)
	res, err := s.Import(KindBg, ThemeNew, []string{writeFile(t, testkit.TempDir(t), "x.png", renderPNG(t, 70, 40))})
	if err != nil || len(res.Added) != 1 || res.Added[0].Theme != ThemeMine {
		t.Errorf("импорт в «new» должен лечь в «Мои»: %v %+v", err, res.Added)
	}

}

// Реестр без Base читается, мусорный Base зажимается; RemovePack дописывает границу оставшимся.
func TestRemovePackKeepsBase(t *testing.T) {
	s, cfg := openTemp(t)
	now := setNow(t, t0)
	installAt(t, s, now, "aaa", t0, 1)
	installAt(t, s, now, "bbb", t0+60*day, 2)
	installAt(t, s, now, "ccc", t0+90*day, 3)
	regs := filepath.Join(cfg, RootName, packsDir)
	read := func(id string) *packReg {
		t.Helper()
		r, err := readPackReg(filepath.Join(regs, id+".json"), id)
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	if read("bbb").Base != 0 || read("ccc").Base != 0 {
		t.Fatal("пока ничего не удаляли, писать нечего")
	}
	// Удаляется не держащий границу (последний): граница не сдвигается, записи не нужны.
	if _, err := s.RemovePack("ccc", nil); err != nil {
		t.Fatal(err)
	}
	if read("bbb").Base != 0 {
		t.Errorf("Base без нужды: %d", read("bbb").Base)
	}
	installAt(t, s, now, "ccc", t0+90*day, 3)
	if _, err := s.RemovePack("aaa", nil); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"bbb", "ccc"} {
		if b := read(id).Base; b != t0 {
			t.Errorf("%s: Base %d, ждали %d", id, b, t0)
		}
	}
	noTmpAnywhere(t, cfg)
	// Повторное удаление уже стоящей границы ничего не меняет и Base остаётся.
	if _, err := s.RemovePack("bbb", nil); err != nil {
		t.Fatal(err)
	}
	if read("ccc").Base != t0 {
		t.Errorf("Base потерян: %d", read("ccc").Base)
	}
	// Мусорный Base в реестре зажимается.
	for name, body := range map[string]string{
		"junk":  `{"v":1,"id":"junk","base":5,"installed":` + "1800000000000" + `,"items":[]}`,
		"minus": `{"v":1,"id":"minus","base":-9,"items":[]}`,
		"plain": `{"v":1,"id":"plain","items":[]}`,
	} {
		if err := os.WriteFile(filepath.Join(regs, name+".json"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		if r := read(name); r.Base != 0 {
			t.Errorf("%s: Base %d", name, r.Base)
		}
	}
}

// Сбой чтения не по содержимому (папка вместо файла) — удаление не начинается (повтор доделает),
// реестры на месте; запись границы при сбое записи — TestRemovePackWriteFailsReadable.
func TestRemovePackBaseWriteFails(t *testing.T) {
	s, cfg := openTemp(t)
	now := setNow(t, t0)
	installAt(t, s, now, "aaa", t0, 1)
	installAt(t, s, now, "bbb", t0+60*day, 2)
	regs := filepath.Join(cfg, RootName, packsDir)
	bad := filepath.Join(regs, "bbb.json")
	orig, err := os.ReadFile(bad)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(bad); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(bad, "x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RemovePack("aaa", nil); err == nil || !strings.Contains(err.Error(), "не удалился") {
		t.Fatalf("ждали отказ: %v", err)
	}
	if len(s.Packs()) != 3 || !regularFile(filepath.Join(regs, "aaa.json")) {
		t.Errorf("набор пропал при сбое записи границы: %+v", s.Packs())
	}
	// Сбой устранён — повтор удаляет и дописывает границу.
	if err := os.RemoveAll(bad); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bad, orig, 0o644); err != nil { // реестр на месте: на диске keepBase правит только существующий
		t.Fatal(err)
	}
	if _, err := s.RemovePack("aaa", nil); err != nil {
		t.Fatalf("повтор: %v", err)
	}
	if r, err := readPackReg(bad, "bbb"); err != nil || r.Base != t0 {
		t.Errorf("после повтора: %+v %v", r, err)
	}
}

// У набора записана граница позже настоящей (правка руками, чужая версия) — удаление держателя
// границы опускает её.
func TestRemovePackLowersBase(t *testing.T) {
	s, cfg := openTemp(t)
	now := setNow(t, t0)
	installAt(t, s, now, "aaa", t0, 1)
	installAt(t, s, now, "bbb", t0+60*day, 2)
	installAt(t, s, now, "ccc", t0+90*day, 3)
	regs := filepath.Join(cfg, RootName, packsDir)
	reg, err := readPackReg(filepath.Join(regs, "bbb.json"), "bbb")
	if err != nil {
		t.Fatal(err)
	}
	reg.Base = t0 + 30*day // позже границы, но раньше собственной даты
	if err := s.writePackReg(reg); err != nil {
		t.Fatal(err)
	}
	s = Open(cfg)
	if _, err := s.RemovePack("aaa", nil); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"bbb", "ccc"} {
		if r, err := readPackReg(filepath.Join(regs, id+".json"), id); err != nil || r.Base != t0 {
			t.Errorf("%s: Base %+v, ждали %d (%v)", id, r, t0, err)
		}
	}
}

// Окна макетов альбома «Новых» нет (спека 133 «Не входит»): макет свежего набора в New не попадает.
func TestCatalogNewNotForAlbum(t *testing.T) {
	s, _ := openTemp(t)
	now := setNow(t, t0)
	installAt(t, s, now, "aaa", t0, 1)
	*now = t0 + 5*day
	bg := renderPNG(t, 40, 30)
	m, src := testPack("layouts", 1, pit{"bg/a.png", bg}, pit{"looks/main.json", lookBytes(t, lookOf(libKey(bg), ""), "")})
	install(t, s, m, src, nil)
	if c := newOf(t, s, KindBg); len(c.New) != 1 {
		t.Fatalf("фон свежего набора: %v", c.New)
	}
	cat := newOf(t, s, KindAlbum)
	if len(cat.New) != 0 || len(cat.Items) == 0 {
		t.Errorf("макеты: New %v, плиток %d (макет набора должен быть в каталоге)", cat.New, len(cat.Items))
	}
}

// FuzzFormStyle: любое имя и размер — без паники; стиль и вид не пересекаются, стиль короче имени.
func FuzzFormStyle(f *testing.F) {
	for _, c := range []formCase{{Name: "Значки: Колокольчик"}, {Name: "Золотой уголок", W: 5, H: 5}, {Name: "x", W: 2800, H: 700}} {
		f.Add(c.Name, c.W, c.H)
	}
	f.Add("\xff\xfe: \x00", -1, 1<<62)
	f.Add("Уголок", 1<<62, 1<<62)
	f.Fuzz(func(t *testing.T, name string, w, h int) {
		form := formOf(name, w, h)
		if form != "" && form != FormCorner && form != FormDivider {
			t.Fatalf("вид %q", form)
		}
		if st := styleOf(name); len(st) >= len(name) && st != "" {
			t.Fatalf("стиль %q длиннее имени %q", st, name)
		}
	})
}
