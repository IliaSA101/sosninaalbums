package app

import (
	"bytes"
	"errors"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/backup"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// orphanRig — альбом из двух разворотов по 3 фото, у которого убрали корневую папку (осталась
// подпапка): фото корня остаются на развороте («сироты»), но в ленту больше не попадают.
type orphanRig struct {
	a       *App
	root    string     // корневая папка (из списка убрана)
	sub     string     // подпапка, осталась в списке
	v       model.View // вид сразу после удаления папки
	orphans []string

	// События пишет фоновая проверка недостающих (checkMissing) — под замком; a.emit ставится
	// один раз до первого фонового запуска: подмена позже гоняется с горутинами rescan.
	mu     sync.Mutex
	events int           // сколько раз ушло событие view
	added  []FolderAdded // события folders:added по порядку
}

func newOrphanRig(t *testing.T) *orphanRig {
	t.Helper()
	r := &orphanRig{a: newApp(t)}
	r.a.cfgDir = testkit.TempDir(t)
	r.a.emit = func(name string, data ...any) {
		r.mu.Lock()
		defer r.mu.Unlock()
		switch name {
		case "view":
			r.events++
		case "folders:added":
			r.added = append(r.added, data[0].(FolderAdded))
		}
	}
	root := testkit.TempDir(t)
	sub := filepath.Join(root, "sub")
	r.root, r.sub = root, sub
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"r1.jpg", "r2.jpg", "r3.jpg"} {
		testkit.JPEG(t, root, n, 60, 40)
	}
	for _, n := range []string{"s1.jpg", "s2.jpg", "s3.jpg"} {
		testkit.JPEG(t, sub, n, 60, 40)
	}
	// Корень уже в списке, поэтому подпапка в него «входит» и сама не добавляется: сначала
	// корень (лента видит и его подпапку), сборка, потом корень убирают и добавляют подпапку.
	if _, err := r.a.AddFolderPath(root); err != nil {
		t.Fatal(err)
	}
	waitScan(t, r.a)
	var ids []string
	fromRoot := map[string]bool{}
	for _, p := range r.a.GetState().Photos {
		ids = append(ids, p.ID)
		fromRoot[p.ID] = filepath.Dir(p.Path) == root
	}
	if v, err := r.a.AutoBuild(ids, -1, model.AutoOpts{Spreads: 2, MaxPer: 3}); err != nil || len(v.Spreads) != 2 {
		t.Fatalf("сборка: %v, разворотов %d", err, len(v.Spreads))
	}
	// Корень убран (вместе с вложенными записями), осталась подпапка: фото корня стоят в альбоме,
	// но из ленты пропадают.
	r.a.RemoveFolder(root)
	if _, err := r.a.AddFolderPath(sub); err != nil {
		t.Fatal(err)
	}
	waitScan(t, r.a)
	if n := len(r.a.GetState().Photos); n != 3 {
		t.Fatalf("в ленте %d фото, ждали 3 из подпапки", n)
	}
	for _, p := range r.a.GetState().Photos {
		if fromRoot[p.ID] {
			t.Fatalf("фото корня %s осталось в ленте", p.ID)
		}
	}
	r.v = r.a.editor().View()
	for _, s := range r.v.Spreads {
		for _, id := range s.Photos {
			if fromRoot[id] {
				r.orphans = append(r.orphans, id)
			}
		}
	}
	if len(r.orphans) != 3 {
		t.Fatalf("сирот на разворотах %d, ждали 3", len(r.orphans))
	}
	return r
}

// viewEvents — сколько событий view ушло к этому моменту.
func (r *orphanRig) viewEvents() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.events
}

// addedSince — события folders:added после первых n.
func (r *orphanRig) addedSince(n int) []FolderAdded {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.added[n:])
}

// rev — нынешняя ревизия (отмена тоже растит rev, поэтому берём заново перед каждой правкой).
func (r *orphanRig) rev() int { return r.a.editor().View().Rev }

// orphanHome — индекс разворота с фото id в виде v, -1 — нет нигде.
func orphanHome(v model.View, id string) int {
	for i, s := range v.Spreads {
		if slices.Contains(s.Photos, id) {
			return i
		}
	}
	return -1
}

