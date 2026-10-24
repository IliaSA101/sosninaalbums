//go:build ignore

// sheetcut режет лист стикеров (одна картинка PNG, много предметов) на отдельные PNG RGBA для наборов
// библиотеки. Инструмент разработчика, не функция программы; логика — internal/devtools/sheetcut.
//
//	go run scripts/sheetcut.go -grid 3x4 -preview -out .gotmp/cut лист.png
//
// Вход: PNG с альфой (после Magnific, rembg) или лист на белом фоне без альфы. Фон: -bg auto (по
// умолчанию: есть содержательная альфа — как есть; белый лист — rembg, если команда задана, иначе
// свой алгоритм с предупреждением), none, own, rembg.
//
// Основной путь для белого листа — rembg с моделью birefnet-general (CPU ≈ 20 с на лист 4K): на
// листах navy-silver почти как эталон Magnific (IoU маски 0,96), чистые края, пузырьки, крылья, кольца.
// Установить rembg в отдельный venv: pip install "rembg[cpu,cli]"; папка моделей — переменная U2NET_HOME.
// Команда — -rembg или переменная SHEETCUT_REMBG (путь к rembg.exe/rembg.cmd из своего venv); запуск
// «<cmd> i -m <модель> вход выход.png» без оболочки. Модель — -model; bria-rmbg отклоняется (лицензия
// весов BRIA несовместима с CC0 наборов); birefnet-massive даёт серые пятна, birefnet-hrsod — облако.
// -defringe (по умолчанию вкл.) у rembg: цвет полупрозрачных пикселей восстанавливается от белого,
// светлой каймы на тёмной подложке нет.
//
// Свой алгоритм (-bg own) хуже: белые клинья между листьями венка (замкнутые белые области у основания
// не сняты), белые крошки у ножек бокалов. Заливка от краёв по «почти белому» (-white), замкнутые «чистые»
// белые области (внутри колец) — тоже фон, белое с фактурой (крылья, бумага) остаётся, у границы мягкий
// край с восстановлением цвета (defringe уже внутри).
//
// Порядок: фон -> -dense (альфа ≥ порога → 255; листы Magnific просвечивают, видимое 224–254; 0 —
// выкл.) -> связные области по -alpha -> мусор меньше -minarea прочь -> склейка в стикеры:
//   - -grid КОЛОНКИxРЯДЫ (3x4): область относится к ячейке по центру рамки, все области ячейки —
//     один стикер (пузырьки, отдельные бокалы), стикеров ровно столько, сколько ячеек с предметами;
//   - без -grid: мелкие куски клеятся к ближайшей крупной области, если рамки ближе -glue px.
//
// Выход в -out (по умолчанию «<вход>-cut» рядом с входом): NN.png слева направо, сверху вниз,
// обрезка по содержимому с полем -pad px; сторона ≤ 4096 (больше — уменьшает; предел импорта своих
// стикеров, decor.MaxStickerSide), файл ≤ 30 МБ (проверка scripts/library-pack.mjs, kind sticker).
// Остатки прошлого прогона в -out (NN.png с номером больше числа стикеров) удаляются, остальные файлы — нет. -preview добавляет preview-dark.png и
// preview-color.png (контактные листы на #1e1e22 и #4a7a6e). Отчёт в stdout: px, см при 300 dpi,
// предупреждения (длинная сторона < 600 px; «слиплось?» — рамка > 1,8 медианной или заходит
// в соседнюю ячейку); -json — отчёт в JSON.
//
// Запрос к модели для листа: «sticker sheet, N stickers, evenly spaced, not touching, pure white
// background, no shadows, no text». 4K и сетка 4x4 ≈ 850 px на стикер (печать до ~7 см), 3x3 ≈ 1200 px
// (~10 см); 1K — только выбрать стиль. См. docs/qa/library.md «Лист стикеров».
package main

import (
	"flag"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/devtools/sheetcut"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "sheetcut:", err)
		os.Exit(1)
	}
}

func run() error {
	d := sheetcut.DefaultOptions()
	out := flag.String("out", "", "папка вывода (по умолчанию «<вход>-cut» рядом с входом)")
	bg := flag.String("bg", d.BG, "фон: auto, none (альфа как есть), own (белый фон снять самим), rembg")
	rembg := flag.String("rembg", os.Getenv("SHEETCUT_REMBG"), "команда rembg (путь к rembg.exe/rembg.cmd); по умолчанию SHEETCUT_REMBG")
	model := flag.String("model", d.Model, "-bg rembg: модель (bria-rmbg отклоняется — лицензия)")
	defringe := flag.Bool("defringe", d.Defringe, "-bg rembg: убрать светлую кайму (цвет полупрозрачных пикселей — от белого)")
	dense := flag.Int("dense", int(d.Dense), "альфа ≥ порога → 255 (0 — выключено)")
	alpha := flag.Int("alpha", int(d.Alpha), "порог альфы связной области")
	grid := flag.String("grid", "", "сетка КОЛОНКИxРЯДЫ, например 3x4")
	glue := flag.Int("glue", d.Glue, "без сетки: расстояние приклейки мелких кусков, px (0 — только соприкасающиеся)")
	minArea := flag.Int("minarea", 0, "мусор меньше этой площади, px (0 — 1/10000 листа)")
	pad := flag.Int("pad", 0, "прозрачное поле вокруг предмета, px")
	white := flag.Int("white", int(d.White), "-bg own: «почти белый» — минимум каналов не меньше")
	hole := flag.Int("hole", 0, "-bg own: замкнутая белая область меньше — предмет, px (0 — авто)")
	edge := flag.Int("edge", d.Edge, "-bg own: полоса мягкого края, px")
	preview := flag.Bool("preview", false, "контактные листы preview-dark.png и preview-color.png")
	asJSON := flag.Bool("json", false, "отчёт в JSON")
	flag.Parse()
	if flag.NArg() != 1 {
		return fmt.Errorf("нужен один входной PNG; флаги — перед именем файла")
	}
	in := flag.Arg(0)
	o := d
	o.Rembg, o.Model, o.Defringe = *rembg, *model, *defringe
	o.BG, o.Dense, o.Alpha = *bg, uint8(min(255, max(0, *dense))), uint8(min(255, max(0, *alpha)))
	o.Glue, o.MinArea, o.Pad, o.White, o.HoleMin, o.Edge = *glue, *minArea, *pad, uint8(min(255, max(0, *white))), *hole, *edge
	var err error
	if o.Cols, o.Rows, err = sheetcut.ParseGrid(*grid); err != nil {
		return err
	}
	dir := *out
	if dir == "" {
		dir = strings.TrimSuffix(in, filepath.Ext(in)) + "-cut"
	}
	return process(in, dir, o, *preview, *asJSON)
}

func process(in, dir string, o sheetcut.Options, preview, asJSON bool) error {
	t0 := time.Now()
	f, err := os.Open(in)
	if err != nil {
		return err
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		return fmt.Errorf("%s: %w", in, err)
	}
	res, err := sheetcut.Cut(img, o)
	if err != nil {
		return err
	}
	if err := sheetcut.Save(res, dir); err != nil {
		return err
	}
	if preview {
		if err := sheetcut.SavePreview(res, dir, o.Cols); err != nil {
			return err
		}
	}
	if asJSON {
		b, err := res.JSON()
		if err != nil {
			return err
		}
		fmt.Println(string(b))
		return nil
	}
	fmt.Print(res.Text())
	fmt.Printf("готово за %s, папка: %s\n", time.Since(t0).Round(time.Millisecond), dir)
	return nil
}
