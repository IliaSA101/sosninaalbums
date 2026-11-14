package prefs

// Привязка места к диску (0.35/153, design §4.1): чистка vol, круг config.json, совместимость с
// 0.34, ключ «номер тома + путь внутри», места внутри друг друга на одном и разных дисках.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

func vol(id, rel string) *PlaceVol { return &PlaceVol{ID: id, Label: "Фото-архив", Rel: rel} }

// Негодный vol теряет привязку, место остаётся; годный чистится (метка, путь), пределы держатся.
func TestPlaceVolClean(t *testing.T) {
	dir := filepath.Join(testkit.TempDir(t), "Клиент")
	cases := []struct {
		name string
		in   *PlaceVol
		want *PlaceVol // nil — привязка потеряна
	}{
		{"годный", &PlaceVol{ID: "win:1A2B3C4D:NTFS", Label: "Архив", Rel: "Фото/Клиент"}, &PlaceVol{ID: "win:1A2B3C4D:NTFS", Label: "Архив", Rel: "Фото/Клиент"}},
		{"корень тома", &PlaceVol{ID: "uuid:abc", Rel: ""}, &PlaceVol{ID: "uuid:abc"}},
		{"nil", nil, nil},
		{"ID пуст", &PlaceVol{Rel: "a"}, nil},
		{"ID длиннее 256", &PlaceVol{ID: strings.Repeat("я", 129)}, nil},
		{"ID ровно 256", &PlaceVol{ID: strings.Repeat("a", 256)}, &PlaceVol{ID: strings.Repeat("a", 256)}},
		{"ID не UTF-8", &PlaceVol{ID: "uuid:\xff"}, nil},
		{"ID с управляющим", &PlaceVol{ID: "uuid:a\x00b"}, nil},
		{"Rel с управляющим", &PlaceVol{ID: "x", Rel: "a\tb"}, nil},
		{"Rel не UTF-8", &PlaceVol{ID: "x", Rel: "a\xfe"}, nil},
		{"Rel длиннее 1 КБ", &PlaceVol{ID: "x", Rel: strings.Repeat("я", 513)}, nil},
		{"Rel с ..", &PlaceVol{ID: "x", Rel: "../etc"}, nil},
		{"Rel вверх через середину", &PlaceVol{ID: "x", Rel: "a/../.."}, nil},
		{"Rel с начальным /", &PlaceVol{ID: "x", Rel: "/etc"}, nil},
		{"Rel чистится", &PlaceVol{ID: "x", Rel: "a//b/./c/../d/"}, &PlaceVol{ID: "x", Rel: "a/b/d"}},
		{"метка чистится", &PlaceVol{ID: "x", Label: "A\x00B\n\xffC"}, &PlaceVol{ID: "x", Label: "ABC"}},
		{"метка режется", &PlaceVol{ID: "x", Label: strings.Repeat("я", 200)}, &PlaceVol{ID: "x", Label: strings.Repeat("я", 128)}},
	}
	for _, c := range cases {
		got, ok := cleanPlace(Place{Dir: dir, How: PlaceChosen, Vol: c.in})
		if !ok || got.Dir != dir {
			t.Fatalf("%s: место потеряно: %+v", c.name, got)
		}
		switch {
		case c.want == nil && got.Vol != nil:
			t.Errorf("%s: привязка осталась: %+v", c.name, got.Vol)
		case c.want != nil && (got.Vol == nil || *got.Vol != *c.want):
			t.Errorf("%s: %+v, ждали %+v", c.name, got.Vol, c.want)
		}
	}
	// Через сверенный вид: много мест с негодным vol — предел MaxPlaces и места на месте.
	var list []Place
	for i := range 3 * MaxPlaces {
		list = append(list, Place{Dir: filepath.Join(dir, strings.Repeat("x", i+1)), How: PlaceParent, Vol: &PlaceVol{ID: ""}})
	}
	got := AgentPerms{Places: list}.clean().Places
	if len(got) != MaxPlaces {
		t.Fatalf("после чистки %d мест, ждали %d", len(got), MaxPlaces)
	}
	for _, p := range got {
		if p.Vol != nil {
			t.Fatalf("негодный vol уцелел: %+v", p)
		}
	}
}

