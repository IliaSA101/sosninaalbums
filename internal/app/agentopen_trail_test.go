package app

// Тихое открытие и хвост «пробел/точка» в элементе пути (160 Т2, М5): на Windows Win32 срезает хвостовые точки и
// пробелы, и `<место>\Петровы\.. \x.jpg` на диске лежит выше проверенной папки. Такие строки — place, диск не
// трогается. Правило проверяется на любой ОС через шов quietTrail; настоящий обход на диске — в _windows_test.go.

import (
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// trailRule включает правило Windows для хвоста «пробел/точка» на любой ОС (на Windows оно и так действует).
func trailRule(t *testing.T) {
	t.Helper()
	old := quietTrail
	t.Cleanup(func() { quietTrail = old })
	quietTrail = func(s string) bool { return paths.TrailingDotFor(true, s) }
}

// trailPaths — строки с хвостом «пробел/точка» в элементе: «.. » (выход на уровень выше после среза), «a.», «a », «...»; хвост у самого файла отсекает
// расширение (x.jpg. — не фото).
func trailPaths(base string) map[string]string {
	s := string(filepath.Separator)
	return map[string]string{
		"dotdot_space": base + s + ".. " + s + "x.jpg",
		"two_up":       base + s + ".. " + s + ".. " + s + "x.jpg",
		"dir_dot":      base + s + "a." + s + "x.jpg",
		"dir_space":    base + s + "a " + s + "x.jpg",
		"dots_only":    base + s + "..." + s + "x.jpg",
	}
}

// ranOrAsked ждёт итог открытия: вопрос человеку (ответ — нет) или тихое завершение. asked = true — спросили.
func (r *oqRig) ranOrAsked(t *testing.T, c *askCall) (why string, asked bool) {
	t.Helper()
	n := len(r.evOf("agents:ask"))
	deadline := time.Now().Add(testkit.Limit(10 * time.Second))
	for time.Now().Before(deadline) {
		select {
		case <-c.done:
			return "", len(r.evOf("agents:ask")) > n
		default:
		}
		if ev := r.evOf("agents:ask"); len(ev) > n {
			q := ev[n].(AgentAsk)
			if _, err := r.a.AnswerAgent(q.ID, false); err != nil {
				t.Fatal(err)
			}
			c.wait(t)
			return q.Why, true
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatal("открытие не завершилось и не спросило")
	return "", false
}

// watchTouch считает случаи, когда диск трогают по пути, который не прошёл бы quietPlain, или по пути, который Win32
// приводит за пределы мест (filepath.Abs на Windows — GetFullPathName).
func (r *oqRig) watchTouch(t *testing.T) *atomic.Int32 {
	t.Helper()
	var touched atomic.Int32
	old := quietAllowed
	t.Cleanup(func() { quietAllowed = old })
	quietAllowed = func(f func(string) bool) func(string) bool {
		return func(p string) bool {
			ok := f(p)
			if !ok {
				return false
			}
			abs, err := filepath.Abs(p)
			inside := err == nil && (paths.FoldEqual(abs, r.place) || r.a.askM().rules().Inside(abs, r.place))
			if !quietPlain(p) || !inside && !r.cfgPath(abs) {
				touched.Add(1)
			}
			return true
		}
	}
	return &touched
}

// cfgPath — путь в папке настроек (там библиотека своих фонов).
func (r *oqRig) cfgPath(p string) bool {
	return paths.FoldEqual(filepath.Dir(p), filepath.Clean(r.a.cfgDir)) || r.a.askM().rules().Inside(p, r.a.cfgDir)
}

// TestOpenQuietTrailingDot: каждая строка с хвостом — place, вопрос человеку, диск вне мест не тронут. Фото без
// разворота, отдельное, запасное и вложенное свёрнутое проверяются так же, как строки сухого перечня.
func TestOpenQuietTrailingDot(t *testing.T) {
	trailRule(t)
	r := newOQRig(t)
	touched := r.watchTouch(t)
	for name, p := range trailPaths(r.shoot) {
		for kind, f := range map[string]salbum.FileProject{
			"loose":  {Loose: loose(p)},
			"nospr":  {Folders: []salbum.FileFolder{{Path: r.shoot}}, Photos: map[string]salbum.FilePhoto{"cccccccccccc": {PhotoRef: model.PhotoRef{Path: p}}}},
			"nested": {Folders: []salbum.FileFolder{{Path: r.shoot}}, Loose: loose(p)},
		} {
			// Фото без разворота в сухом перечне нет (PruneRefs после загрузки): альбом может открыться тихо, но диск по
			// такой строке не трогается — это ловит watchTouch. Строки перечня (loose, вложенное) — place.
			alb := r.rawAlbum(t, name+"_"+kind, f)
			why, asked := r.ranOrAsked(t, r.ask(r.req(alb)))
			if kind != "nospr" && (!asked || why != whyPlace) {
				t.Errorf("%s/%s: вопрос %v, Why = %q, ждали place", name, kind, asked, why)
			}
			if asked && r.a.editor().Path() != r.prev {
				t.Fatalf("%s/%s: после отказа альбом сменился: %q", name, kind, r.a.editor().Path())
			}
			if !asked { // открылся тихо: вернуть прежний для следующего круга
				if _, err := r.a.openPath(r.prev); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	// Хвост в папке альбома и в запасном пути.
	for name, p := range trailPaths(r.shoot) {
		r.asks(t, "folder_"+name, r.rawAlbum(t, "Папка_"+name, salbum.FileProject{Folders: []salbum.FileFolder{{Path: p}}}), whyPlace)
		r.asks(t, "alt_"+name, r.altAlbum(t, "Запас_"+name, r.shoot, p), whyPlace)
	}
	if touched.Load() != 0 {
		t.Errorf("диск тронут по путям с хвостом или вне мест: %d раз", touched.Load())
	}
}

// TestOpenQuietTrailingDotUnits: allowed и openFullWhy отвечают «нет» на хвост «пробел/точка» первой проверкой, а
// «.», «..» в составе канонического пути и обычные точки в именах не мешают.
func TestOpenQuietTrailingDotUnits(t *testing.T) {
	trailRule(t)
	r := newOQRig(t)
	good := filepath.Join(r.shoot, "Кадр_0.jpg")
	pl := &openPlan{rules: r.a.askM().rules(), rows: map[string]bool{good: true}, checked: map[string]bool{r.shoot: true}}
	if !pl.allowed(good) {
		t.Fatalf("обычный путь в проверенной папке не принят")
	}
	hidden := filepath.Join(r.shoot, ".hidden", "a.b.jpg")
	pl.checked[filepath.Dir(hidden)] = true
	if !pl.allowed(hidden) {
		t.Errorf("точка в начале и середине имени мешает: %q", hidden)
	}
	full := model.New()
	for name, p := range trailPaths(r.shoot) {
		pl.checked[filepath.Dir(p)], pl.rows[p] = true, true // папка и строка «проверены»: остановить должен только хвост
		if pl.allowed(p) {
			t.Errorf("allowed принял %s: %q", name, p)
		}
		full.Loose = []model.LooseAdd{{ID: "0123456789ab", Files: []model.LooseFile{{Path: p}}}}
		if why := r.a.openFullWhy(pl, full); why != whyPlace {
			t.Errorf("openFullWhy(%s) = %q, ждали place", name, why)
		}
	}
}
