package sharer

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/api/albums"
	"github.com/IliaSA101/sosninaalbums/api/albums/albumstest"
	"github.com/IliaSA101/sosninaalbums/internal/testkit"
)

// Тесты Т6 (0.32-146-T6): правка срока, ссылка, которой уже нет на сервере, тексты ошибок.

// deleteOnServer удаляет альбом текущей записи на сервисе мимо программы (владелец удалил его сам).
func deleteOnServer(r *rig, id string) {
	r.t.Helper()
	c, err := albums.New(r.srv.URL, r.srv.Token, albums.Options{})
	if err != nil {
		r.t.Fatal(err)
	}
	if err := c.Delete(context.Background(), id); err != nil {
		r.t.Fatal(err)
	}
}

// Продлить: срок записи берётся из ответа сервиса, а не из запроса. Сервис зажимает срок до своего
// max_days — в записи и в окне стоит зажатое число; после перезапуска (круг записи и чтения) то же.
func TestShareProlongFromResponse(t *testing.T) {
	r := newRigLimits(t, albums.Limits{MaxDays: 90})
	r.connect()
	r.must(Options{Days: 30})
	before, _ := r.svc.look(r.curJob().Path)
	day := 24 * time.Hour
	if d := time.Until(before.ExpiresAt); d < 29*day || d > 31*day {
		t.Fatalf("исходный срок: %v", before.ExpiresAt)
	}
	v, err := r.svc.Change(Change{Days: 365})
	if err != nil {
		t.Fatal(err)
	}
	rec, ok := r.svc.look(r.curJob().Path)
	if !ok {
		t.Fatal("записи нет")
	}
	if d := time.Until(rec.ExpiresAt); d < 89*day || d > 91*day {
		t.Errorf("срок %v: ждали ≈90 дней (зажим сервиса), не 365", rec.ExpiresAt)
	}
	if rec.ExpiresAt.Equal(before.ExpiresAt) || v.Link.ExpiresAt != rec.ExpiresAt.Format(time.RFC3339) || !strings.HasPrefix(v.Link.Until, "до ") {
		t.Errorf("запись %v, окно %+v", rec.ExpiresAt, v.Link)
	}
	if rec.URL != before.URL || rec.Password != before.Password || rec.ID != before.ID {
		t.Errorf("продление тронуло ссылку или пароль: было %+v стало %+v", before, rec)
	}
	r.restart()
	if got, _ := r.svc.look(r.curJob().Path); !got.ExpiresAt.Equal(rec.ExpiresAt) {
		t.Errorf("после перезапуска срок %v, ждали %v", got.ExpiresAt, rec.ExpiresAt)
	}
	// Сокращение тоже от сегодняшнего дня: 30 дней вместо 90.
	if _, err := r.svc.Change(Change{Days: 30}); err != nil {
		t.Fatal(err)
	}
	if got, _ := r.svc.look(r.curJob().Path); time.Until(got.ExpiresAt) > 31*day {
		t.Errorf("срок после «30 дней»: %v", got.ExpiresAt)
	}
}

// Change при 404 (альбом уже удалён на сервисе): ошибка «ссылки больше нет», запись цела и
// помечена «нет на сервере»; пароль и срок прежние. Повтор шлёт один PATCH и ничего не меняет.
func TestShareChangeGone(t *testing.T) {
	for _, c := range []Change{{Days: 90}, {SetPassword: true, Password: "new-5678"}} {
		r := newRig(t)
		r.connect()
		r.must(Options{Password: "pass-1234", Days: 30})
		id, _ := r.rec()
		before, _ := r.svc.look(r.curJob().Path)
		deleteOnServer(r, id)
		_, err := r.svc.Change(c)
		if err == nil || !strings.Contains(err.Error(), "Ссылки больше нет") || strings.Contains(err.Error(), "404") {
			t.Fatalf("%+v: ошибка %v", c, err)
		}
		rec, ok := r.svc.look(r.curJob().Path)
		if !ok || !rec.Gone || rec.ID != id || rec.URL != before.URL || rec.Password != "pass-1234" || !rec.ExpiresAt.Equal(before.ExpiresAt) {
			t.Fatalf("%+v: запись после 404: %v %+v", c, ok, rec)
		}
		if l := r.state().Link; l == nil || l.State != "gone" || l.URL != before.URL {
			t.Errorf("%+v: окно: %+v", c, l)
		}
		if r.svc.Busy() {
			t.Error("флаг выгрузки остался")
		}
		// Повтор: ровно один PATCH (сервис мог вернуть альбом), без повторов на 404; ошибка та же, запись цела.
		var patches atomic.Int32
		r.setFault(func(q *http.Request) *albumstest.Fault {
			if q.Method == http.MethodPatch {
				patches.Add(1)
			}
			return nil
		})
		if _, err := r.svc.Change(c); err == nil || !strings.Contains(err.Error(), "Ссылки больше нет") {
			t.Errorf("%+v: повтор: ошибка %v", c, err)
		}
		if n := patches.Load(); n != 1 {
			t.Errorf("%+v: повтор отправил %d запросов PATCH, ждали 1", c, n)
		}
		if again, _ := r.svc.look(r.curJob().Path); again != rec {
			t.Errorf("%+v: повтор изменил запись: %+v -> %+v", c, rec, again)
		}
	}
}

