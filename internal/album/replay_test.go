package album_test

// Голден-реплей поведения Editor (023 Ш0, docs/specs/0.8/023-editor-core/critic.md п.7):
// каждый публичный метод хотя бы раз по фикстуре, после правки — View, глубина отмены
// и возврата, rev и sha256 байтов Save. Ловит дрейф поведения при переносе ядра,
// который точечные тесты не видят. Эталон — testdata/replay.json (строка — шаг),
// перезапись: go test ./internal/album -run Replay -update (флаг общий с marks_file_test.go).
//
// Чтения в сценарии тоже есть (их результат — короткий хэш нормализованного JSON);
// отдельными шагами не зовутся только View (его отдаёт каждая правка) и Save (после
// каждой правки). Новый публичный метод без шага роняет тест.

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// replayStep — строка эталона. View и Save пусты у чтений: альбом они не меняют
// (rev и глубины это подтверждают).
type replayStep struct {
	Step string          `json:"step"`
	Err  string          `json:"err,omitempty"`
	Out  any             `json:"out,omitempty"` // прочий результат правки или хэш чтения
	Undo int             `json:"undo"`
	Redo int             `json:"redo"`
	Rev  int             `json:"rev"`
	Save string          `json:"save,omitempty"`
	View json.RawMessage `json:"view,omitempty"`
}

// replay — запись сценария. Случайные ID (NewID: 12 hex) заменяются на id1, id2…
// по первому появлению; marksRev — из общего счётчика процесса, заменяется на
// порядковый номер; путь файла — на <album>. ID фото — library.IDFor от путей
// на диске Q:, от машины не зависят и остаются как есть.
type replay struct {
	t     *testing.T
	e     *album.Editor
	dir   string
	file  string
	ids   map[string]string
	revs  map[int]int
	steps []replayStep
	v     model.View // View последней правки: из него берутся ID для следующих шагов
}

var replayID = regexp.MustCompile(`"[0-9a-f]{12}"`)

func newReplay(t *testing.T) *replay {
	dir := testkit.TempDir(t)
	return &replay{t: t, e: edtest.Open(t, model.New(), ""), dir: dir, file: filepath.Join(dir, "album.salbum"),
		ids: map[string]string{}, revs: map[int]int{}}
}

func (r *replay) norm(data []byte) []byte {
	return replayID.ReplaceAllFunc(data, func(m []byte) []byte {
		k := string(m)
		if _, ok := r.ids[k]; !ok {
			r.ids[k] = fmt.Sprintf(`"id%d"`, len(r.ids)+1)
		}
		return []byte(r.ids[k])
	})
}

// depth — глубина отмены и возврата: публичного API для неё нет, поле Editor
// читается только здесь (при переносе ядра правится одна строка).
func (r *replay) depth() (int, int) {
	edtest.Mu(r.e).Lock()
	defer edtest.Mu(r.e).Unlock()
	return edtest.UndoLen(r.e), edtest.RedoLen(r.e)
}

func (r *replay) push(s replayStep) {
	s.Undo, s.Redo = r.depth()
	s.Rev = r.e.View().Rev
	r.steps = append(r.steps, s)
}

// edit — шаг-правка: View правки, затем Save и хэш записанного.
func (r *replay) edit(name string, v model.View, out any, err error) {
	r.t.Helper()
	r.v = v
	v.Path = strings.Replace(v.Path, r.file, "<album>", 1)
	if _, ok := r.revs[v.MarksRev]; !ok {
		r.revs[v.MarksRev] = len(r.revs)
	}
	v.MarksRev = r.revs[v.MarksRev]
	data, jerr := json.Marshal(v)
	if jerr != nil {
		r.t.Fatalf("%s: %v", name, jerr)
	}
	s := replayStep{Step: name, Out: out, View: r.norm(data)}
	if err != nil {
		s.Err = err.Error()
	}
	if serr := r.e.Save(r.file); serr != nil {
		r.t.Fatalf("%s: Save: %v", name, serr)
	}
	s.Save = r.hashFile(name, r.file, 32)
	r.push(s)
}

