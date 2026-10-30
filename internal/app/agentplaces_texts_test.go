package app

// Правила мест в продукте (155 Т1): Default стоит за askM().rules, тексты отказов и коды причин не
// изменились при переносе в agentplaces — их читают человек, агент и фронт.

import (
	"path/filepath"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/app/agentplaces"
)

// TestDefaultRulesInProduct: у NewApp и у нулевого App правила — настоящие Default: домашняя папка и корень
// тома отвергаются, а не принимаются мягкими или пустыми правилами.
func TestDefaultRulesInProduct(t *testing.T) {
	home := realDir(t, "Домашняя")
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	root := filepath.VolumeName(home) + string(filepath.Separator)
	for name, a := range map[string]*App{"NewApp": NewApp(), "App{}": {}} {
		r := a.askM().rules()
		if r == nil {
			t.Errorf("%s: правил нет", name)
			continue
		}
		if r.Timeout() == 0 {
			t.Errorf("%s: правила собраны не Default (предел пробы 0)", name)
		}
		for what, dir := range map[string]string{"домашняя": home, "корень тома": root} {
			if err := r.Folder(dir); err == nil {
				t.Errorf("%s: %s принята", name, what)
			}
			if _, err := r.JudgeWrite(dir); err == nil {
				t.Errorf("%s: %s принята для записи", name, what)
			}
		}
	}
}

// TestPlaceTextsStable: тексты Err* и коды причин — побайтно прежние (чистый перенос, 155-1).
func TestPlaceTextsStable(t *testing.T) {
	for _, c := range []struct {
		name, got, want string
	}{
		{"ErrTooWide", agentplaces.ErrTooWide.Error(), "папка не подходит: слишком широкая, системная или недоступная"},
		{"ErrOurs", agentplaces.ErrOurs.Error(), "папка не подходит: это папка данных или кэша программы"},
		{"ErrPlaceBad", agentplaces.ErrPlaceBad.Error(), "эту папку нельзя сделать местом съёмок: слишком широкая, системная или недоступная"},
		{"ErrVolUnknown", agentplaces.ErrVolUnknown.Error(), "не удалось узнать диск"},
		{"WhyVol", agentplaces.WhyVol, "vol"},
		{"WhyDisk", agentplaces.WhyDisk, "disk"},
		{"whyVol", whyVol, "vol"},
		{"whyDisk", whyDisk, "disk"},
		{"StateNoDisk", agentplaces.StateNoDisk, "nodisk"},
		{"StateNoFolder", agentplaces.StateNoFolder, "nofolder"},
		{"StateHere", agentplaces.StateHere, "here"},
	} {
		if c.got != c.want {
			t.Errorf("%s = %q, ждали %q", c.name, c.got, c.want)
		}
	}
}
