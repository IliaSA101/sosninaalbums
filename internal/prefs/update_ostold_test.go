package prefs

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// osTold: запись → чтение; вместе с Seen и Checked — одной записью; ключа нет, пока всё пусто.
// Объект update без ключа (старая версия переписала config.json) — тост повторится.
func TestUpdateOSToldRoundTrip(t *testing.T) {
	dir := testkit.TempDir(t)
	if u := UpdateOf(dir); u.OSTold != "" {
		t.Fatalf("без файла: %+v", u)
	}
	UpdateUpdate(dir, func(u *Update) { u.Checked, u.Seen, u.OSTold = 5, "0.27.0", "0.27.0" })
	if u := UpdateOf(dir); u.OSTold != "0.27.0" || u.Seen != "0.27.0" || u.Checked != 5 {
		t.Fatalf("круг: %+v", u)
	}
	UpdateUpdate(dir, func(u *Update) { u.OSTold = "0.28.1" }) // другая версия — перезапись, остальное цело
	if u := UpdateOf(dir); u.OSTold != "0.28.1" || u.Seen != "0.27.0" {
		t.Fatalf("новая версия: %+v", u)
	}
	// Старая версия программы сохранила объект update без osTold.
	if err := os.WriteFile(ConfigPath(dir), []byte(`{"update":{"checked":5,"seen":"0.27.0"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if u := UpdateOf(dir); u.OSTold != "" || u.Seen != "0.27.0" {
		t.Errorf("объект без ключа: %+v", u)
	}
	// Одно osTold — тоже непустой объект; сброс — ключа update нет.
	UpdateUpdate(dir, func(u *Update) { *u = Update{OSTold: "0.30.0"} })
	if u := UpdateOf(dir); u.OSTold != "0.30.0" {
		t.Errorf("одно osTold: %+v", u)
	}
	UpdateUpdate(dir, func(u *Update) { *u = Update{} })
	if c := LoadConfig(dir); c.Update != nil {
		t.Errorf("после сброса ключ остался: %+v", c.Update)
	}
}

// Правка руками: osTold не версия — пусто (как seen); пустой список и пограничные значения.
func TestUpdateCleanOSTold(t *testing.T) {
	dir := testkit.TempDir(t)
	for _, c := range []struct{ in, want string }{
		{"0.27.0", "0.27.0"}, {"1.2.3", "1.2.3"}, {"9999.9999.9999", "9999.9999.9999"},
		{"", ""}, {"0.27", ""}, {"0.27.0.1", ""}, {"v0.27.0", ""}, {"0.27.0\n", ""}, {"10000.0.0", ""}, {"../x", ""}, {" ", ""},
	} {
		q, _ := json.Marshal(c.in)
		if err := os.WriteFile(ConfigPath(dir), []byte(`{"update":{"osTold":`+string(q)+`,"checked":1}}`), 0o644); err != nil {
			t.Fatal(err)
		}
		if got := UpdateOf(dir).OSTold; got != c.want {
			t.Errorf("osTold %q → %q, ждали %q", c.in, got, c.want)
		}
	}
	// Не строка — config.json не читается как есть: настройки по умолчанию, без паники.
	if err := os.WriteFile(ConfigPath(dir), []byte(`{"update":{"osTold":5}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := UpdateOf(dir); got.OSTold != "" {
		t.Errorf("osTold числом: %+v", got)
	}
}
