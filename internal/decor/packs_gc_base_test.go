package decor

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// rawRegOf — разбор реестра как есть (map, без зажима readPackReg).
func rawRegOf(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

// twoPacks — aaa (граница) и bbb в папке реестров; Store открыт заново, как после запуска.
func twoPacks(t *testing.T) (*Store, string) {
	t.Helper()
	s, cfg := openTemp(t)
	now := setNow(t, t0)
	installAt(t, s, now, "aaa", t0, 1)
	installAt(t, s, now, "bbb", t0+60*day, 2)
	return Open(cfg), filepath.Join(cfg, RootName, packsDir)
}

// RemovePack сохраняет незнакомые записи и поля соседних реестров (пара TestLookFileKeysRaw):
// 0.28 не должна стирать то, что поставила 0.29, — записывается только base.
func TestRemovePackKeepsUnknownInNeighbour(t *testing.T) {
	_, regs := twoPacks(t)
	bbb := filepath.Join(regs, "bbb.json")
	m := rawRegOf(t, bbb)
	m["future"] = map[string]any{"form": []any{1.0, "x"}}
	m["items"] = append(m["items"].([]any), map[string]any{"kind": "font", "key": "my:00000000000000f1", "name": "Шрифт", "form": "ttf"})
	data, _ := json.MarshalIndent(m, "", "  ")
	if err := os.WriteFile(bbb, data, 0o644); err != nil {
		t.Fatal(err)
	}
	s := Open(filepath.Dir(filepath.Dir(regs)))
	if _, err := s.RemovePack("aaa", nil); err != nil {
		t.Fatal(err)
	}
	got := rawRegOf(t, bbb)
	if got["base"] != float64(t0) {
		t.Fatalf("base %v, ждали %d", got["base"], t0)
	}
	delete(got, "base")
	delete(m, "base")
	if !reflect.DeepEqual(got, m) {
		t.Errorf("соседний реестр изменился не только в base:\n%v\n%v", got, m)
	}
	noTmpAnywhere(t, filepath.Dir(filepath.Dir(regs)))
}

// Реестр соседа удалён на диске после Open — RemovePack его не воскрешает.
func TestRemovePackDoesNotResurrectNeighbour(t *testing.T) {
	s, regs := twoPacks(t)
	bbb := filepath.Join(regs, "bbb.json")
	if err := os.Remove(bbb); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RemovePack("aaa", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(bbb); !os.IsNotExist(err) {
		t.Errorf("bbb.json воскрес: %v", err)
	}
}

// Реестр соседа новее программы, битый, пустой или огромный — не переписан, удаление проходит.
func TestRemovePackSkipsForeignNeighbour(t *testing.T) {
	huge := `{"v":1,"id":"bbb","pad":"` + strings.Repeat("x", packRegMax) + `"}`
	for name, body := range map[string]string{
		"новее":     `{"v":2,"id":"bbb","installed":1860000000000,"items":[],"future":true}`,
		"битый":     `{"v":1,"id":`,
		"пустой":    ``,
		"не объект": `[1,2]`,
		"без v":     `{"id":"bbb"}`,
		"огромный":  huge,
	} {
		t.Run(name, func(t *testing.T) {
			s, regs := twoPacks(t)
			bbb := filepath.Join(regs, "bbb.json")
			if err := os.WriteFile(bbb, []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
			if _, err := s.RemovePack("aaa", nil); err != nil {
				t.Fatalf("удаление не должно упасть: %v", err)
			}
			if after, _ := os.ReadFile(bbb); !bytes.Equal(after, []byte(body)) {
				t.Errorf("чужой реестр переписан: %.80q", after)
			}
			if _, err := os.Stat(filepath.Join(regs, "aaa.json")); !os.IsNotExist(err) {
				t.Errorf("aaa.json остался: %v", err)
			}
		})
	}
}

// Смешанные соседи: bbb чужой (v=2), ccc обычный — пропуск bbb не обрывает цикл, ccc получает границу.
func TestRemovePackMixedNeighbours(t *testing.T) {
	s, cfg := openTemp(t)
	now := setNow(t, t0)
	installAt(t, s, now, "aaa", t0, 1)
	installAt(t, s, now, "bbb", t0+60*day, 2)
	installAt(t, s, now, "ccc", t0+90*day, 3)
	regs := filepath.Join(cfg, RootName, packsDir)
	bbb := filepath.Join(regs, "bbb.json")
	// installed раньше, чем у ccc: чужой реестр идёт в цикле первым — пропуск не должен обрывать его.
	foreign := `{"v":2,"id":"bbb","installed":1805184000000,"items":[],"future":true}`
	if err := os.WriteFile(bbb, []byte(foreign), 0o644); err != nil {
		t.Fatal(err)
	}
	s = Open(cfg)
	if _, err := s.RemovePack("aaa", nil); err != nil {
		t.Fatalf("удаление не должно упасть: %v", err)
	}
	if after, _ := os.ReadFile(bbb); string(after) != foreign {
		t.Errorf("чужой реестр переписан: %.80q", after)
	}
	if got := rawRegOf(t, filepath.Join(regs, "ccc.json")); got["base"] != float64(t0) {
		t.Errorf("ccc: base %v, ждали %d", got["base"], t0)
	}
	if _, err := os.Stat(filepath.Join(regs, "aaa.json")); !os.IsNotExist(err) {
		t.Errorf("aaa.json остался: %v", err)
	}
}

// Сбой именно записи (реестр читается, папку писать нельзя) — удаление не начинается, повтор доделывает.
func TestRemovePackWriteFailsReadable(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("права на папку не действуют")
	}
	s, regs := twoPacks(t)
	if err := os.Chmod(regs, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(regs, 0o755) })
	if _, err := s.RemovePack("aaa", nil); err == nil || !strings.Contains(err.Error(), "не удалился") {
		t.Fatalf("ждали отказ: %v", err)
	}
	if len(s.Packs()) != 3 || !regularFile(filepath.Join(regs, "aaa.json")) {
		t.Errorf("набор пропал при сбое записи: %+v", s.Packs())
	}
	if err := os.Chmod(regs, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RemovePack("aaa", nil); err != nil {
		t.Fatalf("повтор: %v", err)
	}
	if got := rawRegOf(t, filepath.Join(regs, "bbb.json")); got["base"] != float64(t0) {
		t.Errorf("base после повтора: %v", got["base"])
	}
}

// FuzzWriteBase: любой файл реестра соседа — без паники; записан — остаётся JSON-объектом с base.
func FuzzWriteBase(f *testing.F) {
	for _, seed := range []string{``, `null`, `{`, `[]`, `{"v":1}`, `{"v":2}`, `{"v":1,"base":"x","items":[{"kind":"font"}]}`, `{"v":1e99}`, `{"v":1,"v":2}`} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		s := &Store{root: testkit.TempDir(t)}
		dir := filepath.Join(s.root, packsDir)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(dir, "x.json")
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
		written, err := s.writeBase("x", 42)
		after, _ := os.ReadFile(path)
		if err != nil || !written {
			if !bytes.Equal(after, data) {
				t.Fatalf("файл изменён без записи: %v", err)
			}
			return
		}
		var m, old map[string]json.RawMessage
		if json.Unmarshal(after, &m) != nil || string(m["base"]) != "42" {
			t.Fatalf("после записи: %q", after)
		}
		// Остальные ключи не тронуты (значения — с точностью до пробелов).
		if err := json.Unmarshal(data, &old); err != nil {
			t.Fatalf("записали то, что не объект: %q", data)
		}
		delete(m, "base")
		delete(old, "base")
		if len(m) != len(old) {
			t.Fatalf("набор ключей изменился: %q -> %q", data, after)
		}
		for k, ov := range old {
			var a, b bytes.Buffer
			if json.Compact(&a, ov) != nil || json.Compact(&b, m[k]) != nil || a.String() != b.String() {
				t.Fatalf("ключ %q изменился: %q -> %q", k, ov, m[k])
			}
		}
	})
}
