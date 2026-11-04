package printprof

import (
	"bytes"
	"slices"
	"strings"
	"testing"
)

// own — копия профиля base под свой id: изделия own-<id>-<n>, название name.
func own(t *testing.T, base, id, name string) Profile {
	t.Helper()
	p, err := Parse(builtinData(t, base))
	if err != nil {
		t.Fatal(err)
	}
	p.ID, p.Name, p.Gen = id, name, name
	m := map[string]string{}
	for i := range p.Products {
		old := p.Products[i].ID
		p.Products[i].ID = OwnProductID(id, i+1)
		m[old] = p.Products[i].ID
	}
	p.Default = m[p.Default]
	return p
}

// libOf — профиль встроенного base с версией v (как набор из каталога).
func libOf(t *testing.T, base string, v int) Profile {
	t.Helper()
	p, err := Parse(builtinData(t, base))
	if err != nil {
		t.Fatal(err)
	}
	p.Version = v
	return p
}

func ids(s *Snapshot) []string {
	var out []string
	for _, p := range s.Profiles() {
		out = append(out, p.ID+":"+s.Source(p.ID))
	}
	return out
}

// TestRegistryPrecedence — встроенная ← библиотека (строго старшая версия) ← свои; свои
// встроенную не перекрывают; из двух библиотечных побеждает старшая.
func TestRegistryPrecedence(t *testing.T) {
	b := builtinVersion(t, "fabrika")
	o := own(t, "fabrika", "own-0a1b2c3d", "Моя типография")

	snap, skipped := Compose(nil, nil)
	if len(skipped) != 0 || !slices.Equal(ids(snap), []string{"fsfera:builtin", "netprint:builtin", "fabrika:builtin"}) {
		t.Fatalf("без слоёв: %v %v", ids(snap), skipped)
	}
	// Набор новее — перекрывает на месте (порядок рядов прежний), равный и младший — нет.
	for _, c := range []struct {
		v    int
		want string
	}{{b + 1, "fabrika:library"}, {b, "fabrika:builtin"}, {b - 1, "fabrika:builtin"}} {
		if c.v < 1 {
			continue
		}
		snap, skipped = Compose([]Profile{libOf(t, "fabrika", c.v)}, nil)
		if !slices.Equal(ids(snap), []string{"fsfera:builtin", "netprint:builtin", c.want}) {
			t.Errorf("версия %d: %v", c.v, ids(snap))
		}
		if (c.want == "fabrika:builtin") != (len(skipped) == 1) {
			t.Errorf("версия %d: пропуски %v", c.v, skipped)
		}
		if got, _ := snap.ByID("fabrika"); got.Version != max(c.v, b) && c.want == "fabrika:library" {
			t.Errorf("версия профиля %d", got.Version)
		}
	}
	// Два набора с одним id — старшая версия, в любом порядке.
	for _, in := range [][]Profile{{libOf(t, "fabrika", b+1), libOf(t, "fabrika", b+2)}, {libOf(t, "fabrika", b+2), libOf(t, "fabrika", b+1)}} {
		snap, _ = Compose(in, nil)
		if got, _ := snap.ByID("fabrika"); got.Version != b+2 {
			t.Errorf("победила версия %d, ждали %d", got.Version, b+2)
		}
	}
	// Своя — после всех, главнее по выбору; встроенную своей не занять ни id, ни названием.
	snap, skipped = Compose(nil, []Profile{o})
	if len(skipped) != 0 || ids(snap)[3] != "own-0a1b2c3d:own" || snap.Source("own-0a1b2c3d") != SourceOwn {
		t.Fatalf("своя: %v %v", ids(snap), skipped)
	}
	fake := own(t, "fabrika", "own-0a1b2c3d", "Фабрика Фотокниги") // название встроенной
	bad := []struct {
		name string
		p    Profile
	}{
		{"название встроенной", fake},
		{"id встроенной", func() Profile { p := o; p.ID = "fabrika"; return p }()},
		{"id без own-", func() Profile { p := o; p.ID, p.Name = "mine-0a1b2c3d", "Другая"; return p }()},
		{"изделие встроенной", func() Profile {
			p := o
			p.Name = "Третья"
			p.Products = slices.Clone(p.Products)
			p.Products[0].ID = "fabrika-photo"
			return p
		}()},
	}
	for _, c := range bad {
		snap, skipped = Compose(nil, []Profile{c.p})
		if len(snap.Profiles()) != 3 || len(skipped) != 1 {
			t.Errorf("%s: принята (%v) %v", c.name, ids(snap), skipped)
		}
	}
	// Набор не занимает own-…, одинаковые названия — позже загруженный пропускается.
	lp := libOf(t, "fabrika", b+1)
	lp.ID = "own-0a1b2c3d"
	if snap, skipped = Compose([]Profile{lp}, nil); len(snap.Profiles()) != 3 || len(skipped) != 1 {
		t.Errorf("набор с own-: %v %v", ids(snap), skipped)
	}
	o2 := own(t, "fabrika", "own-ffffffff", "Моя типография")
	snap, skipped = Compose(nil, []Profile{o, o2})
	if len(snap.Profiles()) != 4 || len(skipped) != 1 || !strings.Contains(skipped[0], "own-ffffffff") {
		t.Errorf("одинаковое название: %v %v", ids(snap), skipped)
	}
	// Негодный профиль не роняет остальных.
	broken := o
	broken.Version = 0
	snap, skipped = Compose([]Profile{{}}, []Profile{broken, o2})
	if len(snap.Profiles()) != 4 || len(skipped) != 2 {
		t.Errorf("негодные: %v %v", ids(snap), skipped)
	}
}

