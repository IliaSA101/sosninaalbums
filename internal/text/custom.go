package text

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"golang.org/x/image/font/sfnt"
	"golang.org/x/text/unicode/norm"
)

// CustomGroup — раздел списка шрифтов для своих гарнитур («Мои шрифты»).
const CustomGroup = "my"

// FamilyID — id своей гарнитуры по имени (082-2): альбом находит шрифт на
// другом компьютере по названию, а не по файлу. Регистр, лишние пробелы и форма
// Unicode (NFC) на id не влияют.
func FamilyID(name string) string {
	s := strings.ToLower(strings.Join(strings.Fields(norm.NFC.String(name)), " "))
	h := sha256.Sum256([]byte("font\x00" + s))
	return "my:" + hex.EncodeToString(h[:8])
}

// NFKC — совместимая нормальная форма: «Й» из двух знаков, буквы во всю ширину и лигатуры
// сводятся к обычным. Для сравнения названий типографий (printprof.NameKey): пакетам модели
// x/text напрямую не положен (depguard), а text его уже тянет.
func NFKC(s string) string { return norm.NFKC.String(s) }

// IsCustomID — id своей гарнитуры: «my:» и 16 знаков hex в нижнем регистре.
func IsCustomID(id string) bool {
	return len(id) == 19 && strings.HasPrefix(id, "my:") && isHex16(id[3:])
}

