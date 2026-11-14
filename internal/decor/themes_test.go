package decor

import (
	"bytes"
	"log"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/model/looktpl"
)

// Таблица тем наборов (спека 170, design §4): порядок, доверие, общие id, пределы, свои в теме набора.

// thName — тема набора для теста: id, имя, база, order.
func thName(id, name, base string, order int) PackTheme {
	return PackTheme{ID: id, Name: name, Base: base, Order: order}
}

// lookData — макет-предмет теста: у каждой пары (seed, i) свои байты, значит свой ключ. Макеты —
// потому что у вида «макет альбома» нет встроенных плиток: числа тем считаются без примеси.
func lookData(t *testing.T, seed, i int) []byte {
	t.Helper()
	l := lookOf("", "")
	l.Steps[0].Name = "Титул " + string(rune('а'+seed)) + string(rune('а'+i))
	return lookBytes(t, l, "")
}

// topicPack — набор макетов, по одному на тему из topics; предмет i лежит в базе base, а его topic —
// topics[i] ("" — без topic, остаётся в базе). seed разводит наборы между собой.
func topicPack(t *testing.T, id, base string, seed int, themes []PackTheme, topics ...string) (PackManifest, fakeSrc) {
	t.Helper()
	var items []pit
	for i := range topics {
		items = append(items, pit{"looks/l" + string(rune('a'+i)) + ".json", lookData(t, seed, i)})
	}
	m, src := testPack(id, 1, items...)
	for i := range m.Items {
		m.Items[i].Theme, m.Items[i].Topic = base, topics[i]
	}
	m.Themes = themes
	return m, src
}

// installFrom ставит набор с заданным origin.
func installFrom(t *testing.T, s *Store, origin string, m PackManifest, src fakeSrc) PackInfo {
	t.Helper()
	st, err := s.StagePack(m, src, origin, strings.Repeat("cd", 32))
	if err != nil || st.Ask != "" || len(st.Skipped) != 0 {
		t.Fatalf("StagePack %s: %v %+v", m.ID, err, st)
	}
	defer st.Close()
	info, err := s.InstallPack(st, nil)
	if err != nil {
		t.Fatalf("InstallPack %s: %v", m.ID, err)
	}
	return info
}

// themeLine — строки тем каталога вида kind: «id:число» в порядке показа.
func themeLine(t *testing.T, s *Store, kind string) []string {
	t.Helper()
	cat, err := s.Catalog(kind, nil, 300, 300)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, th := range cat.Themes {
		out = append(out, th.ID+":"+string(rune('0'+th.Count)))
	}
	return out
}

func themeOf(t *testing.T, s *Store, kind, key string) string {
	t.Helper()
	cat, err := s.Catalog(kind, nil, 300, 300)
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range cat.Items {
		if it.ID == key {
			return it.Theme
		}
	}
	t.Fatalf("плитки %s нет", key)
	return ""
}

func topicKey(t *testing.T, seed, i int) string {
	t.Helper()
	return looktpl.LookKey(lookData(t, seed, i))
}

func tickClock(t *testing.T) {
	t.Helper()
	clock := t0
	old := nowMs
	nowMs = func() int64 { clock += 10; return clock }
	t.Cleanup(func() { nowMs = old })
}

// Сц. 1–2: тема сразу под своей базой, числа верны; предмет без topic остаётся в базе.
func TestCatalogPackThemes(t *testing.T) {
	s, cfg := openTemp(t)
	tickClock(t)
	m, src := topicPack(t, "newyear-set", "holiday", 1, []PackTheme{thName("newyear", "Новый год", "holiday", 10)},
		"newyear", "newyear", "")
	installFrom(t, s, OriginNet, m, src)
	for when, st := range map[string]*Store{"сеанс": s, "после Open": Open(cfg)} {
		if got, want := themeLine(t, st, KindAlbum), []string{"all:3", "holiday:1", "newyear:2", "mine:0"}; !slices.Equal(got, want) {
			t.Errorf("%s: темы %v, ждали %v", when, got, want)
		}
		if th := themeOf(t, st, KindAlbum, topicKey(t, 1, 0)); th != "newyear" {
			t.Errorf("%s: тема предмета %q", when, th)
		}
		if th := themeOf(t, st, KindAlbum, topicKey(t, 1, 2)); th != "holiday" {
			t.Errorf("%s: предмет без topic в %q, ждали базу", when, th)
		}
	}
	// В виде без предметов темы набора её строки нет (пустая скрыта).
	if cat, _ := s.Catalog(KindBg, nil, 300, 300); slices.ContainsFunc(cat.Themes, func(th Theme) bool { return th.ID == "newyear" }) {
		t.Errorf("пустая тема в фонах: %+v", cat.Themes)
	}
	cat, _ := s.Catalog(KindAlbum, nil, 300, 300)
	if cat.Themes[len(cat.Themes)-2].Name != "Новый год" {
		t.Errorf("имя из набора: %+v", cat.Themes)
	}
}

