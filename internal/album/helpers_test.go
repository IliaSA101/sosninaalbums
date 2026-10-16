package album

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
	"github.com/IliaSA101/sosninaalbums/internal/text"
)

// Разметки обложки не было (диск не подключён): свежая неудача не перечитывается, через
// guideRetry файл перечитывается в фоне без e.mu, и появившийся файл показывает следующий вид.
func TestGuideRetry(t *testing.T) {
	path := filepath.Join(testkit.TempDir(t), "разметка.png")
	p := model.New()
	pr, _ := model.CoverPresetOf(model.CoverDefault)
	p.Spreads = []*model.Spread{{ID: "0123456789ab", Photos: []string{}}}
	p.Cover = &model.Cover{Spec: pr.Spec, Sheet: &model.Spread{ID: model.CoverID}, Guide: path}
	e, err := NewEditor(p, "")
	if err != nil {
		t.Fatal(err)
	}
	if v := e.View(); v.Cover == nil || v.Cover.GuideOK {
		t.Fatalf("разметки нет, а вид её показывает: %+v", v.Cover)
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 4, 3))); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	if v := e.View(); v.Cover.GuideOK {
		t.Fatal("свежая неудача перечитана сразу")
	}
	e.mu.Lock()
	busy := e.guide.busy
	e.guide.at = time.Now().Add(-guideRetry)
	e.mu.Unlock()
	if busy {
		t.Fatal("перечитывание до guideRetry")
	}
	e.View() // запускает фоновое перечитывание
	for deadline := time.Now().Add(testkit.Limit(5 * time.Second)); ; time.Sleep(5 * time.Millisecond) {
		e.mu.Lock()
		ok, busy := e.guide.OK, e.guide.busy
		e.mu.Unlock()
		if ok && !busy {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("разметка не перечитана")
		}
	}
	if v := e.View(); !v.Cover.GuideOK || !v.Cover.GuideWarn { // 4×3 — не пропорции листа
		t.Fatalf("после перечитывания: ok=%v warn=%v", v.Cover.GuideOK, v.Cover.GuideWarn)
	}
}

func ip(v int) *int { return &v }