// Круг: UpdateAgentPerms → config.json → LoadAgentPerms сохраняет vol; 0.34 (структура без поля)
// читает файл и при записи vol теряет, место остаётся; файл 0.34 без vol читается как непривязанное.
func TestPlaceVolRoundTrip034(t *testing.T) {
	cfg := testkit.TempDir(t)
	d1, d2 := filepath.Join(cfg, "Клиент"), filepath.Join(cfg, "Другое")
	UpdateAgentPerms(cfg, func(p *AgentPerms) {
		p.Places = PushPlace(p.Places, Place{Dir: d1, How: PlaceChosen, Vol: vol("win:1:NTFS", "Клиент")})
		p.Places = PushPlace(p.Places, Place{Dir: d2, How: PlaceParent})
	})
	got := LoadAgentPerms(cfg).Places
	if len(got) != 2 || got[0].Dir != d2 || got[0].Vol != nil || got[1].Vol == nil || *got[1].Vol != *vol("win:1:NTFS", "Клиент") {
		t.Fatalf("круг: %+v", got)
	}

	// 0.34: читает config.json своими типами и пишет обратно.
	raw := readConfigRaw(t, cfg)
	type place034 struct {
		Dir string `json:"dir"`
		How string `json:"how"`
	}
	var old struct {
		AgentPerms struct {
			Places []place034 `json:"places"`
		} `json:"agentPerms"`
	}
	if err := json.Unmarshal(raw, &old); err != nil {
		t.Fatal(err)
	}
	if len(old.AgentPerms.Places) != 2 {
		t.Fatalf("0.34 не видит мест: %s", raw)
	}
	back, err := json.Marshal(old)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(back), "vol") {
		t.Fatalf("vol пережил запись 0.34: %s", back)
	}
	cfg2 := testkit.TempDir(t)
	writeConfigRaw(t, cfg2, back)
	got = LoadAgentPerms(cfg2).Places
	if len(got) != 2 || got[0].Vol != nil || got[1].Vol != nil || got[1].Dir != d1 {
		t.Fatalf("после 0.34: %+v", got)
	}
}

// Ключ места: номер тома и путь внутри, буква (Dir) в него не входит; два диска под одной буквой — два
// места; привязанное и непривязанное с одним Dir — разные; наружу — 12 hex.
func TestPushPlaceVolKey(t *testing.T) {
	e1, f1 := filepath.Join(testkit.TempDir(t), "E", "Клиент"), filepath.Join(testkit.TempDir(t), "F", "Клиент")
	a := Place{Dir: e1, How: PlaceParent, Vol: vol("uuid:A", "Клиент")}
	moved := Place{Dir: f1, How: PlaceParent, Vol: vol("uuid:A", "Клиент")}
	other := Place{Dir: e1, How: PlaceParent, Vol: vol("uuid:B", "Клиент")}
	bare := Place{Dir: e1, How: PlaceParent}
	if a.Key() != moved.Key() || PlaceKey(a) != PlaceKey(moved) {
		t.Error("смена буквы изменила ключ")
	}
	if a.Key() == other.Key() || PlaceKey(a) == PlaceKey(other) || a.Key() == bare.Key() {
		t.Error("разные диски или привязка дали один ключ")
	}
	if k := PlaceKey(a); len(k) != 12 || strings.Trim(k, "0123456789abcdef") != "" {
		t.Errorf("внешний ключ %q — не 12 hex", k)
	}

	// Одна карта дважды (буква сменилась) — одно место, свежее наверху.
	list := PushPlace(PushPlace(nil, a), moved)
	if len(list) != 1 || list[0].Dir != f1 {
		t.Fatalf("тот же диск дважды: %+v", list)
	}
	// Две карты под одной буквой — два места, порядок: новое сверху; повтор первой — вверх без дубля.
	list = PushPlace(PushPlace(nil, a), other)
	if len(list) != 2 || list[0].Vol.ID != "uuid:B" || list[1].Vol.ID != "uuid:A" {
		t.Fatalf("две карты: %+v", list)
	}
	list = PushPlace(list, a)
	if len(list) != 2 || list[0].Vol.ID != "uuid:A" {
		t.Fatalf("повтор первой карты: %+v", list)
	}
	// Привязанное и непривязанное — не внутри друг друга.
	if list = PushPlace(PushPlace(nil, bare), a); len(list) != 2 {
		t.Fatalf("привязанное и непривязанное слиплись: %+v", list)
	}
}