// Предметы темы в двух видах сразу (фон и стикер, у них есть встроенные плитки): число темы набора и
// её место сразу под базой считаются в каждом виде.
func TestCatalogPackThemesKinds(t *testing.T) {
	s, _ := openTemp(t)
	bg, st := renderPNG(t, 41, 30), renderPNG(t, 17, 17)
	m, src := testPack("kinds-set", 1, pit{"bg/a.png", bg}, pit{"stickers/a.png", st})
	m.Themes = []PackTheme{thName("spring", "Весна", "wedding", 0)}
	for i := range m.Items {
		m.Items[i].Topic = "spring"
	}
	installFrom(t, s, OriginNet, m, src)
	for kind, key := range map[string]string{KindBg: libKey(bg), KindSticker: libKey(st)} {
		cat, _ := s.Catalog(kind, nil, 300, 300)
		var ids []string
		for _, th := range cat.Themes {
			ids = append(ids, th.ID)
			if th.ID == "spring" && (th.Count != 1 || th.Name != "Весна") {
				t.Errorf("%s: строка %+v", kind, th)
			}
		}
		i := slices.Index(ids, "spring")
		if i < 1 || ids[i-1] != "wedding" {
			t.Errorf("%s: «Весна» не сразу под «Свадьбой»: %v", kind, ids)
		}
		if th := themeOf(t, s, kind, key); th != "spring" {
			t.Errorf("%s: тема предмета %q", kind, th)
		}
	}
}

// Порядок: сразу под своей базой, внутри — по order, затем по времени установки и id; темы разных баз.
// Набор, поставленный позже, с меньшим order встаёт выше раннего («(at, order)» не пройдёт); при равном
// order раньше стоит поставленный раньше, хотя его id по алфавиту больше («(order, id)» не пройдёт).
func TestCatalogPackThemesOrder(t *testing.T) {
	s, _ := openTemp(t)
	tickClock(t)
	mA, sA := topicPack(t, "set-a", "holiday", 1, []PackTheme{thName("summer", "Лето", "holiday", 20), thName("birthday", "День рождения", "holiday", 10)},
		"summer", "birthday", "")
	installFrom(t, s, OriginNet, mA, sA)
	mB, sB := topicPack(t, "set-b", "kids", 2, []PackTheme{thName("baby", "Малыш", "kids", 0), thName("zoo", "Зоопарк", "holiday", 5)},
		"baby", "zoo", "")
	installFrom(t, s, OriginNet, mB, sB)
	mC, sC := topicPack(t, "set-c", "holiday", 3, []PackTheme{thName("autumn", "Осень", "holiday", 20)}, "autumn")
	installFrom(t, s, OriginNet, mC, sC)
	// kids: предмет mB без topic — в базе kids; holiday: предмет mA без topic — в базе holiday
	want := []string{"all:7", "kids:1", "baby:1", "holiday:1", "zoo:1", "birthday:1", "summer:1", "autumn:1", "mine:0"}
	if got := themeLine(t, s, KindAlbum); !slices.Equal(got, want) {
		t.Errorf("порядок:\n%v\nждали\n%v", got, want)
	}
}

