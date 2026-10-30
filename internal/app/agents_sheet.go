package app

// Лист-сетка для агента (151, design §2): Host.Sheet. Миниатюры ленты (400 px) читаются классом
// прогрева (library.Cache.FileBgCtx, 0.34-151-22), каждая сразу уменьшается до клетки и отпускается,
// затем contactsheet рисует лист и кодирует JPEG ≤ 600 КБ. Не успевшая к сроку клетка — «…» (pending),
// повторный вызов берёт готовое. Пути, имена и метаданные в лист не попадают: только пиксели.

import (
	"context"
	"errors"
	"image"
	"sync"
	"time"

	xdraw "golang.org/x/image/draw"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/app/agents"
	"github.com/IliaSA101/sosninaalbums/internal/contactsheet"
	"github.com/IliaSA101/sosninaalbums/internal/imgfile"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// Пределы листа.
const (
	sheetReaders = 4                // миниатюр читается одновременно
	sheetSlack   = time.Second      // запас срока вызова на рисунок и кодирование листа
	sheetNoLimit = 19 * time.Second // срок клеток, если у вызова срока нет (двойной срок инструмента минус запас)
)

var errAgentsSheet = errors.New("лист фото не собран")

// sheetCell — состояние и картинка одной клетки.
type sheetCell struct {
	img   image.Image
	state contactsheet.State
}

// agentsSheet — Host.Sheet: лист из фото in.IDs (пустой ID — «×»), состояния клеток по порядку.
func (a *App) agentsSheet(ctx context.Context, in agents.SheetReq) ([]byte, []string, error) {
	grid, side := contactsheet.Clamp(in.Grid, in.Side)
	if a.cache == nil || len(in.IDs) == 0 || len(in.IDs) > grid*grid {
		return nil, nil, errAgentsSheet
	}
	limit := sheetNoLimit
	if dl, ok := ctx.Deadline(); ok {
		limit = time.Until(dl) - sheetSlack
	}
	cctx, cancel := context.WithTimeout(ctx, max(limit, 0))
	defer cancel()

	cells := make([]sheetCell, len(in.IDs))
	box := contactsheet.CellBox(grid, side)
	next := make(chan int)
	var wg sync.WaitGroup
	for range min(sheetReaders, len(in.IDs)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range next {
				cells[i] = a.sheetCell(cctx, in.IDs[i], box)
			}
		}()
	}
	for i := range in.IDs {
		next <- i
	}
	close(next)
	wg.Wait()
	if ctx.Err() != nil { // вызов снят или срок вышел целиком: лист никому не нужен
		return nil, nil, ctx.Err()
	}

	states := make([]string, len(cells))
	draw := make([]contactsheet.Cell, len(cells))
	for i, c := range cells {
		states[i] = c.state.String()
		draw[i] = contactsheet.Cell{Img: c.img, N: in.First + i, State: c.state}
	}
	jpg, _, _, err := contactsheet.Render(draw, grid, side)
	if err != nil {
		return nil, nil, errAgentsSheet
	}
	return jpg, states, nil
}

// sheetCell готовит одну клетку: фото ленты или альбома (тот же источник и те же проверки, что у
// agentsPreview), миниатюра классом прогрева, декод через imgfile с бюджетом памяти. Срок cctx вышел —
// pending; нет файла, не читается, не локальный путь, закрытый том — broken.
func (a *App) sheetCell(ctx context.Context, id string, box int) sheetCell {
	if id == "" {
		return sheetCell{state: contactsheet.Broken}
	}
	if ctx.Err() != nil {
		return sheetCell{state: contactsheet.Pending}
	}
	// UNC и прочее не локальное отсекается до пробы тома: проба сетевого пути — сетевое обращение (0.31-143-61).
	a.mu.Lock()
	sp, inStrip := a.byID[model.BaseID(id)]
	a.mu.Unlock()
	if inStrip && paths.CheckLocalPath(sp.Path) != nil {
		return sheetCell{state: contactsheet.Broken}
	}
	p, ok, gone := a.mediaPhoto(id)
	if !ok || gone || paths.CheckLocalPath(p.Path) != nil {
		return sheetCell{state: contactsheet.Broken}
	}
	file, err := a.cache.FileBgCtx(ctx, p)
	if err != nil {
		return sheetFail(ctx)
	}
	img, release, err := imgfile.OpenOrientedCtx(ctx, file)
	if err != nil {
		return sheetFail(ctx)
	}
	defer release()
	return sheetCell{img: fitBox(img, box), state: contactsheet.Ready}
}

// sheetFail — клетка не вышла: срок — pending (повторный вызов возьмёт готовое), иначе — broken.
func sheetFail(ctx context.Context) sheetCell {
	if ctx.Err() != nil {
		return sheetCell{state: contactsheet.Pending}
	}
	return sheetCell{state: contactsheet.Broken}
}

// fitBox уменьшает img до квадрата box по длинной стороне (в копию: исходная картинка принадлежит
// бюджету и освобождается сразу); меньшая миниатюра остаётся как есть.
func fitBox(img image.Image, box int) image.Image {
	sb := img.Bounds()
	w, h := sb.Dx(), sb.Dy()
	if box <= 0 || max(w, h) <= box {
		dst := image.NewRGBA(image.Rect(0, 0, w, h))
		xdraw.Draw(dst, dst.Bounds(), img, sb.Min, xdraw.Src)
		return dst
	}
	if w >= h {
		h = max(1, h*box/w)
		w = box
	} else {
		w = max(1, w*box/h)
		h = box
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	xdraw.ApproxBiLinear.Scale(dst, dst.Bounds(), img, sb, xdraw.Src, nil)
	return dst
}
