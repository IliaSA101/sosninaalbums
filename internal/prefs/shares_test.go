package prefs

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/paths"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// testRules — правила адресов как у share.ServiceURL и share.CheckLink (пакет share в prefs не
// импортируется): origin без пути, https или http, ссылка https до 500 знаков.
var testRules = ShareRules{
	ServiceURL: func(raw string) (string, error) {
		u, err := url.Parse(strings.TrimSpace(raw))
		if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" ||
			(u.Path != "" && u.Path != "/") || (u.Scheme != "https" && u.Scheme != "http") {
			return "", errors.New("негодный адрес")
		}
		return strings.ToLower(u.Scheme + "://" + u.Host), nil
	},
	CheckLink: func(raw string) error {
		if len(raw) > 500 || !strings.HasPrefix(raw, "https://") || strings.ContainsAny(raw, " \\\n") {
			return errors.New("негодная ссылка")
		}
		return nil
	},
}

const (
	testOrigin = "https://albums.sosnina-films.ru"
	testID     = "AbCdEfGhIjKlMnOpQrStUv" // 22 знака
)

// albumPath — абсолютный путь «альбома» в папке данных теста (файл не нужен).
// fileKey — ключ записи в самом файле: на Windows и macOS путь в нижнем регистре (shareKey),
// а имя временной папки теста с заглавными (TestShares…) иначе отбросилось бы как «не ключ».
func fileKey(p string) string { return paths.Fold(filepath.Clean(p)) }

func albumPath(dir, name string) string { return filepath.Join(dir, "albums", name+".salbum") }

func rec(n int) ShareRecord {
	return ShareRecord{
		Origin:      testOrigin,
		ID:          fmt.Sprintf("%s%02d", testID, n),
		URL:         fmt.Sprintf("%s/a/%s%02d/", testOrigin, testID, n),
		Title:       fmt.Sprintf("Свадьба %d", n),
		Password:    fmt.Sprintf("слово-%04d", n),
		ExpiresAt:   time.Date(2027, 1, 2, 3, 4, 5, 0, time.UTC),
		PublishedAt: time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC),
		Scene:       strings.Repeat("ab", 32),
	}
}

func TestSharesFile(t *testing.T) {
	dir := testkit.TempDir(t)
	s := NewShares(dir, testRules)

	// Пусто: нет файла — нет записей, файл не появляется от чтения.
	if _, ok := s.Get(albumPath(dir, "a")); ok {
		t.Fatal("запись из пустоты")
	}
	if _, err := os.Lstat(SharesPath(dir)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("чтение создало файл: %v", err)
	}

	// Несколько записей сразу; круг Put → Load (новый объект) → Get.
	const n = 5
	for i := range n {
		if err := s.Put(albumPath(dir, fmt.Sprint(i)), rec(i)); err != nil {
			t.Fatal(err)
		}
	}
	re := NewShares(dir, testRules)
	for i := range n {
		got, ok := re.Get(albumPath(dir, fmt.Sprint(i)))
		if !ok || !reflect.DeepEqual(got, rec(i)) {
			t.Errorf("запись %d после круга:\n got %+v\nwant %+v", i, got, rec(i))
		}
	}
	// Разные записи не путаются: другой альбом — другая запись, чужого пути — нет.
	if got, _ := re.Get(albumPath(dir, "0")); got.ID == rec(1).ID {
		t.Error("записи разных альбомов совпали")
	}
	if _, ok := re.Get(albumPath(dir, "нет")); ok {
		t.Error("запись у альбома без выгрузки")
	}

	// Права 0600 и никаких временных файлов рядом.
	if runtime.GOOS != "windows" {
		if st, err := os.Stat(SharesPath(dir)); err != nil || st.Mode().Perm() != 0o600 {
			t.Errorf("права shares.json: %v, %v", st, err)
		}
	}
	if left, _ := filepath.Glob(filepath.Join(dir, "shares.json.*")); len(left) != 0 {
		t.Errorf("остатки записи: %v", left)
	}

	sharesUpdate(t, dir, re)
	sharesDelete(t, dir, re)
}

