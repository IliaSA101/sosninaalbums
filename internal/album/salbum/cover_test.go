package salbum

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Обложка в файле (спека 032, критерии 7–9 и страж полей листа).

// coverAlbum — альбом 281×284 в dir: фото a (место обложки), b (фон лицевой), c (разворот),
// файл-разметка dir/Разметка/c.png; обложка «тонкие листы», раскладка both.
func coverAlbum(t *testing.T, dir string) *model.Project {
	t.Helper()
	p := model.New()
	p.Settings.PageW, p.Settings.PageH = 281, 284
	p.Folders = []string{filepath.Join(dir, "Отбор")}
	ids := map[string]string{}
	for _, n := range []string{"a", "b", "c"} {
		path := filepath.Join(dir, "Отбор", n+".jpg")
		putFile(t, path, 10)
		ids[n] = library.IDFor(path)
		p.Photos[ids[n]] = model.PhotoRef{Path: path, W: 300, H: 200, Size: 10}
	}
	guide := filepath.Join(dir, "Разметка", "c.png")
	putFile(t, guide, 10)
	pr, _ := model.CoverPresetOf(model.CoverDefault)
	p.Cover = &model.Cover{
		Sheet: &model.Spread{ID: model.CoverID, Photos: []string{ids["a"]},
			Grid:     []*model.Place{{ID: "0123456789ab", X: 400, Y: 100, W: 100, H: 80, Photo: ids["a"]}},
			Texts:    []*model.TextBlock{{ID: "0123456789ac", X: 280, Y: 160, W: 60, Rot: -90, Text: "Ивановы", Style: model.PlainStyle()}},
			BgPhotoR: &model.Backdrop{Photo: ids["b"], X: 318.5, W: 304.5, H: 331, Fade: 0.25},
			BgR:      "#102030"},
		Spec: pr.Spec, Layout: model.CoverBoth, SpineBg: "#112233", Guide: guide,
	}
	p.Spreads = []*model.Spread{{ID: "0123456789ad", Photos: []string{ids["c"]}}}
	return p
}

func saveLoad(t *testing.T, p *model.Project, path string) ([]byte, *model.Project) {
	t.Helper()
	if err := Save(p, path); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	q, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := gridInvariants(q); err != nil {
		t.Fatal(err)
	}
	return data, q
}

// Save → Load → Save: тот же JSON; всё обложки на месте, разметка — относительная.
func TestCoverRoundTrip(t *testing.T) {
	dir := testkit.TempDir(t)
	p := coverAlbum(t, dir)
	path := filepath.Join(dir, "a.salbum")
	first, q := saveLoad(t, p, path)
	second, r := saveLoad(t, q, path)
	if !bytes.Equal(first, second) || !bytes.Equal(model.Marshal(q), model.Marshal(r)) {
		t.Fatalf("круг записи меняет файл:\n%s\n%s", first, second)
	}
	c := q.Cover
	if c == nil || c.Sheet.ID != model.CoverID || len(c.Sheet.Grid) != 1 || c.Sheet.Grid[0].Photo != p.Cover.Sheet.Photos[0] ||
		len(c.Sheet.Texts) != 1 || c.Sheet.BgPhotoR == nil || c.Sheet.BgPhotoR.Fade != 0.25 || c.Sheet.BgR != "#102030" {
		t.Fatalf("лист обложки: %+v", c)
	}
	if c.Layout != model.CoverBoth || c.SpineBg != "#112233" || c.Guide != p.Cover.Guide || !reflect.DeepEqual(c.Spec, p.Cover.Spec) {
		t.Fatalf("обложка: %+v", c)
	}
	f := fileJSON(t, path)
	if f.Cover == nil || f.Cover.Guide != "Разметка/c.png" || f.Cover.GuideAbs != p.Cover.Guide {
		t.Fatalf("разметка в файле: %+v", f.Cover)
	}
	// Фото на фоне обложки остаётся и в развороте (032 §2): фон фото не забирает.
	if !slices.Equal(q.Spreads[0].Photos, p.Spreads[0].Photos) {
		t.Fatalf("развороты: %v", q.Spreads[0].Photos)
	}
}

