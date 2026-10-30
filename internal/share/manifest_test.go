package share

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/api/albums"
	"github.com/IliaSA101/sosninaalbums/internal/export"
	"github.com/IliaSA101/sosninaalbums/internal/render"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

func pagesDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(testkit.TempDir(t), "img")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

// writeJPEG пишет JPEG w×h и возвращает его байты.
func writeJPEG(t *testing.T, dir, name string, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = byte(i), byte(i>>8), 90, 255
	}
	var b bytes.Buffer
	if err := jpeg.Encode(&b, img, &jpeg.Options{Quality: 80}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), b.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func sumHex(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }

var specM = export.WebSpec{DPI: 100, PageW: 100, PageH: 60, BleedX: 3, BleedY: 3}

// Без обложки: страницы идут парами, белая — без файла и хэша, sha/байты/px — из файлов.
func TestShareManifest(t *testing.T) {
	dir := pagesDir(t)
	b1 := writeJPEG(t, dir, "001.jpg", 40, 30)
	b4 := writeJPEG(t, dir, "004.jpg", 50, 20)
	pages := []export.WebPage{{Name: "001.jpg"}, {Name: "002.jpg", Blank: true}, {Name: "003.jpg", Blank: true}, {Name: "004.jpg"}}
	m, err := BuildManifest(context.Background(), dir, pages, false, specM, "Свадьба И. и А.", "Sosnina Albums 0.32.0", albums.Limits{})
	if err != nil {
		t.Fatal(err)
	}
	if m.Schema != 1 || m.Title != "Свадьба И. и А." || m.Generator != "Sosnina Albums 0.32.0" || m.Book.Cover || m.Book.PageMM != (albums.Size{W: 97, H: 54}) {
		t.Fatalf("шапка: %+v", m)
	}
	want := []albums.Page{
		{N: 1, Role: "page", Type: "image/jpeg", SHA256: sumHex(b1), Bytes: int64(len(b1)), W: 40, H: 30},
		{N: 2, Role: "page", Blank: true},
		{N: 3, Role: "page", Blank: true},
		{N: 4, Role: "page", Type: "image/jpeg", SHA256: sumHex(b4), Bytes: int64(len(b4)), W: 50, H: 20},
	}
	for i, w := range want {
		if m.Pages[i] != w {
			t.Errorf("страница %d: %+v, ждали %+v", i+1, m.Pages[i], w)
		}
	}
	// JSON: белая страница — без хэша и размеров; ни пути, ни имени файла, ни пароля, ни срока.
	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	var back albums.Manifest
	if err := json.Unmarshal(raw, &back); err != nil || back.Pages[3] != want[3] {
		t.Fatalf("круг JSON: %v %+v", err, back)
	}
	for _, bad := range []string{dir, ".jpg", "password", "expires_at", "file"} {
		if strings.Contains(string(raw), bad) {
			t.Errorf("в манифесте %q: %s", bad, raw)
		}
	}
	if !strings.Contains(string(raw), `{"n":2,"role":"page","blank":true}`) {
		t.Errorf("белая страница: %s", raw)
	}
}

// С обложкой: первая — front, последняя — back, остальные — page; белая обложка допустима.
func TestShareManifestCover(t *testing.T) {
	dir := pagesDir(t)
	writeJPEG(t, dir, "000.jpg", 30, 30)
	writeJPEG(t, dir, "003.jpg", 30, 30)
	pages := []export.WebPage{{Name: "000.jpg"}, {Name: "001.jpg", Blank: true}, {Name: "002.jpg", Blank: true}, {Name: "003.jpg"}}
	m, err := BuildManifest(context.Background(), dir, pages, true, specM, "Т", "g", albums.Limits{})
	if err != nil {
		t.Fatal(err)
	}
	roles := []string{m.Pages[0].Role, m.Pages[1].Role, m.Pages[2].Role, m.Pages[3].Role}
	if !m.Book.Cover || strings.Join(roles, ",") != "front,page,page,back" {
		t.Errorf("роли %v, обложка %v", roles, m.Book.Cover)
	}
	// Белая лицевая сторона: роль front, файла нет.
	pages[0].Blank = true
	m, err = BuildManifest(context.Background(), dir, pages, true, specM, "Т", "g", albums.Limits{})
	if err != nil || m.Pages[0] != (albums.Page{N: 1, Role: "front", Blank: true}) {
		t.Errorf("белая обложка: %+v %v", m, err)
	}
}

