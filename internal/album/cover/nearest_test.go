package cover_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/internal/edtest"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Обложка Фотосферы — размер шаблона, а не страницы (070 Е, decisions 0.16 №28): на нетабличной
// странице — ближайший формат; смена формата не смотрит на безопасные линии; обложка «из
// страницы» (W/H 0) сама не меняется и лечится «Исправить».

var fsferaPresets = []string{"fsfera-thin", "fsfera-card", "fsfera-double"}

// oldFile — файл старой версии (v10): Save → правка JSON без безопасных линий обложки →
// LoadBase → редактор. zeroSide — сторонка обложки «из страницы» (W/H 0), как в 0.10–0.11.
func oldFile(t *testing.T, e *album.Editor, zeroSide bool) *album.Editor {
	t.Helper()
	path := filepath.Join(testkit.TempDir(t), "v10.salbum")
	if err := e.Save(path); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	doc["version"] = 10
	spec := doc["cover"].(map[string]any)["spec"].(map[string]any)
	delete(spec, "safe")
	delete(spec, "spineSafe")
	if zeroSide {
		delete(spec, "w")
		delete(spec, "h")
	}
	if data, err = json.Marshal(doc); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	q, err := album.LoadBase(path, filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	return edtest.Open(t, q, path)
}

// TestFollowFormatIgnoresSafe — обложка v10 (числа шаблона, линий нет) при смене формата
// переходит на шаблон нового и получает его линии 8/12; один шаг отмены, Undo возвращает и
// страницу, и «обложку без линий»; Save → Load держит линии; все три типа листов.
func TestFollowFormatIgnoresSafe(t *testing.T) {
	for _, id := range fsferaPresets {
		t.Run(id, func(t *testing.T) {
			src := coverAlbum(t, 2)
			if _, err := src.ApplyCoverPreset(id); err != nil {
				t.Fatal(err)
			}
			e := oldFile(t, src, false)
			if c := spec(e); c.Preset != id || c.W != 286 || c.Safe != 0 || c.SpineSafe != 0 {
				t.Fatalf("v10 после Load: %+v", c)
			}
			undo := edtest.UndoLen(e)
			page(e, 201, 203)
			if c := spec(e); c.W != 206 || c.H != 206 || c.Safe != 8 || c.SpineSafe != 12 || c.Preset != id {
				t.Fatalf("v10 не перешла на шаблон 20×20: %+v", c)
			}
			if edtest.UndoLen(e) != undo+1 {
				t.Fatalf("шагов отмены %d, ждали 1", edtest.UndoLen(e)-undo)
			}
			path := filepath.Join(testkit.TempDir(t), "после.salbum")
			if err := e.Save(path); err != nil {
				t.Fatal(err)
			}
			if q, err := album.LoadBase(path, filepath.Dir(path)); err != nil || q.Cover.Spec.W != 206 || q.Cover.Spec.Safe != 8 || q.Cover.Spec.SpineSafe != 12 {
				t.Fatalf("Save → Load: %v %+v", err, q.Cover.Spec)
			}
			if e.Undo(); edtest.P(e).Settings.PageW != 281 || spec(e).W != 286 || spec(e).Safe != 0 {
				t.Fatalf("Undo: страница %v, обложка %+v", edtest.P(e).Settings.PageW, spec(e))
			}
		})
	}
}

// TestCoverNonTableTemplate — обложка, созданная на нетабличной странице, — шаблон ближайшего
// формата (не «из страницы»): переходит за форматом страницы, пока числа шаблонные; те же числа
// нового формата — холостой переход (обложка не меняется, лишних записей нет); 400×300 — снова 30×30.
func TestCoverNonTableTemplate(t *testing.T) {
	for _, id := range fsferaPresets {
		t.Run(id, func(t *testing.T) {
			e := coverAlbum(t, 2)
			page(e, 300, 300)
			edtest.Change(e, func(p *model.Project) { p.Cover = nil })
			if _, err := e.CreateCover(id, nil); err != nil {
				t.Fatal(err)
			}
			if c := spec(e); c.W != 286 || c.H != 286 || c.Wrap != 20 || c.Preset != id {
				t.Fatalf("новая обложка на 300×300: %+v", c)
			}
			if fix := e.View().Cover.Fix; fix != "" {
				t.Fatalf("новая обложка не по шаблону: %q", fix)
			}
			before := spec(e)
			page(e, 281, 284) // тот же формат 30×30 — числа те же
			if spec(e).W != before.W || spec(e).H != before.H || spec(e).Wrap != before.Wrap || e.View().Cover.Fix != "" {
				t.Fatalf("30×30 → 30×30: %+v", spec(e))
			}
			page(e, 200, 300) // нетабличная, ближайший 21×30
			if c := spec(e); c.W != 211 || c.H != 296 || c.Wrap != 16 || e.View().Cover.Fix != "" {
				t.Fatalf("200×300: %+v", c)
			}
			page(e, 400, 300)
			if c := spec(e); c.W != 286 || c.H != 286 || c.Wrap != 20 {
				t.Fatalf("400×300: %+v", c)
			}
		})
	}
}

// TestCoverOffTemplateNonTable — обложка «из страницы» (W/H 0) на 300×300 не шаблон: в карточке
// «Не по шаблону Фотосферы 30×30: … нужно 626 × 326»; смена формата её не двигает; «Исправить» —
// шаблон 286×286 одним шагом, повтор холостой, Undo возвращает «из страницы», Save → Load держит.
func TestCoverOffTemplateNonTable(t *testing.T) {
	for _, id := range fsferaPresets {
		t.Run(id, func(t *testing.T) {
			src := coverAlbum(t, 20) // 18–22 разворотов: корешок «Тонких листов» 14 мм, лист 626 × 326
			if _, err := src.ApplyCoverPreset(id); err != nil {
				t.Fatal(err)
			}
			page(src, 300, 300)
			e := oldFile(t, src, true)
			if c := spec(e); c.W != 0 || c.H != 0 || c.Preset != id {
				t.Fatalf("v10 «из страницы» после Load: %+v", c)
			}
			fix := e.View().Cover.Fix
			// у «Картона» и «Двойного» корешок другой — размер листа свой, но формат назван
			if !strings.HasPrefix(fix, "Не по шаблону Фотосферы 30×30: ") ||
				id == "fsfera-thin" && fix != "Не по шаблону Фотосферы 30×30: 654 × 340 мм, нужно 626 × 326" {
				t.Fatalf("строка: %q", fix)
			}
			if page(e, 400, 300); spec(e).W != 0 {
				t.Fatalf("обложка «из страницы» сдвинулась: %+v", spec(e))
			}
			page(e, 300, 300)
			fixOffTemplate(t, e, id)
		})
	}
}

// fixOffTemplate — часть TestCoverOffTemplateNonTable: «Исправить» обложки «из страницы».
func fixOffTemplate(t *testing.T, e *album.Editor, id string) {
	t.Helper()
	undo := edtest.UndoLen(e)
	v, err := e.ApplyCoverPreset(id)
	if err != nil || v.Cover.Fix != "" || spec(e).W != 286 || spec(e).H != 286 || edtest.UndoLen(e) != undo+1 {
		t.Fatalf("«Исправить»: %v, строка %q, %+v", err, v.Cover.Fix, spec(e))
	}
	rev, undo := edtest.Rev(e), edtest.UndoLen(e)
	if _, err := e.ApplyCoverPreset(id); err != nil {
		t.Fatal(err)
	}
	edtest.Same(t, e, rev, undo, "повтор «Исправить»")
	path := filepath.Join(testkit.TempDir(t), "после.salbum")
	if err := e.Save(path); err != nil {
		t.Fatal(err)
	}
	if q, err := album.LoadBase(path, filepath.Dir(path)); err != nil || q.Cover.Spec.W != 286 || q.Cover.Spec.H != 286 {
		t.Fatalf("Save → Load: %v", err)
	}
	if e.Undo(); spec(e).W != 0 || e.View().Cover.Fix == "" {
		t.Fatalf("Undo «Исправить»: %+v", spec(e))
	}
}