func builtinVersion(t *testing.T, id string) int {
	t.Helper()
	p, ok := Builtin().ByID(id)
	if !ok {
		t.Fatal(id)
	}
	return p.Version
}

// TestRegistryRollback — убрали слой, и снимок снова встроенный: откат набора и «Удалить» своей.
func TestRegistryRollback(t *testing.T) {
	b := builtinVersion(t, "fabrika")
	o := own(t, "fsfera", "own-00000001", "Моя")
	with, _ := Compose([]Profile{libOf(t, "fabrika", b+1)}, []Profile{o})
	if len(with.Profiles()) != 4 {
		t.Fatal(ids(with))
	}
	UseForTest(t, with)
	if got := Current(); got != with {
		t.Fatal("Install не действует")
	}
	back, _ := Compose(nil, nil)
	Install(back)
	for _, p := range Current().Profiles() {
		if want, _ := Builtin().ByID(p.ID); want.Version != p.Version || Current().Source(p.ID) != SourceBuiltin {
			t.Errorf("%s не вернулась встроенной", p.ID)
		}
	}
	if _, ok := Current().ByID(o.ID); ok {
		t.Error("своя осталась после удаления")
	}
	Install(nil)
	if Current() != Builtin() {
		t.Error("Install(nil) не вернул встроенные")
	}
}

// TestOwnIDs — id своей: грамматика, свежий не занят, изделия по номеру.
func TestOwnIDs(t *testing.T) {
	taken := map[string]bool{}
	for range 50 {
		id := NewOwnID(func(s string) bool { return taken[s] })
		if !ValidOwnID(id) || !ValidID(id) || taken[id] {
			t.Fatalf("id %q", id)
		}
		taken[id] = true
	}
	for _, s := range []string{"", "own-", "own-0123abc", "own-0123abcde", "own-0123ABCD", "fabrika", "own-0123abcd-1"} {
		if ValidOwnID(s) {
			t.Errorf("%q принят", s)
		}
	}
	if OwnProductID("own-0123abcd", 3) != "own-0123abcd-3" {
		t.Error("id изделия")
	}
	if infos := Builtin().Infos(); len(infos) != 3 || infos[2].Source != SourceBuiltin || !infos[2].HasExport || infos[1].HasExport || len(infos[2].Formats) != 5 {
		t.Errorf("Infos: %+v", infos)
	}
}

