package exporter

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/export"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// Job — выгрузка по заданию: альбом, ревизия, настройки и откуда брать папку. Нужна агенту (152):
// вопрос показан для одного альбома и одной ревизии, а выгрузка идёт после щелчка.
type Job struct {
	// Ed — редактор выгрузки: снимок, типография, проверка 121 и план берутся из него и нигде
	// не из Host.Editor. nil — текущий (Export).
	Ed *album.Editor
	// Rev — View.Rev, показанный в вопросе; снимок с другим — ErrAlbumEdited, выгрузки нет.
	// −1 — без сверки.
	Rev int
	Opt Options
	// Name — имя книги типографии (черновик без файла, имя от агента); пусто — по файлу Ed.
	Name string
	// Pick — папка выгрузки; nil — окно выбора папки (Host.PickDir). create — папки ещё нет:
	// создаётся один последний уровень после всех вопросов; hits — свои файлы, о замене которых
	// вопрос уже предупредил: тот же набор — ReplaceQuestion не задаётся; nil — вопрос как в меню.
	Pick func(title string) (dir string, create bool, hits []string, err error)
	// Recheck — повторная сверка папки после последнего окна, прямо перед созданием папок и записью: dir —
	// папка выгрузки (книги). Ошибка — выгрузки нет, ничего не создано. nil — без сверки.
	Recheck func(dir string) error
}

// Export собирает развороты по настройкам окна экспорта: спрашивает папку и пишет в фоне.
// Ход работы приходит событиями export:progress и export:done; остановка — CancelExport.
func (s *Service) Export(o Options) (bool, error) { return s.ExportWith(Job{Rev: -1, Opt: o}) }

// ExportWith — Export по заданию j. Host.Editor зовётся только при j.Ed == nil и после того, как
// флаг busy поднят.
func (s *Service) ExportWith(j Job) (bool, error) {
	// Флаг ставим до диалога: пока он открыт, второй вызов (из браузера) не пройдёт.
	if !s.acquire() {
		return false, fmt.Errorf("экспорт уже идёт")
	}
	started := false
	defer func() {
		if !started {
			s.release()
		}
	}()
	if j.Ed == nil {
		j.Ed = s.h.Editor()
	}
	pick := j.Pick
	if pick == nil {
		pick = func(title string) (string, bool, []string, error) {
			d, err := s.h.PickDir(title)
			return d, false, nil, err
		}
	}

	// Всё, что может отказать, — до диалога папки и удаления прежних файлов: иначе отказ пришёл бы после потерь.
	pr, err := s.prepare(j.Ed, j.Opt, j.Name, j.Rev)
	if err != nil {
		return false, err
	}
	pj, pl := pr.pj, pr.pl
	drafts, conv, ok, err := s.warnBeforeDir(pr) // «нет» не должно стоить выбора папки
	if !ok || err != nil {
		return false, err
	}
	title := "Куда выгрузить"
	if pj != nil {
		title = "Куда выгрузить — папка книги будет внутри"
	}
	ch, ok, err := s.chooseDir(j, pick, title)
	if !ok || err != nil {
		return false, err
	}
	dir, mkBook, err := bookDir(pj, ch.dir) // дальше dir — папка книги; mkBook — создать её
	if err != nil {
		return false, err
	}
	// Пустые места и заготовки (131) после выбора папки, до вопроса о перезаписи: «нет» ничего не удаляет.
	if text := export.EmptyDraftWarning(drafts); text != "" {
		if ok, err := s.h.Ask(text); !ok {
			return false, err
		}
	}
	// Новых папок ещё нет — читать нечего.
	stale, ok, err := s.confirmDir(pr, dir, ch.mkOut != "" || mkBook != "" || ch.mkPick != "", mkBook, ch.hits)
	if !ok || err != nil {
		return false, err
	}
	if j.Recheck != nil {
		if err := j.Recheck(dir); err != nil {
			return false, err
		}
	}
	if err := makeDirs(ch.mkPick, ch.mkOut, mkBook); err != nil {
		return false, err
	}
	// Запоминаем после всех отказов и созданных папок: «нет» в окне корень не меняет.
	s.h.RememberDir(ch.dir, ch.root)
	var memo []string // памятка окна «Готово» типографии; nil — обычный тост
	if pj != nil {
		memo = pj.plan.Memo
	}
	spreads := export.FromView(pr.p, pr.v)
	export.PinFonts(spreads, pr.gen)
	rj := exportJob{spreads: spreads, pl: pl, opt: pr.opt, dir: dir, stale: stale, conv: conv, memo: memo}
	started = true
	return s.launch(j.Ed.ID(), pl, rj), nil
}

