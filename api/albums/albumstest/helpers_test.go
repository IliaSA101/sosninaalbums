package albumstest

import (
	"bytes"
	"errors"
	"image/jpeg"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/api/albums"
)

// TestCreateRawCleansUp: альбом, созданный по ошибке сервера (201 на негодный запрос), удаляется в конце блока.
func TestCreateRawCleansUp(t *testing.T) {
	f, err := NewFake(Options{})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(mutants(f.opt.Token)["badfile"].wrap(f)) // принимает file с обходом пути
	defer srv.Close()
	f.SetURL(srv.URL)
	api, err := albums.New(srv.URL, f.opt.Token, albums.Options{})
	if err != nil {
		t.Fatal(err)
	}
	c := &conf{t: t, base: srv.URL, token: f.opt.Token, api: api, http: srv.Client()}
	t.Run("блок", func(t *testing.T) {
		bk := newBook(5)
		bk.m.Pages[0].File = "../../x"
		if r := c.createRaw(t, c.request(bk, "")); r.status != 201 {
			t.Fatalf("мутант не принял запрос: %d", r.status)
		}
		if f.Albums() != 1 {
			t.Fatalf("альбомов %d, нужен 1", f.Albums())
		}
	})
	if f.Albums() != 0 {
		t.Errorf("после блока осталось альбомов: %d", f.Albums())
	}
}

// TestNetCause: причина сбоя печатается без адреса — в нём id альбома (ключ доступа) или токен.
func TestNetCause(t *testing.T) {
	err := &url.Error{Op: "Get", URL: "http://h/a/SECRETID/manifest.json?token=SECRETTOKEN", Err: errors.New("connection refused")}
	got := netCause(err).Error()
	if strings.Contains(got, "SECRET") || got != "connection refused" {
		t.Errorf("netCause = %q", got)
	}
	if plain := errors.New("x"); netCause(plain) != plain {
		t.Error("не url.Error должна проходить как есть")
	}
}

// TestSampleSalt: картинки разных прогонов различаются байтами, но остаются годным JPEG одного размера.
func TestSampleSalt(t *testing.T) {
	a, b := sample(7, [8]byte{1}), sample(7, [8]byte{2})
	if string(a) == string(b) {
		t.Error("соль не меняет байты картинки")
	}
	if len(a) != len(b) || a[0] != 0xFF || a[1] != 0xD8 {
		t.Errorf("картинка с солью: %d и %d байт, начало % x", len(a), len(b), a[:4])
	}
	if _, err := jpeg.Decode(bytes.NewReader(a)); err != nil {
		t.Errorf("картинка с солью не декодируется: %v", err)
	}
	if pageOf(1, a).SHA256 == pageOf(1, b).SHA256 {
		t.Error("хэши совпали")
	}
}
