package svgcut

import (
	"io"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// wrap — документ с одним путём и дополнительным содержимым корня.
func wrap(inner string) []byte {
	return []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100">` + inner + `</svg>`)
}

const okPath = `<path transform="translate(0,0)" fill="rgb(1,2,3)" d="M 10 10 L 20 10 L 20 20 z"/>`

// Строчная z и transform="translate(0,0)" — так устроены все 616 путей школы.
func TestSvgCutLowerZ(t *testing.T) {
	for name, p := range map[string]string{
		"строчная z":        `<path transform="translate(0,0)" fill="rgb(1,2,3)" d="M 10 10 L 20 10 L 20 20 z"/>`,
		"заглавная Z":       `<path transform="translate(0,0)" fill="rgb(1,2,3)" d="M 10 10 L 20 10 L 20 20 Z"/>`,
		"без transform":     `<path fill="rgb(1,2,3)" d="M 10 10 L 20 10 L 20 20 z"/>`,
		"translate(0 0)":    `<path transform="translate(0 0)" fill="rgb(1,2,3)" d="M 10 10 L 20 10 L 20 20 z"/>`,
		"translate(0, 0.0)": `<path transform="translate(0, 0.0)" fill="rgb(1,2,3)" d="M 10 10 L 20 10 L 20 20 z"/>`,
		"два подпути":       `<path fill="rgb(1,2,3)" d="M 10 10 L 20 10 L 20 20 z M 30 30 L 40 30 L 40 40 z"/>`,
		"кривая и запятые":  `<path fill="rgb(1,2,3)" d="M10,10C20,20 30,20 40,10z"/>`,
		"неявный L":         `<path fill="rgb(1,2,3)" d="M 10 10 20 10 20 20 z"/>`,
		"порядок числа":     `<path fill="rgb(1,2,3)" d="M 1e1 1.0E+1 L 2e1 10 L .5e2 2e1 z"/>`,
	} {
		doc, err := Parse(wrap(p))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if len(doc.Paths) != 1 || doc.Paths[0].Box.X0 != 10 {
			t.Fatalf("%s: %+v", name, doc.Paths)
		}
	}
	// Файл школы целиком: все пути проходят (если исходник на этой машине есть).
	if p := os.Getenv("SA_SVGCUT_SAMPLE"); p != "" {
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := Parse(data); err != nil {
			t.Fatalf("образец: %v", err)
		}
	}
}

func TestSvgCutRejects(t *testing.T) {
	long := strings.Repeat(`<g>`, maxDepth+1) + strings.Repeat(`</g>`, maxDepth+1)
	many := strings.Repeat(`<path fill="rgb(0,0,0)" d="M 0 0 L 1 1 z"/>`, 3)
	for name, tc := range map[string]struct{ in, want string }{
		"пусто":                {"", "нет элемента svg"},
		"только пробелы":       {"  \n ", "нет элемента svg"},
		"усечённый":            {`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><path fill="rgb(1,2,3)" d="M 0 0 L 5`, "разбор XML"},
		"не XML":               {"\x00\x01\x02", "разбор XML"},
		"не svg":               {`<html><body/></html>`, "не допускается"},
		"DOCTYPE":              {`<!DOCTYPE svg [<!ENTITY a "b">]><svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"/>`, "DOCTYPE"},
		"script":               {string(wrap(`<script>alert(1)</script>` + okPath)), "<script>"},
		"image":                {string(wrap(`<image href="http://x/y.png"/>` + okPath)), "<image>"},
		"use":                  {string(wrap(`<use href="#a"/>` + okPath)), "<use>"},
		"foreignObject":        {string(wrap(`<foreignObject/>` + okPath)), "<foreignObject>"},
		"style элемент":        {string(wrap(`<style>path{fill:red}</style>` + okPath)), "<style>"},
		"текст":                {string(wrap(`hello` + okPath)), "текст"},
		"инструкция":           {`<?xml-stylesheet href="a.css"?><svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"/>`, "инструкция"},
		"onload":               {`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10" onload="x()">` + okPath + `</svg>`, `"onload"`},
		"xlink":                {`<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" viewBox="0 0 10 10">` + okPath + `</svg>`, "xlink"},
		"чужое пространство":   {`<svg xmlns="http://example.org/x" viewBox="0 0 10 10">` + okPath + `</svg>`, "чужого пространства"},
		"чужой атрибут пути":   {string(wrap(`<path fill="rgb(1,2,3)" stroke="red" d="M 0 0 L 1 1 z"/>`)), `"stroke"`},
		"относительная m":      {string(wrap(`<path fill="rgb(1,2,3)" d="m 10 10 l 5 5 z"/>`)), "не поддерживается"},
		"дуга":                 {string(wrap(`<path fill="rgb(1,2,3)" d="M 0 0 A 5 5 0 0 1 9 9 z"/>`)), "не поддерживается"},
		"H V":                  {string(wrap(`<path fill="rgb(1,2,3)" d="M 0 0 H 5 V 9 z"/>`)), "не поддерживается"},
		"L после z":            {string(wrap(`<path fill="rgb(1,2,3)" d="M 0 0 L 5 5 z L 9 9"/>`)), "после z нужна M"},
		"без M":                {string(wrap(`<path fill="rgb(1,2,3)" d="L 0 0 L 5 5"/>`)), "вне подпути"},
		"число без команды":    {string(wrap(`<path fill="rgb(1,2,3)" d="5 5"/>`)), "число без команды"},
		"мало чисел":           {string(wrap(`<path fill="rgb(1,2,3)" d="M 0 0 L 5"/>`)), "ожидалось число"},
		"пустой d":             {string(wrap(`<path fill="rgb(1,2,3)" d=""/>`)), "пустой путь"},
		"NaN":                  {string(wrap(`<path fill="rgb(1,2,3)" d="M 0 NaN L 5 5"/>`)), "ожидалось число"},
		"огромное число":       {string(wrap(`<path fill="rgb(1,2,3)" d="M 0 0 L 1e999 5"/>`)), "вне предела"},
		"число за пределом":    {string(wrap(`<path fill="rgb(1,2,3)" d="M 0 0 L 99999999 5"/>`)), "вне предела"},
		"порядок без цифр":     {string(wrap(`<path fill="rgb(1,2,3)" d="M 0 0 L 1e 5"/>`)), "порядок числа"},
		"transform rotate":     {string(wrap(`<path transform="rotate(5)" fill="rgb(1,2,3)" d="M 0 0 L 1 1 z"/>`)), "transform"},
		"translate(1,0)":       {string(wrap(`<path transform="translate(1,0)" fill="rgb(1,2,3)" d="M 0 0 L 1 1 z"/>`)), "transform"},
		"transform у g":        {string(wrap(`<g transform="scale(2)">` + okPath + `</g>`)), "transform"},
		"fill red":             {string(wrap(`<path fill="red" d="M 0 0 L 1 1 z"/>`)), "fill"},
		"fill без":             {string(wrap(`<path d="M 0 0 L 1 1 z"/>`)), "fill"},
		"fill > 255":           {string(wrap(`<path fill="rgb(1,2,300)" d="M 0 0 L 1 1 z"/>`)), "канал больше 255"},
		"градиент нет":         {string(wrap(`<path fill="url(#nope)" d="M 0 0 L 1 1 z"/>`)), "не определён"},
		"градиент вне defs":    {string(wrap(`<linearGradient id="a"/>` + okPath)), "только в <defs>"},
		"градиент без id":      {string(wrap(`<defs><linearGradient/></defs>` + okPath)), "без id"},
		"градиент id плохой":   {string(wrap(`<defs><linearGradient id="a&quot;b"/></defs>` + okPath)), "id градиента"},
		"градиент повтор":      {string(wrap(`<defs><linearGradient id="a"/><linearGradient id="a"/></defs>` + okPath)), "повторяется"},
		"градиент значение":    {string(wrap(`<defs><linearGradient id="a" x1="&lt;script"/></defs>` + okPath)), "не допускается"},
		"stop вне градиента":   {string(wrap(`<defs><stop offset="0"/></defs>` + okPath)), "вне градиента"},
		"viewBox нет":          {`<svg xmlns="http://www.w3.org/2000/svg">` + okPath + `</svg>`, "viewBox"},
		"viewBox три числа":    {`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10">` + okPath + `</svg>`, "четыре числа"},
		"viewBox со смещением": {`<svg xmlns="http://www.w3.org/2000/svg" viewBox="5 0 10 10">` + okPath + `</svg>`, "начало"},
		"viewBox огромный":     {`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1000000 10">` + okPath + `</svg>`, "вне пределов"},
		"viewBox нулевой":      {`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 0 0">` + okPath + `</svg>`, "вне пределов"},
		"viewBox NaN":          {`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 NaN 10">` + okPath + `</svg>`, "плохое число"},
		"нет путей":            {string(wrap(`<g/>`)), "нет путей"},
		"глубина":              {string(wrap(long + okPath)), "вложенность"},
		"path в metadata":      {string(wrap(`<metadata>` + okPath + `</metadata>`)), "на недопустимом месте"},
		"два svg":              {string(wrap(`<svg viewBox="0 0 1 1"/>` + okPath)), "на недопустимом месте"},
		"g с мусором":          {string(wrap(`<g id="a">` + okPath + `</g>`)), `"id"`},
		"лишнее после":         {string(wrap(many)) + "<x/>", "<x>"},
	} {
		_, err := Parse([]byte(tc.in))
		if err == nil {
			t.Errorf("%s: принято", name)
			continue
		}
		if !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: ошибка %q, ждали %q", name, err, tc.want)
		}
	}
}

