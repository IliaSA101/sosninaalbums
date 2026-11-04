package exporter

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/export"
	"github.com/IliaSA101/sosninaalbums/internal/export/printshop"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
	"github.com/IliaSA101/sosninaalbums/internal/render"
	"github.com/IliaSA101/sosninaalbums/internal/text"
)

// ErrAlbumEdited — альбом изменился с тех пор, как показан вопрос (Job.Rev не совпал со снимком):
// строки вопроса (число файлов, dpi) верны только для прежнего состояния. Выгрузки нет.
var ErrAlbumEdited = errors.New("альбом изменился")

// prepared — проверенная выгрузка до чтения фото и окон: снимок альбома, настройки после зажима,
// план и холст. Общая часть Check и ExportWith: то, что не должно ничего менять и ничего читать.
type prepared struct {
	p     *model.Project
	v     model.View
	gen   uint64 // поколение шрифтов до снимка (export.PinFonts)
	o     Options
	pj    *printJob // режим «для типографии» (047, 081); nil — обычная
	cover *export.CoverJob
	opt   render.Options
	pl    export.Plan
	sel   map[string]bool // фото плана
	n     int             // разворотов в альбоме
}

// albumPath — «файл альбома» для имени папки книги типографии: name от вызывающего (черновик
// без файла, имя от агента) важнее файла ed; пусто — как у меню.
func albumPath(ed *album.Editor, name string) string {
	if name != "" && paths.FolderName(name, "") == name {
		return name + ".salbum" // расширение защищает имена с точкой: Ext не съест кусок имени
	}
	return ed.Path()
}

// albumName — имя альбома для сравнения с папкой выгрузки: name или файл ed без расширения.
func albumName(ed *album.Editor, name string) string {
	if name != "" {
		return name
	}
	base := filepath.Base(ed.Path())
	if ed.Path() == "" {
		return ""
	}
	return strings.TrimSuffix(base, filepath.Ext(base))
}

// prepare — всё, что может отказать до окон и чтения фото (ColorReport): снимок ed, типография,
// зажим настроек, обложка, диапазон, холсты, пропавшие файлы фото. rev ≥ 0 сверяется со снимком
// сразу, до остального; −1 — без сверки. Служба и её флаг busy не трогаются.
func (s *Service) prepare(ed *album.Editor, o Options, name string, rev int) (*prepared, error) {
	if ed == nil {
		return nil, errors.New("нет открытого альбома")
	}
	gen := text.Gen() // до снимка: смена шрифтов между ними не проскочит (export.PinFonts)
	p, v := ed.Snapshot()
	if rev >= 0 && v.Rev != rev {
		return nil, ErrAlbumEdited
	}
	var pj *printJob // режим «для типографии» (047, 081): папка книги, имена и памятка
	if o.Print != "" {
		prof, ok := printshop.ProfileOf(o.Print)
		if !ok {
			return nil, errors.New("неизвестная типография")
		}
		var err error
		if pj, err = printOptions(p, albumPath(ed, name), &o, prof); err != nil {
			return nil, err
		}
	}
	o.Quality, o.DPI = export.ClampOptions(o.Quality, o.DPI, p.Settings.DPI)
	n := len(v.Spreads)
	// Обложка (032): нет её или проект-лист — флаги игнорируются; «только обложка» включает обложку.
	cover := export.NewCoverJob(p, v, o.DPI, o.Quality)
	if p.Settings.Sheet || (!o.Cover && !o.CoverOnly) {
		cover = nil
	}
	o.Cover, o.CoverOnly = cover != nil, o.CoverOnly && cover != nil
	if n == 0 && cover == nil {
		return nil, errors.New("в альбоме нечего экспортировать")
	}
	o.Pages = o.Pages && !p.Settings.Sheet // у листа 031 «отдельными страницами» нет: один файл на лист
	g := p.Geo(nil)                        // холст — половина листа (032 §7): у разворота страница, у листа 031 — W/2
	opt := render.Options{PageW: g.W / 2, PageH: g.H, DPI: o.DPI, Quality: o.Quality}
	if pj != nil { // формат типографии с px: файл разворота строго по цели (123 §5), обложка не подгоняется
		opt.FitW, opt.FitH = pj.plan.FitW, pj.plan.FitH
	}
	var pl export.Plan // «только обложка»: пустой план, не Full — хвосты разворотов не удаляются
	if !o.CoverOnly {
		units, err := exportUnits(o.Range, n, p.Settings.Sheet)
		if err != nil {
			return nil, err
		}
		pl = export.NewPlan(units, n, o.Pages)
		if err := export.CheckCanvases(opt, o.Pages); err != nil {
			return nil, err
		}
	}
	if cover != nil {
		if err := export.CheckCanvas(cover.Opt); err != nil {
			return nil, err
		}
		pl = pl.WithCover(cover)
	}
	if pj != nil { // имена типографии (1-01.jpg у Фабрики) — и в записи, и в разборе папки
		pl = pl.WithPrefix(pj.prof.Prefix)
	}
	sel := pl.SelectedPhotos(v, opt.PageW)
	if err := export.MissingPhotos(p, sel, s.h.Exists); err != nil {
		return nil, err
	}
	if err := pl.MissingBorderFiles(v, opt.PageW, s.h.Exists); err != nil {
		return nil, err
	}
	return &prepared{p: p, v: v, gen: gen, o: o, pj: pj, cover: cover, opt: opt, pl: pl, sel: sel, n: n}, nil
}