// Предел объявлений на набор: StagePack получает манифест не только из разбора pack, и 17-я тема в
// реестр не попадает (правило decor держит свой предел, 170-29).
func TestStagePackThemesLimit(t *testing.T) {
	s, _ := openTemp(t)
	tickClock(t)
	var th []PackTheme
	for i := range maxThemeDecls + 1 {
		th = append(th, thName("t"+string(rune('a'+i)), "Тема "+string(rune('а'+i)), "holiday", i+1))
	}
	m, src := topicPack(t, "many-set", "holiday", 1, th, "ta")
	installFrom(t, s, OriginNet, m, src)
	if got := len(s.packOf("many-set").Themes); got != maxThemeDecls {
		t.Errorf("тем в реестре %d, ждали %d", got, maxThemeDecls)
	}
}

// Пустая база скрыта, но её темы стоят на своём месте — сразу после последней видимой встроенной
// темы перед ней (170-22).
func TestCatalogEmptyBase(t *testing.T) {
	s, _ := openTemp(t)
	tickClock(t)
	m, src := topicPack(t, "nyy", "holiday", 1, []PackTheme{thName("newyear", "Новый год", "holiday", 10)}, "newyear")
	installFrom(t, s, OriginNet, m, src)
	if got := themeLine(t, s, KindAlbum); !slices.Equal(got, []string{"all:1", "newyear:1", "mine:0"}) {
		t.Errorf("пустой «Праздник»: %v", got)
	}
	own := saveOwnLook(t, s, "grad", 1)
	if got := themeLine(t, s, KindAlbum); !slices.Equal(got, []string{"all:2", "grad:1", "newyear:1", "mine:0"}) {
		t.Errorf("после «Выпускного»: %v (своё %s)", got, own)
	}
}

// saveOwnLook — свой макет seed в тему theme (макет — единственный вид без встроенных плиток).
func saveOwnLook(t *testing.T, s *Store, theme string, seed int) string {
	t.Helper()
	it, err := s.SaveLook(theme, "Свой "+string(rune('а'+seed)), looksOwn(seed))
	if err != nil {
		t.Fatalf("SaveLook: %v", err)
	}
	return it.ID
}

// looksOwn — макет со своим именем титула: у каждого seed свой ключ.
func looksOwn(seed int) looktpl.AlbumTpl {
	l := lookOf("", "")
	l.Steps[0].Name = "Свой титул " + string(rune('а'+seed))
	return l
}

// Id тем уникальны в каталоге; ни одна не совпадает со служебным «get» (ключ {#each} в интерфейсе).
func TestCatalogThemeIDsUnique(t *testing.T) {
	s, _ := openTemp(t)
	tickClock(t)
	m, src := topicPack(t, "u-set", "holiday", 1, []PackTheme{thName("newyear", "Новый год", "holiday", 10), thName("get", "Скачать", "holiday", 5),
		thName("mine", "Моё", "holiday", 5), thName("new", "Новинка", "holiday", 5), thName("recent", "Прежнее", "holiday", 5)},
		"newyear", "get", "mine", "new", "recent")
	installFrom(t, s, OriginNet, m, src)
	cat, _ := s.Catalog(KindAlbum, []string{topicKey(t, 1, 0)}, 300, 300)
	seen := map[string]bool{}
	for _, th := range cat.Themes {
		if seen[th.ID] {
			t.Errorf("id %q повторяется: %+v", th.ID, cat.Themes)
		}
		seen[th.ID] = true
		if th.ID == "get" {
			t.Error("служебный get принят темой")
		}
	}
	if !seen["newyear"] || !seen["recent"] || seen["new"] {
		t.Errorf("темы: %+v", cat.Themes) // recent — выборка «Недавние»; new без предметов нет
	}
	// Служебные id объявить нельзя: «recent» — выборка недавних, а не тема набора; предмет остался в базе.
	if th := themeOf(t, s, KindAlbum, topicKey(t, 1, 1)); th != "holiday" {
		t.Errorf("предмет с topic get: %q", th)
	}
}

