package agentplaces

// Облака по настройкам клиентов (160, design §4): обёртка над osx.CloudRoots с фильтром локальности и сроком и над
// osx.VirtualCloud. Строковые корни OneDrive и iCloud — CloudRoots в places.go.

import (
	"runtime"
	"slices"
	"time"

	"github.com/IliaSA101/sosninaalbums/internal/app/appkit"
	"github.com/IliaSA101/sosninaalbums/internal/osx"
	"github.com/IliaSA101/sosninaalbums/internal/paths"
)

// cloudBudget — срок чтения настроек клиентов (реестр и два файла — доли мс, дольше — повисший профиль на сети);
// переменная — тест сокращает.
var cloudBudget = 700 * time.Millisecond

// osCloudRoots — шов тестов: настоящее чтение настроек клиентов.
var osCloudRoots = osx.CloudRoots

// CloudRootsLive — CloudRoots (окружение) и корни из настроек клиентов этой машины: Dropbox, Яндекс.Диск, реестр
// Windows. Для другой ОС, чем у процесса (таблицы тестов), настроек нет — только окружение. Не уложились в срок —
// только окружение (корней меньше, а не больше: облако реже признано, место не появляется само).
func CloudRootsLive(goos, home string, env func(string) string) []string {
	out := CloudRoots(goos, home, env)
	if goos != runtime.GOOS {
		return out
	}
	read := osCloudRoots // подмену читаем до горутины: она может пережить срок
	extra, _ := appkit.Within(cloudBudget, func() []string { return read(home, paths.CheckLocalPath) })
	for _, p := range extra {
		if !slices.Contains(out, p) {
			out = append(out, p)
		}
	}
	return out
}

// osVirtualCloud — шов тестов: признак виртуального облачного диска.
var osVirtualCloud = osx.VirtualCloud

// VirtualCloud — путь на виртуальном облачном диске (Google Drive, Яндекс.Диск, pCloud, Box на Windows). Читает
// сведения о томе — вызывать под сроком вызывающего (Within). Тихий путь считает такой диск облаком (160, §4).
func VirtualCloud(path string) bool { return osVirtualCloud(path) }
