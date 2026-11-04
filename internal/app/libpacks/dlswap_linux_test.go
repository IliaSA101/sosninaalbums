//go:build linux

package libpacks

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/decor/fetch"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Безопасность загрузок (аудит 144 Г2): временный файл не открывается заново по пути — ссылка или
// FIFO на его месте не задевают цель и не вешают задание, которое держит свой мьютекс.

const swapKeep = "не трогать"

func swapTarget(t *testing.T) string {
	t.Helper()
	p := filepath.Join(testkit.TempDir(t), "target.txt")
	if err := os.WriteFile(p, []byte(swapKeep), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func wantKept(t *testing.T, p string) {
	t.Helper()
	if got, err := os.ReadFile(p); err != nil || string(got) != swapKeep {
		t.Fatalf("цель ссылки тронута: %q, %v", got, err)
	}
}

// plants — чем подменяют временный файл.
var plants = map[string]func(path, target string) error{
	"ссылка": func(path, target string) error { return os.Symlink(target, path) },
	"FIFO":   func(path, _ string) error { return syscall.Mkfifo(path, 0o600) },
}

// bounded — вызов не виснет (open FIFO на запись без читателя не прерывается отменой контекста).
func bounded(t *testing.T, run func()) {
	t.Helper()
	done := make(chan struct{})
	go func() { run(); close(done) }()
	select {
	case <-done:
	case <-time.After(testkit.Limit(10 * time.Second)):
		t.Fatal("загрузка зависла")
	}
}

// Лист образцов: временный файл подменён, пока сервер держит ответ, — цель не тронута, листа нет,
// временного файла нет.
func TestFetchSheetSwappedTemp(t *testing.T) {
	for name, plant := range plants {
		t.Run(name, func(t *testing.T) {
			fx := newPrevFx(t, nil)
			p := fx.publish("pack-a", "pack-a.jpg", sheetJPEG(t, 10))
			dir := filepath.Join(fx.dir, previewDirName)
			if err := os.MkdirAll(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			target := swapTarget(t)
			fx.setOverride("pack-a.jpg", func(w http.ResponseWriter, _ *http.Request) {
				list, _ := filepath.Glob(filepath.Join(dir, tmpPattern))
				for _, tmp := range list {
					_ = os.Remove(tmp)
					if err := plant(tmp, target); err != nil {
						http.Error(w, err.Error(), http.StatusInternalServerError)
						return
					}
				}
				_, _ = w.Write(fx.body["pack-a.jpg"])
			})
			var err error
			bounded(t, func() { err = fx.s.fetchSheet(context.Background(), fx.s.h.Fetcher(), dir, "pack-a", p) })
			if !errors.Is(err, fetch.ErrDisk) {
				t.Fatalf("ошибка: %v", err)
			}
			wantKept(t, target)
			if got := fx.folder(); len(got) != 0 {
				t.Errorf("в папке %v", got)
			}
		})
	}
}

// Набор: на месте Наборы/.dl-<id>.tmp до запуска лежит ссылка или FIFO — загрузка пишет в свой
// новый файл, цель не тронута, набор встаёт.
func TestNetInstallPlantedTemp(t *testing.T) {
	for name, plant := range plants {
		t.Run(name, func(t *testing.T) {
			e := newDemoEnv(t, false, nil)
			e.publishDemoAsset()
			st, err := e.svc.h.Store()
			if err != nil {
				t.Fatal(err)
			}
			tmp, err := st.DownloadPath(demoPackID)
			if err != nil {
				t.Fatal(err)
			}
			target := swapTarget(t)
			if err := plant(tmp, target); err != nil {
				t.Fatal(err)
			}
			var res Update
			bounded(t, func() { res, err = e.svc.Download(demoPackID) })
			if err != nil || !res.Picked {
				t.Fatalf("Download: %v %+v", err, res)
			}
			wantKept(t, target)
			e.mustStand(demoPackID, "demo")
			if _, err := os.Lstat(tmp); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("временный остался: %v", err)
			}
		})
	}
}

// Набор: путь Наборы/.dl-<id>.tmp подменён ссылкой или FIFO, пока идёт сеть, — архив открывается
// тем же дескриптором, задание не виснет, цель не тронута, набор встаёт, временного файла нет.
func TestNetInstallSwappedTemp(t *testing.T) {
	for name, plant := range plants {
		t.Run(name, func(t *testing.T) {
			e := newDemoEnv(t, false, nil)
			e.publishDemoAsset()
			st, err := e.svc.h.Store()
			if err != nil {
				t.Fatal(err)
			}
			tmp, err := st.DownloadPath(demoPackID)
			if err != nil {
				t.Fatal(err)
			}
			target := swapTarget(t)
			e.srv.setHook(func() {
				// Хук идёт после заголовков: ждём, пока клиент создаст файл, и подменяем путь.
				for end := time.Now().Add(testkit.Limit(5 * time.Second)); time.Now().Before(end); time.Sleep(time.Millisecond) {
					if _, err := os.Lstat(tmp); err == nil {
						break
					}
				}
				_ = os.Remove(tmp)
				if err := plant(tmp, target); err != nil {
					t.Error(err)
				}
			})
			var res Update
			bounded(t, func() { res, err = e.svc.Download(demoPackID) })
			if err != nil || !res.Picked {
				t.Fatalf("Download: %v %+v", err, res)
			}
			wantKept(t, target)
			e.mustStand(demoPackID, "demo")
			if _, err := os.Lstat(tmp); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("временный остался: %v", err)
			}
		})
	}
}

// setHook — действие перед отдачей любого архива (подмена временного файла на время сети).
// Стоит здесь, а не в demo_env_test.go: зовёт его только этот linux-тест, на других ОС
// линтер unused считал метод лишним и падал на Windows.
func (d *demoSrv) setHook(f func()) {
	d.mu.Lock()
	d.hook = f
	d.mu.Unlock()
}
