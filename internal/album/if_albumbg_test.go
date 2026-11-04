package album_test

// Фон альбома для агента (154 Т1, design §1.3): SetAlbumBgIf и backdrop.SetAlbumBgAgent — один шаг,
// свои фоны каталога снимаются, фото клиента — только по согласию на лист.

import (
	"bytes"
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// abPat — узор листа (другой вид, чем фон альбома).
func abPat() *model.Pattern { return &model.Pattern{Kind: "stripes", Preset: "sa:stripes"} }

// abBd — фото фона на листе (запись Photos уже есть).
func abBd(id string) *model.Backdrop { return &model.Backdrop{Photo: id, W: 100, H: 100} }

// abEd — альбом из n разворотов (ifBgProject) с настройкой листов; client(i) — фото клиента, lib — фото набора.
func abEd(t *testing.T, n int, path string, set func(sp []*model.Spread, client func(int) string, lib string)) (*album.Editor, []string) {
	t.Helper()
	p, client, lib := ifBgProject(n)
	p.Settings.Bg = "#f0e0d0"
	set(p.Spreads, client, lib)
	e := edtest.Open(t, p, path)
	return e, edtest.SpreadIDs(e.View())
}

// abLibSrc — фон альбома из фото набора (ключ библиотеки).
func abLibSrc() model.BgSource {
	return model.BgSource{Photo: model.PhotoInfo{ID: model.LibPhotoID(ifLibKey), Path: edtest.Abs("lib.jpg"), Lib: ifLibKey, W: 300, H: 200}}
}

func abBytes(e *album.Editor) []byte { return model.Marshal(edtest.P(e)) }

// TestSetAlbumBgIf: отказы сверки (чужой rev, срок, закрытый Editor) — ни шага; ошибка источника —
// ни шага; фон альбома ставится одним шагом rev+1, на страницы — pages; круг Save → Load; отмена
// возвращает всё, включая свои фоны листов; альбом без разворотов — тоже шаг; обложка не тронута.
func TestSetAlbumBgIf(t *testing.T) {
	path := filepath.Join(testkit.TempDir(t), "ab.salbum")
	e, ids := abEd(t, 3, path, func(sp []*model.Spread, _ func(int) string, _ string) {
		sp[0].Pat = abPat()
		sp[1].NoAlbumBg = true
	})
	if _, err := e.CreateCover("", nil); err != nil {
		t.Fatal(err)
	}
	edtest.Change(e, func(p *model.Project) { p.Cover.Sheet.Pat = abPat() })
	before, coverPat := abBytes(e), edtest.P(e).Cover.Sheet.Pat

	for name, src := range map[string]model.BgSource{
		"пустой источник": {},
		"плохой ключ":     {Photo: model.PhotoInfo{ID: "x", Lib: "плохой"}},
	} {
		s := take(e)
		if _, out, err := e.SetAlbumBgIf(src, false, nil, album.Expect{Rev: s.rev}); err == nil || out.Cleared != nil || out.Skipped != nil {
			t.Fatalf("%s: %+v, %v", name, out, err)
		}
		s.same(t, e, name)
	}
	if _, _, err := e.SetAlbumBgIf(model.BgSource{}, false, nil, album.Expect{Rev: edtest.Rev(e)}); !errors.Is(err, model.ErrPattern) {
		t.Fatalf("ErrPattern: %v", err)
	}

	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	v, out, err := e.SetAlbumBgIf(ifDots("dots"), true, nil, album.Expect{Rev: rev})
	if err != nil || v.Rev != rev+1 || edtest.UndoLen(e) != undo+1 || !reflect.DeepEqual(out.Cleared, ids[:1]) {
		t.Fatalf("SetAlbumBgIf: %+v, rev %d → %d, шагов %d, %v", out, rev, v.Rev, edtest.UndoLen(e)-undo, err)
	}
	p := edtest.P(e)
	if p.AlbumBg == nil || !p.AlbumBg.Pages || p.AlbumBg.Pat == nil || p.AlbumBg.Pat.Kind != "dots" {
		t.Fatalf("фон альбома: %+v", p.AlbumBg)
	}
	if p.Spreads[0].Pat != nil || p.Spreads[1].NoAlbumBg != true || p.Spreads[2].Pat != nil {
		t.Fatalf("листы: узор 0 %v, скрыт 1 %v", p.Spreads[0].Pat, p.Spreads[1].NoAlbumBg)
	}
	if !reflect.DeepEqual(p.Cover.Sheet.Pat, coverPat) {
		t.Fatal("фон альбома тронул обложку")
	}
	if err := e.Save(""); err != nil {
		t.Fatal(err)
	}
	q, err := salbum.Load(path)
	if err != nil || q.AlbumBg == nil || !q.AlbumBg.Pages || q.Spreads[0].Pat != nil || !q.Spreads[1].NoAlbumBg {
		t.Fatalf("после Save → Load: %v, %+v", err, q.AlbumBg)
	}
	// Одна отмена возвращает и фон альбома, и снятый узор листа.
	if _, err := e.UndoIfRev(album.Expect{Rev: v.Rev}); err != nil || !bytes.Equal(abBytes(e), before) {
		t.Fatalf("отмена не вернула проект: %v", err)
	}
	ifRefuse(t, e, func(x album.Expect) error {
		_, _, err := e.SetAlbumBgIf(ifDots("dots"), false, nil, x)
		return err
	})

	// Альбом без разворотов: фон альбома — тоже правка (шаг есть), снимать нечего.
	empty := edtest.Open(t, model.New(), "")
	rev, undo = edtest.Rev(empty), edtest.UndoLen(empty)
	v, out, err = empty.SetAlbumBgIf(abLibSrc(), false, nil, album.Expect{Rev: rev})
	if err != nil || v.Rev != rev+1 || edtest.UndoLen(empty) != undo+1 || len(out.Cleared)+len(out.Skipped) != 0 || edtest.P(empty).AlbumBg == nil {
		t.Fatalf("пустой альбом: %+v, rev %d → %d, %v", out, rev, v.Rev, err)
	}
}

// TestAlbumBgAgentClearsDecor: узор, фото набора и «скрыт + узор» уходят с листов, Cleared — ровно
// эти листы по порядку; «скрыт» без своего фона, лист без фона, цвет страниц и обложка — как были.
func TestAlbumBgAgentClearsDecor(t *testing.T) {
	e, ids := abEd(t, 6, "", func(sp []*model.Spread, _ func(int) string, lib string) {
		sp[0].PatL = abPat()                       // узор на странице
		sp[1].BgPhoto = abBd(lib)                  // фото набора на весь лист
		sp[2].NoAlbumBg, sp[2].Pat = true, abPat() // «скрыт + узор»: уходят оба
		sp[3].NoAlbumBg = true                     // «скрыт» без своего фона: остаётся (q115-5)
		sp[5].BgL = "#112233"                      // цвет страниц своим фоном не считается
	})
	if _, err := e.CreateCover("", nil); err != nil {
		t.Fatal(err)
	}
	edtest.Change(e, func(p *model.Project) { p.Cover.Sheet.Pat = abPat() })
	cover := edtest.P(e).Cover.Sheet.Pat

	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	v, out, err := e.SetAlbumBgIf(ifDots("dots"), false, nil, album.Expect{Rev: rev})
	if err != nil || !reflect.DeepEqual(out.Cleared, ids[:3]) || len(out.Skipped) != 0 || out.Replaced != 0 ||
		v.Rev != rev+1 || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("итог: %+v (ждали Cleared %v), %v", out, ids[:3], err)
	}
	p := edtest.P(e)
	for i := range 3 {
		s := p.Spreads[i]
		if s.PatL != nil || s.Pat != nil || s.BgPhoto != nil || s.NoAlbumBg {
			t.Errorf("лист %d не вернулся к фону альбома: %+v", i, s)
		}
	}
	if !p.Spreads[3].NoAlbumBg || p.Spreads[3].Pat != nil {
		t.Error("«скрыт» без своего фона изменился")
	}
	if p.Spreads[5].BgL != "#112233" || p.Settings.Bg != "#f0e0d0" {
		t.Errorf("цвет страниц тронут: лист %q, альбом %q", p.Spreads[5].BgL, p.Settings.Bg)
	}
	if !reflect.DeepEqual(p.Cover.Sheet.Pat, cover) {
		t.Error("обложка тронута")
	}
	// Отмена возвращает узоры и отметку «скрыт».
	if _, err := e.UndoIfRev(album.Expect{Rev: v.Rev}); err != nil {
		t.Fatal(err)
	}
	if p := edtest.P(e); p.AlbumBg != nil || p.Spreads[0].PatL == nil || p.Spreads[1].BgPhoto == nil || !p.Spreads[2].NoAlbumBg || p.Spreads[2].Pat == nil {
		t.Fatal("отмена не вернула свои фоны листов")
	}
}

