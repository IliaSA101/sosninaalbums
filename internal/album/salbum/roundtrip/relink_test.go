package roundtrip_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

func TestRelinkRekeys(t *testing.T) {
	edtest.SkipNotWindows(t)
	s := edtest.NewRelinkSetup(t)
	before := s.E.View()
	v := s.E.Relink(map[string]string{s.IdA: s.NewA, s.IdB: s.NewB}, "")
	nA, nB := library.IDFor(s.NewA), library.IDFor(s.NewB)
	if got := v.Spreads[0].Photos; !slices.Equal(got, []string{nA, nB}) {
		t.Fatalf("ID в развороте: %v, ждали [%s %s]", got, nA, nB)
	}
	refs := s.E.Refs()
	if r := refs[nA]; r.Path != s.NewA || r.Frame == nil || r.Frame.S != 2 {
		t.Errorf("фото A: %+v (кадр должен остаться)", r)
	}
	if _, old := refs[s.IdA]; old {
		t.Error("старый ID остался")
	}
	// Папка переехала по общему хвосту: …\E\Съёмки → …\F.
	if !slices.Equal(v.Folders, []string{s.NewFd}) {
		t.Errorf("папки: %v, ждали [%s]", v.Folders, s.NewFd)
	}
	// Один шаг отмены возвращает пути, ID и папку.
	u := s.E.Undo()
	if !slices.Equal(u.Spreads[0].Photos, before.Spreads[0].Photos) || !slices.Equal(u.Folders, before.Folders) {
		t.Errorf("после отмены: %v %v", u.Spreads[0].Photos, u.Folders)
	}
	if r := s.E.Refs()[s.IdA]; r.Frame == nil {
		t.Error("отмена откатила и правку кадра: relink не один шаг")
	}
}

// Фото только на месте обложки — в альбоме, а не «только на фоне» (BgOnly): папка
// переезжает по его паре путей, место обложки получает новый ID (032 §1).
func TestRelinkCoverPhoto(t *testing.T) {
	edtest.SkipNotWindows(t)
	s := edtest.NewRelinkSetup(t)
	pr, _ := model.CoverPresetOf(model.CoverDefault)
	edtest.Change(s.E, func(p *model.Project) {
		p.Cover = &model.Cover{Spec: pr.Spec, Sheet: &model.Spread{ID: model.CoverID}}
		cv := p.Cover.Sheet
		pl, err := p.AddPlace(cv, 1, 1, p.PlaceIDs())
		if err != nil {
			t.Fatal(err)
		}
		p.Put(cv, pl, s.IdA)
	})
	if slices.Contains(edtest.P(s.E).Spreads[0].Photos, s.IdA) {
		t.Fatal("фото осталось в развороте")
	}
	v := s.E.Relink(map[string]string{s.IdA: s.NewA}, "")
	if !slices.Equal(v.Folders, []string{s.NewFd}) {
		t.Fatalf("папка не переехала за фото обложки: %v", v.Folders)
	}
	if got := edtest.P(s.E).Cover.Sheet.Grid[0].Photo; got != library.IDFor(s.NewA) {
		t.Fatalf("место обложки: %q", got)
	}
	edtest.MustGrid(t, s.E)
}

func TestRelinkAddFolderAndGuards(t *testing.T) {
	edtest.SkipNotWindows(t)
	s := edtest.NewRelinkSetup(t)
	other := filepath.Join(filepath.Dir(s.NewFd), "Чужая")
	// Имя файла другое: префикс не находится, папки с новым фото нет — добавляется выбранная.
	renamed := filepath.Join(s.NewFd, "Отбор", "новое.jpg")
	edtest.PutFile(t, renamed, 30)
	s.E.RemoveFolder(s.OldFolder)
	v := s.E.Relink(map[string]string{s.IdA: renamed}, s.NewFd)
	if !slices.Equal(v.Folders, []string{s.NewFd}) {
		t.Errorf("addFolder: %v", v.Folders)
	}
	// Ничего не перепривязано (неизвестный ID, UNC) — проект и папки прежние.
	rev := s.E.View().Rev
	v = s.E.Relink(map[string]string{"нет": s.NewA, s.IdB: `\\srv\share\2.jpg`}, other)
	if v.Rev != rev || len(v.Folders) != 1 || s.E.Refs()[s.IdB].Path == `\\srv\share\2.jpg` {
		t.Errorf("пустая перепривязка изменила проект: %v", v.Folders)
	}
	// Путь, занятый другим фото альбома, не берётся.
	s.E.Relink(map[string]string{s.IdB: renamed}, "")
	if s.E.Refs()[s.IdB].Path == renamed {
		t.Error("два фото на одном файле")
	}
}

