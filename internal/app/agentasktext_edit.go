package app

// Тексты вопроса о правке при «Спрашивать» (150, design §5.2, 0.34-150-4): шаблоны Go по числам и
// названиям входа. Своего текста агент не вносит: в окно попадают только номера разворотов, названия
// семейств, фонов и типографий по каталогу и числа.

import (
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// Инструменты MCP, которым фасад задаёт вопрос о правке (у add_folder и create_album свои виды).
const (
	toolAutobuild = "autobuild"
	toolVary      = "vary_layouts"
	toolShow      = "show_spread"
	toolUndo      = "undo_last_edit"
	toolMarks     = "set_marks"
	toolCover     = "set_cover_photo"
	toolBg        = "set_background"
	toolSticker   = "add_sticker"
	toolPrint     = "apply_print"
)

const (
	askUndoLine   = "Ctrl+Z отменит одним шагом."
	askCoverWord  = "обложка" // метка обложки в editAsk.Places
	askSpreadsMax = 6         // сколько номеров перечислять; больше — «на 12 разворотах»
)

// editAsk — что вошло в вопрос о правке: числа и названия, уже разобранные фасадом по каталогам.
type editAsk struct {
	Client   string
	Tool     string
	Count    int      // фото или стикеров; 0 — не названо
	Build    int      // autobuild: сколько разворотов просит агент; 0 — не названо
	Places   []string // номера разворотов по возрастанию; askCoverWord — обложка
	Families []string // названия семейств раскладок
	Item     string   // фон или типография по каталогу; "" — неизвестно или не годится для показа
	Format   string   // формат типографии
	Sheet    bool     // альбом-лист: «лист» вместо «разворот»
	// set_background (154): фон альбома или замена фото фона; числа фасад считает по виду, а не со слов агента.
	AlbumBg    bool
	OwnCat     []int // фон альбома: листы, с которых уйдёт свой фон каталога (и листы согласия)
	KeepClient []int // фон альбома: листы с фото клиента вне согласия — остаются
	TakeClient []int // фон альбома: листы согласия с фото клиента
	Photos     int   // фото фона, которые уйдут: на листах согласия (фон альбома) или по плану (фон на набор); 0 — строки нет
}

// familyNames — подписи семейств раскладок (variety.Family), те же, что FAMILIES в variants.ts.
var familyNames = map[string]string{
	"dense": "Плотно", "hero": "С главным фото", "air": "Воздух", "bleed": "В край", "strip": "Ленты", "figure": "Фигуры",
}

// editAskTools — инструменты с шаблоном вопроса о правке.
func editAskTools() []string {
	return []string{toolAutobuild, toolVary, toolShow, toolUndo, toolMarks, toolCover, toolBg, toolSticker, toolPrint}
}

// editAskBody — заголовок, абзацы и кнопка вопроса; false — шаблона для инструмента нет (вопрос не
// задаётся, агент получает ошибку).
func editAskBody(e editAsk) (askBody, bool) {
	n := max(e.Count, 0)
	undo := true // правка альбома даёт один шаг Ctrl+Z
	var title, ok, more string
	var extra []string
	switch e.Tool {
	case toolAutobuild:
		title, ok = "Собрать "+e.buildWhat()+" из "+freePhotos(n)+" в конец альбома?", "Собрать"
		if len(e.Families) > 0 {
			more = "Раскладки только " + quoteList(e.Families) + "."
		}
	case toolVary:
		title, ok = "Другие раскладки "+e.onSpreads()+famSuffix(e.Families)+"?", "Изменить раскладки"
	case toolShow:
		title, ok, undo = "Показать "+e.what()+"?", "Показать", false
		more = "Альбом не изменится."
	case toolUndo:
		title, ok, undo = "Отменить последнюю правку агента?", "Отменить правку", false
		more = "Альбом вернётся к тому, каким был до неё."
	case toolMarks:
		title, ok = "Изменить отметки "+forPhotos(n), "Изменить отметки"
		more = "Оценка, флаг или цвет — как просит агент."
	case toolCover:
		title, ok = "Поставить другое фото на обложку?", "Поставить"
	case toolBg:
		ok = "Поставить фон"
		if e.AlbumBg {
			title, more, extra = "Поставить "+e.itemOr("выбранный фон")+" фоном альбома?", e.albumBgMore(), e.albumBgLines()
		} else if title = "Поставить " + e.itemOr("выбранный фон") + " " + e.onSpreads() + "?"; e.Photos > 0 {
			more = "Фото фона заменятся: " + strconv.Itoa(e.Photos)
		}
	case toolSticker:
		title, ok = "Добавить "+stickers(n)+" "+e.onSpreads()+"?", "Добавить"
	case toolPrint:
		title, ok = "Поставить формат "+e.printName()+"?", "Применить формат"
		more = "Размер страниц изменится у всех листов и обложки."
	default:
		return askBody{}, false
	}
	lines := []string{clientQuoted(e.Client) + " просит об этом."}
	if more != "" {
		lines = append(lines, more)
	}
	lines = append(lines, extra...)
	if undo {
		lines = append(lines, askUndoLine)
	}
	return askBody{Title: title, Lines: append(lines, askWho), OK: ok}, true
}

// albumBgMore — «Он ляжет на все развороты без своего фона.»
func (e editAsk) albumBgMore() string {
	if e.Sheet {
		return "Он ляжет на все листы без своего фона."
	}
	return "Он ляжет на все развороты без своего фона."
}

// albumBgLines — что фон альбома сделает со своими фонами листов (числа — по виду, 154 design §1.4);
// строки без листов не пишутся.
func (e editAsk) albumBgLines() (lines []string) {
	from, on := "разворотов", "разворотах"
	if e.Sheet {
		from, on = "листов", "листах"
	}
	if len(e.OwnCat) > 0 {
		lines = append(lines, "Свои фоны из каталога уйдут с "+from+": "+numsText(e.OwnCat))
	}
	if len(e.KeepClient) > 0 {
		lines = append(lines, "Фото клиента на фоне останутся на "+on+": "+numsText(e.KeepClient))
	}
	if len(e.TakeClient) > 0 {
		lines = append(lines, "Фото фона уйдут с "+from+" "+numsText(e.TakeClient)+": "+strconv.Itoa(e.Photos)+" фото")
	}
	return lines
}

// buildWhat — «3 разворота» / «развороты» (число не названо); у листа — «листы».
func (e editAsk) buildWhat() string {
	one, few, many, none := "разворот", "разворота", "разворотов", "развороты"
	if e.Sheet {
		one, few, many, none = "лист", "листа", "листов", "листы"
	}
	if e.Build <= 0 {
		return none
	}
	return strconv.Itoa(e.Build) + " " + plural(e.Build, one, few, many)
}

// freePhotos — «96 свободных фото» / «21 свободного фото»; 0 — «всех свободных фото».
func freePhotos(n int) string {
	switch {
	case n == 0:
		return "всех свободных фото"
	case n%10 == 1 && n%100 != 11:
		return strconv.Itoa(n) + " свободного фото"
	}
	return strconv.Itoa(n) + " свободных фото"
}

// forPhotos — «у 12 фото» / «у выбранных фото».
func forPhotos(n int) string {
	if n == 0 {
		return "у выбранных фото?"
	}
	return "у " + strconv.Itoa(n) + " фото?"
}

// stickers — «3 стикера»; 0 — «стикеры».
func stickers(n int) string {
	if n == 0 {
		return "стикеры"
	}
	return strconv.Itoa(n) + " " + plural(n, "стикер", "стикера", "стикеров")
}

// quoteList — «Плотно», «Воздух».
func quoteList(names []string) string {
	q := make([]string, len(names))
	for i, n := range names {
		q[i] = "«" + n + "»"
	}
	return strings.Join(q, ", ")
}

// famSuffix — « — «Плотно», «Воздух»»; без семейств пусто.
func famSuffix(names []string) string {
	if len(names) == 0 {
		return ""
	}
	return " — " + quoteList(names)
}

// onSpreads — «на развороте 8», «на разворотах 3, 8», «на 12 разворотах», «на обложке», «на обложке и развороте 8».
func (e editAsk) onSpreads() string {
	one, many := "развороте", "разворотах"
	if e.Sheet {
		one, many = "листе", "листах"
	}
	cover := slices.Contains(e.Places, askCoverWord)
	nums := slices.DeleteFunc(slices.Clone(e.Places), func(s string) bool { return s == askCoverWord })
	var tail string
	switch {
	case len(nums) == 1:
		tail = one + " " + nums[0]
	case len(nums) > askSpreadsMax:
		tail = strconv.Itoa(len(nums)) + " " + many
	case len(nums) > 1:
		tail = many + " " + strings.Join(nums, ", ")
	}
	switch {
	case cover && tail != "":
		return "на обложке и " + tail
	case cover:
		return "на обложке"
	case tail == "":
		return "на выбранных " + many
	}
	return "на " + tail
}

// what — «разворот 8» / «обложку» для show_spread.
func (e editAsk) what() string {
	unit := "разворот"
	if e.Sheet {
		unit = "лист"
	}
	switch {
	case slices.Contains(e.Places, askCoverWord):
		return "обложку"
	case len(e.Places) == 0:
		return unit
	}
	return unit + " " + e.Places[0]
}

// itemOr — «фон «Цветы»» или запасная фраза.
func (e editAsk) itemOr(none string) string {
	if e.Item == "" {
		return none
	}
	return "фон «" + e.Item + "»"
}

// printName — ««Фабрика Фотокниги 30×20»»; без названия — «выбранной типографии».
func (e editAsk) printName() string {
	switch {
	case e.Item != "" && e.Format != "":
		return "«" + e.Item + " " + e.Format + "»"
	case e.Item != "":
		return "«" + e.Item + "»"
	}
	return "выбранной типографии"
}

// catalogName — название из каталога для показа: плохие знаки и пустое — "" (текст подставит запасное),
// длинное — с «…».
func catalogName(s string) string {
	s = strings.TrimSpace(s)
	if s == "" || paths.BadRunes(s) {
		return ""
	}
	if utf8.RuneCountInString(s) > askNameMax {
		return string([]rune(s)[:askNameMax-1]) + "…"
	}
	return s
}
