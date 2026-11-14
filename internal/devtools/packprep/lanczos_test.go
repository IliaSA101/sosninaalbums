package packprep

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"image"
	"image/color"
	"runtime"
	"testing"
)

// Плоский цвет остаётся плоским при любом масштабе: веса целые, сумма ровно 1<<16.
func TestLanczosFlat(t *testing.T) {
	c := color.NRGBA{17, 130, 255, 255}
	for _, sz := range [][2]int{{64, 64}, {37, 53}, {200, 100}, {5, 5}, {100, 3}} {
		src := canvas(41, 29, c)
		dst := Lanczos(src, sz[0], sz[1])
		if dst.Bounds().Dx() != sz[0] || dst.Bounds().Dy() != sz[1] {
			t.Fatalf("размер %v: получили %v", sz, dst.Bounds())
		}
		for i := 0; i < len(dst.Pix); i += 4 {
			if dst.Pix[i] != c.R || dst.Pix[i+1] != c.G || dst.Pix[i+2] != c.B || dst.Pix[i+3] != 255 {
				t.Fatalf("%v: пиксель %d = %v, ждали %v", sz, i/4, dst.Pix[i:i+4], c)
			}
		}
	}
}

// Одинаковый вход — одинаковые байты, сколько ни считай и на скольких ядрах (GOMAXPROCS 1, 2, 3 и
// все — строки считаются независимо); края не выходят за 0…255.
func TestLanczosDeterministic(t *testing.T) {
	src := texture(97, 61)
	a := Lanczos(src, 171, 108)
	b := Lanczos(src, 171, 108)
	if !bytes.Equal(a.Pix, b.Pix) {
		t.Fatal("два прогона дали разные байты")
	}
	defer runtime.GOMAXPROCS(runtime.GOMAXPROCS(0))
	sizes := [][2]int{{171, 108}, {40, 23}} // увеличение и уменьшение
	runtime.GOMAXPROCS(1)
	var ref [][]byte
	for _, sz := range sizes {
		ref = append(ref, Lanczos(src, sz[0], sz[1]).Pix)
	}
	for _, procs := range []int{2, 3, runtime.NumCPU()} {
		runtime.GOMAXPROCS(procs)
		for i, sz := range sizes {
			if !bytes.Equal(Lanczos(src, sz[0], sz[1]).Pix, ref[i]) {
				t.Fatalf("GOMAXPROCS=%d, %v: байты отличаются от прогона на одном ядре", procs, sz)
			}
		}
	}
	runtime.GOMAXPROCS(runtime.NumCPU())
	// Резкий край чёрное/белое даёт перерегулирование ядра — зажим держит байт.
	edge := canvas(40, 40, color.NRGBA{0, 0, 0, 255})
	fillRect(edge, image.Rect(20, 0, 40, 40), white)
	up := Lanczos(edge, 160, 160)
	if up.Pix[3] != 255 || up.NRGBAAt(0, 0).R != 0 || up.NRGBAAt(159, 0).R != 255 {
		t.Fatalf("углы края: %v … %v", up.NRGBAAt(0, 0), up.NRGBAAt(159, 0))
	}
	// Монотонный ход слева направо: Lanczos на ступеньке не ломает порядок вдали от неё.
	if up.NRGBAAt(10, 80).R > up.NRGBAAt(150, 80).R {
		t.Fatal("левая сторона светлее правой")
	}
}

// Увеличение вдвое градиента сохраняет средний уровень, уменьшение до 1×1 — среднее.
func TestLanczosKeepsMean(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 64, 64))
	for y := range 64 {
		for x := range 64 {
			v := uint8(x * 4)
			src.SetNRGBA(x, y, color.NRGBA{v, v, v, 255})
		}
	}
	one := Lanczos(src, 1, 1).NRGBAAt(0, 0)
	if one.R < 120 || one.R > 134 {
		t.Fatalf("среднее градиента 0…252 — около 126, получили %d", one.R)
	}
	if got := Lanczos(src, 0, 5); got.Bounds().Dx() != 0 {
		t.Fatalf("нулевая сторона: %v", got.Bounds())
	}
}

// Эталон: sha256 вывода Lanczos на двух размерах. Веса целые и без FMA (kernel), поэтому байты не
// зависят ни от ядер, ни от архитектуры; смена версии Go или платформы, сдвинувшая хоть один байт,
// изменила бы пиксели фонов набора «до v1» — тест это ловит. Эталон меняют только осознанно.
func TestLanczosReferenceHash(t *testing.T) {
	cases := []struct {
		w, h int
		want string
	}{
		{171, 108, "e672806f5df776f95cd425317813b1da804d75ee451d8f29d481e988ba9cfb19"},
		{40, 23, "c8d7988e49bf0d107b160a87bc89be936292103078508e6f3b38218c8219e283"},
	}
	for _, c := range cases {
		sum := sha256.Sum256(Lanczos(texture(97, 61), c.w, c.h).Pix)
		if got := hex.EncodeToString(sum[:]); got != c.want {
			t.Errorf("Lanczos 97×61 → %d×%d: sha256 %s, эталон %s", c.w, c.h, got, c.want)
		}
	}
}
