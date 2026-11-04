package decor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Пример темы (142 design §3): происхождение, «Новые», ссылки на макеты, DemoOnly.

const demoID = "wedding-demo"

// installOrigin ставит набор с заданным origin.
func installOrigin(t *testing.T, s *Store, origin string, m PackManifest, src fakeSrc) {
	t.Helper()
	st, err := s.StagePack(m, src, origin, strings.Repeat("ab", 32))
	if err != nil || st.Ask != "" || len(st.Skipped) != 0 {
		t.Fatalf("StagePack %s: %v %+v", m.ID, err, st)
	}
	defer st.Close()
	if _, err := s.InstallPack(st, nil); err != nil {
		t.Fatalf("InstallPack %s: %v", m.ID, err)
	}
}

func originOf(s *Store, id string) string {
	for _, p := range s.Packs() {
		if p.ID == id {
			return p.Origin
		}
	}
	return "?"
}

// installDemo ставит пример с заданными предметами.
func installDemo(t *testing.T, s *Store, items ...pit) {
	t.Helper()
	m, src := testPack(demoID, 1, items...)
	installOrigin(t, s, OriginDemo, m, src)
}

// Происхождение: net и demo доходят до реестра и переживают открытие заново; прочее — file;
// у примера есть «Удалить».
func TestDemoOrigin(t *testing.T) {
	s, cfg := openTemp(t)
	origins := []string{OriginDemo, OriginNet, OriginFile, "", "demo ", "DEMO", "builtin", "../x"}
	for i, o := range origins {
		m, src := testPack("pack-"+string(rune('a'+i)), 1, pit{"stickers/a.png", renderPNG(t, 20+i, 20)})
		installOrigin(t, s, o, m, src)
	}
	want := []string{OriginDemo, OriginNet, OriginFile, OriginFile, OriginFile, OriginFile, OriginFile, OriginFile}
	for _, st := range []*Store{s, Open(cfg)} { // второй — круг запись → чтение реестра
		for i, w := range want {
			id := "pack-" + string(rune('a'+i))
			if got := originOf(st, id); got != w {
				t.Errorf("origin %q (%s) → %q, ждали %q", origins[i], id, got, w)
			}
		}
		for _, p := range st.Packs() {
			if p.ID == "pack-a" && !p.Removable {
				t.Errorf("пример нельзя удалить: %+v", p)
			}
		}
	}
}

// Реестр, правленный руками: origin demo читается, мусор — file.
func TestDemoOriginRegRaw(t *testing.T) {
	s, cfg := openTemp(t)
	installDemo(t, s, pit{"stickers/a.png", renderPNG(t, 20, 20)})
	reg := filepath.Join(cfg, RootName, packsDir, demoID+".json")
	raw := rawRegOf(t, reg)
	if raw["origin"] != "demo" {
		t.Fatalf("в реестре %v", raw["origin"])
	}
	for _, bad := range []any{"Demo", "", nil, "net2"} {
		raw["origin"] = bad
		data, err := json.Marshal(raw)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(reg, data, 0o600); err != nil {
			t.Fatal(err)
		}
		if got := originOf(Open(cfg), demoID); got != OriginFile {
			t.Errorf("origin %v → %q", bad, got)
		}
	}
}

// SetDemo: мусорный id снимает пример, мусорные доноры и сам пример в связи отбрасываются.
func TestDemoSetDemo(t *testing.T) {
	s, _ := openTemp(t)
	s.SetDemo(demoID, []string{"abc-def", "../x", demoID, "", "con", "xy"})
	if s.demo.id != demoID || !slices.Equal(s.demo.related, []string{"abc-def"}) {
		t.Fatalf("%+v", s.demo)
	}
	for _, bad := range []string{"", "..", "A", "con", strings.Repeat("a", 41)} {
		s.SetDemo(demoID, []string{"abc-def"})
		s.SetDemo(bad, []string{"abc-def"})
		if s.demo.id != "" || s.demo.related != nil {
			t.Errorf("SetDemo(%q) = %+v", bad, s.demo)
		}
	}
	s.SetDemo(demoID, nil)
	if s.demo.id != demoID || len(s.demo.related) != 0 {
		t.Errorf("без доноров: %+v", s.demo)
	}
}

