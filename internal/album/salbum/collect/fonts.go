package collect

import (
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// Свои шрифты альбома в папке сбора (082-8, 082-13): файлы библиотеки шрифтов копируются в
// «Шрифты» рядом с «Фото» — на другом компьютере открытие альбома добавит их в библиотеку.

// DirFonts — папка файлов своих шрифтов в папке сбора; её же читает открытие собранного альбома.
const DirFonts = "Шрифты"

// KindFont — файл своего шрифта (TTF/OTF из библиотеки шрифтов); Refs пусты: в альбоме шрифт
// стоит по id гарнитуры, а не по пути.
const KindFont = "font"

// FontSrc — начертание своей гарнитуры альбома из библиотеки шрифтов (App берёт fontlib.Faces).
type FontSrc struct {
	Path         string // файл библиотеки
	Name         string // название гарнитуры: из него имя файла в папке сбора
	Bold, Italic bool
	Restricted   bool // лицензия в файле запрещает перенос (OS/2.fsType)
}

// AddFonts — файлы своих шрифтов альбома в план: fonts — начертания по id гарнитуры (только тех,
// что есть на этом компьютере), missing — сколько стоящих в альбоме гарнитур здесь нет. Начертание
// с Restricted не копируется, название его гарнитуры — в FontsRestricted. Имя в папке —
// «<гарнитура>-<Regular|Bold|Italic|BoldItalic>.<ext>», занятое — « (N)»; dir и stat — как у Make.
func (pl *Plan) AddFonts(fonts map[string][]FontSrc, missing int, dir string, stat func(string) (fs.FileInfo, error)) error {
	pl.MissingFonts = missing
	nm := names{used: map[string]bool{}, onDisk: map[string]fs.DirEntry{}}
	if dir != "" {
		if ents, err := os.ReadDir(filepath.Join(dir, DirFonts)); err == nil {
			for _, e := range ents {
				nm.onDisk[nameKey(e.Name())] = e
			}
		}
	}
	for _, id := range slices.Sorted(maps.Keys(fonts)) {
		for _, f := range fonts[id] {
			if f.Restricted {
				if !slices.Contains(pl.FontsRestricted, f.Name) {
					pl.FontsRestricted = append(pl.FontsRestricted, f.Name)
				}
				continue
			}
			st, ok := regular(KindFont, f.Path, stat)
			if !ok {
				continue // файл библиотеки пропал за время сбора — шрифт просто не попадёт в папку
			}
			ext := strings.ToLower(filepath.Ext(f.Path))
			base := paths.FileName(f.Name+"-"+faceSuffix(f.Bold, f.Italic)+ext, "Шрифт")
			name, skip, ok := nm.pick(base, st)
			if !ok {
				return fmt.Errorf("в папке «%s» слишком много файлов с именем «%s»", DirFonts, base)
			}
			pl.Items = append(pl.Items, Item{Kind: KindFont, Src: filepath.Clean(f.Path), Rel: DirFonts + "/" + name,
				Size: st.Size(), ModTime: st.ModTime(), Skip: skip})
			if !skip {
				pl.Bytes += st.Size()
			}
		}
	}
	return nil
}

// faceSuffix — начертание в имени файла, как у файлов шрифтов в архивах.
func faceSuffix(bold, italic bool) string {
	switch {
	case bold && italic:
		return "BoldItalic"
	case bold:
		return "Bold"
	case italic:
		return "Italic"
	}
	return "Regular"
}
