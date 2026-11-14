package edit

import (
	"errors"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/app/appkit"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Пустое поле Host — паника New с именем поля (шаблон служб: testkit.NewPanics).
func TestEditNewPanics(t *testing.T) {
	full := Host{
		Editor:      func() *album.Editor { return nil },
		Changed:     func(v model.View) model.View { return v },
		Notice:      func(string) {},
		Infos:       func([]string) []model.PhotoInfo { return nil },
		InfosOrRefs: func([]string) []model.PhotoInfo { return nil },
		Templates:   func() []model.Template { return nil },
	}
	testkit.NewPanics(t, full, func(h Host) { New(h) }, "edit.New")
}

// ErrInternal и Guard в edit — те же appkit: errors.Is и сравнение в app работают по одной ошибке.
func TestEditErrInternalIsAppkit(t *testing.T) {
	if !errors.Is(ErrInternal, appkit.ErrInternal) || ErrInternal != appkit.ErrInternal {
		t.Fatal("edit.ErrInternal не равна appkit.ErrInternal")
	}
	var err error
	func() {
		defer GuardErr("тест", &err)
		panic("бум")
	}()
	if !errors.Is(err, appkit.ErrInternal) {
		t.Fatalf("edit.GuardErr отдала %v, ждали appkit.ErrInternal", err)
	}
}
