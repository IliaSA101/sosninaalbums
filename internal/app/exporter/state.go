package exporter

// Состояния последней выгрузки для агента (get_album.export).
const (
	StateIdle    = "idle"    // выгрузок не было
	StateRunning = "running" // идёт
	StateDone    = "done"    // готово
	StateStopped = "stopped" // остановлена человеком или программой
	StateFailed  = "failed"  // ошибка записи
)

// ExportState — ход последней выгрузки. AlbumID — номер Editor альбома выгрузки (agents.AlbumKey
// строит из него ключ): get_album другого альбома видит idle. Error — без пути: в export:done
// текст ошибки записи содержит путь, агенту он не нужен.
type ExportState struct {
	AlbumID uint64
	State   string
	Done    int
	Total   int
	Error   string
}

// Last — ход последней выгрузки; до первой — State «idle».
func (s *Service) Last() ExportState {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.last.State == "" {
		return ExportState{State: StateIdle}
	}
	return s.last
}

// progress запоминает ход выгрузки, пока она идёт.
func (s *Service) progress(done, total int) {
	s.mu.Lock()
	if s.last.State == StateRunning {
		s.last.Done, s.last.Total = done, total
	}
	s.mu.Unlock()
}
