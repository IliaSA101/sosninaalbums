//go:build ignore

// library-testvideos-verify проверяет тестовый каталог глазами программы (140, Т7): конверт каталога
// открывает fetch.Stand + Client.Parse с открытым тестовым ключом, как стенд с SA_LIBRARY_URL/SA_LIBRARY_KEY.
//
//	go run scripts/library-testvideos-verify.go <catalog.json> <открытый ключ base64> [id/версия]...
//
// Печатает наборы и ролики, которые взял разбор, и пропущенные строки. Код 1 — каталог не прошёл, есть
// пропуск или названного ролика нет среди разобранных. Инструмент разработчика, не функция программы.
package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/decor/fetch"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "использование: go run scripts/library-testvideos-verify.go <catalog.json> <ключ base64> [id/версия]...")
		os.Exit(2)
	}
	// Файл читается с пределом: чужой каталог не должен тянуть память (конверт ≤ fetch.MaxEnvelope).
	st, err := os.Stat(os.Args[1])
	if err != nil || !st.Mode().IsRegular() || st.Size() > fetch.MaxEnvelope {
		fmt.Fprintln(os.Stderr, "каталог: не обычный файл или больше предела")
		os.Exit(1)
	}
	env, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, "каталог:", err)
		os.Exit(1)
	}
	src, ok := fetch.Stand("http://127.0.0.1:34190/", os.Args[2])
	if !ok {
		fmt.Fprintln(os.Stderr, "fetch.Stand: источник не принят")
		os.Exit(1)
	}
	cat, err := fetch.New(src).Parse(env, 0)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Parse:", err)
		os.Exit(1)
	}
	got := map[string]bool{}
	fmt.Printf("Parse: seq %d, наборов %d, роликов %d, пропущено %d\n", cat.Seq, len(cat.Packs), len(cat.Videos), len(cat.Skipped))
	for _, v := range cat.Videos {
		kinds := make([]string, 0, len(v.Files))
		for k, f := range v.Files {
			kinds = append(kinds, fmt.Sprintf("%s %d Б", k, f.Size))
		}
		sort.Strings(kinds)
		got[fmt.Sprintf("%s/%d", v.ID, v.Version)] = true
		fmt.Printf("  ролик %s v%d: %d с, minApp %s, %s\n", v.ID, v.Version, v.Seconds, v.MinApp, strings.Join(kinds, ", "))
	}
	bad := false
	for _, s := range cat.Skipped {
		fmt.Printf("  ПРОПУЩЕНО %s: %s\n", s.ID, s.Why)
		bad = true
	}
	for _, w := range os.Args[3:] {
		if !got[w] {
			fmt.Printf("  НЕТ в разборе: %s\n", w)
			bad = true
		}
	}
	if bad {
		os.Exit(1)
	}
}
