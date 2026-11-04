package decor

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Задел совместимости с 0.28 (решение 0.27-131-30): сборка мусора 0.27 держит картинки, на которые
// ссылаются записи незнакомого вида и поля новее программы, читая сырые байты.

const rawK1, rawK2 = "my:0123456789abcdef", "my:fedcba9876543210"

func TestRawLibKeys(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{"пусто", "", nil},
		{"нет ключей", `{"a":"my:","b":"my:XYZ0123456789abc"}`, nil},
		{"один", `{"lib":"` + rawK1 + `"}`, []string{rawK1}},
		{"повтор и два", rawK1 + rawK2 + rawK1, []string{rawK1, rawK2}},
		{"усечён на 15 знаках", "my:0123456789abcde", nil},
		{"ровно 16 в конце", "xmy:0123456789abcdef", []string{rawK1}},
		{"заглавные не ключ", "my:0123456789ABCDEF", nil},
		{"хвост длиннее 16 — по первым 16", "my:0123456789abcdef00", []string{rawK1}},
		{"префикс внутри префикса", "my:my:0123456789abcdef", []string{rawK1}},
		{"бинарный мусор", "\x00\xff my:\x00\x00", nil},
	}
	for _, c := range cases {
		got := rawLibKeys([]byte(c.in))
		if strings.Join(got, ",") != strings.Join(c.want, ",") {
			t.Errorf("%s: %v, ждали %v", c.name, got, c.want)
		}
	}
}

func FuzzRawLibKeys(f *testing.F) {
	f.Add([]byte(`{"libs":["` + rawK1 + `"]}`))
	f.Add([]byte("my:my:my:"))
	f.Fuzz(func(t *testing.T, data []byte) {
		for _, k := range rawLibKeys(data) {
			if !model.ValidLibKey(k) || !bytes.Contains(data, []byte(k)) {
				t.Fatalf("ключ %q не из входа или не по грамматике", k)
			}
		}
	})
}

// rawLibsFixture — набор из четырёх картинок (a, b, c, d); ключи — по содержимому.
type rawLibsFixture struct {
	s       *Store
	cfg     string
	key     [4]string
	root    string
	packReg string
}

func newRawFixture(t *testing.T) *rawLibsFixture {
	t.Helper()
	s, cfg := openTemp(t)
	imgs := [4][]byte{renderPNG(t, 91, 30), renderPNG(t, 92, 30), renderPNG(t, 93, 30), renderPNG(t, 94, 30)}
	m, src := testPack("ivory", 1, pit{"bg/a.png", imgs[0]}, pit{"bg/b.png", imgs[1]}, pit{"bg/c.png", imgs[2]}, pit{"bg/d.png", imgs[3]})
	install(t, s, m, src, nil)
	fx := &rawLibsFixture{s: s, cfg: cfg, root: filepath.Join(cfg, RootName)}
	fx.packReg = filepath.Join(fx.root, packsDir, "ivory.json")
	for i := range imgs {
		fx.key[i] = libKey(imgs[i])
	}
	return fx
}

