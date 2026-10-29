package albums

import (
	"encoding/json"
	"errors"
	"math"
	"strings"
	"testing"
	"time"
)

// testSHA — sha256 из одного знака: годится как значение, содержимое не проверяется.
func testSHA(c byte) string { return strings.Repeat(string(c), 64) }

// goodManifest — книга из k страниц; с обложкой первая front, последняя back; вторая белая.
func goodManifest(k int, cover bool) *Manifest {
	m := &Manifest{
		Schema: 1, Generator: "Sosnina Albums 0.32.0", Title: "Свадьба И. и А.",
		Book: Book{Cover: cover, PageMM: Size{W: 297, H: 294}},
	}
	for i := 0; i < k; i++ {
		p := Page{N: i + 1, Role: RolePage, Type: TypeJPEG, SHA256: testSHA("0123456789abcdef"[i%16]), Bytes: 1000 + int64(i), W: 1600, H: 1584}
		if cover && i == 0 {
			p.Role = RoleFront
		}
		if cover && i == k-1 {
			p.Role = RoleBack
		}
		if i == 1 {
			p = Page{N: 2, Role: p.Role, Blank: true}
		}
		m.Pages = append(m.Pages, p)
	}
	return m
}

func wantInvalid(t *testing.T, what string, err error) {
	t.Helper()
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("%s: ожидалась ValidationError, получено %v", what, err)
	}
}

func TestValidate(t *testing.T) {
	for _, tc := range []struct {
		k     int
		cover bool
	}{{2, false}, {2, true}, {4, false}, {62, true}, {300, false}} {
		if err := Validate(goodManifest(tc.k, tc.cover), Limits{}); err != nil {
			t.Errorf("k=%d cover=%v: %v", tc.k, tc.cover, err)
		}
	}
	// веб-формат тоже годен
	m := goodManifest(4, false)
	m.Pages[0].Type = TypeWebP
	if err := Validate(m, Limits{}); err != nil {
		t.Errorf("webp: %v", err)
	}
}

func TestValidateBad(t *testing.T) {
	cases := map[string]func(m *Manifest){
		"schema 2":             func(m *Manifest) { m.Schema = 2 },
		"schema 0":             func(m *Manifest) { m.Schema = 0 },
		"нет страниц":          func(m *Manifest) { m.Pages = nil },
		"одна страница":        func(m *Manifest) { m.Pages = m.Pages[:1] },
		"нечётное число":       func(m *Manifest) { m.Pages = m.Pages[:3] },
		"пропуск в n":          func(m *Manifest) { m.Pages[2].N = 4 },
		"n с нуля":             func(m *Manifest) { m.Pages[0].N = 0 },
		"роль чужая":           func(m *Manifest) { m.Pages[0].Role = "cover" },
		"front без обложки":    func(m *Manifest) { m.Pages[0].Role = RoleFront },
		"back без обложки":     func(m *Manifest) { m.Pages[3].Role = RoleBack },
		"тип gif":              func(m *Manifest) { m.Pages[0].Type = "image/gif" },
		"тип пуст":             func(m *Manifest) { m.Pages[0].Type = "" },
		"sha короткий":         func(m *Manifest) { m.Pages[0].SHA256 = "abc" },
		"sha в верхнем":        func(m *Manifest) { m.Pages[0].SHA256 = strings.ToUpper(testSHA('a')) },
		"sha не hex":           func(m *Manifest) { m.Pages[0].SHA256 = testSHA('g') },
		"bytes 0":              func(m *Manifest) { m.Pages[0].Bytes = 0 },
		"bytes минус":          func(m *Manifest) { m.Pages[0].Bytes = -5 },
		"bytes больше":         func(m *Manifest) { m.Pages[0].Bytes = DefaultLimits().PageBytes + 1 },
		"w 0":                  func(m *Manifest) { m.Pages[0].W = 0 },
		"h больше":             func(m *Manifest) { m.Pages[0].H = MaxSide + 1 },
		"белая с хэшем":        func(m *Manifest) { m.Pages[1].SHA256 = testSHA('a') },
		"белая с байтами":      func(m *Manifest) { m.Pages[1].Bytes = 1 },
		"белая с типом":        func(m *Manifest) { m.Pages[1].Type = TypeJPEG },
		"белая с размером":     func(m *Manifest) { m.Pages[1].W = 10 },
		"file на входе":        func(m *Manifest) { m.Pages[0].File = "001.jpg" },
		"file с обходом":       func(m *Manifest) { m.Pages[2].File = "../../etc/x" },
		"generator пуст":       func(m *Manifest) { m.Generator = "" },
		"generator длинный":    func(m *Manifest) { m.Generator = strings.Repeat("g", MaxGenerator+1) },
		"generator с \\n":      func(m *Manifest) { m.Generator = "g\nx" },
		"generator не UTF-8":   func(m *Manifest) { m.Generator = "g\xff" },
		"title пуст":           func(m *Manifest) { m.Title = "" },
		"title из пробелов":    func(m *Manifest) { m.Title = "   " },
		"title 201 знак":       func(m *Manifest) { m.Title = strings.Repeat("я", MaxTitle+1) },
		"title 801 байт":       func(m *Manifest) { m.Title = strings.Repeat("😀", 199) + "ab" },
		"title bidi":           func(m *Manifest) { m.Title = "Свадьба \u202eкот" },
		"title нулевая ширина": func(m *Manifest) { m.Title = "А\u200bБ" },
		"title с \\n":          func(m *Manifest) { m.Title = "А\nБ" },
		"title не UTF-8":       func(m *Manifest) { m.Title = "А\xffБ" },
		"title разделитель":    func(m *Manifest) { m.Title = "А\u2028Б" },
		"page_mm мал":          func(m *Manifest) { m.Book.PageMM.W = 9.9 },
		"page_mm велик":        func(m *Manifest) { m.Book.PageMM.H = 2000.1 },
		"page_mm NaN":          func(m *Manifest) { m.Book.PageMM.W = math.NaN() },
		"page_mm Inf":          func(m *Manifest) { m.Book.PageMM.H = math.Inf(1) },
		"page_mm 0":            func(m *Manifest) { m.Book.PageMM = Size{} },
	}
	for name, mut := range cases {
		m := goodManifest(4, false)
		mut(m)
		wantInvalid(t, name, Validate(m, Limits{}))
	}
	wantInvalid(t, "nil", Validate(nil, Limits{}))
	// с обложкой: роли первой и последней обязательны
	for name, mut := range map[string]func(m *Manifest){
		"первая не front":   func(m *Manifest) { m.Pages[0].Role = RolePage },
		"последняя не back": func(m *Manifest) { m.Pages[3].Role = RolePage },
		"front в середине":  func(m *Manifest) { m.Pages[2].Role = RoleFront },
		"back первой":       func(m *Manifest) { m.Pages[0].Role = RoleBack },
	} {
		m := goodManifest(4, true)
		mut(m)
		wantInvalid(t, name, Validate(m, Limits{}))
	}
}

