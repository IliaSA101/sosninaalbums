package libpacks

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/decor/fetch"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Сохранённый список (137, безопасность Д1–Д2, З1–З4): круг запись → чтение, подмена, откат,
// предел, не обычный файл, остатки .tmp. Сеть не нужна — fetch.Client.Parse работает по байтам.

const cachePrefix = "https://github.com/x/releases/download/"

type cacheFx struct {
	t    *testing.T
	priv ed25519.PrivateKey
	src  fetch.Source
	dir  string
	s    *Service
}

func newCacheFx(t *testing.T) *cacheFx {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	fx := &cacheFx{t: t, priv: priv, src: fetch.Source{CatalogURL: cachePrefix + "c.json", Prefix: cachePrefix,
		Hosts: []string{"github.com"}, Keys: []ed25519.PublicKey{pub}}}
	fx.s = fx.service()
	fx.dir = fx.s.h.CfgDir()
	return fx
}

// service — служба над той же папкой настроек (перезапуск программы).
func (fx *cacheFx) service() *Service {
	h := testHost(fx.t, &evs{}, func(h *Host) {
		h.Fetcher = func() *fetch.Client { return fetch.New(fx.src) }
		if fx.dir != "" {
			h.CfgDir = func() string { return fx.dir }
		}
	})
	return New(h)
}

// env — подписанный конверт каталога seq с наборами ids.
func (fx *cacheFx) env(seq int64, ids ...string) []byte {
	fx.t.Helper()
	rows := []map[string]any{}
	for _, id := range ids {
		rows = append(rows, map[string]any{"id": id, "version": 1, "minApp": "0.1.0", "title": "T " + id, "size": 10,
			"sha256": strings.Repeat("a", 64), "url": cachePrefix + id + "-v1/" + id + ".zip"})
	}
	body, _ := json.Marshal(map[string]any{"format": 1, "seq": seq, "packs": rows})
	sig := base64.StdEncoding.EncodeToString(ed25519.Sign(fx.priv, body))
	env, _ := json.Marshal(map[string]string{"sig": sig, "body": base64.StdEncoding.EncodeToString(body)})
	return env
}

func (fx *cacheFx) catalog(env []byte) fetch.Catalog {
	fx.t.Helper()
	cat, err := fetch.New(fx.src).Parse(env, 0)
	if err != nil {
		fx.t.Fatal(err)
	}
	cat.Env = env
	return cat
}

func (fx *cacheFx) path() string { return filepath.Join(fx.dir, catalogFile) }

func logTo(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	return &buf
}

// Круг: запись → новая служба читает то же; права 0600, Env в памяти обнулён, дата — время файла.
func TestCatalogCacheRoundTrip(t *testing.T) {
	fx := newCacheFx(t)
	if cat, at, bad := fx.s.cached(); cat != nil || !at.IsZero() || bad {
		t.Fatalf("до записи: %v %v %v", cat, at, bad)
	}
	fx.s.remember(fx.catalog(fx.env(3, "sage", "ruby")))
	if cat, _, _ := fx.s.cached(); cat == nil || cat.Env != nil || cat.Seq != 3 {
		t.Fatalf("в памяти: %+v", cat)
	}
	got, err := os.ReadFile(fx.path())
	if err != nil || !bytes.Equal(got, fx.env(3, "sage", "ruby")) {
		t.Fatalf("файл: %v", err)
	}
	if fi, _ := os.Stat(fx.path()); runtime.GOOS != "windows" && fi.Mode().Perm() != 0o600 {
		t.Errorf("права %v", fi.Mode().Perm())
	}
	again := fx.service()
	cat, at, bad := again.cached()
	if bad || cat == nil || len(cat.Packs) != 2 || cat.Packs[1].ID != "ruby" || at.IsZero() {
		t.Fatalf("после перезапуска: %+v %v %v", cat, at, bad)
	}
}

