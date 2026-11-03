package decor

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// Готовые развороты в библиотеке (067, design §3): вид spread, файл Развороты/<16 hex>.json —
// канонический JSON шаблона (ключ — model.TplKey), встроенные — spreads/*.json в сборке.
// Шаблон — не картинка: Find его не ищет, плитку рисует App по /media/tpl.

// TplPath — плитки шаблонов: /media/tpl/<sa:tpl-… | 16 hex>?v=&px=&r= (рисует App: decor не
// знает album и render).
const TplPath = "/media/tpl/"

//go:embed spreads/*.json
var spreadFS embed.FS

// spreadPreset — встроенный разворот: ID заморожен (как пресеты, решение 0.12 №99), файл — в spreads/.
type spreadPreset struct{ id, theme, name, file string }

// spreadPresets — порядок каталога.
var spreadPresets = []spreadPreset{
	{"sa:tpl-ivory-hero", "wedding", "Айвори: главное и два", "ivory-hero"},
	{"sa:tpl-linen-three", "wedding", "Лён: три в ряд", "linen-three"},
	{"sa:tpl-rose-page", "wedding", "Акварель: два на странице", "rose-page"},
	{"sa:tpl-gold-one", "wedding", "Золото: одно фото", "gold-one"},
	{"sa:tpl-notebook", "school", "Тетрадь", "notebook"},
	{"sa:tpl-wave", "school", "Волна", "wave"},
	{"sa:tpl-violet-waves", "grad", "Фиолетовые волны", "violet-waves"},
	{"sa:tpl-stars", "grad", "Звёздный выпуск", "stars"},
	{"sa:tpl-board", "kindergarten", "Доска", "board"},
	{"sa:tpl-rainbow", "kindergarten", "Радуга", "rainbow"},
	{"sa:tpl-party", "holiday", "Праздник", "party"},
	{"sa:tpl-watercolor", "basic", "Акварель", "watercolor"},
}

// builtinTpl — шаблон встроенного разворота: разбирается из сборки на каждый вызов (копия —
// правка вызывающего не портит встроенный; файл в сотни байт).
func builtinTpl(id string) (model.SpreadTpl, bool) {
	for _, p := range spreadPresets {
		if p.id != id {
			continue
		}
		data, err := spreadFS.ReadFile("spreads/" + p.file + ".json")
		if err != nil {
			return model.SpreadTpl{}, false
		}
		t, err := model.ParseSpreadTpl(data)
		return t, err == nil
	}
	return model.SpreadTpl{}, false
}

// spreadBuiltins — плитки встроенных разворотов (у других видов — пусто). Битый файл сборки —
// без плитки (его ловит TestSpreadTplBuiltin).
func spreadBuiltins(kind string) []Item {
	if kind != KindSpread {
		return nil
	}
	out := make([]Item, 0, len(spreadPresets))
	for _, p := range spreadPresets {
		if t, ok := builtinTpl(p.id); ok {
			out = append(out, tplItem(Item{ID: p.id, Theme: p.theme, Name: p.name, URL: TplURL(p.id)}, t))
		}
	}
	return out
}

// tplItem — сведения шаблона на плитке: страниц, мест, пропорция области.
func tplItem(it Item, t model.SpreadTpl) Item {
	it.Pages, it.Places, it.Aspect = t.Pages, len(t.Places), t.Ratio
	return it
}

// TplURL — адрес плитки шаблона; v=1 — чтобы UI мог дописать &px=&r=.
func TplURL(id string) string {
	return TplPath + strings.TrimPrefix(id, libPrefix) + "?v=1"
}

// TplID — ID шаблона по части пути /media/tpl/<…>: встроенный — как есть, свой — 16 hex.
func TplID(seg string) (string, bool) {
	if strings.HasPrefix(seg, presetPrefix) {
		_, ok := builtinTpl(seg)
		return seg, ok
	}
	id := libPrefix + seg
	return id, model.ValidLibKey(id)
}

// tplCache — разбор своих шаблонов и шаблонов наборов для плиток каталога по ключу (файл под ключом не меняется:
// ключ — хэш содержимого); битый файл тоже запоминается — не разбирать его на каждое открытие.
type tplCache struct {
	mu sync.Mutex
	m  map[string]tplInfo
}

func (c *tplCache) put(key string, info tplInfo) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.m == nil {
		c.m = map[string]tplInfo{}
	}
	c.m[key] = info
}

type tplInfo struct {
	ok            bool
	pages, places int
	ratio         float64
	unread        bool     // файл в индексе не прочитался (занят, велик, не обычный): ссылок не знаем — сборка мусора не удаляет
	libs          []libRef // картинки библиотеки шаблона: их держит сборка мусора (tplRefs)
	raw           []string // ключи my:… из сырых байт файла, в том числе в полях новее программы (rawLibKeys)
}

