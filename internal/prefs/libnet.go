package prefs

import (
	"slices"
	"strings"
)

// LibraryNet — согласие на сеть библиотеки (067 Ф3, решения 16–18): Allowed — «Разрешить»
// (без него программа в сеть не ходит), AtStart — «Проверять при запуске», Checked — unix мс
// последней удачной проверки, Skip — ID наборов, удалённых пользователем: «Обновить» их не
// возвращает, «Добавить из файла» снимает отсюда. CatalogSeq — seq последнего принятого
// каталога: меньший — откат к старому подписанному каталогу, отказ (0.17 решение №2).
type LibraryNet struct {
	Allowed    bool     `json:"allowed,omitempty"`
	AtStart    bool     `json:"atStart,omitempty"`
	Checked    int64    `json:"checked,omitempty"`
	CatalogSeq int64    `json:"catalogSeq,omitempty"`
	Skip       []string `json:"skip,omitempty"`
}

// maxSkip — удалённых наборов в памяти (= наборов в каталоге).
const maxSkip = 500

// clean — после правки руками: флажок без согласия не действует, ID по грамматике без
// повторов, время и seq не отрицательные.
func (n LibraryNet) clean() LibraryNet {
	n.AtStart = n.AtStart && n.Allowed
	n.Checked, n.CatalogSeq = max(n.Checked, 0), max(n.CatalogSeq, 0)
	out := make([]string, 0, min(len(n.Skip), maxSkip))
	for _, id := range n.Skip {
		if len(out) < maxSkip && validPackID(id) && !slices.Contains(out, id) {
			out = append(out, id)
		}
	}
	n.Skip = out
	return n
}

// LibraryNetOf — настройки сети из config.json (нет — всё выключено).
func LibraryNetOf(dir string) LibraryNet {
	c := LoadConfig(dir)
	if c.LibraryNet == nil {
		return LibraryNet{Skip: []string{}}
	}
	return c.LibraryNet.clean()
}

// UpdateLibraryNet — правка настроек сети под замком config.json; итог после чистки.
func UpdateLibraryNet(dir string, fn func(n *LibraryNet)) LibraryNet {
	var out LibraryNet
	UpdateConfig(dir, func(c *Config) {
		var n LibraryNet
		if c.LibraryNet != nil {
			n = *c.LibraryNet
		}
		fn(&n)
		out = n.clean()
		if out.Allowed || out.Checked != 0 || out.CatalogSeq != 0 || len(out.Skip) != 0 {
			c.LibraryNet = &out
		} else {
			c.LibraryNet = nil // всё выключено — ключа в файле нет
		}
	})
	return out
}

// SetSkip — id в список удалённых (on) или из него.
func (n *LibraryNet) SetSkip(id string, on bool) {
	out := n.Skip[:0:0]
	for _, v := range n.Skip {
		if v != id {
			out = append(out, v)
		}
	}
	if on && validPackID(id) {
		out = append(out, id)
		out = out[max(len(out)-maxSkip, 0):] // переполнение — уходят самые старые
	}
	n.Skip = out
}

// validPackID — ^[a-z0-9][a-z0-9-]{2,39}$ (как pack.ValidID; prefs не знает decor).
func validPackID(id string) bool {
	return len(id) >= 3 && len(id) <= 40 && id[0] != '-' && strings.Trim(id, "abcdefghijklmnopqrstuvwxyz0123456789-") == ""
}
