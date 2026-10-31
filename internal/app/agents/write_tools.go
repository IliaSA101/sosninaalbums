package agents

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/google/jsonschema-go/jsonschema"

	"github.com/IliaSA101/sosninaalbums/internal/paths"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

// Инструменты «только с вопросом» (152, класс Q): выгрузка и открытие альбома по просьбе агента. Путь агент
// не передаёт и не получает: у выгрузки папку показывает и подтверждает человек, у открытия — ключ из
// list_albums или подсказка имени среди недавних. Вопрос, окно ОС и действие — у фасада (Host.Ask, виды
// AskExport и AskOpen).

// maxNameHint — длина name_hint в знаках.
const maxNameHint = 60

// minStem — самая короткая основа слова подсказки: «Орлов» не должен стать «орл» (= «Орленко»).
const minStem = 5

func registerWriteTools(e *toolEnv) {
	regEdit(e, toolSpec{name: "export_album", perm: prefs.ClassAskOnly, title: "Выгрузить в JPEG", edit: true, ask: true, danger: true,
		desc: "Выгрузить открытый альбом в JPEG, как окно «Файл ▸ Экспорт…»: range (как в окне: «3-5, 8», «л», «п»; пусто — все), pages, " +
			"cover, cover_only, quality (100, 95 или 80), dpi (72…1200; пусто — dpi альбома), print (id из list_print_profiles для " +
			"типографии, «plain» — обычная выгрузка; пусто — как открывается окно). folder_hint — только имя папки («Петровы») или " +
			"подсказка пути: программа сама покажет человеку полный путь и спросит его в своём окне; путь агент не получает. " +
			"Программа всегда спрашивает (отдельного «Всегда» нет), перезапись прежних файлов человек видит в вопросе. Ответ: " +
			"выгрузка начата и сколько файлов; ход — get_album, поле export. Выгрузка не отменяется через undo_last_edit." + askNote},
		exportProps, func(in exportIn) *AlbumRef { return &in.Album }, true,
		func(c *call, in exportIn) (askOut, error) { return c.exportAlbum(in) })

	regEditPre(e, toolSpec{name: "open_album", perm: prefs.ClassAskOnly, title: "Открыть альбом", edit: true, ask: true, danger: true,
		desc: "Открыть сохранённый альбом из последних по просьбе человека: ключ album из list_albums или name_hint — как человек его " +
			"назвал («Ивановы»); можно без обоих. Путь агент не передаёт и не получает. Программа всегда спрашивает человека в своём окне " +
			"(показывает найденный альбом или открывает выбор файла) и сама не сообщает, нашла ли подсказку; имя сверяется буква в " +
			"букву — не нашёлся, возьмите ключ из list_albums. Уже открытый альбом по ключу album возвращается без вопроса " +
			"(opened: false); по name_hint вопрос задаётся всегда. Открытый сейчас альбом сохранится, прежний вернёт кнопка «Открыть прежний» в тосте. " +
			"Открытие не отменяется через undo_last_edit." + askNote},
		openProps, func(openIn) *AlbumRef { return nil }, false, (*call).openPre,
		func(c *call, in openIn) (askOut, error) { return c.openAlbum(in) })
}

type exportIn struct {
	Album     AlbumRef `json:"album"`
	Range     string   `json:"range,omitempty" jsonschema:"диапазон как в окне экспорта: «3-5, 8», «л» (левая), «п» (правая); пусто — все развороты"`
	Pages     bool     `json:"pages,omitempty" jsonschema:"по страницам: левая и правая отдельными файлами"`
	Cover     bool     `json:"cover,omitempty" jsonschema:"выгрузить и обложку"`
	CoverOnly bool     `json:"cover_only,omitempty" jsonschema:"только обложка"`
	Quality   int      `json:"quality,omitempty" jsonschema:"качество JPEG: 100, 95 или 80 (по умолчанию 95)"`
	DPI       int      `json:"dpi,omitempty" jsonschema:"разрешение 72…1200 (по умолчанию dpi альбома)"`
	Print     string   `json:"print,omitempty" jsonschema:"id типографии из list_print_profiles или plain (обычная выгрузка); пусто — как открывается окно"`
	// FolderHint — имя папки выгрузки или подсказка пути: программа не берёт его как есть.
	FolderHint string `json:"folder_hint,omitempty" jsonschema:"имя папки выгрузки («Петровы») или подсказка, где её искать (путь не используется как есть)"`
}