// hashFile — sha256 нормализованных байтов файла, первые n байт хэша в hex.
func (r *replay) hashFile(name, path string, n int) string {
	r.t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		r.t.Fatalf("%s: %v", name, err)
	}
	sum := sha256.Sum256(r.norm(raw))
	return hex.EncodeToString(sum[:n])
}

// Обёртки под сигнатуры методов: r.ve("X")(e.X(...)) пишет шаг вместе с ошибкой.
func (r *replay) do(name string, v model.View) { r.edit(name, v, nil, nil) }

func (r *replay) ve(name string) func(model.View, error) {
	return func(v model.View, err error) { r.edit(name, v, nil, err) }
}

func (r *replay) vn(name string) func(model.View, int) {
	return func(v model.View, n int) { r.edit(name, v, n, nil) }
}

func (r *replay) vne(name string) func(model.View, int, error) {
	return func(v model.View, n int, err error) { r.edit(name, v, n, err) }
}

// read — шаг-чтение: короткий хэш нормализованного JSON результата.
func (r *replay) read(name string, res any) {
	r.t.Helper()
	data, err := json.Marshal(res)
	if err != nil {
		r.t.Fatalf("%s: %v", name, err)
	}
	sum := sha256.Sum256(r.norm(data))
	r.push(replayStep{Step: name, Out: hex.EncodeToString(sum[:8])})
}

// write — шаг-запись в сторонний файл (черновик, копия): хэш записанного.
func (r *replay) write(name string, err error, path string) {
	r.t.Helper()
	if err != nil {
		r.t.Fatalf("%s: %v", name, err)
	}
	r.push(replayStep{Step: name, Out: r.hashFile(name, path, 8)})
}

// spreadOf — ID разворота с фото id (в раскладке, на месте или на фоне).
func (r *replay) spreadOf(id string) string {
	r.t.Helper()
	for _, s := range r.v.Spreads {
		for _, p := range s.Photos {
			if p == id {
				return s.ID
			}
		}
		for _, b := range s.Backdrops {
			if b.Photo == id {
				return s.ID
			}
		}
	}
	r.t.Fatalf("фото %s ни на одном развороте", id)
	return ""
}

func (r *replay) spread(id string) model.SpreadView {
	r.t.Helper()
	for _, s := range r.v.Spreads {
		if s.ID == id {
			return s
		}
	}
	r.t.Fatalf("нет разворота %s", id)
	return model.SpreadView{}
}

// emptyPlace — ID первого пустого места разворота.
func (r *replay) emptyPlace(sid string) string {
	r.t.Helper()
	for _, pl := range r.spread(sid).Places {
		if pl.Photo == "" {
			return pl.ID
		}
	}
	r.t.Fatalf("на %s нет пустых мест", sid)
	return ""
}

// lastText — последний блок текста разворота.
func (r *replay) lastText(sid string) model.TextView {
	r.t.Helper()
	ts := r.spread(sid).Texts
	if len(ts) == 0 {
		r.t.Fatalf("на %s нет текстов", sid)
	}
	return ts[len(ts)-1]
}

// replayPhoto — фото ленты с диска Q: (ID как у сканера); kind задаёт пропорции.
func replayPhoto(n int, kind byte, x model.XmpMarks) model.PhotoInfo {
	ph := edtest.Lib(fmt.Sprintf(`Q:\Свадьба\%02d.jpg`, n), x)
	switch kind {
	case 'v':
		ph.W, ph.H = 2000, 3000
	case 's':
		ph.W, ph.H = 2400, 2400
	default:
		ph.W, ph.H = 3000, 2000
	}
	ph.Size = int64(1000 + n)
	return ph
}

