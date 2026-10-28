package roundtrip_test

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
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Обрез и разметка разворотов (спека 066), формат v11.

// v10 (0.14) открывается как v11 без потерь: меняется только номер, первая запись делает
// .v10.bak с байтами исходника; файл новее v11 — ErrNewer. v11 общий на 0.15: у того же
// layflat-файла шаг 064 (pinFoldAuto) снимает FoldAny, поля 066 нулевые, в записи есть foldAny.
func TestVersionsV10ToV11(t *testing.T) {
	oldVersionOpens(t, 10)
	src, err := os.ReadFile(filepath.Join("..", "testdata", "v6.salbum.json"))
	if err != nil {
		t.Fatal(err)
	}
	dir := testkit.TempDir(t)
	p := edtest.MustDecode(t, bytes.Replace(src, []byte(`"version": 6`), []byte(`"version": 10`), 1), dir)
	if st := p.Settings; !st.Layflat || st.FoldAny || st.Bleed != 0 || p.SpreadGuide != "" || p.SpreadGuideAlt != "" {
		t.Fatalf("v10 → v11: layflat %v, foldAny %v, обрез %v, разметка %q %q", st.Layflat, st.FoldAny, st.Bleed, p.SpreadGuide, p.SpreadGuideAlt)
	}
	out, err := salbum.Encode(p, dir)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out, []byte(`"foldAny": false`)) || bytes.Contains(out, []byte(`"bleed"`)) || bytes.Contains(out, []byte(`"spreadGuide`)) {
		t.Fatalf("запись v11: ждали foldAny без обреза и разметки:\n%s", out)
	}
}

// TestGoldenV11Markup — голден v11: обрез, разметка разворотов и PSD-разметка обложки, обе
// с запасным путём (файлов нет ни там, ни там). Decode → Encode = исходник байт в байт, поля на
// месте, исходная версия 11.
func TestGoldenV11Markup(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("голден с путями C:\\album")
	}
	data, err := os.ReadFile(filepath.Join("..", "testdata", "v11-markup.salbum.json"))
	if err != nil {
		t.Fatal(err)
	}
	p, err := salbum.Decode(data, goldenBase)
	if err != nil {
		t.Fatal(err)
	}
	if p.Version != model.FormatVersion || edtest.From(p) != 11 || p.Settings.Bleed != 2 || p.Settings.Safe != 10 {
		t.Fatalf("версия %d, from %d, обрез %v", p.Version, edtest.From(p), p.Settings.Bleed)
	}
	if p.SpreadGuide != goldenBase+`\Разметка\spread_30x30.psd` || p.SpreadGuideAlt != `D:\Шаблоны\spread_30x30.psd` {
		t.Fatalf("разметка разворотов %q, запасной %q", p.SpreadGuide, p.SpreadGuideAlt)
	}
	if c := p.Cover; c == nil || c.Guide != goldenBase+`\Разметка\Cover_thin_30x30_1-5.psd` || c.GuideAlt != `D:\Шаблоны\Cover_thin_30x30_1-5.psd` {
		t.Fatalf("разметка обложки: %+v", c)
	}
	out, err := salbum.Encode(p, goldenBase)
	if err != nil {
		t.Fatal(err)
	}
	// Голден v11: запись — в текущей версии, остальное байт в байт.
	want := bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
	want = bytes.Replace(want, []byte(`"version": 11,`), fmt.Appendf(nil, `"version": %d,`, model.FormatVersion), 1)
	if !bytes.Equal(out, want) {
		t.Fatalf("Decode → Encode v11 отличается от голдена:\n%s", out)
	}
	if err := salbum.Check(p); err != nil {
		t.Fatal(err)
	}
	e := edtest.Open(t, p, "")
	if v := e.View(); len(v.Bleed) != 4 || v.SpreadMarkup == nil || v.SpreadMarkup.OK || v.Cover.Markup == nil {
		t.Fatalf("вид: обрез %+v, разметки %+v %+v", v.Bleed, v.SpreadMarkup, v.Cover.Markup)
	}
	if !bytes.Equal(model.Marshal(edtest.P(e)), model.Marshal(p)) {
		t.Fatal("Editor изменил проект при открытии")
	}
}
