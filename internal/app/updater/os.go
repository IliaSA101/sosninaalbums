package updater

import (
	"log"

	"github.com/IliaSA101/sosninaalbums/internal/app/appkit"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
	"github.com/IliaSA101/sosninaalbums/internal/update"
)

// sysState — что проверка узнала о системе (171, правило minOS), под mu службы. Переживает ошибку
// следующей проверки: «Страница выпуска» после неудачи по-прежнему ведёт на установленную версию.
type sysState struct {
	os      string // GOOS после подмены стенда; "" — проверки не было
	blocked string // версия, которой нужна система новее нашей
	need    string // её minOS
	last    string // одноразово для Info(): показать тост
	told    string // стенд: версия, о которой в этом сеансе уже сказано (osTold в config.json не пишется, 171-15)
}

// needTell — тост нужен: версия недоступна системе и предложить нечего (со ступенью lastOn идёт
// обычное update:offer со страницей).
func needTell(res update.Result) bool { return res.Blocked != "" && res.Offer == nil }

// toldOnStand — решение о тосте на стенде: «уже показан» — поле службы на сеанс.
func (s *Service) toldOnStand(res update.Result) bool {
	if !needTell(res) {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sysOS.told == res.Blocked {
		return false
	}
	s.sysOS.told = res.Blocked
	return true
}

// noteSystem — запомнить итог проверки о системе под mu. Откат (старый файл повторно) блокировку
// не меняет: он ничего о системе не говорит.
func (s *Service) noteSystem(res update.Result, sys update.System) {
	if res.Rollback {
		return
	}
	s.sysOS.os, s.sysOS.blocked, s.sysOS.need = sys.GOOS, res.Blocked, res.Need
}

// emitLast — update:last: тост «последняя версия для вашей системы». Одно и то же отдаёт и Info()
// (интерфейс мог подписаться позже события), повтор отсекает интерфейс.
func (s *Service) emitLast(blocked string) {
	s.mu.Lock()
	s.sysOS.last = blocked
	s.mu.Unlock()
	info := s.snap()
	info.Last = blocked
	s.h.Emit("update:last", info)
}

// logSystem — строки log.txt о системе: блокировка, неопределённая версия ОС, пропущенные поля.
// Значения из манифеста — только имена полей через %q.
func logSystem(res update.Result, sys update.System) {
	if res.Blocked != "" {
		log.Printf("обновление: версия %s требует %s ≥ %s, система %s — не предлагается", res.Blocked, sys.GOOS, res.Need, sys.Version)
	}
	if res.OSUnknown {
		log.Printf("обновление: версия системы не определилась, правило minOS не применяется")
	}
	if len(res.Skipped) > 0 {
		log.Printf("обновление: пропущены неверные поля другой ОС: %q", res.Skipped)
	}
}

// pageURL — адрес страницы выпуска (171-16). Есть предложение (в том числе ступень lastOn) — его
// тег; нет, но система блокирует (в сеансе или по osTold новее текущей) — страница установленной
// версии, а cur не версия — список выпусков; иначе последний выпуск.
func (s *Service) pageURL() string {
	cur := s.h.Version()
	told := prefs.UpdateOf(s.h.CfgDir()).OSTold
	s.mu.Lock()
	defer s.mu.Unlock()
	if o := s.offer; o != nil && update.SetupName(o.Version) != "" {
		return appkit.RepoURL + "/releases/tag/v" + o.Version
	}
	if s.sysOS.blocked != "" || told != "" && update.Newer(told, cur) {
		if update.SetupName(cur) != "" {
			return appkit.RepoURL + "/releases/tag/v" + cur
		}
		return appkit.RepoURL + "/releases"
	}
	return appkit.RepoURL + "/releases/latest"
}
