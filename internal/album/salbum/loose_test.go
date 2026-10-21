package salbum

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// looseFileJSON — файл альбома с отдельными фото list и папками folders (v — версия).
func looseFileJSON(t *testing.T, v int, folders []FileFolder, list []FileLoose) []byte {
	t.Helper()
	f := FileProject{Version: v, Settings: model.DefaultSettings(), Spreads: []*model.Spread{},
		Folders: folders, Photos: map[string]FilePhoto{}, Loose: list}
	data, err := json.Marshal(f)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// Save → Load без потерь: ID, время, порядок добавлений и файлов; файл внутри альбома — path
// относительный и abs, вне — только абсолютный; второй круг — те же байты.
func TestLooseRoundTrip(t *testing.T) {
	root := testkit.TempDir(t)
	dir := filepath.Join(root, "Ивановы", "Альбом") // вторая временная папка — выше двух «..»
	in, out, out2 := filepath.Join(root, "Телефон", "b.jpg"), filepath.Join(testkit.TempDir(t), "Загрузки", "a.png"), filepath.Join(dir, "Своё", "c.JPEG")
	for _, f := range []string{in, out, out2} {
		putFile(t, f, 10)
	}
	p := model.New()
	p.Folders = []string{filepath.Join(root, "Март")}
	p.Loose = []model.LooseAdd{
		{ID: "3fa9c01b7d22", At: 1792501500, Files: []model.LooseFile{{Path: in}, {Path: out}}},
		{ID: "0a0b0c0d0e0f", At: 1792501560, Files: []model.LooseFile{{Path: out2}}},
	}
	path := filepath.Join(dir, "a.salbum")
	first, q := saveLoad(t, p, path)
	f := fileOf(t, first)
	if len(f.Loose) != 2 || f.Loose[0].At != 1792501500 || f.Loose[0].Files[0] != (FileFolder{Path: "../../Телефон/b.jpg", Abs: in}) ||
		f.Loose[0].Files[1] != (FileFolder{Path: out}) || f.Loose[1].Files[0] != (FileFolder{Path: "Своё/c.JPEG", Abs: out2}) {
		t.Fatalf("в файле: %+v", f.Loose)
	}
	if !bytes.Contains(first, []byte(`"at": 1792501500,`)) {
		t.Fatalf("время не целым числом:\n%s", first)
	}
	if !reflect.DeepEqual(q.Loose, p.Loose) {
		t.Fatalf("после круга:\n%+v\nждали\n%+v", q.Loose, p.Loose)
	}
	if second, _ := saveLoad(t, q, path); !bytes.Equal(first, second) {
		t.Fatalf("второй круг другой:\n%s", second)
	}
	// Без отдельных фото ключа в файле нет.
	data, err := Encode(model.New(), dir)
	if err != nil || bytes.Contains(data, []byte(`"loose"`)) {
		t.Fatalf("пустой: %v\n%s", err, data)
	}
}

// Пара путей: основного нет, запасной есть — запасной; нет нигде — основной и Alt, Encode пишет
// запасной обратно как был; основной есть — запасной забыт.
func TestLoadLooseAlt(t *testing.T) {
	dir := testkit.TempDir(t)
	other := testkit.TempDir(t)
	here, moved := filepath.Join(dir, "Фото", "1.jpg"), filepath.Join(other, "2.jpg")
	putFile(t, here, 5)
	putFile(t, moved, 5)
	gone := filepath.Join(other, "нет", "3.jpg")
	data := looseFileJSON(t, model.FormatVersion, nil, []FileLoose{{ID: "0123456789ab", At: 7, Files: []FileFolder{
		{Path: "Фото/1.jpg", Abs: filepath.Join(other, "старое.jpg")},
		{Path: "Фото/2.jpg", Abs: moved},
		{Path: "Фото/3.jpg", Abs: gone},
	}}})
	p, err := Decode(data, dir)
	if err != nil {
		t.Fatal(err)
	}
	want := []model.LooseFile{{Path: here}, {Path: moved}, {Path: filepath.Join(dir, "Фото", "3.jpg"), Alt: gone}}
	if len(p.Loose) != 1 || !reflect.DeepEqual(p.Loose[0].Files, want) {
		t.Fatalf("файлы: %+v", p.Loose)
	}
	out, err := Encode(p, dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := fileOf(t, out).Loose[0].Files[2]; got != (FileFolder{Path: "Фото/3.jpg", Abs: gone}) {
		t.Fatalf("запасной путь не вернулся в файл: %+v", got)
	}
}

// Битые записи пропускаются, альбом открывается: путь с «..» выше предела, UNC, поток NTFS, не
// фото, null; ID не 12 hex и повтор — новый, время меньше нуля — 0, пустое добавление — нет.
// В лог — счёты без путей.
func TestLoadLooseBad(t *testing.T) {
	dir := testkit.TempDir(t)
	ok := filepath.Join(dir, "ok.jpg")
	putFile(t, ok, 3)
	buf := captureLog(t)
	bad := []string{`"../../../x.jpg"`, `"\\\\srv\\share\\a.jpg"`, `"C:\\a.jpg:ads"`, `"а.txt"`, `null`, `{"path":"тайное.gif"}`,
		`"Отдельно/CON"`, `"C:\\x\\NUL"`} // последние два — имя устройства (без расширения — на любой Windows): свой счёт в логе
	raw := fmt.Sprintf(`{"version":%d,"loose":[null,
		{"id":"3fa9c01b7d22","at":-1e18,"files":[%s,{"path":"ok.jpg"}]},
		{"id":"3fa9c01b7d22","at":1.5,"files":[{"path":"ok.jpg"}]},
		{"id":"ZZ","at":5,"files":[{"path":"ok2.jpg"}]},
		{"id":"0123456789ab","files":null}]}`, model.FormatVersion, strings.Join(bad, ","))
	p, err := Decode([]byte(raw), dir)
	if err != nil {
		t.Fatalf("альбом не открылся: %v", err)
	}
	if len(p.Loose) != 2 || p.Loose[0].ID != "3fa9c01b7d22" || p.Loose[0].At != 0 ||
		!reflect.DeepEqual(p.Loose[0].Files, []model.LooseFile{{Path: ok}}) {
		t.Fatalf("первое: %+v", p.Loose)
	}
	if b := p.Loose[1]; !model.ValidID(b.ID) || b.ID == "3fa9c01b7d22" || b.At != 5 || b.Files[0].Path != filepath.Join(dir, "ok2.jpg") {
		t.Fatalf("второе (новый ID вместо ZZ, повтор пути убран): %+v", b)
	}
	logged := buf.String()
	devs := runtime.GOOS != "windows" || strings.Contains(logged, "с именем устройства 2") // вне Windows это обычные имена (не фото)
	if !strings.Contains(logged, "отдельные фото: пропущено") || !devs ||
		strings.Contains(logged, "тайное") || strings.Contains(logged, "srv") || strings.Contains(logged, "CON") {
		t.Fatalf("лог: %q", logged)
	}
}

// Пределы при чтении: хвост добавлений сверх MaxLooseAdds и файлов сверх MaxLooseFiles отброшен;
// сырой список сверх двойного предела не разбирается; в лог — счёт.
func TestLoadLooseLimits(t *testing.T) {
	dir := testkit.TempDir(t)
	captureLog(t)
	var list []FileLoose
	for i := range 2*model.MaxLooseAdds + 50 {
		list = append(list, FileLoose{ID: fmt.Sprintf("%012x", i), At: float64(i), Files: []FileFolder{{Path: fmt.Sprintf("%d.jpg", i)}}})
	}
	p, err := Decode(looseFileJSON(t, model.FormatVersion, nil, list), dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Loose) != model.MaxLooseAdds || p.Loose[model.MaxLooseAdds-1].At != model.MaxLooseAdds-1 {
		t.Fatalf("добавлений %d", len(p.Loose))
	}
	big := make([]FileFolder, 3*model.MaxLooseFiles)
	for i := range big {
		big[i] = FileFolder{Path: fmt.Sprintf("б%d.jpg", i)}
	}
	p, err = Decode(looseFileJSON(t, model.FormatVersion, nil, []FileLoose{{ID: "0123456789ab", Files: big}}), dir)
	if err != nil {
		t.Fatal(err)
	}
	if files := p.LooseFiles(); len(files) != model.MaxLooseFiles || files[len(files)-1] != filepath.Join(dir, fmt.Sprintf("б%d.jpg", model.MaxLooseFiles-1)) {
		t.Fatalf("файлов %d", len(files))
	}
}

// Старый файл «корень + подпапка» двумя записями (и повтором в другом регистре) — одна запись корня, запасной путь
// подпапки снят; отдельное фото под корнем ушло в папку; строка в лог.
func TestLoadFoldsNestedFolders(t *testing.T) {
	dir := testkit.TempDir(t)
	spring := filepath.Join(dir, "Весна 2026")
	if err := os.MkdirAll(spring, 0o755); err != nil { // «Избранного» на диске нет: Abs тоже, FolderAlt едет в loadFolders
		t.Fatal(err)
	}
	buf := captureLog(t)
	folders := []FileFolder{{Path: "Весна 2026/Избранное", Abs: filepath.Join(testkit.TempDir(t), "нет")},
		{Path: "Весна 2026"}, {Path: "весна 2026"}}
	for _, v := range []int{15, model.FormatVersion} {
		p, err := Decode(looseFileJSON(t, v, folders, []FileLoose{{ID: "0123456789ab", Files: []FileFolder{{Path: "Весна 2026/1.jpg"}}}}), dir)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(p.Folders, []string{spring}) || p.FolderAlt != nil || p.Loose != nil {
			t.Fatalf("v%d: папки %q, запасные %v, отдельные %+v", v, p.Folders, p.FolderAlt, p.Loose)
		}
	}
	if !strings.Contains(buf.String(), "вложенных слито 2") {
		t.Fatalf("лог: %q", buf.String())
	}
}

// Вложенная запись за ссылкой (junction) на диске — отдельный корень: сканер в ссылки не заходит,
// свёртка в корень потеряла бы её фото при Load (093-4). Подпапка рядом — сворачивается.
func TestLoadKeepsLinkedFolder(t *testing.T) {
	dir := testkit.TempDir(t)
	root, target := filepath.Join(dir, "A"), filepath.Join(testkit.TempDir(t), "Чужая")
	for _, d := range []string{filepath.Join(root, "Обычная"), target} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	link := testkit.DirLink(t, target, filepath.Join(root, "Ссылка"))
	folders := []FileFolder{{Path: "A"}, {Path: "A/Ссылка"}, {Path: "A/Обычная"}}
	for _, v := range []int{15, model.FormatVersion} {
		p, err := Decode(looseFileJSON(t, v, folders, nil), dir)
		if err != nil {
			t.Fatal(err)
		}
		if want := []string{root, link}; !reflect.DeepEqual(p.Folders, want) {
			t.Fatalf("v%d: папки %q, ждали %q", v, p.Folders, want)
		}
	}
}

// Корень-ссылка (0.22-sec-1): сканер из неё не берёт ни файла, поэтому её подпапка и отдельное
// фото в ней при Load остаются своими записями — проект 0.21 `[Ссылка, Ссылка\День2]` не
// пустеет. Save → Load — то же.
func TestLoadKeepsUnderLinkedRoot(t *testing.T) {
	dir := testkit.TempDir(t)
	target := filepath.Join(testkit.TempDir(t), "Цель")
	if err := os.MkdirAll(filepath.Join(target, "День2"), 0o755); err != nil {
		t.Fatal(err)
	}
	link := testkit.DirLink(t, target, filepath.Join(dir, "Ссылка"))
	putFile(t, filepath.Join(target, "a.jpg"), 10)
	folders := []FileFolder{{Path: "Ссылка"}, {Path: "Ссылка/День2"}}
	loose := []FileLoose{{ID: "0123456789ab", At: 7, Files: []FileFolder{{Path: "Ссылка/a.jpg"}}}}
	wantF := []string{link, filepath.Join(link, "День2")}
	for _, v := range []int{15, model.FormatVersion} {
		p, err := Decode(looseFileJSON(t, v, folders, loose), dir)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(p.Folders, wantF) {
			t.Fatalf("v%d: папки %q, ждали %q", v, p.Folders, wantF)
		}
		if v == 15 {
			continue // в v15 отдельных фото нет
		}
		if len(p.Loose) != 1 || len(p.Loose[0].Files) != 1 {
			t.Fatalf("отдельное фото в корне-ссылке свёрнуто: %+v", p.Loose)
		}
		path := filepath.Join(dir, "круг.salbum")
		if err := Save(p, path); err != nil {
			t.Fatal(err)
		}
		q, err := Load(path)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(q.Folders, wantF) || len(q.Loose) != 1 {
			t.Fatalf("Save → Load: папки %q, отдельные %+v", q.Folders, q.Loose)
		}
	}
}

// Том недоступен (0.22-sec-1): альбом на отключённом диске открыт — ссылка ли запись, не узнать,
// поэтому вложенная запись не сворачивается (иначе автосохранение убрало бы junction-запись
// навсегда). Та же пара на живом томе без ссылок — сворачивается, как прежде.
func TestLoadMissingVolumeKeepsNested(t *testing.T) {
	gone := filepath.Join(testkit.MissingDrive(t), "Альбомы")
	folders := []FileFolder{{Path: "Свадьба"}, {Path: "Свадьба/Ссылка"}, {Path: "Свадьба/Ссылка"}}
	p, err := Decode(looseFileJSON(t, model.FormatVersion, folders, nil), gone)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{filepath.Join(gone, "Свадьба"), filepath.Join(gone, "Свадьба", "Ссылка")}
	if !reflect.DeepEqual(p.Folders, want) {
		t.Fatalf("тома нет: папки %q, ждали %q (повтор уходит, вложенная остаётся)", p.Folders, want)
	}
	live := testkit.TempDir(t)
	p, err = Decode(looseFileJSON(t, model.FormatVersion, folders, nil), live)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{filepath.Join(live, "Свадьба")}; !reflect.DeepEqual(p.Folders, want) {
		t.Fatalf("живой том: папки %q, ждали %q", p.Folders, want)
	}
}

// v15 (0.21) читается как v16: отдельных нет, from 15; первая запись — .v15.bak с исходником, в
// файле версия 16; файл новее (17) — ErrNewer.
func TestMigrateV15ToV16(t *testing.T) {
	dir := testkit.TempDir(t)
	p := binsProject(dir)
	data, err := Encode(p, dir)
	if err != nil {
		t.Fatal(err)
	}
	cur := fmt.Appendf(nil, `"version": %d,`, model.FormatVersion)
	src := bytes.Replace(data, cur, []byte(`"version": 15,`), 1)
	if bytes.Equal(src, data) {
		t.Fatal("заготовка v15 без номера версии")
	}
	path := filepath.Join(dir, "old.salbum")
	if err := os.WriteFile(path, src, 0o644); err != nil {
		t.Fatal(err)
	}
	q, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if q.From() != 15 || q.Version != 16 || q.Loose != nil || len(q.Loaded().Bins) != 2 {
		t.Fatalf("from %d, версия %d, отдельные %+v, подборок %d", q.From(), q.Version, q.Loose, len(q.Loaded().Bins))
	}
	if err := BackupOld(path, q.From()); err != nil {
		t.Fatal(err)
	}
	if err := Save(q, path); err != nil {
		t.Fatal(err)
	}
	if bak, err := os.ReadFile(path + ".v15.bak"); err != nil || !bytes.Equal(bak, src) {
		t.Fatalf(".v15.bak: %v", err)
	}
	if out, _ := os.ReadFile(path); fileOf(t, out).Version != 16 || !bytes.Equal(out, data) {
		t.Fatalf("запись v15 отличается не только версией:\n%s", out)
	}
	if _, err := Decode(bytes.Replace(data, cur, []byte(`"version": 17,`), 1), dir); err != ErrNewer {
		t.Fatalf("v17: %v", err)
	}
}

// Check: пути пар отдельных фото (UNC и в запасном) и пределы — отказ; годное — проходит.
func TestCheckLoose(t *testing.T) {
	dir := testkit.TempDir(t)
	good := filepath.Join(dir, "1.jpg")
	cases := map[string][]model.LooseAdd{
		"UNC":      {{ID: "0123456789ab", Files: []model.LooseFile{{Path: `\\srv\share\1.jpg`}}}},
		"запасной": {{ID: "0123456789ab", Files: []model.LooseFile{{Path: good, Alt: "1.jpg"}}}},
		"пустой":   {{ID: "0123456789ab", Files: []model.LooseFile{{}}}},
	}
	for name, list := range cases {
		p := model.New()
		p.Loose = list
		if err := Check(p); err == nil {
			t.Errorf("%s: Check пропустил", name)
		}
	}
	p := model.New()
	p.Loose = []model.LooseAdd{{ID: "0123456789ab", Files: make([]model.LooseFile, model.MaxLooseFiles+1)}}
	for i := range p.Loose[0].Files {
		p.Loose[0].Files[i].Path = filepath.Join(dir, fmt.Sprint(i, ".jpg"))
	}
	if err := Check(p); err == nil {
		t.Error("сверх MaxLooseFiles: Check пропустил")
	}
	p.Loose[0].Files = p.Loose[0].Files[:2]
	if err := Check(p); err != nil {
		t.Errorf("годное: %v", err)
	}
}

// Обрезка отметок: отметки отдельных фото — «внутри», как фото папок, уходят после чужих.
func TestTrimMarksLoose(t *testing.T) {
	dir := testkit.TempDir(t)
	t.Cleanup(func() { model.SetMarksLimit(model.MaxMarks) })
	model.SetMarksLimit(2)
	r := 3
	stray, loose, inFolder := filepath.Join(dir, "чужое.jpg"), filepath.Join(dir, "Телефон", "1.jpg"), filepath.Join(dir, "Март", "2.jpg")
	out := map[string]model.Mark{}
	var order []string
	for _, f := range []string{stray, loose, inFolder} {
		id := library.IDFor(f)
		out[id] = model.Mark{Path: f, Rating: &r}
		order = append(order, id)
	}
	TrimMarks(out, order, []string{filepath.Join(dir, "Март")}, []string{strings.ToUpper(loose)})
	if _, ok := out[library.IDFor(stray)]; ok || len(out) != 2 {
		t.Fatalf("осталось %d, чужая на месте: %v", len(out), ok)
	}
	// Без отдельных — уходит последняя из «вне папок» (порядок файла), отдельное фото — в их числе.
	out2 := map[string]model.Mark{}
	for _, id := range order {
		out2[id] = out[id]
	}
	out2[library.IDFor(stray)] = model.Mark{Path: stray, Rating: &r}
	TrimMarks(out2, order, []string{filepath.Join(dir, "Март")}, nil)
	if _, ok := out2[library.IDFor(loose)]; ok {
		t.Fatal("без списка отдельных их отметка — не «внутри»: ждали, что уйдёт")
	}
}
