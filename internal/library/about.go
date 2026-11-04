package library

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/imgfile"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// About — сведения о файле фото для карточки (084): готовые строки по-русски.
type About struct {
	File string   // «6000 × 4000 px · 24 Мп · 12,3 МБ»; "" — размеры неизвестны
	Shot []string // по порядку: камера · объектив, параметры, «Снято …»; пусто — нет данных
}

// Пределы правдоподобия: мусорный EXIF не должен давать «ISO 4294967295» или «f/4000000».
const (
	aboutMaxFocal = 10_000.0 // мм
	aboutMaxF     = 1_000.0
	aboutMaxSec   = 100_000.0
)

// ReadAbout читает файл один раз: размеры по заголовку (только JPEG и PNG, imgfile.DimsOf), затем
// заголовок JPEG — ориентация и съёмка. Путь из .salbum недоверенный: нелокальный (UNC, поток NTFS)
// — отказ до открытия (утечка NTLM). Пробу тома (мёртвый сетевой диск) делает App до вызова.
// Ошибка открытия или не-картинка — ошибка (App превращает её в «Файл не найден»).
func ReadAbout(path string) (About, error) {
	if err := paths.CheckLocalPath(path); err != nil {
		return About{}, err
	}
	f, err := imgfile.OpenRegular(path)
	if err != nil {
		return About{}, err
	}
	defer f.Close() //nolint:errcheck // чтение: ошибка закрытия ничего не теряет
	st, err := f.Stat()
	if err != nil {
		return About{}, err
	}
	d, err := imgfile.DimsOf(f, 0)
	if err != nil {
		return About{}, err
	}
	head, shot := imgfile.Head{Orient: 1}, imgfile.Shot{}
	if _, err := f.Seek(0, 0); err == nil {
		head, shot = imgfile.HeaderShot(f)
	}
	w, h := d.W, d.H
	if head.Orient >= 5 {
		w, h = h, w
	}
	return About{File: FileLine(w, h, st.Size()), Shot: ShotLines(shot)}, nil
}

// ShotLines — строки блока «О снимке»: камера · объектив, параметры, «Снято …». Нет данных — nil.
func ShotLines(s imgfile.Shot) []string {
	var out []string
	if c := cameraLine(s); c != "" {
		out = append(out, c)
	}
	if p := paramsLine(s); p != "" {
		out = append(out, p)
	}
	if s.Taken > 0 {
		// Taken — «стенное» время камеры как UTC: в местном поясе оно сдвинулось бы.
		out = append(out, "Снято "+time.Unix(s.Taken, 0).UTC().Format("02.01.2006, 15:04"))
	}
	return out
}

// cameraLine — «Canon EOS R6 · RF24-70mm F2.8 L IS USM»; нет ни камеры, ни объектива — "".
func cameraLine(s imgfile.Shot) string {
	cam := makeModel(s.Make, s.Model)
	switch {
	case cam == "":
		return s.Lens
	case s.Lens == "":
		return cam
	}
	return cam + " · " + s.Lens
}

// makeModel склеивает производителя и модель без повтора: «Canon» + «Canon EOS R6» → «Canon EOS R6»,
// «NIKON CORPORATION» + «NIKON Z 6_2» → «NIKON Z 6_2» (совпало первое слово).
func makeModel(mk, model string) string {
	mk, model = strings.TrimSpace(mk), strings.TrimSpace(model)
	if mk == "" || model == "" {
		return mk + model
	}
	mw, dw := strings.Fields(mk), strings.Fields(model)
	if strings.EqualFold(mw[0], dw[0]) {
		return model
	}
	return mk + " " + model
}

// paramsLine — «50 мм · f/2,8 · 1/200 с · ISO 400»; выпавшие параметры пропускаются.
func paramsLine(s imgfile.Shot) string {
	var parts []string
	if v, ok := ratio(s.Focal, aboutMaxFocal); ok {
		parts = append(parts, num1(v)+" мм")
	}
	if v, ok := ratio(s.FNumber, aboutMaxF); ok {
		parts = append(parts, "f/"+num1(v))
	}
	if v, ok := ratio(s.Exposure, aboutMaxSec); ok {
		parts = append(parts, exposureText(v, s.Exposure))
	}
	if s.ISO > 0 {
		parts = append(parts, "ISO "+strconv.Itoa(s.ISO))
	}
	return strings.Join(parts, " · ")
}

// ratio — дробь как число; ok = false для нуля, нулевого знаменателя и значений за пределом.
func ratio(r [2]uint32, limit float64) (float64, bool) {
	if r[0] == 0 || r[1] == 0 {
		return 0, false
	}
	v := float64(r[0]) / float64(r[1])
	return v, v <= limit
}

// exposureText — быстрая выдержка дробью «1/200 с», от полусекунды — секундами «0,5 с», «2 с».
func exposureText(v float64, r [2]uint32) string {
	if v < 0.5 {
		return fmt.Sprintf("1/%d с", int64(math.Round(float64(r[1])/float64(r[0]))))
	}
	return num1(v) + " с"
}

// num1 — число до одного знака с десятичной запятой, без «,0»: 50 → «50», 2.8 → «2,8».
func num1(v float64) string {
	s := strconv.FormatFloat(v, 'f', 1, 64)
	s = strings.TrimSuffix(s, ".0")
	return strings.Replace(s, ".", ",", 1)
}

// FileLine — «6000 × 4000 px · 24 Мп · 12,3 МБ»; размеры неизвестны — "". Мп: от 10 целые,
// меньше — один знак; размер: от 1 МБ — «12,3 МБ», меньше — «850 КБ».
func FileLine(w, h int, size int64) string {
	if w <= 0 || h <= 0 {
		return ""
	}
	mp := float64(w) * float64(h) / 1e6
	mpText := num1(mp)
	if mp < 0.05 {
		mpText = "<0,1"
	}
	if mp >= 10 {
		mpText = strconv.FormatFloat(math.Round(mp), 'f', 0, 64)
	}
	parts := []string{fmt.Sprintf("%d × %d px", w, h), mpText + " Мп"}
	if size > 0 {
		parts = append(parts, sizeText(size))
	}
	return strings.Join(parts, " · ")
}

func sizeText(size int64) string {
	const kb, mb = 1 << 10, 1 << 20
	switch {
	case size >= mb:
		return num1(float64(size)/mb) + " МБ"
	case size >= kb:
		k := int64(math.Round(float64(size) / kb))
		if k >= kb { // 1048500 Б — уже «1 МБ», а не «1024 КБ»
			return "1 МБ"
		}
		return strconv.FormatInt(k, 10) + " КБ"
	}
	return strconv.FormatInt(size, 10) + " Б"
}
