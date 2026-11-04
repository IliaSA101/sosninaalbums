package album_test

// Бросок на плитку «+ Обложка» в альбоме с макетом (145 Ч2, решения 0.31-145-3, -4): фото встаёт
// в первое пустое место лицевой по model.CoverSeats, а не фоном; фото из альбома — копией.

import (
	"bytes"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// csFilled — ID фото в местах обложки по порядку CoverSeats ("" — место пустое).
func csFilled(p *model.Project) (out []string) {
	for _, pl := range p.CoverSeats() {
		out = append(out, pl.Photo)
	}
	return out
}

// TestCreateCoverLookPhotoInPlace: у макета на лицевой 3 места, на задней 2. Фото вне альбома
// встаёт в первое место лицевой, фона-фото нет, остальные четыре места пусты; один шаг отмены;
// Undo — обложки нет, Redo — тот же проект; круг Save → Load держит место.
func TestCreateCoverLookPhotoInPlace(t *testing.T) {
	tpl := lkCoverFull()
	path := filepath.Join(testkit.TempDir(t), "seat.salbum")
	e := edtest.Open(t, model.New(), path)
	lkApply(t, e, lkRefB, tpl, nil, 6)
	lkHost(e, tpl)
	before, undo := model.Marshal(edtest.P(e)), edtest.UndoLen(e)
	ph := edtest.Lib(edtest.Abs("new1.jpg"), model.XmpMarks{}) // ID от пути: так записи переживают Save → Load

	_, styled, err := e.CreateCoverLook("", &ph, nil, e.Look())
	if err != nil || styled == nil || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("styled %v, шагов %d, %v", styled != nil, edtest.UndoLen(e)-undo, err)
	}
	p := edtest.P(e)
	got := csFilled(p)
	if len(got) != 5 || got[0] != ph.ID || got[1] != "" || got[2] != "" || got[3] != "" || got[4] != "" {
		t.Fatalf("места по CoverSeats: %q (ждали фото только в первом)", got)
	}
	if c := p.Cover.Sheet; c.BgPhotoR != nil || c.BgPhotoL != nil || c.BgPhotoS != nil || p.Photos[ph.ID].Path != ph.Path {
		t.Fatalf("фото осталось фоном или нет записи: %+v", c)
	}
	after := model.Marshal(p)

	e.Undo()
	if edtest.P(e).Cover != nil || !bytes.Equal(model.Marshal(edtest.P(e)), before) {
		t.Fatal("Undo не вернул альбом до обложки одним шагом")
	}
	e.Redo()
	if !bytes.Equal(model.Marshal(edtest.P(e)), after) {
		t.Fatal("Redo дал другой проект")
	}

	if err := e.Save(""); err != nil {
		t.Fatal(err)
	}
	q, err := salbum.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := csFilled(q); len(got) != 5 || got[0] != ph.ID || q.Cover.Sheet.BgPhotoR != nil {
		t.Fatalf("после Save → Load: %q", got)
	}
}

// Без фото (nil) места лицевой остаются пустыми и фона нет; вторая обложка — ошибка без следа
// (шаг отмены, rev, проект прежние).
func TestCreateCoverLookNoPhotoAndRepeat(t *testing.T) {
	tpl := lkCoverFull()
	e := lkEditor(t)
	lkApply(t, e, lkRefB, tpl, nil, 6)
	lkHost(e, tpl)
	if _, styled, err := e.CreateCoverLook("", nil, nil, e.Look()); err != nil || styled == nil {
		t.Fatalf("без фото: %v", err)
	}
	for i, id := range csFilled(edtest.P(e)) {
		if id != "" {
			t.Fatalf("место %d занято: %q", i, id)
		}
	}
	ph := edtest.Photo("late")
	lkNoTrace(t, e, "вторая обложка", func() error {
		_, _, err := e.CreateCoverLook("", &ph, nil, e.Look())
		return err
	})
}

// TestCreateCoverLookPhotoCopy: фото уже стоит в развороте — на обложку встаёт копия (CopyNo > 0)
// в первое место лицевой, исходная запись и разворот не тронуты, записей фото ровно на одну больше.
func TestCreateCoverLookPhotoCopy(t *testing.T) {
	e := lkBuilt(t, 2) // фото pha, phb в местах разворотов
	tpl := lkCoverFull()
	lkHost(e, tpl)
	ph := edtest.Photo("pha")
	if !edtest.P(e).Placed("pha") {
		t.Fatal("фото не стоит в альбоме")
	}
	spreads, nPhotos := lkSnap(e), len(edtest.P(e).Photos)

	if _, styled, err := e.CreateCoverLook("", &ph, nil, e.Look()); err != nil || styled == nil {
		t.Fatalf("styled %v, %v", styled != nil, err)
	}
	p := edtest.P(e)
	seat := csFilled(p)[0]
	if model.CopyNo(seat) < 2 {
		t.Fatalf("в месте %q, ждали копию («-N»)", seat)
	}
	if len(p.Photos) != nPhotos+1 || p.Photos[seat].Path != ph.Path || !p.Placed("pha") || p.Cover.Sheet.BgPhotoR != nil {
		t.Fatalf("записей %d (было %d), исходная в альбоме %v, фон %v", len(p.Photos), nPhotos, p.Placed("pha"), p.Cover.Sheet.BgPhotoR)
	}
	if now := lkSnap(e); !reflect.DeepEqual(spreads, now) {
		t.Fatalf("развороты изменились:\n%v\n%v", spreads, now)
	}
	// Отмена одним шагом возвращает альбом: копии нет.
	e.Undo()
	if q := edtest.P(e); q.Cover != nil || len(q.Photos) != nPhotos {
		t.Fatalf("после Undo: обложка %v, записей %d (было %d)", q.Cover != nil, len(q.Photos), nPhotos)
	}
}

