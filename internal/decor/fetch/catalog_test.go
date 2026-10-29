package fetch

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
)

// catalogJSON — body каталога формата 1, seq 1, из строк (поля как пишет library-pack.mjs).
func catalogJSON(t *testing.T, packs ...map[string]any) []byte {
	t.Helper()
	return bodyJSON(t, 1, packs...)
}

func bodyJSON(t *testing.T, seq int64, packs ...map[string]any) []byte {
	t.Helper()
	data, err := json.Marshal(map[string]any{"format": 1, "seq": seq, "packs": packs})
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func row(id string, version int, url string, body []byte) map[string]any {
	sum := sha256.Sum256(body)
	return map[string]any{"id": id, "version": version, "minApp": "0.16.0", "title": "Набор " + id, "about": "о наборе",
		"license": "CC0-1.0", "size": len(body), "sha256": hex.EncodeToString(sum[:]), "url": url,
		"counts": map[string]int{"bg": 2, "sticker": 1, "spread": 1, "frame": 3}}
}

// Подпись конверта: верная (основным и запасным ключом) — каталог читается; чужой ключ,
// обрезанная подпись, подпись не по байтам body, изменённый байт body — ErrSignature; подпись
// сверяется до разбора body.
func TestCatalogSignature(t *testing.T) {
	l := newTestLib(t)
	asset := l.srv.URL + "/releases/download/ivory-v1/ivory-1.zip"
	body := catalogJSON(t, row("ivory", 1, asset, []byte("zip")))
	var env []byte
	l.mux.HandleFunc("/main/catalog.json", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(env) })
	// Старого файла подписи больше нет: его запрос был бы лишним обращением к сети.
	l.mux.HandleFunc("/main/catalog.json.sig", func(w http.ResponseWriter, _ *http.Request) { t.Error("запрошен catalog.json.sig") })
	c := l.client()
	env = l.envelope(body)
	cat, err := c.Catalog(context.Background(), 0)
	if err != nil || cat.Seq != 1 || len(cat.Packs) != 1 || cat.Packs[0].ID != "ivory" || cat.Packs[0].URL != asset || cat.Packs[0].Counts.Bg != 2 || cat.Packs[0].Counts.Frame != 3 {
		t.Fatalf("верная подпись: %v %+v", err, cat)
	}
	if n := l.hits.Load(); n != 1 {
		t.Errorf("запросов %d, ждали один конверт", n)
	}
	// Запасной ключ: первый в списке чужой.
	other, _, _ := ed25519.GenerateKey(rand.Reader)
	src := l.source()
	src.Keys = []ed25519.PublicKey{other, l.pub}
	if _, err := New(src).Catalog(context.Background(), 0); err != nil {
		t.Fatalf("запасной ключ: %v", err)
	}
	src.Keys = []ed25519.PublicKey{other, ed25519.PublicKey(make([]byte, 5))}
	if _, err := New(src).Catalog(context.Background(), 0); !errors.Is(err, ErrSignature) {
		t.Fatalf("чужой ключ: %v", err)
	}
	b64 := base64.StdEncoding.EncodeToString(body)
	good := l.sig(body)
	wrap := func(sig, body string) []byte {
		data, _ := json.Marshal(map[string]string{"sig": sig, "body": body})
		return data
	}
	for name, e := range map[string][]byte{
		"обрезана":       wrap(good[:40], b64),
		"не base64":      wrap("!!!!", b64),
		"пусто":          wrap(" \n", b64),
		"нет sig":        []byte(`{"body":"` + b64 + `"}`),
		"длиннее":        wrap(strings.Repeat("A", MaxSig+1), b64),
		"чужая":          wrap(l.sig([]byte("другой каталог")), b64),
		"по base64 body": wrap(l.sig([]byte(b64)), b64), // подпись по тексту base64, а не по байтам body
		"body изменён":   wrap(good, base64.StdEncoding.EncodeToString(append([]byte(" "), body...))),
		"body не JSON":   wrap(l.sig([]byte("{")), base64.StdEncoding.EncodeToString([]byte("["))), // разбор после подписи
		"нули":           wrap(base64.StdEncoding.EncodeToString(make([]byte, 64)), b64),
	} {
		env = e
		if _, err := c.Catalog(context.Background(), 0); !errors.Is(err, ErrSignature) {
			t.Errorf("%s: %v", name, err)
		}
	}
	env = wrap(" "+good+"\n", b64) // пробелы по краям подписи допустимы
	if _, err := c.Catalog(context.Background(), 0); err != nil {
		t.Errorf("с пробелом: %v", err)
	}
}

