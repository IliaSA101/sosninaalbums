package decor

import (
	"image"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Порядок каталога 133 (design §3): наборы (новый выше), встроенные, свои. Тесты G1.

// steppedClock — часы библиотеки: каждый вызов на 10 мс позже (step 0 — стоят).
func steppedClock(t *testing.T, step int64) {
	t.Helper()
	clock := int64(1000)
	old := nowMs
	nowMs = func() int64 { clock += step; return clock }
	t.Cleanup(func() { nowMs = old })
}

// catOrder — плитки вида в порядке каталога: «PackID:id», встроенные свёрнуты в «sa».
func catOrder(t *testing.T, s *Store, kind string) []string {
	t.Helper()
	cat, err := s.Catalog(kind, nil, 300, 300)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, it := range cat.Items {
		tag := it.PackID + ":" + it.ID
		if it.PackID == BuiltinPack {
			tag = "sa"
		}
		if len(out) > 0 && tag == "sa" && out[len(out)-1] == "sa" {
			continue
		}
		out = append(out, tag)
	}
	return out
}

func TestCatalogOrderPacksFirst(t *testing.T) {
	t.Run("фоны: набор новее выше, ключ у старшего, свой перекрывает набор", func(t *testing.T) {
		s, cfg := openTemp(t)
		steppedClock(t, 10)
		x, y, z, v, w := renderPNG(t, 71, 30), renderPNG(t, 72, 30), renderPNG(t, 73, 30), renderPNG(t, 74, 30), renderPNG(t, 75, 30)
		mb, sb := testPack("bbb", 1, pit{"bg/z.png", z}, pit{"bg/y.png", y})
		install(t, s, mb, sb, nil)
		ma, sa := testPack("aaa", 1, pit{"bg/y.png", y}, pit{"bg/x.png", x}, pit{"bg/v.png", v})
		install(t, s, ma, sa, nil)
		dir := testkit.TempDir(t)
		ownX := importOK(t, s, "mine", writeFile(t, dir, "x.png", x))[0] // тот же ключ, что у x набора aaa
		ownW := importOK(t, s, "mine", writeFile(t, dir, "w.png", w))[0]
		// aaa новее: v (y — у bbb как старшего, x перекрыт своим); затем bbb: z, y; встроенные; свои новыми первыми.
		want := []string{"aaa:" + libKey(v), "bbb:" + libKey(z), "bbb:" + libKey(y), "sa", ":" + ownW.ID, ":" + ownX.ID}
		if got := catOrder(t, s, KindBg); !slices.Equal(got, want) {
			t.Errorf("порядок:\n%v\nждали\n%v", got, want)
		}
		if got := catOrder(t, Open(cfg), KindBg); !slices.Equal(got, want) { // круг записи и чтения реестров
			t.Errorf("после перезапуска:\n%v", got)
		}
	})

	t.Run("ничья по installed — по id, ключ у того же", func(t *testing.T) {
		s, _ := openTemp(t)
		steppedClock(t, 0)
		y, z, v := renderPNG(t, 72, 30), renderPNG(t, 73, 30), renderPNG(t, 74, 30)
		mb, sb := testPack("bbb", 1, pit{"bg/z.png", z})
		install(t, s, mb, sb, nil)
		ma, sa := testPack("aaa", 1, pit{"bg/y.png", y}, pit{"bg/v.png", v})
		install(t, s, ma, sa, nil)
		mc, sc := testPack("ccc", 1, pit{"bg/y.png", y})
		install(t, s, mc, sc, nil)
		want := []string{"aaa:" + libKey(y), "aaa:" + libKey(v), "bbb:" + libKey(z), "sa"}
		if got := catOrder(t, s, KindBg); !slices.Equal(got, want) {
			t.Errorf("порядок:\n%v\nждали\n%v", got, want)
		}
	})

	t.Run("без наборов — встроенные, затем свои", func(t *testing.T) {
		s, _ := openTemp(t)
		own := importOK(t, s, "mine", writeFile(t, testkit.TempDir(t), "a.png", renderPNG(t, 70, 30)))[0]
		if got := catOrder(t, s, KindBg); !slices.Equal(got, []string{"sa", ":" + own.ID}) {
			t.Errorf("порядок %v", got)
		}
	})

	t.Run("стикеры и рамочки", func(t *testing.T) {
		s, _ := openTemp(t)
		steppedClock(t, 10)
		ring := pngBytes(t, ringImage(120, 160, 24, 32, 24, 32))
		open := pngBytes(t, cornersImage(100, 100, 20))
		st1, st2 := renderPNG(t, 16, 16), renderPNG(t, 18, 18)
		m1, s1 := testPack("first", 1, pit{"stickers/a.png", st1}, pit{"frames/r.png", ring})
		install(t, s, m1, s1, nil)
		m2, s2 := testPack("second", 1, pit{"stickers/b.png", st2}, pit{"frames/o.png", open})
		install(t, s, m2, s2, nil)
		if got, want := catOrder(t, s, KindSticker), []string{"second:" + libKey(st2), "first:" + libKey(st1), "sa"}; !slices.Equal(got, want) {
			t.Errorf("стикеры:\n%v\nждали\n%v", got, want)
		}
		if got, want := catOrder(t, s, KindFrame), []string{"second:" + libKey(open), "first:" + libKey(ring), "sa"}; !slices.Equal(got, want) {
			t.Errorf("рамочки:\n%v\nждали\n%v", got, want)
		}
	})

	t.Run("развороты: набор выше встроенных, свой последний", func(t *testing.T) {
		s, _ := openTemp(t)
		steppedClock(t, 10)
		box := []model.Box{{X: .1, Y: .1, W: .8, H: .8}}
		tpl := func(c string) []byte {
			return tplJSON(t, model.SpreadTpl{V: 1, Pages: 1, Ratio: 1, Places: box, Page: []model.TplBg{{Color: c}}})
		}
		t1, t2 := tpl("#eeeeee"), tpl("#dddddd")
		m1, s1 := testPack("old", 1, pit{"spreads/a.json", t1})
		install(t, s, m1, s1, nil)
		m2, s2 := testPack("new", 1, pit{"spreads/b.json", t2})
		install(t, s, m2, s2, nil)
		own, err := s.SaveSpreadTpl("mine", "Свой", model.SpreadTpl{V: 1, Pages: 1, Ratio: 1, Places: box, Page: []model.TplBg{{Color: "#cccccc"}}})
		if err != nil {
			t.Fatal(err)
		}
		k1, k2 := model.TplKey(mustTpl(t, t1)), model.TplKey(mustTpl(t, t2))
		want := []string{"new:" + k2, "old:" + k1, "sa", ":" + own.ID}
		if got := catOrder(t, s, KindSpread); !slices.Equal(got, want) {
			t.Errorf("порядок:\n%v\nждали\n%v", got, want)
		}
	})
}

func TestItemPackIDPeek(t *testing.T) {
	s, cfg := openTemp(t)
	dir := testkit.TempDir(t)
	ring := pngBytes(t, ringImage(120, 160, 24, 32, 24, 32))
	box := []model.Box{{X: .1, Y: .1, W: .8, H: .8}}
	tplB := tplJSON(t, model.SpreadTpl{V: 1, Pages: 1, Ratio: 1, Places: box, Page: []model.TplBg{{Color: "#eeeeee"}}})
	bg, st := renderPNG(t, 40, 30), renderPNG(t, 16, 16)
	packLook := lookOf("", "") // макет набора отличается от своего именем шага — ключи разные
	packLook.Steps[0].Name = "Из набора"
	lookB := lookBytes(t, packLook, "")
	m, src := testPack("ivory", 1, pit{"bg/one.png", bg}, pit{"stickers/dot.png", st}, pit{"frames/r.png", ring}, pit{"spreads/t.json", tplB}, pit{"looks/l.json", lookB})
	install(t, s, m, src, nil)
	// Свои: фон, стикер, рамочка и шаблон.
	ownBg := importOK(t, s, "mine", writeFile(t, dir, "bg.png", renderPNG(t, 50, 30)))[0]
	own := map[string]Item{KindBg: ownBg}
	for kind, data := range map[string][]byte{KindSticker: renderPNG(t, 20, 20), KindFrame: pngBytes(t, cornersImage(100, 100, 20))} {
		res, err := s.Import(kind, "mine", []string{writeFile(t, dir, kind+".png", data)})
		if err != nil || len(res.Added) != 1 {
			t.Fatalf("%s: %v %+v", kind, err, res)
		}
		own[kind] = res.Added[0]
	}
	tpl, err := s.SaveSpreadTpl("mine", "Свой", model.SpreadTpl{V: 1, Pages: 1, Ratio: 1, Places: box, Page: []model.TplBg{{Color: "#cccccc"}}})
	if err != nil {
		t.Fatal(err)
	}
	own[KindSpread] = tpl
	// Макет альбома: свой; peek у своего и набора пуст.
	ownLook, err := s.SaveLook("mine", "Свой макет", lookOf("", ""))
	if err != nil {
		t.Fatal(err)
	}
	own[KindAlbum] = ownLook
	packKey := map[string]string{KindBg: libKey(bg), KindSticker: libKey(st), KindFrame: libKey(ring), KindSpread: model.TplKey(mustTpl(t, tplB)), KindAlbum: model.LookKey(lookB)}

	checkOwners(t, s, "после установки", own, packKey)
	checkOwners(t, Open(cfg), "после перезапуска", own, packKey)
	// Peek — адрес, который отдаёт /media/lib/p/: ключ в реестре, префикс как у LibURL.
	if got := own[KindBg].Peek; !strings.HasPrefix(got, LibPath+"p/") || !strings.HasSuffix(got, "?v=1") {
		t.Errorf("адрес %q", got)
	}
	if _, ok := s.registered(ownBg.ID); !ok {
		t.Error("ключ своего не в реестре /media/lib")
	}
	// Peek отдаётся обработчиком: своё и предмет набора — 200 с картинкой; у JSON (шаблон, макет)
	// peek нет, а запрос по ключу макета — 404.
	cache, err := library.NewCache(testkit.TempDir(t))
	if err != nil {
		t.Fatal(err)
	}
	testkit.BeforeRemove(t, cache.Close)
	h := s.Handler(cache)
	for _, it := range []Item{own[KindBg], findPackItem(t, s, KindBg, packKey[KindBg])} {
		path, query, _ := strings.Cut(it.Peek, "?")
		rec := get(h, http.MethodGet, path, query+"&px=256")
		if _, _, err := image.Decode(rec.Body); rec.Code != http.StatusOK || err != nil {
			t.Errorf("peek %s: %d %v", it.ID, rec.Code, err)
		}
	}
	for _, k := range []string{packKey[KindAlbum], ownLook.ID} {
		if _, ok := s.registered(k); ok { // JSON макета не картинка: ReadInfo до него не доходит
			t.Errorf("макет %s в реестре картинок", k)
		}
		if rec := get(h, http.MethodGet, LibPath+"p/"+strings.TrimPrefix(k, libPrefix), "v=1"); rec.Code != http.StatusNotFound {
			t.Errorf("peek макета %s: %d", k, rec.Code)
		}
	}
}

// checkOwners — у каждой плитки каждого вида верный PackID и Peek: встроенные, один предмет набора
// ivory и одна своя плитка на вид.
func checkOwners(t *testing.T, s *Store, when string, own map[string]Item, packKey map[string]string) {
	t.Helper()
	for _, kind := range []string{KindBg, KindSticker, KindFrame, KindSpread, KindAlbum} {
		cat, err := s.Catalog(kind, nil, 300, 300)
		if err != nil {
			t.Fatal(err)
		}
		checkKindOwners(t, when, kind, cat.Items, own[kind].ID, packKey[kind])
	}
}

// checkKindOwners — плитки одного вида: peek только у картинок (у JSON шаблона и макета — пусто).
func checkKindOwners(t *testing.T, when, kind string, items []Item, ownID, packID string) {
	t.Helper()
	peek := func(key string) string {
		if kind == KindSpread || kind == KindAlbum {
			return ""
		}
		return LibURL(key, true)
	}
	var builtin, pack, mine int
	for _, it := range items {
		switch {
		case it.PackID == BuiltinPack:
			builtin++
			if it.Peek != "" || it.Pack != "" || it.Own {
				t.Errorf("%s %s встроенный %s: %+v", when, kind, it.ID, it)
			}
		case it.PackID == "ivory":
			pack++
			if it.ID != packID || it.Peek != peek(it.ID) || it.Pack != "Набор ivory" {
				t.Errorf("%s %s набор: %+v", when, kind, it)
			}
		case it.PackID == "" && it.Own:
			mine++
			if it.ID != ownID || it.Peek != peek(it.ID) {
				t.Errorf("%s %s свой: %+v", when, kind, it)
			}
		default:
			t.Errorf("%s %s: плитка без владельца %+v", when, kind, it)
		}
	}
	if (builtin == 0) == (kind != KindAlbum) || pack != 1 || mine != 1 { // у макетов встроенных нет
		t.Errorf("%s %s: встроенных %d, набора %d, своих %d", when, kind, builtin, pack, mine)
	}
}

// findPackItem — плитка каталога по ключу (предмет набора).
func findPackItem(t *testing.T, s *Store, kind, key string) Item {
	t.Helper()
	it, ok := findItem(t, s, kind, key)
	if !ok {
		t.Fatalf("%s: плитки %s нет", kind, key)
	}
	return it
}
