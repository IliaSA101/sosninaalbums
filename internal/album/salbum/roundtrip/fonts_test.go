package roundtrip_test

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Свои шрифты в файле (082, v15): раздел fonts (id → название), id в стилях, блок без шрифта.

var writeGolden15 = flag.Bool("golden15fonts", false, "перезаписать testdata/v15-fonts.salbum.json из fontsGolden")

const (
	golden15Fonts = "v15-fonts.salbum.json"
	bodoniID      = "my:b0d0b0d0b0d0b0d0" // своя гарнитура, которой нет ни на одном компьютере тестов
	fontFile      = "Oswald-Regular.ttf"  // байты «своей» Lato: узкий, ширины не как у Montserrat
)

// fontsAlbum — альбом со своими шрифтами под папкой base: подпись фото и текст «по тексту» —
// Lato, текст с закреплённой шириной (жирный курсив) и общий стиль подписей обложки — Bodoni
// Moda, текст встроенным шрифтом. Таблица fonts — с названиями, как её держит SyncFonts.
func fontsAlbum(base string) *model.Project {
	lato := edtest.FontID("Lato")
	st := func(font string, size float64) model.TextStyle {
		return model.TextStyle{Font: font, Size: size, Color: "#222222", Align: "center"}
	}
	p := model.New()
	path := filepath.Join(base, "Фото", "a.jpg")
	a := library.IDFor(path)
	p.Photos[a] = model.PhotoRef{Path: path, W: 3000, H: 2000, Size: 1000, Caption: &model.Caption{Text: "Анна", Style: st(lato, 10)}}
	bold := st(bodoniID, 24)
	bold.Bold, bold.Italic = true, true
	p.Spreads = []*model.Spread{{ID: "5e1f0a2b3c41", Photos: []string{a}, Texts: []*model.TextBlock{
		{ID: "0a0b0c0d0e01", X: 40, Y: 50, W: 100, Auto: true, Text: "Анна и Павел", Style: st(lato, 36)},
		{ID: "0a0b0c0d0e02", X: 300, Y: 200, W: 80, Text: "12 июня", Style: bold},
		{ID: "0a0b0c0d0e03", X: 300, Y: 100, W: 10, Auto: true, Text: "Свадьба", Style: st("montserrat", 20)},
	}}}
	pr, _ := model.CoverPresetOf(model.CoverDefault)
	p.Cover = &model.Cover{Sheet: &model.Spread{ID: model.CoverID, Caps: &model.SheetCaps{Style: st(bodoniID, 9), Pad: 1}},
		Spec: pr.Spec, Layout: model.CoverFront}
	p.Fonts = map[string]model.FontRef{lato: {Name: "Lato"}, bodoniID: {Name: "Bodoni Moda"}}
	p.FitTexts()
	p.SyncFonts()
	return p
}

// textOf — блок текста разворота по ID.
func textOf(t *testing.T, p *model.Project, id string) model.TextBlock {
	t.Helper()
	for _, s := range p.Sheets() {
		for _, tb := range s.Texts {
			if tb.ID == id {
				return *tb
			}
		}
	}
	t.Fatalf("нет текста %s", id)
	return model.TextBlock{}
}