// Настоящие страницы RunWeb (1600 px, q82): манифест сходится с файлами и тем же хэшем при повторе.
func TestShareManifestFromRunWeb(t *testing.T) {
	spec := specM
	tw, th := spec.Trimmed()
	spec.DPI = export.WebDPI(tw, th, 0)
	dir := pagesDir(t)
	sp := []render.Spread{{Bg: [2]color.RGBA{{250, 235, 205, 255}, {250, 235, 205, 255}}}, {Bg: [2]color.RGBA{{255, 255, 255, 255}, {255, 255, 255, 255}}}}
	pages, err := export.RunWeb(context.Background(), sp, nil, spec, dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	m, err := BuildManifest(context.Background(), dir, pages, false, spec, "Т", "g", albums.Limits{})
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Pages) != 4 || m.Pages[0].Blank || !m.Pages[2].Blank || !m.Pages[3].Blank {
		t.Fatalf("страницы: %+v", m.Pages)
	}
	if long := max(m.Pages[0].W, m.Pages[0].H); long < export.ShareLongSide || long > export.ShareLongSide+10 {
		t.Errorf("длинная сторона %d px", long)
	}
	raw, _ := os.ReadFile(filepath.Join(dir, pages[0].Name))
	if m.Pages[0].SHA256 != sumHex(raw) || m.Pages[0].Bytes != int64(len(raw)) {
		t.Errorf("sha/байты не из файла: %+v", m.Pages[0])
	}
}

