package decor

import (
	"bytes"
	"errors"
	"image/png"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// stickerOwn — свои стикеры каталога (скрытых там нет).
func stickerOwn(t *testing.T, s *Store) []Item {
	t.Helper()
	cat, err := s.Catalog(KindSticker, nil, 300, 300)
	if err != nil {
		t.Fatal(err)
	}
	var out []Item
	for _, it := range cat.Items {
		if it.Own {
			out = append(out, it)
		}
	}
	return out
}

// stickerFiles — файлы папки стикеров библиотеки (остаток tmp был бы виден здесь).
func stickerFiles(t *testing.T, s *Store) []string {
	t.Helper()
	ents, err := os.ReadDir(filepath.Join(s.root, kindDirs[KindSticker]))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range ents {
		out = append(out, e.Name())
	}
	return out
}

// Код попадает в «Мои» PNG-стикером с именем «QR · адрес», квадратный; путь файла находится;
// круг Save → Load: запись, тема, имя, размер и файл живы после нового Open.
func TestAddQR(t *testing.T) {
	s, cfg := openTemp(t)
	it, err := s.AddQR("t.me/test")
	if err != nil {
		t.Fatal(err)
	}
	if it.Theme != ThemeMine || it.Name != "QR · t.me/test" || !it.Own || it.W != it.H || it.W == 0 || it.W > MaxStickerSide || !strings.HasPrefix(it.ID, libPrefix) {
		t.Fatalf("плитка: %+v", it)
	}
	p, ok := s.Find(it.ID)
	if !ok || !strings.HasSuffix(p, ".png") {
		t.Fatalf("Find: %q %v", p, ok)
	}
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(raw))
	if err != nil || img.Bounds().Dx() != it.W {
		t.Fatalf("PNG: %v", err)
	}
	if st, err := s.Sticker(it.ID); err != nil || st.Photo.Lib != it.ID {
		t.Fatalf("Sticker: %+v %v", st, err)
	}
	s2 := Open(cfg)
	got := stickerOwn(t, s2)
	if len(got) != 1 || got[0].ID != it.ID || got[0].Name != it.Name || got[0].Theme != ThemeMine || got[0].W != it.W {
		t.Fatalf("после открытия: %+v", got)
	}
	if p2, ok := s2.Find(it.ID); !ok || p2 != p {
		t.Errorf("Find после открытия: %q %v", p2, ok)
	}
	if f := stickerFiles(t, s2); len(f) != 1 {
		t.Errorf("файлы стикеров: %v", f)
	}
}

// Та же ссылка (и её дополнение схемой) — одна запись; другие — другие; скрытая возвращается со
// своим именем; стёртый руками файл пишется заново.
func TestAddQRSame(t *testing.T) {
	s, cfg := openTemp(t)
	a, err := s.AddQR("t.me/test")
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range []string{"t.me/test", "https://t.me/test", "  t.me/test\n"} {
		b, err := s.AddQR(l)
		if err != nil || b.ID != a.ID {
			t.Fatalf("повтор %q: %+v %v", l, b, err)
		}
	}
	if n := len(stickerOwn(t, s)); n != 1 || len(stickerFiles(t, s)) != 1 {
		t.Fatalf("после повторов своих %d, файлов %v", n, stickerFiles(t, s))
	}
	c, err := s.AddQR("https://example.com/long/path?x=1")
	if err != nil || c.ID == a.ID {
		t.Fatalf("другая ссылка: %+v %v", c, err)
	}
	d, err := s.AddQR("mailto:a@b.ru")
	if err != nil || d.ID == a.ID || d.ID == c.ID {
		t.Fatalf("третья: %+v %v", d, err)
	}
	if n := len(stickerOwn(t, s)); n != 3 {
		t.Fatalf("трёх кодов — %d записей", n)
	}

	// Скрытая возвращается той же записью с прежним (своим) именем; на диске по-прежнему три файла.
	if err := s.Rename(KindSticker, a.ID, "Мой телеграм"); err != nil {
		t.Fatal(err)
	}
	if err := s.Hide(KindSticker, a.ID); err != nil {
		t.Fatal(err)
	}
	if n := len(stickerOwn(t, s)); n != 2 {
		t.Fatalf("после скрытия %d", n)
	}
	back, err := s.AddQR("t.me/test")
	if err != nil || back.ID != a.ID || back.Name != "Мой телеграм" || back.Theme != ThemeMine {
		t.Fatalf("возврат скрытой: %+v %v", back, err)
	}
	var ids []string
	for _, it := range stickerOwn(t, Open(cfg)) { // и после перезапуска
		ids = append(ids, it.ID)
	}
	want := []string{a.ID, c.ID, d.ID}
	slices.Sort(ids)
	slices.Sort(want)
	if !slices.Equal(ids, want) || len(stickerFiles(t, s)) != 3 {
		t.Errorf("после возврата: %v, файлы %v", ids, stickerFiles(t, s))
	}

	p, _ := s.Find(a.ID)
	if err := os.Remove(p); err != nil {
		t.Fatal(err)
	}
	again, err := s.AddQR("t.me/test")
	if err != nil || again.ID != a.ID {
		t.Fatalf("после стирания: %+v %v", again, err)
	}
	if _, err := os.Stat(p); err != nil {
		t.Errorf("файл не восстановлен: %v", err)
	}
}

