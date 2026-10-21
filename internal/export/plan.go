package export

import (
	"fmt"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/geom"
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
	// Cover — обложка (00.jpg, 032); nil — не пишется. Задаётся WithCover.
	Cover *CoverJob
	// Prefix — перед именами файлов (081 №9: 1-00.jpg, 1-01.jpg у Фабрики Фотокниги); "" — прежние
	// имена. Задаётся WithPrefix.
	Prefix string
}

// WithPrefix ставит префикс имён плану, его заданиям и обложке: имена рождаются и узнаются
// (PlanDirPrefix, AddCoverFile, ReplaceQuestion) с одним префиксом.
func (pl Plan) WithPrefix(prefix string) Plan {
	pl.Prefix = prefix
	pl.Jobs = slices.Clone(pl.Jobs)
	for i := range pl.Jobs {
		pl.Jobs[i].Prefix = prefix
	}
	if pl.Cover != nil {
		pl = pl.WithCover(pl.Cover)
	}
	return pl
}

// CoverName — имя файла обложки плана: 00.jpg, с префиксом — 1-00.jpg.
func (pl Plan) CoverName() string { return pl.Prefix + CoverName }

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
	if pl.Cover != nil {
		out = append(out, pl.CoverName())
	}
	for _, j := range pl.Jobs {
		out = append(out, j.Names(total)...)
	}
	return out
}

// hitsFile — задевает ли файл папки цель плана: цель-разворот — любой файл с этим номером
// (NN.jpg, NN-1.jpg, NN-2.jpg, любой ширины), цель-страница — только файл этой страницы.
func (pl Plan) hitsFile(f DirFile) bool {
	if f.Spread == 0 { // обложка: PlanDir её не отдаёт, AddCoverFile — только при обложке в плане
		return pl.Cover != nil
	}
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
// в местах, на фонах, в своих стикерах и в рамочках-картинках мест с фото. Фон цели-страницы
// считается, если его видимая часть заходит на неё, стикер и рамочка — если описанная рамка; фон и
// стикер с Fade ≥ 1 не печатаются и файл не читается — их пропажа экспорт не держит. Рамочка
// пустого места не печатается (079).
func (pl Plan) SelectedPhotos(v model.View, pageW float64) map[string]bool {
	out := map[string]bool{}
	if pl.Cover != nil && v.Cover != nil {
		coverPhotos(v.Cover.Sheet, out)
	}
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
		for _, st := range s.Stickers {
			if shownOwn(st) && pl.wants(i+1, stickerAABB(st), pageW) {
				out[st.Photo] = true
			}
		}
		pl.borderPhotoWants(s, i+1, pageW, out)
	}
	return out
}

// shownOwn — свой стикер, который попадёт в печать (Fade < 1): его файл читается.
func shownOwn(st model.StickerView) bool { return st.Photo != "" && st.Fade < 1 }

// stickerAABB — описанная рамка стикера, повёрнутого вокруг центра, мм листа.
func stickerAABB(st model.StickerView) layout.Rect {
	return geom.RotRect{X: st.X, Y: st.Y, W: st.W, H: st.H, Rot: st.Rot}.AABB()
}

// coverPhotos — фото обложки, которые попадут в печать: места, видимые фоны, свои стикеры и
// рамочки-картинки мест с фото.
func coverPhotos(s model.SpreadView, out map[string]bool) {
	for _, id := range s.Photos {
		out[id] = true
	}
	for _, b := range s.Backdrops {
		if b.Fade < 1 {
			out[b.Photo] = true
		}
	}
	for _, st := range s.Stickers {
		if shownOwn(st) {
			out[st.Photo] = true
		}
	}
	borderPhotos(s, out)
}

