package app

// Тихое открытие: доправка по ревью и аудиту 160 Т2 — «..» после ссылки, ссылки внутри папки, срок и бюджет, сумма
// фото, родители, шрифты, Bounded вместо OSDisk, «Всегда» без окна согласия.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/app/agents"
	"github.com/IliaSA101/sosninaalbums/internal/decor"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
	"github.com/IliaSA101/sosninaalbums/internal/text"
)

// rawAlbum пишет файл альбома из FileProject как есть (пути не очищаются, как у чужого файла) и кладёт в «Последние».
func (r *oqRig) rawAlbum(t *testing.T, name string, f salbum.FileProject, extra ...map[string]any) string {
	t.Helper()
	f.Version, f.Settings = model.FormatVersion, model.DefaultSettings()
	f.Spreads = []*model.Spread{}
	if f.Photos == nil {
		f.Photos = map[string]salbum.FilePhoto{}
	}
	data, err := json.Marshal(f)
	if err != nil {
		t.Fatal(err)
	}
	if len(extra) > 0 { // поля, которых нет в экспортируемых типах (отметки)
		var m map[string]any
		if err := json.Unmarshal(data, &m); err != nil {
			t.Fatal(err)
		}
		for k, v := range extra[0] {
			m[k] = v
		}
		if data, err = json.Marshal(m); err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(r.dir, name+projectExt)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	r.a.rememberProject(path)
	return path
}

func loose(path string) []salbum.FileLoose {
	return []salbum.FileLoose{{ID: "0123456789ab", Files: []salbum.FileFolder{{Path: path}}}}
}

// TestOpenQuietDotDotAfterLink (аудит Б1): «..» после папки-ссылки в абсолютном пути. По тексту путь внутри места,
// на диске — выше ссылки. Тихо нельзя, диск вне настоящих мест не трогается (шпион — по EvalSymlinks).
func TestOpenQuietDotDotAfterLink(t *testing.T) {
	r := newOQRig(t)
	outParent := realPath(t, filepath.Join(testkit.UserTemp(t), "Скрины"))
	out := filepath.Join(outParent, "Вложенная")
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	testkit.JPEG(t, outParent, "secret.jpg", 40, 30)
	if os.Symlink(out, filepath.Join(r.shoot, "НАС")) != nil {
		t.Skip("ссылку создать нельзя")
	}
	sneaky := r.shoot + string(filepath.Separator) + "НАС" + string(filepath.Separator) + ".." + string(filepath.Separator) + "secret.jpg"
	alb := r.rawAlbum(t, "Обход", salbum.FileProject{Loose: loose(sneaky)})
	var touched atomic.Int32
	oldDisk := quietDisk
	t.Cleanup(func() { quietDisk = oldDisk })
	quietDisk = func(d salbum.Disk) salbum.Disk {
		return salbum.Spy(d, func(op, p string) {
			if op == "lib" {
				return
			}
			if resolved, err := filepath.EvalSymlinks(p); err == nil && strings.HasPrefix(resolved, outParent) {
				touched.Add(1)
			}
		})
	}
	// Строка не канонична — place до любого обращения к диску (без этой проверки спасает только обход папки: link).
	r.asks(t, "dotdot", alb, whyPlace)
	if touched.Load() != 0 {
		t.Errorf("тронут диск за пределами мест: %d раз", touched.Load())
	}
	// Строка с «.» без всякой ссылки: тихо нельзя (текст проверки мест и путь на диске должны совпадать).
	plain := r.mkdir(t, "Чистая", "a.jpg")
	odd := plain + string(filepath.Separator) + "." + string(filepath.Separator) + "a.jpg"
	r.asks(t, "non_canonical", r.rawAlbum(t, "Негодный", salbum.FileProject{Loose: loose(odd)}), whyPlace)
}

// TestOpenQuietLinkInside (аудит В1): ссылка-файл и ссылка-папка внутри папки альбома — link; то же у тихой папки агента (150).
func TestOpenQuietLinkInside(t *testing.T) {
	r := newOQRig(t)
	outside := testkit.JPEG(t, filepath.Join(testkit.UserTemp(t), "Скрины"), "s.jpg", 40, 30)
	dir := r.mkdir(t, "СоСсылкой", "a.jpg")
	if os.Symlink(outside, filepath.Join(dir, "x.jpg")) != nil {
		t.Skip("ссылку создать нельзя")
	}
	r.asks(t, "ссылка-файл", r.album(t, "СсылкаФайл", dir), whyLink)
	n := len(r.evOf("agents:ask"))
	c := r.ask(r.addQuiet(dir))
	if ev := r.refuse(t, c, n+1); ev.Why != whyLink {
		t.Errorf("тихая папка 150: Why = %q, ждали link", ev.Why)
	}
}

// TestOpenQuietParentLink: родитель фото внутри корня сам оказался ссылкой — link, а не gone (ревью п. 1).
func TestOpenQuietParentLink(t *testing.T) {
	r := newOQRig(t)
	target := r.mkdir(t, "Настоящая", "a.jpg")
	if os.Symlink(target, filepath.Join(r.shoot, "Ссылка")) != nil {
		t.Skip("ссылку создать нельзя")
	}
	alb := r.rawAlbum(t, "ПодСсылкой", salbum.FileProject{Folders: []salbum.FileFolder{{Path: r.shoot}}},
		map[string]any{"marks": []map[string]any{{"path": filepath.Join(r.shoot, "Ссылка", "a.jpg"), "rating": 3}}}) // отметка: строка EachPath
	r.asks(t, "link_below", alb, whyLink)
}

// TestOpenQuietLimits: много родителей, сумма фото, зависший корень, бюджет полного разбора.
func TestOpenQuietLimits(t *testing.T) {
	r := newOQRig(t)
	files := make([]salbum.FileFolder, 0, openMaxParents+1)
	for i := range openMaxParents + 1 { // отдельные фото в разных папках: у каждого свой родитель
		files = append(files, salbum.FileFolder{Path: filepath.Join(r.place, fmt.Sprintf("Д%d", i), "a.jpg")})
	}
	r.asks(t, "many_parents", r.rawAlbum(t, "Родители", salbum.FileProject{Loose: []salbum.FileLoose{{ID: "0123456789ab", Files: files}}}), whyBig)

	t.Run("max_photos", func(t *testing.T) {
		old := quietOpenMaxPhotos
		t.Cleanup(func() { quietOpenMaxPhotos = old })
		quietOpenMaxPhotos = 1 // в r.shoot два фото
		r.asks(t, "max_photos", r.target, whyBig)
	})
	t.Run("slow_root", func(t *testing.T) {
		oldB, oldV := quietOpenBudget, quietVirtual
		hang := make(chan struct{})
		t.Cleanup(func() { close(hang); quietOpenBudget, quietVirtual = oldB, oldV })
		quietOpenBudget = testkit.Limit(200 * time.Millisecond)
		quietVirtual = func(string) bool { <-hang; return false }
		start := time.Now()
		r.asks(t, "slow_root", r.target, whyBig)
		if d := time.Since(start); d > testkit.Limit(3*time.Second) {
			t.Errorf("корень держал вызов %v", d)
		}
	})
	t.Run("budget", func(t *testing.T) {
		oldB, oldD := quietOpenBudget, quietDisk
		t.Cleanup(func() { quietOpenBudget, quietDisk = oldB, oldD })
		quietOpenBudget = testkit.Limit(500 * time.Millisecond)
		quietDisk = func(d salbum.Disk) salbum.Disk { time.Sleep(quietOpenBudget + 100*time.Millisecond); return d } // срок вышел до разбора
		r.asks(t, "budget", r.target, whyBig)
	})
}

// TestOpenQuietNoneAndHint: мест нет — place; слова агента без найденного ключа — hint (тихо нельзя ни в одном случае).
func TestOpenQuietNoneAndHint(t *testing.T) {
	r := newOQRig(t)
	q := agents.AskReq{Kind: agents.AskOpen, Client: "Claude Code", Hint: "Ивановых", Always: true}
	n := len(r.evOf("agents:ask"))
	c := r.ask(q)
	if ev := r.refuse(t, c, n+1); ev.Why != whyHint {
		t.Errorf("hint_missed: Why = %q", ev.Why)
	}
	prefs.UpdateAgentPerms(r.a.cfgDir, func(p *prefs.AgentPerms) { p.Places = nil })
	r.asks(t, "place_none", r.target, whyPlace)
}

// TestOpenQuietBoundedGuard: полный разбор идёт через Bounded: искатель библиотеки отдаёт файл вне мест и вне папки
// настроек — к нему нет ни одного обращения. Заменить Bounded на OSDisk — тест краснеет (обращение к чужому пути).
func TestOpenQuietBoundedGuard(t *testing.T) {
	r := newOQRig(t)
	outside := testkit.JPEG(t, filepath.Join(testkit.UserTemp(t), "Чужое"), "lib.jpg", 40, 30)
	salbum.SetLibFinder(func(string) (string, bool) { return outside, true })
	t.Cleanup(func() { salbum.SetLibFinder(nil) })
	var touched atomic.Int32
	oldDisk := quietDisk
	t.Cleanup(func() { quietDisk = oldDisk })
	quietDisk = func(d salbum.Disk) salbum.Disk {
		return salbum.Spy(d, func(op, p string) {
			if op != "lib" && p == outside {
				touched.Add(1)
			}
		})
	}
	alb := r.rawAlbum(t, "Своё", salbum.FileProject{
		Folders: []salbum.FileFolder{{Path: r.shoot}},
		Photos:  map[string]salbum.FilePhoto{"bbbbbbbbbbbb": {PhotoRef: model.PhotoRef{Lib: "my:0123456789abcdef"}}},
	})
	if c := r.quietly(t, alb); c.err != nil || c.res.Why != agents.WhyQuiet {
		t.Fatalf("итог: %+v, %v", c.res, c.err)
	}
	if touched.Load() != 0 {
		t.Errorf("обращений к чужому пути библиотеки: %d", touched.Load())
	}
}

// TestOpenQuietFonts: альбому нужны шрифты из его папки — fonts, библиотека не меняется. Контроль: openPath их ставит.
func TestOpenQuietFonts(t *testing.T) {
	r := newOQRig(t)
	t.Cleanup(func() { text.SetCustom(nil, nil) })
	r.a.openFonts()
	osw, _ := familyOf(t, "Oswald-Regular.ttf")
	path := collectedAlbum(t, []string{osw}, map[string]string{"z-Oswald-Regular.ttf": "Oswald-Regular.ttf"})
	r.a.rememberProject(path)
	lib := filepath.Join(r.a.cfgDir, decor.RootName, fontsDir)
	count := func() int { list, _ := os.ReadDir(lib); return len(list) }
	before := count()
	r.asks(t, "fonts", path, whyFonts)
	if count() != before || text.Available(osw) {
		t.Fatalf("тихое открытие поставило шрифты: файлов %d → %d", before, count())
	}
	if _, err := r.a.openPath(path); err != nil || !text.Available(osw) || count() == before {
		t.Errorf("контроль: openPath не поставил шрифт (%v)", err)
	}
}

// TestOpenAllowIsAllow (160-Т6-1): «Всегда» у open_album — настоящее «Всегда». Служба видит allow (Host.PermLevel — тихий
// слот берётся), вызов по ключу кончается тихим открытием без вопроса; «Спрашивать» тот же вызов превращает в окно.
func TestOpenAllowIsAllow(t *testing.T) {
	r := newOQRig(t)
	if mode, _, _ := r.a.agentPermLevel(toolOpenAlbum); mode != prefs.PermAllow {
		t.Errorf("служба видит режим %q, ждали allow", mode)
	}
	c := r.quietly(t, r.target)
	if c.err != nil || c.res.Why != agents.WhyQuiet || r.a.editor().Path() != r.target {
		t.Fatalf("«Всегда» не открыло тихо: %+v, %v", c.res, c.err)
	}
	if n := len(r.evOf("agents:ask")); n != 0 {
		t.Errorf("вопрос при «Всегда»: %d", n)
	}
	r.setMode(toolOpenAlbum, prefs.PermAsk)
	if mode, _, _ := r.a.agentPermLevel(toolOpenAlbum); mode != prefs.PermAsk {
		t.Errorf("после «Спрашивать» служба видит %q", mode)
	}
}

// TestOpenQuietDotDotOutsideDry (аудит Н1): строки, которых нет в сухом перечне (фото без разворота, свёрнутые вложенные),
// проверяются так же строго — путь с «..» (в том числе от папки настроек) диск не трогает.
func TestOpenQuietDotDotOutsideDry(t *testing.T) {
	r := newOQRig(t)
	secret := filepath.Join(filepath.Dir(r.a.cfgDir), "outside", "secret.jpg")
	testkit.JPEG(t, filepath.Dir(secret), "secret.jpg", 40, 30)
	sneaky := r.a.cfgDir + string(filepath.Separator) + ".." + string(filepath.Separator) + "outside" + string(filepath.Separator) + "secret.jpg"
	nested := r.shoot + string(filepath.Separator) + "Sub" + string(filepath.Separator) + ".." + string(filepath.Separator) + "Кадр_0.jpg"
	alb := r.rawAlbum(t, "БезРазворота", salbum.FileProject{
		Folders: []salbum.FileFolder{{Path: r.shoot}},
		Loose:   loose(nested),                                                                         // внутри папки: FoldSources сворачивает
		Photos:  map[string]salbum.FilePhoto{"cccccccccccc": {PhotoRef: model.PhotoRef{Path: sneaky}}}, // без разворота: PruneRefs
	})
	var touched atomic.Int32
	oldAllowed := quietAllowed
	t.Cleanup(func() { quietAllowed = oldAllowed })
	quietAllowed = func(f func(string) bool) func(string) bool { // диск трогается там, где allowed ответил «да»
		return func(p string) bool {
			ok := f(p)
			if ok && filepath.Clean(p) != p {
				touched.Add(1)
			}
			return ok
		}
	}
	c := r.ask(r.req(alb))
	if ev := r.evOf("agents:ask"); len(ev) > 0 {
		_, _ = r.a.AnswerAgent(ev[0].(AgentAsk).ID, false)
	}
	c.wait(t)
	if touched.Load() != 0 {
		t.Errorf("диск тронут по путям с «..»: %d раз", touched.Load())
	}
}

// TestOpenQuietFullKnownClean: openFullWhy не принимает путь с «..» даже под папкой настроек (аудит Н1).
func TestOpenQuietFullKnownClean(t *testing.T) {
	r := newOQRig(t)
	pl := &openPlan{rules: r.a.askM().rules(), rows: map[string]bool{}, checked: map[string]bool{}, cfgDir: filepath.Clean(r.a.cfgDir) + string(filepath.Separator)}
	full := model.New()
	full.Loose = []model.LooseAdd{{ID: "0123456789ab", Files: []model.LooseFile{{Path: pl.cfgDir + "x.jpg"}}}}
	if why := r.a.openFullWhy(pl, full); why != "" {
		t.Fatalf("чистый путь в папке настроек: %q", why)
	}
	full.Loose[0].Files[0].Path = pl.cfgDir + ".." + string(filepath.Separator) + "x.jpg"
	if why := r.a.openFullWhy(pl, full); why != whyPlace {
		t.Errorf("путь с «..»: %q", why)
	}
}
