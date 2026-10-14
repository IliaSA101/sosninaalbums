package export

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

func fileNames(fs []DirFile) []string {
	out := make([]string, len(fs))
	for i, f := range fs {
		out[i] = f.Name
	}
	slices.Sort(out)
	return out
}

// Папка с файлами обоих видов: номер и страница из имени, метка решает «наш/чужой», обложка не в счёт.
func TestPlanExportDirPages(t *testing.T) {
	dir := testkit.TempDir(t)
	ourData := append([]byte{0xFF, 0xD8, 0xFF, 0xFE, 0, 16}, Marker...)
	for name, data := range map[string][]byte{
		"05.jpg": ourData, "05-1.jpg": ourData, "05-2.jpg": []byte("чужой"), "005-2.jpg": ourData,
		"00-1.jpg": ourData, "000.jpg": ourData, "07-3.jpg": ourData, "07-.jpg": ourData, "spread_03.jpg": []byte("x"),
	} {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	ours, foreign, err := PlanDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"005-2.jpg", "05-1.jpg", "05.jpg", "spread_03.jpg"}; !slices.Equal(fileNames(ours), want) {
		t.Fatalf("ours = %v, ждали %v", fileNames(ours), want)
	}
	if want := []string{"05-2.jpg"}; !slices.Equal(fileNames(foreign), want) {
		t.Fatalf("foreign = %v, ждали %v", fileNames(foreign), want)
	}
	for _, f := range ours {
		if f.Name == "005-2.jpg" && (f.Spread != 5 || f.Page != 2) {
			t.Errorf("005-2.jpg разобран как %+v", f)
		}
	}
	// Цель-страница задевает только файл своей страницы; цель-разворот — любой вид и ширину имени.
	pl := NewPlan([]Unit{{5, 2}}, 10, false)
	if got := fileNames(HitFiles(pl, append(ours, foreign...))); !slices.Equal(got, []string{"005-2.jpg", "05-2.jpg"}) {
		t.Errorf("страница 5п задела %v", got)
	}
	pl = NewPlan([]Unit{{5, 0}}, 10, true)
	if got := HitFiles(pl, append(ours, foreign...)); len(got) != 4 {
		t.Errorf("разворот 5 задел %v, ждали 4 файла", fileNames(got))
	}
	pl = NewPlan([]Unit{{3, 0}}, 10, false)
	if got := fileNames(HitFiles(pl, ours)); !slices.Equal(got, []string{"spread_03.jpg"}) {
		t.Errorf("разворот 3 задел %v", got)
	}
}

func TestNewPlan(t *testing.T) {
	// «7, 7п» разворотами: целиком и правая из отдельного рендера; страницами — 7п поглощается.
	pl := NewPlan([]Unit{{2, 0}, {7, 0}, {7, 2}}, 10, false)
	want := []string{"02.jpg", "07.jpg", "07-2.jpg"}
	if got := pl.Names(10); !slices.Equal(got, want) {
		t.Errorf("names = %v, ждали %v", got, want)
	}
	pl = NewPlan([]Unit{{7, 0}, {7, 2}, {8, 1}}, 10, true)
	if got, want := pl.Names(10), []string{"07-1.jpg", "07-2.jpg", "08-1.jpg"}; !slices.Equal(got, want) || len(pl.Jobs) != 2 {
		t.Errorf("names = %v, ждали %v", got, want)
	}
	if pl = NewPlan(nil, 3, false); !pl.Full || len(pl.Jobs) != 3 {
		t.Errorf("все развороты: %+v", pl)
	}
}