// replayScenario — весь сценарий; ph[1…18] — фото ленты (ph[0] не используется).
func replayScenario(t *testing.T) *replay {
	r := newReplay(t)
	kinds := "hvhshvvhhshvhvhsvh"
	ph := make([]model.PhotoInfo, len(kinds)+1)
	for i := 1; i < len(ph); i++ {
		x := model.XmpMarks{}
		if i == 3 {
			x.Rating = 2
		}
		ph[i] = replayPhoto(i, kinds[i-1], x)
	}
	q := replayAlbum(r, ph)
	replayGrid(r, ph)
	replayCover(r, ph)
	replayVignette(r, ph)
	replayTail(r, ph, q)
	return r
}

// replayAlbum — развороты, раскладка, кадр, тексты, подписи, отметки, фон.
// Возвращает снимок альбома для Restore в конце.
func replayAlbum(r *replay, ph []model.PhotoInfo) *model.Project {
	e, id := r.e, func(i int) string { return ph[i].ID }
	r.ve("AddFolder")(e.AddFolder(`Q:\Свадьба`))
	r.ve("AddFolder:второй")(e.AddFolder(`Q:\Банкет`))
	r.ve("AddFolder:относительный")(e.AddFolder(`Банкет`))
	r.ve("NewSpread")(e.NewSpread(-1, ph[1:4]))
	r.ve("NewSpread:второй")(e.NewSpread(-1, ph[4:6]))
	if ap, err := e.AutoPlan(ph[6:12], model.AutoOpts{Spreads: 2, MaxPer: 64}); err != nil || ap.Spreads != 2 {
		r.t.Fatalf("AutoPlan: %+v %v", ap, err)
	} else {
		r.read("AutoPlan", ap)
	}
	r.ve("AutoBuild")(e.AutoBuild(ph[6:12], -1, model.AutoOpts{Spreads: 2, MaxPer: 64}))
	s0 := r.spreadOf(id(1))
	r.ve("AddPhotos")(e.AddPhotos(s0, ph[12:13], 1))
	r.ve("MovePhoto")(e.MovePhoto(id(5), s0, 0))
	r.do("SwapPhotos", e.SwapPhotos(id(1), id(4)))
	st := r.v.Settings
	st.Gap, st.Bg, st.Layflat = 4, "#f4f0ea", false
	r.do("SetSettings", e.SetSettings(st))

	ch := e.LayoutChoices(s0)
	r.read("LayoutChoices", ch)
	r.read("MoreLayouts", e.MoreLayouts(s0, len(ch.Whole.InOrder)))
	if n := len(ch.Whole.InOrder); n > 1 {
		r.do("SetLayout", e.SetLayout(s0, ch.Whole.InOrder[n-1].Key))
	} else {
		r.t.Fatalf("у %s одна раскладка", s0)
	}
	r.do("StepLayout", e.StepLayout(s0, "spread", 1))
	r.do("StepLayout:левая", e.StepLayout(s0, "left", -1))
	r.do("SetFrame", e.SetFrame(id(2), 1.6, 0.4, -0.3))
	r.do("SetFrameAngle", e.SetFrameAngle(id(2), 7.5))
	r.do("TurnPhotos", e.TurnPhotos([]string{id(3)}, 1))
	r.do("ResetFrame", e.ResetFrame(id(2)))
	r.do("SetFlip", e.SetFlip([]string{id(2), id(3)}, "h"))
	r.do("SetBackground", e.SetBackground(s0, "left", "#ffeedd"))

	r.ve("AddText")(e.AddText(s0, 0, "t1"))
	tid := r.lastText(s0).ID
	r.do("SetText", e.SetText(tid, "Анна и Пётр", "t1"))
	r.do("SetText:слияние", e.SetText(tid, "Анна и Пётр\nвенчание", "t1"))
	stl := r.lastText(s0).Style
	stl.Font, stl.Align, stl.Italic, stl.Color = "cormorant", "center", true, "#552211"
	r.do("SetTextStyle", e.SetTextStyle(tid, stl))
	r.do("SetTextBox", e.SetTextBox(tid, 30, 40, 90))
	r.do("SetTextAngle", e.SetTextAngle(tid, 12))
	tv := r.lastText(s0)
	r.do("SetTextScale", e.SetTextScale(tid, tv.Style.Size*1.5, tv.X-5, tv.Y-5))
	r.do("SetTextAuto", e.SetTextAuto(tid))
	r.do("SetSpreadName", e.SetSpreadName(s0, "Сборы"))
	r.do("SetCaption", e.SetCaption(id(3), "Кольца", "c1"))
	cs := model.NewCaptionStyle(r.v.Settings.PageH)
	cs.Bold, cs.Align = true, "right"
	r.do("SetCaptionStyle", e.SetCaptionStyle(id(3), cs, true, 1))
	r.do("SetCaption:соседняя", e.SetCaption(id(4), "Утро", "c2"))
	// Поля подписей (030): поле шапки, имя, список, стиль листа с шаблоном.
	r.do("SetCaption:поле", e.SetCaption(id(4), "Утро {Школа}, {Класс}", "c3"))
	r.do("SetPerson", e.SetPerson(id(3), model.Person{Last: "Иванов", First: "Петя"}, "n1"))
	pv, _ := e.Person(id(3))
	r.read("Person", pv)
	v, extra, missing, err := e.PasteNames(s0, "1. Сидоров Иван\nПетрова Анна Сергеевна", false)
	r.edit("PasteNames", v, []int{extra, missing}, err)
	r.do("SetSheetCaps", e.SetSheetCaps(s0, true, model.SheetCaps{Style: cs, Pad: 2, Text: "{Фамилия}\n{Имя}"}))

	r.ve("SetMarks")(e.SetMarks(ph[1:3], model.MarkPatch{Rating: edtest.Ip(4)}))
	r.ve("SetMarks:метка")(e.SetMarks(ph[1:2], model.MarkPatch{Label: edtest.Sp("red"), Toggle: true}))
	r.ve("SetMarks:шаг")(e.SetMarks(ph[3:4], model.MarkPatch{Step: 1}))
	r.ve("SetMarks:флаг")(e.SetMarks(ph[4:6], model.MarkPatch{Flag: edtest.Ip(-1)}))
	r.read("Marks", e.Marks())
	r.do("ResetMarks", e.ResetMarks([]string{id(2)}))
	r.do("Undo", e.Undo())
	r.do("Undo:правка", e.Undo())
	r.do("Redo", e.Redo())

	sb := r.spreadOf(id(1))
	r.ve("SetBackdrop")(e.SetBackdrop(sb, model.ScopeSpread, ph[13]))
	r.ve("SetBackdrop:область")(e.SetBackdrop(sb, "top", ph[13]))
	r.do("SetBackdropBox", e.SetBackdropBox(sb, model.ScopeSpread, 10, 5, 500, 260, 4, "b1"))
	r.do("SetBackdropBox:слияние", e.SetBackdropBox(sb, model.ScopeSpread, 14, 7, 500, 260, 4, "b1"))
	r.do("SetBackdropFrame", e.SetBackdropFrame(sb, model.ScopeSpread, 1.3, 0.2, -0.1))
	r.do("FlipBackdrop", e.FlipBackdrop(sb, model.ScopeSpread, "v"))
	r.do("SetBackdropFade", e.SetBackdropFade(sb, model.ScopeSpread, 0.35, "f1"))
	r.do("FitBackdrop", e.FitBackdrop(sb, model.ScopeSpread))
	sc := r.spreadOf(id(6))
	r.ve("SetBackdrop:левая")(e.SetBackdrop(sc, model.ScopeLeft, ph[14]))
	r.do("ClearBackdrop", e.ClearBackdrop(sc, model.ScopeLeft))
	r.do("Undo:фон", e.Undo())

	q, _ := e.Snapshot()
	r.read("Snapshot", q)
	return q
}