// maxRange — длина диапазона окна экспорта в знаках.
const maxRange = 256

func exportProps(p map[string]*jsonschema.Schema) {
	p["range"].MaxLength = ptr(maxRange)
	p["quality"].Enum = []any{100, 95, 80}
	p["dpi"].Minimum, p["dpi"].Maximum = ptr(72.0), ptr(1200.0)
	p["print"].MaxLength = ptr(maxID)
	p["folder_hint"].MaxLength = ptr(maxHint)
}

// exportPlain — print: обычная выгрузка явно (агент сказал «просто в JPEG»), без выбора окна по умолчанию.
const exportPlain = "plain"

// printKind — print из входа → вид типографии Host: "" и "plain" как есть; иначе настоящий id профиля с выгрузкой
// (на уровне 1 чужой профиль виден только маской own-…, как у apply_print).
func (c *call) printKind(id string) (string, error) {
	id = strings.TrimSpace(id)
	if id == "" || id == exportPlain {
		return id, nil
	}
	hidden := c.hiddenPrints()
	for _, p := range c.s.h.Prints() {
		_, hide := hidden[p.ID]
		hide = hide && !c.names()
		if p.HasExport && (hide && id == ownPrintID(p.ID) || !hide && id == p.ID) {
			return p.ID, nil
		}
	}
	return "", errNoProfile
}

func (c *call) exportAlbum(in exportIn) (askOut, error) {
	h, err := hint(in.FolderHint)
	if err != nil {
		return askOut{}, err
	}
	if utf8.RuneCountInString(in.Range) > maxRange || !utf8.ValidString(in.Range) || paths.BadRunes(in.Range) {
		return askOut{}, errBadExport
	}
	switch in.Quality {
	case 0, 100, 95, 80:
	default:
		return askOut{}, errBadExport
	}
	if in.DPI != 0 && (in.DPI < 72 || in.DPI > 1200) {
		return askOut{}, errBadExport
	}
	kind, err := c.printKind(in.Print)
	if err != nil {
		return askOut{}, err
	}
	return c.ask(AskReq{Kind: AskExport, Client: askClient(c.client()), Hint: h, Key: in.Album.Key, Rev: in.Album.Rev,
		Range: strings.TrimSpace(in.Range), Pages: in.Pages, Cover: in.Cover, CoverOnly: in.CoverOnly,
		Quality: in.Quality, DPI: in.DPI, Print: kind})
}

type openIn struct {
	Album    string `json:"album,omitempty" jsonschema:"ключ альбома из list_albums (только из последних)"`
	NameHint string `json:"name_hint,omitempty" jsonschema:"как человек назвал альбом, до 60 знаков: «Ивановы», «свадьба Петровых»; вместо album"`
}

func openProps(p map[string]*jsonschema.Schema) {
	p["album"].MinLength, p["album"].MaxLength = ptr(1), ptr(maxID)
	p["name_hint"].MaxLength = ptr(maxNameHint)
}

// openPre — разбор open_album до места правки и «идёт выгрузка» (0.35-152-26): ошибки аргументов и ответ «уже открыт»
// не зависят от выгрузки. done — ответ готов без вопроса.
func (c *call) openPre(in openIn) (out askOut, done bool, err error) {
	out, _, done, err = c.openResolve(in)
	return out, done, err
}

func (c *call) openAlbum(in openIn) (askOut, error) {
	out, req, done, err := c.openResolve(in)
	if err != nil || done {
		return out, err
	}
	return c.ask(req)
}

