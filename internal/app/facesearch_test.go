package app

import (
	"fmt"
	"math"
	"runtime"
	"sync"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/faces"
	"github.com/IliaSA101/sosninaalbums/internal/imgfile"
)

func faceAt(x, y, h float64) faces.Face {
	b := faces.Rect{X: x - h*0.375, Y: y - h/2, W: h * 0.75, H: h}
	return faces.Face{Score: 0.9, Box: b, Eyes: [2]faces.Pt{{X: x - 0.02, Y: y - h/8}, {X: x + 0.02, Y: y - h/8}}}
}

// TestPickFace: без лиц — пустая рамка; одно — оно; несколько похожих — «много», берётся
// ближайшее к центру; мелкое второе (< 70 %) «много» не делает; рамка нулевой высоты не лицо.
func TestPickFace(t *testing.T) {
	if box, _, many := pickFace(nil); box.H != 0 || many {
		t.Fatalf("без лиц: %+v %v", box, many)
	}
	if box, _, many := pickFace([]faces.Face{{}, {Box: faces.Rect{H: -1}}}); box.H != 0 || many {
		t.Fatalf("рамки без высоты: %+v %v", box, many)
	}
	one := faceAt(0.3, 0.4, 0.2)
	box, eye, many := pickFace([]faces.Face{one})
	if many || box.H != 0.2 || math.Abs(eye.X-0.3) > 1e-9 || math.Abs(eye.Y-(0.4-0.2/8)) > 1e-9 {
		t.Fatalf("одно лицо: %+v %+v %v", box, eye, many)
	}
	// Крупнее — у края, чуть меньше (80 %) — у центра: «несколько лиц», берётся центральное.
	edge, mid := faceAt(0.9, 0.5, 0.3), faceAt(0.5, 0.5, 0.24)
	box, _, many = pickFace([]faces.Face{edge, mid})
	if !many || box.H != 0.24 {
		t.Fatalf("похожие: %+v many=%v, ждали центральное 0,24", box, many)
	}
	// Второе мельче 70 % — фон, не «несколько»: берётся главное, хоть и у края.
	small := faceAt(0.5, 0.5, 0.15)
	box, _, many = pickFace([]faces.Face{edge, small})
	if many || box.H != 0.3 {
		t.Fatalf("мелкое второе: %+v many=%v", box, many)
	}
	// Порядок входа не влияет на выбор.
	box, _, many = pickFace([]faces.Face{mid, edge})
	if !many || box.H != 0.24 {
		t.Fatalf("другой порядок: %+v many=%v", box, many)
	}
}

// TestFaceCache: LRU — предел 4096, давние уходят первыми, чтение освежает, пустой список — ответ.
func TestFaceCache(t *testing.T) {
	var c faceCache
	key := func(i int) faceKey { return faceKey{path: fmt.Sprint("p", i), size: 1, version: faces.Version} }
	if _, ok := c.get(key(0)); ok || c.size() != 0 {
		t.Fatal("пустой кэш что-то отдал")
	}
	c.put(key(0), nil) // «лица нет» — тоже результат
	if fs, ok := c.get(key(0)); !ok || len(fs) != 0 {
		t.Fatalf("пустой список не запомнен: %v %v", fs, ok)
	}
	for i := 1; i < faceCacheMax; i++ {
		c.put(key(i), []faces.Face{{Score: float64(i)}})
	}
	c.get(key(0)) // освежили: следующий вытесненный — key(1)
	c.put(key(faceCacheMax), nil)
	if c.size() != faceCacheMax {
		t.Fatalf("записей %d, предел %d", c.size(), faceCacheMax)
	}
	if _, ok := c.get(key(1)); ok {
		t.Fatal("давняя запись не вытеснена")
	}
	if _, ok := c.get(key(0)); !ok {
		t.Fatal("освежённая запись вытеснена")
	}
	// Любая часть ключа — другая запись.
	base := key(5)
	for name, k := range map[string]faceKey{
		"размер":   {path: base.path, size: 2, version: base.version},
		"время":    {path: base.path, size: 1, mtime: 9, version: base.version},
		"четверти": {path: base.path, size: 1, turn: 1, version: base.version},
		"цвет":     {path: base.path, size: 1, color: imgfile.ColorAdobe, version: base.version},
		"версия":   {path: base.path, size: 1, version: "другая"},
	} {
		if _, ok := c.get(k); ok {
			t.Errorf("ключ с другим полем «%s» нашёл чужую запись", name)
		}
	}
	// Перезапись того же ключа не растит кэш.
	c.put(key(5), nil)
	if c.size() != faceCacheMax {
		t.Fatalf("перезапись выросла: %d", c.size())
	}
}

// TestFaceWorkers: min(4, ядра − 1), не меньше одного и не больше мест.
func TestFaceWorkers(t *testing.T) {
	if got := faceWorkers(0); got != 1 {
		t.Errorf("0 мест: %d", got)
	}
	if got := faceWorkers(1); got != 1 {
		t.Errorf("1 место: %d", got)
	}
	want := max(1, min(4, runtime.NumCPU()-1))
	if got := faceWorkers(1000); got != want {
		t.Errorf("1000 мест: %d, ждали %d", got, want)
	}
	if got := faceWorkers(2); got > 2 || got < 1 {
		t.Errorf("2 места: %d", got)
	}
}

// TestFaceTick: события не чаще раза в faceEvery, числа растут, последнее (N, N) — всегда.
func TestFaceTick(t *testing.T) {
	var mu sync.Mutex
	var got [][2]int
	tk := &faceTick{total: 500, emit: func(d, n int) { mu.Lock(); got = append(got, [2]int{d, n}); mu.Unlock() }}
	tk.begin()
	var wg sync.WaitGroup
	for range 5 {
		wg.Go(func() {
			for range 100 {
				tk.step()
			}
		})
	}
	wg.Wait()
	mu.Lock()
	defer mu.Unlock()
	if len(got) < 2 || got[0] != [2]int{0, 500} || got[len(got)-1] != [2]int{500, 500} {
		t.Fatalf("события %v", got)
	}
	if len(got) > 20 { // 500 шагов за миллисекунды: сотни событий — значит, рамки нет
		t.Fatalf("событий %d: частота не ограничена", len(got))
	}
	for i := 1; i < len(got); i++ {
		if got[i][0] < got[i-1][0] {
			t.Fatalf("числа убывают: %v", got)
		}
	}
	// Медленный ход шлёт каждый шаг, когда прошло больше faceEvery.
	got = nil
	slow := &faceTick{total: 3, emit: func(d, n int) { got = append(got, [2]int{d, n}) }}
	slow.begin()
	for range 3 {
		slow.last = slow.last.Add(-2 * faceEvery)
		slow.step()
	}
	if len(got) != 4 {
		t.Fatalf("медленный ход: %v", got)
	}
}