func TestValidateLimits(t *testing.T) {
	m := goodManifest(8, false)
	wantInvalid(t, "страниц больше", Validate(m, Limits{Pages: 6}))
	if err := Validate(m, Limits{Pages: 8}); err != nil {
		t.Errorf("ровно предел: %v", err)
	}
	wantInvalid(t, "страница больше", Validate(m, Limits{PageBytes: 1000}))
	var sum int64
	for _, p := range m.Pages {
		sum += p.Bytes
	}
	wantInvalid(t, "альбом больше", Validate(m, Limits{AlbumBytes: sum - 1}))
	if err := Validate(m, Limits{AlbumBytes: sum}); err != nil {
		t.Errorf("альбом ровно предел: %v", err)
	}
	wantInvalid(t, "title длиннее title_len", Validate(m, Limits{TitleLen: 5}))
	// переполнение суммы: предел альбома — максимум int64, байты у страниц — максимум предела
	big := goodManifest(4, false)
	for i := range big.Pages {
		if !big.Pages[i].Blank {
			big.Pages[i].Bytes = math.MaxInt64 / 2
		}
	}
	wantInvalid(t, "сумма не переполняет", Validate(big, Limits{PageBytes: math.MaxInt64, AlbumBytes: math.MaxInt64}))
}

// TestValidateStrict — правила разбора С7: что сервис не должен принять от клиента с токеном.
func TestValidateStrict(t *testing.T) {
	// file и неизвестные поля: после разбора JSON поле file остаётся и отвергается, чужие — теряются
	var m Manifest
	raw := `{"schema":1,"generator":"g","title":"T","evil":"x","book":{"cover":false,"page_mm":{"w":100,"h":100},"x":1},
	  "pages":[{"n":1,"role":"page","type":"image/jpeg","sha256":"` + testSHA('a') + `","bytes":5,"w":10,"h":10,"file":"../../x"},
	  {"n":2,"role":"page","blank":true}]}`
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatal(err)
	}
	wantInvalid(t, "file из JSON", Validate(&m, Limits{}))
	m.Pages[0].File = ""
	if err := Validate(&m, Limits{}); err != nil {
		t.Fatalf("без file: %v", err)
	}
	if out, _ := json.Marshal(&m); strings.Contains(string(out), "evil") || strings.Contains(string(out), `"x":1`) {
		t.Errorf("повторная сериализация несёт чужие поля: %s", out)
	}
	// пароль и срок: 4…64 знака, ≤ 256 байт
	now := time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)
	ok := func(pw string) error {
		return ValidateCreate(&CreateRequest{Manifest: *goodManifest(4, false), ExpiresAt: now.Add(time.Hour), Password: pw}, Limits{}, now)
	}
	for _, pw := range []string{"", "abcd", "лето-4827", strings.Repeat("я", 64), "a b c d"} {
		if err := ok(pw); err != nil {
			t.Errorf("пароль %q: %v", pw, err)
		}
	}
	for name, pw := range map[string]string{
		"3 знака": "abc", "65 знаков": strings.Repeat("a", 65), "мегабайт": strings.Repeat("a", 1<<20),
		"65 знаков, 260 байт": strings.Repeat("😀", 65), "с \\n": "abc\ndef", "с NUL": "abc\x00def",
		"bidi": "abc\u202edef", "не UTF-8": "abc\xffdef", "разделитель": "abc\u2028def",
	} {
		wantInvalid(t, "пароль "+name, ok(pw))
	}
	// срок: в будущем, не нулевой
	for name, exp := range map[string]time.Time{"нулевой": {}, "сейчас": now, "прошлое": now.Add(-time.Second)} {
		wantInvalid(t, "срок "+name, ValidateCreate(&CreateRequest{Manifest: *goodManifest(4, false), ExpiresAt: exp}, Limits{}, now))
	}
	wantInvalid(t, "nil", ValidateCreate(nil, Limits{}, now))
	// манифест внутри запроса тоже проверяется
	bm := goodManifest(4, false)
	bm.Pages[0].File = "x"
	wantInvalid(t, "file в запросе", ValidateCreate(&CreateRequest{Manifest: *bm, ExpiresAt: now.Add(time.Hour)}, Limits{}, now))
}

