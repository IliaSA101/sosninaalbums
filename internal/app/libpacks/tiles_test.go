package libpacks

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/decor"
	"github.com/IliaSA101/sosninaalbums/internal/decor/fetch"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Плитки «Можно скачать» (спека 144 Г3): правила показа, порядок, пределы, приглашение.

// tilesFx — стенд с тремя наборами pack-a..c и их листами на диске; mut правит строки до сохранения.
func tilesFx(t *testing.T, mut func(rows []fetch.Entry)) (*prevFx, []fetch.Entry) {
	t.Helper()
	fx := newPrevFx(t, nil)
	rows, ps := rowsFx(fx)
	for i, p := range ps {
		fx.put(p, sheetJPEG(t, 10+i))
	}
	fx.seq++
	for i := range rows {
		rows[i].Version, rows[i].MinApp, rows[i].Size = 1, "0.1.0", 10
		rows[i].Title, rows[i].SHA256 = "T "+rows[i].ID, strings.Repeat("a", 64)
		rows[i].Counts = fetch.Counts{Bg: 3, Sticker: 4}
	}
	if mut != nil {
		mut(rows)
	}
	fx.s.remember(fetch.Catalog{Seq: fx.seq, Packs: rows})
	return fx, rows
}

func tileIDs(ts []Tile) (out []string) {
	for _, t := range ts {
		out = append(out, t.ID)
	}
	return out
}

func tilesOf(ts []Tile, id string) bool {
	return slices.ContainsFunc(ts, func(x Tile) bool { return x.Pack == id })
}

// Порядок каталога, внутри — порядок клеток; адрес, размеры клетки, состав и размер набора.
func TestTilesRules(t *testing.T) {
	fx, rows := tilesFx(t, nil)
	got := fx.s.Tiles("bg")
	if len(got) != 9 {
		t.Fatalf("плиток %d, нужно 9: %v", len(got), tileIDs(got))
	}
	src := rows[0].Preview.Rows[0].Items[0]
	f := got[0]
	if f.ID != "get:pack-a:"+src || f.Src != src || f.Pack != "pack-a" || f.Title != "T pack-a" ||
		f.URL != "/media/lp/pack-a/bg/0?v="+rows[0].Preview.SHA256[:8] || f.W != 64 || f.H != 48 || f.Bytes != 10 ||
		f.With == nil || len(f.With) != 0 || f.Bg != 3 || f.Sticker != 4 {
		t.Errorf("первая плитка %+v", f)
	}
	for i, id := range []string{"pack-a", "pack-a", "pack-a", "pack-b", "pack-b", "pack-b", "pack-c", "pack-c", "pack-c"} {
		if got[i].Pack != id || !strings.HasSuffix(strings.SplitN(got[i].URL, "?", 2)[0], "/bg/"+string(rune('0'+i%3))) {
			t.Errorf("плитка %d: %s %s", i, got[i].Pack, got[i].URL)
		}
	}
	if st := fx.s.Tiles("sticker"); len(st) != 12 || st[0].W != 32 {
		t.Errorf("стикеры: %d", len(st))
	}
	for _, kind := range []string{"album", "spread", "frame", "print", "", "../bg"} {
		if got := fx.s.Tiles(kind); got == nil || len(got) != 0 {
			t.Errorf("вид %q: %v (нужен пустой не nil)", kind, got)
		}
	}
}

// Пределы на набор: до 8 фонов, 12 стикеров, 6 рамочек, 4 разворотов, 1 макет.
func TestTilesPerPackLimit(t *testing.T) {
	items := func(n int) []string {
		out := make([]string, n)
		for i := range out {
			out[i] = strings.Repeat("a", 15) + string(rune('a'+i))
		}
		return out
	}
	for kind, limit := range map[string]int{"bg": 8, "sticker": 12, "frame": 6, "spread": 4, "album": 1} {
		fx, _ := tilesFx(t, func(rows []fetch.Entry) { // тот же лист на диске, другие ряды строки
			p := *rows[0].Preview
			p.Rows = []fetch.PreviewRow{{Kind: kind, W: 32, H: 32, Items: items(12)}}
			rows[0].Preview = &p
		})
		n := 0
		for _, tile := range fx.s.Tiles(kind) {
			if tile.Pack == "pack-a" {
				n++
			}
		}
		if n != limit {
			t.Errorf("%s: плиток набора %d, нужно %d", kind, n, limit)
		}
	}
}

