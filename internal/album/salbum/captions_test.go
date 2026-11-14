package salbum

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Поля подписей 030 в формате v6 (все omitempty): круг записи без потерь, без полей —
// в файле их нет.
func TestSalbumCaptionFieldsRoundTrip(t *testing.T) {
	dir := testkit.TempDir(t)
	mk := func(name string) (string, string) {
		path := filepath.Join(dir, "Отбор", name)
		putFile(t, path, 10)
		return path, library.IDFor(path)
	}
	pa, a := mk("Иванов Петя.jpg")
	pb, b := mk("b.jpg")
	pad := 4.5
	p := model.New()
	p.Folders = []string{filepath.Join(dir, "Отбор")}
	vars := &model.Vars{Title: "Школа № 5", Subtitle: "11 «А»", Year: "2026", Lead: "Мария Ивановна"}
	p.Photos[a] = model.PhotoRef{Path: pa, W: 300, H: 200, Size: 10,
		Caption: &model.Caption{Text: "{Фамилия}", Style: model.DefaultCaptionStyle(), Pad: &pad}}
	p.Photos[b] = model.PhotoRef{Path: pb, W: 300, H: 200, Size: 10, Person: &model.Person{Last: "Петрова", First: "Анна", Middle: "Сергеевна"}}
	p.Spreads = []*model.Spread{
		{ID: "s1", Photos: []string{a, b}, Vars: vars, Caps: &model.SheetCaps{Style: model.DefaultCaptionStyle(), Pad: 2, Over: true, Text: "{Фамилия}\n{Имя}"}},
		{ID: "s2", Caps: &model.SheetCaps{Style: model.DefaultCaptionStyle(), Wrap: true}}, // пустой лист со стилем живёт
	}
	path := filepath.Join(dir, "a.salbum")
	if err := Save(p, path); err != nil {
		t.Fatal(err)
	}
	if f := fileJSON(t, path); f.Version != model.FormatVersion { // 030 номер формата не меняет
		t.Fatalf("версия %d", f.Version)
	}
	data, _ := os.ReadFile(path)
	for _, key := range []string{`"vars"`, `"person"`, `"pad": 4.5`, `"caps"`} {
		if !bytes.Contains(data, []byte(key)) {
			t.Fatalf("в файле нет %s:\n%s", key, data)
		}
	}
	q, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if x, y := model.Marshal(p), model.Marshal(q); !bytes.Equal(x, y) {
		t.Fatalf("потери при записи:\n%s\n%s", x, y)
	}

	// Проект без новых полей пишет файл без них.
	plain, _ := onePhoto(pa)
	plain.Photos[a] = model.PhotoRef{Path: pa, W: 300, H: 200, Caption: &model.Caption{Text: "т", Style: model.DefaultCaptionStyle()}}
	if err := Save(plain, path); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(path)
	for _, key := range []string{`"vars"`, `"person"`, `"pad"`, `"caps"`} {
		if bytes.Contains(data, []byte(key)) {
			t.Fatalf("пустое поле %s записано:\n%s", key, data)
		}
	}
}

