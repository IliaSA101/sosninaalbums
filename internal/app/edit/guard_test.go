package edit

import (
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Как метод реагирует на панику в Editor.
const (
	kView   = iota // правка без ошибки: текущий вид + тост ErrInternal
	kErr           // правка с ошибкой: текущий вид + ErrInternal, тоста нет
	kRead          // только чтение: паника гаснет в Guard, вызывающий получает нулевое значение
	kReadEr        // чтение с ошибкой: ErrInternal (GuardErr)
	kStatic        // статика: Editor не нужен
)

type probe struct {
	name string
	kind int
	call func(m *Methods) (model.View, error)
}

func ok(v model.View) (model.View, error) { return v, nil }

// probes — все экспортируемые методы Methods с безобидными аргументами.
func probes() []probe {
	var (
		ids  = []string{"a"}
		tst  model.TextStyle
		none = func(m *Methods) (model.View, error) { return model.View{}, nil }
	)
	return []probe{
		// sheets.go
		{"CoverPresets", kStatic, func(m *Methods) (model.View, error) { m.CoverPresets(); return none(m) }},
		{"PagePresets", kStatic, func(m *Methods) (model.View, error) { m.PagePresets(); return none(m) }},
		{"CreateCover", kErr, func(m *Methods) (model.View, error) { return m.CreateCover("", "") }},
		{"DeleteCover", kView, func(m *Methods) (model.View, error) { return ok(m.DeleteCover()) }},
		{"ApplyCoverPreset", kErr, func(m *Methods) (model.View, error) { return m.ApplyCoverPreset("x") }},
		{"ApplyPrint", kErr, func(m *Methods) (model.View, error) { return m.ApplyPrint(model.Settings{}, "x") }},
		{"SetCoverSpec", kErr, func(m *Methods) (model.View, error) { return m.SetCoverSpec(model.CoverSpec{}) }},
		{"SetCoverPx", kErr, func(m *Methods) (model.View, error) { return m.SetCoverPx(8031, 2835) }},
		{"SetCoverLayout", kErr, func(m *Methods) (model.View, error) { return m.SetCoverLayout("front") }},
		{"CoverSpineText", kErr, func(m *Methods) (model.View, error) { return m.CoverSpineText() }},
		{"SetCoverSpineBg", kView, func(m *Methods) (model.View, error) { return ok(m.SetCoverSpineBg("#112233")) }},
		{"ClearCoverGuide", kView, func(m *Methods) (model.View, error) { return ok(m.ClearCoverGuide()) }},
		{"ClearSpreadGuide", kView, func(m *Methods) (model.View, error) { return ok(m.ClearSpreadGuide()) }},
		{"StepCoverGrid", kView, func(m *Methods) (model.View, error) { return ok(m.StepCoverGrid("right", 1)) }},
		{"VignetteChoices", kRead, func(m *Methods) (model.View, error) { m.VignetteChoices("s1", 1, 1, "2:3"); return none(m) }},
		{"PreviewVignette", kReadEr, func(m *Methods) (model.View, error) {
			_, err := m.PreviewVignette("s1", model.VigSetup{})
			return model.View{}, err
		}},
		{"AddVignette", kErr, func(m *Methods) (model.View, error) { return m.AddVignette(0) }},
		{"ClearVignette", kView, func(m *Methods) (model.View, error) { return ok(m.ClearVignette("s1")) }},
		{"SortVignette", kView, func(m *Methods) (model.View, error) { return ok(m.SortVignette("s1")) }},
		// spreads.go
		{"AddPhotos", kErr, func(m *Methods) (model.View, error) { return m.AddPhotos("s1", ids, 0) }},
		{"NewSpread", kErr, func(m *Methods) (model.View, error) { return m.NewSpread(0, nil) }},
		{"MovePhoto", kErr, func(m *Methods) (model.View, error) { return m.MovePhoto("a", "s2", 0) }},
		{"SwapPhotos", kView, func(m *Methods) (model.View, error) { return ok(m.SwapPhotos("a", "b")) }},
		{"RemovePhotos", kView, func(m *Methods) (model.View, error) { return ok(m.RemovePhotos(ids)) }},
		{"DeleteSpread", kView, func(m *Methods) (model.View, error) { return ok(m.DeleteSpread("s1")) }},
		{"DeleteSpreads", kView, func(m *Methods) (model.View, error) { return ok(m.DeleteSpreads([]string{"s1"})) }},
		{"MoveSpread", kView, func(m *Methods) (model.View, error) { return ok(m.MoveSpread("s1", 1)) }},
		{"SplitSpread", kErr, func(m *Methods) (model.View, error) { return m.SplitSpread("s1", 1) }},
		{"MergeSpreadsAuto", kErr, func(m *Methods) (model.View, error) { return m.MergeSpreadsAuto("s1", true) }},
		{"SplitSpreadAuto", kErr, func(m *Methods) (model.View, error) { return m.SplitSpreadAuto("s1", 1, true) }},
		{"PurgePhotos", kView, func(m *Methods) (model.View, error) { return ok(m.PurgePhotos(ids)) }},
		{"MergeSpreads", kView, func(m *Methods) (model.View, error) { return ok(m.MergeSpreads("s1")) }},
		{"SetLayout", kView, func(m *Methods) (model.View, error) { return ok(m.SetLayout("s1", "x")) }},
		{"StepLayout", kView, func(m *Methods) (model.View, error) { return ok(m.StepLayout("s1", "spread", 1)) }},
		{"LayoutChoices", kRead, func(m *Methods) (model.View, error) { m.LayoutChoices("s1"); return none(m) }},
		{"MoreLayouts", kRead, func(m *Methods) (model.View, error) { m.MoreLayouts("s1", 0); return none(m) }},
		{"SetSettings", kView, func(m *Methods) (model.View, error) { return ok(m.SetSettings(model.Settings{})) }},
		{"SetSettingsToAll", kView, func(m *Methods) (model.View, error) { return ok(m.SetSettingsToAll(model.Settings{})) }},
		{"AutoPlan", kReadEr, func(m *Methods) (model.View, error) {
			_, err := m.AutoPlan(ids, model.AutoOpts{Spreads: 1, MaxPer: 4})
			return model.View{}, err
		}},
		{"AutoBuild", kErr, func(m *Methods) (model.View, error) {
			return m.AutoBuild(ids, 0, model.AutoOpts{Spreads: 1, MaxPer: 4})
		}},
		{"AutoPreview", kReadEr, func(m *Methods) (model.View, error) {
			_, err := m.AutoPreview(ids, 0, model.AutoOpts{Spreads: 1, MaxPer: 4})
			return model.View{}, err
		}},
		{"AutoSimilar", kReadEr, func(m *Methods) (model.View, error) {
			_, err := m.AutoSimilar(ids, model.AutoOpts{Spreads: 1, MaxPer: 4, Similar: 1})
			return model.View{}, err
		}},
		{"VaryLayouts", kErr, func(m *Methods) (model.View, error) { return m.VaryLayouts([]string{"s1"}, 1, nil) }},
		{"SetMarks", kErr, func(m *Methods) (model.View, error) { return m.SetMarks(ids, model.MarkPatch{}) }},
		{"ResetMarks", kView, func(m *Methods) (model.View, error) { return ok(m.ResetMarks(ids)) }},
		{"Marks", kRead, func(m *Methods) (model.View, error) { m.Marks(); return none(m) }},
		// bins.go
		{"NewBin", kErr, func(m *Methods) (model.View, error) { return m.NewBin("A", ids) }},
		{"RenameBin", kErr, func(m *Methods) (model.View, error) { return m.RenameBin("b1", "A") }},
		{"DeleteBin", kView, func(m *Methods) (model.View, error) { return ok(m.DeleteBin("b1")) }},
		{"SetBin", kErr, func(m *Methods) (model.View, error) { return m.SetBin("b1", ids, true) }},
		// grid.go
		{"SetGrid", kView, func(m *Methods) (model.View, error) { return ok(m.SetGrid("s1", true)) }},
		{"BackToAuto", kErr, func(m *Methods) (model.View, error) { return m.BackToAuto("s1", "", true) }},
		{"CropsToKeep", kRead, func(m *Methods) (model.View, error) { m.CropsToKeep("s1"); return none(m) }},
		{"AddPlace", kErr, func(m *Methods) (model.View, error) { return m.AddPlace("s1", 0, "") }},
		{"SetPlaces", kView, func(m *Methods) (model.View, error) { return ok(m.SetPlaces("s1", nil, "")) }},
		{"SetPlaceAngle", kView, func(m *Methods) (model.View, error) { return ok(m.SetPlaceAngle("s1", "a", 5)) }},
		{"DeletePlaces", kView, func(m *Methods) (model.View, error) { return ok(m.DeletePlaces("s1", ids)) }},
		{"DeleteSelection", kView, func(m *Methods) (model.View, error) { return ok(m.DeleteSelection("s1", ids, nil, nil)) }},
		{"ArrangePlaces", kErr, func(m *Methods) (model.View, error) { return m.ArrangePlaces("s1", "same", ids) }},
		{"ArrangeLayers", kView, func(m *Methods) (model.View, error) { return ok(m.ArrangeLayers("s1", "front", ids)) }},
		{"AlignItems", kView, func(m *Methods) (model.View, error) { return ok(m.AlignItems("s1", ids, "left", "sel")) }},
		{"AlignOverlap", kRead, func(m *Methods) (model.View, error) { m.AlignOverlap("s1", ids, "distH", "sel"); return none(m) }},
		{"TransformGroup", kView, func(m *Methods) (model.View, error) { return ok(m.TransformGroup("s1", ids, 1, 0, 0, 1, 1, "")) }},
		{"RotateGroup", kView, func(m *Methods) (model.View, error) { return ok(m.RotateGroup("s1", ids, 5, 0, 0)) }},
		{"ScaleGroupAxes", kView, func(m *Methods) (model.View, error) { return ok(m.ScaleGroupAxes("s1", ids, 1, 1, 0, 0, 0)) }},
		{"SetPlaceFit", kView, func(m *Methods) (model.View, error) { return ok(m.SetPlaceFit("s1", ids, "contain")) }},
		{"SetPlaceShape", kView, func(m *Methods) (model.View, error) { return ok(m.SetPlaceShape("s1", ids, "oval")) }},
		{"SetPlaceRound", kView, func(m *Methods) (model.View, error) { return ok(m.SetPlaceRound("s1", ids, 10)) }},
		{"SetBorder", kView, func(m *Methods) (model.View, error) {
			return ok(m.SetBorder("s1", ids, model.BorderPatch{Width: 2}, ""))
		}},
		{"MirrorGrid", kView, func(m *Methods) (model.View, error) { return ok(m.MirrorGrid("s1", "left")) }},
		{"DuplicateGrid", kErr, func(m *Methods) (model.View, error) { return m.DuplicateGrid("s1", 1, false) }},
		{"PlanCopyGrid", kRead, func(m *Methods) (model.View, error) { m.PlanCopyGrid("s1", "s1", nil, false, false); return none(m) }},
		{"PlanCopyDecor", kRead, func(m *Methods) (model.View, error) { m.PlanCopyDecor("s1", "s1", nil, false, false); return none(m) }},
		{"CoverGridChoices", kRead, func(m *Methods) (model.View, error) { m.CoverGridChoices("right", 2); return none(m) }},
		{"GridChoices", kRead, func(m *Methods) (model.View, error) { m.GridChoices("s1", 2, "spread"); return none(m) }},
		// design.go
		{"SetFrame", kView, func(m *Methods) (model.View, error) { return ok(m.SetFrame("a", 2, 0, 0)) }},
		{"ResetFrame", kView, func(m *Methods) (model.View, error) { return ok(m.ResetFrame("a")) }},
		{"SetFrameAngle", kView, func(m *Methods) (model.View, error) { return ok(m.SetFrameAngle("s1", "a", 5, "")) }},
		{"StraightenPhoto", kView, func(m *Methods) (model.View, error) { return ok(m.StraightenPhoto("s1", "a", 10, 1)) }},
		{"TurnPhotos", kView, func(m *Methods) (model.View, error) { return ok(m.TurnPhotos(ids, 1)) }},
		{"SetFlip", kView, func(m *Methods) (model.View, error) { return ok(m.SetFlip(ids, "h")) }},
		{"SetGray", kView, func(m *Methods) (model.View, error) { return ok(m.SetGray(ids)) }},
		{"SetTone", kView, func(m *Methods) (model.View, error) { return ok(m.SetTone("a", 10, -5)) }},
		{"SetBackground", kView, func(m *Methods) (model.View, error) { return ok(m.SetBackground("s1", "spread", "#112233")) }},
		{"AddText", kErr, func(m *Methods) (model.View, error) { return m.AddText("s1", 0, "") }},
		{"SetText", kView, func(m *Methods) (model.View, error) { return ok(m.SetText("t", "x", "")) }},
		{"SetTextBox", kView, func(m *Methods) (model.View, error) { return ok(m.SetTextBox("t", 1, 1, 10)) }},
		{"SetTextDraft", kView, func(m *Methods) (model.View, error) { return ok(m.SetTextDraft("s", "t", false)) }},
		{"SetTextAngle", kView, func(m *Methods) (model.View, error) { return ok(m.SetTextAngle("t", 5)) }},
		{"SetTextScale", kErr, func(m *Methods) (model.View, error) { return m.SetTextScale("t", 12, 0, 0) }},
		{"SetTextAuto", kView, func(m *Methods) (model.View, error) { return ok(m.SetTextAuto("t")) }},
		{"SetSpreadName", kView, func(m *Methods) (model.View, error) { return ok(m.SetSpreadName("s1", "x")) }},
		{"SetTextStyle", kView, func(m *Methods) (model.View, error) { return ok(m.SetTextStyle("t", tst)) }},
		{"DeleteText", kView, func(m *Methods) (model.View, error) { return ok(m.DeleteText("t")) }},
		{"SetCaption", kView, func(m *Methods) (model.View, error) { return ok(m.SetCaption("a", "x", "")) }},
		{"SetCaptionStyle", kView, func(m *Methods) (model.View, error) { return ok(m.SetCaptionStyle("a", tst, true, 1)) }},
		{"SetPerson", kView, func(m *Methods) (model.View, error) { return ok(m.SetPerson("a", model.Person{Last: "Х"}, "")) }},
		{"SetPlacePerson", kView, func(m *Methods) (model.View, error) {
			return ok(m.SetPlacePerson("s1", "00000000000b", model.Person{Last: "Х"}, ""))
		}},
		{"ForgetFileNames", kView, func(m *Methods) (model.View, error) { return ok(m.ForgetFileNames(ids)) }},
		{"PreviewNames", kReadEr, func(m *Methods) (model.View, error) {
			_, err := m.PreviewNames("s1", model.NamesOpts{})
			return model.View{}, err
		}},
		{"SetSheetCaps", kView, func(m *Methods) (model.View, error) { return ok(m.SetSheetCaps("s1", true, model.SheetCaps{})) }},
		{"SetBackdrop", kErr, func(m *Methods) (model.View, error) { return m.SetBackdrop("s1", "spread", "a") }},
		{"SetBackdropBox", kView, func(m *Methods) (model.View, error) { return ok(m.SetBackdropBox("s1", "spread", 0, 0, 10, 10, 0, "")) }},
		{"TurnBackdrop", kView, func(m *Methods) (model.View, error) { return ok(m.TurnBackdrop("s1", "spread", 90, "")) }},
		{"SetBackdropFrame", kView, func(m *Methods) (model.View, error) { return ok(m.SetBackdropFrame("s1", "spread", 2, 0, 0)) }},
		{"SetBackdropAngle", kView, func(m *Methods) (model.View, error) { return ok(m.SetBackdropAngle("s1", "spread", 5, "")) }},
		{"StraightenBackdrop", kView, func(m *Methods) (model.View, error) { return ok(m.StraightenBackdrop("s1", "spread", 10, 1)) }},
		{"SetBackdropFade", kView, func(m *Methods) (model.View, error) { return ok(m.SetBackdropFade("s1", "spread", 0.5, "")) }},
		{"SetPhotoFade", kView, func(m *Methods) (model.View, error) { return ok(m.SetPhotoFade(ids, 0.5, "")) }},
		{"FlipBackdrop", kView, func(m *Methods) (model.View, error) { return ok(m.FlipBackdrop("s1", "spread", "h")) }},
		{"FitBackdrop", kView, func(m *Methods) (model.View, error) { return ok(m.FitBackdrop("s1", "spread")) }},
		{"ClearBackdrop", kView, func(m *Methods) (model.View, error) { return ok(m.ClearBackdrop("s1", "spread")) }},
		{"ClearAlbumBg", kView, func(m *Methods) (model.View, error) { return ok(m.ClearAlbumBg()) }},
		{"SetAlbumBgFade", kView, func(m *Methods) (model.View, error) { return ok(m.SetAlbumBgFade(0.5, "")) }},
		{"SetNoAlbumBg", kView, func(m *Methods) (model.View, error) { return ok(m.SetNoAlbumBg("s1", true)) }},
		{"SpreadBgToAlbum", kView, func(m *Methods) (model.View, error) { return ok(m.SpreadBgToAlbum("s1")) }},
		// sticker.go
		{"SetStickerBox", kView, func(m *Methods) (model.View, error) { return ok(m.SetStickerBox("s1", "k", 0, 0, 10, 10, 0, "")) }},
		{"FlipSticker", kView, func(m *Methods) (model.View, error) { return ok(m.FlipSticker("s1", "k", "h")) }},
		{"SetStickerFade", kView, func(m *Methods) (model.View, error) { return ok(m.SetStickerFade("s1", "k", 0.5, "")) }},
		{"SetStickerInks", kView, func(m *Methods) (model.View, error) { return ok(m.SetStickerInks("s1", "k", ids, "")) }},
	}
}

// Каждый экспортируемый метод Methods есть в таблице probes: новый метод без проверки
// паники и провязки не пройдёт (набор методов — это набор методов App для JS).
func TestProbesCoverAllMethods(t *testing.T) {
	var have []string
	for _, p := range probes() {
		have = append(have, p.name)
	}
	typ := reflect.TypeOf(&Methods{})
	var want []string
	for i := range typ.NumMethod() {
		want = append(want, typ.Method(i).Name)
	}
	slices.Sort(have)
	slices.Sort(want)
	if !slices.Equal(have, want) {
		t.Fatalf("таблица probes и методы Methods расходятся:\n таблица: %v\n методы:  %v", have, want)
	}
}

// Паника в Editor: проект цел, вызывающий получает текущий вид и ErrInternal (ошибкой или
// тостом), чтения молча возвращают нулевое значение, Changed не зовётся ни у кого.
func TestPanicInEveryMethod(t *testing.T) {
	for _, pr := range probes() {
		t.Run(pr.name, func(t *testing.T) {
			k := twoSpreads(t)
			rev := k.rev()
			if pr.kind != kStatic {
				k.panicNext = true
			}
			v, err := pr.call(k.Methods)
			if k.panicNext {
				t.Fatal("метод не обратился к Editor: паника не сработала")
			}
			if k.calls != 0 || k.rev() != rev {
				t.Errorf("после паники: Changed %d раз, rev %d → %d", k.calls, rev, k.rev())
			}
			if pr.kind == kView || pr.kind == kErr {
				if len(v.Spreads) != 2 || v.Rev != rev {
					t.Errorf("вид после паники: %d разворотов, rev %d (ждали 2, %d)", len(v.Spreads), v.Rev, rev)
				}
			}
			checkPanicReport(t, pr.kind, k, err)
		})
	}
}

// checkPanicReport — как паника дошла до вызывающего: ошибкой, тостом или никак.
func checkPanicReport(t *testing.T, kind int, k *kit, err error) {
	t.Helper()
	switch kind {
	case kView:
		if err != nil || !slices.Equal(k.notices, []string{ErrInternal.Error()}) {
			t.Errorf("ждали тост ErrInternal без ошибки: err %v, тосты %q", err, k.notices)
		}
	case kErr, kReadEr:
		if !errors.Is(err, ErrInternal) || len(k.notices) != 0 {
			t.Errorf("ждали ErrInternal без тоста: err %v, тосты %q", err, k.notices)
		}
	default:
		if err != nil || len(k.notices) != 0 {
			t.Errorf("чтение/статика: err %v, тосты %q", err, k.notices)
		}
	}
}

// Editor, который паникует всегда: Fail не падает сам (вид после паники не достать),
// вызывающий всё равно получает ErrInternal.
func TestFailWhenEditorAlwaysPanics(t *testing.T) {
	var notes []string
	m := New(Host{
		Editor:      func() *album.Editor { panic("нет Editor") },
		Changed:     func(v model.View) model.View { t.Error("Changed после паники"); return v },
		Notice:      func(s string) { notes = append(notes, s) },
		Infos:       func([]string) []model.PhotoInfo { return nil },
		InfosOrRefs: func([]string) []model.PhotoInfo { return nil },
		Templates:   func() []model.Template { return nil },
	})
	if _, err := m.AddPlace("s1", 0, ""); !errors.Is(err, ErrInternal) {
		t.Errorf("AddPlace: %v", err)
	}
	if v := m.SetGrid("s1", true); len(v.Spreads) != 0 {
		t.Errorf("вид из ничего: %+v", v)
	}
	if len(notes) != 1 || notes[0] != ErrInternal.Error() {
		t.Errorf("тосты: %q", notes)
	}
}

// New отказывается от Host с пустым полем и называет его: паника здесь лучше nil-вызова
// посреди правки пользователя.
func TestNewRejectsEmptyHostField(t *testing.T) {
	full := func() Host {
		return Host{
			Editor:      func() *album.Editor { return nil },
			Changed:     func(v model.View) model.View { return v },
			Notice:      func(string) {},
			Infos:       func([]string) []model.PhotoInfo { return nil },
			InfosOrRefs: func([]string) []model.PhotoInfo { return nil },
			Templates:   func() []model.Template { return nil },
		}
	}
	if New(full()) == nil {
		t.Fatal("полный Host отвергнут")
	}
	typ := reflect.TypeFor[Host]()
	for i := range typ.NumField() {
		name := typ.Field(i).Name
		t.Run(name, func(t *testing.T) {
			h := full()
			reflect.ValueOf(&h).Elem().Field(i).SetZero()
			defer func() {
				r, _ := recover().(string)
				if !strings.Contains(r, "Host."+name) {
					t.Errorf("паника %q не называет поле Host.%s", r, name)
				}
			}()
			New(h)
		})
	}
}