// replayGrid — сетка: места, группа, выравнивание, шаблоны, копия и размножение.
func replayGrid(r *replay, ph []model.PhotoInfo) {
	e, id := r.e, func(i int) string { return ph[i].ID }
	sb := r.spreadOf(id(1))
	r.do("SetGrid", e.SetGrid(sb, true))
	r.ve("AddPlace")(e.AddPlace(sb, 1))
	r.ve("AddPlace:левая")(e.AddPlace(sb, 0))
	r.do("SetPlaces", e.SetPlaces(sb, []model.PlaceBox{{ID: id(1), X: 20, Y: 30, W: 200, H: 150}}, "g1"))
	r.do("SetPlaces:слияние", e.SetPlaces(sb, []model.PlaceBox{{ID: id(1), X: 25, Y: 30, W: 200, H: 150}}, "g1"))
	r.do("SetPlaceAngle", e.SetPlaceAngle(sb, id(1), 12))
	r.do("SetPlaceFit", e.SetPlaceFit(sb, []string{id(1)}, model.FitContain))
	r.ve("ArrangePlaces")(e.ArrangePlaces(sb, "splitH", []string{r.emptyPlace(sb)}))
	r.do("ArrangeLayers", e.ArrangeLayers(sb, "front", []string{id(1)}))
	r.ve("AddText:сетка")(e.AddText(sb, 1, "t2"))
	t2 := r.lastText(sb).ID
	r.vne("PlacePhotos")(e.PlacePhotos(sb, r.emptyPlace(sb), ph[15:17]))
	r.vn("FillPlaces")(e.FillPlaces(ph[16:18], sb))

	group := []string{id(1), id(15), t2}
	r.read("AlignOverlap", e.AlignOverlap(sb, group, "distH", "spread"))
	r.do("AlignItems", e.AlignItems(sb, group, "top", "page"))
	r.do("TransformGroup", e.TransformGroup(sb, group, 0.9, 20, 20, 3, 2, ""))
	pw, phh := r.v.Settings.PageW, r.v.Settings.PageH
	r.do("RotateGroup", e.RotateGroup(sb, group, 10, pw, phh/2))
	r.do("ScaleGroupAxes", e.ScaleGroupAxes(sb, group, 1.1, 0.9, 0, 20, 20))
	r.do("MirrorGrid", e.MirrorGrid(sb, model.ScopeSpread))
	r.do("DeletePlaces", e.DeletePlaces(sb, []string{id(16)}))

	s0 := r.spreadOf(id(2))
	boxes, mode, err := e.PlaceBoxes(s0, model.ScopeLeft)
	r.read("PlaceBoxes", []any{boxes, mode, fmt.Sprint(err)})
	tpls := []model.Template{{ID: "tpl", Name: "Два", Scope: model.TplPage, Boxes: []model.Box{{X: 0, Y: 0, W: 1, H: 0.5}, {X: 0, Y: 0.5, W: 1, H: 0.5}}}}
	r.read("GridChoices", e.GridChoices(s0, 0, model.ScopeSpread, tpls))

	sc := r.spreadOf(id(6))
	r.vne("ApplyGrid")(e.ApplyGrid(sc, model.ScopeLeft, model.TplPage, tpls[0].Boxes))
	r.vne("ApplyGrid:отказ")(e.ApplyGrid(sc, model.ScopeLeft, model.TplSpread, tpls[0].Boxes))
	targets := []model.GridTarget{{Spread: sc, Scope: model.ScopeRight}}
	r.read("PlanCopyGrid", e.PlanCopyGrid(sb, model.ScopeLeft, targets, true, true))
	r.vne("CopyGrid")(e.CopyGrid(sb, model.ScopeLeft, targets, true, true))
	r.ve("DuplicateGrid")(e.DuplicateGrid(sb, 2, true))
	r.do("DeleteSelection", e.DeleteSelection(sb, []string{id(15)}, []string{r.spread(sb).Places[1].ID}, []string{t2}))
	r.do("SetGrid:авто", e.SetGrid(sb, false))
	r.do("Undo:авто", e.Undo())
}

