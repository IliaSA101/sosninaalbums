package ownprint

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/model/printprof"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// Файл типографии .saprint (123 Б1, design §4, §6): «Добавить из файла…» и «Поделиться файлом…».
// Файл — чужие данные: путь через paths.CheckLocalPath и StatLocal, предел 64 КБ до чтения,
// разбор строгий (printprof.Parse). Образец вопроса о замене — AddPackFile/ConfirmPack.

// Ext — расширение файла типографии.
const Ext = ".saprint"

var (
	ErrRemote = errors.New("файл типографии не на локальном диске")
	ErrFile   = errors.New("это не файл типографии (.saprint)")
	ErrGone   = errors.New("файл типографии не найден")
	ErrBig    = errors.New("файл типографии больше 64 КБ")
	ErrStale  = errors.New("вопрос устарел — добавьте файл ещё раз")
	ErrSave   = errors.New("файл типографии не записан")
	// ErrExists — без пути: имя с дописанным расширением диалог не проверял.
	ErrExists = errors.New("такой файл уже есть")
)

// Import — итог «Добавить из файла». Ask: "" — типография добавлена; replace — стоит своя
// с тем же id, ответ — ConfirmImportPrintProfile(Token); same — такая уже стоит, ничего не
// записано. Copied — id файла был чужим (встроенной, библиотечной или без own-…): добавлена
// копией под новым.
type Import struct {
	Canceled bool           `json:"canceled"`
	Info     printprof.Info `json:"info"`
	Ask      string         `json:"ask"`
	Question string         `json:"question"`
	Token    string         `json:"token"`
	Copied   bool           `json:"copied"`
	Lines    []string       `json:"lines"` // сводка: форматы в px, имена файлов, хост сайта
}

// Share — итог «Поделиться файлом»: имя записанного файла (без пути).
type Share struct {
	Canceled bool   `json:"canceled"`
	Name     string `json:"name"`
}

// readErr — отказ разбора: причина без пути (текст Parse ограничен по длине).
func readErr(why string) error {
	return errors.New("Файл типографии не читается: " + why) //nolint:staticcheck // текст уходит пользователю тостом как есть
}

// ImportFile — «Добавить из файла…»: диалог *.saprint в Go. Отказ от диалога — Canceled.
func (s *Service) ImportFile() (res Import, err error) {
	path, err := s.h.PickOpen()
	if err != nil {
		return res, err
	}
	if path == "" {
		return Import{Canceled: true, Lines: []string{}}, nil
	}
	res, err = s.ImportPath(path)
	if err == nil {
		s.h.RememberDir(filepath.Dir(path))
	}
	return res, err
}

// readPrintFile читает файл типографии: локальный обычный .saprint не больше 64 КБ. Размер
// проверяется до чтения, чтение ограничено ещё раз (файл мог вырасти между проверкой и чтением);
// канал и устройство отклоняются без ожидания, подмена ссылки после проверки — library.OpenLocal.
func readPrintFile(path string) ([]byte, error) {
	if paths.CheckLocalPath(path) != nil { // ошибка несёт полный путь — в тост не идёт
		return nil, ErrRemote
	}
	if !strings.EqualFold(filepath.Ext(path), Ext) {
		return nil, ErrFile
	}
	f, fi, err := library.OpenLocal(path) // проверенная цель и сверка с открытым (аудит 123 №8)
	if err != nil {
		return nil, ErrGone
	}
	defer f.Close() //nolint:errcheck // чтение: ошибка закрытия ничего не теряет
	if fi.Size() > printprof.MaxSize {
		return nil, ErrBig
	}
	data, err := io.ReadAll(io.LimitReader(f, printprof.MaxSize+1))
	switch {
	case err != nil:
		return nil, readErr("чтение прервано")
	case len(data) > printprof.MaxSize:
		return nil, ErrBig
	}
	return data, nil
}

// ImportPath — импорт по пути (диалог и тесты). Правила id:
//   - своя с тем же id уже стоит — вопрос «Заменить…» (или «такая уже есть» при равных данных);
//   - id вида own-… и новый — сохраняется как есть: дальше файл от того же человека заменяет её;
//   - любой другой (встроенная, библиотечная, без own-…, файл хранилища, отвергнутый реестром) —
//     копия под новым own-…, изделия тоже.
//
// Название, занятое другой типографией, — «Такое название уже есть».
func (s *Service) ImportPath(path string) (res Import, err error) {
	res.Lines = []string{}
	data, err := readPrintFile(path)
	if err != nil {
		return res, err
	}
	p, err := printprof.Parse(data)
	if err != nil {
		return res, readErr(err.Error())
	}
	st, err := s.h.Store()
	if err != nil {
		return res, err
	}
	s.mu.Lock()
	s.pend = nil // следующая операция отменяет неотвеченный вопрос
	snap := printprof.Current()
	cur, replace := s.own[p.ID]
	switch {
	case replace:
		if err := nameClash(snap, p.Name, p.ID); err != nil {
			s.mu.Unlock()
			return res, err
		}
		res = s.askReplace(res, cur, p)
		s.mu.Unlock()
		return res, nil
	case !printprof.ValidOwnID(p.ID) || !ownProductIDs(p) || s.stored[p.ID]:
		p = rekey(p, printprof.NewOwnID(s.taken))
		res.Copied = true
	}
	if err := nameClash(snap, p.Name, ""); err != nil {
		s.mu.Unlock()
		return res, err
	}
	err = s.storeOwn(st, "", p, nil)
	s.mu.Unlock()
	if err != nil {
		return res, err
	}
	res.Info = printprof.InfoOf(p, printprof.SourceOwn)
	res.Lines = Summary(p)
	s.changed()
	return res, nil
}

