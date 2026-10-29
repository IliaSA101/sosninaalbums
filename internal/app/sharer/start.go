package sharer

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/prefs"
	"github.com/IliaSA101/sosninaalbums/internal/share"
)

// Сроки ссылки в окне (Options.Days, Change.Days).
var validDays = map[int]bool{30: true, 90: true, 365: true}

const (
	tempPrefix = "up-"
	tempKeep   = 24 * time.Hour
)

// job — проверенный запуск: снимок альбома, подключение и то, что выбрал фотограф.
type job struct {
	share.Job
	origin, token string
	title         string
	password      string
	days          int
	base          *prefs.ShareRecord // продолжаем эту запись (та же ссылка); nil — новая ссылка
	dir           string             // временная папка страниц
	run           *runState          // флаг этой выгрузки: сверка записи с OnSaved
}

// Share — «Выгрузить» / «Обновить по ссылке» / «Продолжить» (design §3.3): проверки и вопросы
// здесь, остальное — в фоне, ход приходит событиями share:progress и share:done. false, nil — ответ
// «Нет» на вопрос: ничего не отправлено и не создано.
func (s *Service) Share(o Options) (bool, error) {
	run, err := s.begin()
	if err != nil {
		return false, err
	}
	started := false
	defer func() {
		if !started {
			s.end(run)
		}
	}()

	// Снимок хода (разбор В1): путь записи, хэш сцены и страницы — от одного состояния альбома;
	// дальше служба «текущий альбом» не спрашивает.
	snap := s.h.Plan()
	if snap.Block != "" {
		return false, fail(snap.Block)
	}
	if snap.Refuse != "" {
		return false, fail(snap.Refuse)
	}
	s.mu.Lock()
	run.path = snap.Path // как в begin, но от снимка: дальше только он
	s.mu.Unlock()
	origin, token, err := s.need()
	if err != nil {
		return false, err
	}
	j := &job{Job: snap, origin: origin, token: token, run: run}
	rec, had, foreign := s.get(snap.Path, ownUID(snap)) // чужая запись (T4-9): ни названия, ни пароля из неё
	if err := s.options(j, o, rec, had); err != nil {
		return false, err
	}
	if ok, err := s.askLink(j, rec, had, foreign); !ok || err != nil {
		return false, err
	}
	if snap.Questions != nil {
		for _, q := range snap.Questions() {
			if ok, err := s.h.Ask(q); !ok || err != nil {
				return false, err
			}
		}
	}
	// Отпечаток — в файл до первой записи о ссылке: иначе после перезапуска альбом до 0.32 получил
	// бы новый, и своя запись стала бы чужой (T4-9). Печать не этого пути — новый отпечаток.
	if j.UID, err = s.h.Pin(snap.Path, snap.UID); err != nil {
		return false, fail("Не удалось записать альбом перед выгрузкой: " + err.Error())
	}
	if j.dir, err = s.tempDir(); err != nil {
		return false, err
	}
	ctx, cancel := context.WithCancel(s.h.StopCtx())
	s.mu.Lock()
	if run.stop { // пока шли вопросы, на путь легло сохранение другого проекта: снимок устарел
		s.mu.Unlock()
		cancel()
		removeDir(j.dir)
		return false, fail("Альбом сохранён заново, пока шли вопросы — начните выгрузку ещё раз")
	}
	run.cancel = cancel
	s.mu.Unlock()
	started = true
	if !s.h.GoBG(func() { s.runShare(ctx, cancel, run, j) }) {
		cancel()
		removeDir(j.dir)
		s.end(run) // фон закрыт: выгрузки не будет, флаг не оставляем
		return false, nil
	}
	return true, nil
}

// begin ставит флаг «идёт» до вопросов: пока диалог открыт, второй вызов не пройдёт.
func (s *Service) begin() (*runState, error) {
	if s.h.ExportBusy() {
		return nil, fail("Сейчас идёт экспорт — дождитесь его конца")
	}
	path, _ := s.h.Ident() // дешёвый путь: до Plan, чтобы свой альбом не принимался за чужой
	s.mu.Lock()
	switch {
	case s.net:
		s.mu.Unlock()
		return nil, fail(netBusyText)
	case s.run != nil:
		other := s.otherLocked(path)
		s.mu.Unlock()
		if other {
			return nil, fail("Идёт выгрузка другого альбома — дождитесь её конца")
		}
		return nil, fail("Выгрузка уже идёт")
	}
	run := &runState{path: path}
	s.run = run
	s.mu.Unlock()
	return run, nil
}

