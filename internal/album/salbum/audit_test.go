package salbum

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/layout"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Файл с записью, чей ID не равен IDFor(путь), и второй записью того же пути:
// ID приводятся к пути, записи сливаются (первая в ленте побеждает), повтор фото уходит.
func TestLoadRekeyMergesSamePath(t *testing.T) {
	dir := testkit.TempDir(t)
	a, b := filepath.Join(dir, "a.jpg"), filepath.Join(dir, "b.jpg")
	idA, idB := library.IDFor(a), library.IDFor(b)
	f := FileProject{Version: 2, Settings: model.DefaultSettings(), Folders: []FileFolder{},
		Spreads: []*model.Spread{
			{ID: "s1", Photos: []string{idB}},
			{ID: "s2", Photos: []string{idA, idB}},
		},
		Photos: map[string]FilePhoto{
			idB: {PhotoRef: model.PhotoRef{Path: a, W: 3, H: 2, Frame: &model.Frame{S: 2}}},
			idA: {PhotoRef: model.PhotoRef{Path: a, W: 3, H: 2, Frame: &model.Frame{S: 3}}},
		}}
	data, _ := json.Marshal(f)
	p, err := Decode(data, dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Photos) != 1 || p.Photos[idA].Path != a {
		t.Fatalf("фото: %+v", p.Photos)
	}
	if fr := p.Photos[idA].Frame; fr == nil || fr.S != 2 {
		t.Fatalf("кадр должен быть от первой записи: %+v", fr)
	}
	if len(p.Spreads) != 1 || !slices.Equal(p.Spreads[0].Photos, []string{idA}) {
		t.Fatalf("развороты: %+v", p.Spreads)
	}
	for id, ref := range p.Photos {
		if id != library.IDFor(ref.Path) {
			t.Fatalf("ID %s не равен IDFor(%s)", id, ref.Path)
		}
	}
}

// Без слияния: ID просто приводится к пути, ссылки переезжают.
func TestRekeyRenamesRefs(t *testing.T) {
	dir := testkit.TempDir(t)
	a := filepath.Join(dir, "a.jpg")
	p := model.New()
	p.Photos["old"] = model.PhotoRef{Path: a, W: 1, H: 1}
	p.Spreads = []*model.Spread{{ID: "s", Photos: []string{"old"}}}
	Rekey(p, library.IDFor)
	id := library.IDFor(a)
	if _, ok := p.Photos[id]; !ok || len(p.Photos) != 1 || p.Spreads[0].Photos[0] != id {
		t.Fatalf("rekey: %+v %+v", p.Photos, p.Spreads[0])
	}
}

func TestLoadTooManySpreads(t *testing.T) {
	mk := func(n int) []byte {
		f := FileProject{Version: 2, Settings: model.DefaultSettings(), Folders: []FileFolder{}, Photos: map[string]FilePhoto{}}
		for i := range n {
			f.Spreads = append(f.Spreads, &model.Spread{ID: fmt.Sprintf("s%d", i), Photos: []string{}})
		}
		data, _ := json.Marshal(f)
		return data
	}
	if _, err := Decode(mk(model.MaxSpreads), testkit.TempDir(t)); err != nil {
		t.Fatalf("ровно %d разворотов: %v", model.MaxSpreads, err)
	}
	if _, err := Decode(mk(model.MaxSpreads+1), testkit.TempDir(t)); err == nil || !strings.Contains(err.Error(), "слишком много разворотов") {
		t.Fatalf("ждали отказ, получили %v", err)
	}
}

