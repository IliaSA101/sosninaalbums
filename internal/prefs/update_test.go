package prefs

import (
	"os"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// TestUpdatePrefs: круг запись → чтение, чужие ключи целы, всё по умолчанию — ключа нет, правка
// руками чистится.
func TestUpdatePrefs(t *testing.T) {
	dir := testkit.TempDir(t)
	if u := UpdateOf(dir); u != (Update{}) {
		t.Fatalf("без файла: %+v", u)
	}
	if err := os.WriteFile(ConfigPath(dir), []byte(`{"future":{"a":1},"level":"pro"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	want := Update{Off: true, Checked: 1234, Seen: "0.25.1", Pending: "0.25.2", PendingAt: 99}
	got := UpdateUpdate(dir, func(u *Update) { *u = want })
	if got != want || UpdateOf(dir) != want {
		t.Fatalf("круг: %+v / %+v", got, UpdateOf(dir))
	}
	data, _ := os.ReadFile(ConfigPath(dir))
	if !strings.Contains(string(data), `"future"`) || !strings.Contains(string(data), `"update"`) || LoadConfig(dir).Level != "pro" {
		t.Fatalf("чужие ключи или ключ update: %s", data)
	}
	// Правка одного поля не трогает остальные (флажок и итог — из разных методов).
	UpdateUpdate(dir, func(u *Update) { u.Off = false })
	if u := UpdateOf(dir); u.Off || u.Seen != "0.25.1" || u.Pending != "0.25.2" || u.Checked != 1234 {
		t.Fatalf("одно поле: %+v", u)
	}
	// Итог снят — отметка времени уходит вместе с версией.
	UpdateUpdate(dir, func(u *Update) { u.Pending = "" })
	if u := UpdateOf(dir); u.Pending != "" || u.PendingAt != 0 {
		t.Fatalf("итог снят: %+v", u)
	}
	// Всё по умолчанию — ключа в файле нет.
	UpdateUpdate(dir, func(u *Update) { *u = Update{} })
	if data, _ := os.ReadFile(ConfigPath(dir)); strings.Contains(string(data), `"update"`) || !strings.Contains(string(data), `"future"`) {
		t.Fatalf("пустой ключ записан или потерян чужой: %s", data)
	}
	// Правка руками: плохие версии и отрицательное время.
	raw := `{"update":{"off":true,"checked":-5,"seen":"скоро","pending":"1.2","pendingAt":-3}}`
	if err := os.WriteFile(ConfigPath(dir), []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	if u := UpdateOf(dir); u != (Update{Off: true}) {
		t.Fatalf("чистка: %+v", u)
	}
}

func TestValidVersion(t *testing.T) {
	for s, want := range map[string]bool{"0.25.1": true, "9999.9999.9999": true, "": false, "1.2": false, "1.2.3.4": false, "1..3": false,
		"10000.1.1": false, "1.2.3-rc": false, "a.b.c": false, ".1.2": false, "1.2.": false} {
		if validVersion(s) != want {
			t.Errorf("%q: ждали %v", s, want)
		}
	}
}
