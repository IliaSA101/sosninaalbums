//go:build ignore

// Лист образцов набора для library-verify (спека 144, design §3, security Н5 и Н7): четвёртый файл того же
// package main. Без него (сборка для 0.26/0.27) spriteRun пуст, и -sprite отвечает отказом.
//
//	library-verify -sprite набор.zip -out папка [-pick pick.json] [-with донор.zip]...
//
// Клетки режутся из байтов опубликованного zip (pack.OpenFile + Extract со сверкой sha256), а не из
// папки набора: клетка не может вести на предмет, которого в zip нет. В -out ложатся
// <id>-<версия>-preview.jpg (JPEG q80 baseline, YCbCr — SOF0) и <id>-<версия>-preview.json
// ({w, h, rows[{kind, w, h, items}]}); их читает library-pack.mjs (--preview-only и обычный ход).
// items — sha256[:16] записи манифеста, вид ряда равен виду записи. Повторный прогон того же zip даёт
// тот же лист байт в байт. Отбор — первые по манифесту: фонов 8, стикеров 12, рамочек 6, разворотов 4,
// макетов 1; -pick (JSON {"bg": ["bg/a.png", …], …}, имена записей манифеста, неизвестное поле — отказ)
// называет свои. Провалы копятся, при любом провале файлы листа не пишутся.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"io"
	"os"
	"path/filepath"
	"slices"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/decor"
	"github.com/IliaSA101/sosninaalbums/internal/decor/pack"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/disintegration/imaging"
)

func init() {
	spriteRun = runSprite
	spriteUsage = checkSpriteFlags
}

// checkSpriteFlags — сочетания ключей -sprite/-pick/-out/позиционных наборов; "" — всё в порядке.
func checkSpriteFlags(sprite, pick, out string, nArgs int) string {
	if sprite != "" && (nArgs > 0 || out == "") {
		return "-sprite <zip> -out <папка>: без других наборов"
	}
	if pick != "" && sprite == "" {
		return "-pick только вместе с -sprite"
	}
	return ""
}

const (
	spriteQuality = 80        // начальное качество JPEG; ниже — только если лист не влезает в предел
	spriteMaxJPEG = 384 << 10 // fetch.MaxPreview: больше клиент лист не берёт
	spriteMaxW    = 4096      // пределы design §2: w ≤ 4096, h ≤ 2048, w·h ≤ 4 Мп
	spriteMaxH    = 2048
	spriteMaxPix  = 4 << 20
	spritePad     = 6           // поле вокруг стикера, рамочки и плитки шаблона, px
	spriteMaxPick = 12          // клеток в ряду не больше (design §2)
	spritePickMax = 64 << 10    // файл -pick
	spriteBgMax   = 100_000_000 // фон: предел набора, Мп по заголовку до декодирования
	spriteSideMax = 4096        // сторона стикера и рамочки
)

// spriteKind — вид ряда: размер клетки и сколько клеток берётся по умолчанию.
type spriteKind struct {
	kind string
	w, h int
	n    int
}

// Порядок рядов — порядок листа сверху вниз.
var spriteKinds = []spriteKind{
	{"bg", 240, 160, 8}, {"sticker", 160, 160, 12}, {"frame", 160, 160, 6},
	{"spread", 240, 160, 4}, {"album", 240, 160, 1},
}

// spriteRow и spriteMeta — preview.json; ключи те же, что в поле preview строки каталога.
type spriteRow struct {
	Kind  string   `json:"kind"`
	W     int      `json:"w"`
	H     int      `json:"h"`
	Items []string `json:"items"`
}

type spriteMeta struct {
	W    int         `json:"w"`
	H    int         `json:"h"`
	Rows []spriteRow `json:"rows"`
}

// pickFile — свой отбор: имена записей манифеста по видам; неизвестное поле — отказ разбора.
type pickFile struct {
	Bg      []string `json:"bg"`
	Sticker []string `json:"sticker"`
	Frame   []string `json:"frame"`
	Spread  []string `json:"spread"`
	Album   []string `json:"album"`
}

func (p pickFile) of(kind string) []string {
	switch kind {
	case "bg":
		return p.Bg
	case "sticker":
		return p.Sticker
	case "frame":
		return p.Frame
	case "spread":
		return p.Spread
	}
	return p.Album
}

