package app

import (
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model/printprof"
	"github.com/IliaSA101/sosninaalbums/internal/app/ownprint"
)

// Находки аудита безопасности 123 в слое App: ключ файла у принятой своей (№6), двойники
// названий (№2), хост в сводке импорта (№1).

// №6: два файла с одним id, первый реестр отверг (название занято встроенной) — ключ у второго:
// «Удалить» стирает видимый файл, невидимый остаётся; импорт файла с этим id — копия под новым.
func TestReloadKeyOfAccepted(t *testing.T) {
	a, _ := printApp(t)
	s := a.mustStore(t)
	const id = "own-dddd0001"
	bad := ownFile(t, a, id, "Отвергнутая", 1)
	bad.Name, bad.Gen = "Фабрика Фотокниги", "Фабрики Фотокниги" // занято встроенной
	k1, err := s.SavePrint("", "первый", encoded(t, bad))
	if err != nil {
		t.Fatal(err)
	}
	k2, err := s.SavePrint("", "второй", encoded(t, ownFile(t, a, id, "Видимая", 1)))
	if err != nil {
		t.Fatal(err)
	}
	a.ownp.Reload()
	if cur, ok := printprof.Current().ByID(id); !ok || cur.Name != "Видимая" || a.ownp.OwnKey(id) != k2 {
		t.Fatalf("принята %+v, ключ %q (ждали %q)", cur.Name, a.ownp.OwnKey(id), k2)
	}
	if _, err := a.DeletePrintProfile(id); err != nil {
		t.Fatal(err)
	}
	var keys []string
	for _, f := range s.Prints() {
		keys = append(keys, f.Key)
	}
	if !slices.Equal(keys, []string{k1}) {
		t.Errorf("после удаления в хранилище %v, ждали только невидимый %q", keys, k1)
	}
	// id лежит в хранилище (отвергнутым) — файл с ним добавляется копией под новым own-….
	res, err := a.ownp.ImportPath(profileFile(t, "t.saprint", encoded(t, ownFile(t, a, id, "Третья", 1))))
	if err != nil || !res.Copied || res.Info.ID == id || !printprof.ValidOwnID(res.Info.ID) {
		t.Fatalf("импорт занятого хранилищем id: %+v %v", res, err)
	}
	if !a.ownp.Taken(id) {
		t.Error("id из хранилища не считается занятым")
	}
}

// №2: двойник названия своей или встроенной — «Такое название уже есть» при импорте, в окне и
// при сборке набора; несколько двойников сразу, ни один не записан.
func TestImportTwinNames(t *testing.T) {
	a, ev := printApp(t)
	saved(t, a, editOf(t, a, "fabrika", "Моя студия"))
	for i, name := range []string{
		"Фабрика\u00a0Фотокниги", "ФАБРИКА ФОТОКНИГИ", "Фабрика\u200bФотокниги",
		"Моя\u00a0студия", "МОЯ СТУДИЯ", "моястудия",
	} {
		p := ownFile(t, a, "own-eeee000"+string(rune('1'+i)), "Временная", 1)
		p.Name, p.Gen = name, name
		if p.Validate() != nil { // невидимые знаки разбор не пускает вовсе — тоже отказ
			continue
		}
		if _, err := a.ownp.ImportPath(profileFile(t, "x.saprint", encoded(t, p))); err != ownprint.ErrDup {
			t.Errorf("%q: %v", name, err)
		}
		if c := a.CheckPrintProfile(editOf(t, a, "fabrika", name)); c.OK || c.Errs["name"] != ownprint.ErrDup.Error() {
			t.Errorf("окно, %q: %+v", name, c.Errs)
		}
	}
	if ownCount(a) != 1 || ev.count("print:changed") != 1 {
		t.Errorf("своих %d, событий %d", ownCount(a), ev.count("print:changed"))
	}
	// Название с латинской «а» в слове — не разбирается.
	p := ownFile(t, a, "own-eeee0009", "Временная", 1)
	p.Name, p.Gen = "Фaбрика", "Фaбрики"
	if p.Validate() == nil || !strings.Contains(p.Validate().Error(), "разных алфавитов") {
		t.Errorf("смесь алфавитов: %v", p.Validate())
	}
}

// №1: сводка импорта показывает хост сайта и тогда, когда адрес есть только у изделий.
func TestPrintSummaryHost(t *testing.T) {
	p, _ := printprof.Builtin().ByID("fabrika")
	if !slices.Contains(ownprint.Summary(p), "Сайт: fabrika-fotoknigi.com") {
		t.Errorf("сводка: %v", ownprint.Summary(p))
	}
	p.Export = nil
	if got := ownprint.Summary(p); !slices.Contains(got, "Сайт: fabrika-fotoknigi.com") {
		t.Errorf("без export: %v", got)
	}
	n, _ := printprof.Builtin().ByID("netprint")
	for _, l := range ownprint.Summary(n) {
		if strings.HasPrefix(l, "Сайт:") {
			t.Errorf("у Нетпринта адресов нет: %v", l)
		}
	}
}
