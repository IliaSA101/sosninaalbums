package export

import (
	"context"
	"fmt"
	"image"
	"log"
	"os"
	"path/filepath"
	"runtime/debug"
	"sync"

	"github.com/IliaSA101/sosninaalbums/internal/render"
)

// Швы для тестов; продукт их не меняет.
var (
	renderSheet = render.RenderCtx
	saveSheet   = saveJPEG
	// pipeMaxPixels — наибольший холст, при котором держим два холста сразу; выше — по
	// одному, как до конвейера (091-D3): 120 Мп ≈ 480 МБ RGBA.
	pipeMaxPixels = 120_000_000
)

// sheetPart — один файл из холста: целиком — весь холст, страница — её половина.
// name — имя файла без папки.
type sheetPart struct {
	name string
	img  image.Image
}

// sheet — холст одного рендера задания и файлы из него. img == nil — задание без файлов
// (нужен только сдвиг прогресса).
type sheet struct {
	n     int // номер задания в jobs
	img   *image.RGBA
	parts []sheetPart
	last  bool // последний лист задания: после записи — progress
}

// maxCanvases — сколько холстов живёт сразу: два (следующий рисуется, пока пишется
// предыдущий) или один, если холст огромен.
func maxCanvases(opt render.Options) int {
	w, h := render.CanvasSize(opt)
	if w*h > pipeMaxPixels {
		return 1
	}
	return 2
}

// runPipe — рендер в вызывающей горутине, запись в одной своей; между ними небуферный канал
// листов и токены холстов. Имена, порядок записи и .tmp+rename те же, что при записи по очереди.
// Решение «остановиться» обе стороны принимают по ctx.Err() исходного контекста; rctx лишь
// будит рендер, когда запись вышла (иначе он завис бы на токене или канале).
func runPipe(ctx context.Context, spreads []render.Spread, jobs []Job, opt render.Options, dir string, progress func(done, total int)) error {
	rctx, stopRender := context.WithCancel(ctx)
	defer stopRender()
	sheets := make(chan sheet)
	tokens := make(chan struct{}, maxCanvases(opt))
	var (
		wg   sync.WaitGroup
		werr error
	)
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer stopRender()
		// Паника записи (кодер, прогресс) — ошибка экспорта: guard горутины App сюда не дотягивается.
		werr = noPanic("записи", func() error { return writeSheets(ctx, sheets, tokens, jobs, opt, dir, progress) })
	}()
	// close — и при панике рендера: иначе запись висела бы на range sheets.
	rerr := noPanic("рендера", func() error {
		defer close(sheets)
		return renderSheets(ctx, rctx, sheets, tokens, spreads, jobs, opt)
	})
	wg.Wait()
	if rerr == nil && werr == nil {
		return nil
	}
	// Отмена важнее ошибки (App по Canceled пишет «остановлен»); ошибка записи — раньше
	// ошибки рендера: она либо по номеру раньше, либо сама остановила рендер.
	if err := ctx.Err(); err != nil {
		return err
	}
	if werr != nil {
		return werr
	}
	return rerr
}

// noPanic выполняет f; паника — ошибка с её текстом (стек — в лог), а не падение программы.
func noPanic(what string, f func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("ПАНИКА %s экспорта: %v\n%s", what, r, debug.Stack())
			err = fmt.Errorf("сбой %s: %v", what, r)
		}
	}()
	return f()
}

// renderSheets рисует листы заданий по порядку и отдаёт записи.
func renderSheets(ctx, rctx context.Context, sheets chan<- sheet, tokens chan struct{}, spreads []render.Spread, jobs []Job, opt render.Options) error {
	r := renderer{rctx: rctx, sheets: sheets, tokens: tokens, opt: opt, total: len(spreads)}
	for n, j := range jobs {
		if err := ctx.Err(); err != nil {
			return err
		}
		if j.Index < 0 || j.Index >= len(spreads) {
			return fmt.Errorf("разворот %d: нет в альбоме", j.Index+1)
		}
		if err := r.job(n, j, spreads[j.Index]); err != nil {
			return fmt.Errorf("разворот %d: %w", j.Index+1, err)
		}
	}
	return nil
}

