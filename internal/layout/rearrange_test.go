package layout

import (
	"reflect"
	"strings"
	"testing"
)

func TestPermSearchMore(t *testing.T) {
	ps := &permSearch{used: permBudget - 1}
	if !ps.more() || ps.stopped {
		t.Fatalf("бюджет не кончился, а перебор остановлен")
	}
	ps.used = permBudget
	if ps.more() || !ps.stopped {
		t.Fatalf("бюджет кончился, а перебор не остановлен (stopped=%t)", ps.stopped)
	}
}

func TestLeftSetsOrder(t *testing.T) {
	// От меньших к большим, при равном размере — в порядке перебора.
	got := leftSets([]int{1, 2})
	want := [][]int{{0, 0}, {0, 1}, {1, 0}, {0, 2}, {1, 1}, {1, 2}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("leftSets = %v, ждали %v", got, want)
	}
}

func TestPermRunSplit(t *testing.T) {
	run := newPermRun(&permSearch{}, album30, []int{0, 1, 0, 1, 0}, 2)
	lp, rp, rm := run.split([]int{1, 2})
	// Левая берёт самые ранние фото каждого класса.
	if want := [][]int{{0}, {1, 3}}; !reflect.DeepEqual(lp, want) {
		t.Errorf("левая %v, ждали %v", lp, want)
	}
	if want := [][]int{{2, 4}, {}}; !reflect.DeepEqual(rp, want) {
		t.Errorf("правая %v, ждали %v", rp, want)
	}
	if want := []int{2, 0}; !reflect.DeepEqual(rm, want) {
		t.Errorf("мультимножество правой %v, ждали %v", rm, want)
	}
}

func TestPutPage(t *testing.T) {
	rects := make([]Rect, 3)
	if k, idx := putPage(rects, pageBlock{key: "-"}, nil); k != "-" || idx != nil || rects[0] != (Rect{}) {
		t.Fatalf("пустая страница: ключ %q, фото %v", k, idx)
	}
	r0, r1 := Rect{1, 2, 3, 4}, Rect{5, 6, 7, 8}
	b := pageBlock{seq: []int{0, 1}, key: "H(0,1)", rects: []Rect{r0, r1}}
	k, idx := putPage(rects, b, [][]int{{2}, {0}})
	if k != "H(2,0)" || !reflect.DeepEqual(idx, []int{2, 0}) {
		t.Fatalf("ключ %q, фото %v", k, idx)
	}
	if rects[2] != r0 || rects[0] != r1 || rects[1] != (Rect{}) {
		t.Fatalf("прямоугольники не на своих фото: %+v", rects)
	}
}

func TestPageTopDedup(t *testing.T) {
	ps := &permSearch{seqs: map[string][]pageBlock{}}
	run := newPermRun(ps, album30, []int{0, 1, 0}, 2)
	// Блоки в порядке ленты — из кэша последовательностей, без перебора.
	ps.seqs[seqKey([]int{0, 1})] = []pageBlock{{seq: []int{0, 1}, key: "A", score: 0.5}}
	m := []int{1, 1}
	picks := [][]int{{0}, {1}}
	run.full = map[string][]pageBlock{seqKey(m): {
		{seq: []int{1, 0}, key: "B", score: 0.9},
		{seq: []int{1, 0}, key: "A", score: 0.55},
		{seq: []int{0, 1}, key: "A", score: 0.5},
	}}
	got := pageKeys(run.pageTop(m, picks))
	// Дубль — тот же ключ на той же последовательности; тот же ключ на другой — не дубль.
	if want := []string{"1,0,B", "1,0,A", "0,1,A"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("pageTop = %v, ждали %v", got, want)
	}
	// Больше perSide различных — обрезается по оценке.
	run.full[seqKey(m)] = append(run.full[seqKey(m)],
		pageBlock{seq: []int{1, 0}, key: "C", score: 0.7},
		pageBlock{seq: []int{1, 0}, key: "D", score: 0.6})
	got = pageKeys(run.pageTop(m, picks))
	if want := []string{"1,0,B", "1,0,C", "1,0,D", "1,0,A"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("pageTop = %v, ждали %v", got, want)
	}
}

func pageKeys(bs []pageBlock) []string {
	out := make([]string, len(bs))
	for i, b := range bs {
		out[i] = seqKey(b.seq) + b.key
	}
	return out
}

func TestPagedEmptyPage(t *testing.T) {
	// До maxPermPage фото одна страница может остаться пустой, больше — нет.
	for _, c := range []struct {
		asp   []float64
		empty bool
	}{
		{[]float64{P, L}, true},
		{[]float64{P, L, P, L, P, L}, false},
	} {
		cls, rep := classes(c.asp)
		ps := &permSearch{g: newGeom(album30), rep: rep, seqs: map[string][]pageBlock{}}
		run := newPermRun(ps, album30, cls, len(rep))
		subs := leftSets(run.cnt)
		run.full = run.searchPages(subs)
		all := run.paged(subs)
		if len(all) == 0 {
			t.Fatalf("%v: нет постраничных раскладок", c.asp)
		}
		empty := false
		for _, r := range all {
			if strings.HasPrefix(r.Key, "P[-|") || strings.HasSuffix(r.Key, "|-]") {
				empty = true
			}
			checkResult(t, c.asp, album30, r)
		}
		if empty != c.empty {
			t.Errorf("%v: пустая страница %t, ждали %t", c.asp, empty, c.empty)
		}
	}
}
