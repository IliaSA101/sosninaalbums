package salbum

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Версия программы в файле и имя копии старого формата (098 design §6).

// Строка есть у каждого формата раньше текущего, вид «0.N», версии не убывают; текущий и новее —
// ""; строка v17 → 0.23 добавлена вместе с v18; файл без номера — как v1.
func TestLastAppTable(t *testing.T) {
	prev := 0
	for v := 1; v < model.FormatVersion; v++ {
		got := LastApp(v)
		major, minor, ok := strings.Cut(got, ".")
		n, err := strconv.Atoi(minor)
		if !ok || major != "0" || err != nil {
			t.Fatalf("формат %d: %q — нет строки или не «0.N»", v, got)
		}
		if n < prev {
			t.Fatalf("формат %d: %q раньше предыдущего 0.%d", v, got, prev)
		}
		prev = n
	}
	known := map[int]string{0: "0.3", 1: "0.3", 3: "0.3", 4: "0.6", 5: "0.9", 6: "0.10", 12: "0.17", 13: "0.19", 16: "0.22", 17: "0.23",
		-5: "0.3", model.FormatVersion: "", model.FormatVersion + 1: "", 1 << 20: ""}
	for v, want := range known {
		if got := LastApp(v); got != want {
			t.Errorf("LastApp(%d) = %q, ждали %q", v, got, want)
		}
	}
}

// Имя копии: «<стем> (для X.Y).salbum», k ≥ 2 — « k»; расширение любого регистра снимается, без
// него — стем как есть; папка та же.
func TestBackupNameTable(t *testing.T) {
	dir := filepath.Join("альбомы", "2026")
	cases := []struct {
		name     string
		from, k  int
		wantName string
	}{
		{"Свадьба.salbum", 16, 1, "Свадьба (для 0.22).salbum"},
		{"Свадьба.salbum", 16, 2, "Свадьба (для 0.22) 2.salbum"},
		{"Свадьба.salbum", 16, 0, "Свадьба (для 0.22).salbum"},
		{"Свадьба.SALBUM", 12, 1, "Свадьба (для 0.17).salbum"},
		{"Свадьба", 4, 1, "Свадьба (для 0.6).salbum"},
		{"a.b.salbum", 1, 99, "a.b (для 0.3) 99.salbum"},
		{"черновик.salbum", 15, 1, "черновик (для 0.21).salbum"},
	}
	for _, c := range cases {
		if got := BackupName(filepath.Join(dir, c.name), c.from, c.k); got != filepath.Join(dir, c.wantName) {
			t.Errorf("BackupName(%s, %d, %d) = %s, ждали %s", c.name, c.from, c.k, got, c.wantName)
		}
	}
}

// Открыли саму копию: хвост « (для X.Y)» и « (для X.Y) n» снимается, копия копии — от исходного
// имени; чужие скобки и хвост не по грамматике остаются. BackupOld самой копии в себя не пишет —
// берёт « 2».
func TestBackupNameSelf(t *testing.T) {
	cases := map[string]string{
		"Свадьба (для 0.22).salbum":           "Свадьба (для 0.22).salbum",
		"Свадьба (для 0.17) 3.salbum":         "Свадьба (для 0.22).salbum",
		"Мама (для бабушки).salbum":           "Мама (для бабушки) (для 0.22).salbum",
		"Свадьба (для 0.22) x.salbum":         "Свадьба (для 0.22) x (для 0.22).salbum",
		"Свадьба (для 0.22)2.salbum":          "Свадьба (для 0.22)2 (для 0.22).salbum",
		"Свадьба (для 0.22) .salbum":          "Свадьба (для 0.22)  (для 0.22).salbum",
		"Свадьба (для 22).salbum":             "Свадьба (для 22) (для 0.22).salbum",
		"(для 0.22).salbum":                   "(для 0.22) (для 0.22).salbum",
		"Свадьба (для 0.2 (для 0.21)).salbum": "Свадьба (для 0.2 (для 0.21)) (для 0.22).salbum",
	}
	for in, want := range cases {
		if got := BackupName(in, 16, 1); got != want {
			t.Errorf("%q → %q, ждали %q", in, got, want)
		}
	}
	dir := testkit.TempDir(t)
	self := filepath.Join(dir, "Свадьба (для 0.22).salbum")
	src := []byte(`{"version":16}`)
	if err := os.WriteFile(self, src, 0o644); err != nil {
		t.Fatal(err)
	}
	bak, err := BackupOld(self, 16)
	if want := filepath.Join(dir, "Свадьба (для 0.22) 2.salbum"); err != nil || bak != want {
		t.Fatalf("копия самой копии: %q, %v; ждали %q", bak, err, want)
	}
	if got, _ := os.ReadFile(bak); !bytes.Equal(got, src) {
		t.Fatalf("в копии %q", got)
	}
}

