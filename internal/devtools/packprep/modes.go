package packprep

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/png"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/devtools/sheetcut"
	"github.com/IliaSA101/sosninaalbums/internal/devtools/svgcut"
)

// piece — готовый к записи предмет, отданный режимом.
type piece struct {
	file    string
	name    string
	index   int          // номер нарезки в источнике, 0 — источник даёт один файл
	img     *image.NRGBA // для миниатюры, dHash и каймы
	data    []byte       // уже закодированный файл (фон); nil — PNG из img
	flags   []string
	stretch *float64
	upscale *Upscale
}

var (
	quarterSuffix = [4]string{"tl", "tr", "bl", "br"}
	quarterWords  = [4]string{"сверху слева", "сверху справа", "снизу слева", "снизу справа"}
)

// item обрабатывает один источник рецепта; ошибка источника копится в отчёте и не прерывает остальные.
func (r *runner) item(idx int, it Item) {
	defer func() {
		if p := recover(); p != nil { // битая картинка не должна ронять весь прогон
			r.rep.fail("%s: внутренний сбой обработки: %v", it.Src, p)
		}
	}()
	if why, dead := r.rec.Skip[it.Src]; dead {
		r.rep.Skipped = append(r.rep.Skipped, it.Src+": "+why)
		return
	}
	var err error
	switch it.Mode {
	case ModeBg:
		err = r.doBg(idx, it)
	case ModeSvg:
		err = r.doSvg(idx, it)
	case ModeCornerFrame:
		err = r.doCornerFrame(idx, it)
	default:
		err = r.doSheet(idx, it)
	}
	if err != nil {
		r.rep.fail("%s: %v", it.Src, err)
	}
}

// skipped — предмет №k источника отбракован в рецепте (src#k).
func (r *runner) skipped(it Item, k int) bool {
	key := fmt.Sprintf("%s#%d", it.Src, k)
	why, dead := r.rec.Skip[key]
	if dead {
		r.rep.Skipped = append(r.rep.Skipped, key+": "+why)
	}
	return dead
}

// corners снимает углы растрового источника для edges.png один раз.
func (r *runner) corners(it Item, img image.Image) {
	if r.seen[it.Src] {
		return
	}
	r.seen[it.Src] = true
	r.edges = append(r.edges, edgeBlock{label: path.Base(it.Src), img: cornerBlock(img)})
}

func (r *runner) doBg(idx int, it Item) error {
	sf, err := readSrc(r.cfg.Src, it.Src)
	if err != nil {
		return err
	}
	img, err := sf.decode()
	if err != nil {
		return err
	}
	r.corners(it, img)
	res, err := makeBg(sf.Data, sf.Ext, img, it.Out, it.Trim)
	if err != nil {
		return err
	}
	p := piece{file: it.Out, name: it.Name, img: res.Img, data: res.Data, flags: res.Notes}
	if res.Factor > 1 {
		p.upscale = &Upscale{From: res.From, Factor: res.Factor}
		if res.Factor > UpscaleFlag {
			p.flags = append(p.flags, res.upscaleNote()+": больше ×"+fmt.Sprint(UpscaleFlag)+", решает кроп 300 dpi")
		}
	}
	if res.Copied {
		p.flags = append(p.flags, "файл байт в байт")
	}
	return r.emit(idx, it, p)
}

// sheetOptions — параметры sheetcut для предмета рецепта.
func (r *runner) sheetOptions(it Item) (sheetcut.Options, error) {
	o := sheetcut.DefaultOptions()
	o.Mode = map[string]string{ModeFrame: sheetcut.ModeFrame, ModeQuarters: sheetcut.ModeQuarters, ModeRows: sheetcut.ModeRows}[it.Mode]
	o.Rembg = r.cfg.Rembg
	if it.BG != "" {
		o.BG = it.BG
	}
	o.Thr, o.Holes, o.SplitRows = uint8(min(255, max(0, it.Thr))), it.Holes, it.N
	var err error
	o.Cols, o.Rows, err = sheetcut.ParseGrid(it.Grid)
	return o, err
}

