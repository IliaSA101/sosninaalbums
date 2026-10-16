package gen

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"image"
	"io/fs"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Обложка 680×350 мм при 300 dpi (спека 048, критерии): 8031×4133 px.
const (
	coverK = 300 / 25.4
	coverW = 8031
	coverH = 4133
)

// TestGenCoverFast — каждый узор на обложку 300 dpi быстрее Limit(3 s) и без лишней памяти
// сверх холста (полосы маски, мягкий слой, текстура — ≤ 64 МБ); плитка 160 px — Limit(10 ms).
func TestGenCoverFast(t *testing.T) {
	if testing.Short() || testkit.Race {
		t.Skip("замер времени: обычный прогон без -short и -race")
	}
	dst := image.NewRGBA(image.Rect(0, 0, coverW, coverH))
	var slowest time.Duration
	var slowKind string
	for _, m := range Patterns() {
		clear(dst.Pix)
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		start := time.Now()
		DrawPattern(dst, dst.Rect, Spec{Kind: m.Kind, Seed: 1}, coverK, 0)
		d := time.Since(start)
		runtime.ReadMemStats(&after)
		if extra := after.TotalAlloc - before.TotalAlloc; extra > 64<<20 {
			t.Errorf("%s: обложка заняла %d МБ сверх холста", m.Kind, extra>>20)
		}
		if d > slowest {
			slowest, slowKind = d, m.Kind
		}
		if d > testkit.Limit(3*time.Second) {
			t.Errorf("%s: обложка 300 dpi %v", m.Kind, d)
		}
		// Плитка каталога — разворот 600×300 мм шириной 160 px.
		tile := image.NewRGBA(image.Rect(0, 0, 160, 80))
		best := testkit.Fastest(5, func() { DrawPattern(tile, tile.Rect, Spec{Kind: m.Kind, Seed: 1}, 160.0/600, 0) })
		if best > testkit.Limit(10*time.Millisecond) {
			t.Errorf("%s: плитка 160 px %v", m.Kind, best)
		}
	}
	t.Logf("самый тяжёлый узор на обложке: %s, %v", slowKind, slowest)
}

// BenchmarkGenPatterns — каждый узор: обложка 300 dpi, экран (60 dpi) и плитка 160 px.
func BenchmarkGenPatterns(b *testing.B) {
	sizes := []struct {
		name string
		k    float64
		w, h int
	}{
		{"cover300", coverK, coverW, coverH},
		{"screen60", 60 / 25.4, 1606, 827},
		{"tile160", 160.0 / 680, 160, 82},
	}
	for _, m := range Patterns() {
		for _, sz := range sizes {
			b.Run(m.Kind+"/"+sz.name, func(b *testing.B) {
				dst := image.NewRGBA(image.Rect(0, 0, sz.w, sz.h))
				for b.Loop() {
					DrawPattern(dst, dst.Rect, Spec{Kind: m.Kind, Seed: 1}, sz.k, 0)
				}
			})
		}
	}
}

// source — запись textures/sources.json (research §1).
type source struct {
	File, Title, URL, Author, License, Date, SHA256, Note string
}

// TestTexturesLicensed — каждый вшитый файл текстуры описан в sources.json: CC0-1.0, адрес,
// автор, дата, sha256 совпадает; лишних записей и файлов нет; вместе ≤ 2 МБ; текстура в
// каталоге ⇔ её файл вшит.
func TestTexturesLicensed(t *testing.T) {
	data, err := textureFS.ReadFile("textures/sources.json")
	if err != nil {
		t.Fatal(err)
	}
	var list []source
	if err := json.Unmarshal(data, &list); err != nil {
		t.Fatalf("sources.json: %v", err)
	}
	listed := map[string]bool{}
	total := 0
	for _, s := range list {
		if listed[s.File] || s.License != "CC0-1.0" || !strings.HasPrefix(s.URL, "https://") || s.Author == "" || s.Date == "" {
			t.Errorf("%+v: повтор или нет лицензии CC0-1.0, адреса, автора, даты", s)
		}
		listed[s.File] = true
		b, err := textureFS.ReadFile("textures/" + s.File)
		if err != nil {
			t.Errorf("%s: файла нет", s.File)
			continue
		}
		total += len(b)
		if sum := sha256.Sum256(b); hex.EncodeToString(sum[:]) != strings.ToLower(s.SHA256) {
			t.Errorf("%s: sha256 не совпал", s.File)
		}
	}
	ents, _ := fs.ReadDir(textureFS, "textures")
	for _, e := range ents {
		if e.Name() != "sources.json" && !listed[e.Name()] {
			t.Errorf("%s: файл без записи в sources.json", e.Name())
		}
	}
	if total > 2<<20 {
		t.Errorf("текстуры вместе %d байт > 2 МБ", total)
	}
	for _, g := range patternGens {
		if g.meta.Paper {
			file := g.meta.Kind + ".jpg"
			if Known(g.meta.Kind) != listed[file] {
				t.Errorf("%s: Known = %v, файл в sources.json = %v", g.meta.Kind, Known(g.meta.Kind), listed[file])
			}
		}
	}
}
