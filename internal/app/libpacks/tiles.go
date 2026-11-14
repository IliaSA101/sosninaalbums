package libpacks

import (
	"slices"
	"strconv"

	"github.com/IliaSA101/sosninaalbums/internal/decor"
	"github.com/IliaSA101/sosninaalbums/internal/decor/fetch"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

// Плитки «⤓ набор, размер» в галереях (спека 144, design §6): чистая логика над сохранённым списком,
// без сети; лист читается только по Lstat (разбор — при показе, в ServePreview).

// Tile — плитка образца нескачанного набора. В App — type DecorGet libpacks.Tile: тип над
// libpacks.Tile, поля общие.
type Tile struct {
	ID    string   `json:"id"`    // "get:<packID>:<src>" — ключ плитки
	Pack  string   `json:"pack"`  // id набора для DownloadLibraryPack
	Src   string   `json:"src"`   // 16 hex: предмет набора для PackItem
	Title string   `json:"title"` // название набора (cleanText; в UI только текстом)
	URL   string   `json:"url"`   // /media/lp/<pack>/<kind>/<i>?v=<sha[:8]>
	W     int      `json:"w"`     // px клетки: пропорция картинки
	H     int      `json:"h"`
	Bytes int64    `json:"bytes"` // всё, что поставит «Скачать» (closure)
	With  []string `json:"with"`  // остальные наборы closure; не nil

	// Состав нажатого набора — для вопроса «Скачать набор?».
	Bg      int `json:"bg"`
	Sticker int `json:"sticker"`
	Spread  int `json:"spread"`
	Frame   int `json:"frame"`
}

// tileLimit — сколько плиток одного вида даёт один набор (design §6: до 8 фонов, 12 стикеров,
// 6 рамочек, 4 разворотов, 1 макет).
var tileLimit = map[string]int{decor.KindBg: 8, decor.KindSticker: 12, decor.KindFrame: 6, decor.KindSpread: 4, decor.KindAlbum: 1}

// Tiles — плитки вида kind для строки «Можно скачать»: образцы наборов, которых нет в библиотеке,
// в порядке каталога, внутри — в порядке клеток. Пусто без согласия, без папки образцов, без
// списка, при библиотеке «только чтение». Набор даёт плитки, если он (и его состав «Скачать») не
// серый и не «удалён вами», а его лист на диске есть и не отвергнут разбором.
func (s *Service) Tiles(kind string) []Tile {
	out := []Tile{}
	limit := tileLimit[kind]
	if limit == 0 || s.previewDir() == "" {
		return out
	}
	net := prefs.LibraryNetOf(s.h.CfgDir())
	cat, _, _ := s.cached()
	st, err := s.h.Store()
	if !net.Allowed || cat == nil || err != nil || st.ReadOnly() {
		return out
	}
	have := haveOf(st)
	ix := indexOf(*cat)
	for _, e := range previewWanted(*cat, have, net.Skip, s.h.Version()) { // общий каталог: только чтение
		p := e.Preview
		ri := slices.IndexFunc(p.Rows, func(r fetch.PreviewRow) bool { return r.Kind == kind })
		if ri < 0 || !s.sheetPresent(p) || s.sheetBad(p) {
			continue
		}
		row := p.Rows[ri]
		t := Tile{Pack: e.ID, Title: e.Title, W: row.W, H: row.H, With: []string{},
			Bg: e.Counts.Bg, Sticker: e.Counts.Sticker, Spread: e.Counts.Spread, Frame: e.Counts.Frame}
		for _, m := range ix.closure(*cat, e.ID, have, net.Skip, s.h.Version()).ents {
			t.Bytes += m.Size
			if m.ID != e.ID {
				t.With = append(t.With, m.Title)
			}
		}
		for i, src := range row.Items[:min(limit, len(row.Items))] {
			c := t
			c.Src, c.ID = src, "get:"+e.ID+":"+src
			c.URL = "/media/lp/" + e.ID + "/" + kind + "/" + strconv.Itoa(i) + "?v=" + p.SHA256[:8]
			out = append(out, c)
		}
	}
	return out
}

// Invite — список наборов ни разу не получен и библиотека не «только чтение»: в конце «Все» — плитка
// «⤓ Ещё из интернета». Согласие не важно (щелчок открывает окно «Наборы», там вопрос согласия); при
// снятом согласии и сохранённом списке приглашения нет — программа в сеть не ходит.
func (s *Service) Invite() bool {
	cat, _, bad := s.cached()
	if cat != nil || bad {
		return false
	}
	st, err := s.h.Store()
	return err == nil && !st.ReadOnly()
}

// StartPreviews — докачка листов фоном для окна: без согласия не запускается вовсе (ни горутины,
// ни запроса), App закрывается — тоже. Ответ сразу; итог — событие library:previews.
func (s *Service) StartPreviews() {
	if !prefs.LibraryNetOf(s.h.CfgDir()).Allowed {
		return
	}
	s.h.GoBG(func() { s.FetchPreviews() })
}
