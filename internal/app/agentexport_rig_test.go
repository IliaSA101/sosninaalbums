package app

// Стенд выгрузки по просьбе агента (152 Т3б): настоящий App, альбом «Петровы» с разворотами и фото, корень
// выгрузок в настройках, окна ОС и вопросы «Да / Нет» подменены. Тесты: agentexport_flow_test.go (ход),
// agentexport_dir_test.go (папка), agentexport_test.go (правила и тексты).

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/app/agents"
	"github.com/IliaSA101/sosninaalbums/internal/app/exporter"
	"github.com/IliaSA101/sosninaalbums/internal/osx"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// realTemp — временная папка настоящим путём: правило папки отдаёт путь после ссылок и подмены тома, а на Windows
// TempDir раннера в 8.3 (RUNNER~1) — сравнивать «показанное» с «созданным» можно только в одной форме.
func realTemp(t *testing.T) string {
	t.Helper()
	p, err := filepath.EvalSymlinks(testkit.TempDir(t))
	if err == nil {
		p, err = osx.RealPath(p)
	}
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// exportRig — askRig с альбомом из n разворотов (по фото) и корнем выгрузок.
type exportRig struct {
	*askRig
	ed    *album.Editor
	src   string   // папка фото альбома
	root  string   // корень выгрузок (config)
	file  string   // файл альбома
	yes   bool     // ответ на «Да / Нет» окон выгрузки
	warns []string // тексты этих вопросов
	osWin []string // заголовки окон ОС «Куда выгрузить»
	osDir string   // что вернёт окно ОС; "" — отмена
}

func newExportRig(t *testing.T, n int) *exportRig { return newExportRigWith(t, n, nil) }

// newExportRigWith — стенд, у которого проект перед открытием правит tune (nil — как есть).
func newExportRigWith(t *testing.T, n int, tune func(*model.Project)) *exportRig {
	t.Helper()
	r := &exportRig{askRig: newAskRig(t), yes: true}
	base := realTemp(t)
	r.src, r.root, r.file = filepath.Join(base, "Фото"), filepath.Join(base, "Выгрузки"), filepath.Join(base, "Петровы.salbum")
	for _, d := range []string{r.src, r.root} {
		if err := os.Mkdir(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	p := model.New()
	p.Settings.PageW, p.Settings.PageH = 60, 40
	for i := range n {
		id := fmt.Sprintf("p%d", i)
		p.Photos[id] = model.PhotoRef{Path: testkit.JPEG(t, r.src, id+".jpg", 60, 40), W: 60, H: 40}
		p.Spreads = append(p.Spreads, &model.Spread{ID: fmt.Sprintf("s%d", i), Photos: []string{id}})
	}
	r.a.askM().rules = func() *folderRules { // настоящий диск: «нет папки» должно давать «создать», а не «есть»
		fr := permissiveRules()
		fr.disk = diskProbe
		return fr
	}
	if tune != nil {
		tune(p)
	}
	r.ed = mustEditor(t, p, r.file)
	r.a.ed.Store(r.ed)
	r.a.rememberDir(prefs.DirExportRoot, r.root)
	oldAsk, oldPick := askYes, pickExportDir
	t.Cleanup(func() { askYes, pickExportDir = oldAsk, oldPick })
	askYes = func(_ *App, msg string) (bool, error) { r.warns = append(r.warns, msg); return r.yes, nil }
	pickExportDir = func(_ *App, title string) (string, error) {
		r.osWin = append(r.osWin, title)
		if r.osDir == "" {
			return "", nil
		}
		return r.osDir, os.MkdirAll(r.osDir, 0o755)
	}
	return r
}

// addBlank добавляет пустой разворот в конец: выгрузка спросит «пустые развороты — выгружать?» (askYes).
func (r *exportRig) addBlank(t *testing.T) {
	t.Helper()
	if _, err := r.ed.NewSpread(-1, nil); err != nil {
		t.Fatal(err)
	}
}

// req — просьба выгрузить альбом r.ed в 72 dpi (фото 60×40 мм при 300 dpi вызвали бы «мало разрешения»).
func (r *exportRig) req(hint string) agents.AskReq {
	return agents.AskReq{Kind: agents.AskExport, Client: "Claude Code", Key: r.a.askKey(r.ed), Rev: r.ed.View().Rev,
		DPI: 72, Print: "plain", Hint: hint}
}

// question задаёт вопрос и ждёт его окно.
func (r *exportRig) question(t *testing.T, req agents.AskReq) (*askCall, AgentAsk) {
	t.Helper()
	n := len(r.evOf("agents:ask"))
	c := r.ask(req)
	return c, r.asked(t, n+1)
}

// finish ждёт конца фона выгрузки (флаг службы снят) и возвращает ход для get_album.
func (r *exportRig) finish(t *testing.T) agents.ExportStatus {
	t.Helper()
	waitUntil(t, "выгрузка закончена", func() bool {
		busy, _ := r.a.exp.Busy()
		return !busy && r.a.agentExportState(r.ed.ID()).State != exporter.StateRunning
	})
	return r.a.agentExportState(r.ed.ID())
}

// jpgs — файлы *.jpg папки, как их видит человек.
func jpgs(t *testing.T, dir string) []string {
	t.Helper()
	m, err := filepath.Glob(filepath.Join(dir, "*.jpg"))
	if err != nil {
		t.Fatal(err)
	}
	for i := range m {
		m[i] = filepath.Base(m[i])
	}
	return m
}