// tplMeta дописывает к плитке своего шаблона (или из набора) страницы, места и пропорцию; false — файл не
// разбирается (новее программы, битый) — плитки нет.
func (s *Store) tplMeta(it *Item) bool {
	info := s.tplLookup(it.ID)
	it.Pages, it.Places, it.Aspect = info.pages, info.places, info.ratio
	return info.ok
}

// tplLookup — разбор шаблона по ключу из кэша или с диска.
func (s *Store) tplLookup(key string) tplInfo {
	s.tpl.mu.Lock()
	info, hit := s.tpl.m[key]
	s.tpl.mu.Unlock()
	if hit {
		return info
	}
	t, data, err := s.spreadTplRaw(key)
	info = tplInfo{ok: err == nil, pages: t.Pages, places: len(t.Places), ratio: t.Ratio, libs: tplLibs(t), raw: rawLibKeys(data), unread: errors.Is(err, errTplUnread)}
	if !errors.Is(err, ErrNotFound) && !info.unread { // не прочитался (стёрт, занят) — не запоминать
		s.tpl.put(key, info)
	}
	return info
}

// errTplBroken — файл шаблона прочитан, но не разбирается.
var errTplBroken = errors.New("файл готового разворота испорчен")

// errTplUnread — файл из индекса не прочитался (занят, больше MaxTplFile, не обычный файл), но он
// есть: для сборки мусора это «ссылки неизвестны», а не «шаблона нет». Наружу (SpreadTpl) — ErrNotFound.
var errTplUnread = errors.New("файл готового разворота не прочитан")

// warmTpl — разбор шаблонов (своих и наборов), которых нет в кэше, в несколько потоков: первое
// открытие окна с тысячами своих упирается в чтение файлов (антивирус, облако), а не в разбор.
func (s *Store) warmTpl(keys []string) {
	var todo []string
	s.tpl.mu.Lock()
	for _, key := range keys {
		if _, hit := s.tpl.m[key]; !hit {
			todo = append(todo, key)
		}
	}
	s.tpl.mu.Unlock()
	if len(todo) < 2 {
		return
	}
	ch := make(chan string)
	var wg sync.WaitGroup
	for range min(len(todo), 8) {
		wg.Go(func() {
			for key := range ch {
				it := Item{ID: key}
				s.tplMeta(&it)
			}
		})
	}
	for _, key := range todo {
		ch <- key
	}
	close(ch)
	wg.Wait()
}

// SpreadTpl — шаблон по ID плитки: встроенный или файл из индекса папки Развороты/ (свой и
// из набора; путь — от ключа, не от интерфейса). Файл новее программы — model.ErrTplNewer.
func (s *Store) SpreadTpl(id string) (model.SpreadTpl, error) {
	t, _, err := s.spreadTplRaw(id)
	if errors.Is(err, errTplUnread) {
		err = ErrNotFound
	}
	return t, err
}

// spreadTplRaw — SpreadTpl и сырые байты файла (для сборки мусора: ссылки читаются и там, где разбор
// не знает поля — borders[].lib 0.28 — или отказал, ErrTplNewer, битый). У встроенного байт нет.
func (s *Store) spreadTplRaw(id string) (model.SpreadTpl, []byte, error) {
	if strings.HasPrefix(id, presetPrefix) {
		if t, ok := builtinTpl(id); ok {
			return t, nil, nil
		}
		return model.SpreadTpl{}, nil, ErrNotFound
	}
	if !model.ValidLibKey(id) {
		return model.SpreadTpl{}, nil, ErrNotFound
	}
	s.mu.RLock()
	path, ok := s.files[KindSpread][id]
	s.mu.RUnlock()
	if !ok {
		return model.SpreadTpl{}, nil, ErrNotFound
	}
	data, err := readCapped(path, model.MaxTplFile)
	if errors.Is(err, os.ErrNotExist) { // стёрт между индексом и чтением — шаблона нет
		return model.SpreadTpl{}, nil, ErrNotFound
	}
	if err != nil {
		return model.SpreadTpl{}, nil, errTplUnread
	}
	t, err := model.ParseSpreadTpl(data)
	if errors.Is(err, model.ErrTplNewer) {
		return model.SpreadTpl{}, data, err
	}
	if err != nil {
		return model.SpreadTpl{}, data, errTplBroken
	}
	return t, data, nil
}

