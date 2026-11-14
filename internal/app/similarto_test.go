package app

// SimilarTo (138): окно кадров вокруг исходного, порядок ленты, досчёт подписей с пределом, пределы входа.

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album/similar"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/library/imgsig"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// simSig — подпись кадра tN: «близкие» — k младших бит dHash (Dist между ними = разница k), «далёкие» (N ≥ 100) — чужие.
func simSig(p library.Photo) imgsig.Sig {
	var n int
	_, _ = fmt.Sscanf(p.ID, "t%d", &n)
	if n >= 100 {
		x := uint64(n) * 0x9e3779b97f4a7c15
		return imgsig.Sig{D: x ^ x>>29, P: x * 0xbf58476d1ce4e5b9, Ok: true, Sharp: 1}
	}
	return imgsig.Sig{D: uint64(1)<<n - 1, Ok: true, Sharp: 1}
}

func simShot(calls *atomic.Int32) func(p library.Photo) (imgsig.Sig, error) {
	return func(p library.Photo) (imgsig.Sig, error) {
		calls.Add(1)
		return simSig(p), nil
	}
}

// TestSimilarToOrder: порядок ленты берётся из order (а не из byID), неизвестные и повторные
// id пропускаются, первым в ответе не обязательно исходный кадр.
func TestSimilarToOrder(t *testing.T) {
	var calls atomic.Int32
	a, _, _ := sigApp(t, 6, simShot(&calls))
	order := []string{"t4", "нет", "t0", "t4", "t2", "", "t1"}
	res, err := a.SimilarTo("t2", order)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"t4", "t0", "t2", "t1"}; !slices.Equal(res.IDs, want) || res.More != 0 || res.Why != "" {
		t.Fatalf("%+v, ждали %v", res, want)
	}
	// Исходный кадр не в order или неизвестен — «похожих нет», не ошибка.
	for _, id := range []string{"t5", "нет", ""} {
		if res, err = a.SimilarTo(id, order); err != nil || res.Why != "none" || len(res.IDs) != 0 || res.IDs == nil {
			t.Errorf("%q: %+v, %v", id, res, err)
		}
	}
	if res, err = a.SimilarTo("t0", nil); err != nil || res.Why != "none" {
		t.Errorf("пустой order: %+v, %v", res, err)
	}
}

// TestSimilarToLimit: больше трёх похожих — кадр и три ближайших, остальные в More.
func TestSimilarToLimit(t *testing.T) {
	var calls atomic.Int32
	a, ids, _ := sigApp(t, 6, simShot(&calls))
	res, err := a.SimilarTo("t0", ids)
	if err != nil || !slices.Equal(res.IDs, []string{"t0", "t1", "t2", "t3"}) || res.More != 2 {
		t.Fatalf("%+v, %v", res, err)
	}
	// Окно: 12 по ленте — дальше не похожи, даже с общей подписью.
	a, ids, _ = sigApp(t, 40, func(p library.Photo) (imgsig.Sig, error) { return imgsig.Sig{D: 1, Ok: true, Sharp: 1}, nil })
	if res, err = a.SimilarTo("t0", ids); err != nil || len(res.IDs) != 4 || res.More != similar.Window-3 {
		t.Fatalf("окно: %+v, %v", res, err)
	}
}

// TestSimilarToFillsSigs: подписи нет в памяти — считается, кладётся в память, повтор не считает снова.
func TestSimilarToFillsSigs(t *testing.T) {
	var calls atomic.Int32
	a, ids, _ := sigApp(t, 4, simShot(&calls))
	res, err := a.SimilarTo("t1", ids)
	if err != nil || len(res.IDs) != 4 {
		t.Fatalf("%+v, %v", res, err)
	}
	if calls.Load() != 4 || len(a.sig.mem) != 4 {
		t.Fatalf("посчитано %d, в памяти %d", calls.Load(), len(a.sig.mem))
	}
	if _, err = a.SimilarTo("t2", ids); err != nil || calls.Load() != 4 {
		t.Fatalf("повтор считал снова: %d, %v", calls.Load(), err)
	}
	// Файл сменил размер — подпись не годится, считается заново.
	p := a.byID["t1"]
	p.Size++
	a.byID["t1"] = p
	if _, err = a.SimilarTo("t2", ids); err != nil || calls.Load() != 5 {
		t.Fatalf("смена размера: посчитано %d, %v", calls.Load(), err)
	}
}