// Общий запас: блоки и знаки считаются по всему проекту, подписи тоже.
func TestClampTextBudget(t *testing.T) {
	p := model.New()
	for range 150 { // 150 × MaxTexts > MaxTotalTexts блоков
		s := &model.Spread{ID: model.NewID()}
		for range model.MaxTexts {
			s.Texts = append(s.Texts, &model.TextBlock{ID: model.NewID(), Text: "t", W: 50})
		}
		p.Spreads = append(p.Spreads, s)
	}
	p.Photos["p"] = model.PhotoRef{Path: "/p.jpg", Caption: &model.Caption{Text: "подпись"}}
	p.Spreads[0].Photos = []string{"p"}
	p.ClampDesign()
	n := 0
	for _, s := range p.Spreads {
		n += len(s.Texts)
	}
	if n != model.MaxTotalTexts {
		t.Fatalf("блоков %d, ждали %d", n, model.MaxTotalTexts)
	}
	if p.Photos["p"].Caption != nil {
		t.Fatal("подпись сверх запаса должна сниматься")
	}

	q := model.New()
	long := strings.Repeat("я", model.MaxTextRunes)
	for range 30 { // 30 × MaxTexts × 1000 знаков > MaxTotalRunes
		s := &model.Spread{ID: model.NewID()}
		for range model.MaxTexts {
			s.Texts = append(s.Texts, &model.TextBlock{ID: model.NewID(), Text: long, W: 50})
		}
		q.Spreads = append(q.Spreads, s)
	}
	q.ClampDesign()
	runes, blocks := 0, 0
	for _, s := range q.Spreads {
		for _, tb := range s.Texts {
			runes += len([]rune(tb.Text))
			blocks++
		}
	}
	if runes > model.MaxTotalRunes || blocks != model.MaxTotalRunes/model.MaxTextRunes {
		t.Fatalf("знаков %d, блоков %d", runes, blocks)
	}
}

// Альбом открыт с UNC: относительный путь не разрешить, остаётся abs; без abs — отказ.
func TestDecodeUNCBase(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("UNC — только Windows")
	}
	const base = `\\srv\share\dir`
	abs := filepath.Join(testkit.TempDir(t), "a.jpg")
	mk := func(fp FilePhoto) []byte {
		f := FileProject{Version: 2, Settings: model.DefaultSettings(), Folders: []FileFolder{},
			Spreads: []*model.Spread{{ID: "s1", Photos: []string{"p"}}}, Photos: map[string]FilePhoto{"p": fp}}
		data, _ := json.Marshal(f)
		return data
	}
	p, err := Decode(mk(FilePhoto{PhotoRef: model.PhotoRef{Path: "a.jpg", W: 3, H: 2}, Abs: abs}), base)
	if err != nil {
		t.Fatal(err)
	}
	if ref := onlyRef(t, p); ref.Path != abs {
		t.Fatalf("путь %q, ждали abs %q", ref.Path, abs)
	}
	if _, err := Decode(mk(FilePhoto{PhotoRef: model.PhotoRef{Path: "a.jpg", W: 3, H: 2}}), base); err == nil {
		t.Fatal("без abs на UNC-базе ждали отказ")
	}
	// Папка — то же правило.
	f := FileProject{Version: 2, Settings: model.DefaultSettings(), Photos: map[string]FilePhoto{},
		Folders: []FileFolder{{Path: "Отбор", Abs: filepath.Dir(abs)}}}
	data, _ := json.Marshal(f)
	q, err := Decode(data, base)
	if err != nil || len(q.Folders) != 1 || q.Folders[0] != filepath.Dir(abs) {
		t.Fatalf("папка: %v %+v", err, q)
	}
	f.Folders[0].Abs = ""
	data, _ = json.Marshal(f)
	if _, err := Decode(data, base); err == nil {
		t.Fatal("папка без abs на UNC-базе ждали отказ")
	}
}

// Паника разбора — ошибка «файл повреждён», а не падение приложения при старте.
func TestDecodePanicIsError(t *testing.T) {
	orig := decodeImpl
	t.Cleanup(func() { decodeImpl = orig })
	decodeImpl = func([]byte, string) (*model.Project, error) { panic("сбой разбора") }

	p, err := Decode([]byte(`{"version":2}`), testkit.TempDir(t))
	if p != nil || err == nil || !strings.Contains(err.Error(), "файл проекта повреждён") {
		t.Fatalf("проект %v, ошибка %v", p, err)
	}
}

