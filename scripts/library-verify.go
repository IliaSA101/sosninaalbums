//go:build ignore

// library-verify проверяет набор библиотеки глазами программы (спека 129, design §3): архив читает
// pack.Open, ставят StagePack и InstallPack в пустую временную библиотеку, каждый шаблон разворота
// строит album.PreviewTpl на страницах 20×30, 30×30 и 40×30 см, рамки рисует ServeFramePic.
// Инструмент разработчика, не функция программы.
//
//	go run scripts/library-verify.go scripts/library-verify-sheets.go scripts/library-verify-looks.go [-app x.y.z] [-with донор.zip]... [-out папка] [-preview лист.png] набор.zip...
//	go run scripts/library-verify.go scripts/library-verify-sheets.go scripts/library-verify-looks.go -pull <Библиотека> -pack <id> -out папка
//	go run scripts/library-verify.go scripts/library-verify-sheets.go scripts/library-verify-looks.go scripts/library-verify-sprite.go -sprite набор.zip -out папка [-pick pick.json] [-with донор.zip]...
//
// Четыре файла: лимит 600 строк на файл (рамки, листы и -pull — во втором, макеты альбома 0.28 — в третьем,
// лист образцов набора для каталога (спека 144) — в четвёртом, тот же package main; без третьего и четвёртого
// скрипт собирается и для 0.26/0.27, макеты тогда не проверяются, а -sprite отказывает).
//
// Версия программы для minApp: -app x.y.z, иначе info.productVersion из wails.json (в текущей папке
// или выше — дерево сборки); minApp набора новее — провал. Набор для 0.27.0 в дереве, где wails.json
// ещё 0.26.0, проверяется с -app 0.27.0.
//
// Пути — абсолютные (paths.CheckLocalPath). Наборы ставятся по порядку в одну пустую библиотеку
// (-tmp, по умолчанию $GOTMPDIR или .gotmp): на машине с наборами checkTplLibs слеп. -with — набор-
// донор для дополнения (ставится первым, сам не проверяется). Проверка набора:
//   - Skipped и Ask пусты, окна рамок найдены;
//   - шаблон: ключей библиотеки нет 0, LowDPI 0, слои не потеряны, места и стикеры целиком внутри
//     области листа (-tol мм на округление);
//   - пропорция мест на 30×30: меньше 80 % мест, где кадр 3:2 / 2:3 режется ≤ 15 %, — WARN в итоге
//     (имя шаблона и доля), не провал, код выхода прежний; места-фигуры не считаются;
//   - макет альбома (вид album, library-verify-looks.go): missing 0, LowDPI 0, места внутри области на
//     20×30, 30×30 и 40×30; WARN, если в шаге меньше 80 % мест 3:2 / 2:3 на эталоне Ratio шага;
//   - рамка: окно прозрачно, рамка вровень с краем места 20×20, 20×30, 30×20 см; «тянется» % (design
//     §2.1) выше 20 — флаг-подсказка для отбора, не провал.
//
// Провалы копятся и печатаются в конце (код 1), независимые блоки друг друга не прерывают. С -out:
// frames.png (рамки на шахматке — по нему идёт отбор) и fail-*.png (шаблон на размере, где провал);
// -preview — общий лист «фоны / стикеры / рамки / развороты» и строка на макет альбома «Обложка, 1…N».
// -pull забирает из Развороты/ библиотеки шаблоны, все ключи которых из набора -pack (правки Марины), в -out/*.json.
//
// Скрипт опирается только на API, общее для 0.26.0 и dev: для проверки 0.26 он копируется в
// git worktree метки v0.26.0 и собирается там (флага версии нет). Под ignore precheck его не
// собирает — собирает TestLibraryVerifyScript.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"image"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/decor"
	"github.com/IliaSA101/sosninaalbums/internal/decor/pack"
	"github.com/IliaSA101/sosninaalbums/internal/export"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
	"github.com/IliaSA101/sosninaalbums/internal/render"
	"github.com/disintegration/imaging"
)

const (
	rowH         = 200  // высота плитки листов, px
	cellH        = 220  // высота ячейки листа рамок, px
	flowW        = 3200 // ширина строки общего листа, px
	maxTiles     = 400  // плиток одного вида на листе
	maxFrameRows = 64
	framePx      = 360 // ширина ответа ServeFramePic
	stretchFlag  = 20.0
)

