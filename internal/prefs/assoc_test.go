package prefs

import (
	"os"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Открытие .salbum (112): круг записи и чтения двух полей, пустые значения не пишутся,
// чужие ключи и соседние поля переживают правку.
func TestConfigAssocRoundTrip(t *testing.T) {
	dir := testkit.TempDir(t)
	if c := LoadConfig(dir); c.Assoc != "" || c.AssocLater != "" {
		t.Fatalf("без файла: %+v", c)
	}
	raw := `{"future":{"x":1},"level":"full","assoc":"on","assocLater":"0.24"}`
	if err := os.WriteFile(ConfigPath(dir), []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	if c := LoadConfig(dir); c.Assoc != "on" || c.AssocLater != "0.24" {
		t.Fatalf("чтение: %+v", c)
	}
	UpdateConfig(dir, func(c *Config) { c.Assoc, c.AssocLater = "off", "0.25"; c.LastProject = "p.salbum" })
	c := LoadConfig(dir)
	if c.Assoc != "off" || c.AssocLater != "0.25" || c.Level != "full" || c.LastProject != "p.salbum" {
		t.Errorf("круг: %+v", c)
	}
	if data, _ := os.ReadFile(ConfigPath(dir)); !strings.Contains(string(data), `"future": {`) {
		t.Errorf("чужой ключ потерян: %s", data)
	}
	// Пустые поля исчезают из файла (omitempty), остальное цело.
	UpdateConfig(dir, func(c *Config) { c.Assoc, c.AssocLater = "", "" })
	data, _ := os.ReadFile(ConfigPath(dir))
	if strings.Contains(string(data), "assoc") || !strings.Contains(string(data), `"level": "full"`) {
		t.Errorf("пустые поля остались или потерян соседний: %s", data)
	}
}