// 049: предмет учителя (`person.subj`, v8, omitempty) — круг Save→Load без потерь у
// нескольких фото сразу (с именем, только предмет, без предмета), повторная запись — байт в
// байт; файл без предметов ключа не пишет и после Load→Save тот же; Load зажимает мусор.
func TestSalbumPersonSubjectRoundTrip(t *testing.T) {
	dir := testkit.TempDir(t)
	p := model.New()
	p.Folders = []string{filepath.Join(dir, "Отбор")}
	people := []*model.Person{
		{Last: "Сидорова", First: "Ольга", Middle: "Ивановна", Subject: "Математика"},
		{Subject: "Физика"}, // имя из названия файла, предмет свой
		{Last: "Козлов", First: "Иван"},
		nil,
	}
	var ids []string
	for i, who := range people {
		path := filepath.Join(dir, "Отбор", fmt.Sprintf("Петров Пётр %d.jpg", i))
		putFile(t, path, 10)
		id := library.IDFor(path)
		p.Photos[id] = model.PhotoRef{Path: path, W: 300, H: 200, Size: 10, Person: who}
		ids = append(ids, id)
	}
	p.Spreads = []*model.Spread{{ID: "s1", Photos: ids}}
	path := filepath.Join(dir, "a.salbum")
	if err := Save(p, path); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if f := fileJSON(t, path); f.Version != model.FormatVersion || bytes.Count(data, []byte(`"subj"`)) != 2 {
		t.Fatalf("версия %d, предметов в файле %d:\n%s", f.Version, bytes.Count(data, []byte(`"subj"`)), data)
	}
	q, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if x, y := model.Marshal(p), model.Marshal(q); !bytes.Equal(x, y) {
		t.Fatalf("потери при записи:\n%s\n%s", x, y)
	}
	if who, file := q.Photos[ids[1]].Name(); who == nil || !file || who.Last != "Петров" || who.Subject != "Физика" {
		t.Fatalf("предмет к имени из файла после Load: %+v %v", who, file)
	}
	again := filepath.Join(dir, "b.salbum")
	if err := Save(q, again); err != nil {
		t.Fatal(err)
	}
	if data2, _ := os.ReadFile(again); !bytes.Equal(data, data2) {
		t.Fatalf("повторная запись не байт в байт:\n%s\n%s", data, data2)
	}

	// Без предметов: ключа нет, Load→Save — тот же файл.
	for _, id := range ids {
		r := p.Photos[id]
		if r.Person != nil {
			who := *r.Person
			who.Subject = ""
			r.Person = model.ClampPerson(who)
		}
		p.Photos[id] = r
	}
	if err := Save(p, path); err != nil {
		t.Fatal(err)
	}
	plain, _ := os.ReadFile(path)
	q, err = Load(path)
	if err != nil || bytes.Contains(plain, []byte(`"subj"`)) {
		t.Fatalf("файл без предмета: %v\n%s", err, plain)
	}
	if err := Save(q, again); err != nil {
		t.Fatal(err)
	}
	if data2, _ := os.ReadFile(again); !bytes.Equal(plain, data2) {
		t.Fatalf("файл без предмета изменился:\n%s\n%s", plain, data2)
	}

	// Мусор: длинный и многострочный предмет — одна строка ≤ MaxPersonRunes, пробельный — снят.
	raw := bytes.Replace(data, []byte(`"subj": "Математика"`), []byte(`"subj": " Мате\nматика`+strings.Repeat("я", 100)+`"`), 1)
	raw = bytes.Replace(raw, []byte(`"subj": "Физика"`), []byte(`"subj": " \t "`), 1)
	if bytes.Equal(raw, data) {
		t.Fatal("подмена в файле не сработала")
	}
	q = mustDecode(t, raw, dir)
	if s := q.Photos[ids[0]].Person.Subject; utf8.RuneCountInString(s) != model.MaxPersonRunes || !strings.HasPrefix(s, "Мате матика") {
		t.Fatalf("зажим предмета: %q", s)
	}
	if q.Photos[ids[1]].Person != nil {
		t.Fatalf("пробельный предмет без имени не снят: %+v", q.Photos[ids[1]].Person)
	}
}

