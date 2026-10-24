package prefs

import (
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Настройки сети: запись → чтение, чужие ключи целы, всё выключено — ключа нет, правка руками
// чистится (флажок без согласия, плохие и повторные ID, отрицательное время).
func TestLibraryNetConfig(t *testing.T) {
	dir := testkit.TempDir(t)
	if n := LibraryNetOf(dir); n.Allowed || n.AtStart || n.Checked != 0 || n.Skip == nil || len(n.Skip) != 0 {
		t.Fatalf("без файла: %+v", n)
	}
	if err := os.WriteFile(ConfigPath(dir), []byte(`{"future":{"a":1},"level":"pro"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	got := UpdateLibraryNet(dir, func(n *LibraryNet) {
		n.Allowed, n.AtStart, n.Checked, n.CatalogSeq = true, true, 1234, 7
		n.SetSkip("ivory", true)
		n.SetSkip("sage", true)
		n.SetSkip("ivory", true) // повтор — в конец, без дубля
	})
	want := LibraryNet{Allowed: true, AtStart: true, Checked: 1234, CatalogSeq: 7, Skip: []string{"sage", "ivory"}}
	if !equalNet(got, want) || !equalNet(LibraryNetOf(dir), want) {
		t.Fatalf("круг: %+v / %+v", got, LibraryNetOf(dir))
	}
	data, _ := os.ReadFile(ConfigPath(dir))
	if !strings.Contains(string(data), `"future"`) || LoadConfig(dir).Level != "pro" {
		t.Fatalf("чужие ключи: %s", data)
	}
	// Отзыв: согласие и флажок сняты, удалённые, время и seq остаются.
	got = UpdateLibraryNet(dir, func(n *LibraryNet) { n.Allowed = false })
	if got.Allowed || got.AtStart || got.Checked != 1234 || got.CatalogSeq != 7 || len(got.Skip) != 2 {
		t.Fatalf("отзыв: %+v", got)
	}
	// Только seq — ключ в файле остаётся: иначе откат каталога после отзыва и согласия прошёл бы.
	UpdateLibraryNet(dir, func(n *LibraryNet) { *n = LibraryNet{CatalogSeq: 3} })
	if n := LibraryNetOf(dir); n.CatalogSeq != 3 {
		t.Fatalf("только seq: %+v", n)
	}
	// Всё пусто — ключа в файле нет.
	UpdateLibraryNet(dir, func(n *LibraryNet) { *n = LibraryNet{} })
	if data, _ := os.ReadFile(ConfigPath(dir)); strings.Contains(string(data), "libraryNet") {
		t.Fatalf("пустой ключ записан: %s", data)
	}
	// Правка руками.
	raw := `{"libraryNet":{"allowed":false,"atStart":true,"checked":-5,"catalogSeq":-2,"skip":["ok-id","OK","ab","../x","ok-id","con",""]}}`
	if err := os.WriteFile(ConfigPath(dir), []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	if n := LibraryNetOf(dir); n.AtStart || n.Checked != 0 || n.CatalogSeq != 0 || !slices.Equal(n.Skip, []string{"ok-id", "con"}) {
		t.Fatalf("чистка: %+v", n)
	}
}

// Удалённые: снять, снять отсутствующий, переполнение — уходят самые старые.
func TestLibraryNetSkip(t *testing.T) {
	var n LibraryNet
	n.SetSkip("nope", false)
	if len(n.Skip) != 0 {
		t.Fatalf("снять из пустого: %v", n.Skip)
	}
	n.SetSkip("Bad", true)
	if len(n.Skip) != 0 {
		t.Fatalf("плохой id: %v", n.Skip)
	}
	for i := range maxSkip + 3 {
		n.SetSkip("p-"+strconv.Itoa(i), true)
	}
	if len(n.Skip) != maxSkip || n.Skip[0] != "p-3" || n.Skip[maxSkip-1] != "p-"+strconv.Itoa(maxSkip+2) {
		t.Fatalf("переполнение: %d %s…%s", len(n.Skip), n.Skip[0], n.Skip[len(n.Skip)-1])
	}
	n.SetSkip("p-10", false)
	if slices.Contains(n.Skip, "p-10") || len(n.Skip) != maxSkip-1 {
		t.Fatalf("снять: %d", len(n.Skip))
	}
}

func equalNet(a, b LibraryNet) bool {
	return a.Allowed == b.Allowed && a.AtStart == b.AtStart && a.Checked == b.Checked && a.CatalogSeq == b.CatalogSeq && slices.Equal(a.Skip, b.Skip)
}