func TestRelinkDrive(t *testing.T) {
	edtest.SkipNotWindows(t)
	root := testkit.TempDir(t)
	vol := filepath.VolumeName(root) // реальный диск; «пропавший» — любая другая буква
	to, _ := paths.DriveLetter(vol)
	from := "Q"
	if to == from {
		from = "P"
	}
	tail := root[len(vol):]
	newFolder := filepath.Join(root, "Съёмка")
	edtest.PutFile(t, filepath.Join(newFolder, "1.jpg"), 10)
	oldPath := from + ":" + filepath.Join(tail, "Съёмка", "1.jpg")
	e := edtest.Open(t, model.New(), "")
	edtest.AddFolder(e, from+":"+filepath.Join(tail, "Съёмка"))
	id := library.IDFor(oldPath)
	edtest.NewSpread(e, -1, []model.PhotoInfo{{ID: id, Path: oldPath, W: 3, H: 2, Size: 10}})
	v := e.RelinkDrive(from, to)
	nid := library.IDFor(filepath.Join(newFolder, "1.jpg"))
	if v.Spreads[0].Photos[0] != nid || !slices.Equal(v.Folders, []string{newFolder}) {
		t.Fatalf("смена диска: %v %v", v.Spreads[0].Photos, v.Folders)
	}
	if v2 := e.RelinkDrive(from, to); v2.Rev != v.Rev {
		t.Error("повторная смена диска — не холостая правка")
	}
}

// Папка не нашлась ни по относительному пути, ни по abs: abs не теряется при записи.
func TestFolderAltKept(t *testing.T) {
	dir := testkit.TempDir(t)
	abs := filepath.Join(testkit.TempDir(t), "нет", "Съёмка")
	f := salbum.FileProject{
		Version: model.FormatVersion, Settings: model.DefaultSettings(), Spreads: []*model.Spread{},
		Folders: []salbum.FileFolder{{Path: "Съёмка", Abs: abs}},
		Photos:  map[string]salbum.FilePhoto{},
	}
	data, _ := json.Marshal(f)
	path := filepath.Join(dir, "a.salbum")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := salbum.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	rel := filepath.Join(dir, "Съёмка")
	if !slices.Equal(p.Folders, []string{rel}) || p.FolderAlt[rel] != abs {
		t.Fatalf("папка: %v, alt %v", p.Folders, p.FolderAlt)
	}
	if err := edtest.Open(t, p, path).Save(""); err != nil {
		t.Fatal(err)
	}
	if got := fileJSON(t, path).Folders; len(got) != 1 || got[0].Path != "Съёмка" || got[0].Abs != abs {
		t.Fatalf("после сохранения: %+v", got)
	}
	// Папка появилась на месте — запасной путь не нужен.
	if err := os.MkdirAll(rel, 0o755); err != nil {
		t.Fatal(err)
	}
	p2, err := salbum.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(p2.FolderAlt) != 0 {
		t.Errorf("папка нашлась, alt не нужен: %v", p2.FolderAlt)
	}
}

// libBg кладёт фон-«своё» с ключом key и путём path на разворот i (ID — от ключа).
func libBg(e *album.Editor, i int, key, path string) string {
	id := model.LibPhotoID(key)
	edtest.Change(e, func(p *model.Project) {
		p.Photos[id] = model.PhotoRef{Lib: key, Path: path, W: 3, H: 2}
		p.Spreads[i].BgPhoto = &model.Backdrop{Photo: id, W: 100, H: 50}
	})
	return id
}

