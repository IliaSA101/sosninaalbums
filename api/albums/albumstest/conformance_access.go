package albumstest

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/api/albums"
)

// password: пароль не встречается в ответах API; страницы с паролем без входа закрыты.
func (c *conf) password(t *testing.T) {
	const word = "Пароль-Conf-7391"
	bk := c.newBook(30)
	// Создание сырым запросом: клиент albums разбирает ответ в типы и отбросил бы лишнее поле password.
	r := c.createRaw(t, c.request(bk, word))
	var cr albums.Created
	if r.status != 201 || json.Unmarshal(r.body, &cr) != nil {
		t.Fatalf("создание с паролем: %d %s", r.status, r.errCode())
	}
	c.noSecret(t, "создание", r.body, word)
	c.upload(t, cr.ID, bk, cr.Missing)
	if _, err := c.api.Publish(context.Background(), cr.ID); err != nil {
		t.Fatal(err)
	}
	list, err := c.api.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, a := range list {
		if a.ID == cr.ID {
			found = a.HasPassword && a.Published
		}
	}
	if !found {
		t.Error("в списке нет альбома с has_password и published")
	}
	// верный вход нужен, чтобы узнать имя файла страницы: оно выбирается сервисом
	s := c.login(t, cr.ID, word)
	if s.cookie == "" || s.status < 200 || s.status >= 400 {
		t.Fatalf("вход: cookie %q, статус %d", s.cookie, s.status)
	}
	for _, p := range cookieProblems(s.set, strings.HasPrefix(c.base, "https://")) {
		t.Error(p)
	}
	file := c.checkViewer(t, cr.ID, s.cookie, bk, word)
	// без входа, с поддельной и с искажённой настоящей cookie закрыто всё: страница, манифест, файл страницы
	c.denyCookie(t, "без входа", cr.ID, "", file)
	c.denyCookie(t, "подделанная cookie", cr.ID, "sa_session=0.deadbeef", file)
	c.denyCookie(t, "искажённая cookie", cr.ID, tamper(s.cookie), file)
	c.denyLogin(t, "неверный пароль", cr.ID, word+"x", file)
	c.denyLogin(t, "пустой пароль", cr.ID, "", file)
	// Пароль не в ответах API: сырые ответы, а не разобранные типы. После проверок выше: PATCH с паролем
	// иначе скрыл бы потерю пароля при создании.
	c.patchRaw(t, cr.ID, albums.Patch{Password: ptr(word), RevokeSessions: true}, word)
	for what, r := range map[string]*resp{
		"список":  c.do(t, "GET", "/v1/albums", c.token, nil, nil),
		"publish": c.do(t, "POST", "/v1/albums/"+cr.ID+"/publish", c.token, nil, nil),
		"PUT":     c.json(t, "PUT", "/v1/albums/"+cr.ID, c.token, albums.UpdateRequest{Manifest: bk.m}),
	} {
		if r.status != 200 {
			t.Errorf("ответ API (%s): %d", what, r.status)
		}
		c.noSecret(t, what, r.body, word)
	}
}