// replayCover — обложка (032): создание с фото на лицевой, пресет, свои числа, раскладки,
// надпись на корешке, разметка, место с фото. Обложка остаётся до конца: хвост (перепривязка,
// удаление разворотов — Reflow, восстановление) идёт с ней.
func replayCover(r *replay, ph []model.PhotoInfo) {
	e := r.e
	r.ve("CreateCover")(e.CreateCover("", &ph[16]))
	r.ve("CreateCover:повтор")(e.CreateCover("", nil))
	r.ve("ApplyCoverPreset")(e.ApplyCoverPreset("netprint-premium"))
	r.ve("ApplyCoverPreset:неизвестный")(e.ApplyCoverPreset("nope"))
	spec := r.v.Cover.Spec
	spec.W, spec.Hinge = 290, 6
	r.ve("SetCoverSpec")(e.SetCoverSpec(spec))
	r.ve("SetCoverLayout")(e.SetCoverLayout("wrap"))
	r.ve("SetCoverLayout:обе")(e.SetCoverLayout("both"))
	r.ve("SetBackdrop:задняя")(e.SetBackdrop(model.CoverID, model.ScopeLeft, ph[17]))
	r.ve("CoverSpineText")(e.CoverSpineText())
	r.ve("CoverSpineText:повтор")(e.CoverSpineText())
	r.do("SetCoverSpineBg", e.SetCoverSpineBg("#203040"))
	r.ve("SetCoverGuide")(e.SetCoverGuide(`Q:\Разметка\cover.png`))
	r.read("CoverGuidePath", e.CoverGuidePath())
	r.ve("SetCoverGuide:UNC")(e.SetCoverGuide(`\\server\share\cover.png`))
	r.do("ClearCoverGuide", e.ClearCoverGuide())
	r.ve("AddPlace:обложка")(e.AddPlace(model.CoverID, 1))
	r.vne("PlacePhotos:обложка")(e.PlacePhotos(model.CoverID, r.v.Cover.Sheet.Places[0].ID, ph[18:19]))
	r.do("DeleteCover", e.DeleteCover())
	r.do("Undo:обложка", e.Undo())
}

