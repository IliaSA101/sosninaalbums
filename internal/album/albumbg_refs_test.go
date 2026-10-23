package album_test

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum/collect"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Страж обходов ссылок на фото (098 design §4.3, G5): фото, стоящее только в фоне альбома, должен
// видеть каждый обход — иначе запись фото снесут или потеряют при переезде и сборе. Новый обход
// ссылок добавляется сюда же. Обход export.MissingPhotos — в export (album его не импортирует).

const refsLibKey = "my:00000000000000cd"

// bgCase — фото фона альбома: info — как его шлёт интерфейс, id — запись в Photos сейчас.
type bgCase struct {
	info  model.PhotoInfo
	id    string
	fresh model.PhotoInfo // фото ленты, не стоящее в альбоме
	a     model.PhotoInfo // фото в месте первого разворота
}

// seen — каждый обход видит фото фона: запись цела, «где стоит» и «Использовано», набор
// «в альбоме», Refs, автосборка и «Заполнить пустые» его не берут (и не меняют проект).
func (c *bgCase) seen(t *testing.T, e *album.Editor, what string) {
	t.Helper()
	p := edtest.P(e)
	if p.AlbumBg == nil || p.AlbumBg.Photo != c.id {
		t.Fatalf("%s: фон альбома %+v, ждали фото %s", what, p.AlbumBg, c.id)
	}
	if _, ok := p.Photos[c.id]; !ok {
		t.Fatalf("%s: запись фото фона снесена", what)
	}
	uses, text := e.PhotoUses(c.id)
	if len(uses) != 1 || uses[0].Sheet != -1 || uses[0].Label != "Фон альбома" || text != "Использовано 1 раз" {
		t.Fatalf("%s: Uses %+v, %q", what, uses, text)
	}
	if w := e.WhereUsed([]string{c.id})[0]; w.Places != 0 || w.Move != "" || len(w.Uses) != 1 {
		t.Fatalf("%s: WhereUsed %+v", what, w)
	}
	if !slices.Contains(e.View().BackdropUsed, c.id) || !p.OnDecor()[c.id] {
		t.Fatalf("%s: фото фона не в наборе «в альбоме»", what)
	}
	if _, ok := e.Refs()[c.id]; !ok {
		t.Fatalf("%s: Refs без фото фона", what)
	}
	plan, err := e.AutoPlan([]model.PhotoInfo{c.info, c.fresh}, model.AutoOpts{Spreads: 1, MaxPer: 64})
	if err != nil || plan.Photos != 1 {
		t.Fatalf("%s: автосборка взяла %d фото, ждали 1 (фон альбома не берёт): %v", what, plan.Photos, err)
	}
	rev := edtest.Rev(e)
	if _, left, _ := e.FillPlaces([]model.PhotoInfo{c.info}, ""); left != 0 || edtest.Rev(e) != rev || edtest.P(e).Placed(c.id) {
		t.Fatalf("%s: «Заполнить пустые» взяло фото фона (left %d)", what, left)
	}
}

// refsScene — альбом: разворот 1 с фото a, разворот-сетка с пустым местом (для «Заполнить
// пустые»), фон альбома — фото kind ("file" — файл ленты, "lib" — своё из библиотеки). У "lib"
// файла в библиотеке пока нет: путь пуст.
func refsScene(t *testing.T, kind string) (*album.Editor, *bgCase, string) {
	t.Helper()
	dir := testkit.TempDir(t)
	mk := func(name string) model.PhotoInfo {
		path := testkit.JPEG(t, filepath.Join(dir, "old"), name, 60, 40)
		return model.PhotoInfo{ID: library.IDFor(path), Path: path, W: 60, H: 40}
	}
	c := &bgCase{a: mk("a.jpg"), fresh: mk("c.jpg")}
	b := mk("b.jpg")
	e := edtest.Open(t, model.New(), "")
	edtest.NewSpread(e, -1, []model.PhotoInfo{c.a})
	edtest.NewSpread(e, -1, []model.PhotoInfo{b})
	grid := edtest.P(e).Spreads[1].ID
	e.SetGrid(grid, true)
	e.RemovePhotos([]string{b.ID}) // место сетки пустеет
	c.info = mk("bg.jpg")
	if kind == "lib" {
		c.info = model.PhotoInfo{ID: model.LibPhotoID(refsLibKey), Lib: refsLibKey, W: 60, H: 40}
		edtest.LibFinder(t, map[string]string{})
	}
	c.id = c.info.ID
	if _, err := e.SetAlbumBg(model.BgSource{Photo: c.info}, false); err != nil {
		t.Fatal(err)
	}
	return e, c, dir
}

