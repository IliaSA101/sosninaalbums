package app

// Места съёмок на стороне App (150, design §4.2): источники пополнения, засев, окно
// «Папка со съёмками…». Правила — agentplaces_test.go.

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"

	wr "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

// placeApp — App без окна, правила мест — без запретов (тестовые папки лежат под TMPDIR), диск
// подменён; считает события agents:state (каждое пополнение мест — одно).
func placeApp(t *testing.T) (a *App, states func() int, e *placeEnv) {
	t.Helper()
	a = newApp(t)
	var mu sync.Mutex
	n := 0
	a.emit = func(name string, _ ...any) {
		if name == "agents:state" {
			mu.Lock()
			n++
			mu.Unlock()
		}
	}
	fakeDisk(t)
	e = realEnv()
	a.askM().rules = func() *folderRules { return e.rules }
	return a, func() int { mu.Lock(); defer mu.Unlock(); return n }, e
}

func placeDirs(t *testing.T, a *App) []string {
	t.Helper()
	var out []string
	for _, p := range prefs.LoadAgentPerms(a.cfgDir).Places {
		out = append(out, p.Dir)
	}
	return out
}

// mkFolder — папка съёмки «<родитель>/<имя>» на диске; возвращает папку и её родителя (настоящий путь).
func mkFolder(t *testing.T, name string) (folder, parent string) {
	t.Helper()
	parent = realDir(t, "Год")
	folder = filepath.Join(parent, name)
	if err := os.Mkdir(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	return folder, parent
}

// Места пополняют только действия человека: «Добавить папку…» (ChooseFolder → addChosenFolder, одна
// запись) и бросок папки; AddFolderPath сама — нет; отказ правил не мешает добавить папку.
func TestPlacePushedByHuman(t *testing.T) {
	a, states, e := placeApp(t)
	captureLog(t)
	f1, _ := mkFolder(t, "Петровы")

	// Не источник: AddFolderPath (её зовёт ChooseFolder, но фронт напрямую — нет).
	if _, err := a.AddFolderPath(f1); err != nil {
		t.Fatal(err)
	}
	if got := placeDirs(t, a); len(got) != 0 || states() != 0 {
		t.Fatalf("AddFolderPath пополнила места: %v (событий %d)", got, states())
	}

	// ChooseFolder: родитель — место «parent», одна запись и одно событие.
	f2, p2 := mkFolder(t, "Ивановы")
	if _, err := a.addChosenFolder(f2); err != nil {
		t.Fatal(err)
	}
	pl := prefs.LoadAgentPerms(a.cfgDir).Places
	if len(pl) != 1 || pl[0].Dir != p2 || pl[0].How != prefs.PlaceParent || states() != 1 {
		t.Fatalf("после ChooseFolder: %v, событий %d; ждали одно место %s", pl, states(), p2)
	}
	if got := prefs.LoadConfig(a.cfgDir).LastDirs[prefs.DirPhotos]; got != p2 {
		t.Errorf("lastDirs.photos = %q, ждали %q", got, p2)
	}
	// Повтор того же родителя — холостая правка: ни записи, ни события.
	f2b := filepath.Join(p2, "Сидоровы")
	if err := os.Mkdir(f2b, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := a.addChosenFolder(f2b); err != nil {
		t.Fatal(err)
	}
	if len(placeDirs(t, a)) != 1 || states() != 1 {
		t.Errorf("тот же родитель: места %v, событий %d", placeDirs(t, a), states())
	}
	// Ошибка добавления (путь не локальный) — места те же.
	if _, err := a.addChosenFolder("относительная/папка"); err == nil {
		t.Error("относительный путь принят")
	}
	if got := placeDirs(t, a); len(got) != 1 {
		t.Errorf("места после неудачи: %v", got)
	}

	// Бросок: два каталога из одного родителя и один из другого — по одному месту, одно событие.
	before := states()
	d1, q1 := mkFolder(t, "А")
	d2 := filepath.Join(q1, "Б")
	d3, q3 := mkFolder(t, "В")
	if err := os.Mkdir(d2, 0o755); err != nil {
		t.Fatal(err)
	}
	a.dropPaths([]string{d1, d2, d3})
	waitScan(t, a)
	got := placeDirs(t, a)
	if len(got) != 3 || got[0] != q1 || got[1] != q3 || got[2] != p2 || states() != before+1 {
		t.Fatalf("после броска: %v (событий +%d); ждали [%s %s %s]", got, states()-before, q1, q3, p2)
	}

	// Родитель не прошёл правила: папка добавилась, места прежние.
	d4, q4 := mkFolder(t, "Г")
	e.rules.trees = []string{q4}
	a.dropPaths([]string{d4})
	waitScan(t, a)
	if got := placeDirs(t, a); len(got) != 3 {
		t.Errorf("запрещённый родитель стал местом: %v", got)
	}
	if !contains(a.editor().View().Folders, d4) {
		t.Error("папка не добавилась из-за отказа места")
	}

	// Пустой бросок и бросок одних файлов — места те же.
	n := states()
	a.dropPaths(nil)
	if states() != n {
		t.Error("пустой бросок вызвал запись мест")
	}
}

// Предел: мест не больше MaxPlaces, бросок многих папок — первые брошенные наверху.
func TestPlacePushLimit(t *testing.T) {
	a, _, _ := placeApp(t)
	captureLog(t)
	var dirs, parents []string
	for range prefs.MaxPlaces + 5 {
		f, p := mkFolder(t, "Съёмка")
		dirs, parents = append(dirs, f), append(parents, p)
	}
	a.dropPaths(dirs)
	waitScan(t, a)
	got := placeDirs(t, a)
	if len(got) == 0 {
		t.Fatalf("мест нет (ждали %d), первое ждали %q", prefs.MaxPlaces, parents[0])
	}
	if len(got) != prefs.MaxPlaces || got[0] != parents[0] {
		t.Errorf("мест %d (ждали %d), первое %q (ждали %q)", len(got), prefs.MaxPlaces, got[0], parents[0])
	}
}

// Ответ на вопрос агента (Добавить, Создать, отказ) мест не пополняет, а lastDirs.photos им пишется —
// засев не берёт эту папку: он делается до перезаписи.
func TestPlaceNotFromAgentAnswer(t *testing.T) {
	r := newAskRig(t)
	a := r.a
	fakeDisk(t)
	rules := realEnv().rules
	a.askM().rules = func() *folderRules { return rules }

	// Добавить: выбранная в окне ОС папка попала в альбом, lastDirs.photos записан, мест нет.
	c := r.ask(r.addReq())
	if _, err := a.AnswerAgent(r.asked(t, 1).ID, true); err != nil {
		t.Fatal(err)
	}
	c.wait(t)
	if !contains(a.editor().View().Folders, r.other) {
		t.Fatal("папка не добавлена")
	}
	if got := prefs.LoadConfig(a.cfgDir).LastDirs[prefs.DirPhotos]; got != filepath.Dir(r.other) {
		t.Fatalf("lastDirs.photos = %q (опора теста: ответ агента его пишет)", got)
	}
	if got := placeDirs(t, a); len(got) != 0 {
		t.Fatalf("«Добавить» пополнило места: %v", got)
	}
	// Первая запись разрешений после этого не засевает папку, названную агентом: засев уже был.
	a.SetAgentPerm("autobuild", prefs.PermAllow)
	if got := placeDirs(t, a); len(got) != 0 {
		t.Errorf("засев взял папку из ответа агента: %v", got)
	}

	// Создать: новый альбом из выбранной папки.
	r.mu.Lock()
	r.picked = r.other
	r.mu.Unlock()
	c = r.ask(r.createReq())
	if _, err := a.AgentAnswerCreate(r.asked(t, 2).ID, false); err != nil {
		t.Fatal(err)
	}
	c.wait(t)
	if got := placeDirs(t, a); len(got) != 0 {
		t.Errorf("«Создать» пополнило места: %v", got)
	}

	// Отказ человека.
	c = r.ask(r.addReq())
	if _, err := a.AnswerAgent(r.asked(t, 3).ID, false); err != nil {
		t.Fatal(err)
	}
	c.wait(t)
	if got := placeDirs(t, a); len(got) != 0 {
		t.Errorf("отказ пополнил места: %v", got)
	}
	// «Выбрать другую…» (AnswerAgentOther): окно ОС, выбранная папка в альбом, места те же.
	r.mu.Lock()
	r.picked = filepath.Join(filepath.Dir(r.other), "Третья")
	r.mu.Unlock()
	if err := os.Mkdir(r.picked, 0o755); err != nil {
		t.Fatal(err)
	}
	c = r.ask(r.addReq())
	if _, err := a.AnswerAgentOther(r.asked(t, 4).ID); err != nil {
		t.Fatal(err)
	}
	c.wait(t)
	if !contains(a.editor().View().Folders, r.picked) {
		t.Error("«Выбрать другую…»: папка не добавлена")
	}
	if got := placeDirs(t, a); len(got) != 0 {
		t.Errorf("AnswerAgentOther пополнил места: %v", got)
	}
}

// Засев при переходе: lastDirs.photos без agentPerms — одно место «parent», один раз; «Забыть
// места» его не возвращает; негодная последняя папка — ключ есть, мест нет (и не появятся позже).
func TestPlaceSeed(t *testing.T) {
	a, _, _ := placeApp(t)
	_, parent := mkFolder(t, "Съёмка")
	prefs.RememberDir(a.cfgDir, prefs.DirPhotos, parent)
	if prefs.LoadConfig(a.cfgDir).AgentPerms != nil {
		t.Fatal("опора: ключа agentPerms быть не должно")
	}
	// Чтение (AgentsInfo) файл не меняет.
	a.AgentsInfo()
	if prefs.LoadConfig(a.cfgDir).AgentPerms != nil {
		t.Error("чтение создало ключ agentPerms")
	}
	a.SetAgentPerm("autobuild", prefs.PermAllow)
	pl := prefs.LoadAgentPerms(a.cfgDir).Places
	if len(pl) != 1 || pl[0].Dir != parent || pl[0].How != prefs.PlaceParent {
		t.Fatalf("засев: %v", pl)
	}
	a.ForgetAgentPlaces()
	a.SetAgentPerm("autobuild", prefs.PermDeny)
	if got := placeDirs(t, a); len(got) != 0 {
		t.Errorf("засев повторился после «Забыть места»: %v", got)
	}

	// Негодная последняя папка: ключ создаётся, мест нет, и следующая последняя папка не засеется.
	b, _, e2 := placeApp(t)
	_, bad := mkFolder(t, "Съёмка")
	e2.rules.trees = []string{bad}
	prefs.RememberDir(b.cfgDir, prefs.DirPhotos, bad)
	b.SetAgentPerm("autobuild", prefs.PermAllow)
	_, good := mkFolder(t, "Съёмка")
	prefs.RememberDir(b.cfgDir, prefs.DirPhotos, good)
	b.SetAgentPerm("autobuild", prefs.PermAllow)
	if prefs.LoadConfig(b.cfgDir).AgentPerms == nil || len(placeDirs(t, b)) != 0 {
		t.Errorf("негодный засев: ключ %v, места %v", prefs.LoadConfig(b.cfgDir).AgentPerms, placeDirs(t, b))
	}

	// Нет последней папки — места пусты, ключ создан первой записью.
	c, _, _ := placeApp(t)
	c.SetAgentPerm("autobuild", prefs.PermAllow)
	if len(placeDirs(t, c)) != 0 {
		t.Error("места без lastDirs")
	}
}

// Предпросмотр засева (150): без ключа agentPerms окно уже показывает место из lastDirs.photos, а
// config.json от показа не меняется; после первой записи место то же и записано; негодная
// последняя папка — пусто; ключ есть и мест нет («Забыть места») — пусто, засева нет.
func TestPlaceSeedPreview(t *testing.T) {
	shown := func(a *App) []string {
		var out []string
		for _, p := range a.AgentsInfo().Places {
			if p.Chosen {
				t.Errorf("предпросмотр засева — «parent», не «chosen»: %v", p)
			}
			out = append(out, p.Dir)
		}
		return out
	}
	cfgBytes := func(a *App) []byte {
		b, err := os.ReadFile(filepath.Join(a.cfgDir, "config.json"))
		if err != nil {
			t.Fatal(err)
		}
		return b
	}

	a, _, _ := placeApp(t)
	_, parent := mkFolder(t, "Съёмка")
	prefs.RememberDir(a.cfgDir, prefs.DirPhotos, parent)
	before := cfgBytes(a)
	for range 2 { // повторный показ тоже ничего не пишет
		if got := shown(a); len(got) != 1 || got[0] != parent {
			t.Fatalf("предпросмотр: %v, want [%s]", got, parent)
		}
	}
	if string(cfgBytes(a)) != string(before) || prefs.LoadConfig(a.cfgDir).AgentPerms != nil {
		t.Error("показ предпросмотра изменил config.json")
	}
	a.SetAgentPerm("autobuild", prefs.PermAllow)
	if got := shown(a); len(got) != 1 || got[0] != parent {
		t.Errorf("после записи: %v", got)
	}
	if got := placeDirs(t, a); len(got) != 1 || got[0] != parent {
		t.Errorf("засев не записан: %v", got)
	}

	// Негодная последняя папка (вне разрешённых деревьев): пусто, файл не тронут.
	b, _, e2 := placeApp(t)
	_, bad := mkFolder(t, "Съёмка")
	e2.rules.trees = []string{bad}
	prefs.RememberDir(b.cfgDir, prefs.DirPhotos, bad)
	bb := cfgBytes(b)
	if got := shown(b); len(got) != 0 {
		t.Errorf("негодная папка показана: %v", got)
	}
	if string(cfgBytes(b)) != string(bb) {
		t.Error("показ изменил config.json")
	}

	// Нет lastDirs.photos: пусто.
	if c, _, _ := placeApp(t); len(shown(c)) != 0 {
		t.Error("места без lastDirs")
	}

	// Ключ есть, мест нет: засева нет, даже если lastDirs.photos годится.
	d, _, _ := placeApp(t)
	_, p4 := mkFolder(t, "Съёмка")
	prefs.RememberDir(d.cfgDir, prefs.DirPhotos, p4)
	d.SetAgentPerm("autobuild", prefs.PermAllow)
	d.ForgetAgentPlaces()
	if got := shown(d); len(got) != 0 {
		t.Errorf("засев вернулся после «Забыть места»: %v", got)
	}
}

// Засев при переходе с флажком 0.33: столбец «правки вкл.» закреплён тем же ключом.
func TestPlaceSeedKeepsMigration(t *testing.T) {
	a, _, _ := placeApp(t)
	a.updateConfig(func(c *prefs.Config) { c.Agents = &prefs.Agents{Edit: true, Level: prefs.AgentsMinLevel} })
	before := prefs.LoadAgentPerms(a.cfgDir)
	_, parent := mkFolder(t, "Съёмка")
	a.rememberDir(prefs.DirPhotos, parent) // засев раньше перезаписи
	after := prefs.LoadAgentPerms(a.cfgDir)
	for k, v := range before.Tools {
		if after.Tools[k] != v {
			t.Errorf("%s: %q → %q после засева", k, v, after.Tools[k])
		}
	}
	if !after.Edit || !prefs.AgentsOf(a.cfgDir).Edit {
		t.Errorf("флажок правок слетел: perms.edit=%v agents.edit=%v", after.Edit, prefs.AgentsOf(a.cfgDir).Edit)
	}
}

// Окно «Папка со съёмками…» открывается в первом месте или в «Изображениях», а не в lastDirs.photos
// (его пишет и ответ на вопрос агента); явный выбор — «chosen» наверху; отмена и негодная папка — места те же.
func TestChooseAgentPlaceStart(t *testing.T) {
	a, states, e := placeApp(t)
	var mu sync.Mutex
	var opts []wr.OpenDialogOptions
	pick, pickErr := "", error(nil)
	old := openDirDialog
	t.Cleanup(func() { openDirDialog = old })
	openDirDialog = func(_ context.Context, o wr.OpenDialogOptions) (string, error) {
		mu.Lock()
		defer mu.Unlock()
		opts = append(opts, o)
		return pick, pickErr
	}
	start := func() string { mu.Lock(); defer mu.Unlock(); return opts[len(opts)-1].DefaultDirectory }

	// lastDirs.photos — «приманка» агента; Изображения — подставленный HOME.
	home := realDir(t, "home")
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home) // os.UserHomeDir на Windows читает её, а не HOME
	t.Setenv("XDG_CONFIG_HOME", realDir(t, "cfg"))
	pics := filepath.Join(home, "Pictures")
	if err := os.Mkdir(pics, 0o755); err != nil {
		t.Fatal(err)
	}
	_, bait := mkFolder(t, "Приманка")
	a.rememberDir(prefs.DirPhotos, bait) // как ответ на вопрос агента: засев (пустой) идёт до записи

	if _, err := a.ChooseAgentPlace(); err != nil {
		t.Fatal(err)
	}
	if start() != pics {
		t.Errorf("мест нет: окно в %q, ждали «Изображения» %q (не lastDirs.photos %q)", start(), pics, bait)
	}

	// Отмена окна — без изменений и без события.
	n := states()
	if _, err := a.ChooseAgentPlace(); err != nil || states() != n {
		t.Errorf("отмена: err = %v, событий +%d", err, states()-n)
	}
	if got := placeDirs(t, a); len(got) != 0 {
		t.Fatalf("отмена пополнила места: %v", got)
	}

	// Явный выбор: сама папка, «chosen»; корень ~/Pictures тоже можно.
	chosen := realDir(t, "Свадьбы")
	pick = chosen
	info, err := a.ChooseAgentPlace()
	if err != nil {
		t.Fatal(err)
	}
	if len(info.Places) != 1 || info.Places[0].Dir != chosen || !info.Places[0].Chosen || states() != n+1 {
		t.Fatalf("после выбора: %+v, событий +%d", info.Places, states()-n)
	}
	// Теперь окно — в первом месте, всё ещё не в lastDirs.photos.
	pick = ""
	if _, err := a.ChooseAgentPlace(); err != nil {
		t.Fatal(err)
	}
	if start() != chosen {
		t.Errorf("есть место: окно в %q, ждали %q", start(), chosen)
	}
	// Первое место пропало с диска — берётся «Изображения».
	if err := os.RemoveAll(chosen); err != nil {
		t.Fatal(err)
	}
	if _, err := a.ChooseAgentPlace(); err != nil {
		t.Fatal(err)
	}
	if start() != pics {
		t.Errorf("место удалено: окно в %q, ждали %q", start(), pics)
	}

	// Негодная папка (системное дерево) — ошибка, места те же. Ошибка окна ОС — тоже.
	bad := realDir(t, "Системное")
	e.rules.trees = []string{bad}
	pick = bad
	before := placeDirs(t, a)
	if _, err := a.ChooseAgentPlace(); !errors.Is(err, errPlaceBad) {
		t.Errorf("негодная папка: err = %v", err)
	}
	pick, pickErr = "", errors.New("окно не открылось")
	if _, err := a.ChooseAgentPlace(); err == nil {
		t.Error("ошибка окна ОС потеряна")
	}
	if got := placeDirs(t, a); len(got) != len(before) {
		t.Errorf("места изменились: %v → %v", before, got)
	}
}