// size — страница или место, мм.
type size struct {
	name string
	w, h float64
}

var (
	pageSizes = []size{{"20×30", 200, 300}, {"30×30", 300, 300}, {"40×30", 400, 300}}
	spotSizes = []size{{"20×20", 200, 200}, {"20×30", 200, 300}, {"30×20", 300, 200}}
)

type multi []string

func (m *multi) String() string     { return strings.Join(*m, ",") }
func (m *multi) Set(v string) error { *m = append(*m, v); return nil }

// verifier — прогон: библиотека, накопленные провалы, проверенные наборы.
type verifier struct {
	s     *decor.Store
	w     io.Writer
	out   string
	tol   float64
	fails []string
	warns []string // предупреждения: в итог попадают, код выхода не меняют
	ctx   string
	done  []decor.PackInfo
	app   string // версия программы для minApp (x.y.z)
	// Последний поставленный набор: макетов альбома в манифесте и его minApp (проверяет lookCheck).
	albums int
	minApp string
}

// lookCheck — проверка макетов альбома набора (library-verify-looks.go, вид album с 0.28); без этого
// файла (сборка для 0.26/0.27) макеты не проверяются.
var lookCheck func(v *verifier, path string, info decor.PackInfo)

// spriteRun — лист образцов набора (-sprite, library-verify-sprite.go, спека 144); без этого файла пуст.
var spriteRun func(v *verifier, zipPath, pickPath string)

// spriteUsage — проверка ключей -sprite/-pick (текст ошибки или ""); library-verify-sprite.go подменяет её своей.
var spriteUsage = func(sprite, pick, _ string, _ int) string {
	if sprite != "" || pick != "" {
		return "-sprite и -pick: собирайте с library-verify-sprite.go"
	}
	return ""
}

// lookRows — строки листа превью для макетов набора: по строке на макет «Обложка, 1…N» (то же —
// library-verify-looks.go).
var lookRows func(v *verifier, info decor.PackInfo) []image.Image

func (v *verifier) failf(format string, a ...any) {
	msg := fmt.Sprintf(format, a...)
	if v.ctx != "" {
		msg = v.ctx + ": " + msg
	}
	v.fails = append(v.fails, msg)
	fmt.Fprintln(v.w, "ПРОВАЛ", msg)
}

// warnf — предупреждение (решение 0.27-131-24): печатается сразу и повторяется в итоге, код выхода прежний.
func (v *verifier) warnf(format string, a ...any) {
	msg := fmt.Sprintf(format, a...)
	if v.ctx != "" {
		msg = v.ctx + ": " + msg
	}
	v.warns = append(v.warns, msg)
	fmt.Fprintln(v.w, "  WARN", msg)
}

