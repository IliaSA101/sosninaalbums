package svgcut

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/devtools/devtest"
)

// tempDir — папка теста вне GOTMPDIR (кэш go test), см. devtest.TempDir.
func tempDir(t testing.TB) string {
	t.Helper()
	return devtest.TempDir(t, "svgcut")
}

// fakeBrowser подменяет запуск браузера тестовым бинарём (TestHelperProcess); режимы FAKE_CHROME:
// "ok" — PNG нужного размера с красной точкой, "fail" — код 3, "small" — снимок меньшего размера,
// "nofile" — успех без файла, "garbage" — не PNG.
func fakeBrowser(t *testing.T, mode string) *[][]string {
	t.Helper()
	self := devtest.SelfExe(t)
	var calls [][]string
	old := chromeCommand
	t.Cleanup(func() { chromeCommand = old })
	chromeCommand = func(ctx context.Context, cmd string, args ...string) *exec.Cmd {
		calls = append(calls, append([]string{cmd}, args...))
		c := exec.CommandContext(ctx, self, append([]string{"-test.run=TestHelperProcess", "--"}, args...)...)
		c.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1", "FAKE_CHROME="+mode)
		return c
	}
	return &calls
}

func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	args := os.Args
	for i, a := range args {
		if a == "--" {
			args = args[i+1:]
			break
		}
	}
	var out string
	w, h := 0, 0
	for _, a := range args {
		switch {
		case strings.HasPrefix(a, "--screenshot="):
			out = strings.TrimPrefix(a, "--screenshot=")
		case strings.HasPrefix(a, "--window-size="):
			_, _ = fmt.Sscanf(strings.TrimPrefix(a, "--window-size="), "%d,%d", &w, &h)
		}
	}
	switch os.Getenv("FAKE_CHROME") {
	case "fail":
		fmt.Fprintln(os.Stderr, "нет дисплея")
		os.Exit(3)
	case "nofile":
		os.Exit(0)
	case "garbage":
		_ = os.WriteFile(out, []byte("не png"), 0o644)
		os.Exit(0)
	case "small":
		w, h = 2, 2
	}
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	img.SetNRGBA(0, 0, color.NRGBA{255, 0, 0, 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		os.Exit(2)
	}
	if err := os.WriteFile(out, buf.Bytes(), 0o644); err != nil {
		os.Exit(2)
	}
	os.Exit(0)
}

func TestSvgRenderNeedsCommand(t *testing.T) {
	tmp := tempDir(t)
	if _, err := (Chrome{Tmp: tmp}).Render([]byte("<svg/>"), 10, 10); err == nil || !strings.Contains(err.Error(), "PACKPREP_CHROME") {
		t.Fatalf("без команды: %v", err)
	}
	if _, err := (Chrome{Cmd: "  ", Tmp: tmp}).Render([]byte("<svg/>"), 10, 10); err == nil {
		t.Fatal("пробельная команда принята")
	}
	if _, err := (Chrome{Cmd: "x"}).Render([]byte("<svg/>"), 10, 10); err == nil || !strings.Contains(err.Error(), "временных") {
		t.Fatalf("без папки временных файлов: %v", err)
	}
	// Несуществующая команда — ошибка запуска, а не паника.
	if _, err := (Chrome{Cmd: filepath.Join(tmp, "нет-такого"), Tmp: tmp}).Render([]byte("<svg/>"), 10, 10); err == nil {
		t.Fatal("несуществующая команда принята")
	}
	// Размеры вне пределов отклоняются до запуска.
	calls := fakeBrowser(t, "ok")
	for _, sz := range [][2]int{{0, 10}, {10, -1}, {MaxRenderSide + 1, 10}, {8000, 8000}} {
		if _, err := (Chrome{Cmd: "x", Tmp: tmp}).Render([]byte("<svg/>"), sz[0], sz[1]); err == nil {
			t.Fatalf("размер %v принят", sz)
		}
	}
	if len(*calls) != 0 {
		t.Fatalf("браузер запускался для негодного размера: %v", *calls)
	}
}