// Load зажимает мусор в полях 030: отступы в 0..20 (1 мм — nil), имена и переменные —
// одна строка в пределах, пустое — nil; число вне float64 отвергает файл без паники.
func TestLoadClampsCaptionFields(t *testing.T) {
	dir := testkit.TempDir(t)
	f := FileProject{Version: model.FormatVersion, Settings: model.DefaultSettings(), Folders: []FileFolder{},
		Photos: map[string]FilePhoto{}}
	pad := func(v float64) *float64 { return &v }
	ids := make([]string, 5)
	refs := []model.PhotoRef{
		{Caption: &model.Caption{Text: "а", Pad: pad(-5)}, Person: &model.Person{Last: strings.Repeat("я", 10000), First: "Петя\nПетров"}},
		{Caption: &model.Caption{Text: "б", Pad: pad(1e300)}, Person: &model.Person{Last: " \t ", First: "\u200f"}},
		{Caption: &model.Caption{Text: "в", Pad: pad(1)}},
		{Caption: &model.Caption{Text: "г", Pad: pad(7.77)}},
		{Caption: &model.Caption{Text: "  ", Pad: pad(3)}},
	}
	for i, r := range refs {
		r.Path, r.W, r.H = filepath.Join(dir, string(rune('a'+i))+".jpg"), 300, 200
		ids[i] = library.IDFor(r.Path)
		f.Photos[ids[i]] = FilePhoto{PhotoRef: r}
	}
	f.Spreads = []*model.Spread{
		{ID: "s1", Photos: ids, Vars: &model.Vars{Title: "Школа\r\n№ 5\u202e", Year: strings.Repeat("9", 500)}, Caps: &model.SheetCaps{Pad: -1, Text: "   ", Style: model.TextStyle{Font: "нет", Size: 1e9, Color: "red", Align: "x"}}},
		{ID: "s2", Caps: &model.SheetCaps{Pad: 300, Text: strings.Repeat("ш", 5000)}},
	}
	data, err := json.Marshal(f)
	if err != nil {
		t.Fatal(err)
	}
	p := mustDecode(t, data, dir)
	if v := p.Spreads[0].Vars; v == nil || v.Title != "Школа № 5" || utf8.RuneCountInString(v.Year) != model.MaxVarRunes || v.Subtitle != "" {
		t.Fatalf("поля шапки: %+v", p.Spreads[0].Vars)
	}
	r := func(i int) model.PhotoRef { return p.Photos[ids[i]] }
	if pr := r(0).Person; pr == nil || utf8.RuneCountInString(pr.Last) != model.MaxPersonRunes || pr.First != "Петя Петров" {
		t.Fatalf("имя: %+v", pr)
	}
	if r(1).Person != nil {
		t.Fatalf("пробельное имя не снято: %+v", r(1).Person)
	}
	pads := []any{*r(0).Caption.Pad, *r(1).Caption.Pad, r(2).Caption.Pad, *r(3).Caption.Pad, r(4).Caption}
	if pads[0] != 0.0 || pads[1] != model.MaxPad || pads[2] != (*float64)(nil) || pads[3] != 7.8 || pads[4] != (*model.Caption)(nil) {
		t.Fatalf("отступы: %v", pads)
	}
	if len(p.Spreads) != 2 {
		t.Fatalf("разворотов %d: лист только со стилем подписей не пустой", len(p.Spreads))
	}
	c0, c1 := p.Spreads[0].Caps, p.Spreads[1].Caps
	if c0 == nil || c0.Pad != 0 || c0.Text != "" || c0.Style != model.ClampStyle(c0.Style) || c0.Style.Align != "center" {
		t.Fatalf("стиль листа: %+v", c0)
	}
	if c1 == nil || c1.Pad != model.MaxPad || utf8.RuneCountInString(c1.Text) != model.MaxCaptionRunes {
		t.Fatalf("стиль листа 2: pad %v, знаков %d", c1.Pad, utf8.RuneCountInString(c1.Text))
	}

	// NaN в JSON не бывает, а 1e309 encoding/json не разбирает: файл отвергается целиком.
	big := bytes.Replace(data, []byte(`"pad":-5`), []byte(`"pad":1e309`), 1)
	if bytes.Equal(big, data) {
		t.Fatal("подмена pad не нашла поле")
	}
	if _, err := Decode(big, dir); err == nil {
		t.Fatal("1e309 в pad принят")
	}
}

// withAlbumVars — файл проекта p с полями альбома dev-сборки v6 (030) на верхнем уровне.
func withAlbumVars(t *testing.T, p *model.Project, dir string, old albumVars) []byte {
	t.Helper()
	data, err := Encode(p, dir)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	if m["vars"], err = json.Marshal(old); err != nil {
		t.Fatal(err)
	}
	if data, err = json.Marshal(m); err != nil {
		t.Fatal(err)
	}
	return data
}

func textSheet(s string) *model.Spread {
	return &model.Spread{ID: model.NewID(), Photos: []string{},
		Texts: []*model.TextBlock{{ID: model.NewID(), X: 10, Y: 10, W: 50, Text: s, Style: model.PlainStyle()}}}
}

