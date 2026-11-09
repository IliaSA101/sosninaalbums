package fetch

import (
	"encoding/json"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

const (
	pvItemA = "0123456789abcdef"
	pvItemB = "fedcba9876543210"
)

// previewMap — годное поле preview (design §2): три ряда, лист 1920×576.
func previewMap(src Source) map[string]any {
	return map[string]any{
		"url": src.Prefix + "ivory-v1/ivory-1-preview.jpg", "sha256": strings.Repeat("c", 64), "size": 115712, "w": 1920, "h": 576,
		"rows": []any{
			map[string]any{"kind": "bg", "w": 240, "h": 160, "items": []any{pvItemA, pvItemB}},
			map[string]any{"kind": "sticker", "w": 160, "h": 160, "items": []any{pvItemA}},
			map[string]any{"kind": "frame", "w": 160, "h": 160, "items": []any{pvItemB}},
		}}
}

// pvRows — ряды preview из карты как []map для правки.
func pvRows(m map[string]any) []any { return m["rows"].([]any) }

func pvRow(m map[string]any, i int) map[string]any { return pvRows(m)[i].(map[string]any) }

// previewCatalog — body каталога из строк: preview — любой JSON-текст (в том числе негодный по типам).
func previewCatalog(t *testing.T, src Source, previews map[string]string, ids ...string) []byte {
	t.Helper()
	var packs []map[string]any
	for _, id := range ids {
		r := row(id, 1, src.Prefix+id+"-v1/"+id+"-1.zip", []byte("zip "+id))
		if p, ok := previews[id]; ok {
			r["preview"] = json.RawMessage(p)
		}
		packs = append(packs, r)
	}
	return bodyJSON(t, 5, packs...)
}

func pvText(t *testing.T, v any) string {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// Годное поле читается целиком; PreviewRaw после разбора пуст; формат каталога остаётся 1.
func TestPreviewParse(t *testing.T) {
	src := newTestLib(t).source()
	cat, err := ParseCatalog(previewCatalog(t, src, map[string]string{"ivory": pvText(t, previewMap(src))}, "ivory", "plain"), src)
	if err != nil || len(cat.Packs) != 2 || len(cat.Skipped) != 0 {
		t.Fatalf("каталог: %v %+v", err, cat)
	}
	p := cat.Packs[0].Preview
	if p == nil || p.Size != 115712 || p.W != 1920 || p.H != 576 || len(p.Rows) != 3 || p.SHA256 != strings.Repeat("c", 64) ||
		p.Rows[0].Kind != "bg" || p.Rows[0].W != 240 || p.Rows[0].H != 160 || len(p.Rows[0].Items) != 2 || p.Rows[2].Items[0] != pvItemB {
		t.Fatalf("preview: %+v", p)
	}
	if cat.Packs[0].PreviewRaw != nil || cat.Packs[1].PreviewRaw != nil || cat.Packs[1].Preview != nil {
		t.Errorf("сырое поле не обнулено или у набора без поля есть превью: %+v", cat.Packs)
	}
}

// Старый каталог без preview читается как раньше: ни образцов, ни записей в Skipped. Поле null, пусто
// и пробелы — то же самое, не ошибка.
func TestPreviewAbsentOrEmpty(t *testing.T) {
	src := newTestLib(t).source()
	for name, p := range map[string]string{"нет поля": "", "null": "null", "null с пробелами": " null\n"} {
		var previews map[string]string
		if name != "нет поля" {
			previews = map[string]string{"ivory": p}
		}
		cat, err := ParseCatalog(previewCatalog(t, src, previews, "ivory"), src)
		if err != nil || len(cat.Packs) != 1 || cat.Packs[0].Preview != nil || len(cat.Skipped) != 0 {
			t.Errorf("%s: %v %+v", name, err, cat)
		}
	}
	if p, why := parsePreview(nil, src); p != nil || why != "" {
		t.Errorf("пустое сырое поле: %v %q", p, why)
	}
	if p, why := parsePreview(json.RawMessage(" \n\t"), src); p != nil || why != "" {
		t.Errorf("пробелы: %v %q", p, why)
	}
}

// Битое поле одной строки не роняет каталог (security Н1): ошибка типа, не объект, пределы — набор на
// месте, Preview == nil, в Skipped «превью». Соседние наборы и их образцы целы.
func TestPreviewBadTypeKeepsCatalog(t *testing.T) {
	src := newTestLib(t).source()
	good := pvText(t, previewMap(src))
	lit := func(s string) string { return strings.ReplaceAll(s, "PFX", src.Prefix) }
	base := `"url":"PFXa-v1/a-1.jpg","sha256":"` + strings.Repeat("c", 64) + `"`
	for name, bad := range map[string]string{
		"w строкой":     lit(`{` + base + `,"size":10,"w":"240","h":160,"rows":[{"kind":"bg","w":32,"h":32,"items":["` + pvItemA + `"]}]}`),
		"w дробью":      lit(`{` + base + `,"size":10,"w":1920.0,"h":576,"rows":[{"kind":"bg","w":32,"h":32,"items":["` + pvItemA + `"]}]}`),
		"rows объектом": lit(`{` + base + `,"size":10,"w":64,"h":64,"rows":{}}`),
		"items числом":  lit(`{` + base + `,"size":10,"w":64,"h":64,"rows":[{"kind":"bg","w":32,"h":32,"items":[1]}]}`),
		"size дробью":   lit(`{` + base + `,"size":1.5,"w":64,"h":64,"rows":[{"kind":"bg","w":32,"h":32,"items":["` + pvItemA + `"]}]}`),
		"size порядком": lit(`{` + base + `,"size":1e2,"w":64,"h":64,"rows":[{"kind":"bg","w":32,"h":32,"items":["` + pvItemA + `"]}]}`),
		"url числом":    `{"url":5}`,
		"список":        `[]`,
		"строка":        `"preview"`,
		"число":         `42`,
		"true":          `true`,
		"пустой объект": `{}`,
	} {
		cat, err := ParseCatalog(previewCatalog(t, src, map[string]string{"ivory": bad, "other": good}, "ivory", "other", "plain"), src)
		if err != nil {
			t.Errorf("%s: каталог отвергнут: %v", name, err)
			continue
		}
		if len(cat.Packs) != 3 || cat.Packs[0].ID != "ivory" || cat.Packs[0].Preview != nil || cat.Packs[1].Preview == nil {
			t.Errorf("%s: наборы: %+v", name, cat.Packs)
		}
		if len(cat.Skipped) != 1 || cat.Skipped[0].ID != "ivory" || !strings.HasPrefix(cat.Skipped[0].Why, "превью: ") {
			t.Errorf("%s: Skipped: %+v", name, cat.Skipped)
		}
	}
}

// Пределы design §2: каждый выход за предел — Preview == nil и «превью» в Skipped; граница изнутри годна.
func TestPreviewLimits(t *testing.T) {
	src := newTestLib(t).source()
	sheet := func(w, h, cw, ch, n int) map[string]any {
		items := make([]any, n)
		for i := range items {
			items[i] = strconv.FormatInt(int64(0x1000000000000000+i), 16)
		}
		return map[string]any{"url": src.Prefix + "a-v1/a.jpg", "sha256": strings.Repeat("d", 64), "size": 1000, "w": w, "h": h,
			"rows": []any{map[string]any{"kind": "bg", "w": cw, "h": ch, "items": items}}}
	}
	withRows := func(rows ...any) map[string]any {
		m := sheet(4096, 1024, 32, 32, 1)
		m["rows"] = rows
		return m
	}
	rowOf := func(kind string, n int) map[string]any {
		items := make([]any, n)
		for i := range items {
			items[i] = strconv.FormatInt(int64(0x2000000000000000+i), 16)
		}
		return map[string]any{"kind": kind, "w": 64, "h": 64, "items": items}
	}
	twoRows := func(h int) map[string]any {
		m := sheet(64, h, 32, 32, 1)
		m["rows"] = []any{pvRow(m, 0), map[string]any{"kind": "sticker", "w": 32, "h": 32, "items": []any{pvItemA}}}
		return m
	}
	setRoot := func(k string, v any) map[string]any { m := sheet(4096, 1024, 32, 32, 1); m[k] = v; return m }
	setRow := func(k string, v any) map[string]any { m := sheet(4096, 1024, 32, 32, 1); pvRow(m, 0)[k] = v; return m }
	cases := []struct {
		name string
		m    map[string]any
		ok   bool
	}{
		{"минимум", sheet(32, 32, 32, 32, 1), true},
		{"предел листа 4096×1024", sheet(4096, 1024, 480, 480, 8), true},
		{"предел площади 2048×2048", sheet(2048, 2048, 480, 480, 4), true},
		{"площадь 4 Мп+", sheet(4096, 2048, 32, 32, 1), false},
		{"w 4097", sheet(4097, 32, 32, 32, 1), false},
		{"h 2049", sheet(64, 2049, 32, 32, 1), false},
		{"w 0", sheet(0, 32, 32, 32, 1), false},
		{"h отрицательная", sheet(64, -1, 32, 32, 1), false},
		{"size 0", setRoot("size", 0), false},
		{"size отрицательный", setRoot("size", -5), false},
		{"size MaxPreview", setRoot("size", MaxPreview), true},
		{"size MaxPreview+1", setRoot("size", MaxPreview+1), false},
		{"size огромный", setRoot("size", int64(1)<<62), false},
		{"sha короткий", setRoot("sha256", strings.Repeat("c", 63)), false},
		{"sha в верхнем регистре", setRoot("sha256", strings.Repeat("C", 64)), false},
		{"sha пустой", setRoot("sha256", ""), false},
		{"url чужого хоста", setRoot("url", "https://example.com/a-v1/a.jpg"), false},
		{"url с ..", setRoot("url", src.Prefix+"a-v1/../a.jpg"), false},
		{"url пустой", setRoot("url", ""), false},
		{"url глубже", setRoot("url", src.Prefix+"a/b/c.jpg"), false},
		{"клетка 31", sheet(64, 64, 31, 32, 1), false},
		{"клетка высота 31", sheet(64, 64, 32, 31, 1), false},
		{"клетка 481", sheet(1024, 1024, 481, 32, 1), false},
		{"клетка высота 481", sheet(1024, 1024, 32, 481, 1), false},
		{"12 предметов", sheet(384, 32, 32, 32, 12), true},
		{"13 предметов", sheet(4096, 32, 32, 32, 13), false},
		{"0 предметов", sheet(64, 64, 32, 32, 0), false},
		{"ряд шире листа", sheet(95, 64, 32, 32, 3), false},
		{"ряд впритык", sheet(96, 64, 32, 32, 3), true},
		{"ряды впритык по высоте", twoRows(64), true},
		{"ряды выше листа", twoRows(63), false},
		{"пять рядов", withRows(rowOf("bg", 3), rowOf("sticker", 3), rowOf("frame", 3), rowOf("spread", 3), rowOf("album", 3)), true},
		{"шесть рядов", withRows(rowOf("bg", 1), rowOf("sticker", 1), rowOf("frame", 1), rowOf("spread", 1), rowOf("album", 1), rowOf("bg", 1)), false},
		{"ноль рядов", withRows(), false},
		{"повтор вида", withRows(rowOf("bg", 1), rowOf("bg", 2)), false},
		{"неизвестный вид", setRow("kind", "video"), false},
		{"вид в верхнем регистре", setRow("kind", "BG"), false},
		{"вид пустой", setRow("kind", ""), false},
		{"предмет короткий", setRow("items", []any{"0123456789abcde"}), false},
		{"предмет длинный", setRow("items", []any{"0123456789abcdef0"}), false},
		{"предмет не hex", setRow("items", []any{"0123456789abcdeg"}), false},
		{"предмет в верхнем регистре", setRow("items", []any{"0123456789ABCDEF"}), false},
		{"повтор предмета в ряду", setRow("items", []any{pvItemA, pvItemA}), false},
		{"предмет null", setRow("items", []any{nil}), false},
	}
	for _, tc := range cases {
		p, why := parsePreview(json.RawMessage(pvText(t, tc.m)), src)
		if tc.ok && (p == nil || why != "") || !tc.ok && (p != nil || !strings.HasPrefix(why, "превью: ")) {
			t.Errorf("%s: %+v %q", tc.name, p, why)
		}
	}
	// Одни и те же предметы в разных рядах допустимы: повтор ищется только внутри ряда.
	if p, why := parsePreview(json.RawMessage(pvText(t, previewMap(src))), src); p == nil || why != "" {
		t.Errorf("предмет в двух рядах: %q", why)
	}
}

// Сырое поле больше 8 КиБ — отказ до разбора JSON: и годное по смыслу с пробелами, и огромный список,
// и глубокая вложенность. Ровно 8 КиБ ещё читается.
func TestPreviewRawOverLimit(t *testing.T) {
	src := newTestLib(t).source()
	good := pvText(t, previewMap(src))
	pad := func(n int) string { return good[:len(good)-1] + strings.Repeat(" ", n-len(good)) + "}" }
	for name, tc := range map[string]struct {
		raw string
		ok  bool
	}{
		"ровно предел":     {pad(MaxPreviewRaw), true},
		"предел и байт":    {pad(MaxPreviewRaw + 1), false},
		"в 100 раз больше": {pad(MaxPreviewRaw * 100), false},
		"глубокая вложенность": {strings.Repeat("[", MaxPreviewRaw+1) +
			strings.Repeat("]", MaxPreviewRaw+1), false},
		"глубокая в пределе": {strings.Repeat("[", MaxPreviewRaw/2) + strings.Repeat("]", MaxPreviewRaw/2), false},
		"тысячи предметов":   {`{"rows":[{"items":[` + strings.Repeat(`"`+pvItemA+`",`, 5000) + `"x"]}]}`, false},
	} {
		p, why := parsePreview(json.RawMessage(tc.raw), src)
		if tc.ok && p == nil || !tc.ok && (p != nil || !strings.HasPrefix(why, "превью: ")) {
			t.Errorf("%s: %v %q", name, p, why)
		}
	}
	if _, why := parsePreview(json.RawMessage(pad(MaxPreviewRaw+1)), src); !strings.Contains(why, "8 КиБ") {
		t.Errorf("причина: %q", why)
	}
	// Тот же отказ через весь каталог: набор остаётся, каталог читается. Пробелы Marshal вычёркивает —
	// длину даёт лишнее поле.
	big := good[:len(good)-1] + `,"pad":"` + strings.Repeat("x", MaxPreviewRaw) + `"}`
	cat, err := ParseCatalog(previewCatalog(t, src, map[string]string{"ivory": big}, "ivory"), src)
	if err != nil || len(cat.Packs) != 1 || cat.Packs[0].Preview != nil || cat.Packs[0].PreviewRaw != nil || len(cat.Skipped) != 1 {
		t.Errorf("каталог: %v %+v", err, cat)
	}
}

// Незнакомые поля — в preview, рядах и корне строки — пропускаются: каталог новее программы читается.
func TestPreviewUnknownFields(t *testing.T) {
	src := newTestLib(t).source()
	m := previewMap(src)
	m["next"] = map[string]any{"a": []any{1}}
	pvRow(m, 0)["label"] = "x"
	pvRow(m, 1)["alt"] = []any{1, 2}
	cat, err := ParseCatalog(previewCatalog(t, src, map[string]string{"ivory": pvText(t, m)}, "ivory"), src)
	if err != nil || len(cat.Skipped) != 0 || cat.Packs[0].Preview == nil || len(cat.Packs[0].Preview.Rows) != 3 {
		t.Errorf("лишние поля: %v %+v", err, cat)
	}
}

// Несколько строк каталога: у каждой своё поле; плохие превью не мешают соседям и порядку наборов;
// строка, отброшенная по другой причине (повтор id, плохой sha), превью не разбирает и «превью» не пишет.
func TestPreviewSeveralRows(t *testing.T) {
	src := newTestLib(t).source()
	good := pvText(t, previewMap(src))
	badSum := row("badsum", 1, src.Prefix+"badsum-v1/badsum-1.zip", []byte("z"))
	badSum["sha256"] = "x"
	badSum["preview"] = json.RawMessage(`{"w":"x"}`)
	dupBad := row("first", 1, src.Prefix+"first-v1/first-1.zip", []byte("z"))
	dupBad["preview"] = json.RawMessage(`{"w":"x"}`)
	packs := []map[string]any{}
	for _, id := range []string{"first", "second", "third", "fourth"} {
		r := row(id, 1, src.Prefix+id+"-v1/"+id+"-1.zip", []byte("z"+id))
		switch id {
		case "first", "fourth":
			r["preview"] = json.RawMessage(good)
		case "second":
			r["preview"] = json.RawMessage(`{"rows":"x"}`)
		}
		packs = append(packs, r)
	}
	packs = append(packs, badSum, dupBad)
	cat, err := ParseCatalog(bodyJSON(t, 2, packs...), src)
	if err != nil || len(cat.Packs) != 4 {
		t.Fatalf("каталог: %v %+v", err, cat.Packs)
	}
	have := ""
	for _, e := range cat.Packs {
		if e.Preview != nil {
			have += e.ID + ","
		}
	}
	if have != "first,fourth," {
		t.Errorf("образцы у: %q", have)
	}
	var why []string
	for _, s := range cat.Skipped {
		why = append(why, s.ID+"|"+s.Why)
	}
	got := strings.Join(why, "; ")
	if len(cat.Skipped) != 3 || !strings.HasPrefix(cat.Skipped[0].Why, "превью: ") || cat.Skipped[0].ID != "second" ||
		cat.Skipped[1].ID != "badsum" || cat.Skipped[2].Why != "повтор id" || strings.Count(got, "превью") != 1 {
		t.Errorf("Skipped: %s", got)
	}
}

// Круг конверта: каталог с образцами проходит тот же путь, что список на диске (Client.Parse), —
// подпись, пределы, поле. Env остаётся байт в байт, поле в нём не обрезано.
func TestPreviewThroughEnvelope(t *testing.T) {
	l := newTestLib(t)
	src := l.source()
	body := previewCatalog(t, src, map[string]string{"ivory": pvText(t, previewMap(src))}, "ivory")
	cat, err := l.client().Parse(l.envelope(body), 0)
	if err != nil || len(cat.Packs) != 1 || cat.Packs[0].Preview == nil || cat.Packs[0].Preview.Rows[1].Kind != "sticker" || len(cat.Skipped) != 0 {
		t.Fatalf("Parse: %v %+v", err, cat)
	}
	// Повторный разбор тех же байт даёт то же.
	again, err := ParseCatalog(body, src)
	if err != nil || again.Packs[0].Preview == nil || again.Packs[0].Preview.URL != cat.Packs[0].Preview.URL {
		t.Errorf("повтор: %v %+v", err, again.Packs)
	}
}

// Фаззинг разбора preview: любые байты — без паники; принятое поле уложено в пределы.
func FuzzParsePreview(f *testing.F) {
	src := GitHub()
	good := previewMap(src)
	data, _ := json.Marshal(good)
	f.Add(data)
	f.Add([]byte(`null`))
	f.Add([]byte(`{"rows":{}}`))
	f.Add([]byte(`{"w":"240","rows":[{"items":[1]}]}`))
	f.Add([]byte(`{"size":1.5}`))
	f.Add([]byte(`[[[[[[[`))
	f.Add([]byte(``))
	f.Fuzz(func(t *testing.T, raw []byte) {
		p, why := parsePreview(raw, src)
		if p == nil {
			return
		}
		if why != "" || len(raw) > MaxPreviewRaw || p.Size < 1 || p.Size > MaxPreview || !validSum(p.SHA256) || !src.assetURL(p.URL) ||
			p.W < 1 || p.W > maxPreviewW || p.H < 1 || p.H > maxPreviewH || p.W*p.H > maxPreviewArea || len(p.Rows) < 1 || len(p.Rows) > maxPreviewRows {
			t.Fatalf("негодное принято: %+v", p)
		}
		sumH := 0
		for _, r := range p.Rows {
			sumH += r.H
			if !previewKinds[r.Kind] || r.W < minPreviewCell || r.W > maxPreviewCell || r.H < minPreviewCell || r.H > maxPreviewCell ||
				len(r.Items) < 1 || len(r.Items) > maxPreviewItems || len(r.Items)*r.W > p.W || sumH > p.H {
				t.Fatalf("ряд: %+v", r)
			}
			for _, it := range r.Items {
				if !validItem(it) {
					t.Fatalf("предмет %q", it)
				}
			}
		}
	})
}

// entry030 — строка каталога версии 0.30: Entry без поля preview. Так читают каталог клиенты 0.17–0.30
// (поле needs — с 0.27, но для сравнения достаточно полного набора ключей строки).
type entry030 struct {
	ID      string `json:"id"`
	Version int    `json:"version"`
	MinApp  string `json:"minApp"`
	Title   string `json:"title"`
	About   string `json:"about"`
	License string `json:"license"`
	Size    int64  `json:"size"`
	SHA256  string `json:"sha256"`
	URL     string `json:"url"`
	Counts  Counts `json:"counts"`
	Needs   string `json:"needs,omitempty"`
}

// old030 — тело каталога так, как его читает клиент 0.30: Entry без preview.
type old030 struct {
	Format int        `json:"format"`
	Seq    int64      `json:"seq"`
	Packs  []entry030 `json:"packs"`
}

// oldBody — тело из трёх строк (у addon — needs) и ролика; preview, если не пуст, у каждой строки.
func oldBody(t *testing.T, src Source, preview string) []byte {
	t.Helper()
	var packs []map[string]any
	for _, id := range []string{"ivory", "addon", "plain"} {
		r := row(id, 1, src.Prefix+id+"-v1/"+id+"-1.zip", []byte("zip "+id))
		if id == "addon" {
			r["needs"] = "ivory"
		}
		if preview != "" {
			r["preview"] = json.RawMessage(preview)
		}
		packs = append(packs, r)
	}
	data, err := json.Marshal(map[string]any{"format": 1, "seq": 7, "packs": packs, "videos": []any{videoRowMap(src, "intro", 1)}})
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// sameButPreview — строки, ролики и номер каталога совпадают с телом без поля, кроме Preview; в Skipped
// только записи «превью».
func sameButPreview(cat, plain Catalog) string {
	if cat.Seq != plain.Seq || len(cat.Packs) != len(plain.Packs) || !reflect.DeepEqual(cat.Videos, plain.Videos) {
		return "номер, число наборов или videos"
	}
	for i, e := range cat.Packs {
		e.Preview = nil
		if !reflect.DeepEqual(e, plain.Packs[i]) {
			return "строка " + strconv.Itoa(i) + ": " + e.ID
		}
	}
	for _, s := range cat.Skipped {
		if !strings.HasPrefix(s.Why, "превью: ") {
			return "лишний Skipped: " + s.ID + ": " + s.Why
		}
	}
	return ""
}

// Старый клиент (Entry без preview) читает тело с любым preview — годным, битым по типам, в 8 КиБ и
// больше — без ошибки и получает строки, как у тела без поля. Новый ParseCatalog: строки совпадают во
// всём, кроме Preview; формат остаётся 1, лишний Skipped — только «превью».
func TestCatalogOldClientIgnoresPreview(t *testing.T) {
	src := newTestLib(t).source()
	good := pvText(t, previewMap(src))
	pad := func(n int) string { return good[:len(good)-1] + `,"pad":"` + strings.Repeat("x", n) + `"}` }
	plain := oldBody(t, src, "")
	plainCat, err := ParseCatalog(plain, src)
	if err != nil || len(plainCat.Packs) != 3 || len(plainCat.Skipped) != 0 || len(plainCat.Videos) != 1 || plainCat.Packs[1].Needs != "ivory" {
		t.Fatalf("тело без preview: %v %+v", err, plainCat)
	}
	var oldPlain old030
	if err := json.Unmarshal(plain, &oldPlain); err != nil || len(oldPlain.Packs) != 3 {
		t.Fatalf("старый клиент, тело без preview: %v", err)
	}
	for name, preview := range map[string]string{
		"годное":         good,
		"w строкой":      `{"url":"x","sha256":"y","size":10,"w":"240","h":160,"rows":[{"items":[1]}]}`,
		"rows объектом":  `{"rows":{}}`,
		"список":         `[1,"a",null]`,
		"число":          `42`,
		"строка":         `"preview"`,
		"null":           `null`,
		"ровно 8 КиБ":    pad(MaxPreviewRaw - len(good) - 9),
		"8 КиБ и больше": pad(MaxPreviewRaw),
		"в 20 раз более": pad(MaxPreviewRaw * 20),
	} {
		data := oldBody(t, src, preview)
		var old old030
		if err := json.Unmarshal(data, &old); err != nil || !reflect.DeepEqual(old, oldPlain) {
			t.Errorf("%s: старый клиент: %v %+v", name, err, old.Packs)
		}
		cat, err := ParseCatalog(data, src)
		if err != nil {
			t.Errorf("%s: ParseCatalog: %v", name, err)
			continue
		}
		if why := sameButPreview(cat, plainCat); why != "" {
			t.Errorf("%s: отличается не только Preview: %s", name, why)
		}
		if name == "годное" && (cat.Packs[0].Preview == nil || cat.Packs[1].Preview == nil || len(cat.Skipped) != 0) {
			t.Errorf("годное не прочитано: %+v", cat)
		}
	}
	if Format != 1 {
		t.Errorf("формат каталога %d, ждали 1", Format)
	}
}