// replayVignette — виньетка (031): новая по умолчанию, превью, «Применить» с фото и сверх
// предела, сортировка, «Обычный разворот» и его отмена (виньетка идёт в хвост).
func replayVignette(r *replay, ph []model.PhotoInfo) {
	e := r.e
	r.ve("AddVignette")(e.AddVignette(-1))
	sid := r.v.Spreads[len(r.v.Spreads)-1].ID
	r.read("VignetteChoices", e.VignetteChoices(sid, 2, 10, "4:5", nil))
	r.vne("SetVignette")(e.SetVignette(sid, model.Vignette{Tpl: "g-top", Teachers: 2, Pupils: 10, Ratio: "4:5"}, nil))
	r.vn("FillPlaces:виньетка")(e.FillPlaces(ph[17:19], sid))
	r.vne("SetVignette:предел")(e.SetVignette(sid, model.Vignette{Tpl: "grid", Teachers: 18, Pupils: 64}, nil))
	// Окно «Настроить виньетку»: сухой прогон (чтение) и «Применить» с полями шапки.
	setup := model.VigSetup{Vig: model.Vignette{Tpl: "head-g-top", Teachers: 1, Pupils: 12, Ratio: "3:4"},
		Vars: model.Vars{Title: "Гимназия № 1", Year: "2026"}}
	pr, err := e.PreviewVignette(sid, setup, nil)
	if err != nil {
		r.t.Fatalf("PreviewVignette: %v", err)
	}
	r.read("PreviewVignette", pr)
	r.vne("SetupVignette")(e.SetupVignette(sid, setup, nil))
	// Свой шаблон (038): лист → шаблон → «Применить» своего, «Вернуть авто».
	tpl, err := e.VigTemplate(sid)
	if err != nil {
		r.t.Fatalf("VigTemplate: %v", err)
	}
	tpl.ID, tpl.Name = "tpl001", "Мой"
	r.read("VigTemplate", tpl)
	r.vne("SetVignette:свой")(e.SetVignette(sid, model.Vignette{Tpl: "my:tpl001", Teachers: 1, Pupils: 14}, []model.Template{tpl}))
	r.vne("ResetVignette")(e.ResetVignette(sid))
	r.do("SortVignette", e.SortVignette(sid))
	r.do("ClearVignette", e.ClearVignette(sid))
	r.do("Undo:виньетка", e.Undo())
}

