package decor

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// fakeSrc — архив набора в памяти: предмет i — байты; nil — битая запись (Extract — ошибка).
type fakeSrc [][]byte

func (f fakeSrc) Extract(i int, w io.Writer) error {
	if f[i] == nil {
		return errors.New("архив набора повреждён: «x» не сходится с манифестом (sha256)")
	}
	_, err := w.Write(f[i])
	return err
}

// pit — предмет тестового набора.
type pit struct {
	file string
	data []byte
}

// kindOfFile — вид по папке, как в наборах скрипта D1.
func kindOfFile(file string) string {
	switch strings.Split(file, "/")[0] {
	case "bg":
		return KindBg
	case "stickers":
		return KindSticker
	}
	return KindSpread
}

func testPack(id string, version int, items ...pit) (PackManifest, fakeSrc) {
	m := PackManifest{ID: id, Version: version, Title: "Набор " + id, License: "CC0-1.0"}
	src := make(fakeSrc, len(items))
	for i, it := range items {
		m.Items = append(m.Items, PackItem{File: it.file, Kind: kindOfFile(it.file), Theme: "wedding", Name: it.file, Size: int64(len(it.data))})
		src[i] = it.data
	}
	return m, src
}

func libKey(b []byte) string {
	s := sha256.Sum256(b)
	return "my:" + hex.EncodeToString(s[:8])
}