// Подмена, чужой ключ, откат по seq, пустой, усечённый, огромный — пусто и «не читается»; сеть не
// нужна; одна строка в log.txt за сеанс, повторные вызовы файл не перечитывают.
func TestCatalogCacheBroken(t *testing.T) {
	fx := newCacheFx(t)
	good := fx.env(5, "sage")
	other := newCacheFx(t).env(5, "sage") // подпись чужим ключом
	big := bytes.Repeat([]byte(" "), fetch.MaxEnvelope+1)
	for name, data := range map[string][]byte{
		"подмена байта":  bytes.Replace(good, []byte(`"sig":"`), []byte(`"sig":"A`), 1),
		"чужой ключ":     other,
		"пустой":         {},
		"усечённый":      good[:len(good)/2],
		"больше предела": big,
		"мусор":          []byte("не json"),
	} {
		if err := os.WriteFile(fx.path(), data, 0o600); err != nil {
			t.Fatal(err)
		}
		buf := logTo(t)
		s := fx.service()
		for range 3 {
			if cat, _, bad := s.cached(); cat != nil || !bad {
				t.Errorf("%s: %v %v", name, cat, bad)
			}
		}
		if n := strings.Count(buf.String(), "\n"); n != 1 || strings.Contains(buf.String(), fx.dir) {
			t.Errorf("%s: строк в log.txt %d: %q", name, n, buf.String())
		}
	}
	// Откат: seq файла меньше принятого CatalogSeq.
	if err := os.WriteFile(fx.path(), fx.env(3, "sage"), 0o600); err != nil {
		t.Fatal(err)
	}
	prefs.UpdateLibraryNet(fx.dir, func(n *prefs.LibraryNet) { n.CatalogSeq = 4 })
	if cat, _, bad := fx.service().cached(); cat != nil || !bad {
		t.Errorf("откат: %v %v", cat, bad)
	}
}

// Чтение без побочных действий (З3): seq файла больше принятого — config.json не меняется.
func TestCatalogCacheNoSideEffects(t *testing.T) {
	fx := newCacheFx(t)
	prefs.UpdateLibraryNet(fx.dir, func(n *prefs.LibraryNet) { n.CatalogSeq = 2 })
	before, _ := os.ReadFile(prefs.ConfigPath(fx.dir))
	if err := os.WriteFile(fx.path(), fx.env(9, "sage"), 0o600); err != nil {
		t.Fatal(err)
	}
	if cat, _, _ := fx.service().cached(); cat == nil || cat.Seq != 9 {
		t.Fatalf("чтение: %+v", cat)
	}
	after, _ := os.ReadFile(prefs.ConfigPath(fx.dir))
	if !bytes.Equal(before, after) {
		t.Errorf("config.json изменился: %s → %s", before, after)
	}
}

// Ссылка и папка на месте списка — пусто, без чтения цели.
func TestCatalogCacheNotRegular(t *testing.T) {
	fx := newCacheFx(t)
	decoy := filepath.Join(testkit.TempDir(t), "decoy.json")
	if err := os.WriteFile(decoy, fx.env(5, "sage"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(decoy, fx.path()); err != nil {
		t.Skipf("ссылки недоступны: %v", err)
	}
	if cat, _, bad := fx.service().cached(); cat != nil || !bad {
		t.Errorf("ссылка: %v %v", cat, bad)
	}
	_ = os.Remove(fx.path())
	if err := os.Mkdir(fx.path(), 0o700); err != nil {
		t.Fatal(err)
	}
	if cat, _, bad := fx.service().cached(); cat != nil || !bad {
		t.Errorf("папка: %v %v", cat, bad)
	}
}

// Ленивое чтение после записи не затирает свежее (Д1); параллельно List-подобные чтения и запись
// под -race.
func TestCatalogCacheLoadAfterSave(t *testing.T) {
	fx := newCacheFx(t)
	if err := os.WriteFile(fx.path(), fx.env(2, "old"), 0o600); err != nil {
		t.Fatal(err)
	}
	s := fx.service()
	s.remember(fx.catalog(fx.env(9, "new")))
	if err := os.WriteFile(fx.path(), fx.env(2, "old"), 0o600); err != nil { // запоздалый файл
		t.Fatal(err)
	}
	if cat, _, _ := s.cached(); cat == nil || cat.Seq != 9 {
		t.Fatalf("затёрто старым: %+v", cat)
	}
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if i%2 == 0 {
				s.cached()
			} else {
				s.remember(fx.catalog(fx.env(int64(10+i), "new")))
			}
		}()
	}
	wg.Wait()
}