// replayTail — развороты, папки, перепривязка, восстановление, удаление, запись копий.
func replayTail(r *replay, ph []model.PhotoInfo, q *model.Project) {
	e, id := r.e, func(i int) string { return ph[i].ID }
	s0 := r.spreadOf(id(2))
	r.do("SplitSpread", e.SplitSpread(s0, 2))
	r.do("MergeSpreads", e.MergeSpreads(s0))
	r.do("MoveSpread", e.MoveSpread(r.v.Spreads[len(r.v.Spreads)-1].ID, 0))
	r.do("RemovePhotos", e.RemovePhotos([]string{id(12)}))
	r.do("DeleteText", e.DeleteText(r.lastText(s0).ID))

	r.read("Refs", e.Refs())
	ref, ok := e.Ref(id(2))
	r.read("Ref", []any{ref, ok})
	r.read("Folders", e.Folders())
	r.read("Path", strings.Replace(e.Path(), r.file, "<album>", 1))
	r.do("Relink", e.Relink(map[string]string{id(7): `Q:\Перенос\07.jpg`}, `Q:\Перенос`))
	// Сменить букву по-настоящему можно только с файлами на двух дисках
	// (editor_relink_test.go): здесь на R: ничего нет — холостая.
	r.do("RelinkDrive", e.RelinkDrive("Q", "R"))
	r.do("RemoveFolder", e.RemoveFolder(`Q:\Банкет`))
	r.ve("Restore")(e.Restore(q))
	r.do("Undo:восстановление", e.Undo())

	r.do("DeleteSpread", e.DeleteSpread(r.spreadOf(id(9))))
	r.do("DeleteSpreads", e.DeleteSpreads([]string{r.v.Spreads[0].ID, r.v.Spreads[1].ID}))
	r.do("Undo:удаление", e.Undo())
	r.do("Undo:ещё", e.Undo())
	r.do("Redo:удаление", e.Redo())
	r.do("Redo:второй", e.Redo())
	r.do("Redo:холостой", e.Redo())

	cp := filepath.Join(r.dir, "copy.salbum")
	r.write("WriteCopy", e.WriteCopy(cp), cp)
	dr := filepath.Join(r.dir, "draft.salbum")
	r.write("WriteDraft", e.WriteDraft(dr), dr)
	r.read("PhotosOf", e.PhotosOf([]string{s0, "нет"}))
	// «Разнообразить» — на развороте «Авто» с оценкой (развороты сценария — сетки, там шаг холостой):
	// 5★ у третьего фото (XMP) делает его главным, раскладка должна смениться.
	r.ve("NewSpread:авто")(e.NewSpread(-1, ph[6:10]))
	last := func() model.SpreadView { return r.v.Spreads[len(r.v.Spreads)-1] }
	sid, was := last().ID, last().Layout
	rated := slices.Clone(ph[6:10])
	rated[2].Xmp.Rating = 5
	r.do("VaryLayouts", e.VaryLayouts([]string{sid}, model.VarietyNormal, rated))
	if last().ID != sid || last().Layout == was {
		r.t.Fatalf("VaryLayouts: раскладка %q не сменилась", was)
	}
	// Бросок 042 (в конце: прежние шаги эталона не сдвигаются): у закреплённого разворота одно
	// пустое место, три новых фото — вопрос без шага, ответ «add» — место и плитка одним шагом.
	// ID новых мест случайны: в out — только числа.
	r.ve("AddPlace:042")(e.AddPlace(sid, 1))
	more := []model.PhotoInfo{replayPhoto(19, 'h', model.XmpMarks{}), replayPhoto(20, 'v', model.XmpMarks{}), replayPhoto(21, 'h', model.XmpMarks{})}
	for _, rest := range []string{"", "add"} {
		v, o, err := e.DropPhotos(sid, "", more, 0.75, rest)
		r.edit("DropPhotos:"+rest, v, fmt.Sprintf("ask=%v fit=%d total=%d fresh=%d grown=%d out=%v left=%d small=%v",
			o.Ask, o.Fit, o.Total, len(o.Fresh), o.Grown, o.Out, o.Left, o.Small), err)
	}
	// Имена своих шаблонов (QA 038): своих шаблонов на листах нет — холостая, без шага и rev.
	v, changed := e.SyncOwnNames([]model.Template{{ID: "0123456789ab", Name: "Свой", Kind: model.TplVignette}})
	r.edit("SyncOwnNames", v, changed, nil)
}

