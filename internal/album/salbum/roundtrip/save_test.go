package roundtrip_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/geom"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Golden v4 (testdata/v4-dev.salbum.json) открывается как текущая версия (6): первая
// запись делает .v4.bak с исходными байтами и пишет версию 6; данные те же, поля fade нет.
func TestMigrateV4ToV5(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "testdata", "v4-dev.salbum.json"))
	if err != nil {
		t.Fatal(err)
	}
	dir := testkit.TempDir(t)
	path := filepath.Join(dir, "a.salbum")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := salbum.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if p.Version != model.FormatVersion || edtest.From(p) != 4 {
		t.Fatalf("версия %d, from %d", p.Version, edtest.From(p))
	}
	e := edtest.Open(t, p, path)
	if !edtest.Backup(e) {
		t.Fatal("запись поверх v4 должна делать бэкап")
	}
	before := model.Marshal(p)
	if err := e.Save(path); err != nil {
		t.Fatal(err)
	}
	bak, err := os.ReadFile(path + ".v4.bak")
	if err != nil || !bytes.Equal(bak, data) {
		t.Fatalf(".v4.bak: %v, совпадает с исходником: %v", err, bytes.Equal(bak, data))
	}
	if f := fileJSON(t, path); f.Version != model.FormatVersion {
		t.Fatalf("версия в файле %d", f.Version)
	}
	if saved, _ := os.ReadFile(path); bytes.Contains(saved, []byte(`"fade"`)) {
		t.Fatalf("в файле без прозрачности появилось fade:\n%s", saved)
	}
	q, err := salbum.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, model.Marshal(q)) {
		t.Fatalf("потери при v4 → v6:\n%s\n%s", before, model.Marshal(q))
	}
	// Вторая запись копию не трогает: она уже есть, а файл уже v6.
	if err := os.WriteFile(path+".v4.bak", []byte("старое"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := edtest.Open(t, q, path).Save(path); err != nil {
		t.Fatal(err)
	}
	if bak, _ := os.ReadFile(path + ".v4.bak"); string(bak) != "старое" {
		t.Fatal("копия v4 перезаписана")
	}
}

func TestSaveLoadV3RoundTrip(t *testing.T) {
	dir := testkit.TempDir(t)
	ph := filepath.Join(dir, "Отбор", "a.jpg")
	edtest.PutFile(t, ph, 10)
	p, id := onePhoto(ph)
	p.Folders = []string{filepath.Join(dir, "Отбор")}
	p.Settings.Bg = "#f6f1e7"
	r := p.Photos[id]
	r.Size = 10
	r.Frame = &model.Frame{S: 1.35, X: -0.4, Y: 0.25}
	r.Caption = &model.Caption{Text: "Первый танец", Over: true, Style: model.ClampStyle(model.TextStyle{Font: "ptserif", Size: 9, Color: "#555555", Align: "left", Italic: true})}
	p.Photos[id] = r
	s := p.Spreads[0]
	s.Layout, s.BgL, s.BgR = "P[0|-]", "#1c1c1c", "#ffffff"
	s.Texts = []*model.TextBlock{{ID: "c0ffee000001", X: 293, Y: 18, W: 257, Text: "Иван\nи Мария",
		Style: model.ClampStyle(model.TextStyle{Font: "cormorant", Size: 28, Color: "#333333", Align: "center", Bold: true})}}
	// Сетка: место с фото и пустое «целиком» поверх (порядок = слои).
	s.Grid = []*model.Place{
		{ID: "c0ffee000002", X: 12.5, Y: 12, W: 127.25, H: 84.83, Photo: id},
		{ID: "c0ffee000003", X: 300, Y: 150, W: 60, H: 30, Fit: model.FitContain},
	}
	path := filepath.Join(dir, "a.salbum")
	if err := salbum.Save(p, path); err != nil {
		t.Fatal(err)
	}
	q, err := salbum.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if a, b := model.Marshal(p), model.Marshal(q); !bytes.Equal(a, b) {
		t.Fatalf("потери при записи:\n%s\n%s", a, b)
	}
	if edtest.From(q) != model.FormatVersion || edtest.Backup(edtest.Open(t, q, path)) {
		t.Fatal("файл текущей версии не требует бэкапа")
	}
}

func TestSaveAsRecomputesRelative(t *testing.T) {
	dir := testkit.TempDir(t)
	ph := filepath.Join(dir, "Отбор", "a.jpg")
	p, id := onePhoto(ph)
	e := edtest.Open(t, p, "")
	if err := e.Save(filepath.Join(dir, "a.salbum")); err != nil {
		t.Fatal(err)
	}
	if got := fileJSON(t, filepath.Join(dir, "a.salbum")).Photos[id].Path; got != "Отбор/a.jpg" {
		t.Fatalf("первое сохранение: %q", got)
	}
	as := filepath.Join(dir, "Альбом", "b.salbum")
	if err := e.Save(as); err != nil {
		t.Fatal(err)
	}
	if got := fileJSON(t, as).Photos[id].Path; got != "../Отбор/a.jpg" {
		t.Fatalf("«Сохранить как»: %q", got)
	}
}

func TestDraftWritesAbsolute(t *testing.T) {
	dir := testkit.TempDir(t)
	ph := filepath.Join(dir, "a.jpg")
	p, id := onePhoto(ph)
	p.Folders = []string{dir}
	draft := filepath.Join(dir, "draft.salbum")
	if err := edtest.Open(t, p, "").WriteCopy(draft); err != nil {
		t.Fatal(err)
	}
	f := fileJSON(t, draft)
	if got := f.Photos[id]; got.Path != ph || got.Abs != "" || f.Folders[0] != (salbum.FileFolder{Path: dir}) {
		t.Fatalf("черновик: %+v %+v", got, f.Folders)
	}
}

func TestBackupV1OnFirstSave(t *testing.T) {
	dir := testkit.TempDir(t)
	path := filepath.Join(dir, "old.salbum")
	orig := golden(t, dir)
	os.WriteFile(path, orig, 0o644)
	p, err := salbum.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	e := edtest.Open(t, p, path)
	if err := e.Save(""); err != nil {
		t.Fatal(err)
	}
	bak := path + ".v1.bak"
	if got, _ := os.ReadFile(bak); !bytes.Equal(got, orig) {
		t.Fatal("в .v1.bak не исходные байты")
	}
	if fileJSON(t, path).Version != model.FormatVersion {
		t.Fatal("файл не записан в текущей версии")
	}
	// Второе сохранение копию не трогает.
	os.WriteFile(bak, []byte("метка"), 0o644)
	edtest.NewSpread(e, -1, edtest.Photos("x"))
	if err := e.Save(""); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(bak); string(got) != "метка" {
		t.Fatal("копия перезаписана")
	}
	// «Сохранить как» копию не делает; новый проект — тоже.
	other := filepath.Join(dir, "v1b.salbum")
	os.WriteFile(other, orig, 0o644)
	p, _ = salbum.Load(other)
	e = edtest.Open(t, p, other)
	if err := e.Save(filepath.Join(dir, "new.salbum")); err != nil {
		t.Fatal(err)
	}
	if err := edtest.Open(t, model.New(), "").Save(filepath.Join(dir, "fresh.salbum")); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{other + ".v1.bak", filepath.Join(dir, "new.salbum.v1.bak"), filepath.Join(dir, "fresh.salbum.v1.bak")} {
		if _, err := os.Stat(f); err == nil {
			t.Fatalf("лишняя копия %s", f)
		}
	}
}

func TestLoadKeepsAltWhenMissing(t *testing.T) {
	dir := testkit.TempDir(t)
	abs := filepath.Join(testkit.TempDir(t), "нет", "a.jpg")
	path := writeV2(t, dir, salbum.FilePhoto{PhotoRef: model.PhotoRef{Path: "Отбор/a.jpg", W: 3, H: 2, Size: 77}, Abs: abs})
	p, err := salbum.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if r := onlyRef(t, p); r.Path != filepath.Join(dir, "Отбор", "a.jpg") || r.Alt != abs || r.Size != 77 {
		t.Fatalf("ненайденное фото: %+v", r)
	}
	if err := edtest.Open(t, p, path).Save(""); err != nil {
		t.Fatal(err)
	}
	for _, got := range fileJSON(t, path).Photos {
		if got.Path != "Отбор/a.jpg" || got.Abs != abs || got.Alt != "" {
			t.Fatalf("после сохранения: %+v", got)
		}
	}
}

func FuzzLoad(f *testing.F) {
	f.Add(golden(f, "C:/Съёмки"))
	f.Add([]byte(`{"version":2,"folders":[{"path":"Отбор","abs":"C:\\a\\Отбор"},"x"],"settings":{"pageW":1e308,"bg":"#ABCDEF"},
"spreads":[{"id":"s","photos":["p","p","q"],"bgL":"red","texts":[null,{"id":"zz","x":-5,"y":1e9,"w":0,"text":"\u202eабв\r\nг\u0000","style":{"font":"../x","size":-1,"color":"#FFF","align":"justify","bold":true}}]}],
"photos":{"p":{"path":"Отбор/a.jpg","abs":"C:\\a\\a.jpg","w":-3,"h":2,"size":-1,"frame":{"s":99,"x":-7,"y":0.33333},"caption":{"text":"  ","style":{}}}}}`))
	f.Add([]byte(`{"version":2,"photos":{"p":{"path":"../../../x"}}}`))
	f.Add([]byte(`{"version":3}`))
	f.Add([]byte(`{"version":4}`))
	f.Add([]byte(`[]`))
	// Сетка: null, повторы ID и фото, фото только в месте, вне разворота, огромные числа.
	f.Add([]byte(`{"version":3,"settings":{"pageW":50,"pageH":1000},"spreads":[{"id":"s","photos":["p","q"],"layout":"x",
"grid":[null,{"id":"c0ffee000001","x":-1e308,"y":1e308,"w":0,"h":-5,"photo":"p","fit":"cover"},{"id":"c0ffee000001","photo":"p"},{"photo":"r"},{"photo":"нет"}]},
{"id":"s","photos":["r"],"grid":[]},{"id":"t","grid":[{"x":1,"y":1,"w":1,"h":1,"photo":"q","fit":"contain"}]}],
"photos":{"p":{"path":"Отбор/a.jpg"},"q":{"path":"Отбор/b.jpg","w":1,"h":1e9},"r":{"path":"Отбор/a.jpg"}}}`))
	f.Add([]byte(`{"version":3,"spreads":[{"id":"s","grid":[null]},null]}`))
	// 008: огромный угол, название с bidi, переводами строк и 10 000 знаков, «по тексту» у пустого и у края.
	f.Add([]byte(`{"version":3,"spreads":[{"id":"s","photos":[],"name":"\u202eСборы\n\t` + strings.Repeat("я", 10000) + `",
"texts":[{"id":"0123456789ab","x":590,"y":-5,"w":0,"rot":1e308,"auto":true,"text":"Текст","style":{}},
{"id":"0123456789ac","x":-1e9,"y":1e9,"w":1e9,"rot":-540,"text":"широкий","style":{}},
{"id":"0123456789ad","auto":true,"text":"","style":{}}]}]}`))
	f.Add([]byte(`{"version":3,"spreads":[{"id":"s","texts":[{"rot":"NaN","text":"x"}]}]}`))
	// Гигантские x/y/w с поворотом: раньше давали Inf, и View не сериализовался.
	f.Add([]byte(`{"version":3,"spreads":[{"id":"s","texts":[{"x":1.7e308,"y":1.7e308,"w":1.7e308,"rot":45,"auto":true,"text":"т"}]}]}`))
	// 009: фон-фото с гигантскими числами, повторами ID, чужими и нелепыми фото (1×2e9, отрицательные размеры).
	f.Add([]byte(`{"version":4,"settings":{"pageW":50,"pageH":1000},"spreads":[{"id":"s","photos":["p"],
"bgPhoto":{"photo":"p","x":1e308,"y":-1e308,"w":1e308,"h":1e-308,"rot":1e308,"frame":{"s":1e308,"x":-1e308,"y":5},"flipH":true},
"bgPhotoL":{"photo":"q","x":0,"y":0,"w":-1,"h":0},"bgPhotoR":{"photo":"нет"}},
{"id":"s","bgPhotoR":{"photo":"q","w":1e-300,"h":1e300,"rot":-180}},{"id":"t","photos":["p"],"bgPhotoL":{"photo":"p","w":0.5,"h":0.5}}],
"photos":{"p":{"path":"Отбор/a.jpg","w":1,"h":2000000000},"q":{"path":"Отбор/b.jpg","w":-5,"h":0,"flipV":true,"frame":{"s":3}}}}`))
	f.Add([]byte(`{"version":4,"spreads":[{"id":"s","bgPhoto":{"photo":"p","w":10,"h":10}},{"id":"s","bgPhoto":null}],"photos":{"p":{"path":"a.jpg"}}}`))
	// 004: свои отметки — UNC, три «..», 9★, чужая метка, повтор путей, пустая запись, null.
	f.Add([]byte(`{"version":5,"folders":["Отбор"],"marks":[{"path":"Отбор/1.jpg","abs":"C:\\a\\1.jpg","rating":4,"label":"red","flag":1},
{"path":"отбор/1.JPG","rating":1},{"path":"../../../x.jpg","rating":2},{"path":"x.jpg","abs":"\\\\srv\\s\\x.jpg","flag":-1},
{"path":"C:\\b\\..\\c\\2.jpg","rating":9,"label":"pink","flag":2},{"path":"3.jpg"},null,{"path":"Отбор/4.jpg","label":"other","rating":-1}]}`))
	f.Add([]byte(`{"version":5,"marks":{"path":"a.jpg"}}`))
	f.Add([]byte(`{"version":5,"folders":["a","a",{"path":5}],"marks":[{"path":"1.jpg","rating":"5"},{"path":5},3.5,{"path":"2.jpg","rating":2}]}`))
	// 032: обложка — чужой ID листа, nil-места, повторы, огромные числа, UNC разметки, null.
	f.Add([]byte(`{"version":6,"spreads":[{"id":"cover","photos":["p"]}],"cover":{"sheet":{"id":"x","photos":["p","q"],"name":"x","layout":"y",
"grid":[null,{"id":"c0ffee000001","x":1e308,"w":-1,"h":1e9,"photo":"p"},{"id":"c0ffee000001","photo":"q"}],"texts":[{"x":1e308,"text":"т","rot":1e308}],
"bgPhoto":{"photo":"p","w":1e308,"h":1},"bgPhotoR":{"photo":"q"}},"spec":{"preset":"x","w":1e308,"wrap":-1,"steps":[{"upto":5,"mm":1e308},{"upto":5},{"upto":-1}]},
"layout":"x","spineBg":"#FFF","guide":"\\\\srv\\s\\a.png","guideAbs":"C:\\a\\b.jpg"},"photos":{"p":{"path":"a.jpg"},"q":{"path":"b.jpg"}}}`))
	f.Add([]byte(`{"version":6,"cover":{"sheet":null,"spec":null,"guide":"../a.png"}}`))
	// v6 (030–032, аудит 0.10): шаблон листа с полями при длинных переменных, имя и подпись фото,
	// виньетка с чужими ролями и числами, подписи на обложке у линии обреза.
	long := strings.Repeat("я", 200)
	f.Add([]byte(`{"version":6,"vars":{"school":"` + long + `","year":"` + long + `","class":"\n","teacher":"\u0000x"},
"spreads":[{"id":"s","photos":["p","q"],"caps":{"style":{"size":1e9,"font":"../x"},"pad":-5,"wrap":true,"text":"` + strings.Repeat("{Год}", 80) + `"},
"grid":[{"id":"c0ffee000001","x":1,"y":1,"w":50,"h":50,"photo":"p"},{"id":"c0ffee000002","x":60,"y":1,"w":50,"h":50},{"id":"c0ffee000003","x":1,"y":60,"w":5,"h":5}]}],
"photos":{"p":{"path":"Иванов Пётр.jpg","person":{"last":"` + long + `","first":"\n","middle":"\t"}},"q":{"path":"b.jpg","caption":{"text":"{Отчество}{Школа}","pad":1e308}}}}`))
	f.Add([]byte(`{"version":6,"spreads":[{"id":"v","kind":"vignette","vig":{"tpl":"x","teachers":99,"pupils":-1,"ratio":"9:1","ttext":"{Фамилия}{Год}","head":"nope"},
"caps":{"text":"{Фамилия}"},"texts":[{"id":"0123456789ab","text":"{Школа}","style":{}}],
"grid":[{"id":"c0ffee000004","x":1,"y":1,"w":20,"h":30,"role":"teacher"},{"id":"c0ffee000005","role":"pupil"},{"role":"group"},{"role":"x"}]},
{"id":"w","kind":"calendar","vig":{"tpl":"grid"}},{"id":"x","vig":{"ttext":"т"}}]}`))
	f.Add([]byte(`{"version":6,"spreads":[{"id":"s"}],"vars":{"year":"2026"},"cover":{"sheet":{"id":"cover","caps":{"text":"{Год}"},
"grid":[{"id":"c0ffee000006","x":300,"y":250,"w":60,"h":50},{"id":"c0ffee000007","x":300,"y":320,"w":60,"h":50,"role":"teacher"}]},
"spec":{"preset":"fsfera-thin"},"layout":"front"}}`))
	// 051 (v9): ключи форм, лишний m, ключ длиннее layout.MaxKeyLen.
	f.Add([]byte(`{"version":9,"spreads":[{"id":"a","photos":["p","q","r"],"layout":"H(m0,1,m2)"},{"id":"b","photos":["s","t"],"layout":"QD[0|1]"},
{"id":"c","photos":["u","v"],"layout":"H(m0,m1)"},{"id":"d","photos":["w"],"layout":"` + strings.Repeat("H(", 1100) + `"}],
"photos":{"p":{"path":"a.jpg","w":3,"h":2},"q":{"path":"b.jpg","w":2,"h":3},"r":{"path":"c.jpg"},"s":{"path":"d.jpg","w":1,"h":1},
"t":{"path":"e.jpg","w":4,"h":1},"u":{"path":"f.jpg"},"v":{"path":"g.jpg"},"w":{"path":"h.jpg"}}}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		dir := testkit.TempDir(t)
		path := filepath.Join(dir, "f.salbum")
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
		p, err := salbum.Load(path)
		if err != nil {
			return
		}
		if err := edtest.GridInvariants(p); err != nil {
			t.Fatal(err)
		}
		if len(p.Folders) > model.MaxFolders {
			t.Fatalf("папок %d", len(p.Folders))
		}
		marks := edtest.Loaded(p)
		fuzzMarks(t, marks)
		if c := p.Cover; c != nil && (c.Sheet == nil || !reflect.DeepEqual(model.ClampCoverSpec(c.Spec), c.Spec) || !model.ValidCoverLayout(c.Layout)) {
			t.Fatalf("обложка: %+v", c)
		}
		for _, s := range p.Sheets() {
			if len(s.Photos) > model.MaxPhotos || len(s.Texts) > model.MaxTexts || len(s.Layout) > layout.MaxKeyLen {
				t.Fatalf("лимит: %d фото, %d текстов, ключ %d байт", len(s.Photos), len(s.Texts), len(s.Layout))
			}
			if utf8.RuneCountInString(s.Name) > model.MaxNameRunes || s.Name != model.CleanName(s.Name) {
				t.Fatalf("название: %q", s.Name)
			}
			for _, tb := range s.Texts {
				minW := model.MinTextW
				if tb.Auto {
					minW = 1
				}
				a := model.BoxOf(tb).AABB()
				if len([]rune(tb.Text)) > model.MaxTextRunes || tb.W < minW || a.X < -1e-6 || a.Y < -1e-6 || !model.IsTextID(tb.ID) ||
					tb.Rot != geom.NormDeg(tb.Rot) {
					t.Fatalf("блок: %+v", tb)
				}
			}
		}
		for _, r := range p.Photos {
			if fr := r.Frame; fr != nil && (fr.S < 1 || fr.S > model.MaxScale || fr.X < -1 || fr.X > 1) {
				t.Fatalf("кадр: %+v", fr)
			}
		}
		// Load оставляет то, что пропустит CheckTotals: иначе открытый проект «замёрз» бы (79).
		if err := p.CheckTotals(p); err != nil {
			t.Fatalf("после Load запас превышен: %v", err)
		}
		e := edtest.Open(t, p, "")
		v := e.View()
		if _, err := json.Marshal(v); err != nil {
			t.Fatalf("View не сериализуется: %v", err)
		}
		fuzzCaptions(t, v)
		if _, pv := e.Snapshot(); pv.Spreads == nil {
			t.Fatal("вид печати пуст")
		} else if _, err := json.Marshal(pv); err != nil {
			t.Fatalf("вид печати не сериализуется: %v", err)
		}
		_ = p.Clone() // не паникует: NaN после Load не бывает
		if err := salbum.SaveWith(p, path, marks); err != nil {
			t.Fatal(err)
		}
		q, err := salbum.Load(path)
		if err != nil {
			t.Fatalf("свой же файл не открылся: %v", err)
		}
		if len(edtest.Loaded(q)) != len(marks) || len(marks) > 0 && !reflect.DeepEqual(edtest.Loaded(q), marks) {
			t.Fatalf("отметки после записи и чтения:\n%+v\nждали\n%+v", edtest.Loaded(q), marks)
		}
		if err := edtest.GridInvariants(q); err != nil {
			t.Fatalf("после записи и чтения: %v", err)
		}
	})
}

// fuzzCaptions — подписи и заглушки вида (с обложкой) не длиннее подписи: шаблон листа
// размножается по местам, раскрытие режется (decisions 79).
func fuzzCaptions(t *testing.T, v model.View) {
	t.Helper()
	sheets := v.Spreads
	if v.Cover != nil {
		sheets = append(slices.Clone(sheets), v.Cover.Sheet)
	}
	for _, sv := range sheets {
		for _, c := range sv.Captions {
			if n := utf8.RuneCountInString(c.Text); n > model.MaxCaptionRunes {
				t.Fatalf("подпись %d знаков", n)
			}
		}
		for _, s := range sv.Stubs {
			if n := utf8.RuneCountInString(s.Text); n > model.MaxCaptionRunes {
				t.Fatalf("заглушка %d знаков", n)
			}
		}
	}
}

// golden — testdata/v1.salbum.json (файл v1; .salbum в git не пускает хук) с подставленной папкой (пути v1 абсолютные).
func golden(t testing.TB, dir string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "testdata", "v1.salbum.json"))
	if err != nil {
		t.Fatal(err)
	}
	q, _ := json.Marshal(filepath.ToSlash(dir))
	return bytes.ReplaceAll(data, []byte("{{DIR}}"), bytes.Trim(q, `"`))
}

// fileJSON — сохранённый файл как дерево JSON для проверок.
func fileJSON(t *testing.T, path string) salbum.FileProject {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var f salbum.FileProject
	if err := json.Unmarshal(data, &f); err != nil {
		t.Fatal(err)
	}
	return f
}

// onePhoto — проект с одним фото по пути path в одном развороте.
func onePhoto(path string) (*model.Project, string) {
	p := model.New()
	id := library.IDFor(path)
	p.Photos[id] = model.PhotoRef{Path: path, W: 300, H: 200}
	p.Spreads = []*model.Spread{{ID: "s1", Photos: []string{id}}}
	return p, id
}

// writeV2 пишет файл v2 с одним фото (path/abs как в файле) и возвращает путь.
func writeV2(t *testing.T, dir string, ph salbum.FilePhoto) string {
	t.Helper()
	f := salbum.FileProject{Version: 2, Settings: model.DefaultSettings(), Folders: []salbum.FileFolder{},
		Spreads: []*model.Spread{{ID: "s1", Photos: []string{"p"}}}, Photos: map[string]salbum.FilePhoto{"p": ph}}
	data, _ := json.Marshal(f)
	path := filepath.Join(dir, "a.salbum")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func onlyRef(t *testing.T, p *model.Project) model.PhotoRef {
	t.Helper()
	if len(p.Photos) != 1 {
		t.Fatalf("фото: %+v", p.Photos)
	}
	for _, r := range p.Photos {
		return r
	}
	return model.PhotoRef{}
}

// gridFile — файл v3 с фото по путям dir/<имя>.jpg (ID = IDFor) и разворотами.
func gridFile(t *testing.T, dir string, names []string, spreads []*model.Spread) ([]byte, map[string]string) {
	t.Helper()
	ids := map[string]string{}
	f := salbum.FileProject{Version: model.FormatVersion, Settings: model.DefaultSettings(), Folders: []salbum.FileFolder{},
		Spreads: spreads, Photos: map[string]salbum.FilePhoto{}}
	for _, n := range names {
		path := filepath.Join(dir, n+".jpg")
		ids[n] = library.IDFor(path)
		f.Photos[ids[n]] = salbum.FilePhoto{PhotoRef: model.PhotoRef{Path: path, W: 300, H: 200}}
	}
	data, err := json.Marshal(f)
	if err != nil {
		t.Fatal(err)
	}
	return data, ids
}

func box(id string, x, y, w, h float64, photo string) *model.Place {
	return &model.Place{ID: id, X: x, Y: y, W: w, H: h, Photo: photo}
}

// Перепривязка фото, стоящего только на фоне, не добавляет его папку и не двигает папки проекта.
func TestBackdropRelink(t *testing.T) {
	edtest.SkipNotWindows(t)
	s := edtest.NewRelinkSetup(t)
	other := filepath.Join(filepath.Dir(s.NewFd), "Своя")
	oldBg := filepath.Join(filepath.Dir(s.OldFolder), "Чужая", "fon.jpg")
	newBg := filepath.Join(other, "fon.jpg")
	edtest.PutFile(t, newBg, 20)
	idBg := library.IDFor(oldBg)
	if _, err := s.E.SetBackdrop(s.E.View().Spreads[0].ID, "spread", model.PhotoInfo{ID: idBg, Path: oldBg, W: 300, H: 200, Size: 20}); err != nil {
		t.Fatal(err)
	}
	v := s.E.Relink(map[string]string{idBg: newBg}, other)
	if !slices.Equal(v.Folders, []string{s.OldFolder}) {
		t.Fatalf("папки изменились из-за фото фона: %v", v.Folders)
	}
	nid := library.IDFor(newBg)
	if b := v.Spreads[0].Backdrops; len(b) != 1 || b[0].Photo != nid {
		t.Fatalf("ID фона после перепривязки: %+v", b)
	}
	if r := s.E.Refs()[nid]; r.Path != newBg {
		t.Fatalf("путь: %+v", r)
	}
}

func TestPathsRejected(t *testing.T) {
	e := edtest.Open(t, model.New(), "")
	rev := e.View().Rev
	if v, err := e.AddFolder(`\\host\share`); v.Rev != rev || err == nil {
		t.Fatal("AddFolder с UNC должен отказать")
	}
	p := model.New()
	p.Folders = []string{`\\host\share`}
	path := filepath.Join(testkit.TempDir(t), "a.salbum")
	if err := salbum.Save(p, path); err != nil {
		t.Fatal(err)
	}
	if _, err := salbum.Load(path); err == nil || !strings.Contains(err.Error(), "папка") {
		t.Fatalf("Load с UNC-папкой: %v", err)
	}
	// Относительный путь выше двух уровней (в памяти таких нет, encode пишет как есть).
	p = model.New()
	p.Photos["a"] = model.PhotoRef{Path: "../../../rel.jpg", W: 1, H: 1}
	p.Spreads = []*model.Spread{{ID: "s", Photos: []string{"a"}}}
	if err := salbum.Save(p, path); err != nil {
		t.Fatal(err)
	}
	if _, err := salbum.Load(path); err == nil || !strings.Contains(err.Error(), "фото") {
		t.Fatalf("Load с относительным путём фото: %v", err)
	}
}
