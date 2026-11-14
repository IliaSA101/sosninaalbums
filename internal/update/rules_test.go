package update

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
)

// ver — тело выпуска 0.60.0 с правилами minOS и lastOn (raw — готовый JSON поля, "" — поля нет).
func ver(minOS, lastOn string) []byte {
	b := `{"format":1,"app":"sosninaalbums","version":"0.60.0"`
	if minOS != "" {
		b += `,"minOS":` + minOS
	}
	if lastOn != "" {
		b += `,"lastOn":` + lastOn
	}
	return []byte(b + `}`)
}

func TestParseOS(t *testing.T) {
	good := map[string][3]int{"13": {13}, "13.0": {13}, "12.6.1": {12, 6, 1}, "10.0.22631": {10, 0, 22631}, "0": {}, "999999.999999.999999": {999999, 999999, 999999}, "013.00": {13}}
	for s, want := range good {
		if got, ok := ParseOS(s); !ok || got != want {
			t.Errorf("%q: %v %v", s, got, ok)
		}
	}
	for _, s := range []string{"", ".", "13.", ".13", "1..2", "1.2.3.4", "1000000", "-1", "+1", "1.2.x", " 13", "13 ", "13.0\n", "v13", "١٣", "1,2", "13.0-beta", "1e3"} {
		if got, ok := ParseOS(s); ok {
			t.Errorf("%q принята как %v", s, got)
		}
	}
}

func TestOSLess(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"12.4", "13.0", true}, {"13.0", "13.0", false}, {"13.1", "13.0", false}, {"13", "13.0.0", false}, {"13", "13.0.1", true},
		{"9.9", "10", true}, {"10.0.19045", "10.0.22000", true}, {"10.0.22000", "10.0.22000", false},
		{"", "13.0", false}, {"13.0", "", false}, {"abc", "13.0", false}, {"12", "x", false},
	}
	for _, c := range cases {
		if got := OSLess(c.a, c.b); got != c.want {
			t.Errorf("OSLess(%q, %q) = %v", c.a, c.b, got)
		}
	}
}

func TestStepFor(t *testing.T) {
	ladder := map[string]string{"14.0": "0.47.0", "13.0": "0.42.0"}
	cases := []struct{ sys, want string }{
		{"12.4", "0.42.0"}, {"12.99.9", "0.42.0"}, {"13.0", "0.47.0"}, {"13.5", "0.47.0"}, {"14.0", ""}, {"15", ""}, {"", ""}, {"мусор", ""},
	}
	for _, c := range cases {
		if got := stepFor(ladder, c.sys); got != c.want {
			t.Errorf("stepFor(%q) = %q, ждали %q", c.sys, got, c.want)
		}
	}
	if stepFor(nil, "12.0") != "" || stepFor(map[string]string{}, "12.0") != "" {
		t.Error("пустая лестница")
	}
	// Равные пороги в разной записи (разбор их не пропускает) — результат не зависит от порядка карты.
	for range 20 {
		if got := stepFor(map[string]string{"13": "0.43.0", "13.0": "0.42.0", "13.0.0": "0.44.0"}, "12.0"); got != "0.42.0" {
			t.Fatalf("равные пороги: %q", got)
		}
	}
}

