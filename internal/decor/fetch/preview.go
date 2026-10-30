package fetch

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
)

// Образцы наборов (спека 144, Г1): необязательное поле preview строки каталога — лист-картинка с
// клетками предметов. Формат каталога остаётся 1: старые версии поле пропускают. Поле разбирается
// отдельно от строки (security Н1): ошибка типа в нём не роняет ни строку, ни каталог.

// Пределы поля preview (design §2); лист — один JPEG.
const (
	// MaxPreview — размер листа в байтах.
	MaxPreview = 384 << 10
	// MaxPreviewRaw — сырое поле preview в каталоге; длиннее — отказ до разбора JSON.
	MaxPreviewRaw = 8 << 10

	maxPreviewW     = 4096
	maxPreviewH     = 2048
	maxPreviewArea  = 4 << 20 // пикселей листа: оценка памяти разбора JPEG
	maxPreviewRows  = 5
	minPreviewCell  = 32
	maxPreviewCell  = 480
	maxPreviewItems = 12
)

// Preview — лист образцов набора: адрес и sha256 JPEG, его размер, размеры листа и ряды клеток.
type Preview struct {
	URL    string       `json:"url"`
	SHA256 string       `json:"sha256"`
	Size   int64        `json:"size"`
	W      int          `json:"w"`
	H      int          `json:"h"`
	Rows   []PreviewRow `json:"rows"`
}

// PreviewRow — ряд клеток одного вида: размер клетки и предметы (sha256[:16] записи манифеста).
// Ряд i лежит на листе с y = Σ h предыдущих рядов, клетка j — x = j·w.
type PreviewRow struct {
	Kind  string   `json:"kind"`
	W     int      `json:"w"`
	H     int      `json:"h"`
	Items []string `json:"items"`
}

// previewKinds — виды рядов; другие (каталог новее программы) — отказ всего поля.
var previewKinds = map[string]bool{"bg": true, "sticker": true, "frame": true, "spread": true, "album": true}

// parsePreview — поле preview строки: пусто и null — образцов нет (p == nil, why == ""); негодное —
// p == nil и причина (без чужого текста, в Skipped уходит как есть). Размер сырого поля проверяется до
// Unmarshal, поэтому счётчики внутри (ряды, предметы) памяти не растят.
func parsePreview(raw json.RawMessage, src Source) (p *Preview, why string) {
	if len(raw) > MaxPreviewRaw {
		return nil, "превью: поле больше 8 КиБ"
	}
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return nil, ""
	}
	var v Preview
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, "превью: поле не читается"
	}
	if err := checkPreview(&v, src); err != nil {
		return nil, "превью: " + err.Error()
	}
	return &v, ""
}

// checkPreview — пределы design §2; ошибка называет правило, не значение.
func checkPreview(v *Preview, src Source) error {
	switch {
	case v.Size < 1 || v.Size > MaxPreview:
		return errors.New("размер файла вне предела")
	case !validSum(v.SHA256):
		return errors.New("нет sha256")
	case !src.assetURL(v.URL):
		return errors.New("адрес не из библиотеки")
	case v.W < 1 || v.W > maxPreviewW || v.H < 1 || v.H > maxPreviewH || v.W*v.H > maxPreviewArea:
		return errors.New("размер листа вне предела")
	case len(v.Rows) < 1 || len(v.Rows) > maxPreviewRows:
		return errors.New("рядов вне 1…5")
	}
	kinds, sumH := map[string]bool{}, 0
	for _, r := range v.Rows {
		switch {
		case !previewKinds[r.Kind]:
			return errors.New("неизвестный вид ряда")
		case kinds[r.Kind]:
			return errors.New("повтор вида ряда")
		case r.W < minPreviewCell || r.W > maxPreviewCell || r.H < minPreviewCell || r.H > maxPreviewCell:
			return errors.New("клетка вне 32…480 px")
		case len(r.Items) < 1 || len(r.Items) > maxPreviewItems:
			return errors.New("предметов в ряду вне 1…12")
		case len(r.Items)*r.W > v.W:
			return errors.New("ряд шире листа")
		}
		kinds[r.Kind] = true
		if sumH += r.H; sumH > v.H {
			return errors.New("ряды выше листа")
		}
		seen := map[string]bool{}
		for _, it := range r.Items {
			if !validItem(it) || seen[it] {
				return errors.New("предмет не 16 hex или повтор")
			}
			seen[it] = true
		}
	}
	return nil
}

// validItem — sha256[:16] предмета: ровно 16 строчных hex.
func validItem(s string) bool {
	return len(s) == 16 && strings.Trim(s, "0123456789abcdef") == ""
}
