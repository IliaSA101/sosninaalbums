package model

import (
	"reflect"
	"testing"
)

// Страж empty(): любое поле разворота, кроме производных, делает его непустым.
// Новое поле с данными пользователя без строки в empty() этот тест уронит.
func TestSpreadEmptyCoversFields(t *testing.T) {
	// Order — производное от мест и текстов: в empty() его нет, иначе устаревший
	// Order (normalize раньше syncOrder) не дал бы удалить опустевший разворот.
	derived := map[string]bool{"ID": true, "Layout": true, "Variant": true, "Order": true}
	if !(&Spread{ID: "s", Layout: "H(0,1)", Variant: 2}).Empty() {
		t.Fatal("разворот без данных не пустой")
	}
	typ := reflect.TypeFor[Spread]()
	for i := range typ.NumField() {
		f := typ.Field(i)
		if derived[f.Name] {
			continue
		}
		var s Spread
		v := reflect.ValueOf(&s).Elem().Field(i)
		switch v.Kind() {
		case reflect.String:
			v.SetString("x")
		case reflect.Slice:
			v.Set(reflect.MakeSlice(f.Type, 1, 1))
		case reflect.Int:
			v.SetInt(1)
		case reflect.Pointer:
			v.Set(reflect.New(f.Type.Elem()))
		default:
			t.Fatalf("поле %s: тип %s — дописать в тест и в empty()", f.Name, f.Type)
		}
		if s.Empty() {
			t.Errorf("поле %s не учтено в empty()", f.Name)
		}
	}
}