// sameLayout — раскладка фото по разворотам равна исходной (по составу, не по rev).
func (r *orphanRig) sameLayout(v model.View) bool {
	if len(v.Spreads) != len(r.v.Spreads) {
		return false
	}
	for i, s := range v.Spreads {
		if !slices.Equal(s.Photos, r.v.Spreads[i].Photos) {
			return false
		}
	}
	return true
}

// target — разворот, чужой для первой сироты.
func (r *orphanRig) target() string {
	return r.v.Spreads[1-orphanHome(r.v, r.orphans[0])].ID
}

// 092: фото альбома, чьей папки больше нет в ленте (убрали корень, осталась подпапка), переносится
// на «Авто» чужого разворота и в новый разворот — по сведениям проекта, как бросок на ручной лист.
// Каждая правка — один шаг отмены.
func TestOrphanAfterRemoveFolder(t *testing.T) {
	r := newOrphanRig(t)
	a, orphan := r.a, r.orphans[0]

	// AddPhotos сироты на чужой разворот: rev +1, фото на нём; отмена — один шаг.
	rev := r.rev()
	v, err := a.AddPhotos(r.target(), []string{orphan}, -1)
	if err != nil || v.Rev != rev+1 || v.Spreads[orphanHome(v, orphan)].ID != r.target() {
		t.Fatalf("AddPhotos сироты: err %v, rev %d→%d", err, rev, v.Rev)
	}
	if u := a.Undo(); !r.sameLayout(u) {
		t.Fatal("отмена AddPhotos одним шагом не вернула раскладку")
	}

	// NewSpread из сироты: rev +1, разворотов +1; отмена — один шаг.
	rev = r.rev()
	v, err = a.NewSpread(-1, []string{r.orphans[1]})
	if err != nil || v.Rev != rev+1 || len(v.Spreads) != len(r.v.Spreads)+1 {
		t.Fatalf("NewSpread сироты: err %v, rev %d→%d, разворотов %d", err, rev, v.Rev, len(v.Spreads))
	}
	if u := a.Undo(); !r.sameLayout(u) {
		t.Fatalf("отмена NewSpread одним шагом: разворотов %d", len(u.Spreads))
	}

	// FillPlaces сироты: договор grid.FillPlaces — фото альбома пропускаются; без ошибки, rev тот же.
	rev = r.rev()
	if fr, err := a.FillPlaces([]string{orphan}, ""); err != nil || fr.View.Rev != rev {
		t.Fatalf("FillPlaces сироты: err %v, rev %d→%d", err, rev, fr.View.Rev)
	}
}

// Несколько сирот сразу — один шаг, все на целевом развороте.
func TestOrphanAddSeveral(t *testing.T) {
	r := newOrphanRig(t)
	rev := r.rev()
	v, err := r.a.AddPhotos(r.target(), r.orphans, -1)
	if err != nil || v.Rev != rev+1 {
		t.Fatalf("AddPhotos всех сирот: err %v, rev %d→%d", err, rev, v.Rev)
	}
	for _, id := range r.orphans {
		// Опустевший разворот-источник уходит, индексы сдвигаются: ищем по ID разворота.
		if i := orphanHome(v, id); i < 0 || v.Spreads[i].ID != r.target() {
			t.Errorf("сирота %s не на целевом развороте", id)
		}
	}
	if u := r.a.Undo(); !r.sameLayout(u) {
		t.Fatal("отмена пачки одним шагом не вернула раскладку")
	}
}

// Неизвестный ID: ErrNoPhoto, rev прежний, события view нет, строка журнала без ID.
func TestOrphanUnknownID(t *testing.T) {
	r := newOrphanRig(t)
	var buf bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(prev) })
	rev, events := r.rev(), r.viewEvents()
	if _, err := r.a.AddPhotos(r.target(), []string{"нет"}, -1); !errors.Is(err, errNoPhoto) {
		t.Fatalf("AddPhotos неизвестного: err %v", err)
	}
	if _, err := r.a.NewSpread(-1, []string{"нет"}); !errors.Is(err, errNoPhoto) {
		t.Fatalf("NewSpread неизвестного: err %v", err)
	}
	if got := r.a.editor().View(); got.Rev != rev || len(got.Spreads) != len(r.v.Spreads) || r.viewEvents() != events {
		t.Fatalf("после отказов: rev %d→%d, событий view %d→%d", rev, got.Rev, events, r.viewEvents())
	}
	if !strings.Contains(buf.String(), "не найдено 1 из 1") || strings.Contains(buf.String(), "нет") {
		t.Errorf("журнал: %q", buf.String())
	}
}