// guard — блок проверки: паника становится провалом, остальные блоки идут дальше.
func (v *verifier) guard(name string, fn func()) {
	defer func() {
		if r := recover(); r != nil {
			v.failf("%s: паника %v", name, r)
		}
	}()
	fn()
}

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("library-verify", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var with multi
	fs.Var(&with, "with", "набор-донор (zip), повторяемый")
	out := fs.String("out", "", "папка листов и снимков")
	prev := fs.String("preview", "", "общий лист (png)")
	tmp := fs.String("tmp", "", "где держать временную библиотеку (по умолчанию $GOTMPDIR или .gotmp)")
	tol := fs.Float64("tol", 0.5, "допуск выхода за область листа, мм")
	pull := fs.String("pull", "", "библиотека, откуда забрать шаблоны")
	packID := fs.String("pack", "", "id набора для -pull")
	app := fs.String("app", "", "версия программы x.y.z для minApp (по умолчанию wails.json дерева сборки)")
	sprite := fs.String("sprite", "", "набор (zip), для которого собрать лист образцов в -out")
	pick := fs.String("pick", "", "свой отбор клеток листа (json) для -sprite")
	if fs.Parse(args) != nil {
		return 2
	}
	usage := func(msg string) int { fmt.Fprintln(stderr, "library-verify:", msg); return 2 }
	for _, p := range append(slices.Clone(with), append(fs.Args(), *out, *prev, *pull, *sprite, *pick)...) {
		if p == "" {
			continue
		}
		if err := paths.CheckLocalPath(p); err != nil {
			return usage(err.Error())
		}
	}
	if *pull != "" {
		if *packID == "" || *out == "" {
			return usage("для -pull нужны -pack и -out")
		}
		if err := pullTpls(stdout, *pull, *packID, *out); err != nil {
			fmt.Fprintln(stderr, "library-verify:", err)
			return 1
		}
		return 0
	}
	if msg := spriteUsage(*sprite, *pick, *out, fs.NArg()); msg != "" {
		return usage(msg)
	}
	if fs.NArg() == 0 && *sprite == "" {
		return usage("нужен хотя бы один набор (zip)")
	}
	appV, err := appVersion(*app)
	if err != nil {
		return usage(err.Error())
	}
	base := cmpOr(*tmp, os.Getenv("GOTMPDIR"), ".gotmp") // пустой путь не бывает: иначе MkdirTemp ушёл бы в системный temp
	if err := os.MkdirAll(base, 0o755); err != nil {
		return usage(err.Error())
	}
	dir, err := os.MkdirTemp(base, "libverify-")
	if err != nil {
		return usage(err.Error())
	}
	defer os.RemoveAll(dir)
	if *out != "" {
		if err := os.MkdirAll(*out, 0o755); err != nil {
			return usage(err.Error())
		}
	}
	v := &verifier{s: decor.Open(dir), w: stdout, out: *out, tol: *tol, app: appV}
	fmt.Fprintln(stdout, "версия программы для minApp:", appV)
	for _, z := range with {
		fmt.Fprintln(stdout, "донор:", filepath.Base(z))
		v.ctx = filepath.Base(z) + " (донор)"
		v.guard("донор", func() { v.install(z) })
	}
	if *sprite != "" {
		v.ctx = filepath.Base(*sprite)
		v.guard("лист образцов", func() { spriteRun(v, *sprite, *pick) })
	}
	for _, z := range fs.Args() {
		v.ctx = filepath.Base(z)
		v.guard("набор", func() { v.verifyPack(z) })
	}
	v.ctx = ""
	if *prev != "" {
		v.guard("лист", func() { v.previewSheet(*prev) })
	}
	for _, p := range v.s.Packs() {
		fmt.Fprintf(stdout, "в библиотеке: %s v%d (%s)\n", p.ID, p.Version, p.Origin)
	}
	if len(v.warns) > 0 {
		fmt.Fprintf(stdout, "ПРЕДУПРЕЖДЕНИЙ %d:\n", len(v.warns))
		for _, w := range v.warns {
			fmt.Fprintln(stdout, " - WARN", w)
		}
	}
	if len(v.fails) > 0 {
		fmt.Fprintf(stdout, "ПРОВАЛОВ %d:\n", len(v.fails))
		for _, f := range v.fails {
			fmt.Fprintln(stdout, " -", f)
		}
		return 1
	}
	fmt.Fprintln(stdout, "OK")
	return 0
}

var semver = regexp.MustCompile(`^\d+\.\d+\.\d+$`)

// appVersion — версия программы для сверки minApp: явный -app либо info.productVersion из ближайшего
// wails.json (текущая папка и выше). Пусто и «dev» не годятся: тогда minApp не сверялся бы вовсе.
func appVersion(flagVal string) (string, error) {
	if flagVal != "" {
		if !semver.MatchString(flagVal) {
			return "", fmt.Errorf("-app %q: нужна версия x.y.z", flagVal)
		}
		return flagVal, nil
	}
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		data, err := os.ReadFile(filepath.Join(dir, "wails.json"))
		if err == nil {
			var w struct {
				Info struct {
					ProductVersion string `json:"productVersion"`
				} `json:"info"`
			}
			if json.Unmarshal(data, &w) != nil || !semver.MatchString(w.Info.ProductVersion) {
				return "", fmt.Errorf("%s: нет info.productVersion вида x.y.z; укажите -app", filepath.Join(dir, "wails.json"))
			}
			return w.Info.ProductVersion, nil
		}
		up := filepath.Dir(dir)
		if up == dir {
			return "", errors.New("нет wails.json в текущей папке и выше: запускайте из корня дерева или укажите -app x.y.z")
		}
		dir = up
	}
}

