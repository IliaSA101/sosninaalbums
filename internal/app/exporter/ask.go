package exporter

import (
	"fmt"

	"github.com/IliaSA101/sosninaalbums/internal/export"
	"github.com/IliaSA101/sosninaalbums/internal/export/printshop"
)

// warnBeforeDir — вопросы «Да / Нет» до выбора папки (цвет, пустые развороты, поля, шрифты, обложка):
// ответ «нет» не должен стоить выбора папки и не удаляет файлов. drafts нужны потом, после папки
// (131); conv — для строки о цвете в итоге. ok = false — «нет» или ошибка окна.
func (s *Service) warnBeforeDir(pr *prepared) (drafts export.DraftSummary, conv map[string]int, ok bool, err error) {
	p, v, pl := pr.p, pr.v, pr.pl
	// Профили проверяем тоже до диалога папки.
	conv, other := s.h.ColorReport(export.PickPhotos(p, pr.sel))
	// Пустые развороты — по тому же доводу (спека 013).
	// Лист только с фоном альбома считается пустым, но белым он не выйдет (0.23-g3g4 G4-4);
	// обложку фон альбома не красит (098-10) — одна пустая обложка остаётся «белой».
	// Лист и обложка только с заготовками — не здесь, а в вопросе о заготовках после папки (0.28-131-19).
	drafts = export.DraftSummaryFor(v, &pl, pr.opt.PageW)
	blank, blankCover := drafts.TakeBlank(export.BlankSpreads(v, &pl), pr.cover != nil && export.BlankCover(v))
	white := (p.Settings.Bg == "" || p.Settings.Bg == "#ffffff") && (p.AlbumBg == nil || len(blank) == 0)
	for _, text := range []string{
		export.ColorWarning(other),
		export.BlankWarningCover(blank, blankCover, white),
		// Пустые поля подписей (030) и имена без фото (071): в печати они пусты. Только листы плана.
		export.FieldsWarning(export.PlanFields(v, &pl)),
		// Своих шрифтов нет на этом компьютере (082): текст выйдет шрифтом Montserrat.
		export.FontsWarning(export.PlanFonts(v, &pl)),
		coverQuestion(pr.pj, pr.cover, v),
	} {
		if text == "" {
			continue
		}
		if yes, err := s.h.Ask(text); !yes {
			return drafts, conv, false, err
		}
	}
	return drafts, conv, true, nil
}

// confirmDir — разбор выбранной папки dir до записи: чужие файлы с номерами — отказ, свои — вопрос о
// замене (кроме набора hits, о котором уже предупредил вопрос агента), посторонние файлы папки книги.
// fresh — папки ещё нет (создаётся после вопросов): читать нечего. mkBook — папка книги ещё не создана.
// stale — прежние файлы, которые удалятся после записи новых. ok = false — «нет».
func (s *Service) confirmDir(pr *prepared, dir string, fresh bool, mkBook string, hits []string) (stale []string, ok bool, err error) {
	pl, pj := pr.pl, pr.pj
	var ours, foreign []export.DirFile
	if !fresh { // новых папок ещё нет: читать нечего
		if ours, foreign, err = export.PlanDirPrefix(dir, pl.Prefix); err != nil {
			return nil, false, fmt.Errorf("не удалось прочитать папку экспорта: %w", err)
		}
	}
	ours, foreign = export.AddCoverFile(pl, dir, ours, foreign) // 00.jpg (1-00.jpg): наш заменяется, чужой — отказ
	if foreign = export.HitFiles(pl, foreign); len(foreign) > 0 {
		return nil, false, fmt.Errorf("В папке есть файлы с номерами разворотов (или %s) не из Sosnina Albums (%d шт., например %s). Выберите другую папку или уберите их.", pl.CoverName(), len(foreign), foreign[0].Name) //nolint:revive,staticcheck // текст уходит пользователю тостом как есть
	}
	if hit := export.HitFiles(pl, ours); len(hit) > 0 {
		// Тот же набор, что назвал вопрос («заменится»), — человек уже согласился; другой — спросим.
		if hits == nil || !sameNames(fileNames(hit), hits) {
			if yes, err := s.h.Ask(export.ReplaceQuestion(pl, hit)); !yes {
				return nil, false, err
			}
		}
		stale = export.StaleNames(hit, pl.Names(pr.n)) // удалятся после записи новых
	}
	if pj != nil && pr.cover == nil && mkBook == "" { // свой старый 00.jpg: загрузчик взял бы прежнюю обложку
		stale = append(stale, printshop.OldCover(dir, pj.prof.CoverName())...)
	}
	if pj != nil && mkBook == "" {
		if text := extraQuestion(pj.prof, dir, pl.Names(pr.n)); text != "" {
			if yes, err := s.h.Ask(text); !yes {
				return nil, false, err
			}
		}
	}
	return stale, true, nil
}
