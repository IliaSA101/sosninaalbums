package text

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

var updateGolden = flag.Bool("update", false, "переписать голдены testdata")

// builtinWidths — переносы и ширины строк всех встроенных гарнитур во всех
// начертаниях: свои шрифты (082) не должны сдвинуть ни одной строки старых альбомов.
func builtinWidths() string {
	var b strings.Builder
	const txt = "Анна и Павел — 12.08.2026, AVATAR То Ёж\nWedding day «Ура!»"
	for _, id := range FamilyIDs() {
		for _, bi := range [][2]bool{{false, false}, {true, false}, {false, true}, {true, true}} {
			st := Style{Font: id, Size: 18, Align: "center", Bold: bi[0], Italic: bi[1]}
			for _, w := range []float64{55, 1000} {
				box := Layout(txt, st, w)
				fmt.Fprintf(&b, "%s b=%v i=%v w=%g h=%.6f\n", id, bi[0], bi[1], w, box.H)
				for _, l := range box.Lines {
					fmt.Fprintf(&b, "\t%q x=%.6f y=%.6f w=%.6f\n", l.Text, l.X, l.Y, l.W)
				}
			}
		}
	}
	return b.String()
}

// Голден снят до правки 082 (G1); при зарегистрированных своих — тот же.
func TestBuiltinWidthsUnchanged(t *testing.T) {
	got := builtinWidths()
	path := filepath.Join("testdata", "builtin-widths.golden")
	if *updateGolden {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != strings.ReplaceAll(string(want), "\r\n", "\n") {
		t.Fatal("ширины встроенных гарнитур изменились (testdata/builtin-widths.golden)")
	}
	r, rb := face(t, "Lobster-Regular.ttf", false, false)
	setCustom(t, []CustomFamily{{ID: FamilyID("Montserrat"), Name: "Montserrat", Faces: []CustomFace{r}}}, loader(rb))
	if builtinWidths() != got {
		t.Fatal("своя гарнитура с именем встроенной сдвинула встроенные")
	}
}

// keyOf — ключ файла, как у библиотеки: 16 hex sha256.
func keyOf(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:8])
}

// loader — load для SetCustom по набору файлов (ключ считается сам).
func loader(files ...[]byte) func(string) ([]byte, error) {
	m := map[string][]byte{}
	for _, b := range files {
		m[keyOf(b)] = b
	}
	return func(key string) ([]byte, error) {
		if b, ok := m[key]; ok {
			return b, nil
		}
		return nil, errors.New("нет файла")
	}
}

// face — начертание своей гарнитуры из встроенного файла.
func face(t testing.TB, file string, bold, italic bool) (CustomFace, []byte) {
	b := builtinBytes(t, file)
	return CustomFace{Key: keyOf(b), Ext: "ttf", Bold: bold, Italic: italic}, b
}

// setCustom регистрирует гарнитуры и снимает их после теста.
func setCustom(t *testing.T, fams []CustomFamily, load func(string) ([]byte, error)) {
	t.Helper()
	SetCustom(fams, load)
	t.Cleanup(func() { SetCustom(nil, nil) })
}

// sameBox — одинаковые переносы и ширины.
func sameBox(a, b Box) bool {
	if a.W != b.W || a.H != b.H || a.Size != b.Size || len(a.Lines) != len(b.Lines) {
		return false
	}
	for i := range a.Lines {
		if a.Lines[i] != b.Lines[i] {
			return false
		}
	}
	return true
}

const sample = "Анна и Павел — приглашают вас на свадьбу Wedding day AVATAR"

