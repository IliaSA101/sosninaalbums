package update

import (
	"bytes"
	"crypto/ed25519"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/decor/fetch"
)

// Версии: x.y.z по 1…4 цифры; всё остальное — не версия.
func TestParseVersion(t *testing.T) {
	good := map[string]Version{"0.0.0": {}, "0.25.1": {0, 25, 1}, "1.2.3": {1, 2, 3}, "9999.9999.9999": {9999, 9999, 9999}, "0.025.1": {0, 25, 1}}
	for s, want := range good {
		if got, ok := ParseVersion(s); !ok || got != want {
			t.Errorf("%q: %v %v", s, got, ok)
		}
	}
	for _, s := range []string{"", "dev", "0.25", "0.25.1.2", "0..1", ".1.2", "1.2.", "1.2.3-rc1", "v1.2.3", " 1.2.3", "1.2.3 ", "10000.0.0", "-1.0.0", "1.2.+3", "١.٢.٣", "1,2,3"} {
		if got, ok := ParseVersion(s); ok {
			t.Errorf("%q принята как %v", s, got)
		}
	}
	if (Version{0, 25, 1}).String() != "0.25.1" {
		t.Error("String")
	}
}

// Сравнение числом, а не строкой; не версия — никогда «новее»; равные — не новее.
func TestNewer(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"0.10.0", "0.9.0", true}, {"0.9.0", "0.10.0", false}, {"0.25.1", "0.25.0", true}, {"0.25.0", "0.25.0", false},
		{"1.0.0", "0.99.99", true}, {"0.25.10", "0.25.9", true}, {"0.24.9", "0.25.0", false},
		{"dev", "0.1.0", false}, {"0.1.0", "dev", false}, {"", "", false}, {"0.25.1-rc1", "0.25.0", false}, {"0.25.1", "", false},
	}
	for _, c := range cases {
		if got := Newer(c.a, c.b); got != c.want {
			t.Errorf("Newer(%q, %q) = %v", c.a, c.b, got)
		}
	}
	if (Version{0, 9, 0}).Compare(Version{0, 10, 0}) != -1 || (Version{1}).Compare(Version{1}) != 0 || (Version{0, 0, 2}).Compare(Version{0, 0, 1}) != 1 {
		t.Error("Compare")
	}
}

// Имя установщика строится только из версии.
func TestSetupName(t *testing.T) {
	if got := SetupName("0.25.1"); got != "SosninaAlbums-0.25.1-windows-x64-setup.exe" {
		t.Errorf("имя %q", got)
	}
	for _, v := range []string{"", "dev", "../1.0.0", "1.0.0/../x", "1.0", `1.0.0"`, "1.0.0-rc1"} {
		if got := SetupName(v); got != "" {
			t.Errorf("%q → %q", v, got)
		}
	}
}

func TestParseManifest(t *testing.T) {
	_, f := setup("0.25.1", MinSetup)
	ok, err := Parse(manifestBody(t, "0.25.1", &f, map[string]any{"minAuto": "0.25.0", "extra": 1}))
	if err != nil || ok.Version != "0.25.1" || ok.MinAuto != "0.25.0" || ok.Setup == nil || *ok.Setup != f || len(ok.Notes) != 2 || ok.Date != "2026-11-02" {
		t.Fatalf("верный манифест: %v %+v", err, ok)
	}
	// Без записи Windows — манифест годен, установщика нет (только страница).
	if m, err := Parse(manifestBody(t, "0.25.1", nil, nil)); err != nil || m.Setup != nil {
		t.Errorf("без Windows: %v %+v", err, m)
	}
	bad := func(name string, f File, rest map[string]any, want error) {
		t.Helper()
		if _, err := Parse(manifestBody(t, "0.25.1", &f, rest)); !errors.Is(err, want) {
			t.Errorf("%s: %v", name, err)
		}
	}
	bad("формат новее", f, map[string]any{"format": 2}, ErrNewerFormat)
	bad("формат 0", f, map[string]any{"format": 0}, ErrManifest)
	bad("чужое приложение", f, map[string]any{"app": "other"}, ErrManifest)
	bad("версия не x.y.z", f, map[string]any{"version": "0.25"}, ErrManifest)
	bad("версия с суффиксом", f, map[string]any{"version": "0.25.1-rc1"}, ErrManifest)
	bad("minAuto не версия", f, map[string]any{"minAuto": "скоро"}, ErrManifest)
	g := f
	g.Name = "SosninaAlbums-0.25.0-windows-x64-setup.exe"
	bad("имя от другой версии", g, nil, ErrManifest)
	g.Name = "../" + f.Name
	bad("имя с путём", g, nil, ErrManifest)
	for _, size := range []int64{0, -1, MinSetup - 1, MaxSetup + 1} {
		g = f
		g.Size = size
		bad("размер", g, nil, ErrManifest)
	}
	for _, size := range []int64{MinSetup, MaxSetup} { // края годны
		g = f
		g.Size = size
		if _, err := Parse(manifestBody(t, "0.25.1", &g, nil)); err != nil {
			t.Errorf("размер %d на краю: %v", size, err)
		}
	}
	for _, sum := range []string{"", strings.Repeat("a", 63), strings.Repeat("A", 64), strings.Repeat("g", 64)} {
		g = f
		g.SHA256 = sum
		bad("sha256 "+sum, g, nil, ErrManifest)
	}
	for _, body := range []string{"", "null", "[]", "{", `{"format":"1"}`} {
		if _, err := Parse([]byte(body)); !errors.Is(err, ErrManifest) {
			t.Errorf("%q: %v", body, err)
		}
	}
	if _, err := Parse([]byte(`{"format":1,"app":"sosninaalbums","version":"1.0.0","pad":"` + strings.Repeat("x", maxBody) + `"}`)); !errors.Is(err, ErrManifest) {
		t.Errorf("тело больше предела: %v", err)
	}
}

