package agents

import (
	"slices"
	"strconv"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Тела правящих инструментов 0.34 (151, design §1.2–1.6): служба сверяет вход, переводит ссылки агента
// (номера листов, маски own-<хэш>, id каталога) в настоящие и зовёт замыкание Host; сама альбом не правит.
// Правка с шагом (changed) запоминается для undo_last_edit, холостая — нет.

// remember записывает правку для undo_last_edit; холостая (changed false) не записывается.
func (c *call) remember(ref AlbumRef, changed bool) {
	if changed {
		c.s.ed.setRec(c.sess, editRec{key: ref.Key, rev: ref.Rev})
	}
}

func (c *call) setMarks(in marksIn) (marksOut, error) {
	if in.Rating == nil && in.Flag == "" && in.Label == "" {
		return marksOut{}, badArgs("нужно хоть одно из полей rating, flag, label")
	}
	if in.Rating != nil && (*in.Rating < 0 || *in.Rating > 5) ||
		in.Flag != "" && !slices.Contains(flagKeys, in.Flag) || in.Label != "" && !slices.Contains(labelKeys, in.Label) {
		return marksOut{}, badArgs("rating 0…5, flag pick | reject | none, label red | yellow | green | blue | purple | none")
	}
	// Копия с обложки («id-2») — то же фото ленты, а не чужой id (skipped только для настоящих чужих).
	// «id-new» (запрос копии интерфейса) ленте не принадлежит: отметка под таким ключом после
	// переоткрытия легла бы на настоящее фото, поэтому такой id — в skipped, до Host не доходит.
	ids := make([]string, 0, len(in.IDs))
	var copyReq []string
	for _, id := range in.IDs {
		if id = model.BaseID(strings.TrimSpace(id)); id == "" {
			continue
		} else if model.IsCopyReq(id) {
			copyReq = append(copyReq, id)
		} else {
			ids = append(ids, id)
		}
	}
	ids, copyReq = dedupe(ids), dedupe(copyReq)
	if len(ids)+len(copyReq) == 0 || len(ids)+len(copyReq) > maxMarkIDs {
		return marksOut{}, badArgs("ids: от 1 до " + strconv.Itoa(maxMarkIDs) + " id фото")
	}
	req := MarkReq{Key: in.Album.Key, Rev: in.Album.Rev, Client: askClient(c.client()), IDs: ids, Flag: in.Flag, Label: in.Label}
	if in.Rating != nil {
		req.SetRating, req.Rating = true, *in.Rating
	}
	res, err := c.s.h.Mark(c.ctx, req)
	if err != nil {
		return marksOut{}, c.hostErr(err)
	}
	c.remember(res.Album, res.Changed > 0)
	out := marksOut{Album: res.Album, Changed: res.Changed}
	for _, id := range append(res.Skipped, copyReq...) {
		out.Skipped = append(out.Skipped, cleanID(id))
	}
	return out, nil
}

// Допустимые слова flag и label (перечисления схемы).
var (
	flagKeys  = []string{"pick", "reject", "none"}
	labelKeys = []string{"red", "yellow", "green", "blue", "purple", "none"}
)

func (c *call) setCoverPhoto(in setCoverIn) (setCoverOut, error) {
	pc, err := c.photoCtx()
	if err != nil {
		return setCoverOut{}, err
	}
	id := model.BaseID(strings.TrimSpace(in.Photo))
	if _, ok := pc.byID[id]; !ok {
		return setCoverOut{}, errNoPhoto
	}
	if pc.status(id) != "ok" {
		return setCoverOut{}, errPhotoGone
	}
	res, err := c.s.h.Cover(c.ctx, CoverReq{Key: in.Album.Key, Rev: in.Album.Rev, Client: askClient(c.client()), Photo: id})
	if err != nil {
		return setCoverOut{}, c.hostErr(err)
	}
	c.remember(res.Album, res.Changed)
	out := setCoverOut{Album: res.Album, Created: res.Created, Place: cleanID(res.Place), Changed: res.Changed}
	if res.Product != "" {
		out.Product = c.coverPreset(res.Product)
	}
	return out, nil
}

// decorID сверяет id с каталогом вида kind: нескачанное («Можно скачать») и чужое — errNoDecor.
func (c *call) decorID(kind, id string) (string, error) {
	id = strings.TrimSpace(id)
	dc, err := c.catalog(kind)
	if err != nil {
		return "", err
	}
	if _, ok := dc.byID[id]; !ok {
		return "", errNoDecor
	}
	return id, nil
}

// bgArgs — проверки входа set_background, общие для pre (до вопроса человеку) и тела: to, pages, scope, replace,
// spreads и «cover» в листах. Каталога и вида не касается: item и номера сверяет тело.
func bgArgs(in bgIn) error {
	switch in.To {
	case "", "spreads":
		if in.Pages != nil {
			return badArgs("pages передают только с to: album")
		}
		if len(in.Spreads) == 0 {
			return badArgs("spreads: назовите хотя бы один разворот")
		}
		if !slices.Contains([]string{"", "spread", "left", "right"}, in.Scope) {
			return badArgs("scope: spread, left или right")
		}
	case "album":
		switch {
		case in.Scope != "":
			return badArgs("с to: album scope не передают")
		case in.Replace && len(in.Spreads) == 0:
			return badArgs("с to: album и replace назовите в spreads номера из skipped, на которые согласился человек")
		case !in.Replace && len(in.Spreads) > 0:
			return badArgs("с to: album spreads передают только с replace: номера из skipped, на которые согласился человек")
		}
	default:
		return badArgs("to: spreads или album")
	}
	if slices.ContainsFunc(in.Spreads, func(r spreadRef) bool { return isCoverRef(string(r)) }) {
		return errCoverNotTarget("set_background")
	}
	return nil
}

// bgPre — разбор set_background до вопроса и места правки: человеку не показывают вопрос, на который агент
// всё равно получит «неверные аргументы».
func bgPre(_ *call, in bgIn) (bgOut, bool, error) {
	return bgOut{}, false, bgArgs(in)
}

func (c *call) setBackground(in bgIn) (bgOut, error) {
	item, err := c.decorID("bg", in.Item)
	if err != nil {
		return bgOut{}, err
	}
	if err := bgArgs(in); err != nil { // и тот же разбор в pre; здесь — для процесса от администратора и прямого вызова
		return bgOut{}, err
	}
	if in.To == "album" {
		return c.setAlbumBackground(item, in)
	}
	ids, err := c.spreadIDs(in.Spreads, false)
	if err != nil {
		return bgOut{}, err
	}
	scope := in.Scope
	if scope == "" {
		scope = "spread"
	}
	v, _ := c.s.h.View()
	res, err := c.s.h.Bg(c.ctx, BgReq{Key: in.Album.Key, Rev: in.Album.Rev, Client: askClient(c.client()), Item: item,
		Spreads: ids, Scope: scope, Replace: in.Replace})
	if err != nil {
		return bgOut{}, c.hostErr(err)
	}
	c.remember(res.Album, res.Changed > 0)
	out := bgOut{Album: res.Album, Changed: res.Changed, Replaced: res.Replaced, Skipped: spreadNums(v, res.Skipped)}
	return out, nil
}

// spreadNums — ID листов → номера с 1 по виду v; чужие ID пропускаются.
func spreadNums(v model.View, ids []string) (nums []int) {
	for _, id := range ids {
		if i := slices.IndexFunc(v.Spreads, func(s model.SpreadView) bool { return s.ID == id }); i >= 0 {
			nums = append(nums, i+1)
		}
	}
	return nums
}

// setAlbumBackground — set_background с to: album (154, design §1.1): фон альбома. Проверки входа (bgArgs)
// уже сделаны. spreads здесь — листы согласия на снятие фото клиента (только с replace).
func (c *call) setAlbumBackground(item string, in bgIn) (bgOut, error) {
	consent, err := c.spreadIDs(in.Spreads, false)
	if err != nil {
		return bgOut{}, err
	}
	v, _ := c.s.h.View() // вид до правки: порядок листов фон не меняет, ревизию уже сверил gate
	pages := v.AlbumBg != nil && v.AlbumBg.Pages
	if in.Pages != nil {
		pages = *in.Pages
	}
	res, err := c.s.h.Bg(c.ctx, BgReq{Key: in.Album.Key, Rev: in.Album.Rev, Client: askClient(c.client()), Item: item,
		Spreads: consent, Replace: in.Replace, Album: true, Pages: pages})
	if err != nil {
		return bgOut{}, c.hostErr(err)
	}
	c.remember(res.Album, res.Album.Rev != in.Album.Rev) // фон альбома — шаг и без разворотов
	return bgOut{Album: res.Album, Changed: res.Changed, Replaced: res.Replaced, AlbumBg: res.AlbumBg,
		Skipped: spreadNums(v, res.Skipped), Cleared: spreadNums(v, res.Cleared), Hidden: spreadNums(v, res.Hidden)}, nil
}

// coverPre — разбор add_sticker до вопроса и места правки: «cover» без обложки (или у альбома-листа) — errNoCover
// сразу, а не «нет такого разворота» из spreadIDs.
func coverPre(c *call, in addStickerIn) (addStickerOut, bool, error) {
	return addStickerOut{}, false, c.coverCheck(stickerRefs(in))
}

// stickerRefs — ссылки на развороты из стикеров входа.
func stickerRefs(in addStickerIn) []string {
	refs := make([]string, len(in.Stickers))
	for i, s := range in.Stickers {
		refs[i] = string(s.Spread)
	}
	return refs
}

// coverCheck — общая проверка обложки (154, design §2): среди ссылок есть «cover», а обложки нет
// или альбом — лист, значит errNoCover. Альбома нет — не её дело (скажет gate).
func (c *call) coverCheck(refs []string) error {
	if !slices.ContainsFunc(refs, isCoverRef) {
		return nil
	}
	v, ed := c.s.h.View()
	if ed != 0 && (v.Cover == nil || v.Settings.Sheet) {
		return errNoCover
	}
	return nil
}

func (c *call) addSticker(in addStickerIn) (addStickerOut, error) {
	if len(in.Stickers) == 0 || len(in.Stickers) > MaxStickers {
		return addStickerOut{}, badArgs("stickers: от 1 до " + strconv.Itoa(MaxStickers) + " стикеров")
	}
	v, ed := c.s.h.View()
	if ed == 0 {
		return addStickerOut{}, errNoAlbum
	}
	if err := c.coverCheck(stickerRefs(in)); err != nil { // обложку могли убрать, пока правка ждала ответа
		return addStickerOut{}, err
	}
	req := StickerReq{Key: in.Album.Key, Rev: in.Album.Rev, Client: askClient(c.client())}
	for _, s := range in.Stickers {
		sv, _, ok := findSpread(v, string(s.Spread))
		if !ok {
			return addStickerOut{}, errNoSpreadFlat
		}
		item, err := c.decorID("sticker", s.Item)
		if err != nil {
			return addStickerOut{}, err
		}
		page, place, size := orWord(s.Page, "right"), orWord(s.Place, "center"), orWord(s.Size, "m")
		if !slices.Contains(pageKeys, page) || !slices.Contains(placeKeys, place) || !slices.Contains(sizeKeys, size) {
			return addStickerOut{}, badArgs("page left | right, place center | top-left | top-right | bottom-left | bottom-right, size s | m | l")
		}
		req.Spreads, req.Pages, req.Items = append(req.Spreads, sv.ID), append(req.Pages, page), append(req.Items, item)
		req.Places, req.Sizes = append(req.Places, place), append(req.Sizes, size)
	}
	res, err := c.s.h.Stickers(c.ctx, req)
	if err != nil {
		return addStickerOut{}, c.hostErr(err)
	}
	c.remember(res.Album, res.Added > 0)
	out := addStickerOut{Album: res.Album, Added: res.Added}
	for _, id := range res.IDs {
		out.IDs = append(out.IDs, cleanID(id))
	}
	return out, nil
}

// Допустимые слова стикера.
var (
	pageKeys  = []string{"left", "right"}
	placeKeys = []string{"center", "top-left", "top-right", "bottom-left", "bottom-right"}
	sizeKeys  = []string{"s", "m", "l"}
)

func orWord(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

// formatKey — имя формата для сравнения: без регистра и пробелов, x X х Х (латиница и кириллица) = ×.
func formatKey(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch r {
		case ' ', '\t', ' ':
		case 'x', 'х':
			b.WriteRune('×')
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// printTarget находит типографию и формат по словам агента. На уровне 1 не встроенные профили видны
// только маской own-<хэш>, их форматы — «Формат <хэш>»; настоящие id и имена там не принимаются (как оракул
// по скрытым именам). Пустой формат — первый.
func (c *call) printTarget(profile, format string) (id, name string, err error) {
	hidden := c.hiddenPrints()
	profile = strings.TrimSpace(profile)
	for _, p := range c.s.h.Prints() {
		_, hide := hidden[p.ID]
		hide = hide && !c.names()
		if hide && profile != ownPrintID(p.ID) || !hide && profile != p.ID {
			continue
		}
		if len(p.Formats) == 0 {
			return "", "", errNoProfile
		}
		if strings.TrimSpace(format) == "" {
			return p.ID, p.Formats[0].Name, nil
		}
		want := formatKey(format)
		for _, f := range p.Formats {
			label := f.Name
			if hide {
				label = ownFormatName(p.ID, f.Name)
			}
			if formatKey(label) == want {
				return p.ID, f.Name, nil
			}
		}
		return "", "", errNoProfile
	}
	return "", "", errNoProfile
}

func (c *call) applyPrint(in applyPrintIn) (applyPrintOut, error) {
	id, name, err := c.printTarget(in.Profile, in.Format)
	if err != nil {
		return applyPrintOut{}, err
	}
	res, err := c.s.h.Print(c.ctx, PrintReq{Key: in.Album.Key, Rev: in.Album.Rev, Client: askClient(c.client()), Profile: id, Format: name})
	if err != nil {
		return applyPrintOut{}, c.hostErr(err)
	}
	c.remember(res.Album, res.Changed)
	out := applyPrintOut{Album: res.Album, Page: applyPrintPage{W: res.PageW, H: res.PageH}, CoverChanged: res.CoverChanged,
		Changed: res.Changed, OverLimit: res.OverLimit}
	if res.Cover != "" {
		out.Cover = c.coverPreset(res.Cover)
	}
	return out, nil
}
