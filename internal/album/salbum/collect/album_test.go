package collect

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

const libKey = "my:0123456789abcdef"

// scene — альбом для сбора: две папки с одинаковым именем «JPG» на разных «дисках» (у обеих
// IMG_0001.jpg; сайдкары обоих видов), подпапка, фото вне папок, фон, свой стикер из библиотеки,
// разметки обложки и разворотов, пропавшее фото; отметки — у фото альбома, у фото ленты вне
// альбома, у пропавшего и у чужого фото. Альбом прочитан Load (как в программе).
type scene struct {
	root, a, b string
	p          *model.Project
	marks      map[string]model.Mark
	extra      []string // лента: неиспользованное фото папки A, фото вне папок, фото альбома
	gone       string   // путь пропавшего фото
	unused     string
}

func newScene(t *testing.T) *scene {
	t.Helper()
	root := testkit.TempDir(t)
	sc := &scene{root: root, a: filepath.Join(root, "Диск1", "Свадьба", "JPG"), b: filepath.Join(root, "Диск2", "JPG")}
	a1, a2 := filepath.Join(sc.a, "IMG_0001.jpg"), filepath.Join(sc.a, "Ресторан", "IMG_0002.jpg")
	b1, bg := filepath.Join(sc.b, "IMG_0001.jpg"), filepath.Join(sc.a, "Ресторан", "bg.jpg")
	other := filepath.Join(root, "Прочее", "x.jpg")
	own := filepath.Join(root, "Библиотека", "0123456789abcdef.png")
	spreadGuide, coverGuide := filepath.Join(root, "Типография", "spreads.psd"), filepath.Join(root, "Типография", "cover.png")
	sc.gone, sc.unused = filepath.Join(root, "Нет", "gone.jpg"), filepath.Join(sc.a, "unused.jpg")
	for i, f := range []string{a1, a2, b1, bg, other, own, spreadGuide, coverGuide, sc.unused} {
		put(t, f, strings.Repeat(string(rune('a'+i)), 10+i))
	}
	put(t, a1+".xmp", "<xmp a1/>")
	put(t, strings.TrimSuffix(b1, ".jpg")+".xmp", "<xmp b1/>")
	edtest.LibFinder(t, map[string]string{libKey: own})

	p := model.New()
	p.Folders = []string{sc.a, sc.b}
	id := func(path string) string {
		st, _ := os.Stat(path)
		var size int64
		if st != nil {
			size = st.Size()
		}
		p.Photos[library.IDFor(path)] = model.PhotoRef{Path: path, W: 300, H: 200, Size: size}
		return library.IDFor(path)
	}
	ownID := model.LibPhotoID(libKey)
	p.Photos[ownID] = model.PhotoRef{Lib: libKey, Path: own, W: 100, H: 100, Size: 15}
	pr, _ := model.CoverPresetOf(model.CoverDefault)
	p.Cover = &model.Cover{Sheet: &model.Spread{ID: model.CoverID}, Spec: pr.Spec, Layout: model.CoverBoth, Guide: coverGuide}
	p.SpreadGuide = spreadGuide
	p.Spreads = []*model.Spread{
		{ID: "0123456789a1", Photos: []string{id(a1), id(b1)}},
		{ID: "0123456789a2", Photos: []string{id(a2), id(other)}, BgPhoto: &model.Backdrop{Photo: id(bg), W: 300, H: 200},
			Stickers: []*model.Sticker{{ID: "0123456789a3", Photo: ownID, X: 10, Y: 10, W: 30, H: 30}}},
		{ID: "0123456789a4", Photos: []string{id(sc.gone)}},
	}
	src := filepath.Join(root, "Альбом", "Иванов.salbum")
	if err := salbum.Save(p, src); err != nil {
		t.Fatal(err)
	}
	q, err := salbum.Load(src)
	if err != nil {
		t.Fatal(err)
	}
	sc.p = q
	sc.marks = map[string]model.Mark{}
	for _, f := range []string{a1, sc.unused, sc.gone, filepath.Join(root, "Чужое", "z.jpg")} {
		sc.marks[library.IDFor(f)] = model.Mark{Path: f, Rating: edtest.Ip(4)}
	}
	sc.extra = []string{sc.unused, other, a1, filepath.Join(root, "Чужое", "z.jpg")}
	return sc
}

