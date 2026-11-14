package printshop

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Эталон планов «Печати» (0.26-123 (В)): снят с кода до переноса таблиц типографий в JSON и после
// переноса проходит без -update-plans. На странице-пробе × обложке (или без неё) считаются Plans
// при разном числе разворотов и разном последнем выборе; в файле — sha256 всего JSON этих планов.
var updatePlans = flag.Bool("update-plans", false, "перезаписать testdata/golden/print-plans.txt")

const plansGoldenPath = "testdata/golden/print-plans.txt"

// plansSpreads — числа разворотов вокруг границ пределов Фотосферы (20, 30, 57), Нетпринта (8, 10,
// 30, 60) и Фабрики (2, 9, 10, 12, 30, 40).
var plansSpreads = []int{0, 1, 2, 3, 8, 9, 10, 12, 20, 21, 30, 31, 41, 58, 61}

// plansPages — страницы-пробы: кнопки всех рядов с их зоной и обрезом, те же размеры в стиле
// album() (зона 10 без обреза), чужие размеры, допуск PageTol и лист 031.
func plansPages() (names []string, sets []model.Settings) {
	add := func(name string, s model.Settings) {
		if !slices.ContainsFunc(sets, func(x model.Settings) bool { return x == s }) {
			names, sets = append(names, name), append(sets, s)
		}
	}
	base := model.New().Settings
	for _, pr := range model.PagePresets() {
		s := base
		s.PageW, s.PageH, s.Safe, s.Bleed = pr.W, pr.H, pr.Safe, pr.Bleed
		if pr.DPI > 0 {
			s.DPI = pr.DPI
		}
		add(pr.Group+"/"+pr.Label, s)
		s = base
		s.PageW, s.PageH, s.Safe = pr.W, pr.H, 10
		add(pr.Group+"/"+pr.Label+"/safe10", s)
	}
	for _, w := range [][2]float64{{300, 200}, {303.95, 202.95}, {200.1, 203}, {120, 90}, {400, 400}, {250, 250}, {304.3, 203}} {
		s := base
		s.PageW, s.PageH, s.Safe = w[0], w[1], 10
		add(fmt.Sprintf("чужая/%vx%v", w[0], w[1]), s)
	}
	sh := base
	sh.PageW, sh.PageH, sh.Safe, sh.Sheet = 304, 203, 5, true
	add("лист 031", sh)
	return names, sets
}

// plansProject — альбом с n пустыми разворотами и обложкой пресета preset ("" — без обложки)
// по шаблону страницы s, как это делает «Создать обложку».
func plansProject(s model.Settings, n int, preset string) *model.Project {
	p := model.New()
	p.Settings = s
	for range n {
		p.Spreads = append(p.Spreads, &model.Spread{ID: "s", Photos: []string{}})
	}
	if preset != "" && !s.Sheet {
		pr, _ := model.CoverPresetFor(preset, s)
		p.Cover = &model.Cover{Sheet: &model.Spread{ID: model.CoverID}, Spec: pr.Spec}
	}
	return p
}

// plansLines — по строке на «страница | обложка»: sha256 планов при всех n и последних выборах.
// dump != nil получает те же планы целиком (для diff до и после правки).
func plansLines(t *testing.T, dump *strings.Builder) []string {
	t.Helper()
	names, sets := plansPages()
	covers := []string{""}
	for _, c := range model.CoverPresets() {
		covers = append(covers, c.ID)
	}
	var out []string
	for i, s := range sets {
		for _, c := range covers {
			h := sha256.New()
			for _, n := range plansSpreads {
				for _, last := range []string{"", "fsfera", "fabrika"} {
					b, err := json.Marshal(Plans(plansProject(s, n, c), "", last))
					if err != nil {
						t.Fatal(err)
					}
					h.Write(b)
					h.Write([]byte{'\n'})
					if dump != nil {
						fmt.Fprintf(dump, "%s|%s|%d|%s\t%s\n", names[i], c, n, last, b)
					}
				}
			}
			out = append(out, fmt.Sprintf("%s | %s %s", names[i], cmpCover(c), hex.EncodeToString(h.Sum(nil))[:16]))
		}
	}
	return out
}

func cmpCover(c string) string {
	if c == "" {
		return "без обложки"
	}
	return c
}

// TestPrintPlansSnapshot сверяет планы «Печати» (запреты, предупреждения, памятка, пределы, px,
// пункт по умолчанию) с эталоном. SA_PLANS_DUMP=<файл> пишет планы целиком — снять до и после правки
// и сравнить diff, если хеш разошёлся.
func TestPrintPlansSnapshot(t *testing.T) {
	if testkit.Lite() {
		t.Skip("~4 с на 700 пар страница × обложка — не быстрый уровень")
	}
	var dump *strings.Builder
	dumpPath, wantDump := os.LookupEnv("SA_PLANS_DUMP")
	if wantDump {
		if dumpPath == "" {
			t.Fatal("SA_PLANS_DUMP задан пустым: путь обязателен")
		}
		dump = &strings.Builder{}
	}
	got := plansLines(t, dump)
	if wantDump {
		if err := os.WriteFile(dumpPath, []byte(dump.String()), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if *updatePlans {
		if err := os.WriteFile(plansGoldenPath, []byte(strings.Join(got, "\n")+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("эталон перезаписан: %s (%d строк)", plansGoldenPath, len(got))
		return
	}
	data, err := os.ReadFile(plansGoldenPath)
	if err != nil {
		t.Fatalf("нет эталона (%v): go test ./internal/export/printshop -run PrintPlansSnapshot -update-plans", err)
	}
	want := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	if slices.Equal(want, got) {
		return
	}
	var diff []string
	for i := 0; i < max(len(want), len(got)) && len(diff) < 10; i++ {
		w, g := "", ""
		if i < len(want) {
			w = want[i]
		}
		if i < len(got) {
			g = got[i]
		}
		if w != g {
			diff = append(diff, fmt.Sprintf("строка %d:\n  эталон: %s\n  сейчас: %s", i+1, w, g))
		}
	}
	t.Fatalf("планы «Печати» разошлись с эталоном (строк %d и %d); SA_PLANS_DUMP=<файл> снимает планы целиком:\n%s",
		len(want), len(got), strings.Join(diff, "\n"))
}
