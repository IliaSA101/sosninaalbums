package decor

// DemoCovered — пример стоит и каждая пара (вид, ключ) его предметов есть среди предметов стоящих
// доноров (SetDemo): удаление примера не унесёт ни одного файла и ни одной плитки (0.30-142-17).
// Ключ открытого альбома покрытием не считается — только наборы. Пример без предметов или без
// доноров не покрыт.
func (s *Store) DemoCovered() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.demo.id == "" {
		return false
	}
	demo := s.packOf(s.demo.id)
	if demo == nil || len(demo.Items) == 0 {
		return false
	}
	type pair struct{ kind, key string }
	held := map[pair]bool{}
	for _, id := range s.demo.related {
		if r := s.packOf(id); r != nil {
			for _, e := range r.Items {
				held[pair{e.Kind, e.Key}] = true
			}
		}
	}
	for _, e := range demo.Items {
		if !held[pair{e.Kind, e.Key}] {
			return false
		}
	}
	return true
}
