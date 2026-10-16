package decor

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

func keyOf(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	return "my:" + hex.EncodeToString(sum[:8])
}

// sampleFiles — JPEG, PNG, JPEG под именем .png, копия первого, не картинки, нет файла,
// относительный и пустой путь.
func sampleFiles(t *testing.T, src string) []string {
	t.Helper()
	jpg := testkit.JPEG(t, src, "Свадьба.jpg", 60, 40)
	pngPath := testkit.PNG(t, src, "Узор.png", 30, 30, color.NRGBA{200, 10, 10, 255})
	// JPEG под именем .png: расширение копии — по содержимому.
	disguised := filepath.Join(src, "Подмена.png")
	if err := os.WriteFile(disguised, mustRead(t, testkit.JPEG(t, src, "x.jpg", 61, 40)), 0o644); err != nil {
		t.Fatal(err)
	}
	dup := filepath.Join(src, "копия.jpg")
	if err := os.WriteFile(dup, mustRead(t, jpg), 0o644); err != nil {
		t.Fatal(err)
	}
	txt := filepath.Join(src, "notes.txt")
	garbage := filepath.Join(src, "битый.jpg")
	for _, p := range []string{txt, garbage} {
		if err := os.WriteFile(p, []byte("не картинка"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return []string{jpg, pngPath, disguised, dup, txt, garbage, filepath.Join(src, "нет.jpg"), "rel.jpg", ""}
}

// TestDecorImport — копия под ключ от байтов исходника, имя файла от ключа и расширение по
// содержимому, повтор в том же наборе — «уже есть»; плохие файлы — пропуск без путей в причине.
func TestDecorImport(t *testing.T) {
	src := testkit.TempDir(t)
	s, cfg := openTemp(t)
	files := sampleFiles(t, src)
	res, err := s.Import(KindBg, ThemeAll, files)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Added) != 3 || len(res.Skipped) != 6 {
		t.Fatalf("итог %+v", res)
	}
	if res.Skipped[0].Name != "копия" || !strings.Contains(res.Skipped[0].Why, "уже есть") {
		t.Errorf("повтор: %+v", res.Skipped[0])
	}
	if !strings.Contains(res.Skipped[4].Why, "локальном") || res.Skipped[5].Name != noName {
		t.Errorf("относительный и пустой: %+v", res.Skipped[4:])
	}
	for _, sk := range res.Skipped {
		if strings.Contains(sk.Why, src) {
			t.Errorf("путь в причине: %q", sk.Why)
		}
	}
	wantExt := []string{".jpg", ".png", ".jpg"}
	for i, it := range res.Added {
		want := keyOf(t, files[i])
		if it.ID != want || !it.Own || it.Theme != ThemeMine || it.W == 0 {
			t.Errorf("%d: %+v, ключ %s", i, it, want)
		}
		p := mustFind(t, s, it.ID)
		wantPath := filepath.Join(cfg, RootName, "Фоны", strings.TrimPrefix(want, "my:")+wantExt[i])
		if p != wantPath || string(mustRead(t, p)) != string(mustRead(t, files[i])) {
			t.Errorf("%d: файл %q, ждали %q с теми же байтами", i, p, wantPath)
		}
	}
	if res.Added[0].Name != "Свадьба" || res.Added[0].W != 60 || res.Added[0].H != 40 {
		t.Errorf("первый: %+v", res.Added[0])
	}
	assertNoTmp(t, filepath.Join(cfg, RootName, "Фоны"))

	// Своё для ApplyBackground: ID записи — от ключа, путь — библиотеки.
	bg, err := s.Background(res.Added[0].ID)
	if err != nil || bg.Pattern != nil || bg.Photo.ID != model.LibPhotoID(res.Added[0].ID) ||
		bg.Photo.Lib != res.Added[0].ID || bg.Photo.Path != mustFind(t, s, res.Added[0].ID) || bg.Photo.W != 60 || bg.Photo.Size == 0 {
		t.Fatalf("Background: %+v, %v", bg, err)
	}
}

// TestDecorImportAgain — скрытый возвращается тем же файлом, стёртый руками — восстанавливается,
// видимый — «уже есть»; пустой список — без записи; стикеры — не в этой пачке.
func TestDecorImportAgain(t *testing.T) {
	src := testkit.TempDir(t)
	s, cfg := openTemp(t)
	files := sampleFiles(t, src)
	jpg, pngPath, disguised := files[0], files[1], files[2]
	res := ImportResult{Added: importOK(t, s, "mine", jpg, pngPath, disguised)}
	if err := s.Hide(KindBg, res.Added[1].ID); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(mustFind(t, s, res.Added[2].ID)); err != nil {
		t.Fatal(err)
	}
	again, err := s.Import(KindBg, "school", []string{pngPath, disguised, jpg})
	if err != nil || len(again.Added) != 2 || len(again.Skipped) != 1 {
		t.Fatalf("повторный импорт: %+v, %v", again, err)
	}
	if again.Added[0].ID != res.Added[1].ID || again.Added[0].Theme != ThemeMine || s.entryOf(KindBg, res.Added[1].ID).Hidden {
		t.Errorf("возвращённый: %+v", again.Added[0])
	}
	if !fileExists(mustFind(t, s, res.Added[2].ID)) {
		t.Error("стёртый файл не восстановлен")
	}
	if n := len(readReg(t, cfg).Items); n != 3 {
		t.Errorf("записей %d, ждали 3", n)
	}

	// Пустой список и одни повторы реестр не пишут: метка в файле остаётся.
	reg := filepath.Join(cfg, RootName, regFile)
	if err := os.WriteFile(reg, []byte("метка"), 0o644); err != nil {
		t.Fatal(err)
	}
	if r, err := s.Import(KindBg, "mine", nil); err != nil || len(r.Added)+len(r.Skipped) != 0 {
		t.Errorf("пустой импорт: %+v, %v", r, err)
	}
	if r, _ := s.Import(KindBg, "mine", []string{jpg}); len(r.Skipped) != 1 || string(mustRead(t, reg)) != "метка" {
		t.Errorf("повтор записал реестр: %+v", r)
	}
	if _, err := s.Import("set", "mine", []string{jpg}); !errors.Is(err, ErrKind) {
		t.Errorf("чужой вид: %v", err)
	}
}

func mustRead(t *testing.T, p string) []byte {
	t.Helper()
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func assertNoTmp(t *testing.T, dir string) {
	t.Helper()
	list, _ := os.ReadDir(dir)
	for _, de := range list {
		if strings.HasSuffix(de.Name(), ".tmp") {
			t.Errorf("остался временный %s", de.Name())
		}
	}
}

// TestDecorImportLimits — пределы решения №93: файлов за раз, байты фона, своих на вид,
// вся библиотека, пиксели; остатки оборванного импорта убирает открытие.
func TestDecorImportLimits(t *testing.T) {
	src := testkit.TempDir(t)
	s, cfg := openTemp(t)
	small := testkit.JPEG(t, src, "a.jpg", 20, 20)

	// 101 файл: последний — пропуск по числу (до чтения).
	files := make([]string, MaxImport+1)
	for i := range files {
		files[i] = small
	}
	files[MaxImport] = filepath.Join(src, "сотый.jpg")
	res, err := s.Import(KindBg, "mine", files)
	if err != nil || len(res.Added) != 1 || len(res.Skipped) != MaxImport {
		t.Fatalf("итог: %d добавлено, %d пропущено, %v", len(res.Added), len(res.Skipped), err)
	}
	if last := res.Skipped[MaxImport-1]; last.Name != "сотый" || !strings.Contains(last.Why, fmt.Sprint(MaxImport)) {
		t.Errorf("сверх предела: %+v", last)
	}

	// Больше 200 МБ: заголовок JPEG, хвост — разреженный.
	big := testkit.JPEG(t, src, "big.jpg", 30, 30)
	if err := os.Truncate(big, MaxBgBytes+1); err != nil {
		t.Fatal(err)
	}
	// Больше MaxPixels по заголовку — отказ до чтения.
	huge := testkit.HugeJPEG(t, src, "huge.jpg", 20000, 20000)
	res, _ = s.Import(KindBg, "mine", []string{big, huge})
	if len(res.Skipped) != 2 || !strings.Contains(res.Skipped[0].Why, "200 МБ") || !strings.Contains(res.Skipped[1].Why, "Мп") {
		t.Errorf("большие: %+v", res.Skipped)
	}

	// Своих 2000 (со скрытыми) — новое не помещается; уже бывшее возвращается.
	s.mu.Lock()
	for i := range MaxOwn - 1 {
		s.entries = append(s.entries, &entry{Key: fmt.Sprintf("my:%016x", i+1), Kind: KindBg, Hidden: true})
	}
	s.mu.Unlock()
	next := testkit.JPEG(t, src, "b.jpg", 21, 20)
	if res, _ = s.Import(KindBg, "mine", []string{next}); len(res.Skipped) != 1 || !strings.Contains(res.Skipped[0].Why, "2000") {
		t.Errorf("сверх 2000: %+v", res)
	}
	// Стикеры считаются отдельно, а байты — вся библиотека.
	s.mu.Lock()
	s.entries = s.entries[:1]
	s.entries = append(s.entries, &entry{Key: "my:00000000000000ff", Kind: "set", Bytes: MaxLibraryLen - 10})
	s.mu.Unlock()
	if res, _ = s.Import(KindBg, "mine", []string{next}); len(res.Skipped) != 1 || !strings.Contains(res.Skipped[0].Why, "заполнена") {
		t.Errorf("сверх 2 ГБ: %+v", res)
	}
	assertNoTmp(t, filepath.Join(cfg, RootName, "Фоны"))

	// Остаток оборванного импорта открытие убирает; чужие файлы не трогает.
	dir := filepath.Join(cfg, RootName, "Фоны")
	tmp := filepath.Join(dir, tmpPrefix+"123.tmp")
	other := filepath.Join(dir, "заметка.txt")
	for _, p := range []string{tmp, other} {
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	Open(cfg)
	if fileExists(tmp) || !fileExists(other) {
		t.Errorf("после открытия: tmp %v, чужой %v", fileExists(tmp), fileExists(other))
	}
}

// TestDecorImportBadPaths — папка вместо файла и путь с «..» — пропуск.
func TestDecorImportBadPaths(t *testing.T) {
	src := testkit.TempDir(t)
	s, _ := openTemp(t)
	dir := filepath.Join(src, "папка.jpg")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	res, err := s.Import(KindBg, "mine", []string{dir, filepath.Join(src, "..", filepath.Base(src), "x.jpg")})
	if err != nil || len(res.Skipped) != 2 || len(res.Added) != 0 {
		t.Fatalf("итог %+v, %v", res, err)
	}
}