// Revoke при 404: сервис альбома не знает — отзывать нечего, запись убирается без ошибки; «Нет» на
// вопрос не трогает ничего, даже когда альбома на сервисе уже нет.
func TestShareRevokeGone(t *testing.T) {
	r := newRig(t)
	r.connect()
	r.must(Options{Days: 30})
	id, _ := r.rec()
	deleteOnServer(r, id)
	r.answer = false
	if v, err := r.svc.Revoke(); err != nil || v.Link == nil {
		t.Fatalf("«Нет»: %v %+v", err, v.Link)
	}
	r.answer = true
	v, err := r.svc.Revoke()
	if err != nil || v.Link != nil {
		t.Fatalf("«Да»: %v %+v", err, v.Link)
	}
	if _, ok := r.svc.look(r.curJob().Path); ok {
		t.Error("запись осталась в shares.json")
	}
	r.restart()
	if _, ok := r.svc.look(r.curJob().Path); ok {
		t.Error("запись вернулась после перезапуска")
	}
	if _, err := r.svc.Revoke(); err == nil {
		t.Error("повторный отзыв без ссылки принят")
	}
	// Запись, уже помеченная «нет на сервере», отзывается так же.
	r.must(Options{Days: 30})
	id2, _ := r.rec()
	deleteOnServer(r, id2)
	if _, err := r.svc.Change(Change{Days: 90}); err == nil {
		t.Fatal("правка по удалённому альбому без ошибки")
	}
	if l := r.state().Link; l == nil || l.State != "gone" {
		t.Fatalf("пометка: %+v", l)
	}
	if v, err := r.svc.Revoke(); err != nil || v.Link != nil {
		t.Errorf("отзыв помеченной записи: %v %+v", err, v.Link)
	}
}

// netFailure — настоящий сбой связи: запрос на порт, где никто не слушает.
func netFailure(t *testing.T) error {
	t.Helper()
	c, err := albums.New("http://127.0.0.1:1", "tok-0123456789abcdef", albums.Options{})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), testkit.Limit(10*time.Second))
	defer cancel()
	_, err = c.Health(ctx)
	var ne *albums.NetError
	if !errors.As(err, &ne) {
		t.Fatalf("ждали NetError: %v", err)
	}
	return err
}

