package roundtrip_test

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/library"
)

// Формат v19 (общий на 0.28): заготовки texts[].draft, макет look и spreads[].lookStep (131).

// v18Bytes — файл v18 (два листа «Авто», подрезка 10 %): Encode текущей версии с номером 18.
func v18Bytes(t *testing.T, base string) []byte {
	t.Helper()
	data, err := salbum.Encode(cropGolden(base, library.IDFor), base)
	if err != nil {
		t.Fatal(err)
	}
	out := bytes.Replace(data, verTag(model.FormatVersion), verTag(18), 1)
	for _, k := range []string{`"look"`, `"lookStep"`, `"draft"`} {
		if bytes.Equal(out, data) || bytes.Contains(data, []byte(k)) {
			t.Fatalf("v18 из Encode не вышел (%s):\n%s", k, data)
		}
	}
	return out
}

// lookUp — правка, которая кладёт макет: оба листа с фоном и шагами 3 и 4, на каждом заготовка,
// на втором ещё обычная надпись.
func lookUp(p *model.Project) {
	p.Look = &model.LookRef{Key: "my:00112233445566ff", Name: "Классика", Pack: "wedding-layouts", Item: "layouts/classic.json"}
	for i, s := range p.Spreads {
		s.LookStep = 3 + i
		s.BgL = "#ffeeee"
		s.Texts = append(s.Texts, &model.TextBlock{ID: model.NewID(), X: 20, Y: 20, W: 60, Auto: true, Text: "[Имена]", Style: model.PlainStyle(), Draft: true})
	}
	s := p.Spreads[1]
	s.Texts = append(s.Texts, &model.TextBlock{ID: model.NewID(), X: 20, Y: 60, W: 60, Auto: true, Text: "Свадьба", Style: model.PlainStyle()})
}

// v18 на диске: Load → Save делает копию «<имя> (для 0.27).salbum» с байтами исходника и пишет
// v19 без новых ключей; потом макет и заготовки переживают Save → Load, повторная запись — те же
// байты; копия не трогается.
func TestMigrateV18(t *testing.T) {
	withAppVersion(t, "0.28.0")
	base := albumSetup(t)
	src := v18Bytes(t, base)
	path := filepath.Join(base, "Свадьба.salbum")
	if err := os.WriteFile(path, src, 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := salbum.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if p.Version != model.FormatVersion || edtest.From(p) != 18 || p.Look != nil || p.Settings.Crop != 10 {
		t.Fatalf("версия %d, from %d, макет %+v, crop %d", p.Version, edtest.From(p), p.Look, p.Settings.Crop)
	}
	e := edtest.Open(t, p, path)
	if !edtest.Backup(e) {
		t.Fatal("до записи копия нужна, но не сделана")
	}
	before := model.Marshal(p)
	if err := e.Save(path); err != nil {
		t.Fatal(err)
	}
	bak := filepath.Join(base, "Свадьба (для 0.27).salbum")
	if got, err := os.ReadFile(bak); err != nil || !bytes.Equal(got, src) || bak != salbum.BackupName(path, 18, 1) {
		t.Fatalf("копия для 0.27: %v, байты исходника: %v", err, bytes.Equal(got, src))
	}
	if f := fileJSON(t, path); f.Version != 19 || f.Look != nil {
		t.Fatalf("в файле версия %d, макет %+v", f.Version, f.Look)
	}
	q, err := salbum.Load(path)
	if err != nil || !bytes.Equal(before, model.Marshal(q)) || edtest.From(q) != 19 {
		t.Fatalf("потери при v18 → v19: %v\n%s\n%s", err, before, model.Marshal(q))
	}

	// Макет и заготовки: Save → Load без потерь, вторая запись — те же байты.
	e2 := edtest.Open(t, q, path)
	edtest.Change(e2, lookUp)
	if err := e2.Save(path); err != nil {
		t.Fatal(err)
	}
	first, _ := os.ReadFile(path)
	for _, k := range []string{`"lookStep": 3`, `"lookStep": 4`, `"draft": true`, `"key": "my:00112233445566ff"`, `"item": "layouts/classic.json"`} {
		if !bytes.Contains(first, []byte(k)) {
			t.Fatalf("в файле нет %s", k)
		}
	}
	r, err := salbum.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(model.Marshal(edtest.P(e2)), model.Marshal(r)) {
		t.Fatalf("потери при Save → Load:\n%s\n%s", model.Marshal(edtest.P(e2)), model.Marshal(r))
	}
	if r.Look == nil || r.Spreads[0].LookStep != 3 || r.Spreads[1].LookStep != 4 || len(r.Spreads[1].Texts) != 2 ||
		!r.Spreads[1].Texts[0].Draft || r.Spreads[1].Texts[1].Draft {
		t.Fatalf("после чтения: макет %+v, листы %+v", r.Look, r.Spreads)
	}
	if err := edtest.Open(t, r, path).Save(path); err != nil {
		t.Fatal(err)
	}
	if again, _ := os.ReadFile(path); !bytes.Equal(first, again) {
		t.Fatalf("повторная запись изменила файл:\n%s\n%s", first, again)
	}
	if got, _ := os.ReadFile(bak); !bytes.Equal(got, src) || e2.TakeBackup() != "" {
		t.Fatal("копия для 0.27 изменилась или названа снова")
	}
}

// fuzzLook — макет после Load (131 design §3): ссылка зажата, шаг 0…MaxLookSteps, без макета
// и у обложки 0, лист с шагом оформлен (FixLook), заготовка — только у непустого текста;
// Load∘Save∘Load оставляет ссылку, шаги и заготовки как после первого Load.
func fuzzLook(t *testing.T, p, q *model.Project) {
	t.Helper()
	if !reflect.DeepEqual(model.ClampLookRef(p.Look), p.Look) {
		t.Fatalf("макет не зажат: %+v", p.Look)
	}
	if c := p.Cover; c != nil && c.Sheet != nil && c.Sheet.LookStep != 0 {
		t.Fatalf("шаг у обложки: %d", c.Sheet.LookStep)
	}
	for _, s := range p.Spreads {
		if s.LookStep < 0 || s.LookStep > model.MaxLookSteps || p.Look == nil && s.LookStep != 0 {
			t.Fatalf("шаг %d при макете %+v", s.LookStep, p.Look)
		}
	}
	fix := p.Clone()
	fix.FixLook()
	if !bytes.Equal(model.Marshal(fix), model.Marshal(p)) {
		t.Fatal("FixLook после Load меняет проект")
	}
	if !reflect.DeepEqual(q.Look, p.Look) {
		t.Fatalf("макет после записи и чтения: %+v, ждали %+v", q.Look, p.Look)
	}
	qs := q.Sheets()
	for i, s := range p.Sheets() {
		if i >= len(qs) || qs[i].LookStep != s.LookStep || len(qs[i].Texts) != len(s.Texts) {
			t.Fatalf("лист %d после записи и чтения", i)
		}
		for j, tb := range s.Texts {
			if tb.Draft && strings.TrimSpace(tb.Text) == "" || qs[i].Texts[j].Draft != tb.Draft {
				t.Fatalf("заготовка %+v, после чтения %+v", tb, qs[i].Texts[j])
			}
		}
	}
}
