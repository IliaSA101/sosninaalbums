package app

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum/collect"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
	"github.com/IliaSA101/sosninaalbums/internal/text"
)

// Свои шрифты на выходе (082 G4): вопрос перед выгрузкой, сбор в папку «Шрифты», сводка окна сбора.

// withTexts — редактор того же альбома с текстами по листам: texts[i] — блоки листа i.
func withTexts(t *testing.T, a *App, path string, texts [][]*model.TextBlock) {
	t.Helper()
	p, _ := a.editor().Snapshot()
	for i, list := range texts {
		p.Spreads[i].Texts = list
	}
	p.SyncFonts()
	a.ed.Store(mustEditor(t, p, path))
}

// TestExportAsksMissingFonts — тексты своими гарнитурами, которых нет на этом компьютере: вопрос
// до выбора папки с названиями (без названия — «Свой шрифт»), только по листам плана и непустым
// текстам; «нет» — выгрузки нет; шрифт загрузили — его в вопросе нет; все на месте — вопроса нет.
func TestExportAsksMissingFonts(t *testing.T) {
	e := newExportEnv(t, 3)
	t.Cleanup(func() { text.SetCustom(nil, nil) })
	e.a.openFonts()
	osw, oswName := familyOf(t, "Oswald-Regular.ttf")
	unnamed, quiet := text.FamilyID("Без названия"), text.FamilyID("Пусто")
	empty := fontBlock(quiet, "0a0b0c0d0e03", 50)
	empty.Text = " "
	withTexts(t, e.a, "", [][]*model.TextBlock{{fontBlock(osw, "0a0b0c0d0e01", 100)},
		{fontBlock(unnamed, "0a0b0c0d0e02", 100), empty}})
	if p, _ := e.a.editor().Snapshot(); p.Fonts[osw].Name != "" || len(p.Spreads[1].Texts) != 2 {
		t.Fatalf("альбом: %+v, текстов %d", p.Fonts, len(p.Spreads[1].Texts))
	}
	// Название с другого компьютера — из таблицы альбома.
	p, _ := e.a.editor().Snapshot()
	p.Fonts[osw] = model.FontRef{Name: oswName}
	e.a.ed.Store(mustEditor(t, p, ""))
	picked := 0
	pick := pickExportDir
	pickExportDir = func(a *App, title string) (string, error) { picked++; return pick(a, title) }
	ask := func(name string) string {
		return "В выгрузке нет шрифтов: " + name + ". Текст будет шрифтом Montserrat. Выгрузить так?"
	}

	e.answer = false
	ok, err := e.a.Export(ExportOptions{DPI: 72})
	if ok || err != nil || picked != 0 || !slices.Equal(e.asked, []string{ask("„" + oswName + "“, „Свой шрифт“")}) {
		t.Fatalf("«нет»: %v %v, папок %d, вопросы %q", ok, err, picked, e.asked)
	}
	if ents, _ := os.ReadDir(e.dir); len(ents) != 0 {
		t.Fatalf("после «нет» записано %d файлов", len(ents))
	}
	e.asked = nil
	if ok, _ := e.a.Export(ExportOptions{DPI: 72, Range: "2"}); ok || !slices.Equal(e.asked, []string{ask("„Свой шрифт“")}) {
		t.Fatalf("лист 2: %q", e.asked)
	}
	e.asked = nil
	if ok, _ := e.a.Export(ExportOptions{DPI: 72, Range: "3"}); !ok || len(e.asked) != 0 {
		t.Fatalf("лист без текстов спросил: %q", e.asked)
	}
	<-e.done

	// Oswald загрузили — в вопросе его нет; «да» — выгрузка идёт.
	if _, err := e.a.ImportFontPaths([]string{writeFont(t, "o.ttf", "Oswald-Regular.ttf")}); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(e.dir); err != nil { // без вопроса о замене прежней выгрузки
		t.Fatal(err)
	}
	e.answer, e.asked = true, nil
	if msg := e.export(t, ExportOptions{}); msg != "" || !slices.Equal(e.asked, []string{ask("„Свой шрифт“")}) {
		t.Fatalf("Oswald есть: %q, вопросы %q", msg, e.asked)
	}
	e.asked = nil
	if msg := e.export(t, ExportOptions{Range: "1"}); msg != "" || len(e.asked) != 1 || strings.Contains(e.asked[0], "шрифт") {
		t.Fatalf("все на месте: %q, вопросы %q", msg, e.asked)
	}
}

