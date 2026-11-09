package exporter

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// rig — Host без App для ExportWith: вопросы и запомненные папки в журнале, фон — вручную или сразу.
// Host.Editor по умолчанию — ошибка теста: ExportWith с Job.Ed текущий редактор звать не должна.
type rig struct {
	t         *testing.T
	s         *Service
	asked     []string
	answer    func(msg string) bool
	remembers [][2]string
	bg        []func() // фон, не запущенный (sync == false)
	sync      bool
	hostEd    int // сколько раз позвали Host.Editor
}

func newRig(t *testing.T, hostEd *album.Editor) *rig {
	t.Helper()
	r := &rig{t: t, answer: func(string) bool { return true }}
	h, _, _ := fakeHost(t, func() *album.Editor { r.hostEd++; return hostEd })
	h.Ask = func(msg string) (bool, error) { r.asked = append(r.asked, msg); return r.answer(msg), nil }
	h.PickDir = func(string) (string, error) { t.Error("окно папки при Job.Pick"); return "", nil }
	h.RememberDir = func(dir, root string) { r.remembers = append(r.remembers, [2]string{dir, root}) }
	h.GoBG = func(fn func()) bool {
		if r.sync {
			fn()
		} else {
			r.bg = append(r.bg, fn)
		}
		return true
	}
	h.StopCtx = context.Background
	r.s = New(h)
	return r
}

// picks — Job.Pick, отдающий одну папку.
func picks(dir string, create bool, hits []string) func(string) (string, bool, []string, error) {
	return func(string) (string, bool, []string, error) { return dir, create, hits, nil }
}

func replaceAsked(asked []string) int {
	n := 0
	for _, q := range asked {
		if strings.Contains(q, "Заменить?") {
			n++
		}
	}
	return n
}

// photoAlbum — альбом n пустых разворотов с папкой фото folder.
func photoAlbum(t *testing.T, n int, folder string) *album.Editor {
	t.Helper()
	return spreadsEditor(t, n, func(p *model.Project) { p.Folders = []string{folder} })
}

// ExportWith(Job.Pick) идёт в выбранную заданием папку, окно ОС не зовёт; Export(o) — через
// Host.PickDir, как раньше (меню не меняется).
func TestExportWithPick(t *testing.T) {
	ed := spreadsEditor(t, 2, nil)
	dir := testkit.TempDir(t)
	r := newRig(t, nil)
	r.sync = true
	var title string
	ok, err := r.s.ExportWith(Job{Ed: ed, Rev: -1, Opt: Options{DPI: 72, Range: "1"}, Pick: func(ti string) (string, bool, []string, error) {
		title = ti
		return dir, false, nil, nil
	}})
	if !ok || err != nil || title != "Куда выгрузить" {
		t.Fatalf("ExportWith: %v, %v, окно %q", ok, err, title)
	}
	if _, err := os.Stat(filepath.Join(dir, "01.jpg")); err != nil {
		t.Fatalf("файл разворота: %v", err)
	}
	if r.hostEd != 0 {
		t.Fatalf("Host.Editor позван %d раз при Job.Ed", r.hostEd)
	}

	// Export(o): текущий редактор и окно ОС.
	dir2 := testkit.TempDir(t)
	m := newRig(t, ed)
	m.sync = true
	m.s.h.PickDir = func(string) (string, error) { return dir2, nil }
	if ok, err := m.s.Export(Options{DPI: 72, Range: "1"}); !ok || err != nil {
		t.Fatalf("Export: %v, %v", ok, err)
	}
	if _, err := os.Stat(filepath.Join(dir2, "01.jpg")); err != nil || m.hostEd != 1 {
		t.Fatalf("файл меню: %v; Host.Editor позван %d раз", err, m.hostEd)
	}
	if len(m.remembers) != 1 || m.remembers[0][0] != dir2 {
		t.Fatalf("запомнено %q", m.remembers)
	}
}