// Безопасные линии обложки (047, v8): 8/12 переживают Save → Load; у Нетпринта (0) ключей
// в файле нет; старый файл без них — 0 (Settings.Safe); чужие числа зажимаются в 0…30.
func TestSalbumCoverSafeRoundTrip(t *testing.T) {
	dir := testkit.TempDir(t)
	p := coverAlbum(t, dir)
	path := filepath.Join(dir, "a.salbum")
	first, q := saveLoad(t, p, path)
	if s := q.Cover.Spec; s.Safe != 8 || s.SpineSafe != 12 || !bytes.Contains(first, []byte(`"spineSafe"`)) {
		t.Fatalf("линии после Save → Load: %+v", s)
	}
	np, _ := model.CoverPresetOf("netprint-premium")
	q.Cover.Spec = np.Spec
	if data, r := saveLoad(t, q, path); bytes.Contains(data, []byte(`"spineSafe"`)) || r.Cover.Spec.Safe != 0 {
		t.Fatalf("линии 0 в файле: %s", data)
	}
	for in, want := range map[string][2]float64{
		``:                               {0, 0},
		`,"safe":99,"spineSafe":-3`:      {30, 0},
		`,"safe":7.777,"spineSafe":12.5`: {7.78, 12.5},
	} {
		r := mustDecode(t, coverJSON(dir, `[{"id":"s1","photos":["p"]}]`, `,"cover":{"spec":{"preset":"fsfera-thin","wrap":23.5`+in+`}}`), dir)
		if s := r.Cover.Spec; s.Safe != want[0] || s.SpineSafe != want[1] || s.Wrap != 23.5 {
			t.Errorf("%q: %+v, ждали %v", in, s, want)
		}
	}
}

// Без обложки ключа cover в файле нет: файл тот же, что до 032 (omitempty).
func TestCoverAbsentSame(t *testing.T) {
	dir := testkit.TempDir(t)
	p := coverAlbum(t, dir)
	p.Cover = nil
	p.PruneRefs() // фото обложки без неё не нужны
	path := filepath.Join(dir, "a.salbum")
	first, q := saveLoad(t, p, path)
	if bytes.Contains(first, []byte(`"cover"`)) || bytes.Contains(first, []byte(`"guide`)) || q.Cover != nil {
		t.Fatalf("без обложки в файле есть cover:\n%s", first)
	}
	if second, _ := saveLoad(t, q, path); !bytes.Equal(first, second) {
		t.Fatal("круг записи без обложки меняет файл")
	}
}

// coverJSON — файл v6 с фото p, q, r (dir/<id>.jpg), разворотами spreads и обложкой cover (сырой JSON).
func coverJSON(dir, spreads, cover string) []byte {
	ph := func(id string) string {
		b, _ := json.Marshal(filepath.Join(dir, id+".jpg"))
		return fmt.Sprintf(`%q:{"path":%s,"w":300,"h":200}`, id, b)
	}
	return fmt.Appendf(nil, `{"version":6,"settings":{"pageW":281,"pageH":284},"spreads":%s,"photos":{%s,%s,%s}%s}`,
		spreads, ph("p"), ph("q"), ph("r"), cover)
}

