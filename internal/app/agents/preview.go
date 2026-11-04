package agents

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"strings"

	xdraw "golang.org/x/image/draw"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Превью для агента (design §3, §6): картинка только на уровне 3, JPEG без метаданных, не больше maxJPEG.

// Пределы картинки.
const (
	maxJPEG        = 600 << 10 // байт JPEG: base64 ≈ 800 КБ, строка ответа остаётся под maxReply
	maxPreviewSide = 2048      // длинная сторона, которую служба вообще примет от Host
	minPreviewSide = 64        // ниже этого EncodePreview не сжимает: лучше отказ
	photoDefPx     = 768       // size по умолчанию у get_photo_preview
	spreadDefPx    = 1200      // size по умолчанию у get_spread_preview
)

// ErrUnavailable — Host отвечает ею, когда файл фото недоступен (нет, не читается, чужой том, UNC):
// агент получает понятный отказ без пути, а не «внутреннюю ошибку».
var ErrUnavailable = errors.New("agents: файл недоступен")

// ErrNoSpread — Host.Render не нашёл разворот: альбом сменился между разбором входа и рендером.
var ErrNoSpread = errors.New("agents: нет такого разворота")

// previewOut — подпись к картинке превью фото.
type previewOut struct {
	meta
	ID   string `json:"id"`
	File string `json:"file,omitempty" jsonschema:"имя файла (с уровня 2)"`
	W    int    `json:"w" jsonschema:"ширина картинки, px"`
	H    int    `json:"h" jsonschema:"высота картинки, px"`
	img  []byte
}

func (o previewOut) jpegBytes() []byte { return o.img }

// spreadPreviewOut — подпись к картинке разворота.
type spreadPreviewOut struct {
	meta
	ID          string `json:"id"`
	N           int    `json:"n" jsonschema:"номер разворота с 1; 0 — обложка"`
	Cover       bool   `json:"cover,omitempty"`
	W           int    `json:"w" jsonschema:"ширина картинки, px"`
	H           int    `json:"h" jsonschema:"высота картинки, px"`
	Unavailable int    `json:"unavailable,omitempty" jsonschema:"сколько фото разворота пропали или не читаются (см. status в list_photos): на картинке их места пустые"`
	img         []byte
}

func (o spreadPreviewOut) jpegBytes() []byte { return o.img }

// withImage — ответ с картинкой: okResult кладёт её в Content после текста.
type withImage interface{ jpegBytes() []byte }

// getPhotoPreview — уменьшенное фото через Host.Preview.
func (c *call) getPhotoPreview(in previewIn) (previewOut, error) {
	pc, err := c.photoCtx()
	if err != nil {
		return previewOut{}, err
	}
	id := model.BaseID(strings.TrimSpace(in.ID))
	p, ok := pc.byID[id]
	if !ok && !pc.missing[id] {
		return previewOut{}, errNoPhoto
	}
	if pc.status(id) != "ok" {
		return previewOut{}, errPhotoGone
	}
	px := defInt(in.Size, photoDefPx)
	b, w, h, err := c.image(func(w, h int) bool { return max(w, h) <= px }, func() ([]byte, error) { return c.s.h.Preview(c.ctx, id, px) })
	if err != nil {
		return previewOut{}, err
	}
	return previewOut{meta: c.metaOf(pc.v, pc.ed), ID: id, File: c.text(p.Name, maxFile), W: w, H: h, img: b}, nil
}

// getSpreadPreview — разворот или обложка «видом печати» через Host.Render; рендеры идут по одному.
func (c *call) getSpreadPreview(in spreadPreviewIn) (spreadPreviewOut, error) {
	pc, err := c.photoCtx()
	if err != nil {
		return spreadPreviewOut{}, err
	}
	sv, n, ok := findSpread(pc.v, string(in.Spread))
	if !ok {
		return spreadPreviewOut{}, errNoSpread
	}
	select {
	case c.env.render <- struct{}{}:
		defer func() { <-c.env.render }()
	default:
		return spreadPreviewOut{}, errCallBusy
	}
	px := defInt(in.Size, spreadDefPx)
	b, w, h, err := c.image(func(w, _ int) bool { return w <= px }, func() ([]byte, error) { return c.s.h.Render(c.ctx, sv.ID, px) })
	if err != nil {
		return spreadPreviewOut{}, err
	}
	return spreadPreviewOut{meta: c.metaOf(pc.v, pc.ed), ID: cleanID(sv.ID), N: n, Cover: n == 0, W: w, H: h,
		Unavailable: pc.unavailable(sv), img: b}, nil
}

// unavailable — сколько разных фото разворота (в местах и на фонах) пропали или не читаются по Status.
// Фото недоступного тома Render тоже рисует пустыми: Host.Status включает их в пропавшие (0.31-143-71).
func (pc *photoCtx) unavailable(sv model.SpreadView) int {
	seen := map[string]bool{}
	count := func(id string) {
		if id = model.BaseID(id); id != "" {
			seen[id] = true
		}
	}
	for _, id := range sv.Photos {
		count(id)
	}
	for _, b := range sv.Backdrops {
		count(b.Photo)
	}
	n := 0
	for id := range seen {
		if pc.status(id) != "ok" {
			n++
		}
	}
	return n
}

