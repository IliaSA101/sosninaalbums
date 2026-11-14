package paths

import "testing"

func TestTrailingDotFor(t *testing.T) {
	for _, c := range []struct {
		path string
		want bool
	}{
		{`C:\Фото\.. \x.jpg`, true},
		{`C:\Фото\a.\b`, true},
		{`C:\Фото\a \b`, true},
		{`C:\Фото\...\b`, true},
		{`C:\Фото\b.jpg `, true},
		{`C:\Фото\b.`, true},
		{`C:/Фото/a./b`, true},
		{`C:\Фото\.\x.jpg`, false},
		{`C:\Фото\..\x.jpg`, false},
		{`C:\Фото\.hidden\x.jpg`, false},
		{`C:\Фото\a.b\c d\x.jpg`, false},
		{`C:\`, false},
		{``, false},
	} {
		if got := TrailingDotFor(true, c.path); got != c.want {
			t.Errorf("TrailingDotFor(true, %q) = %v, ждали %v", c.path, got, c.want)
		}
		if TrailingDotFor(false, c.path) {
			t.Errorf("на другой ОС имя с точкой на конце обычное: %q", c.path)
		}
	}
}
