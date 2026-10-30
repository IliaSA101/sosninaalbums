package app

// Страж пар DTO (126 Ш4, Ш5): вложенный тип ответа App написан в app заново (определённый тип
// `type X svc.X` годится только для плоских). Рефлексия сверяет у каждой пары поля, их порядок и
// JSON-теги, а JSON туда-обратно — что фасад ничего не теряет: новое поле службы без пары в app
// молча пропало бы из ответа интерфейсу.

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/model/printprof"
	"github.com/IliaSA101/sosninaalbums/internal/app/agents"
	"github.com/IliaSA101/sosninaalbums/internal/app/libpacks"
	"github.com/IliaSA101/sosninaalbums/internal/app/ownprint"
	"github.com/IliaSA101/sosninaalbums/internal/app/sharer"
)

type dtoPair struct{ app, svc reflect.Type }

func pairOf[A, S any]() dtoPair { return dtoPair{reflect.TypeFor[A](), reflect.TypeFor[S]()} }

// dtoPairs — все пары app ↔ служба; вложенные пары (DecorSkip ↔ Skip) — в том же списке.
var dtoPairs = []dtoPair{
	pairOf[PackInfo, libpacks.Info](),
	pairOf[LibraryNet, libpacks.Net](),
	pairOf[PackList, libpacks.List](),
	pairOf[PackAvail, libpacks.Avail](),
	pairOf[LibraryOffer, libpacks.Offer](),
	pairOf[PackAdd, libpacks.Add](),
	pairOf[PackRemove, libpacks.Remove](),
	pairOf[LibraryUpdate, libpacks.Update](),
	pairOf[DecorSkip, libpacks.Skip](),
	pairOf[PrintEdit, ownprint.Edit](),
	pairOf[PrintEditFormat, ownprint.EditFormat](),
	pairOf[PrintEditCheck, ownprint.EditCheck](),
	pairOf[PrintCheckRow, ownprint.CheckRow](),
	pairOf[PrintSaved, ownprint.Saved](),
	pairOf[PrintImport, ownprint.Import](),
	pairOf[PrintShare, ownprint.Share](),
	pairOf[ShareView, sharer.View](),
	pairOf[ShareService, sharer.ServiceInfo](),
	pairOf[ShareAlbum, sharer.Album](),
	pairOf[ShareLink, sharer.Link](),
	pairOf[AgentSetup, agents.Setup](), // AgentCall и AgentClient — время в unix мс: пары нет, конвертеры в TestAgentTypesConvert
}

// sameShape — типы в паре согласованы: те же, либо пара из списка, либо срезы согласованных.
func sameShape(a, s reflect.Type) bool {
	if a == s {
		return true
	}
	if a.Kind() == reflect.Slice && s.Kind() == reflect.Slice || a.Kind() == reflect.Pointer && s.Kind() == reflect.Pointer {
		return sameShape(a.Elem(), s.Elem())
	}
	for _, p := range dtoPairs {
		if p.app == a && p.svc == s {
			return true
		}
	}
	return false
}

// exportedFields — экспортируемые поля со своим JSON-тегом, по порядку; неэкспортируемые (titles
// в Update: в JSON не идут) пропускаются.
func exportedFields(t reflect.Type) []reflect.StructField {
	var out []reflect.StructField
	for f := range t.Fields() {
		if f.IsExported() {
			out = append(out, f)
		}
	}
	return out
}

func TestDTOPairs(t *testing.T) {
	for _, p := range dtoPairs {
		af, sf := exportedFields(p.app), exportedFields(p.svc)
		if len(af) != len(sf) {
			t.Errorf("%s ↔ %s: полей %d и %d", p.app, p.svc, len(af), len(sf))
			continue
		}
		for i := range af {
			a, s := af[i], sf[i]
			if a.Name != s.Name || a.Tag.Get("json") != s.Tag.Get("json") || !sameShape(a.Type, s.Type) {
				t.Errorf("%s.%s (%s %q) ↔ %s.%s (%s %q): имя, тег или тип разошлись", p.app, a.Name, a.Type, a.Tag.Get("json"),
					p.svc, s.Name, s.Type, s.Tag.Get("json"))
			}
		}
	}
}

// requireFilled — все экспортируемые поля полного литерала ненулевые (рекурсивно по структурам
// служб и срезам): поле, забытое и в конвертере, и в литерале, иначе прошло бы круг молча. Прочие
// типы (model.View) — листья: проверяется только их ненулевость.
func requireFilled(t *testing.T, path string, v reflect.Value) {
	t.Helper()
	if v.IsZero() {
		t.Fatalf("поле %s нулевое: литерал теста неполный, новое поле службы осталось бы непроверенным", path)
	}
	switch v.Kind() {
	case reflect.Struct:
		if pkg := v.Type().PkgPath(); !strings.HasSuffix(pkg, "/libpacks") && !strings.HasSuffix(pkg, "/ownprint") && !strings.HasSuffix(pkg, "/printprof") {
			return
		}
		for i := range v.NumField() {
			if v.Type().Field(i).IsExported() {
				requireFilled(t, path+"."+v.Type().Field(i).Name, v.Field(i))
			}
		}
	case reflect.Slice:
		for i := range v.Len() {
			requireFilled(t, path+"[]", v.Index(i))
		}
	}
}

func filled[T any](t *testing.T, v T) T {
	t.Helper()
	requireFilled(t, reflect.TypeFor[T]().String(), reflect.ValueOf(v))
	return v
}

