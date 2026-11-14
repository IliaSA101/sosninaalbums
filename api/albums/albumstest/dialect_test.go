package albumstest

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// hashFirst — сервер, который проверяет хэш раньше сигнатуры: не-JPEG под image/jpeg получает 409
// hash_mismatch вместо 400 bad_page (контракт порядок не фиксирует).
func hashFirst(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isPut(r) {
			next.ServeHTTP(w, r)
			return
		}
		raw, _ := io.ReadAll(r.Body)
		r.Body = io.NopCloser(bytes.NewReader(raw))
		bw := &bodyWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(bw, r)
		out, st := bw.buf.Bytes(), bw.status
		if st == 400 && r.Header.Get("Content-Type") == "image/jpeg" && !bytes.HasPrefix(raw, []byte{0xFF, 0xD8, 0xFF}) {
			st, out = 409, bytes.ReplaceAll(out, []byte("bad_page"), []byte("hash_mismatch"))
		}
		w.WriteHeader(st)
		_, _ = w.Write(out)
	})
}

// TestConformanceDialect: законное отличие реализации от фейка не должно ронять набор — имена файлов
// страниц свои («img-…», берутся из манифеста), неверный пароль входа — 200 со страницей формы и
// cookie, ничего не открывающей, хэш проверяется раньше сигнатуры.
func TestConformanceDialect(t *testing.T) {
	if testing.Short() {
		t.Skip("полный набор проверок: не в быстром уровне")
	}
	f, err := NewFake(Options{OtherToken: otherTok})
	if err != nil {
		t.Fatal(err)
	}
	rename := onBody(isManifest, func(_ []byte, m map[string]any) {
		pages, _ := m["pages"].([]any)
		for _, p := range pages {
			if pm, ok := p.(map[string]any); ok {
				if file, _ := pm["file"].(string); file != "" {
					pm["file"] = "img-" + file
				}
			}
		}
	})
	unrename := onRequest(func(r *http.Request) bool {
		return strings.HasPrefix(r.URL.Path, "/a/") && strings.Contains(r.URL.Path, "/img-")
	},
		func(r *http.Request) { r.URL.Path = strings.Replace(r.URL.Path, "/img-", "/", 1) })
	formOn200 := onStatus(func(r *http.Request) bool { return r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/login") },
		func(h http.Header, s int) int {
			if s == 401 {
				h.Set("Set-Cookie", "sa_session=0.bad; Path=/") // cookie на неверный пароль, не открывающая ничего: разрешено
				return 200
			}
			return s
		})
	srv := httptest.NewServer(unrename(rename(formOn200(hashFirst(f)))))
	defer srv.Close()
	f.SetURL(srv.URL)
	Conformance(t, srv.URL, f.opt.Token, WithOtherToken(otherTok))
}
