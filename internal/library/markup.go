package library

import (
	"bytes"
	"context"
	"errors"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/disintegration/imaging"

	"github.com/IliaSA101/sosninaalbums/internal/imgfile"
	"github.com/IliaSA101/sosninaalbums/internal/library/psd"
)

// Файл-разметка типографии (066): PSD, JPEG или PNG. Подложка только для экрана: разбор PSD —
// пакет psd (вход недоверенный), здесь — проверка пути и открытие файла.

// MarkupMaxSide — превью разметки не больше этого по длинной стороне, px (design §2.3). JPEG/PNG
// до этого размера отдаются файлом как есть (WebP под .jpg/.png — никогда, см. Big).
const MarkupMaxSide = 4096

// MarkupInfo — что известно о файле-разметке. У JPEG/PNG направляющих и DPI нет, картинка есть.
type MarkupInfo struct {
	Path   string // по чему читать (у ссылки — её цель, как у PlainFile)
	Size   int64
	Mod    int64 // ModTime в нс: вместе с путём и размером — отпечаток файла (GuideView.Key)
	W, H   int
	DPI    float64       // 0 — неизвестен
	Guides []MarkupGuide // направляющие PSD, px документа
	Image  bool          // есть картинка для подложки
	Format string        // "jpeg" | "png" | "webp" по содержимому; у PSD пусто
	Note   string        // "" или почему видны только направляющие
}

// MarkupGuide — направляющая PSD (Pos в px документа, Horz — горизонтальная). Псевдоним: пакеты
// альбома знают library, а пакет разбора psd им не нужен.
type MarkupGuide = psd.Guide

// IsPSD — разметка в PSD (WebView его не показывает: картинку всегда строит Go).
func (m MarkupInfo) IsPSD() bool { return strings.EqualFold(filepath.Ext(m.Path), ".psd") }

// Big — картинку подложки строит Go (превью ≤ MarkupMaxSide): PSD, JPEG/PNG длиннее MarkupMaxSide
// и любой WebP (он проходит под .jpg/.png, а отдавать его байты в WebView2 не нужно: libwebp
// браузера разбирал бы недоверенный файл). Остальные JPEG/PNG окно показывает самим файлом.
func (m MarkupInfo) Big() bool {
	return m.IsPSD() || m.Format == "webp" || max(m.W, m.H) > MarkupMaxSide
}

// ErrPSB — отказ PSB с текстом для пользователя: salbum проверяет путь тем же текстом, не зная
// пакета psd.
var ErrPSB = psd.ErrPSB

// errMarkupExt — расширение не из разрешённых; PSB — свой текст (ErrPSB).
var errMarkupExt = errors.New("файл разметки — только PSD, JPEG или PNG")

// MarkupFile — проверка файла-разметки при выборе и для вида: локальный путь без «..», обычный
// файл, PSD/JPEG/PNG по расширению и заголовку, пределы размера. PSD читается без данных
// изображения (заголовок и ресурсы), JPEG/PNG — как свой фон (BackdropFile).
func MarkupFile(path string) (MarkupInfo, error) {
	if strings.EqualFold(filepath.Ext(path), ".psb") {
		return MarkupInfo{}, ErrPSB
	}
	path, size, err := PlainFile(path, ".jpg", ".jpeg", ".png", ".psd")
	if errors.Is(err, ErrExt) {
		return MarkupInfo{}, errMarkupExt
	}
	if err != nil {
		return MarkupInfo{}, err
	}
	m := MarkupInfo{Path: path, Size: size}
	if st, err := os.Stat(path); err == nil {
		m.Mod = st.ModTime().UnixNano()
	}
	if !m.IsPSD() {
		d, err := imgfile.FileDims(path, 0) // размер и формат по заголовку, предел MaxPixels
		if err != nil {
			return MarkupInfo{}, err
		}
		m.Format = d.Format
		ph, err := ReadInfo(path)
		if err != nil {
			return MarkupInfo{}, err
		}
		m.W, m.H, m.Image = ph.W, ph.H, true
		return m, nil
	}
	err = withPSD(path, func(r io.ReadSeeker, size int64) error {
		in, err := psd.ReadInfo(r, size)
		m.Size, m.W, m.H, m.DPI, m.Guides, m.Image, m.Note = size, in.W, in.H, in.DPI, in.Guides, in.Image, in.Note
		return err
	})
	if err != nil {
		return MarkupInfo{}, err
	}
	return m, nil
}

