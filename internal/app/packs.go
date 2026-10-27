package app

import (
	"context"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/app/libpacks"
	"github.com/IliaSA101/sosninaalbums/internal/decor/fetch"
	wr "github.com/wailsapp/wails/v2/pkg/runtime"
)

// Наборы библиотеки (спеки 067, 073, 079, 123, 129; разрез 126 Ш4): тело — internal/app/libpacks,
// здесь фасад с прежними методами для интерфейса, определённые типы ответов и Host службы.
// Типы определены (не псевдонимы) от типов службы: поля и теги те же, а models.ts остаётся в
// пространстве app (126-2). Вложенные DTO написаны заново — пару стережёт TestDTOPairs.

// PackInfo — строка окна «Наборы библиотеки» (decor.PackInfo).
type PackInfo libpacks.Info

// LibraryNet — согласие на сеть и проверка при запуске (prefs.LibraryNet без списка удалённых:
// наружу — только сколько их, для «Вернуть удалённые»).
type LibraryNet struct {
	Allowed bool  `json:"allowed"`
	AtStart bool  `json:"atStart"`
	Checked int64 `json:"checked"` // unix мс последней удачной проверки
	Skipped int   `json:"skipped"` // удалённых пользователем: «Обновить» их не ставит
}

// PackList — содержимое окна «Наборы»: Busy — идёт операция (кнопки неактивны).
type PackList struct {
	Packs    []PackInfo `json:"packs"`
	Net      LibraryNet `json:"net"`
	Busy     bool       `json:"busy"`
	ReadOnly bool       `json:"readOnly"`
}

// PackAdd — итог «Добавить из файла». Ask: "" — поставлен; same — эта версия уже стоит;
// update — стоит FromVersion, замена — ConfirmPack(Token); older — стоит новее, не ставится.
type PackAdd struct {
	Pack        PackInfo    `json:"pack"`
	Ask         string      `json:"ask"`
	FromVersion int         `json:"fromVersion"`
	Token       string      `json:"token"`
	Skipped     []DecorSkip `json:"skipped"`
}

// PackRemove — числа вопроса «Удалить набор?» (decor.PackRemove).
type PackRemove libpacks.Remove

// LibraryUpdate — итог «Обновить библиотеку»: новых и обновлённых наборов, пропущенное с
// причиной (нужна новая программа, предметы набора), байты скачанного.
type LibraryUpdate struct {
	Added   int         `json:"added"`
	Updated int         `json:"updated"`
	Skipped []DecorSkip `json:"skipped"`
	Bytes   int64       `json:"bytes"`
}

// dirPack — своя последняя папка у диалога «Добавить из файла».
const dirPack = "pack"

// Проверка при запуске (решение 17); общие с обновлением программы (updHost), тесты ставят их
// через setStart.
var (
	startDelay       = 15 * time.Second
	startEvery       = 20 * time.Hour
	startOffer int64 = 100 << 20
)

func libNet(n libpacks.Net) LibraryNet { return LibraryNet(n) }

func decorSkips(in []libpacks.Skip) []DecorSkip {
	out := make([]DecorSkip, len(in)) // не nil: интерфейс не проверяет null
	for i, k := range in {
		out[i] = DecorSkip(k)
	}
	return out
}

func packList(l libpacks.List) PackList {
	out := PackList{Packs: make([]PackInfo, len(l.Packs)), Net: libNet(l.Net), Busy: l.Busy, ReadOnly: l.ReadOnly}
	for i, p := range l.Packs {
		out.Packs[i] = PackInfo(p)
	}
	return out
}

func packAdd(r libpacks.Add) PackAdd {
	return PackAdd{Pack: PackInfo(r.Pack), Ask: r.Ask, FromVersion: r.FromVersion, Token: r.Token, Skipped: decorSkips(r.Skipped)}
}

func libraryUpdate(r libpacks.Update) LibraryUpdate {
	return LibraryUpdate{Added: r.Added, Updated: r.Updated, Skipped: decorSkips(r.Skipped), Bytes: r.Bytes}
}

// LibraryPacks — список наборов: встроенные и поставленные.
func (a *App) LibraryPacks() (PackList, error) {
	l, err := a.libp.List()
	return packList(l), err
}

