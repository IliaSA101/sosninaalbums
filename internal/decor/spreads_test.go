package decor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/decor/gen"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// tplN — свой шаблон-страница с n местами и цветом: разные n — разные ключи.
func tplN(n int) model.SpreadTpl {
	t := model.SpreadTpl{V: model.TplV, Pages: 1, Ratio: 1, Page: []model.TplBg{{Color: "#ffeedd"}}}
	for i := range n {
		t.Places = append(t.Places, model.Box{X: 0.1, Y: float64(i) / float64(n+1), W: 0.5, H: 0.5 / float64(n+1)})
	}
	return t
}

func spreadItems(t *testing.T, s *Store) []Item {
	t.Helper()
	cat, err := s.Catalog(KindSpread, nil, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	return cat.Items
}

func ownSpreads(t *testing.T, s *Store) []Item {
	t.Helper()
	var out []Item
	for _, it := range spreadItems(t, s) {
		if it.Own {
			out = append(out, it)
		}
	}
	return out
}

// TestSpreadTplBuiltin — сторож встроенных: каждый разбирается, зажим не меняет файл (файл уже
// канонический), генераторы известны сборке, своих ключей нет; ID и файлы не повторяются, лишних
// файлов в spreads/ нет; плитки — в каталоге с темами окна.
func TestSpreadTplBuiltin(t *testing.T) {
	if len(spreadPresets) != 12 {
		t.Fatalf("встроенных %d, по спеке 12", len(spreadPresets))
	}
	seen := map[string]bool{}
	files := map[string]bool{}
	for _, p := range spreadPresets {
		if seen[p.id] || files[p.file] || !strings.HasPrefix(p.id, "sa:tpl-") || !isTheme(p.theme) || p.name == "" {
			t.Errorf("пресет %+v", p)
		}
		seen[p.id], files[p.file] = true, true
		checkBuiltin(t, p)
	}
	list, err := spreadFS.ReadDir("spreads")
	if err != nil {
		t.Fatal(err)
	}
	for _, de := range list {
		if !files[strings.TrimSuffix(de.Name(), ".json")] {
			t.Errorf("лишний файл spreads/%s", de.Name())
		}
	}
	s, _ := openTemp(t)
	items := spreadItems(t, s)
	if len(items) != len(spreadPresets) {
		t.Fatalf("плиток %d", len(items))
	}
	for i, it := range items {
		if it.ID != spreadPresets[i].id || it.Own || it.URL != TplPath+it.ID+"?v=1" || it.Pages < 1 || it.Places < 1 || !(it.Aspect > 0) {
			t.Errorf("плитка %+v", it)
		}
	}
}

// checkBuiltin — файл встроенного: канонический, генераторы известны, своих ключей нет, места есть.
func checkBuiltin(t *testing.T, p spreadPreset) {
	t.Helper()
	data, err := spreadFS.ReadFile("spreads/" + p.file + ".json")
	if err != nil {
		t.Fatal(err)
	}
	var raw model.SpreadTpl
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("%s: %v", p.id, err)
	}
	got, err := model.ParseSpreadTpl(data)
	if err != nil {
		t.Fatalf("%s: %v", p.id, err)
	}
	if !reflect.DeepEqual(raw, got) {
		t.Errorf("%s: зажим меняет файл\nбыл   %+v\nстал  %+v", p.id, raw, got)
	}
	for _, pg := range append(slices.Clone(got.Page), ptrBg(got.Whole)...) {
		if pg.Lib != "" || pg.Pat != nil && !gen.Known(pg.Pat.Kind) {
			t.Errorf("%s: фон %+v", p.id, pg)
		}
		if pg.Pat != nil && pg.Pat.Preset != "" && !slices.ContainsFunc(bgPresets, func(b preset) bool { return b.id == pg.Pat.Preset }) {
			t.Errorf("%s: нет пресета фона %s", p.id, pg.Pat.Preset)
		}
	}
	for _, st := range got.Stickers {
		if st.Lib != "" || st.Art == nil || !gen.Known(st.Art.Kind) || !gen.IsArt(st.Art.Kind) {
			t.Errorf("%s: стикер %+v", p.id, st)
		}
	}
	if len(got.Places) == 0 {
		t.Errorf("%s: без мест", p.id)
	}
}

func ptrBg(b *model.TplBg) []model.TplBg {
	if b == nil {
		return nil
	}
	return []model.TplBg{*b}
}

