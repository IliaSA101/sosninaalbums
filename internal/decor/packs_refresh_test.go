package decor

import (
	"bytes"
	"errors"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// Перекачка тем и квота обновления (170-9, 170-19, 170-27): StagePack/InstallPack той же версии.

// as039 — набор id глазами 0.39: реестр без tv (в памяти и на диске), темы в нём не хранились.
func as039(t *testing.T, s *Store, id string) {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	r := s.packOf(id)
	if r == nil {
		t.Fatalf("набора %s нет", id)
	}
	r.TV, r.Themes = 0, nil
	if err := s.writePackReg(r); err != nil {
		t.Fatal(err)
	}
}

// stageAs — StagePack с произвольной суммой zip.
func stageAs(t *testing.T, s *Store, origin, zip string, m PackManifest, src fakeSrc) (*Staged, error) {
	t.Helper()
	st, err := s.StagePack(m, src, origin, zip)
	if st != nil {
		t.Cleanup(st.Close)
	}
	return st, err
}

func tvOf(t *testing.T, s *Store, id string) int {
	t.Helper()
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.packOf(id).TV
}

// Та же версия: из сети поверх «сетевого» реестра без tv — AskUpdate (и при нуле принятых тем);
// из файла, пример, стоящий из файла, tv = 1 — AskSame; старее — AskOlder.
func TestStageSameVersionThemes(t *testing.T) {
	th := []PackTheme{thName("newyear", "Новый год", "holiday", 10)}
	cases := []struct {
		name        string
		oldOrigin   string
		tv0         bool
		origin      string
		version     int
		themes      []PackTheme
		want        string
		wantRefresh bool
	}{
		{"сеть, tv 0, тема принята", OriginNet, true, OriginNet, 1, th, AskUpdate, true},
		{"сеть, tv 0, нет объявлений", OriginNet, true, OriginNet, 1, nil, AskUpdate, true},
		{"сеть, tv 0, объявление отвергнуто", OriginNet, true, OriginNet, 1, []PackTheme{thName("recent", "Служебная", "holiday", 1)}, AskUpdate, true},
		{"из файла поверх сети", OriginNet, true, OriginFile, 1, th, AskSame, false},
		{"пример поверх сети", OriginNet, true, OriginDemo, 1, th, AskSame, false},
		{"сеть поверх стоящего из файла", OriginFile, true, OriginNet, 1, th, AskSame, false},
		{"сеть поверх примера", OriginDemo, true, OriginNet, 1, th, AskSame, false},
		{"tv 1", OriginNet, false, OriginNet, 1, th, AskSame, false},
		{"старее, tv 0", OriginNet, true, OriginNet, 0, th, AskOlder, false},
		{"новее — обычное обновление", OriginNet, false, OriginNet, 2, th, AskUpdate, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s, _ := openTemp(t)
			tickClock(t)
			m1, src1 := topicPack(t, "ny-set", "holiday", 1, nil, "", "")
			m1.Version = 1
			if c.version == 0 { // «старее»: стоит версия 2
				m1.Version = 2
			}
			installFrom(t, s, c.oldOrigin, m1, src1)
			if c.tv0 {
				as039(t, s, "ny-set")
			}
			m, src := topicPack(t, "ny-set", "holiday", 1, c.themes, "newyear", "newyear")
			m.Version = max(c.version, 1)
			st, err := stageAs(t, s, c.origin, strings.Repeat("cd", 32), m, src)
			if err != nil || st.Ask != c.want || st.refresh != c.wantRefresh {
				t.Fatalf("Ask %q refresh %v, ждали %q %v (%v)", st.Ask, st.refresh, c.want, c.wantRefresh, err)
			}
			if got := len(st.items) > 0; got != (c.want == AskUpdate) {
				t.Errorf("распаковано %v при ответе %q", got, c.want)
			}
		})
	}
}

