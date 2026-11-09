package sharer

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/IliaSA101/sosninaalbums/api/albums"
	"github.com/IliaSA101/sosninaalbums/internal/export"
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
	base          *prefs.ProjectRecord // продолжаем эту запись (та же ссылка); nil — новая ссылка
	moved         bool                 // «Обновить этот альбом» с другого пути (167-18): 404 — остановка, не новая ссылка
	dir           string               // временная папка страниц
	run           *runState            // флаг этой выгрузки: сверка записи с OnSaved
	pages         []export.WebPage     // готовые страницы и их манифест (первой подготовки): для keep (159)
	man           *albums.Manifest
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
	rec, had, own, kind, fresh, err := s.decide(j, o)
	if err != nil {
		return false, err
	}
	if ok, err := s.askLink(j, rec, had, own, kind); !ok || err != nil {
		return false, err
	}
	if snap.Questions != nil {
		for _, q := range snap.Questions() {
			if ok, err := s.h.Ask(q); !ok || err != nil {
				return false, err
			}
		}
	}
	if j.moved && !s.sameRecord(snap, rec) { // выбор «обновить» относится к альбому и записи, что видел фотограф
		return false, fail(changedText)
	}
	// Отпечаток — в файл до первой записи о ссылке (альбом до 0.32); «как новый» — новый отпечаток.
	if j.UID, err = s.h.Pin(snap.Path, snap.UID, fresh); err != nil {
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
		text := s.netText
		s.mu.Unlock()
		return nil, fail(text)
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

// Отказы выгрузке, пока флаг net у правки ссылки и у поиска альбома (holdNet).
const (
	netBusyText  = "Идёт изменение ссылки — дождитесь его конца"
	findBusyText = "Идёт поиск альбома на сервисе — дождитесь его конца"
)

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
// OnSaved по path ставит netStop. Выгрузка идёт — отказ. text — что ответит Share, пока флаг стоит
// (правка и поиск называются по-разному). Вызывающий отпускает флаг через release.
func (s *Service) holdNet(path, text string) (release func(), err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch {
	case s.run != nil:
		return nil, fail("Идёт выгрузка — дождитесь её конца")
	case s.net:
		return nil, fail(s.netText)
	}
	s.net, s.netPath, s.netStop, s.netText = true, path, false, text
	return func() { s.mu.Lock(); s.net, s.netPath, s.netStop, s.netText = false, "", false, ""; s.mu.Unlock() }, nil
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

// decide — что делает выгрузка с записью проекта: поиск до вопросов, новая ссылка или продолжение,
// проверка выбора окна (167-8, 167-12, 167-16). fresh — Pin выдаст новый отпечаток.
func (s *Service) decide(j *job, o Options) (rec prefs.ProjectRecord, had, own bool, kind share.Kind, fresh bool, err error) {
	snap := j.Job
	_, had, own, kind = s.classify(snap, j.origin)
	if fresh, err = s.searchFirst(snap, j.origin, j.token, had, own, kind); err != nil {
		return rec, had, own, kind, false, err
	}
	rec, had, own, kind = s.classify(snap, j.origin) // поиск мог обновить свою запись (167-19)
	// Новая ссылка с новым отпечатком: поиск не нашёл прежний альбом (167-17), запись другого сервиса
	// или та, что не живёт на сервере, с другого пути (167-12), «выгрузить как новый» на moved (167-16).
	fresh = fresh || had && !own && kind != stateMoved || kind == stateMoved && o.Choice == choiceNew
	return rec, had, own, kind, fresh, s.choose(j, o, rec, had, own, kind)
}

// Значения Options.Choice.
const (
	choiceUpdate = "update"
	choiceNew    = "new"
)

// changedText — альбом или его запись сменились между выбором и стартом.
const changedText = "Альбом сменился — начните выгрузку ещё раз"

// sameRecord — запись проекта снимка всё та же (тот же альбом на сервисе).
func (s *Service) sameRecord(snap share.Job, rec prefs.ProjectRecord) bool {
	path, uid := s.h.Ident()
	cur, had := s.look(snap.UID)
	return s.h.Fold(path) == s.h.Fold(snap.Path) && uid == snap.UID && had && cur.ID == rec.ID
}

// searchFirst — «Выгрузить» с привязками в файле и без поиска ищет сам до вопросов (167-8): нашёлся —
// отказ «выберите в окне»; не нашёлся или поиск не удался — fresh: выгрузка с новым отпечатком
// (167-17). Нужен проекту без записи, у своей записи — только при истёкшем сроке (167-19).
func (s *Service) searchFirst(snap share.Job, origin, token string, had, own bool, kind share.Kind) (fresh bool, err error) {
	if len(snap.Refs) == 0 {
		return false, nil
	}
	need, _ := s.findView(snap.UID, had, own, kind)
	st, searched := s.findOf(snap.UID)
	if need == findNeed {
		if st, err = s.search(snap, origin, token); err != nil {
			return false, err
		}
		searched = true
		s.setFind(snap.UID, st)
		if _, again := s.look(snap.UID); again && !had { // нашёлся: окно спросит «обновить или как новый»
			return false, fail("Этот альбом уже выгружен — выберите в окне: обновить его или выгрузить как новый")
		}
	}
	return !had && searched && (st.kind == findLost || st.kind == findFail), nil
}

// choose проверяет выбор окна и заполняет задание: название, пароль, срок и режим (167-16).
// choice допустим только на moved: на своей записи «как новый» молча бросил бы живую ссылку.
func (s *Service) choose(j *job, o Options, rec prefs.ProjectRecord, had, own bool, kind share.Kind) error {
	switch {
	case kind == stateMoved && o.Choice != choiceUpdate && o.Choice != choiceNew:
		return fail(chooseText)
	case kind != stateMoved && o.Choice != "":
		return fail("Выбор «обновить или выгрузить как новый» нужен только для перенесённого файла")
	}
	use := had && (own || o.Choice == choiceUpdate) // чужая запись не даёт ни названия, ни пароля
	if err := s.options(j, o, rec, use); err != nil {
		return err
	}
	if o.Choice == choiceUpdate {
		j.moved = true
	}
	return nil
}

// options проверяет выбор окна и дополняет его записью: название, пароль и срок обновления.
// Запись (use) — своя или выбранная для обновления; чужая сюда не приходит (167-12).
func (s *Service) options(j *job, o Options, rec prefs.ProjectRecord, use bool) error {
	if o.Days != 0 && !validDays[o.Days] {
		return fail("Срок ссылки: месяц, 3 месяца или год")
	}
	j.title, j.password, j.days = strings.TrimSpace(o.Title), o.Password, o.Days
	if use {
		recTitle := s.titleFor(rec, j.Job)
		if o.Days == 0 { // продолжить / обновить с параметрами записи: поля окна игнорируются (T5-6)
			j.title, j.password = recTitle, rec.Password
		}
		if j.title == "" {
			j.title = recTitle
		}
		if rec.Origin == j.origin && !rec.Gone && o.Password == "" {
			j.password = rec.Password // «Обновить по ссылке» не снимает пароль; снять — Change
		}
	} else if j.title == "" {
		j.title = j.Title
	}
	if j.password != "" {
		if err := share.CheckPassword(j.password); err != nil {
			return err
		}
	}
	return nil
}

// titleFor — название обновляемого альбома (167-7): было именем файла — теперь имя этого файла,
// введённое руками остаётся прежним. У записи без пути (найдена на сервисе) «имя файла» — название
// задания.
func (s *Service) titleFor(rec prefs.ProjectRecord, job share.Job) string {
	clean := func(t string) string { return share.CleanTitle(t, 0) }
	if rec.TitleAuto(clean) || rec.Path == "" && rec.TitleIs(job.Title, clean) {
		return job.Title
	}
	return rec.Title
}

// askLink решает, продолжать ли прежнюю запись. Запись другого сервиса или та, что уже не живёт
// на сервере (срок прошёл, отозвана, 404), — только новой ссылкой и только после вопроса; запросы по
// чужой записи не идут (С2). moved: вопрос уже задан окном (167-5) — «обновить» продолжает прежний
// альбом, «как новый» — новая ссылка. Нет записи — новая ссылка без вопроса.
func (s *Service) askLink(j *job, rec prefs.ProjectRecord, had, own bool, kind share.Kind) (bool, error) {
	var q string
	switch {
	case kind == stateMoved && j.moved:
		j.base = &rec
		return true, nil
	case kind == share.StateOther:
		q = fmt.Sprintf("Альбом уже выгружен на другой сервис (%s). Выгрузить на %s с новой ссылкой? "+lostLink, hostOf(rec.Origin), hostOf(j.origin))
	case kind == share.StateExpired || kind == share.StateGone:
		q = goneQuestion
	case had && own:
		j.base = &rec
		return true, nil
	}
	if err := needDays(j); err != nil || q == "" {
		return err == nil, err
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

const goneQuestion = "Прежней ссылки больше нет на сервере (истёк срок или её отозвали). Выгрузить с новой ссылкой?"

// OnSaved — альбом сохранён по пути path (вызывает App после записи). newProject — по этому пути
// записан другой проект («Сохранить как» поверх файла, первое сохранение нового альбома поверх
// существующего): идущая выгрузка и правка этого пути останавливаются, сохранённые страницы
// сбрасываются. Записи о ссылках не стираются (167-10): они по uid, другой проект на пути имеет
// другой uid, а запись прежнего проекта к файлу с другим uid не относится.
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
	s.dropKeptFor(path) // страницы прежнего проекта не годятся новому (159)
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
		if err := os.RemoveAll(dir); err != nil {
			log.Printf("выгрузка: временная папка не удалена") // без пути (личные данные)
		}
	}
}

// CleanTemp при запуске программы убирает оставшиеся от прошлых выгрузок папки <cache>/share/up-*
// старше суток и все kp-* (страницы «Продолжить» живут до конца сессии, 159). Идёт по Lstat: ссылки и обычные файлы не трогаются, чужие имена тоже (разбор С5).
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
		up, kp := strings.HasPrefix(e.Name(), tempPrefix), strings.HasPrefix(e.Name(), keptPrefix)
		if !up && !kp {
			continue
		}
		p := filepath.Join(root, e.Name())
		st, err := os.Lstat(p)
		if err != nil || !st.IsDir() || st.Mode()&os.ModeSymlink != 0 {
			continue
		}
		if up && now.Sub(st.ModTime()) < tempKeep { // kp-* — сохранённые страницы прошлой сессии: без срока (159)
			continue
		}
		if err := os.RemoveAll(p); err != nil {
			log.Printf("выгрузка: старая временная папка не удалена")
		}
	}
}