func cmpOr(vals ...string) string {
	for _, s := range vals {
		if s != "" {
			return s
		}
	}
	return ""
}

// install открывает архив и ставит набор в библиотеку; провалы пишет в v. false — не поставлен.
func (v *verifier) install(path string) (decor.PackInfo, bool) {
	f, _, err := library.OpenLocal(path)
	if err != nil {
		v.failf("файл не открывается: %v", err)
		return decor.PackInfo{}, false
	}
	arc, err := pack.OpenFile(f, v.app)
	if err != nil {
		v.failf("pack.Open: %v", err)
		return decor.PackInfo{}, false
	}
	defer arc.Close()
	m := arc.Manifest
	v.albums, v.minApp = 0, m.MinApp
	for _, it := range m.Items {
		if it.Kind == "album" {
			v.albums++
		}
	}
	for _, sk := range m.Skipped {
		v.failf("манифест пропустил «%s»: %s", sk.File, sk.Why)
	}
	dm := decor.PackManifest{ID: m.ID, Version: m.Version, Title: m.Title, About: m.About, Author: m.Author,
		License: m.License, Source: m.Source, MinApp: m.MinApp, Items: make([]decor.PackItem, len(m.Items))}
	for i, it := range m.Items {
		dm.Items[i] = decor.PackItem{File: it.File, Kind: it.Kind, Theme: it.Theme, Name: it.Name, Size: it.Size}
	}
	st, err := v.s.StagePack(dm, arc, decor.OriginFile, arc.SHA256)
	if st != nil {
		defer st.Close()
		for _, sk := range st.Skipped {
			v.failf("StagePack пропустил «%s»: %s", sk.Name, sk.Why)
		}
		if st.Ask != "" {
			v.failf("StagePack спрашивает %q: набор с таким id уже стоит", st.Ask)
		}
	}
	if err != nil {
		v.failf("StagePack: %v", err)
		return decor.PackInfo{}, false
	}
	if st.Ask != "" {
		return decor.PackInfo{}, false
	}
	info, err := v.s.InstallPack(st, nil)
	if err != nil {
		v.failf("InstallPack: %v", err)
		return decor.PackInfo{}, false
	}
	fmt.Fprintf(v.w, "%s: %s v%d — фонов %d, стикеров %d, рамок %d, разворотов %d, %d байт, minApp %s\n",
		filepath.Base(path), info.Title, info.Version, info.Bg, info.Sticker, info.Frame, info.Spread, info.Bytes, m.MinApp)
	return info, true
}

// titleClash — у другого установленного набора то же название: плитки каталога несут только название
// (поля с id нет), и предметы донора и дополнения смешались бы. Провал, набор дальше не проверяется.
func (v *verifier) titleClash(info decor.PackInfo) bool {
	for _, p := range v.s.Packs() {
		if p.ID != info.ID && p.Title == info.Title {
			v.failf("название «%s» совпадает с набором %s: плитки различаются по названию, дайте наборам разные title", info.Title, p.ID)
			return true
		}
	}
	return false
}

// tiles — плитки вида kind, принадлежащие набору (по названию; одинаковые названия отсекает titleClash).
func (v *verifier) tiles(kind string, info decor.PackInfo) []decor.Item {
	cat, err := v.s.Catalog(kind, nil, 300, 300)
	if err != nil {
		v.failf("каталог %s: %v", kind, err)
		return nil
	}
	return slices.DeleteFunc(cat.Items, func(it decor.Item) bool { return it.Pack != info.Title })
}

func (v *verifier) verifyPack(path string) {
	info, ok := v.install(path)
	if !ok {
		return
	}
	v.done = append(v.done, info)
	if v.titleClash(info) {
		return
	}
	spreads := v.tiles(decor.KindSpread, info)
	if len(spreads) != info.Spread {
		v.failf("плиток разворотов %d, в наборе %d", len(spreads), info.Spread)
	}
	for _, it := range spreads {
		v.ctx = filepath.Base(path) + " · " + it.Name
		v.guard("шаблон", func() { v.checkTpl(it) })
	}
	v.ctx = filepath.Base(path)
	v.guard("рамки", func() { v.checkFrames(info) })
	if lookCheck != nil {
		v.guard("макеты", func() { lookCheck(v, path, info) })
		v.ctx = filepath.Base(path)
	} else if v.albums > 0 {
		v.failf("макеты альбома не проверены: соберите с library-verify-looks.go")
	}
}

