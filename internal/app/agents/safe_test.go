package agents

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"unicode"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// Меры Б1 для инструментов чтения (security.md §2): только чтение, уровни, пути, чистка текста,
// паника, пределы.

// TestHostFields: список полей Host — замок. Новое поле — правка этого списка, а значит ревью
// безопасности: через Host служба получает доступ ко всему App.
func TestHostFields(t *testing.T) {
	want := []string{
		"CfgDir func() string", "Version func() string", "Sock func() string", "Changed func()",
		"View func() (model.View, uint64)", "Photos func() []library.Photo", "Marks func() map[string]model.MarkView",
		"Status func() ([]string, map[string]string)", "Decor func(string) (agents.DecorCatalogLite, error)",
		"Packs func() (agents.PacksLite, error)", "Prints func() []printprof.Info", "Recent func() []prefs.RecentProject",
		"FontsMissing func([]model.FontUse) []string",
		"Preview func(context.Context, string, int) ([]uint8, error)", "Render func(context.Context, string, int) ([]uint8, error)",
		// Ступень 3 (149): правки; всё — числа, строки и срезы строк (TestHostTypesScalar).
		"Perm func(string) (string, string)", "PermLevel func(string) (string, string, int)", "Scanning func() bool", "Busy func(context.Context) bool", "Backup func(string) error",
		"Ask func(context.Context, agents.AskReq) (agents.AskRes, error)", "Auto func(context.Context, agents.AutoReq) (agents.AutoRes, error)",
		"Vary func(context.Context, agents.VaryReq) (agents.VaryRes, error)", "Show func(string, string) (int, error)",
		"Undo func(context.Context, agents.AlbumRef) (agents.AlbumRef, error)",
		// Ступень 3б (151): лист-сетка, семейство и правки 0.34; то же правило типов.
		"Sheet func(context.Context, agents.SheetReq) ([]uint8, []string, error)", "Families func([]string) []string",
		"Mark func(context.Context, agents.MarkReq) (agents.MarkRes, error)", "Cover func(context.Context, agents.CoverReq) (agents.CoverRes, error)",
		"Bg func(context.Context, agents.BgReq) (agents.BgRes, error)", "Stickers func(context.Context, agents.StickerReq) (agents.StickerRes, error)",
		"Print func(context.Context, agents.PrintReq) (agents.PrintRes, error)",
		// 152: ход выгрузки для get_album.
		"ExportState func(uint64) agents.ExportStatus", "ExportHold func(string) bool", // ExportHold: имя инструмента → bool, путей нет
	}
	rt := reflect.TypeOf(Host{})
	var got []string
	for i := range rt.NumField() {
		got = append(got, rt.Field(i).Name+" "+rt.Field(i).Type.String())
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("поля Host изменились — нужна правка списка и ревью безопасности:\n got %q\nwant %q", got, want)
	}
}

// TestHostTypesScalar: через Host правок идут только числа, bool, строки и срезы строк — ни
// указателя, ни замыкания, ни карты, ни интерфейса; полей с Dir и Path в имени нет (путь агента — не
// путь программы). Вложенные структуры — только такие же.
func TestHostTypesScalar(t *testing.T) {
	var check func(path string, rt reflect.Type)
	check = func(path string, rt reflect.Type) {
		switch rt.Kind() {
		case reflect.Bool, reflect.Int, reflect.Int64, reflect.Uint64, reflect.Float64, reflect.String:
		case reflect.Slice:
			if rt.Elem().Kind() != reflect.String {
				t.Errorf("%s: срез не из строк: %v", path, rt)
			}
		case reflect.Struct:
			for i := range rt.NumField() {
				f := rt.Field(i)
				if n := strings.ToLower(f.Name); strings.Contains(n, "dir") || strings.Contains(n, "path") {
					t.Errorf("%s.%s: путь в имени поля", path, f.Name)
				}
				check(path+"."+f.Name, f.Type)
			}
		default:
			t.Errorf("%s: недопустимый вид %v", path, rt)
		}
	}
	for _, v := range []any{AskReq{}, AskRes{}, AutoReq{}, AutoRes{}, VaryReq{}, VaryRes{}, AlbumRef{},
		SheetReq{}, MarkReq{}, MarkRes{}, CoverReq{}, CoverRes{}, BgReq{}, BgRes{}, StickerReq{}, StickerRes{}, PrintReq{}, PrintRes{}, ExportStatus{}} {
		rt := reflect.TypeOf(v)
		check(rt.Name(), rt)
	}
	// Вызов агента не может нести «Album» и «Again» окна: фасад ставит их сам (М10).
	for _, bad := range []string{"Album", "Again"} {
		if _, ok := reflect.TypeOf(AutoReq{}).FieldByName(bad); ok {
			t.Errorf("у AutoReq есть поле %s", bad)
		}
	}
}

