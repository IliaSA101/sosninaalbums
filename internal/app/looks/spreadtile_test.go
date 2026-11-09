package looks

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album/model/looktpl"
	"github.com/IliaSA101/sosninaalbums/internal/decor"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Плитка 256 px рисуется быстро (design §3: гипотеза ≤ 30 мс; кэш в обход — drawTpl).
func TestMediaTplFast(t *testing.T) {
	s := decor.Open(testkit.TempDir(t))
	var worst time.Duration
	for _, id := range []string{"sa:tpl-ivory-hero", "sa:tpl-stars", "sa:tpl-watercolor"} {
		tp, err := s.SpreadTpl(id)
		if err != nil {
			t.Fatal(err)
		}
		d := testkit.Fastest(3, func() {
			if _, err := drawTpl(tp, tplQuery{px: 256, r: 1.5}, nil); err != nil {
				t.Fatal(err)
			}
		})
		worst = max(worst, d)
	}
	testkit.CostTest(t)
	if limit := testkit.Limit(30 * time.Millisecond); worst > limit {
		t.Errorf("плитка %v, предел %v", worst, limit)
	}
}

// Разбор запроса плитки: принятое — в пределах; лишнее и повторы — отказ.
func FuzzMediaTplQuery(f *testing.F) {
	for _, s := range []string{"v=1&px=256&r=2", "px=64&r=0.2", "px=1024&r=5", "px=256&r=1e400", "px=256&r=-0", "px=%zz", "v=99999&px=256&r=1"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, raw string) {
		q, ok := parseTplQuery(raw)
		if ok && (q.px < tplMinPx || q.px > tplMaxPx || !(q.r >= tplMinR && q.r <= tplMaxR)) {
			t.Fatalf("%q: %+v", raw, q)
		}
	})
}

// TestTplKeysBorders — ключи шаблона для Service.TplLibs: фоны, стикеры и рамочки-картинки
// (131 G1); встроенная рамочка ключа не даёт.
func TestTplKeysBorders(t *testing.T) {
	tp := looktpl.SpreadTpl{Page: []looktpl.TplBg{{Lib: "my:00000000000000b1"}}, Whole: &looktpl.TplBg{Lib: "my:00000000000000b2"},
		Stickers: []looktpl.TplItem{{Lib: "my:00000000000000c1"}, {}},
		Borders:  []looktpl.TplBorder{{I: 0, Kind: "line"}, {I: 1, Lib: "my:00000000000000d1"}}}
	want := []string{"my:00000000000000b2", "my:00000000000000b1", "my:00000000000000c1", "my:00000000000000d1"}
	if got := tplKeys(tp); !slices.Equal(got, want) {
		t.Errorf("ключи %v, ждали %v", got, want)
	}
	if got := tplKeys(looktpl.SpreadTpl{}); len(got) != 0 {
		t.Errorf("пустой шаблон: %v", got)
	}
}

// TestServeTplBounds: чужой путь — не наш (false); без библиотеки — 404; с настоящей библиотекой
// и встроенным шаблоном мусорный запрос и не-GET/HEAD — 404, а тот же корректный GET — 200.
func TestServeTplBounds(t *testing.T) {
	h, _ := fakeHost(t)
	l := New(h)
	if l.ServeTpl(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/media/look/0123456789abcdef", nil)) {
		t.Error("чужой путь принят")
	}
	w := httptest.NewRecorder()
	if !l.ServeTpl(w, httptest.NewRequest(http.MethodGet, "/media/tpl/sa:tpl-stars?px=256&r=1", nil)) || w.Code != http.StatusNotFound {
		t.Errorf("без библиотеки: код %d", w.Code)
	}

	// Настоящее хранилище: до проверки запроса и метода доходит только оно, не s == nil.
	s := decor.Open(testkit.TempDir(t))
	h.Store = func() (*decor.Store, error) { return s, nil }
	l = New(h)
	const ok = "/media/tpl/sa:tpl-stars?px=256&r=1"
	// Отказ по !okQ снаружи не отличим: без него q нулевой, и рисование всё равно даёт 404
	// (глубокая защита). Различимы метод, шаблон и s == nil; разбор запроса сторожит FuzzMediaTplQuery.
	for _, c := range []struct {
		method, url string
		code        int
	}{
		{http.MethodGet, ok, http.StatusOK}, // контроль: иначе 404 ниже ничего не доказывают
		{http.MethodHead, ok, http.StatusOK},
		{http.MethodGet, "/media/tpl/sa:tpl-stars?px=1&r=1", http.StatusNotFound}, // мусорный запрос
		{http.MethodGet, "/media/tpl/sa:tpl-stars", http.StatusNotFound},          // без px и r
		{http.MethodPost, ok, http.StatusNotFound},
		{http.MethodPut, ok, http.StatusNotFound},
		{http.MethodGet, "/media/tpl/sa:tpl-net-takogo?px=256&r=1", http.StatusNotFound}, // нет шаблона
	} {
		w := httptest.NewRecorder()
		if !l.ServeTpl(w, httptest.NewRequest(c.method, c.url, nil)) || w.Code != c.code {
			t.Errorf("%s %s: код %d, ждали %d", c.method, c.url, w.Code, c.code)
		}
	}
}