// Relink и RelinkDrive своё из библиотеки не трогают: его нет в папке фотографа (путь даёт
// искатель, RelinkLib); соседнее фото той же правкой переезжает.
func TestRelinkSkipsLib(t *testing.T) {
	edtest.SkipNotWindows(t)
	s := edtest.NewRelinkSetup(t)
	lid := libBg(s.E, 0, "my:0123456789abcdef", "")
	s.E.Relink(map[string]string{s.IdA: s.NewA, lid: s.NewB}, "")
	refs := s.E.Refs()
	if r, ok := refs[lid]; !ok || r.Path != "" || r.Lib != "my:0123456789abcdef" {
		t.Errorf("своё после Relink: %+v %v", r, ok)
	}
	if _, ok := refs[library.IDFor(s.NewA)]; !ok {
		t.Error("фото A не переехало")
	}
	// Смена диска: своё на «пропавшем» диске, файл — на настоящем.
	root := testkit.TempDir(t)
	vol := filepath.VolumeName(root)
	to, _ := paths.DriveLetter(vol)
	from := "Q"
	if to == from {
		from = "P"
	}
	edtest.PutFile(t, filepath.Join(root, "Библиотека", "x.png"), 10)
	edtest.PutFile(t, filepath.Join(root, "Съёмка", "1.jpg"), 10)
	oldLib := from + ":" + filepath.Join(root[len(vol):], "Библиотека", "x.png")
	oldPh := from + ":" + filepath.Join(root[len(vol):], "Съёмка", "1.jpg")
	e := edtest.Open(t, model.New(), "")
	edtest.NewSpread(e, -1, []model.PhotoInfo{{ID: library.IDFor(oldPh), Path: oldPh, W: 3, H: 2, Size: 10}})
	lid = libBg(e, 0, "my:fedcba9876543210", oldLib)
	e.RelinkDrive(from, to)
	if r := e.Refs()[lid]; r.Path != oldLib {
		t.Errorf("своё после смены диска: %+v", r)
	}
	if _, ok := e.Refs()[library.IDFor(filepath.Join(root, "Съёмка", "1.jpg"))]; !ok {
		t.Error("фото не переехало на другой диск")
	}
}

// RelinkLib: нашлось после загрузки — один шаг отмены на все свои; ничего нового (нет в
// библиотеке, тот же путь) — холостая, без шага; ненайденное прежний путь не теряет.
func TestRelinkLib(t *testing.T) {
	root := testkit.TempDir(t)
	e, _, _ := edtest.NewTwo(t)
	k1, k2, k3 := "my:0123456789abcdef", "my:fedcba9876543210", "my:00000000000000aa"
	f1, f2 := filepath.Join(root, "1.png"), filepath.Join(root, "2.png")
	f3 := filepath.Join(root, "3.png")
	for _, f := range []string{f1, f3} {
		edtest.PutFile(t, f, 10)
	}
	writePNG(t, f2, 40, 20)
	id1, id2 := libBg(e, 0, k1, ""), libBg(e, 1, k2, "")
	edtest.Change(e, func(p *model.Project) { // размеры неизвестны: прочтутся из найденного файла
		r := p.Photos[id2]
		r.W, r.H = 0, 0
		p.Photos[id2] = r
	})
	id3 := model.LibPhotoID(k3)
	edtest.Change(e, func(p *model.Project) {
		p.Photos[id3] = model.PhotoRef{Lib: k3, Path: f3, W: 3, H: 2}
		p.Spreads[0].BgPhotoL, p.Spreads[0].BgPhoto = &model.Backdrop{Photo: id3, W: 50, H: 50}, nil
		p.Spreads[0].BgPhotoR = &model.Backdrop{Photo: id1, W: 50, H: 50}
	})
	found := map[string]string{}
	edtest.LibFinder(t, found)
	n, rev := edtest.UndoLen(e), e.View().Rev
	// Пустой искатель и nil (поставленный, тоже пустой) — холостые.
	if v := e.RelinkLib(func(string) (string, bool) { return "", false }); v.Rev != rev || edtest.UndoLen(e) != n {
		t.Fatal("ничего не нашлось — а шаг есть")
	}
	if v := e.RelinkLib(nil); v.Rev != rev {
		t.Fatal("пустой искатель — а шаг есть")
	}
	found[k1], found[k2] = f1, f2 // загрузили два; третий в библиотеке пропал — путь остаётся
	v := e.RelinkLib(nil)
	refs := e.Refs()
	if v.Rev == rev || edtest.UndoLen(e) != n+1 || refs[id1].Path != f1 || refs[id2].Path != f2 || refs[id3].Path != f3 ||
		refs[id1].W != 3 || refs[id2].W != 40 || refs[id2].H != 20 {
		t.Fatalf("после загрузки: шагов %d, %+v", edtest.UndoLen(e)-n, refs)
	}
	if v2 := e.RelinkLib(nil); v2.Rev != v.Rev || edtest.UndoLen(e) != n+1 {
		t.Error("повтор — не холостой")
	}
	e.Undo()
	if refs = e.Refs(); refs[id1].Path != "" || refs[id2].Path != "" || refs[id3].Path != f3 {
		t.Errorf("отмена: %+v", refs)
	}
}