func put(t testing.TB, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	// Время на час раньше: копия без переноса времени источника (Chtimes) не сошлась бы при повторе.
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
}

// collectTo — весь сбор, как его делает App: план с папкой, копирование, копия альбома, запись.
func collectTo(t *testing.T, sc *scene, dir string, all bool) (Plan, string) {
	t.Helper()
	var extra []string
	if all {
		extra = sc.extra
	}
	pl, err := Make(sc.p, extra, dir, os.Stat)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Copy(context.Background(), dir, &pl, nil); err != nil {
		t.Fatal(err)
	}
	q, marks := Album(sc.p, model.Side{Marks: sc.marks}, dir, pl)
	path := filepath.Join(dir, "Иванов.salbum")
	if err := Save(q, marks, path, pl); err != nil {
		t.Fatal(err)
	}
	return pl, path
}

// canon — альбом без путей: ID записей — по содержимому файла (пропавшее — по пути), пути,
// размеры и ключи библиотеки сняты. Одинаковый canon — тот же вид альбома.
func canon(t *testing.T, p *model.Project) string {
	t.Helper()
	c := p.Clone()
	for id, r := range c.Photos {
		r.Lib = ""
		c.Photos[id] = r
	}
	salbum.Rekey(c, func(path string) string {
		data, err := os.ReadFile(path)
		if err != nil {
			return "нет:" + library.IDFor(path)
		}
		sum := sha256.Sum256(data)
		return hex.EncodeToString(sum[:8])
	})
	for id, r := range c.Photos {
		if !strings.HasPrefix(id, "нет:") {
			r.Path, r.Alt, r.Size = "", "", 0
		}
		c.Photos[id] = r
	}
	c.Folders, c.FolderAlt = nil, nil
	c.SpreadGuide, c.SpreadGuideAlt = "", ""
	c.Cover.Guide, c.Cover.GuideAlt = "", ""
	return string(model.Marshal(c))
}

// lostOf — записи Photos, чьего файла нет.
func lostOf(p *model.Project) []string {
	var out []string
	for _, r := range p.Photos {
		if st, err := os.Stat(r.Path); r.Path == "" || err != nil || !st.Mode().IsRegular() {
			out = append(out, r.Path)
		}
	}
	return out
}

