package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// 167 T3 (часть 2б): «Новый альбом из шаблона…» разбирает файл тем же путём, что «Открыть…»
// (UNC в путях фото), не показывает тост шрифтов прежнего альбома при отказе, а регистр пути
// записи о ссылке переписывает только открытие, ставшее текущим.

// uncTemplate — файл альбома с одним фото; path и abs — как записаны в файле.
func uncTemplate(t *testing.T, path, abs string) string {
	t.Helper()
	f := salbum.FileProject{Version: 2, Settings: model.DefaultSettings(), Folders: []salbum.FileFolder{},
		Spreads: []*model.Spread{{ID: "s1", Photos: []string{"p"}}},
		Photos:  map[string]salbum.FilePhoto{"p": {PhotoRef: model.PhotoRef{Path: path, W: 3, H: 2}, Abs: abs}}}
	data, err := json.Marshal(f)
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(testkit.TempDir(t), "Шаблон"+projectExt)
	if err := os.WriteFile(file, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return file
}

// Фото шаблона по сетевому пути (UNC) не открываются: файл отвергнут, как у «Открыть…», прежний
// альбом остаётся, к сети нет обращений (проба тома — только для самого файла шаблона).
func TestNewFromTemplateUNCPhotos(t *testing.T) {
	const unc = `\\host\share\a.jpg`
	cases := map[string]string{
		"путь":     uncTemplate(t, unc, ""),
		"запасной": uncTemplate(t, "a.jpg", unc),
	}
	for name, file := range cases {
		t.Run(name, func(t *testing.T) {
			a, _, _ := backupApp(t, 1)
			cur := a.editor()
			var mu sync.Mutex
			var probed []string
			old := reachable
			reachable = func(p string) bool { mu.Lock(); probed = append(probed, p); mu.Unlock(); return true }
			t.Cleanup(func() { reachable = old })

			_, errT := a.newFromTemplate(file)
			_, errO := a.openPath(file)
			if errT == nil || errO == nil || errT.Error() != errO.Error() {
				t.Fatalf("шаблон: %v; открытие: %v — ждали один и тот же отказ", errT, errO)
			}
			if a.editor() != cur {
				t.Error("прежний альбом заменён")
			}
			mu.Lock()
			defer mu.Unlock()
			for _, p := range probed {
				if p != file || strings.Contains(p, "host") {
					t.Errorf("проба тома для %q: к сети обращаться нельзя", p)
				}
			}
		})
	}
	// Тот же файл с обычным фото открывается: отказ выше — из-за UNC, а не формата.
	ok := uncTemplate(t, "a.jpg", "")
	a, _, _ := backupApp(t, 1)
	if _, err := a.newFromTemplate(ok); err != nil {
		t.Fatalf("шаблон с обычным фото: %v", err)
	}
}

// Отказ startNew (прежний черновик не убран): альбом прежний, и тост «Добавлены шрифты…» шаблона
// ему не достаётся — ни в ответе, ни в следующем GetState. При успехе тост в ответе.
func TestNewFromTemplateRefusedNoFontsToast(t *testing.T) {
	osw, _ := familyOf(t, "Oswald-Regular.ttf")
	files := map[string]string{"z-Oswald-Regular.ttf": "Oswald-Regular.ttf"}
	t.Run("отказ", func(t *testing.T) {
		path := collectedAlbum(t, []string{osw}, files)
		e := newFontEnv(t, nil)
		a := e.a
		cur := a.editor()
		a.GetState()
		if err := os.WriteFile(a.draftPath(), []byte("{не json"), 0o644); err != nil {
			t.Fatal(err)
		}
		a.draftLocked.Store(true)
		a.rename = func(string, string) error { return os.ErrPermission } // .bad не вышло: новый не создаём
		st, err := a.newFromTemplate(path)
		if err != nil {
			t.Fatal(err)
		}
		if a.editor() != cur {
			t.Fatal("оснастка: отказа не было, альбом сменился")
		}
		if st.Info != "" || st.FreshFonts != nil || a.startInfo != "" || a.startFresh != nil {
			t.Errorf("тост шрифтов шаблона у прежнего альбома: %q %v", st.Info, st.FreshFonts)
		}
		if again := a.GetState(); again.Info != "" || again.FreshFonts != nil {
			t.Errorf("тост во втором ответе: %q %v", again.Info, again.FreshFonts)
		}
	})
	t.Run("успех", func(t *testing.T) {
		path := collectedAlbum(t, []string{osw}, files)
		e := newFontEnv(t, nil)
		cur := e.a.editor()
		st, err := e.a.newFromTemplate(path)
		if err != nil {
			t.Fatal(err)
		}
		if e.a.editor() == cur || !strings.HasPrefix(st.Info, "Добавлены шрифты из папки альбома") || len(st.FreshFonts) == 0 {
			t.Errorf("успех: новый альбом %v, тост %q, начертаний %d", e.a.editor() != cur, st.Info, len(st.FreshFonts))
		}
	})
}

// caseOf — путь файла в другом регистре имени.
func caseOf(path string) string {
	return filepath.Join(filepath.Dir(path), strings.ToUpper(filepath.Base(path)))
}

// Регистр пути записи о ссылке переписывает открытие, ставшее текущим (Settled после удачного
// retireIf); отказ смены альбома (from уже не текущий) запись не трогает. Где регистр не значим
// (Windows, macOS) — путь становится настоящим, на Linux имя в другом регистре — другой файл.
func TestOpenSettlesPathCase(t *testing.T) {
	a, file, _ := backupApp(t, 1)
	uid := mustUID(t, a)
	if _, err := a.pinShare(file, uid, false); err != nil {
		t.Fatal(err)
	}
	variant := caseOf(file)
	putProjectRecord(t, a, uid, variant)
	recPath := func() string { rec, _ := shareStore(a).Get(uid); return rec.Path }
	if recPath() != variant {
		t.Fatalf("оснастка: путь записи %q", recPath())
	}
	want := variant
	if paths.Fold("A") != "A" {
		want = file
	}

	// Смена отказана: текущим остался другой альбом (from не тот) — ничего не переписано.
	p, err := salbum.Load(file)
	if err != nil {
		t.Fatal(err)
	}
	_, changed, err := a.openParsed(file, p, albumFonts{}, mustEditor(t, model.New(), ""))
	if err != nil || !changed {
		t.Fatalf("openParsed при чужом from: changed=%v, %v", changed, err)
	}
	if got := recPath(); got != variant {
		t.Fatalf("отказанное открытие переписало путь: %q", got)
	}

	// Просто показ состояния записи не меняет.
	a.GetState()
	if got := recPath(); got != variant {
		t.Fatalf("GetState переписал путь: %q", got)
	}

	// Удачное открытие: путь записи — настоящий.
	a.ed.Store(mustEditor(t, model.New(), ""))
	if _, err := a.openPath(file); err != nil {
		t.Fatal(err)
	}
	if got := recPath(); got != want {
		t.Errorf("после открытия путь записи %q, ждали %q", got, want)
	}
}
