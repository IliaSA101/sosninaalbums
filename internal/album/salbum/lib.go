package salbum

import (
	"path/filepath"
	"sync/atomic"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// Свои фоны и стикеры по ключу (спека 048, design §2): в файле у записи Photos вместо пути
// ключ my:<16 hex>, путь в памяти даёт искатель библиотеки этого компьютера.

// LibFinder — искатель своего элемента: путь файла в библиотеке по ключу. Тип, а не пакет
// decor: формат библиотеку не знает (design §12).
type LibFinder func(key string) (path string, ok bool)

// libFinder — искатель, поставленный App до первого Load; nil — «не найдено».
var libFinder atomic.Pointer[LibFinder]

// frameFinder — искатель файла рамочки по ключу (079-40): у одной картинки в «Фонах» и в
// «Рамочках» ключ и запись Photos общие, а путь записи — фона (исходник больше 4096 px).
var frameFinder atomic.Pointer[LibFinder]

// SetLibFinder ставит искатель для Load и RelinkLib(nil); nil — снять.
func SetLibFinder(f LibFinder) {
	if f == nil {
		libFinder.Store(nil)
		return
	}
	libFinder.Store(&f)
}

// SetFrameFinder ставит искатель файлов рамочек для FramePath; nil — снять.
func SetFrameFinder(f LibFinder) {
	if f == nil {
		frameFinder.Store(nil)
		return
	}
	frameFinder.Store(&f)
}

// FramePath — файл картинки записи ref под рамочкой места (079-40): своя — копия из «Рамочек»,
// если она есть (уменьшена до 4096 px при загрузке), иначе путь записи, как у фона и стикера.
// Запись без пути — пропавшая (State.missing) до RelinkLib: копию не подставляем и ей.
func FramePath(ref model.PhotoRef) string {
	if ref.Lib != "" && ref.Path != "" {
		if fp := frameFinder.Load(); fp != nil {
			if path := findLib(*fp, ref.Lib); path != "" {
				return path
			}
		}
	}
	return ref.Path
}

// findLib — путь своего элемента или "" (нет искателя, ключ не по грамматике, не найдено).
// Путь искателя проверяется, как путь из файла: дальше он идёт в Check, рендер и /media.
func findLib(find LibFinder, key string) string {
	if find == nil {
		if fp := libFinder.Load(); fp != nil {
			find = *fp
		}
	}
	if find == nil || !model.ValidLibKey(key) {
		return ""
	}
	path, ok := find(key)
	if !ok || paths.CheckLocalPath(path) != nil {
		return ""
	}
	return filepath.Clean(path)
}

// resolveLib — запись со своим ключом при чтении: path и abs из файла не берутся (путь к
// библиотеке на каждом компьютере свой, а чужой путь в файле — не наш). Ключ не по
// грамматике — ok = false, запись выбрасывается (фон с ней снимет зажим). Нет файла —
// путь пуст: фото пропало, размеры из файла держат пропорцию и dpi.
func resolveLib(fp FilePhoto) (model.PhotoRef, bool) {
	if !model.ValidLibKey(fp.Lib) {
		return model.PhotoRef{}, false
	}
	ref := fp.PhotoRef
	ref.Alt = ""
	ref.W, ref.H, ref.Size = max(ref.W, 0), max(ref.H, 0), max(ref.Size, 0)
	ref.Path = findLib(nil, ref.Lib)
	fixDims(&ref, false)
	return ref, true
}

// RelinkLib — тело Editor.RelinkLib: своим элементам, которые нашлись (после загрузки в
// библиотеку), ставится путь искателя find (nil — поставленный SetLibFinder). Ненайденные не
// трогаются: правка не теряет путь. ID от пути не зависит — Rekey не нужен.
func RelinkLib(p *model.Project, find LibFinder) {
	for id, ref := range p.Photos {
		if ref.Lib == "" {
			continue
		}
		np := findLib(find, ref.Lib)
		if np == "" || np == ref.Path {
			continue
		}
		ref.Path, ref.Alt = np, ""
		if ref.W <= 0 || ref.H <= 0 {
			if w, h, ok := probeDims(np); ok {
				ref.W, ref.H = w, h
			}
		}
		p.Photos[id] = ref
	}
}
