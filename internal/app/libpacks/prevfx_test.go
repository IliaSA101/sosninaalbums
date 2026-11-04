package libpacks

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"image"
	"image/color"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/decor/fetch"
	"github.com/IliaSA101/sosninaalbums/internal/prefs"
)

// Стенд тестов образцов (спека 144 Г2): служба над своей папкой настроек, библиотека на httptest
// и листы JPEG, нарисованные в тесте (клетка — свой цвет).

// Раскладка тестового листа: ряд фонов 3 × 64×48 и ряд стикеров 4 × 32×32.
const (
	fxW, fxH = 192, 80
	fxPrefix = "/dl/"
)

// palette — цвета клеток по порядку: ряд фонов, затем ряд стикеров.
var palette = []color.RGBA{
	{220, 30, 30, 255}, {30, 200, 30, 255}, {30, 30, 220, 255},
	{230, 230, 40, 255}, {40, 230, 230, 255}, {230, 40, 230, 255}, {20, 20, 20, 255},
}

// sheetJPEG — лист fxW×fxH: клетка j ряда bg — palette[j], клетка j ряда sticker — palette[3+j].
func sheetJPEG(t *testing.T, tweak int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, fxW, fxH))
	fill := func(r image.Rectangle, c color.RGBA) {
		for y := r.Min.Y; y < r.Max.Y; y++ {
			for x := r.Min.X; x < r.Max.X; x++ {
				img.SetRGBA(x, y, c)
			}
		}
	}
	for j := range 3 {
		fill(image.Rect(j*64, 0, (j+1)*64, 48), palette[j])
	}
	for j := range 4 {
		fill(image.Rect(j*32, 48, (j+1)*32, 80), palette[3+j])
	}
	fill(image.Rect(fxW-8, fxH-8, fxW, fxH), color.RGBA{uint8(tweak), 0, 0, 255}) // разные байты у разных листов
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// previewOf — поле preview для байтов листа data (раскладка выше), адрес — name на стенде.
func previewOf(url string, data []byte) *fetch.Preview {
	sum := sha256.Sum256(data)
	items := func(prefix string, n int) []string {
		out := make([]string, n)
		for i := range out {
			out[i] = prefix + strings.Repeat("0", 14) + string(rune('0'+i))
		}
		return out
	}
	return &fetch.Preview{URL: url, SHA256: hex.EncodeToString(sum[:]), Size: int64(len(data)), W: fxW, H: fxH,
		Rows: []fetch.PreviewRow{
			{Kind: "bg", W: 64, H: 48, Items: items("b", 3)},
			{Kind: "sticker", W: 32, H: 32, Items: items("c", 4)},
		}}
}

type prevFx struct {
	t   *testing.T
	dir string
	ev  *evs
	s   *Service
	srv *httptest.Server

	mu       sync.Mutex
	body     map[string][]byte // имя листа → тело
	hits     map[string]int
	override map[string]http.HandlerFunc
	seq      int64
}

// newPrevFx — служба с согласием на сеть и стендом; mut правит Host до New.
func newPrevFx(t *testing.T, mut func(h *Host)) *prevFx {
	t.Helper()
	fx := &prevFx{t: t, ev: &evs{}, body: map[string][]byte{}, hits: map[string]int{}, override: map[string]http.HandlerFunc{}}
	fx.srv = httptest.NewTLSServer(http.HandlerFunc(fx.serve))
	t.Cleanup(fx.srv.Close)
	src := fetch.Source{Prefix: fx.srv.URL + fxPrefix, Hosts: []string{"127.0.0.1"}, CatalogURL: fx.srv.URL + "/c.json",
		Roots: fx.srv.Client().Transport.(*http.Transport).TLSClientConfig.RootCAs}
	h := testHost(t, fx.ev, func(h *Host) {
		h.Fetcher = func() *fetch.Client { return fetch.New(src) }
		if mut != nil {
			mut(h)
		}
	})
	fx.s, fx.dir = New(h), h.CfgDir()
	prefs.UpdateLibraryNet(fx.dir, func(n *prefs.LibraryNet) { n.Allowed = true })
	return fx
}

func (fx *prevFx) serve(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, fxPrefix)
	fx.mu.Lock()
	fx.hits[name]++
	h := fx.override[name]
	body, ok := fx.body[name]
	fx.mu.Unlock()
	switch {
	case h != nil:
		h(w, r)
	case !ok:
		http.NotFound(w, r)
	default:
		_, _ = w.Write(body)
	}
}

// publish — лист name на стенде и строка каталога id с ним; возвращает preview строки.
func (fx *prevFx) publish(id, name string, data []byte) *fetch.Preview {
	fx.mu.Lock()
	fx.body[name] = data
	fx.mu.Unlock()
	return previewOf(fx.srv.URL+fxPrefix+name, data)
}

// list — сохранённый список из строк (id → preview): память, без файла.
func (fx *prevFx) list(rows ...fetch.Entry) {
	fx.t.Helper()
	fx.seq++
	for i := range rows {
		rows[i].Version, rows[i].MinApp, rows[i].Size = 1, "0.1.0", 10
		rows[i].Title, rows[i].SHA256 = "T "+rows[i].ID, strings.Repeat("a", 64)
		rows[i].URL = fx.srv.URL + fxPrefix + rows[i].ID + ".zip"
	}
	fx.s.remember(fetch.Catalog{Seq: fx.seq, Packs: rows})
}

func (fx *prevFx) hitsOf(name string) int {
	fx.mu.Lock()
	defer fx.mu.Unlock()
	return fx.hits[name]
}

func (fx *prevFx) totalHits() (n int) {
	fx.mu.Lock()
	defer fx.mu.Unlock()
	for _, h := range fx.hits {
		n += h
	}
	return n
}

func (fx *prevFx) setOverride(name string, h http.HandlerFunc) {
	fx.mu.Lock()
	fx.override[name] = h
	fx.mu.Unlock()
}

// folder — имена в папке листов.
func (fx *prevFx) folder() []string {
	list, err := os.ReadDir(filepath.Join(fx.dir, previewDirName))
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range list {
		out = append(out, e.Name())
	}
	return out
}

func (fx *prevFx) sheetFile(p *fetch.Preview) string {
	return filepath.Join(fx.dir, previewDirName, p.SHA256+previewExt)
}

// advance — часы службы уходят вперёд (паузы заданий).
func (fx *prevFx) advance(d time.Duration) {
	cur := fx.s.h.Now
	fx.s.h.Now = func() time.Time { return cur().Add(d) }
}