func TestSvgRenderFakeCommand(t *testing.T) {
	tmp := filepath.Join(tempDir(t), "tmp") // папки ещё нет: Render создаёт сам
	calls := fakeBrowser(t, "ok")
	img, err := (Chrome{Cmd: "chrome-x", Tmp: tmp}).Render([]byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`), 40, 30)
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Dx() != 40 || img.Bounds().Dy() != 30 || img.NRGBAAt(0, 0) != (color.NRGBA{255, 0, 0, 255}) || img.NRGBAAt(5, 5).A != 0 {
		t.Fatalf("снимок: %v %v", img.Bounds(), img.NRGBAAt(0, 0))
	}
	if len(*calls) != 1 || (*calls)[0][0] != "chrome-x" {
		t.Fatalf("вызовы: %v", *calls)
	}
	args := strings.Join((*calls)[0][1:], "\n")
	for _, want := range []string{"--headless", "--window-size=40,30", "--default-background-color=00000000",
		"--force-device-scale-factor=1", "--screenshot=", "--user-data-dir=", "file://",
		"--disable-background-networking", "--no-default-browser-check", "--disable-component-update"} {
		if !strings.Contains(args, want) {
			t.Errorf("в аргументах нет %q:\n%s", want, args)
		}
	}
	// Файл SVG лежит внутри Tmp и удалён после запуска; Tmp остаётся пустым.
	if ents, err := os.ReadDir(tmp); err != nil || len(ents) != 0 {
		t.Fatalf("временные файлы остались в %s: %v %v", tmp, ents, err)
	}
	// Сбои: код выхода (с хвостом вывода), снимок не того размера, нет файла, не PNG.
	for mode, want := range map[string]string{"fail": "нет дисплея", "small": "снимок 2×2", "nofile": "не создал снимок", "garbage": "снимок браузера"} {
		fakeBrowser(t, mode)
		if _, err := (Chrome{Cmd: "x", Tmp: tmp}).Render([]byte("<svg/>"), 40, 30); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("режим %s: %v, ждали %q", mode, err, want)
		}
	}
}

func TestFileURL(t *testing.T) {
	if got := fileURL("/tmp/a b/in.svg"); got != "file:///tmp/a%20b/in.svg" {
		t.Fatalf("fileURL: %s", got)
	}
	if got := tail("абвгд", 4); !strings.HasPrefix(got, "…") || !strings.HasSuffix(got, "д") {
		t.Fatalf("tail режет посреди символа: %q", got)
	}
}

// chromeOrSkip — браузер из PACKPREP_CHROME; без переменной тест пропускается, а заданная, но не
// запускающаяся — провал: тихий Skip скрыл бы сломанный CI-браузер.
func chromeOrSkip(t *testing.T) Chrome {
	t.Helper()
	cmd := os.Getenv("PACKPREP_CHROME")
	skip, err := chromeEnv(cmd)
	if skip {
		t.Skip("нужен Chromium: задайте PACKPREP_CHROME (chrome-headless-shell, Chrome или Edge)")
	}
	if err != nil {
		t.Fatal(err)
	}
	return Chrome{Cmd: cmd, Tmp: filepath.Join(tempDir(t), "tmp")}
}

// chromeEnv решает судьбу теста по PACKPREP_CHROME: пусто — пропуск, задано и не запускается — ошибка.
func chromeEnv(cmd string) (skip bool, err error) {
	if cmd == "" {
		return true, nil
	}
	if _, err := exec.LookPath(cmd); err != nil {
		return false, fmt.Errorf("PACKPREP_CHROME=%q не запускается: %w", cmd, err)
	}
	return false, nil
}

func TestChromeEnv(t *testing.T) {
	if skip, err := chromeEnv(""); !skip || err != nil {
		t.Fatalf("пустая переменная: skip=%v err=%v", skip, err)
	}
	if skip, err := chromeEnv(filepath.Join(tempDir(t), "нет-браузера")); skip || err == nil {
		t.Fatalf("заданный, но не запускающийся браузер: skip=%v err=%v — должен быть провал, а не пропуск", skip, err)
	}
	if skip, err := chromeEnv(devtest.SelfExe(t)); skip || err != nil { // запускаемый файл найден
		t.Fatalf("существующая команда: skip=%v err=%v", skip, err)
	}
}

// Те же утверждения, что у фальшивого растра, но попадание точек и маски считает настоящий браузер.
func TestSvgCutChromeKnockout(t *testing.T) {
	c := chromeOrSkip(t)
	o := DefaultOptions()
	o.Render = c.Render
	knockoutCase(t, o)
	// Промах определяет тот же браузер.
	o.Knock = [][2]float64{{30, 30}}
	if _, err := Cut(readTestdata(t, "sheet.svg"), o); err == nil || !strings.Contains(err.Error(), "нет пути цвета листа") {
		t.Fatalf("промах в Chromium: %v", err)
	}
	// Служебный растр: цвет-номер точно доходит до пикселя (без сглаживания краёв).
	doc, err := Parse(readTestdata(t, "sheet.svg"))
	if err != nil {
		t.Fatal(err)
	}
	sheet := doc.Sheet()
	isWhite, _, _ := whiteIndex(doc, sheet)
	sx, sy, pw, ph := hitScale(doc)
	img, err := c.Render(hitSVG(doc, sheet, isWhite, pw, ph), pw, ph)
	if err != nil {
		t.Fatal(err)
	}
	if got := img.NRGBAAt(int(70*sx), int(70*sy)); got != (color.NRGBA{0, 0, 3, 255}) {
		t.Fatalf("цвет-номер белого пути: %v", got)
	}
	if got := img.NRGBAAt(int(30*sx), int(30*sy)); got != (color.NRGBA{0, 0, 0, 255}) {
		t.Fatalf("не белый путь должен быть чёрным: %v", got)
	}
	if got := img.NRGBAAt(int(150*sx), int(150*sy)); got.A != 0 {
		t.Fatalf("пустой лист должен быть прозрачным: %v", got)
	}
}