func TestEditorReplay(t *testing.T) {
	edtest.SkipNotWindows(t) // пути фото и папок — с буквой диска
	r := replayScenario(t)
	// Шаг на каждый публичный метод Editor, кроме View и Save (они идут в каждом
	// шаге-правке): новый метод без шага — провал.
	seen := map[string]bool{"View": true, "Save": true}
	for _, s := range r.steps {
		seen[strings.SplitN(s.Step, ":", 2)[0]] = true
	}
	typ := reflect.TypeFor[*album.Editor]()
	for i := range typ.NumMethod() {
		if name := typ.Method(i).Name; !seen[name] {
			t.Errorf("метод %s без шага в реплее", name)
		}
	}
	var buf bytes.Buffer
	buf.WriteString("[\n")
	for i, s := range r.steps {
		line, err := json.Marshal(s)
		if err != nil {
			t.Fatal(err)
		}
		buf.Write(line)
		if i < len(r.steps)-1 {
			buf.WriteByte(',')
		}
		buf.WriteByte('\n')
	}
	buf.WriteString("]\n")
	compareReplay(t, filepath.Join("testdata", "replay.json"), buf.Bytes())
}

// compareReplay сверяет с эталоном построчно (строка — шаг) и называет первый расходящийся шаг.
func compareReplay(t *testing.T, path string, got []byte) {
	t.Helper()
	if *edtest.Update {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (создать: -update)", err)
	}
	if bytes.Equal(got, want) {
		return
	}
	gl, wl := strings.Split(string(got), "\n"), strings.Split(string(want), "\n")
	for i := range max(len(gl), len(wl)) {
		var g, w string
		if i < len(gl) {
			g = gl[i]
		}
		if i < len(wl) {
			w = wl[i]
		}
		if g == w {
			continue
		}
		at := 0
		for at < min(len(g), len(w)) && g[at] == w[at] {
			at++
		}
		cut := func(s string) string { return s[max(0, at-150):min(len(s), at+150)] }
		t.Fatalf("строка %d разошлась с эталоном (символ %d):\nесть:  …%s…\nждали: …%s…", i, at, cut(g), cut(w))
	}
	t.Fatal("эталон отличается (перевод строк?)")
}