// chosen — выбранная папка выгрузки: dir (после «рядом» 121 — соседняя), корень выгрузок для
// RememberDir ("" — не менять), новые папки (mkPick — выбранная человеком, mkOut — соседняя) и
// набор своих файлов, о замене которых уже предупредил вопрос.
type chosen struct {
	dir, root     string
	mkPick, mkOut string
	hits          []string
}

// chooseDir спрашивает папку (pick), сверяет её с папками фото альбома (121: вопрос «выгрузить
// рядом?») и решает, какие папки создать после вопросов. ok = false — отказ или «нет».
func (s *Service) chooseDir(j Job, pick func(string) (string, bool, []string, error), title string) (ch chosen, ok bool, err error) {
	picked, create, hits, err := pick(title)
	if err != nil || picked == "" {
		return chosen{}, false, err
	}
	dir, mkOut, ok, err := s.outsideAlbum(j.Ed, picked)
	if !ok || err != nil {
		return chosen{}, false, err // «Нет» — err == nil: ничего не пишем и не создаём
	}
	ch = chosen{dir: dir, mkOut: mkOut, hits: hits}
	// Новая папка человека создаётся, только если «рядом» не увело в соседнюю: фото добавили между
	// вопросом и щелчком — тогда создаётся одна соседняя (mkOut), пустой папки среди фото нет.
	if create && dir == picked {
		ch.mkPick = dir
	}
	if dir == picked { // «рядом» (121) корень выгрузок не меняет
		ch.root = exportRoot(dir, ch.mkPick != "", albumName(j.Ed, j.Name))
	}
	return ch, true, nil
}

// launch запускает запись в фоне (флаг busy уже поднят): ход агента — «идёт» для альбома id. Фон
// закрыт (программа выходит) — выгрузки не будет, флаг не оставляем.
func (s *Service) launch(id uint64, pl export.Plan, rj exportJob) bool {
	ctx, cancel := context.WithCancel(s.h.StopCtx())
	s.mu.Lock()
	s.dir, s.cancel = rj.dir, cancel
	s.last = ExportState{AlbumID: id, State: StateRunning, Total: planTotal(pl)}
	s.mu.Unlock()
	if !s.h.GoBG(func() { s.runExport(ctx, cancel, rj) }) {
		cancel()
		s.endAs(StateStopped, "")
		return false
	}
	return true
}

// acquire поднимает флаг «выгрузка идёт»; false — уже идёт.
func (s *Service) acquire() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.busy {
		return false
	}
	s.busy = true
	return true
}

// release снимает флаг, если выгрузка не дошла до фона.
func (s *Service) release() {
	s.mu.Lock()
	s.busy = false
	s.mu.Unlock()
}

// makeDirs создаёт новые папки после всех вопросов, внешняя раньше внутренней: выбранная человеком,
// соседняя (121), книги. Появившаяся с момента вопроса — ошибка, а не запись в неразобранную папку.
func makeDirs(mkPick, mkOut, mkBook string) error {
	for _, mk := range []struct{ path, what string }{{mkPick, "папку"}, {mkOut, "папку выгрузки"}, {mkBook, "папку книги"}} {
		if mk.path == "" {
			continue
		}
		if err := os.Mkdir(mk.path, 0o755); err != nil {
			log.Printf("экспорт: не создана папка (%s): %v", mk.what, paths.NoPath(err))
			return fmt.Errorf("не удалось создать %s: %w", mk.what, err)
		}
	}
	return nil
}

// exportRoot — корень выгрузок после выгрузки в dir (152-15): папка, созданная выгрузкой (created)
// или названная как альбом, — «на альбом», корнем будет её родитель; иначе корень — сама папка.
func exportRoot(dir string, created bool, album string) string {
	if created || album != "" && strings.EqualFold(filepath.Base(dir), album) {
		return filepath.Dir(dir)
	}
	return dir
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
