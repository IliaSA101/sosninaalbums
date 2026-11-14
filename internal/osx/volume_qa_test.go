//go:build qa

package osx

import (
	"io/fs"
	"testing"
	"time"
)

// Шов SA_QA_VOLDELAY: тесты тоже под тегом qa (go test -tags qa ./internal/osx -run VolDelay).

func TestVolDelayParse(t *testing.T) {
	ok := map[string]struct {
		vol string
		d   time.Duration
	}{
		"Q:4000":  {"Q:", 4 * time.Second},
		"q:1":     {"Q:", time.Millisecond},
		"Z:30000": {"Z:", 30 * time.Second}, // ровно предел
	}
	for in, want := range ok {
		vol, d, good := qaVolDelay(in)
		if !good || vol != want.vol || d != want.d {
			t.Errorf("%q: %q %v %v, ждали %q %v", in, vol, d, good, want.vol, want.d)
		}
	}
	for _, in := range []string{"", "Q", "Q:", "Q:abc", "Q:-1", "Q:0", "Q:30001", ":4000", "QQ:4000", "1:4000",
		`C:\x:4000`, "Q:4000:1", "Q: 4000", "Q:99999999999999999999", "Q:9223372036855"} {
		if _, _, good := qaVolDelay(in); good {
			t.Errorf("%q: шов должен быть выключен", in)
		}
	}
}

// Вне стенда переменная игнорируется; на стенде первая проба буквы ждёт, вторая и чужая буква — нет.
func TestVolDelayNeedsStand(t *testing.T) {
	stat := func(string) (fs.FileInfo, error) { return nil, nil }
	var slept []time.Duration
	was, wasSleep := standOn, qaSleep
	defer func() { standOn, qaSleep = was, wasSleep }()
	qaSleep = func(d time.Duration) { slept = append(slept, d) }
	t.Setenv(qaVolDelayEnv, "Q:300")

	standOn = func() bool { return false }
	_, _ = qaStat(stat)(`Q:\`)
	if len(slept) != 0 {
		t.Fatalf("без стенда шов сработал: %v", slept)
	}

	standOn = func() bool { return true }
	f := qaStat(stat)
	_, _ = f(`E:\`)
	if len(slept) != 0 {
		t.Fatalf("чужая буква ждала: %v", slept)
	}
	_, _ = f(`q:\`)
	if len(slept) != 1 || slept[0] != 300*time.Millisecond {
		t.Fatalf("первая проба Q: ждала %v, ждали 300 мс", slept)
	}
	_, _ = f(`Q:\`)
	if len(slept) != 1 {
		t.Fatalf("вторая проба Q: ждала: %v", slept)
	}

	t.Setenv(qaVolDelayEnv, "Q:abc") // неверное значение — шов выключен и на стенде
	slept = nil
	_, _ = qaStat(stat)(`Q:\`)
	if len(slept) != 0 {
		t.Fatalf("неверное значение: проба ждала %v", slept)
	}
}
