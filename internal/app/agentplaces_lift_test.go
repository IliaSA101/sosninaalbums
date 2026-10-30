package app

// Корень архива (0.35/153, design §2): датированное имя, подъём по трём предкам, узкие места.
// Строковые таблицы идут через правила Windows и Unix на любой ОС; полный подъём (с диском) — на
// настоящих папках во временном каталоге. Источники и проход — agentplaces_sweep_test.go.

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

func TestDatedName(t *testing.T) {
	yes := []string{
		"2026", "2026-05", "2026_05_12", "2026.05", "2026 Петровы", "2026-05-12 Петровы", "1990", "1999 архив",
		"12.05.2026", "1.5.2026 Петровы", "31-12-2026_Ивановы", "05.12.2026 – Свадьба", "2026—лето",
		"2026 " + strings.Repeat("я", 35), // ровно 40 рун
		"2026 Ре\u0301",                   // разложенное «е + ´» → NFC, а цифры в начале всё равно годны
	}
	no := []string{
		"", "1989", "2100", "Свадьбы 2026", "20261", "2026а", "31.13.2026", "32.01.2026", "00.05.2026", "12.05.26",
		"Бухгалтерия 2025", "1 2026", "٢٠٢٦", // арабские цифры — не год
		"2026 " + strings.Repeat("я", 36), // 41 руна
		strings.Repeat("2026 ", 1<<18),    // огромное имя (≈ 1,3 МБ): быстрый отказ, не паника
	}
	for _, n := range yes {
		if !datedName(n) {
			t.Errorf("%.30q: не датировано, ждали да", n)
		}
	}
	for _, n := range no {
		if datedName(n) {
			t.Errorf("%.30q: датировано, ждали нет", n)
		}
	}
	// Разложенное «й»: 40 рун после NFC — датировано, хотя до NFC рун больше.
	if d := "2026 " + strings.Repeat("и\u0306", 35); !datedName(d) {
		t.Errorf("NFC не учтён: %.30q", d)
	}
}

func FuzzDatedName(f *testing.F) {
	for _, s := range []string{"", "2026", "12.05.2026 Петровы", "2026-05", "\xff2026", "٢٠٢٦", strings.Repeat("я", 300)} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		if datedName(s) && len(s) > 4*maxDatedRunes {
			t.Fatalf("длинное имя датировано: %d байт", len(s))
		}
	})
}

