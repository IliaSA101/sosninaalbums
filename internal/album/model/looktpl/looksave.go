package looktpl

import (
	"errors"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// «Сохранить альбом как макет…» (131 design §8): параметры окна и сборка серии из шагов разворотов.
// Снимок разворота — grid.AlbumTplOf; здесь только чистая часть — роли, слияние, предел шагов.

var (
	// ErrLookMany — разных шагов больше MaxLookSteps (текст уходит в тост окна как есть).
	ErrLookMany = errors.New("Не больше 24 разных разворотов — выделите нужные в ленте") //nolint:staticcheck // текст уходит пользователю как есть
	// ErrLookVigRep — лист-виньетка стоит между повторяемыми: серия держит повтор одним блоком.
	ErrLookVigRep = errors.New("Лист-виньетка стоит среди повторяемых — сдвиньте границы «Повторяемые» или выделите нужные в ленте") //nolint:staticcheck // текст уходит пользователю как есть
)

// MergeLookSteps — серия макета из шагов разворотов в порядке альбома: rep[i] — шаг i повторяемый.
// Повторяемые получают роль repeat (виньетка — никогда: она стоит в начале или конце серии) и
// сливаются по TplKey — остаётся первый из одинаковых, порядок круга сохраняется; остальные шаги
// стоят как есть, даже одинаковые (число разворотов начала и конца — выбор автора). Виньетка между
// повторяемыми — ErrLookVigRep, больше MaxLookSteps шагов после слияния — ErrLookMany. Входные
// шаги не меняются.
func MergeLookSteps(steps []LookStep, rep []bool) ([]LookStep, error) {
	first, last := -1, -1
	roles := make([]string, len(steps))
	for i, st := range steps {
		roles[i] = st.Role
		if i < len(rep) && rep[i] && st.Role != RoleVignette {
			roles[i] = RoleRepeat
			if first < 0 {
				first = i
			}
			last = i
		}
	}
	for i := first; i >= 0 && i < last; i++ {
		if roles[i] != RoleRepeat {
			return nil, ErrLookVigRep // при диапазоне подряд между повторяемыми бывают только виньетки
		}
	}
	seen := map[string]bool{}
	out := make([]LookStep, 0, min(len(steps), model.MaxLookSteps+1))
	for i, st := range steps {
		st.Role = roles[i]
		if st.Role == RoleRepeat {
			key := TplKey(st.Tpl)
			if seen[key] {
				continue
			}
			seen[key] = true
		}
		out = append(out, st)
	}
	if len(out) > model.MaxLookSteps {
		return nil, ErrLookMany
	}
	return out, nil
}
