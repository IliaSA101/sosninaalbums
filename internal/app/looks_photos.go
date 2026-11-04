package app

import (
	"context"
	"slices"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/library"
)

// Фото на листе макета (163 design §2): плитки и просмотр показывают серию на свободных фото
// ленты. Здесь только выбор кандидатов, сдвиг шага и файлы копий; сборка листа — album.PreviewTplWith,
// рисование и кэш — looks_media.go.

// lookMaxPhotos — кандидатов не больше: листу нужны десятки мест, лента бывает на десятки тысяч.
const lookMaxPhotos = 256

// lookPhotos — кандидаты на места листа по порядку ленты: свободные (не в альбоме, не на фоне, не
// отклонённые) — тем же freePick, что у автосборки агента; свободных нет — все фото без
// отклонённых; ленты нет — пусто. Занятые файлы — Editor.UsedFiles (не полный вид: на окно в
// 30 плиток это 30 видов под замком). Замок Editor берётся до a.mu (порядок замков, как agentFree).
func (a *App) lookPhotos(ed *album.Editor) []library.Photo {
	placed, onBg := ed.UsedFiles()
	marks := ed.Marks()
	a.mu.Lock()
	photos := slices.Clone(a.photos)
	a.mu.Unlock()
	bg := make([]string, 0, len(onBg))
	for id := range onBg {
		bg = append(bg, id)
	}
	ids, _ := freePick(photos, marks, placed, bg)
	if len(ids) == 0 {
		ids, _ = freePick(photos, marks, nil, nil)
	}
	ids = ids[:min(len(ids), lookMaxPhotos)]
	want := make(map[string]bool, len(ids))
	for _, id := range ids {
		want[id] = true
	}
	out := make([]library.Photo, 0, len(ids))
	for _, p := range photos {
		if want[p.ID] {
			out = append(out, p)
		}
	}
	return out
}

// lookSeats — места, которые рисует сцена у шага: у шаблона страницы — одной страницы (LookPlaces
// считает × 2, для зеркала на обе страницы альбома), у виньетки — 0 (её лист строит виньетка, не
// шаг: lookTile убирает места).
func lookSeats(st model.LookStep) int {
	if st.Role == model.RoleVignette {
		return 0
	}
	return len(st.Tpl.Places)
}

// lookBackShift — сдвиг задней стороны обложки внутри обложки: она берёт фото после мест лицевой
// (стороны рисуются слева направо — задняя, лицевая, но фото идут от лицевой).
func lookBackShift(c *model.CoverTpl) int {
	if c == nil || c.Front == nil {
		return 0
	}
	return len(c.Front.Places)
}

// lookOffset — с какого по счёту кандидата шаг i берёт фото (163-5): с места, где кончились
// предыдущие. Порядок показа: лицевая обложки — с 0, задняя после неё, затем шаги 1…n; каждый шаг
// считается один раз (шаг-повтор в ленте один). Индекс по модулю числа кандидатов берёт lookFiles.
// i = 0 (обложка) и вне 0…len(Steps) — 0 и все места.
func lookOffset(t model.AlbumTpl, i int) int {
	if i <= 0 {
		return 0
	}
	n := 0
	if c := t.Cover; c != nil {
		n = lookBackShift(c)
		if c.Back != nil {
			n += len(c.Back.Places)
		}
	}
	for k := 0; k < i-1 && k < len(t.Steps); k++ {
		n += lookSeats(t.Steps[k])
	}
	return n
}

// lookFiles — до need фото для мест листа по кругу от сдвига off: файл копии нужного размера из
// кэша (big — превью 2048, иначе миниатюра). Фото без файла (нет, том пропал, копию не сделать, нет
// размеров) пропускается; попыток не больше 2 × need — остальные места серые. Повтор того же фото
// получает ID копии «<ID>-n» (n 2…MaxCopy): FillPlaces различает фото по ID. Снятый запрос — его
// ошибка, без частичного ответа (в кэш серое не попадёт). Размер — ориентированный (Photo.W/H уже
// повёрнуты по EXIF), файл копии тоже: кадр совпадает с оригиналом.
func (a *App) lookFiles(ctx context.Context, list []library.Photo, off, need int, big bool) ([]model.PhotoInfo, error) {
	n := len(list)
	if a.cache == nil || n == 0 || need <= 0 {
		return nil, ctx.Err()
	}
	out := make([]model.PhotoInfo, 0, need)
	info := make([]model.PhotoInfo, n) // сведения о фото; Path "" — ещё не брали
	bad := make([]bool, n)
	used := make([]int, n)
	nBad := 0
	start := (off%n + n) % n
	for k := 0; k < 2*need && len(out) < need && nBad < n; k++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		i := (start + k) % n
		if bad[i] || used[i] >= model.MaxCopy {
			continue
		}
		if info[i].Path == "" {
			ph, ok, gone := a.mediaPhoto(list[i].ID)
			if !ok || gone || ph.W <= 0 || ph.H <= 0 {
				bad[i], nBad = true, nBad+1
				continue
			}
			file, err := a.cache.FileCtx(ctx, ph, big)
			if ctx.Err() != nil { // и когда копия успела доделаться: отвечать некому
				return nil, ctx.Err()
			}
			if err != nil {
				bad[i], nBad = true, nBad+1
				continue
			}
			info[i] = model.PhotoInfo{ID: ph.ID, Path: file, W: ph.W, H: ph.H}
		}
		used[i]++
		ph := info[i]
		ph.ID = model.CopyID(ph.ID, used[i]) // первый раз — сам ID, дальше «-2», «-3»…
		out = append(out, ph)
	}
	return out, nil
}

// lookTileFiles — фото на места каждой плитки tpls (результат lookTile) шага i: у обложки с двумя
// сторонами задняя берёт фото после мест лицевой (lookBackShift), лицевая — с 0; у шага — со
// сдвига lookOffset. Мест у плитки нет (фон, виньетка) — фото не берутся. Ошибка — только снятый
// запрос (см. lookFiles).
func (a *App) lookTileFiles(ctx context.Context, t model.AlbumTpl, tpls []model.SpreadTpl, i int, list []library.Photo, big bool) ([][]model.PhotoInfo, error) {
	out := make([][]model.PhotoInfo, len(tpls))
	for k, tpl := range tpls {
		off := lookOffset(t, i)
		if i == 0 && len(tpls) == 2 && k == 0 {
			off = lookBackShift(t.Cover)
		}
		files, err := a.lookFiles(ctx, list, off, len(tpl.Places), big)
		if err != nil {
			return nil, err
		}
		out[k] = files
	}
	return out, nil
}

// lookCandidates — кандидаты на места листа текущего альбома (lookPhotos); альбома нет — пусто.
func (a *App) lookCandidates() []library.Photo {
	if ed := a.editor(); ed != nil {
		return a.lookPhotos(ed)
	}
	return nil
}