// Фасад переносит всё: JSON ответа службы и ответа App совпадают.
func TestDTOPairsRoundTrip(t *testing.T) {
	info := filled(t, libpacks.Info{ID: "ivory", Title: "Т", About: "О", Author: "А", License: "CC0", Source: "И", Origin: "file", Version: 2,
		Bg: 1, Sticker: 2, Spread: 3, Frame: 4, Print: 5, Bytes: 6, Removable: true})
	skips := filled(t, []libpacks.Skip{{Name: "a", Why: "b"}})
	net := filled(t, libpacks.Net{Allowed: true, AtStart: true, Checked: 7, Skipped: 8, DemoSkipped: true, DemoBack: true, DemoOffer: true})
	avail := filled(t, libpacks.Avail{ID: "sage", Title: "Т", About: "О", License: "CC0", Version: 2, Bg: 1, Sticker: 2, Spread: 3, Frame: 4, Print: 5,
		Bytes: 6, AddonTitles: []string{"Д"}, Donor: "Свадьба", Why: "нужна версия", Removed: true})
	list := filled(t, libpacks.List{Packs: []libpacks.Info{info}, Net: net, Busy: true, ReadOnly: true, Available: []libpacks.Avail{avail}, All: libpacks.Offer{Count: 1, Bytes: 2}, AvailAt: 9})
	add := filled(t, libpacks.Add{Pack: info, Ask: "update", FromVersion: 1, Token: "т", Skipped: skips})
	upd := filled(t, libpacks.Update{Added: 1, Updated: 2, Skipped: skips, Bytes: 3, Failed: skips, Available: 4, Fresh: 5, Picked: true, Why: "п"})
	rem := filled(t, libpacks.Remove{Files: 1, Kept: 2, Bytes: 3, Addons: []string{"А"}})
	for name, c := range map[string]struct{ svc, app any }{
		"список":                   {list, packList(list)},
		"список пустой":            {libpacks.List{Packs: []libpacks.Info{}, Available: []libpacks.Avail{}}, packList(libpacks.List{Packs: []libpacks.Info{}, Available: []libpacks.Avail{}})},
		"добавление":               {add, packAdd(add)},
		"добавление без пропусков": {libpacks.Add{Skipped: []libpacks.Skip{}}, packAdd(libpacks.Add{Skipped: []libpacks.Skip{}})},
		"обновление":               {upd, libraryUpdate(upd)},
		"сеть":                     {net, libNet(net)},
		"числа удаления":           {rem, PackRemove(rem)},
	} {
		want, err1 := json.Marshal(c.svc)
		got, err2 := json.Marshal(c.app)
		if err1 != nil || err2 != nil || string(want) != string(got) {
			t.Errorf("%s: служба %s, app %s (%v %v)", name, want, got, err1, err2)
		}
	}
}

// То же для типографий: поля окна проходят через фасад туда и обратно, срезы nil и пустые не
// путаются (в JSON это null и []).
func TestDTOPairsPrintRoundTrip(t *testing.T) {
	fm := filled(t, []ownprint.EditFormat{{Name: "30×20", PxW: 7181, PxH: 2398, CoverW: 7819, CoverH: 2835}})
	edit := filled(t, ownprint.Edit{Name: "Н", Base: "fabrika", DPI: 300, Layflat: true, Safe: 10, Bleed: 3, Formats: fm, Spreads: 2, Prefix: "1-", Quality: 95, URL: "https://a.ru"})
	rows := filled(t, []ownprint.CheckRow{{PageW: 1, PageH: 2, SideW: 3, SideH: 4, Err: "e", Field: "px", Hit: true}})
	info := filled(t, printprof.Info{ID: "own-1", Name: "Н", Source: "own", Version: 2, HasExport: true,
		Formats: []printprof.InfoFormat{{Name: "ф", PageW: 1, PageH: 2, PxW: 3, PxH: 4}}})
	check := filled(t, ownprint.EditCheck{OK: true, Errs: map[string]string{"a": "б"}, Formats: rows, Spine: 8, Lines: []string{"л"}, Names: "н"})
	saved := filled(t, ownprint.Saved{ID: "own-1", View: model.View{Rev: 3}})
	imp := filled(t, ownprint.Import{Canceled: true, Info: info, Ask: "replace", Question: "?", Token: "т", Copied: true, Lines: []string{"л"}})
	share := filled(t, ownprint.Share{Canceled: true, Name: "n.saprint"})
	for name, c := range map[string]struct{ svc, app any }{
		"поля":           {edit, editOut(edit)},
		"поля пустые":    {ownprint.Edit{Formats: []ownprint.EditFormat{}}, editOut(ownprint.Edit{Formats: []ownprint.EditFormat{}})},
		"поля без среза": {ownprint.Edit{}, editOut(ownprint.Edit{})},
		"расчёт":         {check, checkOut(check)},
		"расчёт пустой":  {ownprint.EditCheck{Formats: []ownprint.CheckRow{}}, checkOut(ownprint.EditCheck{Formats: []ownprint.CheckRow{}})},
		"сохранено":      {saved, PrintSaved(saved)},
		"импорт":         {imp, PrintImport(imp)},
		"отправка":       {share, PrintShare(share)},
	} {
		want, err1 := json.Marshal(c.svc)
		got, err2 := json.Marshal(c.app)
		if err1 != nil || err2 != nil || string(want) != string(got) {
			t.Errorf("%s: служба %s, app %s (%v %v)", name, want, got, err1, err2)
		}
	}
	// Вход: app → служба → app без потерь.
	in := editOut(edit)
	if back := editOut(editIn(in)); !reflect.DeepEqual(back, in) {
		t.Errorf("круг PrintEdit: %+v → %+v", in, back)
	}
	if nilIn := editIn(PrintEdit{}); nilIn.Formats != nil {
		t.Error("nil-срез Formats стал непустым")
	}
}
