package decor

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Макеты альбома в библиотеке (спека 131, design §7): вид album, файл Макеты/<16 hex>.json — байты
// как пришли из набора (ключ — model.LookKey, sha256 байтов: поле новее программы его не меняет),
// свои — канонический JSON в момент сохранения. Встроенных нет: макеты приносят наборы.
//
// Контракт со сборкой мусора 0.27 (0.27-131-30): ключи картинок в файле — обычные строки
// "my:<16 hex>"; файл ≤ model.MaxLookFile. Ссылки макетов держит сырой разбор Макеты/*.json
// (packs_gc_raw.go) — то же правило, что у 0.27, без отдельного списка в реестре.

// LookPath — плитки макетов: /media/look/<16 hex>?v=&i=&px=&r=&bg= (рисует App, G6).
const LookPath = "/media/look/"

// LookURL — адрес плитки макета; v=1 — чтобы UI мог дописать параметры.
func LookURL(key string) string {
	return LookPath + strings.TrimPrefix(key, libPrefix) + "?v=1"
}

var (
	// errLookUnread — файл из индекса не прочитан (занят, больше предела, не обычный): макета не знаем.
	errLookUnread = errors.New("файл макета альбома не прочитан")
	errLookBroken = errors.New("файл макета альбома испорчен")
	errLookBuf    = errors.New("макет альбома больше предела")
)

// lookCache — разбор макетов по ключу для плиток (файл под ключом не меняется: ключ — хэш байтов);
// битый тоже запоминается, чтобы не разбирать его на каждое открытие окна.
type lookCache struct {
	mu sync.Mutex
	m  map[string]lookInfo
}

type lookInfo struct {
	ok   bool
	card model.LookCard
}

func (c *lookCache) get(key string) (lookInfo, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	info, ok := c.m[key]
	return info, ok
}

func (c *lookCache) put(key string, info lookInfo) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.m == nil {
		c.m = map[string]lookInfo{}
	}
	c.m[key] = info
}

// Look — макет по ключу плитки (свой или из набора; путь — из индекса Макеты/, не от интерфейса).
// Нет файла — ErrNotFound, файл новее программы — model.ErrLookNewer (и ErrLookSteps), битый —
// ошибка «испорчен». Разбор берётся из кэша (looks_cache.go), если файл не менялся; результат —
// всегда своя копия, вызывающий вправе её править.
func (s *Store) Look(id string) (model.AlbumTpl, error) {
	if !model.ValidLibKey(id) {
		return model.AlbumTpl{}, ErrNotFound
	}
	s.mu.RLock()
	path, ok := s.files[KindAlbum][id]
	s.mu.RUnlock()
	if !ok {
		return model.AlbumTpl{}, ErrNotFound
	}
	st, serr := os.Stat(path) // до чтения: подмена файла после стата даст другой стат при следующем попадании
	if serr != nil || !st.Mode().IsRegular() {
		s.lookTpl.drop(id)
		st = nil
	} else if t, hit := s.lookTpl.get(id, path, st); hit {
		return cloneLook(t), nil
	}
	t, _, err := s.lookRaw(id)
	if errors.Is(err, errLookUnread) {
		err = ErrNotFound
	}
	if err != nil {
		return t, err
	}
	if st != nil {
		s.lookTpl.put(id, path, st, cloneLook(t))
	}
	return t, nil
}

// lookRaw — макет и сырые байты файла.
func (s *Store) lookRaw(id string) (model.AlbumTpl, []byte, error) {
	if !model.ValidLibKey(id) {
		return model.AlbumTpl{}, nil, ErrNotFound
	}
	s.mu.RLock()
	path, ok := s.files[KindAlbum][id]
	s.mu.RUnlock()
	if !ok {
		return model.AlbumTpl{}, nil, ErrNotFound
	}
	data, err := readCapped(path, model.MaxLookFile)
	if errors.Is(err, os.ErrNotExist) {
		return model.AlbumTpl{}, nil, ErrNotFound
	}
	if err != nil {
		return model.AlbumTpl{}, nil, errLookUnread
	}
	t, err := model.ParseLook(data)
	switch {
	case errors.Is(err, model.ErrLookNewer), errors.Is(err, model.ErrLookSteps):
		return model.AlbumTpl{}, data, err
	case err != nil:
		return model.AlbumTpl{}, data, errLookBroken
	case !lookKnown(t):
		return model.AlbumTpl{}, data, model.ErrLookNewer // узор или рисунок новее программы
	}
	return t, data, nil
}

