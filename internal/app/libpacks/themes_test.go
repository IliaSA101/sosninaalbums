package libpacks

import (
	"bytes"
	"log"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/decor"
	"github.com/IliaSA101/sosninaalbums/internal/decor/pack"
)

// Перенос манифеста pack в decor несёт объявления тем и topic предметов (спека 170); строка пропусков
// тем — в log.txt, одна на набор.
func TestDecorManifestThemes(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	m := pack.Manifest{ID: "ny-set", Version: 2, Title: "Новый год",
		Themes: []pack.Theme{{ID: "newyear", Name: "Новый год", Base: "holiday", Order: 10}, {ID: "snow", Name: "Снег", Base: "basic", Order: 50}},
		Items: []pack.Item{{File: "a.png", Kind: "sticker", Theme: "holiday", Topic: "newyear", Name: "A", Size: 5},
			{File: "b.png", Kind: "bg", Theme: "basic", Name: "B", Size: 6}}}
	got := decorManifest(m)
	wantThemes := []decor.PackTheme{{ID: "newyear", Name: "Новый год", Base: "holiday", Order: 10}, {ID: "snow", Name: "Снег", Base: "basic", Order: 50}}
	if !reflect.DeepEqual(got.Themes, wantThemes) {
		t.Errorf("темы: %+v", got.Themes)
	}
	if got.Items[0].Topic != "newyear" || got.Items[1].Topic != "" || got.Items[0].Theme != "holiday" {
		t.Errorf("предметы: %+v", got.Items)
	}
	if buf.Len() != 0 {
		t.Errorf("без пропусков журнал пуст: %q", buf.String())
	}
	m.ThemeSkips = pack.ThemeSkips{Why: []string{"имя", "id"}, N: 2}
	m.ID = "bad\nset" // перевод строки в id не подделывает строку журнала (%q)
	decorManifest(m)
	if out := buf.String(); strings.Count(out, "\n") != 1 || !strings.Contains(out, `набор "bad\nset": темы пропущены: 2 (имя, id)`) {
		t.Errorf("журнал: %q", out)
	}
	// Пустой манифест без тем — как раньше: Themes пусто.
	if d := decorManifest(pack.Manifest{ID: "plain-set"}); len(d.Themes) != 0 {
		t.Errorf("темы %+v", d.Themes)
	}
}