// «Новые»: пример не задаёт границу первой партии. У нового пользователя (пример первым) набор через
// 10 суток не «новый»; без SetDemo — как раньше.
func TestPackBaseSkipsDemo(t *testing.T) {
	s, _ := openTemp(t)
	now := setNow(t, t0)
	s.SetDemo(demoID, nil)
	installDemo(t, s, pit{"stickers/d.png", renderPNG(t, 30, 30)})
	s.mu.RLock()
	b, ok := s.packBase(nil)
	s.mu.RUnlock()
	if ok {
		t.Fatalf("только пример: граница %d", b)
	}
	if c := newOf(t, s, KindSticker); len(c.New) != 0 {
		t.Fatalf("только пример: %v", c.New)
	}
	installAt(t, s, now, "aaa", t0+10*day, 1)
	kb := installAt(t, s, now, "bbb", t0+20*day, 2)
	*now = t0 + 20*day
	if c := newOf(t, s, KindSticker); !slices.Equal(c.New, []string{kb}) {
		t.Errorf("новые %v: aaa — первая партия, пример не новый (ждали только bbb)", c.New)
	}
	// Без SetDemo пример — обычный набор: граница по нему, aaa и bbb — новые.
	s.SetDemo("", nil)
	if c := newOf(t, s, KindSticker); len(c.New) != 2 {
		t.Errorf("без SetDemo %v", c.New)
	}
}

// У обновившегося пользователя пример встаёт позже каталога и попадает в «Новые».
func TestPackBaseDemoUpdatedUser(t *testing.T) {
	s, _ := openTemp(t)
	now := setNow(t, t0)
	s.SetDemo(demoID, nil)
	installAt(t, s, now, "aaa", t0, 1)
	*now = t0 + 5*day
	installDemo(t, s, pit{"stickers/d.png", renderPNG(t, 30, 30)})
	c := newOf(t, s, KindSticker)
	if len(c.New) != 1 {
		t.Fatalf("у обновившегося пример должен быть в «Новых»: %v", c.New)
	}
	if it, _ := findItem(t, s, KindSticker, c.New[0]); it.PackID != demoID {
		t.Errorf("новая плитка не из примера: %+v", it)
	}
}

// Уход примера не пишет границу соседям и не сдвигает её; уход первого набора при стоящем примере —
// граница по каталогу, пример на неё не влияет.
func TestPackBaseDemoRemove(t *testing.T) {
	s, cfg := openTemp(t)
	now := setNow(t, t0)
	s.SetDemo(demoID, nil)
	installDemo(t, s, pit{"stickers/d.png", renderPNG(t, 30, 30)})
	installAt(t, s, now, "aaa", t0+10*day, 1)
	installAt(t, s, now, "bbb", t0+20*day, 2)
	if _, err := s.RemovePack(demoID, nil); err != nil {
		t.Fatal(err)
	}
	regs := filepath.Join(cfg, RootName, packsDir)
	for _, id := range []string{"aaa", "bbb"} {
		if _, has := rawRegOf(t, filepath.Join(regs, id+".json"))["base"]; has {
			t.Errorf("уход примера записал base в %s", id)
		}
	}
	s.mu.RLock()
	b, _ := s.packBase(nil)
	s.mu.RUnlock()
	if b != t0+10*day {
		t.Errorf("граница %d сдвинулась", b)
	}
	// Пример вернулся (позже всех), уходит aaa: bbb — не первая партия, граница дописана.
	*now = t0 + 25*day
	installDemo(t, s, pit{"stickers/d.png", renderPNG(t, 30, 30)})
	if _, err := s.RemovePack("aaa", nil); err != nil {
		t.Fatal(err)
	}
	s.mu.RLock()
	b, _ = s.packBase(nil)
	s.mu.RUnlock()
	if b != t0+10*day {
		t.Errorf("после ухода aaa граница %d, ждали прежнюю", b)
	}
}

