package app

import (
	"log"
	"net/http"
	"strings"

	"github.com/IliaSA101/sosninaalbums/internal/app/libpacks"
)

// Плитки «⤓ набор, размер» в галереях (спека 144): тонкие методы над libpacks и decor.

// DecorGet — плитка образца нескачанного набора в строке «Можно скачать» (тип над libpacks.Tile — поля общие).
type DecorGet libpacks.Tile

// themeGet — id темы «Можно скачать»: последняя строка списка тем, только при плитках.
const themeGet = "get"

// decorGet — плитки вида kind и приглашение для DecorCatalog; без сети и без записи на диск.
func (a *App) decorGet(kind string) (get []DecorGet, invite bool) {
	get = []DecorGet{}
	// Сбой плиток не роняет каталог (design §6): пустой «Можно скачать», в журнале только тип паники.
	defer func() {
		if rec := recover(); rec != nil {
			log.Printf("библиотека: плитки «Можно скачать»: сбой (%T)", rec)
			get, invite = []DecorGet{}, false
		}
	}()
	if a.libp == nil {
		return get, false
	}
	for _, t := range a.libp.Tiles(kind) {
		get = append(get, DecorGet(t))
	}
	return get, a.libp.Invite()
}

// FetchPackPreviews — докачка картинок-образцов для плиток «Можно скачать»: окно зовёт при открытии
// без ожидания. Задание идёт фоном (Host.GoBG), без согласия и без списка — молча nil; итог — событие
// library:previews.
func (a *App) FetchPackPreviews() (err error) {
	defer guardErr("FetchPackPreviews", &err)
	a.libp.StartPreviews()
	return nil
}

// PackItem — плитка предмета только что скачанного набора (вид, id набора, src плитки «Можно
// скачать»): окно ставит её как щелчком по стоящей. Формат входа проверяется до поиска, искать в
// архиве нечего — реестр набора. Нет предмета — decor.ErrNoItem.
func (a *App) PackItem(kind, pack, src string) (item DecorItem, err error) {
	defer guardErr("PackItem", &err)
	s, err := a.decorStore()
	if err != nil {
		return DecorItem{}, err
	}
	w, h := a.editor().SheetSize()
	it, err := s.PackItem(pack, kind, src, w, h)
	if err != nil {
		return DecorItem{}, err
	}
	return DecorItem(it), nil
}

// serveLibPreview отдаёт /media/lp/<pack>/<kind>/<i> (клетки образцов); false — адрес другой.
// Любой хвост под префиксом получает ответ службы: в общий разбор /media/<t|p>/<id> он не уходит.
func (a *App) serveLibPreview(w http.ResponseWriter, r *http.Request) bool {
	tail, ok := strings.CutPrefix(r.URL.Path, "/media/lp/")
	if !ok {
		return false
	}
	if a.libp == nil {
		http.NotFound(w, r)
		return true
	}
	a.libp.ServePreview(w, r, tail)
	return true
}

// withGetTheme дописывает тему «Можно скачать» последней, если есть плитки.
func withGetTheme(themes []DecorTheme, n int) []DecorTheme {
	if n == 0 {
		return themes
	}
	return append(themes, DecorTheme{ID: themeGet, Name: "Можно скачать", Count: n})
}
