package pack

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// limitsFile — блок limits файла testdata/pack-manifest-cases.json: пределы, которые копирует scripts/pack-check.mjs.
type limitsFile struct {
	Format       int64            `json:"format"`
	MaxZip       int64            `json:"maxZip"`
	MaxEntries   int64            `json:"maxEntries"`
	MaxManifest  int64            `json:"maxManifest"`
	MaxItems     int64            `json:"maxItems"`
	MaxTotal     int64            `json:"maxTotal"`
	Kind         map[string]int64 `json:"kind"`
	MaxDir       int64            `json:"maxDir"`
	TailLen      int64            `json:"tailLen"`
	MaxRatio     int64            `json:"maxRatio"`
	Slack        int64            `json:"slack"`
	BgMaxPixels  int64            `json:"bgMaxPixels"`
	MaxSide      int64            `json:"maxSide"`
	TplV         int64            `json:"tplV"`
	LookV        int64            `json:"lookV"`
	MaxLookSteps int64            `json:"maxLookSteps"`
}

// loadLimits читает блок limits; неизвестное поле — ошибка (новый предел не должен остаться без сверки).
func loadLimits(t testing.TB) limitsFile {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "pack-manifest-cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var file struct {
		Limits json.RawMessage `json:"limits"`
	}
	if err := json.Unmarshal(data, &file); err != nil || len(file.Limits) == 0 {
		t.Fatalf("нет блока limits: %v", err)
	}
	dec := json.NewDecoder(bytes.NewReader(file.Limits))
	dec.DisallowUnknownFields()
	var l limitsFile
	if err := dec.Decode(&l); err != nil {
		t.Fatal(err)
	}
	return l
}

// TestLimits: пределы pack равны файлу, который читает pack-check.mjs; decor.TestPackLimits сверяет остальные (packMax*, TplV, LookV).
func TestLimits(t *testing.T) {
	l := loadLimits(t)
	want := map[string][2]int64{
		"format": {l.Format, Format}, "maxZip": {l.MaxZip, MaxZip}, "maxEntries": {l.MaxEntries, MaxEntries},
		"maxManifest": {l.MaxManifest, MaxManifest}, "maxItems": {l.MaxItems, MaxItems}, "maxTotal": {l.MaxTotal, MaxTotal},
		"maxDir": {l.MaxDir, maxDir}, "tailLen": {l.TailLen, tailLen}, "maxRatio": {l.MaxRatio, maxRatio}, "slack": {l.Slack, slack},
	}
	for k, v := range want {
		if v[0] != v[1] {
			t.Errorf("%s: в файле %d, в коде %d", k, v[0], v[1])
		}
	}
	kinds := []string{KindBg, KindSticker, KindSpread, KindFrame, KindPrint, KindAlbum}
	if len(l.Kind) != len(kinds) {
		t.Errorf("видов в файле %d, в коде %d", len(l.Kind), len(kinds))
	}
	for _, k := range kinds {
		if got, ok := l.Kind[k]; !ok || got != KindLimit(k) {
			t.Errorf("предел вида %s: в файле %d, в коде %d", k, got, KindLimit(k))
		}
	}
}