// LookRefOf: макет есть и в доноре, и в примере — запись на донора, в любом порядке установки.
func TestLookRefPrefersDonor(t *testing.T) {
	for _, demoFirst := range []bool{true, false} {
		s, _ := openTemp(t)
		s.SetDemo(demoID, []string{"wedding-layouts"})
		v1 := lookBytes(t, lookOf("", ""), "")
		donor := func() {
			m, src := testPack("wedding-layouts", 1, pit{"looks/main.json", v1})
			install(t, s, m, src, nil)
		}
		if demoFirst {
			installDemo(t, s, pit{"looks/main.json", v1})
			donor()
		} else {
			donor()
			installDemo(t, s, pit{"looks/main.json", v1})
		}
		ref, ok := s.LookRefOf(model.LookKey(v1))
		if !ok || ref.Pack != "wedding-layouts" || ref.Item != "looks/main.json" || ref.Name == "" {
			t.Errorf("demoFirst=%v: %+v %v", demoFirst, ref, ok)
		}
	}
}

// Только пример: запись на пример; донор без этого макета пример не вытесняет; без SetDemo пример —
// обычный набор.
func TestLookRefDemoOnly(t *testing.T) {
	s, _ := openTemp(t)
	s.SetDemo(demoID, []string{"wedding-layouts"})
	v1 := lookBytes(t, lookOf("", ""), "")
	key := model.LookKey(v1)
	installDemo(t, s, pit{"looks/main.json", v1})
	ref, ok := s.LookRefOf(key)
	if !ok || ref.Pack != demoID || ref.Item != "looks/main.json" {
		t.Fatalf("%+v %v", ref, ok)
	}
	m, src := testPack("wedding-layouts", 1, pit{"looks/other.json", lookBytes(t, lookOf("", ""), `"x":1`)})
	install(t, s, m, src, nil)
	if ref, _ := s.LookRefOf(key); ref.Pack != demoID {
		t.Errorf("донор без макета вытеснил пример: %+v", ref)
	}
	s.SetDemo("", nil)
	if ref, _ := s.LookRefOf(key); ref.Pack != demoID {
		t.Errorf("без SetDemo: %+v", ref)
	}
}

// TestLookRefSurvivesLayoutsV2 — альбом, начатый на примере, находит макет после ухода примера в
// «-layouts» v2 с новым ключом; пока пример стоит — берётся его запись; чужие пути не находятся.
func TestLookRefSurvivesLayoutsV2(t *testing.T) {
	s, _ := openTemp(t)
	s.SetDemo(demoID, []string{"wedding-layouts", "other-layouts"})
	v1 := lookBytes(t, lookOf("", ""), "")
	installDemo(t, s, pit{"looks/main.json", v1})
	ref, ok := s.LookRefOf(model.LookKey(v1))
	if !ok || ref.Pack != demoID {
		t.Fatalf("%+v %v", ref, ok)
	}
	if k, ok := s.LookKeyOf(ref); !ok || k != model.LookKey(v1) {
		t.Fatalf("пока пример стоит: %q %v", k, ok)
	}
	// Пример ушёл, доноров нет — макета нет.
	if _, err := s.RemovePack(demoID, nil); err != nil {
		t.Fatal(err)
	}
	if s.HasLook(ref) {
		t.Fatal("макет найден без примера и доноров")
	}
	// Донор v2: путь тот же, байты другие.
	l2 := lookOf("", "")
	l2.Steps[0].Name = "Начало"
	v2 := lookBytes(t, l2, "")
	m, src := testPack("wedding-layouts", 2, pit{"looks/main.json", v2})
	install(t, s, m, src, nil)
	if k, ok := s.LookKeyOf(ref); !ok || k != model.LookKey(v2) || !s.HasLook(ref) {
		t.Errorf("после ухода примера: %q %v", k, ok)
	}
	if k, ok := s.LookByItem(demoID, "looks/main.json"); !ok || k != model.LookKey(v2) {
		t.Errorf("LookByItem: %q %v", k, ok)
	}
	// Края: другой путь, пустой и мусорный путь, не пример, набор не из related.
	third := lookBytes(t, lookOf("", ""), `"y":2`)
	m3, src3 := testPack("third-layouts", 1, pit{"looks/third.json", third})
	install(t, s, m3, src3, nil)
	for _, q := range [][2]string{{demoID, "looks/other.json"}, {demoID, ""}, {demoID, "../main.json"},
		{"fourth-layouts", "looks/main.json"}, {"", "looks/main.json"}, {demoID, "looks/third.json"}} {
		if k, ok := s.LookByItem(q[0], q[1]); ok {
			t.Errorf("LookByItem%v найден: %q", q, k)
		}
	}
	// Порядок related: первый донор без записи пропускается, второй отдаёт.
	fifth := lookBytes(t, lookOf("", ""), `"z":3`)
	m4, src4 := testPack("other-layouts", 1, pit{"looks/third.json", fifth})
	install(t, s, m4, src4, nil)
	if k, ok := s.LookByItem(demoID, "looks/third.json"); !ok || k != model.LookKey(fifth) {
		t.Errorf("второй донор: %q %v", k, ok)
	}
	// Без SetDemo псевдонима нет.
	s.SetDemo("", nil)
	if s.HasLook(ref) {
		t.Error("без SetDemo ушедший пример найден")
	}
}