// Нет плиток: нет списка, нет согласия, нет папки, нет листа, лист плохой, библиотека недоступна.
func TestTilesNone(t *testing.T) {
	t.Run("нет списка", func(t *testing.T) {
		fx := newPrevFx(t, nil)
		if got := fx.s.Tiles("bg"); got == nil || len(got) != 0 {
			t.Errorf("%v", got)
		}
	})
	t.Run("нет согласия", func(t *testing.T) {
		fx, _ := tilesFx(t, nil)
		prefs.UpdateLibraryNet(fx.dir, func(n *prefs.LibraryNet) { n.Allowed = false })
		if got := fx.s.Tiles("bg"); len(got) != 0 {
			t.Errorf("без согласия %d плиток", len(got))
		}
	})
	t.Run("нет папки настроек", func(t *testing.T) {
		fx, _ := tilesFx(t, nil)
		fx.s.h.CfgDir = func() string { return "" }
		if got := fx.s.Tiles("bg"); len(got) != 0 {
			t.Errorf("без папки %d плиток", len(got))
		}
	})
	t.Run("нет листа", func(t *testing.T) {
		fx, rows := tilesFx(t, nil)
		if err := os.Remove(fx.sheetFile(rows[1].Preview)); err != nil {
			t.Fatal(err)
		}
		got := fx.s.Tiles("bg")
		if len(got) != 6 || tilesOf(got, "pack-b") {
			t.Errorf("без листа pack-b: %v", tileIDs(got))
		}
	})
	t.Run("лист плохой", func(t *testing.T) {
		fx, rows := tilesFx(t, nil)
		fx.s.cells.markBad(layoutKey(rows[0].Preview))
		got := fx.s.Tiles("bg")
		if len(got) != 6 || tilesOf(got, "pack-a") {
			t.Errorf("с плохим листом pack-a: %v", tileIDs(got))
		}
	})
	t.Run("только чтение", func(t *testing.T) {
		fx, _ := tilesFx(t, nil)
		dir := testkit.TempDir(t)
		root := filepath.Join(dir, decor.RootName)
		if err := os.MkdirAll(root, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "library.json"), []byte(`{"version":99,"items":[]}`), 0o600); err != nil {
			t.Fatal(err)
		}
		ro := decor.Open(dir)
		if !ro.ReadOnly() {
			t.Fatal("библиотека не стала только для чтения")
		}
		fx.s.h.Store = func() (*decor.Store, error) { return ro, nil }
		if got := fx.s.Tiles("bg"); len(got) != 0 {
			t.Errorf("только чтение: %d плиток", len(got))
		}
		if fx.s.Invite() {
			t.Error("только чтение: приглашения нет")
		}
	})
}

// Нужна новая программа, «удалён вами», донор в skip, стоящий набор — без плиток; дополнение к
// нестоящему донору — с размером и названием обоих.
func TestTilesWhySkipAndNeeds(t *testing.T) {
	t.Run("нужна новая программа", func(t *testing.T) {
		fx, _ := tilesFx(t, func(rows []fetch.Entry) { rows[0].MinApp = "99.0.0" })
		if got := fx.s.Tiles("bg"); len(got) != 6 || tilesOf(got, "pack-a") {
			t.Errorf("%v", tileIDs(got))
		}
	})
	t.Run("донор просит новую программу", func(t *testing.T) {
		fx, _ := tilesFx(t, func(rows []fetch.Entry) { rows[0].Needs, rows[1].MinApp = "pack-b", "99.0.0" })
		if got := fx.s.Tiles("bg"); tilesOf(got, "pack-a") || tilesOf(got, "pack-b") || !tilesOf(got, "pack-c") {
			t.Errorf("%v", tileIDs(got))
		}
	})
	t.Run("удалён вами", func(t *testing.T) {
		fx, _ := tilesFx(t, nil)
		prefs.UpdateLibraryNet(fx.dir, func(n *prefs.LibraryNet) { n.Skip = []string{"pack-b"} })
		if got := fx.s.Tiles("bg"); len(got) != 6 || tilesOf(got, "pack-b") {
			t.Errorf("%v", tileIDs(got))
		}
	})
}

