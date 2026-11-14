package decor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Реестр набора с темами (спека 170, design §2.2): поля themes, tv, topic; 0.39 их пропускает и видит
// базу в theme; чужой тип поля не роняет реестр.

// worstThemes — худший случай: 16 объявлений, id по 16 знаков, имена по 20 букв кириллицы.
func worstThemes() []PackTheme {
	var out []PackTheme
	for i := range 16 {
		out = append(out, thName(string(rune('a'+i))+"bcdefghijklmnop", "Абвгдеёжзийклмнопр"+string(rune('а'+i))+"я", "holiday", 10+i))
	}
	return out
}

// reg039 — реестр глазами 0.39: известные ей поля, остальное пропускается.
type reg039 struct {
	V       int    `json:"v"`
	ID      string `json:"id"`
	Version int    `json:"version"`
	Origin  string `json:"origin"`
	Base    int64  `json:"base"`
	Items   []struct {
		Key   string `json:"key"`
		Kind  string `json:"kind"`
		Theme string `json:"theme"`
		Name  string `json:"name"`
	} `json:"items"`
}

// Круг запись → чтение: themes, topic, tv на месте; 0.39 видит v = 1 и theme = база.
func TestPackRegRoundTrip(t *testing.T) {
	s, cfg := openTemp(t)
	tickClock(t)
	themes := worstThemes()
	var topics []string
	for _, th := range themes {
		topics = append(topics, th.ID)
	}
	topics = append(topics, "") // предмет без topic
	m, src := topicPack(t, "worst-set", "holiday", 1, themes, topics...)
	installFrom(t, s, OriginNet, m, src)
	path := filepath.Join(cfg, RootName, packsDir, "worst-set.json")
	if data, err := os.ReadFile(path); err != nil || len(data) >= packRegMax {
		t.Fatalf("реестр: %v, %d Б", err, len(data))
	}
	for when, st := range map[string]*Store{"сеанс": s, "после Open": Open(cfg)} {
		st.mu.RLock()
		r := st.packOf("worst-set")
		got, tv := r.Themes, r.TV
		var gotTopics []string
		for _, e := range r.Items {
			gotTopics = append(gotTopics, e.Topic)
		}
		st.mu.RUnlock()
		var want []themeDecl
		for _, th := range themes {
			want = append(want, themeDecl(th))
		}
		if !reflect.DeepEqual(got, want) || tv != 1 || !reflect.DeepEqual(gotTopics, topics) {
			t.Errorf("%s: themes %d (ждали 16), tv %d, topics %v", when, len(got), tv, gotTopics)
		}
		if ps := st.Packs(); len(ps) != 2 || !ps[1].ThemesRead {
			t.Errorf("%s: ThemesRead не поднят у реестра с tv: %+v", when, ps)
		}
		if !st.ownTheme(themes[15].ID) || len(st.tabIdx) != 16 {
			t.Errorf("%s: таблица не построена из реестра", when)
		}
	}
	// Реестр без новых полей (0.39): тем нет, tv = 0, предметы в базах.
	raw := rawRegOf(t, path)
	delete(raw, "themes")
	delete(raw, "tv")
	for _, it := range raw["items"].([]any) {
		delete(it.(map[string]any), "topic")
	}
	b, _ := json.Marshal(raw)
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := readPackReg(path, "worst-set")
	if err != nil || len(r.Themes) != 0 || r.TV != 0 || r.Items[0].Topic != "" || r.Items[0].Theme != "holiday" {
		t.Errorf("реестр 0.39: %v %+v", err, r)
	}
}

// 0.39 читает реестр 0.40: v = 1, theme предмета — база, незнакомые поля пропущены.
func TestPackRegKeepsBase(t *testing.T) {
	s, cfg := openTemp(t)
	tickClock(t)
	m, src := topicPack(t, "base-set", "holiday", 1, worstThemes(), "abcdefghijklmnop", "bbcdefghijklmnop", "")
	installFrom(t, s, OriginNet, m, src)
	data, err := os.ReadFile(filepath.Join(cfg, RootName, packsDir, "base-set.json"))
	if err != nil {
		t.Fatal(err)
	}
	var old reg039
	if err := json.Unmarshal(data, &old); err != nil || old.V != 1 || old.Origin != OriginNet || len(old.Items) != 3 {
		t.Fatalf("0.39 не читает реестр: %v %+v", err, old)
	}
	for _, it := range old.Items {
		if it.Theme != "holiday" {
			t.Errorf("0.39 видит тему %q, ждали базу holiday", it.Theme)
		}
	}
}

// Реестр, прочитанный с диска (Raw-поля обнулены) и записанный обратно, не теряет themes, tv и topic:
// синхронизацию Raw-полей делает сама запись.
func TestWritePackRegSyncsRaw(t *testing.T) {
	s, cfg := openTemp(t)
	tickClock(t)
	m, src := topicPack(t, "keep-set", "holiday", 1, worstThemes()[:2], "abcdefghijklmnop", "bbcdefghijklmnop", "")
	installFrom(t, s, OriginNet, m, src)
	path := filepath.Join(cfg, RootName, packsDir, "keep-set.json")
	r, err := readPackReg(path, "keep-set")
	if err != nil || len(r.Themes) != 2 || r.ThemesRaw != nil {
		t.Fatalf("чтение: %v %+v", err, r)
	}
	if err := s.writePackReg(r); err != nil {
		t.Fatal(err)
	}
	again, err := readPackReg(path, "keep-set")
	if err != nil || !reflect.DeepEqual(again.Themes, r.Themes) || again.TV != 1 || again.Items[0].Topic != "abcdefghijklmnop" || again.Items[2].Topic != "" {
		t.Errorf("после записи: %v %+v", err, again)
	}
}

