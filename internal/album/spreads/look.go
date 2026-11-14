package spreads

import (
	"slices"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// DropEmpty — убирает пустые развороты (Spread.Empty: «+» без дела) и возвращает, сколько их
// было. Постановка серии макета (131) начинает с чистого списка: пустой разворот не должен
// стоять между главами. Обложка не затрагивается.
func DropEmpty(p *model.Project) int {
	n := len(p.Spreads)
	p.Spreads = slices.DeleteFunc(p.Spreads, func(s *model.Spread) bool { return s.Empty() })
	return n - len(p.Spreads)
}
