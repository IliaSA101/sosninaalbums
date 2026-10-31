package testkit

import (
	"runtime"
	"testing"
)

// AbsPath: абсолютный путь по правилам текущей ОС; разделители в имени приводятся к её виду.
func TestAbsPath(t *testing.T) {
	cases := []struct{ in, win, other string }{
		{"a.jpg", `C:\a.jpg`, "/a.jpg"},
		{"lib/a.jpg", `C:\lib\a.jpg`, "/lib/a.jpg"},
		{`lib\a.jpg`, `C:\lib\a.jpg`, "/lib/a.jpg"},
		{"свадьба/1.jpg", `C:\свадьба\1.jpg`, "/свадьба/1.jpg"},
		{"", `C:\`, "/"},
	}
	for _, c := range cases {
		want := c.other
		if runtime.GOOS == "windows" {
			want = c.win
		}
		if got := AbsPath(c.in); got != want {
			t.Errorf("AbsPath(%q) = %q, ждали %q", c.in, got, want)
		}
	}
}
