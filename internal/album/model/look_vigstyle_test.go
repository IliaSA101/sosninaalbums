package model

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"
)

// Оформление шага-виньетки (164): зажим style и htext, совместимость с читателем 0.35.

// vsFile — файл макета: обычный шаг и шаг-виньетка с vig (JSON тела "vig").
func vsFile(vig string) []byte {
	return []byte(`{"v":1,"steps":[{"tpl":{"v":1,"pages":1,"ratio":1,"page":[{}],"places":[{"x":0.1,"y":0.1,"w":0.5,"h":0.5}]}},` +
		`{"role":"vignette","vig":` + vig + `,"tpl":{"v":1,"pages":2,"page":[{},{}]}}]}`)
}

func vsVig(t *testing.T, vig string) *LookVig {
	t.Helper()
	lt, err := ParseLook(vsFile(vig))
	if err != nil || len(lt.Steps) != 2 || lt.Steps[1].Vig == nil {
		t.Fatalf("разбор: %v, %+v", err, lt)
	}
	return lt.Steps[1].Vig
}

// style и htext проходят зажимы: у шапки и подписей остаются гарнитура и цвет, кегль 0, прочее
// обнуляется; «my:…» и неизвестный шрифт — умолчание; плохой цвет — «не менять»; htext — ≤ 3 строк.
func TestLookVigFieldsClamp(t *testing.T) {
	v := vsVig(t, `{"tpl":"split-g","teachers":6,"pupils":0,"htext":"Наши воспитатели","style":{`+
		`"head":{"font":"marmelad","color":"#C0563A","size":99,"bold":true,"align":"left"},`+
		`"caps":{"style":{"font":"nunito","color":"#5a4a42","size":40,"italic":true},"pad":9,"over":true,"wrap":true,"text":"x"},`+
		`"ttext":"{Фамилия}\n{Имя}","shape":"oval","round":5}}`)
	want := &VigStyle{Head: &TextStyle{Font: "marmelad", Color: "#c0563a"},
		Caps: &SheetCaps{Style: TextStyle{Font: "nunito", Color: "#5a4a42"}}, TText: "{Фамилия}\n{Имя}"}
	if v.HText != "Наши воспитатели" || v.Style == nil || *v.Style.Head != *want.Head || *v.Style.Caps != *want.Caps || v.Style.TText != want.TText ||
		v.Style.Shape != "" || v.Style.Round != 0 {
		t.Fatalf("зажим: %+v / %+v", v, v.Style)
	}
	cases := []struct {
		name, style string
		font, col   string // ждём у Caps.Style
	}{
		{"my", `{"font":"my:0123456789abcdef","color":"#112233"}`, "montserrat", "#112233"},
		{"unknown", `{"font":"нет-такого","color":"zzz"}`, "montserrat", ""},
		{"empty", `{"color":"#ffffff"}`, "", "#ffffff"},
		{"big-size", `{"font":"lora","size":1e308}`, "lora", ""},
	}
	for _, c := range cases {
		got := vsVig(t, `{"tpl":"split-g","teachers":1,"pupils":1,"style":{"caps":{"style":`+c.style+`}}}`)
		if got.Style == nil || got.Style.Caps == nil || got.Style.Caps.Style.Font != c.font || got.Style.Caps.Style.Color != c.col || got.Style.Caps.Style.Size != 0 {
			t.Errorf("%s: %+v", c.name, got.Style)
		}
	}
	// Пустое после зажима — nil: шаг как в 0.35 (и круг записи его не меняет).
	for _, s := range []string{`{}`, `{"caps":null,"head":null}`, `{"ttext":"  \n "}`, `{"shape":"oval","round":3}`, `{"head":{}}`, `{"caps":{}}`, `{"caps":{"style":{"size":20,"bold":true}},"head":{"align":"left"}}`} {
		if got := vsVig(t, `{"tpl":"split-g","teachers":1,"pupils":1,"style":`+s+`}`); got.Style != nil {
			t.Errorf("style %s: %+v", s, got.Style)
		}
	}
	// htext: края.
	for in, want := range map[string]string{
		`""`: "", `"   "`: "", `"a\nb\nc\nd\ne"`: "a\nb\nc", `"\u202eX\u0000"`: "X",
	} {
		if got := vsVig(t, `{"tpl":"split-g","teachers":1,"pupils":1,"htext":`+in+`}`).HText; got != want {
			t.Errorf("htext %.20s: %q, ждали %q", in, got, want)
		}
	}
	// Огромный текст шапки и подписи: предел знаков, а не ошибка.
	got := vsVig(t, `{"tpl":"split-g","teachers":1,"pupils":1,"htext":"`+strings.Repeat("ы", 1<<16)+`","style":{"ttext":"`+strings.Repeat("ы", 1<<16)+`"}}`)
	if utf8.RuneCountInString(got.HText) != MaxCaptionRunes || utf8.RuneCountInString(got.Style.TText) != MaxCaptionRunes {
		t.Errorf("пределы: htext %d, ttext %d знаков", utf8.RuneCountInString(got.HText), utf8.RuneCountInString(got.Style.TText))
	}
}