// Сц. 1–4, 10: структура папки, тот же вид, ни одного пропавшего сверх исходного, разметки и
// отметки на местах; повторное открытие даёт тот же альбом.
func TestCollectRoundtrip(t *testing.T) {
	sc := newScene(t)
	dir := filepath.Join(sc.root, "Флешка", "Иванов")
	pl, path := collectTo(t, sc, dir, false)
	for _, rel := range []string{
		"Иванов.salbum",
		"Фото/JPG/IMG_0001.jpg", "Фото/JPG/IMG_0001.jpg.xmp", "Фото/JPG/Ресторан/IMG_0002.jpg", "Фото/JPG/Ресторан/bg.jpg",
		"Фото/JPG (2)/IMG_0001.jpg", "Фото/JPG (2)/IMG_0001.xmp", "Фото/Прочие/x.jpg",
		"Свои фоны и стикеры/0123456789abcdef.png", "Разметка/spreads.psd", "Разметка/cover.png",
	} {
		if st, err := os.Lstat(filepath.Join(dir, filepath.FromSlash(rel))); err != nil || !st.Mode().IsRegular() {
			t.Errorf("нет %s: %v", rel, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "Фото", "JPG", "unused.jpg")); err == nil {
		t.Error("«Только фото альбома»: в папку попало фото вне альбома")
	}
	if len(pl.Missing) != 1 || pl.MissingGuides != 0 || pl.SideBusy != 0 {
		t.Fatalf("пропавшие %v, разметки %d, сайдкары %d", pl.Missing, pl.MissingGuides, pl.SideBusy)
	}
	wantIn := Info{Photos: 5, Own: 1, Guides: 2, Missing: 1, Bytes: pl.Bytes, FontsRestricted: []string{}}
	if in := pl.Info(); !reflect.DeepEqual(in, wantIn) || pl.Bytes == 0 {
		t.Fatalf("сводка: %+v", in)
	}
	lp, err := salbum.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if lost := lostOf(lp); len(lost) != 1 || lost[0] != sc.gone {
		t.Fatalf("пропавшие в собранном: %v", lost)
	}
	if got, want := canon(t, lp), canon(t, sc.p); got != want {
		t.Fatalf("вид альбома изменился:\n%s\n%s", got, want)
	}
	if lp.SpreadGuide != filepath.Join(dir, "Разметка", "spreads.psd") || lp.Cover == nil || lp.Cover.Guide != filepath.Join(dir, "Разметка", "cover.png") {
		t.Fatalf("разметки: %q, %+v", lp.SpreadGuide, lp.Cover)
	}
	if len(lp.Folders) != 1 || lp.Folders[0] != filepath.Join(dir, DirPhotos) {
		t.Fatalf("папки: %v", lp.Folders)
	}
	marks := edtest.Loaded(lp).Marks
	want := map[string]bool{filepath.Join(dir, "Фото", "JPG", "IMG_0001.jpg"): true, sc.gone: true}
	if len(marks) != len(want) {
		t.Fatalf("отметки: %v", marks)
	}
	for id, m := range marks {
		if !want[m.Path] || id != library.IDFor(m.Path) || m.Rating == nil || *m.Rating != 4 {
			t.Errorf("отметка %s: %+v", id, m)
		}
	}
	// Круг Save → Load собранного — тот же альбом.
	again := filepath.Join(dir, "again.salbum")
	if err := salbum.WriteRel(lp, again, dir, model.Side{Marks: marks}); err != nil {
		t.Fatal(err)
	}
	if rp, err := salbum.Load(again); err != nil || !bytes.Equal(model.Marshal(rp), model.Marshal(lp)) {
		t.Fatalf("второй круг: %v", err)
	}
}

// Сц. 10: собранную папку перенесли целиком — альбом открывается без новых пропавших.
func TestCollectMovedFolder(t *testing.T) {
	sc := newScene(t)
	dir := filepath.Join(sc.root, "Флешка", "Иванов")
	collectTo(t, sc, dir, true)
	moved := filepath.Join(sc.root, "Другой ПК", "Работа", "Иванов")
	if err := os.MkdirAll(filepath.Dir(moved), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(dir, moved); err != nil {
		t.Fatal(err)
	}
	lp, err := salbum.Load(filepath.Join(moved, "Иванов.salbum"))
	if err != nil {
		t.Fatal(err)
	}
	if lost := lostOf(lp); len(lost) != 1 || lost[0] != sc.gone {
		t.Fatalf("пропавшие после переноса: %v", lost)
	}
	if got, want := canon(t, lp), canon(t, sc.p); got != want {
		t.Fatalf("вид после переноса:\n%s\n%s", got, want)
	}
	// «Все фото»: отметка фото ленты вне альбома переехала вместе с ним.
	unused := filepath.Join(moved, "Фото", "JPG", "unused.jpg")
	if m, ok := edtest.Loaded(lp).Marks[library.IDFor(unused)]; !ok || m.Path != unused {
		t.Fatalf("отметка фото ленты: %v", edtest.Loaded(lp).Marks)
	}
}

// Сц. 4: в собранном файле нет abs и путей дисков — кроме пропавшего фото (его путь и отметка).
func TestCollectRelOnly(t *testing.T) {
	sc := newScene(t)
	_, path := collectTo(t, sc, filepath.Join(sc.root, "Флешка", "Иванов"), true)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte(`"abs"`)) || bytes.Contains(data, []byte(`Abs"`)) {
		t.Fatalf("в собранном альбоме есть запасные абсолютные пути:\n%s", data)
	}
	enc := func(s string) string { b, _ := json.Marshal(s); return strings.Trim(string(b), `"`) }
	root, gone, text := enc(sc.root), enc(sc.gone), string(data)
	if n, m := strings.Count(text, root), strings.Count(text, gone); n != m || m != 2 {
		t.Fatalf("абсолютных путей %d, из них пропавшего %d (ждали 2: фото и отметка):\n%s", n, m, data)
	}
	if vol := filepath.VolumeName(sc.root); vol != "" && strings.Count(text, vol) != 2 {
		t.Fatalf("буква диска %s встречается не только у пропавшего:\n%s", vol, data)
	}
}

// Сц. 9: своё из библиотеки — обычный файл из «Свои фоны и стикеры»: без lib, стикер на месте и
// рисуется по существующему файлу; на компьютере без библиотеки — тоже.
func TestCollectLibToFile(t *testing.T) {
	sc := newScene(t)
	dir := filepath.Join(sc.root, "Флешка", "Иванов")
	_, path := collectTo(t, sc, dir, false)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte(`"lib"`)) {
		t.Fatalf("в собранном альбоме остался ключ библиотеки:\n%s", data)
	}
	salbum.SetLibFinder(nil) // другой компьютер: библиотеки нет
	lp, err := salbum.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	st := lp.Spreads[1].Stickers
	if len(st) != 1 {
		t.Fatalf("стикеры: %+v", lp.Spreads[1])
	}
	ref, ok := lp.Photos[st[0].Photo]
	want := filepath.Join(dir, DirOwn, "0123456789abcdef.png")
	if !ok || ref.Lib != "" || ref.Path != want || st[0].Photo != library.IDFor(want) {
		t.Fatalf("стикер %q → %+v", st[0].Photo, ref)
	}
	if b := lp.Spreads[1].BgPhoto; b == nil || lp.Photos[b.Photo].Path != filepath.Join(dir, "Фото", "JPG", "Ресторан", "bg.jpg") {
		t.Fatalf("фон: %+v", lp.Spreads[1].BgPhoto)
	}
	if sc.p.Photos[model.LibPhotoID(libKey)].Lib != libKey {
		t.Fatal("Album изменил исходный проект")
	}
}

