package library

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/library/imgsig"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// blocksJPEG пишет JPEG 800×600 из цветных блоков: seed задаёт рисунок, разные seed — разные кадры.
func blocksJPEG(t testing.TB, dir, name string, seed uint64) Photo {
	t.Helper()
	rng := rand.New(rand.NewPCG(seed, 77))
	var cells [8][6]color.RGBA
	for i := range cells {
		for j := range cells[i] {
			cells[i][j] = color.RGBA{uint8(rng.IntN(256)), uint8(rng.IntN(256)), uint8(rng.IntN(256)), 255}
		}
	}
	img := image.NewRGBA(image.Rect(0, 0, 800, 600))
	for y := range 600 {
		for x := range 800 {
			img.SetRGBA(x, y, cells[x/100][y/100])
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return Photo{Path: path, Name: name}
}

func sigCache(t testing.TB) (*Cache, string) {
	t.Helper()
	dir := testkit.TempDir(t)
	c, err := NewCache(filepath.Join(dir, "cache"))
	if err != nil {
		t.Fatal(err)
	}
	testkit.BeforeRemove(t, c.Close)
	return c, dir
}

// sigFileOf — путь `_s.bin` фото.
func sigFileOf(t testing.TB, c *Cache, p Photo) string {
	t.Helper()
	key, err := cacheKey(p.Path, p.Color)
	if err != nil {
		t.Fatal(err)
	}
	return c.sigPath(key)
}

// Круг: подпись считается по миниатюре, пишется рядом с ней и читается из файла без миниатюры.
func TestCacheSigRoundTrip(t *testing.T) {
	c, dir := sigCache(t)
	p := blocksJPEG(t, dir, "a.jpg", 1)
	s, err := c.Sig(p)
	if err != nil || !s.Ok {
		t.Fatalf("Sig: %+v, %v", s, err)
	}
	thumb, err := c.File(p, false)
	if err != nil {
		t.Fatal(err)
	}
	img, err := decodeThumb(thumb)
	if err != nil {
		t.Fatal(err)
	}
	if want := imgsig.Of(img); s != want {
		t.Errorf("подпись %+v, по миниатюре %+v", s, want)
	}
	sf := sigFileOf(t, c, p)
	if st, err := os.Stat(sf); err != nil || st.Size() != imgsig.FileSize {
		t.Fatalf("файл подписи: %v, %v", st, err)
	}
	if filepath.Dir(sf) != filepath.Dir(thumb) {
		t.Errorf("подпись %s не рядом с миниатюрой %s", sf, thumb)
	}
	// Миниатюры нет — подпись всё равно берётся из файла и миниатюру не делает заново.
	if err := os.Remove(thumb); err != nil {
		t.Fatal(err)
	}
	again, err := c.Sig(p)
	if err != nil || again != s {
		t.Fatalf("из файла: %+v, %v; ждали %+v", again, err, s)
	}
	if _, err := os.Stat(thumb); err == nil {
		t.Error("чтение готовой подписи заново сделало миниатюру")
	}
}

// Кэш 0.19: миниатюра есть, `_s.bin` нет — подпись считается по миниатюре, оригинал не декодируется
// (его подменили мусором того же размера и времени: ключ кэша прежний), файл подписи появляется.
func TestCacheSigFromOldThumb(t *testing.T) {
	c, dir := sigCache(t)
	var photos []Photo
	for i := range 3 { // несколько кадров: у каждого свой файл подписи
		p := blocksJPEG(t, dir, fmt.Sprintf("old-%d.jpg", i), uint64(i+1))
		if _, err := c.File(p, false); err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(sigFileOf(t, c, p)); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
		st, err := os.Stat(p.Path)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p.Path, bytes.Repeat([]byte{0xAB}, int(st.Size())), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(p.Path, st.ModTime(), st.ModTime()); err != nil {
			t.Fatal(err)
		}
		photos = append(photos, p)
	}
	for _, p := range photos {
		s, err := c.Sig(p)
		if err != nil || !s.Ok {
			t.Fatalf("%s: Sig %+v, %v", p.Name, s, err)
		}
		thumb, err := c.File(p, false)
		if err != nil {
			t.Fatal(err)
		}
		img, err := decodeThumb(thumb)
		if err != nil {
			t.Fatal(err)
		}
		if want := imgsig.Of(img); s != want {
			t.Errorf("%s: подпись %+v, по миниатюре %+v", p.Name, s, want)
		}
		if st, err := os.Stat(sigFileOf(t, c, p)); err != nil || st.Size() != imgsig.FileSize {
			t.Errorf("%s: файл подписи: %v, %v", p.Name, st, err)
		}
	}
}

// Повреждённый `_s.bin` (пустой, обрезанный, длиннее, чужая версия, NaN, огромный) — нет подписи:
// считается заново и файл переписывается верным.
func TestCacheSigCorruptFile(t *testing.T) {
	c, dir := sigCache(t)
	p := blocksJPEG(t, dir, "a.jpg", 2)
	want, err := c.Sig(p)
	if err != nil {
		t.Fatal(err)
	}
	sf := sigFileOf(t, c, p)
	good, err := os.ReadFile(sf)
	if err != nil {
		t.Fatal(err)
	}
	nan := bytes.Clone(good)
	nan[17], nan[18], nan[19], nan[20] = 0, 0, 0xC0, 0x7F // float32 NaN
	ver := bytes.Clone(good)
	ver[0] = 9
	garbage := bytes.Repeat([]byte{0xAB}, imgsig.FileSize)
	cases := map[string][]byte{
		"пустой":     {},
		"обрезанный": good[:imgsig.FileSize-3],
		"длиннее":    append(bytes.Clone(good), 1, 2, 3),
		"версия":     ver,
		"NaN":        nan,
		"мусор":      garbage,
		"огромный":   make([]byte, 1<<20),
	}
	for name, data := range cases {
		if err := os.WriteFile(sf, data, 0o644); err != nil {
			t.Fatal(err)
		}
		if s, ok := readSig(sf); ok {
			t.Errorf("%s: файл принят как %+v", name, s)
		}
		got, err := c.Sig(p)
		if err != nil || got != want {
			t.Errorf("%s: Sig = %+v, %v; ждали %+v", name, got, err, want)
		}
		if fixed, err := os.ReadFile(sf); err != nil || !bytes.Equal(fixed, good) {
			t.Errorf("%s: файл не исправлен (%v)", name, err)
		}
	}
}

// Несколько фото: у разных кадров разные подписи и свои файлы; параллельные запросы одного кадра
// не ломают друг друга.
func TestCacheSigManyConcurrent(t *testing.T) {
	c, dir := sigCache(t)
	var photos []Photo
	for i := range 5 {
		photos = append(photos, blocksJPEG(t, dir, string(rune('a'+i))+".jpg", uint64(10+i)))
	}
	const reps = 4 // каждое фото запрашивают одновременно несколько раз
	res := make([][]imgsig.Sig, reps)
	var wg sync.WaitGroup
	for rep := range reps {
		res[rep] = make([]imgsig.Sig, len(photos))
		for i, p := range photos {
			wg.Go(func() {
				s, err := c.Sig(p)
				if err != nil {
					t.Errorf("фото %d, запрос %d: %v", i, rep, err)
				}
				res[rep][i] = s
			})
		}
	}
	wg.Wait()
	got := res[0]
	for rep := 1; rep < reps; rep++ {
		for i := range photos {
			if res[rep][i] != got[i] || !got[i].Ok {
				t.Errorf("фото %d: запрос %d дал %+v, первый %+v", i, rep, res[rep][i], got[i])
			}
		}
	}
	files := map[string]bool{}
	for i, p := range photos {
		s, err := c.Sig(p) // после гонки — стабильно
		if err != nil || !s.Ok {
			t.Fatalf("фото %d: %+v, %v", i, s, err)
		}
		files[sigFileOf(t, c, p)] = true
		for j := range i {
			if imgsig.Dist(s, got[j]) <= imgsig.DistSimilar {
				t.Errorf("кадры %d и %d похожи (Dist %d)", i, j, imgsig.Dist(s, got[j]))
			}
		}
	}
	if len(files) != len(photos) {
		t.Errorf("файлов подписей %d, ждали %d", len(files), len(photos))
	}
}

// Файл фото заменили (другой размер) — ключ новый, подпись считается заново.
func TestCacheSigFollowsFile(t *testing.T) {
	c, dir := sigCache(t)
	p := blocksJPEG(t, dir, "a.jpg", 3)
	first, err := c.Sig(p)
	if err != nil {
		t.Fatal(err)
	}
	old := sigFileOf(t, c, p)
	blocksJPEG(t, dir, "a.jpg", 4) // тот же путь, другой кадр
	second, err := c.Sig(p)
	if err != nil {
		t.Fatal(err)
	}
	if second == first {
		t.Error("подпись не изменилась после замены файла")
	}
	if sigFileOf(t, c, p) == old {
		t.Error("ключ файла подписи не сменился")
	}
}

// Края: нет файла фото, битая миниатюра, PNG (миниатюра PNG), чёрно-белый кадр.
func TestCacheSigErrorsAndKinds(t *testing.T) {
	c, dir := sigCache(t)
	if _, err := c.Sig(Photo{Path: filepath.Join(dir, "нет.jpg"), Name: "нет.jpg"}); err == nil {
		t.Error("нет файла: ждали ошибку")
	}
	// Миниатюра есть, но не читается — ошибка, а не паника, и подписи не остаётся.
	p := blocksJPEG(t, dir, "bad.jpg", 5)
	thumb, err := c.File(p, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(thumb, []byte("не картинка"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Sig(p); err == nil {
		t.Error("битая миниатюра: ждали ошибку")
	}
	if _, err := os.Stat(sigFileOf(t, c, p)); err == nil {
		t.Error("для битой миниатюры записан файл подписи")
	}
	// PNG с прозрачностью — миниатюра PNG, подпись считается.
	alpha := testkit.PNG(t, dir, "logo.png", 80, 60, color.NRGBA{200, 30, 30, 255})
	s, err := c.Sig(Photo{Path: alpha, Name: "logo.png"})
	if err != nil || !s.Ok {
		t.Errorf("PNG: %+v, %v", s, err)
	}
	// Серый кадр: цветность ниже порога «исходно ч/б».
	g := image.NewGray(image.Rect(0, 0, 300, 200))
	for i := range g.Pix {
		g.Pix[i] = uint8(i * 7 % 256)
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, g, nil); err != nil {
		t.Fatal(err)
	}
	gp := filepath.Join(dir, "gray.jpg")
	if err := os.WriteFile(gp, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	gs, err := c.Sig(Photo{Path: gp, Name: "gray.jpg"})
	if err != nil || !gs.Ok || gs.Color >= imgsig.ColorGray || math.IsNaN(float64(gs.Sharp)) {
		t.Errorf("серый кадр: %+v, %v", gs, err)
	}
}

// Прогрев делает подписи вместе с миниатюрами; Trim убирает `_s.bin` как любой файл кэша,
// после чего подпись считается заново.
func TestWarmMakesSigsAndTrimRemoves(t *testing.T) {
	c, dir := sigCache(t)
	var photos []Photo
	for i := range 3 {
		photos = append(photos, blocksJPEG(t, dir, string(rune('a'+i))+".jpg", uint64(20+i)))
	}
	c.Warm(context.Background(), photos, func(int, int) {})
	want := make([]imgsig.Sig, len(photos))
	for i, p := range photos {
		if _, err := os.Stat(sigFileOf(t, c, p)); err != nil {
			t.Fatalf("после прогрева нет подписи фото %d: %v", i, err)
		}
		s, err := c.Sig(p)
		if err != nil || !s.Ok {
			t.Fatalf("фото %d: %+v, %v", i, s, err)
		}
		want[i] = s
	}
	c.Close() // фоновая чистка после Warm держит замок Trim: без этого ручной Trim молча уходит
	c.Trim(0, 0)
	for i, p := range photos {
		if _, err := os.Stat(sigFileOf(t, c, p)); err == nil {
			t.Errorf("фото %d: Trim оставил файл подписи", i)
		}
		s, err := c.Sig(p)
		if err != nil || s != want[i] {
			t.Errorf("фото %d после Trim: %+v, %v; ждали %+v", i, s, err, want[i])
		}
	}
}

// BenchmarkCacheSig — подпись из готовой миниатюры (декод + хеши + запись файла): то, что
// Warm добавляет на фото в старом кэше.
func BenchmarkCacheSig(b *testing.B) {
	c, dir := sigCache(b)
	p := blocksJPEG(b, dir, "a.jpg", 1)
	if _, err := c.File(p, false); err != nil {
		b.Fatal(err)
	}
	sf := sigFileOf(b, c, p)
	b.ResetTimer()
	for range b.N {
		_ = os.Remove(sf)
		if s, err := c.Sig(p); err != nil || !s.Ok {
			b.Fatal(s, err)
		}
	}
}

// TestCacheShotSigAlpha: для отбора похожих (078 §5) у картинки с прозрачностью подписи нет
// (миниатюра PNG — оформление, не кадр), у непрозрачного PNG и JPEG — та же, что у Sig.
func TestCacheShotSigAlpha(t *testing.T) {
	c, dir := sigCache(t)
	glass := testkit.PNG(t, dir, "glass.png", 80, 60, color.NRGBA{200, 30, 30, 100})
	if s, err := c.ShotSig(Photo{Path: glass, Name: "glass.png"}); err == nil || s.Ok {
		t.Fatalf("прозрачный PNG: %+v, %v", s, err)
	}
	// testkit.PNG прозрачен на половину ширины: непрозрачный PNG пишем сами.
	img := image.NewNRGBA(image.Rect(0, 0, 80, 60))
	for i := range img.Pix {
		img.Pix[i] = 255
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	solid := filepath.Join(dir, "solid.png")
	if err := os.WriteFile(solid, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, p := range []Photo{{Path: solid, Name: "solid.png"}, blocksJPEG(t, dir, "shot.jpg", 9)} {
		want, err := c.Sig(p)
		if err != nil {
			t.Fatal(err)
		}
		if got, err := c.ShotSig(p); err != nil || got != want || !got.Ok {
			t.Fatalf("%s: %+v (%v), Sig %+v", p.Name, got, err, want)
		}
	}
}
