package model

import (
	"slices"

	"github.com/IliaSA101/sosninaalbums/internal/text"
)

// FontRef — своя гарнитура, стоящая в тексте альбома (082): имя нужно для полосы «нет шрифта» на
// компьютере, где её нет. Пути к файлам в альбом не пишутся (082-8).
type FontRef struct {
	Name string `json:"name"`
}

// FontUse — своя гарнитура альбома для интерфейса (View.AlbumFonts): Name "" — название
// неизвестно (id пришёл из шаблона).
type FontUse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

const (
	// MaxFonts — записей своих гарнитур в альбоме; лишние Load отбрасывает (теряется только имя:
	// id в стилях остаётся).
	MaxFonts = 64
	// MaxFontNameRunes — длина названия гарнитуры (как у библиотеки шрифтов).
	MaxFontNameRunes = 60
)

// eachFont — гарнитура каждого стиля текста проекта: тексты и общий стиль подписей листов (с
// обложкой; шапка виньетки — тоже текст), подписи фото. Новое поле стиля в Project — сюда же
// (страж TestUsedFontsCoversStyles). Обложка — отдельно от Sheets: та собирает новый срез.
func (p *Project) eachFont(fn func(id string)) {
	sheet := func(s *Spread) {
		for _, t := range s.Texts {
			fn(t.Style.Font)
		}
		if s.Caps != nil {
			fn(s.Caps.Style.Font)
		}
	}
	if p.Cover != nil && p.Cover.Sheet != nil {
		sheet(p.Cover.Sheet)
	}
	for _, s := range p.Spreads {
		sheet(s)
	}
	for _, r := range p.Photos {
		if r.Caption != nil {
			fn(r.Caption.Style.Font)
		}
	}
}

// UsedFonts — id своих гарнитур в стилях проекта, по возрастанию, без повторов.
func (p *Project) UsedFonts() []string {
	var out []string
	p.eachFont(func(id string) {
		if text.IsCustomID(id) {
			out = append(out, id)
		}
	})
	slices.Sort(out)
	return slices.Compact(out)
}

// SyncFonts — таблица Fonts по стилям (в конце правки и в Load): запись у каждой стоящей своей
// гарнитуры, имя — из реестра шрифтов, если она есть на этом компьютере, иначе прежнее (пустое —
// неизвестно, например из шаблона); неиспользуемые убираются. Идемпотентна; без своих шрифтов —
// без выделения памяти (зовётся на каждую правку).
func (p *Project) SyncFonts() {
	var used []string
	p.eachFont(func(id string) {
		if text.IsCustomID(id) {
			used = append(used, id)
		}
	})
	if len(used) == 0 {
		p.Fonts = nil
		return
	}
	slices.Sort(used)
	if used = slices.Compact(used); len(used) > MaxFonts {
		used = used[:MaxFonts] // как ClampFonts: Save → Load даёт ту же таблицу
	}
	old := p.Fonts
	p.Fonts = make(map[string]FontRef, len(used))
	for _, id := range used {
		ref := old[id]
		if name, ok := text.CustomName(id); ok {
			ref.Name = cleanLine(name, MaxFontNameRunes)
		}
		p.Fonts[id] = ref
	}
}

// ClampFonts — таблица своих гарнитур из файла: остаются записи стоящих в стилях (used — как
// UsedFonts: по возрастанию, без повторов; не больше первых MaxFonts, как у SyncFonts), название —
// одна строка ≤ MaxFontNameRunes. Ключ не id своей в used не бывает — такая запись выброшена; лишние
// записи чужого файла не вытесняют стоящие. Пустая — nil.
func ClampFonts(in map[string]FontRef, used []string) map[string]FontRef {
	var out map[string]FontRef
	for _, id := range used[:min(len(used), MaxFonts)] {
		r, ok := in[id]
		if !ok || !text.IsCustomID(id) {
			continue
		}
		if out == nil {
			out = map[string]FontRef{}
		}
		out[id] = FontRef{Name: cleanLine(r.Name, MaxFontNameRunes)}
	}
	return out
}

// fontMissing — своя гарнитура, которой нет на этом компьютере: FitTexts такой блок не трогает
// (082-5), иначе ширина «по тексту» и положение посчитались бы по Montserrat и испортили файл.
func fontMissing(id string) bool { return text.IsCustomID(id) && !text.Available(id) }
