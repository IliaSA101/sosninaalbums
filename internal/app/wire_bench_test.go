package app

import (
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// bigAlbum — n разворотов «Авто» по k фото, смесь пропорций как у bigProject в album/view:
// составы почти не повторяются, кэш раскладок честно перебирает.
func bigAlbum(n, k int) *model.Project {
	p := model.New()
	root := filepath.VolumeName(os.TempDir()) + string(filepath.Separator)
	x := uint64(n*1000 + k)
	for i := range n {
		s := &model.Spread{ID: fmt.Sprintf("s%03d", i)}
		for j := range k {
			x = x*6364136223846793005 + 1442695040888963407
			w, h := 3000, 2000
			switch r := x >> 33 % 100; {
			case r < 30:
				w, h = 2000, 3000
			case r < 35:
				w, h = 2000, 2000
			case r < 40:
				w, h = 3200, 1800
			}
			id := fmt.Sprintf("p%03d-%02d", i, j)
			p.Photos[id] = model.PhotoRef{Path: filepath.Join(root, id+".jpg"), W: w, H: h}
			s.Photos = append(s.Photos, id)
		}
		p.Spreads = append(p.Spreads, s)
	}
	return p
}

// wailsCost — что делает Wails с видом сверх нашего кода: ответ метода (json.Marshal, затем
// json.Marshal строки в Callback) и событие (json.Marshal, затем JSEscapeString в Notify).
func wailsReply(v any) int {
	js, _ := json.Marshal(v)
	_, _ = json.Marshal(string(js))
	return len(js)
}

func wailsEvent(v any) int {
	js, _ := json.Marshal(v)
	_ = template.JSEscapeString(string(js))
	return len(js)
}

// BenchmarkEditBig200 — переименование разворота на 200 × 16 (кэш раскладок тёплый) путём
// метода App: правка, цепочка вида, событие и ответ — как их кодирует Wails. Цель 075: ≤ 30 мс
// в оптимизированной сборке (exe); «КБ/правка» — сколько уходит по мосту (ответ + событие).
func BenchmarkEditBig200(b *testing.B) {
	a := newApp(b)
	a.cfgDir = testkit.TempDir(b)
	sent := 0
	a.emit = func(name string, data ...any) {
		if name == "view" {
			sent += wailsEvent(data[0])
		}
	}
	a.ed.Store(mustEditor(b, bigAlbum(200, 16), ""))
	a.GetState() // прогрев раскладок и первый полный вид
	i, n := 0, 0
	sent = 0
	for b.Loop() {
		i++
		v := a.SetSpreadName("s005", fmt.Sprint("Имя ", i))
		sent += wailsReply(v)
		n++
	}
	b.ReportMetric(float64(sent)/1024/float64(n), "KB/edit")
}

// BenchmarkWireLoose — та же правка на 20 × 8 с предельными 2 000 отдельных фото (097 §11, §13):
// шапка вида несёт их при каждой правке. «KB/edit» — ответ + событие, «KB/loose» — доля отдельных
// в одном виде; порога нет: цифра решает, нужна ли `loose` в дельте только при изменении.
func BenchmarkWireLoose(b *testing.B) {
	p := bigAlbum(20, 8)
	root := filepath.VolumeName(os.TempDir()) + string(filepath.Separator)
	adds := model.MaxLooseAdds / 5 // 100 добавлений по 20 файлов
	for i := range adds {
		add := model.LooseAdd{ID: model.NewID(), At: int64(1_700_000_000 + i)}
		for j := range model.MaxLooseFiles / adds {
			add.Files = append(add.Files, model.LooseFile{Path: filepath.Join(root, "Фото отдельно", fmt.Sprintf("a%03d", i), fmt.Sprintf("IMG_%04d.jpg", j))})
		}
		p.Loose = append(p.Loose, add)
	}
	a := newApp(b)
	a.cfgDir = testkit.TempDir(b)
	sent := 0
	a.emit = func(name string, data ...any) {
		if name == "view" {
			sent += wailsEvent(data[0])
		}
	}
	a.ed.Store(mustEditor(b, p, ""))
	v := a.GetState().View
	if n := len(p.LooseFiles()); n != model.MaxLooseFiles || len(v.Loose) != len(p.Loose) {
		b.Fatalf("отдельных %d, в виде добавлений %d", n, len(v.Loose))
	}
	looseKB := float64(wailsReply(v.Loose)) / 1024
	i, n := 0, 0
	sent = 0
	for b.Loop() {
		i++
		sent += wailsReply(a.SetSpreadName("s005", fmt.Sprint("Имя ", i)))
		n++
	}
	b.ReportMetric(float64(sent)/1024/float64(n), "KB/edit")
	b.ReportMetric(looseKB, "KB/loose")
}
