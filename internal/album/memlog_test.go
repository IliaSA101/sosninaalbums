package album_test

import (
	"regexp"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
)

// TestMemLogLine — «память: <этап> — Go N МБ, куча M МБ» (089 G3): формат под разбор e2e-отчётом,
// несколько этапов подряд, этап с пробелами и без путей. Строку автосборки пишет App.AutoBuild
// (internal/app/edit, TestAutoBuildLogsMem): Editor ниже порога медленных операций молчит.
func TestMemLogLine(t *testing.T) {
	re := regexp.MustCompile(`(?m)^.* память: (.+?) — Go (\d+) МБ, куча (\d+) МБ$`)
	buf := captureLog(t)
	stages := []string{"чтение папок", "прогрев миниатюр", "экспорт"}
	for _, stage := range stages {
		album.LogMem(stage)
	}
	got := re.FindAllStringSubmatch(buf.String(), -1)
	if len(got) != len(stages) {
		t.Fatalf("строк памяти %d, ждали %d: %q", len(got), len(stages), buf.String())
	}
	for i, m := range got {
		if m[1] != stages[i] {
			t.Errorf("этап %q, ждали %q", m[1], stages[i])
		}
		if m[2] == "0" { // процесс Go всегда держит у системы не меньше мегабайта
			t.Errorf("Go 0 МБ: %v", m)
		}
	}
}
