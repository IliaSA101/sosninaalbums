package decor

import (
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// TestCheckTplLibsFrame — шаблон с рамочкой-картинкой (131 G1): ссылка на рамочку этого набора
// проходит и попадает в ссылки шаблона (их держит сборка мусора); на чужую, на картинку другого
// вида и на неизвестную встроенную рамочку — пропуск с причиной.
func TestCheckTplLibsFrame(t *testing.T) {
	s, _ := openTemp(t)
	ring := pngBytes(t, ringImage(120, 160, 24, 32, 24, 32))
	good := renderPNG(t, 44, 30)
	tpl := func(b model.TplBorder) []byte {
		return tplJSON(t, model.SpreadTpl{V: 1, Pages: 1, Page: []model.TplBg{{}}, Places: []model.Box{{X: .1, Y: .1, W: .5, H: .5}},
			Borders: []model.TplBorder{b}})
	}
	m, src := testPack("rama-tpl", 1, pit{"frames/ring.png", ring}, pit{"bg/good.png", good},
		pit{"spreads/ok.json", tpl(model.TplBorder{Lib: libKey(ring), Scale: 1.2})},
		pit{"spreads/foreign.json", tpl(model.TplBorder{Lib: "my:0123456789abcdef"})},
		pit{"spreads/wrongkind.json", tpl(model.TplBorder{Lib: libKey(good)})},
		pit{"spreads/future.json", tpl(model.TplBorder{Kind: "zzz-future"})},
		pit{"spreads/builtin.json", tpl(model.TplBorder{Kind: "line"})})
	st, err := s.StagePack(m, src, OriginFile, "")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	var why []string
	for _, k := range st.Skipped {
		why = append(why, k.Name+": "+k.Why)
	}
	want := []string{"spreads/future.json: шаблон из новой версии программы",
		"spreads/foreign.json: шаблон ссылается на фон, стикер или рамочку не из набора",
		"spreads/wrongkind.json: шаблон ссылается на фон, стикер или рамочку не из набора"}
	if !slices.Equal(why, want) || len(st.items) != 4 {
		t.Fatalf("пропуски %q, прошло %d", why, len(st.items))
	}
	if _, err := s.InstallPack(st, nil); err != nil {
		t.Fatal(err)
	}
	ok := model.TplKey(mustTpl(t, tpl(model.TplBorder{Lib: libKey(ring), Scale: 1.2})))
	if info := s.tplLookup(ok); !info.ok || !slices.Contains(info.libs, libRef{KindFrame, libKey(ring)}) {
		t.Errorf("ссылка шаблона на рамочку не попала в libs (сборка мусора её не увидит): %+v", info.libs)
	}
}
