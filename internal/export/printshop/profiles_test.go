package printshop

import (
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model/printprof"
)

// evilCopy — копия встроенной Фабрики под id и названием id, все адреса — на host; подпись
// кнопки сайта из файла — как у настоящей.
func evilCopy(t *testing.T, id, name, host string) printprof.Profile {
	t.Helper()
	p, _ := printprof.Builtin().ByID("fabrika")
	p.ID, p.Name, p.Gen = id, name, name
	for i := range p.Products {
		p.Products[i].ID = printprof.OwnProductID(id, i+1)
		p.Products[i].URL = "https://" + host + "/p"
	}
	p.Default = p.Products[0].ID
	p.Export.URL = "https://" + host + "/"
	p.Export.SiteButton = "Открыть Фабрику Фотокниги"
	if err := p.Validate(); err != nil {
		t.Fatal(err)
	}
	return p
}

// Повторный аудит 123 №3: у своей и библиотечной кнопка сайта окна «Готово» называет хост, а
// не подпись из файла; у встроенных — подпись из файла. Своя без адресов — общая подпись.
func TestSiteButtonNotFromFile(t *testing.T) {
	own := evilCopy(t, "own-0000000a", "Злая", "evil.example")
	lib := evilCopy(t, "newlab", "Злая библиотечная", "lib.example")
	bare := evilCopy(t, "own-0000000b", "Без сайта", "x.example")
	bare.Export.URL = ""
	for i := range bare.Products {
		bare.Products[i].URL = ""
	}
	snap, skipped := printprof.Compose([]printprof.Profile{lib}, []printprof.Profile{own, bare})
	if len(skipped) != 0 {
		t.Fatal(skipped)
	}
	printprof.UseForTest(t, snap)
	for id, want := range map[string]string{
		"own-0000000a": "Открыть evil.example", "newlab": "Открыть lib.example", "own-0000000b": "Открыть сайт типографии",
		"fabrika": "Открыть Фабрику Фотокниги", "fsfera": "Открыть Фотосферу",
	} {
		pf, ok := ProfileOf(id)
		if !ok {
			t.Fatalf("%s: нет профиля", id)
		}
		if got := Check(album(300, 305, 20, ""), "", pf).SiteButton; got != want {
			t.Errorf("%s: кнопка %q, ждали %q", id, got, want)
		}
	}
	// Библиотечная поверх встроенной (версия новее) — уже не встроенная: подпись по хосту.
	over, _ := printprof.Builtin().ByID("fabrika")
	over.Version++
	for i := range over.Products {
		over.Products[i].URL = "https://fabrika-fotoknigi.com.evil.example/p"
	}
	over.Export.URL = "https://fabrika-fotoknigi.com.evil.example/"
	snap, skipped = printprof.Compose([]printprof.Profile{over}, nil)
	if len(skipped) != 0 {
		t.Fatal(skipped)
	}
	printprof.UseForTest(t, snap)
	if pf, _ := ProfileOf("fabrika"); !strings.HasSuffix(pf.siteButton, "evil.example") {
		t.Errorf("библиотечная поверх встроенной: %q", pf.siteButton)
	}
}