// Перекачка той же версии: tv = 1 при любом числе принятых тем, Installed и Base прежние, уборка
// ничего не удаляет, предметы встают в тему; без пометки та же версия — errPackSame.
func TestInstallPackRefresh(t *testing.T) {
	s, cfg := openTemp(t)
	tickClock(t)
	th := []PackTheme{thName("newyear", "Новый год", "holiday", 10)}
	m1, src1 := topicPack(t, "ny-set", "holiday", 1, nil, "", "")
	installFrom(t, s, OriginNet, m1, src1)
	other, osrc := topicPack(t, "other-set", "holiday", 2, nil, "")
	installFrom(t, s, OriginNet, other, osrc)
	s.mu.Lock()
	s.packOf("ny-set").Base = 12345
	s.mu.Unlock()
	as039(t, s, "ny-set")
	if got := themeLine(t, s, KindAlbum); !slices.Equal(got, []string{"all:3", "holiday:3", "mine:0"}) {
		t.Fatalf("до перекачки: %v", got)
	}
	before := s.packOf("ny-set").Installed
	m2, src2 := topicPack(t, "ny-set", "holiday", 1, th, "newyear", "") // те же предметы, один — в теме
	st, err := stageAs(t, s, OriginNet, strings.Repeat("cd", 32), m2, src2)
	if err != nil || st.Ask != AskUpdate {
		t.Fatalf("StagePack: %v %q", err, st.Ask)
	}
	// Без пометки та же версия не ставится.
	plain := *st
	plain.refresh = false
	if _, err := s.InstallPack(&plain, nil); !errors.Is(err, errPackSame) {
		t.Fatalf("без пометки: %v", err)
	}
	if tvOf(t, s, "ny-set") != 0 {
		t.Fatal("отказ записал tv")
	}
	files := regularFiles(t, cfg)
	info, err := s.InstallPack(st, nil)
	if err != nil || info.Version != 1 || !info.ThemesRead {
		t.Fatalf("InstallPack: %v %+v", err, info)
	}
	if r := s.packOf("ny-set"); r.Installed != before || r.Base != 12345 || r.TV != 1 {
		t.Errorf("Installed %d (было %d), Base %d, TV %d", r.Installed, before, r.Base, r.TV)
	}
	if now := regularFiles(t, cfg); !slices.Equal(now, files) {
		t.Errorf("перекачка изменила файлы:\n%v\n%v", files, now)
	}
	for when, store := range map[string]*Store{"сеанс": s, "после Open": Open(cfg)} {
		if got := themeLine(t, store, KindAlbum); !slices.Equal(got, []string{"all:3", "holiday:2", "newyear:1", "mine:0"}) {
			t.Errorf("%s: %v", when, got)
		}
		if p := store.Packs(); !p[0].ThemesRead && !p[1].ThemesRead {
			t.Errorf("%s: tv не прочитан", when)
		}
	}
	// Дальше тишина: тот же набор из сети — AskSame.
	if again, err := stageAs(t, s, OriginNet, strings.Repeat("cd", 32), m2, src2); err != nil || again.Ask != AskSame {
		t.Errorf("повтор: %v %q", err, again.Ask)
	}
}

// Ни одно объявление не принято — tv = 1 всё равно: следующий ход набор не берёт (170-19).
func TestInstallPackRefreshNoThemes(t *testing.T) {
	s, cfg := openTemp(t)
	tickClock(t)
	m1, src1 := topicPack(t, "ny-set", "holiday", 1, nil, "")
	installFrom(t, s, OriginNet, m1, src1)
	as039(t, s, "ny-set")
	bad := []PackTheme{thName("recent", "Служебная", "holiday", 1)}
	m2, src2 := topicPack(t, "ny-set", "holiday", 1, bad, "recent")
	st, err := stageAs(t, s, OriginNet, strings.Repeat("cd", 32), m2, src2)
	if err != nil || st.Ask != AskUpdate {
		t.Fatalf("StagePack: %v %q", err, st.Ask)
	}
	if _, err := s.InstallPack(st, nil); err != nil || tvOf(t, s, "ny-set") != 1 {
		t.Fatalf("InstallPack: %v, tv %d", err, tvOf(t, s, "ny-set"))
	}
	if again, err := stageAs(t, Open(cfg), OriginNet, strings.Repeat("cd", 32), m2, src2); err != nil || again.Ask != AskSame {
		t.Errorf("после перезапуска: %v %q", err, again.Ask)
	}
}

