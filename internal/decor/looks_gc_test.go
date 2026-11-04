package decor

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Ссылки макетов альбома и сборка мусора (аудит 131 G3): экранированный ключ, ключи полей новее
// программы, keep, выживший макет, junction.

// escapeKey — ключ my:<hex> в байтах файла записан JSON-экранированием двоеточия (обратная косая и
// u003a): разбор его раскодирует, сырой поиск — нет.
func escapeKey(data []byte, key string) []byte {
	esc := "my" + string(rune(92)) + "u003a" + strings.TrimPrefix(key, libPrefix)
	return bytes.ReplaceAll(data, []byte(key), []byte(esc))
}

// stageWhys — причины пропусков при постановке набора (набор не ставится); в наборе должно быть что
// ставить, иначе StagePack — ошибка.
func stageWhys(t *testing.T, s *Store, id string, items ...pit) []string {
	t.Helper()
	m, src := testPack(id, 1, items...)
	st, err := s.StagePack(m, src, OriginFile, strings.Repeat("9a", 32))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	var whys []string
	for _, sk := range st.Skipped {
		whys = append(whys, sk.Name+": "+sk.Why)
	}
	slices.Sort(whys)
	return whys
}

// ownLookFile — макет data как свой (файл Макеты/<ключ>.json и запись library.json, как его оставила бы
// другая версия программы); Store открывается заново.
func ownLookFile(t *testing.T, cfg string, data []byte) (*Store, string) {
	t.Helper()
	key := model.LookKey(data)
	p := filepath.Join(cfg, RootName, kindDirs[KindAlbum], strings.TrimPrefix(key, libPrefix)+".json")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	reg := `{"version":1,"items":[{"key":"` + key + `","kind":"album","theme":"mine","name":"М"}]}`
	if err := os.WriteFile(filepath.Join(cfg, RootName, regFile), []byte(reg), 0o644); err != nil {
		t.Fatal(err)
	}
	return Open(cfg), key
}

// А1: ключ картинки, записанный экранированием. Набор такой макет не ставит (сборка мусора 0.27 не
// увидела бы ссылку); уже лежащий макет держит картинку разбором — удаление донора её не стирает.
func TestLookEscapedKey(t *testing.T) {
	s, cfg := openTemp(t)
	bg := renderPNG(t, 43, 30)
	put(t, s, "donor", pit{"bg/d.png", bg})
	key := libKey(bg)
	data := escapeKey(lookBytes(t, lookOf(key, ""), ""), key)
	if len(rawLibKeys(data)) != 0 {
		t.Fatalf("сырой поиск видит ключ: %v", rawLibKeys(data))
	}
	if l, err := model.ParseLook(data); err != nil || len(lookLibs(l)) == 0 {
		t.Fatalf("разбор не видит ключ: %v", err)
	}
	want := []string{"looks/esc.json: макет альбома не читается"}
	if whys := stageWhys(t, s, "donor-layouts", pit{"bg/x.png", renderPNG(t, 50, 30)}, pit{"looks/esc.json", data}); !slices.Equal(whys, want) {
		t.Errorf("набор с экранированным ключом: %q", whys)
	}
	s2, look := ownLookFile(t, cfg, data)
	if _, err := s2.Look(look); err != nil {
		t.Fatalf("макет не встал: %v", err)
	}
	if plan, err := s2.PlanRemovePack("donor", nil); err != nil || plan.Files != 0 || plan.Kept != 1 {
		t.Errorf("план удаления донора: %v %+v", err, plan)
	}
	if res, err := s2.RemovePack("donor", nil); err != nil || res.Files != 0 {
		t.Fatalf("удаление донора: %v %+v", err, res)
	}
	if _, ok := Open(cfg).Find(key); !ok {
		t.Error("фон макета удалён сборкой мусора")
	}
}

// Макет в индексе не читается (больше предела разбора, но в пределе сырого) — его ссылок не знаем,
// сборка мусора ничего не удаляет.
func TestLookUnreadKeepsAll(t *testing.T) {
	s, cfg := openTemp(t)
	bg := renderPNG(t, 45, 30)
	put(t, s, "donor", pit{"bg/d.png", bg})
	data := lookBytes(t, lookOf("", ""), `"pad":"`+strings.Repeat("x", model.MaxLookFile)+`"`)
	s2, _ := ownLookFile(t, cfg, data)
	if res, err := s2.RemovePack("donor", nil); err != nil || res.Files != 0 || res.Kept != 1 {
		t.Fatalf("удаление донора: %v %+v", err, res)
	}
	if _, ok := Open(cfg).Find(libKey(bg)); !ok {
		t.Error("картинка удалена при непрочитанном макете")
	}
}