// snapshot — всё, что инструменты могли бы изменить: вид, фото, отметки, config.json, файлы папки.
func snapshot(t *testing.T, r *rig) string {
	t.Helper()
	r.fx.mu.Lock()
	defer r.fx.mu.Unlock()
	b, err := json.Marshal(map[string]any{"rev": r.fx.view.Rev, "dirty": r.fx.view.Dirty, "view": r.fx.view, "photos": r.fx.photos,
		"marks": r.fx.marks, "missing": r.fx.missing, "unread": r.fx.unread, "decor": r.fx.decor, "packs": r.fx.packs,
		"prints": r.fx.prints, "recent": r.fx.recent})
	if err != nil {
		t.Fatal(err)
	}
	var files []string
	_ = filepath.WalkDir(r.cfg, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			fi, _ := d.Info()
			data, _ := os.ReadFile(p)
			files = append(files, p+"|"+fi.ModTime().String()+"|"+string(data))
		}
		return nil
	})
	return string(b) + "\n" + strings.Join(files, "\n")
}

// TestToolsReadOnly: все инструменты по всем веткам входа на всех уровнях не меняют ни вид (rev,
// dirty), ни данные Host, ни config.json и файлы папки данных; службе не отправляют состояний.
func TestToolsReadOnly(t *testing.T) {
	for _, lv := range []int{1, 2, 3} {
		r, cs := toolsRig(t, lv, Options{})
		waitFor(t, "клиент записан", func() bool { return len(r.svc.Clients()) == 1 })
		before, events := snapshot(t, r), r.changed.Load()
		for _, c := range allCalls() {
			tcall(t, cs, c.name, c.args)
		}
		if n := r.fx.edit.writes(); n != 0 {
			t.Fatalf("уровень %d: чтение дошло до замыканий правок (%d)", lv, n)
		}
		for _, c := range []struct {
			n string
			a map[string]any
		}{{"get_photo", map[string]any{"id": "ffff"}}, {"get_spread", map[string]any{"spread": 99}}, {"get_photo_preview", map[string]any{"id": idA}},
			{"get_spread_preview", map[string]any{"spread": 1}}, {"list_decor", map[string]any{"kind": "spread", "inAlbum": true}}} {
			tcall(t, cs, c.n, c.a)
		}
		r.fx.mu.Lock()
		r.fx.ed = 0
		r.fx.mu.Unlock()
		for _, c := range allCalls() {
			tcall(t, cs, c.name, c.args)
		}
		r.fx.mu.Lock()
		r.fx.ed = 7
		r.fx.mu.Unlock()
		if after := snapshot(t, r); after != before {
			t.Fatalf("уровень %d: данные или файлы изменились", lv)
		}
		if got := r.changed.Load(); got != events {
			t.Fatalf("уровень %d: события состояния (%d → %d) от чтения", lv, events, got)
		}
	}
}