// Поле app файла в имя копии не попадает (строка из файла в пути — «../»): версия — из таблицы.
// Негодная строка app отбрасывается при чтении и в NewerError не идёт.
func TestBackupNameIgnoresApp(t *testing.T) {
	base := testkit.TempDir(t)
	dir := filepath.Join(base, "альбом")
	path := filepath.Join(dir, "a.salbum")
	src := []byte(`{"version":16,"app":"../../x"}`)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, src, 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	bak, err := BackupOld(path, p.From())
	if err != nil || bak != filepath.Join(dir, "a (для 0.22).salbum") {
		t.Fatalf("копия %q, %v", bak, err)
	}
	for _, d := range []string{base, dir} {
		ents, _ := os.ReadDir(d)
		var names []string
		for _, e := range ents {
			names = append(names, e.Name())
		}
		if want := map[string]int{base: 1, dir: 2}[d]; len(names) != want {
			t.Fatalf("в %s: %v — лишний файл", d, names)
		}
	}
	_, err = Decode([]byte(fmt.Sprintf(`{"version":%d,"app":"../../x"}`, model.FormatVersion+1)), dir)
	var ne *NewerError
	if !errors.As(err, &ne) || ne.App != "" {
		t.Fatalf("NewerError: %#v", err)
	}
}