// А3: ключ в поле новее программы проверяется, как ссылка разбора: чужой — пропуск, свой набора или
// стоящий в библиотеке (любого вида) — ставится. Р1: шаг с шаблоном новее — «из новой версии».
func TestLookRawOnlyKeys(t *testing.T) {
	s, _ := openTemp(t)
	donor := renderPNG(t, 46, 30)
	own := renderPNG(t, 30, 46)
	put(t, s, "donor", pit{"bg/d.png", donor})
	extra := func(key string) []byte { return lookBytes(t, lookOf("", ""), `"auto":{"lib":"`+key+`"}`) }
	newer := lookBytes(t, lookOf("", ""), "")
	newer = bytes.Replace(newer, []byte(`"tpl":{"v":1`), []byte(`"tpl":{"v":2`), 1)
	whys := stageWhys(t, s, "set-layouts",
		pit{"stickers/own.png", own},
		pit{"looks/alien.json", extra("my:0123456789abcdef")},
		pit{"looks/own.json", extra(libKey(own))},
		pit{"looks/donor.json", extra(libKey(donor))},
		pit{"looks/newer-step.json", newer},
	)
	want := []string{
		"looks/alien.json: макет ссылается на картинку не из набора",
		"looks/newer-step.json: макет из новой версии программы",
	}
	if !slices.Equal(whys, want) {
		t.Errorf("пропуски:\n%s", strings.Join(whys, "\n"))
	}
}

// Р4: keep держит макет — и его ссылки: макет и картинка набора остаются (план — так же).
func TestRemovePackLookKeep(t *testing.T) {
	s, cfg := openTemp(t)
	bg := renderPNG(t, 47, 30)
	data := lookBytes(t, lookOf(libKey(bg), ""), "")
	put(t, s, "solo-layouts", pit{"bg/s.png", bg}, pit{"looks/s.json", data})
	look := model.LookKey(data)
	keep := func(k string) bool { return k == look }
	plan, err := s.PlanRemovePack("solo-layouts", keep)
	if err != nil || plan.Files != 0 || plan.Kept != 2 {
		t.Fatalf("план: %v %+v", err, plan)
	}
	if res, err := s.RemovePack("solo-layouts", keep); err != nil || res != plan {
		t.Fatalf("удаление: %v %+v, план %+v", err, res, plan)
	}
	s2 := Open(cfg)
	if _, ok := s2.Find(libKey(bg)); !ok {
		t.Error("картинка макета, оставленного keep, удалена")
	}
	if _, ok := s2.files[KindAlbum][look]; !ok {
		t.Error("макет, оставленный keep, удалён")
	}
}

// А4: макет держат только свои, наборы и keep; ссылка на его ключ из записи незнакомого вида его не
// держит (иначе он остался бы без своих картинок). Макет из drop не удалился — картинки остаются.
func TestRemovePackLookRefsAndSurvivor(t *testing.T) {
	t.Run("ключ макета в незнакомой записи", func(t *testing.T) {
		_, cfg := openTemp(t)
		bg := renderPNG(t, 48, 30)
		data := lookBytes(t, lookOf(libKey(bg), ""), "")
		look := model.LookKey(data)
		reg := `{"version":1,"items":[{"key":"my:00000000000000f1","kind":"future","x":"` + look + `"}]}`
		if err := os.MkdirAll(filepath.Join(cfg, RootName), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(cfg, RootName, regFile), []byte(reg), 0o644); err != nil {
			t.Fatal(err)
		}
		s := Open(cfg)
		put(t, s, "solo-layouts", pit{"bg/s.png", bg}, pit{"looks/s.json", data})
		if res, err := s.RemovePack("solo-layouts", nil); err != nil || res.Files != 2 {
			t.Fatalf("удаление: %v %+v", err, res)
		}
		s2 := Open(cfg)
		_, lookLeft := s2.files[KindAlbum][look]
		_, bgLeft := s2.Find(libKey(bg))
		if lookLeft || bgLeft {
			t.Errorf("макет остался %v, картинка осталась %v — ждали оба удалены", lookLeft, bgLeft)
		}
	})
	t.Run("макет не удалился", func(t *testing.T) {
		if runtime.GOOS == "windows" || os.Geteuid() == 0 {
			t.Skip("права папки на запись здесь удаление не запрещают")
		}
		s, cfg := openTemp(t)
		bg := renderPNG(t, 49, 30)
		data := lookBytes(t, lookOf(libKey(bg), ""), "")
		put(t, s, "solo-layouts", pit{"bg/s.png", bg}, pit{"looks/s.json", data})
		dir := filepath.Join(cfg, RootName, kindDirs[KindAlbum])
		if err := os.Chmod(dir, 0o555); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
		if res, err := s.RemovePack("solo-layouts", nil); err != nil || res.Files != 0 || res.Kept != 1 {
			t.Fatalf("удаление: %v %+v", err, res)
		}
		if _, ok := Open(cfg).Find(libKey(bg)); !ok {
			t.Error("картинка выжившего макета удалена")
		}
	})
}

// А2: junction (Go ≥ 1.23 на Windows — ModeIrregular) — как ссылка: не читается, держим всё.
func TestLinkLike(t *testing.T) {
	for _, c := range []struct {
		m    os.FileMode
		want bool
	}{
		{os.ModeSymlink, true},
		{os.ModeIrregular, true},
		{os.ModeDir | os.ModeIrregular, true},
		{0, false},
		{os.ModeDir, false},
		{os.ModeNamedPipe, false},
	} {
		if got := linkLike(c.m); got != c.want {
			t.Errorf("linkLike(%v) = %v", c.m, got)
		}
	}
}