// outputs — ответы всех инструментов на всех ветках входа, одной строкой.
func outputs(t *testing.T, call func(string, map[string]any) reply) string {
	t.Helper()
	var b strings.Builder
	for _, c := range allCalls() {
		b.WriteString(call(c.name, c.args).Text + "\n")
	}
	b.WriteString(call("get_photo", map[string]any{"id": idE}).Text)
	b.WriteString(call("list_photos", map[string]any{"order": "name"}).Text)
	b.WriteString(call("get_photo_preview", map[string]any{"id": idA}).Text)
	b.WriteString(call("get_spread_preview", map[string]any{"spread": 1}).Text)
	return b.String()
}

// TestLevelGates: методом меток — каждая строка человека несёт ZQ<n>. Уровень 1: ни одной метки
// ни в одном ответе, вместо своего — «Свой фон N»; уровень 2 и 3: все; «Можно скачать» и таблица
// имён — нигде.
func TestLevelGates(t *testing.T) {
	for _, lv := range []int{1, 2, 3} {
		_, cs := toolsRig(t, lv, Options{})
		all := outputs(t, func(n string, a map[string]any) reply { return tcall(t, cs, n, a) })
		for _, never := range []string{"ZQ14", namesMark} {
			if strings.Contains(all, never) {
				t.Fatalf("уровень %d: %s вышла наружу (образец «Можно скачать» или таблица имён)", lv, never)
			}
		}
		if lv > 1 && !strings.Contains(all, "{Фамилия} {Имя}") {
			t.Errorf("уровень %d: подпись с полями имени должна идти шаблоном", lv)
		}
		for _, m := range levelMarks {
			if has := strings.Contains(all, m); lv == 1 && has {
				t.Errorf("уровень 1: метка %s вышла наружу", m)
			} else if lv > 1 && !has {
				t.Errorf("уровень %d: метки %s нет ни в одном ответе (проекция потеряла поле?)", lv, m)
			}
		}
		if lv == 1 {
			wantAll(t, all, "Свой стикер 1", "Свой фон 1", "Свой фон 2", "Мой макет 1", "Мой шаблон 1", "Набор из файла 1",
				"Набор из файла 2", "Свой профиль 1", "Свой профиль 2", ownPrintID("own1"), "Альбом")
		}
		wantPreviewGate(t, cs, lv)
	}
}

func wantAll(t *testing.T, all string, words ...string) {
	t.Helper()
	for _, w := range words {
		if !strings.Contains(all, w) {
			t.Errorf("уровень 1: нет замены %q", w)
		}
	}
}

// wantPreviewGate: картинки — только с уровня 3.
func wantPreviewGate(t *testing.T, cs *mcp.ClientSession, lv int) {
	t.Helper()
	for _, c := range []struct {
		n string
		a map[string]any
	}{{"get_photo_preview", map[string]any{"id": idA}}, {"get_spread_preview", map[string]any{"spread": 1}}} {
		res := pcall(t, cs, c.n, c.a)
		if has := imageOf(res) != nil; has != (lv == 3) || res.IsError == (lv == 3) ||
			lv < 3 && !strings.Contains(textOf(res), "Картинки агенту выключены") {
			t.Errorf("%s на уровне %d: %+v", c.n, lv, res)
		}
	}
}

// TestLevelSwitch: уровень берётся на каждый вызов — смена в config.json действует сразу.
func TestLevelSwitch(t *testing.T) {
	r, cs := toolsRig(t, 1, Options{})
	if got := tcall(t, cs, "get_album", nil).Data["name"]; got != "Альбом" {
		t.Fatalf("уровень 1: %v", got)
	}
	setLevel(r.cfg, 2)
	if got := tcall(t, cs, "get_album", nil).Data["name"]; got != "ZQ5_wedding" {
		t.Fatalf("после смены на 2: %v", got)
	}
	setLevel(r.cfg, 1)
	if got := tcall(t, cs, "get_album", nil).Data["name"]; got != "Альбом" {
		t.Fatalf("после возврата на 1: %v", got)
	}
}