func TestSvgCutLimits(t *testing.T) {
	// Путей больше предела — отказ, не рост памяти без границы.
	var b strings.Builder
	b.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10">`)
	for i := 0; i <= maxPaths; i++ {
		b.WriteString(`<path fill="rgb(0,0,0)" d="M 0 0 L 1 1 z"/>`)
	}
	b.WriteString(`</svg>`)
	if _, err := Parse([]byte(b.String())); err == nil || !strings.Contains(err.Error(), "путей больше") {
		t.Fatalf("предел путей: %v", err)
	}
	// Градиентов больше предела.
	b.Reset()
	b.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><defs>`)
	for i := 0; i <= maxGrads; i++ {
		b.WriteString(`<linearGradient id="g` + strconv.Itoa(i) + `"/>`)
	}
	b.WriteString(`</defs></svg>`)
	if _, err := Parse([]byte(b.String())); err == nil || !strings.Contains(err.Error(), "градиентов больше") {
		t.Fatalf("предел градиентов: %v", err)
	}
	// Остановок.
	b.Reset()
	b.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><defs><linearGradient id="a">`)
	for i := 0; i <= maxStops; i++ {
		b.WriteString(`<stop offset="0"/>`)
	}
	b.WriteString(`</linearGradient></defs></svg>`)
	if _, err := Parse([]byte(b.String())); err == nil || !strings.Contains(err.Error(), "остановок") {
		t.Fatalf("предел остановок: %v", err)
	}
	// Огромный файл: чтение обрывается на пределе, а не читает до конца потока.
	if _, err := ReadLimited(io.LimitReader(zeroReader{}, MaxSVGBytes+10)); err == nil || !strings.Contains(err.Error(), "больше") {
		t.Fatalf("огромный файл: %v", err)
	}
	if _, err := Parse(make([]byte, MaxSVGBytes+1)); err == nil {
		t.Fatal("Parse принял файл больше предела")
	}
	if data, err := ReadLimited(strings.NewReader("ok")); err != nil || string(data) != "ok" {
		t.Fatalf("малый файл: %v", err)
	}
}

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) { return len(p), nil }

