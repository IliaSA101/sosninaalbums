package libpacks

import "github.com/IliaSA101/sosninaalbums/internal/prefs"

// Согласие на сеть в памяти службы: раздача клеток /media/lp/ зовётся сотнями раз при показе
// галереи, а чтение config.json идёт под его замком. Значение читается из файла при первом
// обращении (и если папка настроек сменилась), дальше его обновляет SetNet — единственная запись
// Allowed в программе. Отзыв виден следующему запросу сразу.
type consent struct {
	dir     string // для какой папки настроек значение
	known   bool
	allowed bool
}

// allowed — согласие сейчас. Без папки настроек согласия нет и в память оно не кладётся.
func (s *Service) allowed() bool {
	dir := s.h.CfgDir()
	if dir == "" {
		return false
	}
	s.consMu.Lock()
	defer s.consMu.Unlock()
	if !s.cons.known || s.cons.dir != dir {
		s.cons = consent{dir: dir, known: true, allowed: prefs.LibraryNetOf(dir).Allowed}
	}
	return s.cons.allowed
}

// setConsent — SetNet записал согласие в config.json: память идёт следом.
func (s *Service) setConsent(dir string, allowed bool) {
	s.consMu.Lock()
	s.cons = consent{dir: dir, known: true, allowed: allowed}
	s.consMu.Unlock()
}
