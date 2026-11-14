package looks

import (
	"reflect"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/model/looktpl"
)

// testLook — макет: титул, повторяемый шаг, финал и обложка; в каждом шаге два места
// (копия помощника тестов App: пакеты тестов не общие).
func testLook(color string) looktpl.AlbumTpl {
	tpl := looktpl.SpreadTpl{V: looktpl.TplV, Pages: 2, Ratio: 2, Places: []model.Box{{X: 0.05, Y: 0.1, W: 0.4, H: 0.8}, {X: 0.55, Y: 0.1, W: 0.4, H: 0.8}},
		Page: []looktpl.TplBg{{Color: color}, {Color: color}}}
	side := looktpl.SpreadTpl{V: looktpl.TplV, Pages: 1, Ratio: 1, Places: []model.Box{{X: 0.2, Y: 0.2, W: 0.6, H: 0.6}}, Page: []looktpl.TplBg{{Color: color}}}
	return looktpl.AlbumTpl{V: looktpl.LookV,
		Cover: &looktpl.CoverTpl{Bg: &looktpl.TplBg{Color: color}, Front: &side},
		Steps: []looktpl.LookStep{{Name: "Титул", Tpl: tpl}, {Role: looktpl.RoleRepeat, Tpl: tpl}, {Name: "Финал", Tpl: tpl}}}
}

func TestLookTile(t *testing.T) {
	l := testLook("#ffffff")
	l.Steps = append(l.Steps, looktpl.LookStep{Role: looktpl.RoleVignette, Tpl: l.Steps[0].Tpl, Vig: &looktpl.LookVig{Tpl: "classic", Teachers: 1, Pupils: 4}})
	l.Steps[0].Tpl.Texts = []looktpl.TplText{{Text: "Привет", W: 0.5}}
	one := func(i int, bg, front bool, r float64) (looktpl.SpreadTpl, bool) {
		t.Helper()
		tp, ok := lookTile(l, i, bg, front, r)
		if ok && len(tp) != 1 {
			t.Fatalf("шаг %d: шаблонов %d, ждали 1", i, len(tp))
		}
		if !ok {
			return looktpl.SpreadTpl{}, false
		}
		return tp[0], true
	}
	if tp, ok := one(1, true, false, 1.5); !ok || len(tp.Places) != 0 || len(tp.Texts) != 0 || tp.Page[0].Color != "#ffffff" {
		t.Errorf("bg: %+v", tp)
	}
	if tp, ok := one(1, false, false, 1); !ok || len(tp.Places) != 2 || len(tp.Texts) != 1 || tp.Places[0].Y == l.Steps[0].Tpl.Places[0].Y {
		t.Errorf("места по LookBoxes для r=1: %+v", tp.Places) // композиция Ratio 2 в квадрате ниже
	}
	if tp, ok := one(4, false, false, 1.5); !ok || len(tp.Places) != 0 {
		t.Errorf("виньетка: %+v", tp)
	}
	// Обложка: две стороны рядом (задней у testLook нет — пустая страница с фоном листа).
	cv, ok := lookTile(l, 0, false, false, 1.5)
	if !ok || len(cv) != 2 || len(cv[0].Places) != 0 || len(cv[1].Places) != 1 || cv[0].Page[0].Color != "#ffffff" || cv[1].Pages != 1 {
		t.Fatalf("обложка: %+v", cv)
	}
	// Места лицевой — в её зоне (вся сторона, отношение r/2 = 0,75): Ratio 1 вписывается по высоте, по центру.
	if want := looktpl.LookBoxes(*l.Cover.Front, 0.75); !reflect.DeepEqual(cv[1].Places, want) {
		t.Errorf("места лицевой: %v, ждали %v", cv[1].Places, want)
	}
	if fr, ok := lookTile(l, 0, false, true, 0.75); !ok || len(fr) != 1 || !reflect.DeepEqual(fr[0].Places, cv[1].Places) {
		t.Errorf("f=1 (r — отношение стороны) совпадает с правой частью листа: %+v", fr)
	}
	if bgT, ok := lookTile(l, 0, true, false, 1.5); !ok || len(bgT) != 1 || bgT[0].Pages != 2 || len(bgT[0].Places) != 0 {
		t.Errorf("фон листа обложки — один разворот: %+v", bgT)
	}
	if bgF, ok := lookTile(l, 0, true, true, 0.75); !ok || len(bgF) != 1 || bgF[0].Pages != 1 || len(bgF[0].Places) != 0 {
		t.Errorf("фон лицевой — одна страница без мест: %+v", bgF)
	}
	for _, i := range []int{-1, 5, 25} {
		if _, ok := lookTile(l, i, false, false, 1); ok {
			t.Errorf("шаг %d", i)
		}
	}
	l.Cover = nil
	for _, front := range []bool{false, true} {
		if _, ok := lookTile(l, 0, false, front, 1); ok {
			t.Error("обложки нет")
		}
	}
}

