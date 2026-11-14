package decor

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// TestPackFrame — вид frame в наборе (079, решение 079-19): рамочки с окном ставятся, окно найдено
// сразу (Frame без чтения диска), плитки — с названием набора, Find и /media/lib находят файл;
// сплошная, JPEG под именем .png и длиннее 4096 px — пропуск с причиной; удаление набора убирает
// файлы (кроме нужных keep); после перезапуска всё на месте.
func TestPackFrame(t *testing.T) {
	s, cfg := openTemp(t)
	ring := pngBytes(t, ringImage(120, 160, 24, 32, 24, 32))
	open := pngBytes(t, cornersImage(100, 100, 20))
	solid := pngBytes(t, ringImage(100, 100, 50, 50, 50, 50))
	jpg := mustRead(t, testkit.JPEG(t, testkit.TempDir(t), "a.jpg", 20, 10))
	long := pngBytes(t, ringImage(4097, 8, 1, 1, 1, 1))
	m, src := testPack("rama", 1, pit{"frames/ring.png", ring}, pit{"frames/open.png", open}, pit{"frames/solid.png", solid},
		pit{"frames/jpeg.png", jpg}, pit{"frames/long.png", long})
	st, err := s.StagePack(m, src, OriginFile, strings.Repeat("ab", 32))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	var why []string
	for _, k := range st.Skipped {
		why = append(why, k.Name+": "+k.Why)
	}
	want := []string{"frames/solid.png: в середине нет прозрачного окна — фото под рамочкой не будет видно",
		"frames/jpeg.png: содержимое не совпадает с расширением файла", "frames/long.png: рамочка больше 4096 px по стороне"}
	if !slices.Equal(why, want) || len(st.items) != 2 {
		t.Fatalf("пропуски %q, прошло %d", why, len(st.items))
	}
	info, err := s.InstallPack(st, nil)
	if err != nil || info.Frame != 2 || info.Bg != 0 || info.Sticker != 0 || info.Spread != 0 {
		t.Fatalf("установка: %v %+v", err, info)
	}
	noTmpAnywhere(t, cfg)
	rk, ok := libKey(ring), libKey(open)
	if w, have := s.frame.get(rk); !have || w.cut[0] < 0.19 { // окно нашла установка, не Frame
		t.Errorf("окно не в памяти после установки: %+v %v", w, have)
	}
	checkFramePack(t, s, cfg, "после установки", rk, ok)
	checkFramePack(t, Open(cfg), cfg, "после перезапуска", rk, ok)
	// Удаление: ring нужен открытому альбому (keep) — остаётся, open уходит.
	plan, err := s.RemovePack("rama", func(key string) bool { return key == rk })
	if err != nil || plan.Files != 1 || plan.Kept != 1 {
		t.Fatalf("удаление: %v %+v", err, plan)
	}
	if !regularFile(filepath.Join(frameDir(cfg), strings.TrimPrefix(rk, "my:")+".png")) ||
		regularFile(filepath.Join(frameDir(cfg), strings.TrimPrefix(ok, "my:")+".png")) {
		t.Error("файлы рамочек после удаления набора")
	}
	if _, err := s.Frame(ok); err != ErrNotFound {
		t.Errorf("удалённая рамочка: %v", err)
	}
	if _, found := s.Find(rk); !found { // альбом цел: файл найдётся, хотя плитки в каталоге уже нет
		t.Error("файл рамочки открытого альбома не находится")
	}
	if got := catIDs(t, s, KindFrame); len(got) != 0 {
		t.Errorf("каталог после удаления: %v", got)
	}
}

// checkFramePack — что видно после установки набора «rama» с кольцом rk и уголками ok.
func checkFramePack(t *testing.T, s *Store, cfg, when, rk, ok string) {
	t.Helper()
	if got := catIDs(t, s, KindFrame); !slices.Equal(got, []string{rk + "|Набор rama", ok + "|Набор rama"}) {
		t.Errorf("%s: каталог рамочек %v", when, got)
	}
	fs, err := s.Frame(rk)
	if err != nil || fs.Photo.Lib != rk || fs.Photo.W != 120 || fs.Open || fs.Cut[0] < 0.19 || fs.Cut[0] > 0.21 {
		t.Errorf("%s: рамочка набора: %v %+v", when, err, fs)
	}
	if fo, err := s.Frame(ok); err != nil || !fo.Open {
		t.Errorf("%s: уголки набора: %v %+v", when, err, fo)
	}
	if p, found := s.Find(rk); !found || filepath.Dir(p) != frameDir(cfg) {
		t.Errorf("%s: Find %q %v", when, p, found)
	}
	if _, found := s.registered(rk); !found {
		t.Errorf("%s: /media/lib не отдаёт плитку рамочки набора", when)
	}
	if ps := s.Packs(); len(ps) != 2 || ps[1].Frame != 2 || ps[0].Frame != len(framePresets) {
		t.Errorf("%s: наборы %+v", when, ps)
	}
	for _, kind := range []string{KindBg, KindSticker} { // рамочки набора не видны в других каталогах
		if got := catIDs(t, s, kind); len(got) != 0 {
			t.Errorf("%s: %s: %v", when, kind, got)
		}
	}
}
