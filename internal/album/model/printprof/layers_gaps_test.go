package printprof

import (
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// TestComposeLibraryBranches — библиотечный слой: чужой own-…, новее встроенной, но спорит
// названием или изделием; новая типография (не перекрытие) со спором и без.
func TestComposeLibraryBranches(t *testing.T) {
	b := builtinVersion(t, "fabrika")
	skippedOne := func(name string, lib []Profile, want string) {
		t.Helper()
		snap, skipped := Compose(lib, nil)
		if len(skipped) != 1 || !strings.Contains(skipped[0], want) || len(snap.Profiles()) != 3 {
			t.Errorf("%s: %v %v", name, ids(snap), skipped)
		}
	}
	// Валидный профиль с id own-… в наборе: набор в пространство имён своих не пускаем.
	skippedOne("набор с own-", []Profile{own(t, "fabrika", "own-0a1b2c3d", "Из набора")}, "только у своих")
	// Перекрытие fabrika, но название занято другой встроенной.
	dup := libOf(t, "fabrika", b+1)
	dup.Name = mustProfile(t, "fsfera").Name
	skippedOne("перекрытие с чужим названием", []Profile{dup}, "уже есть")
	// Новая типография набора: свой id и изделия (по образцу своей, но без own-).
	fresh := own(t, "fabrika", "newlab", "Новая лаборатория")
	snap, skipped := Compose([]Profile{fresh}, nil)
	if len(skipped) != 0 || len(snap.Profiles()) != 4 || snap.Source("newlab") != SourceLibrary {
		t.Fatalf("новая из набора: %v %v", ids(snap), skipped)
	}
	// Новая, но спорит с реестром названием.
	clashName := own(t, "fabrika", "newlab", mustProfile(t, "netprint").Name)
	skippedOne("новая с чужим названием", []Profile{clashName}, "уже есть")
}

func mustProfile(t *testing.T, id string) Profile {
	t.Helper()
	p, ok := Builtin().ByID(id)
	if !ok {
		t.Fatal(id)
	}
	return p
}

// TestComposeOwnBranches — своя с верным id, но изделием не по грамматике; предел MaxOwn;
// снимок из Compose всегда собирается заново NewSnapshot (инвариант, на который опирается Compose).
func TestComposeOwnBranches(t *testing.T) {
	o := own(t, "fabrika", "own-0a1b2c3d", "Моя")
	o.Products = slices.Clone(o.Products)
	o.Products[0].ID = "own-0a1b2c3d-x"
	o.Default = o.Products[0].ID
	if err := o.Validate(); err != nil { // должна дойти до грамматики изделий, а не отсекаться Validate
		t.Fatal(err)
	}
	snap, skipped := Compose(nil, []Profile{o})
	if len(skipped) != 1 || len(snap.Profiles()) != 3 {
		t.Errorf("изделие не по грамматике: %v %v", ids(snap), skipped)
	}

	var many []Profile
	for i := range MaxOwn + 2 {
		id := "own-" + strconv.FormatInt(int64(0x10000000+i), 16)
		many = append(many, own(t, "fabrika", id, "Своя "+strconv.Itoa(i)))
	}
	snap, skipped = Compose(nil, many)
	if len(snap.Profiles()) != 3+MaxOwn || len(skipped) != 2 || !strings.Contains(skipped[0], "своих больше") {
		t.Errorf("предел: %d профилей, пропуски %v", len(snap.Profiles()), skipped)
	}
	if _, err := NewSnapshot(snap.Profiles()); err != nil {
		t.Errorf("снимок Compose не собирается заново: %v", err)
	}
}

// TestOwnProducts — изделие своей: только own-<id>-<число>, без знака и чужого префикса.
func TestOwnProducts(t *testing.T) {
	mk := func(ids ...string) Profile {
		p := Profile{ID: "own-0a1b2c3d"}
		for _, id := range ids {
			p.Products = append(p.Products, Product{ID: id})
		}
		return p
	}
	for _, c := range []struct {
		ids  []string
		want bool
	}{
		{nil, true},
		{[]string{"own-0a1b2c3d-1", "own-0a1b2c3d-22"}, true},
		{[]string{"own-0a1b2c3d-+1"}, false},
		{[]string{"own-0a1b2c3d--1"}, false},
		{[]string{"own-0a1b2c3d-x"}, false},
		{[]string{"own-0a1b2c3d"}, false},
		{[]string{"own-ffffffff-1"}, false},
	} {
		if got := ownProducts(mk(c.ids...)); got != c.want {
			t.Errorf("%v: %v", c.ids, got)
		}
	}
}

// TestInfoOfPx — кнопка формата несёт px цели, когда они заданы, и не несёт, когда нет.
func TestInfoOfPx(t *testing.T) {
	p := mustProfile(t, "fabrika")
	p.Formats = slices.Clone(p.Formats)
	p.Formats[0].Px = &PxPair{7000, 3000}
	p.Formats[1].Px = nil
	in := InfoOf(p, SourceLibrary)
	if in.Formats[0].PxW != 7000 || in.Formats[0].PxH != 3000 || in.Formats[1].PxW != 0 || in.Source != SourceLibrary {
		t.Errorf("%+v", in.Formats[:2])
	}
}

// TestSnapshotProductClash — одно изделие у двух типографий с разными id и названиями: id «ab» с
// изделием «ab-c-1» и id «ab-c» с изделием «ab-c-1» (префикс-правило Validate их не разводит).
func TestSnapshotProductClash(t *testing.T) {
	a := own(t, "fsfera", "ab", "А")
	old := a.Products[0].ID
	a.Products[0].ID = "ab-c-1"
	if a.Default == old {
		a.Default = "ab-c-1"
	}
	b := own(t, "fabrika", "ab-c", "Б")
	if err := errorsOf(a.Validate(), b.Validate()); err != nil {
		t.Fatal(err)
	}
	if _, err := NewSnapshot([]Profile{a, b}); err == nil || !strings.Contains(err.Error(), "изделие") {
		t.Errorf("снимок собран или не та причина: %v", err)
	}
}

func errorsOf(errs ...error) error {
	for _, e := range errs {
		if e != nil {
			return e
		}
	}
	return nil
}

// TestFieldTypesNoTag — поле без json-тега зовётся по имени Go, «-» тоже; не структура — nil.
func TestFieldTypesNoTag(t *testing.T) {
	type s struct {
		Plain int
		Tag   string `json:"tag,omitempty"`
		Skip  int    `json:"-"`
	}
	got := fieldTypes(reflect.TypeOf(s{}))
	if len(got) != 3 || got["Plain"] == nil || got["tag"] == nil || got["Skip"] == nil {
		t.Errorf("%v", got)
	}
	if fieldTypes(nil) != nil || fieldTypes(reflect.TypeOf(0)) != nil {
		t.Error("не структура")
	}
}

// TestComposeLibraryTies — два набора с одним id: старшая версия побеждает при любом порядке,
// равная версия — прежняя (первый установленный).
func TestComposeLibraryTies(t *testing.T) {
	b := builtinVersion(t, "fabrika")
	mk := func(v int, mark string) Profile {
		p := libOf(t, "fabrika", v)
		p.Page.Hint += mark
		return p
	}
	for name, c := range map[string]struct {
		lib  []Profile
		want string
	}{
		"позже старше":  {[]Profile{mk(b+1, "A"), mk(b+2, "B")}, "B"},
		"раньше старше": {[]Profile{mk(b+2, "A"), mk(b+1, "B")}, "A"},
		"равные":        {[]Profile{mk(b+1, "A"), mk(b+1, "B")}, "A"},
	} {
		snap, _ := Compose(c.lib, nil)
		got, _ := snap.ByID("fabrika")
		if !strings.HasSuffix(got.Page.Hint, c.want) || len(snap.Profiles()) != 3 {
			t.Errorf("%s: действует %q (%d типографий)", name, got.Page.Hint[len(got.Page.Hint)-1:], len(snap.Profiles()))
		}
	}
}

// TestUseBuiltinForTest — подмена встроенных действует до конца теста: Compose и Current видят
// новую версию, после теста встроенные прежние.
func TestUseBuiltinForTest(t *testing.T) {
	orig := builtinVersion(t, "fabrika")
	t.Run("подмена", func(t *testing.T) {
		var ps []Profile
		for _, p := range Builtin().Profiles() {
			if p.ID == "fabrika" {
				p.Version = orig + 2
			}
			ps = append(ps, p)
		}
		snap, err := NewSnapshot(ps)
		if err != nil {
			t.Fatal(err)
		}
		UseBuiltinForTest(t, snap)
		if got := builtinVersion(t, "fabrika"); got != orig+2 {
			t.Fatalf("версия %d", got)
		}
		// Набор версии orig+1 новой встроенной не перекрывает.
		c, _ := Compose([]Profile{libOf(t, "fabrika", orig+1)}, nil)
		if got, _ := c.ByID("fabrika"); got.Version != orig+2 || c.Source("fabrika") != SourceBuiltin {
			t.Errorf("перекрыла старая версия: %d", got.Version)
		}
	})
	if got := builtinVersion(t, "fabrika"); got != orig {
		t.Errorf("после теста версия %d", got)
	}
}
