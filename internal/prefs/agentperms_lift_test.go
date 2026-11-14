package prefs

// Подъём мест до корня архива (0.35/153): внутренние места, lifted, круг Save→Load.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/paths"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Место внутри другого места убирается (в том числе «chosen» внутри «parent» и наоборот); порядок
// остальных и предел MaxPlaces — после чистки; несколько вложенных сразу.
func TestPlaceInnerPrefs(t *testing.T) {
	dir := testkit.TempDir(t)
	at := func(elem ...string) string { return filepath.Join(append([]string{dir}, elem...)...) }

	list := PushPlace(nil, Place{Dir: at("A", "2026"), How: PlaceParent})
	list = PushPlace(list, Place{Dir: at("A", "2025"), How: PlaceParent})
	list = PushPlace(list, Place{Dir: at("B"), How: PlaceChosen})
	// Широкое поглощает сразу два узких; «chosen» B остаётся.
	list = PushPlace(list, Place{Dir: at("A"), How: PlaceParent})
	if len(list) != 2 || list[0].Dir != at("A") || list[1].Dir != at("B") {
		t.Fatalf("широкое не поглотило узкие: %+v", list)
	}
	// Узкое внутри широкого не добавляется (в том числе «chosen» внутри «parent»), список тот же.
	for _, p := range []Place{{Dir: at("A", "x", "y"), How: PlaceParent}, {Dir: at("A", "x"), How: PlaceChosen}, {Dir: at("B", "z"), How: PlaceParent}} {
		if got := PushPlace(list, p); len(got) != 2 || got[0].Dir != at("A") || got[1].Dir != at("B") {
			t.Fatalf("внутреннее %+v попало в список: %+v", p, got)
		}
	}
	// «parent» внутри «chosen» убирается, когда «chosen» приходит позже.
	list = PushPlace([]Place{{Dir: at("C", "d"), How: PlaceParent}, {Dir: at("E"), How: PlaceParent}}, Place{Dir: at("C"), How: PlaceChosen})
	if len(list) != 2 || list[0] != (Place{Dir: at("C"), How: PlaceChosen}) || list[1].Dir != at("E") {
		t.Fatalf("parent внутри chosen: %+v", list)
	}
	// Соседи с общим началом имени — не внутренние: «A» и «AB».
	list = PushPlace([]Place{{Dir: at("A"), How: PlaceParent}}, Place{Dir: at("AB"), How: PlaceParent})
	if len(list) != 2 {
		t.Fatalf("«AB» принят за внутреннее «A»: %+v", list)
	}
	// Предел 10 — после чистки: 9 соседей и 12 вложенных в одно дают 10 мест, а не обрезку до чистки.
	var many []Place
	for i := range 9 {
		many = append(many, Place{Dir: at(fmt.Sprint("s", i)), How: PlaceParent})
	}
	for i := range 12 {
		many = append(many, Place{Dir: at("Корень", fmt.Sprint("n", i)), How: PlaceParent})
	}
	got := PushPlace(many[:9], Place{Dir: at("Корень"), How: PlaceParent})
	if len(got) != 10 || got[0].Dir != at("Корень") {
		t.Fatalf("предел после чистки: %d мест", len(got))
	}
	if got := DropInnerPlaces(many); len(got) != MaxPlaces {
		t.Fatalf("21 место: %d", len(got))
	}
	// Пусто и одно место: не падает.
	if got := DropInnerPlaces(nil); len(got) != 0 {
		t.Fatalf("пусто: %+v", got)
	}
	if got := DropInnerPlaces([]Place{{Dir: at("A"), How: PlaceChosen}}); len(got) != 1 {
		t.Fatalf("одно: %+v", got)
	}
}