// sharesUpdate: Update меняет только свою запись, холостой и негодный файл не трогают.
func sharesUpdate(t *testing.T, dir string, re *Shares) {
	t.Helper()
	before, _ := os.ReadFile(SharesPath(dir))
	// Нет записи — ok=false, файл не пишется.
	got, ok, err := re.Update(albumPath(dir, "2"), func(r *ShareRecord) { r.Gone, r.Title = true, "Новое" })
	if err != nil || !ok || !got.Gone || got.Title != "Новое" {
		t.Fatalf("Update: %+v %v %v", got, ok, err)
	}
	if g, _ := NewShares(dir, testRules).Get(albumPath(dir, "3")); !reflect.DeepEqual(g, rec(3)) {
		t.Errorf("Update задел соседа: %+v", g)
	}
	mid, _ := os.ReadFile(SharesPath(dir))
	if _, ok, err := re.Update(albumPath(dir, "нет"), func(*ShareRecord) { t.Error("fn без записи") }); ok || err != nil {
		t.Errorf("Update без записи: %v %v", ok, err)
	}
	if after, _ := os.ReadFile(SharesPath(dir)); string(after) != string(mid) || string(mid) == string(before) {
		t.Error("холостой Update изменил файл или рабочий не изменил")
	}
	// Update, делающий запись негодной, отвергается; файл прежний.
	if _, _, err := re.Update(albumPath(dir, "2"), func(r *ShareRecord) { r.URL = "http://чужой" }); !errors.Is(err, ErrShareRecord) {
		t.Errorf("Update негодной: %v", err)
	}
	if after, _ := os.ReadFile(SharesPath(dir)); string(after) != string(mid) {
		t.Error("негодный Update изменил файл")
	}
}

// sharesDelete: Delete убирает одну запись; повторный и холостой файл не переписывают.
func sharesDelete(t *testing.T, dir string, re *Shares) {
	t.Helper()
	// Delete: одной записи; повторный и холостой — без ошибки и без записи файла.
	if err := re.Delete(albumPath(dir, "1")); err != nil {
		t.Fatal(err)
	}
	if _, ok := NewShares(dir, testRules).Get(albumPath(dir, "1")); ok {
		t.Error("запись осталась после Delete")
	}
	if g, ok := NewShares(dir, testRules).Get(albumPath(dir, "0")); !ok || g.ID != rec(0).ID {
		t.Error("Delete убрал лишнее")
	}
	mid, _ := os.ReadFile(SharesPath(dir))
	if err := re.Delete(albumPath(dir, "1")); err != nil {
		t.Fatal(err)
	}
	if after, _ := os.ReadFile(SharesPath(dir)); string(after) != string(mid) {
		t.Error("холостой Delete переписал файл")
	}
}

