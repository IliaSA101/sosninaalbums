package similar

import (
	"fmt"
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

func wantIdx(t *testing.T, got []int, more int, want []int, wantMore int) {
	t.Helper()
	if !slices.Equal(got, want) || more != wantMore {
		t.Fatalf("idx %v more %d, ждали %v more %d", got, more, want, wantMore)
	}
}

// TestAroundNearestFirst — берутся ближайшие по Dist, ничья решается близостью по ленте, затем
// ранним; результат — в порядке ленты, а не по близости.
func TestAroundNearestFirst(t *testing.T) {
	// Кадр 2 — near(0); соседи: 0 (Dist 9), 1 (Dist 3), 3 (Dist 3), 4 (Dist 5), 5 (Dist 20).
	in := shots(near(9), near(3), near(0), near(3), near(5), near(20))
	idx, more := Around(in, 2, 3)
	wantIdx(t, idx, more, []int{1, 2, 3}, 3) // 1 и 3 — ближайшие (Dist 3), 0, 4, 5 не вошли
	// Ничья по Dist 3 и по расстоянию в ленте (0 и 4 от кадра 2): раньше — 0; кадр 5 с Dist 34 тоже в числе похожих.
	in = shots(near(3), far(t, 1), near(0), far(t, 2), near(3), near(40-6))
	idx, more = Around(in, 2, 2)
	wantIdx(t, idx, more, []int{0, 2}, 2)
	// Порог «Похожие»: 34 входит, 35 — нет.
	idx, more = Around(shots(near(0), near(34), near(35)), 0, 4)
	wantIdx(t, idx, more, []int{0, 1}, 0)
}

// TestAroundLimitMore — похожих больше limit−1: лишние считаются в more; limit 1 — только сам.
func TestAroundLimitMore(t *testing.T) {
	in := shots(near(0), near(1), near(2), near(3), near(4), near(5))
	idx, more := Around(in, 0, 4)
	wantIdx(t, idx, more, []int{0, 1, 2, 3}, 2)
	idx, more = Around(in, 0, 6)
	wantIdx(t, idx, more, []int{0, 1, 2, 3, 4, 5}, 0)
	idx, more = Around(in, 0, 1)
	wantIdx(t, idx, more, []int{0}, 5)
	if idx, more = Around(in, 0, 0); idx != nil || more != 0 {
		t.Errorf("limit 0: %v %d", idx, more)
	}
	// Окно ±Window: 13-й по ленте не виден.
	wide := shots(near(0))
	for k := 1; k <= Window; k++ {
		wide = append(wide, shot(k, far(t, k)))
	}
	wide = append(wide, shot(Window+1, near(0)))
	idx, _ = Around(wide, 0, 4)
	wantIdx(t, idx, 0, []int{0}, 0)
	wide[Window].Sig = near(0) // двенадцатый — в окне
	idx, _ = Around(wide, 0, 4)
	wantIdx(t, idx, 0, []int{0, Window}, 0)
}

// TestAroundWallGapOrientation — стена серии, разрыв по времени и другая ориентация отсекают пару.
func TestAroundWallGapOrientation(t *testing.T) {
	// Другая папка — стена.
	in := shots(near(0), near(1), near(1), near(1))
	for i := 2; i < 4; i++ {
		in[i].Path = fmt.Sprintf("/съёмка/b/%02d.jpg", i)
	}
	idx, more := Around(in, 0, 4)
	wantIdx(t, idx, more, []int{0, 1}, 0)
	// Разрыв по времени больше MaxGap.
	in = shots(near(0), near(1), near(1))
	const t0 = 1_700_000_000
	in[0].Taken, in[1].Taken, in[2].Taken = t0, t0+100, t0+100+MaxGap+1
	idx, more = Around(in, 0, 4)
	wantIdx(t, idx, more, []int{0, 1}, 0)
	idx, more = Around(in, 2, 4)
	wantIdx(t, idx, more, []int{2}, 0)
	// Ориентация: вертикальный не сравнивается с горизонтальным.
	in = shots(near(0), near(1), near(1))
	in[1].W, in[1].H = 200, 300
	idx, more = Around(in, 0, 4)
	wantIdx(t, idx, more, []int{0, 2}, 0)
}

// TestAroundNoSig — у кадра нет подписи: nil; у соседей нет — они пропускаются.
func TestAroundNoSig(t *testing.T) {
	in := shots(near(0), near(1), near(2))
	in[0].Sig.Ok = false
	if idx, more := Around(in, 0, 4); idx != nil || more != 0 {
		t.Errorf("кадр без подписи: %v %d", idx, more)
	}
	idx, more := Around(in, 1, 4)
	wantIdx(t, idx, more, []int{1, 2}, 0)
	for _, at := range []int{-1, 3, 100} {
		if idx, _ := Around(in, at, 4); idx != nil {
			t.Errorf("at %d вне входа: %v", at, idx)
		}
	}
	if idx, more := Around(nil, 0, 4); idx != nil || more != 0 {
		t.Errorf("пустой вход: %v %d", idx, more)
	}
}

// TestAroundLibSkipped — свой из библиотеки (Lib) ни с чем не сравнивается и сам не берётся.
func TestAroundLibSkipped(t *testing.T) {
	in := shots(near(0), near(1), near(2))
	in[1].Lib = "my:0123456789abcdef"
	idx, more := Around(in, 0, 4)
	wantIdx(t, idx, more, []int{0, 2}, 0)
	if idx, _ := Around(in, 1, 4); idx != nil {
		t.Errorf("свой кадр как исходный: %v", idx)
	}
}

// TestAroundEdges — кадр первый и последний в ленте; один кадр — только он сам.
func TestAroundEdges(t *testing.T) {
	in := shots(near(0), near(1), near(2), far(t, 1))
	idx, more := Around(in, 0, 4)
	wantIdx(t, idx, more, []int{0, 1, 2}, 0)
	idx, more = Around(in, 2, 4)
	wantIdx(t, idx, more, []int{0, 1, 2}, 0)
	idx, more = Around(in, 3, 4)
	wantIdx(t, idx, more, []int{3}, 0)
	idx, more = Around(in[:1], 0, 4)
	wantIdx(t, idx, more, []int{0}, 0)
	// Последний в ленте, похожие — слева.
	idx, more = Around(shots(near(1), near(2), near(0)), 2, 4)
	wantIdx(t, idx, more, []int{0, 1, 2}, 0)
	// Входной срез не меняется.
	before := slices.Clone(in)
	Around(in, 1, 4)
	if !slices.EqualFunc(in, before, func(a, b model.PhotoInfo) bool { return a == b }) {
		t.Error("Around изменил вход")
	}
}
