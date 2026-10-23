package prefs

import (
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/osx"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// Уровни интерфейса (спеки 010, 011): чужое значение из файла или от UI — simple.
const (
	LevelSimple   = "simple"
	LevelStandard = "standard"
	LevelFull     = "full"
	LevelCustom   = "custom" // «Свой»: основа + разница (CustomLevel)
)

// Пределы набора «Своего»: config.json правят руками, длинные списки не пропускаем.
const (
	maxCustomIDs   = 64
	maxCustomIDLen = 32
)

// maxRecent — сколько последних альбомов помним.
const maxRecent = 8

// Prefs — настройки программы для интерфейса. First — первый запуск (в config.json нет
// уровня): интерфейс покажет выбор уровня; при записи игнорируется.
type Prefs struct {
	Level     string `json:"level"`
	ShowStart bool   `json:"showStart"` // показывать стартовое окно
	First     bool   `json:"first"`
	// Custom — набор «Своего»; null — «Свой» не настраивали (и уровень не custom).
	Custom *CustomLevel `json:"custom"`
}

// CustomLevel — набор уровня «Свой»: разница с основой, а не полный список, чтобы функция
// из новой версии вела себя как в основе. ID — строки FEATURES интерфейса: Go их не знает
// и только чистит.
type CustomLevel struct {
	Base string   `json:"base"` // simple | standard | full
	On   []string `json:"on"`   // включено сверх основы
	Off  []string `json:"off"`  // снято из основы
}

// RecentProject — строка списка последних альбомов. Modified — unix-миллисекунды
// (0, если файла нет).
type RecentProject struct {
	Path     string `json:"path"`
	Name     string `json:"name"`
	Exists   bool   `json:"exists"`
	Modified int64  `json:"modified"`
}

// NormLevel: известный уровень как есть, остальное — simple.
func NormLevel(s string) string {
	if s == LevelCustom {
		return s
	}
	return normBase(s)
}

// normBase — основа «Своего»: только уровни с готовым набором, custom и чужое — simple.
func normBase(s string) string {
	switch s {
	case LevelSimple, LevelStandard, LevelFull:
		return s
	}
	return LevelSimple
}

// customID — id строки: 1–32 латинские буквы (в JSON и UI ничего другого не нужно).
func customID(s string) bool {
	if s == "" || len(s) > maxCustomIDLen {
		return false
	}
	for i := range len(s) {
		if c := s[i]; (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') {
			return false
		}
	}
	return true
}

// customIDs оставляет годные id без повторов и без тех, что в skip, не больше maxCustomIDs.
// Списки не nil: в JSON — [], а не null.
func customIDs(list []string, skip []string) []string {
	out := []string{}
	for _, id := range list {
		if len(out) == maxCustomIDs {
			break
		}
		if customID(id) && !slices.Contains(out, id) && !slices.Contains(skip, id) {
			out = append(out, id)
		}
	}
	return out
}

// SanitizeCustom чистит набор из config.json или от UI. id сразу в on и off остаётся
// только в off: меньше инструментов — тише.
func SanitizeCustom(c CustomLevel) CustomLevel {
	off := customIDs(c.Off, nil)
	return CustomLevel{Base: normBase(c.Base), On: customIDs(c.On, off), Off: off}
}

// Of — настройки из конфига; пустой уровень — первый запуск.
func Of(c Config) Prefs {
	p := Prefs{Level: NormLevel(c.Level), ShowStart: !c.HideStart, First: c.Level == ""}
	switch {
	case c.Custom != nil:
		cl := SanitizeCustom(*c.Custom)
		p.Custom = &cl
	case p.Level == LevelCustom: // уровень есть, набора нет — пустой «Свой» от «Простого»
		p.Custom = &CustomLevel{Base: LevelSimple, On: []string{}, Off: []string{}}
	}
	return p
}

// PushRecent ставит path наверх списка без повторов (paths.FoldEqual).
// Нелокальные пути (UNC, относительные) не пишем; список режется до maxRecent.
func PushRecent(list []string, path string) []string {
	if paths.CheckLocalPath(path) != nil {
		return list
	}
	out := make([]string, 0, maxRecent)
	out = append(out, path)
	for _, p := range list {
		if len(out) == maxRecent {
			break
		}
		if !paths.FoldEqual(p, path) && paths.CheckLocalPath(p) == nil {
			out = append(out, p)
		}
	}
	return out
}

// DropRecent убирает path из списка (paths.FoldEqual).
func DropRecent(list []string, path string) []string {
	return slices.DeleteFunc(slices.Clone(list), func(p string) bool { return paths.FoldEqual(p, path) })
}

// RecentIndex — позиция path в списке (paths.FoldEqual) или -1.
func RecentIndex(list []string, path string) int {
	return slices.IndexFunc(list, func(p string) bool { return paths.FoldEqual(p, path) })
}

// RecentView строит строки списка: имя без расширения, наличие и время правки из os.Stat.
// config.json могут править руками: нелокальные строки пропускаем (UNC — Stat по SMB
// при каждом открытии меню), список режем до maxRecent.
func RecentView(list []string) []RecentProject {
	out := make([]RecentProject, 0, min(len(list), maxRecent))
	for _, p := range list {
		if len(out) == maxRecent {
			break
		}
		if paths.CheckLocalPath(p) != nil {
			continue
		}
		r := RecentProject{Path: p, Name: strings.TrimSuffix(filepath.Base(p), filepath.Ext(p))}
		// Альбом на отключённом сетевом диске — «нет файла» без ожидания таймаута SMB.
		if !osx.Reachable(p) {
			out = append(out, r)
			continue
		}
		if st, err := os.Stat(p); err == nil && st.Mode().IsRegular() {
			r.Exists, r.Modified = true, st.ModTime().UnixMilli()
		}
		out = append(out, r)
	}
	return out
}
