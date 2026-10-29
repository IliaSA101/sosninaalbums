package exporter

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/export"
	"github.com/IliaSA101/sosninaalbums/internal/export/printshop"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// fakeHost — Host без App: чистый альбом без разворотов, события и вопросы в журнале.
func fakeHost(t *testing.T, ed func() *album.Editor) (h Host, urls *[]string, shown *[]string) {
	t.Helper()
	urls, shown = new([]string), new([]string)
	h = Host{
		Editor:      ed,
		Exists:      func(string) bool { return true },
		Emit:        func(string, ...any) {},
		GoBG:        func(func()) bool { return false },
		StopCtx:     context.Background,
		ColorReport: func(*model.Project) (map[string]int, map[string]int) { return nil, nil },
		Ask:         func(string) (bool, error) { t.Error("вопрос до отказа"); return false, nil },
		PickDir:     func(string) (string, error) { t.Error("выбор папки до отказа"); return "", nil },
		RememberDir: func(string) {},
		OpenURL:     func(u string) { *urls = append(*urls, u) },
		ShowDir:     func(d string) { *shown = append(*shown, d) },
		LiftMemory:  func() func() { return func() {} },
	}
	return h, urls, shown
}

func blankEditor(t *testing.T) *album.Editor {
	t.Helper()
	ed, err := album.NewEditor(model.New(), "")
	if err != nil {
		t.Fatal(err)
	}
	return ed
}

// Флаг «идёт» ставится до первого шага: пока первый вызов ещё в проверках, второй получает отказ, а
// после отказа первого (нечего выгружать) флаг снят и службу можно звать снова.
func TestExportBusyFlag(t *testing.T) {
	ed := blankEditor(t)
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	h, _, _ := fakeHost(t, func() *album.Editor {
		once.Do(func() { close(entered); <-release })
		return ed
	})
	s := New(h)
	type res struct {
		ok  bool
		err error
	}
	first := make(chan res)
	go func() {
		ok, err := s.Export(Options{})
		first <- res{ok, err}
	}()
	<-entered
	if busy, cancelable := s.Busy(); !busy || cancelable {
		t.Fatalf("первый вызов в проверках: идёт %v, отмена %v", busy, cancelable)
	}
	if ok, err := s.Export(Options{}); ok || err == nil || err.Error() != "экспорт уже идёт" {
		t.Fatalf("второй вызов: %v, %v", ok, err)
	}
	close(release)
	r := <-first
	if r.ok || r.err == nil || !strings.Contains(r.err.Error(), "нечего экспортировать") {
		t.Fatalf("пустой альбом: %v, %v", r.ok, r.err)
	}
	if busy, _ := s.Busy(); busy {
		t.Fatal("флаг остался после отказа")
	}
	if _, err := s.Export(Options{}); err == nil || err.Error() == "экспорт уже идёт" {
		t.Fatalf("после отказа служба должна принимать вызов: %v", err)
	}
}

// spreadsEditor — редактор с n пустыми разворотами: отказы по диапазону и холсту идут до проверки
// «нечего экспортировать», на пустом альбоме все случаи дали бы только её.
func spreadsEditor(t *testing.T, n int, mod func(*model.Project)) *album.Editor {
	t.Helper()
	p := model.New()
	for i := range n {
		p.Spreads = append(p.Spreads, &model.Spread{ID: "s" + string(rune('1'+i))})
	}
	if mod != nil {
		mod(p)
	}
	ed, err := album.NewEditor(p, "")
	if err != nil {
		t.Fatal(err)
	}
	return ed
}

// Каждый отказ до диалога снимает флаг и не трогает окна: неизвестная типография, плохой диапазон,
// диапазон за пределом, холст больше предела; у каждого свой текст ошибки.
func TestExportRefusalsBeforeDialogs(t *testing.T) {
	huge := func(p *model.Project) { p.Settings.PageW, p.Settings.PageH, p.Settings.DPI = 1000, 1000, 1200 }
	cases := []struct {
		name string
		n    int
		mod  func(*model.Project)
		o    Options
		want string
	}{
		{"типография", 2, nil, Options{Print: "нет-такой"}, "неизвестная типография"},
		{"диапазон-мусор", 2, nil, Options{Range: "x"}, "не понимаю «x»"},
		{"диапазон-за-пределом", 1, nil, Options{Range: "99", Pages: true}, "разворота"},
		{"диапазон-наоборот", 2, nil, Options{Range: "2-1"}, "задан наоборот"},
		{"холст", 2, huge, Options{}, "слишком большой"},
		{"холст-по-страницам", 2, huge, Options{Pages: true}, "слишком большой"},
		{"пустой-альбом", 0, nil, Options{Range: "1"}, "нечего экспортировать"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ed := spreadsEditor(t, c.n, c.mod)
			h, _, _ := fakeHost(t, func() *album.Editor { return ed })
			s := New(h)
			ok, err := s.Export(c.o)
			if ok || err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("%+v: %v, %v; ждали «%s»", c.o, ok, err, c.want)
			}
			if busy, _ := s.Busy(); busy {
				t.Fatal("флаг остался")
			}
			if s.LastDir() != "" {
				t.Fatalf("папка запомнена без выгрузки: %q", s.LastDir())
			}
		})
	}
}

