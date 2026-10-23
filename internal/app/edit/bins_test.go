package edit

import (
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Подборки (083): в членство попадают только фото ленты — ID не из ленты пропускаются, путь от
// интерфейса в файл не идёт; удачная правка рассылает вид, ошибка и холостая — без рассылки шага.
func TestSetBinSkipsUnknownIDs(t *testing.T) {
	k := twoSpreads(t)
	k.lentPhoto("a")
	k.lentPhoto("b")
	v, err := k.NewBin("Обязательно", []string{"a", "призрак", testkit.AbsPath("чужой.jpg")})
	if err != nil || k.calls != 1 || len(v.Bins) != 1 {
		t.Fatalf("NewBin: err %v, Changed %d, подборок %d", err, k.calls, len(v.Bins))
	}
	id := v.Bins[0].ID
	if v, err = k.SetBin(id, []string{"b", "призрак"}, true); err != nil || k.calls != 2 {
		t.Fatalf("SetBin: err %v, Changed %d", err, k.calls)
	}
	marks := k.Marks()
	if len(marks) != 2 || !slices.Equal(marks["a"].Bins, []string{id}) || !slices.Equal(marks["b"].Bins, []string{id}) {
		t.Fatalf("членство: %+v", marks)
	}
	if got := k.ed.View().Bins; len(got) != 1 {
		t.Fatalf("подборки %+v", got)
	}
	// Только чужие ID — холостая: rev прежний.
	rev := v.Rev
	if v, err = k.SetBin(id, []string{"призрак"}, false); err != nil || v.Rev != rev || len(k.Marks()) != 2 {
		t.Fatalf("SetBin чужих: err %v, rev %d → %d", err, rev, v.Rev)
	}
	// Ошибки — тексты для интерфейса, без рассылки.
	calls := k.calls
	if _, err = k.NewBin("обязательно", nil); err == nil || err.Error() != "Такая подборка уже есть" {
		t.Errorf("повтор имени: %v", err)
	}
	if _, err = k.RenameBin("b0", "X"); err == nil || err.Error() != "Подборки больше нет" {
		t.Errorf("переименование неизвестной: %v", err)
	}
	if _, err = k.SetBin("b0", []string{"a"}, true); err == nil || err.Error() != "Подборки больше нет" {
		t.Errorf("SetBin неизвестной: %v", err)
	}
	if k.calls != calls {
		t.Errorf("ошибка разослала вид: Changed %d → %d", calls, k.calls)
	}
	if v, err = k.RenameBin(id, "Запас"); err != nil || v.Bins[0].Name != "Запас" {
		t.Fatalf("RenameBin: %v %+v", err, v.Bins)
	}
	if v = k.DeleteBin(id); len(v.Bins) != 0 || len(k.Marks()) != 0 {
		t.Fatalf("DeleteBin: подборок %d, отметок %d", len(v.Bins), len(k.Marks()))
	}
}
