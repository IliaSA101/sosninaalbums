package edit

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// TestAutoBuildLogsMem (089 G3): удачная сборка пишет одну строку «память: автосборка», отказ
// (пересборка без прошлой сборки) и холостая сборка без фото — не пишет; путей в строке нет.
func TestAutoBuildLogsMem(t *testing.T) {
	var buf bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(prev) })
	re := regexp.MustCompile(`память: автосборка — Go \d+ МБ, куча \d+ МБ\n`)

	k := twoSpreads(t)
	var ids []string
	for i := range 6 {
		id := fmt.Sprintf("m%d", i)
		k.lentPhoto(id)
		ids = append(ids, id)
	}
	// Отказ: пересборка, когда собирать ещё нечего.
	if _, err := k.AutoBuild(ids, -1, model.AutoOpts{Spreads: 2, MaxPer: 4, Again: true}); err == nil {
		t.Fatal("пересборка без прошлой сборки не отказала")
	}
	if buf.Len() != 0 {
		t.Errorf("отказ написал в журнал: %q", buf.String())
	}
	if _, err := k.AutoBuild(ids, -1, model.AutoOpts{Spreads: 2, MaxPer: 4}); err != nil {
		t.Fatal(err)
	}
	if n := len(re.FindAllString(buf.String(), -1)); n != 1 || strings.Contains(buf.String(), "lent") {
		t.Errorf("строк памяти %d (ждали 1), журнал %q", n, buf.String())
	}
}

// Фото берутся из ленты (Host.Infos): AddPhotos и NewSpread кладут именно их; неизвестные
// ленте ID пропускаются; отказ Editor (лимит фото на развороте) возвращается без события.
func TestPhotosFromRibbon(t *testing.T) {
	k := twoSpreads(t)
	k.lentPhoto("L1")
	k.lentPhoto("L2")

	v, err := k.AddPhotos("s2", []string{"L1", "L2", "призрак"}, 0)
	if err != nil || !slices.Equal(v.Spreads[1].Photos, []string{"L1", "L2", "c"}) || k.calls != 1 {
		t.Fatalf("AddPhotos: err %v, фото %v, Changed %d", err, v.Spreads[1].Photos, k.calls)
	}
	if got := k.asked[0]; !slices.Equal(got, []string{"L1", "L2", "призрак"}) {
		t.Errorf("у ленты спросили %v", got)
	}

	// Ни в ленте, ни в альбоме: ошибка (092), проект и событие прежние.
	rev, calls := k.rev(), k.calls
	if v, err = k.AddPhotos("s1", []string{"призрак"}, 0); !errors.Is(err, ErrNoPhoto) || v.Rev != rev || k.calls != calls {
		t.Errorf("AddPhotos чужих ID: err %v, rev %d → %d, Changed %d → %d", err, rev, v.Rev, calls, k.calls)
	}

	// Лимит фото на развороте: ошибка, проект и событие прежние.
	var many []string
	for i := range model.MaxPhotos {
		id := fmt.Sprintf("m%d", i)
		k.lentPhoto(id)
		many = append(many, id)
	}
	calls = k.calls
	if v, err = k.AddPhotos("s1", many, 0); err == nil || k.calls != calls || v.Rev != rev {
		t.Errorf("AddPhotos сверх лимита: err %v, Changed %d → %d, rev %d → %d", err, calls, k.calls, rev, v.Rev)
	}

	// NewSpread: новый разворот с фото ленты в нужной позиции.
	if v, err = k.NewSpread(1, []string{"L1"}); err != nil {
		t.Fatal(err)
	}
	if len(v.Spreads) != 3 || !slices.Equal(v.Spreads[1].Photos, []string{"L1"}) {
		t.Errorf("NewSpread: %v", photosOf(v))
	}
	// Пустой разворот просят явно (ids нет) — создаётся.
	if v, err = k.NewSpread(0, nil); err != nil || len(v.Spreads) != 4 || len(v.Spreads[0].Photos) != 0 {
		t.Errorf("пустой NewSpread: err %v, %v", err, photosOf(v))
	}
	// Все фото пропали из ленты при переносе: ErrNoPhoto (092), разворот не создаётся, события нет.
	calls, rev = k.calls, k.rev()
	if v, err = k.NewSpread(0, []string{"призрак"}); !errors.Is(err, ErrNoPhoto) || len(v.Spreads) != 4 || k.calls != calls || v.Rev != rev {
		t.Errorf("NewSpread из пропавших: err %v, разворотов %d, Changed %d → %d", err, len(v.Spreads), calls, k.calls)
	}
}

