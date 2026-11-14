package libpacks

import (
	"bytes"
	"context"
	"errors"
	"log"
	"slices"
	"sync"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/app/appkit"
	"github.com/IliaSA101/sosninaalbums/internal/decor"
	"github.com/IliaSA101/sosninaalbums/internal/decor/fetch"
	"github.com/IliaSA101/sosninaalbums/internal/decor/pack"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

// Demo — пример набора в программе (спека 142): что о нём знает App. demopack сюда не
// импортируется (depguard), поэтому App собирает значение сам; пустой ID — примера нет
// (SA_DEMO=off на стенде) или Host.Demo не задан.
type Demo struct {
	ID      string
	Version int
	Related []string // наборы каталога, которые вместе покрывают пример
	Asset   string   // путь ассета от префикса источника библиотеки (докачка в компактной)
	// Entry — строка для netInstall и «Можно скачать»: размер, sha256, описание и счётчики
	// закреплены в коде; URL пуст — адрес собирает служба из Asset.
	Entry fetch.Entry
	// Has — дёшево, один раз за сеанс: файл примера рядом с программой есть и подходит по размеру.
	Has func() bool
	// Read — байты файла, проверенные по размеру и sha256; зовётся только под замком службы.
	Read func() ([]byte, bool)
}

// Пример набора (спека 142, design §4): файл у программы ставит PlaceDemo без сети; в компактной
// редакции строка «Скачать» живёт только в List (0.30-142-39), докачка — внутри StartCheck
// (0.30-142-41). Все проверки и запись идут под замком службы (0.30-142-18).

// whyDemo — id строки каталога занят примером (недостижимо: id зарезервирован в library-pack).
const whyDemo = "совпадает с примером в программе"

// lockPoll — как часто фон ждёт замок службы (тесты ускоряют).
var lockPoll = 100 * time.Millisecond

// demoAsset — результат проверки адреса ассета (один раз за сеанс: окно опрашивает List каждые 2 с).
type demoAsset struct {
	once sync.Once
	url  string
	ok   bool
}

// demoURL — адрес ассета примера: префикс источника и та же проверка, что у строк каталога.
func (s *Service) demoURL(rel string) (string, bool) {
	s.demoAsset.once.Do(func() {
		c := s.h.Fetcher()
		if c == nil {
			return
		}
		if s.demoAsset.url, s.demoAsset.ok = c.AssetURL(rel); !s.demoAsset.ok {
			log.Printf("пример: адрес ассета не прошёл проверку — докачки нет")
		}
	})
	return s.demoAsset.url, s.demoAsset.ok
}

// lockWait — замок службы с ожиданием: TryLock раз в lockPoll до ctx. false — ctx закончился.
func (s *Service) lockWait(ctx context.Context) bool {
	t := time.NewTicker(lockPoll)
	defer t.Stop()
	for {
		if s.lock() {
			return true
		}
		select {
		case <-ctx.Done():
			return false
		case <-t.C:
		}
	}
}

// PlaceDemo — пример из файла рядом с программой, без сети и вопроса (domReady, до проверки при
// запуске). Нет файла — ничего: строку «Скачать» покажет List, докачку сделает StartCheck. Стоящий
// пример при полном наборе доноров уходит здесь же, с файлом или без (компактная редакция: сеть и
// согласие не нужны, 0.30-142-48). Паника не роняет программу: фон на каждом старте.
func (s *Service) PlaceDemo(ctx context.Context) {
	defer appkit.Guard("пример")
	d := s.h.Demo()
	if d.ID == "" {
		return
	}
	st, err := s.h.Store()
	if err != nil || !d.Has() && !st.DemoCovered() {
		return // без файла ждать замок незачем, пока нечему уходить
	}
	if s.placeOrRetire(ctx, st, d) {
		s.h.Relink(st)
		s.emitChanged() // без названия: тоста нет (0.30-142-8)
	}
}

// placeOrRetire — пример встаёт или уходит под замком; true — состав библиотеки изменился. Замок
// снимается отложенно: паника в установке не оставит службу занятой.
func (s *Service) placeOrRetire(ctx context.Context, st *decor.Store, d Demo) bool {
	if !s.lockWait(ctx) {
		return false
	}
	defer s.unlock()
	placed := s.placeLocked(st, d)
	return s.retireDemo(st) || placed
}

// placeLocked — пример из файла; под замком. true — пример встал этим вызовом. Не ставит, если
// библиотека только для чтения, пример или любой из доноров удалён вами, стоит любой донор или
// пример той же версии и новее.
func (s *Service) placeLocked(st *decor.Store, d Demo) bool {
	if d.ID == "" || !d.Has() {
		return false
	}
	if st.ReadOnly() {
		log.Printf("пример: не поставлен — библиотека только для чтения")
		return false
	}
	skip := prefs.LibraryNetOf(s.h.CfgDir()).Skip
	if slices.Contains(skip, d.ID) || slices.ContainsFunc(d.Related, func(id string) bool { return slices.Contains(skip, id) }) {
		return false
	}
	have := haveOf(st)
	if slices.ContainsFunc(d.Related, func(id string) bool { _, ok := have[id]; return ok }) {
		return false // донор уже даёт всё (0.30-142-22)
	}
	if p, ok := have[d.ID]; ok && p.Version >= d.Version {
		return false
	}
	z, ok := d.Read()
	if !ok {
		return false // причина уже в log.txt, дальше Has() == false
	}
	arc, err := pack.OpenReader(bytes.NewReader(z), int64(len(z)), s.h.Version())
	if err != nil {
		logDemoErr("файл не принят", err)
		return false
	}
	// Байты проверены по sha256 константы, но манифест сверяем всё равно: id и версия — те же.
	if arc.Manifest.ID != d.ID || arc.Manifest.Version != d.Version {
		log.Printf("пример: файл не совпадает с ожидаемым набором")
		return false
	}
	sg, err := st.StagePack(decorManifest(arc.Manifest), arc, decor.OriginDemo, d.Entry.SHA256)
	if err != nil {
		sg.Close()
		logDemoErr("не поставлен", err)
		return false
	}
	defer sg.Close()
	// Подпись размера та же, что у примера из сети (137-31).
	sg.SetZipSize(int64(len(z)))
	if sg.Ask != "" && sg.Ask != decor.AskUpdate { // same / older
		return false
	}
	if _, err := st.InstallPack(sg, s.h.AlbumLibs()); err != nil {
		logDemoErr("не поставлен", err)
		return false
	}
	return true
}

// logDemoErr — строка log.txt о примере, что не встал: квота и известные отказы разбора набора
// (версия, манифест, архив) — их текстом: он без путей; прочее — тип (текст ошибки диска может
// нести путь).
func logDemoErr(what string, err error) {
	switch {
	case errors.Is(err, decor.ErrLibFull):
		log.Printf("пример: %s — библиотека заполнена", what)
	case errors.Is(err, pack.ErrAppOld), errors.Is(err, pack.ErrNewer), errors.Is(err, pack.ErrManifest),
		errors.Is(err, pack.ErrDamaged), errors.Is(err, pack.ErrEmpty):
		log.Printf("пример: %s — %s", what, err)
	default:
		log.Printf("пример: %s (%T)", what, err)
	}
}

// retireDemo — пример уходит, когда стоят доноры, покрывающие каждый его предмет (0.30-142-17):
// удаление уносит 0 файлов. Под замком, в конце установки и до emitChanged. true — ушёл.
func (s *Service) retireDemo(st *decor.Store) bool {
	d := s.h.Demo()
	if d.ID == "" || !st.DemoCovered() {
		return false
	}
	if _, err := st.RemovePack(d.ID, s.h.AlbumLibs()); err != nil {
		log.Printf("пример: не убран (%T)", err)
		return false
	}
	log.Printf("библиотека: пример ушёл — стоит полный набор")
	return true
}

// demoRow — строка «Скачать» примера (компактная, 0.30-142-39): нет файла у программы, примера
// нет, ни он, ни доноры не удалены вами и не стоят, адрес ассета годен. Без каталога и сети.
func (s *Service) demoRow(d Demo, have map[string]decor.PackInfo, skip []string) (Avail, bool) {
	if d.ID == "" {
		return Avail{}, false
	}
	gone := func(id string) bool { _, ok := have[id]; return ok || slices.Contains(skip, id) }
	if gone(d.ID) || slices.ContainsFunc(d.Related, gone) || d.Has() {
		return Avail{}, false
	}
	if _, ok := s.demoURL(d.Asset); !ok {
		return Avail{}, false
	}
	e := d.Entry
	return Avail{ID: d.ID, Title: e.Title, About: e.About, License: e.License, Version: e.Version,
		Bg: e.Counts.Bg, Sticker: e.Counts.Sticker, Spread: e.Counts.Spread, Frame: e.Counts.Frame, Print: e.Counts.Print,
		Bytes: e.Size, AddonTitles: []string{}}, true
}

// demoEntry — строка для netInstall: данные примера и адрес ассета.
func (s *Service) demoEntry(d Demo) (fetch.Entry, bool) {
	url, ok := s.demoURL(d.Asset)
	e := d.Entry
	e.URL = url
	return e, ok
}

// downloadDemo — «Скачать» в строке примера: без каталога, согласие и замок уже взяты run.
// Итог как у «Скачать»: Picked и Why.
func (s *Service) downloadDemo(ctx context.Context, st *decor.Store) (res Update, err error) {
	res = newUpdate()
	d := s.h.Demo()
	skip := prefs.LibraryNetOf(s.h.CfgDir()).Skip
	if _, ok := s.demoRow(d, haveOf(st), skip); !ok {
		return res, ErrGone // строка уже ушла: окно перечитает список
	}
	e, _ := s.demoEntry(d)
	err = s.installAll(ctx, st, s.h.Fetcher(), []fetch.Entry{e}, &res, s.progress(), decor.OriginDemo)
	if res.Added+res.Updated > 0 {
		s.retireDemo(st)
		s.h.Relink(st)
		s.emitChanged(res.titles...)
	}
	res.Picked = slices.Contains(res.done, d.ID)
	if !res.Picked {
		if res.Why = res.why[d.ID]; res.Why == "" {
			res.Why = "набор не поставлен"
		}
	}
	if cat, _, _ := s.cached(); cat != nil {
		_, all := shownPlan(*cat, haveOf(st), skip, s.h.Version())
		res.Available = all.Count
	}
	return res, err
}

// startDemo — докачка примера внутри проверки при запуске (компактная, 0.30-142-41): под тем же
// замком и beginNet, после update без ошибки сети; без тоста, ошибка — в log.txt.
func (s *Service) startDemo(ctx context.Context, st *decor.Store) {
	d := s.h.Demo()
	if _, ok := s.demoRow(d, haveOf(st), prefs.LibraryNetOf(s.h.CfgDir()).Skip); !ok {
		return
	}
	e, _ := s.demoEntry(d)
	res := newUpdate()
	if err := s.installAll(ctx, st, s.h.Fetcher(), []fetch.Entry{e}, &res, s.progress(), decor.OriginDemo); err != nil || res.Added+res.Updated == 0 {
		return // причина уже в log.txt, строка «Скачать» остаётся
	}
	s.retireDemo(st)
	s.h.Relink(st)
	s.emitChanged()
}

// netFor — Net с полями примера; библиотека не открылась — без них.
func (s *Service) netFor(n prefs.LibraryNet) Net {
	st, _ := s.h.Store()
	return s.netWith(n, st)
}

// netWith — netView плюс пример: пропуск не считается, пока стоят все доноры (0.30-142-28);
// DemoSkipped — пропуск считается; DemoOffer — есть строка «Скачать».
func (s *Service) netWith(n prefs.LibraryNet, st *decor.Store) Net {
	out := netView(n)
	d := s.h.Demo()
	if d.ID == "" || st == nil {
		return out
	}
	have := haveOf(st)
	if slices.Contains(n.Skip, d.ID) {
		if len(d.Related) > 0 && !slices.ContainsFunc(d.Related, func(id string) bool { _, ok := have[id]; return !ok }) {
			out.Skipped--
		} else {
			out.DemoSkipped = true
		}
	}
	_, out.DemoOffer = s.demoRow(d, have, n.Skip)
	return out
}
