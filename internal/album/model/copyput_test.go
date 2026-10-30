package model

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// copyProj — лист с фото файла a (запись a стоит), файл b нигде не стоит. Пути абсолютные, ID — по пути.
func copyProj() (*Project, string, string) {
	pa, pb := filepath.Join(string(filepath.Separator), "ф", "a.jpg"), filepath.Join(string(filepath.Separator), "ф", "b.jpg")
	a, b := library.IDFor(pa), library.IDFor(pb)
	p := New()
	p.Photos[a] = PhotoRef{Path: pa, W: 300, H: 200, Size: 7, Frame: &Frame{S: 2}, Caption: &Caption{Text: "x"},
		FlipH: true, Adj: &Tone{Gray: true}, Fade: 0.3, Person: &Person{First: "Анна"}, Asp: 1.5}
	p.Photos[b] = PhotoRef{Path: pb, W: 200, H: 300}
	p.Spreads = []*Spread{{ID: "s1", Photos: []string{a}}}
	return p, a, b
}

// Номера — наименьший свободный с 2; ID файла — по пути записи, не по её ID.
func TestAddCopyNumbers(t *testing.T) {
	p, a, _ := copyProj()
	for _, want := range []string{CopyID(a, 2), CopyID(a, 3)} {
		if id, err := AddCopy(p, p.Photos[a]); err != nil || id != want {
			t.Fatalf("AddCopy = %q, %v; ждали %q", id, err, want)
		}
	}
	delete(p.Photos, CopyID(a, 2))
	if id, _ := AddCopy(p, p.Photos[a]); id != CopyID(a, 2) {
		t.Fatalf("освободившийся номер не взят: %q", id)
	}
	if _, err := AddCopy(p, PhotoRef{Path: "", W: 1, H: 1}); !errors.Is(err, ErrCopyLib) {
		t.Fatalf("пустой путь: %v", err)
	}
	if _, err := AddCopy(p, PhotoRef{Path: p.Photos[a].Path, Lib: "my:00000000000000aa"}); !errors.Is(err, ErrCopyLib) {
		t.Fatalf("своё из библиотеки: %v", err)
	}
}

// FitFile: запись берёт свойства файла у другой записи семьи, свои использования не трогает;
// края — нет записи, одна запись, своё из библиотеки: без изменений.
func TestFitFile(t *testing.T) {
	p, a, _ := copyProj()
	c2, _ := AddCopy(p, p.Photos[a])
	c3, _ := AddCopy(p, p.Photos[a])
	r := p.Photos[c3]
	r.W, r.H, r.Size, r.Frame = 1, 2, 3, &Frame{S: 2}
	p.Photos[c3] = r
	f := p.Photos[a]
	f.W, f.H, f.Size = 800, 600, 99
	p.Photos[a], p.Photos[c2] = f, f
	p.FitFile(c3)
	got := p.Photos[c3]
	if got.W != 800 || got.H != 600 || got.Size != 99 || got.Frame == nil || got.Frame.S != 2 {
		t.Fatalf("после FitFile: %+v", got)
	}
	p.FitFile("нет-записи") // нет записи — паники нет
	solo := PhotoRef{Path: filepath.Join(testkit.TempDir(t), "solo.jpg"), W: 5, H: 6}
	sid := library.IDFor(solo.Path)
	p.Photos[sid] = solo
	p.FitFile(sid)
	if p.Photos[sid].W != 5 {
		t.Fatalf("одна запись изменилась: %+v", p.Photos[sid])
	}
	p.Photos["my:00000000000000aa"] = PhotoRef{Lib: "my:00000000000000aa", W: 7}
	p.FitFile("my:00000000000000aa")
	if p.Photos["my:00000000000000aa"].W != 7 {
		t.Fatal("своё из библиотеки изменилось")
	}
}

// 98 копий — предел: следующая — ErrCopies, проект не тронут.
func TestAddCopyLimit(t *testing.T) {
	p, a, _ := copyProj()
	for range MaxCopy - 1 {
		if _, err := AddCopy(p, p.Photos[a]); err != nil {
			t.Fatal(err)
		}
	}
	n := len(p.Photos)
	if _, err := AddCopy(p, p.Photos[a]); !errors.Is(err, ErrCopies) || len(p.Photos) != n {
		t.Fatalf("сверх предела: %v, записей %d → %d", err, n, len(p.Photos))
	}
}