// Сц. 12: пустой альбом — собирается только .salbum, без папок; черновик без имени — «Альбом»
// (имя даёт App, здесь — что пустой план пишется).
func TestCollectEmpty(t *testing.T) {
	root := testkit.TempDir(t)
	dir := filepath.Join(root, "Альбом")
	p := model.New()
	pl, err := Make(p, nil, dir, os.Stat)
	if err != nil || len(pl.Items) != 0 || pl.Bytes != 0 || len(pl.Missing) != 0 {
		t.Fatalf("план пустого: %+v, %v", pl, err)
	}
	if n, err := Copy(context.Background(), dir, &pl, nil); err != nil || n != 0 {
		t.Fatalf("Copy пустого: %d, %v", n, err)
	}
	q, marks := Album(p, model.Side{}, dir, pl)
	if q.Folders != nil || len(marks.Marks) != 0 {
		t.Fatalf("папки %v, отметки %v", q.Folders, marks)
	}
	path := filepath.Join(dir, "Альбом.salbum")
	if err := Save(q, marks, path, pl); err != nil {
		t.Fatal(err)
	}
	ents, _ := os.ReadDir(dir)
	if len(ents) != 1 || ents[0].Name() != "Альбом.salbum" {
		t.Fatalf("в папке: %v", ents)
	}
}

// Save ловит ошибку плана: фото, которого нет в папке сбора, — ошибка, хотя альбом записан.
func TestCollectSaveCatchesLost(t *testing.T) {
	sc := newScene(t)
	dir := filepath.Join(sc.root, "Флешка", "Иванов")
	pl, err := Make(sc.p, nil, dir, os.Stat)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Copy(context.Background(), dir, &pl, nil); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, "Фото", "Прочие", "x.jpg")); err != nil {
		t.Fatal(err)
	}
	q, marks := Album(sc.p, model.Side{Marks: sc.marks}, dir, pl)
	if err := Save(q, marks, filepath.Join(dir, "Иванов.salbum"), pl); err == nil || !strings.Contains(err.Error(), "не нашлось фото: 1") {
		t.Fatalf("Save без файла: %v", err)
	}
}
