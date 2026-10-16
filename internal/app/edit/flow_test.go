package edit

import (
	"fmt"
	"reflect"
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

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

	// Ленте ничего не известно: холостая, не ошибка.
	rev := k.rev()
	if v, err = k.AddPhotos("s1", []string{"призрак"}, 0); err != nil || v.Rev != rev {
		t.Errorf("AddPhotos чужих ID: err %v, rev %d → %d", err, rev, v.Rev)
	}

	// Лимит фото на развороте: ошибка, проект и событие прежние.
	var many []string
	for i := range model.MaxPhotos {
		id := fmt.Sprintf("m%d", i)
		k.lentPhoto(id)
		many = append(many, id)
	}
	calls := k.calls
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
	// Все фото пропали из ленты при переносе: разворот не создаётся, события нет.
	calls, rev = k.calls, k.rev()
	if v, err = k.NewSpread(0, []string{"призрак"}); err != nil || len(v.Spreads) != 4 || k.calls != calls || v.Rev != rev {
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
	v, err := k.VaryLayouts([]string{"s1", "нет"}, 7) // 7 → зажим в 0…2
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
	if v, err = k.VaryLayouts(nil, 1); err != nil || v.Rev != rev {
		t.Errorf("пустой выбор: err %v, rev %d → %d", err, rev, v.Rev)
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
