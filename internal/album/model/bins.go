package model

import (
	"crypto/rand"
	"encoding/hex"
	"maps"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Подборки фото (083): определение — здесь, членство — в своих отметках (Mark.Bins). Список
// живёт в Editor рядом с отметками, а не в Project: правка подборки не трогает снимок альбома
// и projRev (decisions 083-3).

// Bin — подборка фото: ID и имя; фото — в своих отметках (Mark.Bins).
type Bin struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

const (
	MaxBins    = 50 // подборок в альбоме; Load обрезает хвост — поднимать только с версией формата
	MaxBinName = 40 // знаков (рун) имени
	maxBinID   = 16
)

// Side — данные рядом с проектом, вне его снимка и projRev (004, 083): свои отметки и подборки.
type Side struct {
	Marks map[string]Mark
	Bins  []Bin
}

// Clone — глубокая копия: Editor не делит отметки и список с чужим проектом.
func (s Side) Clone() Side {
	out := Side{Bins: slices.Clone(s.Bins)}
	if s.Marks != nil {
		out.Marks = make(map[string]Mark, len(s.Marks))
		for id, m := range s.Marks {
			out.Marks[id] = m.Clone()
		}
	}
	return out
}

// CleanBinName — имя без управляющих знаков и лишних пробелов, не длиннее MaxBinName рун.
func CleanBinName(s string) string {
	s = strings.ToValidUTF8(s, " ")
	s = strings.Join(strings.FieldsFunc(s, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }), " ")
	if utf8.RuneCountInString(s) > MaxBinName {
		s = strings.TrimSpace(string([]rune(s)[:MaxBinName]))
	}
	return s
}

// FreeBinName — «Подборка N» с первым свободным N ≥ 1.
func FreeBinName(bins []Bin) string {
	for n := 1; ; n++ {
		if name := "Подборка " + strconv.Itoa(n); !BinNameTaken(bins, name, "") {
			return name
		}
	}
}

// BinNameTaken — имя занято другой подборкой (без учёта регистра); except — ID, который не в счёт.
func BinNameTaken(bins []Bin, name, except string) bool {
	name = CleanBinName(name)
	return slices.ContainsFunc(bins, func(b Bin) bool { return b.ID != except && strings.EqualFold(CleanBinName(b.Name), name) })
}

// OkBinID — ID подборки: 1–16 знаков a-z, 0-9 (ключ в файле и в UI, не путь).
func OkBinID(id string) bool {
	if id == "" || len(id) > maxBinID {
		return false
	}
	for _, r := range id {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') {
			return false
		}
	}
	return true
}

// NewBinID — случайный ID «b» + 8 hex; занятый (taken) — ещё раз. Случайный, а не счётчик:
// удалённая подборка не отдаёт свой ID новой, и чужое членство из копии не попадёт в неё (083-21).
func NewBinID(taken func(string) bool) string {
	for {
		var b [4]byte
		_, _ = rand.Read(b[:]) // crypto/rand.Read не возвращает ошибку (Go 1.24+)
		if id := "b" + hex.EncodeToString(b[:]); !taken(id) {
			return id
		}
	}
}

// ClampBins — подборки из файла: плохой или повторный ID — прочь, имя очищено, пустое —
// «Подборка N», повтор имени — « 2», « 3»…, сверх MaxBins — хвост прочь. fixed — сколько
// записей отброшено или поправлено (для строки в log.txt).
func ClampBins(bins []Bin) (out []Bin, fixed int) {
	ids := map[string]bool{}
	for _, b := range bins {
		if !OkBinID(b.ID) || ids[b.ID] || len(out) == MaxBins {
			fixed++
			continue
		}
		name := CleanBinName(b.Name)
		switch {
		case name == "":
			name = FreeBinName(out)
		case BinNameTaken(out, name, ""):
			name = numberedName(out, name)
		}
		if name != b.Name {
			fixed++
		}
		ids[b.ID] = true
		out = append(out, Bin{ID: b.ID, Name: name})
	}
	return out, fixed
}

// numberedName — «имя 2», «имя 3»…: первое свободное; имя укорачивается, чтобы с номером
// уложиться в MaxBinName.
func numberedName(bins []Bin, name string) string {
	for n := 2; ; n++ {
		tail := " " + strconv.Itoa(n)
		base := []rune(name)
		if keep := MaxBinName - utf8.RuneCountInString(tail); len(base) > keep {
			base = base[:keep]
		}
		if cand := strings.TrimSpace(string(base)) + tail; !BinNameTaken(bins, cand, "") {
			return cand
		}
	}
}

// BinSet — ID подборок множеством (Load оставляет у отметок только известные).
func BinSet(bins []Bin) map[string]bool {
	out := make(map[string]bool, len(bins))
	for _, b := range bins {
		out[b.ID] = true
	}
	return out
}

// normBins — ID по возрастанию без повторов; пусто — nil (Mark.None, файл без «bins»).
func normBins(ids []string) []string {
	if len(ids) == 0 {
		return nil
	}
	set := map[string]bool{}
	for _, id := range ids {
		set[id] = true
	}
	return slices.Sorted(maps.Keys(set))
}