// Поле minOS: верные значения, неизвестные ключи, края формы; ошибка в ключе своей ОС — отказ,
// другой — пропуск с именем в Skipped (171-13).
func TestParseMinOS(t *testing.T) {
	m, err := Parse(ver(`{"darwin":"13.0","windows":"10.0.22000","linux":"6","bsd":"x","Darwin":"???"}`, ""), "")
	if err != nil || !reflect.DeepEqual(m.MinOS, map[string]string{"darwin": "13.0", "windows": "10.0.22000", "linux": "6"}) || len(m.Skipped) != 0 {
		t.Fatalf("верные: %v %+v", err, m)
	}
	// Нет поля, null и {} — правил нет, ошибки нет.
	for _, f := range []string{"", "null", "{}", " null "} {
		m, err := Parse(ver(f, ""), "darwin")
		if err != nil || len(m.MinOS) != 0 || len(m.Skipped) != 0 {
			t.Errorf("minOS %q: %v %+v", f, err, m)
		}
	}
	// Само поле не объект — отказ всем, даже ОС, которой оно не касается.
	for _, f := range []string{`"13.0"`, `13`, `["13.0"]`, `true`} {
		for _, goos := range []string{"", "darwin", "windows"} {
			if _, err := Parse(ver(f, ""), goos); !errors.Is(err, ErrManifest) {
				t.Errorf("minOS %s для %q: %v", f, goos, err)
			}
		}
	}
	// Неверное значение: пустая строка, не строка, форма, число частей и цифр, пробелы.
	for _, v := range []string{`""`, `null`, `13`, `{"a":1}`, `[]`, `"13.0.0.0"`, `"1234567"`, `"v13"`, `" 13"`, `"13."`, `"13,0"`, `"-1"`, `"13.0-beta"`} {
		body := ver(`{"darwin":`+v+`,"windows":"10.0"}`, "")
		if _, err := Parse(body, "darwin"); !errors.Is(err, ErrManifest) {
			t.Errorf("darwin %s на Mac: %v", v, err)
		}
		if _, err := Parse(body, ""); !errors.Is(err, ErrManifest) {
			t.Errorf("darwin %s без ОС: %v", v, err)
		}
		// На Windows неверный darwin не мешает: ключ пропущен, имя записано, Windows разобран.
		m, err := Parse(body, "windows")
		if err != nil || !reflect.DeepEqual(m.MinOS, map[string]string{"windows": "10.0"}) || !reflect.DeepEqual(m.Skipped, []string{"minOS.darwin"}) {
			t.Errorf("darwin %s на Windows: %v %+v", v, err, m)
		}
	}
	// ОС, которой в ключах нет (linux), смотрит на неверное поле darwin так же — пропуск.
	if m, err := Parse(ver(`{"darwin":"x","windows":"y"}`, ""), "linux"); err != nil || len(m.MinOS) != 0 || !reflect.DeepEqual(m.Skipped, []string{"minOS.darwin", "minOS.windows"}) {
		t.Errorf("два неверных на linux: %v %+v", err, m)
	}
}

// Неизвестные ключи и предел известных.
func TestParseMinOSKeys(t *testing.T) {
	// Регистр ключа строгий: «Darwin» — неизвестный ключ, значение не разбирается.
	if m, err := Parse(ver(`{"Darwin":"мусор","DARWIN":1}`, ""), ""); err != nil || len(m.MinOS) != 0 {
		t.Errorf("регистр: %v %+v", err, m)
	}
	// Неизвестных ключей сколько угодно (память — предел тела): не считаются, значения любые.
	var b strings.Builder
	for i := range 20 {
		fmt.Fprintf(&b, `"k%d":{"x":[%d]},`, i, i)
	}
	m, err := Parse(ver(`{`+b.String()+`"darwin":"13.0"}`, ""), "darwin")
	if err != nil || m.MinOS["darwin"] != "13.0" {
		t.Errorf("20 неизвестных + 1 известный: %v %+v", err, m)
	}
	// Предел известных ключей — 8; сейчас их три имени, поэтому предел проверяется на расширенном списке.
	many := []string{"a", "b", "c", "d", "e", "f", "g", "h", "i"}
	raw := func(n int) json.RawMessage {
		parts := make([]string, n)
		for i := range n {
			parts[i] = fmt.Sprintf(`"%s":"1.0"`, many[i])
		}
		return json.RawMessage("{" + strings.Join(parts, ",") + "}")
	}
	if r, err := parseRules(raw(8), nil, "0.60.0", "", many); err != nil || len(r.minOS) != 8 {
		t.Errorf("8 известных: %v %+v", err, r)
	}
	if _, err := parseRules(raw(9), nil, "0.60.0", "", many); !errors.Is(err, ErrManifest) {
		t.Errorf("9 известных: %v", err)
	}
}

