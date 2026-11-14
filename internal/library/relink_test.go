package library

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

func mkdir(t *testing.T, parts ...string) string {
	t.Helper()
	d := filepath.Join(parts...)
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatal(err)
	}
	return d
}

func sizeOf(t *testing.T, p string) int64 {
	t.Helper()
	st, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	return st.Size()
}

func TestFindByName(t *testing.T) {
	root := testkit.TempDir(t)
	ivan := mkdir(t, root, "Ивановы", "Отбор")
	petr := mkdir(t, root, "Петровы", "Отбор")
	hidden := mkdir(t, root, ".trash")
	a := testkit.JPEG(t, ivan, "IMG_1.jpg", 60, 40)
	testkit.JPEG(t, petr, "img_1.jpg", 60, 40)   // тот же размер в байтах и имя: решает хвост пути
	testkit.JPEG(t, hidden, "img_2.jpg", 50, 30) // точка-папка пропускается
	b := testkit.JPEG(t, mkdir(t, root, "x"), "img_2.jpg", 50, 30)
	testkit.JPEG(t, mkdir(t, root, "y"), "img_2.jpg", 20, 10) // то же имя, другие байты
	testkit.JPEG(t, mkdir(t, root, "y"), "img_3.jpg", 20, 10)
	testkit.JPEG(t, mkdir(t, root, "z"), "img_3.jpg", 20, 10)
	testkit.JPEG(t, mkdir(t, root, "z"), "img_4.jpg", 20, 10) // другие W×H — не подходит
	testkit.JPEG(t, mkdir(t, root, "y"), "img_4.jpg", 90, 70)

	want := []Want{
		{ID: "a", Old: `E:\Съёмки\Ивановы\Отбор\IMG_1.JPG`, Size: sizeOf(t, a), W: 60, H: 40},
		{ID: "b", Old: `E:\Съёмки\img_2.jpg`, Size: sizeOf(t, b), W: 50, H: 30},
		{ID: "c", Old: `E:\Съёмки\img_3.jpg`, W: 20, H: 10}, // без байтов: два кандидата, хвоста нет
		{ID: "d", Old: `E:\Съёмки\img_4.jpg`, W: 90, H: 70}, // без байтов: подходит один по W×H
		{ID: "e", Old: `E:\Съёмки\нет.jpg`, Size: 5, W: 1, H: 1},
	}
	got, err := FindByName(context.Background(), root, want, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got.Paths["a"] != a {
		t.Errorf("a: %q, ждали %q (хвост Ивановы\\Отбор)", got.Paths["a"], a)
	}
	if got.Paths["b"] != b {
		t.Errorf("b: %q, ждали %q (точка-папка пропущена, другие байты не подошли)", got.Paths["b"], b)
	}
	if _, ok := got.Paths["c"]; ok || len(got.Ambiguous) != 1 || got.Ambiguous[0] != "c" {
		t.Errorf("c: ждали неоднозначность, got %+v", got)
	}
	if !strings.HasSuffix(got.Paths["d"], filepath.Join("y", "img_4.jpg")) {
		t.Errorf("d по W×H: %q", got.Paths["d"])
	}
	if _, ok := got.Paths["e"]; ok {
		t.Error("e найден, хотя файла нет")
	}
}

func TestFindByNameLimitAndCancel(t *testing.T) {
	root := testkit.TempDir(t)
	for _, n := range []string{"a.jpg", "b.jpg", "c.jpg"} {
		testkit.JPEG(t, root, n, 10, 10)
	}
	want := []Want{{ID: "a", Old: `E:\a.jpg`, W: 10, H: 10}}
	if _, err := FindByName(context.Background(), root, want, 2); !errors.Is(err, ErrTooManyFiles) {
		t.Errorf("предел: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := FindByName(ctx, root, want, 0); !errors.Is(err, context.Canceled) {
		t.Errorf("отмена: %v", err)
	}
}

// wantByName: имя без учёта регистра и с любым разделителем; пустой старый путь не ищется.
func TestWantByName(t *testing.T) {
	got := wantByName([]Want{{Old: `E:\a\IMG.jpg`}, {Old: ""}, {Old: "/b/img.JPG"}, {Old: `F:\c.jpg`}})
	if len(got) != 2 || !slices.Equal(got["img.jpg"], []int{0, 2}) || !slices.Equal(got["c.jpg"], []int{3}) {
		t.Errorf("wantByName: %v", got)
	}
}

func TestSuggestDrive(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("буквы дисков")
	}
	have := map[string]bool{`F:\`: true, `G:\`: true, `F:\a\1.jpg`: true, `F:\a\2.jpg`: true, `G:\a\1.jpg`: true}
	exists := func(p string) bool { return have[p] }
	h := SuggestDrive([]string{`E:\a\1.jpg`, `E:\a\2.jpg`, `E:\a\3.jpg`, `D:\x.jpg`}, exists)
	if h != (DriveHint{From: "E", To: "F", Found: 2, Total: 3}) {
		t.Errorf("подсказка: %+v", h)
	}
	if h := SuggestDrive([]string{`E:\q.jpg`}, exists); h.From != "" {
		t.Errorf("нигде не нашлось, а подсказка %+v", h)
	}
	if h := SuggestDrive(nil, exists); h.From != "" {
		t.Errorf("пустой список: %+v", h)
	}
}

func TestMissingOf(t *testing.T) {
	refs := map[string]string{"b": "/x/b", "a": "/x/a", "c": "/x/c"}
	path := func(s string) string { return s }
	got := MissingOf(refs, path, func(p string) bool { return p == "/x/b" })
	if !slices.Equal(got, []string{"a", "c"}) {
		t.Errorf("пропавшие по порядку ID: %v", got)
	}
	if got := MissingOf(refs, path, func(string) bool { return true }); got == nil || len(got) != 0 {
		t.Errorf("все на месте: %#v, ждали пустой не-nil список", got)
	}
	if got := MissingOf(map[string]string(nil), path, func(string) bool { return false }); len(got) != 0 {
		t.Errorf("пустая карта: %v", got)
	}
}

// Ветки выбора буквы: ничья по числу путей, пути без буквы диска, кандидат без диска,
// выбор лучшей буквы и проверка не более driveSample путей на кандидата.
func TestSuggestDriveBranches(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("буквы дисков")
	}
	set := func(keys ...string) func(string) bool {
		have := map[string]bool{}
		for _, k := range keys {
			have[k] = true
		}
		return func(p string) bool { return have[p] }
	}

	// Пути без буквы (относительный, UNC) не считаются.
	if h := SuggestDrive([]string{`rel\a.jpg`, `\\srv\share\a.jpg`}, set(`F:\`, `F:\a.jpg`)); h.From != "" {
		t.Errorf("пути без буквы: %+v", h)
	}
	// Ничья E и D по одному пути: берётся меньшая буква (D).
	h := SuggestDrive([]string{`E:\a.jpg`, `D:\a.jpg`}, set(`F:\`, `F:\a.jpg`))
	if h != (DriveHint{From: "D", To: "F", Found: 1, Total: 1}) {
		t.Errorf("ничья: %+v", h)
	}
	// Диск-кандидат без корня пропускается, даже если файл «лежит» по пути.
	if h := SuggestDrive([]string{`E:\a.jpg`}, set(`F:\a.jpg`)); h.From != "" {
		t.Errorf("кандидат без корня: %+v", h)
	}
	// Среди кандидатов выигрывает тот, где нашлось больше.
	exists := set(`F:\`, `G:\`, `F:\1.jpg`, `G:\1.jpg`, `G:\2.jpg`)
	h = SuggestDrive([]string{`E:\1.jpg`, `E:\2.jpg`}, exists)
	if h != (DriveHint{From: "E", To: "G", Found: 2, Total: 2}) {
		t.Errorf("лучший кандидат: %+v", h)
	}
	// Больше driveSample путей: кандидат отбирается по первым driveSample, Found/Total — по всем.
	var missing []string
	have := []string{`F:\`}
	for i := range driveSample + 5 {
		missing = append(missing, fmt.Sprintf(`E:\p%d.jpg`, i))
		have = append(have, fmt.Sprintf(`F:\p%d.jpg`, i))
	}
	h = SuggestDrive(missing, set(have...))
	if h != (DriveHint{From: "E", To: "F", Found: driveSample + 5, Total: driveSample + 5}) {
		t.Errorf("длинный список: %+v", h)
	}
}
