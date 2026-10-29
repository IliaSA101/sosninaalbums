// Package export собирает развороты в JPEG для печати.
package export

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"math"
	"strconv"

	"github.com/IliaSA101/sosninaalbums/internal/colorspace"
	"github.com/IliaSA101/sosninaalbums/internal/imgfile"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/render"
)

// Marker — текст COM-сегмента: по нему программа узнаёт свои файлы в папке экспорта.
const Marker = imgfile.ExportMarker

// FileName — имя файла i-го (с нуля) разворота: 01.jpg, при 100+ разворотах 001.jpg.
// Номер 00 остаётся под обложку, программа его не пишет.
func FileName(i, total int) string {
	return fmt.Sprintf("%0*d.jpg", max(2, len(strconv.Itoa(total))), i+1)
}

// IsOurs — есть ли в начале файла наша COM-метка. Читает не больше 64 байт.
func IsOurs(path string) bool {
	f, err := imgfile.OpenRegular(path)
	if err != nil {
		return false
	}
	defer func() { _ = f.Close() }()
	head, _ := io.ReadAll(io.LimitReader(f, 64))
	return bytes.Contains(head, []byte(Marker))
}

// Run сохраняет развороты целиком как 01.jpg, 02.jpg... в папку dir.
// progress вызывается после каждого готового разворота.
func Run(ctx context.Context, spreads []render.Spread, opt render.Options, dir string, progress func(done, total int)) error {
	jobs := make([]Job, len(spreads))
	for i := range jobs {
		jobs[i] = Job{Index: i, Whole: true}
	}
	return RunJobs(ctx, spreads, jobs, opt, dir, progress)
}

// CheckCanvas проверяет размер холста разворота до любых действий с файлами.
func CheckCanvas(opt render.Options) error {
	return render.CheckSize(render.CanvasSize(opt))
}

// saveJPEG пишет JPEG с отметкой DPI: стандартный кодировщик Go её не ставит,
// а типографии и Photoshop смотрят на неё при открытии файла. После JFIF идут
// COM-метка программы и ICC sRGB (APP2): печать ждёт профиль, а метка нужна,
// чтобы повторный экспорт не принял за свои чужие 01.jpg.
func saveJPEG(img image.Image, path string, opt render.Options) error {
	var buf bytes.Buffer
	q := opt.Quality
	if q == 0 {
		q = 95
	}
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: q}); err != nil {
		return err
	}
	data := buf.Bytes()
	app0 := []byte{0xFF, 0xE0, 0, 16, 'J', 'F', 'I', 'F', 0, 1, 1, 1, 0, 0, 0, 0, 0, 0}
	dpi := uint16(min(max(opt.DPI, 0), math.MaxUint16))
	binary.BigEndian.PutUint16(app0[12:], dpi)
	binary.BigEndian.PutUint16(app0[14:], dpi)
	// COM: FF FE, длина (включает сами 2 байта), текст.
	com := binary.BigEndian.AppendUint16([]byte{0xFF, 0xFE}, uint16(2+len(Marker)))
	com = append(com, Marker...)
	icc := colorspace.SRGBSegment()
	out := make([]byte, 0, len(data)+len(app0)+len(com)+len(icc))
	out = append(out, data[:2]...) // SOI
	out = append(out, app0...)
	out = append(out, com...)
	out = append(out, icc...)
	out = append(out, data[2:]...)
	return library.WriteFileAtomic(path, out)
}