// Лестница lastOn: верная, и каждое нарушение правил design §2.
func TestParseLastOnLadder(t *testing.T) {
	m, err := Parse(ver(`{"darwin":"14.0"}`, `{"darwin":{"14.0":"0.47.0","13.0":"0.42.0"}}`), "darwin")
	if err != nil || !reflect.DeepEqual(m.LastOn, map[string]map[string]string{"darwin": {"14.0": "0.47.0", "13.0": "0.42.0"}}) {
		t.Fatalf("две ступени: %v %+v", err, m)
	}
	// Края допустимого: ключ равен minOS, значение на шаг старше версии, равные версии на соседних ступенях,
	// пустая лестница, null и отсутствие поля.
	good := []string{`{"darwin":{"14.0":"0.59.9"}}`, `{"darwin":{"13.0":"0.42.0","14":"0.42.0"}}`, `{"darwin":{}}`, `null`, `{}`}
	for _, l := range good {
		if _, err := Parse(ver(`{"darwin":"14.0"}`, l), "darwin"); err != nil {
			t.Errorf("lastOn %s: %v", l, err)
		}
	}
	bad := map[string]struct{ minOS, last string }{
		"значение не версия":     {`{"darwin":"14.0"}`, `{"darwin":{"13.0":"0.42"}}`},
		"значение с суффиксом":   {`{"darwin":"14.0"}`, `{"darwin":{"13.0":"0.42.0-rc1"}}`},
		"значение не строка":     {`{"darwin":"14.0"}`, `{"darwin":{"13.0":42}}`},
		"ступень равна version":  {`{"darwin":"14.0"}`, `{"darwin":{"13.0":"0.60.0"}}`},
		"ступень новее version":  {`{"darwin":"14.0"}`, `{"darwin":{"13.0":"0.61.0"}}`},
		"ключ выше minOS":        {`{"darwin":"14.0"}`, `{"darwin":{"14.1":"0.42.0"}}`},
		"ключ не версия системы": {`{"darwin":"14.0"}`, `{"darwin":{"тринадцать":"0.42.0"}}`},
		"немонотонная":           {`{"darwin":"14.0"}`, `{"darwin":{"13.0":"0.47.0","14.0":"0.42.0"}}`},
		"повтор порога":          {`{"darwin":"14.0"}`, `{"darwin":{"13":"0.42.0","13.0":"0.42.0"}}`},
		"lastOn без minOS":       {``, `{"darwin":{"13.0":"0.42.0"}}`},
		"lastOn без minOS своей": {`{"windows":"10.0"}`, `{"darwin":{"13.0":"0.42.0"}}`},
		"лестница не объект":     {`{"darwin":"14.0"}`, `{"darwin":"0.42.0"}`},
		"лестница null":          {`{"darwin":"14.0"}`, `{"darwin":null}`},
		"лестница список":        {`{"darwin":"14.0"}`, `{"darwin":["0.42.0"]}`},
		"версия вместо лестницы": {`{"darwin":"14.0"}`, `{"darwin":"0.42.0"}`},
		"minOS неверен":          {`{"darwin":"x"}`, `{"darwin":{"13.0":"0.42.0"}}`},
		"9 ступеней":             {`{"darwin":"30.0"}`, ladderOf(9)},
		"lastOn не объект":       {`{"darwin":"14.0"}`, `["darwin"]`},
		"lastOn число":           {`{"darwin":"14.0"}`, `7`},
		"lastOn строка":          {`{"darwin":"14.0"}`, `"0.42.0"`},
	}
	for name, c := range bad {
		if _, err := Parse(ver(c.minOS, c.last), "darwin"); !errors.Is(err, ErrManifest) {
			t.Errorf("%s на Mac: %v", name, err)
		}
		if _, err := Parse(ver(c.minOS, c.last), ""); !errors.Is(err, ErrManifest) {
			t.Errorf("%s без ОС: %v", name, err)
		}
	}
	// 8 ступеней — предел, годится.
	if m, err := Parse(ver(`{"darwin":"30.0"}`, ladderOf(8)), "darwin"); err != nil || len(m.LastOn["darwin"]) != 8 {
		t.Errorf("8 ступеней: %v", err)
	}
	// Ошибка лестницы другой ОС не мешает: пропуск и имя; общий случай «lastOn не объект» — отказ всем.
	m, err = Parse(ver(`{"darwin":"14.0","windows":"10.0"}`, `{"darwin":{"13.0":"0.99.0"}}`), "windows")
	if err != nil || len(m.LastOn) != 0 || !reflect.DeepEqual(m.Skipped, []string{"lastOn.darwin"}) || m.MinOS["windows"] != "10.0" {
		t.Errorf("лестница другой ОС: %v %+v", err, m)
	}
	if _, err := Parse(ver(`{"darwin":"14.0"}`, `["x"]`), "windows"); !errors.Is(err, ErrManifest) {
		t.Errorf("lastOn не объект на Windows: %v", err)
	}
	// minOS чужой ОС неверен — её лестница тоже пропущена (без minOS лестницы нет).
	m, err = Parse(ver(`{"darwin":"x"}`, `{"darwin":{"13.0":"0.42.0"}}`), "windows")
	if err != nil || !reflect.DeepEqual(m.Skipped, []string{"minOS.darwin", "lastOn.darwin"}) {
		t.Errorf("чужой minOS неверен: %v %+v", err, m)
	}
}