// Разбор запроса плитки макета: принятое — в пределах белого списка.
func FuzzMediaLookQuery(f *testing.F) {
	for _, s := range []string{"v=1&i=0&px=256&r=2&bg=0", "i=24&px=64&r=0.2&bg=1", "i=1&px=1024&r=5", "i=1&px=2048&r=5&ph=9007199254740991", "i=1&px=256&r=1&ph=0", "i=1&px=1&r=1e400", "i=%zz", "v=99999&i=1&px=256&r=1", "i=0&px=256&r=0.5&f=1", "i=1&px=256&r=1&f=1", ""} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, raw string) {
		q, ok := parseLookQuery(raw)
		if ok && q.front && q.i != 0 {
			t.Fatalf("%q: f=1 только у обложки: %+v", raw, q)
		}
		if ok && (q.i < 0 || q.i > model.MaxLookSteps || q.px < MinPx || q.px > lookPhotoPx || q.px > MaxPx && q.ph == 0 || !(q.r >= MinR && q.r <= MaxR)) {
			t.Fatalf("%q: %+v", raw, q)
		}
	})
}

func TestParseLookQueryPh(t *testing.T) {
	const base = "i=1&px=256&r=1"
	for _, c := range []struct {
		raw string
		ph  uint64
		px  int
	}{
		{base + "&ph=1", 1, 256},
		{base + "&ph=0", 0, 256},
		{base + "&ph=9007199254740991", 1<<53 - 1, 256},
		{base + "&ph=0000000000000007", 7, 256}, // 16 цифр
		{"i=0&px=2048&r=2&ph=5", 5, 2048},
		{"ph=5&r=2&px=1500&i=3", 5, 1500}, // порядок не важен: px проверяется после разбора всех
		{base, 0, 256},
	} {
		q, ok := parseLookQuery(c.raw)
		if !ok || q.ph != c.ph || q.px != c.px {
			t.Errorf("%s: %+v %v", c.raw, q, ok)
		}
	}
	for _, raw := range []string{
		base + "&ph=", base + "&ph=-1", base + "&ph=+1", base + "&ph=1.5", base + "&ph=1e3", base + "&ph=0x10", base + "&ph=abc",
		base + "&ph=1_0", base + "&ph=%20", base + "&ph=1&ph=2", base + "&ph=1&ph=1",
		base + "&ph=9007199254740992",              // 2⁵³
		base + "&ph=00000000000000007",             // 17 цифр
		base + "&ph=12345678901234567890",          // больше 2⁵³
		base + "&ph=18446744073709551616",          // больше uint64
		"i=1&px=2048&r=1",                          // 2048 без ph
		"i=1&px=1025&r=1&ph=0",                     // ph=0 — прежний предел 1024
		"i=1&px=2049&r=1&ph=1",                     // выше 2048 нельзя и с ph
		"i=1&px=63&r=1&ph=1",                       // ниже предела
		base + "&ph=1&bg=2", base + "&ph=1&path=x", // остальные параметры не ослаблены
	} {
		if q, ok := parseLookQuery(raw); ok {
			t.Errorf("%s: принято %+v", raw, q)
		}
	}
}