// Конверт битый — «не читается» до проверки подписи: не JSON, null, старый формат (каталог
// без конверта), нет body, body не base64, body или конверт длиннее предела.
func TestCatalogEnvelope(t *testing.T) {
	l := newTestLib(t)
	body := catalogJSON(t)
	var env []byte
	l.mux.HandleFunc("/main/catalog.json", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(env) })
	c := l.client()
	sig := l.sig(body)
	big := make([]byte, MaxCatalog+1)
	for name, e := range map[string][]byte{
		"не json":        []byte(`{"sig":`),
		"null":           []byte(`null`),
		"массив":         []byte(`[]`),
		"старый формат":  body,
		"нет body":       []byte(`{"sig":"` + sig + `"}`),
		"body не base64": []byte(`{"sig":"` + sig + `","body":"%%%"}`),
		"body велик":     []byte(`{"sig":"` + l.sig(big) + `","body":"` + base64.StdEncoding.EncodeToString(big) + `"}`),
		"конверт велик":  append(l.envelope(body), bytes.Repeat([]byte(" "), MaxEnvelope)...),
	} {
		env = e
		if _, err := c.Catalog(context.Background(), 0); !errors.Is(err, ErrCatalog) {
			t.Errorf("%s: %v", name, err)
		}
	}
	// Пределы: body ровно MaxCatalog в base64 помещается в конверт.
	if base64.StdEncoding.EncodedLen(MaxCatalog)+MaxSig+64 > MaxEnvelope {
		t.Fatal("конверт не вмещает body предельного размера")
	}
}

// seq: меньше принятого — откат к старому каталогу, ErrSignature; равный (повтор) и больший —
// принимаются; 0, отрицательный, дробный и без seq — каталог не читается.
func TestCatalogSeq(t *testing.T) {
	l := newTestLib(t)
	var env []byte
	l.mux.HandleFunc("/main/catalog.json", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(env) })
	c := l.client()
	env = l.envelope(bodyJSON(t, 7))
	for _, tc := range []struct {
		min  int64
		want error
	}{{0, nil}, {6, nil}, {7, nil}, {8, ErrSignature}, {1 << 40, ErrSignature}} {
		cat, err := c.Catalog(context.Background(), tc.min)
		if !errors.Is(err, tc.want) || err == nil && cat.Seq != 7 {
			t.Errorf("min %d: %v %+v", tc.min, err, cat)
		}
	}
	for name, raw := range map[string]string{
		"0":      `{"format":1,"seq":0,"packs":[]}`,
		"-1":     `{"format":1,"seq":-1,"packs":[]}`,
		"дробь":  `{"format":1,"seq":1.5,"packs":[]}`,
		"строка": `{"format":1,"seq":"2","packs":[]}`,
		"нет":    `{"format":1,"packs":[]}`,
	} {
		env = l.envelope([]byte(raw))
		if _, err := c.Catalog(context.Background(), 0); !errors.Is(err, ErrCatalog) {
			t.Errorf("seq %s: %v", name, err)
		}
	}
}

