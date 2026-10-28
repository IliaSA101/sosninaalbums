package agents

import (
	"errors"
	"io/fs"
	"log"
	"net"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// Права: папка сокета — только владелец, сам сокет — тоже (Unix); на Windows права ставит
// perm_windows.go.
const (
	dirMode  = 0o700
	sockMode = 0o600
)

// maxSockPath — предел длины пути сокета, байтов UTF-8: sun_path — 108 (Linux, Windows) и 104 (Mac)
// вместе с нулём. Длиннее Listen отвечает «invalid argument»; проверяем до него, чтобы дать
// понятную причину. Кириллический профиль Windows — 97 из 108 (спайк Т0): запас мал.
const maxSockPath = 100

// probeTimeout — сколько ждём ответа старого сокета перед удалением.
const probeTimeout = 300 * time.Millisecond

// Причины отказа открыть доступ. Тексты идут в окно как есть: словами, без путей.
var (
	errNoDataDir = errors.New("не найдена папка данных")
	errBadPath   = errors.New("папка данных недопустима для доступа агентов")
	errPathLong  = errors.New("путь к папке данных слишком длинный")
	errNoPerm    = errors.New("папка данных на диске без прав доступа")
	errForeign   = errors.New("на месте доступа агентов чужой файл")
	errBusy      = errors.New("доступ занят другой копией программы")
	errOpen      = errors.New("не удалось открыть доступ для агентов")
)

// known — отказы со своим текстом; прочие ошибки (диск, ОС) наружу уходят как errOpen.
var known = []error{errNoDataDir, errBadPath, errPathLong, errNoPerm, errForeign, errBusy}

// checkSock проверяет путь сокета до любых действий на диске. "" — папки данных нет: пути от
// текущей папки не бывает. Только путь в ФС: абстрактное пространство Linux («@…», «\0…») не
// имеет прав файла и никогда не годится.
func checkSock(p string) error {
	switch {
	case p == "":
		return errNoDataDir
	case strings.HasPrefix(p, "@") || strings.ContainsRune(p, 0):
		return errBadPath
	case paths.CheckLocalPath(p) != nil:
		return errBadPath
	case len(p) > maxSockPath:
		return errPathLong
	}
	return nil
}

// bare — ошибка для log.txt без путей: в них имя пользователя.
func bare(err error) error {
	var oe *net.OpError
	if errors.As(err, &oe) {
		return errors.New(oe.Op + ": " + oe.Err.Error())
	}
	return paths.NoPath(err)
}

// guardPanic — для `defer`: SDK паник не ловит, а горутина без recover роняет программу вместе
// с несохранённым альбомом. В log.txt — место и тип значения, без самого значения (в нём бывает путь).
func guardPanic(what string) {
	if r := recover(); r != nil {
		logPanic(what, r)
	}
}

// logPanic — запись о панике: место, тип значения и стек; само значение не пишется.
func logPanic(what string, r any) {
	log.Printf("ПАНИКА в agents %s (%T)\n%s", what, r, debug.Stack())
}

// open занимает путь сокета: проверяет путь, готовит папку, убирает след упавшего запуска и
// слушает. Всё, что не из known, наружу — errOpen (причина в log.txt без путей).
func (s *Service) open() (l net.Listener, err error) {
	sock := s.h.Sock()
	defer func() {
		if err == nil || isKnown(err) {
			return
		}
		log.Printf("agents: не открыт сокет: %v", bare(err))
		l, err = nil, errOpen
	}()
	if err = checkSock(sock); err != nil {
		return nil, err
	}
	if err = secureDir(filepath.Dir(sock)); err != nil {
		return nil, err
	}
	if err = clearStale(sock); err != nil {
		return nil, err
	}
	if l, err = net.Listen("unix", sock); err != nil {
		return nil, err
	}
	if err = secureSock(sock); err != nil {
		_ = l.Close() // Go удаляет файл сокета
		return nil, err
	}
	return l, nil
}

func isKnown(err error) bool {
	for _, k := range known {
		if errors.Is(err, k) {
			return true
		}
	}
	return false
}

// clearStale убирает файл от упавшего запуска. Удаляется только сокет, на который никто не
// отвечает: обычный файл, ссылка и каталог на этом месте — чужое, не трогаем; живой ответ —
// другая копия программы (замок экземпляра этого не допускает, но Remove сокета бы её отрезал).
func clearStale(sock string) error {
	fi, err := os.Lstat(sock)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if fi.Mode()&os.ModeSocket == 0 {
		return errForeign
	}
	if c, derr := net.DialTimeout("unix", sock, probeTimeout); derr == nil {
		_ = c.Close()
		return errBusy
	}
	if err := os.Remove(sock); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

// serve — цикл приёма подключений до закрытия слушателя. Паника одного подключения закрывает
// его (admit), сама служба живёт.
func (r *run) serve() {
	defer r.wg.Done()
	defer func() {
		if p := recover(); p != nil { // цикл умер: служба глухая, окно должно об этом знать
			logPanic("accept", p)
			r.s.fail(r)
		}
	}()
	for {
		c, err := r.ln.Accept()
		if err != nil {
			if r.ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				return
			}
			// Временная ошибка (нет дескрипторов): без паузы цикл жёг бы процессор.
			select {
			case <-time.After(50 * time.Millisecond):
			case <-r.ctx.Done():
				return
			}
			continue
		}
		r.admit(c)
	}
}
