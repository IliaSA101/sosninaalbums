package library

import (
	"image/color"
	"path/filepath"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/imgfile"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// TestCacheCached: готовая копия находится без оригинала и без знания цвета — по любому классу ключа; пустой кэш,
// другой файл (иной размер) и пропавший оригинал — «нет». Миниатюра и превью — разные копии. Блоки независимы.
func TestCacheCached(t *testing.T) {
	dir := testkit.TempDir(t)
	c, err := NewCache(filepath.Join(dir, "cache"))
	if err != nil {
		t.Fatal(err)
	}
	jpg := testkit.JPEG(t, dir, "a.jpg", 64, 48)
	pngPath := testkit.PNG(t, dir, "b.png", 16, 12, color.NRGBA{R: 200, A: 255})

	t.Run("пустой кэш", func(t *testing.T) {
		if f, ok := c.Cached(Photo{Path: jpg}, false); ok {
			t.Errorf("копия из пустого кэша: %s", f)
		}
	})
	t.Run("миниатюра и превью", func(t *testing.T) {
		p := Photo{Path: jpg}
		thumb, err := c.File(p, false)
		if err != nil {
			t.Fatal(err)
		}
		prev, err := c.File(p, true)
		if err != nil {
			t.Fatal(err)
		}
		t1, ok1 := c.Cached(p, false)
		p1, ok2 := c.Cached(p, true)
		if !ok1 || !ok2 || t1 != thumb || p1 != prev || thumb == prev {
			t.Errorf("Cached: %q %v / %q %v; ожидалось %q и %q", t1, ok1, p1, ok2, thumb, prev)
		}
	})
	t.Run("PNG", func(t *testing.T) {
		want, err := c.File(Photo{Path: pngPath}, false)
		if err != nil {
			t.Fatal(err)
		}
		if got, ok := c.Cached(Photo{Path: pngPath}, false); !ok || got != want {
			t.Errorf("Cached(png) = %q, %v; ожидалось %q", got, ok, want)
		}
	})
	t.Run("файл перезаписан", func(t *testing.T) {
		p := Photo{Path: testkit.JPEG(t, dir, "d.jpg", 40, 30)}
		if _, err := c.File(p, false); err != nil {
			t.Fatal(err)
		}
		testkit.JPEG(t, dir, "d.jpg", 80, 60) // другой размер — другой ключ
		if f, ok := c.Cached(p, false); ok {
			t.Errorf("копия прежнего файла выдана для нового: %s", f)
		}
	})
	t.Run("оригинала нет", func(t *testing.T) {
		if f, ok := c.Cached(Photo{Path: filepath.Join(dir, "нет.jpg")}, false); ok {
			t.Errorf("копия для несуществующего файла: %s", f)
		}
		if f, ok := c.Cached(Photo{}, true); ok {
			t.Errorf("копия для пустого пути: %s", f)
		}
	})
}

// TestCacheCachedColorKeys: по одной копии на каждый класс цвета ключа (без профиля, Adobe, P3/ICC) — все находятся
// без знания цвета.
func TestCacheCachedColorKeys(t *testing.T) {
	dir := testkit.TempDir(t)
	c, err := NewCache(filepath.Join(dir, "cache"))
	if err != nil {
		t.Fatal(err)
	}
	for _, col := range []imgfile.Color{imgfile.ColorNone, imgfile.ColorAdobe, imgfile.ColorP3, imgfile.ColorICC} {
		t.Run("цвет "+string(col), func(t *testing.T) {
			p := Photo{Path: testkit.JPEG(t, dir, "c_"+string(col)+".jpg", 40, 30), Color: col}
			want, err := c.File(p, false)
			if err != nil {
				t.Fatal(err)
			}
			got, ok := c.Cached(Photo{Path: p.Path}, false) // цвет неизвестен
			if !ok || got != want {
				t.Errorf("Cached = %q, %v; ожидалось %q", got, ok, want)
			}
		})
	}
}
