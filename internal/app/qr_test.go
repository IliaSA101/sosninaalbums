package app

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/decor"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// ownStickers — свои плитки каталога стикеров.
func ownStickers(t *testing.T, a *App) []DecorItem {
	t.Helper()
	cat, err := a.DecorCatalog(decor.KindSticker)
	if err != nil {
		t.Fatal(err)
	}
	var out []DecorItem
	for _, it := range cat.Items {
		if it.Own {
			out = append(out, it)
		}
	}
	return out
}

// Образец: путь, сторона, минимум и «что будет в коде»; ошибки — текст строки под полем; ничего
// не пишется в библиотеку и не меняет проект (шаг, Rev).
func TestQRPreview(t *testing.T) {
	a := decorApp(t)
	rev := a.editor().View().Rev
	info, err := a.QRPreview("t.me/test")
	if err != nil || info.Text != "https://t.me/test" || info.Side <= 8 || !strings.HasPrefix(info.Path, "M") || info.MinMM < 20 {
		t.Fatalf("образец: %+v %v", info, err)
	}
	q, _ := decor.EncodeQR("t.me/test")
	if info.Side != q.Side() || info.Path != q.SVGPath() || info.MinMM != q.MinMM() {
		t.Errorf("образец не совпал с кодом: %+v", info)
	}
	if info2, _ := a.QRPreview("mailto:a@b.ru"); info2.Text != "mailto:a@b.ru" {
		t.Errorf("mailto дополнен: %q", info2.Text)
	}
	for _, bad := range []string{"", "  ", "x\ny", strings.Repeat("a", decor.QRMaxLen+1)} {
		got, err := a.QRPreview(bad)
		if err == nil || got.Path != "" || got.Side != 0 {
			t.Errorf("%q: %+v %v", bad, got, err)
		}
	}
	if n := len(ownStickers(t, a)); n != 0 {
		t.Errorf("образец записал в библиотеку %d", n)
	}
	if _, err := os.Stat(filepath.Join(a.cfgDir, decor.RootName)); err == nil {
		t.Error("образец создал папку библиотеки")
	}
	if a.editor().View().Rev != rev || a.editor().View().CanUndo {
		t.Error("образец изменил проект")
	}
}

// Стикер встаёт на страницу листа, плитка — в «Мои» и в недавних, один шаг отмены снимает стикер,
// а запись библиотеки остаётся; повтор ссылки — новый стикер с той же плиткой; круг Save → Load
// сохраняет ключ своего; ошибки и неизвестный лист — без шага.
func TestAddQRSticker(t *testing.T) {
	a := decorApp(t)
	ed := a.editor()
	sw, _ := ed.SheetSize()
	rev := ed.View().Rev

	noQRChange(t, a, rev)

	r1, err := a.AddQRSticker("s1", 0, "t.me/test")
	if err != nil || r1.ID == "" || r1.View.Rev != rev+1 || !r1.View.CanUndo {
		t.Fatalf("первый: %v %q rev %d", err, r1.ID, r1.View.Rev)
	}
	own := ownStickers(t, a)
	if len(own) != 1 || own[0].Theme != decor.ThemeMine || own[0].Name != "QR · t.me/test" {
		t.Fatalf("плитка в «Мои»: %+v", own)
	}
	id := own[0].ID
	st := stickerAt(a, r1.ID)
	if st == nil || st.Art != nil || st.Photo != model.LibPhotoID(id) || !near(st.X+st.W/2, sw/4) || !near(st.H, st.W) {
		t.Fatalf("стикер на странице 0: %+v", st)
	}
	if ref, _ := ed.Ref(st.Photo); ref.Lib != id || ref.Path == "" {
		t.Fatalf("без ключа или пути: %+v", ref)
	}
	if rec := prefs.RecentDecor(a.cfgDir, decor.KindSticker); !slices.Equal(rec, []string{id}) {
		t.Errorf("недавние %v", rec)
	}

	// Тот же код на другом листе и другой странице: плитка одна, стикеров два, шаг у каждого свой.
	r2, err := a.AddQRSticker("s2", 1, "https://t.me/test")
	if err != nil || r2.ID == "" || r2.ID == r1.ID || r2.View.Rev != rev+2 {
		t.Fatalf("повтор: %v %q", err, r2.ID)
	}
	if own2 := ownStickers(t, a); len(own2) != 1 || own2[0].ID != id {
		t.Fatalf("повтор породил плитки: %+v", own2)
	}
	if s2 := stickerAt(a, r2.ID); s2 == nil || s2.Photo != st.Photo || !near(s2.X+s2.W/2, sw*3/4) {
		t.Fatalf("второй на странице 1: %+v", s2)
	}
	// Другая ссылка — вторая плитка, недавние новые первыми.
	r3, err := a.AddQRSticker("s1", 0, "mailto:a@b.ru")
	if err != nil || r3.ID == "" {
		t.Fatalf("третий: %v", err)
	}
	if own3 := ownStickers(t, a); len(own3) != 2 {
		t.Fatalf("плиток %d, ждали 2", len(own3))
	}
	if rec := prefs.RecentDecor(a.cfgDir, decor.KindSticker); len(rec) != 2 || rec[1] != id {
		t.Errorf("недавние %v", rec)
	}

	// Save → Load: ключ своего и стикер на месте.
	path := filepath.Join(testkit.TempDir(t), "qr.salbum")
	if err := ed.Save(path); err != nil {
		t.Fatal(err)
	}
	p, err := salbum.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if s, i := p.FindSticker(r1.ID); s == nil || s.Stickers[i].Photo != st.Photo || p.Photos[st.Photo].Lib != id {
		t.Errorf("после чтения: %v %+v", s, p.Photos[st.Photo])
	}

	undoQR(t, a, r1, r2, r3)
}

