package exporter

import (
	"errors"
	"fmt"
	"log"
	"path/filepath"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/model/printprof"
	"github.com/IliaSA101/sosninaalbums/internal/export"
	"github.com/IliaSA101/sosninaalbums/internal/export/printshop"
)

// OpenPrintSite открывает сайт типографии id в браузере: страница изделия обложки альбома, если
// она этой типографии, иначе общий адрес типографии (123 Б1; заменяет OpenFsfera и OpenFabrika).
// Адрес — из профиля реестра, от интерфейса приходит только id (081 №11); перед открытием —
// та же printprof.CheckURL, что при разборе (правила Wails BrowserOpenURL), чтобы проверка была
// одна. Неизвестная типография и типография без выгрузки (Нетпринт) — ничего.
func (s *Service) OpenPrintSite(id string) {
	pf, ok := printshop.ProfileOf(id)
	if !ok {
		return
	}
	p, _ := s.h.Editor().SnapshotSide()
	switch u := pf.URL(p); {
	case u == "":
	case printprof.CheckURL(u) != nil:
		log.Printf("сайт типографии %q не открыт: адрес не прошёл проверку", id)
	default:
		s.h.OpenURL(u)
	}
}

// PrintPlans — готовность альбома к выгрузке для типографий: окно экспорта запрашивает их при
// открытии и при смене rev, флажок на план (081 №13). Страница не формат типографии — план всё
// равно отдаётся (Format "", Off и запрет в Block, 120): иначе запомненный выбор молча уходил
// в обычную выгрузку. Лист 031 — пустой список. last — последний выбор окна ("" — «Обычная»):
// printshop.DefaultKind отмечает пункт Default, с которого окно открывается (123 §1.5).
// Проект не меняется.
func (s *Service) PrintPlans(last string) []printshop.Plan {
	p, _ := s.h.Editor().SnapshotSide()
	return printshop.Plans(p, s.h.Editor().Path(), last)
}

// printJob — режим «для типографии» (047, 081): её профиль и план — папка книги и памятка.
type printJob struct {
	prof printshop.Profile
	plan printshop.Plan
}

// printOptions — режим типографии prof (047 §4.1): запрет — ошибка до всех вопросов, иначе
// настройки окна заменяются заданными: все развороты целиком, 300 dpi, с обложкой, качество
// типографии (Фабрика — 100, 081 №10).
func printOptions(p *model.Project, path string, o *Options, prof printshop.Profile) (*printJob, error) {
	pl := printshop.Check(p, path, prof)
	if len(pl.Block) > 0 {
		return nil, errors.New(strings.Join(pl.Block, ". "))
	}
	o.Range, o.Pages, o.DPI, o.Cover, o.CoverOnly = "", false, model.PrintDPI, true, false
	if prof.Quality > 0 {
		o.Quality = prof.Quality
	}
	return &printJob{prof: prof, plan: pl}, nil
}

// coverQuestion — вопрос об обложке до выбора папки (047): в режиме типографии её нет,
// в обычном — обложка шаблонной группы не по шаблону (CoverView.Fix).
func coverQuestion(pj *printJob, cover *export.CoverJob, v model.View) string {
	switch {
	case pj != nil && cover == nil:
		return pj.prof.NoCoverQuestion()
	case pj == nil && cover != nil && v.Cover != nil && v.Cover.Fix != "":
		return printshop.CoverFixQuestion(v.Cover.Fix)
	}
	return ""
}

// bookDir — папка книги в выбранной dir (режим типографии): mk — путь, если её надо создать
// (os.Mkdir после всех вопросов). Без режима — dir как есть.
func bookDir(pj *printJob, dir string) (book, mk string, err error) {
	if pj == nil {
		return dir, "", nil
	}
	book, exists, err := printshop.BookDir(dir, pj.plan.Folder)
	if err != nil || exists {
		return book, "", err
	}
	return book, book, nil
}

// extraQuestion — вопрос о посторонних файлах в уже существующей папке книги: загрузчик
// типографии возьмёт их вместе с альбомом (решение 59).
func extraQuestion(prof printshop.Profile, dir string, written []string) string {
	n, first := printshop.Extra(dir, written, prof.Prefix)
	if n == 0 {
		return ""
	}
	return fmt.Sprintf("В папке «%s» есть посторонние файлы (%d шт., например %s): %s загрузит их вместе с альбомом. Продолжить?",
		filepath.Base(dir), n, first, prof.Group)
}