// readPick читает -pick: путь уже проверен paths.CheckLocalPath; открывает library.OpenLocal, читает не больше предела.
func readPick(path string) (*pickFile, error) {
	if path == "" {
		return nil, nil
	}
	f, _, err := library.OpenLocal(path)
	if err != nil {
		return nil, fmt.Errorf("-pick не открывается: %w", err)
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, spritePickMax+1))
	if err != nil {
		return nil, fmt.Errorf("-pick не читается: %w", err)
	}
	if len(data) > spritePickMax {
		return nil, fmt.Errorf("-pick больше %d КБ", spritePickMax>>10)
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var p pickFile
	if err := dec.Decode(&p); err != nil {
		return nil, fmt.Errorf("-pick: %w", err)
	}
	if dec.More() {
		return nil, errors.New("-pick: после объекта есть лишнее")
	}
	return &p, nil
}

// spriteItem — предмет манифеста, идущий в клетку.
type spriteItem struct {
	idx  int
	item pack.Item
}

// spriteSelect — предметы ряда: свой отбор (имена записей, без повторов, не больше spriteMaxPick) либо первые
// по манифесту. Одинаковые байты под двумя именами сюда не доходят: StagePack отвергает набор («повтор в наборе»).
func spriteSelect(m pack.Manifest, k spriteKind, pick *pickFile) ([]spriteItem, error) {
	var out []spriteItem
	add := func(i int) { out = append(out, spriteItem{i, m.Items[i]}) }
	if pick != nil {
		if names := pick.of(k.kind); len(names) > 0 {
			if len(names) > spriteMaxPick {
				return nil, fmt.Errorf("-pick %s: %d записей, не больше %d", k.kind, len(names), spriteMaxPick)
			}
			for _, name := range names {
				i := slices.IndexFunc(m.Items, func(it pack.Item) bool { return it.File == name && it.Kind == k.kind })
				if i < 0 {
					return nil, fmt.Errorf("-pick %s: в манифесте нет записи %q этого вида", k.kind, name)
				}
				if slices.ContainsFunc(out, func(s spriteItem) bool { return s.idx == i }) {
					return nil, fmt.Errorf("-pick %s: запись %q названа дважды", k.kind, name)
				}
				add(i)
			}
			return out, nil
		}
	}
	for i, it := range m.Items {
		if it.Kind == k.kind && len(out) < k.n {
			add(i)
		}
	}
	return out, nil
}

// runSprite — режим -sprite: лист образцов набора zipPath в v.out.
func runSprite(v *verifier, zipPath, pickPath string) {
	if v.out == "" {
		v.failf("для -sprite нужен -out")
		return
	}
	pick, err := readPick(pickPath)
	if err != nil {
		v.failf("%v", err)
		return
	}
	info, ok := v.install(zipPath) // ставит набор в пустую библиотеку: шаблоны берут картинки по ключам
	if !ok {
		return
	}
	f, _, err := library.OpenLocal(zipPath)
	if err != nil {
		v.failf("файл не открывается: %v", err)
		return
	}
	arc, err := pack.OpenFile(f, v.app)
	if err != nil {
		v.failf("pack.Open: %v", err)
		return
	}
	defer arc.Close()
	var rows []spriteRow
	var cells [][]image.Image
	for _, k := range spriteKinds {
		sel, err := spriteSelect(arc.Manifest, k, pick)
		if err != nil {
			v.failf("%v", err)
			continue
		}
		var row []image.Image
		var sums []string
		for _, s := range sel {
			v.ctx = filepath.Base(zipPath) + " · " + k.kind + " " + s.item.File
			v.guard("клетка", func() {
				if c := v.spriteCell(arc, s, k); c != nil {
					row = append(row, c)
					sums = append(sums, s.item.SHA256[:16])
				}
			})
		}
		v.ctx = filepath.Base(zipPath)
		if len(sums) > 0 {
			rows = append(rows, spriteRow{Kind: k.kind, W: k.w, H: k.h, Items: sums})
			cells = append(cells, row)
		}
	}
	if len(v.fails) > 0 {
		return // лист с дырой не пишется; провал донора (-with) или чужой клетки — тоже: items должны совпасть с клетками
	}
	if len(rows) == 0 {
		v.failf("нет ни одной клетки: в наборе нечего показывать")
		return
	}
	v.writeSprite(info, rows, cells)
}

