package export

import (
	"context"
	"errors"
	"fmt"
	"image"
	"math"
	"os"
	"path/filepath"
	"strconv"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/render"
)

// Страницы книги для листалки (139, 146): JPEG длинной стороной ≈ ShareLongSide px, без выбора
// размера в интерфейсе (0.32-146-3), обрез срезан по внешним краям (0.30-139-6).
const (
	ShareLongSide = 1600 // px по длинной стороне страницы после обреза (WebSpec.LongSide по умолчанию)
	WebQuality    = 82   // качество JPEG страниц
	maxLongSide   = 8000 // больше — не страница листалки, а печатный файл
)

// WebPage — страница книги по порядку: имя файла во временной папке выгрузки (146) и признак
// белой страницы (файла нет, в манифесте — blank). Имя заполнено и у белой: это номер её места.
type WebPage struct {
	Name  string
	Blank bool
}

// WebSpec — параметры записи страниц. PageW/PageH — страница альбома до обреза (Settings),
// BleedX/BleedY — полоса обреза по внешним краям, мм; Front/Back — сторонки обложки на листе
// обложки (CoverZone front/back), мм; нужны, если обложка есть. LongSide — длинная сторона
// страницы после обреза, px (0 — ShareLongSide): по ней NewWebSpec считает DPI, рендер берёт DPI.
type WebSpec struct {
	DPI            int
	LongSide       int
	PageW, PageH   float64
	BleedX, BleedY float64
	Quality        int // 0 — WebQuality
	Front, Back    layout.Rect
}

// Trimmed — страница после обреза, мм: у сгиба не режем, по высоте — сверху и снизу.
func (w WebSpec) Trimmed() (tw, th float64) { return w.PageW - w.BleedX, w.PageH - 2*w.BleedY }

// WebDPI — dpi, при котором длинная сторона страницы tw×th мм ≈ long px (округление вверх:
// не меньше long; long ≤ 0 — ShareLongSide). Страница 300 мм при 1600 px → 136 dpi → 1606 px.
// Негодная страница (≤ 0, NaN) — 0.
func WebDPI(tw, th float64, long int) int {
	m := math.Max(tw, th)
	if !(m > 0) || math.IsInf(m, 0) {
		return 0
	}
	if long <= 0 {
		long = ShareLongSide
	}
	return int(min(math.Ceil(float64(long)*25.4/m), math.MaxUint16))
}

// NewWebSpec собирает параметры из настроек и вида печати (Editor.Snapshot): обрез — тот же
// model.BleedAxes, что у розовой полосы View.Bleed (формат с направляющими даёт свои оси),
// сторонки — из View.Cover.Zones. Страница после обреза пуста — ошибка.
func NewWebSpec(s model.Settings, v model.View) (WebSpec, error) {
	return NewWebSpecLong(s, v, ShareLongSide)
}

// NewWebSpecLong — NewWebSpec с длинной стороной страницы long px (≤ 0 — ShareLongSide).
func NewWebSpecLong(s model.Settings, v model.View, long int) (WebSpec, error) {
	if long <= 0 {
		long = ShareLongSide
	}
	if long > maxLongSide {
		return WebSpec{}, fmt.Errorf("длинная сторона страницы %d px больше предела %d", long, maxLongSide)
	}
	bx, by := model.BleedAxes(s)
	pg := s.Page() // PageW читает только model (TestPageWReaders)
	w := WebSpec{PageW: pg.W, PageH: pg.H, BleedX: bx, BleedY: by, Quality: WebQuality, LongSide: long}
	tw, th := w.Trimmed()
	if !(tw > 0 && th > 0) {
		return WebSpec{}, fmt.Errorf("после обреза от страницы %v×%v мм ничего не остаётся", pg.W, pg.H)
	}
	w.DPI = WebDPI(tw, th, long)
	if c := v.Cover; c != nil {
		for _, z := range c.Zones {
			switch z.Kind {
			case "front":
				w.Front = z.Rect
			case "back":
				w.Back = z.Rect
			}
		}
		if w.Front.W <= 0 || w.Front.H <= 0 || w.Back.W <= 0 || w.Back.H <= 0 {
			return WebSpec{}, errors.New("на обложке нет лицевой или задней стороны")
		}
	}
	return w, nil
}