// Плохие числа, неупорядоченная и длинная таблица, sheet: null, чужой ID листа, лишние
// поля листа, фото без мест — зажимаются при Load, файл не отвергается (032 §4).
func TestCoverClampOnLoad(t *testing.T) {
	dir := testkit.TempDir(t)
	var rows []string
	for i := range 70 {
		rows = append(rows, fmt.Sprintf(`{"upto":%d,"mm":%d}`, 70-i, i))
	}
	rows = append(rows, `{"upto":3,"mm":1e300}`, `{"upto":-4,"mm":-1}`)
	data := coverJSON(dir, `[{"id":"s1","photos":["r","p"]}]`, `,"cover":{"sheet":{"id":"чужой","photos":["q","p"],
"layout":"H(0,1)","variant":3,"name":"Обложка","grid":[null,{"id":"c0ffee000001","x":1e308,"y":-5,"w":50,"h":50,"photo":"p"}]},
"spec":{"preset":"нет","w":-1,"h":1e9,"wrap":-3,"hinge":1e9,"a":-1,"b":99,"steps":[`+strings.Join(rows, ",")+`]},
"layout":"left","spineBg":"red","guide":"../../../../x.png"}`)
	p := mustDecode(t, data, dir)
	c := p.Cover
	if c == nil || c.Sheet.ID != model.CoverID || c.Sheet.Layout != "" || c.Sheet.Variant != 0 || c.Sheet.Name != "" {
		t.Fatalf("лист: %+v", c)
	}
	sp := c.Spec
	if sp.Preset != model.CoverCustom || sp.W != 0 || sp.H != 1000 || sp.Wrap != 0 || sp.Hinge != 20 || sp.A != 0 || sp.B != 5 || len(sp.Steps) != model.MaxSpineSteps {
		t.Fatalf("числа: %+v", sp)
	}
	if !reflect.DeepEqual(model.ClampCoverSpec(sp), sp) || c.Layout != model.CoverFront || c.SpineBg != "" || c.Guide != "" {
		t.Fatalf("обложка: %+v", c)
	}
	// p — на месте обложки (обложка первой в dedupe) и ушёл из разворота; q без места
	// с обложки ушёл совсем (у обложки нет «Авто»); в развороте остался r.
	id := func(n string) string { return library.IDFor(filepath.Join(dir, n+".jpg")) }
	if got := model.GridPhotos(c.Sheet); !slices.Equal(got, []string{id("p")}) || !slices.Equal(c.Sheet.Photos, got) {
		t.Fatalf("места обложки: %v, Photos %v", got, c.Sheet.Photos)
	}
	if len(p.Spreads) != 1 || !slices.Equal(p.Spreads[0].Photos, []string{id("r")}) || hasRef(p, id("q")) {
		t.Fatalf("развороты: %+v, фото %v", p.Spreads[0], p.Photos)
	}
	for _, v := range []string{`"sheet":null`, `"sheet":{"photos":["p"]}`, `"sheet":{"id":"x","photos":["p"],"grid":[null]}`} {
		p := mustDecode(t, coverJSON(dir, `[{"id":"s1","photos":["p"]}]`, `,"cover":{`+v+`,"spec":{"preset":"fsfera-thin"}}`), dir)
		if p.Cover == nil || p.Cover.Sheet == nil || p.Cover.Sheet.ID != model.CoverID || len(p.Cover.Sheet.Photos) != 0 || p.Cover.Sheet.Grid != nil {
			t.Fatalf("%s: %+v", v, p.Cover)
		}
		if len(p.Spreads) != 1 || len(p.Spreads[0].Photos) != 1 {
			t.Fatalf("%s: фото ушло из разворота: %+v", v, p.Spreads)
		}
		if sp := p.Cover.Spec; sp.Preset != "fsfera-thin" || sp.Wrap != 0 || sp.Steps != nil {
			t.Fatalf("%s: числа из файла — истина: %+v", v, sp)
		}
	}
}

// Мест на обложке больше предела, и все заняты: зажим оставляет MaxPlaces, лишним фото места не
// находится (у обложки нет «Авто»). Сетка снимается, фото обложки уходят целиком вместе со
// своими записями, разворот и фон не страдают; повторный круг файла не меняет.
func TestCoverOverfullGridDropsPhotos(t *testing.T) {
	dir := testkit.TempDir(t)
	p := coverAlbum(t, dir)
	n := model.MaxPlaces + 3
	p.Cover.Sheet.Grid, p.Cover.Sheet.Photos = nil, nil
	var gone []string
	for i := range n {
		path := filepath.Join(dir, "Отбор", fmt.Sprintf("m%03d.jpg", i))
		putFile(t, path, 10)
		id := library.IDFor(path)
		gone = append(gone, id)
		p.Photos[id] = model.PhotoRef{Path: path, W: 300, H: 200, Size: 10}
		p.Cover.Sheet.Grid = append(p.Cover.Sheet.Grid, box(fmt.Sprintf("%012x", 0x1000+i), 10+float64(i%10)*20, 10+float64(i/10)*20, 15, 15, id))
		p.Cover.Sheet.Photos = append(p.Cover.Sheet.Photos, id)
	}
	data, err := Encode(p, dir)
	if err != nil {
		t.Fatal(err)
	}
	q := mustDecode(t, data, dir)
	c := q.Cover.Sheet
	if c.Grid != nil || len(c.Photos) != 0 {
		t.Fatalf("сетка обложки не снята: мест %d, фото %d", len(c.Grid), len(c.Photos))
	}
	for _, id := range gone {
		if hasRef(q, id) {
			t.Fatalf("запись фото обложки %s осталась без дела", id)
		}
	}
	if c.BgPhotoR == nil || len(q.Spreads) != 1 || len(q.Spreads[0].Photos) != 1 || len(q.Photos) != 2 {
		t.Fatalf("фон или разворот пострадали: фон %+v, развороты %+v, фото %d", c.BgPhotoR, q.Spreads, len(q.Photos))
	}
	again, err := Encode(q, dir)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(model.Marshal(q), model.Marshal(mustDecode(t, again, dir))) {
		t.Fatal("круг файла меняет проект")
	}
}

