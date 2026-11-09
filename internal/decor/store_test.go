package decor

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/imgfile"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// openTemp — библиотека в чистой папке данных.
func openTemp(t *testing.T) (*Store, string) {
	t.Helper()
	cfg := testkit.TempDir(t)
	return Open(cfg), cfg
}

// importOK — импорт без пропусков; возвращает добавленные.
func importOK(t *testing.T, s *Store, theme string, files ...string) []Item {
	t.Helper()
	res, err := s.Import(KindBg, theme, files)
	if err != nil || len(res.Skipped) != 0 || len(res.Added) != len(files) {
		t.Fatalf("Import: %+v, %v", res, err)
	}
	return res.Added
}

func readReg(t *testing.T, cfg string) registry {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(cfg, RootName, regFile))
	if err != nil {
		t.Fatal(err)
	}
	var reg registry
	if err := json.Unmarshal(data, &reg); err != nil {
		t.Fatal(err)
	}
	return reg
}

func writeReg(t *testing.T, cfg, data string) {
	t.Helper()
	dir := filepath.Join(cfg, RootName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, regFile), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

func ownIDs(t *testing.T, s *Store) []string {
	t.Helper()
	cat, err := s.Catalog(KindBg, nil, 300, 300)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, it := range cat.Items {
		if it.Own {
			out = append(out, it.ID)
		}
	}
	return out
}

// TestDecorRegistry — круг записи и чтения реестра; битый — в .bad; новее — только чтение;
// чужой вид переживает запись; запись без файла и файл без записи.
func TestDecorRegistry(t *testing.T) {
	src := testkit.TempDir(t)
	s, cfg := openTemp(t)
	a := importOK(t, s, "wedding", testkit.JPEG(t, src, "Невеста.jpg", 40, 30), testkit.JPEG(t, src, "b.jpg", 41, 30))
	if err := s.Rename(KindBg, a[1].ID, "Второй"); err != nil {
		t.Fatal(err)
	}
	if err := s.Hide(KindBg, a[0].ID); err != nil {
		t.Fatal(err)
	}

	// Save → Load: те же записи, скрытый остаётся скрытым, файлы находятся.
	s2 := Open(cfg)
	if got := ownIDs(t, s2); !slices.Equal(got, []string{a[1].ID}) {
		t.Fatalf("после открытия свои %v, ждали %v", got, a[1].ID)
	}
	if e := s2.entryOf(KindBg, a[0].ID); e == nil || !e.Hidden || e.Name != "Невеста" || e.Theme != "wedding" || e.W != 40 {
		t.Fatalf("скрытая запись: %+v", e)
	}
	if e := s2.entryOf(KindBg, a[1].ID); e == nil || e.Name != "Второй" {
		t.Fatalf("переименованная запись: %+v", e)
	}
	for _, it := range a {
		if p, ok := s2.Find(it.ID); !ok || !fileExists(p) {
			t.Fatalf("Find(%s) = %q, %v", it.ID, p, ok)
		}
	}

	// Чужой вид (оформления 049) и мусорные записи: чужое переживает запись, мусор — нет.
	reg := readReg(t, cfg)
	reg.Items = append(reg.Items,
		&entry{Key: "my:00000000000000aa", Kind: "set", Theme: "school", Name: "Набор", Ext: "json"},
		&entry{Key: "../../x", Kind: KindBg, Name: "мусор"},
		&entry{Key: "my:00000000000000bb", Kind: "BG!", Name: "мусор"},
		&entry{Key: a[1].ID, Kind: KindBg, Name: "повтор"},
		nil)
	data, _ := json.Marshal(reg)
	writeReg(t, cfg, string(data))
	s3 := Open(cfg)
	if err := s3.Rename(KindBg, a[1].ID, "Третий"); err != nil {
		t.Fatal(err)
	}
	reg = readReg(t, cfg)
	var kinds []string
	for _, e := range reg.Items {
		kinds = append(kinds, e.Kind+":"+e.Name)
	}
	want := []string{"bg:Невеста", "bg:Третий", "set:Набор"}
	if !slices.Equal(kinds, want) {
		t.Fatalf("реестр после записи %v, ждали %v", kinds, want)
	}

	// Запись без файла в каталог не идёт; файл без записи искатель находит, каталог — нет.
	if err := os.Remove(mustFind(t, s3, a[1].ID)); err != nil {
		t.Fatal(err)
	}
	stray := filepath.Join(cfg, RootName, "Фоны", "0123456789abcdef.jpg")
	if err := os.WriteFile(stray, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	s4 := Open(cfg)
	if got := ownIDs(t, s4); len(got) != 0 {
		t.Fatalf("свои без файлов в каталоге: %v", got)
	}
	if p, ok := s4.Find("my:0123456789abcdef"); !ok || p != stray {
		t.Fatalf("файл без записи: %q, %v", p, ok)
	}
	if _, ok := s4.Find(a[1].ID); ok {
		t.Fatal("стёртый файл найден")
	}
	for _, bad := range []string{"", "my:0123456789ABCDEF", "my:0123", "sa:dots", `..\Фоны\x`} {
		if _, ok := s4.Find(bad); ok {
			t.Errorf("Find(%q) нашёл", bad)
		}
	}

	registryUnreadable(t, cfg, src)
}

// registryUnreadable — случай TestDecorRegistry: реестр не читается (здесь — папка на его
// месте; так же доступ, блокировка) — не в .bad, сеанс только чтение, файлы находятся.
func registryUnreadable(t *testing.T, cfg, src string) {
	t.Helper()
	regPath := filepath.Join(cfg, RootName, regFile)
	if err := os.Remove(regPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(regPath, 0o755); err != nil {
		t.Fatal(err)
	}
	s5 := Open(cfg)
	if !s5.ReadOnly() || fileExists(regPath+".bad") {
		t.Fatalf("ошибка чтения: ReadOnly=%v, .bad есть=%v", s5.ReadOnly(), fileExists(regPath+".bad"))
	}
	if st, err := os.Stat(regPath); err != nil || !st.IsDir() {
		t.Fatalf("реестр тронут: %v", err)
	}
	if _, err := s5.Import(KindBg, "mine", []string{testkit.JPEG(t, src, "c.jpg", 20, 20)}); !errors.Is(err, ErrReadOnly) {
		t.Fatalf("импорт при нечитаемом реестре: %v", err)
	}
	if _, ok := s5.Find("my:0123456789abcdef"); !ok {
		t.Error("файлы не находятся при нечитаемом реестре")
	}
}

// Правленый руками реестр: размеры и байты зажаты, сумма библиотеки не переполняется — две
// записи по MaxInt64 и десять по пределу файла заполняют библиотеку, а не обнуляют счёт.
func TestDecorRegistryClamp(t *testing.T) {
	cfg := testkit.TempDir(t)
	reg := registry{Version: regVersion}
	for i := range 12 {
		e := &entry{Key: fmt.Sprintf("my:%016x", i+1), Kind: KindBg, Name: "x", Ext: "jpg", Bytes: MaxBgBytes}
		if i < 2 {
			e.Bytes, e.W, e.H = math.MaxInt64, -5, 1<<40
		}
		reg.Items = append(reg.Items, e)
	}
	data, _ := json.Marshal(reg)
	writeReg(t, cfg, string(data))
	s := Open(cfg)
	if e := s.entryOf(KindBg, "my:0000000000000001"); e == nil || e.Bytes != MaxBgBytes || e.W != 0 || e.H != imgfile.MaxPixels {
		t.Fatalf("запись не зажата: %+v", e)
	}
	res, err := s.Import(KindBg, "mine", []string{testkit.JPEG(t, testkit.TempDir(t), "a.jpg", 20, 20)})
	if err != nil || len(res.Added) != 0 || len(res.Skipped) != 1 || !strings.Contains(res.Skipped[0].Why, "заполнена") {
		t.Fatalf("переполнение суммы пустило импорт: %v %+v", err, res)
	}
}

func mustFind(t *testing.T, s *Store, key string) string {
	t.Helper()
	p, ok := s.Find(key)
	if !ok {
		t.Fatalf("нет %s", key)
	}
	return p
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// TestDecorRegistryBroken — битый реестр уходит в .bad, библиотека пуста, импорт пишет новый.
func TestDecorRegistryBroken(t *testing.T) {
	for _, data := range []string{"{битый", `{"items":[]}`, "null", `{"version":1,"items":"x"}`} {
		cfg := testkit.TempDir(t)
		writeReg(t, cfg, data)
		s := Open(cfg)
		if s.ReadOnly() || len(ownIDs(t, s)) != 0 {
			t.Fatalf("%q: ReadOnly=%v, свои %v", data, s.ReadOnly(), ownIDs(t, s))
		}
		bad, err := os.ReadFile(filepath.Join(cfg, RootName, regFile+".bad"))
		if err != nil || string(bad) != data {
			t.Fatalf("%q: .bad = %q, %v", data, bad, err)
		}
		importOK(t, s, "mine", testkit.JPEG(t, testkit.TempDir(t), "a.jpg", 20, 20))
		if reg := readReg(t, cfg); reg.Version != regVersion || len(reg.Items) != 1 {
			t.Fatalf("%q: новый реестр %+v", data, reg)
		}
	}
}

// TestDecorRegistryNewer — реестр новой версии: каталог виден, правки закрыты, файл не тронут.
func TestDecorRegistryNewer(t *testing.T) {
	src := testkit.TempDir(t)
	s, cfg := openTemp(t)
	a := importOK(t, s, "mine", testkit.JPEG(t, src, "a.jpg", 20, 20))
	reg := readReg(t, cfg)
	reg.Version = 2
	data, _ := json.Marshal(reg)
	writeReg(t, cfg, string(data))

	s2 := Open(cfg)
	cat, err := s2.Catalog(KindBg, nil, 300, 300)
	if err != nil || !cat.ReadOnly || !slices.Equal(ownIDs(t, s2), []string{a[0].ID}) {
		t.Fatalf("каталог только для чтения: %v, %v", cat.ReadOnly, err)
	}
	if _, err := s2.Import(KindBg, "mine", []string{testkit.JPEG(t, src, "b.jpg", 21, 20)}); !errors.Is(err, ErrReadOnly) {
		t.Errorf("Import: %v", err)
	}
	if err := s2.Rename(KindBg, a[0].ID, "x"); !errors.Is(err, ErrReadOnly) {
		t.Errorf("Rename: %v", err)
	}
	if err := s2.Hide(KindBg, a[0].ID); !errors.Is(err, ErrReadOnly) {
		t.Errorf("Hide: %v", err)
	}
	after, _ := os.ReadFile(filepath.Join(cfg, RootName, regFile))
	if string(after) != string(data) {
		t.Error("реестр новой версии переписан")
	}
}

// TestDecorHideRename — ✎ и × своего: проверки, холостые без записи, встроенное не меняется.
func TestDecorHideRename(t *testing.T) {
	src := testkit.TempDir(t)
	s, cfg := openTemp(t)
	a := importOK(t, s, "mine", testkit.JPEG(t, src, "a.jpg", 20, 20), testkit.JPEG(t, src, "b.jpg", 21, 20))
	if err := s.Rename(KindBg, a[0].ID, "  Новое\tимя\u202e  "); err != nil {
		t.Fatal(err)
	}
	if e := s.entryOf(KindBg, a[0].ID); e.Name != "Новое имя" {
		t.Fatalf("имя %q", e.Name)
	}
	long := ""
	for range 100 {
		long += "я"
	}
	if err := s.Rename(KindBg, a[1].ID, long); err != nil {
		t.Fatal(err)
	}
	if n := len([]rune(s.entryOf(KindBg, a[1].ID).Name)); n != 60 {
		t.Fatalf("длинное имя: %d знаков", n)
	}
	if err := s.Hide(KindBg, a[0].ID); err != nil {
		t.Fatal(err)
	}
	// Холостые правки реестр не пишут: метка в файле остаётся.
	path := filepath.Join(cfg, RootName, regFile)
	if err := os.WriteFile(path, []byte("метка"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := s.Rename(KindBg, a[1].ID, s.entryOf(KindBg, a[1].ID).Name); err != nil {
		t.Errorf("тот же Rename: %v", err)
	}
	if err := s.Hide(KindBg, a[0].ID); err != nil {
		t.Errorf("повторный Hide: %v", err)
	}
	if data, _ := os.ReadFile(path); string(data) != "метка" {
		t.Error("холостая правка записала реестр")
	}
	// Реестр не записался — правка откатывается.
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := s.Rename(KindBg, a[1].ID, "Не запишется"); err == nil || s.entryOf(KindBg, a[1].ID).Name == "Не запишется" {
		t.Errorf("сбой записи: %v, имя %q", err, s.entryOf(KindBg, a[1].ID).Name)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	// Файл скрытого остаётся: альбомы, где он стоит, не меняются (решение №80).
	if p, ok := s.Find(a[0].ID); !ok || !fileExists(p) {
		t.Fatal("скрытый файл пропал")
	}
	if got := ownIDs(t, s); !slices.Equal(got, []string{a[1].ID}) {
		t.Fatalf("каталог после ×: %v", got)
	}
	cases := []struct {
		kind, id, name string
		want           error
	}{
		{KindBg, a[1].ID, " \t\u200b", ErrName},
		{KindBg, "sa:gold-dots", "x", ErrBuiltin},
		{KindBg, "my:ffffffffffffffff", "x", ErrNotFound},
		{KindBg, "../x", "x", ErrNotFound},
		{KindSticker, a[1].ID, "x", ErrNotFound},
		{"set", a[1].ID, "x", ErrKind},
	}
	for _, c := range cases {
		if err := s.Rename(c.kind, c.id, c.name); !errors.Is(err, c.want) {
			t.Errorf("Rename(%s, %s, %q) = %v, ждали %v", c.kind, c.id, c.name, err, c.want)
		}
		if c.want == ErrName {
			continue
		}
		if err := s.Hide(c.kind, c.id); !errors.Is(err, c.want) {
			t.Errorf("Hide(%s, %s) = %v, ждали %v", c.kind, c.id, err, c.want)
		}
	}
}

// TestDecorConcurrent — каталог, искатель и /media читают, пока идут импорт, ✎ и × (для -race).
func TestDecorConcurrent(t *testing.T) {
	src := testkit.TempDir(t)
	s, _ := openTemp(t)
	a := importOK(t, s, "mine", testkit.JPEG(t, src, "a.jpg", 20, 20))
	files := make([]string, 8)
	for i := range files {
		files[i] = testkit.JPEG(t, src, fmt.Sprintf("f%d.jpg", i), 21+i, 20)
	}
	var wg sync.WaitGroup
	wg.Go(func() {
		for _, f := range files {
			_, _ = s.Import(KindBg, "mine", []string{f})
		}
	})
	wg.Go(func() {
		for i := range 20 {
			_ = s.Rename(KindBg, a[0].ID, fmt.Sprintf("имя %d", i))
		}
		_ = s.Hide(KindBg, a[0].ID)
	})
	h := s.Handler(nil)
	for range 20 {
		_, _ = s.Catalog(KindBg, []string{a[0].ID}, 300, 300)
		_, _ = s.Find(a[0].ID)
		_, _ = s.Background(a[0].ID)
		get(h, "GET", "/media/lib/t/"+strings.TrimPrefix(a[0].ID, "my:"), "")
	}
	wg.Wait()
	if n := len(ownIDs(t, s)); n != len(files) {
		t.Fatalf("своих %d, ждали %d", n, len(files))
	}
}