// ladderOf — лестница из n ступеней 10.0, 11.0, … → 0.40.0, 0.41.0, …
func ladderOf(n int) string {
	parts := make([]string, n)
	for i := range n {
		parts[i] = fmt.Sprintf(`"%d.0":"0.%d.0"`, 10+i, 40+i)
	}
	return `{"darwin":{` + strings.Join(parts, ",") + `}}`
}

// Старое тело без новых полей (testdata/body-old.json): правил нет, всё прочее как раньше.
func TestParseOldBody(t *testing.T) {
	for _, goos := range []string{"", "darwin", "windows", "linux"} {
		m, err := Parse(readFixture(t, "body-old.json"), goos)
		if err != nil || m.MinOS != nil || m.LastOn != nil || m.Skipped != nil || m.Version != "0.25.1" {
			t.Errorf("%q: %v %+v", goos, err, m)
		}
	}
}

// Тело выпуска 0.40.0 (testdata/body.json): minOS.darwin 12.0 без lastOn, для своей ОС — правило, для чужой — нет.
func TestParseBodyMinOS(t *testing.T) {
	body := readFixture(t, "body.json")
	for _, goos := range []string{"", "darwin", "windows", "linux"} {
		m, err := Parse(body, goos)
		if err != nil || m.Version != "0.25.1" || m.LastOn != nil || len(m.Skipped) != 0 {
			t.Fatalf("%q: %v %+v", goos, err, m)
		}
		if want := (map[string]string{"darwin": "12.0"}); !reflect.DeepEqual(m.MinOS, want) {
			t.Errorf("%q: minOS %v, ждали %v", goos, m.MinOS, want)
		}
	}
}

// «Манифест будущего» (testdata/future-body.json): лишние поля первыми, ключи linux-glibc и
// freebsd с вложенными объектами, 8 ступеней — 0.40 разбирает его, лишнее пропускает.
func TestParseFutureBody(t *testing.T) {
	body := readFixture(t, "future-body.json")
	for _, goos := range []string{"", "darwin", "windows", "linux"} {
		m, err := Parse(body, goos)
		if err != nil || m.Version != "0.60.0" || len(m.Skipped) != 0 {
			t.Fatalf("%q: %v %+v", goos, err, m)
		}
		if !reflect.DeepEqual(m.MinOS, map[string]string{"darwin": "20.0", "windows": "10.0.22000"}) || len(m.LastOn["darwin"]) != 8 {
			t.Errorf("%q: правила %+v", goos, m)
		}
	}
	// 8 ступеней для Mac 12.4: наименьший порог выше системы — 13.0 → 0.41.0.
	m, _ := Parse(body, "darwin")
	if got := stepFor(m.LastOn["darwin"], "12.4"); got != "0.41.0" {
		t.Errorf("ступень для 12.4: %q", got)
	}
}

