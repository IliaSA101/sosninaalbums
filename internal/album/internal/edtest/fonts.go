package edtest

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/text"
)

// UseFont регистрирует свою гарнитуру name (082) с одним обычным начертанием из байтов
// встроенного файла file — как её отдала бы библиотека шрифтов; id гарнитуры — результат.
// Реестр text общий на процесс: после теста снимается. Файл брать непохожий на Montserrat
// (Oswald-Regular.ttf), иначе ширины «по тексту» не отличить от запасного шрифта.
func UseFont(t testing.TB, name, file string) string {
	t.Helper()
	b, ok := text.FontFile(file)
	if !ok {
		t.Fatalf("нет встроенного шрифта %s", file)
	}
	id := text.FamilyID(name)
	text.SetCustom([]text.CustomFamily{{ID: id, Name: name, Faces: []text.CustomFace{{Key: FontKey(t, file), Ext: "ttf"}}}},
		func(string) ([]byte, error) { return b, nil })
	t.Cleanup(func() { text.SetCustom(nil, nil) })
	if !text.Available(id) {
		t.Fatalf("гарнитура %q не зарегистрирована", name)
	}
	return id
}

// FontKey — ключ файла начертания (16 hex sha256), как его видит библиотека: тесты проверяют,
// что в файл альбома он не попал.
func FontKey(t testing.TB, file string) string {
	t.Helper()
	b, ok := text.FontFile(file)
	if !ok {
		t.Fatalf("нет встроенного шрифта %s", file)
	}
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:8])
}

// FontID — id своей гарнитуры по названию (text.FamilyID): пакетам формата text недоступен.
func FontID(name string) string { return text.FamilyID(name) }
