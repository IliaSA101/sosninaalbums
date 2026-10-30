package albumstest

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"mime"
	"net/http"
	"sort"
	"time"

	"github.com/IliaSA101/sosninaalbums/api/albums"
)

// link — ссылка на страницу клиента. Под f.mu.
func (f *Fake) link(id string) string { return f.url + "/a/" + id + "/" }

// clamp зажимает срок к max_days от сейчас; итог в UTC.
func (f *Fake) clamp(t time.Time) time.Time {
	limit := f.opt.Now().Add(time.Duration(f.lim.MaxDays) * 24 * time.Hour)
	if t.After(limit) {
		t = limit
	}
	return t.UTC().Truncate(time.Second)
}

// missing — номера небелых страниц редакции, чьих картинок у альбома нет.
func (a *album) missing() []int {
	out := []int{}
	for _, p := range a.pending.Pages {
		if !p.Blank && !a.stored[p.SHA256] {
			out = append(out, p.N)
		}
	}
	return out
}

func (f *Fake) list(w http.ResponseWriter, _ *http.Request, owner string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := albums.AlbumList{Albums: []albums.Album{}}
	for _, a := range f.albums {
		if a.owner != owner || f.expired(a) {
			continue
		}
		out.Albums = append(out.Albums, f.view(a))
	}
	sort.Slice(out.Albums, func(i, j int) bool { return out.Albums[i].ID < out.Albums[j].ID })
	writeJSON(w, http.StatusOK, out)
}

func (f *Fake) view(a *album) albums.Album {
	return albums.Album{ID: a.id, URL: f.link(a.id), Title: a.pending.Title, ExpiresAt: a.expires, HasPassword: a.hasPW, Published: a.pub != nil, Pages: len(a.pending.Pages)}
}

