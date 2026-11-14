package salbum

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"image/color"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Разметка разворотов и обрез в файле (066, v11).

// spreadGuideJSON — файл v10 с разметкой разворотов g/abs (как coverJSON, без обложки).
func spreadGuideJSON(dir, g, abs string) []byte {
	gj, _ := json.Marshal(g)
	aj, _ := json.Marshal(abs)
	return coverJSON(dir, `[]`, fmt.Sprintf(`,"spreadGuide":%s,"spreadGuideAbs":%s`, gj, aj))
}

// TestSalbumSpreadGuideRoundTrip — путь разметки разворотов как у обложки: в файле
// относительный с абсолютным рядом, черновик — абсолютный; основного нет — запасной; нет нигде —
// путь остаётся, запасной едет в SpreadGuideAlt и пишется обратно; чужое — ""; круг без потерь.
func TestSalbumSpreadGuideRoundTrip(t *testing.T) {
	dir := testkit.TempDir(t)
	guide := testkit.PSD(t, filepath.Join(dir, "Разметка"), "spread.psd", testkit.PSDSpec{W: 8, H: 4})
	p := model.New()
	p.Settings.PageW, p.Settings.PageH = 281, 284
	p.SpreadGuide = guide
	path := filepath.Join(dir, "a.salbum")
	first, q := saveLoad(t, p, path)
	var f FileProject
	if err := json.Unmarshal(first, &f); err != nil {
		t.Fatal(err)
	}
	if f.Version != model.FormatVersion || f.SpreadGuide != "Разметка/spread.psd" || f.SpreadGuideAbs != guide || bytes.Contains(first, []byte("spreadGuideAlt")) {
		t.Fatalf("в файле v%d: %q, abs %q", f.Version, f.SpreadGuide, f.SpreadGuideAbs)
	}
	second, r := saveLoad(t, q, path)
	if q.SpreadGuide != guide || q.SpreadGuideAlt != "" || !bytes.Equal(first, second) || !bytes.Equal(model.Marshal(q), model.Marshal(r)) {
		t.Fatalf("круг: %q alt %q\n%s\n%s", q.SpreadGuide, q.SpreadGuideAlt, first, second)
	}
	draft, err := Encode(q, "")
	if err != nil {
		t.Fatal(err)
	}
	f = FileProject{}
	if err := json.Unmarshal(draft, &f); err != nil || f.SpreadGuide != guide || f.SpreadGuideAbs != "" {
		t.Fatalf("черновик: %v %q %q", err, f.SpreadGuide, f.SpreadGuideAbs)
	}
	// Без разметки поля в файле нет (omitempty).
	p.SpreadGuide = ""
	if out, _ := saveLoad(t, p, path); bytes.Contains(out, []byte("spreadGuide")) {
		t.Fatalf("пустая разметка в файле:\n%s", out)
	}
	if g := mustDecode(t, spreadGuideJSON(dir, "Нет/s.psd", guide), dir); g.SpreadGuide != guide || g.SpreadGuideAlt != "" {
		t.Fatalf("основного нет — запасной: %q %q", g.SpreadGuide, g.SpreadGuideAlt)
	}
	abs := filepath.Join(dir, "Старая", "s.psd")
	lost := mustDecode(t, spreadGuideJSON(dir, "Нет/s.psd", abs), dir)
	if lost.SpreadGuide != filepath.Join(dir, "Нет", "s.psd") || lost.SpreadGuideAlt != abs {
		t.Fatalf("нет нигде: %q, запасной %q", lost.SpreadGuide, lost.SpreadGuideAlt)
	}
	out, err := Encode(lost, dir)
	if err != nil {
		t.Fatal(err)
	}
	f = FileProject{}
	if err := json.Unmarshal(out, &f); err != nil || f.SpreadGuide != "Нет/s.psd" || f.SpreadGuideAbs != abs {
		t.Fatalf("запасной в файле: %v %q %q", err, f.SpreadGuide, f.SpreadGuideAbs)
	}
	if back := mustDecode(t, out, dir); !bytes.Equal(model.Marshal(back), model.Marshal(lost)) {
		t.Fatal("круг теряет запасной путь разметки разворотов")
	}
	bad := []string{"../../../s.psd", "Разметка/s.psb", "Разметка/s.tif"}
	if runtime.GOOS == "windows" {
		bad = append(bad, `\\srv\share\s.psd`, `C:\a\s.psd:поток`)
	}
	for _, g := range bad {
		if got := mustDecode(t, spreadGuideJSON(dir, g, ""), dir); got.SpreadGuide != "" || got.SpreadGuideAlt != "" {
			t.Errorf("%q принят как %q", g, got.SpreadGuide)
		}
	}
	if got := mustDecode(t, spreadGuideJSON(dir, "", guide), dir); got.SpreadGuide != "" {
		t.Fatalf("только spreadGuideAbs: %q", got.SpreadGuide)
	}
}

