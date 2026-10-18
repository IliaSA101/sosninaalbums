package edit

import (
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// kit — Methods со считающим Host: сколько раз правка дошла до Changed (автосохранение и
// событие view у App), какие тосты ушли в Notice, какие списки фото запрашивали у ленты.
// panicNext — следующий вызов Editor() паникует (дальше Editor работает: Fail берёт вид).
type kit struct {
	*Methods
	ed        *album.Editor
	calls     int
	outs      int // ответы без правки (Host.Out): в цепочку видов, но не в Changed
	notices   []string
	asked     [][]string
	panicNext bool
	lent      map[string]model.PhotoInfo // фото ленты, которых нет в проекте
}

// newKit — kit поверх проекта p; сведения о фото (Infos) отдаются из ленты lent,
// InfosOrRefs — ещё и из проекта, как у App.
func newKit(t testing.TB, p *model.Project) *kit {
	t.Helper()
	ed, err := album.NewEditor(p, "")
	if err != nil {
		t.Fatal(err)
	}
	k := &kit{ed: ed, lent: map[string]model.PhotoInfo{}}
	infos := func(ids []string) []model.PhotoInfo {
		k.asked = append(k.asked, ids)
		var out []model.PhotoInfo
		for _, id := range ids {
			if pi, ok := k.lent[id]; ok {
				out = append(out, pi)
			}
		}
		return out
	}
	k.Methods = New(Host{
		Editor: func() *album.Editor {
			if k.panicNext {
				k.panicNext = false
				panic("тестовая паника")
			}
			return ed
		},
		Changed: func(v model.View) model.View { k.calls++; return v },
		Out:     func(v model.View) model.View { k.outs++; return v },
		Notice:  func(s string) { k.notices = append(k.notices, s) },
		Infos:   infos,
		InfosOrRefs: func(ids []string) []model.PhotoInfo {
			out := infos(ids)
			for _, id := range ids {
				if len(out) > 0 {
					break
				}
				if r, ok := ed.Ref(id); ok {
					out = append(out, model.PhotoInfo{ID: id, Path: r.Path, W: r.W, H: r.H, Size: r.Size})
				}
			}
			return out
		},
		Templates: func() []model.Template { return nil },
	})
	return k
}

// lentPhoto кладёт в ленту фото id (его нет в проекте) и возвращает его сведения.
func (k *kit) lentPhoto(id string) model.PhotoInfo {
	pi := model.PhotoInfo{ID: id, Path: `C:\lent\` + id + ".jpg", W: 300, H: 200, Size: 10}
	k.lent[id] = pi
	return pi
}

// rev — текущая ревизия проекта.
func (k *kit) rev() int { return k.ed.View().Rev }

// twoSpreads — «Авто» s1 [a, b] и «Авто» s2 [c].
func twoSpreads(t testing.TB) *kit {
	t.Helper()
	p := model.New()
	for i, ids := range [][]string{{"a", "b"}, {"c"}} {
		p.Spreads = append(p.Spreads, &model.Spread{ID: "s" + string(rune('1'+i)), Photos: ids})
		for _, id := range ids {
			p.Photos[id] = model.PhotoRef{Path: `C:\` + id + ".jpg", W: 300, H: 200}
		}
	}
	return newKit(t, p)
}
