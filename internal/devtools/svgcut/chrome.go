package svgcut

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// RenderFunc растрирует SVG в изображение w×h px с прозрачным фоном. Подменяется в тестах.
type RenderFunc func(svg []byte, w, h int) (*image.NRGBA, error)

// Пределы растра: сторона и площадь (RGBA 4 байта на пиксель).
const (
	MaxRenderSide = 8192
	maxRenderPx   = 40_000_000
	chromeTimeout = 2 * time.Minute
)

// chromeCommand собирает команду браузера; подменяется в тестах (приём TestHelperProcess).
var chromeCommand = func(ctx context.Context, cmd string, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, cmd, args...) //nolint:gosec // команду задаёт разработчик (PACKPREP_CHROME), запуск без оболочки
}

// Chrome — растр через chrome-headless-shell, Chrome или Edge: по SVG-файлу делается снимок страницы
// с прозрачным фоном. Замер: chrome-headless-shell 149 и Chrome 156 дают один PNG байт в байт.
type Chrome struct {
	Cmd string // путь к браузеру (переменная PACKPREP_CHROME)
	Tmp string // папка временных файлов (-out/.tmp); пусто — ошибка, а не текущая папка
}

// Render записывает svg во временную папку и снимает страницу w×h.
func (c Chrome) Render(svg []byte, w, h int) (*image.NRGBA, error) {
	if strings.TrimSpace(c.Cmd) == "" {
		return nil, errors.New("svgcut: браузер не задан (флаг -chrome или переменная PACKPREP_CHROME)")
	}
	if c.Tmp == "" {
		return nil, errors.New("svgcut: не задана папка временных файлов")
	}
	if w < 1 || h < 1 || w > MaxRenderSide || h > MaxRenderSide || w*h > maxRenderPx {
		return nil, fmt.Errorf("svgcut: растр %d×%d вне пределов (сторона ≤ %d, площадь ≤ %d Мп)", w, h, MaxRenderSide, maxRenderPx/1_000_000)
	}
	if err := os.MkdirAll(c.Tmp, 0o755); err != nil {
		return nil, err
	}
	dir, err := os.MkdirTemp(c.Tmp, "r-")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(dir) }()
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	in, out := filepath.Join(abs, "in.svg"), filepath.Join(abs, "out.png")
	if err := os.WriteFile(in, svg, 0o600); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), chromeTimeout)
	defer cancel()
	cmd := chromeCommand(ctx, c.Cmd, chromeArgs(abs, in, out, w, h)...)
	// Через обёртку (.cmd) таймаут убивает только её: без WaitDelay труба вывода держала бы вызов.
	cmd.WaitDelay = 10 * time.Second
	if msg, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("svgcut: браузер: %w\n%s", err, tail(string(msg), 600))
	}
	return readPNG(out, w, h)
}

// chromeArgs — аргументы снимка. Свой профиль в временной папке: без него второй запуск упирается
// в замок профиля, а чужие расширения и сеть браузеру не нужны.
func chromeArgs(dir, in, out string, w, h int) []string {
	return []string{
		"--headless", "--disable-gpu", "--hide-scrollbars", "--no-first-run", "--disable-extensions",
		"--disable-background-networking", "--no-default-browser-check", "--disable-component-update",
		"--force-device-scale-factor=1",
		"--user-data-dir=" + filepath.Join(dir, "profile"),
		"--default-background-color=00000000",
		"--window-size=" + strconv.Itoa(w) + "," + strconv.Itoa(h),
		"--screenshot=" + out,
		fileURL(in),
	}
}

// fileURL — file:// для абсолютного пути (на Windows путь «C:\…» получает ведущий «/»).
func fileURL(p string) string {
	s := filepath.ToSlash(p)
	if !strings.HasPrefix(s, "/") {
		s = "/" + s
	}
	return (&url.URL{Scheme: "file", Path: s}).String()
}

// readPNG читает снимок, проверив размер до декодирования, и приводит к NRGBA.
func readPNG(path string, w, h int) (*image.NRGBA, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("svgcut: браузер не создал снимок: %w", err)
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("svgcut: снимок браузера: %w", err)
	}
	if cfg.Width != w || cfg.Height != h {
		return nil, fmt.Errorf("svgcut: снимок %d×%d, а нужен %d×%d", cfg.Width, cfg.Height, w, h)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("svgcut: снимок браузера: %w", err)
	}
	if n, ok := img.(*image.NRGBA); ok {
		return n, nil
	}
	n := image.NewNRGBA(img.Bounds())
	draw.Draw(n, n.Bounds(), img, img.Bounds().Min, draw.Src)
	return n, nil
}

// tail — последние n байт вывода команды (без пробелов по краям), начало — на границе символа UTF-8.
func tail(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) > n {
		i := len(s) - n
		for i < len(s) && !utf8.RuneStart(s[i]) {
			i++
		}
		s = "…" + s[i:]
	}
	return s
}
