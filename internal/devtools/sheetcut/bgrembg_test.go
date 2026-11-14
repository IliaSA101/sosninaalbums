package sheetcut

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/IliaSA101/sosninaalbums/internal/devtools/devtest"
)

// Фальшивый rembg — приём TestHelperProcess: команда — сам тестовый бинарь с -test.run.
// Режимы (FAKE_REMBG): "ok" — чистый белый → альфа 0, левый столбец квадрата (x = 100) — альфа 128
// с цветом, смешанным с белым; "fail" — код 3 и сообщение; "small" — выход меньшего размера.
var lastFake *exec.Cmd // последняя собранная фальшивая команда (проверка настроек запуска)

func fakeRembg(t *testing.T, mode string) *[][]string {
	t.Helper()
	// Временные файлы rembg — в папке теста: проверки не зависят от чужого содержимого TEMP.
	tmp := tempDir(t)
	t.Setenv("TMP", tmp)
	t.Setenv("TEMP", tmp)
	t.Setenv("TMPDIR", tmp)
	self := devtest.SelfExe(t)
	var calls [][]string
	old := rembgCommand
	t.Cleanup(func() { rembgCommand = old })
	rembgCommand = func(ctx context.Context, cmd string, args ...string) *exec.Cmd {
		calls = append(calls, append([]string{cmd}, args...))
		c := exec.CommandContext(ctx, self, append([]string{"-test.run=TestHelperProcess", "--"}, args...)...)
		c.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1", "FAKE_REMBG="+mode)
		lastFake = c
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
	if len(args) != 5 || args[0] != "i" || args[1] != "-m" {
		fmt.Fprintln(os.Stderr, "неверные аргументы:", args)
		os.Exit(2)
	}
	mode := os.Getenv("FAKE_REMBG")
	if mode == "fail" {
		fmt.Fprintln(os.Stderr, "модель не найдена")
		os.Exit(3)
	}
	f, err := os.Open(args[3])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	src, err := png.Decode(f)
	f.Close()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	img := toNRGBA(src)
	if mode == "small" {
		img = canvas(10, 10, white)
	}
	for y := 0; y < img.Bounds().Dy(); y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			c := img.NRGBAAt(x, y)
			switch {
			case c == white:
				img.SetNRGBA(x, y, color.NRGBA{255, 255, 255, 0})
			case x == 100 && mode == "ok":
				img.SetNRGBA(x, y, color.NRGBA{uint8((int(c.R) + 255) / 2), uint8((int(c.G) + 255) / 2), uint8((int(c.B) + 255) / 2), 128})
			}
		}
	}
	o, err := os.Create(args[4])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if err := png.Encode(o, img); err != nil {
		os.Exit(2)
	}
	o.Close()
	os.Exit(0)
}

// whiteSheet — белый лист с одним тёмным квадратом 100…200 × 50…150.
func whiteSheet() *image.NRGBA {
	img := canvas(300, 200, white)
	fillRect(img, image.Rect(100, 50, 200, 150), navy)
	return img
}

func TestDefringe(t *testing.T) {
	img := canvas(5, 1, transp)
	for i, a := range []uint8{0, 64, 128, 200, 255} {
		cov := float64(a) / 255
		mix := func(f, bg uint8) uint8 { return uint8(cov*float64(f) + (1-cov)*float64(bg) + 0.5) }
		c := color.NRGBA{mix(navy.R, 255), mix(navy.G, 255), mix(navy.B, 255), a}
		if a == 0 || a == 255 {
			c = color.NRGBA{200, 210, 220, a} // край диапазона трогать нельзя
		}
		img.SetNRGBA(i, 0, c)
	}
	defringe(img)
	if got := img.NRGBAAt(0, 0); got != (color.NRGBA{200, 210, 220, 0}) {
		t.Fatalf("альфа 0 изменена: %v", got)
	}
	if got := img.NRGBAAt(4, 0); got != (color.NRGBA{200, 210, 220, 255}) {
		t.Fatalf("альфа 255 изменена: %v", got)
	}
	for x := 1; x <= 3; x++ { // несколько пикселей сразу; чем меньше альфа, тем грубее восстановление
		got, tol := img.NRGBAAt(x, 0), 6
		if x == 1 {
			tol = 12
		}
		for c, w := range []uint8{navy.R, navy.G, navy.B} {
			if d := int(([]uint8{got.R, got.G, got.B})[c]) - int(w); d > tol || d < -tol {
				t.Fatalf("пиксель %d: цвет %v, ждали около %v", x, got, navy)
			}
		}
	}
}

func TestDefringeClamps(t *testing.T) {
	img := canvas(1, 1, color.NRGBA{10, 10, 10, 128}) // темнее, чем возможно при смеси с белым
	defringe(img)
	if got := img.NRGBAAt(0, 0); got.R != 0 || got.A != 128 {
		t.Fatalf("зажим 0..255: %v", got)
	}
}

func TestRembgCutsWhiteSheet(t *testing.T) {
	calls := fakeRembg(t, "ok")
	tmp := tempDir(t) // после fakeRembg: свой TEMP, чтобы проверить, что папка убрана
	t.Setenv("TMP", tmp)
	t.Setenv("TEMP", tmp)
	t.Setenv("TMPDIR", tmp)
	o := DefaultOptions()
	o.Rembg = "rembg-fake"
	res := mustCut(t, whiteSheet(), o)
	if res.BG != BGRembg || res.BGInfo != nil || len(res.Stickers) != 1 {
		t.Fatalf("режим %q, стикеров %d", res.BG, len(res.Stickers))
	}
	if len(*calls) != 1 || (*calls)[0][0] != "rembg-fake" || (*calls)[0][1] != "i" || (*calls)[0][3] != DefaultModel {
		t.Fatalf("вызовы: %v", *calls)
	}
	if left, _ := os.ReadDir(tmp); len(left) != 0 {
		t.Fatalf("временная папка не убрана: %v", left)
	}
	if len(res.Notes) != 0 {
		t.Fatalf("замечания при заданном rembg: %v", res.Notes)
	}
	if lastFake.WaitDelay != 10*time.Second {
		t.Fatalf("WaitDelay %v: через rembg.cmd таймаут не оборвал бы трубу", lastFake.WaitDelay)
	}
}