// openResolve — аргументы open_album: либо ошибка, либо готовый короткий ответ (done), либо вопрос req.
func (c *call) openResolve(in openIn) (out askOut, req AskReq, done bool, err error) {
	if in.Album != "" && in.NameHint != "" {
		return askOut{}, AskReq{}, false, badArgs("album и name_hint вместе не передают: нужен один из них или ни одного")
	}
	if utf8.RuneCountInString(in.NameHint) > maxNameHint || !utf8.ValidString(in.NameHint) || paths.BadRunes(in.NameHint) {
		return askOut{}, AskReq{}, false, badArgs("name_hint длиннее 60 знаков или с недопустимыми знаками")
	}
	req = AskReq{Kind: AskOpen, Client: askClient(c.client())}
	var pick *prefs.RecentProject
	recent := c.s.h.Recent()
	switch {
	case in.Album != "":
		if pick = recentByKey(recent, in.Album); pick == nil {
			return askOut{}, AskReq{}, false, errNoRecent
		}
		if !pick.Exists {
			return askOut{}, AskReq{}, false, errOpenGone
		}
	case in.NameHint != "":
		// Итог поиска агенту не виден: нашлось — вопрос с альбомом, нет — вопрос с выбором файла.
		if pick = recentByName(recent, in.NameHint); pick != nil {
			req.Hint = in.NameHint
		}
	}
	if pick != nil {
		// Короткий ответ без вопроса — только по ключу: по name_hint он выдал бы агенту итог поиска (0.35-152-25).
		if v, ed := c.s.h.View(); in.Album != "" && ed != 0 && v.Path != "" && paths.SamePath(v.Path, pick.Path) {
			opened := false
			return askOut{Album: &AlbumRef{Key: albumKey(ed), Rev: v.Rev}, Opened: &opened}, req, true, nil
		}
		req.Key = RecentKey(pick.Path)
	}
	return askOut{}, req, false, nil
}

// recentByKey — запись «Последних» с этим ключом (тем, что отдаёт list_albums); nil — нет.
func recentByKey(recent []prefs.RecentProject, key string) *prefs.RecentProject {
	for i := range recent {
		if RecentKey(recent[i].Path) == key {
			return &recent[i]
		}
	}
	return nil
}

// recentByName — единственный существующий альбом из последних, имя которого подходит под подсказку; nil —
// ни одного или несколько. Слово подсказки сначала теряет окончание (основа не короче minStem знаков), затем
// должно быть началом какого-то слова имени: «Ивановых» находит «Свадьба Ивановы».
func recentByName(recent []prefs.RecentProject, hint string) *prefs.RecentProject {
	stems := hintStems(hint)
	if len(stems) == 0 {
		return nil
	}
	var found *prefs.RecentProject
	for i := range recent {
		if !recent[i].Exists || !nameHas(nameWords(recentName(recent[i])), stems) {
			continue
		}
		if found != nil {
			return nil
		}
		found = &recent[i]
	}
	return found
}

// composeRU склеивает разложенные «й» и «ё» (буква + знак): имена из macOS и копипаста бывают в NFD.
// Полного NFC здесь нет — golang.org/x/text службе агентов не разрешён, а для русских имён хватает двух пар.
var composeRU = strings.NewReplacer("и\u0306", "й", "е\u0308", "е", "ё", "е")

// nameWords — слова имени для сравнения: нижний регистр, «й»/«ё» цельные, «ё» как «е», знаки препинания — пробел.
func nameWords(s string) []string {
	s = composeRU.Replace(strings.ToLower(s))
	return strings.FieldsFunc(s, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
}

// hintStems — основы слов подсказки: срезается min(2, длина − minStem) последних знаков (не меньше нуля).
func hintStems(hint string) []string {
	words := nameWords(hint)
	for i, w := range words {
		r := []rune(w)
		words[i] = string(r[:len(r)-max(0, min(2, len(r)-minStem))])
	}
	return words
}

// nameHas — каждая основа подсказки начинает какое-то слово имени.
func nameHas(words, stems []string) bool {
	for _, st := range stems {
		ok := false
		for _, w := range words {
			if strings.HasPrefix(w, st) {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	return true
}
