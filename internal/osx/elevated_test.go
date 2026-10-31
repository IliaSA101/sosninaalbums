package osx

import "testing"

// Elevated отвечает стабильно и совпадает с независимым признаком ОС (refElevated). Процесс не
// обязан быть обычным: раннер GitHub под Windows работает от администратора, разработчик может
// быть под root.
func TestElevated(t *testing.T) {
	want := refElevated(t)
	if first := Elevated(); first != want || Elevated() != first {
		t.Fatalf("Elevated = %v, независимый признак ОС: %v", first, want)
	}
}
