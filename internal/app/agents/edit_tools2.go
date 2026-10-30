package agents

import (
	"github.com/google/jsonschema-go/jsonschema"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

// Инструменты 0.34 (151): лист-сетка, отметки, кадр обложки, фон, стикеры и формат типографии. Образец —
// edit_tools.go; правящие идут через regEdit (флажок, место, темп, ключ и rev, «человек занят», копия).
// Тела правящих — edit_run34.go; Host.Mark…Print заполнены только вместе (canEdit34).

const (
	maxMarkIDs  = 1000 // фото за один вызов set_marks
	MaxStickers = 12   // стикеров за один вызов add_sticker (фасад App сверяет длины тем же пределом)
)

// Перечисления входа.
var (
	flagWords    = enum("pick", "reject", "none")
	labelWords   = enum("red", "yellow", "green", "blue", "purple", "none")
	scopeWords   = enum("spread", "left", "right")
	pageWords    = enum("left", "right")
	placeWords   = enum("center", "top-left", "top-right", "bottom-left", "bottom-right")
	stickerSizes = enum("s", "m", "l")
)

func registerTools2(e *toolEnv) {
	if e.s.h.Sheet != nil {
		registerSheet(e)
	}
	if !e.s.h.canEdit34() {
		return
	}
	regEdit(e, toolSpec{name: "set_marks", perm: prefs.ClassEdit, title: "Отметки фото", edit: true, backup: true,
		desc: "Поставить отметки фото ленты, как в ленте у человека: оценку (rating 0…5, 0 снимает звёзды), флажок (flag: pick — отобрано, " +
			"reject — отклонено, none — снять) и цвет (label: red, yellow, green, blue, purple, none). ids — из list_photos или get_contact_sheet; " +
			"хоть одно из rating, flag, label обязательно. Фото не из ленты попадают в skipped. Один вызов — один шаг отмены." + editNote},
		marksProps, func(in marksIn) *AlbumRef { return &in.Album }, true,
		func(c *call, in marksIn) (marksOut, error) { return c.setMarks(in) })

	regEdit(e, toolSpec{name: "set_cover_photo", perm: prefs.ClassEdit, title: "Фото на обложку", edit: true, backup: true,
		desc: "Поставить фото (id из list_photos) на обложку. Обложки нет — она создаётся с этим фото, как бросок на «+ Обложка». " +
			"Обложка есть: фото встаёт на лицевую — в пустое место, а пустых нет — вместо кадра в наибольшем (прежний уходит в ленту); " +
			"мест нет — на фон лицевой. Тот же кадр повторно — changed: false. Другой кадр — новый вызов. У альбома-листа обложки нет — отказ. " +
			"Кадр проверяйте get_photo_preview." + editNote},
		func(p map[string]*jsonschema.Schema) { p["photo"].MaxLength, p["photo"].MinLength = ptr(maxID), ptr(1) },
		func(in setCoverIn) *AlbumRef { return &in.Album }, true,
		func(c *call, in setCoverIn) (setCoverOut, error) { return c.setCoverPhoto(in) })

	regEdit(e, toolSpec{name: "set_background", perm: prefs.ClassEdit, title: "Фон на развороты", edit: true, backup: true,
		desc: "Поставить фон из каталога (item — id из list_decor kind=bg) на развороты: номера с 1 или id, одним шагом отмены. scope: spread — " +
			"весь разворот (по умолчанию), left или right — страница. Листы, где фон — фото клиента, пропускаются (skipped): спросите человека и " +
			"повторите с replace: true, только если он согласен. Обложка не цель. Порядок работы: формат (apply_print) → сборка → обложка → фон и стикеры." + editNote},
		bgProps, func(in bgIn) *AlbumRef { return &in.Album }, true,
		func(c *call, in bgIn) (bgOut, error) { return c.setBackground(in) })

	regEdit(e, toolSpec{name: "add_sticker", perm: prefs.ClassEdit, title: "Стикеры на развороты", edit: true, backup: true,
		desc: "Добавить стикеры из каталога (item — id из list_decor kind=sticker) на развороты, до 12 за вызов одним шагом отмены. " +
			"spread — номер с 1 или id (обложка не цель), page: left | right (по умолчанию right), place: center или угол страницы в безопасной зоне " +
			"(по умолчанию center), size: s, m, l — ⅙, ¼, ⅓ ширины страницы (по умолчанию m). Нет места на листе или в альбоме — отказ всей пачки. " +
			"Точнее стикеры двигает человек." + editNote},
		stickerProps, func(in addStickerIn) *AlbumRef { return &in.Album }, true,
		func(c *call, in addStickerIn) (addStickerOut, error) { return c.addSticker(in) })

	regEdit(e, toolSpec{name: "apply_print", perm: prefs.ClassEdit, title: "Формат типографии", edit: true, backup: true,
		desc: "Сменить формат типографии, как кнопка формата в «Настройках альбома»: страница, dpi, склейка, зона и обрез; поля, промежуток и цвет " +
			"остаются. profile — id из list_print_profiles, format — имя формата оттуда (30x20 = 30×20; без него — первый). Изделие обложки: стоит " +
			"изделие этой типографии или «Свои размеры» — остаётся, шаблон идёт за форматом; иначе ставится изделие по умолчанию типографии. " +
			"Формат меняйте до сборки и оформления: смена пересчитывает все листы. Если обложку создали после, повторите apply_print." + editNote},
		func(p map[string]*jsonschema.Schema) {
			p["profile"].MinLength, p["profile"].MaxLength = ptr(1), ptr(maxID)
			p["format"].MaxLength = ptr(maxID)
		},
		func(in applyPrintIn) *AlbumRef { return &in.Album }, true,
		func(c *call, in applyPrintIn) (applyPrintOut, error) { return c.applyPrint(in) })
}

type marksIn struct {
	Album  AlbumRef `json:"album"`
	IDs    []string `json:"ids" jsonschema:"id фото из list_photos или get_contact_sheet"`
	Rating *int     `json:"rating,omitempty" jsonschema:"оценка 0…5; 0 — снять звёзды"`
	Flag   string   `json:"flag,omitempty" jsonschema:"pick — отобрано, reject — отклонено, none — снять"`
	Label  string   `json:"label,omitempty" jsonschema:"цвет: red, yellow, green, blue, purple или none"`
}

func marksProps(p map[string]*jsonschema.Schema) {
	p["ids"].MinItems, p["ids"].MaxItems = ptr(1), ptr(maxMarkIDs)
	p["ids"].Items = &jsonschema.Schema{Type: "string", MinLength: ptr(1), MaxLength: ptr(maxID)}
	p["rating"] = &jsonschema.Schema{Type: "integer", Minimum: ptr(0.0), Maximum: ptr(5.0), Description: p["rating"].Description}
	p["flag"].Enum = flagWords
	p["label"].Enum = labelWords
}

type marksOut struct {
	Album   AlbumRef `json:"album" jsonschema:"новая ревизия"`
	Changed int      `json:"changed" jsonschema:"у скольких фото отметка сменилась; 0 — правки нет"`
	Skipped []string `json:"skipped,omitempty" jsonschema:"id, которых нет в ленте: пропущены"`
}

type setCoverIn struct {
	Album AlbumRef `json:"album"`
	Photo string   `json:"photo" jsonschema:"id фото из list_photos"`
}

type setCoverOut struct {
	Album   AlbumRef `json:"album" jsonschema:"новая ревизия"`
	Created bool     `json:"created,omitempty" jsonschema:"обложки не было — создана"`
	Product string   `json:"product,omitempty" jsonschema:"изделие созданной обложки"`
	Place   string   `json:"place" jsonschema:"seat — место лицевой, background — фон лицевой"`
	Changed bool     `json:"changed" jsonschema:"false — этот кадр уже стоит, правки нет"`
}

type bgIn struct {
	Album   AlbumRef    `json:"album"`
	Item    string      `json:"item" jsonschema:"id фона из list_decor kind=bg"`
	Spreads []spreadRef `json:"spreads" jsonschema:"развороты: номер с 1 или id"`
	Scope   string      `json:"scope,omitempty" jsonschema:"spread — весь разворот (по умолчанию), left или right — страница"`
	Replace bool        `json:"replace,omitempty" jsonschema:"заменять и фото клиента на фоне — только с согласия человека"`
}

func bgProps(p map[string]*jsonschema.Schema) {
	p["item"].MinLength, p["item"].MaxLength = ptr(1), ptr(maxID)
	p["spreads"].MinItems, p["spreads"].MaxItems = ptr(1), ptr(model.MaxSpreads)
	p["spreads"].Items = &jsonschema.Schema{Types: []string{"string", "integer"}, MaxLength: ptr(maxID)}
	p["scope"].Enum = scopeWords
}

type bgOut struct {
	Album    AlbumRef `json:"album" jsonschema:"новая ревизия"`
	Changed  int      `json:"changed" jsonschema:"листов изменено; 0 — правки нет"`
	Skipped  []int    `json:"skipped,omitempty" jsonschema:"номера листов, где фон — фото клиента: пропущены"`
	Replaced bool     `json:"replaced,omitempty" jsonschema:"фото фона заменено (только с replace)"`
}

// stickerPut — один стикер пачки add_sticker.
type stickerPut struct {
	Spread spreadRef `json:"spread" jsonschema:"номер разворота с 1 или id"`
	Page   string    `json:"page,omitempty" jsonschema:"left или right (по умолчанию right)"`
	Item   string    `json:"item" jsonschema:"id стикера из list_decor kind=sticker"`
	Place  string    `json:"place,omitempty" jsonschema:"center, top-left, top-right, bottom-left, bottom-right (по умолчанию center)"`
	Size   string    `json:"size,omitempty" jsonschema:"s, m или l — ⅙, ¼, ⅓ ширины страницы (по умолчанию m)"`
}

type addStickerIn struct {
	Album    AlbumRef     `json:"album"`
	Stickers []stickerPut `json:"stickers" jsonschema:"до 12 стикеров за вызов"`
}

func stickerProps(p map[string]*jsonschema.Schema) {
	st := p["stickers"]
	st.MinItems, st.MaxItems = ptr(1), ptr(MaxStickers)
	it := st.Items.Properties
	spreadProp(it, "номер разворота с 1 или id")
	it["page"].Enum, it["place"].Enum, it["size"].Enum = pageWords, placeWords, stickerSizes
	it["item"].MinLength, it["item"].MaxLength = ptr(1), ptr(maxID)
	st.Items.AdditionalProperties = &jsonschema.Schema{Not: &jsonschema.Schema{}}
}

type addStickerOut struct {
	Album AlbumRef `json:"album" jsonschema:"новая ревизия"`
	Added int      `json:"added" jsonschema:"добавлено стикеров"`
	IDs   []string `json:"ids,omitempty" jsonschema:"id добавленных стикеров"`
}

type applyPrintIn struct {
	Album   AlbumRef `json:"album"`
	Profile string   `json:"profile" jsonschema:"id типографии из list_print_profiles"`
	Format  string   `json:"format,omitempty" jsonschema:"имя формата из list_print_profiles (по умолчанию первый)"`
}

type applyPrintPage struct {
	W float64 `json:"w" jsonschema:"ширина страницы, мм"`
	H float64 `json:"h" jsonschema:"высота страницы, мм"`
}

type applyPrintOut struct {
	Album        AlbumRef       `json:"album" jsonschema:"новая ревизия"`
	Page         applyPrintPage `json:"page"`
	Cover        string         `json:"cover,omitempty" jsonschema:"изделие обложки в итоге; пусто — обложки нет"`
	CoverChanged bool           `json:"coverChanged,omitempty" jsonschema:"изделие обложки сменилось"`
	Changed      bool           `json:"changed" jsonschema:"false — формат уже такой, правки нет"`
	OverLimit    bool           `json:"overLimit,omitempty" jsonschema:"разворотов больше предела типографии"`
}