// oldRaw — разбор тела программами 0.25–0.39 (manifest.go тех меток, побайтно общий): структура
// без DisallowUnknownFields. Тест держит совместимость вперёд: новые поля и любые будущие формы
// не должны ломать json.Unmarshal у уже выпущенных версий.
type oldRaw struct {
	Format  int             `json:"format"`
	App     string          `json:"app"`
	Version string          `json:"version"`
	Date    string          `json:"date"`
	MinAuto string          `json:"minAuto"`
	Notes   []string        `json:"notes"`
	Files   map[string]File `json:"files"`
}

func TestOldParseCompat(t *testing.T) {
	_, f := setup("0.60.0", MinSetup)
	bodies := map[string][]byte{
		"будущее":        readFixture(t, "future-body.json"),
		"minOS":          ver(`{"darwin":"13.0"}`, ""),
		"minOS и lastOn": ver(`{"darwin":"14.0"}`, `{"darwin":{"14.0":"0.47.0","13.0":"0.42.0"}}`),
		"lastOn первым":  []byte(`{"lastOn":{"darwin":{"13.0":"0.42.0"}},"minOS":{"darwin":"13.0"},"format":1,"app":"sosninaalbums","version":"0.43.0","notes":[]}`),
		"поля — числа и списки": ver(`[1,2]`, `7`), // сами старые версии разберут: тип поля им не важен
		"с установщиком":        manifestBody(t, "0.60.0", &f, map[string]any{"minOS": map[string]string{"darwin": "13.0"}}),
	}
	for name, b := range bodies {
		var raw oldRaw
		if err := json.Unmarshal(b, &raw); err != nil || raw.Format != 1 || raw.App != appID || raw.Version == "" {
			t.Errorf("%s: старый разбор: %v %+v", name, err, raw)
		}
	}
}

// Без паники и с инвариантами: при успехе все значения проходят свои проверки.
func FuzzParse(f *testing.F) {
	for _, s := range []string{"body.json", "body-old.json", "future-body.json"} {
		f.Add(readFixtureF(f, s), "")
		f.Add(readFixtureF(f, s), "darwin")
	}
	f.Add(ver(`{"darwin":"14.0"}`, `{"darwin":{"14.0":"0.47.0","13.0":"0.42.0"}}`), "darwin")
	f.Add(ver(`{"darwin":"x","windows":"10.0"}`, `{"darwin":{"13.0":"0.42.0"}}`), "windows")
	f.Add(ver(`null`, `{}`), "linux")
	f.Add(ver(`[]`, `"x"`), "")
	f.Add([]byte(`{"format":1,"app":"sosninaalbums","version":"1.0.0","minOS":{"darwin":{"darwin":"1"}},"lastOn":{"darwin":{"1":"0.1.0","1.0.0":"0.1.0"}}}`), "darwin")
	f.Fuzz(func(t *testing.T, body []byte, goos string) {
		if m, err := Parse(body, goos); err == nil {
			checkParsed(t, m, goos)
		}
	})
}

// checkParsed — инварианты разобранного манифеста: значения прошли свои проверки.
func checkParsed(t *testing.T, m Manifest, goos string) {
	t.Helper()
	for k, v := range m.MinOS {
		if !knownGOOS(k) || !validOS(v) {
			t.Fatalf("minOS %q=%q", k, v)
		}
	}
	for k, steps := range m.LastOn {
		if !knownGOOS(k) || m.MinOS[k] == "" || len(steps) > maxSteps {
			t.Fatalf("lastOn %q: %v", k, steps)
		}
		for key, val := range steps {
			if !validOS(key) || !Newer(m.Version, val) {
				t.Fatalf("ступень %q=%q при %s", key, val, m.Version)
			}
		}
	}
	for _, s := range m.Skipped {
		if goos != "" && strings.HasSuffix(s, "."+goos) {
			t.Fatalf("пропущено поле своей ОС: %s", s)
		}
	}
}

