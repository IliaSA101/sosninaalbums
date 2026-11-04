package app

// Замер превью службы агентов на настоящем App (design §6, «факт на App»). Не проверка, а
// повторяемая таблица: идёт только с SA_AGENTS_MEASURE=<папка с JPEG около 4000 px> (обычно
// .testphotos/big), печатает строки таблицы в лог теста: go test ./internal/app -run
// TestAgentsPreviewMeasure -v. Время — полный круг через сокет и SDK (вызов клиента → ответ).

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

func TestAgentsPreviewMeasure(t *testing.T) {
	src := os.Getenv("SA_AGENTS_MEASURE")
	if src == "" {
		t.Skip("замер: задайте SA_AGENTS_MEASURE=<папка с JPEG ~4000 px>")
	}
	files, _ := filepath.Glob(filepath.Join(src, "big-*.jpg"))
	if len(files) < 4 {
		t.Fatalf("в %s нужно не меньше 4 файлов big-*.jpg", src)
	}
	sort.Strings(files)
	r := newAgRig(t)
	dir := filepath.Join(testkit.TempDir(t), "Большие")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	var big []string
	for i := range 7 { // 1 + 2 + 4 фото на трёх разворотах: каждое фото в альбоме один раз, копии одного оригинала
		b, err := os.ReadFile(files[i%4])
		if err != nil {
			t.Fatal(err)
		}
		p := filepath.Join(dir, fmt.Sprintf("big-%d.jpg", i))
		if err := os.WriteFile(p, b, 0o644); err != nil {
			t.Fatal(err)
		}
		big = append(big, p)
	}
	if _, err := r.a.AddFolderPath(dir); err != nil {
		t.Fatal(err)
	}
	waitRead(t, r.a)
	ids := []string{}
	for _, p := range big {
		ids = append(ids, library.IDFor(p))
	}
	first := len(r.a.editor().View().Spreads)
	from := 0
	for _, n := range []int{1, 2, 4} { // развороты с 1, 2 и 4 фото
		if _, err := r.a.AutoBuild(ids[from:from+n], len(r.a.editor().View().Spreads), model.AutoOpts{Spreads: 1, MaxPer: n, MinPer: n, Variety: model.VarietyNormal}); err != nil {
			t.Fatal(err)
		}
		from += n
	}
	r.settle()
	sps := r.a.editor().View().Spreads
	if len(sps) != first+3 {
		t.Fatalf("разворотов %d, ждали %d", len(sps), first+3)
	}
	r.start(t)
	r.a.SetAgentsLevel(3)

	freshCache := func() {
		c, err := library.NewCache(testkit.TempDir(t))
		if err != nil {
			t.Fatal(err)
		}
		r.a.cache = c
	}
	type row struct{ name, cold, warm, jpeg, wire string }
	var rows []row
	one := func(tool string, args map[string]any) (time.Duration, int, bool) {
		t0 := time.Now()
		res := r.call(t, tool, args)
		d := time.Since(t0)
		img := agImage(res)
		return d, len(img), res.IsError
	}
	measure := func(name, tool string, args map[string]any) {
		freshCache()
		cold, n, bad := one(tool, args)
		var ds []time.Duration
		for range 7 {
			d, _, _ := one(tool, args)
			ds = append(ds, d)
		}
		slices.Sort(ds)
		med := ds[len(ds)/2]
		note := ""
		if bad {
			note = " (отказ)"
		}
		ms := func(d time.Duration) string { return fmt.Sprintf("%.0f", float64(d.Microseconds())/1000) }
		rows = append(rows, row{name, ms(cold), fmt.Sprintf("%s (%s…%s)", ms(med), ms(ds[0]), ms(ds[len(ds)-1])),
			fmt.Sprintf("%d%s", n/1024, note), fmt.Sprintf("%d", (n+2)/3*4/1024)})
	}
	for _, px := range []int{512, 768, 1024} {
		measure(fmt.Sprintf("Фото %d, оригинал ~4000 px", px), "get_photo_preview", map[string]any{"id": ids[0], "size": px})
	}
	for i, n := range []int{1, 2, 4} {
		for _, px := range []int{1200, 1600} {
			measure(fmt.Sprintf("Разворот %d фото, %d px", n, px), "get_spread_preview", map[string]any{"spread": first + i + 1, "size": px})
		}
	}
	// Пропавшее фото: файла нет — отказ без обращения к кэшу; разворот рисуется с пустым местом.
	if err := os.Remove(big[3]); err != nil {
		t.Fatal(err)
	}
	r.a.rescan()
	waitRead(t, r.a)
	measure("Фото пропало (get_photo_preview)", "get_photo_preview", map[string]any{"id": ids[3]})
	measure("Разворот 4 фото, одного нет, 1200 px", "get_spread_preview", map[string]any{"spread": first + 3, "size": 1200})

	var b strings.Builder
	b.WriteString("\n| Что | Холодный кэш, мс | Тёплый, мс | JPEG, КБ | Ответ (base64), КБ |\n| --- | --- | --- | --- | --- |\n")
	for _, w := range rows {
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %s |\n", w.name, w.cold, w.warm, w.jpeg, w.wire)
	}
	t.Log(b.String())
}