// AddPackFile — «Добавить из файла…»: диалог *.zip в Go. Отказ от диалога — пустой итог.
func (a *App) AddPackFile() (PackAdd, error) {
	r, err := a.libp.AddFile()
	return packAdd(r), err
}

// AddPackPath — набор по пути (как ImportDecorPaths: для проверки из браузера).
func (a *App) AddPackPath(path string) (PackAdd, error) {
	r, err := a.libp.AddPath(path)
	return packAdd(r), err
}

// ConfirmPack — «Заменить версией N» на вопрос AddPackPath.
func (a *App) ConfirmPack(token string) (PackAdd, error) {
	r, err := a.libp.Confirm(token)
	return packAdd(r), err
}

// PlanRemovePack — числа для вопроса «Удалить набор?».
func (a *App) PlanRemovePack(id string) (PackRemove, error) {
	r, err := a.libp.PlanRemove(id)
	return PackRemove(r), err
}

// RemovePack — удаление набора (решение 10).
func (a *App) RemovePack(id string) (PackList, error) {
	l, err := a.libp.Remove(id)
	return packList(l), err
}

// ShowLibraryFolder открывает папку библиотеки в Проводнике; нет папки — сначала создаёт.
func (a *App) ShowLibraryFolder() error { return a.libp.ShowFolder() }

// ResetLibrarySkip — забыть удалённые наборы: следующее «Обновить библиотеку» вернёт их.
func (a *App) ResetLibrarySkip() (LibraryNet, error) {
	n, err := a.libp.ResetSkip()
	return libNet(n), err
}

// SetLibraryNet — согласие и «Проверять при запуске»; отзыв снимает и флажок, и скачивание.
func (a *App) SetLibraryNet(allowed, atStart bool) (LibraryNet, error) {
	n, err := a.libp.SetNet(allowed, atStart)
	return libNet(n), err
}

// CancelLibrary — «Отменить»: идущее обновление обрывается, поставленные до него наборы остаются.
func (a *App) CancelLibrary() { a.libp.Cancel() }

// UpdateLibrary — «Обновить библиотеку»: каталог, затем новые наборы и новые версии стоящих.
func (a *App) UpdateLibrary() (LibraryUpdate, error) {
	r, err := a.libp.Update()
	return libraryUpdate(r), err
}

// packsHost — что службе наборов нужно от App. Поля — замыкания над a: тесты подменяют a.emit,
// a.cfgDir, openFileDialog, openDir и setStart уже после NewApp.
func (a *App) packsHost() libpacks.Host {
	return libpacks.Host{
		CfgDir:  func() string { return a.cfgDir },
		Version: func() string { return appVersion(wailsJSON) },
		Store:   a.decorStore,
		Emit:    func(name string, data ...any) { a.emit(name, data...) },
		GoBG:    func(fn func()) bool { return a.goBG(fn) },
		StopCtx: func() context.Context { return a.stopCtx },
		// Ключи библиотеки открытого альбома, и в шагах отмены и возврата: иначе Ctrl+Y после
		// удаления набора вернул бы фон или стикер без файла (ревью 067).
		AlbumLibs: func() func(key string) bool {
			keys := a.editor().LibKeys()
			return func(key string) bool { return keys[key] }
		},
		Relink:        a.relinkLib,
		PrintsChanged: func() { a.ownp.LibraryChanged() },
		PickFile: func() (string, error) {
			return openFileDialog(a.ctx, wr.OpenDialogOptions{
				Title:            "Набор библиотеки",
				DefaultDirectory: a.dialogDir(dirPack),
				Filters:          []wr.FileFilter{{DisplayName: "Наборы библиотеки (*.zip)", Pattern: "*.zip"}},
			})
		},
		RememberDir: func(dir string) { a.rememberDir(dirPack, dir) },
		OpenDir:     func(dir string) error { return openDir(dir) },
		Fetcher:     func() *fetch.Client { return fetch.New(fetch.Default()) },
		Now:         time.Now,
		Start:       func() (time.Duration, time.Duration, int64) { return startDelay, startEvery, startOffer },
		// Крючки тестов гонки отзыва: в программе пустые.
		BeforeBegin:   func() {},
		AfterNetWrite: func() {},
	}
}
