package salbum

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Табличка места в файле (071, v13): grid[].person, omitzero.

var (
	seatPetya = model.Person{Last: "Иванов", First: "Петя"}
	seatOlga  = model.Person{Last: "Сидорова", First: "Ольга", Middle: "Ивановна", Subject: "Математика"}
	seatPhys  = model.Person{Subject: "Физика"}
)

// seatProject — два листа-сетки в dir: на первом место с фото и табличкой, пустое с табличкой
// и пустое без; на втором — место с фото без таблички и пустое место только с предметом.
func seatProject(dir string) *model.Project {
	p := model.New()
	var ids []string
	for _, name := range []string{"a.jpg", "b.jpg"} {
		path := filepath.Join(dir, name)
		ids = append(ids, library.IDFor(path))
		p.Photos[ids[len(ids)-1]] = model.PhotoRef{Path: path, W: 300, H: 200}
	}
	pl := func(id string, x float64, photo string, who model.Person) *model.Place {
		return &model.Place{ID: id, X: x, Y: 20, W: 80, H: 60, Photo: photo, Person: who}
	}
	p.Spreads = []*model.Spread{
		{ID: "s1", Photos: []string{ids[0]}, Grid: []*model.Place{
			pl("c0ffee000001", 20, ids[0], seatPetya), pl("c0ffee000002", 120, "", seatOlga), pl("c0ffee000003", 220, "", model.Person{})}},
		{ID: "s2", Photos: []string{ids[1]}, Grid: []*model.Place{
			pl("c0ffee000004", 20, ids[1], model.Person{}), pl("c0ffee000005", 120, "", seatPhys)}},
	}
	return p
}

// seats — таблички мест листов по порядку.
func seats(p *model.Project) []model.Person {
	var out []model.Person
	for _, s := range p.Spreads {
		for _, pl := range s.Grid {
			out = append(out, pl.Person)
		}
	}
	return out
}

func TestV13SeatNameRoundtrip(t *testing.T) {
	dir := testkit.TempDir(t)
	path := filepath.Join(dir, "a.salbum")
	first, q := saveLoad(t, seatProject(dir), path)
	if f := fileJSON(t, path); f.Version != 13 || bytes.Count(first, []byte(`"person"`)) != 3 {
		t.Fatalf("файл v%d, табличек в файле %d (ждали 3: ноль не пишется):\n%s", f.Version, bytes.Count(first, []byte(`"person"`)), first)
	}
	want := []model.Person{seatPetya, seatOlga, {}, {}, seatPhys}
	if got := seats(q); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("таблички после круга: %+v, ждали %+v", got, want)
	}
	if q.Photos[q.Spreads[0].Grid[0].Photo].Person != nil {
		t.Fatal("табличка места ушла в имя фото")
	}
	second, _ := saveLoad(t, q, path)
	if !bytes.Equal(first, second) {
		t.Fatalf("второй круг отличается:\n%s\n%s", first, second)
	}
}

// v12 (0.16–0.17) открывается без табличек; первая запись — v13, BackupOld кладёт .v12.bak с
// исходником. v14 — отказ: иначе автосохранение затёрло бы чужие поля.
func TestV12OpensWithoutSeatNames(t *testing.T) {
	dir := testkit.TempDir(t)
	p := seatProject(dir)
	for _, s := range p.Spreads {
		for _, pl := range s.Grid {
			pl.Person = model.Person{}
		}
	}
	data, err := Encode(p, dir)
	if err != nil {
		t.Fatal(err)
	}
	cur := fmt.Appendf(nil, `"version": %d,`, model.FormatVersion)
	src := bytes.Replace(data, cur, []byte(`"version": 12,`), 1)
	if bytes.Equal(src, data) || bytes.Contains(src, []byte(`"person"`)) {
		t.Fatal("заготовка v12: нет номера версии или есть табличка")
	}
	path := filepath.Join(dir, "old.salbum")
	if err := os.WriteFile(path, src, 0o644); err != nil {
		t.Fatal(err)
	}
	q, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if q.From() != 12 || q.Version != model.FormatVersion || len(q.Spreads) != 2 || len(q.Spreads[0].Grid) != 3 {
		t.Fatalf("from %d, версия %d, листов %d", q.From(), q.Version, len(q.Spreads))
	}
	for i, who := range seats(q) {
		if who != (model.Person{}) {
			t.Fatalf("место %d: табличка %+v в файле v12", i, who)
		}
	}
	if err := BackupOld(path, q.From()); err != nil {
		t.Fatal(err)
	}
	if err := Save(q, path); err != nil {
		t.Fatal(err)
	}
	if bak, err := os.ReadFile(path + ".v12.bak"); err != nil || !bytes.Equal(bak, src) {
		t.Fatalf(".v12.bak: %v", err)
	}
	if out, _ := os.ReadFile(path); bytes.Contains(out, []byte(`"person"`)) || fileJSON(t, path).Version != 13 {
		t.Fatalf("запись v12:\n%s", out)
	}
	newer := bytes.Replace(data, cur, []byte(`"version": 14,`), 1)
	if _, err := Decode(newer, dir); !errors.Is(err, ErrNewer) {
		t.Fatalf("v14: %v", err)
	}
}