func TestRembgDefringeOnOff(t *testing.T) {
	fakeRembg(t, "ok")
	left := func(defr bool) color.NRGBA {
		o := DefaultOptions()
		o.Rembg, o.Defringe = "rembg-fake", defr
		res := mustCut(t, whiteSheet(), o)
		return res.Stickers[0].Img.NRGBAAt(0, 40)
	}
	on, off := left(true), left(false)
	if on.A != 128 || off.A != 128 {
		t.Fatalf("альфа каймы: %d / %d", on.A, off.A)
	}
	if off.R < 100 {
		t.Fatalf("без defringe кайма должна быть светлой: %v", off)
	}
	if d := int(on.R) - int(navy.R); d > 6 || d < -6 {
		t.Fatalf("defringe не вернул цвет: %v", on)
	}
}

func TestAutoModeChoosesRembg(t *testing.T) {
	calls := fakeRembg(t, "ok")
	// белый лист + команда → rembg
	o := DefaultOptions()
	o.Rembg = "rembg-fake"
	if res := mustCut(t, whiteSheet(), o); res.BG != BGRembg {
		t.Fatalf("белый лист с командой: %q", res.BG)
	}
	// лист с содержательной альфой + команда → none, rembg не вызывается
	n := len(*calls)
	if res := mustCut(t, gridSheet(2, 1, 200), o); res.BG != BGNone || len(*calls) != n {
		t.Fatalf("лист с альфой: %q, вызовов %d", res.BG, len(*calls)-n)
	}
	// белый лист без команды → own и предупреждение
	res := mustCut(t, whiteSheet(), DefaultOptions())
	if res.BG != BGOwn || len(*calls) != n || len(res.Notes) != 1 || !strings.Contains(res.Notes[0], "rembg не задан") {
		t.Fatalf("без команды: %q, замечания %v", res.BG, res.Notes)
	}
	if !strings.Contains(res.Text(), "rembg не задан") {
		t.Fatal("предупреждения нет в отчёте")
	}
}

func TestRembgRefusals(t *testing.T) {
	calls := fakeRembg(t, "ok")
	for _, m := range []string{"bria-rmbg", "BRIA-RMBG", "bria-rmbg-2.0", "-x", "  ", "a b", "m;calc", "m&calc", "m|x", "m%PATH%", "m\"x", "m^x", "модель", "m\n"} {
		o := DefaultOptions()
		o.Rembg, o.Model, o.BG = "rembg-fake", m, BGRembg
		if m == "  " {
			o.Model = "  " // пробелы — не «пусто», в умолчание не превращаются
		}
		_, err := Cut(whiteSheet(), o)
		if err == nil {
			t.Fatalf("модель %q должна быть отказом", m)
		}
		if strings.HasPrefix(strings.ToLower(m), "bria") && !strings.Contains(err.Error(), "BRIA") {
			t.Fatalf("нет причины отказа: %v", err)
		}
	}
	o := DefaultOptions()
	o.BG = BGRembg
	if _, err := Cut(whiteSheet(), o); err == nil || !strings.Contains(err.Error(), "SHEETCUT_REMBG") {
		t.Fatalf("rembg без команды: %v", err)
	}
	if len(*calls) != 0 {
		t.Fatalf("команда запускалась при отказе: %v", *calls)
	}
}

func TestRembgFailures(t *testing.T) {
	for _, mode := range []string{"fail", "small"} {
		fakeRembg(t, mode)
		o := DefaultOptions()
		o.Rembg = "rembg-fake"
		_, err := Cut(whiteSheet(), o)
		if err == nil {
			t.Fatalf("режим %s: ждали ошибку", mode)
		}
		if mode == "fail" && !strings.Contains(err.Error(), "модель не найдена") {
			t.Fatalf("вывод команды потерян: %v", err)
		}
	}
}

func TestCheckModelNames(t *testing.T) {
	for _, m := range []string{"birefnet-general", "u2net_human_seg", "isnet-general-use", "Model.v2", "a"} {
		if err := checkModel(m); err != nil {
			t.Errorf("модель %q должна проходить: %v", m, err)
		}
	}
}

func TestTailKeepsUTF8(t *testing.T) {
	cases := []struct {
		in string
		n  int
	}{
		{"короткий", 100},
		{"", 5},
		{strings.Repeat("ё", 50), 7}, // 2 байта на букву: нечётный срез разрезал бы букву
		{strings.Repeat("яблоко€", 9), 10},
		{strings.Repeat("a", 30), 0},
	}
	for _, c := range cases {
		got := tail(c.in, c.n)
		if !utf8.ValidString(got) {
			t.Errorf("tail(%q, %d) = %q: битый UTF-8", c.in, c.n, got)
		}
		if len(c.in) > c.n && len(got) > c.n+len("…") {
			t.Errorf("tail(%q, %d) длиннее предела: %d", c.in, c.n, len(got))
		}
	}
	if got := tail("  abc \n", 10); got != "abc" {
		t.Errorf("пробелы по краям: %q", got)
	}
}