// withPSD открывает обычный файл и отдаёт его с размером по дескриптору (не по пути: файл могли
// подменить между проверкой и чтением).
func withPSD(path string, fn func(r io.ReadSeeker, size int64) error) error {
	f, err := imgfile.OpenRegular(path)
	if err != nil {
		return err
	}
	defer f.Close() //nolint:errcheck // чтение: ошибка закрытия ничего не теряет
	st, err := f.Stat()
	if err != nil {
		return err
	}
	return fn(f, st.Size())
}

// ErrNoMarkupImage — в PSD нет картинки для подложки (видны только направляющие).
var ErrNoMarkupImage = errors.New("в PSD нет картинки для подложки")

// MarkupPreview — картинка разметки для /media, когда её строит Go (MarkupInfo.Big): PSD —
// превью ≤ MarkupMaxSide в PNG (BestSpeed: белые шаблоны с тонкими линиями так меньше и быстрее
// JPEG, research §3); JPEG/PNG больше MarkupMaxSide — уменьшенная копия, JPEG 88 или PNG, если
// есть прозрачность (как миниатюры). Остальным JPEG/PNG — nil без ошибки: их отдаёт файл.
// Превью берёт место в общем бюджете декодирования.
func MarkupPreview(path string) (data []byte, ctype string, err error) {
	return MarkupPreviewCtx(context.Background(), path)
}

// MarkupPreviewCtx — MarkupPreview с отменой: ожидание места в бюджете прерывается ctx (запрос
// /media закрыт — ошибка ctx.Err(), бюджет не тронут), иначе пока экспорт держит бюджет,
// запросы к большой разметке висят и копятся.
func MarkupPreviewCtx(ctx context.Context, path string) (data []byte, ctype string, err error) {
	m, err := MarkupFile(path)
	if err != nil || !m.Big() {
		return nil, "", err
	}
	if !m.IsPSD() {
		return bigPreview(ctx, m.Path)
	}
	if !m.Image {
		return nil, "", ErrNoMarkupImage
	}
	px := previewPixels(m.W, m.H)
	if err := imgfile.DecodeBudget.AcquireCtx(ctx, px); err != nil {
		return nil, "", err
	}
	defer imgfile.DecodeBudget.Release(px)
	var buf bytes.Buffer
	err = withPSD(m.Path, func(r io.ReadSeeker, size int64) error {
		img, _, err := psd.Preview(r, size, MarkupMaxSide)
		if err != nil {
			return err
		}
		if img == nil {
			return ErrNoMarkupImage
		}
		return (&png.Encoder{CompressionLevel: png.BestSpeed}).Encode(&buf, img)
	})
	if err != nil {
		return nil, "", err
	}
	return buf.Bytes(), "image/png", nil
}

// beforeEncode вызывается перед кодированием превью; подменяется в тестах (занят ли бюджет).
var beforeEncode = func() {}

// bigPreview — уменьшенная копия большого JPEG/PNG. Декод — только через OpenOrientedCtx (бюджет
// памяти, пределы размера и EXIF-поворот внутри). Бюджет держится, пока оригинал нужен
// (прозрачность и уменьшение); кодирование копии идёт уже без него, иначе оно зря задерживает
// экспорт и миниатюры.
func bigPreview(ctx context.Context, path string) ([]byte, string, error) {
	img, release, err := imgfile.OpenOrientedCtx(ctx, path)
	if err != nil {
		return nil, "", err
	}
	// OnceFunc: defer страхует панику в fit, явный вызов ниже — освобождение до кодирования.
	release = sync.OnceFunc(release)
	defer release()
	opaque := isOpaque(img)
	small := fit(img, MarkupMaxSide, imaging.Box)
	release()
	beforeEncode()
	var buf bytes.Buffer
	if opaque {
		if err := jpeg.Encode(&buf, small, &jpeg.Options{Quality: 88}); err != nil {
			return nil, "", err
		}
		return buf.Bytes(), "image/jpeg", nil
	}
	if err := (&png.Encoder{CompressionLevel: png.BestSpeed}).Encode(&buf, small); err != nil {
		return nil, "", err
	}
	return buf.Bytes(), "image/png", nil
}

// previewPixels — пикселей в превью w×h (тот же шаг k, что у psd.Preview).
func previewPixels(w, h int) int64 {
	k := max((max(w, h)+MarkupMaxSide-1)/MarkupMaxSide, 1)
	return int64((w+k-1)/k) * int64((h+k-1)/k)
}
