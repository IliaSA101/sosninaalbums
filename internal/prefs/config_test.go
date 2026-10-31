package prefs

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

func TestPickDir(t *testing.T) {
	root := filepath.Clean(testkit.TempDir(t))
	docs := filepath.Join(root, "Documents")
	last := filepath.Join(root, "съёмка", "день1")
	for _, d := range []string{docs, last} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	gone := filepath.Join(last, "удалена", "глубже")
	file := filepath.Join(root, "f.txt")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name, last, fallback, want string
	}{
		{"последняя есть", last, docs, last},
		{"нет папки — ближайший родитель", gone, docs, last},
		{"файл вместо папки — родитель", filepath.Join(file, "x"), docs, root},
		{"нет последней — Документы", "", docs, docs},
		{"относительный путь не берём", "съёмка", docs, docs},
		{"нет и Документов", "", filepath.Join(root, "нет"), ""},
	}
	for _, c := range cases {
		if got := PickDir(c.last, c.fallback); got != c.want {
			t.Errorf("%s: получили %q, ждали %q", c.name, got, c.want)
		}
	}
	for _, bad := range []string{`\server\share\x`, `\?\C:\x`, `\??\C:\x`} {
		if got := PickDir(bad, docs); got != docs {
			t.Errorf("нелокальный %q принят: %q", bad, got)
		}
	}
}

