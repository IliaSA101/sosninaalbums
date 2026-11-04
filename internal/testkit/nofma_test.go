package testkit

import (
	"runtime"
	"runtime/debug"
	"strings"
	"testing"
)

func settings(kv ...string) []debug.BuildSetting {
	var out []debug.BuildSetting
	for i := 0; i+1 < len(kv); i += 2 {
		out = append(out, debug.BuildSetting{Key: kv[i], Value: kv[i+1]})
	}
	return out
}

// Решение стража: таблица по целям, ключу и итогу канарейки. Целевую архитектуру берёт runtime.GOARCH, поэтому
// строки для «другой» архитектуры сверяются через fmaPossible, а не через noFMACheck.
func TestFmaPossible(t *testing.T) {
	for _, c := range []struct {
		arch, v string
		want    bool
	}{
		{"amd64", "v1", false}, {"amd64", "v2", false}, {"amd64", "v3", true}, {"amd64", "v4", true},
		{"amd64", "", false}, {"arm64", "", true}, {"arm64", "v1", true}, {"riscv64", "", true},
	} {
		if got := fmaPossible(c.arch, c.v); got != c.want {
			t.Errorf("%s/%q: %v, ждали %v", c.arch, c.v, got, c.want)
		}
	}
}

func TestHasField(t *testing.T) {
	for _, c := range []struct {
		flags string
		want  bool
	}{
		{NoFMAFlag, true}, {NoFMAFlag + " -N -l", true}, {"-N " + NoFMAFlag, true},
		{"", false}, {"all=-N -l", false},
		{"-d=fmahash=qn", false},             // без all= math и зависимости остаются слитыми
		{"all=-d=fmahash=n", false},          // громкий режим — не наш ключ
		{"all=-d=fmahash=qnx", false},        // продолжение слова — другой ключ
		{"pkg=-d=fmahash=qn", false},         // не all
		{"xall=-d=fmahash=qn", false},        // префикс слова
		{NoFMAFlag + "\t" + NoFMAFlag, true}, // пробелы любые
		{"  " + NoFMAFlag + "  ", true},
	} {
		if got := hasField(c.flags, NoFMAFlag); got != c.want {
			t.Errorf("%q: %v, ждали %v", c.flags, got, c.want)
		}
	}
}

// Канарейка слитая — провал независимо от настроек; текст ведёт к решению 162.
func TestNoFMACheckFused(t *testing.T) {
	err := noFMACheck(settings("-gcflags", NoFMAFlag, "GOAMD64", "v1"), true, runtime.GOARCH)
	if err == nil || !strings.Contains(err.Error(), "162") || !strings.Contains(err.Error(), "fmahash=qn") {
		t.Fatalf("слитая канарейка: %v", err)
	}
}

// Сборочная информация: там, где слияние возможно, нужен ключ в -gcflags; где невозможно — не требуется.
// Архитектура — параметр: строки arm64 проверяются и на amd64-раннере.
func TestNoFMACheckSettings(t *testing.T) {
	cases := []struct {
		name string
		arch string
		s    []debug.BuildSetting
		fail bool
	}{
		{"amd64 v1 без ключа", "amd64", settings("GOAMD64", "v1"), false},
		{"amd64 без сведений", "amd64", nil, false},
		{"amd64 v3 без ключа", "amd64", settings("GOAMD64", "v3"), true},
		{"amd64 v3 с ключом", "amd64", settings("-gcflags", NoFMAFlag, "GOAMD64", "v3"), false},
		{"amd64 v3 перекрыт -N -l", "amd64", settings("-gcflags", "all=-N -l", "GOAMD64", "v3"), true},
		{"amd64 v3 ключ без all=", "amd64", settings("-gcflags", "-d=fmahash=qn", "GOAMD64", "v3"), true},
		{"amd64 v3 ключ и -N -l", "amd64", settings("-gcflags", NoFMAFlag+" -N -l", "GOAMD64", "v3"), false},
		{"arm64 без ключа", "arm64", nil, true},
		{"arm64 без ключа, GOARM64", "arm64", settings("GOARM64", "v8.0"), true},
		{"arm64 с ключом", "arm64", settings("-gcflags", NoFMAFlag), false},
		{"arm64 перекрыт -N -l", "arm64", settings("-gcflags", "all=-N -l"), true},
		{"arm64 ключ без all=", "arm64", settings("-gcflags", "-d=fmahash=qn"), true},
		{"arm64 ключ и -N -l", "arm64", settings("-gcflags", NoFMAFlag+" -N -l"), false},
	}
	for _, c := range cases {
		err := noFMACheck(c.s, false, c.arch)
		if (err != nil) != c.fail {
			t.Errorf("%s: %v, провал должен быть %v", c.name, err, c.fail)
		} else if err != nil && !strings.Contains(err.Error(), "162") {
			t.Errorf("%s: текст без 162: %v", c.name, err)
		}
	}
}

// Настоящая сборка: канарейка и сборочная информация согласованы с тем, что вернул страж. В сборке с ключом
// слияния нет совсем; без ключа там, где оно возможно, страж обязан отказать (иначе эталоны считались бы слитыми).
func TestRequireNoFMA(t *testing.T) {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		t.Skip("нет сборочной информации")
	}
	var gcflags, amd64 string
	for _, s := range bi.Settings {
		switch s.Key {
		case "-gcflags":
			gcflags = s.Value
		case "GOAMD64":
			amd64 = s.Value
		}
	}
	err := noFMAError()
	switch {
	case hasField(gcflags, NoFMAFlag):
		if err != nil {
			t.Errorf("сборка с ключом, а страж отказал: %v", err)
		}
		if fmaFused() {
			t.Error("сборка с ключом, а канарейка слита")
		}
	case fmaPossible(runtime.GOARCH, amd64):
		if err == nil {
			t.Error("слияние возможно и ключа нет, а страж пропустил")
		}
	default:
		if err != nil {
			t.Errorf("слияния нет и быть не может, а страж отказал: %v", err)
		}
	}
	if err == nil {
		RequireNoFMA(t) // зелёная сборка: страж не валит тест
	}
}
