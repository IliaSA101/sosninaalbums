package faces

import (
	"image"
	"image/jpeg"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// photosDir ищет .testphotos/<sub> вверх от папки теста (фото вне git, лежат в основном дереве,
// в том числе из рабочих копий .claude/worktrees); "" — нет.
func photosDir(sub string) string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		p := filepath.Join(dir, ".testphotos", sub)
		if st, err := os.Stat(p); err == nil && st.IsDir() {
			return p
		}
		up := filepath.Dir(dir)
		if up == dir {
			return ""
		}
		dir = up
	}
}

func loadJPEG(t *testing.T, path string) image.Image {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := jpeg.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	return img
}

// eyeRatio — где середина глаз по высоте рамки (0 — верх, 1 — низ).
func eyeRatio(f Face) float64 { return (f.EyeLine().Y - f.Box.Y) / f.Box.H }

// largest — лицо с наибольшей высотой рамки.
func largest(fs []Face) Face {
	best := fs[0]
	for _, f := range fs[1:] {
		if f.Box.H > best.Box.H {
			best = f
		}
	}
	return best
}

// stockSet — стоковые фото одного прогона теста: каждое проходит сеть один раз.
type stockSet struct {
	dir  string
	memo map[string][]Face
}

func (s *stockSet) faces(t *testing.T, n string) []Face {
	t.Helper()
	if fs, ok := s.memo[n]; ok {
		return fs
	}
	fs := Detect(loadJPEG(t, filepath.Join(s.dir, "stock-"+n+".jpg")))
	s.memo[n] = fs
	return fs
}

// TestDetectStock: ожидания design §10 на 100 стоковых фото (docs/qa/stock-photos.md). Нет
// папки — пропуск, как у соседних тестов на .testphotos.
func TestDetectStock(t *testing.T) {
	dir := photosDir("stock")
	if dir == "" {
		t.Skip("нет .testphotos/stock (фото вне git, docs/QA.md «Подготовка»)")
	}
	skipHeavy(t)
	set := &stockSet{dir: dir, memo: map[string][]Face{}}
	cases := []struct {
		n        string
		min, max int
		note     string
	}{
		{"22", 1, 1, "невеста у зеркала"},
		{"80", 2, 2, "пожилая пара"},
		{"37", 1, 1, "профиль"},
		{"73", 1, 1, "в рост, лицо мелкое"},
		{"05", 0, 0, "букет: ложное 0,60 ниже порога"},
		{"57", 0, 0, "руки с кольцами: ложное 0,82 снимает фильтр глаз"},
		// два настоящих лица (жених смотрит вниз, отец сбоку): design писал «1» про лицо жениха,
		// проверка его — ниже; здесь счёт честный, глазами сверено по снимку
		{"39", 2, 2, "жених смотрит вниз и отец сбоку"},
		{"78", 9, 99, "групповое"},
		{"24", 2, 2, "невеста и отражение в зеркале"},
	}
	for _, c := range cases {
		if testing.Short() && !liteStock[c.n] {
			continue
		}
		t.Run("stock-"+c.n, func(t *testing.T) {
			fs := set.faces(t, c.n)
			if len(fs) < c.min || len(fs) > c.max {
				t.Fatalf("stock-%s (%s): лиц %d, ждали %d…%d: %+v", c.n, c.note, len(fs), c.min, c.max, fs)
			}
			for _, f := range fs {
				if f.Score < minScore || f.Box.W <= 0 || f.Box.H <= 0 || !f.plausible() {
					t.Errorf("лицо вне контракта: %+v", f)
				}
			}
		})
	}
	t.Run("stock-57: фильтр, а не детектор", set.falseFace57)
	if testing.Short() {
		return
	}
	t.Run("stock-39: смотрящий вниз остался", set.lookingDown39)
	t.Run("stock-24: наибольшее — невеста", set.bride24)
}

// liteStock — фото быстрого уровня (-short): пара, одиночка, фильтр глаз; остальные — полный прогон.
var liteStock = map[string]bool{"22": true, "80": true, "05": true, "57": true}