// Настройки не затирают друг друга; битый файл игнорируется; нелокальная папка не пишется.
func TestRememberDir(t *testing.T) {
	dir := testkit.TempDir(t)
	d1, d2 := filepath.Clean(testkit.TempDir(t)), filepath.Clean(testkit.TempDir(t))
	RememberDir(dir, DirProject, d1)
	UpdateConfig(dir, func(c *Config) { c.LastProject = "p.salbum" })
	RememberDir(dir, DirExport, d2)
	RememberDir(dir, DirPhotos, `\server\share`)
	c := LoadConfig(dir)
	if c.LastProject != "p.salbum" || c.LastDirs[DirProject] != d1 || c.LastDirs[DirExport] != d2 || len(c.LastDirs) != 2 {
		t.Errorf("конфиг: %+v", c)
	}
	if got := DialogDir(dir, DirProject); got != d1 {
		t.Errorf("DialogDir=%q, ждали %q", got, d1)
	}
	if err := os.WriteFile(ConfigPath(dir), []byte("{не json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if c := LoadConfig(dir); c.LastProject != "" || len(c.LastDirs) != 0 {
		t.Errorf("битый конфиг: %+v", c)
	}
	RememberDir(dir, DirExport, d2) // после битого файла запись снова работает
	if LoadConfig(dir).LastDirs[DirExport] != d2 {
		t.Error("запись после битого конфига не прошла")
	}
}

// Корень выгрузок (0.35/152-15): свой ключ lastDirs, не мешает папке окна экспорта и чужим ключам;
// круг запись → чтение; конфиг до 0.35 (ключа нет) отдаёт последнюю папку экспорта; нелокальный
// корень не пишется; пустой конфиг — "".
func TestRememberExportRoot(t *testing.T) {
	dir := testkit.TempDir(t)
	if got := ExportRoot(LoadConfig(dir)); got != "" {
		t.Fatalf("пустой конфиг: корень %q", got)
	}
	exp, root, other := filepath.Clean(testkit.TempDir(t)), filepath.Clean(testkit.TempDir(t)), filepath.Clean(testkit.TempDir(t))
	// Конфиг 0.34: есть папка окна и незнакомый ключ, корня нет.
	raw := `{"lastDirs": {"export": ` + strconv.Quote(exp) + `, "future": ` + strconv.Quote(other) + `}}`
	if err := os.WriteFile(ConfigPath(dir), []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := ExportRoot(LoadConfig(dir)); got != exp {
		t.Fatalf("корня нет — последняя папка экспорта: %q, ждали %q", got, exp)
	}
	RememberDir(dir, DirExportRoot, root)
	c := LoadConfig(dir)
	if ExportRoot(c) != root || c.LastDirs[DirExport] != exp || c.LastDirs["future"] != other || len(c.LastDirs) != 3 {
		t.Fatalf("после записи корня: %+v", c.LastDirs)
	}
	// Папка окна меняется отдельно, корень остаётся; нелокальный корень не затирает годный.
	RememberDir(dir, DirExport, other)
	RememberDir(dir, DirExportRoot, `\\server\share\x`)
	RememberDir(dir, DirExportRoot, "")
	c = LoadConfig(dir)
	if ExportRoot(c) != root || c.LastDirs[DirExport] != other {
		t.Fatalf("корень и папка окна независимы: %+v", c.LastDirs)
	}
	if DirExportRoot != "exportRoot" {
		t.Fatalf("ключ в config.json: %q", DirExportRoot)
	}
}

// Свои клавиши (074): круг записи и чтения нескольких команд, пустой список («без клавиши»)
// остаётся пустым списком, а не исчезает; пустой набор — ключа в файле нет.
func TestConfigHotkeysRoundTrip(t *testing.T) {
	dir := testkit.TempDir(t)
	if c := LoadConfig(dir); c.Hotkeys != nil {
		t.Fatalf("без файла: %v", c.Hotkeys)
	}
	want := map[string][]string{"toggleBig": {"KeyB", "Mod+KeyB"}, "toggleSafe": {}, "redo": {"Mod+KeyY"}}
	UpdateConfig(dir, func(c *Config) { c.Hotkeys = want })
	got := LoadConfig(dir).Hotkeys
	if len(got) != 3 || !slices.Equal(got["toggleBig"], want["toggleBig"]) || !slices.Equal(got["redo"], want["redo"]) {
		t.Fatalf("прочитано %v", got)
	}
	if l, ok := got["toggleSafe"]; !ok || l == nil || len(l) != 0 {
		t.Errorf("«без клавиши» потеряно: %v", got)
	}
	data, _ := os.ReadFile(ConfigPath(dir))
	if !strings.Contains(string(data), `"toggleSafe": []`) {
		t.Errorf("пустой список записан не как []: %s", data)
	}
	UpdateConfig(dir, func(c *Config) { c.Hotkeys = map[string][]string{} })
	if data, _ := os.ReadFile(ConfigPath(dir)); strings.Contains(string(data), "hotkeys") {
		t.Errorf("пустой набор оставил ключ: %s", data)
	}
}

// Чужие ключи config.json и незнакомые id в hotkeys (от новой версии) переживают правку
// другого поля: prefs id не толкует.
func TestConfigHotkeysForeignKept(t *testing.T) {
	dir := testkit.TempDir(t)
	raw := `{"future":{"x":1},"level":"full","hotkeys":{"futureCmd":["KeyQ"],"toggleBig":["KeyB"],"bad id":["?"]}}`
	if err := os.WriteFile(ConfigPath(dir), []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	UpdateConfig(dir, func(c *Config) { c.LastProject = "p.salbum" })
	c := LoadConfig(dir)
	if len(c.Hotkeys) != 3 || !slices.Equal(c.Hotkeys["futureCmd"], []string{"KeyQ"}) || !slices.Equal(c.Hotkeys["bad id"], []string{"?"}) {
		t.Errorf("hotkeys после правки другого поля: %v", c.Hotkeys)
	}
	if data, _ := os.ReadFile(ConfigPath(dir)); !strings.Contains(string(data), `"future": {`) || c.Level != "full" {
		t.Errorf("чужой ключ потерян: %s", data)
	}
}

// TestRecentDecor — недавние плитки библиотеки: наверх без повторов, предел 12, мусор из файла
// отбрасывается, виды раздельно, круг записи и чтения config.json; чужие ключи целы.
func TestRecentDecor(t *testing.T) {
	var list []string
	for i := range 14 {
		list = PushRecentDecor(list, fmt.Sprintf("sa:p%d", i))
	}
	list = PushRecentDecor(list, "sa:p5")
	if len(list) != maxRecentDecor || list[0] != "sa:p5" || list[1] != "sa:p13" || slices.Index(list[1:], "sa:p5") >= 0 {
		t.Fatalf("список %v", list)
	}
	for _, bad := range []string{"", "sa:", "sa:Узор", "my:0123", "my:0123456789ABCDEF", `C:\x.jpg`, "sa:" + strings.Repeat("a", 49)} {
		if got := PushRecentDecor([]string{"sa:a"}, bad); !slices.Equal(got, []string{"sa:a"}) {
			t.Errorf("%q: %v", bad, got)
		}
	}

	dir := testkit.TempDir(t)
	if err := os.WriteFile(ConfigPath(dir), []byte(`{"future":1,"recentDecor":{"bg":["sa:old","мусор","sa:old"]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := RecentDecor(dir, "bg"); !slices.Equal(got, []string{"sa:old"}) {
		t.Fatalf("из файла %v", got)
	}
	RememberDecor(dir, "bg", "my:0123456789abcdef")
	RememberDecor(dir, "sticker", "sa:heart")
	RememberDecor(dir, "bg", "../x")   // мусор не пишется
	RememberDecor(dir, "Фоны", "sa:x") // вид не по грамматике
	if got := RecentDecor(dir, "bg"); !slices.Equal(got, []string{"my:0123456789abcdef", "sa:old"}) {
		t.Errorf("фоны %v", got)
	}
	if got := RecentDecor(dir, "sticker"); !slices.Equal(got, []string{"sa:heart"}) {
		t.Errorf("стикеры %v", got)
	}
	if got := RecentDecor(dir, ""); got == nil || len(got) != 0 {
		t.Errorf("пустой вид %v", got)
	}
	data, _ := os.ReadFile(ConfigPath(dir))
	if !strings.Contains(string(data), `"future": 1`) || strings.Contains(string(data), "Фоны") {
		t.Errorf("config.json: %s", data)
	}
}

// config.json больше предела читается как битый (пустой конфиг), а правка его не тянет в память:
// переписывается заново без старого содержимого.
func TestConfigTooBig(t *testing.T) {
	dir := testkit.TempDir(t)
	small := `{"level":"full","lastProject":"p.salbum"}`
	if err := os.WriteFile(ConfigPath(dir), []byte(small), 0o644); err != nil {
		t.Fatal(err)
	}
	if c := LoadConfig(dir); c.Level != "full" {
		t.Fatalf("малый файл не прочитан: %+v", c)
	}
	// Ровно на пределе — читается; на байт больше — как битый.
	pad := func(n int) []byte {
		b := []byte(small[:len(small)-1] + `,"x":"`)
		b = append(b, strings.Repeat("a", n-len(b)-2)...)
		return append(b, '"', '}')
	}
	if err := os.WriteFile(ConfigPath(dir), pad(maxConfigSize), 0o644); err != nil {
		t.Fatal(err)
	}
	if c := LoadConfig(dir); c.Level != "full" {
		t.Errorf("файл ровно в предел не прочитан: %+v", c)
	}
	if err := os.WriteFile(ConfigPath(dir), pad(maxConfigSize+1), 0o644); err != nil {
		t.Fatal(err)
	}
	if c := LoadConfig(dir); c.Level != "" || c.LastProject != "" {
		t.Errorf("файл больше предела прочитан: %+v", c)
	}
	UpdateConfig(dir, func(c *Config) { c.LastProject = "q.salbum" })
	if data, _ := os.ReadFile(ConfigPath(dir)); len(data) > 1<<10 || LoadConfig(dir).LastProject != "q.salbum" {
		t.Errorf("после правки: %d байт, %+v", len(data), LoadConfig(dir))
	}
}
