package model_test

import (
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// TestUnmarshalBadSnapshot — негодный снимок — ошибка кода: паника с причиной, а не пустой
// проект, который тихо заменил бы альбом.
func TestUnmarshalBadSnapshot(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("негодный снимок прочитан без паники")
		}
	}()
	model.Unmarshal([]byte(`{"spreads": 5}`))
}