// Summary — что выгрузит Check: цифры и строки для вопроса агенту (152 §3.5). Скаляры и строки;
// план внутри нужен только Hits.
type Summary struct {
	Spreads   int  // единиц плана: разворотов (или разворотов по страницам); 0 — «только обложка»
	Cover     bool // обложка уйдёт в выгрузку
	CoverOnly bool
	DPI       int
	Quality   int
	Files     int      // сколько файлов запишет выгрузка
	Print     string   // Kind типографии; "" — обычная
	Book      string   // имя папки книги типографии внутри выбранной; "" — обычная выгрузка
	Lines     []string // строки вопроса: что выгрузится, режим, предупреждения окна «Экспорт»
	pl        export.Plan
}

// Check — первая часть Export без побочных эффектов: проверки окна «Экспорт» до чтения фото и
// вопросов, по редактору ed (а не текущему). Не трогает busy, папки и файлы фото; ошибки — те же
// тексты, что видит человек в окне. name — имя книги типографии (пусто — по файлу ed).
func (s *Service) Check(ed *album.Editor, o Options, name string) (Summary, error) {
	pr, err := s.prepare(ed, o, name, -1)
	if err != nil {
		return Summary{}, err
	}
	sm := Summary{Spreads: len(pr.pl.Jobs), Cover: pr.o.Cover, CoverOnly: pr.o.CoverOnly,
		DPI: pr.o.DPI, Quality: pr.o.Quality, Files: len(pr.pl.Names(pr.n)), pl: pr.pl}
	if pr.pj != nil {
		sm.Print, sm.Book = pr.pj.prof.Kind, pr.pj.plan.Folder
	}
	sm.Lines = summaryLines(pr, sm)
	return sm, nil
}

// summaryLines — строки вопроса: объём, режим, мало разрешения, предупреждения типографии и предел
// разворотов (его, как в окне, — только в обычной выгрузке).
func summaryLines(pr *prepared, sm Summary) []string {
	what := "Развороты"
	switch {
	case pr.p.Settings.Sheet:
		what = "Лист"
	case pr.n == 1:
		what = "Разворот"
	}
	var head string
	switch {
	case sm.CoverOnly:
		head = "Только обложка"
	case pr.pl.Full && pr.n == 1:
		head = what + " 1"
	case pr.pl.Full:
		head = what + " 1–" + strconv.Itoa(pr.n)
	default:
		rng := strings.Join(strings.Fields(pr.o.Range), " ")
		if !strings.ContainsAny(rng, ",-–—") && pr.n > 1 { // один разворот: «Разворот 2», а не «Развороты 2»
			what = "Разворот"
			if pr.p.Settings.Sheet {
				what = "Лист"
			}
		}
		head = what + " " + short(rng, 60)
	}
	if pr.o.Pages && !sm.CoverOnly {
		head += " по страницам"
	}
	if sm.Cover && !sm.CoverOnly {
		head += " и обложка"
	}
	lines := []string{fmt.Sprintf("%s · %d dpi · качество %d · %d %s", head, sm.DPI, sm.Quality, sm.Files,
		plural(sm.Files, "файл", "файла", "файлов"))}
	if pr.pj == nil {
		lines = append(lines, "Обычная выгрузка")
	} else {
		who := pr.pj.prof.Gen()
		if who == "" {
			who = pr.pj.prof.Title
		}
		lines = append(lines, "Для "+who+" — папка книги будет внутри")
	}
	if n := pr.v.LowDPI; n > 0 {
		lines = append(lines, fmt.Sprintf("Мало разрешения: %d фото", n))
	}
	if pr.pj != nil {
		lines = append(lines, pr.pj.plan.Warn...)
	} else if l := pr.v.PageLimit; l != nil && l.Warn != "" {
		lines = append(lines, l.Warn)
	}
	return lines
}

// Hits — файлы папки dir, которые выгрузка заменит (ours) или которым помешает (foreign, чужие с
// номерами разворотов): те же PlanDirPrefix, AddCoverFile и HitFiles, что в ExportWith — набор
// ours потом сверяется с тем, что показал вопрос. Читает каталог один раз; dir должна существовать.
func (sm Summary) Hits(dir string) (ours, foreign []string, err error) {
	o, f, err := export.PlanDirPrefix(dir, sm.pl.Prefix)
	if err != nil {
		return nil, nil, err
	}
	o, f = export.AddCoverFile(sm.pl, dir, o, f)
	return fileNames(export.HitFiles(sm.pl, o)), fileNames(export.HitFiles(sm.pl, f)), nil
}

// Own — свои файлы выгрузок в dir без фильтра плана (диапазон и «только обложка» не в счёт): по ним видно, что
// папка уже «на клиента». Чужие не считаются. Читает каталог один раз; dir должна существовать.
func (sm Summary) Own(dir string) ([]string, error) {
	o, f, err := export.PlanDirPrefix(dir, sm.pl.Prefix)
	if err != nil {
		return nil, err
	}
	o, _ = export.AddCoverFile(sm.pl, dir, o, f)
	return fileNames(o), nil
}

func fileNames(fs []export.DirFile) []string {
	out := make([]string, 0, len(fs))
	for _, f := range fs {
		out = append(out, f.Name)
	}
	return out
}

// sameNames — наборы имён совпали (порядок и повторы не важны); пустые наборы — совпали.
func sameNames(a, b []string) bool {
	x, y := slices.Clone(a), slices.Clone(b)
	slices.Sort(x)
	slices.Sort(y)
	return slices.Equal(slices.Compact(x), slices.Compact(y))
}

// short обрезает строку до n знаков с «…»: диапазон от агента в строку вопроса целиком не идёт.
func short(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// plural — форма слова при числе n: 1 файл, 2 файла, 5 файлов (11–14 — «много»).
func plural(n int, one, few, many string) string {
	if n < 0 {
		n = -n
	}
	n %= 100
	switch {
	case n >= 11 && n <= 14:
		return many
	case n%10 == 1:
		return one
	case n%10 >= 2 && n%10 <= 4:
		return few
	}
	return many
}
