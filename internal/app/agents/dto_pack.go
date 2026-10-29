package agents

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"strconv"

	"github.com/IliaSA101/sosninaalbums/internal/paths"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

// Наборы, профили типографий, «Последние» альбомы.

type packCounts struct {
	Bg      int `json:"bg"`
	Sticker int `json:"sticker"`
	Spread  int `json:"spread"`
	Frame   int `json:"frame"`
	Print   int `json:"print"`
}

type packOut struct {
	ID      string     `json:"id"`
	Title   string     `json:"title"`
	Version int        `json:"version"`
	Counts  packCounts `json:"counts"`
	Bytes   int64      `json:"bytes"`
	From    string     `json:"from" jsonschema:"builtin, net, file или demo"`
	Author  string     `json:"author,omitempty"`
	License string     `json:"license,omitempty"`
}

type availOut struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Version int    `json:"version"`
	Bytes   int64  `json:"bytes"`
	New     bool   `json:"new"`
}

type netOut struct {
	Allowed bool `json:"allowed"`
	AtStart bool `json:"atStart"`
}

type packsOut struct {
	meta
	Installed []packOut  `json:"installed"`
	Available []availOut `json:"available"`
	ListedAt  int64      `json:"listedAt" jsonschema:"когда сохранён список «Можно скачать», unix мс; 0 — списка нет"`
	Network   netOut     `json:"network"`
}

func (c *call) listPacks() (packsOut, error) {
	p, err := c.s.h.Packs()
	if err != nil {
		return packsOut{}, err
	}
	out := packsOut{meta: c.meta(), Installed: []packOut{}, Available: []availOut{}, ListedAt: p.AvailAt,
		Network: netOut{Allowed: p.Allowed, AtStart: p.AtStart}}
	files := 0
	for _, in := range p.Installed {
		o := packOut{ID: cleanID(in.ID), Title: cleanLine(in.Title, maxFile), Version: in.Version, Bytes: in.Bytes, From: cleanID(in.Origin),
			Counts:  packCounts{Bg: in.Bg, Sticker: in.Sticker, Spread: in.Spread, Frame: in.Frame, Print: in.Print},
			Author:  cleanLine(in.Author, maxFile),
			License: cleanLine(in.License, maxFile)}
		if !trustedOrigin(in.Origin) { // имя и автор набора из файла (и неизвестного источника) — от человека
			files++
			if !c.names() {
				o.ID, o.Title, o.Author, o.License = "pack-"+strconv.Itoa(files), "Набор из файла "+strconv.Itoa(files), "", ""
			}
		}
		out.Installed = append(out.Installed, o)
	}
	for _, a := range p.Available { // список из подписанного каталога
		out.Available = append(out.Available, availOut{ID: cleanID(a.ID), Title: cleanLine(a.Title, maxFile), Version: a.Version, Bytes: a.Bytes, New: a.New})
	}
	return out, nil
}

type formatOut struct {
	Name string  `json:"name"`
	W    float64 `json:"w"`
	H    float64 `json:"h"`
}

type printOut struct {
	ID      string      `json:"id"`
	Name    string      `json:"name"`
	Source  string      `json:"source" jsonschema:"builtin, library или own"`
	Formats []formatOut `json:"formats"`
	Export  bool        `json:"export" jsonschema:"у профиля есть правила выгрузки"`
}

type printsOut struct {
	meta
	Items []printOut `json:"items"`
}

// ownPrintID — id профиля типографии, которого агент на уровне 1 видеть не должен.
func ownPrintID(n int) string { return "own-" + strconv.Itoa(n) }

// hiddenPrints — профили, чьи id и названия на уровне 1 скрыты: все, кроме встроенных (свой
// назвал человек, профиль из набора-файла — его автор). Значение — номер по порядку списка.
func (c *call) hiddenPrints() map[string]int {
	out := map[string]int{}
	for _, p := range c.s.h.Prints() {
		if p.Source != "builtin" {
			out[p.ID] = len(out) + 1
		}
	}
	return out
}

func (c *call) listPrints() (printsOut, error) {
	out := printsOut{meta: c.meta(), Items: []printOut{}}
	hidden := c.hiddenPrints()
	for _, p := range c.s.h.Prints() {
		o := printOut{ID: cleanID(p.ID), Name: cleanLine(p.Name, maxFile), Source: cleanID(p.Source), Export: p.HasExport, Formats: []formatOut{}}
		n, hide := hidden[p.ID]
		if hide && !c.names() {
			o.ID, o.Name = ownPrintID(n), "Свой профиль "+strconv.Itoa(n)
		} else {
			hide = false
		}
		for i, f := range p.Formats {
			fo := formatOut{Name: cleanLine(f.Name, maxFile), W: f.PageW, H: f.PageH}
			if hide {
				fo.Name = "Формат " + strconv.Itoa(i+1)
			}
			o.Formats = append(o.Formats, fo)
		}
		out.Items = append(out.Items, o)
	}
	return out, nil
}

type recentOut struct {
	Key      string `json:"key" jsonschema:"для сравнения внутри запуска; путь не передаётся"`
	Name     string `json:"name,omitempty"`
	Modified int64  `json:"modified"`
	Exists   bool   `json:"exists"`
	Open     bool   `json:"open" jsonschema:"этот альбом открыт сейчас"`
}

type recentsOut struct {
	meta
	Items []recentOut `json:"items"`
}

// maxRecent — сколько «Последних» отдаём.
const maxRecent = 50

func (c *call) listAlbums() (recentsOut, error) {
	v, ed := c.s.h.View()
	out := recentsOut{meta: c.metaOf(v, ed), Items: []recentOut{}}
	for _, r := range c.s.h.Recent() {
		if len(out.Items) == maxRecent {
			break
		}
		sum := sha256.Sum256(append(append([]byte{}, keySalt[:]...), paths.Fold(filepath.Clean(r.Path))...)) // с солью процесса: ключ не проверяет догадку о пути
		out.Items = append(out.Items, recentOut{Key: hex.EncodeToString(sum[:6]), Name: c.text(recentName(r), maxFile),
			Modified: r.Modified, Exists: r.Exists, Open: ed != 0 && v.Path != "" && paths.SamePath(v.Path, r.Path)})
	}
	return out, nil
}

// recentName — имя «Последнего»: заданное или по файлу.
func recentName(r prefs.RecentProject) string {
	if r.Name != "" {
		return r.Name
	}
	return albumName(r.Path)
}
