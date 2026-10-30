package packprep

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// srcItem и srcManifest — manifest.src.json для scripts/library-pack.mjs (sha256 и size считает он).
type srcItem struct {
	File  string `json:"file"`
	Kind  string `json:"kind"`
	Theme string `json:"theme"`
	Name  string `json:"name"`
}

type srcManifest struct {
	ID      string    `json:"id"`
	Version int       `json:"version"`
	MinApp  string    `json:"minApp"`
	Title   string    `json:"title"`
	About   string    `json:"about"`
	Author  string    `json:"author"`
	License string    `json:"license"`
	Source  string    `json:"source"`
	Needs   string    `json:"needs,omitempty"`
	Items   []srcItem `json:"items"`
}

// licenseText — LICENSE.txt набора: CC0 1.0 и происхождение картинок. Условия сервиса ИИ на дату
// выпуска — docs/library.md «Источники» (Т10), здесь только то, что набор утверждает о себе.
const licenseText = `CC0 1.0 Universal (CC0 1.0) — Public Domain Dedication
https://creativecommons.org/publicdomain/zero/1.0/

Набор «%s»: фоны, рамочки, уголки, разделители и стикеры нарисованы нейросетью (Magnific) по
запросам авторов, отобраны и обработаны вручную: %s. Авторы передают результат в общественное
достояние: его можно копировать, менять и использовать, в том числе в коммерческих альбомах,
без разрешения и без указания авторства. Будем рады упоминанию Sosnina Albums.

The backgrounds, frames, corners, dividers and stickers in this pack were generated with an AI service
(Magnific) from the authors' prompts, then selected and processed by hand by the authors, and are
dedicated to the public domain under CC0 1.0. Credit to Sosnina Albums is welcome but not required.
`

// writeManifest пишет manifest.src.json (порядок — как в report.json) и LICENSE.txt.
func (r *runner) writeManifest() error {
	m := srcManifest{ID: r.rec.ID, Version: r.rec.Version, MinApp: r.rec.MinApp, Title: r.rec.Title, About: r.rec.About,
		Author: r.rec.Author, License: r.rec.License, Source: r.rec.Source, Needs: r.rec.Needs}
	for _, o := range r.outs {
		m.Items = append(m.Items, srcItem{File: o.File, Kind: ManifestKind(o.Kind), Theme: o.Theme, Name: o.Name})
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(r.cfg.Out, ManifestSrcName), append(data, '\n'), 0o644); err != nil {
		return err
	}
	lic := fmt.Sprintf(licenseText, r.rec.Title, r.rec.Author)
	return os.WriteFile(filepath.Join(r.cfg.Out, LicenseName), []byte(lic), 0o644)
}