// «Что нового» — чужой текст: ≤ 8 строк, ≤ 300 знаков, без управляющих знаков, пустые отброшены.
func TestParseNotes(t *testing.T) {
	notes := []string{"  Первая\n\tстрока  ", "", "\x00\x07", "с" + string(rune(0x200b)) + "невидимым", strings.Repeat("я", 400)}
	for i := range 8 {
		notes = append(notes, "ещё "+string(rune('a'+i)))
	}
	m, err := Parse(manifestBody(t, "0.25.1", nil, map[string]any{"notes": notes}))
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Notes) != 8 || m.Notes[0] != "Первая строка" || m.Notes[1] != "сневидимым" || len([]rune(m.Notes[2])) != 300 || m.Notes[3] != "ещё a" {
		t.Errorf("notes: %q", m.Notes)
	}
	if m, err := Parse(manifestBody(t, "0.25.1", nil, map[string]any{"notes": nil})); err != nil || m.Notes == nil || len(m.Notes) != 0 {
		t.Errorf("без notes: %v %#v", err, m.Notes)
	}
}

// Общая фикстура testdata/ (её выпускает release-sign.mjs, сверяет release-sign.test.mjs): конверт,
// подписанный тестовым ключом из update.pub, открывается и разбирается так, как задумано в
// design §5.1; подмена body и чужой ключ — отказ ErrVerify; ключ выпуска фикстуру не открывает.
func TestManifestFixture(t *testing.T) {
	keys := fixtureKeys(t)
	body, err := fetch.Open(readFixture(t, "update.json"), keys)
	if err != nil {
		t.Fatal(err)
	}
	if want := readFixture(t, "body.json"); !bytes.Equal(bytes.TrimSpace(body), bytes.TrimSpace(want)) {
		t.Error("body из конверта не равен body.json")
	}
	m, err := Parse(body)
	if err != nil || m.Version != "0.25.1" || m.MinAuto != "0.25.0" || m.Date != "2026-11-02" || len(m.Notes) != 3 ||
		m.Setup == nil || m.Setup.Name != SetupName("0.25.1") || m.Setup.Size != 8166433 || len(m.Setup.SHA256) != 64 {
		t.Fatalf("фикстура: %v %+v", err, m)
	}
	for _, name := range []string{"update-tampered.json", "update-otherkey.json"} {
		_, err := fetch.Open(readFixture(t, name), keys)
		if !errors.Is(humanize(err), ErrVerify) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if _, err := fetch.Open(readFixture(t, "update.json"), fetch.PublicKeys(Keys[:])); err == nil {
		t.Error("фикстура открылась ключом выпуска")
	}
}

// Тестовая пара из helpers_test.go совпадает с update.pub: иначе фикстура и тесты разойдутся.
func TestFixtureKeyMatches(t *testing.T) {
	pub, _ := testPair()
	if keys := fixtureKeys(t); !keys[0].Equal(pub) {
		t.Error("update.pub не из testPair()")
	}
}

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func fixtureKeys(t *testing.T) []ed25519.PublicKey {
	t.Helper()
	keys := fetch.PublicKeys([]string{strings.TrimSpace(string(readFixture(t, "update.pub")))})
	if len(keys) != 1 {
		t.Fatal("update.pub: не ключ ed25519")
	}
	return keys
}
