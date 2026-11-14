package salbum

import (
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// TestFoldLegacyVersions — foldLegacy и migrateV11 по одному условию (foldWas): до v11, layflat,
// не лист. Края: v1 (самый старый), v10, v11 (уже правило сгиба); FoldAny из файла v11 не
// трогается, у «Нет» и листа не ставится.
func TestFoldLegacyVersions(t *testing.T) {
	cases := []struct {
		from           int
		layflat, sheet bool
		foldAny        bool // в файле
		was, wantAny   bool
	}{
		{1, true, false, false, true, true},
		{5, true, false, false, true, true},
		{10, true, false, false, true, true},
		{11, true, false, false, false, false},
		{11, true, false, true, false, true},
		{10, false, false, false, false, false},
		{10, true, true, false, false, false},
		{10, false, true, false, false, false},
	}
	for _, c := range cases {
		s := model.Settings{Layflat: c.layflat, Sheet: c.sheet, FoldAny: c.foldAny}
		if got := foldWas(c.from, s); got != c.was {
			t.Errorf("v%d layflat %v sheet %v: foldWas %v, ждали %v", c.from, c.layflat, c.sheet, got, c.was)
		}
		if got := foldLegacy(c.from, s); got.FoldAny != c.wantAny || got.Layflat != c.layflat || got.Sheet != c.sheet {
			t.Errorf("v%d layflat %v sheet %v foldAny %v: foldLegacy → %+v, ждали foldAny %v", c.from, c.layflat, c.sheet, c.foldAny, got, c.wantAny)
		}
	}
}