// Разворотные правки без папок: одним вызовом сразу несколько разворотов; неизвестные
// пропускаются, шаг отмены один.
func TestSpreadBatchEdits(t *testing.T) {
	k := twoSpreads(t)
	if v, err := k.NewSpread(2, nil); err != nil || len(v.Spreads) != 3 {
		t.Fatalf("подготовка: %v", err)
	}
	r0 := k.rev()
	v := k.DeleteSpreads([]string{"s1", "s2", "нет-такого"})
	if len(v.Spreads) != 1 || v.Rev <= r0 {
		t.Fatalf("DeleteSpreads: %v, rev %d → %d", photosOf(v), r0, v.Rev)
	}
	if v = k.ed.Undo(); len(v.Spreads) != 3 {
		t.Errorf("Undo вернул %d разворотов из 3: удаление нескольких — не один шаг", len(v.Spreads))
	}
	// Ничего не нашлось — холостая, шага отмены нет.
	r1 := k.rev()
	if v = k.DeleteSpreads(nil); v.Rev != r1 {
		t.Errorf("DeleteSpreads(nil): rev %d → %d", r1, v.Rev)
	}
	if v = k.DeleteSpreads([]string{"нет"}); v.Rev != r1 {
		t.Errorf("DeleteSpreads(неизвестный): rev %d → %d", r1, v.Rev)
	}
}

// Галерея раскладок: «Ещё» — чтение, проект и событие не трогает.
func TestMoreLayoutsReadOnly(t *testing.T) {
	k := twoSpreads(t)
	rev := k.rev()
	c := k.MoreLayouts("s1", 0)
	if k.rev() != rev || k.calls != 0 {
		t.Errorf("чтение изменило проект: rev %d → %d, Changed %d", rev, k.rev(), k.calls)
	}
	if !reflect.DeepEqual(c, k.ed.MoreLayouts("s1", 0)) {
		t.Error("MoreLayouts отдал не то, что считает Editor")
	}
}

// Автосборка: план и сборка берут фото из ленты; план проект не трогает; сборка делает ровно
// столько разворотов, сколько обещал план; Variety зажимается; нет фото — пустой план.
func TestAutoPlanAndBuild(t *testing.T) {
	k := twoSpreads(t)
	var ids []string
	for i := range 6 {
		id := fmt.Sprintf("n%d", i)
		k.lentPhoto(id)
		ids = append(ids, id)
	}
	o := model.AutoOpts{Spreads: 3, MaxPer: 2}
	rev := k.rev()
	pl, err := k.AutoPlan(ids, o)
	if err != nil || pl.Photos != 6 || pl.Spreads != 3 {
		t.Fatalf("AutoPlan: err %v, %+v", err, pl)
	}
	if k.rev() != rev || k.calls != 0 {
		t.Errorf("план изменил проект: rev %d → %d, Changed %d", rev, k.rev(), k.calls)
	}
	if empty, err := k.AutoPlan(nil, o); err != nil || empty.Photos != 0 {
		t.Errorf("AutoPlan без фото: err %v, %+v", err, empty)
	}

	o.Variety = 99 // зажим в 0…2 — Go, а не интерфейс
	v, err := k.AutoBuild(ids, 2, o)
	if err != nil || len(v.Spreads) != 2+pl.Spreads || k.calls != 1 {
		t.Fatalf("AutoBuild: err %v, разворотов %d (ждали %d), Changed %d", err, len(v.Spreads), 2+pl.Spreads, k.calls)
	}
	var built []string
	for _, s := range v.Spreads[2:] {
		built = append(built, s.Photos...)
	}
	slices.Sort(built)
	if !slices.Equal(built, ids) {
		t.Errorf("собраны фото %v, а в ленте %v", built, ids)
	}
	if v = k.ed.Undo(); len(v.Spreads) != 2 {
		t.Errorf("Undo: %d разворотов (автосборка — один шаг)", len(v.Spreads))
	}
}

