//go:build ignore

// svgcut режет SVG Recraft (лист предметов из векторных путей) на отдельные PNG RGBA для наборов
// библиотеки. Инструмент разработчика, не функция программы; логика — internal/devtools/svgcut.
//
//	PACKPREP_CHROME=/usr/bin/google-chrome go run scripts/svgcut.go -src docs/private/library-src/pilot \
//	    -out .gotmp/svgcut/school -grid 6x6 -knock 812,640 school-1/school-1-recraft-36.svg
//
// Вход — SVG внутри -src (путь после флагов относительный; «..», абсолютный путь и ссылки наружу —
// отказ). Разбор строгий: элементы svg, defs, linearGradient, radialGradient, stop, path, g, metadata;
// пути — абсолютные M L C и z/Z; transform — только translate(0,0); иное — отказ с номером пути.
//
// Лист (первый прямоугольник во весь лист с каналами ≥ 245) определяется по форме и снимается.
// Предметы: пути, чьи рамки ближе -gap (по умолчанию 2), либо ячейки -grid КОЛОНКИxРЯДЫ (по центру
// рамки пути: склеенное разделяется). -split ГРУППА:x=ЧИСЛО|y=ЧИСЛО режет склеенную группу, -drop 3,5
// снимает группы; номера — по отчёту прогона без split/drop (на листе whites.png они синие).
//
// Белые пути (цвета листа) — белые части предметов (собаки, облака) или пустоты, закрашенные белым
// (скакалка, угольник, ручки кубка). Что есть что, видит глаз: -knock X,Y (повторяется) — точка в
// координатах листа (viewBox) на пустоте; верхний видимый белый путь под ней вырезается (белый путь
// минус пути над ним — кант над пустотой остаётся), прочие белые остаются. Попадание определяет тот
// же браузер (служебный растр: белые пути цветами-номерами); промах (под точкой не белый путь) —
// ошибка, все промахи сразу.
//
// Растр — chrome-headless-shell, Chrome или Edge: -chrome или PACKPREP_CHROME. Длинная сторона
// предмета -side (1600, ≤ 4096), обрезка по альфе. Временные файлы — -out/.tmp, после прогона удаляются.
// Выход в -out: NN.png слева направо, сверху вниз; whites.png — лист с обведёнными белыми путями (wN,
// красные — оставлены, зелёные — вырезаны) и рамками групп. Отчёт в stdout: px, см при 300 dpi, число
// белых путей у предмета; -json — отчёт в JSON.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/devtools/svgcut"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

func main() {
	if err := run(); err != nil {
		msg := err.Error()
		if !strings.HasPrefix(msg, "svgcut:") {
			msg = "svgcut: " + msg
		}
		fmt.Fprintln(os.Stderr, msg)
		os.Exit(1)
	}
}

// listFlag — повторяемый флаг со своим разбором.
type listFlag struct {
	parse func(string) error
}

func (l listFlag) String() string     { return "" }
func (l listFlag) Set(s string) error { return l.parse(s) }

func run() error {
	o := svgcut.DefaultOptions()
	src := flag.String("src", "", "каталог исходников; SVG — внутри него (обязателен)")
	out := flag.String("out", "", "папка вывода (обязательна; временные файлы — в <out>/.tmp)")
	chrome := flag.String("chrome", os.Getenv("PACKPREP_CHROME"), "браузер (chrome-headless-shell, Chrome, Edge); по умолчанию PACKPREP_CHROME")
	grid := flag.String("grid", "", "сетка КОЛОНКИxРЯДЫ, например 6x6")
	drop := flag.String("drop", "", "снять группы по номерам: 3,5")
	whites := flag.Bool("whites", true, "лист whites.png с подсветкой белых путей и рамок групп")
	asJSON := flag.Bool("json", false, "отчёт в JSON")
	flag.Float64Var(&o.Gap, "gap", o.Gap, "пути ближе этого — один предмет, в координатах листа")
	flag.IntVar(&o.Side, "side", o.Side, "длинная сторона предмета, px (≤ 4096)")
	flag.Var(listFlag{func(s string) error {
		p, err := svgcut.ParseKnock(s)
		o.Knock = append(o.Knock, p)
		return err
	}}, "knock", "точка выреза X,Y в координатах листа (повторяется)")
	flag.Var(listFlag{func(s string) error {
		l, err := svgcut.ParseSplit(s)
		o.Split = append(o.Split, l)
		return err
	}}, "split", "резать группу: ГРУППА:x=ЧИСЛО или ГРУППА:y=ЧИСЛО (повторяется)")
	flag.Parse()
	if flag.NArg() != 1 {
		return fmt.Errorf("нужен один SVG относительно -src; флаги — перед именем файла")
	}
	if strings.TrimSpace(*src) == "" || strings.TrimSpace(*out) == "" {
		return fmt.Errorf("-src и -out обязательны: пустое значение не заменяется текущей папкой")
	}
	srcAbs, outAbs, err := localDirs(*src, *out)
	if err != nil {
		return err
	}
	var perr error
	if o.Cols, o.Rows, perr = svgcut.ParseGrid(*grid); perr != nil {
		return perr
	}
	if *drop != "" {
		if o.Drop, perr = svgcut.ParseDrop(*drop); perr != nil {
			return perr
		}
	}
	o.Overlay = *whites
	tmp := filepath.Join(outAbs, ".tmp")
	defer func() { _ = os.RemoveAll(tmp) }()
	o.Render = svgcut.Chrome{Cmd: *chrome, Tmp: tmp}.Render
	t0 := time.Now()
	data, err := svgcut.ReadSrc(srcAbs, flag.Arg(0))
	if err != nil {
		return err
	}
	res, err := svgcut.Cut(data, o)
	if res != nil && len(res.Items) > 0 {
		// Отчёт и картинки удачных предметов выдаём и при сбое других: один прогон — все провалы.
		if serr := svgcut.Save(res, outAbs); serr != nil && err == nil {
			err = serr
		}
		report(res, *asJSON)
	}
	if err != nil {
		return err
	}
	fmt.Printf("готово за %s, папка: %s\n", time.Since(t0).Round(time.Millisecond), outAbs)
	return nil
}

// localDirs приводит -src и -out к абсолютным локальным путям (paths.CheckLocalPath).
func localDirs(src, out string) (string, string, error) {
	var abs [2]string
	for i, p := range []string{src, out} {
		a, err := filepath.Abs(p)
		if err != nil {
			return "", "", err
		}
		if err := paths.CheckLocalPath(a); err != nil {
			return "", "", err
		}
		abs[i] = a
	}
	return abs[0], abs[1], nil
}

func report(res *svgcut.Result, asJSON bool) {
	if asJSON {
		if b, err := res.JSON(); err == nil {
			fmt.Println(string(b))
		}
		return
	}
	fmt.Print(res.Text())
}
