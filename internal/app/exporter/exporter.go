// Package exporter — служба выгрузки альбома в JPEG (спеки 013, 032, 047, 073, 081, 121, 123;
// разрез 126 Ш3): проверки до диалога папки, вопросы «Да / Нет», папка книги типографии, фоновая
// запись разворотов (internal/export), остановка и итог export:done; готовность к выгрузке для
// типографий (PrintPlans) и сайт типографии (OpenPrintSite). Проект не меняется.
//
// Зовёт её internal/app (export.go — фасад с методами для интерфейса). Wails и App службе
// недоступны: диалоги, события, фон, цвет фото из ленты и предел памяти приходят через Host (поля-
// функции, замыкания над App — позднее связывание, подмены тестов действуют и после NewApp).
package exporter

import (
	"context"
	"reflect"
	"sync"

	"github.com/IliaSA101/sosninaalbums/internal/album"
	"github.com/IliaSA101/sosninaalbums/internal/album/model"
)

// Options — настройки окна экспорта. App объявляет их определённым типом (`type ExportOptions
// exporter.Options`): Wails берёт пространство имён models.ts из имени пакета, а `export` —
// ключевое слово TS, и тип службы в сигнатуре метода App переехал бы в другое пространство.
type Options struct {
	Range   string `json:"range"`   // "" — все развороты; «3-5, 8, 7п» (export.ParseRange)
	Quality int    `json:"quality"` // 100 | 95 | 80; другое — 95
	DPI     int    `json:"dpi"`     // 72..1200; 0 — из настроек альбома
	Pages   bool   `json:"pages"`   // по страницам: левая и правая отдельными файлами
	// Cover — выгрузить обложку 00.jpg (спека 032); обложки нет или проект-лист — игнорируется.
	// CoverOnly — только обложка, развороты и Range не выгружаются (включает Cover).
	Cover     bool `json:"cover"`
	CoverOnly bool `json:"coverOnly"`
	// Print — выгрузка для типографии (047, 081 С6): "" — обычная, иначе Kind профиля printshop
	// («fsfera», «fabrika»): папка книги, все развороты, 300 dpi, с обложкой, имена и качество
	// типографии; Range, Pages, DPI, Cover и CoverOnly игнорируются (Quality — при своём у
	// типографии), запреты printshop.Check — ошибка, неизвестная типография — тоже.
	Print string `json:"print"`
}

// Host — что службе нужно от App. Все поля — функции и обязательны (New паникует с именем
// пустого). App отдаёт настоящие значения; тест пересобирает службу с подменой до первого вызова.
type Host struct {
	Editor  func() *album.Editor           // текущий редактор; не кэшировать
	Exists  func(path string) bool         // файл на месте
	Emit    func(name string, data ...any) // событие окну: export:progress, export:done
	GoBG    func(fn func()) bool           // фон под WaitGroup App; false — App закрывается
	StopCtx func() context.Context         // контекст выхода программы: родитель выгрузки
	// ColorReport — фото с не-sRGB профилем: цвет из ленты (под замком App) или из заголовка файла.
	ColorReport func(p *model.Project) (conv, other map[string]int)
	Ask         func(msg string) (bool, error)     // вопрос «Да / Нет»: (да, ошибка окна)
	PickDir     func(title string) (string, error) // окно выбора папки; "" — отмена
	// RememberDir запоминает папку выгрузки для следующего окна (prefs.DirExport остаётся в App).
	RememberDir func(dir string)
	OpenURL     func(url string) // адрес в браузере
	ShowDir     func(dir string) // папка в проводнике
	// LiftMemory снимает наш предел памяти на время выгрузки; restore возвращает его (общий с Run).
	LiftMemory func() (restore func())
}

// Service — флаг «выгрузка идёт», её отмена и папка последней, под mu.
type Service struct {
	h      Host
	mu     sync.Mutex
	busy   bool               // выгрузка идёт: флаг до диалога папки, второй вызов не пройдёт
	cancel context.CancelFunc // «Остановить»; nil — фона нет
	dir    string             // папка последней выгрузки: её открывает Show
}

// New собирает службу; пустое поле Host — паника с его именем (как edit.New и colorpick.New).
func New(h Host) *Service {
	v := reflect.ValueOf(h)
	for i := range v.NumField() {
		if v.Field(i).IsNil() {
			panic("exporter.New: не задано поле Host." + v.Type().Field(i).Name)
		}
	}
	return &Service{h: h}
}

// Busy — выгрузка идёт (running) и у неё есть фон, который можно остановить (cancelable).
func (s *Service) Busy() (running, cancelable bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.busy, s.cancel != nil
}

// LastDir — папка последней выгрузки; "" — выгрузок не было.
func (s *Service) LastDir() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.dir
}

// Show открывает папку последней выгрузки. Путь от интерфейса не принимаем: иначе любая страница
// в окне смогла бы запускать explorer с произвольным аргументом.
func (s *Service) Show() { s.h.ShowDir(s.LastDir()) }