// TestSimilarToDeadline: подпись не успели — кадр без неё в сравнение не идёт, ответ приходит в
// срок (а не по готовности); готовое после срока остаётся в памяти для следующего вызова.
func TestSimilarToDeadline(t *testing.T) {
	var calls atomic.Int32
	release := make(chan struct{})
	var once sync.Once
	t.Cleanup(func() { once.Do(func() { close(release) }) })
	a, ids, _ := sigApp(t, 4, func(p library.Photo) (imgsig.Sig, error) {
		calls.Add(1)
		if p.ID == "t3" {
			<-release
		}
		return simSig(p), nil
	})
	a.sig.wait = 100 * time.Millisecond
	start := time.Now()
	res, err := a.SimilarTo("t0", ids)
	if err != nil {
		t.Fatal(err)
	}
	if d := time.Since(start); d > testkit.Limit(3*time.Second) {
		t.Errorf("ответ через %v: предел досчёта не сработал", d)
	}
	if !slices.Equal(res.IDs, []string{"t0", "t1", "t2"}) || res.Why != "" {
		t.Fatalf("%+v, ждали t0 t1 t2 без t3", res)
	}
	once.Do(func() { close(release) })
	a.bg.Wait()
	if res, err = a.SimilarTo("t0", ids); err != nil || len(res.IDs) != 4 {
		t.Fatalf("после досчёта: %+v, %v", res, err)
	}
}

// TestSimilarToTooLong: order длиннее предела — ошибка до всякой работы; ровно предел — можно.
func TestSimilarToTooLong(t *testing.T) {
	var calls atomic.Int32
	a, _, _ := sigApp(t, 2, simShot(&calls))
	long := make([]string, similarOrderMax+1)
	if _, err := a.SimilarTo("t0", long); err == nil {
		t.Fatal("слишком длинный order принят")
	}
	if calls.Load() != 0 {
		t.Fatal("подписи считались для отклонённого запроса")
	}
	long = long[:similarOrderMax]
	long[5000] = "t0"
	if res, err := a.SimilarTo("t0", long); err != nil || res.Why != "none" {
		t.Fatalf("ровно предел: %+v, %v", res, err)
	}
}

// TestSimilarToNoSig: у кадра подписи нет (не читается, прозрачность) — why nosig; у соседей
// нет — они пропускаются; нет кэша миниатюр — подписей нет, но и ошибки нет.
func TestSimilarToNoSig(t *testing.T) {
	var calls atomic.Int32
	a, ids, _ := sigApp(t, 4, func(p library.Photo) (imgsig.Sig, error) {
		calls.Add(1)
		if p.ID == "t0" || p.ID == "t3" {
			return imgsig.Sig{}, errors.New("битая миниатюра")
		}
		return simSig(p), nil
	})
	res, err := a.SimilarTo("t0", ids)
	if err != nil || res.Why != "nosig" || len(res.IDs) != 0 || res.IDs == nil {
		t.Fatalf("кадр без подписи: %+v, %v", res, err)
	}
	if res, err = a.SimilarTo("t1", ids); err != nil || !slices.Equal(res.IDs, []string{"t1", "t2"}) {
		t.Fatalf("сосед без подписи: %+v, %v", res, err)
	}
	// Единственный кадр с подписью — похожих нет.
	if res, err = a.SimilarTo("t1", []string{"t1", "t0"}); err != nil || res.Why != "none" {
		t.Fatalf("один с подписью: %+v, %v", res, err)
	}
	// Без кэша миниатюр (shot не подменён): подписей нет, паники и ошибки нет.
	b := newApp(t)
	b.byID["t0"] = library.Photo{ID: "t0", Path: `C:\a\t0.jpg`, W: 3, H: 2, Size: 1}
	if res, err = b.SimilarTo("t0", []string{"t0"}); err != nil || res.Why != "nosig" {
		t.Fatalf("без кэша: %+v, %v", res, err)
	}
}