// Паника в раскладке при миграции v5 (нынешний или прежний ряд) — запись в лог и Layout ""
// у этого разворота; альбом открывается, соседний разворот мигрирует как обычно.
func TestMigratePanicKeepsAuto(t *testing.T) {
	orig := migRowImpl
	t.Cleanup(func() { migRowImpl = orig })
	migRowImpl = func(asp []float64, prm layout.Params, legacy bool) migRow {
		if len(asp) == 2 {
			panic("сбой раскладки")
		}
		return orig(asp, prm, legacy)
	}
	var buf strings.Builder
	prev := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(prev) })

	dir := testkit.TempDir(t)
	f := FileProject{Version: 5, Settings: model.DefaultSettings(), Folders: []FileFolder{},
		Photos: map[string]FilePhoto{}}
	for si, n := range []int{2, 3} {
		s := &model.Spread{ID: fmt.Sprintf("s%d", si), Variant: 1}
		for i := range n {
			path := filepath.Join(dir, fmt.Sprintf("s%d-%d.jpg", si, i))
			id := library.IDFor(path)
			f.Photos[id] = FilePhoto{PhotoRef: model.PhotoRef{Path: path, W: 300, H: 200 + 100*i}}
			s.Photos = append(s.Photos, id)
		}
		f.Spreads = append(f.Spreads, s)
	}
	data, err := json.Marshal(f)
	if err != nil {
		t.Fatal(err)
	}
	p, err := Decode(data, dir)
	if err != nil {
		t.Fatal(err)
	}
	if p.Spreads[0].Layout != "" || p.Spreads[1].Layout == "" {
		t.Fatalf("Layout %q и %q, ждали \"\" у сбойного и ключ второго варианта у соседа", p.Spreads[0].Layout, p.Spreads[1].Layout)
	}
	if !strings.Contains(buf.String(), "ПАНИКА в раскладке разворота") {
		t.Fatalf("паники нет в логе: %.300q", buf.String())
	}
}

// Файл новой версии даёт ErrNewer: App по нему не прячет черновик в .bad.
func TestDecodeNewerIsErrNewer(t *testing.T) {
	_, err := Decode([]byte(`{"version":99}`), testkit.TempDir(t))
	if !errors.Is(err, ErrNewer) {
		t.Fatalf("ошибка %v", err)
	}
}