func TestFamilyIDStable(t *testing.T) {
	const lato = "my:0e82892fea1dcf13" // эталон: id в файлах альбомов не должен меняться
	for _, n := range []string{"Lato", "lato", "  LATO ", "\tLa to"[:3] + "to"} {
		if got := FamilyID(n); got != lato {
			t.Errorf("FamilyID(%q) = %s, ждали %s", n, got, lato)
		}
	}
	if FamilyID("Lato Light") == lato || FamilyID("Lato  Light") != FamilyID("lato light") {
		t.Error("пробелы внутри имени")
	}
	if FamilyID("Café") != FamilyID("Café") {
		t.Error("NFC: составная и разложенная é дали разные id")
	}
	for _, id := range []string{lato, FamilyID(""), FamilyID("Шрифт")} {
		if !IsCustomID(id) {
			t.Errorf("IsCustomID(%q) = false", id)
		}
	}
	for _, id := range []string{"", "montserrat", "my:", "my:0E82892FEA1DCF13", "my:0e82892fea1dcf1", "my:0e82892fea1dcf133", "my:0e82892fea1dcf1g", "MY:0e82892fea1dcf13", "my/0e82892fea1dcf13"} {
		if IsCustomID(id) {
			t.Errorf("IsCustomID(%q) = true", id)
		}
	}
}

func TestClampedKeepsCustomID(t *testing.T) {
	r, rb := face(t, "Lobster-Regular.ttf", false, false)
	setCustom(t, []CustomFamily{{ID: FamilyID("Lobster X"), Name: "Lobster X", Faces: []CustomFace{r}}}, loader(rb))
	missing := FamilyID("Нет такого")
	cases := []struct {
		in, want Style
	}{
		// незарегистрированная: id, жирный и курсив остаются (рисует Montserrat со всеми 4)
		{Style{Font: missing, Bold: true, Italic: true}, Style{Font: missing, Bold: true, Italic: true}},
		{Style{Font: missing, Italic: true}, Style{Font: missing, Italic: true}},
		// зарегистрированная: начертаний, которых нет, не бывает
		{Style{Font: FamilyID("Lobster X"), Bold: true, Italic: true}, Style{Font: FamilyID("Lobster X")}},
		// неверная грамматика — как неизвестный встроенный
		{Style{Font: "my:XYZ", Bold: true}, Style{Font: DefaultFont, Bold: true}},
		{Style{Font: "no-such", Italic: true}, Style{Font: DefaultFont, Italic: true}},
	}
	for _, c := range cases {
		got := c.in.Clamped()
		if got.Font != c.want.Font || got.Bold != c.want.Bold || got.Italic != c.want.Italic {
			t.Errorf("%+v → %+v, ждали %+v", c.in, got, c.want)
		}
	}
}