// Круг Save → Load через Editor: названия и id в стилях на месте, у блока без шрифта — ширина,
// положение, жирный и курсив; шрифт есть — ширина «по тексту» его; вторая запись — те же байты.
// Альбом без своих шрифтов раздела fonts не пишет.
func TestFontsRoundTrip(t *testing.T) {
	lato := edtest.UseFont(t, "Lato", fontFile)
	dir := testkit.TempDir(t)
	src := fontsAlbum(dir)
	path := filepath.Join(dir, "a.salbum")
	e := edtest.Open(t, src, path)
	e.RefitFonts() // ширина Lato — по своему файлу
	if err := e.Save(path); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	q, err := salbum.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]model.FontRef{lato: {Name: "Lato"}, bodoniID: {Name: "Bodoni Moda"}}
	if !maps.Equal(q.Fonts, want) {
		t.Fatalf("таблица после чтения %v, ждали %v", q.Fonts, want)
	}
	if got := q.UsedFonts(); !slices.Equal(got, slices.Sorted(maps.Keys(want))) {
		t.Fatalf("id в стилях: %v", got)
	}
	if m := textOf(t, q, "0a0b0c0d0e02"); m.W != 80 || m.X != 300 || m.Y != 200 || !m.Style.Bold || !m.Style.Italic {
		t.Fatalf("блок без шрифта: %+v", m)
	}
	if c := q.Cover.Sheet.Caps.Style; c.Font != bodoniID {
		t.Fatalf("стиль подписей обложки: %+v", c)
	}
	if c := q.Photos[library.IDFor(filepath.Join(dir, "Фото", "a.jpg"))].Caption; c == nil || c.Style.Font != lato {
		t.Fatalf("подпись фото: %+v", c)
	}
	if got, w := textOf(t, q, "0a0b0c0d0e01"), textOf(t, edtest.P(e), "0a0b0c0d0e01"); got != w || got.W == 100 {
		t.Fatalf("Lato после чтения %+v, в редакторе %+v", got, w)
	}
	if err := edtest.Open(t, q, path).Save(path); err != nil {
		t.Fatal(err)
	}
	if again, _ := os.ReadFile(path); !bytes.Equal(again, first) {
		t.Fatalf("вторая запись другая:\n%s\n---\n%s", first, again)
	}

	// Без своих шрифтов — ни ключа, ни пустого объекта.
	plain := model.New()
	plain.Spreads = []*model.Spread{{ID: "s1", Texts: []*model.TextBlock{{ID: "0a0b0c0d0e04", W: 50, Text: "x", Style: model.PlainStyle()}}}}
	data, err := salbum.Encode(plain, dir)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte(`"fonts"`)) {
		t.Fatalf("раздел fonts без своих шрифтов:\n%s", data)
	}
}

// Файл v14 (0.20) открывается без своих шрифтов: только номер, первая запись — .v14.bak; раздела
// fonts нет ни в памяти, ни в записи.
func TestV14OpensWithoutFonts(t *testing.T) {
	oldVersionOpens(t, 14)
	src, err := os.ReadFile(filepath.Join("..", "testdata", "v6.salbum.json"))
	if err != nil {
		t.Fatal(err)
	}
	dir := testkit.TempDir(t)
	p := edtest.MustDecode(t, bytes.Replace(src, []byte(`"version": 6`), []byte(`"version": 14`), 1), dir)
	if p.Fonts != nil || len(p.UsedFonts()) != 0 {
		t.Fatalf("свои шрифты у файла v14: %v %v", p.Fonts, p.UsedFonts())
	}
	data, err := salbum.Encode(p, dir)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte(`"fonts"`)) {
		t.Fatal("раздел fonts в записи файла v14")
	}
}