// Пустые и крайние значения: запись без ссылки, пароля и сроков; id на границах.
func TestSharesEdges(t *testing.T) {
	dir := testkit.TempDir(t)
	s := NewShares(dir, testRules)
	p := albumPath(dir, "x")
	bare := ShareRecord{Origin: testOrigin, ID: testID} // начатая выгрузка: id есть, ссылки нет
	if err := s.Put(p, bare); err != nil {
		t.Fatal(err)
	}
	if got, ok := NewShares(dir, testRules).Get(p); !ok || !reflect.DeepEqual(got, bare) {
		t.Errorf("пустая запись: %+v %v", got, ok)
	}
	raw, _ := os.ReadFile(SharesPath(dir))
	for _, k := range []string{"expires_at", "published_at", "password", "url", "gone"} {
		if strings.Contains(string(raw), `"`+k+`"`) {
			t.Errorf("пустое поле %s записано в файл", k)
		}
	}
	// id: 22 и 64 знака годятся, 21 и 65 — нет, первый «-» — нет, чужие знаки — нет.
	for id, want := range map[string]bool{
		strings.Repeat("a", 22): true, strings.Repeat("A", 64): true, "a-_" + strings.Repeat("9", 19): true,
		strings.Repeat("a", 21): false, strings.Repeat("a", 65): false, "-" + strings.Repeat("a", 21): false,
		strings.Repeat("a", 21) + "/": false, strings.Repeat("a", 21) + "é": false, "": false,
	} {
		r := bare
		r.ID = id
		if err := s.Put(p, r); (err == nil) != want {
			t.Errorf("id %q: ошибка %v, годен должен быть %v", id, err, want)
		}
	}
	// Название: чистится (управляющие, bidi, длина), пароль: слишком длинный — отказ.
	r := bare
	r.Title = "  а\u202eб\x00в" + strings.Repeat("я", 300)
	if err := s.Put(p, r); err != nil {
		t.Fatal(err)
	}
	got, _ := s.Get(p)
	if want := "абв" + strings.Repeat("я", 197); got.Title != want {
		t.Errorf("название %q", got.Title)
	}
	for _, pw := range []string{strings.Repeat("п", 129), "a\nb"} {
		r.Password = pw
		if err := s.Put(p, r); !errors.Is(err, ErrShareRecord) {
			t.Errorf("пароль %.10q: %v", pw, err)
		}
	}
	// Негодный путь альбома: относительный, пустой.
	for _, bad := range []string{"", "rel/a.salbum"} {
		if err := s.Put(bad, bare); !errors.Is(err, ErrSharePath) {
			t.Errorf("Put(%q): %v", bad, err)
		}
		if _, ok := s.Get(bad); ok {
			t.Errorf("Get(%q) нашёл запись", bad)
		}
	}
}

// Подделанный файл: записи с негодным url, id, origin, паролем или ключом отбрасываются,
// годные остаются (разбор №15, №16).
func TestSharesTampered(t *testing.T) {
	dir := testkit.TempDir(t)
	good := albumPath(dir, "good")
	mk := func(mod func(r *ShareRecord)) json.RawMessage {
		r := rec(1)
		mod(&r)
		b, _ := json.Marshal(r)
		return b
	}
	albums := map[string]json.RawMessage{
		fileKey(good):                            mk(func(*ShareRecord) {}),
		fileKey(albumPath(dir, "javascript")):    mk(func(r *ShareRecord) { r.URL = "javascript:alert(1)" }),
		fileKey(albumPath(dir, "http")):          mk(func(r *ShareRecord) { r.URL = "http://evil.example/a" }),
		fileKey(albumPath(dir, "userinfo")):      mk(func(r *ShareRecord) { r.Origin = "https://user:pw@evil.example" }),
		fileKey(albumPath(dir, "origin")):        mk(func(r *ShareRecord) { r.Origin = "ftp://x" }),
		fileKey(albumPath(dir, "noorigin")):      mk(func(r *ShareRecord) { r.Origin = "" }),
		fileKey(albumPath(dir, "shortid")):       mk(func(r *ShareRecord) { r.ID = "short" }),
		fileKey(albumPath(dir, "pathid")):        mk(func(r *ShareRecord) { r.ID = "../../../../etc/passwd/aaaaaaaa" }),
		fileKey(albumPath(dir, "pass")):          mk(func(r *ShareRecord) { r.Password = "a\x01b" }),
		fileKey(albumPath(dir, "time")):          json.RawMessage(`{"origin":"` + testOrigin + `","id":"` + testID + `","expires_at":"вчера"}`),
		fileKey(albumPath(dir, "notobj")):        json.RawMessage(`[1,2]`),
		"relative/path.salbum":                   mk(func(*ShareRecord) {}),
		paths.Fold(dir + "/albums/../up.salbum"): mk(func(*ShareRecord) {}), // не в виде Clean
	}
	writeSharesRaw(t, dir, sharesVersion, albums)
	s := NewShares(dir, testRules)
	if _, ok := s.Get(good); !ok {
		t.Fatal("годная запись потеряна")
	}
	for k := range albums {
		if k == fileKey(good) {
			continue
		}
		if r, ok := s.Get(k); ok {
			t.Errorf("запись %q принята: %+v", k, r)
		}
	}
	// Запись другого файла сохраняет только годные: негодные уходят при первой записи.
	if err := s.Put(albumPath(dir, "new"), rec(2)); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(SharesPath(dir))
	for _, bad := range []string{"javascript:", "evil.example", "passwd", "relative/path"} {
		if strings.Contains(string(raw), bad) {
			t.Errorf("негодное %q осталось в файле", bad)
		}
	}
	// Без правил (nil) ничего не принимается: отказ по умолчанию.
	if _, ok := NewShares(dir, ShareRules{}).Get(good); ok {
		t.Error("запись принята без правил")
	}
}

