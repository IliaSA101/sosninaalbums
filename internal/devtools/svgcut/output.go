package svgcut

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// Имена файлов выхода.
const (
	OverlayName = "whites.png" // лист с подсветкой белых путей и рамок групп
	MaxFileSize = 30 << 20     // предел файла набора (scripts/library-pack.mjs, kind sticker)
)

// ItemName — имя файла предмета: NN.png (две цифры, при больше 99 — шире).
func ItemName(index int) string { return fmt.Sprintf("%02d.png", index) }

var itemFile = regexp.MustCompile(`^(\d{2,})\.png$`)

// removeStale удаляет в dir остатки прошлого прогона: NN.png с номером больше числа предметов.
func removeStale(dir string, count int) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var removed []string
	for _, e := range entries {
		m := itemFile.FindStringSubmatch(e.Name())
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

// Save пишет предметы в dir (NN.png) и лист подсветки; удаляет остатки прошлого прогона (NN.png с
// номером больше числа предметов), прочие файлы не трогает. Пустой dir — ошибка, а не текущая папка.
func Save(res *Result, dir string) error {
	if strings.TrimSpace(dir) == "" {
		return errors.New("svgcut: пустая папка вывода")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	removed, err := removeStale(dir, len(res.Items))
	if err != nil {
		return err
	}
	if len(removed) > 0 {
		res.Notes = append(res.Notes, "удалены остатки прошлого прогона: "+strings.Join(removed, ", "))
	}
	for i := range res.Items {
		it := &res.Items[i]
		if it.Img == nil {
			continue
		}
		var buf bytes.Buffer
		if err := png.Encode(&buf, it.Img); err != nil {
			return err
		}
		it.Bytes = buf.Len()
		if it.Bytes > MaxFileSize {
			it.Warnings = append(it.Warnings, fmt.Sprintf("файл %d МБ больше предела набора %d МБ", it.Bytes>>20, MaxFileSize>>20))
		}
		if err := os.WriteFile(filepath.Join(dir, ItemName(it.Index)), buf.Bytes(), 0o644); err != nil {
			return err
		}
	}
	if res.Overlay != nil {
		var buf bytes.Buffer
		if err := png.Encode(&buf, res.Overlay); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dir, OverlayName), buf.Bytes(), 0o644)
	}
	return nil
}

// JSON — отчёт в JSON (без картинок).
func (r *Result) JSON() ([]byte, error) { return json.MarshalIndent(r, "", "  ") }

// Text — отчёт для человека: по строке на предмет с числом белых путей, затем замечания.
func (r *Result) Text() string {
	var b strings.Builder
	fmt.Fprintf(&b, "лист %g×%g, лист-фон %s, путей %d (белых %d), групп %d, предметов %d, вырезов %d\n",
		r.Sheet[0], r.Sheet[1], r.SheetFill, r.Paths, r.WhiteTotal, r.Groups, len(r.Items), len(r.Knocks))
	for _, it := range r.Items {
		knocked := 0
		for _, w := range it.Whites {
			if w.Knocked {
				knocked++
			}
		}
		fmt.Fprintf(&b, "%s  гр.%d  %d×%d px  %.1f×%.1f см@300dpi  путей %d  белых %d (вырезано %d)",
			ItemName(it.Index), it.Base, it.W, it.H, it.CmW, it.CmH, it.Paths, len(it.Whites), knocked)
		if it.Bytes > 0 {
			fmt.Fprintf(&b, "  %d КБ", it.Bytes>>10)
		}
		b.WriteByte('\n')
		for _, w := range it.Whites {
			mark := "оставлен"
			if w.Knocked {
				mark = "вырез"
			}
			fmt.Fprintf(&b, "    w%d путь %d (%.0f, %.0f)–(%.0f, %.0f): %s\n", w.N, w.Path, w.Box[0], w.Box[1], w.Box[2], w.Box[3], mark)
		}
		for _, w := range it.Warnings {
			fmt.Fprintf(&b, "    ! %s\n", w)
		}
	}
	for _, n := range r.Notes {
		fmt.Fprintf(&b, "! %s\n", n)
	}
	return b.String()
}