// Битое, усечённое, пустое, огромное, чужое — ошибка вместо паники и до чтения всего файла.
func TestShareManifestBad(t *testing.T) {
	ctx := context.Background()
	good := writeJPEG(t, pagesDir(t), "x.jpg", 40, 30)
	two := func(name string) []export.WebPage {
		return []export.WebPage{{Name: name}, {Name: "blank.jpg", Blank: true}}
	}
	build := func(dir string, pages []export.WebPage, lim albums.Limits) error {
		_, err := BuildManifest(ctx, dir, pages, false, specM, "Т", "g", lim)
		return err
	}
	huge := make([]byte, 0, 1<<20)
	huge = append(huge, good...)
	huge = append(huge, make([]byte, 1<<20)...)
	// JPEG-заголовок 60000×60000: декодировать его нельзя, размер отвергается по заголовку.
	bigDims := append([]byte{}, good...)
	if i := bytes.Index(bigDims, []byte{0xFF, 0xC0}); i > 0 {
		copy(bigDims[i+5:], []byte{0xEA, 0x60, 0xEA, 0x60})
	} else {
		t.Fatal("в тестовом JPEG нет SOF0")
	}
	files := map[string][]byte{
		"empty.jpg":   {},
		"trunc.jpg":   good[:len(good)/2],
		"head.jpg":    good[:6],
		"garbage.jpg": bytes.Repeat([]byte{0xAB}, 4096),
		"png.jpg":     {0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A, 0, 0, 0, 0},
		"huge.jpg":    huge,
		"dims.jpg":    bigDims,
	}
	dir := pagesDir(t)
	for name, b := range files {
		if err := os.WriteFile(filepath.Join(dir, name), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	lim := albums.Limits{PageBytes: 1 << 19}
	for name := range files {
		if err := build(dir, two(name), lim); err == nil {
			t.Errorf("%s принят", name)
		}
	}
	// Пустое имя, путь вместо имени, нет файла, папка вместо файла.
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"", ".", "..", "../x.jpg", "sub/x.jpg", "/etc/passwd", `a\b.jpg`, "нет.jpg", "sub"} {
		if err := build(dir, two(name), lim); err == nil {
			t.Errorf("имя %q принято", name)
		}
	}
	// Ссылка вместо файла.
	if err := os.WriteFile(filepath.Join(dir, "real.jpg"), good, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(dir, "real.jpg"), filepath.Join(dir, "link.jpg")); err == nil {
		if err := build(dir, two("link.jpg"), lim); err == nil {
			t.Error("ссылка принята")
		}
	}
	if err := build(dir, two("real.jpg"), lim); err != nil {
		t.Errorf("годный файл: %v", err)
	}
	// Число страниц, пределы сервиса, остановка.
	for name, pages := range map[string][]export.WebPage{
		"нет страниц": nil, "нечётное": {{Name: "real.jpg"}, {Name: "real.jpg"}, {Name: "real.jpg"}},
	} {
		if build(dir, pages, lim) == nil {
			t.Errorf("%s принято", name)
		}
	}
	if build(dir, []export.WebPage{{Name: "real.jpg"}, {Name: "real.jpg"}, {Name: "real.jpg"}, {Name: "real.jpg"}}, albums.Limits{Pages: 2}) == nil {
		t.Error("страниц больше предела принято")
	}
	if build(dir, []export.WebPage{{Name: "real.jpg"}, {Name: "real.jpg"}}, albums.Limits{AlbumBytes: int64(len(good)) + 1}) == nil {
		t.Error("альбом больше предела принят")
	}
	if build(dir, two("real.jpg"), albums.Limits{PageBytes: int64(len(good)) - 1}) == nil {
		t.Error("страница больше предела принята")
	}
	if _, err := BuildManifest(ctx, dir, two("real.jpg"), false, export.WebSpec{PageW: 100, PageH: 60, BleedX: 100}, "Т", "g", lim); err == nil {
		t.Error("страница, съеденная обрезом, принята")
	}
	stopped, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := BuildManifest(stopped, dir, two("real.jpg"), false, specM, "Т", "g", lim); err == nil {
		t.Error("остановка проигнорирована")
	}
}

// Название и обозначение программы в манифесте очищены и обрезаны; генератор не подменяется.
func TestShareManifestTitle(t *testing.T) {
	dir := pagesDir(t)
	writeJPEG(t, dir, "a.jpg", 10, 10)
	pages := []export.WebPage{{Name: "a.jpg"}, {Name: "b.jpg", Blank: true}}
	ctx := context.Background()
	long := strings.Repeat("я", 500)
	m, err := BuildManifest(ctx, dir, pages, false, specM, "Лето\n\x00 2026 "+long, "G\n\u200b"+long, albums.Limits{TitleLen: 50})
	if err != nil {
		t.Fatal(err)
	}
	if n := len([]rune(m.Title)); n != 50 || strings.ContainsAny(m.Title, "\n\x00") {
		t.Errorf("название %q", m.Title)
	}
	if n := len([]rune(m.Generator)); n != 100 || strings.ContainsAny(m.Generator, "\n\u200b") || !strings.HasPrefix(m.Generator, "G") {
		t.Errorf("генератор %q (%d знаков)", m.Generator, n)
	}
	// Предел в байтах: 200 знаков по 2 байта — 400, влезают; по 4 байта (эмодзи) — не больше 800 байт.
	m, err = BuildManifest(ctx, dir, pages, false, specM, strings.Repeat("😀", 300), "g", albums.Limits{})
	if err != nil || len(m.Title) > MaxTitleBytes || len([]rune(m.Title)) != MaxTitleBytes/4 {
		t.Errorf("название из эмодзи: %d байт, %d знаков, %v", len(m.Title), len([]rune(m.Title)), err)
	}
	m, err = BuildManifest(ctx, dir, pages, false, specM, "", "g", albums.Limits{})
	if err != nil || m.Title != "Альбом" {
		t.Errorf("пустое название: %q %v", m.Title, err)
	}
	// Пустой генератор — ошибка вызывающего, а не подстановка «Альбом».
	for _, g := range []string{"", " \n ", "\u200b\x00"} {
		if m, err := BuildManifest(ctx, dir, pages, false, specM, "Т", g, albums.Limits{}); err == nil {
			t.Errorf("пустой генератор %q принят: %+v", g, m)
		}
	}
}

// Размер страницы в мм (разбор С7): 10…2000 после обреза, иначе отказ до чтения файлов.
func TestShareManifestPageMM(t *testing.T) {
	dir := pagesDir(t)
	writeJPEG(t, dir, "a.jpg", 10, 10)
	pages := []export.WebPage{{Name: "a.jpg"}, {Name: "b.jpg", Blank: true}}
	for name, w := range map[string]export.WebSpec{
		"в пределах":    {PageW: 10, PageH: 10},
		"максимум":      {PageW: 2000, PageH: 2000},
		"узкая":         {PageW: 9.9, PageH: 100},
		"низкая":        {PageW: 100, PageH: 9.9},
		"огромная":      {PageW: 2000.5, PageH: 100},
		"нулевая":       {},
		"NaN":           {PageW: nan(), PageH: 100},
		"бесконечность": {PageW: inf(), PageH: 100},
	} {
		_, err := BuildManifest(context.Background(), dir, pages, false, w, "Т", "g", albums.Limits{})
		if good := name == "в пределах" || name == "максимум"; good != (err == nil) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func nan() float64 { z := 0.0; return z / z }
func inf() float64 { z := 0.0; return 1 / z }

// Отказ по размеру называет сумму и предел; предел меньше мегабайта — в килобайтах.
func TestShareManifestSizeText(t *testing.T) {
	dir := pagesDir(t)
	b := writeJPEG(t, dir, "a.jpg", 200, 200)
	pages := []export.WebPage{{Name: "a.jpg"}, {Name: "a.jpg"}}
	_, err := BuildManifest(context.Background(), dir, pages, false, specM, "Т", "g", albums.Limits{PageBytes: 1024})
	if err == nil || !strings.Contains(err.Error(), "КБ") || !strings.Contains(err.Error(), "1 КБ") {
		t.Errorf("страница: %v", err)
	}
	_, err = BuildManifest(context.Background(), dir, pages, false, specM, "Т", "g", albums.Limits{AlbumBytes: int64(len(b)) + 1})
	want := sizeText(2*int64(len(b)), 1) + ", сервис принимает не больше " + sizeText(int64(len(b))+1, 1)
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Errorf("альбом: %v, ждали «%s»", err, want)
	}
	if sizeText(1, 1<<20) != "1 МБ" || sizeText(1<<20, 1<<20) != "1 МБ" || sizeText(1<<20+1, 1<<20) != "2 МБ" || sizeText(1500, 1000) != "2 КБ" {
		t.Error("округление размера")
	}
}