// TestSalbumBleedRoundTrip — обрез переживает Save → Load; 0 в файл не пишется; дикие числа
// из файла зажимаются (0…10), отсутствующее поле — 0.
func TestSalbumBleedRoundTrip(t *testing.T) {
	dir := testkit.TempDir(t)
	path := filepath.Join(dir, "a.salbum")
	for _, c := range []struct{ in, want float64 }{{2, 2}, {2.5, 2.5}, {3, 3}, {10, 10}} {
		p := model.New()
		p.Settings.Bleed = c.in
		out, q := saveLoad(t, p, path)
		if q.Settings.Bleed != c.want || !bytes.Contains(out, fmt.Appendf(nil, `"bleed": %v`, c.in)) {
			t.Fatalf("обрез %v: %v\n%s", c.in, q.Settings.Bleed, out)
		}
	}
	if out, q := saveLoad(t, model.New(), path); q.Settings.Bleed != 0 || bytes.Contains(out, []byte("bleed")) {
		t.Fatalf("обрез 0 в файле:\n%s", out)
	}
	for in, want := range map[string]float64{`25`: 10, `-3`: 0, `"x"`: -1, `1e309`: -1} {
		data := fmt.Appendf(nil, `{"version":11,"settings":{"pageW":281,"pageH":284,"bleed":%s},"spreads":[],"photos":{}}`, in)
		p, err := Decode(data, dir)
		if want < 0 {
			if err == nil {
				t.Errorf("bleed %s: файл принят", in)
			}
			continue
		}
		if err != nil || p.Settings.Bleed != want {
			t.Errorf("bleed %s: %v, %v", in, err, p)
		}
	}
	if p := mustDecode(t, coverJSON(dir, `[]`, ""), dir); p.Settings.Bleed != 0 {
		t.Fatalf("без поля: %v", p.Settings.Bleed)
	}
}

// TestCheckSpreadGuide — Check видит и разметку разворотов (основной и запасной путь);
// CheckGuide принимает PSD, PSB отклоняет своим текстом, прочее — «только PSD, JPEG или PNG».
func TestCheckSpreadGuide(t *testing.T) {
	dir := testkit.TempDir(t)
	p := model.New()
	p.SpreadGuide = filepath.Join(dir, "s.psd")
	if err := Check(p); err != nil {
		t.Fatal(err)
	}
	p.SpreadGuideAlt = filepath.Join(dir, "old.psd")
	if err := Check(p); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS == "windows" {
		p.SpreadGuideAlt = `\\srv\share\s.psd`
		if err := Check(p); err == nil || err.Error()[:len("разметка разворотов")] != "разметка разворотов" {
			t.Fatalf("UNC в запасном: %v", err)
		}
		p.SpreadGuideAlt, p.SpreadGuide = "", `\\srv\share\s.psd`
		if Check(p) == nil {
			t.Fatal("Check пропустил UNC разметки разворотов")
		}
	}
	for _, ok := range []string{"s.psd", "S.PSD", "s.jpg", "s.jpeg", "s.png"} {
		if err := CheckGuide(filepath.Join(dir, ok)); err != nil {
			t.Errorf("%s: %v", ok, err)
		}
	}
	if err := CheckGuide(filepath.Join(dir, "s.psb")); !errors.Is(err, library.ErrPSB) {
		t.Fatalf("PSB: %v", err)
	}
	if err := CheckGuide(filepath.Join(dir, "s.tif")); err == nil || err.Error() != "файл разметки — только PSD, JPEG или PNG" {
		t.Fatalf("TIFF: %v", err)
	}
	if err := CheckGuide("s.psd"); err == nil {
		t.Fatal("относительный путь принят")
	}
}

// TestRelinkSpreadGuide — смена буквы диска переносит разметку разворотов (и обложки рядом),
// запасной путь снимается; файл на месте не трогается; на новом диске файла нет — путь как был.
func TestRelinkSpreadGuide(t *testing.T) {
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
	onDisk := testkit.PSD(t, filepath.Join(root, "Разметка"), "s.psd", testkit.PSDSpec{W: 4, H: 4})
	cov := testkit.PNG(t, filepath.Join(root, "Разметка"), "c.png", 4, 4, color.NRGBA{A: 255})
	p := model.New()
	p.SpreadGuide, p.SpreadGuideAlt = from+":"+onDisk[len(vol):], `C:\старое\s.psd`
	p.Cover = &model.Cover{Sheet: &model.Spread{ID: model.CoverID}, Guide: from + ":" + cov[len(vol):]}
	missing := from + `:\Нет\s.psd`
	RelinkDrive(p, from, to)
	if p.SpreadGuide != onDisk || p.SpreadGuideAlt != "" || p.Cover.Guide != cov {
		t.Fatalf("разметки: %q (запасной %q), обложка %q", p.SpreadGuide, p.SpreadGuideAlt, p.Cover.Guide)
	}
	RelinkDrive(p, to, from)
	if p.SpreadGuide != onDisk {
		t.Fatalf("разметка на месте уехала: %q", p.SpreadGuide)
	}
	p.SpreadGuide = missing
	RelinkDrive(p, from, to)
	if p.SpreadGuide != missing {
		t.Fatalf("файла нет и на новом диске — путь сменился: %q", p.SpreadGuide)
	}
}

// TestGuideInfo — сведения о разметке для вида: PSD с DPI и направляющими, ok; файла нет,
// том не отвечает или пустой путь — ok=false.
func TestGuideInfo(t *testing.T) {
	dir := testkit.TempDir(t)
	g := testkit.PSD(t, dir, "s.psd", testkit.PSDSpec{W: 10, H: 5, DPI: 254, Guides: []testkit.PSDGuide{{Pos: 5}}})
	if m, ok := GuideInfo(g); !ok || m.W != 10 || m.H != 5 || m.DPI != 254 || len(m.Guides) != 1 {
		t.Fatalf("PSD: %v %+v", ok, m)
	}
	if _, ok := GuideInfo(filepath.Join(dir, "нет.psd")); ok {
		t.Fatal("нет файла — ok")
	}
	if _, ok := GuideInfo(""); ok {
		t.Fatal("пустой путь — ok")
	}
	old := reachable
	reachable = func(string) bool { return false }
	t.Cleanup(func() { reachable = old })
	if _, ok := GuideInfo(g); ok {
		t.Fatal("том не отвечает — ok")
	}
}
