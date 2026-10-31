package decor

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Профили типографий в наборах (123 Б4): ставит только origin net, предел 64 КБ, проверка App.

// printPack — набор из профилей data (файл p<i>.saprint).
func printPack(id string, version int, minApp string, data ...[]byte) (PackManifest, fakeSrc) {
	m := PackManifest{ID: id, Version: version, MinApp: minApp, Title: "Набор " + id}
	src := make(fakeSrc, len(data))
	for i, d := range data {
		m.Items = append(m.Items, PackItem{File: "p" + itoa(i) + ".saprint", Kind: KindPrint, Theme: "basic", Name: "Типография " + itoa(i), Size: int64(len(d))})
		src[i] = d
	}
	return m, src
}

func itoa(i int) string { return string(rune('a' + i)) }

func stageNet(t *testing.T, s *Store, m PackManifest, src fakeSrc) (*Staged, error) {
	t.Helper()
	return s.StagePack(m, src, OriginNet, strings.Repeat("ab", 32))
}

func installNet(t *testing.T, s *Store, m PackManifest, src fakeSrc) PackInfo {
	t.Helper()
	st, err := stageNet(t, s, m, src)
	if err != nil || len(st.Skipped) != 0 {
		t.Fatalf("StagePack %s: %v %+v", m.ID, err, st)
	}
	defer st.Close()
	info, err := s.InstallPack(st, nil)
	if err != nil {
		t.Fatal(err)
	}
	return info
}

// installPrints — набор id версии version из профилей data, из каталога, без пропусков.
func installPrints(t *testing.T, s *Store, id string, version int, minApp string, data ...[]byte) PackInfo {
	t.Helper()
	m, src := printPack(id, version, minApp, data...)
	return installNet(t, s, m, src)
}

func TestStagePrintOrigin(t *testing.T) {
	s, cfg := openTemp(t)
	m, src := printPack("sets", 1, "0.26.0", []byte(`{"id":"a"}`))
	// Из файла — пропуск с причиной, файла нет, набор из одних типографий — «нечего ставить».
	st, err := s.StagePack(m, src, OriginFile, strings.Repeat("ab", 32))
	if !errors.Is(err, ErrPackEmpty) || st == nil || len(st.Skipped) != 1 || !strings.Contains(st.Skipped[0].Why, "из файла") {
		t.Fatalf("из файла: %v %+v", err, st)
	}
	if strings.Contains(st.Skipped[0].Why, cfg) {
		t.Errorf("в причине путь: %q", st.Skipped[0].Why)
	}
	if n := len(s.PackPrints()); n != 0 {
		t.Errorf("профилей из файла %d", n)
	}
	// Неизвестный origin сводится к файлу: подменить нельзя.
	if _, err := s.StagePack(m, src, "net2", ""); !errors.Is(err, ErrPackEmpty) {
		t.Errorf("чужой origin принят: %v", err)
	}
	// Из каталога — ставится; строка окна считает типографии.
	info := installNet(t, s, m, src)
	if info.Print != 1 || info.Bg+info.Sticker+info.Spread+info.Frame != 0 || info.Origin != OriginNet || info.Bytes != 10 {
		t.Fatalf("итог: %+v", info)
	}
	got := s.PackPrints()
	if len(got) != 1 || got[0].Pack != "sets" || got[0].Version != 1 || got[0].MinApp != "0.26.0" || string(got[0].Data) != `{"id":"a"}` {
		t.Fatalf("PackPrints: %+v", got)
	}
	// Свои типографии набор не задевает, а их — набор.
	if own := s.Prints(); len(own) != 0 {
		t.Errorf("профиль набора среди своих: %+v", own)
	}
	if _, err := s.SavePrint("", "Своя", []byte(`{"x":1}`)); err != nil {
		t.Fatal(err)
	}
	if len(s.Prints()) != 1 || len(s.PackPrints()) != 1 {
		t.Errorf("свои %d, наборов %d", len(s.Prints()), len(s.PackPrints()))
	}
}

func TestStagePrintCheckAndLimits(t *testing.T) {
	s, _ := openTemp(t)
	bad := errors.New("профиль: это не JSON")
	s.SetPrintCheck(func(d []byte) error {
		if bytes.HasPrefix(d, []byte("bad")) {
			return bad
		}
		return nil
	})
	full := bytes.Repeat([]byte("x"), MaxPrintFile)
	m, src := printPack("sets", 1, "0.26.0", []byte("bad-1"), []byte("ok"), full, []byte{})
	// Заявленные размеры манифеста не доверяются: пустой и больше предела — по фактическим байтам.
	src = append(src, bytes.Repeat([]byte("x"), MaxPrintFile+1))
	m.Items = append(m.Items, PackItem{File: "p9.saprint", Kind: KindPrint, Theme: "basic", Name: "Огромная", Size: 10})
	st, err := stageNet(t, s, m, src)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if len(st.Skipped) != 3 {
		t.Fatalf("пропуски: %+v", st.Skipped)
	}
	for i, want := range []string{"не принята: профиль: это не JSON", "пуст", "больше 64 КБ"} {
		if !strings.Contains(st.Skipped[i].Why, want) {
			t.Errorf("пропуск %d: %q", i, st.Skipped[i].Why)
		}
	}
	if _, err := s.InstallPack(st, nil); err != nil {
		t.Fatal(err)
	}
	if got := s.PackPrints(); len(got) != 2 || len(got[1].Data) != MaxPrintFile {
		t.Errorf("принято %d, предел включительно %d", len(got), len(got[len(got)-1].Data))
	}
	// Без проверки (Store без App) принимается любой профиль в пределе.
	s.SetPrintCheck(nil)
	m2, src2 := printPack("more", 1, "0.26.0", []byte("bad-2"))
	if st2, err := stageNet(t, s, m2, src2); err != nil || len(st2.Skipped) != 0 {
		t.Errorf("без проверки: %v %+v", err, st2)
	}
}