func (r *runner) doSheet(idx int, it Item) error {
	sf, err := readSrc(r.cfg.Src, it.Src)
	if err != nil {
		return err
	}
	img, err := sf.decode()
	if err != nil {
		return err
	}
	r.corners(it, img)
	o, err := r.sheetOptions(it)
	if err != nil {
		return err
	}
	if o.BG == sheetcut.BGRembg && r.cfg.Rembg == "" {
		return errors.New("bg: rembg, а SHEETCUT_REMBG не задан")
	}
	res, err := sheetcut.Cut(img, o)
	if err != nil {
		return err
	}
	for _, n := range res.Notes {
		r.rep.Notes = append(r.rep.Notes, it.Src+": "+n)
	}
	var errs []error
	for _, s := range res.Stickers {
		k := s.Index
		file := fmt.Sprintf("%s%02d.png", it.Out, k)
		switch it.Mode {
		case ModeFrame:
			k, file = 0, it.Out
		case ModeQuarters:
			k = s.Cell[1]*2 + s.Cell[0] + 1
			file = it.Out + quarterSuffix[k-1] + ".png"
		}
		if k > 0 && r.skipped(it, k) {
			continue
		}
		p := piece{file: file, name: r.nameFor(it, k, len(res.Stickers)), index: k, img: s.Img, flags: s.Warnings}
		if s.Frame != nil && s.Frame.Window {
			st := s.Frame.Stretch
			p.stretch = &st
		}
		if err := r.emit(idx, it, p); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (r *runner) doSvg(idx int, it Item) error {
	if r.cfg.Render == nil {
		return errors.New("svg: браузер не задан (PACKPREP_CHROME)")
	}
	data, err := svgcut.ReadSrc(r.cfg.Src, it.Src)
	if err != nil {
		return err
	}
	o := svgcut.DefaultOptions()
	if it.Gap > 0 {
		o.Gap = it.Gap
	}
	if it.Side > 0 {
		o.Side = it.Side
	}
	if o.Cols, o.Rows, err = svgcut.ParseGrid(it.Grid); err != nil {
		return err
	}
	o.Knock, o.Split, o.Drop, o.Overlay, o.Render = it.Knock, it.Split, it.Drop, true, r.cfg.Render
	res, cutErr := svgcut.Cut(data, o)
	if res == nil {
		return cutErr
	}
	for _, n := range res.Notes {
		r.rep.Notes = append(r.rep.Notes, it.Src+": "+n)
	}
	errs := []error{cutErr}
	for _, s := range res.Items {
		if s.Img == nil || r.skipped(it, s.Index) {
			continue
		}
		p := piece{file: fmt.Sprintf("%s%02d.png", it.Out, s.Index), name: r.nameFor(it, s.Index, len(res.Items)), index: s.Index, img: s.Img, flags: s.Warnings}
		if len(s.Whites) > 0 {
			p.flags = append(p.flags, whitesNote(s.Whites, idx))
		}
		errs = append(errs, r.emit(idx, it, p))
	}
	if res.Overlay != nil && res.WhiteTotal > 0 {
		errs = append(errs, r.writeOverlay(idx, res.Overlay))
	}
	return errors.Join(errs...)
}

// whitesNote — белые пути предмета SVG: что оставлено, что вырезано (knock).
func whitesNote(ws []svgcut.White, idx int) string {
	parts := make([]string, len(ws))
	for i, w := range ws {
		mark := "оставлен"
		if w.Knocked {
			mark = "вырез"
		}
		parts[i] = fmt.Sprintf("w%d %s", w.N, mark)
	}
	return fmt.Sprintf("белые пути: %s (лист %s)", strings.Join(parts, ", "), overlayName(idx))
}

// nameFor — имя предмета №k источника из рецепта: names по номерам или основа name.
func (r *runner) nameFor(it Item, k, count int) string {
	switch {
	case len(it.Names) > 0:
		if k >= 1 && k <= len(it.Names) {
			return it.Names[k-1]
		}
		return ""
	case it.Mode == ModeQuarters && k >= 1 && k <= 4:
		return it.Name + " " + quarterWords[k-1]
	case count > 1 && k > 0:
		return fmt.Sprintf("%s %d", it.Name, k)
	}
	return it.Name
}

// emit проверяет и записывает предмет, снимает миниатюру, dHash и кайму.
func (r *runner) emit(idx int, it Item, p piece) error {
	if p.name == "" {
		r.rep.fail("%s: нет имени у предмета №%d источника (%s): names короче нарезки", it.Src, p.index, p.file)
	}
	if !ValidFileName(p.file) {
		return fmt.Errorf("имя файла %q не по правилам набора", p.file)
	}
	low := strings.ToLower(p.file)
	if r.names[low] {
		return fmt.Errorf("повтор файла %q: другой источник уже дал это имя", p.file)
	}
	r.names[low] = true
	kind := it.outKind()
	data := p.data
	if data == nil {
		var buf bytes.Buffer
		if err := png.Encode(&buf, p.img); err != nil {
			return err
		}
		data = buf.Bytes()
		if len(data) > sheetcut.MaxFileBytes {
			return fmt.Errorf("%s: файл %d МБ больше предела набора %d МБ", p.file, len(data)>>20, sheetcut.MaxFileBytes>>20)
		}
	}
	w, h := p.img.Bounds().Dx(), p.img.Bounds().Dy()
	if kind != KindBg && max(w, h) > sheetcut.MaxSidePx {
		return fmt.Errorf("%s: сторона %d px больше предела импорта %d", p.file, max(w, h), sheetcut.MaxSidePx)
	}
	full := filepath.Join(r.cfg.Out, filepath.FromSlash(p.file))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(full, data, 0o644); err != nil {
		return err
	}
	o := &Output{File: p.file, Kind: kind, Style: it.Style, Family: familyOfItem(it), Name: p.name, Theme: it.Theme, Src: it.Src,
		Index: p.index, W: w, H: h, CmW: float64(w) / 300 * 2.54, CmH: float64(h) / 300 * 2.54, Bytes: len(data),
		Stretch: p.stretch, Upscale: p.upscale, Flags: p.flags, item: idx, order: r.order}
	r.order++
	if o.Theme == "" {
		o.Theme = r.rec.Theme
	}
	if kind != KindBg {
		if f := CheckFringe(p.img); f.Flag {
			o.Flags = append(o.Flags, fmt.Sprintf("кайма?: светлых px у контура %.0f %% против %.0f %% внутри", 100*f.RingLight, 100*f.CoreLight))
		}
	}
	o.thumb = fit(p.img, thumbSide)
	o.hash = hashOf(o.thumb)
	r.outs = append(r.outs, o)
	return nil
}
