package sheetcut

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// StickerName — имя файла стикера: NN.png (две цифры, при больше 99 — шире).
func StickerName(index int) string { return fmt.Sprintf("%02d.png", index) }

const (
	PreviewDark  = "preview-dark.png"
	PreviewColor = "preview-color.png"
	previewCell  = 360 // стикер в ячейке контактного листа не больше этого, px
	previewGap   = 10  // поле вокруг ячейки
)

// Фоны контактных листов.
var (
	darkBG  = color.NRGBA{0x1e, 0x1e, 0x22, 0xff}
	colorBG = color.NRGBA{0x4a, 0x7a, 0x6e, 0xff}
)

// stickerFile — имя файла, которое мог оставить прошлый прогон (StickerName: две цифры и больше).
var stickerFile = regexp.MustCompile(`^(\d{2,})\.png$`)

// removeStale удаляет в dir остатки прошлого прогона: NN.png с номером больше числа стикеров.
// Остальные файлы (включая NN.png в пределах числа стикеров — их перепишет Save) не трогает.
func removeStale(dir string, count int) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var removed []string
	for _, e := range entries {
		m := stickerFile.FindStringSubmatch(e.Name())
		if m == nil || e.IsDir() {
			continue
		}
		if n, err := strconv.Atoi(m[1]); err != nil || n <= count {
			continue
		}
		if err := os.Remove(filepath.Join(dir, e.Name())); err != nil {
			return removed, err
		}
		removed = append(removed, e.Name())
	}
	return removed, nil
}

// Save пишет стикеры в папку dir (NN.png) и дописывает в Sticker.Bytes размер файла; больше
// MaxFileBytes — предупреждение (библиотека не примет). Перед записью удаляет остатки прошлого
// прогона (NN.png с номером больше числа стикеров) и отмечает их в Result.Notes; другие файлы
// папки не трогает.
func Save(res *Result, dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	removed, err := removeStale(dir, len(res.Stickers))
	if err != nil {
		return err
	}
	if len(removed) > 0 {
		res.Notes = append(res.Notes, "удалены остатки прошлого прогона: "+strings.Join(removed, ", "))
	}
	for i := range res.Stickers {
		s := &res.Stickers[i]
		var buf bytes.Buffer
		if err := png.Encode(&buf, s.Img); err != nil {
			return err
		}
		s.Bytes = buf.Len()
		if s.Bytes > MaxFileBytes {
			s.Warnings = append(s.Warnings, fmt.Sprintf("файл %d МБ больше предела набора %d МБ", s.Bytes>>20, MaxFileBytes>>20))
		}
		if err := os.WriteFile(filepath.Join(dir, StickerName(s.Index)), buf.Bytes(), 0o644); err != nil {
			return err
		}
	}
	return nil
}

// SavePreview пишет два контактных листа: на тёмном и на цветном фоне.
func SavePreview(res *Result, dir string, cols int) error {
	for name, bg := range map[string]color.NRGBA{PreviewDark: darkBG, PreviewColor: colorBG} {
		var buf bytes.Buffer
		if err := png.Encode(&buf, contactSheet(res.Stickers, cols, bg)); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, name), buf.Bytes(), 0o644); err != nil {
			return err
		}
	}
	return nil
}

// contactSheet — стикеры в ряд по cols колонок (0 — 4), каждый вписан в ячейку previewCell.
func contactSheet(stickers []Sticker, cols int, bg color.NRGBA) *image.NRGBA {
	if cols <= 0 {
		cols = 4
	}
	cols = max(1, min(cols, len(stickers)))
	rows := (len(stickers) + cols - 1) / cols
	cell := previewCell + 2*previewGap
	sheet := image.NewNRGBA(image.Rect(0, 0, cols*cell, max(1, rows)*cell))
	draw.Draw(sheet, sheet.Bounds(), image.NewUniform(bg), image.Point{}, draw.Src)
	for i, s := range stickers {
		nw, nh := fitSide(s.W, s.H, previewCell)
		small := scaleDown(s.Img, nw, nh)
		x := (i%cols)*cell + (cell-nw)/2
		y := (i/cols)*cell + (cell-nh)/2
		draw.Draw(sheet, image.Rect(x, y, x+nw, y+nh), small, image.Point{}, draw.Over)
	}
	return sheet
}

// JSON — отчёт в JSON (без картинок).
func (r *Result) JSON() ([]byte, error) { return json.MarshalIndent(r, "", "  ") }

// Text — отчёт для человека: по строке на стикер, затем замечания.
func (r *Result) Text() string {
	var b strings.Builder
	fmt.Fprintf(&b, "лист %d×%d, фон: %s, областей %d, мусора %d, стикеров %d\n", r.Sheet[0], r.Sheet[1], r.BG, r.Regions, r.Noise, len(r.Stickers))
	if r.BGInfo != nil {
		fmt.Fprintf(&b, "своё снятие фона: замкнутых белых областей снято %d, оставлено %d\n", r.BGInfo.Holes, r.BGInfo.HolesKept)
	}
	for _, s := range r.Stickers {
		fmt.Fprintf(&b, "%s  %d×%d px  %.1f×%.1f см@300dpi  кусков %d", StickerName(s.Index), s.W, s.H, s.CmW, s.CmH, s.Pieces)
		if s.Bytes > 0 {
			fmt.Fprintf(&b, "  %d КБ", s.Bytes>>10)
		}
		b.WriteByte('\n')
		for _, w := range s.Warnings {
			fmt.Fprintf(&b, "    ! %s\n", w)
		}
	}
	for _, n := range r.Notes {
		fmt.Fprintf(&b, "! %s\n", n)
	}
	return b.String()
}