// Строки каталога: плохие отброшены с причиной, повтор id — первая, тексты чищены, числа зажаты.
func TestParseCatalog(t *testing.T) {
	src := GitHub()
	ok := func(id string) map[string]any { return row(id, 1, AssetPrefix+id+"-v1/"+id+"-1.zip", []byte(id)) }
	with := func(id, key string, v any) map[string]any {
		r := ok(id)
		r[key] = v
		return r
	}
	long := ok("long")
	long["title"] = "  Очень\tдлинное\x00 " + strings.Repeat("я", 100)
	long["counts"] = map[string]int{"bg": -5, "sticker": 1 << 30, "frame": -1}
	addon := with("ivory-addon", "needs", "ivory")
	data := catalogJSON(t, ok("ivory"), long,
		with("Bad", "id", "Bad"), with("ab", "id", "ab"),
		with("v0", "version", 0), with("vbig", "version", 1000000),
		with("minapp", "minApp", "0.16"), with("size0", "size", 0), with("sizebig", "size", MaxZip+1),
		with("sum", "sha256", strings.ToUpper(ok("sum")["sha256"].(string))),
		with("host", "url", "https://evil.example/releases/download/a/b.zip"),
		with("other", "url", "https://github.com/IliaSA101/other/releases/download/a/b.zip"),
		with("http", "url", "http://github.com/IliaSA101/sosninaalbums-library/releases/download/a/b.zip"),
		with("query", "url", AssetPrefix+"a/b.zip?x=1"),
		with("dots", "url", AssetPrefix+"../b.zip"),
		with("deep", "url", AssetPrefix+"a/b/c.zip"),
		with("ivory", "version", 2), // повтор id
		with("notitle", "title", "\u200b"),
		addon, with("badneeds", "needs", "Bad Id"), with("selfneeds", "needs", "selfneeds"), with("longneeds", "needs", strings.Repeat("a", 41)),
	)
	cat, err := ParseCatalog(data, src)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, e := range cat.Packs {
		ids = append(ids, e.ID)
	}
	if strings.Join(ids, ",") != "ivory,long,notitle,ivory-addon" || cat.Packs[0].Version != 1 {
		t.Fatalf("строки: %v", ids)
	}
	if n := len(cat.Skipped); n != 18 {
		t.Fatalf("отброшено %d: %+v", n, cat.Skipped)
	}
	if cat.Skipped[0].ID != "#3" || cat.Skipped[0].Why == "" { // плохой id не попадает в лог как есть
		t.Errorf("пропуск: %+v", cat.Skipped[0])
	}
	lg := cat.Packs[1]
	if strings.ContainsAny(lg.Title, "\t\x00") || !strings.HasPrefix(lg.Title, "Очень длинное я") || len([]rune(lg.Title)) != 60 ||
		lg.Counts.Bg != 0 || lg.Counts.Frame != 0 || lg.Counts.Sticker != maxCount {
		t.Errorf("чистка: %q %+v", lg.Title, lg.Counts)
	}
	if cat.Packs[2].Title != "notitle" {
		t.Errorf("пустое название: %q", cat.Packs[2].Title)
	}
	if cat.Packs[3].Needs != "ivory" || cat.Packs[0].Needs != "" {
		t.Errorf("needs: %q / %q", cat.Packs[3].Needs, cat.Packs[0].Needs)
	}
	// Шапка: формат новее, нет формата, не JSON, больше 500 наборов; пустой список — годен.
	for name, tc := range map[string]struct {
		data []byte
		want error
	}{
		"новее":   {[]byte(`{"format":2,"packs":[]}`), ErrNewer}, // новый формат может быть и без seq
		"нет":     {[]byte(`{"packs":[]}`), ErrCatalog},
		"не json": {[]byte(`[`), ErrCatalog},
		"null":    {[]byte(`null`), ErrCatalog},
		"много":   {[]byte(`{"format":1,"seq":1,"packs":[` + strings.Repeat(`{},`, MaxPacks) + `{}]}`), ErrCatalog},
	} {
		if _, err := ParseCatalog(tc.data, src); !errors.Is(err, tc.want) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if cat, err := ParseCatalog([]byte(`{"format":1,"seq":3,"packs":[],"next":"x"}`), src); err != nil || cat.Seq != 3 || len(cat.Packs) != 0 || cat.Skipped == nil {
		t.Errorf("пустой: %v %+v", err, cat)
	}
}

// minApp: новее программы — нужна новая; dev и неверные строки — не проверяются.
func TestNeedsApp(t *testing.T) {
	for _, tc := range []struct {
		min, app string
		want     bool
	}{
		{"0.16.0", "0.17.0", false}, {"0.17.0", "0.17.0", false}, {"0.17.1", "0.17.0", true},
		{"1.0.0", "0.99.99", true}, {"0.18.0", "dev", false}, {"x", "0.17.0", false},
	} {
		if got := (Entry{MinApp: tc.min}).NeedsApp(tc.app); got != tc.want {
			t.Errorf("%s / %s: %v", tc.min, tc.app, got)
		}
	}
}

// Body и конверт любого содержимого: без паники; принятый body — с seq > 0, каждая принятая
// строка годна для скачивания; конверт с чужой подписью не открывается.
func FuzzParseCatalog(f *testing.F) {
	f.Add([]byte(`{"format":1,"seq":1,"packs":[{"id":"ivory","version":1,"minApp":"0.16.0","title":"А","size":3,"sha256":"` +
		strings.Repeat("a", 64) + `","url":"` + AssetPrefix + `ivory-v1/ivory-1.zip"}]}`))
	f.Add([]byte(`{"format":1,"seq":2,"packs":[{"id":"-x"},{"id":"abc","url":"` + AssetPrefix + `..%2f/x"}]}`))
	f.Add([]byte(`{"format":3}`))
	f.Add([]byte(`{"sig":"` + strings.Repeat("A", 86) + `==","body":"eyJmb3JtYXQiOjEsInNlcSI6MSwicGFja3MiOltdfQ=="}`))
	src := GitHub()
	f.Fuzz(func(t *testing.T, data []byte) {
		if _, err := Open(data, src.Keys); err == nil {
			t.Fatal("конверт без подписи ключом выпуска открыт")
		}
		cat, err := ParseCatalog(data, src)
		if err != nil {
			return
		}
		if cat.Seq < 1 {
			t.Fatalf("seq %d", cat.Seq)
		}
		for _, e := range cat.Packs {
			if !ValidID(e.ID) || !validSum(e.SHA256) || e.Size < 1 || e.Size > MaxZip || !src.assetURL(e.URL) || e.Title == "" ||
				len([]rune(e.Title)) > 60 || e.Counts.Bg < 0 || e.Counts.Bg > maxCount {
				t.Fatalf("строка: %+v", e)
			}
		}
	})
}

// Signed: подписанный конверт любого содержимого — байты body; больше предела, чужая подпись,
// порча — отказ до разбора (спека 111, манифест обновления).
func TestFetchSigned(t *testing.T) {
	l := newTestLib(t)
	body := []byte(`{"any":"thing"}`)
	var env []byte
	l.mux.HandleFunc("/m.json", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(env) })
	c := l.client()
	url := l.srv.URL + "/m.json"
	env = l.envelope(body)
	got, err := c.Signed(context.Background(), url, 4096)
	if err != nil || !bytes.Equal(got, body) {
		t.Fatalf("верный конверт: %v %q", err, got)
	}
	if _, err := c.Signed(context.Background(), url, int64(len(env)-1)); !errors.Is(err, ErrCatalog) {
		t.Errorf("больше предела: %v", err)
	}
	env = bytes.Replace(env, []byte(`"body":"`), []byte(`"body":"AA`), 1)
	if _, err := c.Signed(context.Background(), url, 4096); err == nil {
		t.Error("порченый body принят")
	}
	other, _, _ := ed25519.GenerateKey(rand.Reader)
	src := l.source()
	src.Keys = []ed25519.PublicKey{other}
	env = l.envelope(body)
	if _, err := New(src).Signed(context.Background(), url, 4096); !errors.Is(err, ErrSignature) {
		t.Errorf("чужой ключ: %v", err)
	}
	if _, err := c.Signed(context.Background(), "https://example.com/m.json", 4096); !errors.Is(err, ErrAddress) {
		t.Errorf("чужой хост: %v", err)
	}
}
