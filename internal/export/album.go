package export

import (
	"fmt"
	"image/color"
	"slices"
	"strconv"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/geom"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/render"
	"github.com/IliaSA101/sosninaalbums/internal/text"
)

// ColorWarning — текст вопроса перед экспортом при фото с «чужим» профилем; "" — предупреждать не о чем.
// Имён профилей не больше трёх (чаще встречающиеся первыми), дальше «…».
func ColorWarning(other map[string]int) string {
	total := 0
	names := make([]string, 0, len(other))
	for n, c := range other {
		total += c
		names = append(names, n)
	}
	if total == 0 {
		return ""
	}
	slices.SortFunc(names, func(x, y string) int {
		if other[x] != other[y] {
			return other[y] - other[x]
		}
		return strings.Compare(x, y)
	})
	parts := make([]string, 0, 4)
	for i, n := range names {
		if i == 3 {
			parts = append(parts, "…")
			break
		}
		parts = append(parts, fmt.Sprintf("%s — %d", n, other[n]))
	}
	return fmt.Sprintf("Фото с цветовым профилем не sRGB: %d (%s). Программа их не пересчитывает — "+
		"в печати цвета будут искажены. Лучше пересохранить их в sRGB. Экспортировать всё равно?",
		total, strings.Join(parts, ", "))
}

// emptySummary — номера (с 1) разворотов с пустыми местами сетки и общее число пустых мест.
func emptySummary(v model.View) (spreads []int, n int) { return EmptySummaryFor(v, nil, 0) }

// EmptySummaryFor — то же по выбранному: pl == nil — весь альбом; у цели-страницы считаются
// места, заходящие на неё (pageW — ширина страницы, мм).
func EmptySummaryFor(v model.View, pl *Plan, pageW float64) (spreads []int, n int) {
	for i, s := range v.Spreads {
		k := 0
		for _, pc := range s.Places {
			if pc.Photo == "" && (pl == nil || pl.wants(i+1, layout.Rect{X: pc.X, Y: pc.Y, W: pc.W, H: pc.H}, pageW)) {
				k++
			}
		}
		if k > 0 {
			spreads = append(spreads, i+1)
			n += k
		}
	}
	return spreads, n
}

// EmptyWarning — текст вопроса перед экспортом о пустых местах; "" — пустых мест нет.
// Номеров не больше 10, остальные — «…и ещё K».
func EmptyWarning(spreads []int, n int) string {
	if n == 0 {
		return ""
	}
	const maxShown = 10
	parts := make([]string, 0, maxShown)
	for _, s := range spreads[:min(len(spreads), maxShown)] {
		parts = append(parts, strconv.Itoa(s))
	}
	list := strings.Join(parts, ", ")
	if len(spreads) > maxShown {
		list += fmt.Sprintf(" …и ещё %d", len(spreads)-maxShown)
	}
	word := "разворотах"
	if len(spreads) == 1 {
		word = "развороте"
	}
	return fmt.Sprintf("На %s %s пустые места (%d). В файлы они не попадут, там будет фон. Выгрузить?", word, list, n)
}

// ColorNote — сводка для export:done о пересчёте Adobe RGB в sRGB; "" — пересчёта не было.
func ColorNote(adobe int) string {
	if adobe == 0 {
		return ""
	}
	return fmt.Sprintf("Пересчитано из Adobe RGB в sRGB: %d фото.", adobe)
}

// FromView собирает задание экспорта из View: фото с кадром, фон страниц и текст
// (слои — по Z фото и текстов, его сортирует Render). Выгружается каждый разворот, даже пустой (один фон):
// номер файла — номер разворота в альбоме, пропуск сдвинул бы все следующие («05.jpg» стал бы
// шестым разворотом), а типография сверяет файлы с макетом по номерам.
func FromView(p *model.Project, v model.View) []render.Spread {
	out := make([]render.Spread, 0, len(v.Spreads))
	for _, s := range v.Spreads {
		out = append(out, fromSheet(p, s))
	}
	return out
}

// slotColor — пустое место на плитке шаблона (067): серая плашка, как заглушка холста.
var slotColor = color.RGBA{0xcf, 0xcf, 0xd4, 255}

// TplScene — сцена плитки готового разворота (067, design §3) из вида временного проекта
// album.PreviewTpl: как экспорт листа, а пустые места — плашки в их слое (стикер «под
// местами» остаётся под ними). Повёрнутое место — описанным прямоугольником: для плитки
// хватает. Нет листа — false.
func TplScene(p *model.Project, v model.View) (render.Spread, bool) {
	if len(v.Spreads) == 0 {
		return render.Spread{}, false
	}
	s := v.Spreads[0]
	es := fromSheet(p, s)
	for _, pl := range s.Places {
		if pl.Photo != "" {
			continue
		}
		r := geom.RotRect{X: pl.X, Y: pl.Y, W: pl.W, H: pl.H, Rot: pl.Rot}.AABB()
		es.Slots = append(es.Slots, render.Plate{Rect: r, Color: slotColor, Z: pl.Z})
	}
	return es, true
}