// check — параметры годны; cover — обложка в выгрузке.
func (w WebSpec) check(cover bool) error {
	tw, th := w.Trimmed()
	switch {
	case w.DPI <= 0:
		return fmt.Errorf("dpi листалки %d: должен быть больше нуля", w.DPI)
	case w.LongSide < 0 || w.LongSide > maxLongSide:
		return fmt.Errorf("длинная сторона страницы %d px: от 0 до %d", w.LongSide, maxLongSide)
	case !(w.PageW > 0 && w.PageH > 0 && w.BleedX >= 0 && w.BleedY >= 0 && tw > 0 && th > 0):
		return fmt.Errorf("страница листалки %v×%v мм, обрез %v/%v мм: не годится", w.PageW, w.PageH, w.BleedX, w.BleedY)
	case cover && (!(w.Front.W > 0 && w.Front.H > 0) || !(w.Back.W > 0 && w.Back.H > 0)):
		return errors.New("у обложки не заданы лицевая и задняя стороны")
	}
	return nil
}

// WebPageNames — имена страниц книги по порядку: с обложкой [000 (лицевая), 001, 002 … 2N+1
// (задняя)], без — [001 … 2N]. Номер — место в книге; ширина — 3 цифры, больше 999 страниц шире.
func WebPageNames(spreads int, cover bool) []string {
	n, first := 2*spreads, 1
	if cover {
		n, first = n+2, 0
	}
	width := max(3, len(strconv.Itoa(first+n-1)))
	out := make([]string, n)
	for k := range out {
		out[k] = fmt.Sprintf("%0*d.jpg", width, first+k)
	}
	return out
}

// RunWeb рисует обложку и развороты, режет страницы, пишет непустые в imgDir (папка уже есть,
// ссылка вместо неё — отказ) и возвращает список страниц по порядку книги. progress — после
// каждого листа (обложка — один лист). Остановка — ctx: недописанный лист убирается, готовые
// остаются, возвращается ctx.Err(); службе тогда манифест не собирать и ничего не публиковать.
func RunWeb(ctx context.Context, spreads []render.Spread, cover *CoverJob, w WebSpec, imgDir string, progress func(done, total int)) ([]WebPage, error) {
	if err := w.check(cover != nil); err != nil {
		return nil, err
	}
	if st, err := os.Lstat(imgDir); err != nil || !st.IsDir() {
		return nil, fmt.Errorf("папка страниц %q недоступна", imgDir)
	}
	r := webRun{w: w, dir: imgDir, names: WebPageNames(len(spreads), cover != nil)}
	r.pages = make([]WebPage, len(r.names))
	for k, n := range r.names {
		r.pages[k].Name = n
	}
	total, done, first := len(spreads), 0, 0
	if cover != nil {
		total, first = total+1, 1
	}
	step := func() {
		if done++; progress != nil {
			progress(done, total)
		}
	}
	if cover != nil {
		if err := r.cover(ctx, cover); err != nil {
			return nil, err
		}
		step()
	}
	for i, s := range spreads {
		if err := r.spread(ctx, s, first+2*i); err != nil {
			return nil, fmt.Errorf("разворот %d: %w", i+1, err)
		}
		step()
	}
	if err := ctx.Err(); err != nil { // остановка после последнего листа: книга не готова
		return nil, err
	}
	return r.pages, nil
}

// webRun — один запуск: параметры, папка и список страниц, который заполняют листы.
type webRun struct {
	w     WebSpec
	dir   string
	names []string
	pages []WebPage
}

func (r *webRun) opt() render.Options {
	q := r.w.Quality
	if q == 0 {
		q = WebQuality
	}
	return render.Options{PageW: r.w.PageW, PageH: r.w.PageH, DPI: r.w.DPI, Quality: q}
}