func writeSharesRaw(t *testing.T, dir string, version int, albums map[string]json.RawMessage) {
	t.Helper()
	data, err := json.Marshal(sharesFile{Version: version, Albums: albums})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(SharesPath(dir), data, 0o600); err != nil {
		t.Fatal(err)
	}
}

// Битый, пустой, слишком большой и «не тот» файл — в .bad и пусто; программа работает дальше.
func TestSharesBadFile(t *testing.T) {
	big := `{"version":1,"albums":{"x":"` + strings.Repeat("a", maxSharesFile) + `"}}`
	many := map[string]json.RawMessage{}
	for i := range maxShares + 1 {
		many[fmt.Sprintf("/a/%d.salbum", i)] = json.RawMessage(`{}`)
	}
	manyData, _ := json.Marshal(sharesFile{Version: 1, Albums: many})
	cases := map[string]string{
		"битый JSON":     `{"version":1,"albums":{`,
		"пустой":         ``,
		"нет версии":     `{"albums":{}}`,
		"массив":         `[]`,
		"больше предела": big,
		"записей > 1000": string(manyData),
	}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			dir := testkit.TempDir(t)
			if err := os.WriteFile(SharesPath(dir), []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			s := NewShares(dir, testRules)
			if _, ok := s.Get(albumPath(dir, "x")); ok {
				t.Fatal("запись из битого файла")
			}
			if got, err := os.ReadFile(SharesPath(dir) + ".bad"); err != nil || string(got) != content {
				t.Errorf(".bad: %v", err)
			}
			if err := s.Put(albumPath(dir, "x"), rec(1)); err != nil {
				t.Fatal(err)
			}
			if _, ok := NewShares(dir, testRules).Get(albumPath(dir, "x")); !ok {
				t.Error("новая запись не записалась после .bad")
			}
		})
	}
}

// Файл новее программы: читаем, не пишем и не портим.
func TestSharesNewer(t *testing.T) {
	dir := testkit.TempDir(t)
	p := albumPath(dir, "x")
	r, _ := json.Marshal(rec(1))
	writeSharesRaw(t, dir, sharesVersion+1, map[string]json.RawMessage{fileKey(p): r})
	before, _ := os.ReadFile(SharesPath(dir))
	s := NewShares(dir, testRules)
	if _, ok := s.Get(p); !ok {
		t.Error("запись новой версии не прочитана")
	}
	if err := s.Put(albumPath(dir, "y"), rec(2)); !errors.Is(err, ErrSharesNewer) {
		t.Errorf("Put: %v", err)
	}
	if err := s.Delete(p); !errors.Is(err, ErrSharesNewer) {
		t.Errorf("Delete: %v", err)
	}
	if after, _ := os.ReadFile(SharesPath(dir)); string(after) != string(before) {
		t.Error("файл новой версии изменён")
	}
}