// libsOf — картинки библиотеки шаблона и число ключей, которых на этом ПК нет.
func (v *verifier) libsOf(t model.SpreadTpl) (libs map[string]model.PhotoInfo, missing int) {
	libs = map[string]model.PhotoInfo{}
	for _, key := range tplKeys(t) {
		if _, have := libs[key]; have {
			continue
		}
		info, ok := v.s.LibInfo(key)
		if !ok {
			missing++
			continue
		}
		libs[key] = model.PhotoInfo{Path: info.Path, W: info.W, H: info.H, Size: info.Size}
	}
	return libs, missing
}

// tplKeys — ключи библиотеки в шаблоне: фоны страниц, фон разворота, стикеры и рамочки мест
// (borders, 131) — как app.tplKeys и decor.tplLibs.
func tplKeys(t model.SpreadTpl) []string {
	var keys []string
	bgs := slices.Clone(t.Page)
	if t.Whole != nil {
		bgs = append(bgs, *t.Whole)
	}
	for _, b := range bgs {
		if b.Lib != "" {
			keys = append(keys, b.Lib)
		}
	}
	for _, it := range t.Stickers {
		if it.Lib != "" {
			keys = append(keys, it.Lib)
		}
	}
	for _, b := range t.Borders {
		if b.Lib != "" {
			keys = append(keys, b.Lib)
		}
	}
	return keys
}

// checkTpl — шаблон на трёх страницах; каждый размер независим, при провале — снимок в -out.
func (v *verifier) checkTpl(it decor.Item) {
	t, err := v.s.SpreadTpl(it.ID)
	if err != nil {
		v.failf("шаблон не читается: %v", err)
		return
	}
	libs, missing := v.libsOf(t)
	if missing > 0 {
		v.failf("missing %d (ключей библиотеки нет)", missing)
	}
	fmt.Fprintf(v.w, "  %s: мест %d, стикеров %d, страниц %d\n", it.Name, len(t.Places), len(t.Stickers), t.Pages)
	bgs := 0 // фонов шаблона, чьи файлы есть: на листе должны лечь
	for _, b := range append(slices.Clone(t.Page), derefBg(t.Whole)) {
		if _, ok := libs[b.Lib]; ok && b.Lib != "" {
			bgs++
		}
	}
	for _, sz := range pageSizes {
		p, view, err := album.PreviewTpl(t, sz.w*float64(max(t.Pages, 1)), sz.h, libs)
		if err != nil {
			v.failf("%s: PreviewTpl: %v", sz.name, err)
			continue
		}
		sv := view.Spreads[0]
		bad := v.checkView(sz.name, t, sv, view.MinDPI, bgs)
		if sz.name == "30×30" {
			v.checkAspect(t, sv)
		}
		if bad && v.out != "" {
			v.snapshot(t, p, view, filepath.Join(v.out, "fail-"+slug(v.ctx)+"-"+slug(sz.name)+".png"))
		}
	}
}

// Кадр 3:2 или 2:3 держится в месте, пока обрезка не больше cropMax (спека 131 §7, решение 0.27-131-24);
// шаблон предупреждается, если таких мест меньше fitShare от тех, что считаются.
const (
	cropMax  = 0.15
	fitShare = 0.8
)

// cropOf — доля кадра с отношением p, которую режет место с отношением a (заполнение без полей).
func cropOf(a, p float64) float64 { return 1 - min(a, p)/max(a, p) }

// checkAspect — предупреждение о пропорции мест на 30×30; места-фигуры (овал) по замыслу автора не считаются.
func (v *verifier) checkAspect(t model.SpreadTpl, sv model.SpreadView) {
	n, ok := 0, 0
	for i, pl := range sv.Places {
		if i >= len(t.Places) || t.Places[i].Shape != "" || pl.W <= 0 || pl.H <= 0 {
			continue
		}
		n++
		a := pl.W / pl.H
		if min(cropOf(a, 1.5), cropOf(a, 2.0/3)) <= cropMax+1e-9 {
			ok++
		}
	}
	if n > 0 && float64(ok) < fitShare*float64(n)-1e-9 {
		v.warnf("%d из %d мест не 3:2 / 2:3 на 30×30 (подходят %d%%, порог %d%%)", n-ok, n, ok*100/n, int(fitShare*100))
	}
}

