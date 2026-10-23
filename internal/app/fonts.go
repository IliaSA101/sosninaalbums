package app

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/album/model"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum"
	"github.com/IliaSA101/sosninaalbums/internal/album/salbum/collect"
	"github.com/IliaSA101/sosninaalbums/internal/decor"
	"github.com/IliaSA101/sosninaalbums/internal/library"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
	"github.com/IliaSA101/sosninaalbums/internal/text"
	"github.com/IliaSA101/sosninaalbums/internal/text/fontlib"
	wr "github.com/wailsapp/wails/v2/pkg/runtime"
)

// Свои шрифты (082, design §6): библиотека fontlib в «Библиотека/Шрифты», регистрация в text,
// событие `fonts` (полный список гарнитур), загрузка, откат начертаний, не открытых окном, ×,
// шрифты папки собранного альбома при открытии, сбор в папку. Пути файлов — только из диалога и
// ImportFontPaths; в журнал — без путей и названий гарнитур (§11).

// FontAdded — добавленная (или вернувшаяся из скрытых) гарнитура для тоста; Faces — начертаний.
// Files — «<16 hex>.<ttf|otf>» её файлов, записанных этой загрузкой (подмножество
// FontImport.Faces; пусто — гарнитура только вернулась из скрытых): окно сверяет их с тем, что
// откатил DropFonts, и отличает гарнитуру, целиком откатанную, от пропуска. Не nil.
type FontAdded struct {
	ID    string   `json:"id"`
	Name  string   `json:"name"`
	Faces int      `json:"faces"`
	NoCyr bool     `json:"noCyr"`
	Files []string `json:"files"`
}

// FontImport — итог загрузки шрифтов: Faces — «<16 hex>.<ttf|otf>» новых начертаний (окно
// проверяет, что открывает их, и откатывает отказавшие DropFonts). Срезы не nil.
type FontImport struct {
	Added   []FontAdded `json:"added"`
	Faces   []string    `json:"faces"`
	Skipped []DecorSkip `json:"skipped"`
}

func newFontImport() FontImport {
	return FontImport{Added: []FontAdded{}, Faces: []string{}, Skipped: []DecorSkip{}}
}

// fontsDir — папка библиотеки шрифтов внутри «Библиотеки» (переносится вместе с ней).
const fontsDir = "Шрифты"

// dirFont — своя последняя папка у диалога «Свои шрифты».
const dirFont = "font"

// errNoFonts — библиотека шрифтов не открылась (startup не дошёл до неё).
var errNoFonts = errors.New("библиотека шрифтов недоступна")

// faceFile — начертание из окна: только имя файла библиотеки, путь не принимается.
var faceFile = regexp.MustCompile(`^[0-9a-f]{16}\.(ttf|otf)$`)

// openFonts открывает библиотеку и регистрирует её гарнитуры — до первого Load: ширины «по
// тексту» альбома считаются его же шрифтами, а не Montserrat.
func (a *App) openFonts() {
	s := fontlib.Open(filepath.Join(a.cfgDir, decor.RootName, fontsDir))
	a.fontLib.Store(s)
	text.SetCustom(s.Families(), s.Load)
}

// fontGen — поколение набора шрифтов для выгрузки (export.go: в нём имя text занято вопросами).
func fontGen() uint64 { return text.Gen() }

func (a *App) fontStore() (*fontlib.Store, error) {
	if s := a.fontLib.Load(); s != nil {
		return s, nil
	}
	return nil, errNoFonts
}

// publishFonts — набор библиотеки сменился: регистрация в text и полный список окну; refit —
// пересчёт ширин «по тексту» у текстов, ждавших эти шрифты (шаг отмены, если что-то сдвинулось).
func (a *App) publishFonts(s *fontlib.Store, refit bool) {
	a.fontsMu.Lock()
	text.SetCustom(s.Families(), s.Load)
	a.emit("fonts", text.Families())
	a.fontsMu.Unlock()
	if refit { // правка альбома — без замка шрифтов: RefitFonts сам ждёт замок редактора
		a.changed(a.editor().RefitFonts())
	}
}

