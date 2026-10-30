package app

// Подставной диск Windows (subst X: C:\Users\u), аудит 0.34, М4: диск возвращает настоящий путь,
// правила скрытого и домашней смотрят на него, смена тома при разрешении — «ссылка», тихо нельзя.
// Правила Windows идут на любой ОС; сам GetFinalPathNameByHandle — osx, тест windows.

import (
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/app/agents"
)

// noHiddenAttrs — атрибуты «скрытый/системный» в тесте не читаются: временная папка лежит под
// AppData (скрытая), а выдуманные D:\Съёмки на настоящем Windows не существуют.
func noHiddenAttrs(t *testing.T) {
	t.Helper()
	old := quietHidden
	quietHidden = func(string) bool { return false }
	t.Cleanup(func() { quietHidden = old })
}

func TestQuietSubstDrive(t *testing.T) {
	// Диск как у subst X: C:\Users\u и X: D:\Съёмки: введённое на X:, настоящее — на другом томе.
	disks := map[string]string{
		`X:\.ssh`:           `C:\Users\u\.ssh`,
		`X:\Съёмки\Петровы`: `D:\Съёмки\Петровы`,
		`D:\Съёмки\Иван`:    `D:\Съёмки\Иван`,
		`d:\Съёмки\Иван`:    `D:\Съёмки\Иван`, // регистр буквы — не смена тома
	}
	disk := func(p string) (string, error) {
		if r, ok := disks[p]; ok {
			return r, nil
		}
		return p, nil
	}
	old := quietLink
	quietLink = func(string) bool { return false } // Windows-пути на Linux Lstat не прочтёт
	t.Cleanup(func() { quietLink = old })
	noHiddenAttrs(t) // на Windows несуществующий путь osx.Hidden считает скрытым (закрыто по умолчанию)
	cases := []struct {
		hint    string
		why     string
		hasPath bool
	}{
		{`X:\.ssh`, whyBad, false},           // скрытое под домашней — по настоящему пути
		{`X:\Съёмки\Петровы`, whyLink, true}, // том сменился: подмена, тихо нельзя
		{`D:\Съёмки\Иван`, whyPlace, true},   // тома те же: дальше обычные правила (мест в стенде нет)
		{`d:\Съёмки\Иван`, whyPlace, true},
	}
	for _, c := range cases {
		t.Run(c.hint, func(t *testing.T) {
			a := newApp(t)
			rules := winRules(disk)
			a.askM().rules = func() *folderRules { return rules }
			p := a.planFolder(agents.AskReq{Kind: agents.AskAdd, Hint: c.hint, Always: true}, nil, true)
			if p.why != c.why || (p.path != "") != c.hasPath || p.quiet() {
				t.Fatalf("план: %+v, ждали why=%q, путь=%v", p, c.why, c.hasPath)
			}
		})
	}
	// Без «Всегда» причина не считается, но настоящий путь — тот же, что вернул диск.
	a := newApp(t)
	rules := winRules(disk)
	a.askM().rules = func() *folderRules { return rules }
	if p := a.planFolder(agents.AskReq{Kind: agents.AskAdd, Hint: `X:\Съёмки\Петровы`}, nil, false); p.path != `D:\Съёмки\Петровы` || !p.link || p.why != "" {
		t.Errorf("при «Спрашивать»: %+v", p)
	}
}

// winVolume: буква, UNC и пути без тома.
func TestWinVolume(t *testing.T) {
	for in, want := range map[string]string{
		`C:\x`: `C:`, `d:`: `d:`, `\\srv\share\x`: `\\srv\share`, `\\srv\share`: `\\srv\share`, `\\srv`: "", `/x`: "", "": "", `x`: "",
	} {
		if got := winVolume(in); got != want {
			t.Errorf("winVolume(%q) = %q, ждали %q", in, got, want)
		}
	}
}
