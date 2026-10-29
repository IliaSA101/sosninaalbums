package decor

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Поиск предмета только что скачанного набора (спека 144 Г3): packEntry.Src и Store.PackItem.

func srcOf(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:8])
}

// itemPack — набор из двух фонов, стикера, двух разворотов и макета; возвращает хранилище, папку
// настроек и байты предметов.
type itemPack struct {
	s              *Store
	cfg            string
	bg1, bg2, st   []byte
	tpl1, tpl2, lk []byte
}

func newItemPack(t *testing.T) *itemPack {
	t.Helper()
	s, cfg := openTemp(t)
	p := &itemPack{s: s, cfg: cfg, bg1: renderPNG(t, 40, 30), bg2: renderPNG(t, 41, 30), st: renderPNG(t, 16, 16)}
	tpl := func(w float64) []byte {
		return tplJSON(t, model.SpreadTpl{V: 1, Pages: 1, Places: []model.Box{{X: .1, Y: .1, W: w, H: .8}}, Page: []model.TplBg{{Lib: libKey(p.bg1)}}})
	}
	p.tpl1, p.tpl2 = tpl(.8), tpl(.7)
	p.lk = lookBytes(t, lookOf(libKey(p.bg1), ""), `"auto":{"rule":"x"}`)
	m, src := testPack("ivory", 1, pit{"bg/a.png", p.bg1}, pit{"bg/b.png", p.bg2}, pit{"stickers/s.png", p.st},
		pit{"spreads/one.json", p.tpl1}, pit{"spreads/two.json", p.tpl2}, pit{"looks/main.json", p.lk})
	install(t, s, m, src, nil)
	return p
}

// Реестр хранит Src каждого предмета (sha256[:16] байт записи) и переживает перезапуск.
func TestInstallWritesSrc(t *testing.T) {
	p := newItemPack(t)
	reg, err := os.ReadFile(filepath.Join(p.cfg, RootName, packsDir, "ivory.json"))
	if err != nil {
		t.Fatal(err)
	}
	for name, b := range map[string][]byte{"bg1": p.bg1, "bg2": p.bg2, "st": p.st, "tpl1": p.tpl1, "tpl2": p.tpl2, "lk": p.lk} {
		if !strings.Contains(string(reg), `"src": "`+srcOf(b)+`"`) {
			t.Errorf("%s: в реестре нет src %s", name, srcOf(b))
		}
	}
	re := Open(p.cfg) // круг: Save → Load
	for kind, b := range map[string][]byte{KindBg: p.bg2, KindSticker: p.st, KindSpread: p.tpl2, KindAlbum: p.lk} {
		if _, err := re.PackItem("ivory", kind, srcOf(b), 300, 300); err != nil {
			t.Errorf("после перезапуска %s: %v", kind, err)
		}
	}
}

// Картинка находится по ключу ("my:" + src), плитка — та же, что в каталоге; несколько предметов
// набора не путаются.
func TestPackItemImage(t *testing.T) {
	p := newItemPack(t)
	for _, c := range []struct {
		kind string
		b    []byte
	}{{KindBg, p.bg1}, {KindBg, p.bg2}, {KindSticker, p.st}} {
		it, err := p.s.PackItem("ivory", c.kind, srcOf(c.b), 300, 300)
		if err != nil || it.ID != libKey(c.b) || it.PackID != "ivory" || it.Pack == "" || it.URL == "" {
			t.Errorf("%s %s: %+v %v", c.kind, srcOf(c.b), it, err)
		}
	}
	// Вид не тот: фон под видом стикера не находится.
	if _, err := p.s.PackItem("ivory", KindSticker, srcOf(p.bg1), 300, 300); !errors.Is(err, ErrNoItem) {
		t.Errorf("чужой вид: %v", err)
	}
}

// Ключ разворота — TplKey канонического JSON, с sha файла не совпадает: находится по Src.
func TestPackItemSpreadBySrc(t *testing.T) {
	p := newItemPack(t)
	for _, b := range [][]byte{p.tpl1, p.tpl2} {
		it, err := p.s.PackItem("ivory", KindSpread, srcOf(b), 300, 300)
		if err != nil || it.PackID != "ivory" || !strings.HasPrefix(it.ID, "my:") || it.Places != 1 {
			t.Fatalf("разворот %s: %+v %v", srcOf(b), it, err)
		}
		if it.ID == "my:"+srcOf(b) {
			t.Errorf("ключ разворота совпал с sha файла — проверка бессмысленна")
		}
	}
	a, _ := p.s.PackItem("ivory", KindSpread, srcOf(p.tpl1), 300, 300)
	b, _ := p.s.PackItem("ivory", KindSpread, srcOf(p.tpl2), 300, 300)
	if a.ID == b.ID {
		t.Errorf("два разворота — одна плитка %s", a.ID)
	}
}

// Макет: плитка с шагами и картинкой /media/look.
func TestPackItemLook(t *testing.T) {
	p := newItemPack(t)
	it, err := p.s.PackItem("ivory", KindAlbum, srcOf(p.lk), 300, 300)
	if err != nil || it.ID != model.LookKey(p.lk) || it.URL != LookURL(it.ID) || it.Steps == 0 || it.PackID != "ivory" {
		t.Errorf("макет: %+v %v", it, err)
	}
}