// Зажим раздела fonts при чтении: ключ не id своей — выброшен, название — одна строка ≤ 60,
// неиспользуемые убраны (лишние записи сверх MaxFonts не вытесняют стоящие), стоящая без записи —
// запись с пустым названием, зарегистрированная — название из библиотеки; null — как нет раздела.
func TestLoadClampsFonts(t *testing.T) {
	lato := edtest.UseFont(t, "Lato", fontFile)
	style := func(font string, extra string) string {
		return fmt.Sprintf(`{"font":%q,"size":20,"color":"#000000","align":"left"%s}`, font, extra)
	}
	var fonts strings.Builder
	for i := range model.MaxFonts + 10 {
		fmt.Fprintf(&fonts, `,"my:%016x":{"name":"лишний %d"}`, i, i)
	}
	file := fmt.Sprintf(`{"version":15,"spreads":[{"id":"s","texts":[
{"id":"0a0b0c0d0e01","x":10,"y":10,"w":50,"auto":true,"text":"x","style":%s},
{"id":"0a0b0c0d0e02","x":10,"y":60,"w":50,"text":"x","style":%s},
{"id":"0a0b0c0d0e03","x":10,"y":90,"w":50,"text":"x","style":%s},
{"id":"0a0b0c0d0e04","x":10,"y":120,"w":50,"text":"x","style":%s}]}],
"fonts":{%q:{"name":"Чужое имя"},%q:{"name":" Bodoni\nModa \u202e%s"},"montserrat":{"name":"M"},"my:XYZ":{"name":"x"},"../a.ttf":{"name":"p"}%s}}`,
		style(lato, ""), style(bodoniID, `,"bold":true,"italic":true`), style("my:BAD", ""), style("my:0000000000000003", ""),
		lato, bodoniID, strings.Repeat("я", 80), fonts.String())
	p := edtest.MustDecode(t, []byte(file), testkit.TempDir(t))
	want := map[string]model.FontRef{
		lato:                  {Name: "Lato"},
		bodoniID:              {Name: "Bodoni Moda " + strings.Repeat("я", model.MaxFontNameRunes-12)},
		"my:0000000000000003": {Name: "лишний 3"}, // стоит; остальные записи этой серии — нет
	}
	if !maps.Equal(p.Fonts, want) {
		t.Fatalf("таблица:\n%v\nждали\n%v", p.Fonts, want)
	}
	if f := textOf(t, p, "0a0b0c0d0e03").Style.Font; f != "montserrat" {
		t.Fatalf("неверный id: %q", f)
	}
	if b := textOf(t, p, "0a0b0c0d0e02").Style; !b.Bold || !b.Italic {
		t.Fatalf("жирный курсив без шрифта потерян: %+v", b)
	}

	// Стоит, а записи нет (из шаблона или зажатой таблицы) — пустое название.
	p = edtest.MustDecode(t, []byte(`{"version":15,"spreads":[{"id":"s","texts":[{"id":"0a0b0c0d0e01","w":50,"text":"x","style":`+
		style("my:00000000000000dd", "")+`}]}],"fonts":null}`), testkit.TempDir(t))
	if !maps.Equal(p.Fonts, map[string]model.FontRef{"my:00000000000000dd": {}}) {
		t.Fatalf("без записи: %v", p.Fonts)
	}
	// Раздел есть, своих в стилях нет — пусто.
	p = edtest.MustDecode(t, []byte(`{"version":15,"spreads":[],"fonts":{`+strings.TrimPrefix(fonts.String(), ",")+`}}`), testkit.TempDir(t))
	if p.Fonts != nil {
		t.Fatalf("без стоящих: %v", p.Fonts)
	}
}

// fuzzFonts — после Load таблица своих шрифтов зажата и ровно по стилям (FuzzLoad).
func fuzzFonts(t *testing.T, p *model.Project) {
	t.Helper()
	if !reflect.DeepEqual(model.ClampFonts(p.Fonts, p.UsedFonts()), p.Fonts) || len(p.Fonts) > model.MaxFonts {
		t.Fatalf("таблица шрифтов не зажата: %v", p.Fonts)
	}
	used := p.UsedFonts()
	if len(used) > model.MaxFonts {
		used = used[:model.MaxFonts]
	}
	if !slices.Equal(slices.Sorted(maps.Keys(p.Fonts)), used) {
		t.Fatalf("таблица шрифтов %v, в стилях %v", slices.Sorted(maps.Keys(p.Fonts)), used)
	}
}

