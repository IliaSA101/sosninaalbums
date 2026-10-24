package app

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/imgfile"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Копии фото (098 G2) в App: сведения запроса копии, миниатюры копий, перепривязка, профили и
// вопрос о пустых листах — по файлам.

// copiesApp — App с папкой из двух фото в ленте (сканирование дождано): ID файлов x, y.
func copiesApp(t *testing.T) (a *App, x, y string) {
	t.Helper()
	a = newApp(t)
	a.cfgDir = testkit.TempDir(t)
	a.emit = func(string, ...any) {}
	dir := testkit.TempDir(t)
	px := testkit.JPEG(t, dir, "x.jpg", 40, 30)
	py := testkit.JPEG(t, dir, "y.jpg", 30, 40)
	if _, _, err := a.editor().AddFolder(dir); err != nil {
		t.Fatal(err)
	}
	a.rescan()
	waitScan(t, a)
	// Scan.Active гаснет раньше хвоста чтения (warmThumbs читает a.cache): тесты сами ставят a.cache
	// после этой функции, поэтому ждём scanning=false под a.mu — иначе гонка записи с чтением фона.
	waitIdle(t, a)
	return a, library.IDFor(px), library.IDFor(py)
}

// Запрос копии «<ID>-new»: сведения файла из ленты, а без ленты — из любой записи семьи; ID —
// прежний запрос (его разбирает правка). Неизвестный файл и пустой вход — ничего.
func TestInfosCopyReq(t *testing.T) {
	a, x, y := copiesApp(t)
	if got := a.infos(nil); len(got) != 0 {
		t.Fatalf("пустой вход: %+v", got)
	}
	reqX, reqY := model.CopyReq(x), model.CopyReq(y)
	got := a.infos([]string{reqX, reqY, "ffffffffffffffff-new", x})
	if len(got) != 3 || got[0].ID != reqX || got[1].ID != reqY || got[2].ID != x {
		t.Fatalf("infos: %+v", got)
	}
	if got[0].Path != got[2].Path || got[0].W != 40 || got[0].H != 30 || got[0].Size == 0 {
		t.Fatalf("сведения запроса не файла: %+v, файл %+v", got[0], got[2])
	}
	// Два фото с одним файлом: исходная запись и копия на двух разворотах, один шаг отмены на каждый.
	if _, err := a.NewSpread(-1, []string{x}); err != nil {
		t.Fatal(err)
	}
	v, err := a.NewSpread(-1, []string{reqX, reqY})
	if err != nil || len(v.Spreads) != 2 {
		t.Fatalf("NewSpread с запросами: %v, %d разворотов", err, len(v.Spreads))
	}
	if ph := v.Spreads[1].Photos; !slices.Equal(ph, []string{model.CopyID(x, 2), y}) {
		t.Fatalf("второй разворот: %v", ph)
	}
	// Папка закрыта (лента пуста): сведения — из проекта, по семье; ID — запрос.
	a.mu.Lock()
	a.byID = map[string]library.Photo{}
	a.mu.Unlock()
	refs := a.infosOrRefs([]string{reqX, model.CopyID(x, 2), "ffffffffffffffff-new"})
	if len(refs) != 2 || refs[0].ID != reqX || refs[1].ID != model.CopyID(x, 2) ||
		refs[0].Path != refs[1].Path || refs[0].W != 40 || refs[0].Size == 0 {
		t.Fatalf("infosOrRefs без ленты: %+v", refs)
	}
	if v, err := a.AddPhotos(v.Spreads[1].ID, []string{reqX}, -1); err != nil ||
		!slices.Contains(v.Spreads[1].Photos, model.CopyID(x, 3)) {
		t.Fatalf("AddPhotos без ленты: %v %v", err, v.Spreads[1].Photos)
	}
}