// Имя копии занято чужим файлом (или папкой): чужое цело, копия — « 2», « 3»; все 99 заняты —
// ошибка и копии нет. Исходник уже текущей версии или его нет — копировать нечего.
func TestBackupBusyName(t *testing.T) {
	dir := testkit.TempDir(t)
	path := filepath.Join(dir, "Свадьба.salbum")
	src := []byte(`{"version":16,"spreads":[]}`)
	if err := os.WriteFile(path, src, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(BackupName(path, 16, 1), []byte("чужое"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(BackupName(path, 16, 2), 0o755); err != nil {
		t.Fatal(err)
	}
	bak, err := BackupOld(path, 16)
	if err != nil || bak != BackupName(path, 16, 3) {
		t.Fatalf("копия %q, %v; ждали « 3»", bak, err)
	}
	if got, _ := os.ReadFile(BackupName(path, 16, 1)); string(got) != "чужое" {
		t.Fatalf("чужой файл: %q", got)
	}
	if got, _ := os.ReadFile(bak); !bytes.Equal(got, src) {
		t.Fatalf("в копии %q", got)
	}
	// Все имена заняты чужим — ошибка.
	full := filepath.Join(dir, "Полный.salbum")
	if err := os.WriteFile(full, src, 0o644); err != nil {
		t.Fatal(err)
	}
	for k := 1; k <= maxBackupNo; k++ {
		if err := os.WriteFile(BackupName(full, 16, k), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if bak, err := BackupOld(full, 16); err == nil || bak != "" || !strings.Contains(err.Error(), "0.22") {
		t.Fatalf("все заняты: %q, %v", bak, err)
	}
	// Нечего копировать: нет исходника; исходник уже текущей версии.
	if bak, err := BackupOld(filepath.Join(dir, "нет.salbum"), 16); bak != "" || err != nil {
		t.Fatalf("нет исходника: %q, %v", bak, err)
	}
	cur := filepath.Join(dir, "новый.salbum")
	if err := os.WriteFile(cur, fmt.Appendf(nil, `{"version":%d}`, model.FormatVersion), 0o644); err != nil {
		t.Fatal(err)
	}
	if bak, err := BackupOld(cur, 16); bak != "" || err != nil {
		t.Fatalf("уже текущая: %q, %v", bak, err)
	}
	if _, err := os.Stat(BackupName(cur, 16, 1)); err == nil {
		t.Fatal("копия файла текущей версии")
	}
}

// Копия с теми же байтами уже лежит (под первым или следующим именем) — копия сделана: ответ "",
// новых файлов нет; та же длина, другие байты — чужой файл.
func TestBackupAlreadyMade(t *testing.T) {
	dir := testkit.TempDir(t)
	path := filepath.Join(dir, "a.salbum")
	src := []byte(`{"version":15}`)
	if err := os.WriteFile(path, src, 0o644); err != nil {
		t.Fatal(err)
	}
	first, err := BackupOld(path, 15)
	if err != nil || first != BackupName(path, 15, 1) {
		t.Fatalf("первая копия %q, %v", first, err)
	}
	if bak, err := BackupOld(path, 15); bak != "" || err != nil {
		t.Fatalf("повтор: %q, %v", bak, err)
	}
	// Под первым именем — чужой файл той же длины, под вторым — уже сделанная копия.
	if err := os.WriteFile(first, []byte(`{"version":14}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(BackupName(path, 15, 2), src, 0o644); err != nil {
		t.Fatal(err)
	}
	if bak, err := BackupOld(path, 15); bak != "" || err != nil {
		t.Fatalf("копия под « 2»: %q, %v", bak, err)
	}
	if _, err := os.Stat(BackupName(path, 15, 3)); err == nil {
		t.Fatal("лишняя копия « 3»")
	}
}

// Файл новее программы: NewerError с номером и версией из поля app (годная строка — как есть,
// негодная или не строка — ""); errors.Is(…, ErrNewer) и текст — прежние. Файл своей версии с
// мусором в app открывается.
func TestNewerErrorApp(t *testing.T) {
	dir := testkit.TempDir(t)
	next := model.FormatVersion + 1
	cases := map[string]string{
		`"0.25.0"`:         "0.25.0",
		`"0.25.1-rc2+abc"`: "0.25.1-rc2+abc",
		`"dev"`:            "dev",
		`"0.25 бета"`:      "",
		`"` + strings.Repeat("9", maxApp+1) + `"`: "",
		`25`:      "",
		`null`:    "",
		`{"a":1}`: "",
	}
	for raw, want := range cases {
		_, err := Decode(fmt.Appendf(nil, `{"version":%d,"app":%s,"spreads":"новый вид"}`, next, raw), dir)
		var ne *NewerError
		if !errors.As(err, &ne) || ne.App != want || ne.Version != next || !errors.Is(err, ErrNewer) || err.Error() != ErrNewer.Error() {
			t.Errorf("app %s: %#v, ждали App %q", raw, err, want)
		}
	}
	if _, err := Decode(fmt.Appendf(nil, `{"version":%d,"app":{"a":1}}`, model.FormatVersion), dir); err != nil {
		t.Fatalf("своя версия с мусором в app: %v", err)
	}
	if _, err := Decode(fmt.Appendf(nil, `{"version":%d}`, next), dir); !errors.As(err, new(*NewerError)) {
		t.Fatalf("без app: %v", err)
	}
}

// app пишется версией, поставленной SetAppVersion; негодная строка и "" — поля нет.
func TestSetAppVersion(t *testing.T) {
	t.Cleanup(func() { SetAppVersion("") })
	p := model.New()
	for v, want := range map[string]string{"0.23.0": `"app": "0.23.0"`, "": "", "0.23 бета": "", "dev": `"app": "dev"`} {
		SetAppVersion(v)
		data, err := Encode(p, "")
		if err != nil {
			t.Fatal(err)
		}
		if has := bytes.Contains(data, []byte(`"app"`)); has != (want != "") || want != "" && !bytes.Contains(data, []byte(want)) {
			t.Errorf("версия %q: %s", v, data)
		}
	}
}
