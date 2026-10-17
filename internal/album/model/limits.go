package model

import (
	"fmt"
	"log"
	"math"
	"runtime"
	"runtime/debug"
	"slices"
	"sync"
	"sync/atomic"

	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/layout/variety"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// MaxPhotos — предел фото на разворот (спека 027: 64, с формата v6). Load режет
// больший разворот на части — поэтому поднимается только со сменой FormatVersion.
// Склейка и автосборка держатся той же границы.
const MaxPhotos = layout.MaxPhotos

// MaxFileSize — больше файл проекта не читаем: нормальный проект на тысячи
// фото занимает единицы мегабайт.
const MaxFileSize = 50 << 20

// MaxNamesBytes — предел поля списка имён (043): и вставка (names.MaxPasteBytes), и текст,
// который App отдаёт окну из таблицы. Лежит здесь, потому что app не импортирует album/names.
const MaxNamesBytes = 256 << 10

// MaxSpreads — разворотов в проекте: при открытии файла и в правках. Нормальный альбом — десятки;
// 200 (058, было 1000) держит худший прогрев раскладок в десятках секунд. Load больший файл не
// обрезает, а отказывает (salbum.errTooMany): файлы 0.13 до 1000 разворотов остаются целы.
const MaxSpreads = 200

// MaxMarks — своих отметок в проекте (004). Load обрезает лишние с потерей: поднимать только
// со сменой FormatVersion. marksLimit — то же, тест уменьшает.
const MaxMarks = 100_000

var marksLimit = MaxMarks

// MarksLimit — действующий предел своих отметок: MaxMarks, в тесте меньше.
func MarksLimit() int { return marksLimit }

// SetMarksLimit — подмена предела в тесте; рабочему коду линтер вызов запрещает (023).
func SetMarksLimit(n int) { marksLimit = n }

// MaxFolders — папок в проекте. Load отбрасывает лишние с потерей: поднимать только со сменой
// FormatVersion. Съёмка — единицы папок; 10 000 — запас без квадратичных проверок.
const MaxFolders = 10_000

func ErrTooMany() error {
	return fmt.Errorf("на развороте не может быть больше %d фото", MaxPhotos)
}

// Пределы настроек, мм и dpi.
const (
	minPage, maxPage = 50.0, 1000.0
	minDPI, maxDPI   = 72, 1200
)

// ClampSettings приводит настройки к допустимым: NaN/Inf — значение по умолчанию,
// остальное зажимается в пределы. Поля, промежуток и безопасная зона не больше четверти
// меньшей стороны страницы. Лист (Sheet) — всегда layflat: шва у него нет.
func ClampSettings(s Settings) Settings {
	d := DefaultSettings()
	fix := func(v, def, lo, hi float64) float64 {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			v = def
		}
		return math.Max(lo, math.Min(hi, v))
	}
	s.PageW = fix(s.PageW, d.PageW, minPage, maxPage)
	s.PageH = fix(s.PageH, d.PageH, minPage, maxPage)
	top := min(s.PageW, s.PageH) / 4
	s.Margin = fix(s.Margin, d.Margin, 0, top)
	s.Gap = fix(s.Gap, d.Gap, 0, top)
	s.Safe = fix(s.Safe, d.Safe, 0, top)
	if s.DPI == 0 {
		s.DPI = d.DPI
	}
	s.DPI = max(minDPI, min(maxDPI, s.DPI))
	s.Bg = BgColor(s.Bg)
	s.Layflat = s.Layflat || s.Sheet
	return s
}

// SplitBig режет разворот больше MaxPhotos на подряд идущие по ≤ MaxPhotos фото:
// альбомы, собранные до появления лимита, должны открываться. Раскладка такого
// разворота (ключ строился под другое число фото) сбрасывается; сетка с таким
// числом фото (чужой или битый файл) снимается — части становятся «Авто».
func (p *Project) SplitBig() {
	var out []*Spread
	for _, s := range p.Spreads {
		if s == nil || len(s.Photos) <= MaxPhotos {
			out = append(out, s)
			continue
		}
		for i := 0; i < len(s.Photos); i += MaxPhotos {
			photos := slices.Clone(s.Photos[i:min(len(s.Photos), i+MaxPhotos)])
			part := &Spread{ID: NewID(), Photos: photos}
			if i == 0 {
				// Первая часть — копия целиком (тексты, фон, название и будущие
				// поля, как при SplitSpread); обнуляется только то, что не годится.
				first := *s
				first.Photos, first.Grid, first.Layout, first.Variant = photos, nil, "", 0
				part = &first
			}
			out = append(out, part)
		}
	}
	p.Spreads = out
}