// spriteCell — клетка предмета; nil — провал записан.
func (v *verifier) spriteCell(arc *pack.Archive, s spriteItem, k spriteKind) image.Image {
	var buf bytes.Buffer
	if err := arc.Extract(s.idx, &buf); err != nil {
		v.failf("%v", err)
		return nil
	}
	data := buf.Bytes()
	switch k.kind {
	case "bg", "sticker", "frame":
		return v.imageCell(data, s, k)
	case "spread":
		t, err := model.ParseSpreadTpl(data)
		if err != nil {
			v.failf("шаблон не читается: %v", err)
			return nil
		}
		return v.tplCell(s.item.Name, t, k)
	}
	l, err := model.ParseLook(data)
	if err != nil {
		v.failf("макет не читается: %v", err)
		return nil
	}
	var t model.SpreadTpl
	switch {
	case l.Cover != nil && l.Cover.Front != nil:
		t = l.Cover.SideTpl(1, 1) // лицо обложки, как строит плитка программы
	case len(l.Steps) > 0:
		t = lookFit(l.Steps[model.Series(l, len(l.Steps))[0]].Tpl)
	default:
		v.failf("в макете нечего показывать")
		return nil
	}
	return v.tplCell(s.item.Name, t, k)
}

// tplCell — плитка шаблона страницей 30×30 (tileOf, как лист -preview), вписанная в клетку.
func (v *verifier) tplCell(name string, t model.SpreadTpl, k spriteKind) image.Image {
	tile := v.tileOf(name, t)
	if tile == nil {
		v.failf("плитка «%s» не строится", name)
		return nil
	}
	cell := imaging.New(k.w, k.h, color.NRGBA{0xec, 0xec, 0xef, 255})
	return imaging.OverlayCenter(cell, imaging.Fit(tile, k.w-2*spritePad, k.h-2*spritePad, imaging.Lanczos), 1)
}

// imageCell — фон (Fill по центру), стикер или рамочка (вписаны с полем на шашечке).
func (v *verifier) imageCell(data []byte, s spriteItem, k spriteKind) image.Image {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		v.failf("картинка не читается: %v", err)
		return nil
	}
	if k.kind == "bg" && cfg.Width*cfg.Height > spriteBgMax || k.kind != "bg" && max(cfg.Width, cfg.Height) > spriteSideMax {
		v.failf("картинка %d×%d больше предела", cfg.Width, cfg.Height)
		return nil
	}
	if cfg.Width < 1 || cfg.Height < 1 {
		v.failf("картинка %d×%d", cfg.Width, cfg.Height)
		return nil
	}
	if k.kind == "bg" {
		src, err := imaging.Decode(bytes.NewReader(data), imaging.AutoOrientation(true))
		if err != nil {
			v.failf("картинка не читается: %v", err)
			return nil
		}
		return imaging.Fill(src, k.w, k.h, imaging.Center, imaging.Lanczos)
	}
	var src image.Image
	if k.kind == "frame" {
		fr, err := v.s.Frame("my:" + s.item.SHA256[:16])
		if err != nil {
			v.failf("Frame: %v", err)
			return nil
		}
		ov, err := overlay(fr, spotSizes[0]) // место 20×20: клетка квадратная
		if err != nil {
			v.failf("%v", err)
			return nil
		}
		src = ov
	} else {
		src, err = imaging.Decode(bytes.NewReader(data), imaging.AutoOrientation(true))
		if err != nil {
			v.failf("картинка не читается: %v", err)
			return nil
		}
	}
	bg := image.Image(checker(k.w, k.h))
	if k.kind == "sticker" && paleShare(src) > darkShare {
		bg = imaging.New(k.w, k.h, color.NRGBA{0x1e, 0x1e, 0x22, 255}) // светлый стикер — на тёмной шашечке
	}
	fit := imaging.Fit(src, k.w-2*spritePad, k.h-2*spritePad, imaging.Lanczos)
	return imaging.OverlayCenter(bg, fit, 1)
}

