package gen

import (
	"image"
	"math"
	"net/url"
	"strconv"
	"strings"

	"golang.org/x/image/math/f64"
)

// URL превью (design §5.5): строит только Go (View), UI дописывает &px=; разбор — здесь же,
// чтобы /media/gen и View не разошлись. Ответ по URL неизменен (immutable): генератор заморожен.

// Version — параметр v: поднимать при смене формата запроса, не при новых генераторах.
const Version = 1

// Пределы запроса.
const (
	MinPx = 16
	MaxPx = 4096
	// MaxPxPerMM — плотность узора в превью: холст при наибольшем масштабе даёт ≈ 82 px/мм.
	// Выше — только мусорный запрос: мотив в тысячи px и маски бумаги в сотни МБ.
	MaxPxPerMM = 100
)

const (
	patPath = "/media/gen/pat"
	artPath = "/media/gen/art"
)

// Query — URL превью без px: узор области w×h мм (/media/gen/pat) или стикер (/media/gen/art,
// w и h не нужны). Spec — после Clamp: цвета без «#» через запятую, пустое и по умолчанию
// не пишется.
func Query(s Spec, w, h float64) string {
	s = Clamp(s)
	q := []string{"k=" + s.Kind}
	if len(s.Inks) > 0 {
		hex := make([]string, len(s.Inks))
		for i, v := range s.Inks {
			hex[i] = v[1:]
		}
		q = append(q, "i="+strings.Join(hex, ","))
	}
	if s.Scale != 0 {
		q = append(q, "s="+num(s.Scale))
	}
	if s.Seed != 0 {
		q = append(q, "r="+strconv.FormatUint(uint64(s.Seed), 10))
	}
	path := patPath
	if IsArt(s.Kind) {
		path = artPath
		if s.Aspect != 0 {
			q = append(q, "a="+num(s.Aspect))
		}
	} else {
		q = append(q, "w="+num(math.Round(w*100)/100), "h="+num(math.Round(h*100)/100))
	}
	q = append(q, "v="+strconv.Itoa(Version))
	return path + "?" + strings.Join(q, "&")
}

func num(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }

// Request — разобранный запрос /media/gen.
type Request struct {
	Art  bool
	Spec Spec
	W, H float64 // мм области узора
	Px   int     // ширина узора или длинная сторона стикера
}

// Parse разбирает путь и строку запроса /media/gen. ok = false (404): чужой путь, лишний или
// повторённый параметр, нет обязательного, число не читается, неизвестный генератор, px вне
// 16..4096, картинка выше 4096 px, у узора плотнее MaxPxPerMM. Значения в пределах — через Clamp.
func Parse(path, rawQuery string) (Request, bool) {
	var req Request
	allowed := map[string]bool{"k": true, "i": true, "s": true, "r": true, "px": true, "v": true}
	switch path {
	case patPath:
		allowed["w"], allowed["h"] = true, true
	case artPath:
		req.Art = true
		allowed["a"] = true
	default:
		return req, false
	}
	vals, err := url.ParseQuery(rawQuery)
	if err != nil {
		return req, false
	}
	for k, v := range vals {
		if !allowed[k] || len(v) != 1 {
			return req, false
		}
	}
	get := func(k string) (string, bool) {
		v, ok := vals[k]
		if !ok {
			return "", false
		}
		return v[0], true
	}
	k, _ := get("k")
	if !Known(k) || IsArt(k) != req.Art {
		return req, false
	}
	req.Spec.Kind = k
	if !parseFields(get, &req) {
		return req, false
	}
	req.Spec = Clamp(req.Spec)
	if !req.Art && float64(req.Px) > MaxPxPerMM*req.W {
		return req, false
	}
	w, h := req.Size()
	return req, w > 0 && h > 0 && w <= MaxPx && h <= MaxPx
}

// parseFields читает i, s, r, a, w, h, px, v.
func parseFields(get func(string) (string, bool), req *Request) bool {
	if v, ok := get("i"); ok {
		for _, c := range strings.Split(v, ",") {
			ink, good := normInk("#" + c)
			if !good || len(req.Spec.Inks) == MaxInks {
				return false
			}
			req.Spec.Inks = append(req.Spec.Inks, ink)
		}
	}
	okNum := true
	float := func(k string, dst *float64, lo, hi float64) {
		v, ok := get(k)
		if !ok {
			return
		}
		f, err := strconv.ParseFloat(v, 64)
		if err != nil || !(f >= lo && f <= hi) {
			okNum = false
		}
		*dst = f
	}
	float("s", &req.Spec.Scale, 0, 1e6)
	float("a", &req.Spec.Aspect, 0, 1e6)
	if !req.Art {
		if _, ok := get("w"); !ok {
			return false
		}
		if _, ok := get("h"); !ok {
			return false
		}
		float("w", &req.W, 0.01, maxAreaMM)
		float("h", &req.H, 0.01, maxAreaMM)
	}
	if v, ok := get("r"); ok {
		r, err := strconv.ParseUint(v, 10, 32)
		if err != nil {
			return false
		}
		req.Spec.Seed = uint32(r)
	}
	px, okPx := get("px")
	v, okV := get("v")
	n, err := strconv.Atoi(px)
	if _, errV := strconv.ParseUint(v, 10, 16); !okPx || !okV || err != nil || errV != nil || n < MinPx || n > MaxPx {
		return false
	}
	req.Px = n
	return okNum
}

// Size — размер картинки ответа в px: узор — Px × Px·H/W, стикер — длинная сторона Px.
func (r Request) Size() (int, int) {
	if !r.Art {
		if !(r.W > 0) {
			return 0, 0
		}
		return r.Px, max(1, int(math.Round(float64(r.Px)*r.H/r.W)))
	}
	s, _ := Resolve(r.Spec)
	a := s.Aspect
	if !(a > 0) {
		a = 1
	}
	if a >= 1 {
		return r.Px, max(1, int(math.Round(float64(r.Px)/a)))
	}
	return max(1, int(math.Round(float64(r.Px)*a))), r.Px
}

// Draw рисует ответ: прозрачный холст Size с узором или стикером.
func (r Request) Draw() *image.RGBA {
	w, h := r.Size()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	if r.Art {
		DrawArt(img, f64.Aff3{float64(w), 0, 0, 0, float64(h), 0}, r.Spec, 0)
	} else {
		DrawPattern(img, img.Rect, r.Spec, float64(w)/r.W, 0)
	}
	return img
}
