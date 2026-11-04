package salbum

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Подборки фото в файле (083, design §2.4).

const (
	binMust = "b3f9a01c2"
	binPar  = "b07d5e6aa"
)

// binsSide — две подборки и три отметки в dir: с оценкой и двумя подборками, только с подборкой,
// без подборок.
func binsSide(dir string) model.Side {
	r := 4
	a, b, c := filepath.Join(dir, "a.jpg"), filepath.Join(dir, "b.jpg"), filepath.Join(dir, "c.jpg")
	return model.Side{
		Bins: []model.Bin{{ID: binMust, Name: "Обязательно"}, {ID: binPar, Name: "Для родителей"}},
		Marks: map[string]model.Mark{
			library.IDFor(a): {Path: a, Rating: &r, Bins: []string{binPar, binMust}},
			library.IDFor(b): {Path: b, Bins: []string{binMust}},
			library.IDFor(c): {Path: c, Rating: &r},
		},
	}
}

// binsProject — проект seatProject с подборками binsSide.
func binsProject(dir string) *model.Project {
	p := seatProject(dir)
	p.SetLoaded(binsSide(dir))
	return p
}

// Круг Encode → Decode и Save → Load: подборки в порядке создания, членство на месте; вторая
// запись — те же байты.
func TestBinsRoundtrip(t *testing.T) {
	dir := testkit.TempDir(t)
	p := binsProject(dir)
	data, err := Encode(p, dir)
	if err != nil {
		t.Fatal(err)
	}
	var f FileProject
	if err := json.Unmarshal(data, &f); err != nil {
		t.Fatal(err)
	}
	if f.Version != model.FormatVersion || !slices.Equal([]model.Bin(f.Bins), p.Loaded().Bins) {
		t.Fatalf("в файле версия %d, подборки %v", f.Version, f.Bins)
	}
	if n := bytes.Count(data, []byte(`"bins"`)); n != 3 { // корень и две отметки
		t.Fatalf(`"bins" в файле %d раз:\n%s`, n, data)
	}
	q, err := Decode(data, dir)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(q.Loaded(), p.Loaded()) {
		t.Fatalf("после чтения:\n%+v\nждали\n%+v", q.Loaded(), p.Loaded())
	}
	path := filepath.Join(dir, "a.salbum")
	if err := Save(q, path); err != nil {
		t.Fatal(err)
	}
	r, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(r.Loaded(), p.Loaded()) {
		t.Fatalf("после Save → Load: %+v", r.Loaded())
	}
	again, err := Encode(r, dir)
	if err != nil || !bytes.Equal(again, data) {
		t.Fatalf("вторая запись отличается (%v):\n%s", err, again)
	}
}

// Без подборок файл прежний, кроме номера версии: ни ключа bins, Decode файла v14 и запись —
// те же байты. Членство в подборке, которой нет в списке, не пишется (страховка записи).
func TestNoBinsSameFile(t *testing.T) {
	dir := testkit.TempDir(t)
	side := binsSide(dir)
	side.Bins = nil
	p := seatProject(dir)
	p.SetLoaded(side)
	data, err := Encode(p, dir)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte(`"bins"`)) {
		t.Fatalf("файл без подборок с ключом bins:\n%s", data)
	}
	if n := len(fileOf(t, data).Marks); n != 2 { // отметка только с подборкой без подборки — пустая
		t.Fatalf("отметок в файле %d", n)
	}
	old := bytes.Replace(data, fmt.Appendf(nil, `"version": %d,`, model.FormatVersion), []byte(`"version": 14,`), 1)
	if bytes.Equal(old, data) {
		t.Fatal("заготовка v14 без номера версии")
	}
	q, err := Decode(old, dir)
	if err != nil {
		t.Fatal(err)
	}
	if q.From() != 14 || q.Version != model.FormatVersion || q.Loaded().Bins != nil {
		t.Fatalf("v14: from %d, версия %d, подборки %v", q.From(), q.Version, q.Loaded().Bins)
	}
	if out, err := Encode(q, dir); err != nil || !bytes.Equal(out, data) {
		t.Fatalf("v14 → запись отличается не только версией (%v):\n%s", err, out)
	}
}

func fileOf(t *testing.T, data []byte) FileProject {
	t.Helper()
	var f FileProject
	if err := json.Unmarshal(data, &f); err != nil {
		t.Fatal(err)
	}
	return f
}

// captureLog — вывод log на время теста.
func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(prev) })
	return &buf
}