func (f *Fake) create(w http.ResponseWriter, r *http.Request, owner string) {
	var req albums.CreateRequest
	if !readJSON(w, r, &req) {
		return
	}
	if err := albums.ValidateCreate(&req, f.lim, f.opt.Now()); err != nil {
		writeErr(w, http.StatusBadRequest, albums.CodeBadManifest, err.Error())
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	a := &album{id: newID(), owner: owner, pending: req.Manifest, expires: f.clamp(req.ExpiresAt), stored: map[string]bool{}}
	f.setPassword(a, req.Password)
	f.albums[a.id] = a
	writeJSON(w, http.StatusCreated, albums.Created{ID: a.id, URL: f.link(a.id), ExpiresAt: a.expires, Missing: a.missing()})
}

func (f *Fake) setPassword(a *album, pw string) {
	a.hasPW = pw != ""
	if !a.hasPW {
		a.pwHash = [32]byte{}
		return
	}
	a.salt = [16]byte{}
	copy(a.salt[:], randHex(8))
	a.pwHash = pwHash(a.salt, pw)
}

func pwHash(salt [16]byte, pw string) [32]byte { return sha256.Sum256(append(salt[:], pw...)) }

func (f *Fake) update(w http.ResponseWriter, r *http.Request, owner string) {
	var req albums.UpdateRequest
	if !readJSON(w, r, &req) {
		return
	}
	if err := albums.Validate(&req.Manifest, f.lim); err != nil {
		writeErr(w, http.StatusBadRequest, albums.CodeBadManifest, err.Error())
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	a := f.lookup(r.PathValue("id"), owner)
	if a == nil {
		notFound(w)
		return
	}
	a.pending = req.Manifest
	writeJSON(w, http.StatusOK, albums.Updated{Missing: a.missing()})
}

func (f *Fake) putPage(w http.ResponseWriter, r *http.Request, owner string) {
	f.mu.Lock()
	a := f.lookup(r.PathValue("id"), owner)
	if a == nil {
		f.mu.Unlock()
		notFound(w)
		return
	}
	n, err := albums.ParsePageN(r.PathValue("n"), len(a.pending.Pages))
	if err != nil {
		f.mu.Unlock()
		writeErr(w, http.StatusBadRequest, albums.CodeBadPage, "номер страницы вне книги")
		return
	}
	page := a.pending.Pages[n-1]
	f.mu.Unlock()

	ctype, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if page.Blank || ctype != page.Type {
		writeErr(w, http.StatusBadRequest, albums.CodeBadPage, "белая страница или Content-Type не как в манифесте")
		return
	}
	if r.ContentLength > f.lim.PageBytes {
		writeErr(w, http.StatusRequestEntityTooLarge, albums.CodeTooLarge, "страница больше page_bytes")
		return
	}
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, f.lim.PageBytes))
	if err != nil {
		writeErr(w, http.StatusRequestEntityTooLarge, albums.CodeTooLarge, "страница больше page_bytes")
		return
	}
	if !signatureOK(page.Type, data) {
		writeErr(w, http.StatusBadRequest, albums.CodeBadPage, "содержимое не "+page.Type)
		return
	}
	sum := sha256.Sum256(data)
	if int64(len(data)) != page.Bytes || hex.EncodeToString(sum[:]) != page.SHA256 {
		writeErr(w, http.StatusConflict, albums.CodeHashMismatch, "sha256 или размер не как в манифесте")
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	// Альбом мог исчезнуть, пока читали тело.
	if a = f.lookup(r.PathValue("id"), owner); a == nil {
		notFound(w)
		return
	}
	if err := f.store.put(a.id, page.SHA256, extOf(page.Type), data); err != nil {
		writeErr(w, http.StatusInternalServerError, albums.CodeInternal, "диск")
		return
	}
	a.stored[page.SHA256] = true
	f.puts++
	w.WriteHeader(http.StatusNoContent)
}

func extOf(typ string) string {
	if typ == albums.TypeWebP {
		return "webp"
	}
	return "jpg"
}

// signatureOK — начало данных соответствует типу: JPEG FF D8 FF, WebP RIFF….WEBP.
func signatureOK(typ string, b []byte) bool {
	switch typ {
	case albums.TypeJPEG:
		return len(b) >= 3 && b[0] == 0xFF && b[1] == 0xD8 && b[2] == 0xFF
	case albums.TypeWebP:
		return len(b) >= 12 && bytes.Equal(b[:4], []byte("RIFF")) && bytes.Equal(b[8:12], []byte("WEBP"))
	}
	return false
}

func (f *Fake) publish(w http.ResponseWriter, r *http.Request, owner string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	a := f.lookup(r.PathValue("id"), owner)
	if a == nil {
		notFound(w)
		return
	}
	if miss := a.missing(); len(miss) > 0 {
		writeJSON(w, http.StatusConflict, albums.ErrorBody{Error: albums.Error{Code: albums.CodePagesMissing, Message: "не все страницы загружены", Missing: miss}})
		return
	}
	pub := a.pending
	pub.Pages = append([]albums.Page(nil), a.pending.Pages...)
	for i := range pub.Pages {
		if p := &pub.Pages[i]; !p.Blank {
			p.File = fileName(p.N, p.Type)
		}
	}
	a.pub = &pub
	writeJSON(w, http.StatusOK, albums.Published{URL: f.link(a.id), ExpiresAt: a.expires})
}

func (f *Fake) patch(w http.ResponseWriter, r *http.Request, owner string) {
	var p albums.Patch
	if !readJSON(w, r, &p) {
		return
	}
	if err := albums.ValidatePatch(&p, f.opt.Now()); err != nil {
		writeErr(w, http.StatusBadRequest, albums.CodeBadManifest, err.Error())
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	a := f.lookup(r.PathValue("id"), owner)
	if a == nil {
		notFound(w)
		return
	}
	if p.ExpiresAt != nil {
		a.expires = f.clamp(*p.ExpiresAt)
	}
	if p.Password != nil {
		f.setPassword(a, *p.Password)
	}
	if p.RevokeSessions {
		a.gen++
	}
	writeJSON(w, http.StatusOK, f.view(a))
}

func (f *Fake) remove(w http.ResponseWriter, r *http.Request, owner string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	a := f.lookup(r.PathValue("id"), owner)
	if a == nil {
		notFound(w)
		return
	}
	f.drop(a)
	w.WriteHeader(http.StatusNoContent)
}
