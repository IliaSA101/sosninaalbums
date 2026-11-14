package printshop

import (
	"strings"
	"testing"
)

// TestCoverRulesUnknownPreset (123 §4, критика безопасность 6): обложка со своей типографией, которой
// нет на этом компьютере, даёт запрет с id, а не «Обложка размечена для « · »».
func TestCoverRulesUnknownPreset(t *testing.T) {
	for _, prof := range []Profile{Fabrika, Fsfera} {
		p := album(200, 203, 3, "fabrika-photo")
		if prof.Kind == "fsfera" {
			p = album(201, 152, 3, "fsfera-thin")
		}
		p.Cover.Spec.Preset = "own-0a1b2c3d-1" // id из .salbum: ClampCoverSpec его оставляет
		pl := Check(p, "", prof)
		want := "Обложка размечена для типографии, которой нет на этом компьютере (own-0a1b2c3d-1)"
		if len(pl.Block) != 1 || pl.Block[0] != want || strings.Contains(pl.Block[0], "·") || pl.CoverFix {
			t.Errorf("%s: Block %q", prof.Kind, pl.Block)
		}
	}
	// «Свои размеры» — прежняя фраза, известное изделие чужой группы — прежняя фраза с группой.
	cp := album(200, 203, 3, "fabrika-photo")
	cp.Cover.Spec.Preset = "custom"
	if pl := Check(cp, "", Fabrika); len(pl.Block) != 1 || !strings.HasPrefix(pl.Block[0], "Обложка со своими размерами") {
		t.Errorf("custom: %q", pl.Block)
	}
	if pl := Check(album(200, 203, 3, "netprint-premium"), "", Fabrika); len(pl.Block) != 1 || !strings.Contains(pl.Block[0], "Нетпринт · Премиум") {
		t.Errorf("чужая группа: %q", pl.Block)
	}
}
