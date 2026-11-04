package packprep

import (
	"fmt"
	"path"
	"slices"
	"strings"
)

// modelTokens — части имени файла, с которых начинается «модель и вариант»: семейство — всё до них.
var modelTokens = []string{"flux", "mystic", "nb2", "recraft"}

// kindTokens — второй токен имени, обозначающий вид (kg-fr-toys2, ns-frame-diagonal): после него
// семейство берёт ещё один токен — мотив.
var kindTokens = []string{"fr", "bg", "div", "co", "spread", "frame", "corners", "dividers"}

// familyOf — семейство предмета из имени файла (0.27-129-19): sf-leaves-corners-2-flux.jpg → sf-leaves,
// kg-fr-toys2-flux-1.jpg → kg-fr-toys2, kg-bg-rain-mystic-4.png → kg-bg-rain. Правило грубое
// (мотив — токен после набора или после вида): где оно ошибается, в рецепте задаётся family.
func familyOf(src string) string {
	base := strings.ToLower(strings.TrimSuffix(path.Base(src), path.Ext(src)))
	toks := strings.Split(base, "-")
	for i, t := range toks {
		if slices.Contains(modelTokens, t) {
			toks = toks[:i]
			break
		}
	}
	if len(toks) == 0 {
		return base
	}
	keep := 2
	if len(toks) > 1 && slices.Contains(kindTokens, toks[1]) {
		keep = 3
	}
	return strings.Join(toks[:min(keep, len(toks))], "-")
}

// familyOfItem — семейство предмета рецепта: заданное или из имени файла.
func familyOfItem(it Item) string {
	if it.Family != "" {
		return it.Family
	}
	return familyOf(it.Src)
}

// checkFamilies — не больше maxFamilyCap источников одного семейства (0.27-129-19). Считаются
// разные src; отбракованные (skip на весь src) не в счёт. Ошибка на каждое нарушенное семейство.
func checkFamilies(r *Recipe) []string {
	members := map[string][]string{}
	var order []string
	for _, it := range r.Items {
		if _, dead := r.Skip[it.Src]; dead {
			continue
		}
		fam := familyOfItem(it)
		if !slices.Contains(members[fam], it.Src) {
			if members[fam] == nil {
				order = append(order, fam)
			}
			members[fam] = append(members[fam], it.Src)
		}
	}
	var errs []string
	for _, fam := range order {
		if m := members[fam]; len(m) > maxFamilyCap {
			errs = append(errs, fmt.Sprintf("семейство «%s»: %d источников (%s), допустимо не больше %d", fam, len(m), strings.Join(m, ", "), maxFamilyCap))
		}
	}
	return errs
}