// TestAlbumBgRefs — фото только в фоне альбома: любая правка, отмена, круг Save→Load, переезд
// файла (Rekey), сбор альбома, «Убрать из альбома» — запись цела и видна каждому обходу.
func TestAlbumBgRefs(t *testing.T) {
	for _, kind := range []string{"file", "lib"} {
		t.Run(kind, func(t *testing.T) {
			e, c, dir := refsScene(t, kind)
			c.seen(t, e, "начало")
			refsEdits(t, e, c)
			refsRound(t, e, c, "круг записи")
			refsMove(t, e, c, kind, dir)
			refsRound(t, e, c, "круг после переезда")
			refsCollect(t, e, c, dir)
			refsFrame(t, e, c)
			refsPurge(t, e, c)
		})
	}
}

// refsEdits — разные правки подряд (каждая прогоняет PruneRefs) и отмена с возвратом.
func refsEdits(t *testing.T, e *album.Editor, c *bgCase) {
	t.Helper()
	s1 := edtest.P(e).Spreads[0].ID
	steps := []struct {
		name string
		do   func()
	}{
		{"фото в разворот", func() { edtest.AddPhotos(e, s1, []model.PhotoInfo{c.fresh}, -1) }},
		{"фото убрано", func() { e.RemovePhotos([]string{c.fresh.ID}) }},
		{"прозрачность", func() { e.SetAlbumBgFade(0.4, "") }},
		{"не показывать здесь", func() { e.SetNoAlbumBg(s1, true) }},
		{"узор листа", func() { e.ApplyPattern(s1, model.ScopeSpread, model.Pattern{Kind: "dots"}, "") }},
		{"новый разворот", func() { edtest.NewSpread(e, -1, []model.PhotoInfo{c.fresh}) }},
		{"новый разворот убран", func() { e.DeleteSpread(edtest.P(e).Spreads[2].ID) }},
	}
	for _, s := range steps {
		undo := edtest.UndoLen(e)
		s.do()
		if edtest.UndoLen(e) != undo+1 {
			t.Fatalf("%s: шагов отмены %d → %d, ждали +1", s.name, undo, edtest.UndoLen(e))
		}
		c.seen(t, e, s.name)
	}
	for range steps[3:] { // отмена четырёх последних правок и возврат
		e.Undo()
		c.seen(t, e, "отмена")
	}
	for range steps[3:] {
		e.Redo()
		c.seen(t, e, "возврат")
	}
	// Фон скрыт на всех листах: ни один лист его не рисует, но фото всё равно «в альбоме».
	for _, s := range edtest.P(e).Spreads {
		e.SetNoAlbumBg(s.ID, true)
	}
	if slices.ContainsFunc(edtest.P(e).Spreads, func(s *model.Spread) bool { return !s.NoAlbumBg }) {
		t.Fatal("фон альбома скрыт не везде")
	}
	c.seen(t, e, "скрыт на всех листах")
	for _, s := range edtest.P(e).Spreads {
		e.SetNoAlbumBg(s.ID, false)
	}
	c.seen(t, e, "показан на всех листах")
}

// refsRound — круг Save→Load: фон альбома и запись целы, а открытый заново альбом по-прежнему
// их видит. Запись при чтении не должна удаляться до первого PruneRefs (порядок decodeFile).
func refsRound(t *testing.T, e *album.Editor, c *bgCase, what string) {
	t.Helper()
	q := edtest.SaveLoad(t, e)
	if q.AlbumBg == nil || q.AlbumBg.Photo != c.id || q.AlbumBg.Fade != 0.4 {
		t.Fatalf("%s: фон альбома %+v", what, q.AlbumBg)
	}
	if _, ok := q.Photos[c.id]; !ok {
		t.Fatalf("%s: запись фото фона не прочитана", what)
	}
	c.seen(t, edtest.Open(t, q, ""), what)
}