// ReplaceQuestion — вопрос о замене найденных файлов прежнего экспорта; обложка (имя плана) — в тексте отдельно.
func ReplaceQuestion(pl Plan, hit []DirFile) string {
	var nums []int
	cover := false
	for _, f := range hit {
		if f.Spread == 0 {
			cover = true
		} else if !slices.Contains(nums, f.Spread) {
			nums = append(nums, f.Spread)
		}
	}
	files := len(hit)
	if cover {
		files--
	}
	if files == 0 { // только обложка
		return "В папке уже есть обложка прежнего экспорта (" + pl.CoverName() + "). Заменить?"
	}
	if pl.Full {
		if cover {
			return fmt.Sprintf("В папке уже есть обложка и развороты прежнего экспорта: %d. Заменить?", len(hit))
		}
		return fmt.Sprintf("В папке уже есть развороты прежнего экспорта: %d. Заменить?", len(hit))
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
	if cover {
		return fmt.Sprintf("Заменить обложку и файлы %s %s (%d шт.)?", word, list, len(hit))
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
// Своё из библиотеки (PhotoRef.Lib, 048) «Найти пропавшие фото…» не ищет: его строка — отдельно,
// с подсказкой загрузить тот же файл (он найдётся сам, как в MissingBar). Имени у своего нет:
// путь пуст, когда файла нет в библиотеке.
func MissingPhotos(p *model.Project, ids map[string]bool, exists func(string) bool) error {
	sub := PickPhotos(p, ids).Photos
	// Пустой путь (своё, которого нет в библиотеке) — пропавшее без вызова exists.
	all := library.MissingOf(sub, func(r model.PhotoRef) string { return r.Path },
		func(path string) bool { return path != "" && exists(path) })
	if len(all) == 0 {
		return nil
	}
	var gone []string
	onSticker, onBorder := stickerPhotos(p), ownBorders(p)
	var own [3]int // не найденное своё: фоны, стикеры, рамочки
	for _, id := range all {
		switch {
		case sub[id].Lib == "":
			gone = append(gone, id)
		case onSticker[id]:
			own[1]++
		case onBorder[id]:
			own[2]++
		default:
			own[0]++
		}
	}
	var parts []string
	if len(gone) > 0 {
		names := make([]string, 0, 3)
		for _, id := range gone[:min(len(gone), 3)] {
			names = append(names, filepath.Base(sub[id].Path))
		}
		list := strings.Join(names, ", ")
		if len(gone) > len(names) {
			list += fmt.Sprintf(" …и ещё %d", len(gone)-len(names))
		}
		parts = append(parts, fmt.Sprintf("не найдены файлы фото: %d (%s); найдите их: Файл ▸ Найти пропавшие фото…", len(gone), list))
	}
	if n := own[0] + own[1] + own[2]; n > 0 {
		parts = append(parts, fmt.Sprintf("нет в библиотеке этого компьютера: %d (%s); загрузите те же файлы в %s, они найдутся сами",
			n, libWhat(own), libWhere(own)))
	}
	return fmt.Errorf("%s — и повторите выгрузку", strings.Join(parts, "; "))
}

// stickerPhotos — ID фото в стикерах проекта (с обложкой): пропавшее своё у них называется
// «свой стикер», остальное своё — «свой фон».
func stickerPhotos(p *model.Project) map[string]bool {
	out := map[string]bool{}
	for _, s := range p.Sheets() {
		for _, st := range s.Stickers {
			if st.Photo != "" {
				out[st.Photo] = true
			}
		}
	}
	return out
}

// libKinds — виды своего: фон, стикер, рамочка. Единственное число — когда пропал один файл одного
// вида; несколько видов сразу — всегда множественное, как «свои фоны и стикеры».
var libKinds = [3]struct{ one, many, gallery string }{
	{"свой фон", "фоны", "фонов"},
	{"свой стикер", "стикеры", "стикеров"},
	{"своя рамочка", "рамочки", "рамочек"},
}

// joinRu — «а», «а и б», «а, б и в».
func joinRu(w []string) string {
	if len(w) < 2 {
		return strings.Join(w, "")
	}
	return strings.Join(w[:len(w)-1], ", ") + " и " + w[len(w)-1]
}

// libWhat — что пропало из библиотеки (own — число файлов по видам): «свой фон», «свои стикеры»,
// «свои фоны и стикеры», «свои фоны, стикеры и рамочки».
func libWhat(own [3]int) string {
	var many []string
	last := 0
	for i, n := range own {
		if n > 0 {
			many, last = append(many, libKinds[i].many), i
		}
	}
	if len(many) == 1 {
		if own[last] == 1 {
			return libKinds[last].one
		}
		return "свои " + many[0]
	}
	return "свои " + joinRu(many)
}

// libWhere — куда загрузить пропавшее: галерея того вида, где оно стоит.
func libWhere(own [3]int) string {
	var gal []string
	for i, n := range own {
		if n > 0 {
			gal = append(gal, libKinds[i].gallery)
		}
	}
	if len(gal) == 1 {
		return "галерею " + gal[0]
	}
	return "галереи " + joinRu(gal)
}