// TestLoadMovesAlbumVars — поля альбома из файла dev-сборки v6 Load переносит в листы
// (решение 125): с виньетками — в каждую виньетку без своих, без виньеток — в листы и обложку,
// где в тексте есть поле шапки. Save верхний vars не пишет; Load → Save → Load — те же байты.
func TestLoadMovesAlbumVars(t *testing.T) {
	dir := testkit.TempDir(t)
	old := albumVars{School: "Гимназия № 1", Class: "11 «А» класс", Year: "2026", Teacher: "Иванова И. И."}
	want := model.Vars{Title: old.School, Subtitle: old.Class, Year: old.Year, Lead: old.Teacher}
	vig := func(v *model.Vars) *model.Spread {
		d := model.DefaultVignette()
		return &model.Spread{ID: model.NewID(), Photos: []string{}, Kind: model.KindVignette, Vig: &d, Vars: v}
	}
	roundTrip := func(t *testing.T, p *model.Project) {
		t.Helper()
		path := filepath.Join(dir, "a.salbum")
		first, q := saveLoad(t, p, path)
		if f := fileJSON(t, path); f.Vars != nil || !bytes.Contains(first, []byte(`"vars": {`)) {
			t.Fatalf("в файле верхний vars %+v или нет vars листа:\n%s", f.Vars, first)
		}
		second, _ := saveLoad(t, q, path)
		if !bytes.Equal(first, second) {
			t.Fatalf("Load → Save изменил файл:\n%s\n%s", first, second)
		}
	}

	t.Run("виньетки", func(t *testing.T) {
		p := model.New()
		own := &model.Vars{Title: "Свой"}
		p.Spreads = []*model.Spread{textSheet("{Школа}"), vig(nil), vig(own), vig(nil)}
		q := mustDecode(t, withAlbumVars(t, p, dir, old), dir)
		s := q.Spreads
		if s[1].Vars == nil || *s[1].Vars != want || s[3].Vars == nil || *s[3].Vars != want || s[1].Vars == s[3].Vars {
			t.Fatalf("виньетки без своих: %+v, %+v", s[1].Vars, s[3].Vars)
		}
		if s[2].Vars == nil || *s[2].Vars != *own {
			t.Fatalf("свои поля виньетки затёрты: %+v", s[2].Vars)
		}
		if s[0].Vars != nil || q.VarsOf(s[0]) != s[1].Vars {
			t.Fatalf("обычный разворот: свои %+v, по VarsOf %+v", s[0].Vars, q.VarsOf(s[0]))
		}
		roundTrip(t, q)
	})

	t.Run("без виньеток", func(t *testing.T) {
		ph := filepath.Join(dir, "Иванов Петя.jpg")
		id := library.IDFor(ph)
		pr, _ := model.CoverPresetOf(model.CoverDefault)
		p := model.New()
		p.Photos[id] = model.PhotoRef{Path: ph, W: 300, H: 200, Caption: &model.Caption{Text: "{Класс}", Style: model.DefaultCaptionStyle()}}
		capsSheet := &model.Spread{ID: model.NewID(), Photos: []string{}, Caps: &model.SheetCaps{Style: model.DefaultCaptionStyle(), Pad: 1, Text: "{Учитель}"}}
		photoSheet := &model.Spread{ID: model.NewID(), Photos: []string{id}}
		p.Spreads = []*model.Spread{textSheet("Выпуск {школа}"), capsSheet, photoSheet, textSheet("{Фамилия} {Цвет}"), textSheet("{Руководитель}")}
		p.Cover = &model.Cover{Spec: pr.Spec, Sheet: textSheet("{год}")}
		p.Cover.Sheet.ID = model.CoverID
		q := mustDecode(t, withAlbumVars(t, p, dir, old), dir)
		for i, s := range append([]*model.Spread{q.Cover.Sheet}, q.Spreads...) {
			got, wantOwn := s.Vars, i != 4 // 4 — лист без полей шапки
			if wantOwn != (got != nil) || got != nil && *got != want {
				t.Errorf("лист %d: %+v", i, got)
			}
		}
		roundTrip(t, q)
	})

	// Пустые поля альбома и файл без них — ничего не переносится.
	p := model.New()
	p.Spreads = []*model.Spread{textSheet("{Школа}"), vig(nil)}
	for _, v := range []albumVars{{}, {School: " "}} {
		q := mustDecode(t, withAlbumVars(t, p, dir, v), dir)
		if v.School == "" && (q.Spreads[0].Vars != nil || q.Spreads[1].Vars != nil) {
			t.Fatalf("пустые поля перенесены: %+v %+v", q.Spreads[0].Vars, q.Spreads[1].Vars)
		}
		if v.School != "" && q.Spreads[1].Vars != nil { // одни пробелы — зажим даёт nil
			t.Fatalf("пробельные поля остались: %+v", q.Spreads[1].Vars)
		}
	}
}