// renderer — сторона рендера конвейера: общие каналы и параметры одного запуска.
type renderer struct {
	rctx   context.Context
	sheets chan<- sheet
	tokens chan struct{}
	opt    render.Options
	total  int // число разворотов альбома: номер и ширина имени файла
}

// job рисует листы задания n: «целиком» и/или холст из двух страниц.
func (r renderer) job(n int, j Job, s render.Spread) error {
	if !j.Whole && !j.Left && !j.Right {
		if !r.send(sheet{n: n, last: true}) {
			return r.rctx.Err()
		}
		return nil
	}
	if j.Whole {
		name := j.Prefix + FileName(j.Index, r.total)
		sh := sheet{n: n, last: !j.Left && !j.Right}
		err := r.one(s, r.opt, sh, func(img *image.RGBA) []sheetPart { return []sheetPart{{name, img}} })
		if err != nil {
			return err
		}
	}
	if !j.Left && !j.Right {
		return nil
	}
	so := r.opt
	so.Split = true
	if err := CheckCanvas(so); err != nil {
		return err
	}
	return r.one(s, so, sheet{n: n, last: true}, func(img *image.RGBA) []sheetPart {
		half := img.Bounds().Dx() / 2
		var parts []sheetPart
		for page, on := range [2]bool{j.Left, j.Right} {
			if on {
				rc := image.Rect(page*half, 0, (page+1)*half, img.Bounds().Dy())
				parts = append(parts, sheetPart{j.Prefix + PageName(j.Index, r.total, page+1), img.SubImage(rc)})
			}
		}
		return parts
	})
}

// one: токен холста → рендер → лист записи.
func (r renderer) one(s render.Spread, opt render.Options, sh sheet, parts func(*image.RGBA) []sheetPart) error {
	select {
	case r.tokens <- struct{}{}:
	case <-r.rctx.Done():
		return r.rctx.Err()
	}
	var img *image.RGBA
	err := noPanic("рендера", func() (err error) { img, err = renderSheet(r.rctx, s, opt); return err })
	if err != nil {
		<-r.tokens
		return err
	}
	sh.img, sh.parts = img, parts(img)
	if !r.send(sh) {
		<-r.tokens
		return r.rctx.Err()
	}
	return nil
}

func (r renderer) send(sh sheet) bool {
	select {
	case r.sheets <- sh:
		return true
	case <-r.rctx.Done():
		return false
	}
}

// writeSheets пишет листы по порядку. Задание, записанное не до конца (ошибка, отмена,
// рендер его следующего листа упал), убирается: недописанный лист в папке не остаётся (080-11).
// Начатый файл дописывается; следующий файл задания после отмены не начинается.
func writeSheets(ctx context.Context, sheets <-chan sheet, tokens <-chan struct{}, jobs []Job, opt render.Options, dir string, progress func(done, total int)) error {
	var written []string
	drop := func() {
		for _, p := range written {
			_ = os.Remove(p)
		}
		written = nil
	}
	for sh := range sheets {
		j := jobs[sh.n]
		for _, part := range sh.parts {
			// Первый файл задания, чей холст уже отрисован, пишем и при отмене (как до
			// конвейера: рендер вернулся — файл дописан); следующие — только пока не отменили.
			if len(written) > 0 {
				if err := ctx.Err(); err != nil {
					drop()
					return err
				}
			}
			p := filepath.Join(dir, part.name)
			// Паника кодера — как ошибка записи: недописанное задание убирается.
			if err := noPanic("записи", func() error { return saveSheet(part.img, p, opt) }); err != nil {
				drop()
				return fmt.Errorf("разворот %d: %w", j.Index+1, err)
			}
			written = append(written, p)
		}
		if sh.img != nil {
			<-tokens
		}
		if sh.last {
			written = nil
			progress(sh.n+1, len(jobs))
		}
	}
	drop()
	return nil
}
