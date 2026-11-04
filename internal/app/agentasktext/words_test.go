package agentasktext

import "testing"

// TestNumsText: диапазоны от трёх подряд, больше двенадцати — «и ещё N».
func TestNumsText(t *testing.T) {
	long := make([]int, 20)
	for i := range long {
		long[i] = i*2 + 2
	}
	for _, c := range []struct {
		in   []int
		want string
	}{{nil, ""}, {[]int{7, 3, 18}, "3, 7, 18"}, {[]int{3, 5, 6, 7, 8}, "3, 5-8"}, {[]int{1, 2}, "1, 2"}, {long, "2, 4, 6, 8, 10, 12, 14, 16, 18, 20, 22, 24, и ещё 8"}} {
		if got := NumsText(c.in); got != c.want {
			t.Errorf("%v: %q, ждали %q", c.in, got, c.want)
		}
	}
}

func TestPlural(t *testing.T) {
	for n, want := range map[int]string{0: "файлов", 1: "файл", 2: "файла", 4: "файла", 5: "файлов", 11: "файлов", 14: "файлов", 21: "файл", 22: "файла", 111: "файлов", 312: "файлов", -1: "файл"} {
		if got := Plural(n, "файл", "файла", "файлов"); got != want {
			t.Errorf("%d: %q, ждали %q", n, got, want)
		}
	}
}
