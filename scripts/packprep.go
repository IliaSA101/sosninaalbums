//go:build ignore

// packprep собирает папку набора библиотеки по рецепту: нарезка листов, фоны, манифест и лист проверки.
// Инструмент разработчика, не функция программы; логика — internal/devtools/packprep (спека 129, Т3).
//
//	go run scripts/packprep.go -src docs/private/library-src/pilot -out .gotmp/packs/school-autumn \
//	    docs/private/library-src/packs/school-autumn/recipe.json
//
// Рецепт — JSON вне git, неизвестное поле — отказ. Каждый предмет рецепта — один источник внутри -src
// (src без «..» и абсолютных путей) и режим: bg (фон: готовый файл байт в байт или Lanczos до 3602 px,
// разворот 7205×3602, JPEG q92; апскейл ×N > 1,8 — флаг), frame (рамка), cornerframe (рамка из четырёх
// уголков набора: src — «…-tl.png», рядом -tr, -bl, -br; line: none|thin, color — «#rrggbb», пусто — цвет
// образца линии в уголке или доминирующий), quarters (уголки 2×2), rows
// (разделители), sheet (лист стикеров на белом), svg (SVG Recraft по предметам, вырезы knock). Брак
// отмечается в рецепте (skip, thr, holes, knock, bg), а не правкой файлов.
//
// Выход в -out: файлы предметов по out рецепта, manifest.src.json (для scripts/library-pack.mjs), LICENSE.txt
// и _check/: items-dark.png, items-color.png (плитки 168 px по семействам), items-big-dark*.png (512 px),
// edges.png (углы исходных листов 400×400), whites-NN.png (белые пути SVG), report.json. Прогон одинаков
// байт в байт. -out пуста или без manifest.src.json в непустой папке — отказ; временные файлы — -out/.tmp.
//
// Внешние команды — флаг или переменная, без оболочки: PACKPREP_CHROME (chrome-headless-shell, Chrome,
// Edge — для svg) и SHEETCUT_REMBG (rembg birefnet-general — для bg: rembg). Код выхода 1, если в
// отчёте есть ошибки (семейство больше двух, бюджет, нет имени, сбой источника); папка и лист проверки
// при этом записаны — один прогон собирает все провалы.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/devtools/packprep"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

func main() {
	if err := run(); err != nil {
		msg := err.Error()
		if !strings.HasPrefix(msg, "packprep:") {
			msg = "packprep: " + msg
		}
		fmt.Fprintln(os.Stderr, msg)
		os.Exit(1)
	}
}

func run() error {
	src := flag.String("src", "", "каталог исходников; src рецепта — только внутри (обязателен)")
	out := flag.String("out", "", "папка набора (обязательна; временные файлы — <out>/.tmp)")
	chrome := flag.String("chrome", os.Getenv("PACKPREP_CHROME"), "браузер для svg; по умолчанию PACKPREP_CHROME")
	rembg := flag.String("rembg", os.Getenv("SHEETCUT_REMBG"), "команда rembg для листов стикеров; по умолчанию SHEETCUT_REMBG")
	asJSON := flag.Bool("json", false, "отчёт в JSON")
	flag.Parse()
	if flag.NArg() != 1 {
		return fmt.Errorf("нужен один файл рецепта; флаги — перед ним")
	}
	if strings.TrimSpace(*src) == "" || strings.TrimSpace(*out) == "" {
		return fmt.Errorf("-src и -out обязательны: пустое значение не заменяется текущей папкой")
	}
	var abs [3]string
	for i, p := range []string{*src, *out, flag.Arg(0)} {
		a, err := filepath.Abs(p)
		if err != nil {
			return err
		}
		if err := paths.CheckLocalPath(a); err != nil {
			return err
		}
		abs[i] = a
	}
	data, err := readRecipe(abs[2])
	if err != nil {
		return err
	}
	rec, err := packprep.ParseRecipe(data)
	if err != nil {
		return err
	}
	// rembg (sheetcut) держит временные файлы в папке ОС: переводим её в <out>/.tmp (Run создаёт её сам).
	tmp := filepath.Join(abs[1], packprep.TmpDir)
	for _, v := range []string{"TMPDIR", "TMP", "TEMP"} {
		_ = os.Setenv(v, tmp)
	}
	t0 := time.Now()
	rep, err := packprep.Run(packprep.Config{Src: abs[0], Out: abs[1], Chrome: *chrome, Rembg: *rembg}, rec)
	if rep != nil {
		printReport(rep, *asJSON)
	}
	if err != nil {
		return err
	}
	fmt.Printf("готово за %s, папка: %s\n", time.Since(t0).Round(time.Millisecond), abs[1])
	if !rep.OK {
		return fmt.Errorf("в отчёте ошибок: %d", len(rep.Errors))
	}
	return nil
}

// readRecipe читает рецепт: размер проверяется до чтения.
func readRecipe(path string) ([]byte, error) {
	st, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !st.Mode().IsRegular() || st.Size() > packprep.MaxRecipeBytes {
		return nil, fmt.Errorf("рецепт %q: не обычный файл или больше %d КБ", path, packprep.MaxRecipeBytes>>10)
	}
	return os.ReadFile(path) //nolint:gosec // путь прошёл paths.CheckLocalPath, размер проверен выше
}

func printReport(rep *packprep.Report, asJSON bool) {
	if asJSON {
		if b, err := rep.JSON(); err == nil {
			fmt.Println(string(b))
		}
		return
	}
	fmt.Print(rep.Text())
}