// Сц. 3: встроенный и служебный id, имя-двойник (встроенной, в манифесте и между наборами) отвергаются.
func TestPackThemeCollision(t *testing.T) {
	s, _ := openTemp(t)
	tickClock(t)
	// Реестр набора правят руками, а загрузка зовёт правило заново — но и от службы наборов правило
	// проверяется ещё раз: объявления из манифеста с занятыми id и именами в реестр не попадают.
	m, src := topicPack(t, "bad-set", "holiday", 1, []PackTheme{
		thName("kids", "Малыши", "holiday", 5), thName("mine", "Моё", "holiday", 5), thName("constructor", "Конструктор", "holiday", 5),
		thName("fest", "праздник", "holiday", 5), thName("ok", "Хороший", "holiday", 5), thName("okk", "хороший", "holiday", 5)},
		"kids", "mine", "constructor", "fest", "ok", "okk")
	installFrom(t, s, OriginNet, m, src)
	got := themeLine(t, s, KindAlbum)
	// kids — встроенная: topic kids кладёт предмет во встроенную «Дети»; остальные отвергнуты, кроме ok.
	want := []string{"all:6", "kids:1", "holiday:4", "ok:1", "mine:0"}
	if !slices.Equal(got, want) {
		t.Errorf("темы %v, ждали %v", got, want)
	}
	// Между наборами: другой id с тем же именем — отвергнут, предмет в базе; тот же id — общая тема.
	m2, src2 := topicPack(t, "dbl-set", "kids", 2, []PackTheme{thName("copy", "ХОРОШИЙ", "kids", 5), thName("ok", "Другое имя", "kids", 1)}, "copy", "ok")
	installFrom(t, s, OriginNet, m2, src2)
	got = themeLine(t, s, KindAlbum)
	want = []string{"all:8", "kids:2", "holiday:4", "ok:2", "mine:0"}
	if !slices.Equal(got, want) {
		t.Errorf("после второго набора %v, ждали %v", got, want)
	}
	cat, _ := s.Catalog(KindAlbum, nil, 300, 300)
	for _, th := range cat.Themes {
		if th.ID == "ok" && th.Name != "Хороший" {
			t.Errorf("имя общей темы %q: ждали у первого набора", th.Name)
		}
	}
}

// Сц. 3: два набора объявляют один id — строка одна; удалили первого — имя и место у второго.
func TestPackThemeHandover(t *testing.T) {
	s, cfg := openTemp(t)
	tickClock(t)
	mA, sA := topicPack(t, "set-a", "holiday", 1, []PackTheme{thName("newyear", "Новый год", "holiday", 10)}, "newyear")
	installFrom(t, s, OriginNet, mA, sA)
	mB, sB := topicPack(t, "set-b", "kids", 2, []PackTheme{thName("newyear", "Зима", "kids", 5)}, "newyear")
	installFrom(t, s, OriginNet, mB, sB)
	for when, st := range map[string]*Store{"сеанс": s, "после Open": Open(cfg)} {
		if got := themeLine(t, st, KindAlbum); !slices.Equal(got, []string{"all:2", "newyear:2", "mine:0"}) {
			t.Errorf("%s: темы %v", when, got)
		}
		cat, _ := st.Catalog(KindAlbum, nil, 300, 300)
		if cat.Themes[1].Name != "Новый год" {
			t.Errorf("%s: имя %q, ждали первого набора", when, cat.Themes[1].Name)
		}
	}
	if _, err := s.RemovePack("set-a", nil); err != nil {
		t.Fatal(err)
	}
	for when, st := range map[string]*Store{"сеанс": s, "после Open": Open(cfg)} {
		cat, _ := st.Catalog(KindAlbum, nil, 300, 300)
		var got []string
		for _, th := range cat.Themes {
			got = append(got, th.ID+":"+th.Name)
		}
		// база второго — kids: пустая скрыта, тема на месте и с его именем
		if !slices.Equal(got, []string{"all:Все", "newyear:Зима", "mine:Мои"}) {
			t.Errorf("%s: после удаления первого %v", when, got)
		}
	}
	// Сц. 4: удалили оба — строки нет.
	if _, err := s.RemovePack("set-b", nil); err != nil {
		t.Fatal(err)
	}
	if got := themeLine(t, s, KindAlbum); !slices.Equal(got, []string{"all:0", "mine:0"}) {
		t.Errorf("наборов нет: %v", got)
	}
}