func tplJSON(t *testing.T, tpl model.SpreadTpl) []byte {
	t.Helper()
	b, err := json.Marshal(tpl)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// install — набор целиком без пропусков.
func install(t *testing.T, s *Store, m PackManifest, src fakeSrc, keep func(string) bool) PackInfo {
	t.Helper()
	st, err := s.StagePack(m, src, OriginFile, strings.Repeat("ab", 32))
	if err != nil || st.Ask != "" || len(st.Skipped) != 0 {
		t.Fatalf("StagePack %s: %v %+v", m.ID, err, st)
	}
	defer st.Close()
	info, err := s.InstallPack(st, keep)
	if err != nil {
		t.Fatalf("InstallPack %s: %v", m.ID, err)
	}
	return info
}

func catIDs(t *testing.T, s *Store, kind string) []string {
	t.Helper()
	cat, err := s.Catalog(kind, nil, 300, 300)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, it := range cat.Items {
		if !strings.HasPrefix(it.ID, presetPrefix) {
			ids = append(ids, it.ID+"|"+it.Pack)
		}
	}
	return ids
}

func noTmpAnywhere(t *testing.T, cfg string) {
	t.Helper()
	for _, d := range []string{"Фоны", "Стикеры", kindDirs[KindSpread], packsDir} {
		assertNoTmp(t, filepath.Join(cfg, RootName, d))
	}
}

func TestInstallPack(t *testing.T) {
	s, cfg := openTemp(t)
	bg, st := renderPNG(t, 40, 30), renderPNG(t, 16, 16)
	tpl := model.SpreadTpl{V: 1, Pages: 1, Places: []model.Box{{X: .1, Y: .1, W: .8, H: .8}},
		Page: []model.TplBg{{Lib: libKey(bg)}}, Stickers: []model.TplItem{{W: .2, H: .2, Lib: libKey(st)}}}
	m, src := testPack("ivory", 3, pit{"bg/one.png", bg}, pit{"stickers/dot.png", st}, pit{"spreads/hero.json", tplJSON(t, tpl)})
	info := install(t, s, m, src, nil)
	if info.ID != "ivory" || info.Version != 3 || info.Bg != 1 || info.Sticker != 1 || info.Spread != 1 || !info.Removable ||
		info.Origin != OriginFile || info.Bytes == 0 {
		t.Fatalf("итог: %+v", info)
	}
	root := filepath.Join(cfg, RootName)
	for _, p := range []string{
		filepath.Join(root, "Фоны", strings.TrimPrefix(libKey(bg), "my:")+".png"),
		filepath.Join(root, "Стикеры", strings.TrimPrefix(libKey(st), "my:")+".png"),
		filepath.Join(root, packsDir, "ivory.json"),
	} {
		if !regularFile(p) {
			t.Errorf("нет %s", p)
		}
	}
	noTmpAnywhere(t, cfg)
	check := func(s *Store, when string) {
		t.Helper()
		if got := catIDs(t, s, KindBg); !slices.Equal(got, []string{libKey(bg) + "|Набор ivory"}) {
			t.Errorf("%s: каталог фонов %v", when, got)
		}
		if got := catIDs(t, s, KindSticker); !slices.Equal(got, []string{libKey(st) + "|Набор ivory"}) {
			t.Errorf("%s: каталог стикеров %v", when, got)
		}
		if b, err := s.Background(libKey(bg)); err != nil || b.Photo.Lib != libKey(bg) || b.Photo.W != 40 {
			t.Errorf("%s: фон набора: %v %+v", when, err, b)
		}
		if sk, err := s.Sticker(libKey(st)); err != nil || sk.Photo.Lib != libKey(st) {
			t.Errorf("%s: стикер набора: %v", when, err)
		}
		if _, ok := s.registered(libKey(bg)); !ok {
			t.Errorf("%s: /media/lib не отдаёт фон набора", when)
		}
		if _, ok := s.Find(libKey(st)); !ok {
			t.Errorf("%s: Find не находит стикер", when)
		}
		ps := s.Packs()
		if len(ps) != 2 || ps[0].ID != BuiltinPack || ps[0].Removable || ps[0].Bg == 0 || ps[1] != info {
			t.Errorf("%s: наборы %+v", when, ps)
		}
		s.mu.RLock()
		_, tplOK := s.files[KindSpread][model.TplKey(mustTpl(t, tplJSON(t, tpl)))]
		s.mu.RUnlock()
		if !tplOK {
			t.Errorf("%s: шаблон набора не в индексе", when)
		}
	}
	check(s, "после установки")
	check(Open(cfg), "после перезапуска") // круг записи и чтения реестра
	// Та же версия ещё раз — «уже есть», без распаковки.
	again, err := s.StagePack(m, src, OriginFile, "")
	if err != nil || again.Ask != AskSame || again.From != 3 || len(again.items) != 0 || again.Info().Bg != 1 {
		t.Fatalf("повтор: %v %+v", err, again)
	}
	if _, err := s.InstallPack(again, nil); err == nil {
		t.Error("повтор поставился")
	}
	noTmpAnywhere(t, cfg)
}

func mustTpl(t *testing.T, b []byte) model.SpreadTpl {
	t.Helper()
	tpl, err := model.ParseSpreadTpl(b)
	if err != nil {
		t.Fatal(err)
	}
	return tpl
}

// Сбой на N-м предмете — реестр не записан, новых файлов нет, уже стоявший файл цел.
func TestInstallPackAtomic(t *testing.T) {
	s, cfg := openTemp(t)
	a, b, c := renderPNG(t, 41, 30), renderPNG(t, 42, 30), renderPNG(t, 43, 30)
	src := testkit.TempDir(t)
	own := importOK(t, s, "mine", writeFile(t, src, "own.png", a))[0]
	m, fs := testPack("ivory", 1, pit{"bg/a.png", a}, pit{"bg/b.png", b}, pit{"bg/c.png", c})
	st, err := s.StagePack(m, fs, OriginFile, "")
	if err != nil {
		t.Fatal(err)
	}
	old := packStep
	packStep = func(i int) error {
		if i == 2 {
			return errors.New("диск отвалился")
		}
		return nil
	}
	t.Cleanup(func() { packStep = old })
	if _, err := s.InstallPack(st, nil); err == nil || !strings.Contains(err.Error(), "не записался") {
		t.Fatalf("сбой: %v", err)
	}
	st.Close()
	root := filepath.Join(cfg, RootName)
	if regularFile(filepath.Join(root, packsDir, "ivory.json")) || len(s.Packs()) != 1 {
		t.Error("реестр набора записан при сбое")
	}
	list, _ := os.ReadDir(filepath.Join(root, "Фоны"))
	if len(list) != 1 || list[0].Name() != strings.TrimPrefix(own.ID, "my:")+".png" {
		t.Errorf("в папке фонов после отката: %v", list)
	}
	if got := catIDs(t, s, KindBg); !slices.Equal(got, []string{own.ID + "|"}) {
		t.Errorf("каталог после отката: %v", got)
	}
	// Без сбоя тот же набор ставится; своё остаётся своим (одна плитка, ✎ и ×).
	packStep = old
	install(t, s, m, fs, nil)
	if got := catIDs(t, s, KindBg); !slices.Equal(got, []string{libKey(b) + "|Набор ivory", libKey(c) + "|Набор ivory", own.ID + "|"}) {
		t.Errorf("каталог: %v", got)
	}
}

// Плохие предметы пропускаются с причиной; битая запись — отказ целиком без временных файлов.
func TestPackStageSkips(t *testing.T) {
	s, cfg := openTemp(t)
	dir := testkit.TempDir(t)
	good := renderPNG(t, 44, 30)
	jpeg := mustRead(t, testkit.JPEG(t, dir, "a.jpg", 20, 10))
	huge := mustRead(t, testkit.HugeJPEG(t, dir, "h.jpg", 12000, 9000))
	tpl := func(mut func(*model.SpreadTpl)) []byte {
		v := model.SpreadTpl{V: 1, Pages: 1, Places: []model.Box{{X: .1, Y: .1, W: .5, H: .5}}, Page: []model.TplBg{{}}}
		mut(&v)
		return tplJSON(t, v)
	}
	items := []pit{
		{"bg/good.png", good},
		{"bg/svg.png", []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`)},
		{"bg/jpeg.png", jpeg},
		{"bg/huge.jpg", huge},
		{"stickers/long.png", renderPNG(t, 4097, 8)},
		{"bg/again.png", good},
		{"spreads/new-art.json", tpl(func(v *model.SpreadTpl) {
			v.Stickers = []model.TplItem{{W: .2, H: .2, Art: &model.Art{Kind: "zzz-future"}}}
		})},
		{"spreads/new-pat.json", tpl(func(v *model.SpreadTpl) { v.Page[0].Pat = &model.Pattern{Kind: "zzz-future"} })},
		{"spreads/foreign.json", tpl(func(v *model.SpreadTpl) { v.Page[0].Lib = "my:0123456789abcdef" })},
		{"spreads/wrongkind.json", tpl(func(v *model.SpreadTpl) { v.Stickers = []model.TplItem{{W: .2, H: .2, Lib: libKey(good)}} })},
		{"spreads/v2.json", []byte(`{"v":2,"pages":1,"page":[{}]}`)},
		{"spreads/junk.json", []byte(`{не json`)},
		{"spreads/ok.json", tpl(func(v *model.SpreadTpl) { v.Page[0].Lib = libKey(good) })},
		{"stickers/ok.png", renderPNG(t, 4096, 8)},
	}
	m, src := testPack("skips", 1, items...)
	m.Items = append(m.Items, PackItem{File: "x/frame.bin", Kind: "frame", Size: 1})
	src = append(src, []byte{1})
	st, err := s.StagePack(m, src, OriginNet, "")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	var why []string
	for _, k := range st.Skipped {
		why = append(why, k.Name+": "+k.Why)
	}
	want := []string{"bg/svg.png: файл не читается как JPEG или PNG", "bg/jpeg.png: содержимое не совпадает с расширением файла",
		"bg/huge.jpg: фон больше 100 Мп", "stickers/long.png: стикер больше 4096 px по стороне", "bg/again.png: повтор в наборе",
		"spreads/new-art.json: шаблон из новой версии программы", "spreads/new-pat.json: шаблон из новой версии программы",
		"spreads/v2.json: шаблон из новой версии программы", "spreads/junk.json: шаблон разворота не читается",
		"x/frame.bin: незнакомый вид — нужна новая версия программы",
		"spreads/foreign.json: шаблон ссылается на фон или стикер не из набора",
		"spreads/wrongkind.json: шаблон ссылается на фон или стикер не из набора"}
	if !slices.Equal(why, want) {
		t.Errorf("пропуски:\n%s", strings.Join(why, "\n"))
	}
	if len(st.items) != 3 {
		t.Fatalf("прошли %d", len(st.items))
	}
	info, err := s.InstallPack(st, nil)
	if err != nil || info.Bg != 1 || info.Spread != 1 || info.Sticker != 1 || info.Origin != OriginNet {
		t.Fatalf("установка: %v %+v", err, info)
	}
	noTmpAnywhere(t, cfg)

	// Все плохие — ErrPackEmpty с причинами; битая запись — ошибка; временных не остаётся.
	m2, src2 := testPack("bad-all", 1, pit{"bg/svg.png", []byte("<svg/>")})
	if st2, err := s.StagePack(m2, src2, OriginFile, ""); !errors.Is(err, ErrPackEmpty) || len(st2.Skipped) != 1 {
		t.Errorf("все плохие: %v", err)
	}
	m3, src3 := testPack("broken", 1, pit{"bg/a.png", renderPNG(t, 45, 30)}, pit{"bg/b.png", nil})
	if _, err := s.StagePack(m3, src3, OriginFile, ""); err == nil || !strings.Contains(err.Error(), "sha256") {
		t.Errorf("битая запись: %v", err)
	}
	if _, err := s.StagePack(PackManifest{ID: "../x", Version: 1}, fakeSrc{}, OriginFile, ""); err == nil {
		t.Error("id ../x принят")
	}
	noTmpAnywhere(t, cfg)
	if len(s.Packs()) != 2 {
		t.Errorf("наборы: %+v", s.Packs())
	}
}

// Квота 2 ГБ — по размерам манифеста до распаковки; реестр новее — только чтение.
func TestPackStageLimits(t *testing.T) {
	_, cfg := openTemp(t)
	var items []string
	for i := range 11 { // 11 × 200 МБ по реестру — больше 2 ГБ
		items = append(items, fmt.Sprintf(`{"key":"my:%016x","kind":"bg","bytes":%d}`, i, MaxBgBytes))
	}
	writeReg(t, cfg, `{"version":1,"items":[`+strings.Join(items, ",")+`]}`)
	s := Open(cfg)
	m, src := testPack("full", 1, pit{"bg/a.png", renderPNG(t, 46, 30)})
	if _, err := s.StagePack(m, src, OriginFile, ""); !errors.Is(err, errLibFull) {
		t.Errorf("квота: %v", err)
	}
	writeReg(t, cfg, `{"version":99,"items":[]}`)
	s = Open(cfg)
	if _, err := s.StagePack(m, src, OriginFile, ""); !errors.Is(err, ErrReadOnly) {
		t.Errorf("только чтение: %v", err)
	}
	if _, err := s.RemovePack("full", nil); !errors.Is(err, ErrReadOnly) {
		t.Errorf("удаление в только чтении: %v", err)
	}
}

// Обновление: новые ключи ставятся, ключи старой версии, которых нет в новой, удаляются;
// место набора в каталоге прежнее; старая версия поверх новой — «уже стоит новее».
func TestUpdatePack(t *testing.T) {
	s, cfg := openTemp(t)
	clock := int64(1000)
	old := nowMs
	nowMs = func() int64 { clock++; return clock }
	t.Cleanup(func() { nowMs = old })
	a, b, c, k := renderPNG(t, 51, 30), renderPNG(t, 52, 30), renderPNG(t, 53, 30), renderPNG(t, 54, 30)
	m1, s1 := testPack("ivory", 1, pit{"bg/a.png", a}, pit{"bg/b.png", b}, pit{"bg/k.png", k})
	install(t, s, m1, s1, nil)
	mz, sz := testPack("zeta", 1, pit{"bg/c0.png", renderPNG(t, 55, 30)})
	install(t, s, mz, sz, nil)
	m2, s2 := testPack("ivory", 2, pit{"bg/b.png", b}, pit{"bg/c.png", c})
	st, err := s.StagePack(m2, s2, OriginNet, "")
	if err != nil || st.Ask != AskUpdate || st.From != 1 || len(st.items) != 2 {
		t.Fatalf("вопрос обновления: %v %+v", err, st)
	}
	defer st.Close()
	info, err := s.InstallPack(st, func(key string) bool { return key == libKey(k) }) // k — в открытом альбоме
	if err != nil || info.Version != 2 || info.Bg != 2 {
		t.Fatalf("обновление: %v %+v", err, info)
	}
	root := filepath.Join(cfg, RootName, "Фоны")
	for key, want := range map[string]bool{libKey(a): false, libKey(b): true, libKey(c): true, libKey(k): true} {
		if got := regularFile(filepath.Join(root, strings.TrimPrefix(key, "my:")+".png")); got != want {
			t.Errorf("файл %s есть=%v, ждали %v", key, got, want)
		}
	}
	ps := s.Packs()
	if len(ps) != 3 || ps[1].ID != "ivory" || ps[1].Version != 2 || ps[2].ID != "zeta" {
		t.Errorf("порядок наборов после обновления: %+v", ps)
	}
	if ps := Open(filepath.Dir(filepath.Dir(root))).Packs(); ps[1].ID != "ivory" || ps[1].Origin != OriginNet {
		t.Errorf("после перезапуска: %+v", ps)
	}
	// Старая версия поверх новой.
	if st, err := s.StagePack(m1, s1, OriginFile, ""); err != nil || st.Ask != AskOlder || st.From != 2 {
		t.Errorf("старее: %v %+v", err, st)
	}
}

// Удаление: держат другой набор, своё, скрытое своё и открытый альбом; числа плана = итогу.
func TestRemovePackGC(t *testing.T) {
	s, cfg := openTemp(t)
	dir := testkit.TempDir(t)
	a, b, c, d, e := renderPNG(t, 61, 30), renderPNG(t, 62, 30), renderPNG(t, 63, 30), renderPNG(t, 64, 30), renderPNG(t, 65, 30)
	m, src := testPack("ivory", 1, pit{"bg/a.png", a}, pit{"bg/b.png", b}, pit{"bg/c.png", c}, pit{"bg/d.png", d}, pit{"bg/e.png", e},
		pit{"spreads/t.json", tplJSON(t, model.SpreadTpl{V: 1, Pages: 1, Page: []model.TplBg{{Lib: libKey(a)}}})})
	install(t, s, m, src, nil)
	m2, src2 := testPack("other", 1, pit{"bg/b.png", b})
	install(t, s, m2, src2, nil)
	importOK(t, s, "mine", writeFile(t, dir, "c.png", c))
	hid := importOK(t, s, "mine", writeFile(t, dir, "d.png", d))[0]
	if err := s.Hide(KindBg, hid.ID); err != nil {
		t.Fatal(err)
	}
	keep := func(key string) bool { return key == libKey(e) }
	plan, err := s.PlanRemovePack("ivory", keep)
	if err != nil || plan.Files != 2 || plan.Kept != 4 || plan.Bytes != int64(len(a))+int64(len(tplJSON(t, mustTpl(t, src[5])))) {
		t.Fatalf("план: %v %+v", err, plan)
	}
	if again, _ := s.PlanRemovePack("ivory", keep); again != plan || len(s.Packs()) != 3 {
		t.Error("план что-то поменял")
	}
	res, err := s.RemovePack("ivory", keep)
	if err != nil || res != plan {
		t.Fatalf("удаление: %v %+v, план %+v", err, res, plan)
	}
	root := filepath.Join(cfg, RootName)
	for key, want := range map[string]bool{libKey(a): false, libKey(b): true, libKey(c): true, libKey(d): true, libKey(e): true} {
		if got := regularFile(filepath.Join(root, "Фоны", strings.TrimPrefix(key, "my:")+".png")); got != want {
			t.Errorf("файл %s есть=%v, ждали %v", key, got, want)
		}
	}
	list, _ := os.ReadDir(filepath.Join(root, kindDirs[KindSpread]))
	if len(list) != 0 || regularFile(filepath.Join(root, packsDir, "ivory.json")) {
		t.Errorf("шаблон или реестр остались: %v", list)
	}
	want := []string{libKey(b) + "|Набор other", libKey(c) + "|"} // скрытое своё не видно
	if got := catIDs(t, s, KindBg); !slices.Equal(got, want) {
		t.Errorf("каталог: %v", got)
	}
	if got := catIDs(t, Open(cfg), KindBg); !slices.Equal(got, want) {
		t.Errorf("каталог после перезапуска: %v", got)
	}
	// Своё, лежавшее у альбома (e), набор больше не показывает, но Find находит — альбом цел.
	if _, ok := s.Find(libKey(e)); !ok {
		t.Error("файл открытого альбома не находится")
	}
	if _, err := s.RemovePack("ivory", nil); !errors.Is(err, ErrNotFound) {
		t.Errorf("второй раз: %v", err)
	}
	if _, err := s.RemovePack(BuiltinPack, nil); !errors.Is(err, ErrBuiltin) {
		t.Errorf("встроенный: %v", err)
	}
	if _, err := s.PlanRemovePack(BuiltinPack, nil); !errors.Is(err, ErrBuiltin) {
		t.Errorf("план встроенного: %v", err)
	}
}

// Каталог: встроенные → наборы по времени установки (внутри — порядок манифеста) → свои
// новыми первыми; ключ в двух наборах — одна плитка, свой с тем же ключом — плитка своего.
func TestCatalogPacksOrder(t *testing.T) {
	s, _ := openTemp(t)
	clock := int64(1000)
	old := nowMs
	nowMs = func() int64 { clock += 10; return clock }
	t.Cleanup(func() { nowMs = old })
	x, y, z, w := renderPNG(t, 71, 30), renderPNG(t, 72, 30), renderPNG(t, 73, 30), renderPNG(t, 74, 30)
	mb, sb := testPack("bbb", 1, pit{"bg/z.png", z}, pit{"bg/y.png", y})
	install(t, s, mb, sb, nil)
	ma, sa := testPack("aaa", 1, pit{"bg/y.png", y}, pit{"bg/x.png", x})
	install(t, s, ma, sa, nil)
	dir := testkit.TempDir(t)
	ownX := importOK(t, s, "mine", writeFile(t, dir, "x.png", x))[0]
	ownW := importOK(t, s, "mine", writeFile(t, dir, "w.png", w))[0]
	want := []string{libKey(z) + "|Набор bbb", libKey(y) + "|Набор bbb", ownW.ID + "|", ownX.ID + "|"}
	if got := catIDs(t, s, KindBg); !slices.Equal(got, want) {
		t.Errorf("порядок:\n%v\nждали\n%v", got, want)
	}
	cat, _ := s.Catalog(KindBg, nil, 300, 300)
	n := 0
	for _, it := range cat.Items {
		if it.Pack != "" {
			n++
			if it.Own || it.Theme != "wedding" || it.URL != LibURL(it.ID, false) || it.Aspect <= 0 {
				t.Errorf("плитка набора %+v", it)
			}
		}
	}
	if n != 2 || cat.Themes[len(cat.Themes)-1].Count != 2 {
		t.Errorf("плиток наборов %d, темы %+v", n, cat.Themes)
	}
	// Скрытое своё — плитка набора возвращается (набор держит файл).
	if err := s.Hide(KindBg, ownX.ID); err != nil {
		t.Fatal(err)
	}
	if got := catIDs(t, s, KindBg); !slices.Contains(got, libKey(x)+"|Набор aaa") {
		t.Errorf("после скрытия своего: %v", got)
	}
}

// Реестр набора правят руками: битый и чужой id — пропуск, остатки *.tmp убираются,
// записи зажимаются (ключ, вид, тема, имя, байты).
func TestPackRegistryLoad(t *testing.T) {
	_, cfg := openTemp(t)
	dir := filepath.Join(cfg, RootName, packsDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"bad.json":         "{битый",
		"other.json":       `{"v":1,"id":"ivory","items":[]}`,
		"novers.json":      `{"id":"novers","items":[]}`,
		"BIG.json":         `{"v":1,"id":"BIG"}`,
		".dl-ivory.tmp":    "часть",
		"ok.json.123.tmp":  "{",
		"clamp.json":       `{"v":1,"id":"clamp","version":-5,"title":"","origin":"evil","items":[{"key":"my:0123456789abcdef","kind":"bg","theme":"mine","name":"\u0000","w":-1,"h":999999999999,"bytes":99999999999999},{"key":"../x","kind":"bg"},{"key":"my:0123456789abcdef","kind":"bg"},{"key":"my:0123456789abcdef","kind":"frame"}]}`,
		"good-pack.json":   `{"v":1,"id":"good-pack","version":2,"title":"Хороший","origin":"net","installed":5,"items":[]}`,
		"good-pack.json.x": "{",
	}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	ps := Open(cfg).Packs()
	if len(ps) != 3 || ps[1].ID != "clamp" || ps[2].ID != "good-pack" || ps[2].Origin != OriginNet || ps[2].Version != 2 {
		t.Fatalf("наборы %+v", ps)
	}
	c := ps[1]
	if c.Version != 1 || c.Title != "clamp" || c.Origin != OriginFile || c.Bg != 1 || c.Bytes != MaxBgBytes {
		t.Errorf("зажим %+v", c)
	}
	for _, name := range []string{".dl-ivory.tmp", "ok.json.123.tmp"} {
		if regularFile(filepath.Join(dir, name)) {
			t.Errorf("не убран %s", name)
		}
	}
	reg, err := readPackReg(filepath.Join(dir, "clamp.json"), "clamp")
	if err != nil || reg.Items[0].Theme != "basic" || reg.Items[0].Name != noName || reg.Items[0].W != 0 {
		t.Errorf("запись: %v %+v", err, reg)
	}
}