// DemoOnly: пустой вид, только пример, донор, другой вид, без SetDemo; поле JSON есть всегда.
func TestCatalogDemoOnly(t *testing.T) {
	s, _ := openTemp(t)
	cat := func(kind string) Catalog {
		t.Helper()
		c, err := s.Catalog(kind, nil, 300, 300)
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	s.SetDemo(demoID, []string{"wedding-layouts"})
	if cat(KindAlbum).DemoOnly {
		t.Error("пустой вид — demoOnly")
	}
	installDemo(t, s, pit{"looks/main.json", lookBytes(t, lookOf("", ""), "")}, pit{"stickers/d.png", renderPNG(t, 30, 30)})
	if c := cat(KindAlbum); !c.DemoOnly || len(c.Items) != 1 {
		t.Errorf("макеты только из примера: %v, плиток %d", c.DemoOnly, len(c.Items))
	}
	if cat(KindSticker).DemoOnly {
		t.Error("у стикеров есть встроенные — не demoOnly")
	}
	raw, _ := json.Marshal(cat(KindAlbum))
	if !strings.Contains(string(raw), `"demoOnly":true`) {
		t.Errorf("JSON: %.200s", raw)
	}
	raw, _ = json.Marshal(cat(KindBg))
	if !strings.Contains(string(raw), `"demoOnly":false`) {
		t.Errorf("JSON без плиток: поле должно быть всегда: %.200s", raw)
	}
	// Донор с другим макетом рядом — уже не только пример.
	m, src := testPack("wedding-layouts", 1, pit{"looks/other.json", lookBytes(t, lookOf("", ""), `"x":1`)})
	install(t, s, m, src, nil)
	if c := cat(KindAlbum); c.DemoOnly || len(c.Items) != 2 {
		t.Errorf("с макетом донора: %v, плиток %d", c.DemoOnly, len(c.Items))
	}
	s.SetDemo("", nil)
	if cat(KindAlbum).DemoOnly {
		t.Error("без SetDemo — demoOnly")
	}
}

// Свой макет рядом с примером — не demoOnly.
func TestCatalogDemoOnlyWithOwnLook(t *testing.T) {
	s, _ := openTemp(t)
	s.SetDemo(demoID, nil)
	installDemo(t, s, pit{"looks/main.json", lookBytes(t, lookOf("", ""), "")})
	savedLook(t, s, "Свой")
	c, err := s.Catalog(KindAlbum, nil, 300, 300)
	if err != nil {
		t.Fatal(err)
	}
	if c.DemoOnly || len(c.Items) != 2 {
		t.Errorf("со своим макетом: %v, плиток %d", c.DemoOnly, len(c.Items))
	}
}
