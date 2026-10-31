package share

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"hash"
	"math"
	"reflect"
	"sort"

	"github.com/IliaSA101/sosninaalbums/internal/export"
	"github.com/IliaSA101/sosninaalbums/internal/render"
)

// FileStat — размер и время изменения (наносекунды Unix) файла фото; ok = false — файла нет.
// Службу снабжает library.StatLocal: share диск сам не трогает (depguard, граница путей).
type FileStat func(path string) (size, mtime int64, ok bool)

// textDrawType — тип, у которого пропускается Gen (поколение шрифтов).
var textDrawType = reflect.TypeFor[render.TextDraw]()

// maxSceneDepth — глубже сцена не бывает: цикл или чужая структура — ошибка, а не зависание.
const maxSceneDepth = 32

// SceneHash — sha256 (hex) от входа рендера страниц: развороты, обложка, параметры страниц и
// файлы фото (путь, размер, время). Без рендера и без чтения фото — дёшево, для признака
// «альбом изменён после выгрузки» (design §3.4). Метки, подборки и прочее, что в сцену
// не входит, хэш не меняют. Gen текстов (поколение шрифтов, меняется между запусками) не входит.
// stat == nil — файлы не учитываются (пути всё равно в хэше).
func SceneHash(spreads []render.Spread, cover *export.CoverJob, spec export.WebSpec, stat FileStat) (string, error) {
	h := sha256.New()
	w := &sceneWriter{h: h, stat: stat}
	w.str("share-scene-1")
	w.value(reflect.ValueOf(spec), 0)
	w.value(reflect.ValueOf(spreads), 0)
	if cover != nil {
		w.str("cover")
		w.value(reflect.ValueOf(cover.Spread), 0)
		w.value(reflect.ValueOf(cover.Opt), 0)
	}
	if w.err != nil {
		return "", w.err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// sceneWriter пишет значение в хэш обходом через reflect: каждое поле с его видом и длиной,
// поэтому новое поле сцены меняет хэш без правки этого кода. Указатели разыменовываются
// (адрес в хэш не попадает), карты идут по отсортированным ключам.
type sceneWriter struct {
	h    hash.Hash
	stat FileStat
	err  error
}

func (w *sceneWriter) u64(v uint64) {
	var b [8]byte
	binary.LittleEndian.PutUint64(b[:], v)
	_, _ = w.h.Write(b[:])
}

// i64 пишет знаковое число (binary.Write: без приведения знака, которого требует gosec).
func (w *sceneWriter) i64(v int64) { _ = binary.Write(w.h, binary.LittleEndian, v) }

func (w *sceneWriter) str(s string) {
	w.i64(int64(len(s)))
	_, _ = w.h.Write([]byte(s))
}

// float: -0 и 0 — одно число, любое NaN — одно.
func (w *sceneWriter) float(f float64) {
	switch {
	case f == 0:
		f = 0
	case math.IsNaN(f):
		f = math.NaN()
	}
	w.u64(math.Float64bits(f))
}

func (w *sceneWriter) value(v reflect.Value, depth int) {
	if w.err != nil {
		return
	}
	if depth > maxSceneDepth {
		w.err = errors.New("сцена слишком вложена для хэша")
		return
	}
	w.u64(uint64(v.Kind()))
	switch v.Kind() {
	case reflect.Invalid:
	case reflect.Bool:
		if v.Bool() {
			w.u64(1)
		} else {
			w.u64(0)
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		w.i64(v.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		w.u64(v.Uint())
	case reflect.Float32, reflect.Float64:
		w.float(v.Float())
	case reflect.String:
		w.str(v.String())
	case reflect.Pointer, reflect.Interface:
		if v.IsNil() {
			w.u64(0)
			return
		}
		w.u64(1)
		w.value(v.Elem(), depth+1)
	case reflect.Slice, reflect.Array:
		if v.Kind() == reflect.Slice && v.IsNil() { // nil и пустой — одно и то же
			w.u64(0)
			return
		}
		w.i64(int64(v.Len()))
		for i := range v.Len() {
			w.value(v.Index(i), depth+1)
		}
	case reflect.Map:
		w.mapOf(v, depth)
	case reflect.Struct:
		// Нулевые поля не пишутся: новое поле модели с нулевым значением не меняет прежний хэш,
		// а у уже опубликованных альбомов не появляется ложное «изменён после выгрузки».
		t := v.Type()
		for i := range v.NumField() {
			f := t.Field(i)
			if t == textDrawType && f.Name == "Gen" { // поколение шрифтов: другое в каждом запуске, текст и строки — в Box
				continue
			}
			if zero(v.Field(i)) {
				continue
			}
			w.str(f.Name)
			w.value(v.Field(i), depth+1)
			if f.Name == "Path" && f.Type.Kind() == reflect.String {
				w.file(v.Field(i).String())
			}
		}
		w.u64(math.MaxUint64) // конец структуры: без метки Out{A:{X:1},Y:2} и Out{A:{X:1,Y:2}} совпали бы
	default: // func, chan, unsafe.Pointer: в сцене данных нет
		w.u64(0)
	}
}

// mapOf — карта по отсортированным ключам (порядок обхода карты случаен).
func (w *sceneWriter) mapOf(v reflect.Value, depth int) {
	keys := v.MapKeys()
	w.i64(int64(len(keys)))
	type kv struct {
		sum [32]byte
		val reflect.Value
	}
	items := make([]kv, 0, len(keys))
	for _, k := range keys {
		kh := &sceneWriter{h: sha256.New(), stat: w.stat}
		kh.value(k, depth+1)
		if kh.err != nil {
			w.err = kh.err
			return
		}
		var it kv
		copy(it.sum[:], kh.h.Sum(nil))
		it.val = v.MapIndex(k)
		items = append(items, it)
	}
	sort.Slice(items, func(i, j int) bool { return string(items[i].sum[:]) < string(items[j].sum[:]) })
	for _, it := range items {
		_, _ = w.h.Write(it.sum[:])
		w.value(it.val, depth+1)
	}
}

// file подмешивает в хэш размер и время файла по пути; файла нет — отдельная метка.
func (w *sceneWriter) file(path string) {
	if path == "" || w.stat == nil {
		return
	}
	size, mtime, ok := w.stat(path)
	if !ok {
		w.u64(0)
		return
	}
	w.u64(1)
	w.i64(size)
	w.i64(mtime)
}

// zero — значение нулевое для хэша: как reflect.Value.IsZero, но -0 равно 0, пустой список nil
// и структура из одних нулевых полей нулевая. Указатель на нулевую структуру нулевым не считается
// (nil и «есть, но пусто» — разные сцены).
func zero(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Slice, reflect.Map:
		return v.Len() == 0
	case reflect.Struct:
		for i := range v.NumField() {
			if !zero(v.Field(i)) {
				return false
			}
		}
		return true
	case reflect.Array:
		for i := range v.Len() {
			if !zero(v.Index(i)) {
				return false
			}
		}
		return true
	}
	return v.IsZero()
}