// Мусор из файла в табличке зажимается при Load как имя фото (ClampPerson): 60 знаков на
// часть, одна строка; пустая после зажима — ноль, и в файл больше не пишется.
func TestLoadClampsSeatName(t *testing.T) {
	dir := testkit.TempDir(t)
	long := strings.Repeat("я", 200)
	file := []byte(`{"version":13,"settings":{"pageW":300,"pageH":300},"spreads":[{"id":"s1","photos":[],"grid":[
{"id":"c0ffee000001","x":20,"y":20,"w":80,"h":60,"person":{"last":"` + long + `","first":"Пе\nтя\u0000","middle":"\u202e","subj":"\t"}},
{"id":"c0ffee000002","x":120,"y":20,"w":80,"h":60,"person":{"last":"  ","first":"\r\n"}},
{"id":"c0ffee000003","x":220,"y":20,"w":80,"h":60,"person":{}}]}]}`)
	p := mustDecode(t, file, dir)
	g := p.Spreads[0].Grid
	if a := g[0].Person; len([]rune(a.Last)) != model.MaxPersonRunes || a.First != "Пе тя" || a.Middle != "" || a.Subject != "" {
		t.Errorf("длинная табличка: %+v", a)
	}
	if g[1].Person != (model.Person{}) || g[2].Person != (model.Person{}) {
		t.Errorf("пустые таблички: %+v, %+v", g[1].Person, g[2].Person)
	}
	out, err := Encode(p, dir)
	if err != nil {
		t.Fatal(err)
	}
	if n := bytes.Count(out, []byte(`"person"`)); n != 1 {
		t.Fatalf("табличек в файле %d, ждали 1:\n%s", n, out)
	}
	if _, err := Decode(bytes.Replace(file, []byte(`"person":{}`), []byte(`"person":"x"`), 1), dir); err == nil {
		t.Error("табличка-строка принята")
	}
}

// Запас знаков: таблички — в общем запасе, Load и CheckTotals считают их одинаково
// (TestTotalsMatchLoad): проект на границе с табличками переживает круг, знак сверх — отказ.
func TestTotalsSeatNames(t *testing.T) {
	dir := testkit.TempDir(t)
	// budgetProject без имени фото (140 знаков) и с теми же 140 знаками в табличках двух листов.
	seated := func() *model.Project {
		p := budgetProject(dir)
		r := p.Photos[p.Spreads[0].Photos[0]]
		r.Person = nil
		p.Photos[p.Spreads[0].Photos[0]] = r
		s := p.Spreads[0]
		for i, id := range s.Photos {
			s.Grid = append(s.Grid, &model.Place{ID: fmt.Sprintf("c0ffee%06d", i+1), X: 12 + float64(i)*90, Y: 12, W: 80, H: 60, Photo: id})
		}
		s.Grid[0].Person = model.Person{Last: ya(60), First: ya(40)}
		p.Spreads[1].Grid = []*model.Place{{ID: "c0ffee000100", X: 12, Y: 12, W: 80, H: 60, Person: model.Person{Subject: ya(40)}}}
		return p
	}
	p := seated()
	if b, r := budgetCount(p); b != 204 || r != model.MaxTotalRunes {
		t.Fatalf("заготовка: %d блоков, %d знаков", b, r)
	}
	if err := p.CheckTotals(p); err != nil {
		t.Fatalf("граница запаса с табличками не проходит: %v", err)
	}
	_, q := saveLoad(t, p, filepath.Join(dir, "a.salbum"))
	if b, r := budgetCount(q); b != 204 || r != model.MaxTotalRunes {
		t.Fatalf("Load обрезал проект на границе: %d блоков, %d знаков", b, r)
	}
	for name, more := range map[string]func(p *model.Project){
		"табличка с фото": func(p *model.Project) { p.Spreads[0].Grid[0].Person.Middle = "я" },
		"пустое место":    func(p *model.Project) { p.Spreads[1].Grid[0].Person.Subject += "я" },
	} {
		p := seated()
		more(p)
		if _, r := budgetCount(p); r != model.MaxTotalRunes+1 {
			t.Fatalf("%s: заготовка %d знаков", name, r)
		}
		if err := p.CheckTotals(p); err == nil {
			t.Errorf("%s: знак сверх запаса прошёл CheckTotals", name)
		}
		// Load обрезает лишнее, и открытый проект снова проходит CheckTotals («не замёрз»).
		_, q := saveLoad(t, p, filepath.Join(dir, "b.salbum"))
		if _, r := budgetCount(q); r > model.MaxTotalRunes {
			t.Errorf("%s: после Load %d знаков", name, r)
		}
		if err := q.CheckTotals(q); err != nil {
			t.Errorf("%s: после Load запас превышен: %v", name, err)
		}
	}
}
