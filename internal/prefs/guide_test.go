package prefs

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

func writeCfg(t *testing.T, dir, raw string) {
	t.Helper()
	if err := os.WriteFile(ConfigPath(dir), []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
}

// Чистка: мусор и повторы отбрасываются, порядок цел, предел maxLearned, результат не nil.
func TestGuideLearnedClean(t *testing.T) {
	long := strings.Repeat("a", maxLearnedID)
	got := CleanLearned([]string{"a", "", "A", "a b", "-x", "b-", "a", "ключ", "x/y", "..", "b1", long, long + "a", "a\n"})
	if want := []string{"a", "b-", "b1", long}; !slices.Equal(got, want) {
		t.Errorf("чистка: %q, ждали %q", got, want)
	}
	if got := CleanLearned(nil); got == nil || len(got) != 0 {
		t.Errorf("nil → %#v, ждали пустой не nil", got)
	}
	// Предел: 600 разных годных — остаётся первые 500.
	many := make([]string, 600)
	for i := range many {
		many[i] = fmt.Sprintf("id%d", i)
	}
	if got := CleanLearned(many); len(got) != maxLearned || got[0] != "id0" || got[maxLearned-1] != "id499" {
		t.Errorf("предел: %d, %q…%q", len(got), got[0], got[len(got)-1])
	}
	// Из файла: пустой, битый, не тот тип, «null» — пустой список, остальное читается.
	for name, raw := range map[string]string{
		"нет ключа": `{"level":"full"}`, "null": `null`, "битый": `{"guideLearned":`, "пустой": ``,
		"не список": `{"guideLearned":"a"}`, "числа": `{"guideLearned":[1,2]}`, "пустой список": `{"guideLearned":[]}`,
	} {
		dir := testkit.TempDir(t)
		writeCfg(t, dir, raw)
		if got := GuideLearned(dir); got == nil || len(got) != 0 {
			t.Errorf("%s: %#v", name, got)
		}
	}
	dir := testkit.TempDir(t)
	writeCfg(t, dir, `{"guideLearned":["ok","BAD","ok","two-2"]}`)
	if got := GuideLearned(dir); !slices.Equal(got, []string{"ok", "two-2"}) {
		t.Errorf("из файла: %q", got)
	}
	// Файл без конфига вообще.
	if got := GuideLearned(testkit.TempDir(t)); got == nil || len(got) != 0 {
		t.Errorf("нет файла: %#v", got)
	}
}

// MarkLearned: дописывает недостающие в конец, снимает, негодные и повторы пропускает, исходный
// список не меняет, предел не обходится.
func TestMarkLearned(t *testing.T) {
	base := []string{"a", "b"}
	if got := MarkLearned(base, []string{"c", "a", "BAD", "d", "c"}, true); !slices.Equal(got, []string{"a", "b", "c", "d"}) {
		t.Errorf("поставить несколько: %q", got)
	}
	if !slices.Equal(base, []string{"a", "b"}) {
		t.Errorf("исходный список изменён: %q", base)
	}
	if got := MarkLearned(base, []string{"a", "zzz", "BAD"}, false); !slices.Equal(got, []string{"b"}) {
		t.Errorf("снять: %q", got)
	}
	if got := MarkLearned(base, []string{"a", "b"}, false); got == nil || len(got) != 0 {
		t.Errorf("снять всё: %#v", got)
	}
	if got := MarkLearned(nil, nil, true); got == nil || len(got) != 0 {
		t.Errorf("пустое: %#v", got)
	}
	// Сверх предела новый id не дописывается, существующий снимается.
	full := make([]string, maxLearned)
	for i := range full {
		full[i] = fmt.Sprintf("id%d", i)
	}
	if got := MarkLearned(full, []string{"extra", "id0"}, true); len(got) != maxLearned || slices.Contains(got, "extra") {
		t.Errorf("предел: %d", len(got))
	}
	if got := MarkLearned(full, []string{"id0"}, false); len(got) != maxLearned-1 || got[0] != "id1" {
		t.Errorf("снять при полном: %d", len(got))
	}
}

// Круг запись → чтение: порядок «как ставили», повтор и неизвестный id, снятие, пустой список
// убирает ключ из файла.
func TestMarkGuideRoundTrip(t *testing.T) {
	dir := testkit.TempDir(t)
	if got := MarkGuide(dir, []string{"b", "a"}, true); !slices.Equal(got, []string{"b", "a"}) {
		t.Fatalf("первая запись: %q", got)
	}
	if got := MarkGuide(dir, []string{"a", "c"}, true); !slices.Equal(got, []string{"b", "a", "c"}) {
		t.Fatalf("повтор + новый: %q", got)
	}
	if got := GuideLearned(dir); !slices.Equal(got, []string{"b", "a", "c"}) {
		t.Fatalf("чтение: %q", got)
	}
	// Неизвестный интерфейсу id Go не отличает от известного: хранится.
	MarkGuide(dir, []string{"from-future-1"}, true)
	if got := LoadConfig(dir).GuideLearned; !slices.Contains(got, "from-future-1") {
		t.Errorf("чужой id потерян: %q", got)
	}
	if got := MarkGuide(dir, []string{"b", "a", "c", "from-future-1", "нет"}, false); len(got) != 0 {
		t.Fatalf("снять всё: %q", got)
	}
	if data, _ := os.ReadFile(ConfigPath(dir)); strings.Contains(string(data), "guideLearned") {
		t.Errorf("пустой список остался в файле: %s", data)
	}
}

// Мусор, повтор и снятие отсутствующего — файл не пишется (ни создаётся, ни меняется).
func TestMarkGuideInvalidNoWrite(t *testing.T) {
	dir := testkit.TempDir(t)
	for _, c := range []struct {
		ids []string
		on  bool
	}{{nil, true}, {[]string{}, false}, {[]string{"", "A", "a b", "../x", "-a"}, true}, {[]string{"nope"}, false}} {
		if got := MarkGuide(dir, c.ids, c.on); got == nil || len(got) != 0 {
			t.Errorf("%v: %#v", c.ids, got)
		}
		if _, err := os.Stat(ConfigPath(dir)); err == nil {
			t.Fatalf("%v: config.json создан без изменений", c.ids)
		}
	}
	// Повтор уже отмеченного: файл (с мусором и чужими ключами) байт в байт прежний.
	raw := `{"future":1,"guideLearned":["a","BAD"]}`
	writeCfg(t, dir, raw)
	MarkGuide(dir, []string{"a"}, true)
	MarkGuide(dir, []string{"zz", "BAD"}, false)
	if data, _ := os.ReadFile(ConfigPath(dir)); string(data) != raw {
		t.Errorf("файл переписан без изменений: %s", data)
	}
	// Реальное изменение чистит мусор заодно и не теряет чужое.
	MarkGuide(dir, []string{"b"}, true)
	data, _ := os.ReadFile(ConfigPath(dir))
	if strings.Contains(string(data), "BAD") || !strings.Contains(string(data), `"future": 1`) || !slices.Equal(GuideLearned(dir), []string{"a", "b"}) {
		t.Errorf("после записи: %s", data)
	}
}

// Запись отметок не теряет чужие ключи и свои поля; запись другого поля не теряет отметки.
func TestMarkGuideKeepsForeign(t *testing.T) {
	dir := testkit.TempDir(t)
	writeCfg(t, dir, `{"future":{"x":[1,2]},"level":"full","myColors":["#abcdef"],"tours":{"first":"done"}}`)
	MarkGuide(dir, []string{"a"}, true)
	UpdateConfig(dir, func(c *Config) { c.LastProject = "p.salbum" })
	MarkGuide(dir, []string{"b"}, true)
	c := LoadConfig(dir)
	data, _ := os.ReadFile(ConfigPath(dir))
	if c.Level != "full" || c.LastProject != "p.salbum" || !slices.Equal(c.MyColors, []string{"#abcdef"}) || c.Tours["first"] != "done" {
		t.Errorf("свои поля: %+v", c)
	}
	if !strings.Contains(string(data), `"future": {`) {
		t.Errorf("чужой ключ потерян: %s", data)
	}
	if !slices.Equal(c.GuideLearned, []string{"a", "b"}) {
		t.Errorf("отметки: %q", c.GuideLearned)
	}
}

// Прежняя версия, не знающая guideLearned, при записи своих полей ключ не стирает: она
// переписывает чужие ключи верхнего уровня как были (механизм UpdateConfig, config.go).
func TestGuideLearnedOldVersionKeeps(t *testing.T) {
	dir := testkit.TempDir(t)
	MarkGuide(dir, []string{"a", "b"}, true)
	// Прежняя версия: Config без поля guideLearned, тот же алгоритм слияния.
	type oldConfig struct {
		LastProject string `json:"lastProject"`
		Level       string `json:"level,omitempty"`
	}
	data, _ := os.ReadFile(ConfigPath(dir))
	raw := map[string]json.RawMessage{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"lastProject", "level"} {
		delete(raw, k)
	}
	own, _ := json.Marshal(oldConfig{LastProject: "old.salbum", Level: "full"})
	var mine map[string]json.RawMessage
	_ = json.Unmarshal(own, &mine)
	for k, v := range mine {
		raw[k] = v
	}
	out, _ := json.MarshalIndent(raw, "", "  ")
	writeCfg(t, dir, string(out))
	if got := GuideLearned(dir); !slices.Equal(got, []string{"a", "b"}) {
		t.Errorf("отметки после записи старой версией: %q", got)
	}
	// А эта версия, прочитав такой файл, дописывает к прежним.
	if got := MarkGuide(dir, []string{"c"}, true); !slices.Equal(got, []string{"a", "b", "c"}) || LoadConfig(dir).LastProject != "old.salbum" {
		t.Errorf("дописывание: %q", got)
	}
}

// Параллельные вызовы (окно и вкладка браузера) не теряют друг друга; с -race.
func TestMarkGuideConcurrent(t *testing.T) {
	dir := testkit.TempDir(t)
	var wg sync.WaitGroup
	for i := range 16 {
		wg.Go(func() {
			id := fmt.Sprintf("art-%d", i)
			MarkGuide(dir, []string{id, "common"}, true)
			_ = GuideLearned(dir)
			if i%4 == 0 {
				MarkGuide(dir, []string{id}, false)
			}
		})
	}
	wg.Wait()
	got := GuideLearned(dir)
	if len(got) != 1+12 || !slices.Contains(got, "common") {
		t.Fatalf("после 16 параллельных: %d %q", len(got), got)
	}
	if len(slices.Compact(slices.Sorted(slices.Values(got)))) != len(got) {
		t.Errorf("повторы: %q", got)
	}
}

// Сбой записи (папка только для чтения): ответ — правда из файла, отметки в нём нет.
func TestMarkGuideWriteFailure(t *testing.T) {
	dir := testkit.TempDir(t)
	writeCfg(t, dir, `{"guideLearned":["a"]}`)
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Skip("права на папку не ставятся:", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
	if f, err := os.CreateTemp(dir, "probe"); err == nil { // root или ФС без прав: запись проходит
		_ = f.Close()
		t.Skip("запись в папку только для чтения не блокируется")
	}
	if got := MarkGuide(dir, []string{"b", "c"}, true); !slices.Equal(got, []string{"a"}) {
		t.Errorf("поставить: %v", got)
	}
	if got := MarkGuide(dir, []string{"a"}, false); !slices.Equal(got, []string{"a"}) {
		t.Errorf("снять: %v", got)
	}
	if got := GuideLearned(dir); !slices.Equal(got, []string{"a"}) {
		t.Errorf("в файле: %v", got)
	}
}