// Путь разметки — как у фото: относительный в файле, abs рядом; запасной abs, если
// основного нет; UNC, устройства, «../../..» и чужое расширение — "" (032, критерий 8).
func TestCoverGuidePath(t *testing.T) {
	dir := testkit.TempDir(t)
	guide := filepath.Join(dir, "Разметка", "c.png")
	putFile(t, guide, 10)
	load := func(g, abs string) string {
		t.Helper()
		gj, _ := json.Marshal(g)
		aj, _ := json.Marshal(abs)
		p := mustDecode(t, coverJSON(dir, `[]`, fmt.Sprintf(`,"cover":{"spec":{},"guide":%s,"guideAbs":%s}`, gj, aj)), dir)
		return p.Cover.Guide
	}
	if g := load("Разметка/c.png", ""); g != guide {
		t.Fatalf("относительный: %q", g)
	}
	if g := load("Нет/c.png", guide); g != guide {
		t.Fatalf("основного нет — запасной: %q", g)
	}
	if g := load("Нет/c.png", ""); g != filepath.Join(dir, "Нет", "c.png") {
		t.Fatalf("файла нет нигде — путь остаётся: %q", g)
	}
	// Основной на месте — он и остаётся, запасной (есть он или нет) его не вытесняет.
	other := filepath.Join(dir, "Другая", "c.png")
	if g := load("Разметка/c.png", other); g != guide {
		t.Fatalf("основной есть, запасного нет: %q", g)
	}
	putFile(t, other, 10)
	if g := load("Разметка/c.png", other); g != guide {
		t.Fatalf("основной и запасной есть: %q", g)
	}
	bad := []string{"../../../c.png", "Разметка/c.bmp", "Разметка/c"}
	if runtime.GOOS == "windows" {
		bad = append(bad, `\\srv\share\c.png`, `\\?\C:\c.png`, `\\.\C:\c.png`, `C:\a\c.png:поток`)
	}
	for _, g := range bad {
		if got := load(g, ""); got != "" {
			t.Errorf("%q принят как %q", g, got)
		}
	}
	if runtime.GOOS == "windows" {
		p := model.New()
		p.Cover = &model.Cover{Sheet: &model.Spread{ID: model.CoverID}, Guide: `\\srv\share\c.png`}
		if Check(p) == nil {
			t.Fatal("Check пропустил UNC разметки")
		}
		p.Cover.Guide = guide
		if err := Check(p); err != nil {
			t.Fatal(err)
		}
	}
}

