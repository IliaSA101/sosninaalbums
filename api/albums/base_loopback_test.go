package albums

import "testing"

// IsLoopbackHost — единое правило для http: ровно localhost и IP этого компьютера, без поддоменов.
func TestIsLoopbackHost(t *testing.T) {
	for host, want := range map[string]bool{
		"localhost": true, "127.0.0.1": true, "127.1.2.3": true, "::1": true, "::ffff:127.0.0.1": true,
		"app.localhost": false, "localhost.evil.org": false, "example.org": false, "10.0.0.5": false,
		"0.0.0.0": false, "::": false, "": false, "LOCALHOST": false, "127.0.0.1.evil.org": false,
	} {
		if got := IsLoopbackHost(host); got != want {
			t.Errorf("IsLoopbackHost(%q) = %v", host, got)
		}
	}
	if _, err := ParseBase("http://app.localhost"); err == nil {
		t.Error("http к поддомену localhost принят")
	}
}

// MayBeLoopbackHost — запрет «этот компьютер» в выпуске: localhost с точкой и регистром, *.localhost,
// формы IPv4-в-IPv6 и неопределённые адреса; похожие имена — нет.
func TestMayBeLoopbackHost(t *testing.T) {
	for host, want := range map[string]bool{
		"localhost": true, "localhost.": true, "LOCALHOST": true, "app.localhost": true, "APP.LOCALHOST.": true,
		"a.b.localhost": true, "127.0.0.1": true, "127.1.2.3": true, "::1": true,
		"::ffff:127.0.0.1": true, "::ffff:7f00:1": true, "::FFFF:7F00:1": true,
		"0.0.0.0": true, "::": true, "::ffff:0.0.0.0": true, "0:0:0:0:0:0:0:0": true,
		"xlocalhost": false, "localhost.evil.org": false, "localhostx": false, "example.org": false,
		"10.0.0.5": false, "::2": false, "::ffff:10.0.0.5": false, "": false, ".": false, "192.0.2.1": false,
	} {
		if got := MayBeLoopbackHost(host); got != want {
			t.Errorf("MayBeLoopbackHost(%q) = %v, ждали %v", host, got, want)
		}
	}
}