// DedupePhotos убирает повторы id с сохранением порядка: иначе повторы
// обходили бы лимит или ставили одно фото на разворот дважды.
func DedupePhotos(photos []PhotoInfo) []PhotoInfo {
	seen := make(map[string]bool, len(photos))
	out := make([]PhotoInfo, 0, len(photos))
	for _, ph := range photos {
		if !seen[ph.ID] {
			seen[ph.ID] = true
			out = append(out, ph)
		}
	}
	return out
}

// Раскладки разворотов независимы, а холодный перебор на 8–16 фото — десятки мс:
// открытие альбома и смена настроек считают их параллельно (design 020 §4.2).
// Воркеры живут только внутри вызова под e.mu и проект лишь читают; кэш layout —
// под своим замком.

// parallelLayouts — выключатель для теста: без него View считается как прежде, по порядку.
var parallelLayouts = true

// ParallelLayouts — считать ли раскладки параллельно (WarmLayouts, view).
func ParallelLayouts() bool { return parallelLayouts }

// SetParallelLayouts — выключатель в тесте; рабочему коду линтер вызов запрещает (023).
func SetParallelLayouts(on bool) { parallelLayouts = on }

// maxWorkers — больше воркеров раскладке почти не помогает, а пик памяти растёт.
const maxWorkers = 8

// WarmLayouts прогревает ряд «в порядке ленты» (layout.Cached), ряд «Авто» по формам (variety.Row)
// и «Другие варианты» (ExtraRow: счётчик «i / N» в View) у разворотов параллельно: следующий за
// ним последовательный код попадает в кэш. Паника — только в лог: тот же счёт повторится последовательно и поведёт себя как раньше.
func (p *Project) WarmLayouts(spreads []*Spread) {
	if !parallelLayouts {
		return
	}
	prm := p.Params()
	var pl PanicLog
	Each(len(spreads), func(i int) {
		s := spreads[i]
		pl.Safely(s.ID, func() {
			asp := p.Aspects(s.Photos)
			layout.Cached(asp, prm, RowLen(len(s.Photos)))
			variety.Row(asp, prm, RowLen(len(s.Photos)))
			ExtraRow(asp, prm)
		})
	})
	pl.Summary()
}

// Each зовёт f(0…n-1) на min(NumCPU, maxWorkers, n) горутинах; одно задание — без горутин.
// Паники f не ловит: это дело вызывающего (safely), иначе упадёт процесс.
func Each(n int, f func(i int)) {
	w := min(runtime.NumCPU(), maxWorkers, n)
	if w <= 1 {
		for i := range n {
			f(i)
		}
		return
	}
	var next atomic.Int64
	var wg sync.WaitGroup
	for range w {
		wg.Go(func() {
			for i := int(next.Add(1)) - 1; i < n; i = int(next.Add(1)) - 1 {
				f(i)
			}
		})
	}
	wg.Wait()
}

// PanicLog — паники воркеров одного вызова layAll/warmLayouts. Полный стек пишется только
// для первой: при детерминированном сбое такой же стек на каждый разворот — сотни КБ в лог.
type PanicLog struct{ n atomic.Int64 }

// Safely — счёт раскладки разворота spreadID под recover: паника уходит в лог.
func (l *PanicLog) Safely(spreadID string, f func()) {
	defer func() {
		if r := recover(); r != nil {
			if l.n.Add(1) == 1 {
				log.Printf("ПАНИКА в раскладке разворота %s: %v\n%s", paths.LogQ(spreadID), r, debug.Stack())
			}
		}
	}()
	f()
}

// Summary дописывает одной строкой число паник после первой (их стек не пишется).
func (l *PanicLog) Summary() {
	if k := l.n.Load() - 1; k > 0 {
		log.Printf("ПАНИКА в раскладке: ещё %d разворотов", k)
	}
}