// LiftPlaces: несколько мест сразу в одно, «chosen» не двигается, Lifted пишется новыми первыми и не
// больше MaxPlaces; нечего переносить — ничего не меняется.
func TestPlacesLift(t *testing.T) {
	dir := testkit.TempDir(t)
	at := func(elem ...string) string { return filepath.Join(append([]string{dir}, elem...)...) }
	mv := func(from, to string) (string, PlaceLift) {
		return paths.DirKey(at(from)), PlaceLift{From: at(from), To: at(to)}
	}

	p := AgentPerms{Places: []Place{
		{Dir: at("Фото", "2026"), How: PlaceParent}, {Dir: at("Фото", "2025"), How: PlaceParent},
		{Dir: at("Свой", "2026"), How: PlaceChosen}, {Dir: at("Другое"), How: PlaceParent},
	}}
	moves := map[string]PlaceLift{}
	for _, f := range [][2]string{{"Фото/2026", "Фото"}, {"Фото/2025", "Фото"}, {"Свой/2026", "Свой"}} {
		k, v := mv(f[0], f[1])
		moves[k] = v
	}
	done := p.LiftPlaces(moves)
	if len(done) != 2 || len(p.Places) != 3 || p.Places[0] != (Place{Dir: at("Фото"), How: PlaceParent}) ||
		p.Places[1] != (Place{Dir: at("Свой", "2026"), How: PlaceChosen}) || p.Places[2].Dir != at("Другое") {
		t.Fatalf("перенос: done=%+v places=%+v", done, p.Places)
	}
	if len(p.Lifted) != 2 || p.Lifted[0].To != at("Фото") {
		t.Fatalf("lifted: %+v", p.Lifted)
	}
	// Нечего переносить: ни мест, ни lifted не меняется.
	before := fmt.Sprint(p)
	if done := p.LiftPlaces(nil); len(done) != 0 || fmt.Sprint(p) != before {
		t.Fatalf("холостой перенос изменил: %+v %v", done, p)
	}
	// Предел lifted.
	var big AgentPerms
	for i := range 12 {
		from, to := fmt.Sprint("m", i, "/2026"), fmt.Sprint("m", i)
		big.Places = append(big.Places, Place{Dir: at(from), How: PlaceParent})
		k, v := mv(from, to)
		big.LiftPlaces(map[string]PlaceLift{k: v})
	}
	if len(big.Lifted) != MaxPlaces || big.Lifted[0].To != at("m11") {
		t.Fatalf("предел lifted: %d, первое %+v", len(big.Lifted), big.Lifted[0])
	}
}

// Круг Save→Load и чистка lifted: негодные пути, From = To, повторы, огромные значения, чужой тип.
func TestPlacesLiftedClean(t *testing.T) {
	dir := testkit.TempDir(t)
	abs := func(s string) string { return filepath.Join(dir, s) }

	// Круг: UpdateAgentPerms → config.json → LoadAgentPerms; ForgetPlaces чистит заметку.
	UpdateAgentPerms(dir, func(p *AgentPerms) {
		p.Places = []Place{{Dir: abs("Фото"), How: PlaceParent}}
		p.Lifted = []PlaceLift{{abs("Фото/2026"), abs("Фото")}}
	})
	got := LoadAgentPerms(dir)
	if len(got.Lifted) != 1 || got.Lifted[0] != (PlaceLift{abs("Фото/2026"), abs("Фото")}) {
		t.Fatalf("круг: %+v", got.Lifted)
	}
	if data, _ := os.ReadFile(ConfigPath(dir)); !strings.Contains(string(data), `"lifted"`) {
		t.Fatalf("lifted не в файле: %s", data)
	}
	if f := ForgetPlaces(dir); len(f.Lifted) != 0 {
		t.Fatalf("ForgetPlaces не снял заметку: %+v", f.Lifted)
	}
	if data, _ := os.ReadFile(ConfigPath(dir)); strings.Contains(string(data), `"lifted"`) {
		t.Fatalf("lifted остался в файле: %s", data)
	}

	// Чужие значения.
	huge := abs(strings.Repeat("я", 600)) // больше 1 КБ
	list := []PlaceLift{
		{"", abs("a")}, {abs("a"), ""}, {"относительный", abs("a")}, {abs("a"), "x/y"},
		{abs("a\x00"), abs("b")}, {abs("c"), abs("c")}, {huge, abs("a")}, {abs("a"), huge},
		{abs("g"), abs("h")}, {abs("g"), abs("i")}, // повтор From: первый новее
		{abs("bad\xff"), abs("b")},
	}
	for i := range 20 {
		list = append(list, PlaceLift{abs(fmt.Sprint("m", i)), abs("root")})
	}
	out := cleanLifts(list)
	if len(out) != MaxPlaces || out[0] != (PlaceLift{abs("g"), abs("h")}) {
		t.Fatalf("после чистки: %d, %+v", len(out), out)
	}
	for _, l := range out {
		if l.From == l.To || !filepath.IsAbs(l.From) || len(l.From) > maxPlaceBytes {
			t.Fatalf("негодное в итоге: %+v", l)
		}
	}
	if len(cleanLifts(nil)) != 0 {
		t.Fatal("nil даёт не пусто")
	}
	// Файл с мусором в lifted: чтение без паники, итог чистый.
	abs1, abs2, abs3 := abs("a"), abs("b"), abs("c")
	raw := fmt.Sprintf(`{"agentPerms":{"lifted":[null,{"from":5},{"from":%q,"to":%q},{"from":%q,"to":%q}],"edit":false}}`, abs1, abs2, abs1, abs3)
	writeRaw(t, dir, raw)
	if l := LoadAgentPerms(dir).Lifted; len(l) != 1 || l[0].To != abs2 {
		t.Fatalf("мусорный lifted: %+v", l)
	}
}