// Миниатюра и превью копии — тот же файл ленты: реестр по ID файла (и когда запись копии уже
// ушла из проекта — плитка из очереди браузера не промах), без ленты — фото проекта с ID файла.
func TestMediaCopyThumb(t *testing.T) {
	a, x, _ := copiesApp(t)
	cache, err := library.NewCache(testkit.TempDir(t))
	if err != nil {
		t.Fatal(err)
	}
	a.cache = cache
	if _, err := a.NewSpread(-1, []string{x}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.NewSpread(-1, []string{model.CopyReq(x)}); err != nil {
		t.Fatal(err)
	}
	c2, c3 := model.CopyID(x, 2), model.CopyID(x, 3) // c3 нет в проекте
	a.mu.Lock()
	lib := a.byID[x]
	a.mu.Unlock()
	for _, id := range []string{x, c2, c3} {
		p, ok, gone := a.mediaPhoto(id)
		if !ok || gone || p.ID != x || p.Path != lib.Path {
			t.Fatalf("mediaPhoto(%s) = %+v, %v, %v", id, p, ok, gone)
		}
	}
	h := a.media(http.NotFoundHandler())
	for _, u := range []string{"/media/t/" + c2, "/media/p/" + c3} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("GET", u, nil))
		if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "image/jpeg" {
			t.Fatalf("%s: %d %q", u, rec.Code, rec.Header().Get("Content-Type"))
		}
	}
	// Без ленты: фото проекта, ID — файла (ключ кэша общий у семьи); записи нет — не найдено.
	a.mu.Lock()
	a.byID = map[string]library.Photo{}
	a.mu.Unlock()
	if p, ok, gone := a.mediaPhoto(c2); !ok || gone || p.ID != x || p.Path != lib.Path {
		t.Fatalf("без ленты: %+v, %v, %v", p, ok, gone)
	}
	if _, ok, gone := a.mediaPhoto(c3); ok || gone {
		t.Fatalf("без ленты и без записи: %v, %v", ok, gone)
	}
}

// Поиск недостающих в папке: один файл с копией на двух разворотах — «Найдено 1», обе записи
// переезжают; ненайденный файл с копией — «Не найдено 1»; полоса «Не найдено» — по записям
// (серые места обеих копий).
func TestRelinkFolderCountsFiles(t *testing.T) {
	root := testkit.TempDir(t)
	shoot := filepath.Join(root, "Съёмка")
	moved := filepath.Join(root, "Новая")
	pb := testkit.JPEG(t, moved, "b.jpg", 50, 30)
	oldB, oldC := filepath.Join(shoot, "b.jpg"), filepath.Join(shoot, "c.jpg")
	idB, idC := library.IDFor(oldB), library.IDFor(oldC)
	a := newApp(t)
	a.cfgDir = testkit.TempDir(t)
	a.emit = func(string, ...any) {}
	ed := a.editor()
	b := model.PhotoInfo{ID: idB, Path: oldB, W: 50, H: 30, Size: fileLen(t, pb)}
	c := model.PhotoInfo{ID: idC, Path: oldC, W: 50, H: 30, Size: 123}
	if _, err := ed.NewSpread(-1, []model.PhotoInfo{b, c}); err != nil {
		t.Fatal(err)
	}
	b.ID, c.ID = model.CopyReq(idB), model.CopyReq(idC)
	if _, err := ed.NewSpread(-1, []model.PhotoInfo{b, c}); err != nil {
		t.Fatal(err)
	}
	a.refreshMissing()
	waitMissing(t, a)
	if m := a.GetState().Missing; len(m) != 4 {
		t.Fatalf("недостающие до поиска: %v", m)
	}
	res, err := a.RelinkFolderPath(moved)
	if err != nil {
		t.Fatal(err)
	}
	if res.Found != 1 || res.Missing != 1 || res.Ambiguous != 0 {
		t.Fatalf("итог: %+v", res)
	}
	nb := library.IDFor(pb)
	want := [][]string{{nb, idC}, {model.CopyID(nb, 2), model.CopyID(idC, 2)}}
	for i, s := range res.View.Spreads {
		if !slices.Equal(s.Photos, want[i]) {
			t.Fatalf("разворот %d: %v, ждали %v", i+1, s.Photos, want[i])
		}
	}
	waitScan(t, a)
	waitMissing(t, a)
	if m := a.GetState().Missing; !slices.Equal(m, sorted(idC, model.CopyID(idC, 2))) {
		t.Fatalf("недостающие после поиска: %v", m)
	}
	// Повтор — найти нечего: прежние числа по файлам, проект не меняется.
	rev := a.editor().View().Rev
	if res, err := a.RelinkFolderPath(moved); err != nil || res.Found != 0 || res.Missing != 1 || res.View.Rev != rev {
		t.Fatalf("повтор: %+v, %v", res, err)
	}
}

