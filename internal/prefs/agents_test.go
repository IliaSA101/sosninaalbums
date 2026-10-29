package prefs

import (
	"os"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// TestAgentsPrefs: круг запись → чтение, чужие ключи целы, умолчание — ключа нет, мусор уровня
// чистится, правка одного поля не трогает другое.
func TestAgentsPrefs(t *testing.T) {
	dir := testkit.TempDir(t)
	if a := AgentsOf(dir); a != (Agents{Level: 1}) {
		t.Fatalf("без файла: %+v", a)
	}
	if err := os.WriteFile(ConfigPath(dir), []byte(`{"future":{"a":1},"level":"pro"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	want := Agents{On: true, Level: 3}
	if got := UpdateAgents(dir, func(a *Agents) { *a = want }); got != want || AgentsOf(dir) != want {
		t.Fatalf("круг: %+v / %+v", got, AgentsOf(dir))
	}
	data, _ := os.ReadFile(ConfigPath(dir))
	if !strings.Contains(string(data), `"future"`) || !strings.Contains(string(data), `"agents"`) || LoadConfig(dir).Level != "pro" {
		t.Fatalf("чужие ключи или ключ agents: %s", data)
	}
	// Выключение не трогает уровень, смена уровня — включение.
	UpdateAgents(dir, func(a *Agents) { a.On = false })
	if a := AgentsOf(dir); a.On || a.Level != 3 {
		t.Fatalf("выключено, уровень цел: %+v", a)
	}
	UpdateAgents(dir, func(a *Agents) { a.On, a.Level = true, 2 })
	if a := AgentsOf(dir); !a.On || a.Level != 2 {
		t.Fatalf("уровень 2: %+v", a)
	}
	// Края: вне 1…3 — уровень 1 (и при записи, и при чтении правки руками).
	for _, lv := range []int{0, -1, 4, 1 << 30} {
		if got := UpdateAgents(dir, func(a *Agents) { a.Level = lv }); got.Level != 1 {
			t.Fatalf("уровень %d → %d", lv, got.Level)
		}
	}
	if err := os.WriteFile(ConfigPath(dir), []byte(`{"agents":{"on":true,"level":99}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if a := AgentsOf(dir); a != (Agents{On: true, Level: 1}) {
		t.Fatalf("правка руками: %+v", a)
	}
	// Всё по умолчанию — ключа в файле нет.
	UpdateAgents(dir, func(a *Agents) { *a = Agents{} })
	if data, _ := os.ReadFile(ConfigPath(dir)); strings.Contains(string(data), `"agents"`) {
		t.Fatalf("умолчание осталось в файле: %s", data)
	}
	if a := AgentsOf(dir); a != (Agents{Level: 1}) {
		t.Fatalf("после сброса: %+v", a)
	}
}