// noQRChange — плохая ссылка, пустая строка и неизвестный лист: без шага и без недавних; плохая
// ссылка и в библиотеку не пишет.
func noQRChange(t *testing.T, a *App, rev int) {
	t.Helper()
	for _, bad := range []string{"", "a\nb", strings.Repeat("a", decor.QRMaxLen+1)} {
		r, err := a.AddQRSticker("s1", 0, bad)
		if err == nil || r.ID != "" || r.View.Rev != rev {
			t.Fatalf("%q: %v %q rev %d→%d", bad, err, r.ID, rev, r.View.Rev)
		}
	}
	if n := len(ownStickers(t, a)); n != 0 {
		t.Fatalf("плохая ссылка записала плитки: %d", n)
	}
	// Неизвестный лист: стикера нет, а код в библиотеке остаётся (как у загрузки своего).
	if r, err := a.AddQRSticker("нет", 0, "t.me/test"); err != nil || r.ID != "" || r.View.Rev != rev || r.View.CanUndo {
		t.Fatalf("нет листа: %v %q rev %d→%d undo %v", err, r.ID, rev, r.View.Rev, r.View.CanUndo)
	}
	if rec := prefs.RecentDecor(a.cfgDir, decor.KindSticker); len(rec) != 0 {
		t.Fatalf("ошибка попала в недавние: %v", rec)
	}
}

// undoQR — отмена по шагу на стикер (r3, r2, r1); запись библиотеки остаётся.
func undoQR(t *testing.T, a *App, r1, r2, r3 StickerResult) {
	t.Helper()
	for _, c := range []struct{ gone, stay string }{{r3.ID, r2.ID}, {r2.ID, r1.ID}} {
		a.Undo()
		if stickerAt(a, c.gone) != nil || stickerAt(a, c.stay) == nil {
			t.Fatalf("отмена: %q должен уйти, %q остаться", c.gone, c.stay)
		}
	}
	a.Undo()
	if stickerAt(a, r1.ID) != nil || a.editor().View().Rev == r1.View.Rev {
		t.Fatal("третья отмена не сняла первый стикер")
	}
	if len(ownStickers(t, a)) != 2 {
		t.Error("отмена убрала плитку из библиотеки")
	}
}

// Только чтение библиотеки: ErrReadOnly, стикер не поставлен, шага нет; библиотека не открыта —
// ошибка без паники.
func TestAddQRStickerReadOnly(t *testing.T) {
	a := decorApp(t)
	rev := a.editor().View().Rev
	lib := filepath.Join(a.cfgDir, decor.RootName)
	if err := os.MkdirAll(lib, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(lib, "library.json"), []byte(`{"version":99,"items":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	a.openDecor()
	r, err := a.AddQRSticker("s1", 0, "t.me/test")
	if !errors.Is(err, decor.ErrReadOnly) || r.ID != "" || r.View.Rev != rev || r.View.CanUndo {
		t.Fatalf("только чтение: %v %q rev %d→%d", err, r.ID, rev, r.View.Rev)
	}
	// Образец при этом работает: он ничего не пишет.
	if _, err := a.QRPreview("t.me/test"); err != nil {
		t.Errorf("образец при чтении: %v", err)
	}
	a.decorLib.Store(nil)
	if _, err := a.AddQRSticker("s1", 0, "t.me/test"); err == nil {
		t.Error("без библиотеки ошибки нет")
	}
}