// Битая запись архива (sha256 не сошёлся) — ошибка набора целиком, а не пропуск предмета.
func TestStagePrintDamagedArchive(t *testing.T) {
	s, cfg := openTemp(t)
	m, src := printPack("sets", 1, "0.26.0", []byte("a"), nil)
	if st, err := stageNet(t, s, m, src); err == nil || st != nil {
		t.Fatalf("битый архив принят: %v %v", err, st)
	}
	noTmpAnywhere(t, cfg)
	if len(s.PackPrints()) != 0 {
		t.Error("остался профиль битого набора")
	}
}

func TestStagePrintTooMany(t *testing.T) {
	s, _ := openTemp(t)
	var data [][]byte
	for i := range maxPrintsInPack + 3 {
		data = append(data, []byte{byte('A' + i)})
	}
	m, src := printPack("sets", 1, "0.26.0", data...)
	st, err := stageNet(t, s, m, src)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if len(st.Skipped) != 3 || !strings.Contains(st.Skipped[0].Why, "больше 16") {
		t.Errorf("пропуски: %+v", st.Skipped)
	}
}

// Порядок профилей — по установке наборов; одинаковое содержимое в двух наборах — один файл,
// который уходит вместе с последним набором; перезапуск читает те же профили; обновление набора
// уносит профиль, которого в новой версии нет.
func TestPackPrintsLifecycle(t *testing.T) {
	s, cfg := openTemp(t)
	a, b := []byte(`{"id":"a"}`), []byte(`{"id":"b"}`)
	installPrints(t, s, "first", 1, "0.26.0", a, b)
	installPrints(t, s, "second", 2, "0.26.0", a)
	files := func() int {
		list, _ := os.ReadDir(filepath.Join(cfg, RootName, "Типографии"))
		return len(list)
	}
	if got := s.PackPrints(); len(got) != 3 || got[0].Pack != "first" || got[2].Pack != "second" || got[2].Version != 2 || files() != 2 {
		t.Fatalf("профили %+v, файлов %d", got, files())
	}
	s2 := Open(cfg) // перезапуск
	if got := s2.PackPrints(); len(got) != 3 || string(got[1].Data) != string(b) {
		t.Fatalf("после перезапуска: %+v", got)
	}
	if _, err := s.RemovePack("first", nil); err != nil {
		t.Fatal(err)
	}
	if files() != 1 || len(s.PackPrints()) != 1 { // a держит второй набор, b ушёл
		t.Fatalf("после удаления первого: файлов %d, профилей %d", files(), len(s.PackPrints()))
	}
	// Обновление: версия 3 без профиля a — файл уходит (сборка мусора по ключам).
	installPrints(t, s, "second", 3, "0.26.0", b)
	if got := s.PackPrints(); len(got) != 1 || string(got[0].Data) != string(b) || got[0].Version != 3 || files() != 1 {
		t.Fatalf("после обновления: %+v, файлов %d", got, files())
	}
	if _, err := s.RemovePack("second", nil); err != nil || files() != 0 || len(s.PackPrints()) != 0 {
		t.Errorf("после удаления: %v, файлов %d", err, files())
	}
	noTmpAnywhere(t, cfg)
}

// Реестр набора правят руками: minApp не x.y.z сбрасывается, файл профиля больше предела на
// диске не читается, посторонний файл в папке типографий профилем не становится.
func TestPackPrintsRegistryHandEdited(t *testing.T) {
	s, cfg := openTemp(t)
	installPrints(t, s, "sets", 1, "0.26.0", []byte("ok"))
	reg := filepath.Join(cfg, RootName, packsDir, "sets.json")
	data, err := os.ReadFile(reg)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte(`"minApp": "0.26.0"`)) {
		t.Fatalf("minApp не записан:\n%s", data)
	}
	if err := os.WriteFile(reg, bytes.Replace(data, []byte(`"0.26.0"`), []byte(`"новее"`), 1), 0o644); err != nil {
		t.Fatal(err)
	}
	s2 := Open(cfg)
	if got := s2.PackPrints(); len(got) != 1 || got[0].MinApp != "" {
		t.Errorf("кривой minApp: %+v", got)
	}
	dir := filepath.Join(cfg, RootName, "Типографии")
	list, _ := os.ReadDir(dir)
	if err := os.WriteFile(filepath.Join(dir, list[0].Name()), bytes.Repeat([]byte("x"), MaxPrintFile+1), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := Open(cfg).PackPrints(); len(got) != 0 {
		t.Errorf("огромный файл прочитан: %d", len(got))
	}
}