// TestAlbumBgAgentSkipsClient: без согласия лист с фото клиента на фоне и его соседний узор не
// тронуты (ID в Skipped), остальные листы очищены; несколько фото клиента на разных листах.
func TestAlbumBgAgentSkipsClient(t *testing.T) {
	e, ids := abEd(t, 5, "", func(sp []*model.Spread, client func(int) string, lib string) {
		sp[0].BgPhotoL, sp[0].PatR = abBd(client(0)), abPat() // фото клиента + соседний узор
		sp[1].PatL = abPat()
		sp[2].BgPhoto = abBd(client(2))
		sp[3].BgPhotoR, sp[3].BgPhotoL = abBd(lib), abBd(client(3)) // набор и клиент вместе
	})
	_, out, err := e.SetAlbumBgIf(ifDots("dots"), false, nil, album.Expect{Rev: edtest.Rev(e)})
	if err != nil || !reflect.DeepEqual(out.Skipped, []string{ids[0], ids[2], ids[3]}) || !reflect.DeepEqual(out.Cleared, ids[1:2]) || out.Replaced != 0 {
		t.Fatalf("итог: %+v, %v", out, err)
	}
	sp := edtest.P(e).Spreads
	if sp[0].BgPhotoL == nil || sp[0].PatR == nil || sp[2].BgPhoto == nil || sp[3].BgPhotoL == nil || sp[3].BgPhotoR == nil {
		t.Fatal("лист с фото клиента тронут")
	}
	if sp[1].PatL != nil {
		t.Fatal("лист без фото клиента не очищен")
	}
	if edtest.P(e).AlbumBg == nil {
		t.Fatal("фон альбома не поставлен")
	}
}

