package model

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

const fileA = "0123456789abcdef" // ID файла: 16 hex, как library.IDFor

// TestBaseID — разбор ID копии: годный суффикс 2…99 без нулей впереди; иначе ID как есть.
func TestBaseID(t *testing.T) {
	for _, c := range copyCases() {
		if got := BaseID(c.ID); got != c.Base {
			t.Errorf("BaseID(%q) = %q, ждали %q", c.ID, got, c.Base)
		}
		if got := CopyNo(c.ID); got != c.No {
			t.Errorf("CopyNo(%q) = %d, ждали %d", c.ID, got, c.No)
		}
		if c.No > 0 && CopyID(c.Base, c.No) != c.ID {
			t.Errorf("CopyID(%q, %d) = %q, ждали %q", c.Base, c.No, CopyID(c.Base, c.No), c.ID)
		}
		if got := IsCopyReq(c.ID); got != c.Req {
			t.Errorf("IsCopyReq(%q) = %v", c.ID, got)
		}
	}
	for _, n := range []int{0, 1, -1, MaxCopy + 1} {
		if got := CopyID(fileA, n); got != fileA {
			t.Errorf("CopyID(%d) = %q, ждали ID файла", n, got)
		}
	}
	if CopyReq(fileA) != fileA+"-new" || !IsCopyReq(CopyReq(fileA)) || BaseID(CopyReq(fileA)) != CopyReq(fileA) {
		t.Errorf("запрос копии %q", CopyReq(fileA))
	}
}

// copyCase — случай разбора ID: общий с интерфейсом (testdata/parity/copyid.json, base/core/copies.ts).
type copyCase struct {
	ID   string `json:"id"`
	Base string `json:"base"`
	No   int    `json:"no"`
	Req  bool   `json:"req"`
}

func copyCases() []copyCase {
	return []copyCase{
		{ID: fileA, Base: fileA},
		{ID: fileA + "-2", Base: fileA, No: 2},
		{ID: fileA + "-10", Base: fileA, No: 10},
		{ID: fileA + "-99", Base: fileA, No: 99},
		{ID: fileA + "-100", Base: fileA + "-100"},
		{ID: fileA + "-1", Base: fileA + "-1"},
		{ID: fileA + "-0", Base: fileA + "-0"},
		{ID: fileA + "-02", Base: fileA + "-02"},
		{ID: fileA + "-x", Base: fileA + "-x"},
		{ID: fileA + "-", Base: fileA + "-"},
		{ID: fileA + "-+3", Base: fileA + "-+3"},
		{ID: fileA + "-new", Base: fileA + "-new", Req: true},
		{ID: "-new", Base: "-new"},
		{ID: "-3", Base: "-3"},
		{ID: "a-b-3", Base: "a-b", No: 3},
		{ID: "", Base: ""},
	}
}