func setMarks(t *testing.T, e *Editor, ph []model.PhotoInfo, pt model.MarkPatch) model.View {
	t.Helper()
	v, err := e.SetMarks(ph, pt)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

// markRows — Marks() в виде, удобном для сравнения: nil-поле — -9 / "-".
func markRows(e *Editor) map[string][3]any {
	out := map[string][3]any{}
	for id, m := range e.Marks() {
		r := [3]any{-9, "-", -9}
		if m.Rating != nil {
			r[0] = *m.Rating
		}
		if m.Label != nil {
			r[1] = *m.Label
		}
		if m.Flag != nil {
			r[2] = *m.Flag
		}
		out[id] = r
	}
	return out
}

func wantMarks(t *testing.T, e *Editor, want map[string][3]any) {
	t.Helper()
	if got := markRows(e); !reflect.DeepEqual(got, want) {
		t.Fatalf("отметки %v, ждали %v", got, want)
	}
}

func photo(id string) model.PhotoInfo {
	return model.PhotoInfo{ID: id, Path: "/" + id + ".jpg", W: 300, H: 200}
}

func photos(ids ...string) []model.PhotoInfo {
	out := make([]model.PhotoInfo, len(ids))
	for i, id := range ids {
		out[i] = photo(id)
	}
	return out
}

// Обёртки для тестов, где отказ не ждут: ошибка — сразу фатальная паника.
func newSpread(e *Editor, at int, ph []model.PhotoInfo) model.View {
	v, err := e.NewSpread(at, ph)
	if err != nil {
		panic(err)
	}
	return v
}

// newEditor — копия edtest.Open (внутренний тест edtest не импортирует): отказ — Fatal.
func newEditor(t testing.TB, p *model.Project, path string) *Editor {
	t.Helper()
	e, err := NewEditor(p, path)
	if err != nil {
		t.Fatal(err)
	}
	return e
}

// newTwo — проект с двумя разворотами: [a b] и [c].
func newTwo(t *testing.T) (*Editor, string, string) {
	t.Helper()
	e := newEditor(t, model.New(), "")
	newSpread(e, -1, photos("a", "b"))
	v := newSpread(e, -1, photos("c"))
	if len(v.Spreads) != 2 {
		t.Fatalf("развороты: %d", len(v.Spreads))
	}
	return e, v.Spreads[0].ID, v.Spreads[1].ID
}

// guideAltEditor — Editor с обложкой, у которой разметка не нашлась на месте: основной и
// запасной (GuideAlt) пути прочитаны из файла так же, как при Load.
func guideAltEditor(t *testing.T) (e *Editor, dir, alt string) {
	t.Helper()
	dir = testkit.TempDir(t)
	alt = filepath.Join(dir, "Старая", "c.png")
	data := fmt.Appendf(nil, `{"version":6,"settings":{"pageW":281,"pageH":284},"spreads":[],"photos":{},`+
		`"cover":{"spec":{},"guide":"Нет/c.png","guideAbs":%q}}`, alt)
	p, err := salbum.Decode(data, dir)
	if err != nil {
		t.Fatal(err)
	}
	if p.Cover.Guide != filepath.Join(dir, "Нет", "c.png") || p.Cover.GuideAlt != alt {
		t.Fatalf("подготовка: разметка %q, запасной %q", p.Cover.Guide, p.Cover.GuideAlt)
	}
	return newEditor(t, p, ""), dir, alt
}

// guideAbsOf — guideAbs и наличие "guideAlt" в записанном файле проекта.
func guideAbsOf(t *testing.T, e *Editor, dir string) (abs string, hasAlt bool) {
	t.Helper()
	out, err := salbum.Encode(e.p, dir)
	if err != nil {
		t.Fatal(err)
	}
	var f salbum.FileProject
	if err := json.Unmarshal(out, &f); err != nil {
		t.Fatal(err)
	}
	return f.Cover.GuideAbs, bytes.Contains(out, []byte("guideAlt"))
}

// SetCoverGuide: другой файл разметки — запасной путь прежнего не годится, в файл уходит
// только новый guideAbs, отмена возвращает прежний запасной; неверный путь и тот же путь
// заново — холостые правки без шага, запасной цел.
func TestSetCoverGuideResetsAlt(t *testing.T) {
	e, dir, alt := guideAltEditor(t)
	old := e.p.Cover.Guide
	rev, undo := e.rev, len(e.undo)
	if _, err := e.SetCoverGuide(`\\server\share\guide.png`); err == nil {
		t.Fatal("UNC принят")
	}
	if _, err := e.SetCoverGuide(old); err != nil {
		t.Fatal(err)
	}
	if e.rev != rev || len(e.undo) != undo || e.p.Cover.GuideAlt != alt {
		t.Fatalf("холостые правки: rev %d → %d, шагов %d → %d, запасной %q", rev, e.rev, undo, len(e.undo), e.p.Cover.GuideAlt)
	}

	fresh := filepath.Join(dir, "Новая", "c.png")
	if _, err := e.SetCoverGuide(fresh); err != nil {
		t.Fatal(err)
	}
	if c := e.p.Cover; c.Guide != fresh || c.GuideAlt != "" {
		t.Fatalf("после смены: разметка %q, запасной %q", c.Guide, c.GuideAlt)
	}
	if abs, hasAlt := guideAbsOf(t, e, dir); abs != fresh || hasAlt {
		t.Fatalf("в файле guideAbs %q (ждали %q), guideAlt есть: %v", abs, fresh, hasAlt)
	}
	e.Undo()
	if c := e.p.Cover; c.Guide != old || c.GuideAlt != alt {
		t.Fatalf("после отмены: разметка %q, запасной %q", c.Guide, c.GuideAlt)
	}
	if abs, _ := guideAbsOf(t, e, dir); abs != alt {
		t.Fatalf("после отмены guideAbs %q, ждали %q", abs, alt)
	}
}

// RelinkDrive нашёл разметку на новом диске: путь новый, запасной сброшен, в файле один
// guideAbs; файла на новом диске нет — оба пути на месте; отмена возвращает оба.
func TestRelinkDriveGuideResetsAlt(t *testing.T) {
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
	lost := from + ":" + onDisk[len(vol):]
	e, dir, alt := guideAltEditor(t)
	e.change(func(p *model.Project) { p.Cover.Guide = lost })

	e.RelinkDrive(from, to) // на новом диске файла нет — разметка не трогается
	if c := e.p.Cover; c.Guide != lost || c.GuideAlt != alt {
		t.Fatalf("файла нет: разметка %q, запасной %q", c.Guide, c.GuideAlt)
	}
	if err := os.MkdirAll(filepath.Dir(onDisk), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(onDisk, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	e.RelinkDrive(from, to)
	if c := e.p.Cover; c.Guide != onDisk || c.GuideAlt != "" {
		t.Fatalf("файл найден: разметка %q (ждали %q), запасной %q", c.Guide, onDisk, c.GuideAlt)
	}
	if abs, hasAlt := guideAbsOf(t, e, dir); abs != onDisk || hasAlt {
		t.Fatalf("в файле guideAbs %q (ждали %q), guideAlt есть: %v", abs, onDisk, hasAlt)
	}
	e.Undo()
	if c := e.p.Cover; c.Guide != lost || c.GuideAlt != alt {
		t.Fatalf("после отмены: разметка %q, запасной %q", c.Guide, c.GuideAlt)
	}
}

// headFits — шаблонный текст шапки в стиле st кеглем pt по ширине полосы b влезает в её высоту
// и не рвёт слова.
func headFits(st model.TextStyle, pt float64, b layout.Rect) bool {
	st.Size = pt
	box := text.Cached(model.VigHeadText, st, b.W)
	var got []string
	for _, l := range box.Lines {
		got = append(got, strings.Fields(l.Text)...)
	}
	return box.H <= b.H && slices.Equal(got, strings.Fields(model.VigHeadText))
}

// Кегль шапки в ряду (049 §3.2): наибольший целый, при котором шаблонный текст влезает в полосу
// и не рвёт слова, 12…min(48, полоса на две строки); пустая и крошечная полоса — 12.
func TestHeadFit(t *testing.T) {
	plain := model.PlainStyle()
	serif, bold := plain, plain
	serif.Font, bold.Bold = "ptserif", true
	bands := []layout.Rect{{}, {W: 5, H: 300}, {W: 30, H: 60}, {W: 47.1, H: 38.6}, {W: 80, H: 60}, {W: 125, H: 70}, {W: 600, H: 200}, {W: 600, H: 10}}
	for _, st := range []model.TextStyle{plain, serif, bold} {
		for _, b := range bands {
			got, top := headFit(b, st), math.Round(min(max(b.H/2.6/0.3528, 12), 48))
			if got < 12 || got > top || got > 12 && !headFits(st, got, b) {
				t.Fatalf("%s %v: кегль %v вне 12…%v или не влезает", st.Font, b, got, top)
			}
			if got < top && headFits(st, got+1, b) {
				t.Fatalf("%s %v: кегль %v не наибольший — %v тоже влезает", st.Font, b, got, got+1)
			}
		}
	}
	if headFit(layout.Rect{W: 600, H: 200}, plain) != 48 || headFit(layout.Rect{}, plain) != 12 {
		t.Fatal("пределы кегля шапки 12 и 48 не держатся")
	}
}

// vigPlan меряет шапку в ряду гарнитурой надписи шапки листа, а не шрифтом по умолчанию:
// иначе кегль «по полосе» у шапки другим шрифтом выходил бы за полосу или мельче нужного.
func TestVigPlanHeadFitFont(t *testing.T) {
	p := NewVignetteSheet(204, 304)
	s := p.Spreads[0]
	v := *s.Vig
	v.Tpl, v.Pupils = "t-left-cell", 40
	h := s.VigHead(v.Head)
	if h == nil {
		t.Fatal("у листа нет шапки")
	}
	h.Style.Font, h.Style.Italic = "ptserif", true
	pl := vigPlan(p, s, v)
	if !pl.HeadCell || pl.Head == nil {
		t.Fatalf("«t-left-cell»: шапки в ряду нет: %+v", pl.Head)
	}
	serif := model.PlainStyle()
	serif.Font, serif.Italic = "ptserif", true
	want, def := headFit(*pl.Head, serif), headFit(*pl.Head, model.PlainStyle())
	if pl.HeadFit != want || want == def {
		t.Fatalf("кегль %v, ждали %v гарнитурой шапки (умолчание дало бы %v)", pl.HeadFit, want, def)
	}
	v.Head = "" // шапки у листа нет — новая встанет шрифтом по умолчанию
	if got := vigPlan(p, s, v).HeadFit; got != def {
		t.Fatalf("без шапки: кегль %v, ждали %v", got, def)
	}
	v.Tpl = "head" // полоса сверху — без замера
	if got := vigPlan(p, s, v).HeadFit; got != 0 {
		t.Fatalf("полоса сверху: замер %v", got)
	}
}