func TestValidatePatch(t *testing.T) {
	now := time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)
	str := func(s string) *string { return &s }
	tm := func(t time.Time) *time.Time { return &t }
	for name, p := range map[string]*Patch{
		"пустая": {}, "снять пароль": {Password: str("")}, "пароль": {Password: str("abcd")},
		"срок": {ExpiresAt: tm(now.Add(time.Hour))}, "сброс": {RevokeSessions: true},
		"всё сразу": {ExpiresAt: tm(now.Add(time.Hour)), Password: str("лето-4827"), RevokeSessions: true},
	} {
		if err := ValidatePatch(p, now); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	for name, p := range map[string]*Patch{
		"короткий": {Password: str("abc")}, "длинный": {Password: str(strings.Repeat("a", 65))},
		"прошлый срок": {ExpiresAt: tm(now.Add(-time.Hour))}, "нулевой срок": {ExpiresAt: tm(time.Time{})},
		"nil": nil, "управляющий": {Password: str("ab\tcd")},
	} {
		wantInvalid(t, name, ValidatePatch(p, now))
	}
}

func TestValidateMissing(t *testing.T) {
	m := goodManifest(6, false) // страница 2 белая
	for _, ok := range [][]int{nil, {}, {1}, {1, 3, 6}, {6, 1}} {
		if err := ValidateMissing(m, ok); err != nil {
			t.Errorf("%v: %v", ok, err)
		}
	}
	for _, bad := range [][]int{{0}, {7}, {-1}, {2}, {1, 1}, {1, 3, 3}, {math.MaxInt}} {
		wantInvalid(t, "missing", ValidateMissing(m, bad))
	}
}

// TestManifestNoSecrets: пароль и срок в JSON манифеста не бывают по устройству типа.
func TestManifestNoSecrets(t *testing.T) {
	b, err := json.Marshal(goodManifest(4, true))
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"password", "expires_at", "token"} {
		if strings.Contains(string(b), key) {
			t.Errorf("в манифесте есть %q: %s", key, b)
		}
	}
	// круг: JSON → тип → JSON даёт то же
	var back Manifest
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	b2, _ := json.Marshal(&back)
	if string(b) != string(b2) {
		t.Errorf("круг JSON разошёлся:\n%s\n%s", b, b2)
	}
	// белая страница не пишет пустые поля
	if strings.Contains(string(b), `"type":""`) || strings.Contains(string(b), `"bytes":0`) {
		t.Errorf("пустые поля белой страницы в JSON: %s", b)
	}
}

func TestLimitsOrDefault(t *testing.T) {
	d := DefaultLimits()
	if got := (Limits{}).OrDefault(); got != d {
		t.Errorf("нули: %+v", got)
	}
	if got := (Limits{Pages: -1, PageBytes: -1, TitleLen: -5}).OrDefault(); got != d {
		t.Errorf("отрицательные: %+v", got)
	}
	if got := (Limits{TitleLen: 5000}).OrDefault().TitleLen; got != MaxTitle {
		t.Errorf("title_len сверх формата: %d", got)
	}
	if got := (Limits{Pages: 7, MaxDays: 30}).OrDefault(); got.Pages != 7 || got.MaxDays != 30 || got.PageBytes != d.PageBytes {
		t.Errorf("частичные: %+v", got)
	}
}