// darkShare — доля почти белых пикселей стикера, с которой плитка берёт тёмную подложку (как decor, спека 116).
const darkShare = 0.75

// paleShare — доля непрозрачных (альфа ≥ 128) пикселей, почти белых и без цвета; та же мера, что TestStickerDarkMatchesPixels.
func paleShare(img image.Image) float64 {
	n := imaging.Clone(img)
	var pale, all float64
	for i := 0; i+3 < len(n.Pix); i += 4 {
		if n.Pix[i+3] < 128 {
			continue
		}
		r, g, b := float64(n.Pix[i]), float64(n.Pix[i+1]), float64(n.Pix[i+2])
		if 0.2126*r+0.7152*g+0.0722*b > 235 && max(r, g, b)-min(r, g, b) < 24 {
			pale++
		}
		all++
	}
	if all == 0 {
		return 0
	}
	return pale / all
}

// writeSprite собирает лист из рядов, кодирует JPEG baseline и пишет лист и preview.json в v.out.
func (v *verifier) writeSprite(info decor.PackInfo, rows []spriteRow, cells [][]image.Image) {
	w, h := 0, 0
	for _, r := range rows {
		w, h = max(w, len(r.Items)*r.W), h+r.H
	}
	if w > spriteMaxW || h > spriteMaxH || w*h > spriteMaxPix {
		v.failf("лист %d×%d вне пределов (%d×%d, %d Мп)", w, h, spriteMaxW, spriteMaxH, spriteMaxPix>>20)
		return
	}
	sheet := imaging.New(w, h, color.NRGBA{0xd9, 0xd9, 0xdc, 255})
	y := 0
	for i, r := range rows {
		for j, c := range cells[i] {
			draw.Draw(sheet, image.Rect(j*r.W, y, (j+1)*r.W, y+r.H), c, c.Bounds().Min, draw.Src)
		}
		y += r.H
	}
	var data []byte
	quality := 0
	for q := spriteQuality; q >= 50 && data == nil; q -= 10 {
		var buf bytes.Buffer
		if err := jpeg.Encode(&buf, sheet, &jpeg.Options{Quality: q}); err != nil {
			v.failf("JPEG: %v", err)
			return
		}
		if buf.Len() <= spriteMaxJPEG {
			data, quality = buf.Bytes(), q
		}
	}
	if data == nil {
		v.failf("лист не влезает в %d КБ даже при качестве 50: уменьшите отбор (-pick)", spriteMaxJPEG>>10)
		return
	}
	meta, err := json.Marshal(spriteMeta{W: w, H: h, Rows: rows})
	if err != nil {
		v.failf("preview.json: %v", err)
		return
	}
	base := fmt.Sprintf("%s-%d-preview", info.ID, info.Version)
	if err := writeSheetPair(v.out, base, data, append(meta, '\n')); err != nil {
		v.failf("%v", err)
		return
	}
	n := 0
	for _, r := range rows {
		n += len(r.Items)
	}
	fmt.Fprintf(v.w, "лист: %s.jpg — %d×%d, %d байт (q%d), клеток %d, рядов %d\n", base, w, h, len(data), quality, n, len(rows))
}

// writeSheetPair пишет <base>.jpg и <base>.json парой: оба во временные файлы, затем два Rename; при любой ошибке
// не остаётся ни временных файлов, ни половины пары (уже переименованный первый тоже удаляется).
func writeSheetPair(dir, base string, jpg, meta []byte) error {
	names := []string{base + ".jpg", base + ".json"}
	bodies := [][]byte{jpg, meta}
	var tmps, finals []string
	cleanup := func() {
		for _, p := range append(slices.Clone(tmps), finals...) {
			_ = os.Remove(p)
		}
	}
	for i, name := range names {
		tmp := filepath.Join(dir, name+".tmp")
		tmps = append(tmps, tmp)
		if err := os.WriteFile(tmp, bodies[i], 0o644); err != nil {
			cleanup()
			return fmt.Errorf("%s: %w", name, err)
		}
	}
	for i, name := range names {
		final := filepath.Join(dir, name)
		if err := os.Rename(tmps[i], final); err != nil {
			cleanup()
			return fmt.Errorf("%s: %w", name, err)
		}
		finals = append(finals, final)
	}
	return nil
}