// Файл новее программы не уводится в .bad и не переписывается ни при превышении пределов
// (записей > 1000, размер > 1 МБ), ни когда его не разобрать целиком.
func TestSharesNewerOverLimits(t *testing.T) {
	many := map[string]json.RawMessage{}
	for i := range maxShares + 5 {
		many[fmt.Sprintf("/a/%04d.salbum", i)] = mustJSON(rec(i % 90))
	}
	manyData, _ := json.Marshal(sharesFile{Version: sharesVersion + 1, Albums: many})
	cases := map[string]string{
		"записей > 1000": string(manyData),
		"больше 1 МБ":    `{"version": 2, "albums": {"x": "` + strings.Repeat("a", maxSharesFile) + `"}}`,
		"усечён":         `{"version":3,"albums":{"/a/b.salbum":{"origin":`,
		"сложный пробел": "\n {\n \"version\" : 7, \"albums\": [",
	}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			dir := testkit.TempDir(t)
			if err := os.WriteFile(SharesPath(dir), []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			s := NewShares(dir, testRules)
			_, _ = s.Get(albumPath(dir, "x"))
			if err := s.Put(albumPath(dir, "x"), rec(1)); !errors.Is(err, ErrSharesNewer) {
				t.Errorf("Put: %v", err)
			}
			if _, err := os.Lstat(SharesPath(dir) + ".bad"); err == nil {
				t.Error("файл новой версии ушёл в .bad")
			}
			if got, _ := os.ReadFile(SharesPath(dir)); string(got) != content {
				t.Error("файл новой версии изменён")
			}
		})
	}
	// Записей > 1000, файл разбирается: читаются первые 1000 по порядку ключей.
	dir := testkit.TempDir(t)
	_ = os.WriteFile(SharesPath(dir), manyData, 0o600)
	s := NewShares(dir, testRules)
	if _, ok := s.Get("/a/0000.salbum"); !ok && runtime.GOOS != "windows" {
		t.Error("первая запись не прочитана")
	}
	if _, ok := s.Get(fmt.Sprintf("/a/%04d.salbum", maxShares+2)); ok {
		t.Error("прочитана запись за пределом")
	}
}

