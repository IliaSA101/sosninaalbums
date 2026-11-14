package fetch

import (
	"bytes"
	"encoding/json"
	"strconv"
)

// Ролики справочника F1 (спека 140, C): поле videos в теле подписанного каталога. Формат остаётся 1:
// старые версии программы поле пропускают (неизвестные поля ParseCatalog не читает). Строки роликов
// разбираются по одной и не роняют наборы: битая строка — в Skipped, а не ErrCatalog (security Н5).

const (
	// MaxVideos — строк videos; длиннее — роликов нет совсем (наборы целы).
	MaxVideos = 100
	// MaxVideoFile — размер одного файла ролика (замер: 52 с 1080p VP9 — 0,86 МБ).
	MaxVideoFile = 16 << 20
	// maxVideoKinds — ключей в files: видов два, остальное — каталог новее программы или мусор.
	maxVideoKinds = 16
)

// Виды файла ролика: единственные ключи files, которые берутся.
const (
	KindWebM = "webm"
	KindMP4  = "mp4"
)

// Video — ролик справочника: статья id, версия съёмки, с какой программы играет, длина, файлы по
// видам (webm и/или mp4). Текстов из каталога нет: название ролика — заголовок локальной статьи.
type Video struct {
	ID      string               `json:"id"`
	Version int                  `json:"version"` // 1…999999; переснятый ролик — +1
	MinApp  string               `json:"minApp"`  // x.y.z
	Seconds int                  `json:"seconds"` // 1…600
	Files   map[string]VideoFile `json:"files"`
}

// VideoFile — файл ролика: размер, sha256 и адрес (src.assetURL).
type VideoFile struct {
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
	URL    string `json:"url"`
}

// NeedsApp — ролик играет только в программе новее app (x.y.z); другая строка (сборка dev) — нет.
func (v Video) NeedsApp(app string) bool { return needsApp(v.MinApp, app) }

// parseVideos — поле videos каталога: годные строки и пропущенные с причиной. Не список или
// длиннее MaxVideos — ни одной строки и одна запись в Skipped. Число строк считается до накопления:
// список из сотен тысяч «1,1,1» память не растит.
func parseVideos(raw json.RawMessage, src Source) ([]Video, []Skip) {
	out, skipped := []Video{}, []Skip{}
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return out, skipped
	}
	rows, ok := videoRows(raw)
	if !ok {
		return out, append(skipped, Skip{ID: "videos", Why: "ролики: не список или строк больше " + strconv.Itoa(MaxVideos)})
	}
	seen := map[string]bool{}
	for i, row := range rows {
		v, why := videoRow(row, src)
		key := v.ID + "/" + strconv.Itoa(v.Version)
		if why == "" && seen[key] {
			why = "повтор id и версии"
		}
		if why != "" {
			id := "#" + strconv.Itoa(i+1)
			if ValidID(v.ID) {
				id = v.ID
			}
			skipped = append(skipped, Skip{ID: "ролик " + id, Why: why})
			continue
		}
		seen[key] = true
		out = append(out, v)
	}
	return out, skipped
}

// videoRows — строки массива по одной; не массив или больше MaxVideos — ok == false.
func videoRows(raw json.RawMessage) (rows []json.RawMessage, ok bool) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	if t, err := dec.Token(); err != nil || t != json.Delim('[') {
		return nil, false
	}
	for dec.More() {
		if len(rows) >= MaxVideos {
			return nil, false
		}
		var row json.RawMessage
		if dec.Decode(&row) != nil {
			return nil, false
		}
		rows = append(rows, row)
	}
	return rows, true
}

// videoRow — одна строка: годная (why == "") или причина пропуска. Берутся только виды webm и mp4,
// прочие ключи files (каталог новее) отбрасываются; годный из двух файлов не спасает строку с
// негодным другим.
func videoRow(raw json.RawMessage, src Source) (v Video, why string) {
	if err := json.Unmarshal(raw, &v); err != nil {
		return Video{}, "строка не читается"
	}
	switch {
	case !ValidID(v.ID):
		return v, "недопустимый id"
	case v.Version < 1 || v.Version > 999999:
		return v, "версия вне 1…999999"
	case !validVer(v.MinApp):
		return v, "minApp не x.y.z"
	case v.Seconds < 1 || v.Seconds > 600:
		return v, "длина вне 1…600 с"
	case len(v.Files) > maxVideoKinds:
		return v, "слишком много видов файла"
	}
	files := map[string]VideoFile{}
	for _, kind := range []string{KindWebM, KindMP4} {
		f, ok := v.Files[kind]
		if !ok {
			continue
		}
		switch {
		case f.Size < 1 || f.Size > MaxVideoFile:
			return v, kind + ": размер вне предела"
		case !validSum(f.SHA256):
			return v, kind + ": нет sha256"
		case !src.assetURL(f.URL):
			return v, kind + ": адрес не из библиотеки"
		}
		files[kind] = f
	}
	if len(files) == 0 {
		return v, "нет файлов webm или mp4"
	}
	v.Files = files
	return v, ""
}