// oldLookVig — LookVig читателя 0.35 (четыре поля): файл с новыми полями он разбирает так же, как без них.
type oldLookVig struct {
	Tpl      string `json:"tpl"`
	Teachers int    `json:"teachers"`
	Pupils   int    `json:"pupils"`
	Ratio    string `json:"ratio,omitempty"`
}

// Файл без новых полей даёт тот же ClampLook, что и копия LookVig 0.35; файл с полями — те же четыре
// поля у читателя 0.35 (неизвестные пропускаются), ключ — от байтов файла.
func TestParseLookOldReader(t *testing.T) {
	plain := `{"tpl":"split-g","teachers":6,"pupils":0,"ratio":"3:4"}`
	rich := `{"tpl":"split-g","teachers":6,"pupils":0,"ratio":"3:4","htext":"Шапка","style":{"ttext":"x"}}`
	a, err := ParseLook(vsFile(plain))
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range []string{plain, rich} {
		var old struct {
			Steps []struct {
				Vig *oldLookVig `json:"vig"`
			} `json:"steps"`
		}
		if err := json.Unmarshal(vsFile(v), &old); err != nil {
			t.Fatal(err)
		}
		if got := *old.Steps[1].Vig; got != (oldLookVig{Tpl: "split-g", Teachers: 6, Ratio: "3:4"}) {
			t.Errorf("читатель 0.35: %+v", got)
		}
	}
	if v := a.Steps[1].Vig; v.Style != nil || v.HText != "" {
		t.Errorf("файл без полей: %+v", v)
	}
	b, err := json.Marshal(a)
	if err != nil || bytes.Contains(b, []byte("htext")) || bytes.Contains(b, []byte(`"style"`)) {
		t.Errorf("поля попали в запись пустого шага: %s", b)
	}
	if LookKey(vsFile(plain)) == LookKey(vsFile(rich)) {
		t.Error("ключ не зависит от байтов файла")
	}
}

// checkLookVigStyle — свойства принятого макета: стиль шагов-виньеток уже зажат (повторный зажим
// ничего не меняет), кегль и поля подписей нулевые, текст шапки в пределах.
func checkLookVigStyle(t *testing.T, lt AlbumTpl) {
	t.Helper()
	for _, st := range lt.Steps {
		if st.Vig == nil {
			continue
		}
		if s := st.Vig.Style; s != nil {
			again := clampLookVigStyle(s)
			a, _ := json.Marshal(s)
			b, _ := json.Marshal(again)
			if again == nil || !bytes.Equal(a, b) {
				t.Fatalf("повторный зажим менял стиль: %s / %s", a, b)
			}
			if s.Shape != "" || s.Round != 0 || s.Head != nil && s.Head.Size != 0 ||
				s.Caps != nil && (s.Caps.Style.Size != 0 || s.Caps.Pad != 0 || s.Caps.Over || s.Caps.Wrap || s.Caps.Text != "") {
				t.Fatalf("в стиле шага остались лишние поля: %s", a)
			}
		}
		if n := utf8.RuneCountInString(st.Vig.HText); n > MaxCaptionRunes || strings.Count(st.Vig.HText, "\n") >= MaxLookHeadLines {
			t.Fatalf("htext вне предела: %d знаков", n)
		}
	}
}