// Плохой ввод — ErrNoItem без паники; испорченный Src в реестре — запись не находится.
func TestPackItemBadInput(t *testing.T) {
	p := newItemPack(t)
	good := srcOf(p.bg1)
	for _, c := range []struct{ pack, kind, src string }{
		{"ivory", "print", good}, {"ivory", "", good}, {"ivory", "bg ", good}, {"ivory", "BG", good},
		{"", KindBg, good}, {"../x", KindBg, good}, {"CON", KindBg, good}, {"IVORY", KindBg, good}, {strings.Repeat("a", 41), KindBg, good},
		{"ivory", KindBg, ""}, {"ivory", KindBg, good[:15]}, {"ivory", KindBg, good + "0"}, {"ivory", KindBg, strings.ToUpper(good)},
		{"ivory", KindBg, "zzzzzzzzzzzzzzzz"}, {"ivory", KindBg, "../../etc/passwd"}, {"ivory", KindBg, "my:" + good[:13]},
	} {
		if _, err := p.s.PackItem(c.pack, c.kind, c.src, 300, 300); !errors.Is(err, ErrNoItem) {
			t.Errorf("%+v: %v", c, err)
		}
	}
	// Испорченный Src в файле реестра: разворот не находится, картинки (ключ из sha) — находятся.
	path := filepath.Join(p.cfg, RootName, packsDir, "ivory.json")
	reg, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	bad := strings.ReplaceAll(string(reg), srcOf(p.tpl1), "ZZZ")
	if bad == string(reg) {
		t.Fatal("src разворота в реестре не найден")
	}
	if err := os.WriteFile(path, []byte(bad), 0o600); err != nil {
		t.Fatal(err)
	}
	re := Open(p.cfg)
	if _, err := re.PackItem("ivory", KindSpread, srcOf(p.tpl1), 300, 300); !errors.Is(err, ErrNoItem) {
		t.Errorf("испорченный Src: %v", err)
	}
	if _, err := re.PackItem("ivory", KindSpread, srcOf(p.tpl2), 300, 300); err != nil {
		t.Errorf("целый разворот рядом: %v", err)
	}
	// Старый реестр без src вовсе: картинки и макет находятся по ключу, развороты — нет.
	old := srcField.ReplaceAllString(string(reg), `"x": 0`)
	if old == string(reg) || strings.Contains(old, `"src"`) {
		t.Fatal("src в реестре не вычищены")
	}
	if err := os.WriteFile(path, []byte(old), 0o600); err != nil {
		t.Fatal(err)
	}
	re = Open(p.cfg)
	for _, tpl := range [][]byte{p.tpl1, p.tpl2} {
		if _, err := re.PackItem("ivory", KindSpread, srcOf(tpl), 300, 300); !errors.Is(err, ErrNoItem) {
			t.Errorf("реестр без src, разворот: %v", err)
		}
	}
	for kind, items := range map[string][][]byte{KindBg: {p.bg1, p.bg2}, KindSticker: {p.st}, KindAlbum: {p.lk}} {
		for _, b := range items {
			if _, err := re.PackItem("ivory", kind, srcOf(b), 300, 300); err != nil {
				t.Errorf("реестр без src, %s: %v", kind, err)
			}
		}
	}
}

// srcField — поле src записи реестра (в новых реестрах есть у каждого предмета).
var srcField = regexp.MustCompile(`"src":\s*"[^"]*"`)

// Нет предмета: другой набор, набор не стоит, предмет не из набора, файл предмета пропал.
func TestPackItemMissing(t *testing.T) {
	p := newItemPack(t)
	other := srcOf(renderPNG(t, 99, 99))
	for _, c := range []struct{ pack, kind, src string }{
		{"ivory", KindBg, other}, {"nope", KindBg, srcOf(p.bg1)}, {"ivory", KindSpread, srcOf(p.bg1)}, {"ivory", KindAlbum, srcOf(p.tpl1)},
	} {
		if _, err := p.s.PackItem(c.pack, c.kind, c.src, 300, 300); !errors.Is(err, ErrNoItem) {
			t.Errorf("%+v: %v", c, err)
		}
	}
	if err := os.Remove(filepath.Join(p.cfg, RootName, "Фоны", strings.TrimPrefix(libKey(p.bg1), "my:")+".png")); err != nil {
		t.Fatal(err)
	}
	re := Open(p.cfg) // файла нет — плитки нет (как у каталога)
	if _, err := re.PackItem("ivory", KindBg, srcOf(p.bg1), 300, 300); !errors.Is(err, ErrNoItem) {
		t.Errorf("файл предмета пропал: %v", err)
	}
	if _, err := re.PackItem("ivory", KindBg, srcOf(p.bg2), 300, 300); err != nil {
		t.Errorf("соседний предмет: %v", err)
	}
	p.s.RemovePack("ivory", nil) //nolint:errcheck // проверка ниже
	if _, err := p.s.PackItem("ivory", KindBg, srcOf(p.bg2), 300, 300); !errors.Is(err, ErrNoItem) {
		t.Errorf("набор удалён: %v", err)
	}
}
