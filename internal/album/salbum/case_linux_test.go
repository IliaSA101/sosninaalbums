//go:build linux

package salbum

import (
	"crypto/sha1" //nolint:gosec // ID старого вида в тесте, не криптография
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// legacyID — ID фото, как его считала Linux-сборка 0.22: sha1 пути в нижнем регистре.
func legacyID(path string) string {
	sum := sha1.Sum([]byte(strings.ToLower(filepath.Clean(path)))) //nolint:gosec // как IDFor 0.22
	return hex.EncodeToString(sum[:8])
}

// Альбом Linux-сборки 0.22 (ID от нижнего регистра) открывается: Rekey переводит ID к IDFor пути,
// развороты и места ссылаются на новые ID, обе папки (`Весна` и `ВЕСНА`) остаются в альбоме, а
// лента (скан папок) отмечает оба фото «в альбоме» — ID ленты есть в Photos. Сохранение и повторное
// открытие ID не меняют.
func TestLoadRekeyLegacyIDsLinux(t *testing.T) {
	root := testkit.TempDir(t)
	lo, up := filepath.Join(root, "Весна"), filepath.Join(root, "ВЕСНА")
	for _, d := range []string{lo, up} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "весна")); err == nil {
		t.Skip("ФС без учёта регистра")
	}
	pa := testkit.JPEG(t, lo, "IMG_1.JPG", 30, 20)
	pb := testkit.JPEG(t, up, "IMG_2.JPG", 30, 20)
	oa, ob := legacyID(pa), legacyID(pb)
	if oa == library.IDFor(pa) || ob == library.IDFor(pb) {
		t.Fatal("ID старого вида совпал с новым: тест ничего не проверяет")
	}
	f := FileProject{Version: 2, Settings: model.DefaultSettings(),
		Folders: []FileFolder{{Path: lo}, {Path: up}},
		Spreads: []*model.Spread{{ID: "s1", Photos: []string{oa, ob}}},
		Photos: map[string]FilePhoto{
			oa: {PhotoRef: model.PhotoRef{Path: pa, W: 30, H: 20}},
			ob: {PhotoRef: model.PhotoRef{Path: pb, W: 30, H: 20}},
		}}
	data, err := json.Marshal(f)
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(root, "альбом.salbum")
	if err := os.WriteFile(file, data, 0o644); err != nil {
		t.Fatal(err)
	}
	check := func(p *model.Project) {
		t.Helper()
		ia, ib := library.IDFor(pa), library.IDFor(pb)
		if len(p.Photos) != 2 || p.Photos[ia].Path != pa || p.Photos[ib].Path != pb {
			t.Fatalf("фото: %+v", p.Photos)
		}
		if len(p.Spreads) != 1 || !slices.Equal(p.Spreads[0].Photos, []string{ia, ib}) {
			t.Fatalf("разворот: %+v", p.Spreads)
		}
		if len(p.Folders) != 2 {
			t.Fatalf("папки свернулись: %q", p.Folders)
		}
		feed, _ := library.Scan(p.Folders)
		if len(feed) != 2 {
			t.Fatalf("лента: %d фото, ждали 2", len(feed))
		}
		for _, ph := range feed {
			if _, in := p.Photos[ph.ID]; !in {
				t.Errorf("фото ленты %s не отмечено «в альбоме»", ph.Path)
			}
		}
	}
	p, err := Load(file)
	if err != nil {
		t.Fatal(err)
	}
	check(p)
	if err := Save(p, file); err != nil {
		t.Fatal(err)
	}
	if p, err = Load(file); err != nil {
		t.Fatal(err)
	}
	check(p)
}

// Load не ходит по ссылке, звено которой нелокально (101-22): настоящая ссылка ведёт на файл
// рядом (os.Stat прошёл бы), но подменённое чтение цели говорит «UNC» — fileSize и isDir
// отказывают; без подмены та же ссылка читается как файл.
func TestLoadLinkNonLocalNoStat(t *testing.T) {
	dir := testkit.TempDir(t)
	a := testkit.JPEG(t, dir, "a.jpg", 20, 10)
	link, dlink := filepath.Join(dir, "ссылка.jpg"), filepath.Join(dir, "папка-ссылка")
	if err := os.Symlink(a, link); err != nil {
		t.Skipf("ссылку не создать: %v", err)
	}
	if err := os.Symlink(dir, dlink); err != nil {
		t.Skipf("ссылку не создать: %v", err)
	}
	if _, ok := fileSize(link); !ok {
		t.Fatal("локальная ссылка на файл не читается")
	}
	if !isDir(dlink) {
		t.Fatal("локальная ссылка на папку — не папка")
	}
	if _, ok := fileSize(dlink); ok {
		t.Fatal("ссылка на папку прошла как файл")
	}
	t.Cleanup(library.SetReadLink(func(p string) (string, error) {
		if p == link || p == dlink {
			return "//nosuchhost/x", nil
		}
		return "", os.ErrInvalid
	}))
	if _, ok := fileSize(link); ok {
		t.Fatal("fileSize прошёл по нелокальной ссылке")
	}
	if isDir(dlink) {
		t.Fatal("isDir прошёл по нелокальной ссылке")
	}
	if _, ok := fileSize(a); !ok {
		t.Fatal("обычный файл не читается")
	}
}