// «Разнообразить»: оценки берутся у ленты по фото выбранных разворотов (а не всех); один
// шаг отмены; уровень зажимается; пустой выбор — холостая.
func TestVaryLayoutsWiring(t *testing.T) {
	k := twoSpreads(t)
	k.lentPhoto("a")
	k.lentPhoto("b")
	k.lentPhoto("c")
	rev := k.rev()
	v, err := k.VaryLayouts([]string{"s1", "нет"}, 7, nil) // 7 → зажим в 0…2
	if err != nil || k.calls != 1 {
		t.Fatalf("VaryLayouts: err %v, Changed %d", err, k.calls)
	}
	if len(k.asked) != 1 || !slices.Equal(k.asked[0], []string{"a", "b"}) {
		t.Errorf("у ленты спросили %v, ждали фото s1", k.asked)
	}
	if v.Rev < rev {
		t.Errorf("rev упал: %d → %d", rev, v.Rev)
	}
	rev = k.rev()
	if v, err = k.VaryLayouts(nil, 1, nil); err != nil || v.Rev != rev {
		t.Errorf("пустой выбор: err %v, rev %d → %d", err, rev, v.Rev)
	}
}

// lentMixed кладёт в ленту n фото: каждое третье портретное (раскладкам есть из чего выбирать).
func (k *kit) lentMixed(prefix string, n int) []string {
	ids := make([]string, n)
	for i := range ids {
		ids[i] = fmt.Sprint(prefix, i)
		pi := k.lentPhoto(ids[i])
		if i%3 == 0 {
			pi.W, pi.H = 200, 300
			k.lent[ids[i]] = pi
		}
	}
	return ids
}

// layouts — действующие ключи разворотов вида.
func layouts(v model.View) []string {
	out := make([]string, len(v.Spreads))
	for i, s := range v.Spreads {
		out[i] = s.Layout
	}
	return out
}

// Предпросмотр автосборки (065): фото из ленты, ровно o.Spreads разворотов, проект не трогает —
// ни rev, ни Changed; нет фото — пустые ряды.
func TestAutoPreviewWiring(t *testing.T) {
	k := twoSpreads(t)
	ids := k.lentMixed("p", 24)
	rev := k.rev()
	o := model.AutoOpts{Spreads: 4, MaxPer: 8, Variety: 1, Families: []string{"dense", "hero"}}
	pv, err := k.AutoPreview(ids, -1, o)
	if err != nil || len(pv.Spreads) != 4 || len(pv.Samples) == 0 {
		t.Fatalf("AutoPreview: err %v, разворотов %d, образцов %d", err, len(pv.Spreads), len(pv.Samples))
	}
	if k.rev() != rev || k.calls != 0 || !slices.Equal(k.asked[len(k.asked)-1], ids) {
		t.Errorf("предпросмотр: rev %d → %d, Changed %d, у ленты спросили %v", rev, k.rev(), k.calls, k.asked)
	}
	var keys []string
	for _, s := range pv.Spreads {
		keys = append(keys, s.Key)
	}
	if v, err := k.AutoBuild(ids, -1, o); err != nil || !slices.Equal(layouts(v)[2:], keys) {
		t.Errorf("сборка разошлась с предпросмотром %v: %v %v", keys, err, layouts(v))
	}
	if empty, err := k.AutoPreview(nil, -1, o); err != nil || len(empty.Spreads)+len(empty.Samples) != 0 {
		t.Errorf("без фото: err %v, %+v", err, empty)
	}
}

