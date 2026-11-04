package app

import (
	"errors"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// coverLookApp — lookApp с фото «a» в альбоме и макетом «Свадьба» (с обложкой), поставленным на альбом.
func coverLookApp(t *testing.T) (a *App, id string) {
	t.Helper()
	a = lookApp(t)
	p, _ := a.editor().Snapshot()
	p.Spreads[0].Photos = []string{"a"}
	p.Photos["a"] = model.PhotoRef{Path: testkit.AbsPath("a.jpg"), W: 300, H: 200}
	a.setEditor(mustEditor(t, p, ""))
	id = saveLook(t, a, "Свадьба", "#336699")
	if _, err := a.ApplyLook(id, model.LookOpts{Mode: model.LookAdd, Spreads: 3}); err != nil {
		t.Fatal(err)
	}
	return a, id
}

// Макет есть: оформление встало, Styled и имя; фото броска — копией в месте лицевой (145: фото
// уже в альбоме), фона нет; один шаг отмены; «Без оформления» — Undo и CreateCover, фото фоном
// лицевой; круг Save→Load держит обложку.
func TestCreateCoverLookApp(t *testing.T) {
	a, _ := coverLookApp(t)
	r, err := a.CreateCoverLook("", "a")
	if err != nil || !r.Styled || r.Look != "Свадьба" || r.View.Cover == nil {
		t.Fatalf("CreateCoverLook: %+v %v", r, err)
	}
	if r.View.Seq == 0 { // вид идёт по цепочке событий (a.changed)
		t.Fatalf("вид мимо цепочки: Seq=%d", r.View.Seq)
	}
	c := r.View.Cover.Sheet
	if c.BgSet[0] != "#336699" || len(c.Backdrops) != 0 {
		t.Fatalf("оформление: фон %q, фоны фото %+v", c.BgSet[0], c.Backdrops)
	}
	styled := projectJSON(t, a)
	p, _ := a.editor().Snapshot()
	seat := p.CoverSeats()
	if len(seat) != 1 {
		t.Fatalf("мест лицевой: %d, ждали 1", len(seat))
	}
	if model.CopyNo(seat[0].Photo) < 2 || p.Photos[seat[0].Photo].Path != p.Photos["a"].Path || p.Spreads[0].Photos[0] != "a" {
		t.Fatalf("фото не копией в месте лицевой: место %q, разворот %v", seat[0].Photo, p.Spreads[0].Photos)
	}
	if q := reloaded(t, p); q.Cover == nil || q.Cover.Sheet.BgL != "#336699" || q.Look == nil || len(q.CoverSeats()) != 1 || q.CoverSeats()[0].Photo != seat[0].Photo {
		t.Fatalf("после круга: %+v", q.Cover)
	}
	// Один шаг отмены: Undo снимает обложку целиком, макет альбома остаётся.
	if v := a.Undo(); v.Cover != nil || v.Look == nil {
		t.Fatalf("Undo: обложка %v, макет %v", v.Cover, v.Look)
	}
	// «Без оформления»: обычная CreateCover с тем же фото — фон лицевой, но без цвета макета.
	v, err := a.CreateCover("", "a")
	if err != nil || v.Cover == nil || v.Cover.Sheet.BgSet[0] == "#336699" || len(v.Cover.Sheet.Backdrops) != 1 {
		t.Fatalf("без оформления: %v %+v", err, v.Cover)
	}
	if projectJSON(t, a) == styled {
		t.Fatal("без оформления совпало с оформленной")
	}
	// Вторая обложка — ошибка без правки (вид и шаг отмены прежние).
	before := projectJSON(t, a)
	if _, err = a.CreateCoverLook("", ""); err == nil || projectJSON(t, a) != before {
		t.Fatalf("вторая обложка: %v", err)
	}
}

// «Без оформления» после броска (145): Undo снимает обложку с фото в месте, CreateCover с тем же
// фото — обложка без макета, мест нет, фото фоном лицевой; макет альбома остаётся.
func TestCreateCoverLookPlainAfterUndo(t *testing.T) {
	a, _ := coverLookApp(t)
	r, err := a.CreateCoverLook("", "a")
	if err != nil || !r.Styled || len(r.View.Cover.Sheet.Backdrops) != 0 {
		t.Fatalf("бросок: %+v %v", r, err)
	}
	if v := a.Undo(); v.Cover != nil {
		t.Fatalf("Undo оставил обложку: %+v", v.Cover)
	}
	v, err := a.CreateCover("", "a")
	if err != nil || v.Cover == nil || v.Look == nil {
		t.Fatalf("без оформления: %v %+v", err, v.Cover)
	}
	c := v.Cover.Sheet
	p, _ := a.editor().Snapshot()
	if len(p.Cover.Sheet.Grid) != 0 || len(c.Backdrops) != 1 || c.Backdrops[0].Photo != "a" || c.BgSet[0] == "#336699" {
		t.Fatalf("без оформления: мест %d, фоны %+v, цвет %q", len(p.Cover.Sheet.Grid), c.Backdrops, c.BgSet[0])
	}
}

// Без фото: оформление макета есть, фона-фото нет; неизвестное фото — ErrNoPhoto без правки; без макета — как CreateCover.
func TestCreateCoverLookEdges(t *testing.T) {
	a, _ := coverLookApp(t)
	before := projectJSON(t, a)
	if _, err := a.CreateCoverLook("", "нет такого"); !errors.Is(err, errNoPhoto) || projectJSON(t, a) != before {
		t.Fatalf("неизвестное фото: %v", err)
	}
	if _, err := a.CreateCoverLook("нет-пресета", ""); err == nil || projectJSON(t, a) != before {
		t.Fatalf("неизвестный пресет: %v", err)
	}
	r, err := a.CreateCoverLook("", "")
	if err != nil || !r.Styled || len(r.View.Cover.Sheet.Backdrops) != 0 {
		t.Fatalf("без фото: %+v %v", r, err)
	}

	// Альбом не по макету: обычная обложка, Styled = false, имени нет.
	b, _ := spreadsApp(t, []string{"a"}, []string{"b"})
	r, err = b.CreateCoverLook("", "a")
	if err != nil || r.Styled || r.Look != "" || r.View.Cover == nil || len(r.View.Cover.Sheet.Backdrops) != 1 {
		t.Fatalf("без макета: %+v %v", r, err)
	}
}

// Макета нет на этом ПК: обложка обычная, не ошибка. Лист 031 — обложки нет, ошибка, лист цел.
func TestCreateCoverLookNoLookAndSheet(t *testing.T) {
	a, _ := coverLookApp(t)
	p, _ := a.editor().Snapshot()
	p.Look = &model.LookRef{Key: "my:ffffffffffffffff", Name: "Чужой"}
	a.setEditor(mustEditor(t, p, ""))
	r, err := a.CreateCoverLook("", "")
	if err != nil || r.Styled || r.Look != "" || r.View.Cover == nil || r.View.Cover.Sheet.BgSet[0] != "" {
		t.Fatalf("макета нет на ПК: %+v %v", r, err)
	}

	a.setEditor(mustEditor(t, album.NewVignetteSheet(300, 400), ""))
	before := projectJSON(t, a)
	if _, err = a.CreateCoverLook("", ""); err == nil || projectJSON(t, a) != before {
		t.Fatalf("лист 031: %v", err)
	}
}

// Фон обложки макета — файл библиотеки: libs собирает App (lookLibs), без них фон не встаёт.
func TestCreateCoverLookLibBg(t *testing.T) {
	a := lookApp(t)
	p, _ := a.editor().Snapshot()
	p.Spreads[0].Photos = []string{"a"}
	p.Photos["a"] = model.PhotoRef{Path: testkit.AbsPath("a.jpg"), W: 300, H: 200}
	a.setEditor(mustEditor(t, p, ""))
	bg := importBg(t, a, testkit.JPEG(t, testkit.TempDir(t), "Фон.jpg", 60, 40))[0]
	look := testLook("#ffffff")
	look.Cover.Bg = &model.TplBg{Lib: bg}
	s, err := a.decorStore()
	if err != nil {
		t.Fatal(err)
	}
	it, err := s.SaveLook("mine", "С фоном", look)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = a.ApplyLook(it.ID, model.LookOpts{Mode: model.LookAdd, Spreads: 3}); err != nil {
		t.Fatal(err)
	}
	r, err := a.CreateCoverLook("", "")
	if err != nil || !r.Styled || r.Look != "С фоном" {
		t.Fatalf("CreateCoverLook: %+v %v", r, err)
	}
	bds := r.View.Cover.Sheet.Backdrops
	if len(bds) != 1 || bds[0].Lib != bg || bds[0].Photo == "" {
		t.Fatalf("фон-файл макета не встал: %+v", bds)
	}
}
