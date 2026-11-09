package sheetcut

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"
)

// DefaultModel — модель rembg по умолчанию: на листах стикеров с белого фона ближе всех к эталону
// (IoU маски 0,96), чистые края, пузырьки, кольца; massive даёт серые пятна, hrsod — облако и потерю предмета.
const DefaultModel = "birefnet-general"

// rembgTimeout — предел одного запуска (CPU, лист 4K ≈ 20 с, первый запуск ещё качает модель).
const rembgTimeout = 15 * time.Minute

// rembgCommand собирает команду rembg; подменяется в тестах (приём TestHelperProcess).
var rembgCommand = func(ctx context.Context, cmd string, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, cmd, args...) //nolint:gosec // команду задаёт разработчик (-rembg), запуск без оболочки
}

// checkModel отклоняет модели, которые нельзя использовать: веса BRIA (bria-rmbg) — лицензия
// несовместима с CC0 наборов библиотеки; имя с «-» в начале — подмена флага команды; любые знаки,
// кроме [a-z0-9._-], — rembg.cmd запускается через cmd.exe, и аргумент стал бы командой оболочки.
func checkModel(model string) error {
	m := strings.ToLower(model)
	switch {
	case strings.TrimSpace(m) == "":
		return errors.New("sheetcut: пустая модель rembg")
	case strings.HasPrefix(m, "bria"):
		return fmt.Errorf("sheetcut: модель %q не годится: лицензия весов BRIA несовместима с CC0 наборов библиотеки", model)
	case strings.HasPrefix(m, "-"):
		return fmt.Errorf("sheetcut: модель %q: имя не может начинаться с «-»", model)
	}
	for _, r := range m {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '.' && r != '_' && r != '-' {
			return fmt.Errorf("sheetcut: модель %q: допустимы только латиница, цифры и . _ -", model)
		}
	}
	return nil
}

// runRembg снимает фон внешней командой rembg: `<cmd> i -m <model> вход выход.png`; выход — лист
// с альфой того же размера. Промежуточные файлы — во временной папке ОС, удаляются после.
func runRembg(src *image.NRGBA, cmd, model string) (*image.NRGBA, error) {
	if cmd == "" {
		return nil, errors.New("sheetcut: rembg не задан (флаг -rembg или переменная SHEETCUT_REMBG)")
	}
	if err := checkModel(model); err != nil {
		return nil, err
	}
	dir, err := os.MkdirTemp("", "sheetcut-")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(dir) }()
	in, out := filepath.Join(dir, "in.png"), filepath.Join(dir, "out.png")
	var buf bytes.Buffer
	if err := png.Encode(&buf, src); err != nil {
		return nil, err
	}
	if err := os.WriteFile(in, buf.Bytes(), 0o600); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), rembgTimeout)
	defer cancel()
	c := rembgCommand(ctx, cmd, "i", "-m", model, in, out)
	// Через rembg.cmd таймаут убивает только cmd.exe, python держит трубу вывода: без WaitDelay зависли бы.
	c.WaitDelay = 10 * time.Second
	if msg, err := c.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("sheetcut: rembg: %w\n%s", err, tail(string(msg), 600))
	}
	data, err := os.ReadFile(out)
	if err != nil {
		return nil, fmt.Errorf("sheetcut: rembg не создал выход: %w", err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("sheetcut: выход rembg: %w", err)
	}
	if img.Bounds().Dx() != src.Bounds().Dx() || img.Bounds().Dy() != src.Bounds().Dy() {
		return nil, fmt.Errorf("sheetcut: выход rembg %d×%d, а лист %d×%d", img.Bounds().Dx(), img.Bounds().Dy(), src.Bounds().Dx(), src.Bounds().Dy())
	}
	return toNRGBA(img), nil
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