func (fx *rawLibsFixture) write(t *testing.T, rel, data string) {
	t.Helper()
	p := filepath.Join(fx.root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

// remove — план и удаление набора ivory на свежем Open (кэш разбора пуст); возвращает, какие из
// картинок остались на диске.
func (fx *rawLibsFixture) remove(t *testing.T) (kept [4]bool, res PackRemove) {
	t.Helper()
	s := Open(fx.cfg)
	plan, err := s.PlanRemovePack("ivory", nil)
	if err != nil {
		t.Fatal(err)
	}
	res, err = s.RemovePack("ivory", nil)
	if err != nil || res != plan {
		t.Fatalf("удаление: %v %+v, план %+v", err, res, plan)
	}
	for i, k := range fx.key {
		_, kept[i] = Open(fx.cfg).Find(k)
	}
	return kept, res
}

func (fx *rawLibsFixture) want(t *testing.T, kept [4]bool, want [4]bool) {
	t.Helper()
	if kept != want {
		t.Errorf("уцелели картинки %v, ждали %v", kept, want)
	}
}

// Файл Макеты/<hex>.json (запись вида album 0.28) держит картинки шагов, обложки и рамочек;
// без ссылки картинка удаляется, как раньше.
func TestRemovePackKeepsAlbumFile(t *testing.T) {
	fx := newRawFixture(t)
	fx.write(t, "Макеты/00000000000000a1.json",
		`{"v":1,"cover":{"bg":{"lib":"`+fx.key[0]+`"}},"steps":[{"tpl":{"v":1,"borders":[{"i":0,"lib":"`+fx.key[1]+`"}],"stickers":[{"lib":"`+fx.key[2]+`"}]}}]}`)
	kept, res := fx.remove(t)
	fx.want(t, kept, [4]bool{true, true, true, false})
	if res.Files != 1 || res.Kept != 3 {
		t.Errorf("итог %+v", res)
	}
}

// Без записей незнакомого вида — как раньше: все четыре картинки уходят.
func TestRemovePackNoUnknownDeletesAll(t *testing.T) {
	fx := newRawFixture(t)
	fx.write(t, "Макеты/00000000000000a1.json", `{"v":1,"steps":[]}`)
	fx.write(t, "Макеты/readme.txt", "ключ "+fx.key[0]+" в не-json не считается")
	kept, res := fx.remove(t)
	fx.want(t, kept, [4]bool{})
	if res.Files != 4 || res.Kept != 0 {
		t.Errorf("итог %+v", res)
	}
}

// Запись незнакомого вида в library.json (хоть с полем libs, хоть с любым другим) и в реестре
// другого набора держит ссылки; запись известного вида — нет (её держит tplRefs).
func TestRemovePackKeepsUnknownRegistryRecords(t *testing.T) {
	fx := newRawFixture(t)
	fx.write(t, "library.json", `{"version":1,"items":[
	  {"key":"my:00000000000000b1","kind":"album","theme":"mine","name":"Макет","libs":["`+fx.key[0]+`"]},
	  {"key":"my:00000000000000b2","kind":"bg","theme":"mine","name":"Известный","note":"`+fx.key[3]+`"}]}`)
	fx.write(t, "Наборы/layouts.json", `{"v":1,"id":"layouts","version":1,"title":"Макеты","items":[
	  {"key":"my:00000000000000c1","kind":"album","name":"М","extra":{"refs":["`+fx.key[1]+`"]}}]}`)
	kept, _ := fx.remove(t)
	fx.want(t, kept, [4]bool{true, true, false, false})
}

// Запись незнакомого вида в реестре самого удаляемого набора ничего не держит: после удаления её нет.
func TestRemovePackIgnoresOwnRegistryUnknown(t *testing.T) {
	fx := newRawFixture(t)
	data, err := os.ReadFile(fx.packReg)
	if err != nil {
		t.Fatal(err)
	}
	i := strings.Index(string(data), `"items": [`)
	if i < 0 {
		t.Fatal("нет items в реестре набора")
	}
	j := i + len(`"items": [`)
	fx.write(t, "Наборы/ivory.json", string(data[:j])+`{"key":"my:00000000000000c2","kind":"album","libs":["`+fx.key[3]+`"]},`+string(data[j:]))
	kept, _ := fx.remove(t)
	fx.want(t, kept, [4]bool{})
}

// Шаблон v1 с borders[].lib (0.28) держит рамочку: 0.27 не знает поля borders, но читает байты.
// Так же — шаблон новее программы (разбор отказывает) и битый JSON с ключом внутри.
func TestRemovePackKeepsTplBordersLib(t *testing.T) {
	fx := newRawFixture(t)
	box := []model.Box{{X: .1, Y: .1, W: .8, H: .8}}
	it, err := fx.s.SaveSpreadTpl("mine", "Рамочка", model.SpreadTpl{V: 1, Pages: 1, Places: box, Page: []model.TplBg{{Color: "#aabbcc"}}})
	if err != nil {
		t.Fatal(err)
	}
	path, ok := fx.s.files[KindSpread][it.ID]
	if !ok {
		t.Fatal("нет файла шаблона")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	withBorders := strings.Replace(string(raw), `{"v":1`, `{"v":1,"borders":[{"i":0,"lib":"`+fx.key[0]+`"}]`, 1)
	if withBorders == string(raw) {
		t.Fatalf("не вставилось: %s", raw)
	}
	for name, body := range map[string]string{
		"v1 с borders":  withBorders,
		"новее":         `{"v":99,"page":[{"lib":"` + fx.key[0] + `"}]}`,
		"битый":         `{"v":1,"borders":[{"lib":"` + fx.key[0],
		"без ссылки":    `{"v":1,"borders":[{"i":0,"kind":"line"}],"places":[{"x":0.1,"y":0.1,"w":0.8,"h":0.8}],"pages":1}`,
		"ключ не в hex": `{"v":1,"borders":[{"lib":"my:` + strings.Repeat("g", 16) + `"}]}`,
	} {
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		kept, _ := fx.remove(t)
		want := [4]bool{name != "без ссылки" && name != "ключ не в hex", false, false, false}
		if kept != want {
			t.Errorf("%s: уцелели %v, ждали %v", name, kept, want)
		}
		fx = reinstall(t, fx)
		path = fx.s.files[KindSpread][it.ID]
		if _, err := os.Stat(path); err != nil { // шаблон тот же ключ: перезаписанный файл на месте
			t.Fatalf("%s: %v", name, err)
		}
	}
}

// reinstall — тот же набор заново (после удаления) в том же каталоге данных.
func reinstall(t *testing.T, fx *rawLibsFixture) *rawLibsFixture {
	t.Helper()
	s := Open(fx.cfg)
	imgs := [4][]byte{renderPNG(t, 91, 30), renderPNG(t, 92, 30), renderPNG(t, 93, 30), renderPNG(t, 94, 30)}
	m, src := testPack("ivory", 1, pit{"bg/a.png", imgs[0]}, pit{"bg/b.png", imgs[1]}, pit{"bg/c.png", imgs[2]}, pit{"bg/d.png", imgs[3]})
	install(t, s, m, src, nil)
	return &rawLibsFixture{s: s, cfg: fx.cfg, key: fx.key, root: fx.root, packReg: fx.packReg}
}

// Мусор в незнакомых местах не роняет сборку: пустой и бинарный файл, JSON не того вида, каталог
// с именем .json, реестр набора не JSON, library.json со странной записью.
func TestRemovePackGarbageUnknown(t *testing.T) {
	fx := newRawFixture(t)
	fx.write(t, "Макеты/0000000000000001.json", "")
	fx.write(t, "Макеты/0000000000000002.json", "\x00\x01\xff\xfe")
	fx.write(t, "Макеты/0000000000000003.json", `[1,2,3]`)
	fx.write(t, "Макеты/0000000000000004.json", `{"steps":[{"tpl":`+"\x00"+fx.key[3]) // битый, ключ внутри — держим
	if err := os.MkdirAll(filepath.Join(fx.root, "Макеты", "dir.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	fx.write(t, "Наборы/broken.json", `{{{ `+fx.key[0])
	fx.write(t, "library.json", `{"version":1,"items":[null, {"kind":"album"}]}`)
	fx.write(t, "Наборы/weird.json", `{"v":1,"id":"weird","items":[{"key":1,"kind":7,"x":"`+fx.key[1]+`"}]}`) // 0.27 набор не загрузит; 0.28 — возможно
	kept, _ := fx.remove(t)
	// d держит битый файл по ключу внутри; b — запись со странным kind (не строка: считается незнакомой) в реестре, который 0.27 не грузит;
	// a и c ничто не держит (реестр набора broken не разбирается — не держит).
	fx.want(t, kept, [4]bool{false, true, false, true})
}

// Предел размера: файл незнакомого вида больше rawFileMax не читается целиком, и сборка мусора
// ничего не удаляет (лучше оставить лишнее, чем стереть нужное).
func TestRemovePackUnknownTooBigKeepsAll(t *testing.T) {
	fx := newRawFixture(t)
	fx.write(t, "Макеты/0000000000000009.json", `{"x":"`+strings.Repeat("a", rawFileMax)+`"}`)
	kept, res := fx.remove(t)
	fx.want(t, kept, [4]bool{true, true, true, true})
	if res.Files != 0 || res.Kept != 4 {
		t.Errorf("итог %+v", res)
	}
}

// Предел числа файлов: больше rawFiles файлов в незнакомой папке — тоже ничего не удаляется.
func TestRemovePackUnknownTooManyKeepsAll(t *testing.T) {
	fx := newRawFixture(t)
	dir := filepath.Join(fx.root, "Макеты")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for i := range rawFiles + 1 {
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("f%07d.json", i)), []byte(`{}`), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	kept, _ := fx.remove(t)
	fx.want(t, kept, [4]bool{true, true, true, true})
}

// allKept — после удаления набора на свежем Open все четыре картинки на месте и удалилось 0 файлов.
func (fx *rawLibsFixture) allKept(t *testing.T, name string) {
	t.Helper()
	kept, res := fx.remove(t)
	if kept != [4]bool{true, true, true, true} || res.Files != 0 || res.Kept != 4 {
		t.Errorf("%s: уцелели %v, итог %+v — ждали «держим всё»", name, kept, res)
	}
}

// Ссылка (симлинк, junction) на незнакомую папку корня, .json-ссылка в Макеты/ и в Наборы/ не
// читаются — значит, держим всё. Без ссылки (контроль) картинки уходят.
func TestRemovePackSymlinkKeepsAll(t *testing.T) {
	target := testkit.TempDir(t)
	cases := []struct{ name, link string }{
		{"папка в корне", "Чужое"},
		{"json в Макеты", "Макеты/l.json"},
		{"json в Наборы", "Наборы/l.json"},
	}
	for _, c := range cases {
		fx := newRawFixture(t)
		if err := os.MkdirAll(filepath.Join(fx.root, "Макеты"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, filepath.Join(fx.root, c.link)); err != nil {
			t.Skip("симлинк недоступен:", err)
		}
		fx.allKept(t, c.name)
	}
	// Симлинк на известную папку вида или не-json ссылка в Макеты/ — не повод держать всё.
	fx := newRawFixture(t)
	if err := os.MkdirAll(filepath.Join(fx.root, "Макеты"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(fx.root, "Макеты", "l.txt")); err != nil {
		t.Skip("симлинк недоступен:", err)
	}
	if kept, _ := fx.remove(t); kept != [4]bool{} {
		t.Errorf("не-json ссылка держит картинки: %v", kept)
	}
}

// Шаблон из индекса не прочитался (больше MaxTplFile): его ссылок не знаем — сборка мусора ничего не
// удаляет; SpreadTpl наружу по-прежнему ErrNotFound. Стёртый файл — просто нет шаблона.
func TestRemovePackUnreadTplKeepsAll(t *testing.T) {
	fx := newRawFixture(t)
	box := []model.Box{{X: .1, Y: .1, W: .8, H: .8}}
	it, err := fx.s.SaveSpreadTpl("mine", "Большой", model.SpreadTpl{V: 1, Pages: 1, Places: box, Page: []model.TplBg{{Color: "#aabbcc"}}})
	if err != nil {
		t.Fatal(err)
	}
	path := fx.s.files[KindSpread][it.ID]
	if err := os.WriteFile(path, bytes.Repeat([]byte(" "), model.MaxTplFile+1), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(fx.cfg).SpreadTpl(it.ID); err != ErrNotFound {
		t.Errorf("SpreadTpl наружу: %v, ждали ErrNotFound", err)
	}
	fx.allKept(t, "велик")
	// Не обычный файл на месте шаблона — тоже «не прочитан».
	fx = reinstall(t, fx)
	it, err = fx.s.SaveSpreadTpl("mine", "Папка", model.SpreadTpl{V: 1, Pages: 1, Places: box, Page: []model.TplBg{{Color: "#aabbcd"}}})
	if err != nil {
		t.Fatal(err)
	}
	path = fx.s.files[KindSpread][it.ID]
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatal(err)
	}
	fx.allKept(t, "не обычный файл")
}

// Файл шаблона стёрт между индексом и чтением: шаблона нет, держать нечего — картинки уходят, как раньше.
func TestRemovePackErasedTplDoesNotHold(t *testing.T) {
	fx := newRawFixture(t)
	box := []model.Box{{X: .1, Y: .1, W: .8, H: .8}}
	it, err := fx.s.SaveSpreadTpl("mine", "Стёртый", model.SpreadTpl{V: 1, Pages: 1, Places: box, Page: []model.TplBg{{Color: "#aabbce"}}})
	if err != nil {
		t.Fatal(err)
	}
	s := Open(fx.cfg)
	if err := os.Remove(s.files[KindSpread][it.ID]); err != nil {
		t.Fatal(err)
	}
	res, err := s.RemovePack("ivory", nil)
	if err != nil || res.Files != 4 {
		t.Errorf("итог %+v, %v — ждали удаления всех четырёх", res, err)
	}
}

// noReadDir закрывает папку на перечисление (x без r) и возвращает false, если это не сработало
// (Windows, root).
func noReadDir(t *testing.T, dir string) bool {
	t.Helper()
	if runtime.GOOS == "windows" {
		return false
	}
	if err := os.Chmod(dir, 0o100); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	if _, err := os.ReadDir(dir); err == nil {
		return false
	}
	return true
}

// Папку не перечислить (не «её нет») — незнакомое не прочитано, держим всё: корень, незнакомая
// папка, Наборы/.
func TestRemovePackReadDirErrorKeepsAll(t *testing.T) {
	for _, rel := range []string{"", "Макеты", "Наборы"} {
		fx := newRawFixture(t)
		fx.write(t, "Макеты/0000000000000001.json", `{}`)
		s := Open(fx.cfg)
		if !noReadDir(t, filepath.Join(fx.root, rel)) {
			t.Skip("права на папки не действуют")
		}
		plan, err := s.PlanRemovePack("ivory", nil)
		if err != nil {
			t.Fatalf("%q: %v", rel, err)
		}
		if plan.Files != 0 || plan.Kept != 4 {
			t.Errorf("%q: план %+v — ждали «держим всё»", rel, plan)
		}
	}
}

// Общий бюджет байт: ровно по предел — читается, на байт меньше — не всё прочитано.
func TestUnknownRefsTotalBudget(t *testing.T) {
	fx := newRawFixture(t)
	fx.write(t, "Макеты/0000000000000001.json", `{"a":"`+fx.key[0]+`"}`)
	fx.write(t, "Макеты/0000000000000002.json", `{"a":"`+fx.key[1]+`"}`)
	old := rawTotalMax
	t.Cleanup(func() { rawTotalMax = old })
	s := Open(fx.cfg)
	full := s.unknownRefs(nil)
	if !full.complete || full.total < 2 {
		t.Fatalf("без сужения: complete=%v total=%d", full.complete, full.total)
	}
	rawTotalMax = full.total
	if r := s.unknownRefs(nil); !r.complete {
		t.Error("ровно по предел — должно читаться")
	}
	rawTotalMax = full.total - 1
	if r := s.unknownRefs(nil); r.complete {
		t.Error("на байт больше предела — complete должен быть false")
	}
}

// collect без предметов не обходит диск; при незавершённом чтении пишет одну строку без пути.
func TestCollectLogsIncompleteOnce(t *testing.T) {
	fx := newRawFixture(t)
	if err := os.Symlink(testkit.TempDir(t), filepath.Join(fx.root, "Чужое")); err != nil {
		t.Skip("симлинк недоступен:", err)
	}
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	s := Open(fx.cfg)
	s.collect(nil, nil, nil, true)
	if buf.Len() != 0 {
		t.Errorf("пустой collect обходил диск: %q", buf.String())
	}
	s.collect(s.packOf("ivory").Items, nil, nil, true)
	out := buf.String()
	if strings.Count(out, "\n") != 1 || !strings.Contains(out, "сборка мусора ничего не удаляет") || strings.Contains(out, fx.root) {
		t.Errorf("журнал: %q", out)
	}
}

// 1 МБ разных ключей разбирается за линейное время (раньше — квадратичная проверка повторов).
func TestRawLibKeysManyUniqueFast(t *testing.T) {
	var b strings.Builder
	const n = 50000
	for i := range n {
		fmt.Fprintf(&b, "my:%016x,", i)
	}
	data := []byte(b.String() + b.String()) // повторы тоже
	start := time.Now()
	got := rawLibKeys(data)
	if len(got) != n {
		t.Fatalf("ключей %d, ждали %d", len(got), n)
	}
	if d, limit := time.Since(start), testkit.Limit(400*time.Millisecond); d > limit {
		t.Errorf("разбор %d КБ: %v > %v", len(data)>>10, d, limit)
	}
}
