package printprof

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// Слои реестра (123 Б1, design §4): встроенные ← библиотека (набор из подписанного каталога) ←
// свои. Встроенную перекрывает только библиотечный профиль с версией строго больше, свои живут
// в пространстве имён own-… и встроенную не заменяют: альбом выбирает свою явно.

// Источник профиля в снимке.
const (
	SourceBuiltin = "builtin"
	SourceLibrary = "library"
	SourceOwn     = "own"
)

// OwnPrefix — начало id своей типографии и её изделий.
const OwnPrefix = "own-"

// MaxOwn — своих типографий: при старте читается ≤ MaxOwn × MaxSize.
const MaxOwn = 64

var ownRe = regexp.MustCompile(`^own-[0-9a-f]{8}$`)

// ValidOwnID — id своей типографии: own-<8 hex>.
func ValidOwnID(s string) bool { return ownRe.MatchString(s) }

// OwnProductID — id n-го изделия своей типографии: own-<8 hex>-<n>.
func OwnProductID(id string, n int) string { return id + "-" + strconv.Itoa(n) }

// NewOwnID — новый свободный id своей типографии (taken — занят ли); случайный, чтобы файл от
// другого человека не совпал с чужим по умолчанию.
func NewOwnID(taken func(id string) bool) string {
	for {
		var b [4]byte
		_, _ = rand.Read(b[:]) // с Go 1.24 Read не возвращает ошибку (при сбое ОС процесс завершается сам)
		if id := OwnPrefix + hex.EncodeToString(b[:]); !taken(id) {
			return id
		}
	}
}

type layered struct {
	p   Profile
	src string
	own int // номер в списке own у Compose; -1 — не своя
}

// Compose собирает снимок слоёв: встроенные, затем lib и own (каждый список в своём порядке).
// Профиль, который не проходит, пропускается со строкой в skipped — остальные работают:
// битый или спорный файл не должен ронять типографии.
func Compose(lib, own []Profile) (snap *Snapshot, skipped []string) {
	var out []layered
	for _, p := range Builtin().Profiles() {
		out = append(out, layered{p, SourceBuiltin, -1})
	}
	skip := func(p Profile, why string) {
		skipped = append(skipped, fmt.Sprintf("типография %s пропущена: %s", q(p.ID), why))
	}
	for _, p := range lib {
		switch i := slices.IndexFunc(out, func(l layered) bool { return l.p.ID == p.ID }); {
		case p.Validate() != nil:
			skip(p, "профиль негоден")
		case strings.HasPrefix(p.ID, OwnPrefix):
			skip(p, "id "+OwnPrefix+"… только у своих")
		case i >= 0 && p.Version <= out[i].p.Version: // равная версия — прежняя действует (из двух наборов первый)
			skip(p, "версия не новее уже действующей")
		case i >= 0:
			if why := clash(out, p, i); why != "" {
				skip(p, why)
				continue
			}
			out[i] = layered{p, SourceLibrary, -1}
		default:
			if why := clash(out, p, -1); why != "" {
				skip(p, why)
				continue
			}
			out = append(out, layered{p, SourceLibrary, -1})
		}
	}
	for n, p := range own {
		switch why := clash(out, p, -1); {
		case p.Validate() != nil:
			skip(p, "профиль негоден")
		case !ValidOwnID(p.ID) || !ownProducts(p):
			skip(p, "у своей id вида "+OwnPrefix+"<8 знаков>, у изделий — "+OwnPrefix+"<8 знаков>-<n>")
		case countSrc(out, SourceOwn) >= MaxOwn:
			skip(p, fmt.Sprintf("своих больше %d", MaxOwn))
		case why != "":
			skip(p, why)
		default:
			out = append(out, layered{p, SourceOwn, n})
		}
	}
	ps := make([]Profile, len(out))
	src, owns := make(map[string]string, len(out)), map[string]int{}
	for i, l := range out {
		ps[i], src[l.p.ID] = l.p.Clone(), l.src
		if l.own >= 0 {
			owns[l.p.ID] = l.own
		}
	}
	// Validate и clash уже отсекли всё, что отвергла бы NewSnapshot (тест сверяет это заново),
	// поэтому снимок собираем напрямую: ветка отказа тут была бы недостижимой.
	return &Snapshot{profiles: ps, src: src, own: owns}, skipped
}

func countSrc(l []layered, src string) int {
	n := 0
	for _, x := range l {
		if x.src == src {
			n++
		}
	}
	return n
}

// ownProducts — изделия своей: own-<id>-<n>, n — число.
func ownProducts(p Profile) bool {
	for _, pr := range p.Products {
		n, ok := strings.CutPrefix(pr.ID, p.ID+"-")
		if _, err := strconv.Atoi(n); !ok || err != nil || strings.ContainsAny(n, "+-") {
			return false
		}
	}
	return true
}

// clash — чем p спорит с профилями out (кроме слота skip): id, название, изделия. "" — ничем.
func clash(out []layered, p Profile, skip int) string {
	for i, l := range out {
		if i == skip {
			continue
		}
		switch {
		case l.p.ID == p.ID:
			return "id уже занят"
		case SameName(l.p.Name, p.Name): // в log.txt — id, не название (аудит 123 №7)
			return "название уже есть у типографии " + q(l.p.ID)
		}
		for _, pr := range p.Products {
			if slices.ContainsFunc(l.p.Products, func(o Product) bool { return o.ID == pr.ID }) {
				return "изделие " + q(pr.ID) + " уже есть у другой типографии"
			}
		}
	}
	return ""
}

// OwnIndex — номер принятой своей id в списке own, переданном Compose: у двух файлов с одним id
// ключ в хранилище — только у принятого (аудит 123 №6). ok=false — id не своя или отвергнута.
func (s *Snapshot) OwnIndex(id string) (int, bool) {
	n, ok := s.own[id]
	return n, ok
}

// Source — слой профиля id; снимок из NewSnapshot — целиком встроенный.
func (s *Snapshot) Source(id string) string {
	if v, ok := s.src[id]; ok {
		return v
	}
	return SourceBuiltin
}

// Info — строка списка типографий для интерфейса.
type Info struct {
	ID        string       `json:"id"`
	Name      string       `json:"name"`
	Source    string       `json:"source"` // builtin | library | own
	Version   int          `json:"version"`
	HasExport bool         `json:"hasExport"`
	Formats   []InfoFormat `json:"formats"`
}

// InfoFormat — кнопка формата профиля: страница в мм и px цели, если задана.
type InfoFormat struct {
	Name  string  `json:"name"`
	PageW float64 `json:"pageW"`
	PageH float64 `json:"pageH"`
	PxW   int     `json:"pxW,omitempty"`
	PxH   int     `json:"pxH,omitempty"`
}

// InfoOf — строка списка для профиля p слоя src.
func InfoOf(p Profile, src string) Info {
	in := Info{ID: p.ID, Name: p.Name, Source: src, Version: p.Version, HasExport: p.Export != nil, Formats: []InfoFormat{}}
	for _, f := range p.Formats {
		fi := InfoFormat{Name: f.Name, PageW: f.Page[0], PageH: f.Page[1]}
		if f.Px != nil {
			fi.PxW, fi.PxH = f.Px[0], f.Px[1]
		}
		in.Formats = append(in.Formats, fi)
	}
	return in
}

// Infos — строки списка по порядку снимка.
func (s *Snapshot) Infos() []Info {
	out := make([]Info, 0, len(s.profiles))
	for _, p := range s.profiles {
		out = append(out, InfoOf(p, s.Source(p.ID)))
	}
	return out
}