// Запись поверх ссылки заменяет саму запись, приманка цела (Д2); остатки *.tmp убираются, папка с
// таким именем остаётся.
func TestCatalogCacheSaveOverLink(t *testing.T) {
	fx := newCacheFx(t)
	decoy := filepath.Join(testkit.TempDir(t), "decoy.json")
	if err := os.WriteFile(decoy, []byte("приманка"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(decoy, fx.path()); err != nil {
		t.Skipf("ссылки недоступны: %v", err)
	}
	stale := filepath.Join(fx.dir, catalogFile+".123.tmp")
	if err := os.WriteFile(stale, []byte("остаток"), 0o600); err != nil {
		t.Fatal(err)
	}
	fx.s.remember(fx.catalog(fx.env(3, "sage")))
	if fi, err := os.Lstat(fx.path()); err != nil || !fi.Mode().IsRegular() {
		t.Fatalf("на месте списка не обычный файл: %v", err)
	}
	if got, _ := os.ReadFile(decoy); string(got) != "приманка" {
		t.Errorf("приманка изменена: %q", got)
	}
	if _, err := os.Stat(stale); err == nil {
		t.Error("остаток .tmp не убран")
	}
}

// Запись не прошла (на месте файла папка): операция не падает, .tmp нет, в log.txt одна строка
// без пути, список в памяти есть.
func TestCatalogCacheSaveFailNoTmp(t *testing.T) {
	fx := newCacheFx(t)
	if err := os.Mkdir(fx.path(), 0o700); err != nil {
		t.Fatal(err)
	}
	buf := logTo(t)
	fx.s.remember(fx.catalog(fx.env(3, "sage")))
	if cat, _, _ := fx.s.cached(); cat == nil || cat.Seq != 3 {
		t.Fatalf("память: %+v", cat)
	}
	list, _ := os.ReadDir(fx.dir)
	for _, e := range list {
		if strings.HasSuffix(e.Name(), ".tmp") {
			t.Errorf("остался %s", e.Name())
		}
	}
	if out := buf.String(); strings.Count(out, "не сохранён") != 1 || strings.Contains(out, fx.dir) {
		t.Errorf("log.txt: %q", out)
	}
}

// Папки настроек нет (140 Н4): список живёт в памяти, файл не пишется и не читается, а в текущей
// папке ничего не появляется.
func TestCatalogCacheNoCfgDir(t *testing.T) {
	cwd := testkit.TempDir(t)
	t.Chdir(cwd)
	fx := newCacheFx(t)
	h := testHost(t, &evs{}, func(h *Host) {
		h.Fetcher = func() *fetch.Client { return fetch.New(fx.src) }
		h.CfgDir = func() string { return "" }
	})
	s := New(h)
	if s.catPath() != "" {
		t.Fatalf("catPath = %q", s.catPath())
	}
	if cat, _, bad := s.cached(); cat != nil || bad {
		t.Fatalf("до записи: %v %v", cat, bad)
	}
	s.remember(fx.catalog(fx.env(3, "sage")))
	if cat, _, bad := s.cached(); cat == nil || cat.Seq != 3 || bad {
		t.Fatalf("в памяти: %+v %v", cat, bad)
	}
	list, err := os.ReadDir(cwd)
	if err != nil || len(list) != 0 {
		t.Fatalf("в текущей папке: %v %v", list, err)
	}
}
