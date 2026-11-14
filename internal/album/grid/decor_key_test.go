package grid

import (
	"math"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// TestDecorKey — ключ сравнения оформления: ID не входят (у копий они новые), страница не
// учитывает второй цвет; значение, которое JSON не принимает (NaN), даёт ключ «не равно» с
// пометкой, а не панику и не ключ пустого оформления.
func TestDecorKey(t *testing.T) {
	a := srcDesign{bgL: "#111", bgR: "#222", stickers: []*model.Sticker{{ID: "x", Art: &model.Art{Kind: "plate"}, W: 1, H: 1}}}
	b := srcDesign{bgL: "#111", bgR: "#333", stickers: []*model.Sticker{{ID: "y", Art: &model.Art{Kind: "plate"}, W: 1, H: 1}}}
	if decorKey(a, model.ScopeLeft) != decorKey(b, model.ScopeLeft) {
		t.Fatal("страница: ID и второй цвет не должны влиять")
	}
	if decorKey(a, model.ScopeSpread) == decorKey(b, model.ScopeSpread) {
		t.Fatal("весь лист: второй цвет должен влиять")
	}
	bad := srcDesign{stickers: []*model.Sticker{{Art: &model.Art{Kind: "plate"}, X: math.NaN(), W: 1, H: 1}}}
	if k := decorKey(bad, model.ScopeLeft); !strings.HasPrefix(k, "!") {
		t.Fatalf("ключ NaN: %q", k)
	}
	if decorKey(bad, model.ScopeLeft) == decorKey(srcDesign{}, model.ScopeLeft) {
		t.Fatal("ключ с ошибкой совпал с пустым оформлением")
	}
}
