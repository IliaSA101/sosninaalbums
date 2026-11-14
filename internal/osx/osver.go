package osx

// OSVersion — сырая строка версии ОС («13.4.1», «10.0.22631»): без разбора и проверки, правила
// minOS проверяет internal/update (цикл импорта не даёт звать ParseOS отсюда). false — не
// определилась или ОС без правила (Linux, 171-10).
func OSVersion() (string, bool) { return osVersion() }
