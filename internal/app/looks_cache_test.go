package app

import (
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Кэш разобранных макетов (decor.Store.Look) на путях App: LookHost.Find отдаёт каждый раз свою
// копию, плитка /media/look после смены макета в библиотеке — на новом файле.
func TestLookHostFindCacheCopy(t *testing.T) {
	a := lookApp(t)
	id := saveLook(t, a, "Свадьба", "#336699")
	s, _ := a.decorStore()
	ref, _ := s.LookRefOf(id)
	find := a.lookHost().Find
	first := find(ref)
	if first == nil || len(first.Steps) != 3 {
		t.Fatalf("Find: %+v", first)
	}
	want := *find(ref)
	first.Steps[0].Name = "Испорчено"
	first.Cover.Bg.Color = "#000000"
	first.Steps[0].Tpl.Places[0].X = 0.99
	if again := find(ref); again == nil || !reflect.DeepEqual(*again, want) {
		t.Fatalf("второй Find после правки первого: %+v", again)
	}
	// Макета нет в индексе — nil, а не прежний разбор.
	if find(model.LookRef{Key: "my:ffffffffffffffff"}) != nil {
		t.Error("Find чужого ключа")
	}
}

func TestMediaLookAfterFileChange(t *testing.T) {
	a := lookApp(t)
	id := saveLook(t, a, "Свадьба", "#336699")
	hex := strings.TrimPrefix(id, "my:")
	q := "/media/look/" + hex + "?v=1&i=1&px=128&r=1"
	for range 2 { // второй — из кэша разбора
		if c := getLook(a, http.MethodGet, q); c != 200 {
			t.Fatalf("плитка: %d", c)
		}
	}
	s, _ := a.decorStore()
	path := filepath.Join(s.Root(), "Макеты", hex+".json")
	if err := os.WriteFile(path, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if c := getLook(a, http.MethodGet, q); c != 404 {
		t.Errorf("файл испорчен, плитка %d, ждали 404", c)
	}
}
