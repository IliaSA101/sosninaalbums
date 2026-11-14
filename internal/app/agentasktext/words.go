package agentasktext

// Слова и числа для текстов: склонение по числу и список номеров. Те же правила нужны тостам и журналу
// в app, поэтому они здесь одни на всех.

import (
	"slices"
	"strconv"
	"strings"
)

// Plural — форма слова при числе n: 1 файл, 2 файла, 5 файлов (11–14 — «много»).
func Plural(n int, one, few, many string) string {
	n %= 100
	if n < 0 {
		n = -n
	}
	switch {
	case n >= 11 && n <= 14:
		return many
	case n%10 == 1:
		return one
	case n%10 >= 2 && n%10 <= 4:
		return few
	}
	return many
}

// numsMax — сколько номеров перечислять в тосте и вопросе про фон альбома; остальные — «и ещё N»
// (OWN_NUMS_MAX человека).
const numsMax = 12

// NumsText — «3, 5-8, 18»: подряд идущие три и больше — диапазоном; больше numsMax номеров — первые и
// «, и ещё N» (formatRanges и ownNumsText человека).
func NumsText(nums []int) string {
	nums = slices.Compact(slices.Sorted(slices.Values(nums)))
	head, rest := nums, 0
	if len(nums) > numsMax {
		head, rest = nums[:numsMax], len(nums)-numsMax
	}
	var parts []string
	for i := 0; i < len(head); {
		j := i
		for j+1 < len(head) && head[j+1] == head[j]+1 {
			j++
		}
		if j-i >= 2 {
			parts = append(parts, strconv.Itoa(head[i])+"-"+strconv.Itoa(head[j]))
		} else {
			for _, n := range head[i : j+1] {
				parts = append(parts, strconv.Itoa(n))
			}
		}
		i = j + 1
	}
	out := strings.Join(parts, ", ")
	if rest > 0 {
		out += ", и ещё " + strconv.Itoa(rest)
	}
	return out
}