// refsMove — «переезд» файла: фото ленты — Relink (новые ID по путям, Rekey), своё — RelinkLib
// (файл нашёлся в библиотеке; ID от пути не зависит). Вместе с фоном переезжает и фото в месте.
func refsMove(t *testing.T, e *album.Editor, c *bgCase, kind, dir string) {
	t.Helper()
	old, undo := *c, edtest.UndoLen(e)
	if kind == "lib" {
		path := testkit.JPEG(t, filepath.Join(dir, "lib"), "bg.jpg", 60, 40)
		edtest.LibFinder(t, map[string]string{refsLibKey: path})
		e.RelinkLib(nil)
		if r, _ := e.Ref(c.id); r.Path != path {
			t.Fatalf("RelinkLib: путь %q, ждали %q", r.Path, path)
		}
		c.info.Path = path
	} else {
		move := func(ph model.PhotoInfo) model.PhotoInfo {
			ph.Path = testkit.JPEG(t, filepath.Join(dir, "new"), filepath.Base(ph.Path), 60, 40)
			ph.ID = library.IDFor(ph.Path)
			return ph
		}
		na, nbg := move(c.a), move(c.info)
		e.Relink(map[string]string{c.a.ID: na.Path, c.id: nbg.Path}, "")
		p := edtest.P(e)
		if _, ok := p.Photos[c.id]; ok || !slices.Contains(p.Spreads[0].Photos, na.ID) {
			t.Fatalf("Relink: старая запись фона цела или фото в месте не переехало: %v", p.Spreads[0].Photos)
		}
		c.a, c.info, c.id = na, nbg, nbg.ID
	}
	if edtest.UndoLen(e) != undo+1 {
		t.Fatalf("переезд: шагов отмены %d → %d", undo, edtest.UndoLen(e))
	}
	c.seen(t, e, "после переезда")
	e.Undo()
	old.seen(t, e, "отмена переезда")
	e.Redo()
	c.seen(t, e, "возврат переезда")
}

// refsCollect — сбор альбома: файл фона — в «Свои фоны и стикеры», запись смотрит на новый путь
// без ключа библиотеки, ID — по новому пути; пропавший файл — в Missing, фон остаётся.
func refsCollect(t *testing.T, e *album.Editor, c *bgCase, dir string) {
	t.Helper()
	p := edtest.P(e)
	pl, err := collect.Make(p, nil, "", os.Stat)
	if err != nil {
		t.Fatal(err)
	}
	i := slices.IndexFunc(pl.Items, func(it collect.Item) bool { return it.Src == c.info.Path })
	if i < 0 || pl.Items[i].Kind != collect.KindOwn || !slices.Equal(pl.Items[i].Refs, []string{c.id}) {
		t.Fatalf("сбор: элемент фона альбома %+v", pl.Items)
	}
	if in := pl.Info(); in.Own != 1 || in.Photos != 1 || in.Missing != 0 {
		t.Fatalf("сбор: сводка %+v", in)
	}
	out := filepath.Join(dir, "out")
	q, _ := collect.Album(p, model.Side{}, out, pl)
	np := filepath.Join(out, filepath.FromSlash(pl.Items[i].Rel))
	if q.AlbumBg == nil || q.AlbumBg.Photo != library.IDFor(np) {
		t.Fatalf("сбор: фон альбома %+v, ждали ID по пути %s", q.AlbumBg, np)
	}
	if r := q.Photos[q.AlbumBg.Photo]; r.Path != np || r.Lib != "" {
		t.Fatalf("сбор: запись фона %+v", r)
	}
	if p.AlbumBg.Photo != c.id {
		t.Fatal("сбор изменил исходный проект")
	}
	if err := os.Rename(c.info.Path, c.info.Path+".gone"); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Rename(c.info.Path+".gone", c.info.Path); err != nil {
			t.Fatal(err)
		}
	}()
	lost, err := collect.Make(p, nil, "", os.Stat)
	if err != nil || !slices.Equal(lost.Missing, []string{c.id}) {
		t.Fatalf("сбор без файла: Missing %v, %v", lost.Missing, err)
	}
	if q, _ := collect.Album(p, model.Side{}, out, lost); q.AlbumBg == nil || q.AlbumBg.Photo != c.id {
		t.Fatalf("сбор без файла: фон альбома %+v", q.AlbumBg)
	}
}

