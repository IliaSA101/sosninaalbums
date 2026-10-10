package export

import (
	"fmt"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/render"
)

// Маски целей разворота в Plan.mask.
const (
	maskSpread = 1 << iota // разворот целиком (в выбранном виде)
	maskLeft               // только левая страница
	maskRight              // только правая страница
)

// Plan — что пишет экспорт: задания для рендера и цели (по ним отбираются файлы папки).
type Plan struct {
	Full bool          // все развороты: хвосты прежнего экспорта удаляются
	Jobs []Job         // по возрастанию разворота
	mask map[int]uint8 // номер разворота с 1 → маска целей
}

// NewPlan строит план: units — цели из ParseRange, nil — все n разворотов; pages — вид
// «По страницам». Цель «разворот» в этом виде даёт обе страницы, цель «страница» — одну.
func NewPlan(units []Unit, n int, pages bool) Plan {
	pl := Plan{Full: units == nil, mask: map[int]uint8{}}
	if pl.Full {
		for i := 1; i <= n; i++ {
			units = append(units, Unit{i, 0})
		}
	}
	for _, u := range units {
		if len(pl.Jobs) == 0 || pl.Jobs[len(pl.Jobs)-1].Index != u.Spread-1 {
			pl.Jobs = append(pl.Jobs, Job{Index: u.Spread - 1})
		}
		j := &pl.Jobs[len(pl.Jobs)-1]
		switch u.Page {
		case 1:
			pl.mask[u.Spread] |= maskLeft
			j.Left = true
		case 2:
			pl.mask[u.Spread] |= maskRight
			j.Right = true
		default:
			pl.mask[u.Spread] |= maskSpread
			if pages {
				j.Left, j.Right = true, true
			} else {
				j.Whole = true
			}
		}
	}
	return pl
}

// Names — имена файлов, которые запишет план, для альбома из total разворотов.
func (pl Plan) Names(total int) []string {
	var out []string
	for _, j := range pl.Jobs {
		out = append(out, j.Names(total)...)
	}
	return out
}

// hitsFile — задевает ли файл папки цель плана: цель-разворот — любой файл с этим номером
// (NN.jpg, NN-1.jpg, NN-2.jpg, любой ширины), цель-страница — только файл этой страницы.
func (pl Plan) hitsFile(f DirFile) bool {
	if pl.Full {
		return true
	}
	m := pl.mask[f.Spread]
	return m&maskSpread != 0 || f.Page == 1 && m&maskLeft != 0 || f.Page == 2 && m&maskRight != 0
}

// wants — нужна ли план-у фигура r (мм разворота) разворота spread (с 1): у цели-страницы
// это фигуры, заходящие на неё (через корешок — на обе).
func (pl Plan) wants(spread int, r layout.Rect, pageW float64) bool {
	m := pl.mask[spread]
	return m&maskSpread != 0 || m&maskLeft != 0 && r.X < pageW || m&maskRight != 0 && r.X+r.W > pageW
}

// SelectedPhotos — фото, которые попадут в выгрузку по плану (для проверки пропавших и цвета):
// в местах и на фонах. Фон цели-страницы считается, если его видимая часть заходит на неё;
// фон с Fade ≥ 1 не печатается и файл не читается — его пропажа экспорт не держит.
func (pl Plan) SelectedPhotos(v model.View, pageW float64) map[string]bool {
	out := map[string]bool{}
	for i, s := range v.Spreads {
		for k, id := range s.Photos {
			if k >= len(s.Rects) || pl.wants(i+1, s.Rects[k], pageW) {
				out[id] = true
			}
		}
		for _, b := range s.Backdrops {
			if b.Fade >= 1 {
				continue
			}
			if r, ok := b.Shown(); ok && pl.wants(i+1, r, pageW) {
				out[b.Photo] = true
			}
		}
	}
	return out
}

// ReplaceQuestion — вопрос о замене найденных файлов прежнего экспорта.
func ReplaceQuestion(pl Plan, hit []DirFile) string {
	if pl.Full {
		return fmt.Sprintf("В папке уже есть развороты прежнего экспорта: %d. Заменить?", len(hit))
	}
	var nums []int
	for _, f := range hit {
		if !slices.Contains(nums, f.Spread) {
			nums = append(nums, f.Spread)
		}
	}
	slices.Sort(nums)
	shown := make([]string, 0, 10)
	for _, n := range nums[:min(len(nums), 10)] {
		shown = append(shown, strconv.Itoa(n))
	}
	list := strings.Join(shown, ", ")
	if len(nums) > len(shown) {
		list += fmt.Sprintf(" …и ещё %d", len(nums)-len(shown))
	}
	word := "разворотов"
	if len(nums) == 1 {
		word = "разворота"
	}
	return fmt.Sprintf("Заменить файлы %s %s (%d шт.)?", word, list, len(hit))
}

// CheckCanvases — размер холста до выбора папки: обычный и, для «По страницам», холст из двух
// равных страниц 2·px(PageW) (на пиксель шире обычного: на границе лимита отказ иначе пришёл бы
// из RunJobs, уже после удаления прежних файлов).
func CheckCanvases(opt render.Options, pages bool) error {
	if err := CheckCanvas(opt); err != nil {
		return err
	}
	if pages {
		opt.Split = true
		return CheckCanvas(opt)
	}
	return nil
}

// ClampOptions приводит качество и dpi окна экспорта к допустимым: качество — одно из трёх,
// dpi — в пределах настроек альбома, 0 — dpi альбома. Диапазон здесь не трогаем: его разбирает ParseRange.
func ClampOptions(quality, dpi, albumDPI int) (q, d int) {
	if quality != 100 && quality != 80 {
		quality = 95
	}
	if dpi == 0 {
		dpi = albumDPI
	}
	dpi = model.ClampSettings(model.Settings{PageW: 300, PageH: 300, DPI: dpi}).DPI
	return quality, dpi
}

// PickPhotos оставляет в проекте только выбранные фото: отчёт о цвете считает по p.Photos.
func PickPhotos(p *model.Project, ids map[string]bool) *model.Project {
	sub := &model.Project{Photos: map[string]model.PhotoRef{}}
	for id := range ids {
		if ref, ok := p.Photos[id]; ok {
			sub.Photos[id] = ref
		}
	}
	return sub
}

// MissingPhotos — ошибка со списком пропавших файлов выбранных фото; nil — все на месте.
func MissingPhotos(p *model.Project, ids map[string]bool, exists func(string) bool) error {
	sub := PickPhotos(p, ids).Photos
	gone := library.MissingOf(sub, func(r model.PhotoRef) string { return r.Path }, exists)
	if len(gone) == 0 {
		return nil
	}
	names := make([]string, 0, 3)
	for _, id := range gone[:min(len(gone), 3)] {
		names = append(names, filepath.Base(sub[id].Path))
	}
	list := strings.Join(names, ", ")
	if len(gone) > len(names) {
		list += fmt.Sprintf(" …и ещё %d", len(gone)-len(names))
	}
	return fmt.Errorf("не найдены файлы фото: %d (%s); найдите их через «Недостающие фото» и повторите экспорт", len(gone), list)
}