// Load: MaxSpreads проверяется после splitBig — деление большого разворота
// не должно дать больше разворотов, чем Load вообще принимает.
func TestLoadSpreadLimitAfterSplit(t *testing.T) {
	save := func(empty int) (string, error) {
		p, dir := model.New(), testkit.TempDir(t)
		ids := make([]string, model.MaxPhotos+1)
		for i := range ids {
			path := filepath.Join(dir, "p"+strconv.Itoa(i)+".jpg")
			id := library.IDFor(path)
			p.Photos[id] = model.PhotoRef{Path: path, W: 3, H: 2}
			ids[i] = id
		}
		p.Spreads = []*model.Spread{{ID: "big", Photos: ids}}
		for range empty {
			p.Spreads = append(p.Spreads, &model.Spread{ID: model.NewID(), Photos: []string{}})
		}
		path := filepath.Join(dir, "a.salbum")
		if err := Save(p, path); err != nil {
			return "", err
		}
		return path, nil
	}
	// 1 большой + 998 пустых → после деления ровно 1000: проходит.
	path, err := save(model.MaxSpreads - 2)
	if err != nil {
		t.Fatal(err)
	}
	if q, err := Load(path); err != nil || len(q.Spreads) != model.MaxSpreads {
		t.Fatalf("ровно предел после деления: err=%v", err)
	}
	// 1 большой + 999 пустых → 1001: отказ.
	path, err = save(model.MaxSpreads - 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil || !strings.Contains(err.Error(), "слишком много разворотов") {
		t.Fatalf("ждали отказ после деления, получили %v", err)
	}
}

func TestLoadSplitsBigSpread(t *testing.T) {
	p, dir := model.New(), testkit.TempDir(t)
	// ID = library.IDFor(путь): Load пересчитывает ID по путям, такие остаются.
	ref := func(name string) string {
		path := filepath.Join(dir, name+".jpg")
		id := library.IDFor(path)
		p.Photos[id] = model.PhotoRef{Path: path, W: 3, H: 2}
		return id
	}
	ids := make([]string, 2*model.MaxPhotos+3)
	for i := range ids {
		ids[i] = ref("p" + strconv.Itoa(i))
	}
	p.Spreads = []*model.Spread{{ID: "s", Photos: ids, Layout: "H(0,1)"}, {ID: "t", Photos: []string{ref("z")}}}
	path := filepath.Join(dir, "a.salbum")
	if err := Save(p, path); err != nil {
		t.Fatal(err)
	}
	q, err := Load(path)
	if err != nil || len(q.Spreads) != 4 {
		t.Fatalf("ждали 4 разворота: err=%v", err)
	}
	var got []string
	for _, s := range q.Spreads[:3] {
		if len(s.Photos) > model.MaxPhotos || s.Layout != "" {
			t.Fatalf("часть: %d фото, layout %q", len(s.Photos), s.Layout)
		}
		got = append(got, s.Photos...)
	}
	if !slices.Equal(got, ids) || q.Spreads[0].ID != "s" || q.Spreads[1].ID == "s" || q.Spreads[3].ID != "t" {
		t.Fatal("порядок фото или id нарушены")
	}
}

func TestLoadClampsSettings(t *testing.T) {
	p := model.New()
	path := filepath.Join(testkit.TempDir(t), "a.salbum")
	if err := Save(p, path); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	data = []byte(strings.Replace(string(data), `"pageW": 300`, `"pageW": 99999`, 1))
	os.WriteFile(path, data, 0o644)
	q, err := Load(path)
	if err != nil || q.Settings.PageW != 1000 {
		t.Fatalf("Load: %v %+v", err, q.Settings)
	}
}

func TestLoadTooBig(t *testing.T) {
	path := filepath.Join(testkit.TempDir(t), "big.salbum")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	f.Truncate(model.MaxFileSize + 1) // разреженный файл, на диск почти не пишется
	f.Close()
	if _, err := Load(path); err == nil || !strings.Contains(err.Error(), "большой") {
		t.Fatalf("ждали отказ по размеру, получили %v", err)
	}
}

func TestSaveLeavesNoTmp(t *testing.T) {
	dir := testkit.TempDir(t)
	path := filepath.Join(dir, "a.salbum")
	if err := Save(model.New(), path); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Fatal(".tmp остался после успешной записи")
	}
	// Ошибка переименования (цель — непустая папка): .tmp убирается.
	bad := filepath.Join(dir, "d.salbum")
	os.MkdirAll(filepath.Join(bad, "x"), 0o755)
	if err := Save(model.New(), bad); err == nil {
		t.Fatal("ждали ошибку записи поверх папки")
	}
	if _, err := os.Stat(bad + ".tmp"); !os.IsNotExist(err) {
		t.Fatal(".tmp остался после ошибки")
	}
}

// root — корень диска временной папки: абсолютный путь на любой ОС.
func root() string { return filepath.VolumeName(os.TempDir()) + string(filepath.Separator) }