// Стикеры (спека 048 Ф2), формат v8.

// TestSalbumStickerRoundTrip — Save → Load сохраняет стикеры всех видов (своё по ключу без пути,
// рисунок с цветами, ключ новой версии), зеркало, прозрачность, поворот и Order «Авто»; второй
// круг — тот же проект; файл без стикеров поля не несёт.
func TestSalbumStickerRoundTrip(t *testing.T) {
	root := testkit.TempDir(t)
	f1 := filepath.Join(root, "Библиотека", "Стикеры", "0123456789abcdef.png")
	writePNG(t, f1, 40, 20)
	edtest.LibFinder(t, map[string]string{libK1: f1})
	e := edtest.Open(t, model.New(), "")
	sid := edtest.NewSpread(e, -1, edtest.Photos("a")).Spreads[0].ID
	path := filepath.Join(root, "Альбом", "a.salbum")
	if err := e.Save(path); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(path); bytes.Contains(data, []byte(`"stickers"`)) || bytes.Contains(data, []byte(`"order"`)) {
		t.Fatalf("файл без стикеров:\n%s", data)
	}
	add := func(src model.StickerSrc) string {
		_, id, err := e.AddSticker(sid, 0, src)
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	own := add(model.StickerSrc{Photo: model.PhotoInfo{ID: model.LibPhotoID(libK1), Path: f1, Lib: libK1, W: 40, H: 20}})
	plate := add(model.StickerSrc{Art: &model.Art{Kind: "plate", Inks: []string{"#123456"}}})
	future := add(model.StickerSrc{Art: &model.Art{Kind: "zz-future", Inks: []string{"#aabbcc"}, Seed: 9}})
	e.SetStickerBox(sid, plate, 20, 30, 120, 20, 30, "")
	e.FlipSticker(sid, own, "h")
	e.SetStickerFade(sid, own, 0.4, "")
	e.ArrangeLayers(sid, "back", []string{future})
	edtest.AddText(t, e, sid, 1)
	e.ArrangeLayers(sid, "front", []string{plate})
	if err := e.Save(path); err != nil {
		t.Fatal(err)
	}
	fp := fileJSON(t, path)
	if r := fp.Photos[model.LibPhotoID(libK1)]; r.Path != "" || r.Abs != "" || r.Lib != libK1 {
		t.Fatalf("запись своего стикера: %+v", r)
	}
	if s := fp.Spreads[0]; len(s.Stickers) != 3 || len(s.Order) != 4 {
		t.Fatalf("в файле: стикеров %d, order %v", len(s.Stickers), s.Order)
	}
	q, err := salbum.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	// ID фото «a» — не от пути (тестовый проект): сравниваются стикеры, порядок и записи своих.
	was := edtest.P(e).Spreads[0]
	if a, b := string(model.Marshal(&model.Project{Spreads: []*model.Spread{{Stickers: was.Stickers, Order: was.Order}}})),
		string(model.Marshal(&model.Project{Spreads: []*model.Spread{{Stickers: q.Spreads[0].Stickers, Order: q.Spreads[0].Order}}})); a != b ||
		q.Photos[model.LibPhotoID(libK1)] != edtest.P(e).Photos[model.LibPhotoID(libK1)] {
		t.Fatalf("Save → Load:\n%s\n%s", a, b)
	}
	if err := edtest.Reopen(q); err != nil {
		t.Fatal(err)
	}
	if st := q.Spreads[0].Stickers; st[0].ID != future || st[1].Photo == "" || !st[1].FlipH || st[1].Fade != 0.4 ||
		st[2].Rot != 30 || st[2].W != 120 || st[2].H != 20 || st[2].Art.Inks[0] != "#123456" || q.Spreads[0].Order[3] != plate {
		t.Fatalf("стикеры после чтения: %s", model.Marshal(q))
	}
}

// TestClampSticker — зажим при чтении: без фото в Photos, без рисунка, с обоими, ключ не по
// грамматике и ключ узора — отброшены; неизвестный ключ хранится; размеры, поворот, прозрачность,
// центр — в пределах; плохой и повторный ID (и с текстом) — новый; сверх MaxStickers на лист и
// MaxTotalStickers на проект — отброшены.
func TestClampSticker(t *testing.T) {
	art := func(kind string, inks ...string) *model.Art { return &model.Art{Kind: kind, Inks: inks} }
	const keep, dup = "c0ffee0000a1", "c0ffee0000a2"
	first := []*model.Sticker{
		{ID: keep, Photo: "x1", X: 10, Y: 10, W: 80, H: 10},
		{ID: "c0ffee0000b1", Photo: "nope", W: 10, H: 10},
		{ID: "c0ffee0000b2", W: 10, H: 10},
		{ID: "c0ffee0000b3", Photo: "x1", Art: art("plate"), W: 10, H: 10},
		{ID: "c0ffee0000b4", Art: art("BAD!"), W: 10, H: 10},
		{ID: "c0ffee0000b5", Art: art("dots"), W: 10, H: 10},
		{ID: dup, Art: art("zz-future", "#ABCDEF", "nope", "#000000"), W: 10, H: 10},
		{ID: dup, Art: art("plate", "#ABCDEF", "#111111", "#222222"), X: 1e9, Y: -1e9, W: -5, H: 1e9, Rot: 540, Fade: 5},
		{ID: "", Art: art("plate"), W: 10, H: 10},
		{ID: "c0ffee0000t1", Art: art("plate"), W: 10, H: 10}, // ID текста листа
	}
	many := func() []*model.Sticker {
		out := make([]*model.Sticker, model.MaxStickers+6)
		for i := range out {
			out[i] = &model.Sticker{ID: model.NewID(), Art: art("plate"), X: 10, Y: 10, W: 30, H: 10}
		}
		return out
	}
	spreads := []*model.Spread{{ID: "s1", Photos: []string{}, Stickers: first,
		Texts: []*model.TextBlock{{ID: "c0ffee0000t1", X: 10, Y: 10, W: 50, Text: "Т", Style: model.PlainStyle()}}}}
	for i := range model.MaxTotalStickers/model.MaxStickers + 1 {
		spreads = append(spreads, &model.Spread{ID: fmt.Sprintf("%012x", i+1), Photos: []string{}, Stickers: many()})
	}
	f := salbum.FileProject{Version: model.FormatVersion, Settings: model.DefaultSettings(), Spreads: spreads,
		Photos: map[string]salbum.FilePhoto{"x1": {PhotoRef: model.PhotoRef{Lib: libK1, W: 400, H: 200}}}}
	data, err := json.Marshal(f)
	if err != nil {
		t.Fatal(err)
	}
	p := edtest.MustDecode(t, data, testkit.TempDir(t)) // с инвариантами стикеров
	got := p.Spreads[0].Stickers
	if len(got) != 5 {
		t.Fatalf("осталось %d: %+v", len(got), got)
	}
	if st := got[0]; st.ID != keep || st.Photo != model.LibPhotoID(libK1) || st.W != 80 || st.H != 40 || st.Y != -5 {
		t.Errorf("своё (пропорция 2:1 вокруг центра): %+v", *st)
	}
	if st := got[1]; st.ID != dup || st.Art.Kind != "zz-future" || !slices.Equal(st.Art.Inks, []string{"#abcdef"}) {
		t.Errorf("неизвестный ключ: %+v %+v", *st, *st.Art)
	}
	if st := got[2]; st.ID == dup || st.W != model.MinSticker || st.H != 1200 || st.Rot != 180 || st.Fade != 1 ||
		st.X+st.W/2 != 600 || st.Y+st.H/2 != 0 || !slices.Equal(st.Art.Inks, []string{"#abcdef", "#111111"}) {
		t.Errorf("пределы (у плашки два цвета, третий отброшен): %+v %+v", *st, *st.Art)
	}
	if !model.IsTextID(got[3].ID) || got[4].ID == "c0ffee0000t1" || !model.IsTextID(got[4].ID) {
		t.Errorf("новые ID: %q, %q", got[3].ID, got[4].ID)
	}
	total := 0
	for i, s := range p.Spreads {
		if i > 0 && i < len(p.Spreads)-1 && len(s.Stickers) != model.MaxStickers {
			t.Errorf("лист %d: стикеров %d", i, len(s.Stickers))
		}
		total += len(s.Stickers)
	}
	if total != model.MaxTotalStickers {
		t.Errorf("в проекте %d, ждали %d", total, model.MaxTotalStickers)
	}
	if p.ClampSticker(p.Spreads[0], nil) != nil {
		t.Error("nil-стикер")
	}
}

// looseOf — пути отдельных фото проекта по порядку добавлений.
func looseOf(e *album.Editor) []string { return edtest.P(e).LooseFiles() }

// addLoose — одно добавление отдельных фото; все файлы должны войти.
func addLoose(t *testing.T, e *album.Editor, files ...string) {
	t.Helper()
	if _, r, err := e.AddLoose(files, 100); err != nil || r.Added != len(files) {
		t.Fatalf("AddLoose: %+v, %v", r, err)
	}
}

// 097 §5: отдельные фото альбома переехали — записи добавления следуют за ними. Ловушка: пары,
// у которых прежний путь — отдельное фото, не уходят в addFolderFor — выбранная папка ради
// них не добавляется. Один шаг отмены, повтор — холостой, круг Save → Load.
func TestRelinkLooseFollows(t *testing.T) {
	edtest.SkipNotWindows(t)
	root := testkit.TempDir(t)
	old1, old2 := filepath.Join(root, "E", "Отдельно", "1.jpg"), filepath.Join(root, "E", "Отдельно", "2.jpg")
	new1, new2 := filepath.Join(root, "F", "Отдельно", "1.jpg"), filepath.Join(root, "F", "Отдельно", "2.jpg")
	edtest.PutFile(t, new1, 30)
	edtest.PutFile(t, new2, 40)
	e := edtest.Open(t, model.New(), "")
	id1, id2 := library.IDFor(old1), library.IDFor(old2)
	edtest.NewSpread(e, -1, []model.PhotoInfo{{ID: id1, Path: old1, W: 3, H: 2, Size: 30}, {ID: id2, Path: old2, W: 3, H: 2, Size: 40}})
	addLoose(t, e, old1, old2)
	v := e.Relink(map[string]string{id1: new1, id2: new2}, filepath.Dir(new1))
	if len(v.Folders) != 0 {
		t.Fatalf("папка добавлена ради отдельных фото: %v", v.Folders)
	}
	if got := looseOf(e); !slices.Equal(got, []string{new1, new2}) {
		t.Fatalf("отдельные: %v", got)
	}
	if got := v.Spreads[0].Photos; !slices.Equal(got, []string{library.IDFor(new1), library.IDFor(new2)}) {
		t.Fatalf("ID в развороте: %v", got)
	}
	if v2 := e.Relink(map[string]string{id1: new1}, filepath.Dir(new1)); v2.Rev != v.Rev {
		t.Error("повторная перепривязка — не холостая правка")
	}
	if lp := edtest.SaveLoad(t, e); !slices.Equal(lp.LooseFiles(), []string{new1, new2}) || len(lp.Folders) != 0 {
		t.Fatalf("после Save → Load: %v, папки %v", lp.LooseFiles(), lp.Folders)
	}
	if u := e.Undo(); !slices.Equal(looseOf(e), []string{old1, old2}) || u.Spreads[0].Photos[0] != id1 {
		t.Fatalf("после отмены: %v %v", looseOf(e), u.Spreads[0].Photos)
	}
	// Вместе с обычным фото, новое место которого уже в папке альбома: выбранная папка тоже не нужна.
	keep := filepath.Join(root, "F", "Съёмка")
	old3, new3 := filepath.Join(root, "E", "Съёмка", "3.jpg"), filepath.Join(keep, "3.jpg")
	edtest.PutFile(t, new3, 50)
	edtest.AddFolder(e, keep)
	id3 := library.IDFor(old3)
	edtest.NewSpread(e, -1, []model.PhotoInfo{{ID: id3, Path: old3, W: 3, H: 2, Size: 50}})
	v = e.Relink(map[string]string{id1: new1, id3: new3}, filepath.Join(root, "G"))
	if !slices.Equal(v.Folders, []string{keep}) || looseOf(e)[0] != new1 {
		t.Fatalf("вместе с обычным фото: папки %v, отдельные %v", v.Folders, looseOf(e))
	}
}

// 097 §5: пропавшие отдельные не из альбома из той же папки, что прежний путь пары, переезжают
// следом, если файл с тем же именем есть в новой папке. Остаются: фото альбома (его ведёт своя
// пара), файл на прежнем месте, файл, которого нет и в новой папке, файл из другой папки.
func TestRelinkLooseSameDir(t *testing.T) {
	edtest.SkipNotWindows(t)
	root := testkit.TempDir(t)
	oldDir, newDir := filepath.Join(root, "E", "Отдельно"), filepath.Join(root, "F", "Отдельно")
	at := func(dir, name string) string { return filepath.Join(dir, name) }
	for _, n := range []string{"1.jpg", "2.jpg", "3.jpg", "4.jpg"} {
		edtest.PutFile(t, at(newDir, n), 30)
	}
	edtest.PutFile(t, at(oldDir, "4.jpg"), 30) // на месте
	otherOld := filepath.Join(root, "E", "Другое", "6.jpg")
	edtest.PutFile(t, filepath.Join(root, "F", "Другое", "6.jpg"), 30)
	e := edtest.Open(t, model.New(), "")
	id1, id3 := library.IDFor(at(oldDir, "1.jpg")), library.IDFor(at(oldDir, "3.jpg"))
	edtest.NewSpread(e, -1, []model.PhotoInfo{
		{ID: id1, Path: at(oldDir, "1.jpg"), W: 3, H: 2, Size: 30},
		{ID: id3, Path: at(oldDir, "3.jpg"), W: 3, H: 2, Size: 30},
	})
	addLoose(t, e, at(oldDir, "1.jpg"), at(oldDir, "2.jpg"), at(oldDir, "3.jpg"), at(oldDir, "4.jpg"), at(oldDir, "5.jpg"))
	addLoose(t, e, otherOld)
	before := looseOf(e)
	e.Relink(map[string]string{id1: at(newDir, "1.jpg")}, "")
	want := []string{at(newDir, "1.jpg"), at(newDir, "2.jpg"), at(oldDir, "3.jpg"), at(oldDir, "4.jpg"), at(oldDir, "5.jpg"), otherOld}
	if got := looseOf(e); !slices.Equal(got, want) {
		t.Fatalf("отдельные:\n%v\nждали\n%v", got, want)
	}
	e.Undo()
	if got := looseOf(e); !slices.Equal(got, before) {
		t.Fatalf("после отмены: %v", got)
	}
}

// 097 §5, смена буквы диска: отдельное фото альбома следует за своей парой, пропавшее не из
// альбома ищется на новом диске. Остаются: файл на месте, файл, которого нет и там, путь
// с другой буквой. Повтор — холостой, один шаг отмены.
func TestRelinkDriveLoose(t *testing.T) {
	edtest.SkipNotWindows(t)
	root := testkit.TempDir(t)
	vol := filepath.VolumeName(root)
	to, _ := paths.DriveLetter(vol)
	from, other := "Q", "Z"
	if to == from {
		from = "P"
	}
	if to == other {
		other = "Y"
	}
	tail := root[len(vol):]
	here := func(n string) string { return filepath.Join(root, "Отдельно", n) }
	gone := func(d, n string) string { return d + ":" + filepath.Join(tail, "Отдельно", n) }
	for _, n := range []string{"1.jpg", "2.jpg", "3.jpg"} {
		edtest.PutFile(t, here(n), 10)
	}
	e := edtest.Open(t, model.New(), "")
	id := library.IDFor(gone(from, "1.jpg"))
	edtest.NewSpread(e, -1, []model.PhotoInfo{{ID: id, Path: gone(from, "1.jpg"), W: 3, H: 2, Size: 10}})
	addLoose(t, e, gone(from, "1.jpg"), gone(from, "2.jpg"), here("3.jpg"), gone(from, "4.jpg"), gone(other, "5.jpg"))
	before := looseOf(e)
	v := e.RelinkDrive(from, to)
	want := []string{here("1.jpg"), here("2.jpg"), here("3.jpg"), gone(from, "4.jpg"), gone(other, "5.jpg")}
	if got := looseOf(e); !slices.Equal(got, want) {
		t.Fatalf("отдельные:\n%v\nждали\n%v", got, want)
	}
	if v.Spreads[0].Photos[0] != library.IDFor(here("1.jpg")) || len(v.Folders) != 0 {
		t.Fatalf("фото %v, папки %v", v.Spreads[0].Photos, v.Folders)
	}
	if v2 := e.RelinkDrive(from, to); v2.Rev != v.Rev {
		t.Error("повторная смена диска — не холостая правка")
	}
	e.Undo()
	if got := looseOf(e); !slices.Equal(got, before) {
		t.Fatalf("после отмены: %v", got)
	}
}