// Редактор задания — везде: Host.Editor отдаёт другой альбом (без папок фото, пустой), а проверка
// «рядом» (121), снимок и имя папки книги берутся из Job.Ed.
func TestExportWithEdEverywhere(t *testing.T) {
	photo := testkit.TempDir(t)
	ed := photoAlbum(t, 2, photo)
	other := blankEditor(t) // под ним «нечего экспортировать», без папок фото
	r := newRig(t, other)
	sub := filepath.Join(photo, "out")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if ok, err := r.s.ExportWith(Job{Ed: ed, Rev: -1, Opt: Options{DPI: 72}, Pick: picks(sub, false, nil)}); !ok || err != nil {
		t.Fatalf("ExportWith: %v, %v", ok, err)
	}
	if !slices.ContainsFunc(r.asked, func(q string) bool { return strings.Contains(q, "Выгрузить рядом") }) {
		t.Fatalf("вопроса 121 «рядом?» нет — папки фото чужого редактора: %q", r.asked)
	}
	if len(r.remembers) != 1 || filepath.Dir(r.remembers[0][0]) != filepath.Dir(photo) || r.remembers[0][1] != "" {
		t.Fatalf("запомнено %q: ждали соседнюю с папкой фото, корень не меняется", r.remembers)
	}
	if r.hostEd != 0 {
		t.Fatalf("Host.Editor позван %d раз", r.hostEd)
	}

	// Типография: имя книги — Job.Name (черновик без файла), альбом — Job.Ed (у Host.Editor запрет).
	book := book(200, 203, 2, "fabrika-photo")
	ed2, err := album.NewEditor(book, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.s.ExportWith(Job{Ed: other, Rev: -1, Opt: Options{Print: "fabrika"}, Pick: picks(testkit.TempDir(t), false, nil)}); err == nil {
		t.Fatal("пустой альбом в типографии должен отказать")
	}
	r2 := newRig(t, other)
	parent := testkit.TempDir(t)
	if ok, err := r2.s.ExportWith(Job{Ed: ed2, Rev: -1, Name: "Петровы", Opt: Options{Print: "fabrika"}, Pick: picks(parent, false, nil)}); !ok || err != nil {
		t.Fatalf("типография по Job.Ed: %v, %v", ok, err)
	}
	if fi, err := os.Stat(filepath.Join(parent, "Петровы")); err != nil || !fi.IsDir() {
		t.Fatalf("папка книги по Job.Name: %v", err)
	}
	if r2.hostEd != 0 {
		t.Fatalf("Host.Editor позван %d раз", r2.hostEd)
	}
}

// Ревизия сверяется со снимком: изменённый альбом — ErrAlbumEdited до всех окон и без флага busy;
// та же ревизия и −1 проходят.
func TestExportWithRevStale(t *testing.T) {
	ed := spreadsEditor(t, 2, nil)
	_, v := ed.Snapshot()
	p, _ := ed.Snapshot()
	s := p.Settings
	s.DPI = 150
	ed.SetSettings(s)
	r := newRig(t, nil)
	r.s.h.PickDir = func(string) (string, error) { t.Error("окно папки"); return "", nil }
	picked := false
	pick := func(string) (string, bool, []string, error) { picked = true; return "", false, nil, nil }
	ok, err := r.s.ExportWith(Job{Ed: ed, Rev: v.Rev, Opt: Options{}, Pick: pick})
	if ok || !errors.Is(err, ErrAlbumEdited) || picked || len(r.asked) != 0 {
		t.Fatalf("устаревшая ревизия: %v, %v, окно %v, вопросы %q", ok, err, picked, r.asked)
	}
	if busy, _ := r.s.Busy(); busy {
		t.Fatal("флаг busy остался")
	}
	_, now := ed.Snapshot()
	for _, rev := range []int{now.Rev, -1} {
		picked = false
		if _, err := r.s.ExportWith(Job{Ed: ed, Rev: rev, Opt: Options{}, Pick: pick}); errors.Is(err, ErrAlbumEdited) || !picked {
			t.Fatalf("ревизия %d: %v, окно %v", rev, err, picked)
		}
	}
}

// Новая папка выбранного пути: «нет» на вопросе — папки нет; «да» — создана после вопросов, корень
// выгрузок — её родитель.
func TestExportPickCreate(t *testing.T) {
	ed := spreadsEditor(t, 2, nil) // пустые развороты: предупреждение «пустые» — вопрос до выбора папки
	parent := testkit.TempDir(t)
	dir := filepath.Join(parent, "Петровы")
	r := newRig(t, nil)
	r.answer = func(string) bool { return false }
	if ok, err := r.s.ExportWith(Job{Ed: ed, Rev: -1, Opt: Options{DPI: 72}, Pick: picks(dir, true, nil)}); ok || err != nil {
		t.Fatalf("«нет»: %v, %v", ok, err)
	}
	if _, err := os.Lstat(dir); !os.IsNotExist(err) {
		t.Fatalf("папка создана без «да»: %v", err)
	}
	// «Нет» на вопросе после выбора папки (заготовки) тоже не оставляет папки: пустые места — до папки,
	// поэтому здесь проверяем порядок: Mkdir — после всех вопросов.
	r.answer = func(string) bool { return true }
	r.sync = true
	if ok, err := r.s.ExportWith(Job{Ed: ed, Rev: -1, Opt: Options{DPI: 72}, Pick: picks(dir, true, nil)}); !ok || err != nil {
		t.Fatalf("«да»: %v, %v", ok, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "01.jpg")); err != nil {
		t.Fatalf("файл в новой папке: %v", err)
	}
	if got := r.remembers[len(r.remembers)-1]; got != [2]string{dir, parent} {
		t.Fatalf("запомнено %q, ждали папку и корень-родителя", got)
	}
}

// Фото добавили между вопросом и щелчком: выбранный путь оказался в папке фото. Создаётся только
// соседняя папка 121, выбранная новая — нет.
func TestExportPickCreateMovedNext(t *testing.T) {
	photo := testkit.TempDir(t)
	ed := photoAlbum(t, 2, photo)
	dir := filepath.Join(photo, "Петровы") // нет на диске
	r := newRig(t, nil)
	r.sync = true
	if ok, err := r.s.ExportWith(Job{Ed: ed, Rev: -1, Opt: Options{DPI: 72}, Pick: picks(dir, true, nil)}); !ok || err != nil {
		t.Fatalf("ExportWith: %v, %v", ok, err)
	}
	if _, err := os.Lstat(dir); !os.IsNotExist(err) {
		t.Fatalf("создана папка среди фото: %v", err)
	}
	if next := r.s.LastDir(); filepath.Dir(next) != filepath.Dir(photo) || next == photo {
		t.Fatalf("записано в %q, ждали соседнюю с %q", next, photo)
	}
	if got := r.remembers[0]; got[1] != "" {
		t.Fatalf("«рядом» не должно менять корень: %q", got)
	}
}

// Папка появилась между вопросом и Mkdir: ошибка «не удалось создать папку», запись не идёт, флаг снят.
func TestExportPickCreateAppeared(t *testing.T) {
	ed := spreadsEditor(t, 2, nil)
	dir := filepath.Join(testkit.TempDir(t), "Петровы")
	r := newRig(t, nil)
	pick := func(string) (string, bool, []string, error) {
		if err := os.Mkdir(dir, 0o755); err != nil { // «появилась» после вопроса
			t.Fatal(err)
		}
		return dir, true, nil, nil
	}
	ok, err := r.s.ExportWith(Job{Ed: ed, Rev: -1, Opt: Options{DPI: 72}, Pick: pick})
	if ok || err == nil || !strings.Contains(err.Error(), "не удалось создать папку") {
		t.Fatalf("ждали ошибку создания: %v, %v", ok, err)
	}
	if len(r.bg) != 0 || r.s.LastDir() != "" || r.s.Last().State != StateIdle {
		t.Fatalf("запись началась: фон %d, папка %q, ход %+v", len(r.bg), r.s.LastDir(), r.s.Last())
	}
	if busy, _ := r.s.Busy(); busy {
		t.Fatal("флаг busy остался")
	}
}

// Набор своих файлов, названный вопросом («заменится»), заново не спрашивается; другой набор или
// вопрос без набора (меню) — спрашивается.
func TestExportHitsApproved(t *testing.T) {
	ed := spreadsEditor(t, 2, nil)
	dir := testkit.TempDir(t)
	r := newRig(t, nil)
	r.sync = true
	job := func(hits []string) Job {
		return Job{Ed: ed, Rev: -1, Opt: Options{DPI: 72}, Pick: picks(dir, false, hits)}
	}
	if ok, err := r.s.ExportWith(job(nil)); !ok || err != nil { // первая выгрузка: папка пуста
		t.Fatalf("первая: %v, %v", ok, err)
	}
	if replaceAsked(r.asked) != 0 {
		t.Fatalf("в пустой папке спросили о замене: %q", r.asked)
	}
	sm, err := r.s.Check(ed, Options{DPI: 72}, "")
	if err != nil {
		t.Fatal(err)
	}
	ours, foreign, err := sm.Hits(dir)
	if err != nil || len(foreign) != 0 || len(ours) != 2 {
		t.Fatalf("Hits: %q, %q, %v", ours, foreign, err)
	}
	cases := []struct {
		name string
		hits []string
		want int // вопросов о замене
	}{
		{"тот же набор, другой порядок", []string{ours[1], ours[0]}, 0},
		{"другой набор", []string{ours[0]}, 1},
		{"пустой набор при файлах", []string{}, 1},
		{"без набора (меню)", nil, 1},
	}
	for _, c := range cases {
		r.asked = nil
		if ok, err := r.s.ExportWith(job(c.hits)); !ok || err != nil {
			t.Fatalf("%s: %v, %v", c.name, ok, err)
		}
		if got := replaceAsked(r.asked); got != c.want {
			t.Errorf("%s: вопросов о замене %d, ждали %d (%q)", c.name, got, c.want, r.asked)
		}
	}
}

// Чужой файл с номером разворота: Hits говорит о нём, ExportWith отказывает — как меню.
func TestExportHitsForeign(t *testing.T) {
	ed := spreadsEditor(t, 2, nil)
	dir := testkit.TempDir(t)
	if err := os.WriteFile(filepath.Join(dir, "01.jpg"), []byte("чужой"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := newRig(t, nil)
	sm, err := r.s.Check(ed, Options{DPI: 72}, "")
	if err != nil {
		t.Fatal(err)
	}
	if ours, foreign, err := sm.Hits(dir); err != nil || len(ours) != 0 || !slices.Equal(foreign, []string{"01.jpg"}) {
		t.Fatalf("Hits: %q, %q, %v", ours, foreign, err)
	}
	if ok, err := r.s.ExportWith(Job{Ed: ed, Rev: -1, Opt: Options{DPI: 72}, Pick: picks(dir, false, nil)}); ok || err == nil || !strings.Contains(err.Error(), "не из Sosnina Albums") {
		t.Fatalf("ждали отказ из-за чужих файлов: %v, %v", ok, err)
	}
	if _, _, err := sm.Hits(filepath.Join(dir, "нет")); err == nil {
		t.Fatal("несуществующая папка: ждали ошибку чтения")
	}
}

// Корень выгрузок: созданная папка и папка с именем альбома — родитель; обычная — сама; «рядом» — не
// меняет.
func TestExportRememberRoot(t *testing.T) {
	root := testkit.TempDir(t)
	named := filepath.Join(root, "Петровы")
	plain := filepath.Join(root, "Выгрузки")
	for _, d := range []string{named, plain} {
		if err := os.Mkdir(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	p := model.New()
	p.Spreads = append(p.Spreads, &model.Spread{ID: "s1"})
	ed, err := album.NewEditor(p, filepath.Join(root, "ПЕТРОВЫ.salbum"))
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name         string
		dir          string
		create       bool
		wantR        string
		albumName    string
		wantRemember string
	}{
		{"существующая обычная — сама", plain, false, plain, "", plain},
		{"имя как у альбома без учёта регистра — родитель", named, false, root, "", named},
		{"созданная — родитель", filepath.Join(root, "Новая"), true, root, "", filepath.Join(root, "Новая")},
		{"имя от агента — родитель", plain, false, root, "Выгрузки", plain},
	}
	for _, c := range cases {
		r := newRig(t, nil)
		if _, err := r.s.ExportWith(Job{Ed: ed, Rev: -1, Name: c.albumName, Opt: Options{DPI: 72}, Pick: picks(c.dir, c.create, nil)}); err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if len(r.remembers) != 1 || r.remembers[0] != [2]string{c.wantRemember, c.wantR} {
			t.Errorf("%s: запомнено %q, ждали %q", c.name, r.remembers, [2]string{c.wantRemember, c.wantR})
		}
	}
	// Папка книги внутри «рядом» — корень не меняется (см. TestExportWithEdEverywhere); корень диска.
	if got := exportRoot(root, false, ""); got != root {
		t.Fatalf("exportRoot без совпадения: %q", got)
	}
	if got := exportRoot(string(filepath.Separator), true, ""); got != string(filepath.Separator) {
		t.Fatalf("корень диска остался корнем: %q", got)
	}
}

// Умолчание типографии как у окна: у альбома с обложкой Фабрики без запрета — fabrika, у пустого
// и у обычного — "". Редактор — переданный, а не Host.Editor.
func TestExportDefaultPrintKind(t *testing.T) {
	r := newRig(t, nil)
	plain := spreadsEditor(t, 2, nil)
	if got := r.s.DefaultPrint(plain); got != "" {
		t.Fatalf("обычный альбом: %q", got)
	}
	ed, err := album.NewEditor(book(200, 203, 12, "fabrika-photo"), "")
	if err != nil {
		t.Fatal(err)
	}
	if got := r.s.DefaultPrint(ed); got != "fabrika" {
		t.Fatalf("альбом с обложкой Фабрики: %q", got)
	}
	if plans := r.s.PrintPlansFor(ed, ""); len(plans) == 0 || r.hostEd != 0 {
		t.Fatalf("планы: %d, Host.Editor позван %d раз", len(plans), r.hostEd)
	}
}

// book — проект w×h из n пустых разворотов с обложкой preset ("" — без).
func book(w, h float64, n int, preset string) *model.Project {
	p := model.New()
	p.Settings.PageW, p.Settings.PageH, p.Settings.Safe = w, h, 10
	for range n {
		p.Spreads = append(p.Spreads, &model.Spread{ID: model.NewID(), Photos: []string{}})
	}
	if preset != "" {
		pr, _ := model.CoverPresetFor(preset, p.Settings)
		p.Cover = &model.Cover{Sheet: &model.Spread{ID: model.CoverID}, Spec: pr.Spec}
	}
	return p
}