// Чужой тип полей не роняет реестр: набор грузится, предметы в базе, ключи держатся (heldBy).
func TestPackRegForwardCompat(t *testing.T) {
	// Фрагмент — в шапку реестра или в запись предмета.
	type frag struct{ reg, item string }
	cases := map[string]frag{
		"themes строкой":            {reg: `"themes":"newyear"`},
		"themes объектом":           {reg: `"themes":{"id":"a"}`},
		"themes числом":             {reg: `"themes":7`},
		"themes из чисел":           {reg: `"themes":[1,"x",null,[]]`},
		"tv строкой":                {reg: `"tv":"1"`},
		"tv объектом":               {reg: `"tv":{"v":1}`},
		"tv дробью":                 {reg: `"tv":1.5`},
		"tv отрицательным":          {reg: `"tv":-3`},
		"tv огромным":               {reg: `"tv":1e400`},
		"themes null, tv null":      {reg: `"themes":null,"tv":null`},
		"topic объектом":            {item: `"topic":{"id":"x"}`},
		"topic числом":              {item: `"topic":5`},
		"topic по чужой грамматике": {item: `"topic":"Bad Id"`},
		"topic длиннее 16":          {item: `"topic":"abcdefghijklmnopq"`},
		"topic null":                {item: `"topic":null`},
	}
	dir := testkit.TempDir(t)
	for name, f := range cases {
		item := `{"key":"my:0123456789abcdef","kind":"bg","theme":"kids","name":"Фон"`
		if f.item != "" {
			item += "," + f.item
		}
		body := `{"v":1,"id":"compat","version":2,"origin":"net","installed":5,`
		if f.reg != "" {
			body += f.reg + ","
		}
		body += `"items":[` + item + `}]}`
		path := filepath.Join(dir, "compat.json")
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		r, err := readPackReg(path, "compat")
		if err != nil {
			t.Errorf("%s: реестр не читается: %v", name, err)
			continue
		}
		if len(r.Themes) != 0 || r.TV != 0 || len(r.Items) != 1 || r.Items[0].Topic != "" || r.Items[0].Theme != "kids" || r.Version != 2 {
			t.Errorf("%s: %+v", name, r)
		}
	}
	// Целиком: набор в окне «Наборы», тем нет, предмет в базе.
	cfg := testkit.TempDir(t)
	regs := filepath.Join(cfg, RootName, packsDir)
	if err := os.MkdirAll(regs, 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"v":1,"id":"compat","version":2,"origin":"net","installed":5,"themes":"x","tv":"y","items":[{"key":"my:0123456789abcdef","kind":"bg","theme":"kids","topic":{"a":1}}]}`
	if err := os.WriteFile(filepath.Join(regs, "compat.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	s := Open(cfg)
	if ps := s.Packs(); len(ps) != 2 || ps[1].ID != "compat" || ps[1].ThemesRead {
		t.Errorf("наборы %+v", ps)
	}
	if !s.heldBy("my:0123456789abcdef", nil) {
		t.Error("ключ набора не держится")
	}
}

// Реестр с 17 записями themes читает первые 16 и без предела байт (170-29).
func TestPackRegThemesLimit(t *testing.T) {
	var recs []themeDecl
	for i := range 20 {
		recs = append(recs, themeDecl{ID: "t" + string(rune('a'+i)), Name: "Тема " + string(rune('а'+i)), Base: "kids", Order: 5})
	}
	raw, _ := json.MarshalIndent(recs, "", "          ")
	cfg := testkit.TempDir(t)
	regs := filepath.Join(cfg, RootName, packsDir)
	if err := os.MkdirAll(regs, 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"v":1,"id":"many","version":1,"origin":"net","installed":5,"tv":1,"themes":` + string(raw) + `,"items":[]}`
	if err := os.WriteFile(filepath.Join(regs, "many.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := readPackReg(filepath.Join(regs, "many.json"), "many")
	if err != nil || len(r.Themes) != 16 || r.TV != 1 {
		t.Errorf("themes %d, tv %d, %v", len(r.Themes), r.TV, err)
	}
}

// 0.39 при удалении соседа переписывает в реестре только base: tv, themes и topic остаются.
func TestKeepBaseKeepsTV(t *testing.T) {
	s, cfg := openTemp(t)
	now := setNow(t, t0)
	installAt(t, s, now, "aaa", t0, 1)
	*now = t0 + 60*day
	m, src := topicPack(t, "bbb", "holiday", 2, []PackTheme{thName("newyear", "Новый год", "holiday", 10)}, "newyear", "")
	installFrom(t, s, OriginNet, m, src)
	bbb := filepath.Join(cfg, RootName, packsDir, "bbb.json")
	before := rawRegOf(t, bbb)
	s = Open(cfg)
	if _, err := s.RemovePack("aaa", nil); err != nil {
		t.Fatal(err)
	}
	after := rawRegOf(t, bbb)
	if after["base"] != float64(t0) {
		t.Fatalf("base %v: граница не записана, тест ничего не проверяет", after["base"])
	}
	delete(after, "base")
	delete(before, "base")
	if !reflect.DeepEqual(after, before) || after["tv"] != float64(1) || after["themes"] == nil {
		t.Errorf("сосед изменился не только в base:\n%v\n%v", after, before)
	}
	items := after["items"].([]any)
	if items[0].(map[string]any)["topic"] != "newyear" || items[0].(map[string]any)["theme"] != "holiday" {
		t.Errorf("предмет: %v", items[0])
	}
}