func TestPlanExportDir(t *testing.T) {
	dir := testkit.TempDir(t)
	// Минимальный «наш» файл: SOI + COM с меткой (читается только заголовок).
	ourData := append([]byte{0xFF, 0xD8, 0xFF, 0xFE, 0, 16}, Marker...)
	write := func(name string, data []byte) {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("spread_01.jpg", nil) // экспорт v0.2: наш без метки
	write("01.JPG", ourData)    // с меткой, регистр не важен
	write("002.jpg", ourData)
	write("02.jpg", []byte("чужой"))
	for _, n := range []string{"00.jpg", "000.jpg", "keep.jpg", "spread_x.jpg", "03.png", "1.jpg"} {
		write(n, []byte("чужой"))
	}
	if err := os.Mkdir(filepath.Join(dir, "04.jpg"), 0o755); err != nil {
		t.Fatal(err)
	}

	oursF, foreignF, err := PlanDir(dir)
	ours, foreign := fileNames(oursF), fileNames(foreignF)
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(ours)
	if want := []string{"002.jpg", "01.JPG", "spread_01.jpg"}; !slices.Equal(ours, want) {
		t.Fatalf("ours = %v, ждали %v", ours, want)
	}
	if !slices.Equal(foreign, []string{"02.jpg", "04.jpg"}) { // папка 04.jpg — тоже чужая
		t.Fatalf("foreign = %v, ждали [02.jpg 04.jpg]", foreign)
	}

	// removeFiles удаляет только перечисленное; чужое и обложка остаются.
	if err := removeFiles(dir, ours); err != nil {
		t.Fatal(err)
	}
	left, _ := os.ReadDir(dir)
	if len(left) != 8 {
		t.Fatalf("осталось %d записей, ждали 8", len(left))
	}
	for _, n := range []string{"00.jpg", "02.jpg", "keep.jpg"} {
		if _, err := os.Stat(filepath.Join(dir, n)); err != nil {
			t.Fatalf("%s пропал: %v", n, err)
		}
	}
	if err := removeFiles(dir, []string{"../x.jpg"}); err == nil {
		t.Fatal("имя с путём должно отклоняться")
	}
	if _, _, err := PlanDir(filepath.Join(dir, "нет")); err == nil {
		t.Fatal("несуществующая папка — ошибка")
	}
}

// Экспорт при ≥ 1000 разворотов называет файлы 0001.jpg: свой (с меткой) заменяется, чужой — отказ.
func TestPlanExportDirFourDigits(t *testing.T) {
	dir := testkit.TempDir(t)
	ourData := append([]byte{0xFF, 0xD8, 0xFF, 0xFE, 0, 16}, Marker...)
	for name, data := range map[string][]byte{
		"0001.jpg": ourData, "1000.jpg": ourData, "0002.jpg": []byte("чужой"), "0000.jpg": []byte("обложка"), "12345.jpg": ourData,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	oursF, foreignF, err := PlanDir(dir)
	ours, foreign := fileNames(oursF), fileNames(foreignF)
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(ours)
	if want := []string{"0001.jpg", "1000.jpg", "12345.jpg"}; !slices.Equal(ours, want) {
		t.Fatalf("ours = %v, ждали %v", ours, want)
	}
	if !slices.Equal(foreign, []string{"0002.jpg"}) {
		t.Fatalf("foreign = %v, ждали [0002.jpg]", foreign)
	}
}

// Папка под именем разворота (05.jpg, spread_07.jpg) — чужая: не пропускается молча,
// экспорт под неё откажет, а не заменит; обычный файл с меткой по-прежнему наш, 00.jpg не отдаётся.
func TestPlanExportDirFolderIsForeign(t *testing.T) {
	dir := testkit.TempDir(t)
	ourData := append([]byte{0xFF, 0xD8, 0xFF, 0xFE, 0, 16}, Marker...)
	if err := os.WriteFile(filepath.Join(dir, "01.jpg"), ourData, 0o644); err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{"05.jpg", "06-1.jpg", "spread_07.jpg", "00.jpg"} {
		if err := os.Mkdir(filepath.Join(dir, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	ours, foreign, err := PlanDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(fileNames(ours), []string{"01.jpg"}) {
		t.Fatalf("ours = %v, ждали [01.jpg]", fileNames(ours))
	}
	if want := []string{"05.jpg", "06-1.jpg", "spread_07.jpg"}; !slices.Equal(fileNames(foreign), want) {
		t.Fatalf("foreign = %v, ждали %v", fileNames(foreign), want)
	}
	for _, f := range foreign {
		if f.Name == "06-1.jpg" && (f.Spread != 6 || f.Page != 1) {
			t.Errorf("06-1.jpg разобран как %+v", f)
		}
	}
	// Цель-разворот задевает свою папку: экспорт откажет.
	if got := fileNames(HitFiles(NewPlan([]Unit{{5, 0}}, 10, true), foreign)); !slices.Equal(got, []string{"05.jpg"}) {
		t.Errorf("разворот 5 задел %v", got)
	}
}

// Ссылка под именем разворота — чужая, даже если указывает на наш файл: запись по ней не делаем.
func TestPlanExportDirSymlinkIsForeign(t *testing.T) {
	dir := testkit.TempDir(t)
	ourData := append([]byte{0xFF, 0xD8, 0xFF, 0xFE, 0, 16}, Marker...)
	target := filepath.Join(dir, "01.jpg")
	if err := os.WriteFile(target, ourData, 0o644); err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"05.jpg", "spread_07.jpg"} {
		if err := os.Symlink(target, filepath.Join(dir, n)); err != nil {
			t.Skipf("ссылки недоступны: %v", err)
		}
	}
	ours, foreign, err := PlanDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(fileNames(ours), []string{"01.jpg"}) || !slices.Equal(fileNames(foreign), []string{"05.jpg", "spread_07.jpg"}) {
		t.Fatalf("ours = %v, foreign = %v", fileNames(ours), fileNames(foreign))
	}
}