// TestCopyIDParity — вектор разбора ID для интерфейса (copies.test.ts): пишется с -update, сверяется
// с Go каждый прогон — правка правил без перезаписи вектора роняет тест.
func TestCopyIDParity(t *testing.T) {
	path := filepath.Join("..", "..", "..", "testdata", "parity", "copyid.json")
	if *update {
		data, err := json.MarshalIndent(copyCases(), "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var cs []copyCase
	if err := json.Unmarshal(data, &cs); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(cs, copyCases()) {
		t.Fatalf("copyid.json устарел (перезапись: go test ./internal/album/model -run CopyIDParity -update)")
	}
}

// copiesProject — файл fileA записью и копиями 2, 5; копия 2 стоит на развороте, сам файл — нет.
func copiesProject() *Project {
	p := New()
	ref := PhotoRef{Path: "/ф/a.jpg", W: 300, H: 200, Size: 10}
	for _, id := range []string{fileA, CopyID(fileA, 2), CopyID(fileA, 5), "fedcba9876543210"} {
		p.Photos[id] = ref
	}
	p.Spreads = []*Spread{{ID: "s1", Photos: []string{CopyID(fileA, 2), "fedcba9876543210"}}}
	return p
}

func TestFamily(t *testing.T) {
	p := copiesProject()
	want := []string{fileA, fileA + "-2", fileA + "-5"}
	for _, id := range append(want, fileA+"-99") {
		if got := p.Family(id); !slices.Equal(got, want) {
			t.Errorf("Family(%q) = %v", id, got)
		}
	}
	if got := p.Family("ffffffffffffffff"); got != nil {
		t.Errorf("чужой файл: %v", got)
	}
}

// TestPlacedFile — «в альбоме» по файлу: стоит одна копия — стоит весь файл; фон не в счёт.
func TestPlacedFile(t *testing.T) {
	p := copiesProject()
	for _, id := range []string{fileA, fileA + "-2", fileA + "-5"} {
		if !p.PlacedFile(id) {
			t.Errorf("PlacedFile(%q) = false", id)
		}
	}
	if p.Placed(fileA) || !p.Placed(fileA+"-2") {
		t.Error("Placed — по записи")
	}
	p.Spreads[0].Photos = []string{"fedcba9876543210"}
	p.Spreads[0].BgPhoto = &Backdrop{Photo: fileA}
	if p.PlacedFile(fileA) || p.PlacedFile(fileA+"-2") {
		t.Error("фото только на фоне — не в альбоме")
	}
	p.Spreads = nil
	if p.PlacedFile("fedcba9876543210") || p.PlacedFile("") {
		t.Error("пустой альбом")
	}
}

// TestRememberSetsFamily — сведения файла из ленты получает вся семья; свойства использования
// (кадр, подпись, свой кадр) у каждой записи свои и не трогаются.
func TestRememberSetsFamily(t *testing.T) {
	p := copiesProject()
	c := p.Photos[fileA+"-5"]
	c.Frame, c.Asp, c.Caption, c.Alt = &Frame{S: 2}, 1.5, &Caption{Text: "к"}, "/old/a.jpg"
	p.Photos[fileA+"-5"] = c
	other := p.Photos["fedcba9876543210"]
	Remember(p, []PhotoInfo{{ID: fileA, Path: "/ф2/a.jpg", W: 400, H: 100, Size: 77}})
	for _, id := range p.Family(fileA) {
		r := p.Photos[id]
		if r.Path != "/ф2/a.jpg" || r.W != 400 || r.H != 100 || r.Size != 77 || r.Alt != "" {
			t.Errorf("%s: %+v", id, r)
		}
	}
	if r := p.Photos[fileA+"-5"]; r.Frame == nil || r.Frame.S != 2 || r.Asp != 1.5 || r.Caption == nil {
		t.Errorf("свойства копии стёрты: %+v", r)
	}
	if p.Photos["fedcba9876543210"] != other {
		t.Error("чужая запись изменилась")
	}
	// Сведения через копию — тоже всей семье.
	Remember(p, []PhotoInfo{{ID: fileA + "-2", Path: "/ф2/a.jpg", W: 40, H: 10}})
	if p.Photos[fileA].W != 40 || p.Photos[fileA+"-5"].W != 40 || p.Photos[fileA].Size != 77 {
		t.Errorf("через копию: %+v", p.Photos[fileA])
	}
	// Пустой список и запись без семьи.
	Remember(p, nil)
	Remember(p, []PhotoInfo{{ID: "1111111111111111", Path: "/x.jpg", W: 1, H: 1}})
	if len(p.Family("1111111111111111")) != 1 {
		t.Error("новая запись")
	}
}

// TestAspectsTwoKinds — ячейка «Авто» берёт свой кадр, новое место и геометрия фото — нет.
func TestAspectsTwoKinds(t *testing.T) {
	p := New()
	p.Photos["a"] = PhotoRef{W: 300, H: 200, Asp: 0.8}
	p.Photos["b"] = PhotoRef{W: 300, H: 200, Frame: &Frame{S: 1, R: 90}}
	p.Photos["c"] = PhotoRef{W: 300, H: 200, Frame: &Frame{S: 1, R: 90}, Asp: 1.7778}
	ids := []string{"a", "b", "c", "нет"}
	if got := p.Aspects(ids); !slices.Equal(got, []float64{0.8, 200.0 / 300, 1.7778, 1}) {
		t.Errorf("Aspects %v", got)
	}
	if got := p.PhotoAspects(ids); !slices.Equal(got, []float64{1.5, 200.0 / 300, 200.0 / 300, 1}) {
		t.Errorf("PhotoAspects %v", got)
	}
	if got := p.RawAspect("c"); got != 1.5 {
		t.Errorf("RawAspect %v", got)
	}
	if got := p.Aspects(nil); len(got) != 0 {
		t.Errorf("пусто: %v", got)
	}
}

func TestClampAsp(t *testing.T) {
	for in, want := range map[float64]float64{0: 0, -1: 0, math.NaN(): 0, math.Inf(1): 0, 0.009: 0, 100.5: 0,
		0.01: 0.01, 100: 100, 1.77777: 1.7778, 0.66666: 0.6667} {
		if got := ClampAsp(in); got != want {
			t.Errorf("ClampAsp(%v) = %v, ждали %v", in, got, want)
		}
	}
}

// TestPruneRefsDropsAsp — фото без мест теряет свой кадр с кадром; стоящее — нет.
func TestPruneRefsDropsAsp(t *testing.T) {
	p := New()
	p.Photos["a"] = PhotoRef{W: 3, H: 2, Asp: 1.2, Frame: &Frame{S: 2}}
	p.Photos["b"] = PhotoRef{W: 3, H: 2, Asp: 1.2}
	p.Photos["c"] = PhotoRef{W: 3, H: 2, Asp: 1.2}
	p.Spreads = []*Spread{{ID: "s", Photos: []string{"a"}, BgPhoto: &Backdrop{Photo: "b"}}}
	p.PruneRefs()
	if p.Photos["a"].Asp != 1.2 {
		t.Error("стоящее потеряло свой кадр")
	}
	if r, ok := p.Photos["b"]; !ok || r.Asp != 0 {
		t.Errorf("фон: %+v, %v", r, ok)
	}
	if _, ok := p.Photos["c"]; ok {
		t.Error("ненужная запись осталась")
	}
}

// TestClampDesignAsp — свой кадр из файла зажимается при чтении.
func TestClampDesignAsp(t *testing.T) {
	p := New()
	p.Photos["a"] = PhotoRef{W: 3, H: 2, Asp: 1e9}
	p.Photos["b"] = PhotoRef{W: 3, H: 2, Asp: 1.23456}
	p.ClampDesign()
	if p.Photos["a"].Asp != 0 || p.Photos["b"].Asp != 1.2346 {
		t.Errorf("%v %v", p.Photos["a"].Asp, p.Photos["b"].Asp)
	}
}