// Разметки нет ни по основному, ни по запасному пути: запасной едет в GuideAlt и пишется
// обратно в guideAbs (как PhotoRef.Alt), круг Encode → Decode его не теряет; файл нашёлся —
// GuideAlt пуст; guideAlt из файла (поле памяти) не читается и не пишется.
func TestCoverGuideAltKept(t *testing.T) {
	dir := testkit.TempDir(t)
	abs := filepath.Join(dir, "Старая", "c.png")
	gone := filepath.Join(dir, "Нет", "c.png")
	other := filepath.Join(dir, "Другая", "c.png") // guideAlt из файла — не guideAbs: не читается
	data := coverJSON(dir, `[]`, fmt.Sprintf(`,"cover":{"spec":{},"guide":"Нет/c.png","guideAbs":%q,"guideAlt":%q}`, abs, other))
	p := mustDecode(t, data, dir)
	if p.Cover.Guide != gone || p.Cover.GuideAlt != abs {
		t.Fatalf("разметка %q, запасной %q", p.Cover.Guide, p.Cover.GuideAlt)
	}
	out, err := Encode(p, dir)
	if err != nil {
		t.Fatal(err)
	}
	var f FileProject
	if err := json.Unmarshal(out, &f); err != nil {
		t.Fatal(err)
	}
	if f.Cover.Guide != "Нет/c.png" || f.Cover.GuideAbs != abs || bytes.Contains(out, []byte("guideAlt")) {
		t.Fatalf("в файле: guide %q, guideAbs %q\n%s", f.Cover.Guide, f.Cover.GuideAbs, out)
	}
	q := mustDecode(t, out, dir)
	if !bytes.Equal(model.Marshal(p), model.Marshal(q)) {
		t.Fatalf("круг теряет запасной путь:\n%s\n%s", model.Marshal(p), model.Marshal(q))
	}
	if err := Check(q); err != nil {
		t.Fatal(err)
	}
	// Запасной нашёлся — он и становится путём, GuideAlt пуст.
	putFile(t, abs, 10)
	if r := mustDecode(t, out, dir); r.Cover.Guide != abs || r.Cover.GuideAlt != "" {
		t.Fatalf("запасной на месте: %q, alt %q", r.Cover.Guide, r.Cover.GuideAlt)
	}
	// Без разметки и без guideAbs — пусто (края).
	if r := mustDecode(t, coverJSON(dir, `[]`, `,"cover":{"spec":{},"guideAbs":"C:/x.png"}`), dir); r.Cover.Guide != "" || r.Cover.GuideAlt != "" {
		t.Fatalf("без guide: %+v", r.Cover)
	}
	if runtime.GOOS == "windows" {
		q.Cover.GuideAlt = `\\srv\share\c.png`
		if Check(q) == nil {
			t.Fatal("Check пропустил UNC в запасном пути разметки")
		}
	}
}

// Смена буквы диска переносит и разметку обложки (RelinkDrive).
func TestCoverGuideRelinkDrive(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("пути с буквами дисков")
	}
	root := testkit.TempDir(t)
	vol := filepath.VolumeName(root)
	to, _ := paths.DriveLetter(vol)
	from := "Q"
	if to == from {
		from = "P"
	}
	onDisk := filepath.Join(root, "Разметка", "c.png")
	putFile(t, onDisk, 10)
	p := model.New()
	p.Cover = &model.Cover{Sheet: &model.Spread{ID: model.CoverID}, Guide: from + ":" + onDisk[len(vol):]}
	RelinkDrive(p, from, to)
	if p.Cover.Guide != onDisk {
		t.Fatalf("разметка: %q, ждали %q", p.Cover.Guide, onDisk)
	}
	RelinkDrive(p, to, from) // файл на месте — не трогается
	if p.Cover.Guide != onDisk {
		t.Fatalf("разметка на месте уехала: %q", p.Cover.Guide)
	}
}

// Rekey переводит ID фото мест и фонов обложки на ID по пути (032, критерий 9).
func TestCoverRekey(t *testing.T) {
	dir := testkit.TempDir(t)
	data := coverJSON(dir, `[]`, `,"cover":{"spec":{},"sheet":{"photos":["p"],"grid":[{"id":"c0ffee000001","w":50,"h":50,"photo":"p"}],
"bgPhotoR":{"photo":"q","w":100,"h":100}}}`)
	p := mustDecode(t, data, dir)
	np, nq := library.IDFor(filepath.Join(dir, "p.jpg")), library.IDFor(filepath.Join(dir, "q.jpg"))
	cv := p.Cover.Sheet
	if cv.Grid[0].Photo != np || !slices.Equal(cv.Photos, []string{np}) || cv.BgPhotoR == nil || cv.BgPhotoR.Photo != nq {
		t.Fatalf("ID на обложке: %+v %v %+v", cv.Grid[0], cv.Photos, cv.BgPhotoR)
	}
	if !hasRef(p, np) || !hasRef(p, nq) || hasRef(p, "p") || hasRef(p, "r") {
		t.Fatalf("записи фото: %v", p.Photos)
	}
}