func TestScanPathBox(t *testing.T) {
	// Кривая с выпуклостью за пределами опорных точек: экстремум по Безье, а не рамка контрольных точек.
	info, err := scanPath("M 0 0 C 0 100 100 100 100 0 z")
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(info.box.Y1-75) > 1e-9 || info.box.X0 != 0 || info.box.X1 != 100 || info.box.Y0 != 0 || info.rect {
		t.Fatalf("рамка кривой: %+v", info)
	}
	// Выход за концы: точка кривой правее обеих концов.
	info, _ = scanPath("M 0 0 C 150 0 150 10 0 10")
	if info.box.X1 < 100 || info.box.X1 > 150 {
		t.Fatalf("рамка выпуклости: %+v", info.box)
	}
	for in, rect := range map[string]bool{
		"M 0 0 L 10 0 L 10 5 L 0 5 z":             true,
		"M 0 0 L 10 0 L 10 5 L 0 5 z M 1 1 L 2 2": false,
		"M 0 0 L 10 0 L 10 5 L 0 5 L 0 0 z":       true,
		"M 0 0 L 10 0 L 10 5 L 0 5 L 0 0":         true,
		"M 0 0 L 10 1 L 10 5 L 0 5 z":             false,
		"M 0 0 L 10 0 L 10 5 z":                   false,
	} {
		i, err := scanPath(in)
		if err != nil || i.rect != rect {
			t.Fatalf("%q: rect %v (ждали %v), %v", in, i.rect, rect, err)
		}
	}
}