func derefBg(b *model.TplBg) model.TplBg {
	if b == nil {
		return model.TplBg{}
	}
	return *b
}

// checkView — проверки одного листа; true, если был провал.
func (v *verifier) checkView(name string, t model.SpreadTpl, sv model.SpreadView, minDPI, bgs int) bool {
	before := len(v.fails)
	if sv.LowDPI != 0 {
		v.failf("%s: LowDPI %d (порог %d dpi)", name, sv.LowDPI, minDPI)
	}
	if len(sv.Places) != len(t.Places) || len(sv.Stickers) != len(t.Stickers) {
		v.failf("%s: слои потеряны: мест %d из %d, стикеров %d из %d", name, len(sv.Places), len(t.Places), len(sv.Stickers), len(t.Stickers))
	}
	if bgs > 0 && len(sv.Backdrops) == 0 {
		v.failf("%s: фон шаблона не лёг на лист", name)
	}
	area := sv.W // область шаблона: разворот или левая страница
	if t.Pages == 1 {
		area = sv.W / 2
	}
	for i, pl := range sv.Places {
		if !v.inside(pl.X, pl.Y, pl.W, pl.H, pl.Rot, area, sv.H) {
			v.failf("%s: место %d (%.0f,%.0f %.0f×%.0f мм) вне листа %.0f×%.0f", name, i+1, pl.X, pl.Y, pl.W, pl.H, area, sv.H)
		}
	}
	for i, st := range sv.Stickers {
		if !v.inside(st.X, st.Y, st.W, st.H, st.Rot, area, sv.H) {
			v.failf("%s: стикер %d (%.0f,%.0f %.0f×%.0f мм) вне листа %.0f×%.0f", name, i+1, st.X, st.Y, st.W, st.H, area, sv.H)
		}
	}
	return len(v.fails) > before
}

// inside — рамка прямоугольника с поворотом rot (градусы вокруг центра) в области 0..w × 0..h.
func (v *verifier) inside(x, y, w, h, rot, aw, ah float64) bool {
	c, s := math.Abs(math.Cos(rot*math.Pi/180)), math.Abs(math.Sin(rot*math.Pi/180))
	bw, bh := w*c+h*s, w*s+h*c
	cx, cy := x+w/2, y+h/2
	return cx-bw/2 >= -v.tol && cy-bh/2 >= -v.tol && cx+bw/2 <= aw+v.tol && cy+bh/2 <= ah+v.tol
}

var nonSlug = regexp.MustCompile(`[^\p{L}\p{N}]+`)

func slug(s string) string {
	r := []rune(strings.Trim(nonSlug.ReplaceAllString(s, "_"), "_"))
	return string(r[:min(len(r), 48)])
}

// snapshot — снимок шаблона при провале.
func (v *verifier) snapshot(t model.SpreadTpl, p *model.Project, view model.View, path string) {
	img, err := tplImage(t, p, view, 600)
	if err == nil {
		err = imaging.Save(img, path)
	}
	if err != nil {
		fmt.Fprintf(v.w, "  снимок не записан: %v\n", err)
		return
	}
	fmt.Fprintf(v.w, "  снимок: %s\n", filepath.Base(path))
}

// tplImage — плитка шаблона шириной px тем же путём, что окно «Развороты» (TplScene → Render).
func tplImage(t model.SpreadTpl, p *model.Project, view model.View, px int) (image.Image, error) {
	sc, ok := export.TplScene(p, view)
	if !ok {
		return nil, errors.New("нет сцены")
	}
	sv := view.Spreads[0]
	area := sv.W
	if t.Pages == 1 {
		area = sv.W / 2
	}
	img, err := render.Render(sc, render.Options{PageW: sv.W / 2, PageH: sv.H, DPI: int(math.Ceil(float64(px) * 25.4 / area))})
	if err != nil {
		return nil, err
	}
	var out image.Image = img
	if t.Pages == 1 {
		out = img.SubImage(image.Rect(0, 0, img.Bounds().Dx()/2, img.Bounds().Dy()))
	}
	return imaging.Resize(out, px, 0, imaging.Lanczos), nil
}
