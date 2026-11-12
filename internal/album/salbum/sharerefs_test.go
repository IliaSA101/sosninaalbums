package salbum

import (
	"encoding/json"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Привязки к выгруженным альбомам в файле (167, design §2): shareRefs.

// refsProject — проект с отпечатком и привязками к альбомам ids.
func refsProject(ids ...string) *model.Project {
	p := model.New()
	p.UID = model.NewUID()
	var refs []string
	for _, id := range ids {
		refs = append(refs, model.LinkRef(p.UID, id))
	}
	p.SetShareRefs(refs)
	return p
}

// withRefs — тело файла data с полем shareRefs, заменённым на raw ("" — поле убрано).
func withRefs(t testing.TB, data []byte, raw string) []byte {
	t.Helper()
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	if raw == "" {
		delete(m, "shareRefs")
	} else {
		m["shareRefs"] = json.RawMessage(raw)
	}
	out, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func decodeRefs(t testing.TB, data []byte) []string {
	t.Helper()
	q, err := Decode(data, testkit.TempDir(t))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	return q.ShareRefs()
}

func TestShareRefsRoundTrip(t *testing.T) {
	p := refsProject("a", "b", "c")
	want := p.ShareRefs()
	data, err := Encode(p, testkit.TempDir(t))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"shareRefs"`) {
		t.Fatal("в файле нет shareRefs")
	}
	if got := decodeRefs(t, data); !reflect.DeepEqual(got, want) {
		t.Fatalf("круг Encode→Decode: %v, ждали %v", got, want)
	}
	// Файл не несёт ни id альбома, ни адреса: только хэши.
	var doc any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	var walk func(v any)
	walk = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			for _, e := range x {
				walk(e)
			}
		case []any:
			for _, e := range x {
				walk(e)
			}
		case string:
			if x == "a" || x == "b" || x == "c" || strings.Contains(x, "http") {
				t.Errorf("в файле id или адрес: %q", x)
			}
		}
	}
	walk(doc)
	// Пустой список — поля нет (omitempty), файл прежний.
	p.SetShareRefs(nil)
	data, err = Encode(p, testkit.TempDir(t))
	if err != nil || strings.Contains(string(data), "shareRefs") {
		t.Fatalf("пустые привязки: поле в файле (%v)", err)
	}
	// Без годного отпечатка привязки ничьи: в файл не идут.
	q := refsProject("a")
	q.UID = ""
	if data, err = Encode(q, testkit.TempDir(t)); err != nil || strings.Contains(string(data), "shareRefs") {
		t.Fatalf("привязки без отпечатка легли в файл (%v)", err)
	}
}

func TestShareRefsOldFile(t *testing.T) {
	p := refsProject("a")
	data, err := Encode(p, testkit.TempDir(t))
	if err != nil {
		t.Fatal(err)
	}
	old := withRefs(t, data, "")
	q, err := Decode(old, testkit.TempDir(t))
	if err != nil || len(q.ShareRefs()) != 0 || q.UID != p.UID {
		t.Fatalf("файл без shareRefs: %v, привязки %v, uid %q", err, q.ShareRefs(), q.UID)
	}
	// Номер формата прежний: поле не меняет версию.
	var head struct{ Version int }
	if err := json.Unmarshal(data, &head); err != nil || head.Version != model.FormatVersion {
		t.Fatalf("версия %d, %v", head.Version, err)
	}
	// Привязки без отпечатка в файле — ничьи.
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	delete(m, "uid")
	noUID, _ := json.Marshal(m)
	if got := decodeRefs(t, noUID); len(got) != 0 {
		t.Errorf("привязки без отпечатка прочитаны: %v", got)
	}
}

func TestShareRefsBad(t *testing.T) {
	p := refsProject("a")
	base, err := Encode(p, testkit.TempDir(t))
	if err != nil {
		t.Fatal(err)
	}
	good := model.LinkRef(p.UID, "a")
	other := model.LinkRef(p.UID, "b")
	q := func(s string) string { return `"` + s + `"` }
	list := func(ss ...string) string { return "[" + strings.Join(ss, ",") + "]" }
	cases := []struct {
		name, raw string
		want      []string
	}{
		{"годная", list(q(good)), []string{good}},
		{"короткая", list(q(good[:31])), nil},
		{"длинная", list(q(good + "0")), nil},
		{"верхний регистр", list(q(strings.ToUpper(good))), nil},
		{"не hex", list(q(strings.Repeat("z", 32))), nil},
		{"не строки", list("1", "null", "true", "{}", "[]", q(good)), []string{good}},
		{"дубли", list(q(good), q(good), q(other), q(good)), []string{good, other}},
		{"пять годных — четыре", list(q(good), q(other), q(model.LinkRef(p.UID, "c")), q(model.LinkRef(p.UID, "d")), q(model.LinkRef(p.UID, "e"))),
			[]string{good, other, model.LinkRef(p.UID, "c"), model.LinkRef(p.UID, "d")}},
		{"не массив: строка", q(good), nil},
		{"не массив: объект", `{"a":1}`, nil},
		{"не массив: число", `7`, nil},
		{"null", `null`, nil},
		{"пустой", `[]`, nil},
		{"вложенный мусор", `[[["x"]],{"a":[1,2]},"` + good + `"]`, []string{good}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := decodeRefs(t, withRefs(t, base, c.raw))
			if len(got) == 0 && len(c.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("получили %v, ждали %v", got, c.want)
			}
		})
	}
	// Ровно 64 элемента разбираются, 65 — поле отброшено целиком (даже с годным началом).
	items := func(n int) string {
		ss := make([]string, n)
		for i := range ss {
			ss[i] = q(good)
		}
		return list(ss...)
	}
	if got := decodeRefs(t, withRefs(t, base, items(model.RawShareRefs))); !reflect.DeepEqual(got, []string{good}) {
		t.Errorf("массив на пределе: %v", got)
	}
	if got := decodeRefs(t, withRefs(t, base, items(model.RawShareRefs+1))); len(got) != 0 {
		t.Errorf("массив сверх предела не отброшен: %v", got)
	}
}

// Огромный массив (сотни тысяч записей) не держит ни время, ни память: разбор останавливается на
// 65-м элементе. Огромный одиночный элемент тоже пропускается без разбора.
func TestLinksBad(t *testing.T) {
	p := refsProject("a")
	base, err := Encode(p, testkit.TempDir(t))
	if err != nil {
		t.Fatal(err)
	}
	dir := testkit.TempDir(t)
	alloc := func(data []byte) (uint64, time.Duration) {
		runtime.GC()
		var m0, m1 runtime.MemStats
		runtime.ReadMemStats(&m0)
		start := time.Now()
		q, err := Decode(data, dir)
		d := time.Since(start)
		runtime.ReadMemStats(&m1)
		if err != nil {
			t.Fatalf("Decode: %v", err)
		}
		if len(q.ShareRefs()) != 0 {
			t.Fatalf("привязки из мусора: %v", q.ShareRefs())
		}
		return m1.TotalAlloc - m0.TotalAlloc, d
	}
	base0, _ := alloc(withRefs(t, base, ""))

	n := 300_000
	if testkit.Lite() {
		n = 60_000
	}
	elem := `"` + strings.Repeat("a", 32) + `",`
	huge := withRefs(t, base, "["+strings.Repeat(elem, n)+`"x"]`)
	a, d := alloc(huge)
	if d > testkit.Limit(time.Second) {
		t.Errorf("массив из %d записей разбирался %v", n, d)
	}
	// Файл уже в памяти теста; сверх базового разбора Decode не должен брать больше пары МБ.
	if a > base0+2<<20 {
		t.Errorf("массив из %d записей: %d КБ при базе %d КБ", n, a>>10, base0>>10)
	}

	// Один элемент-строка в мегабайты: не привязка, не разбирается; результат пуст.
	big := withRefs(t, base, `["`+strings.Repeat("я", 1<<20)+`"]`)
	if _, d := alloc(big); d > testkit.Limit(time.Second) {
		t.Errorf("огромный элемент разбирался %v", d)
	}
	// Управляющие и двунаправленные знаки, userinfo, схемы и фрагменты — в привязку не годятся
	// (адресов в файле нет вовсе): отброшены как не-hex.
	for _, s := range []string{
		"https://user:pass@host/x", "https://host/x#frag", "file:///etc/passwd", "javascript:alert(1)",
		"\u202e" + strings.Repeat("a", 31), strings.Repeat("a", 31) + "\u2066", "\x00" + strings.Repeat("a", 31),
	} {
		raw, _ := json.Marshal([]string{s})
		if got := decodeRefs(t, withRefs(t, base, string(raw))); len(got) != 0 {
			t.Errorf("негодная строка %q принята: %v", s, got)
		}
	}
}

// Усечённое и битое поле не роняет разбор: ошибка JSON идёт от внешнего Unmarshal, а не паникой.
func TestShareRefsTruncated(t *testing.T) {
	p := refsProject("a")
	base, _ := Encode(p, testkit.TempDir(t))
	for _, raw := range []string{`[`, `["`, `["a`, `[1,`, `]`, `[,]`} {
		data := withRefsRaw(base, raw)
		if _, err := Decode(data, testkit.TempDir(t)); err == nil {
			t.Errorf("усечённое %q принято без ошибки JSON", raw)
		}
	}
	// Прямой вызов UnmarshalJSON с усечённым куском — без паники, привязок нет.
	for _, raw := range []string{``, `[`, `["`, `["a`, `[1,`, `]`, `null`, `[[[[[[`} {
		var l refList
		if err := l.UnmarshalJSON([]byte(raw)); err != nil || len(l) != 0 {
			t.Errorf("UnmarshalJSON(%q): %v %v", raw, l, err)
		}
	}
}

// withRefsRaw — текстовая подстановка поля в конец файла: даёт заведомо невалидный JSON.
func withRefsRaw(data []byte, raw string) []byte {
	s := strings.TrimRight(string(data), " \n\t}")
	return []byte(s + `,"shareRefs":` + raw + "}")
}

func FuzzShareRefs(f *testing.F) {
	good := strings.Repeat("a", 32)
	for _, s := range []string{`["` + good + `"]`, `[`, `null`, `[1,"x",["y"]]`, `{"a":1}`, `["` + good + `","` + good + `"]`} {
		f.Add([]byte(s))
	}
	f.Fuzz(func(t *testing.T, b []byte) {
		var l refList
		_ = l.UnmarshalJSON(b)
		if len(l) > model.MaxShareRefs {
			t.Fatalf("больше предела: %d", len(l))
		}
		for _, r := range l {
			if !model.GoodLinkRef(r) {
				t.Fatalf("негодная привязка %q", r)
			}
		}
	})
}