// ID «cover» у разворота файла занят всегда: с обложкой и без (032, критерий 11).
func TestCoverReservedID(t *testing.T) {
	dir := testkit.TempDir(t)
	for _, cover := range []string{"", `,"cover":{"spec":{}}`} {
		p := mustDecode(t, coverJSON(dir, `[{"id":"cover","photos":["p"]},{"id":"cover","photos":["q"]}]`, cover), dir)
		if len(p.Spreads) != 2 || p.Spreads[0].ID == model.CoverID || p.Spreads[1].ID == model.CoverID || p.Spreads[0].ID == p.Spreads[1].ID {
			t.Fatalf("%q: развороты %s, %s", cover, p.Spreads[0].ID, p.Spreads[1].ID)
		}
		if s := p.Sheet(model.CoverID); (cover == "") != (s == nil) || s != nil && s != p.Cover.Sheet {
			t.Fatalf("%q: Sheet(cover) = %+v", cover, s)
		}
	}
}

// TestCoverSheetFields — страж: каждое поле Spread на обложке либо «можно», либо
// «обнулить» (FixCoverSheet). Новое поле без решения валит тест (032 §4, критик §2.5).
func TestCoverSheetFields(t *testing.T) {
	allowed := map[string]bool{"Photos": true, "Texts": true, "BgL": true, "BgR": true, "Grid": true,
		"Order": true, "BgPhoto": true, "BgPhotoL": true, "BgPhotoR": true, "Caps": true,
		// BgPhotoS — фото корешка: только у обложки (061).
		"BgPhotoS": true,
		// Vars — поля шапки: свои у обложки, перенос старых полей альбома (031 dialog §4);
		// Pat* — узоры фона, как фото фона (048 design §1); стикеры — как тексты (048 сц. 15).
		"Vars": true, "Pat": true, "PatL": true, "PatR": true, "Stickers": true}
	zeroed := map[string]bool{"ID": true, "Layout": true, "Variant": true, "Name": true, "Kind": true, "Vig": true}
	set := func(t *testing.T, v reflect.Value, typ reflect.Type) {
		switch v.Kind() {
		case reflect.String:
			v.SetString("x")
		case reflect.Slice:
			v.Set(reflect.MakeSlice(typ, 1, 1))
		case reflect.Int:
			v.SetInt(1)
		case reflect.Pointer:
			v.Set(reflect.New(typ.Elem()))
		default:
			t.Fatalf("тип %s — дописать в тест", typ)
		}
	}
	var s model.Spread
	typ := reflect.TypeFor[model.Spread]()
	for i := range typ.NumField() {
		f := typ.Field(i)
		if allowed[f.Name] == zeroed[f.Name] {
			t.Errorf("поле Spread.%s: решить, можно ли его на обложке (allowed) или обнулить (FixCoverSheet)", f.Name)
			continue
		}
		set(t, reflect.ValueOf(&s).Elem().Field(i), f.Type)
	}
	s.Grid = []*model.Place{{ID: "0123456789ab", W: 10, H: 10, Photo: "p"}} // nil-место Load выбрасывает
	model.FixCoverSheet(&s)
	v := reflect.ValueOf(s)
	for i := range typ.NumField() {
		name, zero := typ.Field(i).Name, v.Field(i).IsZero()
		switch {
		case name == "ID":
			if s.ID != model.CoverID {
				t.Errorf("ID %q", s.ID)
			}
		case zeroed[name] && !zero:
			t.Errorf("поле %s не обнулено на обложке", name)
		case allowed[name] && zero:
			t.Errorf("поле %s на обложке стёрто", name)
		}
	}
}

func ya(n int) string { return strings.Repeat("я", n) }

func budgetBlock(n int) *model.TextBlock {
	return &model.TextBlock{ID: model.NewID(), X: 10, Y: 10, W: 100, Text: ya(n), Style: model.PlainStyle()}
}

