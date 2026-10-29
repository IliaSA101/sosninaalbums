package names

import (
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// seatRows пропускает nil-место (битый список мест не роняет предпросмотр) и нумерует остальные подряд.
func TestSeatRowsSkipsNilPlace(t *testing.T) {
	s := &model.Spread{ID: "s", Grid: []*model.Place{{ID: "a"}, nil, {ID: "b"}}}
	rows := seatRows(s, false)
	if len(rows) != 2 || rows[0].Place != "a" || rows[0].No != 1 || rows[1].Place != "b" || rows[1].No != 2 {
		t.Fatalf("строки: %+v", rows)
	}
	if rows := seatRows(&model.Spread{ID: "e"}, true); len(rows) != 0 {
		t.Fatalf("пустой лист: %+v", rows)
	}
}