// fromSheet — задание рендера одного листа (разворота или обложки) из его вида.
func fromSheet(p *model.Project, s model.SpreadView) render.Spread {
	es := render.Spread{Bg: [2]color.RGBA{render.ParseHex(s.Bg[0]), render.ParseHex(s.Bg[1])},
		Patterns: patterns(s), Backdrops: backdrops(p, s)}
	for i, id := range s.Photos {
		if i >= len(s.Rects) || i >= len(s.Frames) {
			break // раскладки нет: фото без ячейки не рисуем
		}
		f := s.Frames[i]
		es.Photos = append(es.Photos, render.Photo{Cell: s.Rects[i], Img: f.Img, Path: p.Photos[id].Path,
			FlipH: f.FlipH, FlipV: f.FlipV, Rot: f.CellRot, ImgRot: f.R, Z: f.Z,
			Outline: geom.Outline{Oval: f.Shape == model.ShapeOval, Radius: f.Radius},
			Tone:    render.Tone{Gray: f.Gray, Bright: f.Bright, Contrast: f.Contrast}})
	}
	stickers(p, s, &es)
	for _, c := range s.Captions {
		es.Texts = append(es.Texts, textDraw(c.TextView))
	}
	for _, t := range s.Texts {
		es.Texts = append(es.Texts, textDraw(t))
	}
	return es
}

// textDraw переводит блок из View (строки в мм разворота) в блок экспорта (строки от угла блока).
func textDraw(t model.TextView) render.TextDraw {
	lines := make([]text.Line, len(t.Lines))
	for i, l := range t.Lines {
		l.X -= t.X
		l.Y -= t.Y
		lines[i] = l
	}
	return render.TextDraw{X: t.X, Y: t.Y, Rot: t.Rot, Box: text.Box{W: t.W, H: t.H, Lines: lines}, Style: t.Style, Z: t.Z}
}

// backdrops — фоны разворота для экспорта: рамка, область, фото в осях рамки и его наклон из View;
// фото корешка обложки ложится поверх плашки (061).
func backdrops(p *model.Project, s model.SpreadView) []render.Backdrop {
	var out []render.Backdrop
	for _, b := range s.Backdrops {
		out = append(out, render.Backdrop{
			Area: b.Area, Box: layout.Rect{X: b.Box.X, Y: b.Box.Y, W: b.Box.W, H: b.Box.H}, Rot: b.Box.Rot,
			ImgRot: b.Frame.R, Img: b.Img, FlipH: b.FlipH, FlipV: b.FlipV, Path: p.Photos[b.Photo].Path, Fade: b.Fade,
			OverPlates: b.Scope == model.ScopeSpine, // фото корешка — над плашкой (061)
		})
	}
	return out
}

// patterns — узоры листа для экспорта: область и параметры из View, как у холста (048).
// Неизвестный генератор не рисуется (gen.DrawPattern его пропускает) — передаём как есть.
func patterns(s model.SpreadView) []render.PatternDraw {
	var out []render.PatternDraw
	for _, pv := range s.Patterns {
		pt := pv.Pat
		out = append(out, render.PatternDraw{Area: pv.Area, Kind: pt.Kind, Inks: pt.Inks, Scale: pt.Scale,
			Seed: pt.Seed, Fade: pt.Fade})
	}
	return out
}

// stickers — стикеры листа для экспорта (048): свой — фото целиком в рамке (путь — из Photos,
// у своего его ставит искатель библиотеки), процедурный — параметры как у холста. Неизвестный
// генератор не рисуется (gen.DrawArt его пропускает) — передаём как есть.
func stickers(p *model.Project, s model.SpreadView, es *render.Spread) {
	for _, st := range s.Stickers {
		box := layout.Rect{X: st.X, Y: st.Y, W: st.W, H: st.H}
		if st.Art != nil {
			es.Arts = append(es.Arts, render.ArtDraw{Box: box, Rot: st.Rot, FlipH: st.FlipH, FlipV: st.FlipV,
				Kind: st.Art.Kind, Inks: st.Art.Inks, Seed: st.Art.Seed, Fade: st.Fade, Z: st.Z})
			continue
		}
		es.Stickers = append(es.Stickers, render.Photo{Cell: box, Img: box, Path: p.Photos[st.Photo].Path,
			FlipH: st.FlipH, FlipV: st.FlipV, Rot: st.Rot, Z: st.Z, Fade: st.Fade})
	}
}