// Доноры и дополнения: донор в skip, донора нет в списке, дополнение к нестоящему, набор уже стоит.
func TestTilesDonors(t *testing.T) {
	t.Run("донор удалён вами", func(t *testing.T) {
		fx, _ := tilesFx(t, func(rows []fetch.Entry) { rows[2].Needs = "pack-b" })
		prefs.UpdateLibraryNet(fx.dir, func(n *prefs.LibraryNet) { n.Skip = []string{"pack-b"} })
		if got := fx.s.Tiles("bg"); tilesOf(got, "pack-b") || tilesOf(got, "pack-c") || !tilesOf(got, "pack-a") {
			t.Errorf("%v", tileIDs(got))
		}
	})
	t.Run("донора нет в списке", func(t *testing.T) {
		fx, _ := tilesFx(t, func(rows []fetch.Entry) { rows[2].Needs = "pack-zzz" })
		if got := fx.s.Tiles("bg"); tilesOf(got, "pack-c") || len(got) != 6 {
			t.Errorf("%v", tileIDs(got))
		}
	})
	t.Run("дополнение к нестоящему донору", func(t *testing.T) {
		fx, _ := tilesFx(t, func(rows []fetch.Entry) { rows[1].Needs = "pack-a" })
		var a, b Tile
		for _, tile := range fx.s.Tiles("bg") {
			switch tile.Pack {
			case "pack-a":
				a = tile
			case "pack-b":
				b = tile
			}
		}
		if b.Pack == "" || b.Bytes != 20 || !slices.Equal(b.With, []string{"T pack-a"}) {
			t.Errorf("дополнение: %+v", b)
		}
		if a.Bytes != 20 || !slices.Equal(a.With, []string{"T pack-b"}) {
			t.Errorf("донор: %+v", a)
		}
	})
	t.Run("набор уже стоит", func(t *testing.T) {
		fx, _ := tilesFx(t, func(rows []fetch.Entry) { rows[0].ID = "ivory" })
		if _, err := fx.s.AddPath(d1Zip(t)); err != nil {
			t.Fatal(err)
		}
		if got := fx.s.Tiles("bg"); tilesOf(got, "ivory") || len(got) != 6 {
			t.Errorf("%v", tileIDs(got))
		}
	})
}

// Приглашение: списка ни разу не было и библиотека не «только чтение»; со списком — нет, с любым
// согласием.
func TestInvite(t *testing.T) {
	fx := newPrevFx(t, nil)
	if !fx.s.Invite() {
		t.Error("списка нет — приглашение должно быть")
	}
	prefs.UpdateLibraryNet(fx.dir, func(n *prefs.LibraryNet) { n.Allowed = false })
	if !fx.s.Invite() {
		t.Error("согласие для приглашения не важно")
	}
	fx2, _ := tilesFx(t, nil)
	if fx2.s.Invite() {
		t.Error("список есть — приглашения нет")
	}
	prefs.UpdateLibraryNet(fx2.dir, func(n *prefs.LibraryNet) { n.Allowed = false })
	if fx2.s.Invite() {
		t.Error("список есть, согласия нет — приглашения нет")
	}
}

// Файл списка есть, но не читается (bad): списка «ни разу не было» нет — приглашения нет.
func TestInviteBadList(t *testing.T) {
	fx := newPrevFx(t, nil)
	if err := os.WriteFile(filepath.Join(fx.dir, catalogFile), []byte("не json"), 0o600); err != nil {
		t.Fatal(err)
	}
	logTo(t)
	if fx.s.Invite() {
		t.Error("список не читается — приглашения нет")
	}
	if _, _, bad := fx.s.cached(); !bad {
		t.Error("файл списка должен считаться испорченным")
	}
}
