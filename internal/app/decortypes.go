package app

// Типы биндингов окон оформления («Фоны», «Стикеры», «Рамочки», «Развороты») — копии decor.*
// (решение №178): namespace Wails = имя пакета.

// DecorItem — плитка каталога (decor.Item).
type DecorItem struct {
	ID     string  `json:"id"` // sa:<пресет> | my:<16 hex>
	Theme  string  `json:"theme"`
	Name   string  `json:"name"`
	URL    string  `json:"url"` // без px: UI дописывает &px=
	Own    bool    `json:"own"`
	Aspect float64 `json:"aspect"` // ширина/высота картинки; узор — 0
	W      int     `json:"w"`
	H      int     `json:"h"`
	Color  string  `json:"color,omitempty"` // цвет страниц пресета: подложка плитки
	Dark   bool    `json:"dark,omitempty"`  // светлый стикер (116): плитке — тёмная шашечка
	// Pages, Places — у разворота (067): 1 — страница, 2 — разворот; мест в шаблоне.
	Pages  int    `json:"pages,omitempty"`
	Places int    `json:"places,omitempty"`
	Pack   string `json:"pack,omitempty"` // название набора: подсказка «Набор «…»»
	// Steps, From, Vig, Cover — у макета альбома (131): разворотов и мест при n по умолчанию (с
	// обложкой), есть ли шаг-виньетка и оформление обложки.
	Steps int  `json:"steps,omitempty"`
	From  int  `json:"from,omitempty"`
	Vig   bool `json:"vig,omitempty"`
	Cover bool `json:"cover,omitempty"`
	// PackID — чей предмет (133): "sa:builtin" | id набора; своё — нет. Peek — крупная картинка
	// без px (своё и набор); нет — URL с px.
	PackID string `json:"packId,omitempty"`
	Peek   string `json:"peek,omitempty"`
	// Form, Style — у стикера набора и встроенного: "corner" | "divider" | нет (стикер); стиль — у обычного.
	Form  string `json:"form,omitempty"`
	Style string `json:"style,omitempty"`
}

// DecorTheme — строка списка тем слева.
type DecorTheme struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// DecorCatalog — содержимое окна одного вида.
type DecorCatalog struct {
	Kind     string       `json:"kind"`
	Themes   []DecorTheme `json:"themes"`
	Items    []DecorItem  `json:"items"`
	Recent   []string     `json:"recent"` // ID плиток, новые первыми
	New      []string     `json:"new"`    // ID плиток «Новые» (133) в порядке каталога; пусто — []
	ReadOnly bool         `json:"readOnly"`
}

// DecorSkip — пропущенный при загрузке файл и причина.
type DecorSkip struct {
	Name string `json:"name"`
	Why  string `json:"why"`
}

// DecorSvg — проверенный SVG стикера: окно рисует Data (base64) холстом W×H и присылает PNG
// в AddDecorPNG с Token (design §5.4).
type DecorSvg struct {
	Token string `json:"token"`
	Name  string `json:"name"`
	Data  string `json:"data"`
	W     int    `json:"w"`
	H     int    `json:"h"`
}

// DecorImport — итог «Загрузить своё…»: добавленные (и возвращённые из скрытых), пропущенные
// и задания окну на SVG (только стикеры).
type DecorImport struct {
	Added   []DecorItem `json:"added"`
	Skipped []DecorSkip `json:"skipped"`
	Svg     []DecorSvg  `json:"svg"`
}

// newDecorImport — пустой итог: срезы не nil, интерфейс не проверяет null.
func newDecorImport() DecorImport {
	return DecorImport{Added: []DecorItem{}, Skipped: []DecorSkip{}, Svg: []DecorSvg{}}
}