// Только чтение — ErrReadOnly и ничего не записано; плохая ссылка — ошибка без записи и без tmp.
func TestAddQRReadOnly(t *testing.T) {
	s, _ := openTemp(t)
	for _, bad := range []string{"", "  ", "a\nb", strings.Repeat("a", QRMaxLen+1)} {
		if _, err := s.AddQR(bad); err == nil {
			t.Errorf("%q: ошибки нет", bad)
		}
	}
	if len(stickerOwn(t, s)) != 0 || len(stickerFiles(t, s)) != 0 {
		t.Fatalf("плохие ссылки оставили след: %v", stickerFiles(t, s))
	}
	s.mu.Lock()
	s.readOnly = true
	s.mu.Unlock()
	if _, err := s.AddQR("t.me/test"); !errors.Is(err, ErrReadOnly) {
		t.Fatalf("только чтение: %v", err)
	}
	if len(stickerOwn(t, s)) != 0 || len(stickerFiles(t, s)) != 0 {
		t.Errorf("только чтение записало: %v", stickerFiles(t, s))
	}
}

// Длинное имя обрезается до 60 знаков с «…»; предельная кириллическая ссылка кладётся стикером в
// пределах 4096 px.
func TestAddQRName(t *testing.T) {
	s, _ := openTemp(t)
	it, err := s.AddQR("example.com/" + strings.Repeat("я", 400))
	if err != nil {
		t.Fatal(err)
	}
	if r := []rune(it.Name); len(r) != 60 || r[59] != '…' || !strings.HasPrefix(it.Name, "QR · example.com/") {
		t.Errorf("имя: %q", it.Name)
	}
	big, err := s.AddQR(strings.Repeat("щ", QRMaxLen))
	if err != nil || big.W > MaxStickerSide || big.W < 100 {
		t.Fatalf("предельная ссылка: %+v %v", big, err)
	}
}

// TestIsQR: код ссылки узнаётся по признаку (он переживает перезапуск), старая запись без признака — по
// имени «QR · », переименованная старая и чужие ключи — нет; повтор AddQR ставит признак старой записи.
func TestIsQR(t *testing.T) {
	s, cfg := openTemp(t)
	it, err := s.AddQR("t.me/client")
	if err != nil {
		t.Fatal(err)
	}
	if !s.IsQR(it.ID) {
		t.Fatal("новый код не помечен")
	}
	if !Open(cfg).IsQR(it.ID) {
		t.Error("признак не пережил перезапуск")
	}
	for _, key := range []string{"", "sa:x", "my:0000000000000000", "../x"} {
		if s.IsQR(key) {
			t.Errorf("IsQR(%q)", key)
		}
	}
	set := func(qr bool, name string) {
		s.mu.Lock()
		e := s.entryOf(KindSticker, it.ID)
		e.QR, e.Name = qr, name
		s.mu.Unlock()
	}
	set(false, qrNamePrefix+"t.me/client") // запись 0.23–0.27: признака нет, имя от qrName
	if !s.IsQR(it.ID) {
		t.Error("старая запись не узнана по имени")
	}
	set(false, "Мой стикер") // старая запись, переименованная: узнать нечем
	if s.IsQR(it.ID) {
		t.Error("переименованная старая запись узнана")
	}
	if _, err := s.AddQR("t.me/client"); err != nil || !s.IsQR(it.ID) {
		t.Errorf("повтор AddQR не пометил запись: %v", err)
	}
	set(true, "Мой стикер") // переименование помеченной признак сохраняет
	if !s.IsQR(it.ID) {
		t.Error("признак пропал")
	}
}