// Запись из снимка (099): свойства использования — как в снимке, свойства файла — как у семьи.
func TestAddCopyFromSnapshot(t *testing.T) {
	p, a, _ := copyProj()
	snap := p.Photos[a].Copy()
	snap.W, snap.H, snap.Frame = 999, 1, &Frame{S: 3}
	id, err := AddCopy(p, snap)
	if err != nil {
		t.Fatal(err)
	}
	if c := p.Photos[id]; c.W != 300 || c.H != 200 || c.Frame == nil || c.Frame.S != 3 || c.Caption == nil {
		t.Fatalf("копия из снимка: %+v", c)
	}
}

// Запрос копии стоящего фото — новая запись: тон, зеркало, имя — от исходной (глубокие копии);
// кадр, подпись, прозрачность и свой кадр — пустые. Обычные ID — как есть.
func TestResolveCopiesInherit(t *testing.T) {
	p, a, b := copyProj()
	got, err := ResolveCopies(p, []PhotoInfo{{ID: CopyReq(a), Path: p.Photos[a].Path}, {ID: b}})
	if err != nil {
		t.Fatal(err)
	}
	if got[0].ID != CopyID(a, 2) || got[1].ID != b {
		t.Fatalf("ID: %q, %q", got[0].ID, got[1].ID)
	}
	c, src := p.Photos[got[0].ID], p.Photos[a]
	if c.Frame != nil || c.Caption != nil || c.Fade != 0 || c.Asp != 0 {
		t.Fatalf("копия взяла свойства места: %+v", c)
	}
	if !c.FlipH || c.Adj == nil || !c.Adj.Gray || c.Person == nil || c.Person.First != "Анна" || c.W != 300 || c.Path != src.Path {
		t.Fatalf("копия не взяла тон, зеркало, имя или файл: %+v", c)
	}
	if c.Adj == src.Adj || c.Person == src.Person {
		t.Fatal("тон или имя — общий указатель с исходной")
	}
}

// Исходная запись нигде не стоит — запрос даёт её саму; второй запрос того же броска — копию.
// Своё из библиотеки — исходная запись (копий не бывает).
func TestResolveCopiesFreeBase(t *testing.T) {
	p, a, b := copyProj()
	got, err := ResolveCopies(p, []PhotoInfo{{ID: CopyReq(b)}, {ID: CopyReq(b)}})
	if err != nil {
		t.Fatal(err)
	}
	if got[0].ID != b || got[1].ID != CopyID(b, 2) {
		t.Fatalf("ID: %q, %q", got[0].ID, got[1].ID)
	}
	if got, _ := ResolveCopies(p, []PhotoInfo{{ID: a}, {ID: CopyReq(a)}}); got[1].ID != CopyID(a, 2) {
		t.Fatalf("запись взята тем же броском — нужна копия: %q", got[1].ID)
	}
	lib := LibPhotoID("my:00000000000000aa")
	p.Photos[lib] = PhotoRef{Lib: "my:00000000000000aa", Path: "/л/лого.png"}
	p.Spreads[0].Photos = append(p.Spreads[0].Photos, lib)
	if got, err := ResolveCopies(p, []PhotoInfo{{ID: CopyReq(lib)}}); err != nil || got[0].ID != lib {
		t.Fatalf("своё: %v, %v", got, err)
	}
	// Исходной записи нет: первый запрос — она сама, второй — копия от записи семьи (или от
	// сведений запроса, если семьи нет вовсе).
	pa := p.Photos[a].Path
	delete(p.Photos, a)
	p.Spreads[0].Photos = []string{CopyID(a, 2)}
	p.Photos[CopyID(a, 2)] = PhotoRef{Path: pa, W: 300, H: 200}
	got, _ = ResolveCopies(p, []PhotoInfo{{ID: CopyReq(a)}, {ID: CopyReq(a)}})
	if got[0].ID != a || got[1].ID != CopyID(a, 3) || p.Photos[CopyID(a, 3)].W != 300 {
		t.Fatalf("без исходной: %v", got)
	}
	pc := filepath.Join(filepath.Dir(pa), "c.jpg")
	c := library.IDFor(pc)
	got, _ = ResolveCopies(p, []PhotoInfo{{ID: CopyReq(c), Path: pc, W: 5, H: 4}, {ID: CopyReq(c), Path: pc, W: 5, H: 4}})
	if got[0].ID != c || got[1].ID != CopyID(c, 2) || p.Photos[CopyID(c, 2)].W != 5 {
		t.Fatalf("без семьи: %v", got)
	}
	if ReqBase(CopyReq(a)) != a || ReqBase(a) != a || FileOf(CopyID(a, 7)) != a || FileOf(CopyReq(a)) != a {
		t.Fatal("ReqBase / FileOf")
	}
}
