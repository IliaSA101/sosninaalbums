//go:build !linux

package osx

// На других системах /sys нет, а петлю под dm (LUKS-образ через loop-setup) ищет только Linux.
// Вне Linux sysfs никто не задаёт; если задан — прочитать его нечем, отсутствие петли доказать
// нельзя, поэтому отказ (как в volid_slaves.go). Заодно поле sys остаётся читаемым для линтера.
func (d volDirs) onLoop(string) bool { return d.sys != "" }