// Сквозной сценарий 092: альбом формата v15 со списком [подпапка] и сиротами корня на разворотах.
// Папку-родителя добавили обратно — сироты вернулись в ленту, событие folders:added с Merged 1;
// отмена одним шагом возвращает список [подпапка] и ленту без сирот.
func TestOrphanReturnWithRoot(t *testing.T) {
	r := newOrphanRig(t)
	a := r.a
	file := filepath.Join(testkit.TempDir(t), "Альбом"+projectExt)
	if err := a.editor().Save(file); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	// Версия — первое такое поле файла (корень проекта); вложенные не трогаем.
	loc := regexp.MustCompile(`"version":\s*\d+`).FindIndex(raw)
	if loc == nil {
		t.Fatal("в файле нет поля version")
	}
	old := append(slices.Clone(raw[:loc[0]]), []byte(`"version": 15`)...)
	if err := os.WriteFile(file, append(old, raw[loc[1]:]...), 0o644); err != nil {
		t.Fatal(err)
	}

	// Считаем folders:added с этого места (сборка стенда свои уже прислала).
	base := len(r.addedSince(0))
	added := func() []FolderAdded { return r.addedSince(base) }
	a.backups = backup.New(filepath.Join(testkit.TempDir(t), "backups"))
	if _, err := a.openPath(file); err != nil {
		t.Fatal(err)
	}
	waitScan(t, a)
	if v := a.editor().View(); !slices.Equal(v.Folders, []string{r.sub}) || len(a.GetState().Photos) != 3 {
		t.Fatalf("после открытия v15: папки %v, в ленте %d", v.Folders, len(a.GetState().Photos))
	}
	inRibbon := func() map[string]bool {
		m := map[string]bool{}
		for _, p := range a.GetState().Photos {
			m[p.ID] = true
		}
		return m
	}
	for _, id := range r.orphans {
		if inRibbon()[id] || orphanHome(a.editor().View(), id) < 0 {
			t.Fatalf("сирота %s: в ленте %v, на развороте %d", id, inRibbon()[id], orphanHome(a.editor().View(), id))
		}
	}

	rev := r.rev()
	v, err := a.AddFolderPath(r.root)
	if err != nil || v.Rev != rev+1 || !slices.Equal(v.Folders, []string{r.root}) {
		t.Fatalf("AddFolderPath(корень): err %v, rev %d→%d, папки %v", err, rev, v.Rev, v.Folders)
	}
	waitScan(t, a)
	for _, id := range r.orphans {
		if !inRibbon()[id] {
			t.Errorf("сирота %s не вернулась в ленту", id)
		}
	}
	if n := len(a.GetState().Photos); n != 6 {
		t.Errorf("в ленте %d фото, ждали 6", n)
	}
	if evs := added(); len(evs) != 1 || evs[0] != (FolderAdded{Dir: r.root, Merged: 1}) {
		t.Errorf("folders:added %+v, ждали одно с Merged 1", evs)
	}

	u := a.Undo()
	waitScan(t, a)
	if !slices.Equal(u.Folders, []string{r.sub}) || !r.sameLayout(u) {
		t.Fatalf("после отмены: папки %v, раскладка прежняя %v", u.Folders, r.sameLayout(u))
	}
	for _, id := range r.orphans {
		if inRibbon()[id] {
			t.Errorf("сирота %s осталась в ленте после отмены", id)
		}
	}
	if n := len(a.GetState().Photos); n != 3 {
		t.Errorf("после отмены в ленте %d фото, ждали 3", n)
	}
	if len(added()) != 1 {
		t.Errorf("отмена прислала folders:added: %d", len(added()))
	}
}