func isHex16(s string) bool {
	if len(s) != 16 {
		return false
	}
	for _, c := range []byte(s) {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// CustomFace — файл начертания из библиотеки: Key — 16 hex sha256 байтов файла.
type CustomFace struct {
	Key, Ext     string
	Bold, Italic bool
}

// CustomFamily — своя гарнитура, как её отдаёт библиотека шрифтов (fontlib).
type CustomFamily struct {
	ID, Name      string
	Faces         []CustomFace // как есть: набор бывает неполным (082-19)
	Hidden, NoCyr bool
}

// errKeyMismatch — байты файла не совпали с ключом: файл подменили на диске.
var errKeyMismatch = errors.New("файл шрифта не совпал с ключом")

// faceData — разобранный файл начертания: раз на ключ, общий для поколений реестра.
type faceData struct {
	key, ext string
	once     sync.Once
	sf       *sfnt.Font
	ok       atomic.Bool // разбор удался: без once sf можно читать только после этого признака
	broken   atomic.Bool
	logged   atomic.Bool
}

// parse разбирает файл при первом обращении; ошибка или паника — начертание сломано.
func (d *faceData) parse(load func(string) ([]byte, error)) *sfnt.Font {
	d.once.Do(func() {
		defer func() {
			if recover() != nil {
				d.sf = nil
			}
		}()
		b, err := load(d.key)
		if err == nil && !keyMatches(b, d.key) {
			err = errKeyMismatch
		}
		if err == nil {
			d.sf, _ = sfnt.Parse(b)
			d.ok.Store(d.sf != nil)
		}
	})
	if d.sf == nil {
		d.markBroken("файл не прочитан или не разобран")
	}
	if d.broken.Load() {
		return nil
	}
	return d.sf
}

// markBroken — начертание больше не рисуется (Montserrat вместо всей гарнитуры);
// поколение растёт, чтобы кэш не отдал строки, посчитанные этим файлом. В журнал —
// один раз и без имени гарнитуры и пути (§11).
func (d *faceData) markBroken(why string) {
	if d.broken.CompareAndSwap(false, true) {
		customGen.Add(1)
	}
	if d.logged.CompareAndSwap(false, true) {
		log.Printf("свой шрифт %s.%s не работает: %s — вместо него Montserrat", d.key, d.ext, why)
	}
}

// famFace — начертание гарнитуры: признаки из библиотеки и общий разбор файла.
type famFace struct {
	CustomFace
	d *faceData
}

// customFam — зарегистрированная гарнитура; поля после SetCustom не меняются.
type customFam struct {
	CustomFamily
	faces []famFace
	load  func(string) ([]byte, error)
}

func (c *customFam) broken() bool {
	for _, f := range c.faces {
		if f.d.broken.Load() {
			return true
		}
	}
	return false
}

// markBroken — паника при переносе или отрисовке: файл, который прошёл Inspect,
// не должен её давать, поэтому гарнитура снимается целиком.
func (c *customFam) markBroken(why string) {
	for _, f := range c.faces {
		f.d.markBroken(why)
	}
}

// face — начертание стиля; нужного нет — обычное, а без обычного его роль
// играет первое из курсива, жирного, жирного курсива (082-19).
func (c *customFam) face(bold, italic bool) famFace {
	if f, ok := c.find(bold, italic); ok {
		return f
	}
	for _, bi := range [][2]bool{{false, true}, {true, false}, {true, true}} {
		if f, ok := c.find(bi[0], bi[1]); ok {
			return f
		}
	}
	return c.faces[0]
}

func (c *customFam) find(bold, italic bool) (famFace, bool) {
	for _, f := range c.faces {
		if f.Bold == bold && f.Italic == italic {
			return f, true
		}
	}
	return famFace{}, false
}

// asFamily — гарнитура в форме встроенных: CSS без имени из файла, файл — ключ.
func (c *customFam) asFamily() Family {
	f := Family{ID: c.ID, Name: c.Name, CSS: "SA my " + c.ID[3:], Group: CustomGroup, Hidden: c.Hidden, NoCyr: c.NoCyr}
	for _, fc := range c.faces {
		f.Faces = append(f.Faces, Face{File: fc.Key + "." + fc.Ext, Bold: fc.Bold, Italic: fc.Italic})
	}
	return f
}

// customGen — поколение реестра: растёт при SetCustom и поломке начертания.
var customGen atomic.Uint64

// Gen — поколение набора своих шрифтов, не ноль (0 у пользователей — «не запомнено»). Сменилось —
// строки, разбитые раньше, могли разойтись со шрифтом, которым они нарисуются.
func Gen() uint64 { return customGen.Load() + 1 }

// registry — свои гарнитуры по id и разборы файлов по ключу.
var registry = struct {
	sync.RWMutex
	fams map[string]*customFam
	keys map[string]*faceData
	load func(string) ([]byte, error)
}{}

// SetCustom заменяет набор своих гарнитур целиком; load отдаёт байты файла по
// ключу (библиотека сверяет sha, text — ещё раз). Разобранный файл с тем же
// ключом переиспользуется: раз совпал sha, файл тот же. Записи с неверным id,
// ключом или расширением пропускаются, повтор начертания в гарнитуре — тоже.
func SetCustom(fams []CustomFamily, load func(key string) ([]byte, error)) {
	registry.Lock()
	defer registry.Unlock()
	prev := registry.keys
	registry.fams = map[string]*customFam{}
	registry.keys = map[string]*faceData{}
	registry.load = load
	for _, cf := range fams {
		if !IsCustomID(cf.ID) || registry.fams[cf.ID] != nil || load == nil {
			continue
		}
		c := &customFam{CustomFamily: cf, load: load}
		c.Faces = nil
		for _, fc := range cf.Faces {
			if !isHex16(fc.Key) || (fc.Ext != "ttf" && fc.Ext != "otf") {
				continue
			}
			if _, dup := c.find(fc.Bold, fc.Italic); dup {
				continue
			}
			d := registry.keys[fc.Key]
			if d == nil {
				d = prev[fc.Key]
				if d == nil || !d.ok.Load() || d.broken.Load() || d.ext != fc.Ext {
					d = &faceData{key: fc.Key, ext: fc.Ext}
				}
				registry.keys[fc.Key] = d
			}
			if d.ext != fc.Ext {
				continue // тот же ключ с другим расширением — запись библиотеки неверна
			}
			c.faces = append(c.faces, famFace{fc, d})
			c.Faces = append(c.Faces, fc)
		}
		if len(c.faces) > 0 {
			registry.fams[cf.ID] = c
		}
	}
	customGen.Add(1)
}

// lookupCustom — зарегистрированная гарнитура (в том числе сломанная) или nil.
func lookupCustom(id string) *customFam {
	if !IsCustomID(id) {
		return nil
	}
	registry.RLock()
	defer registry.RUnlock()
	return registry.fams[id]
}

// usableCustom — зарегистрированная и не сломанная своя гарнитура или nil.
func usableCustom(id string) *customFam {
	if c := lookupCustom(id); c != nil && !c.broken() {
		return c
	}
	return nil
}

// Available — гарнитура рисуется своим файлом: встроенная или своя
// зарегистрированная и не сломанная. Свою разбирает сразу: правка с пересчётом
// ширин «по тексту» должна знать, что файл не читается, до замера Montserrat.
func Available(id string) bool {
	if builtinFamily(id) != nil {
		return true
	}
	c := lookupCustom(id)
	if c == nil {
		return false
	}
	for _, f := range c.faces {
		f.d.parse(c.load)
	}
	return !c.broken()
}

// CustomName — название своей зарегистрированной гарнитуры.
func CustomName(id string) (string, bool) {
	if c := lookupCustom(id); c != nil {
		return c.Name, true
	}
	return "", false
}

// customFamilies — свои гарнитуры для списка, по имени без регистра; сломанные
// не отдаются (для интерфейса их «нет на этом компьютере»).
func customFamilies() []Family {
	registry.RLock()
	list := make([]*customFam, 0, len(registry.fams))
	for _, c := range registry.fams {
		list = append(list, c)
	}
	registry.RUnlock()
	sort.Slice(list, func(i, j int) bool {
		a, b := strings.ToLower(list[i].Name), strings.ToLower(list[j].Name)
		if a != b {
			return a < b
		}
		return list[i].ID < list[j].ID
	})
	var out []Family
	for _, c := range list {
		if !c.broken() {
			out = append(out, c.asFamily())
		}
	}
	return out
}

// customFile — байты своего начертания по имени «<ключ>.<ext>» (только
// зарегистрированный ключ, байты сверяются с ключом при каждом чтении).
func customFile(name string) ([]byte, bool) {
	key, ext, ok := strings.Cut(name, ".")
	if !ok || !isHex16(key) || (ext != "ttf" && ext != "otf") {
		return nil, false
	}
	registry.RLock()
	d, load := registry.keys[key], registry.load
	registry.RUnlock()
	if d == nil || d.ext != ext || d.broken.Load() {
		return nil, false
	}
	b, err := load(key)
	if err != nil || !keyMatches(b, key) {
		return nil, false
	}
	return b, true
}

// keyMatches — первые 8 байт sha256 файла совпадают с ключом.
func keyMatches(b []byte, key string) bool {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:8]) == key
}

// guarded выполняет fn; паника — false (свои файлы — недоверенный ввод, §3.4).
func guarded(fn func()) (ok bool) {
	defer func() {
		if recover() != nil {
			ok = false
		}
	}()
	fn()
	return true
}