// TestSimilarToCost: лента ~1000 кадров с подписями в памяти — ответ за миллисекунды. Факт замера
// пишется в журнал теста (гипотеза спеки — меньше 10 мс).
func TestSimilarToCost(t *testing.T) {
	testkit.CostTest(t)
	var calls atomic.Int32
	a, ids, _ := sigApp(t, 1000, func(p library.Photo) (imgsig.Sig, error) {
		calls.Add(1)
		var n int
		_, _ = fmt.Sscanf(p.ID, "t%d", &n)
		return imgsig.Sig{D: uint64(n/4) * 0x9e3779b97f4a7c15, P: uint64(n%4) << 3, Ok: true, Sharp: 1}, nil
	})
	a.PrepareSigs(ids)
	a.bg.Wait()
	if calls.Load() != 1000 {
		t.Fatalf("подписей %d", calls.Load())
	}
	best := testkit.Fastest(20, func() {
		if res, err := a.SimilarTo("t500", ids); err != nil || len(res.IDs) < 2 {
			t.Fatalf("%+v, %v", res, err)
		}
	})
	t.Logf("SimilarTo на ленте 1000 кадров с подписями в памяти: %v (лучший из 20)", best)
	if best > testkit.Limit(50*time.Millisecond) {
		t.Errorf("SimilarTo %v: слишком долго", best)
	}
}

// TestSimilarToRealCache: настоящий кэш миниатюр и файлы — холодный вызов (подписей нет нигде:
// миниатюра и подпись считаются) и тёплый по диску (подписи в `_s.bin`, память пуста). Факты замера
// пишутся в журнал теста; лента 1000 кадров не нужна: в разбор идёт окно не больше 45 кадров.
func TestSimilarToRealCache(t *testing.T) {
	testkit.CostTest(t)
	dir := testkit.TempDir(t)
	a := newApp(t)
	c, err := library.NewCache(testkit.TempDir(t))
	if err != nil {
		t.Fatal(err)
	}
	a.cache = c
	// Предел досчёта продукта (2 с) на медленном раннере Windows не хватал на 45 миниатюр (CI
	// 0.29-c: в памяти 8 подписей): тест проверяет «досчитало всё», а не скорость диска. Ожидание
	// кончается, когда всё готово, — на быстрой машине запас ничего не стоит.
	a.sig.wait = testkit.Limit(60 * time.Second)
	var order []string
	for i := range 50 {
		img := image.NewRGBA(image.Rect(0, 0, 320, 240))
		for y := range 240 {
			for x := range 320 {
				v := uint8((x*(3+i%3) + y*2 + i) % 256)
				img.Set(x, y, color.RGBA{v, uint8(y), uint8(x), 255})
			}
		}
		path := filepath.Join(dir, fmt.Sprintf("%02d.jpg", i))
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := jpeg.Encode(f, img, nil); err != nil {
			t.Fatal(err)
		}
		_ = f.Close()
		st, _ := os.Stat(path)
		id := fmt.Sprintf("r%02d", i)
		a.byID[id] = library.Photo{ID: id, Path: path, W: 320, H: 240, Size: st.Size()}
		order = append(order, id)
	}
	t0 := time.Now()
	if _, err := a.SimilarTo("r25", order); err != nil {
		t.Fatal(err)
	}
	cold := time.Since(t0)
	if len(a.sig.mem) < 40 {
		t.Fatalf("подписей в памяти %d: холодный досчёт не успел", len(a.sig.mem))
	}
	a.bg.Wait()
	a.mu.Lock()
	a.sig.mem = nil
	a.mu.Unlock()
	t0 = time.Now()
	res, err := a.SimilarTo("r25", order)
	if err != nil {
		t.Fatal(err)
	}
	warm := time.Since(t0)
	t.Logf("SimilarTo, 45 кадров окна: холодно (миниатюры и подписи считаются) %v, тепло по диску (_s.bin) %v; ответ %+v", cold, warm, res)
	if warm > testkit.Limit(100*time.Millisecond) {
		t.Errorf("тёплый вызов %v: слишком долго", warm)
	}
}