// Таблица errText: каждому виду ошибки контракта — русская строка без сырых кодов, статусов
// ошибок-классов, «albums:», Message сервиса и адресов.
func TestErrTextTable(t *testing.T) {
	const secret = "SECRET-ТОКЕН-abc123"
	api := func(status int, code string) error {
		return &albums.APIError{Status: status, Code: code, Message: secret}
	}
	cases := []struct {
		name string
		err  error
		want string // фрагмент строки интерфейса
	}{
		{"400 bad_manifest", api(400, albums.CodeBadManifest), "Сервис не принял запрос (ошибка 400)"},
		{"400 bad_page", api(400, albums.CodeBadPage), "Сервис не принял запрос (ошибка 400)"},
		{"401 unauthorized", api(401, albums.CodeUnauthorized), "Токен не подошёл"},
		{"404 not_found", api(404, albums.CodeNotFound), "Ссылки больше нет"},
		{"409 hash_mismatch", api(409, albums.CodeHashMismatch), "повреждённой"},
		{"409 pages_missing", api(409, albums.CodePagesMissing), "Сервис не принял запрос (ошибка 409)"},
		{"413 too_large", api(413, albums.CodeTooLarge), "не принимает альбом такого размера"},
		{"429 rate_limited", api(429, albums.CodeRateLimited), "попробуйте позже"},
		{"500 internal", api(500, albums.CodeInternal), "ошибка 500"},
		{"502 без кода", api(502, ""), "ошибка 502"},
		{"ErrToken", albums.ErrToken, "Токен не подошёл"},
		{"ErrGone", albums.ErrGone, "Ссылки больше нет"},
		{"ErrLimit", albums.ErrLimit, "не принимает альбом такого размера"},
		{"ErrBusy", albums.ErrBusy, "попробуйте позже"},
		{"ErrRedirect", albums.ErrRedirect, "перенаправляет"},
		{"ErrResponse", albums.ErrResponse, "ответил непонятно"},
		{"ErrTooBig", albums.ErrTooBig, "больше 1 МБ"},
		{"сеть", netFailure(t), "Нет связи"},
		{"тайм-аут", context.DeadlineExceeded, "не ответил вовремя"},
		{"остановка", context.Canceled, "остановлена"},
		{"проверка альбома", &albums.ValidationError{Field: "pages", Reason: "слишком много"}, "не прошёл проверку: слишком много"},
		{"своя ошибка", fail("Файл страницы пропал во временной папке"), "Файл страницы пропал"},
		{"в обёртке", fmt.Errorf("страница 3: %w", api(409, albums.CodeHashMismatch)), "повреждённой"},
	}
	codes := []string{albums.CodeBadManifest, albums.CodeBadPage, albums.CodeUnauthorized, albums.CodeNotFound, albums.CodeHashMismatch,
		albums.CodePagesMissing, albums.CodeTooLarge, albums.CodeRateLimited, albums.CodeInternal}
	for _, c := range cases {
		got := errText(c.err)
		if !strings.Contains(got, c.want) {
			t.Errorf("%s: %q, ждали «%s»", c.name, got, c.want)
		}
		for _, raw := range append(codes, "albums:", secret, "127.0.0.1", "APIError") {
			if strings.Contains(got, raw) {
				t.Errorf("%s: в тексте сырое «%s»: %q", c.name, raw, got)
			}
		}
		if !strings.ContainsAny(got, "абвгдеёжзийклмнопрстуфхцчшщъыьэюя") {
			t.Errorf("%s: текст не по-русски: %q", c.name, got)
		}
	}
	// Все коды контракта покрыты таблицей: новый код без строки здесь — повод завести для него текст.
	seen := map[string]bool{}
	for _, c := range cases {
		if ae := (*albums.APIError)(nil); errors.As(c.err, &ae) {
			seen[ae.Code] = true
		}
	}
	for _, code := range codes {
		if !seen[code] {
			t.Errorf("код %s не в таблице", code)
		}
	}
}

// Тексты ошибок одной выгрузки по настоящему проводу: статус и код доходят до окна как русская строка.
func TestShareErrTextOnWire(t *testing.T) {
	cases := []struct {
		status int
		want   string
	}{
		{http.StatusForbidden, "ошибка 403"},
		{http.StatusRequestEntityTooLarge, "не принимает альбом такого размера"},
		{http.StatusBadRequest, "ошибка 400"},
	}
	for _, c := range cases {
		r := newRig(t)
		r.connect()
		r.setFault(func(q *http.Request) *albumstest.Fault {
			if q.Method == http.MethodPost && q.URL.Path == "/v1/albums" {
				return &albumstest.Fault{Status: c.status}
			}
			return nil
		})
		msg, stopped := r.run(Options{Days: 30})
		if stopped || !strings.Contains(msg, c.want) || strings.Contains(msg, "bad_manifest") || strings.Contains(msg, "too_large") {
			t.Errorf("статус %d: %q", c.status, msg)
		}
	}
}
