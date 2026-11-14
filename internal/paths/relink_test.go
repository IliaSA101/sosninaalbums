package paths

import (
	"errors"
	"io/fs"
	"strings"
	"testing"
)

// Помощники перепривязки не уводят на сетевую шару или устройство: нелокальный
// результат — ok = false (аудит 021).
func TestRelinkRejectsNonLocalTarget(t *testing.T) {
	skipNotWindows(t)
	for _, to := range []string{`\\h\s`, "", "E:\\", `\\?\C:\x`, "EE", "1", `\??\UNC\h\s`} {
		if got, ok := SwapDrive(`E:\a\1.jpg`, "E", to); ok {
			t.Errorf("SwapDrive(to=%q) = %q, ждали отказ", to, got)
		}
	}
	for _, to := range []string{`\\h\s`, `\\?\C:\x`, `\??\C:\x`} {
		if got, ok := ReplacePrefix(`E:\a\b`, `E:\a`, to); ok {
			t.Errorf("ReplacePrefix(to=%q) = %q, ждали отказ", to, got)
		}
	}
	if from, to, ok := PrefixMap(`E:\a\b\1.jpg`, `\\h\s\b\1.jpg`); ok {
		t.Errorf("PrefixMap на UNC: %q → %q", from, to)
	}
}

// Инвариант: при ok = true результат проходит CheckLocalPath, если входы проходили.
func TestRelinkResultIsLocal(t *testing.T) {
	skipNotWindows(t)
	locals := []string{`C:\`, `E:\a`, `e:/a/b`, `F:\Съёмки\Ивановы`, `D:\x\y\z.jpg`}
	targets := append([]string{"", `\\h\s`, `E:`, "F", "g:", `\\?\C:\x`, `C:x`, `E:\a:s`}, locals...)
	for _, p := range locals {
		for _, f := range locals {
			for _, to := range targets {
				if got, ok := ReplacePrefix(p, f, to); ok && CheckLocalPath(got) != nil {
					t.Errorf("ReplacePrefix(%q, %q, %q) = %q: ok для нелокального", p, f, to, got)
				}
				for _, fr := range []string{"C", "E", "e", "F", "zz"} {
					if got, ok := SwapDrive(p, fr, to); ok && CheckLocalPath(got) != nil {
						t.Errorf("SwapDrive(%q, %q, %q) = %q: ok для нелокального", p, fr, to, got)
					}
				}
			}
			if from, to, ok := PrefixMap(p, f); ok && (CheckLocalPath(to) != nil || CheckLocalPath(from) != nil) {
				t.Errorf("PrefixMap(%q, %q) = %q, %q: ok для нелокального", p, f, from, to)
			}
		}
	}
}

func TestNoPath(t *testing.T) {
	if NoPath(nil) != nil {
		t.Error("nil должен остаться nil")
	}
	pe := &fs.PathError{Op: "open", Path: `C:\Users\Имя\a.json`, Err: errors.New("denied")}
	for _, err := range []error{pe, &wrapErr{pe}} {
		got := NoPath(err).Error()
		if strings.Contains(got, "Имя") || !strings.Contains(got, "open") || !strings.Contains(got, "denied") {
			t.Errorf("NoPath = %q", got)
		}
	}
	plain := errors.New("просто")
	if NoPath(plain) != plain {
		t.Error("обычная ошибка возвращается как есть")
	}
}

// wrapErr — обёртка с Unwrap: errors.As должен дойти до PathError.
type wrapErr struct{ error }

func (w *wrapErr) Unwrap() error { return w.error }