// TestAutoOptsFamiliesClamp (065): App берёт не больше 16 строк семейств — «Плотно» 17-й строкой
// после мусора не действует (= без семейств), первой — действует, хвост мусора не мешает;
// то же у «Разнообразить». Variety зажимается в 0…2.
func TestAutoOptsFamiliesClamp(t *testing.T) {
	junk := make([]string, 16, 1000)
	for i := range junk {
		junk[i] = fmt.Sprint("мусор", i)
	}
	if got := (model.AutoOpts{Variety: 99, Families: append(slices.Clone(junk), "dense")}).Clamp(); got.Variety != 2 ||
		len(got.Families) != 16 || model.ClampFamilies(nil) != nil {
		t.Fatalf("Clamp: %+v", got)
	}
	build := func(fs []string) (*kit, model.View) {
		k := twoSpreads(t)
		ids := k.lentMixed("q", 30)
		v, err := k.AutoBuild(ids, -1, model.AutoOpts{Spreads: 6, MaxPer: 8, Variety: 1, Families: fs})
		if err != nil {
			t.Fatal(err)
		}
		return k, v
	}
	_, none := build(nil)
	_, late := build(append(slices.Clone(junk), "strip"))
	_, strip := build([]string{"strip"})
	_, long := build(append([]string{"strip"}, append(junk, junk...)...))
	if !slices.Equal(layouts(late), layouts(none)) {
		t.Errorf("17-я строка подействовала: %v ≠ %v", layouts(late), layouts(none))
	}
	if slices.Equal(layouts(strip), layouts(none)) || !slices.Equal(layouts(long), layouts(strip)) {
		t.Errorf("«Ленты» первой строкой: %v, с хвостом %v, без семейств %v", layouts(strip), layouts(long), layouts(none))
	}

	vary := func(fs []string) []string {
		k, v := build(nil)
		var ids []string
		for _, s := range v.Spreads[2:] {
			ids = append(ids, s.ID)
		}
		got, err := k.VaryLayouts(ids, 1, fs)
		if err != nil {
			t.Fatal(err)
		}
		return layouts(got)
	}
	if a, b := vary(append(slices.Clone(junk), "strip")), vary(nil); !slices.Equal(a, b) {
		t.Errorf("«Разнообразить»: 17-я строка подействовала: %v ≠ %v", a, b)
	}
	if a, b := vary([]string{"strip"}), vary(nil); slices.Equal(a, b) {
		t.Errorf("«Разнообразить» с «Лентами» = без семейств: %v", a)
	}
}

// Свои отметки: ставятся только фото ленты, читаются через Marks, сбрасываются ResetMarks;
// холостой патч и сброс без отметок rev не меняют.
func TestMarksWiring(t *testing.T) {
	k := twoSpreads(t)
	k.lentPhoto("a")
	k.lentPhoto("b")
	five := 5
	v, err := k.SetMarks([]string{"a", "b", "призрак"}, model.MarkPatch{Rating: &five})
	if err != nil || k.calls != 1 {
		t.Fatalf("SetMarks: err %v, Changed %d", err, k.calls)
	}
	marks := k.Marks()
	if len(marks) != 2 || marks["a"].Rating == nil || *marks["a"].Rating != 5 || marks["призрак"].Rating != nil {
		t.Errorf("отметки: %+v", marks)
	}
	// Тот же патч второй раз — холостая.
	rev := v.Rev
	if v, err = k.SetMarks([]string{"a", "b"}, model.MarkPatch{Rating: &five}); err != nil || v.Rev != rev {
		t.Errorf("повторный SetMarks: err %v, rev %d → %d", err, rev, v.Rev)
	}
	if v = k.ResetMarks([]string{"a"}); v.Rev <= rev || len(k.Marks()) != 1 {
		t.Errorf("ResetMarks: rev %d → %d, отметок %d", rev, v.Rev, len(k.Marks()))
	}
	rev = v.Rev
	if v = k.ResetMarks([]string{"a", "нет"}); v.Rev != rev {
		t.Errorf("повторный ResetMarks: rev %d → %d", rev, v.Rev)
	}
}