// refsFrame — фото, стоявшее и в месте, и фоном альбома, при уходе из мест теряет кадр места, но
// остаётся записью (PruneRefs: фон альбома — «на оформлении»).
func refsFrame(t *testing.T, e *album.Editor, c *bgCase) {
	t.Helper()
	s1 := edtest.P(e).Spreads[0].ID
	edtest.AddPhotos(e, s1, []model.PhotoInfo{c.info}, -1)
	edtest.Change(e, func(p *model.Project) {
		r := p.Photos[c.id]
		r.Frame, r.Asp = &model.Frame{S: 2}, 1.5
		p.Photos[c.id] = r
	})
	if r, _ := e.Ref(c.id); r.Frame == nil || r.Asp == 0 {
		t.Fatalf("кадр не поставлен: %+v", r)
	}
	e.RemovePhotos([]string{c.id})
	if r, ok := e.Ref(c.id); !ok || r.Frame != nil || r.Asp != 0 {
		t.Fatalf("фото ушло из мест, но осталось фоном: запись %+v, есть %v", r, ok)
	}
	c.seen(t, e, "после ухода из мест")
}

// refsPurge — «Убрать из альбома»: фон альбома и фото в месте уходят одним шагом, отмена
// возвращает всё; неизвестный ID и пустой список — холостая без шага.
func refsPurge(t *testing.T, e *album.Editor, c *bgCase) {
	t.Helper()
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	e.PurgePhotos([]string{"нет такого", ""})
	e.PurgePhotos(nil)
	if edtest.Rev(e) != rev || edtest.UndoLen(e) != undo {
		t.Fatalf("холостая правка дала rev %d → %d, шагов %d → %d", rev, edtest.Rev(e), undo, edtest.UndoLen(e))
	}
	e.PurgePhotos([]string{c.id, c.a.ID})
	p := edtest.P(e)
	_, bg := p.Photos[c.id]
	_, a := p.Photos[c.a.ID]
	if p.AlbumBg != nil || bg || a || p.Placed(c.a.ID) || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("после «Убрать»: фон %+v, записи фона %v и фото %v, шагов %d → %d", p.AlbumBg, bg, a, undo, edtest.UndoLen(e))
	}
	e.Undo()
	c.seen(t, e, "отмена «Убрать»")
	if !edtest.P(e).Placed(c.a.ID) {
		t.Fatal("отмена не вернула фото в место")
	}
}

// TestAlbumBgRefsPattern — края: фон-узор записей не имеет, и ни один обход не принимает пустой ID
// за фото фона; фон альбома с фото без записи (повреждённый файл) при чтении снимается.
func TestAlbumBgRefsPattern(t *testing.T) {
	ph := copyInfos(t, 1)
	e := edtest.Open(t, model.New(), "")
	edtest.NewSpread(e, -1, ph)
	setAlbumBg(t, e, model.BgSource{Pattern: &albumBgPat}, false)
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	e.PurgePhotos([]string{""})
	e.Relink(map[string]string{"": edtest.Abs("x.jpg")}, "")
	if edtest.Rev(e) != rev || edtest.UndoLen(e) != undo || edtest.P(e).AlbumBg == nil {
		t.Fatalf("узор фона альбома тронут пустым ID: rev %d → %d", rev, edtest.Rev(e))
	}
	if p := edtest.P(e); p.OnDecor()[""] || len(p.Photos) != 1 || len(e.Refs()) != 1 {
		t.Fatalf("узор породил записи: %v, %v", p.OnDecor(), p.Photos)
	}
	if uses, _ := e.PhotoUses(""); len(uses) != 0 {
		t.Fatalf("пустой ID нашёл вхождения: %+v", uses)
	}
	if q := edtest.SaveLoad(t, e); q.AlbumBg == nil || q.AlbumBg.Pat == nil || len(q.Photos) != 1 {
		t.Fatalf("круг записи узора: %+v", q.AlbumBg)
	}
	edtest.Change(e, func(p *model.Project) { p.AlbumBg = &model.AlbumBg{Photo: "ghost"} })
	if q := edtest.SaveLoad(t, e); q.AlbumBg != nil {
		t.Fatalf("фон альбома без записи фото прочитан: %+v", q.AlbumBg)
	}
}

