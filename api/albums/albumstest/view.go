package albumstest

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"

	"github.com/IliaSA101/sosninaalbums/api/albums"
)

// Страницы клиента: /a/<id>/ (простая страница с картинками по порядку), manifest.json и файлы
// страниц. Альбом с паролем без входа отвечает 401. Вход — POST /a/<id>/login (поле password),
// cookie sa_session подписан HMAC и привязан к поколению входов (revoke_sessions его сбрасывает).

const (
	cookieName  = "sa_session"
	maxLoginLen = 4 << 10
)

var pageTmpl = template.Must(template.New("page").Parse(`<!doctype html>
<meta charset="utf-8"><title>{{.Title}}</title>
{{range .Files}}<img src="{{.}}" alt="">
{{end}}`))

var loginTmpl = template.Must(template.New("login").Parse(`<!doctype html>
<meta charset="utf-8"><title>Вход</title>
<form method="post" action="login"><input type="password" name="password" autocomplete="off"><button>Войти</button></form>`))

func fileName(n int, typ string) string { return fmt.Sprintf("%03d.%s", n, extOf(typ)) }

func (f *Fake) sign(id string, gen int) string {
	mac := hmac.New(sha256.New, f.key[:])
	_, _ = fmt.Fprintf(mac, "%s|%d", id, gen)
	return strconv.Itoa(gen) + "." + hex.EncodeToString(mac.Sum(nil))
}

// visible — опубликованный неистёкший альбом по id (CheckID и таблица до любых действий); под f.mu.
func (f *Fake) visible(id string) *album {
	if albums.CheckID(id) != nil {
		return nil
	}
	a := f.albums[id]
	if a == nil || a.pub == nil || f.expired(a) {
		return nil
	}
	return a
}

// allowed — вход не нужен или cookie годна. Под f.mu.
func (f *Fake) allowed(a *album, r *http.Request) bool {
	if !a.hasPW {
		return true
	}
	c, err := r.Cookie(cookieName)
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(c.Value), []byte(f.sign(a.id, a.gen))) == 1
}

func viewHeaders(w http.ResponseWriter) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "private, no-store")
	// id в адресе — ключ доступа: Referer с этой страницы не должен нести его на чужие сайты
	w.Header().Set("Referrer-Policy", "no-referrer")
}

func (f *Fake) locked(w http.ResponseWriter) {
	viewHeaders(w)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusUnauthorized)
	_ = loginTmpl.Execute(w, nil)
}

func (f *Fake) viewPage(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	a := f.visible(r.PathValue("id"))
	if a == nil {
		viewHeaders(w)
		http.NotFound(w, r)
		return
	}
	if !f.allowed(a, r) {
		f.locked(w)
		return
	}
	var files []string
	for _, p := range a.pub.Pages {
		if !p.Blank {
			files = append(files, p.File)
		}
	}
	viewHeaders(w)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = pageTmpl.Execute(w, struct {
		Title string
		Files []string
	}{a.pub.Title, files})
}

func (f *Fake) viewLogin(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	a := f.visible(r.PathValue("id"))
	if a == nil {
		viewHeaders(w)
		http.NotFound(w, r)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxLoginLen)
	if err := r.ParseForm(); err != nil {
		f.locked(w)
		return
	}
	pw := r.PostForm.Get("password")
	got := pwHash(a.salt, pw)
	if !a.hasPW || subtle.ConstantTimeCompare(got[:], a.pwHash[:]) != 1 {
		f.locked(w)
		return
	}
	// Secure только при https: в стенде на loopback по http cookie иначе не вернётся.
	http.SetCookie(w, &http.Cookie{ //nolint:gosec // G124: Secure вычисляется от схемы ссылок, HttpOnly и SameSite заданы
		Name: cookieName, Value: f.sign(a.id, a.gen), Path: "/a/" + a.id + "/",
		HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: strings.HasPrefix(f.url, "https://"),
	})
	viewHeaders(w)
	http.Redirect(w, r, f.link(a.id), http.StatusSeeOther)
}

func (f *Fake) viewManifest(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	a := f.visible(r.PathValue("id"))
	if a == nil {
		viewHeaders(w)
		http.NotFound(w, r)
		return
	}
	if !f.allowed(a, r) {
		f.locked(w)
		return
	}
	viewHeaders(w)
	// manifest.json — повторная сериализация типа Manifest: ни пароля, ни срока в нём быть не может.
	writeJSON(w, http.StatusOK, a.pub)
}

func (f *Fake) viewFile(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	a := f.visible(r.PathValue("id"))
	if a == nil {
		viewHeaders(w)
		http.NotFound(w, r)
		return
	}
	if !f.allowed(a, r) {
		f.locked(w)
		return
	}
	for _, p := range a.pub.Pages {
		if p.Blank || p.File != r.PathValue("file") {
			continue
		}
		data, ok := f.store.get(a.id, p.SHA256, extOf(p.Type))
		if !ok {
			break
		}
		viewHeaders(w)
		w.Header().Set("Content-Type", p.Type)
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		_, _ = w.Write(data)
		return
	}
	viewHeaders(w)
	http.NotFound(w, r)
}
