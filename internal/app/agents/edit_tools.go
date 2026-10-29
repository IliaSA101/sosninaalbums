package agents

import (
	"bytes"
	"encoding/json"
	"errors"
	"time"
	"unicode/utf8"

	"github.com/google/jsonschema-go/jsonschema"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Инструменты правок 0.33 (149 design §1). start_program — у посредника (internal/mcpbridge).
// Описание правящего кончается «Правка видна в программе, Ctrl+Z её отменяет» (чтение — dataNote).

const (
	editNote = " Правка видна в программе, Ctrl+Z её отменяет."
	askNote  = " Человек отвечает в окне программы; если он не ответил за несколько секунд, придёт pending с жетоном: " +
		"попросите человека ответить в окне и вызовите await_answer. Отказ — не повторять без его просьбы в чате."
	maxHint    = 4096 // folder_hint, байт
	maxAutoIDs = 5000 // photos у autobuild
	maxPer     = 64   // фото на разворот
)

// families — семейства раскладок (variety.Family): «плотно» dense, «воздух» air.
var families = []string{"dense", "hero", "air", "bleed", "strip", "figure"}

func registerEditTools(e *toolEnv) {
	regEdit(e, toolSpec{name: "create_album", title: "Создать альбом", edit: true, ask: true, danger: true,
		desc: "Создать новый альбом из папки с фотографиями. Программа спросит человека в своём окне и откроет выбор папки; " +
			"путь агент не передаёт и не получает. folder_hint (необязательно) — лишь подсказка, где открыть выбор. " +
			"Открытый альбом сохранится в свой файл. Создание нельзя отменить через undo_last_edit." + askNote + editNote},
		hintProps, func(createIn) *AlbumRef { return nil }, false,
		func(c *call, in createIn) (askOut, error) { return c.createAlbum(in) })

	regEdit(e, toolSpec{name: "add_folder", title: "Добавить папку", edit: true, ask: true,
		desc: "Добавить в открытый альбом папку с фотографиями. Программа спросит человека и откроет выбор папки; " +
			"путь агент не передаёт и не получает. folder_hint — подсказка, где открыть выбор." + askNote + editNote},
		hintProps, func(in addFolderIn) *AlbumRef { return &in.Album }, true,
		func(c *call, in addFolderIn) (askOut, error) { return c.addFolder(in) })

	reg(e, toolSpec{name: "await_answer", title: "Ответ человека", await: true, ask: true,
		desc: "Дождаться ответа человека на вопрос, о котором сообщил pending: возвращает то же, что вернул бы вызов, " +
			"задавший вопрос, либо снова pending (подождите и повторите). Жетон ask работает только в этом подключении."},
		func(p map[string]*jsonschema.Schema) {
			p["ask"].MinLength, p["ask"].MaxLength = ptr(1), ptr(maxID)
		},
		func(c *call, in awaitIn) (askOut, error) { return c.awaitAnswer(in) })

	regEdit(e, toolSpec{name: "autobuild", title: "Автосборка", edit: true, auto: true, backup: true,
		desc: "Собрать развороты в конец альбома из свободных фото (не в альбоме, не на фоне, не отклонённые; фильтр и выделение " +
			"ленты не учитываются) с умолчаниями окна «Автосборка»: «Авто», серии, «Обычное», все семейства. Входы необязательны: " +
			"spreads, perSpread, series, similar, variant, variety (0…2), families, photos (только эти фото из свободных). " +
			"Ждёт, пока программа дочитает папки." + editNote},
		autoProps, func(in autoIn) *AlbumRef { return &in.Album }, true,
		func(c *call, in autoIn) (autobuildOut, error) { return c.autobuild(in) })

	regEdit(e, toolSpec{name: "vary_layouts", title: "Другие раскладки", edit: true, backup: true,
		desc: "Подобрать другие раскладки выбранным разворотам (номер с 1 или id из list_spreads). families: dense — плотнее, " +
			"air — больше воздуха, ещё hero, bleed, strip, figure; без families — любые. variety 1 или 2 (по умолчанию 1)." + editNote},
		varyProps, func(in varyIn) *AlbumRef { return &in.Album }, true,
		func(c *call, in varyIn) (varyOut, error) { return c.varyLayouts(in) })

	regEdit(e, toolSpec{name: "show_spread", title: "Показать разворот", view: true,
		desc: "Перевести программу к развороту (номер с 1, \"cover\" или id): человек увидит его. Альбом не меняется, шага отмены нет."},
		func(p map[string]*jsonschema.Schema) {
			spreadProp(p, "номер разворота с 1, \"cover\" (обложка) или id из list_spreads")
		}, func(in showIn) *AlbumRef { return &AlbumRef{Key: in.Album.Key} }, false,
		func(c *call, in showIn) (showOut, error) { return c.showSpread(in) })

	regEdit(e, toolSpec{name: "undo_last_edit", title: "Отменить свою правку", edit: true,
		desc: "Отменить последнюю правку, которую сделал этот агент (autobuild, vary_layouts, add_folder), если после неё альбом " +
			"не менялся. Только одна правка: второй вызов подряд — отказ. Создание альбома не отменяется." + editNote},
		nil, func(in undoIn) *AlbumRef { return &in.Album }, false,
		func(c *call, in undoIn) (undoOut, error) { return c.undoLast(in) })
}

type createIn struct {
	FolderHint string `json:"folder_hint,omitempty" jsonschema:"подсказка, где открыть выбор папки (путь не используется как есть)"`
}

type addFolderIn struct {
	Album      AlbumRef `json:"album"`
	FolderHint string   `json:"folder_hint,omitempty" jsonschema:"подсказка, где открыть выбор папки (путь не используется как есть)"`
}

type awaitIn struct {
	Ask string `json:"ask" jsonschema:"жетон из pending"`
}

type keyIn struct {
	Key string `json:"key" jsonschema:"ключ альбома из get_album"`
}

type showIn struct {
	Album  keyIn     `json:"album"`
	Spread spreadRef `json:"spread"`
}

type undoIn struct {
	Album AlbumRef `json:"album"`
}

type autoIn struct {
	Album     AlbumRef   `json:"album"`
	Spreads   int        `json:"spreads,omitempty" jsonschema:"сколько разворотов собрать (по умолчанию как в окне: фото / 4)"`
	PerSpread *perSpread `json:"perSpread,omitempty" jsonschema:"фото на разворот: \"auto\" или {min, max}"`
	Series    *bool      `json:"series,omitempty" jsonschema:"не разрывать серии съёмки (по умолчанию да)"`
	Similar   int        `json:"similar,omitempty" jsonschema:"убирать похожие кадры: 0 нет, 1 дубли, 2 очень похожие, 3 похожие"`
	Variant   int        `json:"variant,omitempty" jsonschema:"номер варианта: другие границы и раскладки (по умолчанию 1)"`
	Variety   *int       `json:"variety,omitempty" jsonschema:"разнообразие раскладок 0…2 (по умолчанию 1 — «Обычное»)"`
	Families  []string   `json:"families,omitempty" jsonschema:"разрешённые семейства раскладок (по умолчанию все)"`
	Photos    []string   `json:"photos,omitempty" jsonschema:"id фото из list_photos: собрать только их, если они свободны"`
}

type varyIn struct {
	Album    AlbumRef    `json:"album"`
	Spreads  []spreadRef `json:"spreads" jsonschema:"развороты: номер с 1 или id"`
	Variety  int         `json:"variety,omitempty" jsonschema:"1 или 2 (по умолчанию 1)"`
	Families []string    `json:"families,omitempty" jsonschema:"разрешённые семейства раскладок (по умолчанию все)"`
}

// perSpread — «фото на разворот»: "auto" или {min, max}.
type perSpread struct {
	Auto     bool
	Min, Max int
}

func (p *perSpread) UnmarshalJSON(b []byte) error {
	var s string
	if json.Unmarshal(b, &s) == nil {
		if s != "auto" {
			return errors.New("perSpread")
		}
		*p = perSpread{Auto: true}
		return nil
	}
	var o struct {
		Min int `json:"min"`
		Max int `json:"max"`
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if err := d.Decode(&o); err != nil || o.Min < 1 || o.Max > maxPer || o.Min > o.Max {
		return errors.New("perSpread")
	}
	*p = perSpread{Min: o.Min, Max: o.Max}
	return nil
}

type planOut struct {
	From    int `json:"from" jsonschema:"фото на разворот: от"`
	To      int `json:"to" jsonschema:"фото на разворот: до"`
	Similar int `json:"similar" jsonschema:"похожих кадров не взято"`
}

type autobuildOut struct {
	Album  AlbumRef `json:"album" jsonschema:"новая ревизия"`
	Added  int      `json:"added" jsonschema:"собрано разворотов"`
	Photos int      `json:"photos" jsonschema:"фото в сборке"`
	Plan   planOut  `json:"plan"`
	First  int      `json:"first" jsonschema:"номер первого нового разворота с 1; 0 — нет"`
}

type varyOut struct {
	Album   AlbumRef `json:"album" jsonschema:"новая ревизия"`
	Changed int      `json:"changed" jsonschema:"разворотов изменено; 0 — менять нечего, правки нет"`
}

type showOut struct {
	Spread int `json:"spread" jsonschema:"номер разворота с 1; 0 — обложка"`
}

type undoOut struct {
	Album AlbumRef `json:"album" jsonschema:"новая ревизия"`
}

func hintProps(p map[string]*jsonschema.Schema) { p["folder_hint"].MaxLength = ptr(maxHint) }

func familiesProp() *jsonschema.Schema {
	return &jsonschema.Schema{Type: "array", MaxItems: ptr(len(families)), Description: "разрешённые семейства раскладок (по умолчанию все): " +
		"dense — плотнее, air — больше воздуха, hero, bleed, strip, figure",
		Items: &jsonschema.Schema{Type: "string", Enum: enum("dense", "hero", "air", "bleed", "strip", "figure")}}
}

func autoProps(p map[string]*jsonschema.Schema) {
	p["spreads"].Minimum, p["spreads"].Maximum = ptr(1.0), ptr(float64(model.MaxSpreads))
	p["similar"].Minimum, p["similar"].Maximum = ptr(0.0), ptr(float64(model.MaxAutoLevel))
	p["variant"].Minimum, p["variant"].Maximum = ptr(0.0), ptr(float64(model.MaxVariant))
	p["variety"] = &jsonschema.Schema{Type: "integer", Minimum: ptr(0.0), Maximum: ptr(2.0), Description: p["variety"].Description}
	p["series"] = &jsonschema.Schema{Type: "boolean", Description: p["series"].Description}
	p["families"] = familiesProp()
	p["photos"].MaxItems = ptr(maxAutoIDs)
	p["photos"].Items = &jsonschema.Schema{Type: "string", MaxLength: ptr(maxID)}
	p["perSpread"] = &jsonschema.Schema{Description: p["perSpread"].Description, AnyOf: []*jsonschema.Schema{
		{Type: "string", Enum: enum("auto")},
		{Type: "object", Required: []string{"min", "max"}, AdditionalProperties: &jsonschema.Schema{Not: &jsonschema.Schema{}}, Properties: map[string]*jsonschema.Schema{
			"min": {Type: "integer", Minimum: ptr(1.0), Maximum: ptr(float64(maxPer))},
			"max": {Type: "integer", Minimum: ptr(1.0), Maximum: ptr(float64(maxPer))},
		}},
	}}
}

func varyProps(p map[string]*jsonschema.Schema) {
	p["spreads"].MinItems, p["spreads"].MaxItems = ptr(1), ptr(model.MaxSpreads)
	p["spreads"].Items = &jsonschema.Schema{Types: []string{"string", "integer"}, MaxLength: ptr(maxID)}
	p["variety"].Minimum, p["variety"].Maximum = ptr(1.0), ptr(2.0)
	p["families"] = familiesProp()
}

// dedupe — строки без повторов в прежнем порядке.
func dedupe(in []string) []string {
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

// hint проверяет folder_hint: предел в байтах и UTF-8 (схема считает знаки). Смысл подсказки — у
// фасада; пустая — «нет подсказки».
func hint(s string) (string, error) {
	if len(s) > maxHint || !utf8.ValidString(s) {
		return "", errBadFolder
	}
	return s, nil
}

func (c *call) createAlbum(in createIn) (askOut, error) {
	h, err := hint(in.FolderHint)
	if err != nil {
		return askOut{}, err
	}
	return c.ask(AskReq{Kind: AskCreate, Client: askClient(c.client()), Hint: h})
}

func (c *call) addFolder(in addFolderIn) (askOut, error) {
	h, err := hint(in.FolderHint)
	if err != nil {
		return askOut{}, err
	}
	return c.ask(AskReq{Kind: AskAdd, Client: askClient(c.client()), Hint: h, Key: in.Album.Key, Rev: in.Album.Rev})
}

func (c *call) autobuild(in autoIn) (autobuildOut, error) {
	req := AutoReq{Key: in.Album.Key, Rev: in.Album.Rev, Client: askClient(c.client()), Spreads: in.Spreads, PerAuto: true, Series: true,
		Similar: in.Similar, Variant: max(in.Variant, 1), Variety: 1, Families: dedupe(in.Families), Photos: dedupe(in.Photos)}
	if in.Series != nil {
		req.Series = *in.Series
	}
	if in.Variety != nil {
		req.Variety = *in.Variety
	}
	if p := in.PerSpread; p != nil && !p.Auto {
		req.PerAuto, req.PerMin, req.PerMax = false, p.Min, p.Max
	}
	// Ожидание ленты — остаток срока вызова минус запас: после него ещё нужно собрать и записать (В8).
	if dl, ok := c.ctx.Deadline(); ok {
		req.ScanWaitMs = int(max(time.Until(dl)-c.s.o.ScanSlack, 0) / time.Millisecond)
	}
	res, err := c.s.h.Auto(c.ctx, req)
	if err != nil {
		return autobuildOut{}, c.hostErr(err)
	}
	if res.Added > 0 { // шага не было — отменять нечего
		c.s.ed.setRec(c.sess, editRec{key: res.Album.Key, rev: res.Album.Rev})
	}
	return autobuildOut{Album: res.Album, Added: res.Added, Photos: res.Photos,
		Plan: planOut{From: res.PlanFrom, To: res.PlanTo, Similar: res.Similar}, First: res.First}, nil
}

// spreadIDs переводит ссылки агента на развороты в ID по текущему виду; неизвестные — отказ.
// Обложка («cover») годится, только если cover true (показать можно, менять раскладку нельзя).
func (c *call) spreadIDs(refs []spreadRef, cover bool) ([]string, error) {
	v, ed := c.s.h.View()
	if ed == 0 {
		return nil, errNoAlbum
	}
	ids := make([]string, 0, len(refs))
	for _, r := range refs {
		sv, n, ok := findSpread(v, string(r))
		if !ok || n == 0 && !cover {
			return nil, errNoSpread
		}
		ids = append(ids, sv.ID)
	}
	return dedupe(ids), nil
}

func (c *call) varyLayouts(in varyIn) (varyOut, error) {
	ids, err := c.spreadIDs(in.Spreads, false)
	if err != nil {
		return varyOut{}, err
	}
	req := VaryReq{Key: in.Album.Key, Rev: in.Album.Rev, Client: askClient(c.client()), Spreads: ids, Variety: max(in.Variety, 1), Families: dedupe(in.Families)}
	ref, changed, err := c.s.h.Vary(c.ctx, req)
	if err != nil {
		return varyOut{}, c.hostErr(err)
	}
	if changed > 0 {
		c.s.ed.setRec(c.sess, editRec{key: ref.Key, rev: ref.Rev})
	}
	return varyOut{Album: ref, Changed: changed}, nil
}

func (c *call) showSpread(in showIn) (showOut, error) {
	ids, err := c.spreadIDs([]spreadRef{in.Spread}, true)
	if err != nil {
		return showOut{}, err
	}
	n, err := c.s.h.Show(in.Album.Key, ids[0])
	if err != nil {
		return showOut{}, c.hostErr(err)
	}
	return showOut{Spread: n}, nil
}

func (c *call) undoLast(in undoIn) (undoOut, error) {
	rec, ok := c.s.ed.getRec(c.sess)
	switch {
	case ok && rec.created && rec.key == "":
		return undoOut{}, errCreateNoUndo
	case !ok || rec.key != in.Album.Key || rec.rev != in.Album.Rev:
		return undoOut{}, errUndoNotOwn
	}
	if v, _ := c.s.h.View(); v.Rev != rec.rev { // после правки агента альбом менялся
		c.s.ed.dropRec(c.sess)
		return undoOut{}, errUndoNotOwn
	}
	ref, err := c.s.h.Undo(c.ctx, in.Album)
	switch {
	case errors.Is(err, errUndoNotOwn), errors.Is(err, errAlbumEdited):
		c.s.ed.dropRec(c.sess)
		return undoOut{}, errUndoNotOwn
	case err != nil:
		return undoOut{}, c.hostErr(err)
	}
	c.s.ed.dropRec(c.sess)
	return undoOut{Album: ref}, nil
}