func TestCustomLayoutMatchesDraw(t *testing.T) {
	cff := fixture(t, "sa-test-cff.otf")
	r, rb := face(t, "Lobster-Regular.ttf", false, false)
	nun := []CustomFace{}
	var nunBytes [][]byte
	for _, f := range family("nunito").Faces {
		fc, b := face(t, f.File, f.Bold, f.Italic)
		nun, nunBytes = append(nun, fc), append(nunBytes, b)
	}
	setCustom(t, []CustomFamily{
		{ID: FamilyID("Cff"), Name: "Cff", Faces: []CustomFace{{Key: keyOf(cff), Ext: "otf"}}},
		{ID: FamilyID("Lob"), Name: "Lob", Faces: []CustomFace{r}},
		{ID: FamilyID("Nun"), Name: "Nun", Faces: nun},
	}, loader(append(nunBytes, cff, rb)...))
	const dpi = 300
	mmpx := func(mm float64) float64 { return mm / 25.4 * dpi }
	for _, st := range []Style{
		{Font: FamilyID("Cff"), Size: 20, Color: "#000000", Align: "center"},
		{Font: FamilyID("Lob"), Size: 20, Color: "#000000", Align: "right"},
		{Font: FamilyID("Nun"), Size: 20, Color: "#000000", Align: "left", Bold: true, Italic: true},
	} {
		bw, x0, y0 := 80.0, 20.0, 15.0
		b := Layout(sample, st, bw)
		if len(b.Lines) < 2 {
			t.Fatalf("%s: строк %d", st.Font, len(b.Lines))
		}
		img := image.NewRGBA(image.Rect(0, 0, int(mmpx(130)), int(mmpx(80))))
		draw.Draw(img, img.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
		Draw(img, b, st, x0, y0, 0, dpi)
		ink := inkBounds(img)
		tol := mmpx(1)
		if ink.Empty() || float64(ink.Min.X) < mmpx(x0)-tol || float64(ink.Max.X) > mmpx(x0+b.W)+tol ||
			float64(ink.Min.Y) < mmpx(y0)-tol || float64(ink.Max.Y) > mmpx(y0+b.H)+tol {
			t.Errorf("%s: чернила %v вне рамки блока", st.Font, ink)
		}
	}
	// свой файл того же шрифта — те же переносы и та же картинка, что у встроенного:
	// путь замера и отрисовки один
	for _, c := range []struct {
		custom, builtin Style
	}{
		{Style{Font: FamilyID("Lob"), Size: 16}, Style{Font: "lobster", Size: 16}},
		{Style{Font: FamilyID("Nun"), Size: 16, Bold: true, Italic: true}, Style{Font: "nunito", Size: 16, Bold: true, Italic: true}},
	} {
		a, b := Layout(sample, c.custom, 60), Layout(sample, c.builtin, 60)
		if !sameBox(a, b) {
			t.Errorf("%s: %+v\nвстроенный: %+v", c.custom.Font, a.Lines, b.Lines)
		}
		ia := image.NewRGBA(image.Rect(0, 0, 300, 200))
		ib := image.NewRGBA(image.Rect(0, 0, 300, 200))
		Draw(ia, a, c.custom, 1, 1, 0, 100)
		Draw(ib, b, c.builtin, 1, 1, 0, 100)
		if !bytes.Equal(ia.Pix, ib.Pix) {
			t.Errorf("%s: отрисовка не как у встроенного", c.custom.Font)
		}
	}
}

func TestCustomMissingFallsBackToMontserrat(t *testing.T) {
	missing := FamilyID("Lato")
	for _, bi := range [][2]bool{{false, false}, {true, false}, {false, true}, {true, true}} {
		st := Style{Font: missing, Size: 18, Color: "#000000", Align: "center", Bold: bi[0], Italic: bi[1]}
		mo := st
		mo.Font = DefaultFont
		a, b := Layout(sample, st, 70), Layout(sample, mo, 70)
		if !sameBox(a, b) {
			t.Errorf("b=%v i=%v: замер не Montserrat", bi[0], bi[1])
		}
		ia := image.NewRGBA(image.Rect(0, 0, 300, 200))
		ib := image.NewRGBA(image.Rect(0, 0, 300, 200))
		Draw(ia, a, st, 1, 1, 0, 100)
		Draw(ib, b, mo, 1, 1, 0, 100)
		if !bytes.Equal(ia.Pix, ib.Pix) || inkBounds(ia).Empty() {
			t.Errorf("b=%v i=%v: отрисовка не Montserrat", bi[0], bi[1])
		}
	}
	if Available(missing) {
		t.Error("Available для незарегистрированной")
	}
	if _, ok := CustomName(missing); ok {
		t.Error("CustomName для незарегистрированной")
	}
	for _, f := range Families() {
		if f.ID == missing {
			t.Error("незарегистрированная в списке")
		}
	}
	// встроенные — всегда
	if !Available("lobster") || !Available(DefaultFont) || Available("no-such") {
		t.Error("Available встроенных")
	}
}

func TestCachedGenOnRegister(t *testing.T) {
	id := FamilyID("Lob")
	st := Style{Font: id, Size: 16}
	mont := Cached(sample, st, 60)
	if want := Layout(sample, Style{Font: DefaultFont, Size: 16}, 60); !sameBox(mont, want) {
		t.Fatal("до регистрации — не Montserrat")
	}
	r, rb := face(t, "Lobster-Regular.ttf", false, false)
	setCustom(t, []CustomFamily{{ID: id, Name: "Lob", Faces: []CustomFace{r}}}, loader(rb))
	lob := Cached(sample, st, 60)
	if want := Layout(sample, Style{Font: "lobster", Size: 16}, 60); !sameBox(lob, want) || sameBox(lob, mont) {
		t.Error("после регистрации кэш отдал строки Montserrat")
	}
	SetCustom(nil, nil)
	if got := Cached(sample, st, 60); !sameBox(got, mont) {
		t.Error("после снятия кэш отдал строки своего шрифта")
	}
}

func TestFamiliesCustom(t *testing.T) {
	r, rb := face(t, "Lobster-Regular.ttf", false, false)
	b, bb := face(t, "Caveat-Bold.ttf", true, false)
	setCustom(t, []CustomFamily{
		{ID: FamilyID("zeta"), Name: "zeta", Faces: []CustomFace{r}, Hidden: true},
		{ID: FamilyID("Alpha"), Name: "Alpha", Faces: []CustomFace{b, r}, NoCyr: true},
		{ID: "my:bad", Name: "плохой id", Faces: []CustomFace{r}},
		{ID: FamilyID("Пусто"), Name: "Пусто"},
		{ID: FamilyID("Ключ"), Name: "Ключ", Faces: []CustomFace{{Key: "../../etc", Ext: "ttf"}, {Key: r.Key, Ext: "woff"}}},
		{ID: FamilyID("zeta"), Name: "zeta 2", Faces: []CustomFace{b}},
	}, loader(rb, bb))
	all := Families()
	if len(all) != len(families)+2 {
		t.Fatalf("гарнитур %d, ждали %d", len(all), len(families)+2)
	}
	for i := range families {
		if all[i].ID != families[i].ID || all[i].Hidden || all[i].NoCyr {
			t.Fatalf("встроенная %d: %+v", i, all[i])
		}
	}
	a, z := all[len(families)], all[len(families)+1]
	if a.Name != "Alpha" || z.Name != "zeta" || a.Group != CustomGroup || z.Group != CustomGroup {
		t.Fatalf("порядок своих: %+v, %+v", a, z)
	}
	if a.CSS != "SA my "+FamilyID("Alpha")[3:] || !a.NoCyr || a.Hidden || !z.Hidden {
		t.Errorf("поля: %+v, %+v", a, z)
	}
	if len(a.Faces) != 2 || a.Faces[0].File != b.Key+".ttf" || !a.Faces[0].Bold || a.Faces[1].File != r.Key+".ttf" {
		t.Errorf("начертания: %+v", a.Faces)
	}
	if name, ok := CustomName(FamilyID("Alpha")); !ok || name != "Alpha" {
		t.Errorf("CustomName = %q, %v", name, ok)
	}
	if !Available(FamilyID("zeta")) || Available(FamilyID("Пусто")) || Available(FamilyID("Ключ")) {
		t.Error("Available: скрытая доступна, без начертаний — нет")
	}
	// пустой набор — только встроенные
	SetCustom(nil, nil)
	if len(Families()) != len(families) {
		t.Error("после снятия остались свои")
	}
}

func TestFontFileCustom(t *testing.T) {
	r, rb := face(t, "Lobster-Regular.ttf", false, false)
	cff := fixture(t, "sa-test-cff.otf")
	fake, _ := face(t, "Caveat-Bold.ttf", true, false)
	setCustom(t, []CustomFamily{
		{ID: FamilyID("Lob"), Name: "Lob", Faces: []CustomFace{r}, Hidden: true},
		{ID: FamilyID("Cff"), Name: "Cff", Faces: []CustomFace{{Key: keyOf(cff), Ext: "otf"}}},
		{ID: FamilyID("Подмена"), Name: "Подмена", Faces: []CustomFace{fake}},
	}, func(key string) ([]byte, error) {
		if key == fake.Key {
			return rb, nil // файл на диске подменили
		}
		return loader(rb, cff)(key)
	})
	if got, ok := FontFile(r.Key + ".ttf"); !ok || !bytes.Equal(got, rb) {
		t.Error("скрытая своя не отдана")
	}
	if got, ok := FontFile(keyOf(cff) + ".otf"); !ok || !bytes.Equal(got, cff) {
		t.Error("CFF не отдан")
	}
	if _, ok := FontFile("Montserrat-Regular.ttf"); !ok {
		t.Error("встроенный не отдан")
	}
	for _, name := range []string{
		r.Key + ".otf", keyOf(cff) + ".ttf", fake.Key + ".ttf", strings.ToUpper(r.Key) + ".ttf",
		"0123456789abcdef.ttf", "../" + r.Key + ".ttf", r.Key, r.Key + ".ttf/", "", ".ttf",
	} {
		if _, ok := FontFile(name); ok {
			t.Errorf("отдан %q", name)
		}
	}
	SetCustom(nil, nil)
	if _, ok := FontFile(r.Key + ".ttf"); ok {
		t.Error("отдан после снятия")
	}
}

func TestCustomPartialFaces(t *testing.T) {
	cb, cbb := face(t, "Caveat-Bold.ttf", true, false)
	nr, nrb := face(t, "Nunito-Regular.ttf", false, false)
	ni, nib := face(t, "Nunito-Italic.ttf", false, true)
	nb, nbb := face(t, "Nunito-Bold.ttf", true, false)
	setCustom(t, []CustomFamily{
		{ID: FamilyID("Только Ж"), Name: "Только Ж", Faces: []CustomFace{cb}},
		{ID: FamilyID("О+К"), Name: "О+К", Faces: []CustomFace{nr, ni, {Key: nb.Key, Ext: "ttf"}}}, // повтор обычного — пропуск
		{ID: FamilyID("Ж+К"), Name: "Ж+К", Faces: []CustomFace{nb, ni}},
	}, loader(cbb, nrb, nib, nbb))
	cases := []struct {
		st   Style
		want Style
		file string
	}{
		{Style{Font: FamilyID("Только Ж")}, Style{}, cb.Key},
		{Style{Font: FamilyID("Только Ж"), Bold: true, Italic: true}, Style{Bold: true}, cb.Key},
		{Style{Font: FamilyID("О+К"), Bold: true}, Style{}, nr.Key},
		{Style{Font: FamilyID("О+К"), Italic: true}, Style{Italic: true}, ni.Key},
		{Style{Font: FamilyID("Ж+К")}, Style{}, ni.Key}, // без обычного — сначала курсив
		{Style{Font: FamilyID("Ж+К"), Bold: true}, Style{Bold: true}, nb.Key},
		{Style{Font: FamilyID("Ж+К"), Bold: true, Italic: true}, Style{}, ni.Key},
	}
	for _, c := range cases {
		got := c.st.Clamped()
		if got.Bold != c.want.Bold || got.Italic != c.want.Italic || got.Font != c.st.Font {
			t.Errorf("%+v → %+v", c.st, got)
		}
		if f := faceFile(got); f != c.file+".ttf" {
			t.Errorf("%+v: файл %s, ждали %s", c.st, f, c.file)
		}
		if _, f := font(got); f != c.file+".ttf" {
			t.Errorf("%+v: шрифт из %s", c.st, f)
		}
	}
	for _, f := range Families() {
		if f.ID == FamilyID("О+К") && len(f.Faces) != 2 {
			t.Errorf("повтор начертания не пропущен: %+v", f.Faces)
		}
	}
}

func TestCustomBrokenFallsBack(t *testing.T) {
	r, rb := face(t, "Lobster-Regular.ttf", false, false)
	garbage := bytes.Repeat([]byte("не шрифт"), 50)
	gone := CustomFace{Key: "0123456789abcdef", Ext: "ttf"}
	panicKey := "fedcba9876543210"
	setCustom(t, []CustomFamily{
		{ID: FamilyID("Целый"), Name: "Целый", Faces: []CustomFace{r}},
		{ID: FamilyID("Мусор"), Name: "Мусор", Faces: []CustomFace{{Key: keyOf(garbage), Ext: "ttf"}}},
		{ID: FamilyID("Нет файла"), Name: "Нет файла", Faces: []CustomFace{gone}},
		{ID: FamilyID("Паника"), Name: "Паника", Faces: []CustomFace{{Key: panicKey, Ext: "otf"}}},
		// жирное начертание сломано — гарнитура снимается целиком, без смеси файлов
		{ID: FamilyID("Половина"), Name: "Половина", Faces: []CustomFace{r, {Key: "00000000000000ff", Ext: "ttf", Bold: true}}},
	}, func(key string) ([]byte, error) {
		if key == panicKey {
			panic("сбой чтения")
		}
		return loader(rb, garbage)(key)
	})
	if got := Layout(sample, Style{Font: FamilyID("Половина"), Size: 14}, 50); !sameBox(got, Layout(sample, Style{Font: "lobster", Size: 14}, 50)) {
		t.Error("Половина: обычное начертание до поломки жирного — не своим файлом")
	}
	broken := []string{FamilyID("Мусор"), FamilyID("Нет файла"), FamilyID("Паника"), FamilyID("Половина")}
	for _, id := range broken {
		st := Style{Font: id, Size: 14, Bold: true}
		mo := st.Clamped() // у однофайловых жирный сброшен, у «Половины» есть
		mo.Font = DefaultFont
		gen := customGen.Load()
		if got := Cached(sample, st, 50); !sameBox(got, Layout(sample, mo, 50)) {
			t.Errorf("%s: не Montserrat", id)
		}
		if customGen.Load() == gen {
			t.Errorf("%s: поколение не выросло — кэш отдал бы старые строки", id)
		}
		if Available(id) {
			t.Errorf("%s: Available", id)
		}
		if st.Clamped().Font != id {
			t.Errorf("%s: id потерян", id)
		}
	}
	// после поломки жирного обычное начертание «Половины» тоже Montserrat
	if got := Layout(sample, Style{Font: FamilyID("Половина"), Size: 14}, 50); !sameBox(got, Layout(sample, Style{Font: DefaultFont, Size: 14}, 50)) {
		t.Error("Половина: обычное своим файлом при сломанном жирном")
	}
	if !Available(FamilyID("Целый")) {
		t.Error("соседняя гарнитура сломалась")
	}
	for _, f := range Families() {
		for _, id := range broken {
			if f.ID == id {
				t.Errorf("сломанная %s в списке", id)
			}
		}
	}
	// новая регистрация пробует файл заново (например, вернули на диск)
	SetCustom([]CustomFamily{{ID: FamilyID("Нет файла"), Name: "Нет файла", Faces: []CustomFace{r}}}, loader(rb))
	if !Available(FamilyID("Нет файла")) {
		t.Error("после новой регистрации не ожил")
	}
}

// Параллельные замеры при смене набора (для -race): ни паники, ни пустого блока.
func TestCustomConcurrent(t *testing.T) {
	r, rb := face(t, "Lobster-Regular.ttf", false, false)
	fams := []CustomFamily{{ID: FamilyID("Lob"), Name: "Lob", Faces: []CustomFace{r}}}
	t.Cleanup(func() { SetCustom(nil, nil) })
	var wg sync.WaitGroup
	for i := range 4 {
		wg.Go(func() {
			for j := range 30 {
				st := Style{Font: FamilyID("Lob"), Size: float64(10 + i + j%3)}
				if b := Cached(sample, st, 40); len(b.Lines) == 0 || b.Lines[0].W == 0 {
					t.Error("пустой блок")
					return
				}
				_ = Families()
				_, _ = FontFile(r.Key + ".ttf")
			}
		})
	}
	for j := range 30 {
		if j%2 == 0 {
			SetCustom(fams, loader(rb))
		} else {
			SetCustom(nil, nil)
		}
	}
	wg.Wait()
}