const netBusyText = "Идёт изменение ссылки — дождитесь его конца"

// otherAlbum — идущая выгрузка не альбома path.
func (s *Service) otherAlbum(path string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.otherLocked(path)
}

// otherLocked — otherAlbum под mu. Выгрузка идёт, а у текущего альбома нет пути — он чужой.
func (s *Service) otherLocked(path string) bool {
	return s.run != nil && (path == "" || s.h.Fold(s.run.path) != s.h.Fold(path))
}

// holdNet занимает флаг правки или отзыва ссылки альбома path: пока он стоит, Share не стартует,
// и его последний put не перетрёт запись, которую меняет Change (или не воскресит отозванную);
// OnSaved по path ставит netStop. Выгрузка идёт — отказ. Вызывающий отпускает флаг через release.
func (s *Service) holdNet(path string) (release func(), err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch {
	case s.run != nil:
		return nil, fail("Идёт выгрузка — дождитесь её конца")
	case s.net:
		return nil, fail(netBusyText)
	}
	s.net, s.netPath, s.netStop = true, path, false
	return func() { s.mu.Lock(); s.net, s.netPath, s.netStop = false, "", false; s.mu.Unlock() }, nil
}

// end снимает флаг выгрузки, если он ещё принадлежит run (после события share:done флаг может
// быть уже у новой выгрузки).
func (s *Service) end(run *runState) {
	s.mu.Lock()
	if s.run == run {
		s.run = nil
	}
	s.mu.Unlock()
}

// options проверяет выбор окна и дополняет его записью: название, пароль и срок обновления.
func (s *Service) options(j *job, o Options, rec prefs.ShareRecord, had bool) error {
	if o.Days != 0 && !validDays[o.Days] {
		return fail("Срок ссылки: месяц, 3 месяца или год")
	}
	j.title, j.password, j.days = strings.TrimSpace(o.Title), o.Password, o.Days
	if o.Days == 0 && had { // продолжить / обновить с параметрами записи: поля окна игнорируются (T5-6)
		j.title, j.password = rec.Title, rec.Password
	}
	if j.title == "" {
		j.title = j.Title
		if had {
			j.title = rec.Title
		}
	}
	if had && rec.Origin == j.origin && !rec.Gone && o.Password == "" {
		j.password = rec.Password // «Обновить по ссылке» не снимает пароль; снять — Change
	}
	if j.password != "" {
		if err := share.CheckPassword(j.password); err != nil {
			return err
		}
	}
	return nil
}

// askLink решает, продолжать ли прежнюю запись. Запись другого сервиса или та, что уже не живёт
// на сервере (срок прошёл, отозвана, 404), — только новой ссылкой и только после вопроса; запросы по
// чужой записи не идут (С2). Запись другого проекта на этом пути (foreign, T4-9) — тоже новая
// ссылка после вопроса. Нет записи — новая ссылка без вопроса.
func (s *Service) askLink(j *job, rec prefs.ShareRecord, had, foreign bool) (bool, error) {
	if !had {
		if err := needDays(j); err != nil || !foreign {
			return err == nil, err
		}
		return s.h.Ask(foreignQuestion)
	}
	var q string
	k := share.State(toRecord(rec), j.origin, j.Scene, s.now())
	if k == share.StateNone && rec.ID != "" && rec.Origin != j.origin {
		k = share.StateOther // id без ссылки на старом сервисе: State даёт «нет ссылки», но продолжать её нельзя (T4-7)
	}
	switch k {
	case share.StateOther:
		q = fmt.Sprintf("Альбом уже выгружен на другой сервис (%s). Выгрузить на %s с новой ссылкой? "+lostLink, hostOf(rec.Origin), hostOf(j.origin))
	case share.StateExpired, share.StateGone:
		q = goneQuestion
	default:
		j.base = &rec
		return true, nil
	}
	if err := needDays(j); err != nil {
		return false, err
	}
	return s.h.Ask(q)
}

