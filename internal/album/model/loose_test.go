package model

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// at — абсолютный путь от корня диска временной папки (без обращения к диску).
func at(parts ...string) string {
	root := filepath.VolumeName(os.TempDir()) + string(filepath.Separator)
	return filepath.Join(append([]string{root}, parts...)...)
}

func files(list ...string) []LooseFile {
	out := make([]LooseFile, len(list))
	for i, f := range list {
		out[i] = LooseFile{Path: f}
	}
	return out
}

// Папки: повтор (на Windows и macOS — и в другом регистре, paths.Fold) и вложенные уходят, порядок
// оставшихся прежний, запасной путь ушедшей снят, оставшейся — на месте; корень диска накрывает всё.
func TestFoldSourcesNested(t *testing.T) {
	p := New()
	spring, march, fav := at("Весна 2026"), at("Весна 2026", "Март"), at("Весна 2026", "Март", "Избранное")
	other, prefix := at("Банкет"), at("Весна 2026 копия")
	p.Folders = []string{march, other, spring, fav, strings.ToUpper(other), prefix}
	p.FolderAlt = map[string]string{march: at("E", "Март"), other: at("E", "Банкет")}
	want, n := []string{other, spring, prefix}, 3
	if up := strings.ToUpper(other); !paths.FoldEqual(up, other) { // Linux: другая папка
		want, n = []string{other, spring, up, prefix}, 2
	}
	merged, in := p.FoldSources(nil)
	if merged != n || in != 0 || !reflect.DeepEqual(p.Folders, want) {
		t.Fatalf("папки %q (слито %d), ждали %q", p.Folders, merged, want)
	}
	if !reflect.DeepEqual(p.FolderAlt, map[string]string{other: at("E", "Банкет")}) {
		t.Fatalf("запасные пути: %v", p.FolderAlt)
	}
	q := New()
	q.Folders = []string{other, at(), spring}
	if merged, _ := q.FoldSources(nil); merged != 2 || !reflect.DeepEqual(q.Folders, []string{at()}) {
		t.Fatalf("корень диска: %q, слито %d", q.Folders, merged)
	}
	// Пусто и nil: ничего не меняется, nil остаётся nil.
	e := New()
	if merged, in := e.FoldSources(nil); merged+in != 0 || e.Folders != nil || e.Loose != nil || e.FolderAlt != nil {
		t.Fatalf("пустой проект: %d %d %v %v", merged, in, e.Folders, e.Loose)
	}
}

// Отдельные: под папкой — уходит (looseIn), повтор пути в любом добавлении (Clean; регистр — на
// Windows и macOS) — уходит позднейший, пустое добавление и пустой путь — тоже; запасной путь едет с файлом.
func TestFoldSourcesLoose(t *testing.T) {
	p := New()
	dir := at("Март")
	a, b, c := at("Телефон", "a.jpg"), at("Загрузки", "b.png"), at("Март", "Избранное", "c.jpg")
	p.Folders = []string{dir}
	p.Loose = []LooseAdd{
		{ID: "000000000001", At: 1, Files: []LooseFile{{Path: a, Alt: at("E", "a.jpg")}, {Path: c}}},
		{ID: "000000000002", At: 2, Files: files(strings.ToUpper(a), filepath.Join(at("Загрузки"), ".", "b.png"))},
		{ID: "000000000003", At: 3, Files: files(c)},
		{ID: "000000000004", At: 4},
		{ID: "000000000005", At: 5, Files: files("", b)},
	}
	dotB := filepath.Join(at("Загрузки"), ".", "b.png")
	second := []string{dotB}
	if upA := strings.ToUpper(a); !paths.FoldEqual(upA, a) { // Linux: другой файл, остаётся
		second = []string{upA, dotB}
	}
	_, in := p.FoldSources(nil)
	want := []LooseAdd{
		{ID: "000000000001", At: 1, Files: []LooseFile{{Path: a, Alt: at("E", "a.jpg")}}},
		{ID: "000000000002", At: 2, Files: files(second...)},
	}
	if in != 2 || !reflect.DeepEqual(p.Loose, want) {
		t.Fatalf("в папке %d, добавления:\n%+v\nждали\n%+v", in, p.Loose, want)
	}
	if got := p.LooseFiles(); !reflect.DeepEqual(got, append([]string{a}, second...)) {
		t.Fatalf("LooseFiles: %q", got)
	}
	// Всё ушло в папки — Loose nil (в файл и снимок не пишется).
	q := New()
	q.Folders = []string{at()}
	q.Loose = []LooseAdd{{ID: "000000000001", Files: files(a, b)}}
	if _, in := q.FoldSources(nil); in != 2 || q.Loose != nil {
		t.Fatalf("под корнем: %d, %+v", in, q.Loose)
	}
}

