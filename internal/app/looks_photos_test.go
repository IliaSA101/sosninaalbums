package app

import (
	"context"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg" // DecodeConfig копий
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/app/looks"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// photoRig — App с кэшем миниатюр и лентой из n настоящих JPEG (600×400; orient[i] > 1 — тег EXIF
// у i-го). Папок не сканируем: лента и реестр ставятся прямо, как их держит сканер.
type photoRig struct {
	a        *App
	photos   []library.Photo
	cacheDir string
}

func newPhotoRig(t *testing.T, n int, orient map[int]int) *photoRig {
	t.Helper()
	a := newApp(t)
	r := &photoRig{a: a, cacheDir: testkit.TempDir(t)}
	c, err := library.NewCache(r.cacheDir)
	if err != nil {
		t.Fatal(err)
	}
	a.cache = c
	dir := testkit.TempDir(t)
	for i := range n {
		path := testkit.JPEGOriented(t, dir, fmt.Sprintf("Кадр_%02d.jpg", i), 600, 400, max(orient[i], 1))
		p, err := library.ReadInfo(path)
		if err != nil {
			t.Fatal(err)
		}
		r.photos = append(r.photos, p)
		a.byID[p.ID] = p
	}
	a.photos = slices.Clone(r.photos)
	return r
}

// ids — ID фото по номерам.
func (r *photoRig) ids(idx ...int) []string {
	out := make([]string, len(idx))
	for i, k := range idx {
		out[i] = r.photos[k].ID
	}
	return out
}

func photoIDs(list []library.Photo) []string {
	out := make([]string, len(list))
	for i, p := range list {
		out[i] = p.ID
	}
	return out
}

// Кандидаты: свободные по порядку ленты (не в альбоме, не на фоне, не отклонённые — файлом или
// своей отметкой); свободных нет — все без отклонённых; ленты нет — пусто; не больше 256.
func TestLookPhotosFreeFirst(t *testing.T) {
	r := newPhotoRig(t, 8, nil)
	a := r.a
	ph := func(k int) model.PhotoInfo {
		return model.PhotoInfo{ID: r.photos[k].ID, Path: r.photos[k].Path, W: 600, H: 400}
	}
	p := model.New()
	for _, k := range []int{1, 2} {
		p.Photos[r.photos[k].ID] = model.PhotoRef{Path: r.photos[k].Path, W: 600, H: 400}
	}
	p.Spreads = []*model.Spread{{ID: "0123456789ab", Photos: []string{r.photos[1].ID},
		BgPhoto: &model.Backdrop{Photo: r.photos[2].ID, W: 300, H: 200}}}
	ed := mustEditor(t, p, "")
	a.setEditor(ed)
	a.photos[3].Xmp = &library.Marks{Flag: -1} // отклонено в файле
	if _, err := ed.SetMarks([]model.PhotoInfo{ph(4)}, model.MarkPatch{Flag: ptr(-1)}); err != nil {
		t.Fatal(err)
	}
	rev, undo := ed.View().Rev, ed.View().CanUndo

	// 0 — свободно; 1 в альбоме; 2 на фоне; 3 и 4 отклонены; 5…7 свободны.
	got := photoIDs(a.looks.Candidates(ed))
	if want := r.ids(0, 5, 6, 7); !slices.Equal(got, want) {
		t.Errorf("свободные: %v, ждали %v", got, want)
	}
	// Тот же отбор, что у автосборки агента из полного вида.
	if want := a.agentFree(ed, ed.View(), nil); !slices.Equal(got, want) {
		t.Errorf("agentFree: %v, lookPhotos: %v", want, got)
	}
	if v := ed.View(); v.Rev != rev || v.CanUndo != undo {
		t.Error("отбор кандидатов изменил альбом")
	}

	// Свободных нет: все занятые фото, кроме отклонённых, по порядку ленты.
	for _, k := range []int{0, 5, 6, 7} {
		p.Photos[r.photos[k].ID] = model.PhotoRef{Path: r.photos[k].Path, W: 600, H: 400}
		p.Spreads[0].Photos = append(p.Spreads[0].Photos, r.photos[k].ID)
	}
	full := mustEditor(t, p, "")
	a.setEditor(full)
	if _, err := full.SetMarks([]model.PhotoInfo{ph(4)}, model.MarkPatch{Flag: ptr(-1)}); err != nil {
		t.Fatal(err)
	}
	if got, want := photoIDs(a.looks.Candidates(full)), r.ids(0, 1, 2, 5, 6, 7); !slices.Equal(got, want) {
		t.Errorf("всё занято: %v, ждали %v", got, want)
	}

	// Ленты нет — пусто (не nil-паника).
	a.mu.Lock()
	a.photos = nil
	a.mu.Unlock()
	if got := a.looks.Candidates(ed); len(got) != 0 {
		t.Errorf("ленты нет: %v", got)
	}
	// Лента больше предела: первые 256 по порядку.
	big := make([]library.Photo, 300)
	for i := range big {
		big[i] = library.Photo{ID: fmt.Sprintf("%016x", i+1), W: 3, H: 2}
	}
	a.mu.Lock()
	a.photos = big
	a.mu.Unlock()
	got = photoIDs(a.looks.Candidates(ed))
	if len(got) != looks.MaxPhotos || got[0] != big[0].ID || got[looks.MaxPhotos-1] != big[looks.MaxPhotos-1].ID {
		t.Errorf("предел: %d фото, первое %v", len(got), got[:1])
	}
}

// По кругу от сдвига: фото меньше мест — повторы получают ID копии «-2», «-3»; сдвиг больше числа
// фото берётся по модулю; пропавшая копия пропускается; обход ограничен 2 × мест.
func TestLookFilesRound(t *testing.T) {
	r := newPhotoRig(t, 3, nil)
	a, ctx := r.a, context.Background()
	id := func(k, n int) string { return model.CopyID(r.photos[k].ID, n) }

	got, err := a.looks.Files(ctx, r.photos, 1, 7, false)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, p := range got {
		ids = append(ids, p.ID)
		if _, err := os.Stat(p.Path); err != nil {
			t.Errorf("%s: файла копии нет: %v", p.ID, err)
		}
		if p.W != 600 || p.H != 400 {
			t.Errorf("%s: размер %d×%d", p.ID, p.W, p.H)
		}
	}
	want := []string{id(1, 1), id(2, 1), id(0, 1), id(1, 2), id(2, 2), id(0, 2), id(1, 3)}
	if !slices.Equal(ids, want) {
		t.Errorf("по кругу от 1: %v, ждали %v", ids, want)
	}
	if mod, err := a.looks.Files(ctx, r.photos, 1+3*5, 3, false); err != nil || photoIDs2(mod) != photoIDs2(got[:3]) {
		t.Errorf("сдвиг по модулю: %v %v", mod, err)
	}
	if neg, err := a.looks.Files(ctx, r.photos, -2, 1, false); err != nil || len(neg) != 1 || neg[0].ID != id(1, 1) {
		t.Errorf("отрицательный сдвиг: %v %v", neg, err)
	}
	if none, err := a.looks.Files(ctx, r.photos, 0, 0, false); err != nil || len(none) != 0 {
		t.Errorf("мест нет: %v %v", none, err)
	}
	if none, err := a.looks.Files(ctx, nil, 0, 5, false); err != nil || len(none) != 0 {
		t.Errorf("кандидатов нет: %v %v", none, err)
	}

	// Превью 2048 и миниатюра — разные файлы одного фото.
	thumb, _ := a.looks.Files(ctx, r.photos[:1], 0, 1, false)
	prev, _ := a.looks.Files(ctx, r.photos[:1], 0, 1, true)
	if len(thumb) != 1 || len(prev) != 1 || thumb[0].Path == prev[0].Path {
		t.Errorf("big: миниатюра %v и превью %v должны различаться", thumb, prev)
	}

	// Пропавший оригинал (копию не сделать): пропуск, остальные идут по кругу, обход конечен.
	if err := os.Remove(r.photos[1].Path); err != nil {
		t.Fatal(err)
	}
	// Превью 2048 фото 1 ещё не готовилось, поэтому берём big: копии из кэша не спасут.
	skip, err := a.looks.Files(ctx, r.photos, 0, 3, true)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := photoIDs2(skip), id(0, 1)+id(2, 1)+id(0, 2); got != want {
		t.Errorf("без пропавшего: %v", skip)
	}
	// Все недоступны: пусто, не зависание.
	gone := []library.Photo{{ID: "ffffffffffffffff", W: 3, H: 2}, {ID: "eeeeeeeeeeeeeeee", W: 3, H: 2}}
	if none, err := a.looks.Files(ctx, gone, 0, 50, false); err != nil || len(none) != 0 {
		t.Errorf("фото нет в реестре: %v %v", none, err)
	}
	// Одно фото на много мест: копии до 99, дальше места серые.
	many, err := a.looks.Files(ctx, r.photos[:1], 0, 150, false)
	if err != nil || len(many) != model.MaxCopy || many[model.MaxCopy-1].ID != id(0, model.MaxCopy) {
		t.Errorf("копий %d, последняя %v, %v", len(many), many[len(many)-1].ID, err)
	}
}

func photoIDs2(list []model.PhotoInfo) string {
	s := ""
	for _, p := range list {
		s += p.ID
	}
	return s
}

// Снятый запрос — ошибка запроса без фото и без файлов в кэше; снят посреди обхода — то же, без
// частичного ответа (в кэш ответа серое не попадёт).
func TestLookFilesCanceled(t *testing.T) {
	r := newPhotoRig(t, 4, nil)
	a := r.a
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	got, err := a.looks.Files(ctx, r.photos, 0, 4, true)
	if !errors.Is(err, context.Canceled) || got != nil {
		t.Errorf("снят до начала: %v %v", got, err)
	}
	if n := countFiles(t, r.cacheDir); n != 0 {
		t.Errorf("снятый запрос создал %d файлов в кэше", n)
	}

	// Снят, пока готовится второе фото: reachable зовётся из mediaPhoto перед файлом.
	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	old, calls := reachable, 0
	t.Cleanup(func() { reachable = old })
	reachable = func(string) bool {
		if calls++; calls == 2 {
			cancel2()
		}
		return true
	}
	got, err = a.looks.Files(ctx2, r.photos, 0, 4, false)
	if !errors.Is(err, context.Canceled) || got != nil {
		t.Errorf("снят посреди обхода: %d фото, %v", len(got), err)
	}

	// Копия уже в кэше и отдаётся несмотря на снятый запрос — ответа всё равно нет.
	if _, err := a.looks.Files(context.Background(), r.photos[:1], 0, 1, false); err != nil {
		t.Fatal(err)
	}
	ctx3, cancel3 := context.WithCancel(context.Background())
	defer cancel3()
	reachable = func(string) bool { cancel3(); return true }
	if got, err = a.looks.Files(ctx3, r.photos[:1], 0, 1, false); !errors.Is(err, context.Canceled) || got != nil {
		t.Errorf("снят при готовой копии: %v %v", got, err)
	}
}

func countFiles(t *testing.T, dir string) (n int) {
	t.Helper()
	err := filepath.WalkDir(dir, func(_ string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			n++
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return n
}

// Ориентация EXIF 6 (снимок на боку): ReadInfo меняет W/H, копия кэша повёрнута так же — размеры в
// PhotoInfo и пропорции файла совпадают, render режет окно по копии так же, как по оригиналу.
func TestLookFilesOrient6(t *testing.T) {
	r := newPhotoRig(t, 2, map[int]int{0: 6})
	if p := r.photos[0]; p.W != 400 || p.H != 600 {
		t.Fatalf("ReadInfo с тегом 6: %d×%d, ждали 400×600", p.W, p.H)
	}
	for _, big := range []bool{false, true} {
		got, err := r.a.looks.Files(context.Background(), r.photos, 0, 2, big)
		if err != nil || len(got) != 2 {
			t.Fatalf("big=%v: %v %v", big, got, err)
		}
		for i, want := range [][2]int{{400, 600}, {600, 400}} {
			if got[i].W != want[0] || got[i].H != want[1] {
				t.Errorf("big=%v фото %d: %d×%d, ждали %v", big, i, got[i].W, got[i].H, want)
			}
			f, err := os.Open(got[i].Path)
			if err != nil {
				t.Fatal(err)
			}
			cfg, _, err := image.DecodeConfig(f)
			_ = f.Close()
			if err != nil {
				t.Fatal(err)
			}
			if (cfg.Width > cfg.Height) != (want[0] > want[1]) {
				t.Errorf("big=%v фото %d: копия %d×%d повёрнута иначе, чем размер %v", big, i, cfg.Width, cfg.Height, want)
			}
			if d := float64(cfg.Width)/float64(cfg.Height) - float64(want[0])/float64(want[1]); d > 0.02 || d < -0.02 {
				t.Errorf("big=%v фото %d: пропорция копии %d×%d ≠ %v", big, i, cfg.Width, cfg.Height, want)
			}
		}
	}
}