// needDays — у новой ссылки срок обязателен: 0 («не менять») годится только для продолжения прежней.
func needDays(j *job) error {
	if j.days == 0 {
		return fail("Выберите срок ссылки: месяц, 3 месяца или год")
	}
	return nil
}

// lostLink — что станет с прежней ссылкой после новой: запись о ней затирается (upload.go), сама
// ссылка на сервере живёт до своего срока.
const lostLink = "Прежняя ссылка будет работать до конца срока, но программа перестанет её показывать и не сможет отозвать."

const foreignQuestion = "Ссылка, записанная у этого файла, принадлежит другому альбому (файл заменён или восстановлен из копии). Выгрузить этот альбом с новой ссылкой? " + lostLink

const goneQuestion = "Прежней ссылки больше нет на сервере (истёк срок или её отозвали). Выгрузить с новой ссылкой?"

// OnSaved — альбом сохранён по пути path (вызывает App после записи). newProject — по этому пути
// записан другой проект («Сохранить как» поверх файла, первое сохранение нового альбома поверх
// существующего): запись о выгрузке пути сбрасывается, это новая публикация (разбор С1; иначе
// «Обновить по ссылке» залил бы фото нового клиента в ссылку прежнего). Идущая выгрузка этого
// пути останавливается.
func (s *Service) OnSaved(path string, newProject bool) {
	if !newProject || path == "" {
		return
	}
	s.mu.Lock()
	if s.net && s.netPath != "" && s.h.Fold(s.netPath) == s.h.Fold(path) {
		s.netStop = true // PATCH в пути: его поздняя запись не вернёт запись прежнего альбома
	}
	var cancel context.CancelFunc
	if s.run != nil && s.run.path != "" && s.h.Fold(s.run.path) == s.h.Fold(path) {
		s.run.stop = true // вопросы ещё идут (cancel == nil): старт после них откажет, put не пишет
		cancel = s.run.cancel
	}
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if err := s.drop(path); err != nil {
		log.Printf("выгрузка: запись пути не сброшена при сохранении: %v", err)
	}
}

// tempDir — новая папка страниц <кэш программы>/share/up-*, 0700. Нет абсолютной папки кэша —
// отказ: путь от текущей папки процесса недопустим (разбор С5).
func (s *Service) tempDir() (string, error) {
	base := s.h.CacheDir()
	if base == "" || !filepath.IsAbs(base) {
		return "", fail("Нет папки кэша — выгрузить альбом нельзя")
	}
	root := filepath.Join(base, "share")
	if err := os.MkdirAll(root, 0o700); err != nil {
		log.Printf("выгрузка: папка кэша не создана")
		return "", fail("Не удалось создать временную папку для страниц")
	}
	dir, err := os.MkdirTemp(root, tempPrefix+"*")
	if err != nil {
		log.Printf("выгрузка: временная папка не создана")
		return "", fail("Не удалось создать временную папку для страниц")
	}
	return dir, nil
}

// removeDir удаляет временную папку хода.
func removeDir(dir string) {
	if dir != "" {
		_ = os.RemoveAll(dir)
	}
}

// CleanTemp при запуске программы убирает оставшиеся от прошлых выгрузок папки <cache>/share/up-*
// старше суток. Идёт по Lstat: ссылки и обычные файлы не трогаются, чужие имена тоже (разбор С5).
// cache — папка кэша программы; пустая или не абсолютная — ничего.
func CleanTemp(cache string, now time.Time) {
	if cache == "" || !filepath.IsAbs(cache) {
		return
	}
	root := filepath.Join(cache, "share")
	list, err := os.ReadDir(root)
	if err != nil {
		return
	}
	for _, e := range list {
		if !strings.HasPrefix(e.Name(), tempPrefix) {
			continue
		}
		p := filepath.Join(root, e.Name())
		st, err := os.Lstat(p)
		if err != nil || !st.IsDir() || st.Mode()&os.ModeSymlink != 0 || now.Sub(st.ModTime()) < tempKeep {
			continue
		}
		if err := os.RemoveAll(p); err != nil {
			log.Printf("выгрузка: старая временная папка не удалена")
		}
	}
}
