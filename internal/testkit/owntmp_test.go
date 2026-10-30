package testkit

import "testing"

func TestOwnTmp(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		// свои
		{"import-123456.tmp", true},
		{"import-.tmp", true},
		{".dl-abc.tmp", true},
		{"album.salbum.sacopy.tmp", true},
		{"album.salbum.123456789.tmp", true},
		{"05.jpg.7.tmp", true},
		{"1-05.jpg.42.tmp", true},
		{"8355541DA24DC41F.png.1234.tmp", true},
		{"8355541DA24DC41F.PNG.1234.TMP", true},
		// чужие и не временные
		{"8355541DA24DC41F.PNG.tmp", false},
		{"02.JPG.tmp", false},
		{"ABC~1.tmp", false},
		{"x.tmp", false},
		{".tmp", false},
		{".sacopy.tmp", false},
		{".7.tmp", false},
		{"a.7x.tmp", false},
		{"import-1.png", false},
		{"05.jpg", false},
		{"", false},
	}
	for _, c := range cases {
		if got := OwnTmp(c.name); got != c.want {
			t.Errorf("OwnTmp(%q) = %v, ждали %v", c.name, got, c.want)
		}
	}
}
