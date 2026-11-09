package printprof

import (
	"crypto/sha256"
	"encoding/hex"
	"math"
	"strconv"
	"testing"
)

func nan() float64 { return math.NaN() }

func itoa(n int) string { return strconv.Itoa(n) }

func hashOf(t *testing.T, name string) hashEntry {
	t.Helper()
	data := builtinData(t, name)
	p, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	return hashEntry{SHA256: hex.EncodeToString(sum[:]), Version: p.Version}
}

// fullProfile — профиль, где заняты все поля и все ссылки (срезы, карта, указатели).
func fullProfile() Profile {
	return Profile{
		Format: 1, ID: "own-12345678", Version: 3, Name: "Своя", Gen: "Своей",
		Page: Page{DPI: 300, Layflat: true, Safe: 10.01, Bleed: 3, Hint: "не ближе {safe} мм"},
		Formats: []PageFormat{{Name: "30×20", Page: Pair{304, 203}, Px: &PxPair{7181, 2398}, Safe: 8, Bleed: 2, Side: &Pair{310, 206}, Wrap: 17,
			Guide: &Guide{Bleed: &Pair{3, 3.5}, Fold: &Pair{20.5, 18.5}}}},
		Cover: &Cover{Wrap: 17, Safe: 8, SpineSafe: 12, FoldSafe: 3, GuideSpineSafe: 12},
		Products: []Product{{ID: "own-12345678-1", Name: "Изд", Pages: "Бумага", Item: "Книга", URL: "https://example.com/a",
			Spine: []float64{4, 8, 8, 10}, SpineBy: map[string][]float64{"30×20": {4, 8, 8, 10, 10, 12}},
			Wrap: 15, Limits: []Limit{{Min: 2, Max: 20, Pages: true}, {Format: "30×20", Min: 4}}}},
		Default: "own-12345678-1", NameDefault: true,
		Export: &Export{Acc: "Свою", Title: "Для своей", Prefix: "1-", Quality: 95, CoverWord: "обложки", PagesCap: "Бумага: ", Plain: "Книга",
			WithCover: ", обложка", Done: "Готово", Site: "{item} → загрузить", SiteNoItem: "изделие", SiteButton: "Открыть", URL: "https://example.com/"},
	}
}
