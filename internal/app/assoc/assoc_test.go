package assoc

import (
	"slices"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/osx"
)

// Чистые тесты плана связи. Сквозные (TestAssocSkip/Ask/Sync, TestSetAssoc, TestOpenNewerState*,
// TestOpenInNewer) остаются в internal/app: они стоят на опоре App (0.26-126-15).
const (
	myExe       = "/prog/me/SosninaAlbums-0.25.0.exe"
	oldExe      = "/prog/old/SosninaAlbums-0.24.0.exe"
	instDir     = "/prog/inst"
	instExe     = "/prog/inst/SosninaAlbums.exe"
	foreignProg = "Other.Album"
	appsOld     = `Applications\SosninaAlbums-0.24.0.exe`
	appsEtc     = `Applications\notepad.exe`
	progOurs    = osx.AssocProgID
)

func TestAssocPlan(t *testing.T) {
	live := func(ps ...string) func(string) bool {
		return func(p string) bool { return slices.Contains(ps, p) }
	}
	ours := func(exe, ver string) osx.AssocState {
		return osx.AssocState{ProgID: progOurs, Exe: exe, AppVersion: ver}
	}
	cases := []struct {
		name       string
		st         osx.AssocState
		exe, ver   string
		live       []string
		write      bool
		foreignAct bool
		drop       bool
	}{
		{"пусто", osx.AssocState{}, myExe, "0.25.0", nil, true, false, false},
		{"ProgID наш, команды нет", osx.AssocState{ProgID: progOurs}, myExe, "0.25.0", nil, true, false, false},
		{"exe из команды пропал", ours(oldExe, "0.26.0"), myExe, "0.25.0", nil, true, false, false},
		{"моя версия новее", ours(oldExe, "0.24.0"), myExe, "0.25.0", []string{oldExe}, true, false, false},
		{"моя версия старее", ours(oldExe, "0.26.0"), myExe, "0.25.0", []string{oldExe}, false, false, false},
		{"равны, обе zip", ours(oldExe, "0.25.0"), myExe, "0.25.0", []string{oldExe}, false, false, false},
		{"равны, это же exe", ours(myExe, "0.25.0"), myExe, "0.25.0", []string{myExe}, false, false, false},
		{"пред-выпуск ниже выпуска", ours(oldExe, "0.25.0-rc2"), myExe, "0.25.0", []string{oldExe}, true, false, false},
		{"я пред-выпуск, у них выпуск", ours(oldExe, "0.25.0"), myExe, "0.25.0-rc2", []string{oldExe}, false, false, false},
		{"0.9 против 0.10 числом", ours(oldExe, "0.9.0"), myExe, "0.10.0", []string{oldExe}, true, false, false},
		// Их версия не разбирается, моя — да: update.Newer тут дал бы false, а план пишет.
		{"их версия не разбирается", ours(oldExe, "dev"), myExe, "0.25.0", []string{oldExe}, true, false, false},
		{"их версии нет вовсе", ours(oldExe, ""), myExe, "0.25.0", []string{oldExe}, true, false, false},
		{"моя не разбирается, их да", ours(oldExe, "0.1.0"), myExe, "dev", []string{oldExe}, false, false, false},
		{"обе не разбираются", ours(oldExe, "dev"), myExe, "dev", []string{oldExe}, false, false, false},
		{"чужой ProgID", osx.AssocState{ProgID: foreignProg}, myExe, "0.25.0", nil, false, true, false},
		{"чужой ProgID и наш выбор «всегда»", osx.AssocState{ProgID: foreignProg, UserChoice: appsOld}, myExe, "0.25.0", nil, false, true, true},
		{"равны: я установлена, они zip", osx.AssocState{ProgID: progOurs, Exe: oldExe, AppVersion: "0.25.0", InstallDir: instDir},
			instExe, "0.25.0", []string{oldExe, instExe}, true, false, false},
		{"равны: я zip, они установлены", osx.AssocState{ProgID: progOurs, Exe: instExe, AppVersion: "0.25.0", InstallDir: instDir},
			myExe, "0.25.0", []string{instExe, myExe}, false, false, false},
		{"равны: обе установлены", osx.AssocState{ProgID: progOurs, Exe: instExe, AppVersion: "0.25.0", InstallDir: instDir},
			instExe, "0.25.0", []string{instExe}, false, false, false},
		// AppVersion нет (записал установщик без неё) — версия установки.
		{"версия из установки: новее", osx.AssocState{ProgID: progOurs, Exe: instExe, InstallDir: instDir, InstallVersion: "0.26.0"},
			myExe, "0.25.0", []string{instExe}, false, false, false},
		{"версия из установки: старее", osx.AssocState{ProgID: progOurs, Exe: instExe, InstallDir: instDir, InstallVersion: "0.24.0"},
			myExe, "0.25.0", []string{instExe}, true, false, false},
		{"выбор «всегда» на старой версии", osx.AssocState{ProgID: progOurs, Exe: myExe, AppVersion: "0.25.0", UserChoice: appsOld},
			myExe, "0.25.0", []string{myExe}, false, false, true},
		{"чужой выбор не трогаем", osx.AssocState{ProgID: progOurs, Exe: oldExe, AppVersion: "0.24.0", UserChoice: appsEtc},
			myExe, "0.25.0", []string{oldExe}, true, false, false},
	}
	for _, c := range cases {
		got := plan(planIn{St: c.st, Exe: c.exe, Ver: c.ver, Exists: live(c.live...)})
		want := planAct{Write: c.write, Foreign: c.foreignAct, Drop: c.drop}
		if got != want {
			t.Errorf("%s: %+v, ждали %+v", c.name, got, want)
		}
	}
}

func TestUserChoiceOurs(t *testing.T) {
	ex := func(ps ...string) func(string) bool {
		return func(p string) bool { return slices.Contains(ps, p) }
	}
	cases := []struct {
		name string
		st   osx.AssocState
		live []string
		want bool
	}{
		{"выбора нет", osx.AssocState{}, nil, false},
		{"exe старой версии", osx.AssocState{UserChoice: appsOld}, nil, true},
		{"exe без версии в имени, регистр", osx.AssocState{UserChoice: `applications\SOSNINAALBUMS.EXE`}, nil, true},
		{"чужой exe", osx.AssocState{UserChoice: appsEtc}, nil, false},
		{"чужой ProgID", osx.AssocState{UserChoice: "Other.Album"}, nil, false},
		{"имя похоже, но не наше", osx.AssocState{UserChoice: `Applications\notsosninaalbums.exe`}, nil, false},
		{"наш ProgID, команда жива", osx.AssocState{UserChoice: progOurs, Exe: myExe}, []string{myExe}, false},
		{"наш ProgID, exe пропал", osx.AssocState{UserChoice: progOurs, Exe: myExe}, nil, true},
		{"наш ProgID, команды нет", osx.AssocState{UserChoice: progOurs}, nil, true},
	}
	for _, c := range cases {
		if got := userChoiceOurs(c.st, ex(c.live...)); got != c.want {
			t.Errorf("%s: %v, ждали %v", c.name, got, c.want)
		}
	}
}