// В файле альбома нет путей и ключей файлов шрифтов (082-8, §11): ни при обычной записи, ни при
// записи собранной папки; у записи раздела fonts — только название.
func TestFontsNoPaths(t *testing.T) {
	edtest.UseFont(t, "Lato", fontFile)
	key := edtest.FontKey(t, fontFile)
	dir := testkit.TempDir(t)
	p := fontsAlbum(dir)
	path := filepath.Join(dir, "a.salbum")
	e := edtest.Open(t, p, path)
	e.RefitFonts()
	if err := e.Save(path); err != nil {
		t.Fatal(err)
	}
	rel := filepath.Join(dir, "Собранный", "a.salbum")
	if err := salbum.WriteRel(edtest.P(e), rel, filepath.Dir(rel), model.Side{}); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{path, rel} {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, bad := range []string{key, ".ttf", ".otf", "Шрифты", "Oswald"} {
			if bytes.Contains(data, []byte(bad)) {
				t.Fatalf("%s: в файле %q", filepath.Base(f), bad)
			}
		}
		var raw struct {
			Fonts map[string]map[string]any `json:"fonts"`
		}
		if err := json.Unmarshal(data, &raw); err != nil {
			t.Fatal(err)
		}
		if len(raw.Fonts) != 2 {
			t.Fatalf("%s: записей шрифтов %d", filepath.Base(f), len(raw.Fonts))
		}
		for id, r := range raw.Fonts {
			if len(r) != 1 || r["name"] == nil {
				t.Fatalf("%s: запись %s — %v", filepath.Base(f), id, r)
			}
		}
	}
}

// TestFontsGoldenV15 — голден v15 со своими шрифтами (082): Decode → Encode = исходник байт в байт
// (он же сид FuzzLoad); на компьютере без шрифтов блоки не сдвинуты, названия из файла, жирный
// курсив цел; шрифт нашёлся — ширина «по тексту» его, название — из библиотеки. Пути — от
// C:\album, поэтому только Windows.
func TestFontsGoldenV15(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("голден с путями C:\\album")
	}
	path := filepath.Join("..", "testdata", golden15Fonts)
	if *writeGolden15 {
		data, err := salbum.Encode(fontsAlbum(goldenBase), goldenBase)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	data = bytes.TrimSuffix(bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n")), []byte("\n"))
	p, err := salbum.Decode(data, goldenBase)
	if err != nil {
		t.Fatal(err)
	}
	if p.Version != model.FormatVersion || edtest.From(p) != 15 {
		t.Fatalf("версия %d, from %d", p.Version, edtest.From(p))
	}
	lato := edtest.FontID("Lato")
	if !maps.Equal(p.Fonts, map[string]model.FontRef{lato: {Name: "Lato"}, bodoniID: {Name: "Bodoni Moda"}}) {
		t.Fatalf("таблица: %v", p.Fonts)
	}
	if got := textOf(t, p, "0a0b0c0d0e01"); got.W != 100 || got.X != 40 || got.Style.Font != lato {
		t.Fatalf("Lato без шрифта сдвинут: %+v", got)
	}
	if got := textOf(t, p, "0a0b0c0d0e02"); !got.Style.Bold || !got.Style.Italic || got.W != 80 {
		t.Fatalf("Bodoni: %+v", got)
	}
	out, err := salbum.Encode(p, goldenBase)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out, data) {
		t.Fatalf("Decode → Encode отличается от голдена:\n%s", out)
	}
	if _, err := json.Marshal(edtest.Open(t, p, "").View()); err != nil {
		t.Fatalf("View не сериализуется: %v", err)
	}

	// Шрифт нашёлся (название в библиотеке в другом регистре — тот же id): ширина — по его файлу.
	edtest.UseFont(t, "LATO", fontFile)
	q, err := salbum.Decode(data, goldenBase)
	if err != nil {
		t.Fatal(err)
	}
	if got := textOf(t, q, "0a0b0c0d0e01"); got.W == 100 || got.Style.Font != lato {
		t.Fatalf("Lato с шрифтом: %+v", got)
	}
	if q.Fonts[lato].Name != "LATO" || q.Fonts[bodoniID].Name != "Bodoni Moda" {
		t.Fatalf("названия: %v", q.Fonts)
	}
	if got := textOf(t, q, "0a0b0c0d0e02"); got.W != 80 || got.X != 300 {
		t.Fatalf("Bodoni сдвинут: %+v", got)
	}
}
