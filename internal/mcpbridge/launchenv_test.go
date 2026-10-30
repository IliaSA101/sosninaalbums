package mcpbridge

import (
	"reflect"
	"testing"

	"github.com/IliaSA101/sosninaalbums/internal/osx"
)

func TestChildEnv(t *testing.T) {
	cases := []struct {
		name    string
		goos    string
		environ []string
		want    []string
	}{
		{"nil — пустой, но не nil", "linux", nil, []string{}},
		{"пустой срез", "linux", []string{}, []string{}},
		{"Unix: только список", "linux", []string{
			"PATH=/bin", "HOME=/h", "USER=u", "LOGNAME=u", "LANG=C", "LC_ALL=C", "LC_CTYPE=C", "DISPLAY=:0",
			"WAYLAND_DISPLAY=w", "XDG_RUNTIME_DIR=/r", "XDG_CONFIG_HOME=/c", "XDG_DATA_HOME=/d", "XDG_CACHE_HOME=/k",
			"DBUS_SESSION_BUS_ADDRESS=x", "WEBVIEW2_X=1", "LD_LIBRARY_PATH=/x", "SHELL=/bin/sh", "TERM=xterm",
		}, []string{
			"PATH=/bin", "HOME=/h", "USER=u", "LOGNAME=u", "LANG=C", "LC_ALL=C", "LC_CTYPE=C", "DISPLAY=:0",
			"WAYLAND_DISPLAY=w", "XDG_RUNTIME_DIR=/r", "XDG_CONFIG_HOME=/c", "XDG_DATA_HOME=/d", "XDG_CACHE_HOME=/k",
			"DBUS_SESSION_BUS_ADDRESS=x",
		}},
		{"Unix: похожие имена не проходят", "linux", []string{
			"LC_=x", "XDG__HOME=x", "XDG_HOME=x", "XDG_SESSION_TYPE=x", "XDG_CURRENT_DESKTOP=x", "path=/x", "HOMEX=x",
			"GTK_PATH=x", "WEBKIT_DISABLE_COMPOSITING_MODE=1", "DYLD_INSERT_LIBRARIES=x", "GODEBUG=x",
		}, []string{}},
		{"без знака равенства и с пустым именем", "linux", []string{"PATH", "=x", "", "HOME=/h"}, []string{"HOME=/h"}},
		{"значение с равенством и пустое", "linux", []string{"LANG=a=b", "USER="}, []string{"LANG=a=b", "USER="}},
		{"Windows: без учёта регистра", "windows", []string{
			"Path=C:\\x", "SYSTEMROOT=C:\\Windows", "windir=C:\\Windows", "UserProfile=C:\\U", "APPDATA=a", "LocalAppData=l",
			"TEMP=t", "tmp=t", "ProgramData=p", "WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS=--remote-debugging-port=1",
			"HOME=/x", "DISPLAY=:0", "LC_ALL=C", "=C:=C:\\", "COMPUTERNAME=c", "GODEBUG=x",
		}, []string{
			"Path=C:\\x", "SYSTEMROOT=C:\\Windows", "windir=C:\\Windows", "UserProfile=C:\\U", "APPDATA=a", "LocalAppData=l",
			"TEMP=t", "tmp=t", "ProgramData=p",
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := childEnv(c.environ, c.goos)
			if got == nil || !reflect.DeepEqual(got, c.want) {
				t.Errorf("childEnv = %#v, ждали %#v", got, c.want)
			}
		})
	}
}

// SA_STAND доходит до ребёнка только у стенда QA (сборка dev/qa и заданная переменная); в выпуске
// и без переменной — нет. Проверяется в обеих сборках: go test и go test -tags dev.
func TestChildEnvStand(t *testing.T) {
	environ := []string{"PATH=/bin", "SA_STAND=b"}
	t.Setenv(osx.StandEnv, "b")
	want := []string{"PATH=/bin"}
	if osx.DevBuild {
		want = []string{"PATH=/bin", "SA_STAND=b"}
	}
	for _, goos := range []string{"linux", "darwin"} {
		if got := childEnv(environ, goos); !reflect.DeepEqual(got, want) {
			t.Errorf("%s: childEnv = %#v, ждали %#v (DevBuild=%v)", goos, got, want, osx.DevBuild)
		}
	}
	if osx.DevBuild { // Windows — имя без учёта регистра
		if got := childEnv([]string{"Sa_Stand=b"}, "windows"); len(got) != 1 {
			t.Errorf("windows: %#v", got)
		}
	}
	// Переменная не задана у самой программы: стенда нет, в ребёнка не идёт, как бы ни выглядел environ.
	t.Setenv(osx.StandEnv, "")
	if got := childEnv(environ, "linux"); !reflect.DeepEqual(got, []string{"PATH=/bin"}) {
		t.Errorf("без стенда: %#v", got)
	}
}

func TestAppBundle(t *testing.T) {
	cases := map[string]string{
		"/Applications/Sosnina.app/Contents/MacOS/Sosnina":           "/Applications/Sosnina.app",
		"/Users/u/Sosnina Albums.app/Contents/MacOS/app":             "/Users/u/Sosnina Albums.app",
		"/Applications/Sosnina.app/Contents/MacOS/sub/app":           "",
		"/Applications/Sosnina.app/Contents/Resources/app":           "",
		"/opt/sosninaalbums/SosninaAlbums":                           "",
		".app/Contents/MacOS/x":                                      "",
		"":                                                           "",
		"/a.app/Contents/MacOS/b.app/Contents/MacOS/c":               "/a.app/Contents/MacOS/b.app",
		"/Applications/Sosnina.app/Contents/MacOS/":                  "/Applications/Sosnina.app",
		"/Applications/Sosnina.application/Contents/MacOS/Sosnina":   "",
		"/tmp/x.app/Contents/MacOS/../../../../../../bin/sh/Sosnina": "",
	}
	for exe, want := range cases {
		if got := appBundle(exe); got != want {
			t.Errorf("appBundle(%q) = %q, ждали %q", exe, got, want)
		}
	}
}