// SaveSpreadTpl — «Сохранить разворот…»: шаблон в Развороты/<ключ>.json и запись реестра в
// тему theme (выборки и неизвестная — «Мои»). Тот же шаблон второй раз — «уже есть»; скрытый
// возвращается с новым именем и темой и встаёт первым, как новый.
func (s *Store) SaveSpreadTpl(theme, name string, t model.SpreadTpl) (Item, error) {
	if name = cleanName(name); name == "" {
		return Item{}, ErrName
	}
	if !isTheme(theme) {
		theme = ThemeMine
	}
	c, ok := model.ClampSpreadTpl(t)
	if !ok {
		return Item{}, model.ErrTplEmpty
	}
	data, err := json.Marshal(c)
	key := model.TplKey(c)
	if err != nil || key == "" {
		return Item{}, model.ErrTplEmpty
	}
	s.ioMu.Lock()
	defer s.ioMu.Unlock()
	if s.ReadOnly() {
		return Item{}, ErrReadOnly
	}
	s.mu.RLock()
	e := s.entryOf(KindSpread, key)
	have, onDisk := s.files[KindSpread][key]
	n, total := s.usage(KindSpread)
	s.mu.RUnlock()
	if onDisk {
		_, err := os.Stat(have)
		onDisk = err == nil
	}
	switch {
	case e != nil && onDisk && !e.Hidden:
		return Item{}, errHave
	case e == nil && n >= MaxOwn:
		return Item{}, fmt.Errorf("своих уже %d — больше не помещается", MaxOwn)
	case total+int64(len(data)) > MaxLibraryLen:
		return Item{}, fmt.Errorf("библиотека заполнена (%d ГБ)", MaxLibraryLen>>30)
	}
	final := filepath.Join(s.root, kindDirs[KindSpread], strings.TrimPrefix(key, libPrefix)+".json")
	if !onDisk {
		if err := writeTpl(s.root, final, data); err != nil {
			return Item{}, err
		}
	}
	s.mu.Lock()
	if s.files[KindSpread] == nil {
		s.files[KindSpread] = map[string]string{}
	}
	s.files[KindSpread][key] = final
	if e == nil {
		e = &entry{Key: key, Kind: KindSpread}
		s.entries = append(s.entries, e)
	}
	e.Theme, e.Name, e.Ext, e.Bytes, e.Hidden, e.Added = theme, name, "json", int64(len(data)), false, nowMs()
	it := e.item()
	s.mu.Unlock()
	s.tpl.put(key, tplInfo{ok: true, pages: c.Pages, places: len(c.Places), ratio: c.Ratio, libs: tplLibs(c)})
	s.saveLogged()
	return tplItem(it, c), nil
}

// writeTpl пишет файл шаблона атомарно (tmp + rename) только внутри корня библиотеки.
func writeTpl(root, final string, data []byte) error {
	return writeJSON(root, final, "разворот", data)
}

// writeJSON — общая запись шаблона и макета (what — что не записалось): tmp + rename только внутри корня.
func writeJSON(root, final, what string, data []byte) error {
	if !library.Under(final, root) { // имя от ключа, но проверка дешевле ошибки
		return errors.New("недопустимый путь в библиотеке")
	}
	if err := os.MkdirAll(filepath.Dir(final), 0o755); err != nil {
		return paths.NoPath(err)
	}
	if err := library.WriteFileAtomic(final, data); err != nil {
		return errors.New(what + " не записался: " + paths.NoPath(err).Error())
	}
	return nil
}

// LibInfo — сведения о файле своего фона или стикера по ключу (для шаблона: ставить или
// рисовать плитку); нет файла на этом ПК или не читается — false.
func (s *Store) LibInfo(key string) (library.Photo, bool) {
	path, ok := s.Find(key)
	if !ok {
		return library.Photo{}, false
	}
	info, err := library.ReadInfo(path)
	if err != nil || info.W <= 0 || info.H <= 0 {
		return library.Photo{}, false
	}
	return info, true
}

// CachedPNG — PNG из кэша превью по ключу key или нарисованный draw в общем слоте рисования
// (genSem; слот освобождается и при панике draw). Ожидание слота прерывает ctx.
func (s *Store) CachedPNG(ctx context.Context, key string, draw func() ([]byte, error)) ([]byte, error) {
	if data, ok := s.media.get(key); ok {
		return data, nil
	}
	select {
	case genSem <- struct{}{}:
	case <-ctx.Done(): // окно ушло со страницы, пока ждали слот
		return nil, ctx.Err()
	}
	defer func() { <-genSem }()
	data, err := draw()
	if err != nil {
		return nil, err
	}
	s.media.put(key, data)
	return data, nil
}