// underAny совпадает с library.Under по любой папке: вложенные, корень, регистр, общий префикс имени.
func TestUnderAny(t *testing.T) {
	j := func(parts ...string) string { return filepath.Join(append([]string{root()}, parts...)...) }
	sets := [][]string{
		nil,
		{j("Съёмки")},
		{j("Съёмки"), j("съёмки", "Ивановы"), j("a b"), j("a")},
		{j("a", "b"), j("a b", "c"), j("ab")},
		{root()},
	}
	paths := []string{
		j("Съёмки", "1.jpg"), j("СЪЁМКИ", "Ивановы", "2.jpg"), j("Съёмки2", "1.jpg"), j("Съёмки"),
		j("a", "1.jpg"), j("a b", "c", "1.jpg"), j("ab", "1.jpg"), j("a", "b", "c", "1.jpg"), j("a", "bc", "1.jpg"),
		j("x.jpg"), root(),
	}
	for _, folders := range sets {
		under := UnderAny(folders)
		for _, p := range paths {
			want := false
			for _, f := range folders {
				want = want || library.Under(p, f)
			}
			if got := under(p); got != want {
				t.Errorf("папки %q, путь %q: %v, ждали %v", folders, p, got, want)
			}
		}
	}
}

// trimBig — 200 000 отметок в 2000 папках, половина — вне папок проекта.
func trimBig() (out map[string]model.Mark, order, folders []string) {
	base := filepath.Join(root(), "Съёмки")
	folders = make([]string, 2000)
	for i := range folders {
		folders[i] = filepath.Join(base, fmt.Sprintf("Папка %04d", i))
	}
	out = make(map[string]model.Mark, 200_000)
	order = make([]string, 0, 200_000)
	for i := range 200_000 {
		dir := folders[i%2000]
		if i%2 == 1 {
			dir += "-вне"
		}
		id := fmt.Sprintf("%016x", i)
		out[id] = model.Mark{Path: filepath.Join(dir, fmt.Sprintf("%06d.jpg", i)), Rating: ip(1)}
		order = append(order, id)
	}
	return out, order, folders
}

// Обрезка сверх предела не квадратична по папкам: 200 000 × 2000 — меньше секунды.
func TestTrimMarksFast(t *testing.T) {
	out, order, folders := trimBig()
	runtime.GC() // сборка мусора от подготовки не в замер
	start := time.Now()
	TrimMarks(out, order, folders)
	if d := time.Since(start); d > testkit.Limit(time.Second) {
		t.Fatalf("обрезка %v", d)
	}
	if len(out) != model.MaxMarks {
		t.Fatalf("осталось %d", len(out))
	}
	for _, m := range out {
		if strings.Contains(m.Path, "-вне") {
			t.Fatalf("осталась отметка вне папок: %s", m.Path)
		}
	}
}

func BenchmarkTrimMarks(b *testing.B) {
	for b.Loop() {
		b.StopTimer()
		out, order, folders := trimBig()
		b.StartTimer()
		TrimMarks(out, order, folders)
	}
}

// Папки из файла: повторы — множеством, сверх MaxFolders хвост отбрасывается, порядок прежний.
func TestLoadFoldersLimit(t *testing.T) {
	dir := testkit.TempDir(t)
	list := make([]FileFolder, 0, 50_000)
	for i := range 50_000 {
		list = append(list, FileFolder{Path: filepath.Join(dir, fmt.Sprintf("п%05d", i%(model.MaxFolders+5)))})
	}
	data, err := json.Marshal(FileProject{Version: model.FormatVersion, Folders: list})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "a.salbum")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	p, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if d := time.Since(start); d > testkit.Limit(2*time.Second) {
		t.Fatalf("открытие %v", d)
	}
	if len(p.Folders) != model.MaxFolders || p.Folders[0] != list[0].Path || p.Folders[model.MaxFolders-1] != list[model.MaxFolders-1].Path {
		t.Fatalf("папок %d", len(p.Folders))
	}
}

func TestCheckLocalPathLong(t *testing.T) {
	if err := paths.CheckLocalPath(root() + strings.Repeat("a", 200)); err != nil {
		t.Fatal(err)
	}
	if paths.CheckLocalPath(root()+strings.Repeat("a", paths.MaxPathLen)) == nil {
		t.Fatal("путь длиннее 32 767 пропущен")
	}
}

// ip — копия из helpers_test.go ядра: внутренний тест salbum ядро не импортирует (цикл).
func ip(v int) *int { return &v }
