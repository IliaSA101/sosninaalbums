// Package guidevideo — ролики справочника F1 (спека 140, C; design §3, security.md): список из
// сохранённого каталога, скачивание с проверкой sha256, раздача /media/guide-video/<id> окну и
// «Показать файл». Кэш роликов — папка пользователя, не папка данных: их можно скачать снова.
//
// Служба узкая (depguard): только stdlib и decor/fetch. Согласие, события, проводник, каталог
// и открытие файла приходят через Host, как у libpacks. Недоверенное — сеть, файл кэша каталога
// и адрес запроса; от адреса и каталога на диск попадают только id по fetch.ValidID, версия числом
// и вид из двух констант.
package guidevideo

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/decor/fetch"
)

// Ошибки для тоста: по-русски, без путей и адресов (security Н10). fetch.Err* про наборы
// переводятся в эти тексты (human).
var (
	ErrNoConsent   = errors.New("нужно разрешить обращение к GitHub")
	ErrGone        = errors.New("ролика пока нет в библиотеке — повторите позже")
	ErrBusy        = errors.New("ролик уже скачивается")
	ErrUnavailable = errors.New("ролики недоступны")
	ErrVideoBroken = errors.New("ролик скачался с ошибкой — повторите")
	ErrVideoDisk   = errors.New("ролик не записывается на диск")
	ErrVideoSlow   = errors.New("связь слишком медленная — ролик не скачался за отведённое время")
	ErrVideoTLS    = errors.New("соединение с GitHub не прошло проверку подлинности — ролик не скачан")
	ErrVideoAddr   = errors.New("адрес ролика не из разрешённых — ролик не скачан")
	ErrCancelled   = errors.New("скачивание ролика отменено")
	ErrNoFile      = errors.New("ролика нет — скачайте снова")
	ErrReveal      = errors.New("проводник не открылся")
)

// Host — что службе нужно от App. Все поля — функции и обязательны (New паникует с именем пустого):
// шаблон testkit.HostComplete. Настоящие значения — internal/app/guide.go.
type Host struct {
	CacheDir func() string                                // <UserCacheDir>/<DataName>/guide-videos; "" или не абсолютный — служба выключена
	App      func() string                                // версия программы: minApp роликов
	OS       func() string                                // runtime.GOOS: darwin играет mp4, остальные webm; тест подменяет
	Allowed  func() bool                                  // согласие на сеть (prefs.LibraryNetOf(cfgDir).Allowed)
	StopCtx  func() context.Context                       // контекст выхода программы: родитель сети
	Cached   func() (fetch.Catalog, bool)                 // сохранённый каталог libpacks, без сети
	Fresh    func(context.Context) (fetch.Catalog, error) // свежий каталог: seq и кэш пишет libpacks
	Client   func() *fetch.Client                         // сетевой клиент; тесты подменяют своим сервером
	Emit     func(name string, data ...any)               // событие окну: guide:video
	Reveal   func(path string) error                      // проводник с выделенным файлом
	// OpenRegular открывает обычный файл без ожидания на канале (imgfile.OpenRegular): depguard не
	// пускает службу в imgfile.
	OpenRegular func(path string) (*os.File, error)
	Free        func(dir string) (uint64, error)    // свободное место (osx.FreeBytes)
	Rename      func(oldpath, newpath string) error // os.Rename; тест подменяет, чтобы проверить сбой
}

// Item — ролик в списке: самая новая версия, годная этой программе. Ready — файл есть, обычный
// и размер каталожный (дёшево; sha256 сверяет Download).
type Item struct {
	ID      string `json:"id"`
	Version int    `json:"version"`
	Seconds int    `json:"seconds"`
	Size    int64  `json:"size"`
	Ready   bool   `json:"ready"`
}

// Progress — событие guide:video: байты скачивания одного ролика.
type Progress struct {
	ID    string `json:"id"`
	Done  int64  `json:"done"`
	Total int64  `json:"total"`
}

// progressGap — guide:video не чаще 4 раз в секунду.
const progressGap = 250 * time.Millisecond

// serveSlots — одновременные отдачи: на WebView2 ответ копится в памяти целиком (до 16 МБ).
const serveSlots = 2

// Service — одна загрузка за раз, отмена, раздача.
type Service struct {
	h Host

	mu sync.Mutex // TryLock: вторая загрузка — ErrBusy, а не ожидание

	netMu    sync.Mutex
	cancel   context.CancelFunc
	starting map[*start]struct{} // вошли в Download, begin ещё не прошёл

	miss  atomic.Int64
	slots chan struct{}
}

// New собирает службу; пустая функция Host — паника с её именем (как libpacks.New).
func New(h Host) *Service {
	v := reflect.ValueOf(h)
	for i := range v.NumField() {
		if v.Field(i).IsNil() {
			panic("guidevideo.New: не задано поле Host." + v.Type().Field(i).Name)
		}
	}
	return &Service{h: h, slots: make(chan struct{}, serveSlots)}
}

// dir — папка роликов; "" — служба выключена: путь пустой или не абсолютный (UserCacheDir без
// HOME/LOCALAPPDATA дал бы папку от текущей, security Н4).
func (s *Service) dir() string {
	d := s.h.CacheDir()
	if d == "" || !filepath.IsAbs(d) {
		return ""
	}
	return filepath.Clean(d)
}

// start — загрузка, которая вошла в Download, но ещё не дошла до регистрации отмены в begin.
type start struct{ cancelled bool }

// Cancel — «Отменить» и отзыв согласия: идущая загрузка обрывается; ещё не начавшаяся (вошла в
// Download, но не дошла до begin) тоже — иначе отмена в этом окне терялась и ролики качались.
func (s *Service) Cancel() {
	s.netMu.Lock()
	defer s.netMu.Unlock()
	for st := range s.starting {
		st.cancelled = true
	}
	if s.cancel != nil {
		s.cancel()
	}
}

// arm ставит метку «загрузка начинается» в самом начале Download; снимает её возвращённая функция.
// Метка только у вошедших вызовов: Cancel без загрузки ничего не помнит и следующую не заденет.
func (s *Service) arm() (*start, func()) {
	st := &start{}
	s.netMu.Lock()
	if s.starting == nil {
		s.starting = map[*start]struct{}{}
	}
	s.starting[st] = struct{}{}
	s.netMu.Unlock()
	return st, func() {
		s.netMu.Lock()
		delete(s.starting, st)
		s.netMu.Unlock()
	}
}

// begin — контекст загрузки от StopCtx и повторная сверка согласия уже после регистрации отмены:
// отзыв между первой проверкой и begin иначе не отменил бы ничего (security Н2). Отмена, пришедшая
// после arm, но до begin, видна в st.cancelled: загрузка не начинается (ErrCancelled).
func (s *Service) begin(st *start) (context.Context, func(), error) {
	ctx, cancel := context.WithCancel(s.h.StopCtx())
	s.netMu.Lock()
	s.cancel = cancel
	was := st.cancelled
	s.netMu.Unlock()
	done := func() {
		s.netMu.Lock()
		s.cancel = nil
		s.netMu.Unlock()
		cancel()
	}
	if was {
		done()
		return nil, nil, ErrCancelled
	}
	if !s.h.Allowed() {
		done()
		return nil, nil, ErrNoConsent
	}
	return ctx, done, nil
}