// ImportFonts — «Загрузить свой шрифт…»: диалог в Go, затем ImportFontPaths. Закрыли диалог —
// пустой итог без ошибки; библиотека только для чтения — ошибка до диалога. В фильтре и
// .ttc/.woff: выбранный такой файл получит причину в тосте, а не пропадёт из диалога (082-3).
func (a *App) ImportFonts() (res FontImport, err error) {
	defer guardErr("ImportFonts", &err)
	res = newFontImport()
	s, err := a.fontStore()
	if err != nil {
		return res, err
	}
	if s.ReadOnly() {
		return res, fontlib.ErrReadOnly
	}
	files, err := openFilesDialog(a.ctx, wr.OpenDialogOptions{
		Title:            "Свои шрифты",
		DefaultDirectory: a.dialogDir(dirFont),
		Filters:          []wr.FileFilter{{DisplayName: "Шрифты (*.ttf, *.otf)", Pattern: "*.ttf;*.otf;*.ttc;*.otc;*.woff;*.woff2"}},
	})
	if err != nil || len(files) == 0 {
		return res, err
	}
	res, err = a.ImportFontPaths(files)
	if err == nil {
		a.rememberDir(dirFont, filepath.Dir(files[0]))
	}
	return res, err
}

// ImportFontPaths — загрузка выбранных файлов (как ImportDecorPaths: путь для проверки из
// браузера, 082-22); каждый файл проверяется строго (fontlib.Import → text.Inspect). Добавилось —
// событие `fonts` и пересчёт ширин текстов, ждавших эти гарнитуры.
func (a *App) ImportFontPaths(files []string) (res FontImport, err error) {
	defer guardErr("ImportFontPaths", &err)
	res = newFontImport()
	s, err := a.fontStore()
	if err != nil {
		return res, err
	}
	r, err := s.Import(files)
	if err != nil {
		return res, err
	}
	fresh := map[string]bool{}
	for _, k := range r.Faces {
		fresh[k] = true
	}
	for _, f := range r.Added {
		files := []string{}
		for _, fc := range f.Faces {
			if k := fc.Key + "." + fc.Ext; fresh[k] {
				files = append(files, k)
			}
		}
		res.Added = append(res.Added, FontAdded{ID: f.ID, Name: f.Name, Faces: len(f.Faces), NoCyr: f.NoCyr, Files: files})
	}
	res.Faces = append(res.Faces, r.Faces...)
	for _, k := range r.Skipped {
		res.Skipped = append(res.Skipped, DecorSkip(k))
	}
	if len(r.Added) > 0 {
		a.publishFonts(s, true)
	}
	return res, nil
}

// DropFonts — откат начертаний, которые не открыло окно программы (082-14): удаляются только
// копии, сделанные в этом сеансе (ключи помнит библиотека, а не запрос); возвращает удалённые.
func (a *App) DropFonts(faces []string) (out []string, err error) {
	defer guardErr("DropFonts", &err)
	out = []string{}
	s, err := a.fontStore()
	if err != nil {
		return out, err
	}
	var keys []string
	for _, f := range faces[:min(len(faces), fontlib.MaxFonts)] {
		if faceFile.MatchString(f) {
			keys = append(keys, f)
		}
	}
	if len(keys) == 0 {
		return out, nil
	}
	removed, err := s.Drop(keys)
	if err != nil {
		return out, err
	}
	if len(removed) > 0 {
		out = removed
		a.publishFonts(s, true)
	}
	return out, nil
}

// HideFont — × своей гарнитуры: убрана из списка, файл остаётся и гарнитура зарегистрирована —
// тексты альбомов рисуются ею же, поэтому ширины не пересчитываются.
func (a *App) HideFont(id string) (err error) {
	defer guardErr("HideFont", &err)
	s, err := a.fontStore()
	if err != nil {
		return err
	}
	if err := s.Hide(id); err != nil {
		return err
	}
	a.publishFonts(s, false)
	return nil
}

