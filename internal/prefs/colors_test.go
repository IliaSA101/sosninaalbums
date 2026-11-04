package prefs

import (
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Новый цвет наверх, повтор поднимается без дубля (в том числе в другом регистре и без «#»),
// круг записи и чтения config.json.
func TestPushColorNewFirstNoDupes(t *testing.T) {
	dir := testkit.TempDir(t)
	if got := MyColors(dir); got == nil || len(got) != 0 {
		t.Fatalf("без файла: %#v, ждали пустой не-nil", got)
	}
	RememberColor(dir, "#112233")
	RememberColor(dir, "#445566")
	got := RememberColor(dir, "112233") // повтор: без «#», поднимается
	want := []string{"#112233", "#445566"}
	if !slices.Equal(got, want) {
		t.Fatalf("после повтора %v, ждали %v", got, want)
	}
	if again := MyColors(dir); !slices.Equal(again, want) {
		t.Fatalf("круг Save→Load: %v", again)
	}
	if got := RememberColor(dir, "#ABCDEF"); got[0] != "#abcdef" || len(got) != 3 {
		t.Fatalf("верхний регистр: %v", got)
	}
	// Мусор — список без изменений, файл не переписывается.
	st, _ := os.Stat(ConfigPath(dir))
	for _, bad := range []string{"", "#12", "#gggggg", "#1234567", "red", " "} {
		if got := RememberColor(dir, bad); len(got) != 3 {
			t.Errorf("мусор %q изменил список: %v", bad, got)
		}
	}
	if st2, _ := os.Stat(ConfigPath(dir)); !st2.ModTime().Equal(st.ModTime()) {
		t.Error("мусор переписал config.json")
	}
	// Чистая функция: пустой список и пустой цвет — пусто, не nil.
	if got := PushColor(nil, ""); got == nil || len(got) != 0 {
		t.Errorf("PushColor(nil, \"\") = %#v", got)
	}
}

// Предел 10: одиннадцатый вытесняет самый старый.
func TestPushColorLimit10(t *testing.T) {
	dir := testkit.TempDir(t)
	for i := range 12 {
		RememberColor(dir, fmt.Sprintf("#0000%02x", i))
	}
	got := MyColors(dir)
	if len(got) != 10 || got[0] != "#00000b" || got[9] != "#000002" {
		t.Fatalf("после 12 цветов: %v", got)
	}
	// Длинный список в файле (руками) режется при чтении.
	var big []string
	for i := range 25 {
		big = append(big, fmt.Sprintf("#1000%02x", i))
	}
	UpdateConfig(dir, func(c *Config) { c.MyColors = big })
	if got := MyColors(dir); len(got) != 10 || got[0] != "#100000" {
		t.Fatalf("длинный список: %v", got)
	}
}

// Руками вписанный мусор, верхний регистр, цвета без «#» и повторы: читаются проверенными.
func TestMyColorsDropsJunk(t *testing.T) {
	dir := testkit.TempDir(t)
	raw := `{"myColors":["#AABBCC","zzz","ddeeff","#aabbcc","",null,5,"#12345","#ffffff "]}`
	if err := os.WriteFile(ConfigPath(dir), []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	want := []string{"#aabbcc", "#ddeeff", "#ffffff"}
	if got := MyColors(dir); !slices.Equal(got, want) {
		t.Fatalf("получили %v, ждали %v", got, want)
	}
	// Ключ не массив — как отсутствующий (битый конфиг целиком читается пустым).
	if err := os.WriteFile(ConfigPath(dir), []byte(`{"myColors":"#aabbcc"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := MyColors(dir); got == nil || len(got) != 0 {
		t.Fatalf("ключ-строка: %#v", got)
	}
}

func TestForgetColor(t *testing.T) {
	dir := testkit.TempDir(t)
	for _, c := range []string{"#111111", "#222222", "#333333"} {
		RememberColor(dir, c)
	}
	if got := ForgetColor(dir, "#222222"); !slices.Equal(got, []string{"#333333", "#111111"}) {
		t.Fatalf("после убирания: %v", got)
	}
	// Нет такого, мусор — без изменений и без записи.
	st, _ := os.Stat(ConfigPath(dir))
	for _, c := range []string{"#222222", "#999999", "junk", ""} {
		if got := ForgetColor(dir, c); len(got) != 2 {
			t.Errorf("убирание %q изменило список: %v", c, got)
		}
	}
	if st2, _ := os.Stat(ConfigPath(dir)); !st2.ModTime().Equal(st.ModTime()) {
		t.Error("холостое убирание переписало config.json")
	}
	// Верхний регистр и без «#» тоже находятся; последний убран — ключ исчезает из файла.
	ForgetColor(dir, "111111")
	got := ForgetColor(dir, "#333333")
	if got == nil || len(got) != 0 {
		t.Fatalf("после убирания всех: %#v", got)
	}
	if data, _ := os.ReadFile(ConfigPath(dir)); strings.Contains(string(data), "myColors") {
		t.Errorf("пустой список оставил ключ: %s", data)
	}
}

// Перенос: пишет, только если в config.json пусто; порядок сохраняется, мусор и повторы
// отбрасываются, лишнее сверх 10 отрезается с хвоста.
func TestImportColorsOnlyWhenEmpty(t *testing.T) {
	dir := testkit.TempDir(t)
	if got := ImportColors(dir, nil); got == nil || len(got) != 0 {
		t.Fatalf("пустой перенос: %#v", got)
	}
	in := []string{"#111111", "bad", "#222222", "#111111", "333333"}
	want := []string{"#111111", "#222222", "#333333"}
	if got := ImportColors(dir, in); !slices.Equal(got, want) {
		t.Fatalf("перенос: %v, ждали %v", got, want)
	}
	if got := MyColors(dir); !slices.Equal(got, want) {
		t.Fatalf("круг: %v", got)
	}
	// Второй перенос (другое окно) ничего не меняет.
	if got := ImportColors(dir, []string{"#999999"}); !slices.Equal(got, want) {
		t.Fatalf("повторный перенос изменил список: %v", got)
	}
	// Длинный список: остаются первые 10 в прежнем порядке.
	dir2 := testkit.TempDir(t)
	var long []string
	for i := range 14 {
		long = append(long, fmt.Sprintf("#2000%02x", i))
	}
	got := ImportColors(dir2, long)
	if len(got) != 10 || !slices.Equal(got, long[:10]) {
		t.Fatalf("длинный перенос: %v", got)
	}
}

// Запись другого поля не теряет myColors, а запись цветов — чужие ключи и свои поля.
func TestUpdateConfigKeepsMyColorsAndForeignKeys(t *testing.T) {
	dir := testkit.TempDir(t)
	raw := `{"future":{"x":1},"level":"full","myColors":["#abcdef"]}`
	if err := os.WriteFile(ConfigPath(dir), []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	UpdateConfig(dir, func(c *Config) { c.LastProject = "p.salbum" })
	if got := MyColors(dir); !slices.Equal(got, []string{"#abcdef"}) {
		t.Fatalf("цвета после правки другого поля: %v", got)
	}
	RememberColor(dir, "#123456")
	c := LoadConfig(dir)
	data, _ := os.ReadFile(ConfigPath(dir))
	if c.Level != "full" || c.LastProject != "p.salbum" || !strings.Contains(string(data), `"future": {`) {
		t.Fatalf("после записи цветов: %+v, файл %s", c, data)
	}
	if !slices.Equal(c.MyColors, []string{"#123456", "#abcdef"}) {
		t.Fatalf("цвета: %v", c.MyColors)
	}
}

// Параллельные запоминания (окно и вкладка браузера) не теряют друг друга.
func TestRememberColorConcurrent(t *testing.T) {
	dir := testkit.TempDir(t)
	done := make(chan struct{})
	for i := range 8 {
		go func() {
			RememberColor(dir, fmt.Sprintf("#0000%02x", i))
			done <- struct{}{}
		}()
	}
	for range 8 {
		<-done
	}
	if got := MyColors(dir); len(got) != 8 {
		t.Fatalf("после 8 параллельных: %v", got)
	}
}