// TestNoPathsOut: пути не выходят ни в данных, ни в ошибках: ошибки Host с путём и паника —
// «внутренняя ошибка программы», в log.txt — без пути.
func TestNoPathsOut(t *testing.T) {
	var logs bytes.Buffer
	log.SetOutput(&logs)
	defer log.SetOutput(os.Stderr)
	const secret = "/home/secretuser/albums"
	for _, lv := range []int{1, 2, 3} {
		r, cs := toolsRig(t, lv, Options{})
		all := outputs(t, func(n string, a map[string]any) reply { return tcall(t, cs, n, a) })
		if strings.Contains(all, string(filepath.Separator)+"albums") || strings.Contains(all, "ZQ2dir"+string(filepath.Separator)) || strings.Contains(all, "salbum\"") {
			t.Fatalf("уровень %d: путь в ответах", lv)
		}
		r.fx.mu.Lock()
		r.fx.decorErr = &fs.PathError{Op: "open", Path: secret + "/x", Err: fs.ErrPermission}
		r.fx.packsErr = &fs.PathError{Op: "stat", Path: secret, Err: fs.ErrNotExist}
		r.fx.mu.Unlock()
		for _, c := range []struct {
			n string
			a map[string]any
		}{{"list_decor", map[string]any{"kind": "bg"}}, {"list_packs", nil}, {"list_decor", map[string]any{"kind": "sticker"}}} {
			rep := tcall(t, cs, c.n, c.a)
			if !rep.IsError || rep.Text != "внутренняя ошибка программы" {
				t.Fatalf("%s: %+v", c.n, rep)
			}
		}
	}
	if strings.Contains(logs.String(), secret) {
		t.Fatalf("путь в log.txt: %s", logs.String())
	}
}

// TestTextHygiene: управляющие, невидимые теги, bidi и нулевой ширины вычищены, длина обрезана.
func TestTextHygiene(t *testing.T) {
	dirty := "A\u202eB\u200bC\U000E0041D\x00E\x1bF\u2066G\ufeffH\r\nК\u2028L"
	for _, c := range []struct {
		in, want string
		limit    int
	}{
		{dirty, "ABCDEFGH\nК\nL", 100},
		{"", "", 5},
		{"абвгдеж", "абвгд…", 5},
		{"\xff\xfeok", "ok", 10},
		{"x\ufe0fy\U000E0100z\ue000w\u3164v", "xyzwv", 10},
		{strings.Repeat("я", maxText+500), strings.Repeat("я", maxText) + "…", maxText},
	} {
		if got := cleanText(c.in, c.limit); got != c.want {
			t.Errorf("cleanText(%q, %d) = %q, ждали %q", c.in, c.limit, got, c.want)
		}
	}
	r, cs := toolsRig(t, 2, Options{})
	r.fx.mu.Lock()
	r.fx.view.Spreads[0].Texts[0].Text = dirty
	r.fx.view.Spreads[0].Name = dirty
	r.fx.photos[0].Name = dirty
	r.fx.mu.Unlock()
	r.fx.mu.Lock()
	r.fx.photos[1].Name = "a\nb\u00a0c.jpg"
	r.fx.mu.Unlock()
	for _, a := range []struct {
		n string
		m map[string]any
	}{{"get_spread", map[string]any{"spread": 1}}, {"list_spreads", nil}, {"list_photos", nil}} {
		rep := tcall(t, cs, a.n, a.m)
		var v any
		if err := json.Unmarshal([]byte(rep.Text), &v); err != nil {
			t.Fatal(err)
		}
		for _, s := range jsonStrings(v) {
			for _, r := range s {
				if r != '\n' && (unicode.IsControl(r) || unicode.Is(unicode.Cf, r) || hiddenRune(r) || r == '\uFFFD') {
					t.Errorf("%s: в строке %q остался знак %U", a.n, s, r)
				}
			}
		}
		if a.n == "get_spread" && !strings.Contains(rep.Text, "ABCDEFGH") {
			t.Errorf("текст пропал: %s", rep.Text)
		}
		if a.n == "list_photos" && !strings.Contains(rep.Text, `"a b\u00a0c.jpg"`) && !strings.Contains(rep.Text, "a b\u00a0c.jpg") && !strings.Contains(rep.Text, "a b c.jpg") {
			t.Errorf("перенос в имени файла не заменён пробелом: %s", rep.Text)
		}
	}
}