// writeFont — встроенное начертание builtin файлом name во временной папке теста.
func writeFont(t *testing.T, name, builtin string) string {
	t.Helper()
	p := filepath.Join(testkit.TempDir(t), name)
	if err := os.WriteFile(p, fontBytes(t, builtin), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// fontCollect — альбом сбора: на листе 1 — Oswald (два начертания) и гарнитура, которой нет на
// этом компьютере, на листе 2 — Lora с запретом переноса в файле.
type fontCollect struct {
	*collectEnv
	osw, oswName, lora, loraName, gone string
	regular, bold                      []byte // файлы Oswald
}

func newFontCollect(t *testing.T) *fontCollect {
	t.Helper()
	f := &fontCollect{collectEnv: newCollectEnv(t, 2)}
	t.Cleanup(func() { text.SetCustom(nil, nil) })
	f.a.openFonts()
	f.osw, f.oswName = familyOf(t, "Oswald-Regular.ttf")
	f.lora, f.loraName = familyOf(t, "Lora-Regular.ttf")
	f.gone = text.FamilyID("Bodoni Moda")
	f.regular, f.bold = fontBytes(t, "Oswald-Regular.ttf"), fontBytes(t, "Oswald-Bold.ttf")
	src := testkit.TempDir(t)
	var files []string
	for name, b := range map[string][]byte{"o.ttf": f.regular, "ob.ttf": f.bold, "l.ttf": restricted(t, fontBytes(t, "Lora-Regular.ttf"))} {
		p := filepath.Join(src, name)
		if err := os.WriteFile(p, b, 0o644); err != nil {
			t.Fatal(err)
		}
		files = append(files, p)
	}
	if res, err := f.a.ImportFontPaths(files); err != nil || len(res.Added) != 2 {
		t.Fatalf("загрузка: %v %+v", err, res)
	}
	withTexts(t, f.a, f.file, [][]*model.TextBlock{{fontBlock(f.osw, "0a0b0c0d0e01", 100), fontBlock(f.gone, "0a0b0c0d0e02", 100)},
		{fontBlock(f.lora, "0a0b0c0d0e03", 100)}})
	return f
}

// TestCollectCopiesFonts — сбор кладёт файлы гарнитур альбома в «Шрифты» под понятными именами,
// собранный альбом несёт таблицу шрифтов без путей; на чистой библиотеке он добавляет Oswald сам.
func TestCollectCopiesFonts(t *testing.T) {
	f := newFontCollect(t)
	dir, msg, _, _ := f.run(t, false)
	if msg != "" {
		t.Fatal(msg)
	}
	fonts := filepath.Join(dir, collect.DirFonts)
	ents, err := os.ReadDir(fonts)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range ents {
		names = append(names, e.Name())
	}
	want := []string{f.oswName + "-Bold.ttf", f.oswName + "-Regular.ttf"}
	if !slices.Equal(names, want) {
		t.Fatalf("в «Шрифтах»: %v, ждали %v", names, want)
	}
	for name, b := range map[string][]byte{want[0]: f.bold, want[1]: f.regular} {
		if got, _ := os.ReadFile(filepath.Join(fonts, name)); !bytes.Equal(got, b) {
			t.Errorf("%s: содержимое не то", name)
		}
	}
	raw, err := os.ReadFile(f.albumFile())
	if err != nil {
		t.Fatal(err)
	}
	var file struct {
		Fonts map[string]model.FontRef `json:"fonts"`
	}
	if err := json.Unmarshal(raw, &file); err != nil {
		t.Fatal(err)
	}
	if len(file.Fonts) != 3 || file.Fonts[f.osw].Name != f.oswName || file.Fonts[f.lora].Name != f.loraName {
		t.Fatalf("таблица шрифтов собранного: %+v", file.Fonts)
	}
	if bytes.Contains(raw, []byte(".ttf")) || bytes.Contains(raw, []byte(filepath.Base(f.a.cfgDir))) {
		t.Fatal("в собранном альбоме путь шрифта")
	}

	// Другой компьютер: чистая библиотека — Oswald добавлен из папки альбома, Lora (не в папке) — нет.
	text.SetCustom(nil, nil)
	g := newFontEnv(t, nil)
	st, err := g.a.openPath(f.albumFile())
	if err != nil || st.Info != "Добавлены шрифты из папки альбома: „"+f.oswName+"“" || text.Available(f.lora) {
		t.Fatalf("открытие собранного: %v %q, Lora %v", err, st.Info, text.Available(f.lora))
	}
}

// TestCollectSkipsRestricted — файл с запретом переноса не копируется, его гарнитура — в сводке
// окна; гарнитура без файла на этом компьютере в папку не попадает и считается отдельно.
func TestCollectSkipsRestricted(t *testing.T) {
	f := newFontCollect(t)
	in := f.a.CollectInfo(false)
	if in.Fonts != 2 || !slices.Equal(in.FontsRestricted, []string{f.loraName}) || in.MissingFonts != 1 {
		t.Fatalf("сводка: %+v", in)
	}
	dir, msg, _, _ := f.run(t, false)
	if msg != "" {
		t.Fatal(msg)
	}
	ents, _ := os.ReadDir(filepath.Join(dir, collect.DirFonts))
	for _, e := range ents {
		if strings.HasPrefix(e.Name(), f.loraName) {
			t.Fatalf("скопирован шрифт с запретом переноса: %s", e.Name())
		}
	}
	lp, err := salbum.Load(f.albumFile())
	if err != nil || lp.Spreads[1].Texts[0].Style.Font != f.lora {
		t.Fatalf("гарнитура Lora в собранном: %v", err)
	}
}

// TestCollectInfoFonts — сводка окна сбора: файлы шрифтов к копированию, запрещённые и
// отсутствующие гарнитуры; без своих шрифтов — нули и пустой список (не null для окна).
func TestCollectInfoFonts(t *testing.T) {
	e := newCollectEnv(t, 1)
	in := e.a.CollectInfo(false)
	if in.Fonts != 0 || in.FontsRestricted == nil || len(in.FontsRestricted) != 0 || in.MissingFonts != 0 {
		t.Fatalf("без шрифтов: %+v", in)
	}
	if raw, _ := json.Marshal(in); !bytes.Contains(raw, []byte(`"fontsRestricted":[]`)) {
		t.Fatalf("JSON: %s", raw)
	}
	// Библиотеки нет (startup не дошёл): все свои — «нет на этом компьютере».
	withTexts(t, e.a, e.file, [][]*model.TextBlock{{fontBlock(text.FamilyID("A"), "0a0b0c0d0e01", 50),
		fontBlock(text.FamilyID("B"), "0a0b0c0d0e02", 50)}})
	if in := e.a.CollectInfo(false); in.Fonts != 0 || in.MissingFonts != 2 {
		t.Fatalf("без библиотеки: %+v", in)
	}

	f := newFontCollect(t)
	in = f.a.CollectInfo(true)
	if in.Fonts != 2 || len(in.FontsRestricted) != 1 || in.MissingFonts != 1 || in.Photos != 2 {
		t.Fatalf("со шрифтами: %+v", in)
	}
	// Начертание пропало из библиотеки — его нет и в сводке.
	lib := f.a.fontLib.Load()
	faces := lib.Faces(f.osw)
	if err := os.Remove(faces[1].Path); err != nil {
		t.Fatal(err)
	}
	if in := f.a.CollectInfo(false); in.Fonts != 1 {
		t.Fatalf("пропавший файл: %+v", in)
	}
}