// TestParseStrictB1 — ключи только в точном регистре, без повторов; сайт — ASCII-хост; изделие
// с id профиля; px и обрез в пределах; «custom» занят.
func TestParseStrictB1(t *testing.T) {
	good := builtinData(t, "fabrika")
	rename := func(from, to string) []byte { return bytes.Replace(good, []byte(from), []byte(to), 1) }
	for name, c := range map[string]struct {
		why  string
		data []byte
	}{
		"ключ в другом регистре":     {"неизвестное поле", rename(`"version"`, `"Version"`)},
		"ключ вложенного в регистре": {"неизвестное поле", rename(`"dpi"`, `"DPI"`)},
		"повтор ключа":               {"повторён", rename(`"version": 2,`, `"version": 2, "version": 3,`)},
		"повтор в export":            {"повторён", rename(`"prefix"`, `"quality": 100, "prefix"`)},
		"глубина":                    {"вложенность", []byte(`{"format":1,"id":"ab","formats":` + strings.Repeat("[", 60) + strings.Repeat("]", 60) + `}`)},
	} {
		if _, err := Parse(c.data); err == nil || !strings.Contains(err.Error(), c.why) {
			t.Errorf("%s: %v, ждали отказ %q", name, err, c.why)
		}
	}
	url := func(u string) func(map[string]any) {
		return func(m map[string]any) { m["export"].(map[string]any)["url"] = u }
	}
	fmtPx := func(w, h int) func(map[string]any) {
		return func(m map[string]any) { m["formats"].([]any)[0].(map[string]any)["px"] = []any{w, h} }
	}
	for name, c := range map[string]struct {
		why string
		f   func(map[string]any)
	}{
		"IDN":              {"хост", url("https://fаbrika-fotoknigi.com/")}, // кириллическая «а»
		"IP":               {"хост", url("https://127.0.0.1/x")},
		"IPv6":             {"хост", url("https://[::1]/x")},
		"пользователь":     {"с именем пользователя", url("https://a@evil.example/x")},
		"без точки":        {"хост", url("https://localhost/x")},
		"изделие чужое":    {"id негодный", func(m map[string]any) { m["products"].([]any)[0].(map[string]any)["id"] = "fsfera-photo" }},
		"px мелкий":        {"px от 100", fmtPx(99, 99)},
		"px 400 Мп":        {"400 Мп", fmtPx(30000, 20000)}, // каждая сторона в пределе, площадь 600 Мп
		"обрез 11":         {"page: зона", func(m map[string]any) { m["page"].(map[string]any)["bleed"] = 11 }},
		"id custom":        {"id: нужны", func(m map[string]any) { m["id"] = "custom" }},
		"название 41 знак": {"name: длиннее 40", func(m map[string]any) { m["name"] = strings.Repeat("я", 41) }},
	} {
		if _, err := Parse(mutate(t, "fabrika", c.f)); err == nil || !strings.Contains(err.Error(), c.why) {
			t.Errorf("%s: %v, ждали отказ %q", name, err, c.why)
		}
	}
	// Punycode-хост, порт 443 и путь проходят; другой порт — TestIdnHostRejected (аудит 123 №3).
	ok := mutate(t, "fabrika", func(m map[string]any) {
		m["export"].(map[string]any)["url"] = "https://xn--80ak6aa92e.com:443/photo-books?a=1"
		for _, pr := range m["products"].([]any) { // адреса изделий — на том же сайте (аудит 123 №1)
			pr.(map[string]any)["url"] = "https://www.xn--80ak6aa92e.com/layflat"
		}
	})
	if _, err := Parse(ok); err != nil {
		t.Errorf("punycode: %v", err)
	}
}