// Остановка без фона — ничего; Show отдаёт хосту папку последней выгрузки как есть.
func TestCancelAndShow(t *testing.T) {
	h, _, shown := fakeHost(t, func() *album.Editor { return nil })
	s := New(h)
	s.Cancel()
	called := 0
	s.mu.Lock()
	s.dir, s.cancel = "/x/y", func() { called++ }
	s.mu.Unlock()
	if busy, cancelable := s.Busy(); busy || !cancelable {
		t.Fatalf("есть отмена без флага: идёт %v, отмена %v", busy, cancelable)
	}
	s.Cancel()
	s.Show()
	if called != 1 || len(*shown) != 1 || (*shown)[0] != "/x/y" || s.LastDir() != "/x/y" {
		t.Fatalf("отмена %d раз, папка %q", called, *shown)
	}
}

func TestExportUnits(t *testing.T) {
	if u, err := exportUnits("  ", 5, false); u != nil || err != nil {
		t.Fatalf("пустой диапазон — все: %v, %v", u, err)
	}
	if u, err := exportUnits("2-3", 5, false); len(u) == 0 || err != nil {
		t.Fatalf("2-3: %v, %v", u, err)
	}
	if _, err := exportUnits("9", 5, false); err == nil || !strings.Contains(err.Error(), "разворота") {
		t.Fatalf("за пределом у альбома: %v", err)
	}
	// У листа 031 единица — лист: слово в ошибке меняется, «л»/«п» отказ.
	if _, err := exportUnits("9", 1, true); err == nil || strings.Contains(err.Error(), "разворота") || !strings.Contains(err.Error(), "листа") {
		t.Fatalf("за пределом у листа: %v", err)
	}
	if _, err := exportUnits("1л", 1, true); err == nil {
		t.Fatal("«л» у листа должна отказывать")
	}
	if _, err := exportUnits("", 0, false); err != nil {
		t.Fatal(err)
	}
}

func TestBookDirAndQuestions(t *testing.T) {
	dir := testkit.TempDir(t)
	if b, mk, err := bookDir(nil, dir); b != dir || mk != "" || err != nil {
		t.Fatalf("без типографии папка как есть: %q %q %v", b, mk, err)
	}
	prof, ok := printshop.ProfileOf("fabrika")
	if !ok {
		t.Fatal("нет встроенной типографии fabrika")
	}
	pj := &printJob{prof: prof, plan: printshop.Plan{Folder: "Альбом"}}
	b, mk, err := bookDir(pj, dir)
	if err != nil || mk != b || !strings.HasSuffix(b, "Альбом") {
		t.Fatalf("книга ещё не создана: %q %q %v", b, mk, err)
	}
	if q := extraQuestion(prof, dir, nil); q != "" {
		t.Fatalf("пустая папка — без вопроса: %q", q)
	}
	// Без обложки в режиме типографии — вопрос профиля; без режима и без неё — пусто.
	if q := coverQuestion(pj, nil, model.View{}); q != prof.NoCoverQuestion() || q == "" {
		t.Fatalf("вопрос об обложке типографии: %q", q)
	}
	if q := coverQuestion(nil, nil, model.View{}); q != "" {
		t.Fatalf("обычная выгрузка без обложки: %q", q)
	}
	if q := coverQuestion(nil, &export.CoverJob{}, model.View{Cover: &model.CoverView{Fix: "x"}}); q != printshop.CoverFixQuestion("x") {
		t.Fatalf("обложка не по шаблону: %q", q)
	}
	// Запрет типографии (в чистом проекте нет разворотов) — ошибка, настройки окна не тронуты.
	o := Options{Range: "1", DPI: 72}
	if pj, err := printOptions(model.New(), "", &o, prof); err == nil || pj != nil || o != (Options{Range: "1", DPI: 72}) {
		t.Fatalf("запрет типографии: %v, %v, %+v", pj, err, o)
	}
}

func TestOpenPrintSiteAndPlans(t *testing.T) {
	ed := blankEditor(t)
	h, urls, _ := fakeHost(t, func() *album.Editor { return ed })
	s := New(h)
	s.OpenPrintSite("нет-такой")
	if len(*urls) != 0 {
		t.Fatalf("неизвестная типография открыла %q", *urls)
	}
	s.OpenPrintSite("fabrika")
	if len(*urls) != 1 || !strings.HasPrefix((*urls)[0], "https://") {
		t.Fatalf("адрес типографии: %q", *urls)
	}
	plans := s.PrintPlans("")
	if len(plans) == 0 {
		t.Fatal("планов нет")
	}
	if s.PrintPlans("fabrika") == nil {
		t.Fatal("с запомненным выбором планов нет")
	}
}