// budgetProject — проект ровно на границе запаса знаков (MaxTotalRunes, 204 блока):
// поля шапки 40 (обложка 15, второй разворот 25), обложка — блок 1000 и шаблон подписей 20,
// развороты — 197 блоков по 1000, шесть подписей по 300 и имя 140.
func budgetProject(dir string) *model.Project {
	p := model.New()
	p.Cover = &model.Cover{Sheet: &model.Spread{ID: model.CoverID, Texts: []*model.TextBlock{budgetBlock(1000)},
		Caps: &model.SheetCaps{Style: model.DefaultCaptionStyle(), Pad: 1, Text: ya(20)}, Vars: &model.Vars{Title: ya(15)}}}
	var ids []string
	for i := range 6 {
		path := filepath.Join(dir, fmt.Sprintf("%d.jpg", i))
		id := library.IDFor(path)
		ids = append(ids, id)
		p.Photos[id] = model.PhotoRef{Path: path, W: 300, H: 200,
			Caption: &model.Caption{Text: ya(model.MaxCaptionRunes), Style: model.DefaultCaptionStyle()}}
	}
	r := p.Photos[ids[0]]
	r.Person = &model.Person{Last: ya(60), First: ya(60), Middle: ya(20)}
	p.Photos[ids[0]] = r
	for k := 0; k < 197; k += model.MaxTexts {
		s := &model.Spread{ID: model.NewID(), Photos: []string{}}
		for range min(model.MaxTexts, 197-k) {
			s.Texts = append(s.Texts, budgetBlock(1000))
		}
		p.Spreads = append(p.Spreads, s)
	}
	p.Spreads[0].Photos = ids
	p.Spreads[1].Vars = &model.Vars{Year: ya(25)}
	return p
}

// budgetCount — блоки и знаки проекта, как их считает общий запас.
func budgetCount(p *model.Project) (blocks, runes int) {
	n := func(s string) int { return len([]rune(s)) }
	for _, s := range p.Sheets() {
		if v := s.Vars; v != nil {
			runes += n(v.Title) + n(v.Subtitle) + n(v.Year) + n(v.Lead)
		}
		if s.Caps != nil {
			runes += n(s.Caps.Text)
		}
		for _, tb := range s.Texts {
			blocks, runes = blocks+1, runes+n(tb.Text)
		}
		for _, pl := range s.Grid { // таблички мест (071)
			runes += n(pl.Person.Last) + n(pl.Person.First) + n(pl.Person.Middle) + n(pl.Person.Subject)
		}
	}
	for _, r := range p.Photos {
		if r.Caption != nil {
			blocks, runes = blocks+1, runes+n(r.Caption.Text)
		}
		if r.Person != nil {
			runes += n(r.Person.Last) + n(r.Person.First) + n(r.Person.Middle)
		}
	}
	return blocks, runes
}

// TestTotalsMatchLoad — CheckTotals и Load считают общий запас текста одним обходом:
// проект ровно на границе (Vars, обложка, развороты, подписи, имена) переживает
// Save → Load без потерь, а лишний знак в любой из частей правка не пропускает.
func TestTotalsMatchLoad(t *testing.T) {
	dir := testkit.TempDir(t)
	p := budgetProject(dir)
	if b, r := budgetCount(p); b != 204 || r != model.MaxTotalRunes {
		t.Fatalf("заготовка: %d блоков, %d знаков", b, r)
	}
	if err := p.CheckTotals(p); err != nil {
		t.Fatalf("граница запаса не проходит: %v", err)
	}
	_, q := saveLoad(t, p, filepath.Join(dir, "a.salbum"))
	if b, r := budgetCount(q); b != 204 || r != model.MaxTotalRunes {
		t.Fatalf("Load обрезал проект на границе: %d блоков, %d знаков", b, r)
	}
	for name, more := range map[string]func(p *model.Project){
		"vars":     func(p *model.Project) { p.Spreads[1].Vars.Lead = "я" },
		"vars-обл": func(p *model.Project) { p.Cover.Sheet.Vars.Subtitle = "я" },
		"обложка":  func(p *model.Project) { p.Cover.Sheet.Texts = append(p.Cover.Sheet.Texts, budgetBlock(1)) },
		"шаблон":   func(p *model.Project) { p.Cover.Sheet.Caps.Text += "я" },
		"разворот": func(p *model.Project) { p.Spreads[3].Texts = append(p.Spreads[3].Texts, budgetBlock(1)) },
		"имя":      func(p *model.Project) { p.Photos[p.Spreads[0].Photos[0]].Person.Middle += "я" },
	} {
		p := budgetProject(dir)
		more(p)
		if b, r := budgetCount(p); r != model.MaxTotalRunes+1 || b > 205 {
			t.Fatalf("%s: заготовка %d блоков, %d знаков", name, b, r)
		}
		if err := p.CheckTotals(p); err == nil {
			t.Errorf("%s: знак сверх запаса прошёл CheckTotals", name)
		}
	}
}