// loadAlbum — чтение альбома path и шрифты его папки «Шрифты» (собранный альбом, 082-8): пропавшие
// гарнитуры, чьи файлы лежат рядом, сами попадают в библиотеку, и файл читается ещё раз — ширины
// «по тексту» сразу нужным шрифтом, открытие без шага отмены и без «изменён» (082-16). af — что
// добавилось (тост и начертания для проверки окном). Черновик и резервные копии идут мимо (salbum.Load).
func (a *App) loadAlbum(path string) (p *model.Project, af albumFonts, err error) {
	if p, err = salbum.Load(path); err != nil {
		return nil, af, err
	}
	names, faces := a.albumFolderFonts(p, path)
	if len(names) == 0 {
		return p, af, nil
	}
	if q, err := salbum.Load(path); err == nil {
		p = q
	} else { // файл поменяли снаружи между чтениями: первое чтение годно, шрифты уже в библиотеке
		log.Printf("шрифты папки альбома: повторное чтение: %v", paths.NoPath(err))
	}
	return p, albumFonts{info: fontsAddedText(names), faces: faces}, nil
}

// albumFonts — итог папки «Шрифты» при открытии: текст тоста (State.Info) и новые начертания
// (State.FreshFonts). Пусто — ничего не добавилось.
type albumFonts struct {
	info  string
	faces []string
}

// keepAlbumFonts кладёт итог папки альбома в следующий GetState (разово): событие ушло бы раньше,
// чем окно сменит альбом, а при запуске окно ещё не подписано.
func (a *App) keepAlbumFonts(af albumFonts) {
	if af.info == "" {
		return
	}
	a.mu.Lock()
	a.startInfo, a.startFresh = af.info, af.faces
	a.mu.Unlock()
}

// albumFolderFonts — пропавшие гарнитуры альбома p из папки «Шрифты» рядом с path в библиотеку;
// названия добавленных и «<ключ>.<ext>» новых файлов. Только сама папка (не ссылка), её файлы без
// вложенных, ≤ 64 (fontlib).
func (a *App) albumFolderFonts(p *model.Project, path string) (names, faces []string) {
	want := map[string]bool{}
	for _, id := range p.UsedFonts() {
		if !text.Available(id) {
			want[id] = true
		}
	}
	s := a.fontLib.Load()
	if len(want) == 0 || s == nil {
		return nil, nil
	}
	dir := filepath.Join(filepath.Dir(path), collect.DirFonts)
	if paths.CheckLocalPath(dir) != nil || !reachable(dir) {
		return nil, nil
	}
	if st, err := os.Lstat(dir); err != nil || !st.IsDir() {
		return nil, nil
	}
	r, err := s.ImportFolder(dir, want)
	if err != nil {
		log.Printf("шрифты папки альбома: %v", paths.NoPath(err))
		return nil, nil
	}
	if len(r.Skipped) > 0 {
		log.Printf("шрифты папки альбома: пропущено файлов: %d", len(r.Skipped))
	}
	if len(r.Added) == 0 {
		return nil, nil
	}
	a.publishFonts(s, false) // ширины считает повторное чтение файла, а не шаг отмены
	names = make([]string, len(r.Added))
	for i, f := range r.Added {
		names[i] = f.Name
	}
	return names, r.Faces
}

// fontsAddedText — тост об шрифтах папки альбома; больше трёх названий — «и ещё N».
func fontsAddedText(names []string) string {
	q := make([]string, 0, 3)
	for _, n := range names[:min(len(names), 3)] {
		q = append(q, "„"+n+"“")
	}
	out := "Добавлены шрифты из папки альбома: " + strings.Join(q, ", ")
	if len(names) > 3 {
		out += fmt.Sprintf(" и ещё %d", len(names)-3)
	}
	return out
}

// collectPlan — план сбора с файлами своих шрифтов альбома (082-8): начертания гарнитур, которые
// есть на этом компьютере; остальные — в MissingFonts.
func (a *App) collectPlan(p *model.Project, extra []string, dir string) (collect.Plan, error) {
	pl, err := collect.Make(p, extra, dir, os.Lstat)
	if err != nil {
		return pl, err
	}
	fonts := map[string][]collect.FontSrc{}
	missing := 0
	s := a.fontLib.Load()
	for _, id := range p.UsedFonts() {
		var faces []fontlib.Face
		if s != nil && text.Available(id) {
			faces = s.Faces(id)
		}
		if len(faces) == 0 {
			missing++
			continue
		}
		name, _ := text.CustomName(id)
		for _, f := range faces {
			fonts[id] = append(fonts[id], collect.FontSrc{Path: f.Path, Name: name, Bold: f.Bold, Italic: f.Italic, Restricted: f.Restricted})
		}
	}
	return pl, pl.AddFonts(fonts, missing, dir, library.StatLocal)
}