func defInt(v, def int) int {
	if v <= 0 {
		return def
	}
	return v
}

// image вызывает Host и пересобирает ответ: наружу уходят только байты, выпущенные кодировщиком
// службы (decision 0.31-143-70). fits — картинка не больше запрошенного размера. Срок — c.ctx: после
// него любой ответ Host — «время вышло». Уровень читается заново: окно могло выключить картинки,
// пока шёл рендер.
func (c *call) image(fits func(w, h int) bool, fetch func() ([]byte, error)) (b []byte, w, h int, err error) {
	b, err = fetch()
	switch {
	case c.ctx.Err() != nil:
		return nil, 0, 0, errTimeout
	case errors.Is(err, ErrUnavailable):
		return nil, 0, 0, errPhotoGone
	case errors.Is(err, ErrNoSpread):
		return nil, 0, 0, errNoSpread
	case err != nil:
		return nil, 0, 0, err
	case len(b) > maxJPEG:
		return nil, 0, 0, errTooBigOne
	}
	if b, w, h, err = normalizeJPEG(b, fits); err != nil {
		return nil, 0, 0, err
	}
	if c.s.level() < 3 {
		return nil, 0, 0, errNoImages
	}
	return b, w, h, nil
}

var (
	errNotJPEG  = errors.New("agents: превью не JPEG")
	errJPEGSize = errors.New("agents: размер превью вне предела")
)

// normalizeJPEG принимает ответ Host: сначала заголовок (размер до выделения памяти), затем полное
// декодирование и перекодирование своим EncodePreview. Метаданные (EXIF, XMP, ICC, COM), сегменты
// после скана, хвост со вторым JPEG, второй SOF — всё, что разбор заголовков мог бы пропустить,
// пропадает по построению: у декодера Go нет канала, которым оно дошло бы до кодировщика.
func normalizeJPEG(b []byte, fits func(w, h int) bool) (out []byte, w, h int, err error) {
	cfg, err := jpeg.DecodeConfig(bytes.NewReader(b))
	if err != nil {
		return nil, 0, 0, errNotJPEG
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || max(cfg.Width, cfg.Height) > maxPreviewSide || !fits(cfg.Width, cfg.Height) {
		return nil, 0, 0, errJPEGSize
	}
	img, err := jpeg.Decode(bytes.NewReader(b))
	if err != nil {
		return nil, 0, 0, errNotJPEG
	}
	if out, err = EncodePreview(img, max(cfg.Width, cfg.Height)); err != nil {
		return nil, 0, 0, err
	}
	out2, err := jpeg.DecodeConfig(bytes.NewReader(out))
	if err != nil {
		return nil, 0, 0, err
	}
	return out, out2.Width, out2.Height, nil
}

// EncodePreview — JPEG для превью из готовой картинки: не больше side по длинной стороне (меньшую
// не растягивает), прозрачность заливается белым, метаданных в выходе нет (jpeg.Encode не пишет
// даже APP0). Качество 80 → 70 → 60, затем сторона × 0,8, пока не уложится в maxJPEG. Размер src
// ограничивает вызывающий (читает файл с пределом). Для Host.Preview и Host.Render.
func EncodePreview(src image.Image, side int) ([]byte, error) {
	side = min(max(side, minPreviewSide), maxPreviewSide)
	sb := src.Bounds()
	if sb.Empty() {
		return nil, errJPEGSize
	}
	side = min(side, max(sb.Dx(), sb.Dy())) // источник меньше: уменьшать нечего, кружить на той же картинке незачем
	for {
		dw, dh := fit(sb.Dx(), sb.Dy(), side)
		dst := image.NewRGBA(image.Rect(0, 0, dw, dh))
		draw.Draw(dst, dst.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
		if dw == sb.Dx() && dh == sb.Dy() {
			draw.Draw(dst, dst.Bounds(), src, sb.Min, draw.Over)
		} else {
			xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, sb, xdraw.Over, nil)
		}
		for _, q := range []int{80, 70, 60} {
			var buf bytes.Buffer
			if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: q}); err != nil {
				return nil, err
			}
			if buf.Len() <= maxJPEG {
				return buf.Bytes(), nil
			}
		}
		if side = side * 4 / 5; side < minPreviewSide {
			return nil, errTooBigOne
		}
	}
}

// fit — размер w×h, вписанный в side по длинной стороне; увеличения нет, сторона не меньше 1.
func fit(w, h, side int) (int, int) {
	if max(w, h) <= side {
		return w, h
	}
	if w >= h {
		return side, max(1, h*side/w)
	}
	return max(1, w*side/h), side
}
