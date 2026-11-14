package app

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

// Темы наборов в library-verify (спека 170, design §6): набор ставится как из каталога (OriginNet), проверяются
// объявления, topic и таблица программы; печатается «тема → бг / ст / рам / разв / макет».

// themedItems — items манифеста для файлов набора: theme — база, topic — id темы набора по имени файла ("" — нет).
func themedItems(files []pfile, theme map[string]string, topic map[string]any) []map[string]any {
	items := make([]map[string]any, 0, len(files))
	for _, f := range files {
		sum := sha256.Sum256(f.data)
		th := theme[f.name]
		if th == "" {
			th = "basic"
		}
		it := map[string]any{"file": f.name, "kind": f.kind, "theme": th, "name": f.name, "sha256": hex.EncodeToString(sum[:]), "size": len(f.data)}
		if tp, ok := topic[f.name]; ok {
			it["topic"] = tp
		}
		items = append(items, it)
	}
	return items
}

func decl(id, name, base string) map[string]any {
	return map[string]any{"id": id, "name": name, "base": base, "order": 10}
}

// themedPack — набор id с темами: фон, стикер и рамка в «Новом годе», шаблоны — в базе.
func themedPack(t *testing.T, id string, files []pfile, themes []map[string]any, topic map[string]any) string {
	t.Helper()
	items := themedItems(files, map[string]string{"bg/sky.jpg": "holiday", "stickers/dot.png": "holiday", "frames/plain.png": "holiday"}, topic)
	return patchManifest(t, packZip(t, id, 1, files...), map[string]any{"themes": themes, "items": items})
}

var ny = map[string]any{"bg/sky.jpg": "newyear", "stickers/dot.png": "newyear", "frames/plain.png": "newyear"}

func TestLibraryVerifyThemes(t *testing.T) {
	g := buildLibVerify(t)
	files, _, _ := lvGood(t)

	t.Run("годный набор: объявление принято, таблица напечатана", func(t *testing.T) {
		text, code := g.run(t, themedPack(t, "holiday-newyear", files, []map[string]any{decl("newyear", "Новый год", "holiday")}, ny))
		if code != 0 || !strings.HasSuffix(strings.TrimSpace(text), "OK") {
			t.Fatalf("код %d\n%s", code, text)
		}
		for _, want := range []string{"темы набора (тема → бг / ст / рам / разв / макет)", "Новый год (newyear) → 1 / 1 / 1 / 0 / 0"} {
			if !strings.Contains(text, want) {
				t.Errorf("нет %q\n%s", want, text)
			}
		}
	})

	t.Run("набор без тем: как раньше, строки тем — по базам", func(t *testing.T) {
		text, code := g.run(t, packZip(t, "synth", 1, files...))
		if code != 0 || !strings.Contains(text, "Универсальные (basic) → 1 / 1 / 1 / 2 / 0") {
			t.Fatalf("код %d\n%s", code, text)
		}
	})

	t.Run("все провалы за один прогон: непринятое объявление, topic в базу, чужой topic", func(t *testing.T) {
		themes := []map[string]any{decl("newyear", "Праздник", "holiday"), decl("holiday", "Лето", "basic")} // имя встроенной строки; встроенный id
		topic := map[string]any{"bg/sky.jpg": "newyear", "stickers/dot.png": "ghost", "frames/plain.png": 7}
		text, code := g.run(t, themedPack(t, "bad-themes", files, themes, topic))
		if code != 1 {
			t.Fatalf("код %d\n%s", code, text)
		}
		for _, want := range []string{"объявления тем не приняты: набор \"bad-themes\": темы пропущены: 2 (имя, id)", "topic \"ghost\" у «stickers/dot.png» сброшен в базу", "topic \"newyear\" у «bg/sky.jpg» сброшен в базу", "topic 7 у «frames/plain.png» сброшен в базу"} {
			if !strings.Contains(text, want) {
				t.Errorf("нет провала %q\n%s", want, text)
			}
		}
	})

	t.Run("два набора: одна тема — строка одна; имя-двойник под другим id — не в таблице", func(t *testing.T) {
		a := themedPack(t, "holiday-a", files, []map[string]any{decl("newyear", "Новый год", "holiday")}, ny)
		same := themedPack(t, "holiday-b", files, []map[string]any{decl("newyear", "Новогодний", "holiday")}, ny)
		text, code := g.run(t, a, same)
		if code != 1 || !strings.Contains(text, `тема "newyear" уже объявлена другим набором с именем «Новый год», а здесь «Новогодний»`) {
			t.Fatalf("тот же id, другое имя: код %d\n%s", code, text)
		}
		twin := themedPack(t, "holiday-c", files, []map[string]any{decl("ny", "НОВЫЙ ГОД", "holiday")}, map[string]any{"bg/sky.jpg": "ny", "stickers/dot.png": "ny", "frames/plain.png": "ny"})
		text, code = g.run(t, a, twin)
		if code != 1 || !strings.Contains(text, `тема "ny" не попала в таблицу программы`) {
			t.Fatalf("имя-двойник: код %d\n%s", code, text)
		}
	})

	t.Run("типография в наборе ставится как из сети и проверяется профилем", func(t *testing.T) {
		good := append([]pfile{{name: "prints/p.saprint", kind: "print", data: libProfile(t, "verify-lab", "Лаборатория проверки", 1)}}, files...)
		if text, code := g.run(t, packZip(t, "with-print", 1, good...)); code != 0 || strings.Contains(text, "не принимается") {
			t.Fatalf("годный профиль: код %d\n%s", code, text)
		}
		bad := append([]pfile{{name: "prints/p.saprint", kind: "print", data: []byte(`{"format":1}`)}}, files...)
		if text, code := g.run(t, packZip(t, "bad-print", 1, bad...)); code != 1 || !strings.Contains(text, "типография не принята") {
			t.Fatalf("негодный профиль: код %d\n%s", code, text)
		}
	})
}