// revoke: сброс входов, смена и снятие пароля, срок.
func (c *conf) revoke(t *testing.T) {
	const word, word2 = "Первый-пароль-12", "Второй-пароль-34"
	bk := c.newBook(40)
	cr := c.published(t, bk, word)
	id := cr.ID
	ck := c.login(t, id, word).cookie
	if ck == "" {
		t.Fatal("вход не удался")
	}
	// имя файла страницы — из манифеста с действующей cookie; до сброса вход открывает всё
	file := c.checkViewer(t, id, ck, bk, word)
	if file == "" {
		t.Fatal("до сброса вход ничего не открыл")
	}
	if _, err := c.api.Patch(context.Background(), id, &albums.Patch{RevokeSessions: true}); err != nil {
		t.Fatal(err)
	}
	c.denyCookie(t, "после revoke_sessions прежний вход", id, ck, file)
	if ck2 := c.login(t, id, word).cookie; ck2 == "" {
		t.Error("после сброса новый вход не удался")
	} else if r := c.get(t, "/a/"+id+"/manifest.json", ck2); r.status != 200 {
		t.Errorf("новый вход: %d", r.status)
	}
	// смена пароля: старый не подходит (ответ — без нового пароля)
	a := c.patchRaw(t, id, albums.Patch{Password: ptr(word2), RevokeSessions: true}, word, word2)
	if a == nil || !a.HasPassword {
		t.Fatalf("смена пароля: %+v", a)
	}
	c.denyLogin(t, "старый пароль после смены", id, word, file)
	if c.login(t, id, word2).cookie == "" {
		t.Error("новый пароль не подошёл")
	}
	// снятие пароля: страницы открыты без входа
	a, err := c.api.Patch(context.Background(), id, &albums.Patch{Password: ptr("")})
	if err != nil || a.HasPassword {
		t.Fatalf("снятие пароля: %+v %v", a, err)
	}
	c.checkViewer(t, id, "", bk)
	// срок: ответ — итоговый, не позже запрошенного
	want := time.Now().Add(72 * time.Hour).UTC().Truncate(time.Second)
	a, err = c.api.Patch(context.Background(), id, &albums.Patch{ExpiresAt: &want})
	if err != nil || a.ExpiresAt.After(want.Add(time.Second)) || a.ExpiresAt.Before(time.Now()) {
		t.Errorf("срок: %+v %v (запрошен %v)", a, err, want)
	}
}

// lifecycle: правка при опубликованном альбоме, missing по хэшам, удаление.
func (c *conf) lifecycle(t *testing.T) {
	bk := c.newBook(50)
	cr := c.published(t, bk, "")
	id := cr.ID
	// новая редакция: страница 3 другая — нужна только она
	imgs := map[int][]byte{}
	for n, data := range bk.imgs {
		imgs[n] = data
	}
	imgs[3] = c.sample(77)
	bk2 := newBookWith(imgs)
	up, err := c.api.Update(context.Background(), id, &bk2.m)
	if err != nil || !equalInts(up.Missing, []int{3}) {
		t.Fatalf("Update: %+v %v", up, err)
	}
	// до publish клиент видит прежнюю редакцию
	var man albums.Manifest
	_ = json.Unmarshal(c.get(t, "/a/"+id+"/manifest.json", "").body, &man)
	if len(man.Pages) != 4 || man.Pages[2].SHA256 != bk.m.Pages[2].SHA256 {
		t.Errorf("до publish видна не прежняя редакция: %+v", man.Pages)
	}
	if _, err := c.api.Publish(context.Background(), id); err == nil {
		t.Error("publish без новой страницы прошёл")
	}
	c.upload(t, id, bk2, []int{3})
	if _, err := c.api.Publish(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	c.checkViewer(t, id, "", bk2)
	// список
	list, err := c.api.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var seen bool
	for _, a := range list {
		if a.ID == id {
			seen = a.Pages == 4 && a.Published && !a.HasPassword
		}
	}
	if !seen {
		t.Error("в списке нет альбома (4 страницы, опубликован, без пароля)")
	}
	// удаление: сразу и насовсем
	if err := c.api.Delete(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	if err := c.api.Delete(context.Background(), id); !errors.Is(err, albums.ErrGone) {
		t.Errorf("повторное удаление: %v", err)
	}
	if err := c.api.PutPage(context.Background(), id, 1, albums.TypeJPEG, bytes.NewReader(bk.imgs[1]), int64(len(bk.imgs[1]))); !errors.Is(err, albums.ErrGone) {
		t.Errorf("страница удалённого: %v", err)
	}
	if r := c.get(t, "/a/"+id+"/manifest.json", ""); r.status != 404 {
		t.Errorf("манифест удалённого: %d", r.status)
	}
}