// WhereUsed App — чтение Editor.WhereUsed: по ID файла, запросу и записи; без шага отмены.
func TestAppWhereUsed(t *testing.T) {
	a, x, y := copiesApp(t)
	if got := a.WhereUsed(nil); got == nil || len(got) != 0 {
		t.Fatalf("пустой вход: %#v", got)
	}
	if _, err := a.NewSpread(-1, []string{x, y}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.NewSpread(-1, []string{model.CopyReq(x)}); err != nil {
		t.Fatal(err)
	}
	rev := a.editor().View().Rev
	got := a.WhereUsed([]string{model.CopyReq(x), y, model.CopyID(x, 2)})
	if len(got) != 3 || got[0].ID != x || got[0].Places != 2 || got[0].Move != "" || len(got[0].Uses) != 2 ||
		got[1].ID != y || got[1].Places != 1 || got[1].Move != y || got[2].Places != 2 {
		t.Fatalf("WhereUsed: %+v", got)
	}
	if a.editor().View().Rev != rev {
		t.Fatal("чтение дало шаг")
	}
}

// Одна запись на файл: наименьший номер (исходная первой), порядок — по первому вхождению файла.
func TestOneOfFamily(t *testing.T) {
	x, y := "aaaaaaaaaaaaaaaa", "bbbbbbbbbbbbbbbb"
	for _, c := range []struct{ in, want []string }{
		{nil, []string{}},
		{[]string{x}, []string{x}},
		{[]string{x + "-3", y + "-2", x + "-2", x, y + "-10"}, []string{x, y + "-2"}},
		{[]string{y + "-10", y + "-9"}, []string{y + "-9"}},
	} {
		if got := oneOfFamily(c.in); !slices.Equal(got, c.want) {
			t.Errorf("oneOfFamily(%v) = %v, ждали %v", c.in, got, c.want)
		}
	}
}

// Профили — по файлам: копия не считается второй раз и не читает заголовок (фото из реестра по
// ID файла); копия файла без реестра читается один раз.
func TestColorReportByFile(t *testing.T) {
	dir := testkit.TempDir(t)
	adobe := testkit.ICCSegment(1, 1, testkit.MatrixProfile(testkit.XYZAdobe, 2.2, testkit.DescV2("Adobe RGB (1998)")))
	fAdobe := testkit.JPEGSegments(t, dir, "adobe.jpg", 20, 10, adobe)
	idA := library.IDFor(fAdobe)
	reg := "cccccccccccccccc"
	a := newApp(t)
	a.byID[reg] = library.Photo{ID: reg, Color: imgfile.ColorOther, Profile: "ProPhoto RGB"}
	pr := model.New()
	if conv, other := a.colorReport(pr); len(conv) != 0 || len(other) != 0 {
		t.Fatalf("пустой альбом: %v %v", conv, other)
	}
	gone := filepath.Join(dir, "нет-файла.jpg")
	pr.Photos = map[string]model.PhotoRef{
		reg: {Path: gone}, model.CopyID(reg, 2): {Path: gone}, model.CopyID(reg, 7): {Path: gone},
		idA: {Path: fAdobe}, model.CopyID(idA, 2): {Path: fAdobe},
	}
	conv, other := a.colorReport(pr)
	if len(conv) != 1 || conv["Adobe RGB"] != 1 || len(other) != 1 || other["ProPhoto RGB"] != 1 {
		t.Errorf("conv=%v other=%v", conv, other)
	}
}

// Пустой лист при фоне альбома: вопрос — «с фоном альбома», а не «белым» (0.23-g3g4 G4-4);
// пустая только обложка (фон альбома её не красит) — «белой», как без фона альбома.
func TestExportBlankAlbumBg(t *testing.T) {
	withBg := func(e *exportEnv) {
		p, _ := e.a.editor().SnapshotSide()
		p.AlbumBg = &model.AlbumBg{Pat: &model.Pattern{Kind: "dots"}}
		e.a.ed.Store(mustEditor(t, p, ""))
	}
	e := newExportEnv(t, 2)
	withBg(e)
	if _, err := e.a.editor().NewSpread(-1, nil); err != nil {
		t.Fatal(err)
	}
	e.answer = false
	if ok, err := e.a.Export(ExportOptions{DPI: 72}); ok || err != nil ||
		!slices.Equal(e.asked, []string{"Разворот 3 пустой — выгрузить с фоном альбома?"}) {
		t.Fatalf("Export = %v, %v; вопросы %q", ok, err, e.asked)
	}
	c := coverEnv(t, 1, "")
	c.a.SetCoverSpineBg("")
	withBg(c)
	c.answer = false
	if ok, err := c.a.Export(ExportOptions{DPI: 72, Cover: true}); ok || err != nil ||
		!slices.Equal(c.asked, []string{"Обложка пустая — выгрузить белой?"}) {
		t.Fatalf("обложка: Export = %v, %v; вопросы %q", ok, err, c.asked)
	}
	if _, err := os.Stat(c.dir); err == nil {
		t.Fatal("«нет» — папка выгрузки создана")
	}
}
