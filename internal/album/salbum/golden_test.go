package salbum_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/library"
)

// Голден альбома 0.10.0 (формат v6): обложка, виньетка листом и на половине разворота,
// поля шапки, стиль подписей листа, имена, отступ своей подписи, безопасная зона, 18 фото на
// развороте. Пути — от папки C:\album (файлов нет), поэтому тест только на Windows.
// Файл заморожен (решение 0.11 №29): не пересоздаётся. В 0.11 FormatVersion 7, поэтому сверка
// байт в байт — с исходником, где "version": 6 заменено на "version": 7 (больше v7 в нём ничего не меняет).

const goldenBase = `C:\album`

var golden010 = filepath.Join("testdata", "golden-0.10.salbum.json")

// TestGolden010 — файл 0.10 открывается как v7: Encode(Decode) = исходник с версией 7 байт в байт,
// ключевые поля 0.10 на месте, версия в памяти 7, исходная 6; v7 читается и пишется снова без изменений.
func TestGolden010(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("голден с путями C:\\album")
	}
	data, err := os.ReadFile(golden010)
	if err != nil {
		t.Fatal(err)
	}
	p, err := salbum.Decode(data, goldenBase)
	if err != nil {
		t.Fatal(err)
	}
	if p.Version != model.FormatVersion || edtest.From(p) != 6 || p.Settings.Safe != 5 || len(p.Spreads) != 3 || len(p.Spreads[0].Photos) < 17 {
		t.Fatalf("версия %d, from %d, safe %v, листов %d", p.Version, edtest.From(p), p.Settings.Safe, len(p.Spreads))
	}
	out, err := salbum.Encode(p, goldenBase)
	if err != nil {
		t.Fatal(err)
	}
	if want := bytes.Replace(data, []byte(`"version": 6,`), []byte(`"version": 7,`), 1); !bytes.Equal(out, want) {
		t.Fatalf("Load → Encode файла 0.10 отличается от исходника с версией 7:\n%s", out)
	}
	// v7 стабилен: чтение и запись снова не меняют байты.
	p2, err := salbum.Decode(out, goldenBase)
	if err != nil {
		t.Fatal(err)
	}
	out2, err := salbum.Encode(p2, goldenBase)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out, out2) || !bytes.Contains(out, []byte(`"version": 7`)) || edtest.From(p2) != model.FormatVersion {
		t.Fatalf("Save → Load → Save v7 не стабилен, from %d:\n%s", edtest.From(p2), out)
	}
	vig, half := p.Spreads[1], p.Spreads[2]
	if !vig.IsVignette() || vig.Vig.Tpl != "head-t-left" || vig.Vars == nil || vig.Vars.Title != "Школа №5" || vig.Vars.Lead == "" {
		t.Fatalf("виньетка листом: %+v %+v", vig.Vig, vig.Vars)
	}
	if c := vig.Caps; c == nil || c.Pad != 2 || !c.Wrap || c.Text != model.PupilCaption {
		t.Fatalf("стиль подписей листа: %+v", c)
	}
	if !half.IsVignette() || half.Vig.Tpl != "split-tg" || len(half.Grid) == 0 {
		t.Fatalf("виньетка на развороте: %+v", half.Vig)
	}
	roles := map[string]int{}
	for _, pl := range append(vig.Grid, half.Grid...) {
		roles[pl.Role]++
	}
	if roles[model.RoleTeacher] < 2 || roles[model.RolePupil] < 4 {
		t.Fatalf("роли мест: %v", roles)
	}
	ph := func(i int) model.PhotoRef { return p.Photos[library.IDFor(goldenPhoto(i))] }
	if who := ph(18).Person; who == nil || who.Last != "Петрова" || who.Middle != "Ивановна" {
		t.Fatalf("имя: %+v", who)
	}
	if c := ph(0).Caption; c == nil || c.Text != "{Год}" || c.Pad == nil || *c.Pad != 2.5 || !c.Over {
		t.Fatalf("своя подпись: %+v", c)
	}
	c := p.Cover
	if c == nil || c.Layout != model.CoverBoth || c.SpineBg != "#203040" || len(c.Sheet.Grid) == 0 || len(c.Sheet.Texts) != 1 ||
		c.Guide != goldenBase+`\Разметка\c.png` || c.GuideAlt != `D:\Старая\c.png` {
		t.Fatalf("обложка: %+v", c)
	}
	for _, s := range [][]byte{[]byte(`"guideAlt"`), []byte(`"alt"`), []byte("\n  \"vars\"")} {
		if bytes.Contains(data, s) {
			t.Fatalf("в файле поле памяти %s", s)
		}
	}
}

// goldenPhoto — путь i-го фото голдена.
func goldenPhoto(i int) string {
	return filepath.Join(goldenBase, "Отбор", fmt.Sprintf("%02d.jpg", i))
}