// Места внутри других — только на одном диске; корень тома забирает все места того же ID.
func TestPlaceVolInside(t *testing.T) {
	dir := testkit.TempDir(t)
	at := func(id, rel string) Place {
		return Place{Dir: filepath.Join(dir, id, rel), How: PlaceParent, Vol: vol(id, rel)}
	}
	if !PlaceInside(at("A", "Фото/2026"), at("A", "Фото")) || !PlaceInside(at("A", "Фото"), at("A", "")) {
		t.Error("на одном диске вложенное не внутри")
	}
	if PlaceInside(at("A", "Фото/2026"), at("B", "Фото")) || PlaceInside(at("B", "Фото"), at("A", "")) {
		t.Error("разные диски — не внутри")
	}
	if PlaceInside(at("A", "Фото2"), at("A", "Фото")) {
		t.Error("соседнее имя с общим началом принято за вложенное")
	}
	bare := Place{Dir: filepath.Join(dir, "A", "Фото", "2026"), How: PlaceParent}
	if PlaceInside(bare, at("A", "Фото")) || PlaceInside(at("A", "Фото/2026"), Place{Dir: filepath.Join(dir, "A", "Фото"), How: PlaceParent}) {
		t.Error("привязанное и непривязанное сравнились")
	}
	list := PushPlace(PushPlace(PushPlace(nil, at("B", "Фото")), at("A", "Фото/2026")), at("A", "Фото"))
	if len(list) != 2 || list[0].Vol.ID != "A" || list[1].Vol.ID != "B" {
		t.Fatalf("внутреннее на том же диске осталось, чужое пропало: %+v", list)
	}
}

// BindPlaces и ForgetPlace: привязка по внешнему ключу, холостые вызовы ничего не пишут, привязанные
// места внутри давших диск убираются; убрать — по ключу.
func TestPlaceBindForget(t *testing.T) {
	dir := testkit.TempDir(t)
	u1 := Place{Dir: filepath.Join(dir, "Фото"), How: PlaceParent}
	u2 := Place{Dir: filepath.Join(dir, "Другое"), How: PlaceChosen}
	p := AgentPerms{Places: []Place{u1, u2}}
	if n := p.BindPlaces(map[string]PlaceVol{"нет": {ID: "x"}, PlaceKey(u1): {ID: ""}}); n != 0 || p.Places[0].Vol != nil {
		t.Fatalf("негодное привязано: %d %+v", n, p.Places)
	}
	if n := p.BindPlaces(nil); n != 0 {
		t.Fatalf("пустая привязка: %d", n)
	}
	if n := p.BindPlaces(map[string]PlaceVol{PlaceKey(u1): {ID: "A", Rel: "Фото"}, PlaceKey(u2): {ID: "A", Rel: "Фото/Другое"}}); n != 2 {
		t.Fatalf("привязано %d, ждали 2", n)
	}
	if len(p.Places) != 1 || p.Places[0].Dir != u1.Dir || p.Places[0].Vol.ID != "A" {
		t.Fatalf("внутреннее на том же диске не убрано: %+v", p.Places)
	}
	if n := p.BindPlaces(map[string]PlaceVol{PlaceKey(p.Places[0]): {ID: "B"}}); n != 0 || p.Places[0].Vol.ID != "A" {
		t.Fatalf("привязанное перепривязано: %+v", p.Places)
	}
	if p.ForgetPlace("000000000000") || len(p.Places) != 1 {
		t.Error("чужой ключ что-то убрал")
	}
	if !p.ForgetPlace(PlaceKey(p.Places[0])) || len(p.Places) != 0 {
		t.Errorf("по ключу не убрано: %+v", p.Places)
	}
}

func readConfigRaw(t *testing.T, dir string) []byte {
	t.Helper()
	b, err := os.ReadFile(ConfigPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func writeConfigRaw(t *testing.T, dir string, b []byte) {
	t.Helper()
	if err := os.WriteFile(ConfigPath(dir), b, 0o600); err != nil {
		t.Fatal(err)
	}
}
