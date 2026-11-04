package decor

import (
	"reflect"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Ссылки альбома на макеты (looks_ref.go) и обход deepCopy по видам значений.

// Свой макет: ключ, имя, без набора; мусорные и чужие ключи — нет.
func TestLookRefOwn(t *testing.T) {
	s, _ := openTemp(t)
	it := savedLook(t, s, "Свой")
	ref, ok := s.LookRefOf(it.ID)
	if !ok || ref.Key != it.ID || ref.Name != "Свой" || ref.Pack != "" || ref.Item != "" {
		t.Fatalf("LookRefOf: %+v %v", ref, ok)
	}
	if k, ok := s.LookKeyOf(ref); !ok || k != it.ID || !s.HasLook(ref) {
		t.Fatalf("LookKeyOf: %q %v", k, ok)
	}
	for _, id := range []string{"", "sa:x", "../x", "my:zz", "my:ffffffffffffffff"} {
		if r, ok := s.LookRefOf(id); ok || r != (model.LookRef{}) {
			t.Errorf("LookRefOf(%q) = %+v %v", id, r, ok)
		}
	}
	// Пустая и мусорная записи, запись без набора, но с чужим ключом — нет.
	for _, r := range []model.LookRef{{}, {Key: "../x"}, {Key: "my:ffffffffffffffff"}, {Pack: "wedding-layouts"}, {Item: "looks/main.json"}} {
		if k, ok := s.LookKeyOf(r); ok || k != "" || s.HasLook(r) {
			t.Errorf("%+v найден: %q", r, k)
		}
	}
}

// Макет набора: запись несёт id набора и путь; после обновления набора ключ меняется, а запись со
// старым ключом находит новый макет по точному совпадению пути; стёртый набор — не находит.
func TestLookRefPack(t *testing.T) {
	s, _ := openTemp(t)
	v1 := lookBytes(t, lookOf("", ""), "")
	m, src := testPack("wedding-layouts", 1, pit{"looks/main.json", v1})
	install(t, s, m, src, nil)
	key1 := model.LookKey(v1)
	ref, ok := s.LookRefOf(key1)
	if !ok || ref.Key != key1 || ref.Pack != "wedding-layouts" || ref.Item != "looks/main.json" || ref.Name != "looks/main.json" {
		t.Fatalf("LookRefOf набора: %+v %v", ref, ok)
	}
	l2 := lookOf("", "")
	l2.Steps[0].Name = "Начало"
	v2 := lookBytes(t, l2, "")
	m2, src2 := testPack("wedding-layouts", 2, pit{"looks/main.json", v2})
	st, err := s.StagePack(m2, src2, OriginFile, strings.Repeat("cd", 32))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.InstallPack(st, nil); err != nil {
		t.Fatal(err)
	}
	st.Close()
	key2 := model.LookKey(v2)
	if key2 == key1 {
		t.Fatal("ключ не сменился")
	}
	if k, ok := s.LookKeyOf(ref); !ok || k != key2 || !s.HasLook(ref) {
		t.Errorf("обновлённый набор: %q %v", k, ok)
	}
	// Путь не из манифеста или другой набор — не находится.
	for _, r := range []model.LookRef{
		{Key: key1, Pack: "wedding-layouts", Item: "looks/other.json"},
		{Key: key1, Pack: "no-such-pack", Item: "looks/main.json"},
		{Key: key1, Pack: "wedding-layouts", Item: "../main.json"},
	} {
		if k, ok := s.LookKeyOf(r); ok || s.HasLook(r) {
			t.Errorf("%+v найден: %q", r, k)
		}
	}
	if _, err := s.RemovePack("wedding-layouts", nil); err != nil {
		t.Fatal(err)
	}
	if s.HasLook(ref) {
		t.Error("макет удалённого набора найден")
	}
}

// Вид значений deepCopy: словарь (в том числе со срезом внутри), массив, интерфейс и указатель в
// нём — копия равна оригиналу, но не делит с ним память; nil остаётся nil; неэкспортируемое не трогается.
func TestDeepCopyKinds(t *testing.T) {
	type inner struct{ S []int }
	type box struct {
		M     map[string]*inner
		A     [2][]int
		I     any
		P     *inner
		NilM  map[string]int
		NilI  any
		NilP  *inner
		NilS  []int
		empty []int
	}
	src := box{
		M:     map[string]*inner{"a": {S: []int{1, 2}}},
		A:     [2][]int{{1}, {2, 3}},
		I:     &inner{S: []int{7}},
		P:     &inner{S: []int{9}},
		empty: []int{5},
	}
	got := src
	deepCopy(reflect.ValueOf(&got).Elem())
	if !reflect.DeepEqual(src, got) {
		t.Fatalf("копия не равна оригиналу: %+v", got)
	}
	got.M["a"].S[0] = 100
	got.A[1][0] = 100
	got.I.(*inner).S[0] = 100
	got.P.S[0] = 100
	if src.M["a"].S[0] != 1 || src.A[1][0] != 2 || src.I.(*inner).S[0] != 7 || src.P.S[0] != 9 {
		t.Errorf("оригинал изменился через копию: %+v", src)
	}
	if got.NilM != nil || got.NilI != nil || got.NilP != nil || got.NilS != nil {
		t.Errorf("nil стал непустым: %+v", got)
	}
	if &got.empty[0] != &src.empty[0] {
		t.Error("неэкспортируемое поле скопировано")
	}
}

// Кэш: повторная запись ключа заменяет прежнюю (сумма не копится), drop снимает, чужой ключ — нет-оп,
// вытеснение по сумме байт идёт с самого давнего.
func TestParsedLooksPutDrop(t *testing.T) {
	var c parsedLooks
	c.drop("нет") // пустой кэш: не паникует
	c.put("a", "p", fakeInfo{size: 100}, model.AlbumTpl{})
	c.put("a", "p", fakeInfo{size: 40}, model.AlbumTpl{})
	if len(c.m) != 1 || c.bytes != 40 || c.ll.Len() != 1 {
		t.Fatalf("повтор ключа: %d записей, %d байт", len(c.m), c.bytes)
	}
	c.drop("нет")
	if len(c.m) != 1 {
		t.Error("drop чужого ключа снял запись")
	}
	c.drop("a")
	if len(c.m) != 0 || c.bytes != 0 || c.ll.Len() != 0 {
		t.Errorf("drop: %d записей, %d байт", len(c.m), c.bytes)
	}
	third := int64(lookCacheBytes/3 + 1)
	for _, k := range []string{"x", "y", "z"} {
		c.put(k, "p", fakeInfo{size: third}, model.AlbumTpl{})
	}
	if _, ok := c.m["x"]; ok || len(c.m) != 2 || c.bytes != 2*third {
		t.Errorf("вытеснение по сумме: %d записей, %d байт", len(c.m), c.bytes)
	}
}