// lookMeta дописывает к плитке макета сведения карточки (design §9: steps, from, vig, cover); false —
// файл не разбирается (новее программы, битый) — плитки нет.
func (s *Store) lookMeta(it *Item) bool {
	info, hit := s.look.get(it.ID)
	if !hit {
		t, _, err := s.lookRaw(it.ID)
		info = lookInfo{ok: err == nil}
		if err == nil {
			info.card = model.CardOf(t)
		}
		if !errors.Is(err, ErrNotFound) && !errors.Is(err, errLookUnread) { // не прочитался — не запоминать
			s.look.put(it.ID, info)
		}
	}
	it.Steps, it.From, it.Vig, it.Cover = info.card.Steps, info.card.From, info.card.Vig, info.card.Cover
	return info.ok
}

// LookByItem — текущий ключ макета набора pack по пути записи item в манифесте (design §3 «Поиск
// макета»): набор обновился — ключ новый, запись та же. Нет набора, записи или файла — false.
func (s *Store) LookByItem(pack, item string) (string, bool) {
	if !validPackID(pack) || !ValidItemPath(item) {
		return "", false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	r := s.packOf(pack)
	if r == nil {
		return "", false
	}
	for _, e := range r.Items {
		if e.Kind == KindAlbum && e.Item == item {
			_, ok := s.files[KindAlbum][e.Key]
			return e.Key, ok
		}
	}
	return "", false
}

// CheckLook — макет t, каким SaveLook положил бы его в библиотеку (зажатый), или та же ошибка, что
// у записи: пустой после зажима или файл больше предела. Ничего не пишет и хранилища не касается.
func CheckLook(t model.AlbumTpl) (model.AlbumTpl, error) {
	c, _, err := packLook(t)
	return c, err
}

// packLook — зажатый макет и его канонический JSON; общая часть CheckLook и SaveLook.
func packLook(t model.AlbumTpl) (model.AlbumTpl, []byte, error) {
	c, ok := model.ClampLook(t)
	if !ok {
		return c, nil, model.ErrTplEmpty
	}
	data, err := json.Marshal(c)
	if err != nil {
		return c, nil, model.ErrTplEmpty
	}
	if len(data) > model.MaxLookFile {
		return c, nil, fmt.Errorf("макет больше %d КБ — уберите часть разворотов", model.MaxLookFile>>10)
	}
	return c, data, nil
}

// SaveLook — свой макет («Сохранить альбом как макет…», G6): канонический JSON зажатого макета в
// Макеты/<ключ>.json и запись реестра в тему theme (выборки и неизвестная — «Мои»). Тот же макет второй
// раз — «уже есть»; скрытый возвращается с новым именем и темой.
func (s *Store) SaveLook(theme, name string, t model.AlbumTpl) (Item, error) {
	if name = cleanName(name); name == "" {
		return Item{}, ErrName
	}
	if !isTheme(theme) {
		theme = ThemeMine
	}
	c, data, err := packLook(t)
	if err != nil {
		return Item{}, err
	}
	key := model.LookKey(data)
	s.ioMu.Lock()
	defer s.ioMu.Unlock()
	if s.ReadOnly() {
		return Item{}, ErrReadOnly
	}
	s.mu.RLock()
	e := s.entryOf(KindAlbum, key)
	have, onDisk := s.files[KindAlbum][key]
	n, total := s.usage(KindAlbum)
	s.mu.RUnlock()
	if onDisk {
		onDisk = regularFile(have)
	}
	switch {
	case e != nil && onDisk && !e.Hidden:
		return Item{}, errHave
	case e == nil && n >= MaxOwn:
		return Item{}, fmt.Errorf("своих уже %d — больше не помещается", MaxOwn)
	case total+int64(len(data)) > MaxLibraryLen:
		return Item{}, fmt.Errorf("библиотека заполнена (%d ГБ)", MaxLibraryLen>>30)
	}
	final := s.packPath(KindAlbum, key, "json")
	if !onDisk {
		if err := writeLook(s.root, final, data); err != nil {
			return Item{}, err
		}
	}
	s.mu.Lock()
	s.indexFile(KindAlbum, key, final)
	if e == nil {
		e = &entry{Key: key, Kind: KindAlbum}
		s.entries = append(s.entries, e)
	}
	e.Theme, e.Name, e.Ext, e.Bytes, e.Hidden, e.Added = theme, name, "json", int64(len(data)), false, nowMs()
	it := e.item()
	s.mu.Unlock()
	card := model.CardOf(c)
	s.look.put(key, lookInfo{ok: true, card: card})
	s.saveLogged()
	it.Steps, it.From, it.Vig, it.Cover = card.Steps, card.From, card.Vig, card.Cover
	return it, nil
}

// writeLook пишет файл макета атомарно (writeJSON).
func writeLook(root, final string, data []byte) error {
	return writeJSON(root, final, "макет", data)
}

// stageLook — макет набора в память: предел до чтения, ParseLook (зажим), узоры и рисунки знакомы
// этой версии; хранятся байты как пришли, ключ — sha256 байтов (не пересчитывается после зажима,
// 0.27-131-6). item — путь записи в манифесте: по нему альбом находит макет после обновления набора.
func stageLook(i int, it PackItem, src PackSource) (si stagedItem, why string, err error) {
	if !ValidItemPath(it.File) {
		return si, "недопустимое имя записи макета", nil
	}
	buf := limitBuffer{max: model.MaxLookFile}
	if err := src.Extract(i, &buf); err != nil {
		if errors.Is(err, errLookBuf) {
			return si, fmt.Sprintf("макет больше %d КБ", model.MaxLookFile>>10), nil
		}
		return si, "", err
	}
	data := buf.Bytes()
	t, err := model.ParseLook(data)
	switch {
	case errors.Is(err, model.ErrLookNewer), err == nil && !lookKnown(t):
		return si, "макет из новой версии программы", nil
	case errors.Is(err, model.ErrLookSteps):
		return si, fmt.Sprintf("в макете больше %d разворотов", model.MaxLookSteps), nil
	case err != nil:
		return si, "макет альбома не читается", nil
	}
	libs, ok := lookStageLibs(t, data)
	if !ok {
		return si, "макет альбома не читается", nil
	}
	si.key, si.ext, si.tpl, si.libs, si.bytes, si.item = model.LookKey(data), "json", bytes.Clone(data), libs, int64(len(data)), it.File
	return si, "", nil
}

// lookStageLibs — ссылки макета набора для checkTplLibs. false — ключ из разбора не виден сырому
// поиску (записан экранированием \u…): сборка мусора 0.27 держит картинки только по сырым байтам,
// такой макет потерял бы фон. Ключи из сырых байт, которых нет в разборе (поле новее программы),
// тоже проверяются — вид у них неизвестен (libRef.kind ""): иначе чужой ключ держал бы файлы вечно.
func lookStageLibs(t model.AlbumTpl, data []byte) ([]libRef, bool) {
	libs := lookLibs(t)
	raw := rawLibKeys(data)
	inRaw, parsed := make(map[string]bool, len(raw)), map[string]bool{}
	for _, k := range raw {
		inRaw[k] = true
	}
	for _, ref := range libs {
		if !inRaw[ref.key] {
			return nil, false
		}
		parsed[ref.key] = true
	}
	for _, k := range raw {
		if !parsed[k] {
			libs = append(libs, libRef{"", k})
		}
	}
	return libs, true
}

// limitBuffer — буфер не больше max байт: предел без доверия к размеру из манифеста.
type limitBuffer struct {
	bytes.Buffer
	max int
}

func (b *limitBuffer) Write(p []byte) (int, error) {
	if b.Len()+len(p) > b.max {
		return 0, errLookBuf
	}
	return b.Buffer.Write(p)
}

// lookTpls — шаблоны макета: шаги и стороны обложки (фон обложки — отдельно, lookLibs).
func lookTpls(t model.AlbumTpl) []model.SpreadTpl {
	out := make([]model.SpreadTpl, 0, len(t.Steps)+2)
	for _, st := range t.Steps {
		out = append(out, st.Tpl)
	}
	if t.Cover != nil {
		for _, side := range []*model.SpreadTpl{t.Cover.Back, t.Cover.Front} {
			if side != nil {
				out = append(out, *side)
			}
		}
	}
	return out
}

// lookLibs — картинки библиотеки, на которые ссылается макет: всё, что знает tplLibs у шагов и
// сторон обложки, и фон обложки.
func lookLibs(t model.AlbumTpl) []libRef {
	var out []libRef
	for _, tpl := range lookTpls(t) {
		out = append(out, tplLibs(tpl)...)
	}
	if t.Cover != nil && t.Cover.Bg != nil && t.Cover.Bg.Lib != "" {
		out = append(out, libRef{KindBg, t.Cover.Bg.Lib})
	}
	return out
}

// lookKnown — все узоры и рисунки макета знакомы этой версии (иначе пропуск: шаг встал бы без них).
func lookKnown(t model.AlbumTpl) bool {
	for _, tpl := range lookTpls(t) {
		if !tplKnown(tpl) {
			return false
		}
	}
	if t.Cover != nil && t.Cover.Bg != nil {
		return tplKnown(model.SpreadTpl{Page: []model.TplBg{*t.Cover.Bg}})
	}
	return true
}

// ValidItemPath — путь записи в манифесте (грамматика pack.ValidName: decor не импортирует pack;
// сверка — TestValidItemPathMatchesPack в internal/app):
// ^[a-z0-9][a-z0-9._-]{0,63}(/[a-z0-9][a-z0-9._-]{0,63}){0,2}$ без «..». В ФС не идёт — только поиск.
func ValidItemPath(name string) bool {
	if name == "" || len(name) > 200 || strings.Contains(name, "..") {
		return false
	}
	parts := strings.Split(name, "/")
	if len(parts) > 3 {
		return false
	}
	for _, p := range parts {
		if p == "" || len(p) > 64 || !lowerDigit(p[0]) {
			return false
		}
		for i := 1; i < len(p); i++ {
			if c := p[i]; !lowerDigit(c) && c != '.' && c != '_' && c != '-' {
				return false
			}
		}
	}
	return true
}

func lowerDigit(c byte) bool { return c >= 'a' && c <= 'z' || c >= '0' && c <= '9' }

// lookRefs дописывает в refs ссылки разбора каждого макета индекса, кроме drop: ключ, записанный
// экранированием (\u003a), сырой поиск не видит, а ParseLook раскодирует и макет его ставит. false —
// файл не прочитан (занят, больше предела): его ссылок не знаем. Не разбирается (битый, новее) — не
// ставится, его ключи держит сырой поиск.
func (s *Store) lookRefs(refs, drop map[string]bool) bool {
	s.mu.RLock()
	files := slices.Collect(maps.Values(s.files[KindAlbum]))
	s.mu.RUnlock()
	for _, path := range files {
		if drop[filepath.Clean(path)] {
			continue
		}
		data, err := readCapped(path, model.MaxLookFile)
		if errors.Is(err, os.ErrNotExist) {
			continue // стёрт между индексом и чтением: держать нечего
		}
		if err != nil {
			return false
		}
		if t, err := model.ParseLook(data); err == nil {
			for _, ref := range lookLibs(t) {
				refs[ref.key] = true
			}
		}
	}
	return true
}

// lookDropPaths — файлы макетов среди собираемых, которые удалятся (их не держат свои, другие
// наборы и keep — то же правило, что у макета в collect): их ссылки уже ничего не держат. Под ioMu;
// keep зовётся вне mu, как в collect.
func (s *Store) lookDropPaths(items []packEntry, keep func(key string) bool, except *packReg) map[string]bool {
	cand := map[string]string{} // ключ → путь
	s.mu.RLock()
	for _, e := range items {
		if path, ok := s.files[KindAlbum][e.Key]; e.Kind == KindAlbum && ok && !s.heldBy(e.Key, except) {
			cand[e.Key] = path
		}
	}
	s.mu.RUnlock()
	drop := map[string]bool{}
	for key, path := range cand {
		if keep == nil || !keep(key) {
			drop[filepath.Clean(path)] = true
		}
	}
	return drop
}