// Сц. 6: объявлениям верят только наборам с origin net и demo — по реестру, а не по id; набор из файла
// (в том числе с id примера) кладёт предметы в базу.
func TestPackThemeTrust(t *testing.T) {
	s, cfg := openTemp(t)
	tickClock(t)
	s.SetDemo("example-set", nil)
	th := []PackTheme{thName("newyear", "Новый год", "holiday", 10)}
	m, src := topicPack(t, "file-set", "holiday", 1, th, "newyear")
	installFrom(t, s, OriginFile, m, src)
	m, src = topicPack(t, "example-set", "holiday", 2, th, "newyear")
	installFrom(t, s, OriginFile, m, src) // из файла, с id примера
	for when, st := range map[string]*Store{"сеанс": s, "после Open": Open(cfg)} {
		if got := themeLine(t, st, KindAlbum); !slices.Equal(got, []string{"all:2", "holiday:2", "mine:0"}) {
			t.Errorf("%s: набор из файла принёс тему: %v", when, got)
		}
	}
	// Пример (demo) и сеть — верим.
	m, src = topicPack(t, "demo-set", "holiday", 3, th, "newyear")
	installFrom(t, s, OriginDemo, m, src)
	if got := themeLine(t, s, KindAlbum); !slices.Equal(got, []string{"all:3", "newyear:3", "mine:0"}) {
		t.Errorf("demo: %v", got) // предметы наборов из файла с topic newyear — тоже в ней
	}
	// Предметы наборов из файла с topic этой темы переехали в неё: тема одна на программу, а предмет
	// решает topic (имя — из доверенного набора).
	if got := themeLine(t, s, KindAlbum); !slices.Equal(got, []string{"all:3", "newyear:3", "mine:0"}) {
		t.Errorf("topic из файла при доверенном объявлении: %v", got)
	}
	// topic без объявления в том же манифесте — предмет в базе, как у pack.ParseManifest.
	m, src = topicPack(t, "file-two", "holiday", 4, nil, "newyear")
	installFrom(t, s, OriginFile, m, src)
	if got := themeLine(t, s, KindAlbum); !slices.Equal(got, []string{"all:4", "holiday:1", "newyear:3", "mine:0"}) {
		t.Errorf("topic без объявления: %v", got)
	}
}

// Тем наборов не больше 64: принимаются первые по порядку установки; остальное — одна строка в log.txt
// на набор.
func TestPackThemeLimits(t *testing.T) {
	s, cfg := openTemp(t)
	tickClock(t)
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	idx := 0
	var last string
	for p := range 5 {
		var themes []PackTheme
		for range 16 {
			id := string(rune('a'+idx/26)) + string(rune('a'+idx%26))
			themes = append(themes, thName(id, "Тема "+string(rune('а'+idx/10))+string(rune('а'+idx%10)), "basic", 50))
			last = id
			idx++
		}
		m, src := topicPack(t, "pk-"+string(rune('a'+p)), "basic", p+1, themes, themes[0].ID)
		installFrom(t, s, OriginNet, m, src)
	}
	for when, st := range map[string]*Store{"сеанс": s, "после Open": Open(cfg)} {
		st.mu.RLock()
		kids := len(st.tabIdx)
		_, hasFirst := st.tabIdx["aa"]
		_, hasLast := st.tabIdx[last]
		st.mu.RUnlock()
		if kids != maxPackThemes || !hasFirst || hasLast {
			t.Errorf("%s: принято %d тем, первая %v, последняя %v", when, kids, hasFirst, hasLast)
		}
	}
	out := buf.String()
	if n := strings.Count(out, "темы пропущены"); n != 2 { // сеанс и Open: по одной строке на набор pk-e
		t.Errorf("строк в журнале %d:\n%s", n, out)
	}
	if !strings.Contains(out, `набор "pk-e": темы пропущены: 16 (предел, предел`) {
		t.Errorf("строка о пределе:\n%s", out)
	}
}