// regularFiles — файлы библиотеки (без реестров наборов): относительные пути, по порядку.
func regularFiles(t *testing.T, cfg string) []string {
	t.Helper()
	var out []string
	root := filepath.Join(cfg, RootName)
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && filepath.Base(filepath.Dir(p)) != packsDir { // реестры наборов — не файлы предметов
			rel, _ := filepath.Rel(root, p)
			out = append(out, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(out)
	return out
}

// fillTo добивает библиотеку набором-заполнителем до used = MaxLibraryLen - slack.
func fillTo(t *testing.T, s *Store, slack int64) {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	room := MaxLibraryLen - slack - s.libBytes()
	if room < 0 {
		t.Fatalf("библиотека уже больше нужного на %d", -room)
	}
	s.packs = append(s.packs, &packReg{V: packRegV, ID: "filler", Version: 1, Title: "filler", Items: fillerEntries(room)})
}

// У предела 2 ГБ перекачка и обновление с новыми ключами проходят StagePack (из used вычитается
// стоящий набор), новый набор — ErrLibFull.
func TestStageQuotaUpdate(t *testing.T) {
	s, _ := openTemp(t)
	tickClock(t)
	a, b := renderPNG(t, 61, 40), renderPNG(t, 62, 40)
	m1, s1 := testPack("ivory", 1, pit{"bg/a.png", a}, pit{"bg/b.png", b})
	installFrom(t, s, OriginNet, m1, s1)
	as039(t, s, "ivory")
	fillTo(t, s, 0) // ровно предел
	// Перекачка той же версии.
	if st, err := stageAs(t, s, OriginNet, strings.Repeat("cd", 32), m1, s1); err != nil || st.Ask != AskUpdate {
		t.Errorf("перекачка у предела: %v %q", err, st.Ask)
	}
	// Обновление с новыми ключами того же веса.
	c, d := renderPNG(t, 63, 40), renderPNG(t, 64, 40)
	if len(c)+len(d) > len(a)+len(b) {
		c, d = renderPNG(t, 63, 30), renderPNG(t, 64, 30)
	}
	m2, s2 := testPack("ivory", 2, pit{"bg/c.png", c}, pit{"bg/d.png", d})
	if st, err := stageAs(t, s, OriginNet, strings.Repeat("cd", 32), m2, s2); err != nil || st.Ask != AskUpdate {
		t.Errorf("обновление у предела: %v %q", err, st.Ask)
	}
	// Обновление, которое больше стоящего набора на байт сверх предела, — отказ до распаковки.
	big := renderPNG(t, 600, 400)
	m3, s3 := testPack("ivory", 3, pit{"bg/big.png", big})
	if len(big) <= len(a)+len(b) {
		t.Fatal("тестовая картинка мала")
	}
	if st, err := s.StagePack(m3, s3, OriginNet, ""); !errors.Is(err, ErrLibFull) || st != nil {
		t.Errorf("обновление сверх предела: %v", err)
	}
	// Новый набор у предела — ErrLibFull, как и был.
	m4, s4 := testPack("rose", 1, pit{"bg/c.png", c})
	if _, err := s.StagePack(m4, s4, OriginNet, ""); !errors.Is(err, ErrLibFull) {
		t.Errorf("новый набор у предела: %v", err)
	}
}

// Обновление с новыми ключами у предела проходит InstallPack: вычитаются ключи, которых нет в
// новой версии и которых не держит никто. Ключ, который держит свой, другой набор или открытый
// альбом, не вычитается (иначе библиотека перешла бы предел).
func TestInstallPackQuotaUpdate(t *testing.T) {
	a, b, k := renderPNG(t, 71, 30), renderPNG(t, 72, 30), renderPNG(t, 73, 30)
	c := renderPNG(t, 300, 200)
	if len(c) <= len(a)+len(b) {
		t.Fatalf("тестовая картинка мала: %d <= %d", len(c), len(a)+len(b))
	}
	// slack подобран так, что обновление проходит только при вычете a, b и k вместе: без k не хватает байта.
	slack := int64(len(c) - len(a) - len(b) - 1)
	cases := []struct {
		name   string
		hold   func(t *testing.T, s *Store)
		keep   func(string) bool
		wantOK bool
	}{
		{"никто не держит", func(*testing.T, *Store) {}, nil, true},
		{"открытый альбом", func(*testing.T, *Store) {}, func(key string) bool { return key == libKey(k) }, false},
		{"другой набор", func(t *testing.T, s *Store) {
			m, src := testPack("other", 1, pit{"bg/k.png", k})
			installFrom(t, s, OriginFile, m, src)
		}, nil, false},
		{"своё", func(t *testing.T, s *Store) {
			s.mu.Lock()
			s.entries = append(s.entries, &entry{Key: libKey(k), Kind: KindBg, Bytes: int64(len(k))})
			s.mu.Unlock()
		}, nil, false},
	}
	for _, cs := range cases {
		t.Run(cs.name, func(t *testing.T) {
			s, cfg := openTemp(t)
			tickClock(t)
			m1, s1 := testPack("ivory", 1, pit{"bg/a.png", a}, pit{"bg/b.png", b}, pit{"bg/k.png", k})
			installFrom(t, s, OriginNet, m1, s1)
			cs.hold(t, s)
			fillTo(t, s, slack)
			m2, s2 := testPack("ivory", 2, pit{"bg/c.png", c})
			st, err := s.StagePack(m2, s2, OriginNet, "")
			if err != nil || st.Ask != AskUpdate {
				t.Fatalf("StagePack: %v", err)
			}
			defer st.Close()
			info, err := s.InstallPack(st, cs.keep)
			aFile := filepath.Join(cfg, RootName, "Фоны", strings.TrimPrefix(libKey(a), "my:")+".png")
			if cs.wantOK {
				if err != nil || info.Version != 2 || s.libBytes() > MaxLibraryLen || regularFile(aFile) {
					t.Fatalf("обновление у предела: %v %+v, байт %d, старый ключ остался %v", err, info, s.libBytes(), regularFile(aFile))
				}
				return
			}
			if !errors.Is(err, ErrLibFull) || s.packOf("ivory").Version != 1 || !regularFile(aFile) {
				t.Errorf("ждали ErrLibFull без следов: %v, версия %d, файл есть %v", err, s.packOf("ivory").Version, regularFile(aFile))
			}
		})
	}
}

// Та же версия с другим zip (издатель перепаковал): содержимое заменено, старые ключи убраны,
// одна строка в log.txt с id через %q; тот же zip — без строки.
func TestInstallPackRefreshOtherZip(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	s, cfg := openTemp(t)
	tickClock(t)
	a, b, c := renderPNG(t, 41, 30), renderPNG(t, 42, 30), renderPNG(t, 43, 30)
	m1, s1 := testPack("ivory", 1, pit{"bg/a.png", a}, pit{"bg/b.png", b})
	installFrom(t, s, OriginNet, m1, s1) // zip «cdcd…»
	as039(t, s, "ivory")
	// Тот же zip: тишина.
	st, err := stageAs(t, s, OriginNet, strings.Repeat("cd", 32), m1, s1)
	if err != nil || st.Ask != AskUpdate {
		t.Fatalf("Stage: %v %q", err, st.Ask)
	}
	if _, err := s.InstallPack(st, nil); err != nil || buf.Len() != 0 {
		t.Fatalf("тот же zip: %v, журнал %q", err, buf.String())
	}
	as039(t, s, "ivory")
	// Другой zip и другое содержимое.
	m2, s2 := testPack("ivory", 1, pit{"bg/a.png", a}, pit{"bg/c.png", c})
	st, err = stageAs(t, s, OriginNet, strings.Repeat("ef", 32), m2, s2)
	if err != nil || st.Ask != AskUpdate {
		t.Fatalf("Stage: %v %q", err, st.Ask)
	}
	if info, err := s.InstallPack(st, nil); err != nil || info.Bg != 2 {
		t.Fatalf("InstallPack: %v %+v", err, info)
	}
	root := filepath.Join(cfg, RootName, "Фоны")
	for key, want := range map[string]bool{libKey(a): true, libKey(b): false, libKey(c): true} {
		if got := regularFile(filepath.Join(root, strings.TrimPrefix(key, "my:")+".png")); got != want {
			t.Errorf("файл %s есть=%v, ждали %v", key, got, want)
		}
	}
	if out := buf.String(); strings.Count(out, "\n") != 1 || !strings.Contains(out, `набор "ivory": та же версия 1, другой архив`) {
		t.Errorf("журнал: %q", out)
	}
}
