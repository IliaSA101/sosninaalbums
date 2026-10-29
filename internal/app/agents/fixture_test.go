package agents

import (
	"context"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/model/printprof"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

// fixture — подставной Host: вид, фото, отметки и библиотека лежат в полях, тесты их меняют.
// Каждая строка, которую ввёл бы человек или которая взята из имени файла, несёт метку ZQ<n>:
// по ней TestLevelGates видит, что строка вышла наружу (design §3).
type fixture struct {
	mu      sync.Mutex
	view    model.View
	ed      uint64
	photos  []library.Photo
	marks   map[string]model.MarkView
	missing []string
	unread  map[string]string
	decor   map[string]DecorCatalogLite
	packs   PacksLite
	prints  []printprof.Info
	recent  []prefs.RecentProject
	fonts   []string
	// decorErr, packsErr — ошибки Host (с путями): служба не должна их показывать.
	decorErr, packsErr error
	// hook вызывается в View: тесты вешают сюда задержку или панику.
	hook func()
	// preview, render — ответы Host.Preview/Render; nil — настоящий JPEG нужного размера (см. preview_test.go).
	preview func(ctx context.Context, id string, px int) ([]byte, error)
	render  func(ctx context.Context, id string, px int) ([]byte, error)
	// edit — замыкания правок (edit_fixture_test.go); noEdit — Host без них (инструментов правки нет).
	edit   *editFake
	noEdit bool
	// reads — сколько раз служба читала данные: проверка, что инструмент вообще ходил в Host.
	reads atomic.Int32
}

// Метки: ZQ1 имя файла фото, ZQ2 папка, ZQ3 метка XMP, ZQ4 имя второго фото, ZQ5 имя альбома,
// ZQ6 имя разворота, ZQ7 текст, ZQ8 подпись, ZQ9 подборка, ZQ10 макет, ZQ11 шрифт, ZQ12 свой стикер,
// ZQ13 фон из набора-файла, ZQ15 набор-файл, ZQ16 автор набора, ZQ17 свой профиль, ZQ18 его формат,
// ZQ19 «Последний», ZQ20 имя фото на фоне, ZQ21 свой фон, ZQ22 свой макет в каталоге.
// ZQ24 профиль типографии из набора, ZQ25 свой шаблон (Own не выставлен), ZQ26 набор неизвестного источника.
// ZQ14 — «Можно скачать», ZQ23 — таблица имён: не выходят ни на каком уровне.
var levelMarks = []string{"ZQ1", "ZQ2", "ZQ3", "ZQ4", "ZQ5", "ZQ6", "ZQ7", "ZQ8", "ZQ9", "ZQ10", "ZQ11", "ZQ12", "ZQ13",
	"ZQ15", "ZQ16", "ZQ17", "ZQ18", "ZQ19", "ZQ20", "ZQ21", "ZQ22", "ZQ24", "ZQ25", "ZQ26"}

// namesMark — метка таблицы имён (подпись-шаблон, заглушка, поля шапки): не выходит на всех уровнях.
const namesMark = "ZQ23"

const (
	idA = "aaaa000000000001" // стоит на развороте 1
	idB = "aaaa000000000002" // не использовано
	idC = "aaaa000000000003" // файла нет
	idD = "aaaa000000000004" // не читается
	idE = "aaaa000000000005" // фото на фоне
)

func rc(x, y, w, h float64) layout.Rect { return layout.Rect{X: x, Y: y, W: w, H: h} }

func newFixture() *fixture {
	root := filepath.Join(string(filepath.Separator), "albums")
	dir := filepath.Join(root, "ZQ2dir")
	f := &fixture{ed: 7, marks: map[string]model.MarkView{}, unread: map[string]string{idD: "битый"}, missing: []string{idC}}
	f.photos = []library.Photo{
		{ID: idA, Path: filepath.Join(dir, "ZQ1_Ivanov.jpg"), Name: "ZQ1_Ivanov.jpg", W: 6000, H: 4000, Shot: 1_700_000_000, Size: 5 << 20,
			Color: "srgb", Profile: "ZQ3 profile", Xmp: &library.Marks{Rating: 3, Label: "other", LabelText: "ZQ3"}},
		{ID: idB, Path: filepath.Join(dir, "ZQ4_Petrova.jpg"), Name: "ZQ4_Petrova.jpg", W: 4000, H: 6000, Shot: 1_700_000_100, Size: 4 << 20},
		{ID: idC, Path: filepath.Join(dir, "gone.jpg"), Name: "gone.jpg", W: 100, H: 100},
		{ID: idD, Path: filepath.Join(dir, "bad.jpg"), Name: "bad.jpg", W: 100, H: 100},
		{ID: idE, Path: filepath.Join(root, "elsewhere", "ZQ20_bg.jpg"), Name: "ZQ20_bg.jpg", W: 3000, H: 2000},
	}
	rating := 5
	f.marks[idA] = model.MarkView{Rating: &rating, Bins: []string{"b1"}}
	sp1 := model.SpreadView{
		ID: "sp1", Name: "ZQ6 разворот", W: 600, H: 300, Mid: 300, Photos: []string{idA}, Rects: []layout.Rect{rc(10, 10, 200, 150)},
		Frames: []model.FrameView{{DPI: 250}}, Layout: "r2", Grid: true, LowDPI: 1, LookStep: 2,
		Places: []model.PlaceView{{ID: "pl1", X: 10, Y: 10, W: 200, H: 150, Photo: idA, Shape: "", Role: ""}, {ID: "pl2", X: 250, Y: 10, W: 100, H: 100}},
		Texts: []model.TextView{{ID: "tx1", X: 20, Y: 200, W: 100, H: 20, Text: "ZQ7 текст", Style: model.TextStyle{Font: "serif", Size: 14}},
			{ID: "tx2", Text: namesMark + " Петрова", Src: "{Имя}"}},
		Captions: []model.CaptionView{{Photo: idA, TextView: model.TextView{Text: "ZQ8 подпись"}},
			{Photo: idA, TextView: model.TextView{Text: namesMark + " Иванова Маша", Src: "{Фамилия} {Имя}"}}},
		Stubs:     []model.StubView{{Photo: idB, TextView: model.TextView{Text: namesMark + " [Фамилия]"}}},
		Vars:      model.Vars{Title: namesMark + " заголовок"},
		Stickers:  []model.StickerView{{ID: "st1", Lib: "my:zqsticker", X: 5, Y: 5, W: 20, H: 20}, {ID: "st2", Name: "Звезда", X: 40, Y: 5, W: 20, H: 20}},
		Backdrops: []model.BackdropView{{Scope: "spread", Photo: idE}, {Scope: "left", Photo: idE, Lib: "my:zqbg"}},
		Bg:        [2]string{"#ffffff", "#eeeeee"},
	}
	sp2 := model.SpreadView{ID: "sp2", Kind: model.KindVignette, W: 600, H: 300, Mid: 300, Layout: "", Bg: [2]string{"#ffffff", "#ffffff"}}
	cover := &model.CoverView{Sheet: model.SpreadView{ID: model.CoverID, W: 640, H: 320, Photos: []string{idB}}, Preset: "ZQ24prof-album", Layout: "front", Spine: 12, N: 2}
	f.view = model.View{
		Product: model.ProductAlbum, Spreads: []model.SpreadView{sp1, sp2}, Settings: model.DefaultSettings(),
		Folders: []string{dir}, Path: filepath.Join(root, "ZQ5_wedding.salbum"), Rev: 4, Dirty: true, EmptyPlaces: 1, LowDPI: 1, Drafts: 0,
		BackdropUsed: []string{idE}, Bins: []model.Bin{{ID: "b1", Name: "ZQ9 подборка"}},
		Look: &model.LookView{Key: "my:" + "0123456789abcdef", Name: "ZQ10 макет", Found: true}, Cover: cover,
		AlbumFonts: []model.FontUse{{ID: "f1", Name: "ZQ11 шрифт"}}, Unfilled: []string{"Школа"}, Unnamed: 2,
		PageLimit: &model.PageLimitView{Who: "ZQ17 типография", Count: 2, Min: 10, Max: 60, Warn: "мало"},
	}
	f.edit = newEditFake(f)
	f.fonts = []string{"ZQ11 шрифт"}
	f.decor = map[string]DecorCatalogLite{
		"sticker": {
			Themes: []DecorThemeLite{{ID: "basic", Name: "Основные", Count: 2}, {ID: "get", Name: "ZQ14 Можно скачать", Count: 1}},
			Items: []DecorItemLite{{ID: "sa:star", Theme: "basic", Name: "Звезда", W: 100, H: 100},
				{ID: "my:zqsticker", Theme: "basic", Name: "ZQ12 свой стикер", Own: true, W: 50, H: 50}},
			New: []string{"sa:star"},
			Get: []DecorItemLite{{ID: "get:1", Theme: "get", Name: "ZQ14 образец"}}, Invite: true,
		},
		"bg": {
			Themes: []DecorThemeLite{{ID: "basic", Name: "Основные", Count: 3}},
			Items: []DecorItemLite{{ID: "sa:plain", Theme: "basic", Name: "Однотонный", Pack: "Встроенные"},
				{ID: "my:zqbg", Theme: "basic", Name: "ZQ21 свой фон", Own: true},
				{ID: "p:zq", Theme: "basic", Name: "ZQ13 фон набора", PackID: "zq-pack", Pack: "ZQ15 набор"}},
		},
		"look": {Themes: []DecorThemeLite{{ID: "basic", Name: "Основные", Count: 1}},
			Items: []DecorItemLite{{ID: "my:0123456789abcdef", Theme: "basic", Name: "ZQ22 макет", Own: true, Steps: 5, Vig: true, Cover: true}}},
		"frame": {}, "spread": {},
		"template": {Items: []DecorItemLite{{ID: "my:zqtpl", Theme: "basic", Name: "ZQ25 шаблон"}}},
	}
	f.packs = PacksLite{
		Installed: []PackLite{
			{ID: "sa:builtin", Title: "Встроенные", Origin: "builtin", Version: 1, Bg: 3, Bytes: 1000},
			{ID: "zq-pack", Title: "ZQ15 набор", Author: "ZQ16", License: "MIT", Origin: "file", Version: 2, Bg: 1, Bytes: 2000},
			{ID: "local-pack", Title: "ZQ26 локальный", Origin: "local", Version: 1, Bytes: 10},
			{ID: "net-pack", Title: "Сетевой набор", Author: "Автор", License: "CC0", Origin: "net", Version: 3, Sticker: 5, Bytes: 3000},
		},
		Available: []PackAvailLite{{ID: "av1", Title: "Доступный", Version: 1, Bytes: 4000, New: true}},
		AvailAt:   1_700_000_000_000, Allowed: true,
	}
	f.prints = []printprof.Info{
		{ID: "b1", Name: "Типография", Source: "builtin", HasExport: true, Formats: []printprof.InfoFormat{{Name: "30x30", PageW: 300, PageH: 300}}},
		{ID: "ZQ24prof", Name: "ZQ24 профиль набора", Source: "library", Formats: []printprof.InfoFormat{{Name: "ZQ24 формат", PageW: 210, PageH: 297}}},
		{ID: "own1", Name: "ZQ17 свой профиль", Source: "own", Formats: []printprof.InfoFormat{{Name: "ZQ18 формат", PageW: 200, PageH: 200}}},
	}
	f.recent = []prefs.RecentProject{
		{Path: filepath.Join(root, "ZQ5_wedding.salbum"), Name: "ZQ5_wedding", Exists: true, Modified: 1_700_000_000_000},
		{Path: filepath.Join(root, "ZQ19_old.salbum"), Name: "ZQ19 old", Exists: false, Modified: 1_600_000_000_000},
	}
	return f
}

// host собирает Host над fixture; sock и cfg — как у rig.
func (f *fixture) host(cfg, sock string, changed func()) Host {
	h := f.readHost(cfg, sock, changed)
	if !f.noEdit {
		f.edit.install(&h)
	}
	return h
}

func (f *fixture) readHost(cfg, sock string, changed func()) Host {
	read := func() { f.reads.Add(1) }
	return Host{
		CfgDir: func() string { return cfg }, Version: func() string { return "test" }, Sock: func() string { return sock }, Changed: changed,
		View: func() (model.View, uint64) {
			if f.hook != nil {
				f.hook()
			}
			read()
			f.mu.Lock()
			defer f.mu.Unlock()
			return f.view, f.ed
		},
		Photos: func() []library.Photo {
			read()
			f.mu.Lock()
			defer f.mu.Unlock()
			return append([]library.Photo(nil), f.photos...)
		},
		Marks: func() map[string]model.MarkView {
			read()
			f.mu.Lock()
			defer f.mu.Unlock()
			out := map[string]model.MarkView{}
			for k, v := range f.marks {
				out[k] = v
			}
			return out
		},
		Status: func() ([]string, map[string]string) {
			read()
			f.mu.Lock()
			defer f.mu.Unlock()
			u := map[string]string{}
			for k, v := range f.unread {
				u[k] = v
			}
			return append([]string(nil), f.missing...), u
		},
		Decor: func(kind string) (DecorCatalogLite, error) {
			read()
			f.mu.Lock()
			defer f.mu.Unlock()
			return f.decor[kind], f.decorErr
		},
		Packs: func() (PacksLite, error) {
			read()
			f.mu.Lock()
			defer f.mu.Unlock()
			return f.packs, f.packsErr
		},
		Prints: func() []printprof.Info { read(); return f.prints },
		Recent: func() []prefs.RecentProject { read(); return f.recent },
		FontsMissing: func(used []model.FontUse) []string {
			read()
			return f.fonts
		},
		Preview: func(ctx context.Context, id string, px int) ([]byte, error) {
			if f.preview != nil {
				return f.preview(ctx, id, px)
			}
			return testJPEG(px, px*2/3)
		},
		Render: func(ctx context.Context, id string, px int) ([]byte, error) {
			if f.render != nil {
				return f.render(ctx, id, px)
			}
			return testJPEG(px, px/2)
		},
	}
}

// setLevel пишет уровень доступа в config.json теста.
func setLevel(cfg string, level int) {
	prefs.UpdateAgents(cfg, func(a *prefs.Agents) { a.Level = level })
}

// manySpreads — n пустых разворотов.
func manySpreads(n int) []model.SpreadView {
	out := make([]model.SpreadView, n)
	for i := range out {
		out[i] = model.SpreadView{ID: "s" + strconv.Itoa(i), W: 600, H: 300, Layout: "r1"}
	}
	return out
}