// cover: лист обложки → лицевая (место 0) и задняя (последнее) сторонки.
func (r *webRun) cover(ctx context.Context, j *CoverJob) error {
	opt := r.opt()
	opt.PageW, opt.PageH = j.Opt.PageW, j.Opt.PageH // холст — лист обложки, не страница
	img, err := r.render(ctx, j.Spread, opt)
	if err != nil {
		return fmt.Errorf("обложка: %w", err)
	}
	wins := [2]image.Rectangle{r.mmRect(r.w.Front, img), r.mmRect(r.w.Back, img)}
	if err := r.write(img, wins, [2]int{0, len(r.pages) - 1}, opt); err != nil {
		return fmt.Errorf("обложка: %w", err)
	}
	return nil
}

// spread: лист разворота (две страницы) → левая и правая, места k и k+1 списка страниц.
func (r *webRun) spread(ctx context.Context, s render.Spread, k int) error {
	opt := r.opt()
	opt.Split = true // две равные страницы, как «По страницам»
	img, err := r.render(ctx, s, opt)
	if err != nil {
		return err
	}
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	half, x0, y0 := w/2, mmPx(r.w.BleedX, r.w.DPI), mmPx(r.w.BleedY, r.w.DPI)
	// Окна одного размера: у сгиба не режем, обрез — слева у левой, справа у правой, сверху и снизу.
	wins := [2]image.Rectangle{image.Rect(x0, y0, half, h-y0), image.Rect(half, y0, w-x0, h-y0)}
	return r.write(img, wins, [2]int{k, k + 1}, opt)
}

func (r *webRun) render(ctx context.Context, s render.Spread, opt render.Options) (*image.RGBA, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := CheckCanvas(opt); err != nil {
		return nil, err
	}
	var img *image.RGBA
	err := noPanic("рендера", func() (err error) { img, err = renderSheet(ctx, s, opt); return err })
	if err != nil {
		return nil, err
	}
	return img, nil
}

// write пишет страницы-окна листа на места at; белые файлом не пишутся. Начатый лист
// дописывается; при ошибке его файлы убираются.
func (r *webRun) write(img *image.RGBA, wins [2]image.Rectangle, at [2]int, opt render.Options) (err error) {
	var written []string
	defer func() {
		if err != nil {
			for _, p := range written {
				_ = os.Remove(p)
			}
		}
	}()
	for i, win := range wins {
		if win.Empty() || !win.In(img.Bounds()) {
			return fmt.Errorf("окно страницы %v вне листа %v", win, img.Bounds())
		}
		blank := whiteRect(img, win)
		r.pages[at[i]].Blank = blank
		if blank {
			continue
		}
		p := filepath.Join(r.dir, r.names[at[i]])
		sub := img.SubImage(win)
		if err := noPanic("записи", func() error { return saveSheet(sub, p, opt) }); err != nil {
			return err
		}
		written = append(written, p)
	}
	return nil
}

// mmRect — прямоугольник мм листа в px холста (рёбра округляются как px рендера), в пределах холста.
func (r *webRun) mmRect(m layout.Rect, img *image.RGBA) image.Rectangle {
	d := r.w.DPI
	return image.Rect(mmPx(m.X, d), mmPx(m.Y, d), mmPx(m.X+m.W, d), mmPx(m.Y+m.H, d)).Intersect(img.Bounds())
}

// mmPx — мм в px при dpi: то же округление, что у render.
func mmPx(mm float64, dpi int) int { return int(math.Round(mm / 25.4 * float64(dpi))) }

// whiteRect — все пиксели окна rc чисто белые (#FFFFFF): страница без содержимого на белом
// фоне. Страница в цвете альбома не белая — это картинка (0.30-139-7).
func whiteRect(img *image.RGBA, rc image.Rectangle) bool {
	for y := rc.Min.Y; y < rc.Max.Y; y++ {
		row := img.Pix[img.PixOffset(rc.Min.X, y):img.PixOffset(rc.Max.X, y)]
		for _, b := range row {
			if b != 0xFF {
				return false
			}
		}
	}
	return true
}