// Зажим при чтении: плохой и повторный ID, сверх 50, имя длиннее 40, пустое и повторное имя,
// членство в неизвестной подборке, bins не списком; файл открывается, в log.txt — число без имён.
func TestLoadBinsClamp(t *testing.T) {
	dir := testkit.TempDir(t)
	long := strings.Repeat("Ж", model.MaxBinName+10)
	list := []string{
		`{"id":"b1","name":"Тайное имя"}`, `{"id":"b1","name":"Повтор"}`, `{"id":"B!","name":"x"}`,
		`{"id":"b2","name":"тайное имя"}`, `{"id":"b3","name":""}`, `{"id":"b4","name":` + jsonStr(long) + `}`,
		`null`, `"b5"`, `{"id":7}`, `{"id":"b6","name":["x"]}`,
	}
	for i := range model.MaxBins + 5 {
		list = append(list, fmt.Sprintf(`{"id":"m%d","name":"М %d"}`, i, i))
	}
	marks := `[{"path":"a.jpg","rating":2,"bins":["m0","b1","nope","b1","B!"]},{"path":"b.jpg","bins":["nope"]},` +
		`{"path":"c.jpg","bins":"b1","flag":1},{"path":"d.jpg","bins":["m49","m50"]}]`
	logs := captureLog(t)
	p, err := Decode([]byte(`{"version":15,"bins":[`+strings.Join(list, ",")+`],"marks":`+marks+`}`), dir)
	if err != nil {
		t.Fatal(err)
	}
	bins := p.Loaded().Bins
	if len(bins) != model.MaxBins {
		t.Fatalf("подборок %d", len(bins))
	}
	head := []model.Bin{{ID: "b1", Name: "Тайное имя"}, {ID: "b2", Name: "тайное имя 2"}, {ID: "b3", Name: "Подборка 1"},
		{ID: "b4", Name: strings.Repeat("Ж", model.MaxBinName)}}
	if !slices.Equal(bins[:4], head) || bins[4].ID != "m0" || bins[len(bins)-1].ID != fmt.Sprintf("m%d", model.MaxBins-5) {
		t.Fatalf("подборки: %v … %v", bins[:5], bins[len(bins)-1])
	}
	m := p.Loaded().Marks
	id := func(name string) string { return library.IDFor(filepath.Join(dir, name)) }
	if got := m[id("a.jpg")].Bins; !slices.Equal(got, []string{"b1", "m0"}) {
		t.Fatalf("членство a: %v", got)
	}
	if _, ok := m[id("b.jpg")]; ok {
		t.Fatal("отметка только с неизвестной подборкой осталась")
	}
	if c := m[id("c.jpg")]; c.Flag == nil || c.Bins != nil {
		t.Fatalf("bins не списком у отметки: %+v", c)
	}
	if d, ok := m[id("d.jpg")]; ok { // m49, m50 — за пределом 50, отброшены хвостом
		t.Fatalf("членство в отброшенных хвостом: %+v", d)
	}
	out := logs.String()
	if !strings.Contains(out, "подборки: отброшено или поправлено записей") || strings.Contains(out, "Тайное") || strings.Contains(out, "Ж") {
		t.Fatalf("лог:\n%s", out)
	}
	// bins не списком — подборок нет, отметки целы, файл открывается.
	for _, raw := range []string{`{"id":"b1"}`, `"b1"`, `5`, `null`} {
		q, err := Decode([]byte(`{"version":15,"bins":`+raw+`,"marks":[{"path":"a.jpg","rating":3,"bins":["b1"]}]}`), dir)
		if err != nil {
			t.Fatalf("%s: %v", raw, err)
		}
		if a := q.Loaded().Marks[id("a.jpg")]; q.Loaded().Bins != nil || a.Rating == nil || a.Bins != nil {
			t.Fatalf("%s: подборки %v, отметка %+v", raw, q.Loaded().Bins, a)
		}
	}
}

func jsonStr(s string) string { b, _ := json.Marshal(s); return string(b) }

// Фото только в подборке (без оценки, метки, флажка) — своя отметка: не теряется ни при чтении,
// ни при записи; запись, где подборок не осталось, — пустая и уходит.
func TestBinOnlyMarkKept(t *testing.T) {
	dir := testkit.TempDir(t)
	data := `{"version":15,"bins":[{"id":"b1","name":"Запас"}],"marks":[{"path":"a.jpg","bins":["b1"]},` +
		`{"path":"b.jpg","bins":["b9"]},{"path":"c.jpg","bins":[]}]}`
	p, err := Decode([]byte(data), dir)
	if err != nil {
		t.Fatal(err)
	}
	m := p.Loaded().Marks
	a, ok := m[library.IDFor(filepath.Join(dir, "a.jpg"))]
	if !ok || len(m) != 1 || !slices.Equal(a.Bins, []string{"b1"}) || a.Rating != nil {
		t.Fatalf("отметки: %+v", m)
	}
	path := filepath.Join(dir, "a.salbum")
	if err := Save(p, path); err != nil {
		t.Fatal(err)
	}
	f := fileJSON(t, path)
	if len(f.Marks) != 1 || f.Marks[0].Path != "a.jpg" || !slices.Equal(f.Marks[0].Bins, []string{"b1"}) || f.Marks[0].Rating != nil {
		t.Fatalf("в файле: %+v", f.Marks)
	}
}