// askReplace — вопрос о замене своей или «такая уже есть»; под s.mu.
func (s *Service) askReplace(res Import, cur ownPrint, p printprof.Profile) Import {
	res.Info, res.Lines = printprof.InfoOf(p, printprof.SourceOwn), Summary(p)
	old, _ := printprof.Encode(cur.p)
	if nw, _ := printprof.Encode(p); string(old) == string(nw) {
		res.Ask = "same"
		return res
	}
	var b [8]byte
	_, _ = rand.Read(b[:])
	res.Ask, res.Token = "replace", hex.EncodeToString(b[:])
	res.Question = fmt.Sprintf("Заменить типографию «%s» (версия %d) типографией из файла (версия %d)?", cur.p.Name, cur.p.Version, p.Version)
	s.pend = &pendingPrint{token: res.Token, key: cur.key, p: p, until: time.Now().Add(TTL)}
	return res
}

// ConfirmImport — «Заменить» на вопрос ImportFile. Ответ устарел (прошло десять минут, другая
// операция, типографию удалили) — ErrStale.
func (s *Service) ConfirmImport(token string) (res Import, err error) {
	res.Lines = []string{}
	st, err := s.h.Store()
	if err != nil {
		return res, err
	}
	s.mu.Lock()
	pp := s.pend
	s.pend = nil
	var cur ownPrint
	var ok bool
	if pp != nil {
		cur, ok = s.own[pp.p.ID]
	}
	var clash error
	if pp != nil {
		clash = nameClash(printprof.Current(), pp.p.Name, pp.p.ID)
	}
	switch {
	case pp == nil || token == "" || pp.token != token || time.Now().After(pp.until) || !ok || cur.key != pp.key:
		s.mu.Unlock()
		return res, ErrStale
	case clash != nil:
		s.mu.Unlock()
		return res, clash
	}
	err = s.storeOwn(st, pp.key, pp.p, &cur.p)
	s.mu.Unlock()
	if err != nil {
		return res, err
	}
	res.Info, res.Lines = printprof.InfoOf(pp.p, printprof.SourceOwn), Summary(pp.p)
	s.changed()
	return res, nil
}

// ShareFile — «Поделиться файлом…»: своя типография в <название>.saprint (диалог Go).
// Встроенную и библиотечную не отправить — они и так у всех.
func (s *Service) ShareFile(id string) (res Share, err error) {
	s.mu.Lock()
	o, ok := s.own[id]
	s.mu.Unlock()
	if !ok {
		return res, ErrNotOwn
	}
	data, err := printprof.Encode(o.p)
	if err != nil {
		return res, err
	}
	path, err := s.h.PickSave(paths.FileName(o.p.Name, "Типография") + Ext)
	if err != nil {
		return res, err
	}
	if path == "" {
		return Share{Canceled: true}, nil
	}
	added := !strings.EqualFold(filepath.Ext(path), Ext)
	if added {
		path += Ext
	}
	if paths.CheckLocalPath(path) != nil {
		return res, ErrRemote
	}
	// Замену выбранного файла подтвердил диалог, а файла с дописанным расширением он не видел.
	if _, err := os.Lstat(path); added && err == nil {
		return res, ErrExists
	}
	if err := library.WriteFileAtomic(path, data); err != nil {
		log.Printf("файл типографии: %v", paths.NoPath(err))
		return res, ErrSave
	}
	s.h.RememberDir(filepath.Dir(path))
	return Share{Name: filepath.Base(path)}, nil
}

// ownProductIDs — изделия вида <id>-<n>, как требует реестр для своей.
func ownProductIDs(p printprof.Profile) bool {
	for _, pr := range p.Products {
		n, ok := strings.CutPrefix(pr.ID, p.ID+"-")
		if !ok || n == "" || strings.Trim(n, "0123456789") != "" {
			return false
		}
	}
	return true
}

// rekey — профиль под новым id своей: изделия own-<id>-<n>, изделие по умолчанию следует за ними.
func rekey(p printprof.Profile, id string) printprof.Profile {
	m := map[string]string{}
	for i := range p.Products {
		m[p.Products[i].ID] = printprof.OwnProductID(id, i+1)
		p.Products[i].ID = m[p.Products[i].ID]
	}
	p.ID, p.Default = id, m[p.Default]
	return p
}

// Summary — сводка файла: форматы в px, имена файлов выгрузки и хост сайта (один на все
// адреса профиля — Validate, аудит 123 №1).
func Summary(p printprof.Profile) []string {
	var fm []string
	for _, f := range p.Formats {
		w, h := model.Px(2*f.Page[0], p.Page.DPI), model.Px(f.Page[1], p.Page.DPI)
		if f.Px != nil {
			w, h = f.Px[0], f.Px[1]
		}
		fm = append(fm, fmt.Sprintf("%s — %d × %d px", f.Name, w, h))
	}
	out := []string{"Форматы: " + strings.Join(fm, ", ")}
	if ex := p.Export; ex != nil {
		out = append(out, "Файлы: "+nameSample(ex.Prefix, 12))
	}
	if h := p.SiteHost(); h != "" {
		out = append(out, "Сайт: "+h)
	}
	return out
}