// Не вышло отложить битый файл — запись отказывает и файл остаётся как был (копии нет — затирать нельзя).
func TestSharesQuarantineFails(t *testing.T) {
	dir := testkit.TempDir(t)
	const content = `{"version":1,"albums":{`
	if err := os.WriteFile(SharesPath(dir), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	old := renameFile
	renameFile = func(string, string) error { return errors.New("доступ закрыт") }
	defer func() { renameFile = old }()
	s := NewShares(dir, testRules)
	if err := s.Put(albumPath(dir, "x"), rec(1)); !errors.Is(err, ErrSharesUnread) || strings.Contains(err.Error(), "доступ") {
		t.Errorf("Put: %v", err)
	}
	if got, _ := os.ReadFile(SharesPath(dir)); string(got) != content {
		t.Error("битый файл затёрт без копии")
	}
	renameFile = old // переименование заработало — следующая запись проходит
	if err := s.Put(albumPath(dir, "x"), rec(1)); err != nil {
		t.Errorf("Put после починки: %v", err)
	}
	if got, _ := os.ReadFile(SharesPath(dir) + ".bad"); string(got) != content {
		t.Error("копия не сделана")
	}
}

// T3-7: пока битый файл не отложить, журнал о нём — одна запись, а не две на каждое чтение; после
// удачного переноса новая поломка снова пишется.
func TestSharesQuarantineLogOnce(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	dir := testkit.TempDir(t)
	if err := os.WriteFile(SharesPath(dir), []byte(`{"version":1,"albums":{`), 0o600); err != nil {
		t.Fatal(err)
	}
	old := renameFile
	renameFile = func(string, string) error { return errors.New("доступ закрыт") }
	defer func() { renameFile = old }()
	s := NewShares(dir, testRules)
	for range 6 { // и чтения, и записи: каждое заново разбирает файл
		s.Get(albumPath(dir, "x"))
		_ = s.Put(albumPath(dir, "x"), rec(1))
	}
	if n := strings.Count(buf.String(), "shares.json не разобран"); n != 1 {
		t.Errorf("записей журнала %d, ждали 1:\n%s", n, buf.String())
	}
	renameFile = old
	s.Get(albumPath(dir, "x")) // переименование заработало: файл отложен
	if _, err := os.Stat(SharesPath(dir) + ".bad"); err != nil {
		t.Fatalf("копия не сделана: %v", err)
	}
	renameFile = func(string, string) error { return errors.New("снова закрыт") }
	if err := os.WriteFile(SharesPath(dir), []byte(`{"version":1,"albums":{`), 0o600); err != nil {
		t.Fatal(err)
	}
	buf.Reset()
	s.Get(albumPath(dir, "x"))
	s.Get(albumPath(dir, "x"))
	if n := strings.Count(buf.String(), "shares.json не разобран"); n != 1 {
		t.Errorf("новая поломка: записей %d, ждали 1", n)
	}
}

// Не записался config.json — SetShareURL говорит об этом (адрес прежний).
func TestShareConfigSaveError(t *testing.T) {
	blocker := filepath.Join(testkit.TempDir(t), "file")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	// «Папка данных» — обычный файл: config.json в нём не создать.
	got, err := SetShareURL(blocker, testOrigin, testRules)
	if !errors.Is(err, ErrShareSave) || got != "" {
		t.Errorf("SetShareURL: %q, %v", got, err)
	}
	if _, err := SetShareURL(blocker, "", testRules); !errors.Is(err, ErrShareSave) {
		t.Errorf("SetShareURL(пусто): %v", err)
	}
}

// Предел записей: 1000-я принимается, 1001-я — ErrSharesFull; существующая заменяется.
func TestSharesLimit(t *testing.T) {
	dir := testkit.TempDir(t)
	s := NewShares("", testRules) // в памяти: быстро
	for i := range maxShares {
		if err := s.Put(albumPath(dir, fmt.Sprint(i)), rec(i%90)); err != nil {
			t.Fatal(i, err)
		}
	}
	if err := s.Put(albumPath(dir, "лишний"), rec(1)); !errors.Is(err, ErrSharesFull) {
		t.Errorf("1001-я: %v", err)
	}
	if err := s.Put(albumPath(dir, "5"), rec(7)); err != nil {
		t.Errorf("замена при полном: %v", err)
	}
}

// Истёкшие больше 30 дней назад выбрасываются при записи, свежее истёкшие остаются (№29).
func TestSharesExpire(t *testing.T) {
	dir := testkit.TempDir(t)
	s := NewShares(dir, testRules)
	now := time.Date(2027, 6, 1, 0, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return now }
	put := func(name string, exp time.Time) {
		r := rec(1)
		r.ExpiresAt = exp
		if err := s.Put(albumPath(dir, name), r); err != nil {
			t.Fatal(err)
		}
	}
	put("old", now.Add(-31*24*time.Hour))
	put("edge", now.Add(-30*24*time.Hour))
	put("fresh", now.Add(-time.Hour))
	put("future", now.Add(24*time.Hour))
	put("noexp", time.Time{})
	for name, want := range map[string]bool{"old": false, "edge": true, "fresh": true, "future": true, "noexp": true} {
		if _, ok := NewShares(dir, testRules).Get(albumPath(dir, name)); ok != want {
			t.Errorf("%s: есть=%v, ждали %v", name, ok, want)
		}
	}
	// Читаем со старой записью в файле — чистка при следующей записи, чтение её отдаёт.
	now = now.Add(100 * 24 * time.Hour)
	if _, ok := s.Get(albumPath(dir, "fresh")); !ok {
		t.Error("чтение чистит записи")
	}
	if err := s.Delete(albumPath(dir, "noexp")); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"edge", "fresh", "future"} {
		if _, ok := s.Get(albumPath(dir, name)); ok {
			t.Errorf("%s не вычищена спустя 100 дней", name)
		}
	}
}

