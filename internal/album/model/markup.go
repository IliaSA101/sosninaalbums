package model

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"path/filepath"

	"github.com/IliaSA101/sosninaalbums/internal/library"
)

// Разметка типографии в виде (066 design §4.2): обложка и развороты показывают её одинаково.

// GuideView — разметка листа для интерфейса.
type GuideView struct {
	Name  string      `json:"name"`            // имя файла без папки
	Key   string      `json:"key"`             // отпечаток пути, размера и времени файла: сбивает кэш картинки
	OK    bool        `json:"ok"`              // файл на месте и прочитан
	Image bool        `json:"image"`           // есть картинка для подложки
	Warn  string      `json:"warn,omitempty"`  // размер или пропорции не те
	Note  string      `json:"note,omitempty"`  // почему видны только направляющие
	Lines []GuideLine `json:"lines,omitempty"` // направляющие PSD, мм листа
}

// GuideLine — направляющая PSD: V — вертикальная (x = At), иначе горизонтальная (y = At);
// At — мм от левого или верхнего края листа.
type GuideLine struct {
	V  bool    `json:"v"`
	At float64 `json:"at"`
}

// Куда положена разметка: от этого слова в строках Warn.
const (
	MarkupCover  = "cover"
	MarkupSpread = "spread"
	MarkupSheet  = "sheet" // лист виньетки (031)
)

// maxGuideLines — направляющих в виде не больше: в шаблонах их единицы, а файл может дать 1000.
const maxGuideLines = 200

// markupWords — «у обложки» / «с обложкой» и т. д. для строк Warn.
func markupWords(where string) (at, with string) {
	switch where {
	case MarkupCover:
		return "у обложки", "с обложкой"
	case MarkupSheet:
		return "у листа", "с листом"
	}
	return "у разворота", "с разворотом"
}

// MarkupView — вид разметки path на листе sheetW×sheetH мм; info и ok — что Editor прочитал
// (ok=false — файла нет или не читается: только имя). Картинка растянута на лист, поэтому
// линии считаются от размера файла, а не от DPI: они совпадают с картинкой и при неверном
// размере. Warn: у PSD с DPI — размер в мм с допуском 1 мм, иначе пропорции с допуском 1 %.
func MarkupView(path string, info library.MarkupInfo, ok bool, sheetW, sheetH float64, where string) *GuideView {
	if path == "" {
		return nil
	}
	gv := &GuideView{Name: filepath.Base(path), Key: guideKey(path, info, ok)}
	if !ok || info.W <= 0 || info.H <= 0 || sheetW <= 0 || sheetH <= 0 {
		return gv
	}
	gv.OK, gv.Image, gv.Note = true, info.Image, info.Note
	gv.Warn = markupWarn(info, sheetW, sheetH, where)
	for _, g := range info.Guides {
		if len(gv.Lines) == maxGuideLines {
			break
		}
		at := g.Pos / float64(info.W) * sheetW
		if g.Horz {
			at = g.Pos / float64(info.H) * sheetH
		}
		gv.Lines = append(gv.Lines, GuideLine{V: !g.Horz, At: math.Round(at*100) / 100})
	}
	return gv
}

// guideKey — короткий отпечаток файла разметки: путь не уходит в интерфейс открыто, а смена
// файла (другая папка при том же имени, перезапись) даёт новый ключ. Не прочитан — по одному пути.
func guideKey(path string, info library.MarkupInfo, ok bool) string {
	size, mod := int64(0), int64(0)
	if ok {
		size, mod = info.Size, info.Mod
	}
	sum := sha256.Sum256(fmt.Appendf(nil, "%s|%d|%d", path, size, mod))
	return hex.EncodeToString(sum[:6])
}

// markupWarn — строка о неверном размере разметки или "".
func markupWarn(info library.MarkupInfo, sheetW, sheetH float64, where string) string {
	at, with := markupWords(where)
	if info.DPI > 0 {
		w, h := float64(info.W)/info.DPI*25.4, float64(info.H)/info.DPI*25.4
		if math.Abs(w-sheetW) > 1 || math.Abs(h-sheetH) > 1 {
			return "Размер разметки " + MmText(w) + " × " + MmText(h) + " мм — " + at + " " +
				MmText(sheetW) + " × " + MmText(sheetH) + " мм"
		}
		return ""
	}
	if math.Abs(float64(info.W)/float64(info.H)/(sheetW/sheetH)-1) > 0.01 {
		return "Пропорции разметки не совпадают " + with
	}
	return ""
}