// Сц. 4–5: свои кладутся в тему набора; набор удалили — они в «Моих», library.json прежний, в том числе
// после записи другой правкой; набор вернули — своё снова в теме.
func TestOwnInPackTheme(t *testing.T) {
	s, cfg := openTemp(t)
	tickClock(t)
	m, src := topicPack(t, "nyy", "holiday", 1, []PackTheme{thName("newyear", "Новый год", "holiday", 10)}, "newyear")
	installFrom(t, s, OriginNet, m, src)
	a := saveOwnLook(t, s, "newyear", 1)
	b := saveOwnLook(t, s, "newyear", 2)
	c := saveOwnLook(t, s, "newyearx", 3) // темы нет — «Мои»
	if th := themeOf(t, s, KindAlbum, a); th != "newyear" {
		t.Fatalf("своё в теме набора: %q", th)
	}
	if th := themeOf(t, s, KindAlbum, c); th != ThemeMine {
		t.Fatalf("тема без набора: %q", th)
	}
	if _, err := s.RemovePack("nyy", nil); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{a, b} {
		if th := themeOf(t, s, KindAlbum, id); th != ThemeMine {
			t.Errorf("после удаления набора %s: %q", id, th)
		}
	}
	if err := s.Rename(KindAlbum, b, "Другое имя"); err != nil { // запись library.json другой правкой
		t.Fatal(err)
	}
	for when, st := range map[string]*Store{"сеанс": s, "после Open": Open(cfg)} {
		st.mu.RLock()
		ea, eb := st.entryOf(KindAlbum, a), st.entryOf(KindAlbum, b)
		ta, tb := ea.Theme, eb.Theme
		st.mu.RUnlock()
		if ta != "newyear" || tb != "newyear" {
			t.Errorf("%s: в library.json темы %q и %q, ждали прежнюю", when, ta, tb)
		}
	}
	// Вернули набор — своё снова в теме.
	s2 := Open(cfg)
	m, src = topicPack(t, "nyy", "holiday", 1, []PackTheme{thName("newyear", "Новый год", "holiday", 10)}, "newyear")
	installFrom(t, s2, OriginNet, m, src)
	for _, id := range []string{a, b} {
		if th := themeOf(t, s2, KindAlbum, id); th != "newyear" {
			t.Errorf("после возврата набора %s: %q", id, th)
		}
	}
	if th := themeOf(t, s2, KindAlbum, c); th != ThemeMine {
		t.Errorf("своё без темы: %q", th)
	}
}

// Сохранение разворота и макета в тему набора: кладутся в неё, пока она есть; иначе «Мои».
func TestSaveInPackTheme(t *testing.T) {
	s, _ := openTemp(t)
	tickClock(t)
	if s.ownTheme("newyear") {
		t.Fatal("темы нет, а своё в неё принимается")
	}
	m, src := topicPack(t, "nyy", "holiday", 1, []PackTheme{thName("newyear", "Новый год", "holiday", 10)}, "newyear")
	installFrom(t, s, OriginNet, m, src)
	if !s.ownTheme("newyear") || s.ownTheme(ThemeLegacy) || s.ownTheme(ThemeBrand) || s.ownTheme("recent") || s.ownTheme("new") || !s.ownTheme(ThemeMine) {
		t.Error("ownTheme: тема набора да; legacy, brand и выборки нет")
	}
	l := lookOf("", "")
	if it, err := s.SaveLook("newyear", "Новогодний", l); err != nil || it.Theme != "newyear" {
		t.Errorf("макет в теме набора: %+v %v", it, err)
	}
	spread := func(x float64) looktpl.SpreadTpl {
		return looktpl.SpreadTpl{V: 1, Pages: 1, Places: []model.Box{{X: x, Y: .1, W: .5, H: .5}}, Page: []looktpl.TplBg{{Color: "#ffffff"}}}
	}
	if it, err := s.SaveSpreadTpl("newyear", "Разворот", spread(.1)); err != nil || it.Theme != "newyear" {
		t.Errorf("разворот в теме набора: %+v %v", it, err)
	}
	if it, err := s.SaveSpreadTpl("nosuch", "Разворот 2", spread(.2)); err != nil || it.Theme != ThemeMine {
		t.Errorf("разворот в тему без набора: %+v %v", it, err)
	}
}
