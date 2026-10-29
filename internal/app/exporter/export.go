package exporter

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/export"
	"github.com/IliaSA101/sosninaalbums/internal/export/printshop"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
	"github.com/IliaSA101/sosninaalbums/internal/render"
	"github.com/IliaSA101/sosninaalbums/internal/text"
)

// Export собирает развороты по настройкам окна экспорта: спрашивает папку и пишет в фоне.
// Ход работы приходит событиями export:progress и export:done; остановка — CancelExport.
func (s *Service) Export(o Options) (bool, error) {
	// Флаг ставим до диалога: пока он открыт, второй вызов (из браузера) не пройдёт.
	s.mu.Lock()
	if s.busy {
		s.mu.Unlock()
		return false, fmt.Errorf("экспорт уже идёт")
	}
	s.busy = true
	s.mu.Unlock()
	started := false
	defer func() {
		if !started {
			s.mu.Lock()
			s.busy = false
			s.mu.Unlock()
		}
	}()

	// Всё, что может отказать, — до диалога папки и удаления прежних файлов: иначе отказ пришёл бы после потерь.
	gen := text.Gen() // до снимка: смена шрифтов между ними не проскочит (export.PinFonts)
	p, v := s.h.Editor().Snapshot()
	var pj *printJob // режим «для типографии» (047, 081): папка книги, имена и памятка
	if o.Print != "" {
		prof, ok := printshop.ProfileOf(o.Print)
		if !ok {
			return false, errors.New("неизвестная типография")
		}
		var err error
		if pj, err = printOptions(p, s.h.Editor().Path(), &o, prof); err != nil {
			return false, err
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
		return false, fmt.Errorf("в альбоме нечего экспортировать")
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
			return false, err
		}
		pl = export.NewPlan(units, n, o.Pages)
		if err := export.CheckCanvases(opt, o.Pages); err != nil {
			return false, err
		}
	}
	if cover != nil {
		if err := export.CheckCanvas(cover.Opt); err != nil {
			return false, err
		}
		pl = pl.WithCover(cover)
	}
	if pj != nil { // имена типографии (1-01.jpg у Фабрики) — и в записи, и в разборе папки
		pl = pl.WithPrefix(pj.prof.Prefix)
	}
	sel := pl.SelectedPhotos(v, opt.PageW)
	if err := export.MissingPhotos(p, sel, s.h.Exists); err != nil {
		return false, err
	}
	if err := pl.MissingBorderFiles(v, opt.PageW, s.h.Exists); err != nil {
		return false, err
	}
	// Профили проверяем тоже до диалога папки: ответ «нет» не должен стоить выбора папки.
	conv, other := s.h.ColorReport(export.PickPhotos(p, sel))
	if text := export.ColorWarning(other); text != "" {
		if ok, err := s.h.Ask(text); !ok {
			return false, err
		}
	}
	// Пустые развороты — тоже до папки, по тому же доводу (спека 013).
	// Лист только с фоном альбома считается пустым, но белым он не выйдет (0.23-g3g4 G4-4);
	// обложку фон альбома не красит (098-10) — одна пустая обложка остаётся «белой».
	// Лист и обложка только с заготовками — не здесь, а в вопросе о заготовках после папки (0.28-131-19).
	drafts := export.DraftSummaryFor(v, &pl, opt.PageW)
	blank, blankCover := drafts.TakeBlank(export.BlankSpreads(v, &pl), cover != nil && export.BlankCover(v))
	white := (p.Settings.Bg == "" || p.Settings.Bg == "#ffffff") && (p.AlbumBg == nil || len(blank) == 0)
	if text := export.BlankWarningCover(blank, blankCover, white); text != "" {
		if ok, err := s.h.Ask(text); !ok {
			return false, err
		}
	}
	// Пустые поля подписей (030) и имена без фото (071) — тоже до папки: в печати они пусты. Только листы плана.
	if text := export.FieldsWarning(export.PlanFields(v, &pl)); text != "" {
		if ok, err := s.h.Ask(text); !ok {
			return false, err
		}
	}
	// Своих шрифтов нет на этом компьютере (082) — тоже до папки: текст выйдет шрифтом Montserrat.
	if text := export.FontsWarning(export.PlanFonts(v, &pl)); text != "" {
		if ok, err := s.h.Ask(text); !ok {
			return false, err
		}
	}
	if text := coverQuestion(pj, cover, v); text != "" {
		if ok, err := s.h.Ask(text); !ok {
			return false, err
		}
	}
	title := "Куда выгрузить"
	if pj != nil {
		title = "Куда выгрузить — папка книги будет внутри"
	}
	dir, err := s.h.PickDir(title)
	if err != nil || dir == "" {
		return false, err
	}
	dir, mkOut, ok, err := s.outsideAlbum(dir) // 121: в папке фото альбома — вопрос «выгрузить рядом?»
	if !ok || err != nil {
		return false, err // «Нет» — err == nil: ничего не пишем и не создаём
	}
	s.h.RememberDir(dir)                 // с этого места dir — уже соседняя
	dir, mkBook, err := bookDir(pj, dir) // дальше dir — папка книги; mkBook — создать её
	if err != nil {
		return false, err
	}
	// Пустые места и заготовки (131) после выбора папки, до вопроса о перезаписи: «нет» ничего не удаляет.
	if text := export.EmptyDraftWarning(drafts); text != "" {
		if ok, err := s.h.Ask(text); !ok {
			return false, err
		}
	}

	var ours, foreign []export.DirFile
	if mkOut == "" && mkBook == "" { // новых папок ещё нет: читать нечего
		if ours, foreign, err = export.PlanDirPrefix(dir, pl.Prefix); err != nil {
			return false, fmt.Errorf("не удалось прочитать папку экспорта: %w", err)
		}
	}
	ours, foreign = export.AddCoverFile(pl, dir, ours, foreign) // 00.jpg (1-00.jpg): наш заменяется, чужой — отказ
	if foreign = export.HitFiles(pl, foreign); len(foreign) > 0 {
		return false, fmt.Errorf("В папке есть файлы с номерами разворотов (или %s) не из Sosnina Albums (%d шт., например %s). Выберите другую папку или уберите их.", pl.CoverName(), len(foreign), foreign[0].Name) //nolint:revive,staticcheck // текст уходит пользователю тостом как есть
	}
	var stale []string
	if hit := export.HitFiles(pl, ours); len(hit) > 0 {
		if ok, err := s.h.Ask(export.ReplaceQuestion(pl, hit)); !ok {
			return false, err
		}
		stale = export.StaleNames(hit, pl.Names(n)) // удалятся после записи новых
	}
	if pj != nil && cover == nil && mkBook == "" { // свой старый 00.jpg: загрузчик взял бы прежнюю обложку
		stale = append(stale, printshop.OldCover(dir, pj.prof.CoverName())...)
	}
	if pj != nil && mkBook == "" {
		if text := extraQuestion(pj.prof, dir, pl.Names(n)); text != "" {
			if ok, err := s.h.Ask(text); !ok {
				return false, err
			}
		}
	}
	if mkOut != "" { // соседняя папка выгрузки (121): раньше папки книги внутри неё
		if err := os.Mkdir(mkOut, 0o755); err != nil {
			log.Printf("экспорт: папка выгрузки: %v", paths.NoPath(err))
			return false, fmt.Errorf("не удалось создать папку выгрузки: %w", err)
		}
	}
	if mkBook != "" { // после всех вопросов: «нет» не оставляет пустой папки
		if err := os.Mkdir(mkBook, 0o755); err != nil {
			log.Printf("экспорт: папка книги: %v", paths.NoPath(err))
			return false, fmt.Errorf("не удалось создать папку книги: %w", err)
		}
	}
	var memo []string // памятка окна «Готово» типографии; nil — обычный тост
	if pj != nil {
		memo = pj.plan.Memo
	}

	spreads := export.FromView(p, v)
	export.PinFonts(spreads, gen)
	j := exportJob{spreads: spreads, pl: pl, opt: opt, dir: dir, stale: stale, conv: conv, memo: memo}
	ctx, cancel := context.WithCancel(s.h.StopCtx())
	started = true
	s.mu.Lock()
	s.dir, s.cancel = dir, cancel
	s.mu.Unlock()
	if !s.h.GoBG(func() { s.runExport(ctx, cancel, j) }) {
		cancel()
		s.end() // фон закрыт: выгрузки не будет, флаг не оставляем
		return false, nil
	}
	return true, nil
}

// exportUnits — цели диапазона окна экспорта; "" — nil (все развороты). У листа 031 «л»/«п» — ошибка.
func exportUnits(rng string, n int, sheet bool) ([]export.Unit, error) {
	if strings.TrimSpace(rng) == "" {
		return nil, nil
	}
	units, err := export.ParseRange(rng, n)
	switch {
	case err != nil && sheet:
		// ParseRange общий с альбомом и говорит «разворота 7 нет»; у листа 031 единица — лист.
		err = errors.New(strings.Replace(err.Error(), "разворота", "листа", 1))
	case err == nil && sheet:
		err = export.CheckSheet(units)
	}
	return units, err
}