// Какой предок уходит под место: самый верхний датированный из base и двух его предков (base = a1 —
// родитель добавленной папки); место — его родитель. Таблица по строкам на правилах двух ОС.
func TestPlaceLiftAncestors(t *testing.T) {
	win, unix := winEnv().rules, unixEnv("linux").rules
	type c struct {
		r    *folderRules
		base string
		want string // родитель датированного предка; "" — датированного нет
	}
	for _, tc := range []c{
		// f = D:\Фото\2026\Петровы → base D:\Фото\2026
		{win, `D:\Фото\2026`, `D:\Фото`},
		{win, `D:\Фото\2026\05`, `D:\Фото`},                  // f = …\2026\05\Петровы
		{win, `D:\Фото\2026\Май`, `D:\Фото`},                 // f = …\2026\Май\Петровы
		{win, `D:\Фото\2026\Петровы`, `D:\Фото`},             // f = …\Петровы\JPG
		{win, `D:\Фото\2026\Петровы\Отбор`, `D:\Фото`},       // f = …\Петровы\Отбор\JPG
		{win, `D:\Фото\2026-05-12 Петровы`, `D:\Фото`},       // f = …\JPG
		{win, `D:\Фото\12.05.2026 Петровы`, `D:\Фото`},       // f = …\JPG
		{win, `D:\Архив\2025\2026\Клиент`, `D:\Архив`},       // самый верхний из двух
		{win, `D:\Фото\2026\05\12\Петровы`, ``},              // год — четвёртый предок
		{win, `D:\Фото\Свадьбы 2026`, ``},                    // год в конце имени
		{win, `D:\Фото\1989`, ``},                            //
		{win, `D:\Фото\2100`, ``},                            //
		{win, `D:\Фото\20261`, ``},                           //
		{win, `D:\Фото\31.13.2026`, ``},                      //
		{win, `D:\Фото\2026 ` + strings.Repeat("я", 36), ``}, // 41 руна
		{win, `D:\Фото\2026 ` + strings.Repeat("я", 35), `D:\Фото`},
		{win, `D:\2026`, `D:\`}, // корень тома — место ниже решит ширина
		{win, `D:\`, ``},
		{unix, `/data/Фото/2026`, `/data/Фото`},
		{unix, `/data/Фото/2026/05`, `/data/Фото`},
		{unix, `/data/2026-05-12 Петровы`, `/data`},
		{unix, `/data/Фото/Петровы`, ``},
		{unix, `/`, ``},
		{unix, `/2026`, `/`},
	} {
		got, found := tc.r.liftTarget(tc.base)
		if got != tc.want || found != (tc.want != "") {
			t.Errorf("%s: родитель = %q (%v), ждали %q", tc.base, got, found, tc.want)
		}
	}
}

// layoutEnv — настоящие папки: «диск» с архивом и «домашняя» с Pictures; правила — как в тестах мест,
// плюс дом, чтобы ширина (150 §4.2) работала. rel создаёт папку и возвращает её настоящий путь.
func layoutEnv(t *testing.T) (e *placeEnv, rel func(...string) string, home string) {
	t.Helper()
	fakeDisk(t)
	e = realEnv()
	root, home := realDir(t, "disk"), realDir(t, "home")
	e.rules.homes = []string{home}
	return e, func(el ...string) string {
		d := filepath.Join(append([]string{root}, el...)...)
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		return d
	}, home
}

// Полный подъём на настоящих папках: место — родитель датированного предка, прочее — как в 0.34.
func TestPlaceLiftReal(t *testing.T) {
	e, rel, _ := layoutEnv(t)
	arch := rel("Фото")
	for _, tc := range []struct{ base, want string }{
		{rel("Фото", "2026"), arch},
		{rel("Фото", "2026", "05"), arch},
		{rel("Фото", "2026", "Петровы", "Отбор"), arch},
		{rel("Фото", "12.05.2026 Петровы"), arch},
		{rel("Фото", "2026", "05", "12", "Петровы"), rel("Фото", "2026", "05", "12", "Петровы")},
		{rel("Фото", "Свадьбы 2026"), rel("Фото", "Свадьбы 2026")},
		{rel("Фото", "Петровы"), rel("Фото", "Петровы")},
	} {
		got, narrow, root, err := e.lift(tc.base)
		if err != nil || got != tc.want || narrow || root {
			t.Errorf("lift(%s) = %q narrow=%v root=%v err=%v, ждали %q", tc.base, got, narrow, root, err, tc.want)
		}
	}
}

// Слишком широко: выше года правила не пускают — место остаётся как в 0.34 и помечается узким
// (подсказка в списке); корень диска — «диск целиком». Сама папка вне правил — ошибка.
func TestPlaceLiftTooWide(t *testing.T) {
	e, rel, home := layoutEnv(t)

	pics := filepath.Join(home, "Pictures", "2026")
	if err := os.MkdirAll(pics, 0o755); err != nil {
		t.Fatal(err)
	}
	got, narrow, root, err := e.lift(pics)
	if err != nil || got != pics || !narrow || root {
		t.Errorf("~/Pictures/2026: %q narrow=%v root=%v err=%v", got, narrow, root, err)
	}
	if n, r := e.narrowOf(prefs.Place{Dir: pics, How: prefs.PlaceParent}); !n || r {
		t.Errorf("narrowOf(~/Pictures/2026) = %v %v", n, r)
	}
	if n, r := e.narrowOf(prefs.Place{Dir: pics, How: prefs.PlaceChosen}); n || r {
		t.Errorf("явное место узким не бывает: %v %v", n, r)
	}
	// Выше узкого места (родитель года пустили) подсказки нет.
	if n, _ := e.narrowOf(prefs.Place{Dir: rel("Фото"), How: prefs.PlaceParent}); n {
		t.Error("широкое место помечено узким")
	}

	// Строки без диска: корень тома и каталог томов Unix — «диск целиком».
	for _, tc := range []struct {
		e          *placeEnv
		dir        string
		narrowWant bool
		rootWant   bool
	}{
		{winEnv(), `D:\2026`, true, true},
		{winEnv(), `E:\2026\05`, true, true},
		{winEnv(), `C:\Users\u\Pictures\2026`, true, false},
		{winEnv(), `D:\Фото\2026`, false, false},
		{unixEnv("linux"), `/home/u/Pictures/2026`, true, false},
		{unixEnv("linux"), `/media/u/X/2026`, true, true},
		{unixEnv("linux"), `/mnt/disk/2026`, true, true},
		{unixEnv("linux"), `/data/Фото/2026`, false, false},
		{unixEnv("darwin"), `/Volumes/Диск/2026`, true, true},
	} {
		if n, r := tc.e.narrowOf(prefs.Place{Dir: tc.dir, How: prefs.PlaceParent}); n != tc.narrowWant || r != tc.rootWant {
			t.Errorf("narrowOf(%s) = %v %v, ждали %v %v", tc.dir, n, r, tc.narrowWant, tc.rootWant)
		}
	}
	// Сама папка не годится (облако, «Документы»): места нет вовсе.
	we := winEnv()
	for _, d := range []string{`C:\Users\u\OneDrive\2026`, `C:\Users\u\Documents\2026`, `D:\`, ``, `относительный\2026`} {
		if n, r := we.narrowOf(prefs.Place{Dir: d, How: prefs.PlaceParent}); n || r {
			t.Errorf("narrowOf(%q) = %v %v: места нет, подсказки быть не должно", d, n, r)
		}
	}
	// Ошибки и пределы: пустой путь, относительный, огромный, управляющий знак, nil-среда.
	for _, d := range []string{"", "относительный/2026", strings.Repeat("я/", 1<<14), "/tmp/2026\x00"} {
		if _, _, _, err := e.lift(d); !errors.Is(err, errPlaceBad) {
			t.Errorf("lift(%.20q): err = %v", d, err)
		}
	}
	var none *placeEnv
	if _, _, _, err := none.lift(pics); err == nil {
		t.Error("lift на nil-среде принят")
	}
}

// Диск не пустил родителя года (нет папки, ссылка): место остаётся как в 0.34, но узким оно не
// становится — дело не в ширине.
func TestPlaceLiftDiskRefuses(t *testing.T) {
	e, rel, _ := layoutEnv(t)
	arch, year := rel("Фото"), rel("Фото", "2026")
	probe := e.rules.disk
	e.rules.disk = func(p string) (string, error) {
		if p == arch {
			return "", errors.New("диск не отвечает")
		}
		return probe(p)
	}
	got, narrow, root, err := e.lift(year)
	if err != nil || got != year || narrow || root {
		t.Errorf("отказ диска: %q narrow=%v root=%v err=%v", got, narrow, root, err)
	}
}