// Свое из библиотеки (Lib) встаёт в место исходной записью, не копией; негодный ключ — отказ без
// следа (TestCreateCoverLookStyled).
func TestCreateCoverLookLibPhotoInPlace(t *testing.T) {
	tpl := lkCoverFull()
	e := lkEditor(t)
	lkApply(t, e, lkRefB, tpl, nil, 6)
	lkHost(e, tpl)
	const key = "my:00000000000000ff"
	ph := model.PhotoInfo{ID: model.LibPhotoID(key), Path: edtest.Abs("lib.jpg"), W: 300, H: 200, Lib: key}
	if _, _, err := e.CreateCoverLook("", &ph, nil, e.Look()); err != nil {
		t.Fatal(err)
	}
	p := edtest.P(e)
	if got := csFilled(p)[0]; got != ph.ID || p.Photos[ph.ID].Lib != key || p.Cover.Sheet.BgPhotoR != nil {
		t.Fatalf("место %q, запись %+v", got, p.Photos[ph.ID])
	}
}

// Своё из библиотеки уже стоит в месте разворота: в место обложки оно не идёт (PlacePhotos снял бы
// его с разворота), встаёт фоном лицевой, как в 0.28; разворот не тронут.
func TestCreateCoverLookLibPhotoPlacedBg(t *testing.T) {
	e := lkBuilt(t, 1)
	const key = "my:00000000000000ff"
	ph := model.PhotoInfo{ID: model.LibPhotoID(key), Path: edtest.Abs("lib.jpg"), W: 300, H: 200, Lib: key}
	if _, left, _ := e.FillPlaces([]model.PhotoInfo{ph}, ""); left != 0 {
		t.Fatalf("фото не встало в разворот: %d", left)
	}
	lkHost(e, lkCoverFull())
	spreads := lkSnap(e)
	if _, _, err := e.CreateCoverLook("", &ph, nil, e.Look()); err != nil {
		t.Fatal(err)
	}
	p := edtest.P(e)
	if !p.Placed(ph.ID) || !reflect.DeepEqual(spreads, lkSnap(e)) {
		t.Fatal("кадр ушёл с разворота")
	}
	if b := p.Cover.Sheet.BgPhotoR; b == nil || b.Photo != ph.ID {
		t.Fatalf("фото не фоном лицевой: %+v", b)
	}
	for i, id := range csFilled(p) {
		if id != "" {
			t.Fatalf("место %d занято: %q", i, id)
		}
	}
}

// TestCreateCoverLookNoPlacesBg: мест на лицевой нет (или нет самого макета) — фото фоном
// лицевой, как в 0.28; места задней фото не берут, а фон не оформлен местом.
func TestCreateCoverLookNoPlacesBg(t *testing.T) {
	noFront := lkCoverFull()
	noFront.Cover.Front = lkSide(0, "#998877") // на задней 2 места, на лицевой — ни одного
	cases := map[string]func(*testing.T) (*album.Editor, bool){
		"лицевая без мест": func(t *testing.T) (*album.Editor, bool) {
			e := lkEditor(t)
			lkApply(t, e, lkRefB, noFront, nil, 6)
			lkHost(e, noFront)
			return e, true
		},
		"у альбома нет макета": func(t *testing.T) (*album.Editor, bool) { return lkEditor(t), false },
		"макет не найден на ПК": func(t *testing.T) (*album.Editor, bool) {
			e := lkEditor(t)
			lkApply(t, e, lkRefB, lkCoverFull(), nil, 6)
			e.SetLookHost(album.LookHost{Find: func(model.LookRef) *model.AlbumTpl { return nil }})
			return e, false
		},
	}
	for name, mk := range cases {
		t.Run(name, func(t *testing.T) {
			e, wantStyled := mk(t)
			ph := edtest.Photo("bg2")
			undo := edtest.UndoLen(e)
			_, styled, err := e.CreateCoverLook("", &ph, nil, e.Look())
			if err != nil || (styled != nil) != wantStyled || edtest.UndoLen(e) != undo+1 {
				t.Fatalf("styled %v (ждали %v), шагов %d, %v", styled != nil, wantStyled, edtest.UndoLen(e)-undo, err)
			}
			p := edtest.P(e)
			if b := p.Cover.Sheet.BgPhotoR; b == nil || b.Photo != "bg2" {
				t.Fatalf("фото не фоном лицевой: %+v", b)
			}
			for i, id := range csFilled(p) {
				if id != "" {
					t.Fatalf("место %d занято: %q", i, id)
				}
			}
		})
	}
}