// TestDecorSpreadKind — свой шаблон: файл Развороты/<hex>.json и запись реестра, плитка с
// числами, повтор — «уже есть», ×, возврат скрытого с новым именем, ✎, круг через новое
// открытие библиотеки; несколько шаблонов — новые первыми; Find шаблон не ищет.
func TestDecorSpreadKind(t *testing.T) {
	s, cfg := openTemp(t)
	t.Cleanup(func() { nowMs = func() int64 { return time.Now().UnixMilli() } })
	var keys []string
	for n := 1; n <= 3; n++ {
		nowMs = func() int64 { return int64(100 + n) }
		it, err := s.SaveSpreadTpl("school", fmt.Sprintf("Мой %d", n), tplN(n))
		if err != nil {
			t.Fatal(err)
		}
		if !it.Own || it.Pages != 1 || it.Places != n || it.Aspect != 1 || it.Theme != "school" ||
			it.URL != TplPath+strings.TrimPrefix(it.ID, "my:")+"?v=1" {
			t.Fatalf("плитка %+v", it)
		}
		keys = append(keys, it.ID)
	}
	// Ключ — от канонического JSON: тот же шаблон тем же ключом.
	if keys[0] != model.TplKey(mustClamp(t, tplN(1))) {
		t.Fatalf("ключ %s", keys[0])
	}
	path := filepath.Join(cfg, RootName, "Развороты", strings.TrimPrefix(keys[1], "my:")+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := model.ParseSpreadTpl(data); err != nil || !reflect.DeepEqual(got, mustClamp(t, tplN(2))) {
		t.Fatalf("файл: %+v, %v", got, err)
	}
	if _, ok := s.Find(keys[0]); ok {
		t.Fatal("Find нашёл шаблон: это не картинка")
	}
	if _, err := s.SaveSpreadTpl("mine", "Ещё раз", tplN(1)); !errors.Is(err, errHave) {
		t.Fatalf("повтор: %v", err)
	}
	own := ownSpreads(t, s)
	if len(own) != 3 || own[0].ID != keys[2] || own[2].ID != keys[0] {
		t.Fatalf("свои %+v", own)
	}
	if err := s.Hide(KindSpread, keys[0]); err != nil {
		t.Fatal(err)
	}
	if len(ownSpreads(t, s)) != 2 {
		t.Fatal("скрытый в каталоге")
	}
	nowMs = func() int64 { return 900 }
	back, err := s.SaveSpreadTpl("wedding", "Вернулся", tplN(1))
	if err != nil || back.ID != keys[0] || back.Name != "Вернулся" || back.Theme != "wedding" {
		t.Fatalf("возврат: %+v, %v", back, err)
	}
	if own = ownSpreads(t, s); own[0].ID != keys[0] {
		t.Fatalf("вернувшийся не первый: %+v", own)
	}
	if err := s.Rename(KindSpread, keys[1], "Новое имя"); err != nil {
		t.Fatal(err)
	}
	if err := s.Rename(KindSpread, "sa:tpl-wave", "x"); !errors.Is(err, ErrBuiltin) {
		t.Fatalf("✎ встроенного: %v", err)
	}
	reopenSpreads(t, cfg, keys)
}

// reopenSpreads — круг: реестр и файлы шаблонов читаются заново (✎ сохранился); фоны шаблонов не видят.
func reopenSpreads(t *testing.T, cfg string, keys []string) {
	t.Helper()
	s2 := Open(cfg)
	own2 := ownSpreads(t, s2)
	if len(own2) != 3 || !slices.ContainsFunc(own2, func(it Item) bool { return it.ID == keys[1] && it.Name == "Новое имя" && it.Places == 2 }) {
		t.Fatalf("после открытия: %+v", own2)
	}
	for _, e := range readReg(t, cfg).Items {
		if e.Kind != KindSpread || e.Ext != "json" || e.W != 0 || e.H != 0 || e.Bytes <= 0 {
			t.Errorf("запись %+v", e)
		}
	}
	if got, err := s2.SpreadTpl(keys[2]); err != nil || len(got.Places) != 3 {
		t.Fatalf("SpreadTpl: %+v, %v", got, err)
	}
	// Фоны и стикеры шаблоны не видят, и наоборот.
	if cat, _ := s2.Catalog(KindBg, nil, 300, 300); slices.ContainsFunc(cat.Items, func(it Item) bool { return it.Own }) {
		t.Fatal("шаблон в фонах")
	}
}

func mustClamp(t *testing.T, tp model.SpreadTpl) model.SpreadTpl {
	t.Helper()
	c, ok := model.ClampSpreadTpl(tp)
	if !ok {
		t.Fatal("шаблон не прошёл зажим")
	}
	return c
}

// TestDecorSpreadRefuse — отказы сохранения и чужие файлы: пустое имя и шаблон, только
// чтение, предел своих, загрузка картинок в вид spread; .json вне Разворотов и картинка в них —
// не элементы; битый и новый файл — без плитки, новый — ErrTplNewer.
func TestDecorSpreadRefuse(t *testing.T) {
	s, cfg := openTemp(t)
	if _, err := s.SaveSpreadTpl("mine", "  ", tplN(1)); !errors.Is(err, ErrName) {
		t.Fatalf("пустое имя: %v", err)
	}
	empty := model.SpreadTpl{V: model.TplV, Pages: 1, Page: []model.TplBg{{}}}
	if _, err := s.SaveSpreadTpl("mine", "x", empty); !errors.Is(err, model.ErrTplEmpty) {
		t.Fatalf("пустой шаблон: %v", err)
	}
	if _, err := s.Import(KindSpread, "mine", []string{"x.json"}); !errors.Is(err, ErrKind) {
		t.Fatalf("Import spread: %v", err)
	}
	if _, err := s.SpreadTpl("sa:tpl-нет"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("неизвестный встроенный: %v", err)
	}
	if _, err := s.SpreadTpl("../x"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("мусорный ID: %v", err)
	}
	s.mu.Lock()
	for i := range MaxOwn {
		s.entries = append(s.entries, &entry{Key: fmt.Sprintf("my:%016x", i+1), Kind: KindSpread, Hidden: true})
	}
	s.mu.Unlock()
	if _, err := s.SaveSpreadTpl("mine", "x", tplN(1)); err == nil || !strings.Contains(err.Error(), "не помещается") {
		t.Fatalf("предел своих: %v", err)
	}

	dirs := map[string]string{"Развороты": "", "Фоны": ""}
	for d := range dirs {
		dirs[d] = filepath.Join(cfg, RootName, d)
		if err := os.MkdirAll(dirs[d], 0o755); err != nil {
			t.Fatal(err)
		}
	}
	good, _ := json.Marshal(mustClamp(t, tplN(2)))
	newer := strings.Replace(string(good), `"v":1`, `"v":2`, 1)
	write := func(dir, name, data string) {
		if err := os.WriteFile(filepath.Join(dirs[dir], name), []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("Развороты", "00000000000000a1.json", string(good))
	write("Развороты", "00000000000000a2.json", "{битый")
	write("Развороты", "00000000000000a3.json", newer)
	write("Развороты", "00000000000000a4.jpg", "jpg")
	write("Фоны", "00000000000000a5.json", string(good))
	reg := `{"version":1,"items":[`
	for i, k := range []string{"a1", "a2", "a3", "a4"} {
		if i > 0 {
			reg += ","
		}
		reg += `{"key":"my:00000000000000` + k + `","kind":"spread","theme":"mine","name":"` + k + `","ext":"json"}`
	}
	writeReg(t, cfg, reg+`]}`)
	s = Open(cfg)
	own := ownSpreads(t, s)
	if len(own) != 1 || own[0].ID != "my:00000000000000a1" || own[0].Places != 2 {
		t.Fatalf("свои %+v", own)
	}
	if _, err := s.SpreadTpl("my:00000000000000a3"); !errors.Is(err, model.ErrTplNewer) {
		t.Fatalf("новый файл: %v", err)
	}
	if _, ok := s.files[KindBg]["my:00000000000000a5"]; ok {
		t.Fatal(".json в Фонах стал элементом")
	}
	if _, ok := s.files[KindSpread]["my:00000000000000a4"]; ok {
		t.Fatal("картинка в Разворотах стала шаблоном")
	}
	s.mu.Lock()
	s.readOnly = true
	s.mu.Unlock()
	if _, err := s.SaveSpreadTpl("mine", "x", tplN(5)); !errors.Is(err, ErrReadOnly) {
		t.Fatalf("только чтение: %v", err)
	}
}

// TestDecorOldReaderKeepsSpread — 0.15 не знает вида spread: её чтение и запись реестра
// (правка своего фона) сохраняют запись шаблона и его файл.
func TestDecorOldReaderKeepsSpread(t *testing.T) {
	s, cfg := openTemp(t)
	tpl, err := s.SaveSpreadTpl("mine", "Шаблон", tplN(2))
	if err != nil {
		t.Fatal(err)
	}
	bg := importOK(t, s, "mine", testkit.JPEG(t, testkit.TempDir(t), "a.jpg", 40, 30))
	saved := kindDirs[KindSpread]
	delete(kindDirs, KindSpread) // реестр 0.15: видов только bg и sticker
	old := Open(cfg)
	if err := old.Rename(KindBg, bg[0].ID, "Переименован"); err != nil {
		kindDirs[KindSpread] = saved
		t.Fatal(err)
	}
	kindDirs[KindSpread] = saved
	found := false
	for _, e := range readReg(t, cfg).Items {
		found = found || e.Key == tpl.ID && e.Kind == KindSpread && e.Name == "Шаблон" && e.Ext == "json"
	}
	if !found {
		t.Fatalf("запись шаблона потеряна: %+v", readReg(t, cfg).Items)
	}
	if own := ownSpreads(t, Open(cfg)); len(own) != 1 || own[0].ID != tpl.ID {
		t.Fatalf("после 0.15: %+v", own)
	}
}

// TestDecorSpreadCatalogFast — первое открытие окна разбирает файлы своих шаблонов (design §3:
// гипотеза ≤ 150 мс; 500 файлов, как у фонов), повторное — из кэша разбора. Обычно холодное —
// 15–20 мс; порог — 300 мс (запас ×15, как у Limit): 150 мс под полной нагрузкой машины
// пробивались (153 мс), а зависание и квадратичный рост уходят в секунды.
func TestDecorSpreadCatalogFast(t *testing.T) {
	_, cfg := openTemp(t)
	dir := filepath.Join(cfg, RootName, "Развороты")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	n := 500
	if testkit.Lite() {
		n = 100
	}
	data, _ := json.Marshal(mustClamp(t, tplN(8)))
	reg := registry{Version: regVersion}
	for i := range n {
		hex := fmt.Sprintf("%016x", i+1)
		if err := os.WriteFile(filepath.Join(dir, hex+".json"), data, 0o644); err != nil {
			t.Fatal(err)
		}
		reg.Items = append(reg.Items, &entry{Key: "my:" + hex, Kind: KindSpread, Theme: "mine", Name: "Ш", Ext: "json", Added: int64(i)})
	}
	raw, _ := json.Marshal(reg)
	writeReg(t, cfg, string(raw))
	s := Open(cfg)
	var cold []Item
	// Каждый замер — с пустым кэшем разбора: как первое открытие окна.
	d := testkit.Fastest(5, func() {
		s.tpl.mu.Lock()
		s.tpl.m = nil
		s.tpl.mu.Unlock()
		cold = spreadItems(t, s)
	})
	if len(cold) != n+len(spreadPresets) {
		t.Fatalf("плиток %d", len(cold))
	}
	if limit := testkit.Limit(300 * time.Millisecond); d > limit {
		t.Errorf("первое открытие %v, предел %v", d, limit)
	}
	warm := testkit.Fastest(5, func() { _ = spreadItems(t, s) })
	if limit := testkit.Limit(20 * time.Millisecond); warm > limit {
		t.Errorf("повторное %v, предел %v", warm, limit)
	}
}

// TestDecorCachedPNG — PNG рисуется один раз на ключ; ошибка не запоминается; занятые слоты
// рисования и ушедший запрос — отказ без рисования; слот освобождается и после паники.
func TestDecorCachedPNG(t *testing.T) {
	s, _ := openTemp(t)
	calls := 0
	draw := func() ([]byte, error) { calls++; return []byte("png"), nil }
	for range 3 {
		if data, err := s.CachedPNG(context.Background(), "k1", draw); err != nil || string(data) != "png" {
			t.Fatal(data, err)
		}
	}
	if calls != 1 {
		t.Fatalf("рисований %d", calls)
	}
	bad := func() ([]byte, error) { calls++; return nil, errors.New("нет") }
	for range 2 {
		if _, err := s.CachedPNG(context.Background(), "k2", bad); err == nil {
			t.Fatal("ошибка не дошла")
		}
	}
	if calls != 3 {
		t.Fatalf("ошибка запомнилась: рисований %d", calls)
	}
	func() {
		defer func() { _ = recover() }()
		_, _ = s.CachedPNG(context.Background(), "k3", func() ([]byte, error) { panic("сбой") })
	}()
	for range cap(genSem) {
		genSem <- struct{}{}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := s.CachedPNG(ctx, "k4", draw)
	for range cap(genSem) {
		<-genSem
	}
	if !errors.Is(err, context.Canceled) || calls != 3 {
		t.Fatalf("ушедший запрос: %v, рисований %d", err, calls)
	}
}

// TestDecorTplID — разбор части пути /media/tpl: встроенный, 16 hex, мусор.
func TestDecorTplID(t *testing.T) {
	for seg, want := range map[string]string{
		"sa:tpl-wave": "sa:tpl-wave", "0123456789abcdef": "my:0123456789abcdef",
		"sa:tpl-нет": "", "sa:cream-gradient": "", "0123456789ABCDEF": "", "..": "", "": "", "my:0123456789abcdef": "",
	} {
		id, ok := TplID(seg)
		if ok != (want != "") || ok && id != want {
			t.Errorf("TplID(%q) = %q, %v", seg, id, ok)
		}
	}
	if u := TplURL("my:0123456789abcdef"); u != "/media/tpl/0123456789abcdef?v=1" {
		t.Fatal(u)
	}
}