// TestLoadAlbumVarsHugeFast — поле альбома размером почти с файл и MaxSpreads виньеток:
// зажим один раз, а не в каждом листе (было 2 мин 12 с на 45 МБ и 1000 виньеток). Поле
// обрезано до MaxVarRunes и одно на все листы, листы не делят указатель.
func TestLoadAlbumVarsHugeFast(t *testing.T) {
	dir := testkit.TempDir(t)
	p := model.New()
	p.Spreads = make([]*model.Spread, model.MaxSpreads)
	for i := range p.Spreads {
		d := model.DefaultVignette()
		p.Spreads[i] = &model.Spread{ID: model.NewID(), Photos: []string{}, Kind: model.KindVignette, Vig: &d}
	}
	small := withAlbumVars(t, p, dir, albumVars{})
	// Под детектором гонок побайтная очистка 49 МБ идёт ~25× медленнее (19 с против 0,8 с),
	// и порог не удержать. Поле в 10 раз меньше (~4,5 МБ) укладывается, а квадратичный
	// регресс (1000 листов × 4,5 МБ) по-прежнему дал бы минуты.
	target := model.MaxFileSize
	if testkit.RaceSlowdown > 1 {
		target /= 10
	}
	huge := strings.Repeat("Ж ", (target-len(small)-1024)/3)
	data := withAlbumVars(t, p, dir, albumVars{School: huge})
	if len(data) > target || len(data) < target-64<<10 {
		t.Fatalf("файл %d байт, ждали около %d", len(data), target)
	}
	path := filepath.Join(dir, "a.salbum")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	q, err := Load(path)
	d := time.Since(start)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("открытие %d МБ, %d виньеток: %v", len(data)>>20, len(q.Spreads), d)
	// 10 с — запас против нагрузки на машине (чистый прогон 0,8–2,4 с); ловимый регресс
	// измеряется минутами, так что порог всё равно на порядок строже него.
	if lim := testkit.Limit(10 * time.Second); d > lim {
		t.Fatalf("открытие %v, предел %v", d, lim)
	}
	want := model.ClampVars(model.Vars{Title: huge})
	for i, s := range q.Spreads {
		if s.Vars == nil || *s.Vars != *want || i > 0 && s.Vars == q.Spreads[0].Vars {
			t.Fatalf("лист %d: поля %+v", i, s.Vars)
		}
	}
	if n := utf8.RuneCountInString(want.Title); n != model.MaxVarRunes-1 && n != model.MaxVarRunes {
		t.Fatalf("заголовок %d знаков", n)
	}
}

// Предел 64 фото и места на разворот при чтении файла (спека 027, формат v6).

// TestLoadPlaces64: 65 мест в файле → 64 (лишнее пустое с конца), все 64 фото на местах.
func TestLoadPlaces64(t *testing.T) {
	if model.MaxPlaces != 64 || model.MaxPhotos != 64 {
		t.Fatalf("MaxPlaces = %d, MaxPhotos = %d", model.MaxPlaces, model.MaxPhotos)
	}
	dir := testkit.TempDir(t)
	var names []string
	for i := range 64 {
		names = append(names, fmt.Sprint("g", i))
	}
	_, ids := gridFile(t, dir, names, nil)
	s := &model.Spread{ID: "s"}
	for i := range 65 {
		ph := ""
		if i < 64 {
			ph = ids[names[i]]
			s.Photos = append(s.Photos, ph)
		}
		s.Grid = append(s.Grid, box(model.NewID(), float64(i%8)*30, float64(i/8)*30, 20, 20, ph))
	}
	data, _ := gridFile(t, dir, names, []*model.Spread{s})
	p := mustDecode(t, data, dir)
	if len(p.Spreads) != 1 || len(p.Spreads[0].Grid) != 64 || len(model.GridPhotos(p.Spreads[0])) != 64 {
		t.Fatalf("мест %d, фото на местах %d", len(p.Spreads[0].Grid), len(model.GridPhotos(p.Spreads[0])))
	}
}

// TestLoadSplitBig64: разворот из 100 фото (чужой или битый файл) режется на 64 + 36.
func TestLoadSplitBig64(t *testing.T) {
	p, dir := model.New(), testkit.TempDir(t)
	ids := make([]string, 100)
	for i := range ids {
		path := filepath.Join(dir, fmt.Sprint("p", i, ".jpg"))
		ids[i] = library.IDFor(path)
		p.Photos[ids[i]] = model.PhotoRef{Path: path, W: 3, H: 2}
	}
	p.Spreads = []*model.Spread{{ID: "s", Photos: ids}}
	path := filepath.Join(dir, "a.salbum")
	if err := Save(p, path); err != nil {
		t.Fatal(err)
	}
	q, err := Load(path)
	if err != nil || len(q.Spreads) != 2 || len(q.Spreads[0].Photos) != 64 || len(q.Spreads[1].Photos) != 36 {
		t.Fatalf("ждали 64 + 36: err=%v, разворотов %d", err, len(q.Spreads))
	}
}