// jsonStrings — все строковые значения и ключи разобранного JSON.
func jsonStrings(v any) []string {
	switch x := v.(type) {
	case string:
		return []string{x}
	case []any:
		var out []string
		for _, e := range x {
			out = append(out, jsonStrings(e)...)
		}
		return out
	case map[string]any:
		var out []string
		for k, e := range x {
			out = append(out, k)
			out = append(out, jsonStrings(e)...)
		}
		return out
	}
	return nil
}

// TestLayoutKeyAndColor: ключ раскладки идёт как есть (грамматика ключа, до layout.MaxKeyLen), мусор —
// пустым; цвет фона — #rrggbb, одинаково в list_spreads и get_spread.
func TestLayoutKeyAndColor(t *testing.T) {
	r, cs := toolsRig(t, 1, Options{})
	const key = "P[E(0)|V(H(1,2),H(3,4),H(5,6,7))]"
	r.fx.mu.Lock()
	r.fx.view.Spreads[1].Layout = key
	r.fx.mu.Unlock()
	item := func(n int) map[string]any {
		return tcall(t, cs, "list_spreads", nil).Data["items"].([]any)[n].(map[string]any)
	}
	if got := item(1)["layout"]; got != key {
		t.Fatalf("ключ раскладки: %v", got)
	}
	for _, bad := range []string{"../etc", "P[E(0)]\n", strings.Repeat("H", layout.MaxKeyLen+1)} {
		r.fx.mu.Lock()
		r.fx.view.Spreads[1].Layout = bad
		r.fx.mu.Unlock()
		if got := item(1)["layout"]; got != "" {
			t.Errorf("ключ %.20q прошёл: %v", bad, got)
		}
	}
	if bg := item(0)["bg"].([]any); bg[0] != "#ffffff" || bg[1] != "#eeeeee" {
		t.Fatalf("list_spreads.bg: %v", bg)
	}
	var colors []any
	for _, b := range tcall(t, cs, "get_spread", map[string]any{"spread": 1}).Data["backgrounds"].([]any) {
		if b.(map[string]any)["kind"] == "color" {
			colors = append(colors, b.(map[string]any)["color"])
		}
	}
	if len(colors) != 2 || colors[0] != "#ffffff" || colors[1] != "#eeeeee" {
		t.Fatalf("get_spread.backgrounds: %v", colors)
	}
}

// TestRecentKeySalted: ключ «Последнего» посолен процессом — не совпадает с голым sha256 пути, по
// которому догадку о пути можно проверить словарём.
func TestRecentKeySalted(t *testing.T) {
	r, cs := toolsRig(t, 1, Options{})
	items := tcall(t, cs, "list_albums", nil).Data["items"].([]any)
	plain := sha256.Sum256([]byte(paths.Fold(filepath.Clean(r.fx.recent[0].Path))))
	for _, it := range items {
		k := it.(map[string]any)["key"].(string)
		if len(k) != 12 || k == hex.EncodeToString(plain[:6]) {
			t.Fatalf("ключ %q не посолен", k)
		}
	}
	if len(items) != 2 || items[0].(map[string]any)["open"] != true || items[1].(map[string]any)["open"] != false {
		t.Fatalf("open: %v", items)
	}
}
