package app

import (
	"context"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/app/libpacks"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

// «Скачать ролик» берёт свежий каталог (libpacks.Fresh), и он становится сохранённым. Это не должно
// стирать «новые наборы»: что объявлено пользователю, хранится отдельно (libraryNet.announced) и
// двигается только ходом наборов (140, 0.30-140-24).
func TestVideoFreshKeepsNewPackNotice(t *testing.T) {
	// setup — на сервере вышел набор sage; каталог роликов забирает его раньше проверки наборов.
	setup := func(t *testing.T, mutate func(a *App)) (*App, *capture) {
		t.Helper()
		a, l := netApp(t, true)
		ev := captureEvents(a)
		src := d1Zip(t)
		l.publish(t, nil, src)
		if _, err := a.UpdateLibrary(); err != nil {
			t.Fatal(err)
		}
		if mutate != nil {
			mutate(a)
		}
		sage := rezip(t, src, func(m map[string]any) { m["id"], m["title"] = "sage", "Шалфей" })
		l.seq = 2
		l.publish(t, nil, src, sage)
		if cat, err := a.libp.Fresh(context.Background()); err != nil || len(cat.Packs) != 2 {
			t.Fatalf("каталог роликов: %+v, %v", cat, err)
		}
		return a, ev
	}
	for name, mutate := range map[string]func(a *App){
		"проверка при запуске": nil,
		// Файл настроек прежней версии: поля «объявлено» нет — то же, что сохранённый список.
		"старый config без announced": func(a *App) {
			prefs.UpdateLibraryNet(a.cfgDir, func(n *prefs.LibraryNet) { n.Announced = nil })
		},
	} {
		t.Run(name, func(t *testing.T) {
			defer setStart(0, 20*time.Hour)()
			a, ev := setup(t, mutate)
			a.libp.StartCheck()
			starts := ev.of("library:start")
			if len(starts) != 1 || starts[0].(libpacks.Start).Fresh.Count != 1 {
				t.Fatalf("после каталога роликов новый набор не объявлен: %+v", starts)
			}
			a.libp.StartCheck()
			if len(ev.of("library:start")) != 1 {
				t.Error("новое объявлено дважды")
			}
		})
	}
	t.Run("Обновить", func(t *testing.T) {
		a, _ := setup(t, nil)
		res, err := a.UpdateLibrary()
		if err != nil || res.Fresh != 1 {
			t.Fatalf("Fresh %d, %v", res.Fresh, err)
		}
		if res, err = a.UpdateLibrary(); err != nil || res.Fresh != 0 {
			t.Errorf("повтор: Fresh %d, %v", res.Fresh, err)
		}
	})
}

// Первое обращение к сети — ролик, списка наборов у пользователя не было: все наборы для него
// новые, а не «уже виденные» (baseline пишет пустой объявленный список до замены сохранённого).
func TestVideoFreshFirstContact(t *testing.T) {
	defer setStart(0, 20*time.Hour)()
	a, l := netApp(t, true)
	ev := captureEvents(a)
	l.publish(t, nil, d1Zip(t))
	if _, err := a.libp.Fresh(context.Background()); err != nil {
		t.Fatal(err)
	}
	if n := prefs.LibraryNetOf(a.cfgDir); n.Announced == nil || len(n.Announced) != 0 {
		t.Fatalf("объявлено: %#v", n.Announced)
	}
	a.libp.StartCheck()
	starts := ev.of("library:start")
	if len(starts) != 1 || starts[0].(libpacks.Start).Fresh.Count != 1 {
		t.Fatalf("первый список не объявлен: %+v", starts)
	}
}
