package decor

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Прямые записи в реестры (writeQRScan, writeBase): чужое, больше предела, битое и нечитаемое
// файл не меняют; ошибка чтения возвращается, а «нечего писать» — не ошибка.

func TestWriteQRScanForeignFiles(t *testing.T) {
	s, cfg := openTemp(t)
	path := filepath.Join(cfg, RootName, regFile)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	found := map[string]bool{"my:0000000000000001": true}
	cases := map[string]string{
		"не JSON":         `[1, 2`,
		"не объект":       `[]`,
		"нет версии":      `{"items": []}`,
		"версия новее":    `{"version": 999, "items": []}`,
		"версия ноль":     `{"version": 0, "items": []}`,
		"items не массив": `{"version": 1, "items": 5}`,
		"больше предела":  `{"version": 1, "pad": "` + strings.Repeat("я", maxRegSize) + `"}`,
	}
	for name, body := range cases {
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := s.writeQRScan(found); err != nil {
			t.Errorf("%s: ошибка вместо пропуска: %v", name, err)
		}
		if got, _ := os.ReadFile(path); string(got) != body {
			t.Errorf("%s: чужой файл переписан", name)
		}
	}
	// Реестра нет — ошибка чтения возвращается (а не молчаливый успех).
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := s.writeQRScan(found); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("нет реестра: %v", err)
	}
}

// Записи без key/kind и не объекты пропускаются, остальные и незнакомые поля сохраняются; qr ставится
// только стикерам из found, <>& остаются символами.
func TestWriteQRScanSkipsBadItems(t *testing.T) {
	s, cfg := openTemp(t)
	path := filepath.Join(cfg, RootName, regFile)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"version": 1, "future": "<b>&", "items": [5, {"kind": "sticker"}, ` +
		`{"kind": "sticker", "key": "my:0000000000000001", "name": "<a>"}, ` +
		`{"kind": "bg", "key": "my:0000000000000001"}, ` +
		`{"kind": "sticker", "key": "my:0000000000000002"}]}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := s.writeQRScan(map[string]bool{"my:0000000000000001": true}); err != nil {
		t.Fatal(err)
	}
	m := rawRegOf(t, path)
	items := m["items"].([]any)
	if len(items) != 5 || items[0] != float64(5) {
		t.Fatalf("записи: %v", items)
	}
	if items[2].(map[string]any)["qr"] != true || items[2].(map[string]any)["name"] != "<a>" {
		t.Errorf("стикер из found: %v", items[2])
	}
	for _, i := range []int{1, 3, 4} {
		if _, has := items[i].(map[string]any)["qr"]; has {
			t.Errorf("qr у записи %d: %v", i, items[i])
		}
	}
	if m["qrScan"] != float64(qrScanV) || m["future"] != "<b>&" {
		t.Errorf("корень: %v", m)
	}
	if got, _ := os.ReadFile(path); !strings.Contains(string(got), "<b>&") {
		t.Error("<>& экранированы")
	}
}

func TestWriteBaseForeignFiles(t *testing.T) {
	s, cfg := openTemp(t)
	dir := filepath.Join(cfg, RootName, packsDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "aaa.json")
	// Нет файла — не ошибка и не запись.
	if w, err := s.writeBase("aaa", 5); w || err != nil {
		t.Errorf("нет файла: %v %v", w, err)
	}
	cases := map[string]string{
		"не JSON":        `{"v": `,
		"массив":         `[1]`,
		"нет v":          `{"id": "aaa"}`,
		"v новее":        `{"v": 999}`,
		"v ноль":         `{"v": 0}`,
		"больше предела": `{"v": 1, "pad": "` + strings.Repeat("я", packRegMax) + `"}`,
	}
	for name, body := range cases {
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		if w, err := s.writeBase("aaa", 5); w || err != nil {
			t.Errorf("%s: written=%v err=%v", name, w, err)
		}
		if got, _ := os.ReadFile(path); string(got) != body {
			t.Errorf("%s: чужой реестр переписан", name)
		}
	}
	// Папка на месте файла: ошибка чтения возвращается.
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatal(err)
	}
	if w, err := s.writeBase("aaa", 5); w || err == nil {
		t.Errorf("папка вместо файла: %v %v", w, err)
	}
}

// Удаление набора: встроенный и неизвестный — отказ и в плане, и в самом удалении.
func TestRemovePackRefusals(t *testing.T) {
	s, _ := openTemp(t)
	if _, err := s.PlanRemovePack(BuiltinPack, nil); !errors.Is(err, ErrBuiltin) {
		t.Errorf("план встроенного: %v", err)
	}
	if _, err := s.RemovePack(BuiltinPack, nil); !errors.Is(err, ErrBuiltin) {
		t.Errorf("удаление встроенного: %v", err)
	}
	if _, err := s.PlanRemovePack("nope", nil); !errors.Is(err, ErrNotFound) {
		t.Errorf("план неизвестного: %v", err)
	}
	if _, err := s.RemovePack("nope", nil); !errors.Is(err, ErrNotFound) {
		t.Errorf("удаление неизвестного: %v", err)
	}
}
