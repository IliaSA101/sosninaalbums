package model

import (
	"encoding/json"
	"fmt"
	"math"
	"testing"
)

// fuzzTplSeeds — файлы шаблонов разных версий: сетка с углом и формой, виньетка со стилем.
func fuzzTplSeeds(f *testing.F) {
	caps := VigCaps(304)
	head := TextStyle{Font: "cormorant", Size: 28, Color: "#333333", Align: "center"}
	list := []Template{
		{ID: "a1", Name: "Сетка", Scope: TplPage, Boxes: []Box{{X: 0.1, Y: 0.1, W: 0.4, H: 0.3, Rot: 12.5, Shape: "oval"}}},
		{ID: "a2", Name: "Виньетка", Scope: TplSpread, Kind: TplVignette, Boxes: []Box{{W: 0.2, H: 0.3}, {X: 0.5, W: 0.2, H: 0.3}},
			Vig: &VigTpl{VigGeo: VigGeo{Roles: []string{RoleTeacher, RolePupil}, Head: &Box{X: 0.05, Y: 0.02, W: 0.9, H: 0.12},
				Pages: 2, Aspect: 1.5, Ratio: "4:5", W: 600, H: 300},
				Style: VigStyle{Caps: &caps, TText: TeacherCaption, Head: &head, Shape: "round", Round: 0.2}}},
	}
	seed, err := MarshalTemplates(list)
	if err != nil {
		f.Fatal(err)
	}
	f.Add(seed)
	f.Add([]byte(`{"version":1,"templates":[{"id":"x","name":"","scope":"page","boxes":[{"x":0.9,"y":0.9,"w":0.5,"h":0.5}]}]}`))
	f.Add([]byte(`{"version":2,"templates":[{"id":"x","scope":"spread","boxes":[{"x":-1,"y":2,"w":1e300,"h":0,"rot":-725}]},{"id":"x","scope":"page","boxes":[]}]}`))
	f.Add([]byte(`{"version":4,"templates":[{"id":"v","scope":"page","kind":"vignette","boxes":[{"w":0.1,"h":0.1}],"vig":{"roles":["group"],"pages":3}}]}`))
	f.Add([]byte(`{"version":99,"templates":{"новый":"формат"}}`))
	f.Add([]byte(`{"version":0}`))
	f.Add([]byte(`[]`))
}

// FuzzParseTemplates — templates.json из чужого файла (086: «Загрузить из файла…»): без паники,
// не больше MaxTemplates; записанный итог ≤ MaxTemplatesFile и разбирается обратно тем же
// списком (доли — с точностью округления файла).
func FuzzParseTemplates(f *testing.F) {
	fuzzTplSeeds(f)
	f.Fuzz(func(t *testing.T, data []byte) {
		list, _, err := ParseTemplates(data)
		if err != nil {
			return
		}
		if len(list) > MaxTemplates {
			t.Fatalf("шаблонов %d > %d", len(list), MaxTemplates)
		}
		b, err := MarshalTemplates(list)
		if err != nil {
			t.Fatalf("принятый список не пишется: %v", err)
		}
		if len(b) > MaxTemplatesFile {
			t.Fatalf("запись %d байт > %d", len(b), MaxTemplatesFile)
		}
		again, ver, err := ParseTemplates(b)
		if err != nil || ver > TemplatesVersion {
			t.Fatalf("записанное не читается: %v, версия %d\n%s", err, ver, b)
		}
		if err := nearJSON(RoundTemplates(list), again); err != nil {
			t.Fatalf("круг записи изменил список: %v\n%s", err, b)
		}
	})
}

// nearJSON — a и b совпадают как JSON, числа — до 2/tplPrec: округление файла может сдвинуть
// место у края области на шаг, и зажим возвращает его внутрь.
func nearJSON(a, b any) error {
	var x, y any
	for _, p := range []struct {
		v   any
		out *any
	}{{a, &x}, {b, &y}} {
		data, err := json.Marshal(p.v)
		if err != nil {
			return err
		}
		if err := json.Unmarshal(data, p.out); err != nil {
			return err
		}
	}
	return nearAny("", x, y)
}

func nearAny(at string, x, y any) error {
	switch xv := x.(type) {
	case float64:
		if yv, ok := y.(float64); !ok || math.Abs(xv-yv) > 2/tplPrec {
			return fmt.Errorf("%s: %v ≠ %v", at, x, y)
		}
	case map[string]any:
		yv, ok := y.(map[string]any)
		if !ok || len(xv) != len(yv) {
			return fmt.Errorf("%s: поля %v ≠ %v", at, x, y)
		}
		for k, v := range xv {
			if err := nearAny(at+"."+k, v, yv[k]); err != nil {
				return err
			}
		}
	case []any:
		yv, ok := y.([]any)
		if !ok || len(xv) != len(yv) {
			return fmt.Errorf("%s: длина %v ≠ %v", at, x, y)
		}
		for i := range xv {
			if err := nearAny(fmt.Sprintf("%s[%d]", at, i), xv[i], yv[i]); err != nil {
				return err
			}
		}
	default:
		if x != y {
			return fmt.Errorf("%s: %v ≠ %v", at, x, y)
		}
	}
	return nil
}
