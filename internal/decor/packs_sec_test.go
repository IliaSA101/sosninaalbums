package decor

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// fillerEntries — записи набора-заглушки на total байт (по MaxBgBytes на запись: зажим реестра).
func fillerEntries(total int64) []packEntry {
	var out []packEntry
	for i := 0; total > 0; i++ {
		n := min(total, MaxBgBytes)
		out = append(out, packEntry{Key: fmt.Sprintf("my:f%015x", i), Kind: KindBg, Theme: "basic", Name: "З", Bytes: n})
		total -= n
	}
	return out
}

// writeFiller — реестр набора id на total байт на диске (файлов нет: квота — по реестру).
func writeFiller(t *testing.T, cfg, id string, total int64) {
	t.Helper()
	data, err := json.Marshal(packReg{V: packRegV, ID: id, Version: 1, Title: id, Items: fillerEntries(total)})
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(cfg, RootName, packsDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, id+".json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// Квота 2 ГБ общая (design §4.2): набор у предела — своё (фоны, шаблон) сверху не встаёт и после
// перезапуска; ниже предела — встаёт.
func TestLibQuotaWithPacks(t *testing.T) {
	cfg := testkit.TempDir(t)
	writeFiller(t, cfg, "full", MaxLibraryLen-100)
	s := Open(cfg)
	if got := s.libBytes(); got != MaxLibraryLen-100 {
		t.Fatalf("байты библиотеки %d", got)
	}
	src := testkit.TempDir(t)
	files := []string{writeFile(t, src, "a.png", renderPNG(t, 30, 20)), writeFile(t, src, "b.png", renderPNG(t, 31, 20))}
	check := func(s *Store, when string) {
		t.Helper()
		res, err := s.Import(KindBg, "mine", files)
		if err != nil || len(res.Added) != 0 || len(res.Skipped) != 2 {
			t.Fatalf("%s: импорт сверх квоты: %v %+v", when, err, res)
		}
		for _, k := range res.Skipped {
			if !strings.Contains(k.Why, "заполнена") {
				t.Errorf("%s: причина %q", when, k.Why)
			}
		}
		if _, err := s.SaveSpreadTpl("mine", "Шаблон", tplN(3)); err == nil || !strings.Contains(err.Error(), "заполнена") {
			t.Errorf("%s: шаблон сверх квоты: %v", when, err)
		}
		assertNoTmp(t, filepath.Join(cfg, RootName, "Фоны"))
	}
	check(s, "сразу")
	check(Open(cfg), "после перезапуска")
	if _, err := os.Stat(filepath.Join(cfg, RootName, regFile)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("реестр своих записан при отказе: %v", err)
	}
	// Ниже предела на 10 МБ — оба фона и шаблон встают.
	writeFiller(t, cfg, "full", MaxLibraryLen-10<<20)
	s = Open(cfg)
	importOK(t, s, "mine", files...)
	if _, err := s.SaveSpreadTpl("mine", "Шаблон", tplN(3)); err != nil {
		t.Fatalf("шаблон ниже квоты: %v", err)
	}
	if own := ownIDs(t, Open(cfg)); len(own) != 2 {
		t.Errorf("после перезапуска свои %v", own)
	}
}

// InstallPack сверяет квоту заново: между StagePack и «Заменить?» встал другой набор. Ключи, уже
// лежащие на диске (обновление тем же содержимым), в квоту второй раз не идут.
func TestInstallPackRecheckQuota(t *testing.T) {
	s, cfg := openTemp(t)
	a, b, c := renderPNG(t, 81, 30), renderPNG(t, 82, 30), renderPNG(t, 83, 30)
	m1, s1 := testPack("ivory", 1, pit{"bg/a.png", a}, pit{"bg/b.png", b})
	install(t, s, m1, s1, nil)
	m2, s2 := testPack("ivory", 2, pit{"bg/a.png", a}, pit{"bg/b.png", b})
	upd, err := s.StagePack(m2, s2, OriginFile, "")
	if err != nil || upd.Ask != AskUpdate {
		t.Fatalf("обновление: %v %+v", err, upd)
	}
	defer upd.Close()
	m3, s3 := testPack("rose", 1, pit{"bg/c.png", c})
	fresh, err := s.StagePack(m3, s3, OriginFile, "")
	if err != nil {
		t.Fatal(err)
	}
	defer fresh.Close()
	// Пока ждали ответа, встал набор, добивший библиотеку до предела без одного байта.
	s.mu.Lock()
	used := s.libBytes()
	s.packs = append(s.packs, &packReg{V: packRegV, ID: "late", Version: 1, Title: "late", Items: fillerEntries(MaxLibraryLen - 1 - used)})
	s.mu.Unlock()
	if _, err := s.InstallPack(fresh, nil); !errors.Is(err, ErrLibFull) {
		t.Errorf("новый набор сверх квоты: %v", err)
	}
	if _, ok := s.Find(libKey(c)); ok || regularFile(filepath.Join(cfg, RootName, packsDir, "rose.json")) {
		t.Error("набор сверх квоты записан")
	}
	if info, err := s.InstallPack(upd, nil); err != nil || info.Version != 2 {
		t.Errorf("обновление тем же содержимым: %v %+v", err, info)
	}
	fresh.Close()
	noTmpAnywhere(t, cfg)
	if ps := Open(cfg).Packs(); len(ps) != 2 || ps[1].ID != "ivory" || ps[1].Version != 2 {
		t.Errorf("после перезапуска: %+v", ps)
	}
}

// Картинки, на которые ссылаются свои шаблоны (и скрытые) и шаблоны других наборов, удаление
// набора не стирает; остальные его файлы — стирает. План = итогу; держится и после перезапуска
// (шаблоны читаются с диска, а не из кэша).
func TestRemovePackKeepsTplLibs(t *testing.T) {
	s, cfg := openTemp(t)
	a, b, c, d, st := renderPNG(t, 91, 30), renderPNG(t, 92, 30), renderPNG(t, 93, 30), renderPNG(t, 94, 30), renderPNG(t, 17, 17)
	m, src := testPack("ivory", 1, pit{"bg/a.png", a}, pit{"bg/b.png", b}, pit{"bg/c.png", c}, pit{"bg/d.png", d}, pit{"stickers/s.png", st})
	install(t, s, m, src, nil)
	box := []model.Box{{X: .1, Y: .1, W: .8, H: .8}}
	if _, err := s.SaveSpreadTpl("mine", "Свой", model.SpreadTpl{V: 1, Pages: 1, Places: box,
		Page: []model.TplBg{{Lib: libKey(a)}}, Stickers: []model.TplItem{{W: .2, H: .2, Lib: libKey(st)}}}); err != nil {
		t.Fatal(err)
	}
	hid, err := s.SaveSpreadTpl("mine", "Скрытый", model.SpreadTpl{V: 1, Pages: 1, Places: box, Page: []model.TplBg{{Lib: libKey(c)}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Hide(KindSpread, hid.ID); err != nil {
		t.Fatal(err)
	}
	mo, so := testPack("other", 1, pit{"spreads/t.json", tplJSON(t, model.SpreadTpl{V: 1, Pages: 1, Places: box, Page: []model.TplBg{{Lib: libKey(b)}}})})
	install(t, s, mo, so, nil)
	plan, err := s.PlanRemovePack("ivory", nil)
	if err != nil || plan.Files != 1 || plan.Kept != 4 || plan.Bytes != int64(len(d)) {
		t.Fatalf("план: %v %+v", err, plan)
	}
	s = Open(cfg) // кэш разбора пуст: ссылки — из файлов шаблонов
	res, err := s.RemovePack("ivory", nil)
	if err != nil || res != plan {
		t.Fatalf("удаление: %v %+v, план %+v", err, res, plan)
	}
	for key, want := range map[string]bool{libKey(a): true, libKey(b): true, libKey(c): true, libKey(st): true, libKey(d): false} {
		if _, ok := Open(cfg).Find(key); ok != want {
			t.Errorf("файл %s есть=%v, ждали %v", key, ok, want)
		}
	}
	// Удаление набора с шаблоном: свой шаблон держит a, шаблон other уходит сам.
	if res, err := s.RemovePack("other", nil); err != nil || res.Files != 1 {
		t.Errorf("удаление other: %v %+v", err, res)
	}
}

// Остатки WriteFileAtomic (шаблон, реестр) убирает открытие; файлы шаблонов, реестры и чужое —
// на месте.
func TestOpenSweepsAtomicLeft(t *testing.T) {
	s, cfg := openTemp(t)
	it, err := s.SaveSpreadTpl("mine", "Ш", tplN(2))
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(cfg, RootName)
	dir := filepath.Join(root, kindDirs[KindSpread])
	hex := strings.TrimPrefix(it.ID, libPrefix)
	gone := []string{
		filepath.Join(dir, hex+".json.123456.tmp"),
		filepath.Join(dir, "00000000000000ab.json.9.tmp"),
		filepath.Join(root, regFile+".777.tmp"),
	}
	kept := []string{
		filepath.Join(dir, hex+".json"),
		filepath.Join(dir, "заметка.json.1.tmp"), // имя не от ключа — чужое
		filepath.Join(dir, "0123.json.1.tmp"),
		filepath.Join(root, regFile),
		filepath.Join(root, "other.json.1.tmp"),
	}
	for _, p := range append(gone, kept[1:]...) {
		if p == filepath.Join(root, regFile) {
			continue
		}
		if err := os.WriteFile(p, []byte("{"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	s = Open(cfg)
	for _, p := range gone {
		if fileExists(p) {
			t.Errorf("не убран %s", filepath.Base(p))
		}
	}
	for _, p := range kept {
		if !fileExists(p) {
			t.Errorf("убран %s", filepath.Base(p))
		}
	}
	if own := ownSpreads(t, s); len(own) != 1 || own[0].ID != it.ID {
		t.Errorf("свои после открытия: %+v", own)
	}
}

// id набора — не устройство Windows: реестр Наборы/<id>.json не создать.
func TestPackIDDevice(t *testing.T) {
	for _, id := range []string{"con", "prn", "aux", "nul", "com1", "com9", "lpt1", "lpt9", "com0"} {
		if validPackID(id) {
			t.Errorf("%s принят", id)
		}
	}
	for _, id := range []string{"conx", "com10", "nul-pack", "lpt", "ivory"} {
		if !validPackID(id) {
			t.Errorf("%s отклонён", id)
		}
	}
	s, _ := openTemp(t)
	m, src := testPack("aux", 1, pit{"bg/a.png", renderPNG(t, 33, 20)})
	if _, err := s.StagePack(m, src, OriginFile, ""); err == nil {
		t.Error("набор aux принят")
	}
}

// Скачивание набора: временный файл только по id в Наборы/ (папка создаётся), id не по
// грамматике и устройство Windows — отказ; остаток оборванного убирается при открытии.
// Used — байты своих и наборов (квота до скачивания), общий файл двух наборов — один раз.
func TestDownloadPathAndUsed(t *testing.T) {
	s, cfg := openTemp(t)
	if s.Used() != 0 {
		t.Fatalf("пустая библиотека: %d", s.Used())
	}
	p, err := s.DownloadPath("ivory")
	if err != nil || p != filepath.Join(cfg, RootName, packsDir, ".dl-ivory.tmp") {
		t.Fatalf("путь: %v %q", err, p)
	}
	if fi, err := os.Stat(filepath.Dir(p)); err != nil || !fi.IsDir() {
		t.Fatalf("папка наборов: %v", err)
	}
	for _, bad := range []string{"", "../x", "Ivory", "ab", "con", "a/b", strings.Repeat("a", 41)} {
		if _, err := s.DownloadPath(bad); err == nil {
			t.Errorf("id %q принят", bad)
		}
	}
	if err := os.WriteFile(p, []byte("обрыв"), 0o600); err != nil {
		t.Fatal(err)
	}
	bg, st := renderPNG(t, 40, 30), renderPNG(t, 16, 16)
	m, src := testPack("ivory", 1, pit{"bg/one.png", bg}, pit{"stickers/dot.png", st})
	install(t, s, m, src, nil)
	m2, src2 := testPack("sage", 1, pit{"bg/one.png", bg})
	install(t, s, m2, src2, nil)
	if got := s.Used(); got != int64(len(bg)+len(st)) {
		t.Fatalf("занято %d, ждали %d", got, len(bg)+len(st))
	}
	s2 := Open(cfg)
	if regularFile(p) || s2.Used() != s.Used() {
		t.Fatalf("после перезапуска: остаток %v, занято %d", regularFile(p), s2.Used())
	}
}
