package decor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// TestPackLimits: пределы содержимого набора (packMax*, версии шаблона и макета, шагов макета) равны блоку limits
// файла testdata/pack-manifest-cases.json, который копирует scripts/pack-check.mjs; пределы архива сверяет pack.TestLimits.
func TestPackLimits(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "pack-manifest-cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var file struct {
		Limits struct {
			BgMaxPixels  int64 `json:"bgMaxPixels"`
			MaxSide      int64 `json:"maxSide"`
			TplV         int64 `json:"tplV"`
			LookV        int64 `json:"lookV"`
			MaxLookSteps int64 `json:"maxLookSteps"`
		} `json:"limits"`
	}
	if err := json.Unmarshal(data, &file); err != nil {
		t.Fatal(err)
	}
	for name, v := range map[string][2]int64{
		"bgMaxPixels": {file.Limits.BgMaxPixels, packMaxBgPixels}, "maxSide": {file.Limits.MaxSide, packMaxSide},
		"tplV": {file.Limits.TplV, model.TplV}, "lookV": {file.Limits.LookV, model.LookV},
		"maxLookSteps": {file.Limits.MaxLookSteps, model.MaxLookSteps},
	} {
		if v[0] != v[1] {
			t.Errorf("%s: в файле %d, в коде %d", name, v[0], v[1])
		}
	}
}