// Подпапка, до которой сканер корня не дойдёт (093-4): на «.» (и глубже под ней), через ссылку
// (linked) — отдельные записи, их отдельные фото тоже остаются; корень-ссылка не накрывает ничего
// (0.22-sec-1: сканер из неё не берёт ни файла) — её подпапка и фото в ней остаются; обычная
// подпапка и фото в ней — сворачиваются; точка-файл в корне сканер читает — фото уходит в папку.
func TestFoldKeepsHiddenSubfolder(t *testing.T) {
	root, hidden, deep := at("A"), at("A", ".edits"), at("A", "x", ".cache", "y")
	link, under, plain := at("A", "Ссылка"), at("A", "Ссылка", "z"), at("A", "Март")
	linked := func(dir string) bool { return strings.EqualFold(dir, link) }
	p := New()
	p.Folders = []string{root, hidden, deep, link, under, plain}
	p.FolderAlt = map[string]string{hidden: at("E", ".edits"), plain: at("E", "Март")}
	inHidden, inLink, inPlain, dotFile := at("A", ".edits", "1.jpg"), at("A", "Ссылка", "2.jpg"), at("A", "Март", "3.jpg"), at("A", ".4.jpg")
	p.Loose = []LooseAdd{{ID: "000000000001", Files: files(inHidden, inLink, inPlain, dotFile, at("A", ".x", "5.jpg"))}}
	merged, in := p.FoldSources(linked)
	if want := []string{root, hidden, deep, link, under}; merged != 1 || !reflect.DeepEqual(p.Folders, want) {
		t.Fatalf("папки %q (слито %d), ждали %q", p.Folders, merged, want)
	}
	if !reflect.DeepEqual(p.FolderAlt, map[string]string{hidden: at("E", ".edits")}) {
		t.Fatalf("запасные пути: %v", p.FolderAlt)
	}
	// 1.jpg — внутри отдельной .edits, 3.jpg и .4.jpg — в A; 2.jpg в корне-ссылке остаётся.
	if want := []string{inLink, at("A", ".x", "5.jpg")}; in != 3 || !reflect.DeepEqual(p.LooseFiles(), want) {
		t.Fatalf("в папках %d, отдельные %q, ждали %q", in, p.LooseFiles(), want)
	}
	// Без ссылок на диске (nil) та же ссылка — обычная подпапка: сворачивается.
	q := New()
	q.Folders = []string{root, link}
	if merged, _ := q.FoldSources(nil); merged != 1 || !reflect.DeepEqual(q.Folders, []string{root}) {
		t.Fatalf("без проверки ссылок: %q", q.Folders)
	}
	if merged, in := p.FoldSources(linked); merged+in != 0 {
		t.Fatalf("второй вызов: %d %d", merged, in)
	}
}

// Подпапка с именем устройства (Windows: «CON» не открыть) — сканер в неё не заходит: отдельная.
func TestFoldKeepsDeviceSubfolder(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("устройства — только на Windows")
	}
	p := New()
	dev := at("A", "aux", "b")
	p.Folders = []string{at("A"), dev}
	p.Loose = []LooseAdd{{ID: "000000000001", Files: files(at("A", "nul", "1.jpg"))}}
	if merged, in := p.FoldSources(nil); merged+in != 0 || len(p.Folders) != 2 || len(p.LooseFiles()) != 1 {
		t.Fatalf("устройство: папки %q, отдельные %q", p.Folders, p.LooseFiles())
	}
}

// Пределы (только Load): хвост добавлений сверх MaxLooseAdds, затем файлов сверх MaxLooseFiles.
func TestFoldSourcesLimits(t *testing.T) {
	p := New()
	for i := range MaxLooseAdds + 10 {
		p.Loose = append(p.Loose, LooseAdd{ID: fmt.Sprintf("%012x", i), Files: files(at("a", fmt.Sprint(i, ".jpg")))})
	}
	p.FoldSources(nil)
	if len(p.Loose) != MaxLooseAdds || p.Loose[MaxLooseAdds-1].ID != fmt.Sprintf("%012x", MaxLooseAdds-1) {
		t.Fatalf("добавлений %d", len(p.Loose))
	}
	q := New()
	big := make([]string, MaxLooseFiles+5)
	for i := range big {
		big[i] = at("b", fmt.Sprint(i, ".jpg"))
	}
	q.Loose = []LooseAdd{{ID: "000000000001", Files: files(big[:10]...)}, {ID: "000000000002", Files: files(big[10:]...)}}
	q.FoldSources(nil)
	if n := len(q.LooseFiles()); n != MaxLooseFiles || q.LooseFiles()[n-1] != big[MaxLooseFiles-1] {
		t.Fatalf("файлов %d", n)
	}
}

// Второй вызов ничего не меняет (и не считает) — на всех наборах тестов выше.
func TestFoldSourcesIdempotent(t *testing.T) {
	p := New()
	p.Folders = []string{at("a", "b"), at("a"), at("c"), at("A")}
	p.FolderAlt = map[string]string{at("a", "b"): at("x"), at("c"): at("y")}
	p.Loose = []LooseAdd{{ID: "000000000001", Files: files(at("a", "1.jpg"), at("d", "2.jpg"), at("D", "2.jpg"))}, {ID: "000000000002"}}
	p.FoldSources(nil)
	before := Marshal(p)
	if merged, in := p.FoldSources(nil); merged+in != 0 || string(Marshal(p)) != string(before) {
		t.Fatalf("второй вызов: слито %d, в папках %d, проект изменился", merged, in)
	}
}

// ID записи: ровно 12 строчных hex.
func TestValidID(t *testing.T) {
	for id, want := range map[string]bool{NewID(): true, "0123456789ab": true, "0123456789AB": false, "0123456789a": false,
		"0123456789abc": false, "0123456789ag": false, "": false} {
		if ValidID(id) != want {
			t.Errorf("%q: %v", id, !want)
		}
	}
}