// TestAlbumBgRefsFamily — фото фона альбома — копия файла, стоящего в месте: «где стоит» и Rekey
// ведут семью вместе, сбор кладёт файл один раз и сохраняет номер копии в ID.
func TestAlbumBgRefsFamily(t *testing.T) {
	dir := testkit.TempDir(t)
	mk := func(sub string) model.PhotoInfo {
		path := testkit.JPEG(t, filepath.Join(dir, sub), "a.jpg", 60, 40)
		return model.PhotoInfo{ID: library.IDFor(path), Path: path, W: 60, H: 40}
	}
	a := mk("old")
	a2 := model.CopyID(a.ID, 2)
	e := edtest.Open(t, model.New(), "")
	edtest.NewSpread(e, -1, []model.PhotoInfo{a})
	edtest.Change(e, func(p *model.Project) {
		p.Photos[a2] = p.Photos[a.ID]
		p.AlbumBg = &model.AlbumBg{Photo: a2}
	})
	uses, text := e.PhotoUses(a.ID)
	sp := edtest.P(e).Spreads[0].ID
	want := []model.PhotoUse{
		{Sheet: -1, Label: "Фон альбома", Kind: model.UseAlbumBg, Page: "Фон альбома"},
		{Sheet: 0, Label: "Разворот 1 — место", Kind: model.UsePlace, Spread: sp, Page: "Разворот 1", What: "место", Pick: a.ID},
	}
	if !slices.Equal(uses, want) || text != "Использовано 2 раза" {
		t.Fatalf("вхождения семьи: %+v, %q", uses, text)
	}
	if w := e.WhereUsed([]string{a.ID})[0]; w.Places != 1 || w.Move != a.ID || len(w.Uses) != 2 {
		t.Fatalf("WhereUsed: %+v", w)
	}
	if q := edtest.SaveLoad(t, e); q.AlbumBg == nil || q.AlbumBg.Photo != a2 || !q.Placed(a.ID) {
		t.Fatalf("круг записи: %+v", q.AlbumBg)
	}

	na := mk("new")
	e.Relink(map[string]string{a.ID: na.Path}, "")
	nb := model.CopyID(na.ID, 2)
	p := edtest.P(e)
	if _, ok := p.Photos[a2]; ok || p.AlbumBg == nil || p.AlbumBg.Photo != nb || !p.Placed(na.ID) || p.Photos[nb].Path != na.Path {
		t.Fatalf("Relink: фон %+v, записи %v", p.AlbumBg, p.Photos)
	}
	if err := edtest.FamilyInvariant(p); err != nil {
		t.Fatal(err)
	}

	pl, err := collect.Make(p, nil, "", os.Stat)
	if err != nil || len(pl.Items) != 1 || pl.Items[0].Kind != collect.KindPhoto || len(pl.Items[0].Refs) != 2 {
		t.Fatalf("сбор: файл должен лечь один раз, вместе с фото: %+v, %v", pl.Items, err)
	}
	out := filepath.Join(dir, "out")
	q, _ := collect.Album(p, model.Side{}, out, pl)
	id := library.IDFor(filepath.Join(out, filepath.FromSlash(pl.Items[0].Rel)))
	if q.AlbumBg == nil || q.AlbumBg.Photo != model.CopyID(id, 2) || !q.Placed(id) {
		t.Fatalf("сбор: фон %+v, ждали копию %s", q.AlbumBg, id)
	}
}