// skipHeavy — тяжёлый прогон модели по фото пропускается под -race: сеть считает одной
// горутиной, гонкам там нечего ловить (их ловят TestDetectRace и TestDetectOddInputs), а время
// растёт в разы; целиком эти тесты гоняет обычный прогон.
func skipHeavy(t *testing.T) {
	t.Helper()
	if testkit.Race {
		t.Skip("прогон модели по фото — в обычном режиме, под -race не нужен")
	}
}

// falseFace57: без фильтра ложное лицо есть — иначе «0» значило бы, что сеть сломалась, а не
// что фильтр работает.
func (s *stockSet) falseFace57(t *testing.T) {
	all, err := candidates(loadJPEG(t, filepath.Join(s.dir, "stock-57.jpg")), 1, &memTrack{})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, f := range nms(all, nmsIoU) {
		found = found || eyeRatio(f) < eyeMin
	}
	if !found {
		t.Error("на stock-57 без фильтра нет ложного лица с глазами выше 0,2 рамки: проверка потеряла смысл")
	}
}

// lookingDown39: самое крупное лицо — жених; глаза ниже обычных 0,38, но внутри 0,2…0,7 (замер 0,556).
func (s *stockSet) lookingDown39(t *testing.T) {
	fs := s.faces(t, "39")
	if len(fs) == 0 {
		t.Fatal("нет лиц")
	}
	if e := eyeRatio(largest(fs)); e < 0.5 || e > 0.62 {
		t.Errorf("середина глаз жениха на %.3f высоты рамки, ждали ≈ 0,556", e)
	}
}

// bride24: невеста — в середине кадра (лицо ≈ 22 % высоты), отражение в зеркале — маленькое вверху.
func (s *stockSet) bride24(t *testing.T) {
	fs := s.faces(t, "24")
	b := largest(fs)
	c := Pt{b.Box.X + b.Box.W/2, b.Box.Y + b.Box.H/2}
	if b.Box.H < 0.15 || c.Y < 0.25 || c.Y > 0.5 || c.X < 0.35 || c.X > 0.65 {
		t.Errorf("наибольшее лицо: центр (%.2f, %.2f), высота %.3f — не невеста", c.X, c.Y, b.Box.H)
	}
	for _, f := range fs {
		if f.Box.H != b.Box.H && f.Box.H > 0.7*b.Box.H {
			t.Errorf("отражение (%.3f) не меньше 70%% невесты (%.3f): выбор «наибольшее» стал неоднозначным", f.Box.H, b.Box.H)
		}
	}
}

// TestDetectPortraits: 10 детских портретов (docs/qa/stock-photos.md): на каждом одно лицо, и
// фильтр глаз ничего не отбросил — иначе набор не показывает, что порог и фильтр годятся детям.
func TestDetectPortraits(t *testing.T) {
	dir := photosDir("portraits")
	if dir == "" {
		t.Skip("нет .testphotos/portraits (10 детских портретов вне git, docs/qa/stock-photos.md)")
	}
	skipHeavy(t)
	files, err := filepath.Glob(filepath.Join(dir, "*.jpg"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) < 10 {
		t.Fatalf("в .testphotos/portraits %d фото, ждали 10", len(files))
	}
	if testing.Short() { // быстрый уровень: каждый четвёртый
		slices.Sort(files)
		var every4 []string
		for i := 0; i < len(files); i += 4 {
			every4 = append(every4, files[i])
		}
		files = every4
	}
	for _, f := range files {
		t.Run(filepath.Base(f), func(t *testing.T) {
			img := loadJPEG(t, f)
			all, err := candidates(img, 1, &memTrack{})
			if err != nil {
				t.Fatal(err)
			}
			raw := nms(all, nmsIoU) // без фильтра глаз
			fs := Detect(img)
			if len(fs) != 1 {
				t.Fatalf("лиц %d, ждали 1: %+v", len(fs), fs)
			}
			if len(raw) != len(fs) {
				t.Errorf("фильтр глаз отбросил %d из %d (середина глаз на %.2f)", len(raw)-len(fs), len(raw), eyeRatio(raw[0]))
			}
			if e := eyeRatio(fs[0]); e < 0.25 || e > 0.6 {
				t.Errorf("середина глаз на %.3f высоты рамки: у детей ждём 0,3…0,5", e)
			}
		})
	}
}
