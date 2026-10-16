package decor

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"image"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/imgfile"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// Одноразовые токены SVG (design §5.4): Go проверил SVG и отдал его окну вместе с токеном; окно
// нарисовало PNG и прислало его по токену. Токен связывает PNG с проверенным файлом: ключ,
// тема, имя и размер берутся из токена, а не от интерфейса — под чужим именем ничего не
// попадёт в библиотеку.

// Пределы токенов и PNG от окна.
const (
	svgTokenTTL  = 10 * time.Minute
	maxSVGTokens = 50
	MaxPNGBase64 = 48 << 20 // PNG 4096² с шумом — десятки МБ; больше — не наше
)

// SvgJob — задание окну: нарисовать Data (SVG в base64) холстом W×H и прислать PNG с Token.
type SvgJob struct {
	Token string `json:"token"`
	Name  string `json:"name"`
	Data  string `json:"data"`
	W     int    `json:"w"`
	H     int    `json:"h"`
}

type pendingSVG struct {
	kind, theme, key, name string
	w, h                   int
	expires                time.Time
}

// svgTokens — живые токены; нулевое значение готово к работе.
type svgTokens struct {
	mu sync.Mutex
	m  map[string]pendingSVG
}

// nowFn — часы токенов; в тестах подменяются.
var nowFn = time.Now

// issue выдаёт токен (16 hex из crypto/rand); при переполнении вытесняет самый старый.
func (t *svgTokens) issue(p pendingSVG) (string, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	tok := hex.EncodeToString(b[:])
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.m == nil {
		t.m = map[string]pendingSVG{}
	}
	now := nowFn()
	for k, v := range t.m {
		if now.After(v.expires) {
			delete(t.m, k)
		}
	}
	for len(t.m) >= maxSVGTokens {
		oldest, first := "", true
		for k, v := range t.m {
			if first || v.expires.Before(t.m[oldest].expires) {
				oldest, first = k, false
			}
		}
		delete(t.m, oldest)
	}
	p.expires = now.Add(svgTokenTTL)
	t.m[tok] = p
	return tok, nil
}

// take забирает токен: второй раз он уже не работает, просроченный — тоже.
func (t *svgTokens) take(tok string) (pendingSVG, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	p, ok := t.m[tok]
	delete(t.m, tok)
	if !ok || nowFn().After(p.expires) {
		return pendingSVG{}, false
	}
	return p, true
}

// ErrToken — токена нет, он уже использован или просрочен.
var ErrToken = errors.New("задание на SVG устарело — загрузите файл ещё раз")

// AddPNG принимает PNG, нарисованный окном из проверенного SVG (base64), и кладёт его в
// библиотеку под ключом SVG. Токен одноразовый и расходуется при любом исходе. Принимается
// только PNG заказанного размера в пределах imgfile и не полностью прозрачный.
func (s *Store) AddPNG(token, data string) (Item, error) {
	s.ioMu.Lock()
	defer s.ioMu.Unlock()
	if s.ReadOnly() {
		return Item{}, ErrReadOnly
	}
	p, ok := s.svg.take(token)
	if !ok {
		return Item{}, ErrToken
	}
	if len(data) > MaxPNGBase64 {
		return Item{}, errors.New("картинка из SVG слишком велика")
	}
	raw, err := base64.StdEncoding.DecodeString(data)
	if err != nil || !bytes.HasPrefix(raw, []byte("\x89PNG\r\n\x1a\n")) {
		return Item{}, errors.New("из SVG пришла не PNG-картинка")
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(raw))
	if err != nil || cfg.Width != p.w || cfg.Height != p.h {
		return Item{}, errors.New("размер картинки из SVG не тот, что заказан")
	}
	dir := filepath.Join(s.root, kindDirs[p.kind])
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Item{}, paths.NoPath(err)
	}
	tmp, err := writeTemp(dir, raw)
	if err != nil {
		return Item{}, err
	}
	defer func() { _ = os.Remove(tmp) }() // после rename его уже нет
	// Копия проверяется как любая другая, и PNG один раз декодируется целиком: битое тело
	// иначе всплыло бы позже, на миниатюре и в экспорте.
	info, ext, err := checkCopy(tmp)
	if err != nil || ext != "png" {
		return Item{}, errors.New("из SVG пришла не PNG-картинка")
	}
	img, release, err := imgfile.OpenOriented(tmp)
	if err != nil {
		return Item{}, errors.New("картинка из SVG не читается")
	}
	blank := blankImage(img)
	release()
	if blank {
		return Item{}, errSVGBlank
	}
	it, changed, err := s.commit(p.kind, p.theme, p.name, p.key, tmp, ext, info)
	if changed {
		s.saveLogged()
	}
	return it, err
}

// errSVGBlank — окно прислало полностью прозрачный PNG: рисунок пустой или окно не дорисовало
// его (аудит 8391ede: 3000 полупрозрачных прямоугольников во весь холст дают пустой PNG).
var errSVGBlank = errors.New("SVG не удалось нарисовать — рисунок пустой или слишком сложный")

// blankImage — у всех пикселей нулевая прозрачность. Холст окна приходит NRGBA — по байтам;
// остальное — через At, на первом непрозрачном пикселе выход.
func blankImage(img image.Image) bool {
	b := img.Bounds()
	if m, ok := img.(*image.NRGBA); ok {
		for y := b.Min.Y; y < b.Max.Y; y++ {
			row := m.Pix[m.PixOffset(b.Min.X, y):][:b.Dx()*4]
			for i := 3; i < len(row); i += 4 {
				if row[i] != 0 {
					return false
				}
			}
		}
		return true
	}
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if _, _, _, a := img.At(x, y).RGBA(); a != 0 {
				return false
			}
		}
	}
	return true
}

// writeTemp пишет байты во временный файл папки dir.
func writeTemp(dir string, raw []byte) (string, error) {
	out, err := os.CreateTemp(dir, tmpPrefix+"*.tmp")
	if err != nil {
		return "", paths.NoPath(err)
	}
	tmp := out.Name()
	_, err = io.Copy(out, bytes.NewReader(raw))
	if err == nil {
		err = out.Sync()
	}
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		_ = os.Remove(tmp)
		return "", paths.NoPath(err)
	}
	return tmp, nil
}