func TestResolveSrc(t *testing.T) {
	root := tempDir(t)
	outside := tempDir(t)
	if err := os.MkdirAll(filepath.Join(root, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{filepath.Join(root, "sub", "a.svg"), filepath.Join(outside, "secret.svg")} {
		if err := os.WriteFile(f, wrap(okPath), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := ResolveSrc(root, "sub/a.svg"); err != nil {
		t.Fatal(err)
	}
	if data, err := ReadSrc(root, "sub/a.svg"); err != nil || len(data) == 0 {
		t.Fatalf("ReadSrc: %v", err)
	}
	abs := filepath.Join(outside, "secret.svg")
	for name, tc := range map[string][2]string{
		"пустой root":     {"", "sub/a.svg"},
		"пробельный root": {"  ", "sub/a.svg"},
		"пустой rel":      {root, ""},
		"..":              {root, "../" + filepath.Base(outside) + "/secret.svg"},
		"sub/../..":       {root, "sub/../../x.svg"},
		"абсолютный":      {root, abs},
		"нет файла":       {root, "sub/none.svg"},
		"нет root":        {filepath.Join(root, "нет"), "a.svg"},
	} {
		if _, err := ResolveSrc(tc[0], tc[1]); err == nil {
			t.Errorf("%s: принято", name)
		}
	}
	if runtime.GOOS != "windows" { // символическая ссылка наружу — отдельный путь выхода
		if err := os.Symlink(abs, filepath.Join(root, "link.svg")); err != nil {
			t.Fatal(err)
		}
		if _, err := ResolveSrc(root, "link.svg"); err == nil {
			t.Error("ссылка за пределы -src принята")
		}
		if err := os.Symlink(outside, filepath.Join(root, "dir")); err != nil {
			t.Fatal(err)
		}
		if _, err := ResolveSrc(root, "dir/secret.svg"); err == nil {
			t.Error("ссылка на каталог за пределами -src принята")
		}
	}
	if _, err := ReadSrc(root, "sub"); err == nil {
		t.Error("каталог принят как файл")
	}
}

func FuzzParse(f *testing.F) {
	for _, n := range []string{"sheet.svg", "glued.svg"} {
		if b, err := os.ReadFile(filepath.Join("testdata", n)); err == nil {
			f.Add(b)
		}
	}
	f.Add(wrap(okPath))
	f.Add([]byte(`<svg viewBox="0 0 1 1"><path fill="rgb(1,2,3)" d="M 0 0 C 1e300 1 2 3 4 5 z"/></svg>`))
	f.Fuzz(func(t *testing.T, data []byte) {
		doc, err := Parse(data)
		if err != nil {
			return
		}
		for _, p := range doc.Paths {
			if math.IsNaN(p.Box.X0+p.Box.Y0+p.Box.X1+p.Box.Y1) || p.Box.X1 < p.Box.X0 || p.Box.Y1 < p.Box.Y0 {
				t.Fatalf("плохая рамка %+v", p.Box)
			}
		}
		doc.Sheet()
	})
}