// Параллельные записи не теряют друг друга (ход выгрузки и «Изменить» пишут одновременно, №29).
func TestSharesParallel(t *testing.T) {
	dir := testkit.TempDir(t)
	var wg sync.WaitGroup
	const n = 20
	for i := range n {
		wg.Go(func() {
			s := NewShares(dir, testRules) // у каждого горутины свой объект: замок общий, на пакет
			if err := s.Put(albumPath(dir, fmt.Sprint(i)), rec(i)); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	s := NewShares(dir, testRules)
	for i := range n {
		if _, ok := s.Get(albumPath(dir, fmt.Sprint(i))); !ok {
			t.Errorf("запись %d потеряна", i)
		}
	}
}

// Папки данных нет: записи живут в памяти до выхода, файлов не создаётся.
func TestSharesNoDir(t *testing.T) {
	s := NewShares("", testRules)
	p := filepath.Join(testkit.TempDir(t), "a.salbum")
	if err := s.Put(p, rec(1)); err != nil {
		t.Fatal(err)
	}
	if got, ok := s.Get(p); !ok || !reflect.DeepEqual(got, rec(1)) {
		t.Errorf("запись в памяти: %+v %v", got, ok)
	}
	if _, ok := NewShares("", testRules).Get(p); ok {
		t.Error("память делится между объектами")
	}
	if err := s.Delete(p); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.Get(p); ok {
		t.Error("запись осталась")
	}
	if SharesPath("") != "" {
		t.Error("путь без папки данных")
	}
}

// Токена нет ни в записи, ни в конфиге: ни полем, ни в файле.
func TestSharesNoToken(t *testing.T) {
	for _, typ := range []reflect.Type{reflect.TypeFor[ShareRecord](), reflect.TypeFor[ShareConfig]()} {
		for f := range typ.Fields() {
			if n := strings.ToLower(f.Name + f.Tag.Get("json")); strings.Contains(n, "token") || strings.Contains(n, "secret") {
				t.Errorf("%s.%s похоже на токен", typ.Name(), f.Name)
			}
		}
	}
	dir := testkit.TempDir(t)
	if _, err := SetShareURL(dir, testOrigin, testRules); err != nil {
		t.Fatal(err)
	}
	if err := NewShares(dir, testRules).Put(albumPath(dir, "a"), rec(1)); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{ConfigPath(dir), SharesPath(dir)} {
		raw, _ := os.ReadFile(p)
		if strings.Contains(strings.ToLower(string(raw)), `"token`) {
			t.Errorf("%s содержит token: %s", filepath.Base(p), raw)
		}
	}
}

func TestShareConfig(t *testing.T) {
	dir := testkit.TempDir(t)
	if got := ShareURLOf(dir, testRules); got != "" {
		t.Errorf("пустой конфиг: %q", got)
	}
	// Чужие ключи config.json сохраняются при записи share; круг запись → чтение.
	UpdateConfig(dir, func(c *Config) { c.Level = LevelFull })
	raw, _ := os.ReadFile(ConfigPath(dir))
	var m map[string]json.RawMessage
	_ = json.Unmarshal(raw, &m)
	m["futureKey"] = json.RawMessage(`{"x":1}`)
	raw, _ = json.Marshal(m)
	_ = os.WriteFile(ConfigPath(dir), raw, 0o600)

	got, err := SetShareURL(dir, " HTTPS://Albums.Sosnina-Films.RU/ ", testRules)
	if err != nil || got != testOrigin {
		t.Fatalf("SetShareURL: %q %v", got, err)
	}
	if got := ShareURLOf(dir, testRules); got != testOrigin {
		t.Errorf("ShareURLOf: %q", got)
	}
	raw, _ = os.ReadFile(ConfigPath(dir))
	if !strings.Contains(string(raw), "futureKey") || LoadConfig(dir).Level != LevelFull {
		t.Errorf("чужие ключи потеряны: %s", raw)
	}
	// Негодный адрес — отказ, прежний остаётся.
	for _, bad := range []string{"javascript:x", "https://u:p@h.example", "https://h.example/путь?x", "ftp://h.example", "  "} {
		if _, err := SetShareURL(dir, bad, testRules); !errors.Is(err, ErrShareURL) {
			t.Errorf("SetShareURL(%q): %v", bad, err)
		}
	}
	if got := ShareURLOf(dir, testRules); got != testOrigin {
		t.Errorf("негодный адрес сменил прежний: %q", got)
	}
	// Подделанный файл: адрес не проходит правила — «не подключено».
	UpdateConfig(dir, func(c *Config) { c.Share = &ShareConfig{URL: "https://user:pw@evil.example"} })
	if got := ShareURLOf(dir, testRules); got != "" {
		t.Errorf("подделанный адрес принят: %q", got)
	}
	if got := ShareURLOf(dir, ShareRules{}); got != "" {
		t.Errorf("без правил: %q", got)
	}
	// Пустой адрес убирает ключ.
	_, _ = SetShareURL(dir, testOrigin, testRules)
	if got, err := SetShareURL(dir, "", testRules); got != "" || err != nil {
		t.Fatal(got, err)
	}
	if raw, _ = os.ReadFile(ConfigPath(dir)); strings.Contains(string(raw), `"share"`) {
		t.Errorf("ключ share остался: %s", raw)
	}
}

// FuzzSharesFile: любое содержимое shares.json — без паники; принятые записи проходят правила
// повторно без изменений.
func FuzzSharesFile(f *testing.F) {
	good, _ := json.Marshal(sharesFile{Version: 1, Albums: map[string]json.RawMessage{"/a/b.salbum": mustJSON(rec(1))}})
	for _, seed := range []string{"", "{}", "[]", "null", `{"version":1}`, `{"version":1,"albums":null}`, string(good),
		`{"version":9,"albums":{"/a":{}}}`, `{"version":1,"albums":{"/a/b.salbum":{"expires_at":"x"}}}`} {
		f.Add([]byte(seed))
	}
	dir := testkit.TempDir(f)
	f.Fuzz(func(t *testing.T, data []byte) {
		if err := os.WriteFile(SharesPath(dir), data, 0o600); err != nil {
			t.Fatal(err)
		}
		_ = os.Remove(SharesPath(dir) + ".bad")
		s := NewShares(dir, testRules)
		recs, _ := s.load()
		for k, r := range recs {
			if again, ok := testRules.clean(r); !ok || !sameRec(again, r) {
				t.Fatalf("принятая запись %q не идемпотентна: %+v", k, r)
			}
		}
	})
}

func mustJSON(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

// Отпечаток проекта в записи (146 T4-9): годный проходит круг через диск, негодный (пусто,
// верхний регистр, длина, не hex) — "" и запись остаётся (get в sharer считает её чужой).
func TestSharesUID(t *testing.T) {
	dir := testkit.TempDir(t)
	s := NewShares(dir, testRules)
	uids := []string{
		"0123456789abcdef0123456789abcdef", "", "0123456789ABCDEF0123456789ABCDEF",
		"0123456789abcdef", "0123456789abcdef0123456789abcdeg", "0123456789abcdef0123456789abcdef0",
	}
	for i, u := range uids {
		r := rec(i)
		r.UID = u
		if err := s.Put(albumPath(dir, fmt.Sprint(i)), r); err != nil {
			t.Fatal(err)
		}
	}
	re := NewShares(dir, testRules)
	for i, u := range uids {
		got, ok := re.Get(albumPath(dir, fmt.Sprint(i)))
		want := ""
		if i == 0 {
			want = u
		}
		if !ok || got.UID != want || got.ID != rec(i).ID {
			t.Errorf("запись %d (%q): %v, отпечаток %q", i, u, ok, got.UID)
		}
	}
	// Смена одного отпечатка — запись другая (sameRec): перезапись доходит до диска.
	r := rec(0)
	r.UID = "fedcba9876543210fedcba9876543210"
	if err := re.Put(albumPath(dir, "0"), r); err != nil {
		t.Fatal(err)
	}
	if got, _ := NewShares(dir, testRules).Get(albumPath(dir, "0")); got.UID != r.UID {
		t.Fatalf("смена отпечатка не записана: %q", got.UID)
	}
}