func readFixtureF(f *testing.F, name string) []byte {
	f.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		f.Fatal(err)
	}
	return data
}

// Система: подмена стенда только при enabled; мусор и «10.16» — версия неизвестна.
func TestSystemOf(t *testing.T) {
	osver := func(v string, ok bool) func() (string, bool) { return func() (string, bool) { return v, ok } }
	env := func(v string) func(string) string {
		return func(k string) string {
			if k != "SA_UPDATE_OS" {
				t.Errorf("прочитана переменная %q", k)
			}
			return v
		}
	}
	cases := []struct {
		name    string
		enabled bool
		goos    string
		osver   func() (string, bool)
		env     string
		want    System
	}{
		{"mac 12", false, "darwin", osver("12.4", true), "", System{GOOS: "darwin", Version: "12.4"}},
		{"windows", false, "windows", osver("10.0.22631", true), "", System{GOOS: "windows", Version: "10.0.22631"}},
		{"10.16 на Mac", false, "darwin", osver("10.16", true), "", System{GOOS: "darwin"}},
		{"10.16.0 на Mac", false, "darwin", osver("10.16.0", true), "", System{GOOS: "darwin"}},
		{"10.16 не на Mac", false, "windows", osver("10.16", true), "", System{GOOS: "windows", Version: "10.16"}},
		{"10.15 на Mac", false, "darwin", osver("10.15.7", true), "", System{GOOS: "darwin", Version: "10.15.7"}},
		{"мусор", false, "darwin", osver("Monterey", true), "", System{GOOS: "darwin"}},
		{"пусто", false, "darwin", osver("", true), "", System{GOOS: "darwin"}},
		{"сбой", false, "darwin", osver("12.4", false), "", System{GOOS: "darwin"}},
		{"нет функции", false, "linux", nil, "", System{GOOS: "linux"}},
		{"выпуск не читает стенд", false, "darwin", osver("13.1", true), "darwin:12.6", System{GOOS: "darwin", Version: "13.1"}},
		{"стенд подменяет всё", true, "linux", osver("6.1", true), "darwin:12.6", System{GOOS: "darwin", Version: "12.6"}},
		{"стенд windows", true, "darwin", osver("13.1", true), "windows:10.0.19045", System{GOOS: "windows", Version: "10.0.19045"}},
		{"стенд 10.16", true, "darwin", osver("13.1", true), "darwin:10.16", System{GOOS: "darwin"}},
		{"стенд без переменной", true, "darwin", osver("13.1", true), "", System{GOOS: "darwin", Version: "13.1"}},
	}
	for _, c := range cases {
		if got := systemFrom(c.enabled, c.goos, c.osver, env(c.env)); got != c.want {
			t.Errorf("%s: %+v, ждали %+v", c.name, got, c.want)
		}
	}
	// Неверное значение стенда — без подмены, StandBad для строки в log.txt.
	for _, v := range []string{"darwin", "darwin:", ":12", "darwin:x", "plan9:12", "Darwin:12", "darwin:12:1", " darwin:12"} {
		got := systemFrom(true, "darwin", osver("13.1", true), env(v))
		if want := (System{GOOS: "darwin", Version: "13.1", StandBad: true}); got != want {
			t.Errorf("%q: %+v, ждали %+v", v, got, want)
		}
	}
	// В выпуске значение не читается вовсе (env не вызывается; стенд-ошибка не возникает).
	if got := systemFrom(false, "darwin", osver("13.1", true), func(string) string { t.Error("env прочитан"); return "" }); got.StandBad {
		t.Error("StandBad в выпуске")
	}
	// SystemOf на этой сборке (без тега dev): GOOS процесса, версия из osver.
	if got := SystemOf(osver("13.1", true)); got.GOOS == "" || got.StandBad && !standEnabled {
		t.Errorf("SystemOf: %+v", got)
	}
}