// TestAlbumBgReplaceScoped: согласие на один из двух листов с фото клиента — снят только он
// (Replaced = его фото клиента), другой остался и в Skipped; ID согласия без фото клиента, листа
// без своего фона и неизвестный ничего не меняют сверх обычного.
func TestAlbumBgReplaceScoped(t *testing.T) {
	e, ids := abEd(t, 6, "", func(sp []*model.Spread, client func(int) string, lib string) {
		sp[0].BgPhotoL, sp[0].BgPhotoR = abBd(client(0)), abBd(client(1)) // два фото клиента
		sp[1].BgPhoto = abBd(client(2))                                   // клиент, согласия нет
		sp[2].BgPhoto = abBd(lib)                                         // набор: согласие лишнее
		sp[3].Pat = abPat()
	})
	consent := []string{ids[0], ids[2], ids[4], "нет-такого"} // 4 — без своего фона, неизвестный
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	v, out, err := e.SetAlbumBgIf(ifDots("dots"), false, consent, album.Expect{Rev: rev})
	if err != nil || v.Rev != rev+1 || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("итог: %+v, rev %d → %d, %v", out, rev, v.Rev, err)
	}
	if !reflect.DeepEqual(out.Cleared, []string{ids[0], ids[2], ids[3]}) || !reflect.DeepEqual(out.Skipped, []string{ids[1]}) || out.Replaced != 2 {
		t.Fatalf("Cleared %v, Skipped %v, Replaced %d", out.Cleared, out.Skipped, out.Replaced)
	}
	sp := edtest.P(e).Spreads
	if sp[0].BgPhotoL != nil || sp[0].BgPhotoR != nil || sp[1].BgPhoto == nil || sp[2].BgPhoto != nil || sp[3].Pat != nil {
		t.Fatal("фоны листов не как ждали")
	}
	// Согласие, но фото клиента нет и снимать нечего — как без согласия: холостая.
	s := take(e)
	if v, out, err := e.SetAlbumBgIf(ifDots("dots"), false, []string{ids[4], ids[2]}, album.Expect{Rev: s.rev}); err != nil || v.Rev != s.rev || len(out.Cleared) != 0 || out.Replaced != 0 {
		t.Fatalf("повтор с согласием без фото: %+v, %v", out, err)
	}
	s.same(t, e, "лишнее согласие")
	// Согласие на оставшийся лист добирает его: теперь снимается и он.
	_, out, err = e.SetAlbumBgIf(ifDots("dots"), false, []string{ids[1]}, album.Expect{Rev: s.rev})
	if err != nil || !reflect.DeepEqual(out.Cleared, ids[1:2]) || out.Replaced != 1 || len(out.Skipped) != 0 || edtest.P(e).Spreads[1].BgPhoto != nil {
		t.Fatalf("согласие на второй лист: %+v, %v", out, err)
	}
}

// TestAlbumBgAgentSameAgain: тот же фон и снимать нечего — ни шага, ни rev; тот же фон, но на листе
// появился свой — шаг с Cleared, фон альбома прежний; лист с фото клиента без согласия — холостая.
func TestAlbumBgAgentSameAgain(t *testing.T) {
	e, ids := abEd(t, 3, "", func(sp []*model.Spread, client func(int) string, _ string) {
		sp[2].BgPhoto = abBd(client(2))
	})
	src := ifDots("dots")
	if _, _, err := e.SetAlbumBgIf(src, false, nil, album.Expect{Rev: edtest.Rev(e)}); err != nil {
		t.Fatal(err)
	}
	s := take(e)
	v, out, err := e.SetAlbumBgIf(src, false, nil, album.Expect{Rev: s.rev})
	if err != nil || v.Rev != s.rev || len(out.Cleared) != 0 || !reflect.DeepEqual(out.Skipped, ids[2:]) {
		t.Fatalf("повтор: %+v, rev %d (было %d), %v", out, v.Rev, s.rev, err)
	}
	s.same(t, e, "повтор фона")

	edtest.Change(e, func(p *model.Project) { p.Spreads[0].Pat = abPat() })
	before := edtest.P(e).AlbumBg
	rev := edtest.Rev(e)
	v, out, err = e.SetAlbumBgIf(src, false, nil, album.Expect{Rev: rev})
	if err != nil || v.Rev != rev+1 || !reflect.DeepEqual(out.Cleared, ids[:1]) {
		t.Fatalf("снятие своего фона при том же фоне: %+v, rev %d → %d, %v", out, rev, v.Rev, err)
	}
	if !reflect.DeepEqual(edtest.P(e).AlbumBg, before) {
		t.Fatal("фон альбома изменился при том же источнике")
	}
}
